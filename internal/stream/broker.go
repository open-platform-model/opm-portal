package stream

import (
	"cmp"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// Errors Open, Subscribe and Unsubscribe return. The handler maps them to
// HTTP statuses; the read API maps them to problem documents.
var (
	// ErrUnauthenticated: the session names no principal. No review was sent.
	ErrUnauthenticated = errors.New("stream: unauthenticated")
	// ErrTooManyStreams: the session or the process holds its cap of streams.
	ErrTooManyStreams = errors.New("stream: too many streams")
	// ErrTooManyTopics: the stream would carry more topics than its cap.
	ErrTooManyTopics = errors.New("stream: too many topics")
	// ErrTopicNotServed: the topic parses, but nothing serves it.
	ErrTopicNotServed = errors.New("stream: topic not served")
	// ErrNoStream: no stream with that id belongs to the session. A stream of
	// another session gets the same answer as a missing one.
	ErrNoStream = errors.New("stream: no such stream")
	// ErrClosed: the broker has shut down.
	ErrClosed = errors.New("stream: broker closed")
	// ErrSlowConsumer ends a stream whose client fell behind.
	ErrSlowConsumer = errors.New("stream: client fell behind")
	// ErrReplaced ends a stream's connection when a reconnect takes it over.
	ErrReplaced = errors.New("stream: replaced by a reconnect")
	// ErrDisconnected ends a stream's connection when the client goes away.
	ErrDisconnected = errors.New("stream: client disconnected")
	// ErrIdle ends a stream that carried no topic for the idle timeout.
	ErrIdle = errors.New("stream: idle")
	// ErrNotAdmitted is what an Admitter returns to refuse a topic. The
	// topic is closed with the code a denial gives, so a refusal reads the
	// same as a missing permission.
	ErrNotAdmitted = errors.New("stream: topic not admitted")
)

// Closing codes a closed message carries. They are the read API's problem
// codes.
const (
	CodeForbidden           = "forbidden"
	CodeUnauthenticated     = "unauthenticated"
	CodeUpstreamUnavailable = "upstream_unavailable"
)

// Producer is the read model's side of the broker. See the package
// documentation for the contract it must keep.
type Producer interface {
	// Attributes returns the reads a subscriber must be allowed before t
	// attaches, such as get on the one instance an instance topic shows, or
	// list in the topic's scope for a list topic. ok is false when the
	// producer does not serve t; an empty slice is treated the same way, so
	// a topic is never attached unchecked.
	Attributes(t Topic) (attrs []authz.Attributes, ok bool)
	// Snapshot returns t's current items. The broker gates each one per
	// subscriber.
	Snapshot(ctx context.Context, t Topic) ([]Item, error)
	// Activate tells the producer t has its first subscriber. The broker
	// calls the returned release, once, when t loses its last subscriber.
	// Each Activate gets its own release.
	Activate(t Topic) (release func())
}

// Admitter is an optional extension of Producer, for topics that need more
// than their reads: a Pod log topic, for example, is served only for a Pod an
// OPM inventory the subscriber may read reaches (0030:D10:R1). The broker
// calls Admit after every read Attributes names for t is allowed for who, and
// before t attaches, whenever it authorizes a topic: on Open, Subscribe and
// a reconnect. grants are those reads' grants, in Attributes order. Returning
// ErrNotAdmitted (or wrapping it) closes the topic with the code a denial
// gives; a *authz.DenialError closes it as that denial would; any other error
// closes it with upstream_unavailable. Nothing is activated for a refused
// topic.
type Admitter interface {
	Admit(ctx context.Context, who authz.Identity, t Topic, grants []authz.Grant) error
}

// Follower is an optional extension of Producer. The broker calls Follow
// after a stream newly subscribes to t, on Open or Subscribe, when t was
// already active: after the subscription attached, past every cap and
// admission check. It is never called for the subscription that activates
// t, whose Activate stands for it, nor on a reconnect, which resumes the
// subscriptions a stream already holds. A log producer uses it to start a
// new read on a topic whose read ended, so only an explicit re-follow does.
type Follower interface {
	Follow(t Topic)
}

// Session is who opens a stream: the session it belongs to and the identity
// every read on it is authorized for. Key identifies the session for caps and
// stream ownership; the broker never logs it.
type Session struct {
	Key      string
	Identity authz.Identity
}

// Options tune a Broker. Zero fields take their defaults.
type Options struct {
	// MaxStreamsPerSession caps a session's streams. Default 2, so a
	// browser on HTTP/1.1 keeps four of its six connections for requests.
	MaxStreamsPerSession int
	// MaxStreams caps the process's streams. Default 500.
	MaxStreams int
	// MaxTopicsPerStream caps the topics on one stream. Default 32.
	MaxTopicsPerStream int
	// MaxLogTopicsPerSession caps the log topics one session follows
	// across its streams. Default 4.
	MaxLogTopicsPerSession int
	// MaxLogTopics caps the distinct log topics the process serves at
	// once; each holds one upstream log stream. Default 50.
	MaxLogTopics int
	// RingSize is how many recent changes each topic keeps for resume.
	// Default 1000.
	RingSize int
	// LogRingBytes caps the item data a log topic's resume ring keeps, so
	// a topic of log lines cannot hold RingSize of them. Default 2 MiB.
	// Object and list topics' rings are bounded by RingSize alone.
	LogRingBytes int
	// QueueSize bounds a stream's undelivered messages; a full queue
	// evicts the stream's connection. Default 256.
	QueueSize int
	// HeartbeatInterval spaces heartbeats. Default 15 seconds.
	HeartbeatInterval time.Duration
	// IdleTimeout closes a stream that has carried no topic this long.
	// Default 30 minutes.
	IdleTimeout time.Duration
	// ResumeWindow is how long a disconnected stream stays resumable.
	// Default 1 minute.
	ResumeWindow time.Duration
	// WriteTimeout bounds one write to the client. Default 10 seconds.
	WriteTimeout time.Duration
	// RevalidateTimeout bounds how long the grants of one message are
	// re-validated before the write; a message not confirmed by then closes
	// its topic. Default 30 seconds, one decision lifetime at the
	// authorizer's default.
	RevalidateTimeout time.Duration
	// Logger receives operational logs. Default: discarded.
	Logger *slog.Logger
}

func (o Options) withDefaults() Options {
	setInt := func(v *int, d int) {
		if *v <= 0 {
			*v = d
		}
	}
	setDur := func(v *time.Duration, d time.Duration) {
		if *v <= 0 {
			*v = d
		}
	}
	setInt(&o.MaxStreamsPerSession, 2)
	setInt(&o.MaxStreams, 500)
	setInt(&o.MaxTopicsPerStream, 32)
	setInt(&o.MaxLogTopicsPerSession, 4)
	setInt(&o.MaxLogTopics, 50)
	setInt(&o.RingSize, 1000)
	setInt(&o.LogRingBytes, 2<<20)
	setInt(&o.QueueSize, 256)
	setDur(&o.HeartbeatInterval, 15*time.Second)
	setDur(&o.IdleTimeout, 30*time.Minute)
	setDur(&o.ResumeWindow, time.Minute)
	setDur(&o.WriteTimeout, 10*time.Second)
	setDur(&o.RevalidateTimeout, 30*time.Second)
	if o.Logger == nil {
		o.Logger = slog.New(slog.DiscardHandler)
	}
	return o
}

// Broker fans published changes out to streams, each filtered for the
// identity that opened it.
type Broker struct {
	producer Producer
	az       authz.Authorizer
	opts     Options
	log      *slog.Logger
	epoch    string

	mu      sync.Mutex
	closed  bool
	seq     uint64
	topics  map[Topic]*topicState
	streams map[string]*streamState
}

// topicState is one topic with at least one subscribing stream.
type topicState struct {
	ring    *ring
	subs    map[*streamState]*subscription
	release func()
}

// subscription is one stream's hold on one topic. grants holds one grant per
// attribute, in order; it is guarded by Broker.mu.
type subscription struct {
	topic       Topic
	attrs       []authz.Attributes
	grants      []authz.Grant
	snapshotSeq uint64
}

// streamState is a stream as the broker tracks it. conn is nil while the
// stream is detached.
type streamState struct {
	id         string
	session    string
	who        authz.Identity
	subs       map[Topic]*subscription
	conn       *conn
	detachedAt time.Time
	emptySince time.Time
	expiry     *time.Timer
	// lastID is the last event id the stream handed out. Ids count this
	// stream's own events, so their gaps reveal nothing about other topics,
	// other streams or the items left out of this one (0030:D7:R2).
	lastID uint64
	// marks maps the stream's most recent event ids to the broker sequence
	// each one delivered, oldest first, so a resume finds where it stopped.
	marks []idMark
	// pending holds a closed message, one per topic, for every topic denied
	// or closed on the stream that its client has not been sent yet. An
	// entry leaves only once a connection wrote it (or the client removed or
	// regained the topic), so a denial queued on a connection that then ends
	// reaches the next connection first.
	pending []entry
}

// idMark ties one event id of a stream to the broker sequence it carried.
type idMark struct {
	id, seq uint64
}

// conn is one attachment of a stream to a client connection.
type conn struct {
	queue   chan entry
	backlog []entry
	done    chan struct{}
	reason  error
	served  bool
}

type entryKind int

const (
	entItem entryKind = iota
	entSnapshot
	entClosed
)

// entry is one message waiting for a stream's writer.
type entry struct {
	kind  entryKind
	seq   uint64
	topic Topic
	sub   *subscription
	item  Item
	code  string
}

// New returns a Broker that serves p's topics, authorizing every
// subscription and delivery with az.
func New(p Producer, az authz.Authorizer, opts Options) *Broker {
	opts = opts.withDefaults()
	return &Broker{
		producer: p,
		az:       az,
		opts:     opts,
		log:      opts.Logger,
		epoch:    rand.Text()[:8],
		topics:   map[Topic]*topicState{},
		streams:  map[string]*streamState{},
	}
}

// Publish records items as changes on t and queues them for t's subscribers.
// A topic nobody follows drops them: a later subscriber starts from a
// snapshot. Publish never blocks on a subscriber; one whose queue is full
// loses its connection. It refuses the whole call when an item is invalid.
func (b *Broker) Publish(t Topic, items ...Item) error {
	for i := range items {
		if err := items[i].validate(); err != nil {
			return fmt.Errorf("publishing to %s: %w", t, err)
		}
	}
	var release []func()
	b.mu.Lock()
	if tp := b.topics[t]; tp != nil {
		for i := range items {
			it := &items[i]
			b.seq++
			tp.ring.add(ringEntry{seq: b.seq, item: *it})
			for st, sub := range tp.subs {
				if st.conn != nil {
					release = append(release, b.enqueueLocked(st, entry{kind: entItem, seq: b.seq, topic: t, sub: sub, item: *it})...)
				}
			}
		}
	}
	b.mu.Unlock()
	runAll(release)
	return nil
}

// enqueueLocked queues e for st's writer, evicting st's connection when its
// queue is full.
func (b *Broker) enqueueLocked(st *streamState, e entry) []func() {
	select {
	case st.conn.queue <- e:
		return nil
	default:
		b.log.Info("evicting a stream that fell behind", "stream", st.id)
		return b.endConnLocked(st, ErrSlowConsumer)
	}
}

// checkTopics refuses what can never attach and returns each topic's
// attributes, deduplicated in request order. The cap is checked before the
// producer is asked about any topic.
func (b *Broker) checkTopics(topics []Topic) ([]Topic, map[Topic][]authz.Attributes, error) {
	out := make([]Topic, 0, len(topics))
	for _, t := range topics {
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	if len(out) > b.opts.MaxTopicsPerStream {
		return nil, nil, ErrTooManyTopics
	}
	attrs := make(map[Topic][]authz.Attributes, len(out))
	for _, t := range out {
		if t.Kind() == "" {
			return nil, nil, fmt.Errorf("%w: the zero topic", ErrTopicNotServed)
		}
		a, ok := b.producer.Attributes(t)
		if !ok || len(a) == 0 {
			return nil, nil, fmt.Errorf("%w: %s", ErrTopicNotServed, t)
		}
		if t.Kind() == KindInstances && !isListRead(t, a) {
			// A list topic is served under the list grant a GET list needs
			// and nothing else (0030:D7:R2); a producer naming any other
			// read is not trusted with it.
			return nil, nil, fmt.Errorf("%w: %s (the producer names no list read for it)", ErrTopicNotServed, t)
		}
		attrs[t] = a
	}
	return out, attrs, nil
}

// verbList is the verb of a list topic's read.
const verbList = "list"

// isListRead reports whether attrs is exactly the read a list topic t
// needs: one list of InstancesResource in t's namespace (cluster-wide for
// "instances"), naming no subresource and no object.
func isListRead(t Topic, attrs []authz.Attributes) bool {
	if len(attrs) != 1 {
		return false
	}
	a := attrs[0]
	return a.Verb == verbList && a.Resource == InstancesResource() &&
		a.Subresource == "" && a.Name == "" && a.Namespace == t.Namespace()
}

// authorized is one topic's subscription decision.
type authorized struct {
	topic  Topic
	attrs  []authz.Attributes
	grants []authz.Grant
	code   string // non-empty when denied
}

// authorize checks every attribute of every topic for who. It runs outside
// the lock: a review may take seconds.
func (b *Broker) authorize(ctx context.Context, who authz.Identity, topics []Topic, attrs map[Topic][]authz.Attributes) []authorized {
	out := make([]authorized, 0, len(topics))
	for _, t := range topics {
		a := authorized{topic: t, attrs: attrs[t]}
		for _, req := range a.attrs {
			g, err := b.az.Check(ctx, who, req)
			if err != nil {
				a.code, a.grants = closeCode(err), nil
				break
			}
			a.grants = append(a.grants, g)
		}
		if a.code == "" {
			a.code = b.admit(ctx, who, t, a.grants)
			if a.code != "" {
				a.grants = nil
			}
		}
		out = append(out, a)
	}
	return out
}

// admit asks the producer, when it is an Admitter, whether who may follow t
// now that t's reads are allowed. It returns a closing code, or "".
func (b *Broker) admit(ctx context.Context, who authz.Identity, t Topic, grants []authz.Grant) string {
	ad, ok := b.producer.(Admitter)
	if !ok {
		return ""
	}
	err := ad.Admit(ctx, who, t, grants)
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrNotAdmitted):
		return CodeForbidden
	}
	if code := closeCode(err); code != CodeUpstreamUnavailable {
		return code
	}
	b.log.Warn("admitting a topic failed", "topic", t.String(), "error", err)
	return CodeUpstreamUnavailable
}

