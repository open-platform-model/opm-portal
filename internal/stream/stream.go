package stream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// retryMillis is the reconnect delay a stream asks its client to use.
const retryMillis = 3000

// Stream is one open connection of a stream. Serve writes it to the client.
type Stream struct {
	b    *Broker
	st   *streamState
	conn *conn
}

// ID returns the stream's id: the client names it to add or remove topics,
// and every event id carries it, so a reconnect finds the stream again.
func (s *Stream) ID() string { return s.st.id }

// Serve writes the stream to w as server-sent events until ctx is done, the
// broker closes, the client falls behind, a reconnect takes the stream over,
// the stream idles out, or its session expires, which it ends with an
// expired event. It returns why it ended. Serve is called once.
// The caller has written no body yet; Serve sets no headers (the handler
// does).
func (s *Stream) Serve(ctx context.Context, w http.ResponseWriter) error {
	b, c := s.b, s.conn
	b.mu.Lock()
	if c.served {
		b.mu.Unlock()
		return errors.New("stream: Serve called twice")
	}
	c.served = true
	b.mu.Unlock()

	// Ending the connection (eviction, takeover, Close) also cancels the
	// work done for it: snapshots, reviews and renders. So does the
	// session's end, and from then on the writer refuses every event but
	// the expired one, so no work still running at the end, and no message
	// already queued, reaches the client after it.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	b.mu.Lock()
	ends := s.st.expires
	b.mu.Unlock()
	if !ends.IsZero() {
		var stop context.CancelFunc
		ctx, stop = context.WithDeadlineCause(ctx, ends, errSessionOver)
		defer stop()
	}
	go func() {
		select {
		case <-c.done:
			cancel()
		case <-ctx.Done():
		}
	}()
	wr := &writer{w: w, rc: http.NewResponseController(w), timeout: b.opts.WriteTimeout, ends: ends}
	reason := s.run(ctx, wr)
	if errors.Is(reason, errSessionOver) {
		reason = s.expire(wr)
	}
	b.disconnect(s.st, c, reason)
	return reason
}

func (s *Stream) run(ctx context.Context, wr *writer) error {
	c := s.conn
	open, err := json.Marshal(struct {
		Stream string `json:"stream"`
	}{s.st.id})
	if err != nil {
		return fmt.Errorf("encoding the open event: %w", err)
	}
	if err := wr.event("retry: "+strconv.Itoa(retryMillis)+"\n", "", EventOpen, open); err != nil {
		return err
	}
	ticker := time.NewTicker(s.b.opts.HeartbeatInterval)
	defer ticker.Stop()

	backlog := c.backlog
	c.backlog = nil
	for i := range backlog {
		if err := s.deliver(ctx, wr, &backlog[i]); err != nil {
			return err
		}
	}
	for {
		select {
		case <-ctx.Done():
			return s.ended(ctx)
		case <-c.done:
			return c.reason
		case e := <-c.queue:
			if err := s.deliver(ctx, wr, &e); err != nil {
				return err
			}
		case <-ticker.C:
			if err := s.heartbeat(ctx, wr); err != nil {
				return err
			}
		}
	}
}

// heartbeat re-checks every topic's grants, so a revoked permission closes a
// quiet topic, ends an idle stream, and writes the heartbeat.
func (s *Stream) heartbeat(ctx context.Context, wr *writer) error {
	b := s.b
	b.mu.Lock()
	if b.streams[s.st.id] != s.st || s.st.conn != s.conn {
		b.mu.Unlock()
		return nil
	}
	subs := make([]*subscription, 0, len(s.st.subs))
	for _, sub := range s.st.subs {
		subs = append(subs, sub)
	}
	idle := len(subs) == 0 && !s.st.emptySince.IsZero() &&
		time.Since(s.st.emptySince) >= b.opts.IdleTimeout
	b.mu.Unlock()
	if idle {
		return ErrIdle
	}
	for _, sub := range subs {
		if code := s.gateTopic(ctx, sub); code != "" {
			if err := s.closeTopic(ctx, wr, sub, code); err != nil {
				return err
			}
		}
	}
	return wr.event("", "", EventHeartbeat, []byte("{}"))
}

