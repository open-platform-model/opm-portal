package logs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

var pods = schema.GroupVersionResource{Version: "v1", Resource: "pods"}

// Reacher answers whether an OPM inventory the caller may read reaches a
// Pod. *readmodel.Model implements it.
type Reacher interface {
	ReachPod(ctx context.Context, who authz.Identity, g authz.Grant, namespace, pod string) (readmodel.PodReach, error)
}

// Publisher receives the producer's items. *stream.Broker implements it.
type Publisher interface {
	Publish(t stream.Topic, items ...stream.Item) error
}

// Options bound each log topic. Zero fields take their defaults.
type Options struct {
	// TailLines is the initial tail asked for. Default 200, at most 2000.
	TailLines int64
	// MaxLineBytes caps one line, its timestamp included; a longer line is
	// cut and marked. Default 16 KiB.
	MaxLineBytes int
	// MaxTailBytes caps the initial tail: its newest lines within the cap
	// are sent, the older ones skipped and counted. Default 1 MiB.
	MaxTailBytes int
	// LinesPerSecond and LineBurst bound live lines. Default 200 and 500.
	LinesPerSecond float64
	LineBurst      int
	// BytesPerSecond and ByteBurst bound live bytes. Default 256 KiB and
	// 1 MiB.
	BytesPerSecond float64
	ByteBurst      int
	// Buffer is how many recent messages a snapshot carries. Default 500.
	Buffer int
	// BufferBytes caps the encoded size of the messages a snapshot
	// carries; the oldest leave first. Default 1 MiB.
	BufferBytes int
	// MarkerDelay is how long a dropped line may wait for its marker when
	// no delivered line follows, and how long the initial tail may stay
	// quiet before it is sent. Default 250ms.
	MarkerDelay time.Duration
	// Logger receives operational logs, never log content. Default:
	// discarded.
	Logger *slog.Logger
	// Now is the clock. Default time.Now.
	Now func() time.Time
}

// maxTailLines caps Options.TailLines.
const maxTailLines = 2000

func (o Options) withDefaults() Options {
	if o.TailLines <= 0 {
		o.TailLines = 200
	}
	o.TailLines = min(o.TailLines, maxTailLines)
	setInt := func(v *int, d int) {
		if *v <= 0 {
			*v = d
		}
	}
	setInt(&o.MaxLineBytes, 16<<10)
	setInt(&o.MaxTailBytes, 1<<20)
	setInt(&o.LineBurst, 500)
	setInt(&o.ByteBurst, 1<<20)
	setInt(&o.Buffer, 500)
	setInt(&o.BufferBytes, 1<<20)
	if o.MarkerDelay <= 0 {
		o.MarkerDelay = 250 * time.Millisecond
	}
	if o.LinesPerSecond <= 0 {
		o.LinesPerSecond = 200
	}
	if o.BytesPerSecond <= 0 {
		o.BytesPerSecond = 256 << 10
	}
	if o.Logger == nil {
		o.Logger = slog.New(slog.DiscardHandler)
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return o
}

// Config wires a Producer. Every field but Options is required, and Reader
// must name a principal.
type Config struct {
	// Source reads Pods and logs as Reader.
	Source Source
	// Reach decides whether an inventory reaches a Pod for the caller.
	Reach Reacher
	// Authorizer checks Reader's own access before each upstream read.
	Authorizer authz.Authorizer
	// Reader is the identity Source reads as.
	Reader authz.Identity
	Options
}

// Producer serves log topics to a stream.Broker. It implements
// stream.Producer, stream.Admitter and stream.Follower.
type Producer struct {
	cfg  Config
	opts Options
	log  *slog.Logger

	// seq numbers every message the producer emits, across reads and
	// activations, so a later read's messages always sort after an earlier
	// one's.
	seq atomic.Uint64

	mu    sync.Mutex
	pub   Publisher
	tails map[stream.Topic]*tail
}

var (
	_ stream.Producer = (*Producer)(nil)
	_ stream.Admitter = (*Producer)(nil)
	_ stream.Follower = (*Producer)(nil)
)

// New returns a Producer for cfg. Call SetPublisher before the broker
// activates a topic.
func New(cfg Config) (*Producer, error) {
	switch {
	case cfg.Source == nil:
		return nil, errors.New("logs: no source")
	case cfg.Reach == nil:
		return nil, errors.New("logs: no reacher")
	case cfg.Authorizer == nil:
		return nil, errors.New("logs: no authorizer")
	case !cfg.Reader.Authenticated():
		return nil, errors.New("logs: the reader identity has no username")
	}
	opts := cfg.withDefaults()
	return &Producer{cfg: cfg, opts: opts, log: opts.Logger, tails: map[stream.Topic]*tail{}}, nil
}

// SetPublisher sets where the producer publishes, normally the broker that
// serves its topics.
func (p *Producer) SetPublisher(pub Publisher) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pub = pub
}

