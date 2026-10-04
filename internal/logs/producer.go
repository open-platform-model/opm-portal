package logs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
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
	// MaxTailBytes caps the initial tail; tail lines past it are skipped
	// and counted. Default 1 MiB.
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
// stream.Producer and stream.Admitter.
type Producer struct {
	cfg  Config
	opts Options
	log  *slog.Logger

	mu    sync.Mutex
	pub   Publisher
	tails map[stream.Topic]*tail
}

var (
	_ stream.Producer = (*Producer)(nil)
	_ stream.Admitter = (*Producer)(nil)
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
// without it, and every refusal is the same one.
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

// Snapshot implements stream.Producer: the topic's recent messages. A
// message in the snapshot may arrive again as a later item, with the same
// seq.
func (p *Producer) Snapshot(_ context.Context, t stream.Topic) ([]stream.Item, error) {
	p.mu.Lock()
	tl := p.tails[t]
	p.mu.Unlock()
	if tl == nil {
		return []stream.Item{}, nil
	}
	return tl.recent(), nil
}

// Activate implements stream.Producer: it opens the topic's one upstream
// stream. The release closes it and returns once its reader has stopped.
func (p *Producer) Activate(t stream.Topic) func() {
	ctx, cancel := context.WithCancel(context.Background())
	tl := &tail{p: p, topic: t, read: podLogRead(t), done: make(chan struct{})}
	p.mu.Lock()
	p.tails[t] = tl
	p.mu.Unlock()
	go func() {
		defer close(tl.done)
		tl.run(ctx)
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			<-tl.done
			p.mu.Lock()
			if p.tails[t] == tl {
				delete(p.tails, t)
			}
			p.mu.Unlock()
		})
	}
}

// tail is one activation of a log topic: one upstream stream.
type tail struct {
	p     *Producer
	topic stream.Topic
	read  authz.Attributes
	done  chan struct{}

	mu  sync.Mutex
	seq uint64
	buf []stream.Item
}

func (tl *tail) recent() []stream.Item {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	out := make([]stream.Item, len(tl.buf))
	copy(out, tl.buf)
	return out
}

// emit records m in the buffer, then publishes it, as the producer
// contract requires.
func (tl *tail) emit(m Message) {
	event := stream.EventLog
	if m.Type == TypeEnd {
		event = stream.EventLogEnd
	}
	m.Container = tl.topic.Container()
	tl.mu.Lock()
	tl.seq++
	m.Seq = tl.seq
	data, err := json.Marshal(m)
	if err != nil {
		tl.mu.Unlock()
		tl.p.log.Warn("encoding a log message failed", "topic", tl.topic.String(), "error", err)
		return
	}
	it := stream.Item{Event: event, Attrs: tl.read, Data: data}
	if keep := tl.p.opts.Buffer; len(tl.buf) >= keep {
		tl.buf = append(tl.buf[:0], tl.buf[len(tl.buf)-keep+1:]...)
	}
	tl.buf = append(tl.buf, it)
	tl.mu.Unlock()

	tl.p.mu.Lock()
	pub := tl.p.pub
	tl.p.mu.Unlock()
	if pub == nil {
		return
	}
	if err := pub.Publish(tl.topic, it); err != nil {
		tl.p.log.Warn("publishing a log message failed", "topic", tl.topic.String(), "error", err)
	}
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
	start := p.opts.Now()
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
		if t.Previous() && apierrors.IsBadRequest(err) {
			tl.end(ReasonNoPrevious)
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
	err = tl.copy(ctx, rc, start)
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
// ends. Lines stamped before start are the initial tail, bounded by
// MaxTailBytes; the rest are live, bounded by the rate. The previous
// container's output is all tail.
func (tl *tail) copy(ctx context.Context, r io.Reader, start time.Time) error {
	o := tl.p.opts
	lr := newLineReader(r, o.MaxLineBytes)
	lim := &limiter{lines: newBucket(o.LinesPerSecond, o.LineBurst, start), bytes: newBucket(o.BytesPerSecond, o.ByteBurst, start)}
	var tailBytes int
	var tailFull bool
	var skipped, dropped int64
	flush := func() {
		if skipped > 0 {
			tl.emit(Message{Type: TypeMarker, Marker: MarkerSkipped, Dropped: skipped})
			skipped = 0
		}
		if dropped > 0 {
			tl.emit(Message{Type: TypeMarker, Marker: MarkerRateLimited, Dropped: dropped})
			dropped = 0
		}
	}
	for ctx.Err() == nil {
		line, err := lr.next()
		if err != nil {
			flush()
			return err
		}
		size := len(line.text)
		if tl.topic.Previous() || (!line.time.IsZero() && line.time.Before(start)) {
			// Once the tail cap is reached the rest of the tail is skipped,
			// so the stream skips ahead to live output.
			if tailFull || tailBytes+size > o.MaxTailBytes {
				tailFull = true
				skipped++
				continue
			}
			tailBytes += size
		} else if !lim.allow(o.Now(), size) {
			dropped++
			continue
		}
		flush()
		m := Message{Type: TypeLine, Text: line.text}
		if !line.time.IsZero() {
			m.Time = &line.time
		}
		if line.cut > 0 {
			m.Marker, m.Cut = MarkerTruncated, line.cut
		}
		tl.emit(m)
	}
	return ctx.Err()
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