// expire ends the stream at its session's end: the client is told, and the
// stream is discarded, so nothing more reaches it and a reconnect resumes
// nothing.
func (s *Stream) expire(wr *writer) error {
	if err := wr.expired(); err != nil {
		// The stream is discarded whether or not the client heard.
		return fmt.Errorf("%w: %w", ErrSessionExpired, err)
	}
	return ErrSessionExpired
}

// ended returns why the connection ended: the broker's reason when it ended
// the connection, or the context's cause (errSessionOver at the session's
// end).
func (s *Stream) ended(ctx context.Context) error {
	select {
	case <-s.conn.done:
		return s.conn.reason
	default:
		return context.Cause(ctx)
	}
}

// current reports whether this connection still serves the stream and sub
// is still the stream's subscription to its topic. Entries for a topic
// removed, or removed and added again, are dropped, and a connection a
// reconnect took over delivers nothing more.
func (s *Stream) current(sub *subscription) bool {
	s.b.mu.Lock()
	defer s.b.mu.Unlock()
	return s.st.conn == s.conn && s.st.subs[sub.topic] == sub
}

func (s *Stream) deliver(ctx context.Context, wr *writer, e *entry) error {
	switch e.kind {
	case entClosed:
		return s.writeClosed(wr, e.topic)
	case entSnapshot:
		return s.deliverSnapshot(ctx, wr, e)
	case entItem:
		return s.deliverItem(ctx, wr, e)
	}
	return nil
}

// part is one item of a message: its rendered document, the read it reveals
// and, when it was reviewed on its own, the grant that review gave it. An
// item within the scope of the topic's reads has no grant of its own: it
// rides the topic's grants.
type part struct {
	data  json.RawMessage
	attrs authz.Attributes
	own   bool
	grant authz.Grant
}

// message is one snapshot or item event of sub's topic, together with every
// grant it was built under: the topic's (sub.grants) and each part's own.
// send re-validates all of them right before the write.
type message struct {
	sub      *subscription
	seq      uint64
	snapshot bool
	event    string // the item's event; EventSnapshot for a snapshot
	parts    []part
}

func (s *Stream) deliverSnapshot(ctx context.Context, wr *writer, e *entry) error {
	if !s.current(e.sub) {
		return nil
	}
	if code := s.gateTopic(ctx, e.sub); code != "" {
		return s.closeTopic(ctx, wr, e.sub, code)
	}
	items, err := s.b.producer.Snapshot(ctx, e.topic)
	if err != nil {
		if ctx.Err() != nil {
			return s.ended(ctx)
		}
		s.b.log.Warn("snapshot failed", "topic", e.topic.String(), "error", err)
		return s.closeTopic(ctx, wr, e.sub, CodeUpstreamUnavailable)
	}
	m := &message{sub: e.sub, seq: e.seq, snapshot: true, event: EventSnapshot, parts: make([]part, 0, len(items))}
	for i := range items {
		it := &items[i]
		if err := it.validate(); err != nil {
			s.b.log.Warn("dropping an invalid snapshot item", "topic", e.topic.String(), "error", err)
			continue
		}
		p, ok, code := s.payload(ctx, e.sub, it)
		if code != "" {
			return s.closeTopic(ctx, wr, e.sub, code)
		}
		if ok {
			m.parts = append(m.parts, p)
		}
	}
	return s.send(ctx, wr, m)
}

func (s *Stream) deliverItem(ctx context.Context, wr *writer, e *entry) error {
	if !s.current(e.sub) {
		return nil
	}
	if code := s.gateTopic(ctx, e.sub); code != "" {
		return s.closeTopic(ctx, wr, e.sub, code)
	}
	p, ok, code := s.payload(ctx, e.sub, &e.item)
	if code != "" {
		return s.closeTopic(ctx, wr, e.sub, code)
	}
	if !ok {
		return nil
	}
	return s.send(ctx, wr, &message{sub: e.sub, seq: e.seq, event: e.item.Event, parts: []part{p}})
}