func (p *Producer) publisher() Publisher {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pub
}

// active returns t's current activation, or nil.
func (p *Producer) active(t stream.Topic) *tail {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.tails[t]
}

// podLogRead is a log topic's one read: get on the Pod's log.
func podLogRead(t stream.Topic) authz.Attributes {
	return authz.Attributes{Verb: "get", Resource: pods, Subresource: "log", Namespace: t.Namespace(), Name: t.Name()}
}

// Attributes implements stream.Producer: a log topic needs get pods/log on
// its Pod (0030:D10:R2). Other kinds are not served.
func (p *Producer) Attributes(t stream.Topic) ([]authz.Attributes, bool) {
	if t.Kind() != stream.KindLog {
		return nil, false
	}
	return []authz.Attributes{podLogRead(t)}, true
}

// Admit implements stream.Admitter: the topic attaches only when an OPM
// inventory who may read reaches the Pod (0030:D10:R1). It runs after the
// broker allowed the topic's read, so nothing is looked up for a caller
// without it, and every refusal is the same one. Admit starts nothing: it
// also runs on a reconnect, which never reopens an ended read.
func (p *Producer) Admit(ctx context.Context, who authz.Identity, t stream.Topic, grants []authz.Grant) error {
	if t.Kind() != stream.KindLog || len(grants) != 1 {
		return stream.ErrNotAdmitted
	}
	_, err := p.cfg.Reach.ReachPod(ctx, who, grants[0], t.Namespace(), t.Name())
	switch {
	case err == nil:
		return nil
	case errors.Is(err, readmodel.ErrNotReachable), errors.Is(err, readmodel.ErrNotCovered):
		return stream.ErrNotAdmitted
	}
	return fmt.Errorf("deciding whether an inventory reaches the pod: %w", err)
}

// Follow implements stream.Follower: a new subscription to a topic whose
// read has ended starts a new read, so unsubscribing and subscribing again
// after a logend tails the container afresh, even while other subscribers
// still hold the topic. They receive the new read's messages after their
// logend, with higher seq numbers. A reconnect resumes a subscription and
// is no follow: for it the logend stays the end. A read in progress is left
// as it is.
func (p *Producer) Follow(t stream.Topic) {
	if tl := p.active(t); tl != nil {
		tl.start()
	}
}

// Snapshot implements stream.Producer: the topic's recent messages. A
// message in the snapshot may arrive again as a later item, with the same
// seq.
func (p *Producer) Snapshot(_ context.Context, t stream.Topic) ([]stream.Item, error) {
	tl := p.active(t)
	if tl == nil {
		return []stream.Item{}, nil
	}
	return tl.recent(), nil
}

// Activate implements stream.Producer: it opens the topic's one upstream
// stream. The release closes it and returns once its reader has stopped.
//
// The broker calls a release after it dropped the topic, so a new
// subscriber can activate the topic again before the old activation's
// release ran. The new activation supersedes the old one at once: nothing
// the old one reads afterwards is published. A closed activation leaves
// the topic at once and its snapshot is empty, so a subscriber that
// re-creates the topic never sees the old activation's lines or logend.
func (p *Producer) Activate(t stream.Topic) func() {
	ctx, cancel := context.WithCancel(context.Background())
	tl := &tail{p: p, topic: t, read: podLogRead(t), ctx: ctx, cancel: cancel}
	p.mu.Lock()
	old := p.tails[t]
	p.tails[t] = tl
	p.mu.Unlock()
	if old != nil {
		old.close()
	}
	tl.start()
	var once sync.Once
	return func() {
		once.Do(func() {
			tl.close()
			p.mu.Lock()
			if p.tails[t] == tl {
				delete(p.tails, t)
			}
			p.mu.Unlock()
			tl.runs.Wait()
		})
	}
}

// tail is one activation of a log topic. It reads the container's log
// through one upstream stream at a time; a new follow after a read ended
// starts the next read.
type tail struct {
	p      *Producer
	topic  stream.Topic
	read   authz.Attributes
	ctx    context.Context
	cancel context.CancelFunc
	runs   sync.WaitGroup

	mu       sync.Mutex
	closed   bool // released or superseded: nothing more is read or emitted
	running  bool // a read is in progress and has not emitted its logend
	buf      []stream.Item
	bufBytes int
}