// closeCode maps an authorization failure to a closing code. Anything that
// is not a definite denial is upstream_unavailable.
func closeCode(err error) string {
	d, ok := errors.AsType[*authz.DenialError](err)
	if !ok {
		return CodeUpstreamUnavailable
	}
	switch d.Code {
	case authz.CodeForbidden, authz.CodeInvalid:
		return CodeForbidden
	case authz.CodeUnauthenticated:
		return CodeUnauthenticated
	case authz.CodeUnavailable:
		return CodeUpstreamUnavailable
	}
	return CodeUpstreamUnavailable
}

// Open opens a stream for s carrying topics. Each topic is authorized for
// s.Identity before it attaches; a denied topic is not attached and the
// stream's first messages close it.
//
// lastEventID is the client's Last-Event-ID header, or "". When it names a
// stream of the same session and identity, from this broker, that is still
// within its resume window, Open reattaches that stream instead: its own
// topics (not the ones given) are authorized again, and each gets the
// changes after lastEventID its ring still holds, or a fresh snapshot. A
// stream still attached elsewhere is taken over. Anything else opens a new
// stream. The given topics are checked only when a new stream opens.
func (b *Broker) Open(ctx context.Context, s Session, topics []Topic, lastEventID string) (*Stream, error) {
	if s.Key == "" || !s.Identity.Authenticated() {
		return nil, ErrUnauthenticated
	}
	if st, after, ok := b.resumable(s, lastEventID); ok {
		stream, err := b.reattach(ctx, st, after)
		if !errors.Is(err, ErrNoStream) {
			return stream, err
		}
		// The stream expired while it was being authorized: open afresh.
	}
	topics, attrs, err := b.checkTopics(topics)
	if err != nil {
		return nil, err
	}

	b.mu.Lock()
	release, err := b.makeRoomLocked(s.Key)
	if err == nil {
		err = b.logCapLocked(s.Key, topics)
	}
	b.mu.Unlock()
	runAll(release)
	if err != nil {
		// Refused before any review is sent.
		return nil, err
	}
	decisions := b.authorize(ctx, s.Identity, topics, attrs)

	b.mu.Lock()
	release, err = b.makeRoomLocked(s.Key)
	if err == nil {
		err = b.logCapLocked(s.Key, topics)
	}
	if err != nil {
		b.mu.Unlock()
		runAll(release)
		return nil, err
	}
	st := &streamState{
		id:      rand.Text(),
		session: s.Key,
		who:     s.Identity,
		subs:    map[Topic]*subscription{},
	}
	b.streams[st.id] = st
	c := b.newConn()
	st.conn = c
	activate, follow := b.attachLocked(st, decisions, func(e entry) { c.backlog = append(c.backlog, e) })
	b.mu.Unlock()

	runAll(release)
	b.activate(activate)
	b.follow(follow)
	return &Stream{b: b, st: st, conn: c}, nil
}