// send writes a snapshot or an item event. It is the only path by which
// either reaches the client (TestOnlySendWritesTopicData holds it to that).
// Right before the event id and the write it re-validates, in this one
// place, every grant the message was built under (revalidate), so a message
// is written right after an in-memory pass confirms that every decision it
// used is unexpired: a slow Snapshot, render or review cannot outlive a
// grant unnoticed.
func (s *Stream) send(ctx context.Context, wr *writer, m *message) error {
	if code := s.revalidate(ctx, m); code != "" {
		return s.closeTopic(ctx, wr, m.sub, code)
	}
	topic := m.sub.topic.String()
	var body []byte
	var err error
	if m.snapshot {
		items := make([]json.RawMessage, len(m.parts))
		for i := range m.parts {
			items[i] = m.parts[i].data
		}
		body, err = json.Marshal(struct {
			Topic string            `json:"topic"`
			Items []json.RawMessage `json:"items"`
		}{topic, items})
	} else {
		if len(m.parts) != 1 {
			// The item was left out: it is not written, and leaves no trace.
			return nil
		}
		body, err = json.Marshal(struct {
			Topic string          `json:"topic"`
			Item  json.RawMessage `json:"item"`
		}{topic, m.parts[0].data})
	}
	if err != nil {
		s.b.log.Warn("message payload is not JSON", "topic", topic, "error", err)
		return s.closeTopic(ctx, wr, m.sub, CodeUpstreamUnavailable)
	}
	id, ok := s.eventID(m.sub, m.seq)
	if !ok {
		return nil
	}
	return wr.event("", id, m.event, body)
}

// revalidate makes sure every grant m was built under covers its read, and
// returns "" only right after a pass that confirms it in memory: the pass
// (covered) makes no review call, and send does no I/O between it and the
// write. Any other pass asks again for every grant that has expired, the
// topic's first (gateTopic), then each part's own, and loops, since a slow
// review can outlast a grant that covered when it was looked at.
//
// Re-validation is bounded by time, not by rounds: it runs for at most
// Options.RevalidateTimeout (one decision lifetime by default), so however
// many parts a message carries it cannot keep the stream silent for longer.
// No review starts after the deadline, a review still running at it is
// canceled, and a message not confirmed by then closes its topic with
// upstream_unavailable. A fresh grant covers the read it was asked for, so
// a pass that does not confirm either makes progress or meets the deadline.
//
// The guarantee is: every message is written right after an in-memory pass
// confirms that every decision it used is unexpired. A decision lives one
// TTL (30 s by default) from when its review answers, so a revocation
// reaches the stream within the TTL plus one review: about 35 s with the
// default 5 s review timeout. The moment between that pass and the write
// is inherent to check-then-write.
//
// A topic denial or error returns its closing code. A part reviewed on its
// own that is now forbidden is dropped from m, so a snapshot is written
// without it and an item event not at all, without a trace (0030:D7:R2); any
// other code for it closes the topic. It returns a closing code, or "".
func (s *Stream) revalidate(parent context.Context, m *message) string {
	ctx, cancel := context.WithTimeout(parent, s.b.opts.RevalidateTimeout)
	defer cancel()
	for {
		if s.covered(m) {
			return ""
		}
		if ctx.Err() != nil {
			if parent.Err() == nil {
				s.b.log.Warn("grants kept expiring while a message was re-validated", "topic", m.sub.topic.String())
			}
			return CodeUpstreamUnavailable
		}
		if code := s.gateTopic(ctx, m.sub); code != "" {
			return code
		}
		if code := s.recheckParts(ctx, m); code != "" {
			return code
		}
	}
}

// covered reports whether every topic grant of m's subscription and every
// own grant of its parts covers its read now. It makes no review call.
func (s *Stream) covered(m *message) bool {
	b, who, sub := s.b, s.st.who, m.sub
	b.mu.Lock()
	for i, req := range sub.attrs {
		if sub.grants[i].Covers(who, req) != nil {
			b.mu.Unlock()
			return false
		}
	}
	b.mu.Unlock()
	for i := range m.parts {
		if p := &m.parts[i]; p.own && p.grant.Covers(who, p.attrs) != nil {
			return false
		}
	}
	return true
}