// start begins a read unless one is in progress or the activation is
// closed. A new read starts with an empty snapshot.
func (tl *tail) start() {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	if tl.closed || tl.running {
		return
	}
	tl.running = true
	tl.buf, tl.bufBytes = nil, 0
	tl.runs.Add(1)
	go func() {
		defer tl.runs.Done()
		tl.run(tl.ctx)
	}()
}

// close stops the activation's read and refuses any later emit or start.
// The runs Add under mu before closed is set, so a Wait after close sees
// every read.
func (tl *tail) close() {
	tl.cancel()
	tl.mu.Lock()
	tl.closed = true
	tl.mu.Unlock()
}

// recent returns the snapshot buffer; a closed activation's is empty.
func (tl *tail) recent() []stream.Item {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	if tl.closed {
		return []stream.Item{}
	}
	out := make([]stream.Item, len(tl.buf))
	copy(out, tl.buf)
	return out
}

// emit records m in the buffer, then publishes it, as the producer
// contract requires. Both happen under the tail's lock, so messages are
// published in seq order, and none once the activation is closed. A logend
// ends the read: the next new follow starts a new one.
func (tl *tail) emit(m Message) {
	event := stream.EventLog
	if m.Type == TypeEnd {
		event = stream.EventLogEnd
	}
	m.Container = tl.topic.Container()
	pub := tl.p.publisher()
	tl.mu.Lock()
	defer tl.mu.Unlock()
	if tl.closed {
		return
	}
	m.Seq = tl.p.seq.Add(1)
	data, err := json.Marshal(m)
	if err != nil {
		tl.p.log.Warn("encoding a log message failed", "topic", tl.topic.String(), "error", err)
		return
	}
	it := stream.Item{Event: event, Attrs: tl.read, Data: data}
	tl.keep(it)
	if m.Type == TypeEnd {
		tl.running = false
	}
	if pub == nil {
		return
	}
	if err := pub.Publish(tl.topic, it); err != nil {
		tl.p.log.Warn("publishing a log message failed", "topic", tl.topic.String(), "error", err)
	}
}

// keep adds it to the snapshot buffer, evicting the oldest messages past
// the buffer's count and byte bounds. The newest message is always kept.
func (tl *tail) keep(it stream.Item) {
	o := tl.p.opts
	drop := 0
	bytes := tl.bufBytes + len(it.Data)
	for drop < len(tl.buf) && (len(tl.buf)-drop >= o.Buffer || bytes > o.BufferBytes) {
		bytes -= len(tl.buf[drop].Data)
		drop++
	}
	if drop > 0 {
		clear(tl.buf[:drop])
		tl.buf = append(tl.buf[:0], tl.buf[drop:]...)
	}
	tl.buf = append(tl.buf, it)
	tl.bufBytes = bytes
}

func (tl *tail) end(reason string) { tl.emit(Message{Type: TypeEnd, Reason: reason}) }

// run reads the topic's container log until ctx is done or the log ends.
func (tl *tail) run(ctx context.Context) {
	p, t := tl.p, tl.topic
	if !tl.readerMay(ctx) {
		if ctx.Err() == nil {
			tl.end(ReasonUnavailable)
		}
		return
	}
	pod, err := p.cfg.Source.Pod(ctx, t.Namespace(), t.Name())
	if err != nil {
		tl.failed(ctx, "reading the pod failed", err, ReasonPodNotFound)
		return
	}
	if !hasContainer(pod, t.Container()) {
		tl.end(ReasonContainerNotFound)
		return
	}
	tailLines := p.opts.TailLines
	// LimitBytes is never set: the API server would end a followed stream
	// after that many bytes (0030:D10). The reader bounds the output.
	rc, err := p.cfg.Source.Logs(ctx, t.Namespace(), t.Name(), &corev1.PodLogOptions{
		Container:  t.Container(),
		Follow:     !t.Previous(),
		Previous:   t.Previous(),
		Timestamps: true,
		TailLines:  &tailLines,
	})
	if err != nil {
		// The API server answers bad request for a container with no
		// previous instance, and for one that has not started yet.
		if apierrors.IsBadRequest(err) && ctx.Err() == nil {
			if t.Previous() {
				tl.end(ReasonNoPrevious)
			} else {
				tl.end(ReasonContainerWaiting)
			}
			return
		}
		tl.failed(ctx, "opening the log stream failed", err, ReasonPodNotFound)
		return
	}
	stop := context.AfterFunc(ctx, func() { _ = rc.Close() })
	defer func() {
		stop()
		_ = rc.Close()
	}()
	err = tl.copy(ctx, rc, p.opts.Now())
	if ctx.Err() != nil {
		return
	}
	switch {
	case !errors.Is(err, io.EOF):
		p.log.Warn("reading the log stream failed", "topic", t.String(), "error", err)
		tl.end(ReasonUnavailable)
	case t.Previous():
		tl.end(ReasonCompleted)
	default:
		tl.end(tl.whyEnded(ctx))
	}
}

