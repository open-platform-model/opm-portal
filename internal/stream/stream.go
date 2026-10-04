package stream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
// or the stream idles out. It returns why it ended. Serve is called once.
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
	// work done for it: snapshots, reviews and renders.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-c.done:
			cancel()
		case <-ctx.Done():
		}
	}()
	wr := &writer{w: w, rc: http.NewResponseController(w), timeout: b.opts.WriteTimeout}
	reason := s.run(ctx, wr)
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

// ended returns why the connection ended: the broker's reason when it ended
// the connection, or the context's error.
func (s *Stream) ended(ctx context.Context) error {
	select {
	case <-s.conn.done:
		return s.conn.reason
	default:
		return ctx.Err()
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
		return wr.closed(e.topic, e.code)
	case entSnapshot:
		return s.deliverSnapshot(ctx, wr, e)
	case entItem:
		return s.deliverItem(ctx, wr, e)
	}
	return nil
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
	payloads := make([]json.RawMessage, 0, len(items))
	for i := range items {
		it := &items[i]
		if err := it.validate(); err != nil {
			s.b.log.Warn("dropping an invalid snapshot item", "topic", e.topic.String(), "error", err)
			continue
		}
		data, code := s.payload(ctx, e.sub, it)
		if code != "" {
			return s.closeTopic(ctx, wr, e.sub, code)
		}
		if data != nil {
			payloads = append(payloads, data)
		}
	}
	body, err := json.Marshal(struct {
		Topic string            `json:"topic"`
		Items []json.RawMessage `json:"items"`
	}{e.topic.String(), payloads})
	if err != nil {
		s.b.log.Warn("snapshot payload is not JSON", "topic", e.topic.String(), "error", err)
		return s.closeTopic(ctx, wr, e.sub, CodeUpstreamUnavailable)
	}
	id, ok := s.eventID(e.sub, e.seq)
	if !ok {
		return nil
	}
	return wr.event("", id, EventSnapshot, body)
}

func (s *Stream) deliverItem(ctx context.Context, wr *writer, e *entry) error {
	if !s.current(e.sub) {
		return nil
	}
	if code := s.gateTopic(ctx, e.sub); code != "" {
		return s.closeTopic(ctx, wr, e.sub, code)
	}
	data, code := s.payload(ctx, e.sub, &e.item)
	if code != "" {
		return s.closeTopic(ctx, wr, e.sub, code)
	}
	if data == nil {
		return nil
	}
	body, err := json.Marshal(struct {
		Topic string          `json:"topic"`
		Item  json.RawMessage `json:"item"`
	}{e.topic.String(), data})
	if err != nil {
		s.b.log.Warn("item payload is not JSON", "topic", e.topic.String(), "error", err)
		return s.closeTopic(ctx, wr, e.sub, CodeUpstreamUnavailable)
	}
	id, ok := s.eventID(e.sub, e.seq)
	if !ok {
		return nil
	}
	return wr.event("", id, e.item.Event, body)
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

// payload gates one item for the stream's identity and renders it. It
// returns nil data for an item the reader may not see, or a closing code
// when the decision or the render failed.
func (s *Stream) payload(ctx context.Context, sub *subscription, it *Item) (data json.RawMessage, code string) {
	b, who := s.b, s.st.who
	if !s.coveredByTopic(sub, it.Attrs) {
		if _, err := b.az.Check(ctx, who, it.Attrs); err != nil {
			if code := closeCode(err); code == CodeUpstreamUnavailable {
				return nil, code
			}
			// Forbidden items are left out without a trace (0030:D7:R2).
			return nil, ""
		}
	}
	if it.Render == nil {
		return it.Data, ""
	}
	data, err := it.Render(ctx, who)
	if err != nil {
		b.log.Warn("rendering an item failed", "topic", sub.topic.String(), "error", err)
		return nil, CodeUpstreamUnavailable
	}
	return data, ""
}

func (s *Stream) coveredByTopic(sub *subscription, req authz.Attributes) bool {
	s.b.mu.Lock()
	grants := append([]authz.Grant(nil), sub.grants...)
	s.b.mu.Unlock()
	for _, g := range grants {
		if g.Covers(s.st.who, req) == nil {
			return true
		}
	}
	return false
}

// closeTopic drops sub from the stream and tells the client. It does
// neither when the connection ended, since a failed review or snapshot is
// then the connection's end and not the topic's, or when sub is no longer
// the stream's subscription to its topic: the client is not told a topic it
// holds again has closed.
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
	s.b.mu.Unlock()
	runAll(release)
	return wr.closed(sub.topic, code)
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

// writer writes server-sent events and flushes each one.
type writer struct {
	w       http.ResponseWriter
	rc      *http.ResponseController
	timeout time.Duration
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

// event writes one event. prefix is written first (a retry field); data
// must be one line of JSON, which json.Marshal guarantees.
func (wr *writer) event(prefix, id, name string, data []byte) error {
	if bytes.ContainsAny(data, "\r\n") || strings.ContainsAny(id+name, "\r\n") {
		return errors.New("stream: event field spans lines")
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