// makeRoomLocked makes room for one more stream of session, discarding the
// oldest detached stream of the session, then of the process, when a cap is
// reached. It fails with ErrTooManyStreams when there is nothing to discard.
func (b *Broker) makeRoomLocked(session string) ([]func(), error) {
	if b.closed {
		return nil, ErrClosed
	}
	var release []func()
	inSession := func(st *streamState) bool { return st.session == session }
	if b.countLocked(inSession) >= b.opts.MaxStreamsPerSession {
		old := b.oldestDetachedLocked(inSession)
		if old == nil {
			return nil, ErrTooManyStreams
		}
		release = append(release, b.deleteLocked(old)...)
	}
	if len(b.streams) >= b.opts.MaxStreams {
		old := b.oldestDetachedLocked(func(*streamState) bool { return true })
		if old == nil {
			return release, ErrTooManyStreams
		}
		release = append(release, b.deleteLocked(old)...)
	}
	return release, nil
}

func (b *Broker) countLocked(match func(*streamState) bool) int {
	n := 0
	for _, st := range b.streams {
		if match(st) {
			n++
		}
	}
	return n
}

func (b *Broker) oldestDetachedLocked(match func(*streamState) bool) *streamState {
	var oldest *streamState
	for _, st := range b.streams {
		if st.conn != nil || !match(st) {
			continue
		}
		if oldest == nil || st.detachedAt.Before(oldest.detachedAt) {
			oldest = st
		}
	}
	return oldest
}