// readerMay checks that the reading identity may read the Pod and its log.
func (tl *tail) readerMay(ctx context.Context) bool {
	p, t := tl.p, tl.topic
	podRead := tl.read
	podRead.Subresource = ""
	for _, req := range []authz.Attributes{tl.read, podRead} {
		if _, err := p.cfg.Authorizer.Check(ctx, p.cfg.Reader, req); err != nil {
			p.log.Warn("the reader may not read the pod's log", "topic", t.String(), "error", err)
			return false
		}
	}
	return true
}

// failed ends the topic after an upstream error: notFound when the Pod is
// gone, unavailable otherwise. Nothing is sent when ctx ended it.
func (tl *tail) failed(ctx context.Context, msg string, err error, notFound string) {
	if ctx.Err() != nil {
		return
	}
	if apierrors.IsNotFound(err) {
		tl.end(notFound)
		return
	}
	tl.p.log.Warn(msg, "topic", tl.topic.String(), "error", err)
	tl.end(ReasonUnavailable)
}

// whyEnded tells a stopped container from a stream the API server closed
// while the container still runs.
func (tl *tail) whyEnded(ctx context.Context) string {
	t := tl.topic
	pod, err := tl.p.cfg.Source.Pod(ctx, t.Namespace(), t.Name())
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ReasonPodNotFound
		}
		return ReasonContainerStopped
	}
	for _, list := range [][]corev1.ContainerStatus{pod.Status.ContainerStatuses, pod.Status.InitContainerStatuses, pod.Status.EphemeralContainerStatuses} {
		for i := range list {
			if list[i].Name == t.Container() && list[i].State.Running != nil {
				return ReasonUpstreamClosed
			}
		}
	}
	return ReasonContainerStopped
}

// copy reads lines from r and emits them within the topic's bounds until r
// ends. The initial tail is the lines stamped before start, at most
// TailLines of them, ending at the first line that is not: it is held until
// it ends, then its newest lines within MaxTailBytes are sent after a
// skipped marker counting the older ones. A followed tail also ends when no
// line arrives for MarkerDelay, so a quiet container's tail is not held
// back. The rest are live, bounded by the rate. The previous container's
// output is all tail.
//
// start is the portal's clock and the stamps are the node's, so a node
// clock ahead of the portal's counts tail lines as live (bounded by the
// rate, marked rate-limited when dropped), and one behind counts up to
// TailLines early live lines as tail (bounded by the tail cap, marked
// skipped). Either way every line is bounded and every dropped line is
// counted.
func (tl *tail) copy(ctx context.Context, r io.Reader, start time.Time) error {
	o := tl.p.opts
	lr := newLineReader(r, o.MaxLineBytes)
	lim := &limiter{lines: newBucket(o.LinesPerSecond, o.LineBurst, start), bytes: newBucket(o.BytesPerSecond, o.ByteBurst, start)}
	mk := &marks{tl: tl, delay: o.MarkerDelay, holding: true, maxTail: o.MaxTailBytes, follow: !tl.topic.Previous()}
	defer mk.stop()
	var tailLines int64
	inTail := true
	for ctx.Err() == nil {
		line, err := lr.next()
		if err != nil {
			mk.flush()
			return err
		}
		size := len(line.text)
		m := Message{Type: TypeLine, Text: line.text}
		if !line.time.IsZero() {
			m.Time = &line.time
		}
		if line.cut > 0 {
			m.Marker, m.Cut = MarkerTruncated, line.cut
		}
		inTail = tl.topic.Previous() || (inTail && tailLines < o.TailLines && !line.time.IsZero() && line.time.Before(start))
		if inTail {
			tailLines++
			if mk.hold(m, size) {
				if tailLines >= o.TailLines {
					mk.flush()
				}
				continue
			}
			// The tail was sent after a quiet spell: the rest is live.
			inTail = false
		}
		mk.release()
		if !lim.allow(o.Now(), size) {
			mk.count(&mk.dropped)
			continue
		}
		mk.flushThen(m)
	}
	return ctx.Err()
}