// recheckParts asks again for every own grant of m's parts that has
// expired. A part now forbidden is dropped from m; any other code is
// returned as the topic's closing code.
func (s *Stream) recheckParts(ctx context.Context, m *message) string {
	b, who := s.b, s.st.who
	kept := m.parts[:0]
	for i := range m.parts {
		p := &m.parts[i]
		if p.own && p.grant.Covers(who, p.attrs) != nil {
			if ctx.Err() != nil {
				// No review starts after the deadline or the connection's end.
				return CodeUpstreamUnavailable
			}
			g, err := b.az.Check(ctx, who, p.attrs)
			if err != nil {
				if code := closeCode(err); code != CodeForbidden {
					return code
				}
				continue
			}
			p.grant = g
		}
		kept = append(kept, *p)
	}
	m.parts = kept
	return ""
}

// gateTopic makes sure every grant of sub still covers its read, asking
// again for any that expired. It returns a closing code, or "".
func (s *Stream) gateTopic(ctx context.Context, sub *subscription) string {
	b, who := s.b, s.st.who
	for i, req := range sub.attrs {
		b.mu.Lock()
		g := sub.grants[i]
		b.mu.Unlock()
		if g.Covers(who, req) == nil {
			continue
		}
		if ctx.Err() != nil {
			// No review starts after the deadline or the connection's end.
			return CodeUpstreamUnavailable
		}
		g, err := b.az.Check(ctx, who, req)
		if err != nil {
			return closeCode(err)
		}
		b.mu.Lock()
		sub.grants[i] = g
		b.mu.Unlock()
	}
	return ""
}

// payload gates one item for the stream's identity and renders it, returning
// it as a part of a message with the grant it was read under. ok is false
// for an item the reader may not see; code is a closing code when the
// decision or the render failed.
//
// An item within the scope of the topic's own reads needs no review of its
// own: it rides the topic's grants, which are gated again here so a denial
// stops a slow snapshot before the next render. On a list topic nothing else
// is delivered and nothing is reviewed: a list carries only the items within
// the scope of its list grant, as a GET list does (0030:D7:R2), so no review
// per item is sent and none can fail. On an object topic an item that
// reveals another read is reviewed on its own, and the part keeps that
// grant. Whatever is checked here, send re-validates every grant before the
// write.
func (s *Stream) payload(ctx context.Context, sub *subscription, it *Item) (p part, ok bool, code string) {
	b, who := s.b, s.st.who
	p.attrs = it.Attrs
	switch {
	case withinTopic(sub.attrs, it.Attrs):
		// gateTopic asks again only for a grant that has expired.
		if code := s.gateTopic(ctx, sub); code != "" {
			return part{}, false, code
		}
	case sub.topic.Kind() == KindInstances:
		// A producer fault: the item lies outside the topic's list read.
		b.log.Warn("dropping a list item outside the topic's list read", "topic", sub.topic.String())
		return part{}, false, ""
	default:
		g, err := b.az.Check(ctx, who, it.Attrs)
		if err != nil {
			if code := closeCode(err); code != CodeForbidden {
				return part{}, false, code
			}
			// Forbidden items are left out without a trace (0030:D7:R2).
			return part{}, false, ""
		}
		p.own, p.grant = true, g
	}
	if it.Render == nil {
		p.data = it.Data
		return p, true, ""
	}
	data, err := it.Render(ctx, who)
	if err != nil {
		b.log.Warn("rendering an item failed", "topic", sub.topic.String(), "error", err)
		return part{}, false, CodeUpstreamUnavailable
	}
	p.data = data
	return p, true, ""
}

// withinTopic reports whether one of the topic's reads covers req by scope
// alone, by the rule Grant.Covers applies (authz.Attributes.Covers).
func withinTopic(reads []authz.Attributes, req authz.Attributes) bool {
	return slices.ContainsFunc(reads, func(have authz.Attributes) bool { return have.Covers(req) })
}

// closeTopic drops sub from the stream and tells the client, keeping the
// closing pending until it is written so a connection that ends first
// passes it to the next one. It does neither when the connection ended,
// since a failed review or snapshot is then the connection's end and not the
// topic's, or when sub is no longer the stream's subscription to its topic:
// the client is not told a topic it holds again has closed.
func (s *Stream) closeTopic(ctx context.Context, wr *writer, sub *subscription, code string) error {
	if ctx.Err() != nil {
		return s.ended(ctx)
	}
	s.b.mu.Lock()
	if s.st.conn != s.conn || s.st.subs[sub.topic] != sub {
		s.b.mu.Unlock()
		return nil
	}
	release := s.b.dropSubLocked(s.st, sub)
	pendLocked(s.st, sub.topic, code)
	s.b.mu.Unlock()
	runAll(release)
	return s.writeClosed(wr, sub.topic)
}