// resumable finds the stream lastEventID names, if s may resume it, and the
// broker sequence the client last received. An event id the stream no longer
// remembers resumes at sequence 0, so every topic gets a fresh snapshot.
func (b *Broker) resumable(s Session, lastEventID string) (st *streamState, after uint64, ok bool) {
	epoch, id, local, ok := parseEventID(lastEventID)
	if !ok || epoch != b.epoch {
		return nil, 0, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	st = b.streams[id]
	if st == nil || st.session != s.Key || !sameIdentity(st.who, s.Identity) {
		return nil, 0, false
	}
	if i, found := slices.BinarySearchFunc(st.marks, local, func(m idMark, id uint64) int { return cmp.Compare(m.id, id) }); found {
		return st, st.marks[i].seq, true
	}
	return st, 0, true
}

// nextIDLocked hands out st's next event id for an event carrying broker
// sequence seq, remembering the pair for a resume.
func (b *Broker) nextIDLocked(st *streamState, seq uint64) uint64 {
	st.lastID++
	if len(st.marks) >= b.opts.QueueSize {
		st.marks = slices.Delete(st.marks, 0, len(st.marks)-b.opts.QueueSize+1)
	}
	st.marks = append(st.marks, idMark{id: st.lastID, seq: seq})
	return st.lastID
}

// parseEventID splits an id written by Stream.eventID.
func parseEventID(id string) (epoch, stream string, seq uint64, ok bool) {
	parts := strings.Split(id, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" {
		return "", "", 0, false
	}
	seq, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return "", "", 0, false
	}
	return parts[0], parts[1], seq, true
}