// marks holds one read's initial tail until it ends, and counts the lines
// the read skipped or dropped. It sends their markers before the next
// delivered line, or after the marker delay when no line follows, so a
// burst followed by silence is still marked (0030:D10:R3).
type marks struct {
	tl      *tail
	delay   time.Duration
	maxTail int
	follow  bool

	mu        sync.Mutex
	holding   bool // the initial tail has not ended
	held      []heldLine
	heldBytes int
	tailTimer *time.Timer
	skipped   int64
	dropped   int64
	timer     *time.Timer
	stopped   bool
}

// heldLine is one initial-tail line waiting for the tail to end.
type heldLine struct {
	m    Message
	size int
}

// hold adds m to the initial tail, skipping the oldest held lines past the
// tail cap. It returns false once the tail has ended. A followed tail ends
// MarkerDelay after its latest line.
func (mk *marks) hold(m Message, size int) bool {
	mk.mu.Lock()
	defer mk.mu.Unlock()
	if !mk.holding || mk.stopped {
		return false
	}
	mk.held = append(mk.held, heldLine{m: m, size: size})
	mk.heldBytes += size
	drop := 0
	for drop < len(mk.held) && mk.heldBytes > mk.maxTail {
		mk.heldBytes -= mk.held[drop].size
		drop++
	}
	if drop > 0 {
		mk.skipped += int64(drop)
		clear(mk.held[:drop])
		mk.held = append(mk.held[:0], mk.held[drop:]...)
	}
	if mk.follow {
		if mk.tailTimer == nil {
			mk.tailTimer = time.AfterFunc(mk.delay, mk.flush)
		} else {
			mk.tailTimer.Reset(mk.delay)
		}
	}
	return true
}

// count adds one to *n and arms the marker timer.
func (mk *marks) count(n *int64) {
	mk.mu.Lock()
	defer mk.mu.Unlock()
	*n++
	if mk.timer == nil && !mk.stopped {
		mk.timer = time.AfterFunc(mk.delay, mk.flush)
	}
}

// release ends the initial tail and sends it, if it has not ended yet.
func (mk *marks) release() {
	mk.mu.Lock()
	defer mk.mu.Unlock()
	mk.releaseLocked()
}

// flush ends the initial tail and sends the pending markers.
func (mk *marks) flush() {
	mk.mu.Lock()
	defer mk.mu.Unlock()
	mk.flushLocked()
}

// flushThen sends the pending markers, then m, with no marker between.
func (mk *marks) flushThen(m Message) {
	mk.mu.Lock()
	defer mk.mu.Unlock()
	mk.flushLocked()
	mk.tl.emit(m)
}

// releaseLocked sends the held tail: a skipped marker for the lines past
// the cap, then the lines kept, oldest first.
func (mk *marks) releaseLocked() {
	if !mk.holding {
		return
	}
	mk.holding = false
	if mk.tailTimer != nil {
		mk.tailTimer.Stop()
		mk.tailTimer = nil
	}
	held := mk.held
	mk.held, mk.heldBytes = nil, 0
	if mk.stopped {
		return
	}
	if mk.skipped > 0 {
		mk.tl.emit(Message{Type: TypeMarker, Marker: MarkerSkipped, Dropped: mk.skipped})
		mk.skipped = 0
	}
	for i := range held {
		mk.tl.emit(held[i].m)
	}
}

func (mk *marks) flushLocked() {
	mk.releaseLocked()
	if mk.timer != nil {
		mk.timer.Stop()
		mk.timer = nil
	}
	if mk.stopped {
		return
	}
	if mk.skipped > 0 {
		mk.tl.emit(Message{Type: TypeMarker, Marker: MarkerSkipped, Dropped: mk.skipped})
		mk.skipped = 0
	}
	if mk.dropped > 0 {
		mk.tl.emit(Message{Type: TypeMarker, Marker: MarkerRateLimited, Dropped: mk.dropped})
		mk.dropped = 0
	}
}

// stop disarms the timers once the read is over; its tail and markers were
// flushed, or the activation is closed and they are dropped.
func (mk *marks) stop() {
	mk.mu.Lock()
	defer mk.mu.Unlock()
	mk.stopped = true
	mk.releaseLocked()
	if mk.timer != nil {
		mk.timer.Stop()
		mk.timer = nil
	}
}

// hasContainer reports whether pod has a container, init container or
// ephemeral container named name.
func hasContainer(pod *corev1.Pod, name string) bool {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == name {
			return true
		}
	}
	for i := range pod.Spec.InitContainers {
		if pod.Spec.InitContainers[i].Name == name {
			return true
		}
	}
	for i := range pod.Spec.EphemeralContainers {
		if pod.Spec.EphemeralContainers[i].Name == name {
			return true
		}
	}
	return false
}