// writeClosed writes topic's pending closing and then forgets it. Nothing is
// written when no closing of topic is pending any more (the client removed
// or regained the topic, or another write already sent it) or when this
// connection no longer serves the stream.
func (s *Stream) writeClosed(wr *writer, topic Topic) error {
	b := s.b
	b.mu.Lock()
	i := slices.IndexFunc(s.st.pending, func(p entry) bool { return p.topic == topic })
	if i < 0 || s.st.conn != s.conn {
		b.mu.Unlock()
		return nil
	}
	code := s.st.pending[i].code
	b.mu.Unlock()
	if err := wr.closed(topic, code); err != nil {
		return err
	}
	b.mu.Lock()
	s.st.pending = slices.DeleteFunc(s.st.pending, func(p entry) bool { return p.topic == topic && p.code == code })
	b.mu.Unlock()
	return nil
}

// eventID hands out the stream's next event id for an event of sub carrying
// broker sequence seq. It encodes the broker epoch, the stream and a count of
// the stream's own events, so a reconnect finds the stream and where it
// stopped without the id revealing anything published elsewhere. ok is false
// when this connection no longer serves the stream or sub was replaced: the
// event is then not written.
func (s *Stream) eventID(sub *subscription, seq uint64) (id string, ok bool) {
	b := s.b
	b.mu.Lock()
	defer b.mu.Unlock()
	if s.st.conn != s.conn || s.st.subs[sub.topic] != sub {
		return "", false
	}
	return b.epoch + "." + s.st.id + "." + strconv.FormatUint(b.nextIDLocked(s.st, seq), 10), true
}

// errSessionOver is why a connection ends at its session's end: the
// writer refuses an event, or the context's deadline passes. Serve turns it
// into the expired event and ErrSessionExpired.
var errSessionOver = errors.New("stream: the session is over")

// writer writes server-sent events and flushes each one. From ends on (the
// session's end; zero for none) it writes nothing but the expired event.
type writer struct {
	w       http.ResponseWriter
	rc      *http.ResponseController
	timeout time.Duration
	ends    time.Time
	buf     bytes.Buffer
}

func (wr *writer) closed(t Topic, code string) error {
	body, err := json.Marshal(struct {
		Topic string `json:"topic"`
		Code  string `json:"code"`
	}{t.String(), code})
	if err != nil {
		return fmt.Errorf("encoding a closed event: %w", err)
	}
	return wr.event("", "", EventClosed, body)
}

// expired writes the expired event. It carries the stream's own code only.
func (wr *writer) expired() error {
	body, err := json.Marshal(struct {
		Code string `json:"code"`
	}{CodeUnauthenticated})
	if err != nil {
		return fmt.Errorf("encoding the expired event: %w", err)
	}
	return wr.event("", "", EventExpired, body)
}

// event writes one event. prefix is written first (a retry field); data
// must be one line of JSON, which json.Marshal guarantees.
func (wr *writer) event(prefix, id, name string, data []byte) error {
	if bytes.ContainsAny(data, "\r\n") || strings.ContainsAny(id+name, "\r\n") {
		return errors.New("stream: event field spans lines")
	}
	if name != EventExpired && !wr.ends.IsZero() && !time.Now().Before(wr.ends) {
		return errSessionOver
	}
	if err := wr.rc.SetWriteDeadline(time.Now().Add(wr.timeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return fmt.Errorf("setting the write deadline: %w", err)
	}
	wr.buf.Reset()
	wr.buf.WriteString(prefix)
	if id != "" {
		wr.buf.WriteString("id: " + id + "\n")
	}
	wr.buf.WriteString("event: " + name + "\ndata: ")
	wr.buf.Write(data)
	wr.buf.WriteString("\n\n")
	if _, err := wr.w.Write(wr.buf.Bytes()); err != nil {
		return fmt.Errorf("writing an event: %w", err)
	}
	if err := wr.rc.Flush(); err != nil {
		return fmt.Errorf("flushing an event: %w", err)
	}
	return nil
}