// sameIdentity compares two identities with group and extra-value order
// ignored, as authz does.
func sameIdentity(a, b authz.Identity) bool {
	sorted := func(v []string) []string { return slices.Sorted(slices.Values(v)) }
	return a.Username == b.Username && a.UID == b.UID &&
		slices.Equal(sorted(a.Groups), sorted(b.Groups)) &&
		maps.EqualFunc(a.Extra, b.Extra, func(x, y []string) bool { return slices.Equal(sorted(x), sorted(y)) })
}

// resumeTopicLocked returns what sub's client missed after sequence after:
// the ring's changes, or a fresh snapshot when the client never had the
// topic's snapshot or the ring lost part of the gap.
func (b *Broker) resumeTopicLocked(sub *subscription, after uint64) (replay []entry, snapshot *entry) {
	tp := b.topics[sub.topic]
	if after < sub.snapshotSeq || after > b.seq || !tp.ring.covers(after) {
		b.seq++
		sub.snapshotSeq = b.seq
		return nil, &entry{kind: entSnapshot, seq: b.seq, topic: sub.topic, sub: sub}
	}
	held := tp.ring.since(after)
	replay = make([]entry, 0, len(held))
	for i := range held {
		replay = append(replay, entry{kind: entItem, seq: held[i].seq, topic: sub.topic, sub: sub, item: held[i].item})
	}
	return replay, nil
}

// reattach gives st a new connection that continues after sequence after.
// It returns ErrNoStream when st is gone by the time its topics are
// authorized again.
func (b *Broker) reattach(ctx context.Context, st *streamState, after uint64) (*Stream, error) {
	b.mu.Lock()
	topics := make([]Topic, 0, len(st.subs))
	attrs := make(map[Topic][]authz.Attributes, len(st.subs))
	for t, sub := range st.subs {
		topics = append(topics, t)
		attrs[t] = sub.attrs
	}
	who := st.who
	b.mu.Unlock()
	decisions := b.authorize(ctx, who, topics, attrs)

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, ErrClosed
	}
	if b.streams[st.id] != st {
		b.mu.Unlock()
		return nil, ErrNoStream
	}
	if old := st.conn; old != nil {
		old.reason = ErrReplaced
		close(old.done)
	}
	if st.expiry != nil {
		st.expiry.Stop()
		st.expiry = nil
	}
	c := b.newConn()
	st.conn = c

	var replay, snapshots []entry
	var release []func()
	decided := make(map[Topic]*authorized, len(decisions))
	for i := range decisions {
		decided[decisions[i].topic] = &decisions[i]
	}
	for _, t := range slices.SortedFunc(maps.Keys(st.subs), func(x, y Topic) int { return strings.Compare(x.String(), y.String()) }) {
		sub := st.subs[t]
		if d := decided[t]; d != nil {
			if d.code != "" {
				release = append(release, b.dropSubLocked(st, sub)...)
				pendLocked(st, t, d.code)
				continue
			}
			sub.grants = d.grants
		}
		items, snapshot := b.resumeTopicLocked(sub, after)
		replay = append(replay, items...)
		if snapshot != nil {
			snapshots = append(snapshots, *snapshot)
		}
	}
	slices.SortFunc(replay, func(x, y entry) int { return cmp.Compare(x.seq, y.seq) })
	// Pending closings stay pending until this connection writes them.
	c.backlog = slices.Concat(st.pending, replay, snapshots)
	if len(st.subs) == 0 && st.emptySince.IsZero() {
		st.emptySince = time.Now()
	}
	b.mu.Unlock()
	runAll(release)
	return &Stream{b: b, st: st, conn: c}, nil
}

func (b *Broker) newConn() *conn {
	return &conn{queue: make(chan entry, b.opts.QueueSize), done: make(chan struct{})}
}

// attachLocked registers the allowed topics on st, hands put a snapshot
// marker for each and a closed entry for each denied one, which is also kept
// pending until written, and returns the topics that need activating and
// the already active ones st newly follows.
func (b *Broker) attachLocked(st *streamState, decisions []authorized, put func(entry)) (activate, follow []Topic) {
	for i := range decisions {
		d := &decisions[i]
		if d.code != "" {
			put(pendLocked(st, d.topic, d.code))
			continue
		}
		if _, held := st.subs[d.topic]; held {
			continue
		}
		unpendLocked(st, d.topic)
		tp := b.topics[d.topic]
		switch {
		case tp == nil:
			ringBytes := 0
			if d.topic.Kind() == KindLog {
				ringBytes = b.opts.LogRingBytes
			}
			tp = &topicState{ring: newRing(b.opts.RingSize, ringBytes, b.seq), subs: map[*streamState]*subscription{}}
			b.topics[d.topic] = tp
			activate = append(activate, d.topic)
		case tp.release != nil:
			// While t's activation is still pending it stands for this
			// subscription too, so only an active topic is followed.
			follow = append(follow, d.topic)
		}
		b.seq++
		sub := &subscription{topic: d.topic, attrs: d.attrs, grants: d.grants, snapshotSeq: b.seq}
		tp.subs[st] = sub
		st.subs[d.topic] = sub
		put(entry{kind: entSnapshot, seq: b.seq, topic: d.topic, sub: sub})
	}
	if len(st.subs) > 0 {
		st.emptySince = time.Time{}
	} else if st.emptySince.IsZero() {
		st.emptySince = time.Now()
	}
	return activate, follow
}

// logCapLocked refuses adding the log topics among fresh to session when
// the session would follow more than MaxLogTopicsPerSession log topics, or
// the process would serve more than MaxLogTopics distinct ones. fresh holds
// topics none of the session's streams it is added to carries yet; a topic
// another subscriber already follows does not count again for the process.
func (b *Broker) logCapLocked(session string, fresh []Topic) error {
	var asked, newToProcess int
	for _, t := range fresh {
		if t.Kind() != KindLog {
			continue
		}
		asked++
		if b.topics[t] == nil {
			newToProcess++
		}
	}
	if asked == 0 {
		return nil
	}
	held := 0
	for _, st := range b.streams {
		if st.session != session {
			continue
		}
		for t := range st.subs {
			if t.Kind() == KindLog {
				held++
			}
		}
	}
	active := 0
	for t := range b.topics {
		if t.Kind() == KindLog {
			active++
		}
	}
	if held+asked > b.opts.MaxLogTopicsPerSession || active+newToProcess > b.opts.MaxLogTopics {
		return ErrTooManyTopics
	}
	return nil
}

// pendLocked records that st's client must be told topic closed with code,
// replacing an older closing of the same topic, and returns the entry to
// queue.
func pendLocked(st *streamState, topic Topic, code string) entry {
	e := entry{kind: entClosed, topic: topic, code: code}
	st.pending = append(slices.DeleteFunc(st.pending, func(p entry) bool { return p.topic == topic }), e)
	return e
}

// unpendLocked forgets the closing of topic st has not sent yet.
func unpendLocked(st *streamState, topic Topic) {
	st.pending = slices.DeleteFunc(st.pending, func(p entry) bool { return p.topic == topic })
}

// heldLocked counts what st holds towards its topic cap: its topics, and its
// unsent closings except those of topics about to be asked for again, which
// an allow removes and a denial replaces.
func heldLocked(st *streamState, asked []Topic) int {
	n := len(st.subs)
	for i := range st.pending {
		if !slices.Contains(asked, st.pending[i].topic) {
			n++
		}
	}
	return n
}

// activate asks the producer to start each topic and records its release.
// A topic dropped before its activation is recorded is released at once.
func (b *Broker) activate(topics []Topic) {
	for _, t := range topics {
		release := b.producer.Activate(t)
		b.mu.Lock()
		tp := b.topics[t]
		if tp != nil && tp.release == nil {
			tp.release = release
			release = nil
		}
		b.mu.Unlock()
		if release != nil {
			release()
		}
	}
}

// follow tells the producer, when it is a Follower, that each topic gained
// a subscriber.
func (b *Broker) follow(topics []Topic) {
	f, ok := b.producer.(Follower)
	if !ok {
		return
	}
	for _, t := range topics {
		f.Follow(t)
	}
}

// lookupLocked finds the stream id belonging to s: the same session and the
// same identity, as for a resume.
func (b *Broker) lookupLocked(s Session, id string) (*streamState, error) {
	if b.closed {
		return nil, ErrClosed
	}
	st := b.streams[id]
	if st == nil || st.session != s.Key || !sameIdentity(st.who, s.Identity) {
		return nil, ErrNoStream
	}
	return st, nil
}

// Subscribe attaches topics to an open stream of s. Topics are authorized for
// the identity the stream was opened with, which s must match; a denied topic
// is closed on the stream, at once or, when the connection is gone before
// the closing is written, first thing on the next connection. Topics the
// stream already carries are left as they are. A closing not yet written
// counts towards the cap.
func (b *Broker) Subscribe(ctx context.Context, s Session, streamID string, topics ...Topic) error {
	topics, attrs, err := b.checkTopics(topics)
	if err != nil {
		return err
	}
	b.mu.Lock()
	st, err := b.lookupLocked(s, streamID)
	if err != nil {
		b.mu.Unlock()
		return err
	}
	who := st.who
	fresh := topics[:0:0]
	for _, t := range topics {
		if st.subs[t] == nil {
			fresh = append(fresh, t)
		}
	}
	held := heldLocked(st, fresh)
	capErr := b.logCapLocked(st.session, fresh)
	b.mu.Unlock()
	if held+len(fresh) > b.opts.MaxTopicsPerStream || capErr != nil {
		// Refused before any review is sent.
		return ErrTooManyTopics
	}
	decisions := b.authorize(ctx, who, fresh, attrs)

	b.mu.Lock()
	if cur, lerr := b.lookupLocked(s, streamID); lerr != nil || cur != st {
		b.mu.Unlock()
		return ErrNoStream
	}
	if heldLocked(st, fresh)+len(fresh) > b.opts.MaxTopicsPerStream || b.logCapLocked(st.session, fresh) != nil {
		b.mu.Unlock()
		return ErrTooManyTopics
	}
	var release []func()
	activate, follow := b.attachLocked(st, decisions, func(e entry) {
		// While detached nothing is queued: a closing is pending already,
		// and reattach snapshots a topic whose marker the client missed.
		// An eviction here loses neither, for the same reasons.
		if st.conn != nil {
			release = append(release, b.enqueueLocked(st, e)...)
		}
	})
	b.mu.Unlock()
	runAll(release)
	b.activate(activate)
	b.follow(follow)
	return nil
}

// Unsubscribe detaches topics from a stream of s, and forgets any closing of
// them the client has not been sent. A topic the stream does not carry is
// ignored.
func (b *Broker) Unsubscribe(s Session, streamID string, topics ...Topic) error {
	b.mu.Lock()
	st, err := b.lookupLocked(s, streamID)
	if err != nil {
		b.mu.Unlock()
		return err
	}
	var release []func()
	for _, t := range topics {
		if sub := st.subs[t]; sub != nil {
			release = append(release, b.dropSubLocked(st, sub)...)
		}
		unpendLocked(st, t)
	}
	b.mu.Unlock()
	runAll(release)
	return nil
}

// dropSubLocked removes sub from st and drops its topic when nobody else
// holds it, returning the release to call after unlocking.
func (b *Broker) dropSubLocked(st *streamState, sub *subscription) []func() {
	if st.subs[sub.topic] != sub {
		return nil
	}
	delete(st.subs, sub.topic)
	if len(st.subs) == 0 {
		st.emptySince = time.Now()
	}
	tp := b.topics[sub.topic]
	if tp == nil {
		return nil
	}
	delete(tp.subs, st)
	if len(tp.subs) > 0 {
		return nil
	}
	delete(b.topics, sub.topic)
	if tp.release != nil {
		return []func(){tp.release}
	}
	return nil
}

// endConnLocked ends st's current connection with reason and detaches st.
// A detached stream keeps its topics, so their rings keep recording, and is
// deleted when the resume window passes without a reconnect.
func (b *Broker) endConnLocked(st *streamState, reason error) []func() {
	c := st.conn
	if c == nil {
		return nil
	}
	c.reason = reason
	close(c.done)
	st.conn = nil
	if errors.Is(reason, ErrIdle) || errors.Is(reason, ErrClosed) {
		return b.deleteLocked(st)
	}
	at := time.Now()
	st.detachedAt = at
	if st.expiry != nil {
		st.expiry.Stop()
	}
	st.expiry = time.AfterFunc(b.opts.ResumeWindow, func() { b.expire(st, at) })
	return nil
}

// expire deletes st if it is still detached since at.
func (b *Broker) expire(st *streamState, at time.Time) {
	b.mu.Lock()
	var release []func()
	if st.conn == nil && st.detachedAt.Equal(at) {
		release = b.deleteLocked(st)
	}
	b.mu.Unlock()
	runAll(release)
}

// deleteLocked forgets st and every topic it held.
func (b *Broker) deleteLocked(st *streamState) []func() {
	if b.streams[st.id] != st {
		return nil
	}
	delete(b.streams, st.id)
	if st.expiry != nil {
		st.expiry.Stop()
	}
	if c := st.conn; c != nil {
		c.reason = ErrClosed
		close(c.done)
		st.conn = nil
	}
	var release []func()
	for _, sub := range st.subs {
		release = append(release, b.dropSubLocked(st, sub)...)
	}
	return release
}

// disconnect runs when a writer returns with reason: the connection c is
// gone. A stream ended for any reason but idleness stays resumable.
func (b *Broker) disconnect(st *streamState, c *conn, reason error) {
	if !errors.Is(reason, ErrIdle) {
		reason = ErrDisconnected
	}
	b.mu.Lock()
	var release []func()
	if st.conn == c {
		release = b.endConnLocked(st, reason)
	}
	b.mu.Unlock()
	runAll(release)
}

// Close ends every stream and releases every topic. Open and Subscribe fail
// afterwards; Publish becomes a no-op.
func (b *Broker) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	var release []func()
	for _, st := range b.streams {
		release = append(release, b.deleteLocked(st)...)
	}
	b.mu.Unlock()
	runAll(release)
}

func runAll(fns []func()) {
	for _, f := range fns {
		f()
	}
}
