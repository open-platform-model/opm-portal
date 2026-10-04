package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

// The tests below run log topics through the real broker and the real read
// model over the F1 capture, so reachability comes from the read model's
// runtime children and every line goes through the broker's send funnel.

// decisionTTL is short so a revocation is seen quickly.
const decisionTTL = 200 * time.Millisecond

type system struct {
	b      *stream.Broker
	p      *Producer
	src    *fakeSource
	access *switchRule
}

func newSystem(t *testing.T) *system {
	t.Helper()
	access := &switchRule{}
	az := newAuthorizer(t, access.rule(), decisionTTL, alice, bob, reader)
	m, err := readmodel.New(readmodel.Config{
		Dynamic:     readmodeltest.Dynamic(readmodeltest.LoadCapture(t, f1Dir)...),
		Discovery:   readmodeltest.Discovery(readmodeltest.Kinds...),
		Authorizer:  az,
		Reader:      reader,
		SyncTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Stop)
	src := newFakeSource(f1PodObject(t))
	p, err := New(Config{Source: src, Reach: m, Authorizer: az, Reader: reader})
	if err != nil {
		t.Fatal(err)
	}
	b := stream.New(stream.Mux{stream.KindLog: p}, az, stream.Options{HeartbeatInterval: 50 * time.Millisecond})
	p.SetPublisher(b)
	t.Cleanup(b.Close)
	return &system{b: b, p: p, src: src, access: access}
}

// sseRecorder collects the events a stream writes.
type sseRecorder struct {
	mu  sync.Mutex
	buf bytes.Buffer
	hdr http.Header
}

func (r *sseRecorder) Header() http.Header {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hdr == nil {
		r.hdr = http.Header{}
	}
	return r.hdr
}

func (r *sseRecorder) WriteHeader(int) {}

func (r *sseRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.Write(p)
}

func (r *sseRecorder) Flush() {}

type event struct {
	name string
	data struct {
		Topic string          `json:"topic"`
		Code  string          `json:"code"`
		Item  json.RawMessage `json:"item"`
		Items json.RawMessage `json:"items"`
	}
}

func (r *sseRecorder) events(t testing.TB) []event {
	t.Helper()
	r.mu.Lock()
	body := r.buf.String()
	r.mu.Unlock()
	var out []event
	for block := range strings.SplitSeq(body, "\n\n") {
		var e event
		for line := range strings.SplitSeq(block, "\n") {
			field, value, _ := strings.Cut(line, ": ")
			switch field {
			case "event":
				e.name = value
			case "data":
				if err := json.Unmarshal([]byte(value), &e.data); err != nil {
					t.Fatalf("event data %q: %v", value, err)
				}
			}
		}
		if e.name != "" && e.name != stream.EventHeartbeat {
			out = append(out, e)
		}
	}
	return out
}

// texts returns the text of every log line delivered on the live topic.
func (r *sseRecorder) texts(t testing.TB) []string {
	t.Helper()
	var out []string
	for _, e := range r.events(t) {
		if e.name == stream.EventLog && e.data.Topic == liveTopic {
			if m := decode(t, e.data.Item); m.Type == TypeLine {
				out = append(out, m.Text)
			}
		}
	}
	return out
}

// closed returns the closing code of topic, or "".
func (r *sseRecorder) closed(t testing.TB, topic string) string {
	t.Helper()
	for _, e := range r.events(t) {
		if e.name == stream.EventClosed && e.data.Topic == topic {
			return e.data.Code
		}
	}
	return ""
}

type open struct {
	*stream.Stream
	rec    *sseRecorder
	cancel context.CancelFunc
	done   chan struct{}
}

func (s *system) open(t *testing.T, who string, topics ...string) *open {
	t.Helper()
	ids := map[string]stream.Session{
		"alice": {Key: "session-alice", Identity: alice},
		"bob":   {Key: "session-bob", Identity: bob},
	}
	ts := make([]stream.Topic, 0, len(topics))
	for _, topic := range topics {
		ts = append(ts, mustTopic(t, topic))
	}
	st, err := s.b.Open(t.Context(), ids[who], ts, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	o := &open{Stream: st, rec: &sseRecorder{}, cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(o.done)
		_ = st.Serve(ctx, o.rec)
	}()
	t.Cleanup(func() {
		cancel()
		<-o.done
	})
	return o
}

func TestALogTopicIsRefusedWithoutPodLogAccess(t *testing.T) {
	s := newSystem(t)
	s.access.set(func(user, _, sub string) bool { return user == "alice" && sub == "log" })
	o := s.open(t, "alice", liveTopic)
	eventually(t, "the closing", func() bool { return o.rec.closed(t, liveTopic) != "" })
	if code := o.rec.closed(t, liveTopic); code != stream.CodeForbidden {
		t.Errorf("closing code = %q, want forbidden", code)
	}
	if podCalls, logs := s.src.calls(); podCalls != 0 || logs != 0 {
		t.Errorf("upstream read %d Pods and %d logs for a denied topic", podCalls, logs)
	}
}

func TestAPodNoInventoryReachesIsRefusedLikeAMissingPermission(t *testing.T) {
	s := newSystem(t)
	// alice may read every log, bob none.
	s.access.set(func(user, _, sub string) bool { return user == "bob" && sub == "log" })
	strayTopic := "log:default/stray-0/" + f1Container
	o := s.open(t, "alice", strayTopic)
	o2 := s.open(t, "bob", liveTopic)
	eventually(t, "both closings", func() bool {
		return o.rec.closed(t, strayTopic) != "" && o2.rec.closed(t, liveTopic) != ""
	})
	if code := o.rec.closed(t, strayTopic); code != stream.CodeForbidden {
		t.Errorf("closing code = %q, want forbidden", code)
	}
	stray, denied := closedData(t, o.rec, strayTopic), closedData(t, o2.rec, liveTopic)
	if strings.Replace(stray, strayTopic, "X", 1) != strings.Replace(denied, liveTopic, "X", 1) {
		t.Errorf("an unreachable Pod closes unlike a denied one: %s vs %s", stray, denied)
	}
	if podCalls, logs := s.src.calls(); podCalls != 0 || logs != 0 {
		t.Errorf("upstream read %d Pods and %d logs for refused topics", podCalls, logs)
	}
}

func closedData(t testing.TB, r *sseRecorder, topic string) string {
	t.Helper()
	r.mu.Lock()
	body := r.buf.String()
	r.mu.Unlock()
	for block := range strings.SplitSeq(body, "\n\n") {
		if strings.Contains(block, "event: closed") && strings.Contains(block, topic) {
			_, data, _ := strings.Cut(block, "data: ")
			return data
		}
	}
	t.Fatalf("no closing for %s", topic)
	return ""
}

func TestLinesFlowThroughTheBrokerUntilRevoked(t *testing.T) {
	s := newSystem(t)
	o := s.open(t, "alice", liveTopic)
	fs := s.src.open(t)
	now := time.Now()
	fs.write(t, liveTS(now.Add(time.Second), "hello"))
	eventually(t, "the line", func() bool { return len(o.rec.texts(t)) == 1 })
	if evs := o.rec.events(t); evs[0].name != "open" || evs[1].name != stream.EventSnapshot {
		t.Errorf("events start %s, %s; want open, snapshot", evs[0].name, evs[1].name)
	}

	s.access.set(func(user, _, sub string) bool { return user == "alice" && sub == "log" })
	eventually(t, "the revocation", func() bool { return o.rec.closed(t, liveTopic) != "" })
	if code := o.rec.closed(t, liveTopic); code != stream.CodeForbidden {
		t.Errorf("closing code = %q, want forbidden", code)
	}
	// The topic's last subscriber is gone, so its upstream stream closes.
	eventually(t, "the upstream close", fs.isClosed)
	if got := o.rec.texts(t); len(got) != 1 {
		t.Errorf("lines after the revocation: %v", got)
	}
}

func TestSubscribersShareOneUpstreamUntilTheLastLeaves(t *testing.T) {
	s := newSystem(t)
	a := s.open(t, "alice", liveTopic)
	fs := s.src.open(t)
	b := s.open(t, "bob", liveTopic)
	fs.write(t, liveTS(time.Now().Add(time.Second), "shared"))
	eventually(t, "both deliveries", func() bool {
		return len(a.rec.texts(t)) == 1 && len(b.rec.texts(t)) >= 1
	})
	if _, logs := s.src.calls(); logs != 1 {
		t.Errorf("%d upstream streams for one topic, want 1", logs)
	}
	topic := mustTopic(t, liveTopic)
	if err := s.b.Unsubscribe(stream.Session{Key: "session-bob", Identity: bob}, b.ID(), topic); err != nil {
		t.Fatal(err)
	}
	if fs.isClosed() {
		t.Fatal("the upstream closed while alice still follows the topic")
	}
	if err := s.b.Unsubscribe(stream.Session{Key: "session-alice", Identity: alice}, a.ID(), topic); err != nil {
		t.Fatal(err)
	}
	if !fs.isClosed() {
		t.Error("the upstream is open after the last subscriber left")
	}
}

func TestFollowingAgainAfterTheEndRestartsASharedTopic(t *testing.T) {
	s := newSystem(t)
	a := s.open(t, "alice", liveTopic)
	first := s.src.open(t)
	b := s.open(t, "bob", liveTopic)
	first.write(t, liveTS(time.Now().Add(time.Second), "before"))
	_ = first.w.Close()
	ended := func(r *sseRecorder) bool {
		for _, e := range r.events(t) {
			if e.name == stream.EventLogEnd {
				return true
			}
		}
		return false
	}
	eventually(t, "the end on both streams", func() bool { return ended(a.rec) && ended(b.rec) })

	// Bob follows again while alice still holds the topic.
	topic := mustTopic(t, liveTopic)
	bobSession := stream.Session{Key: "session-bob", Identity: bob}
	if err := s.b.Unsubscribe(bobSession, b.ID(), topic); err != nil {
		t.Fatal(err)
	}
	if err := s.b.Subscribe(t.Context(), bobSession, b.ID(), topic); err != nil {
		t.Fatal(err)
	}
	second := s.src.open(t)
	second.write(t, liveTS(time.Now().Add(time.Second), "after"))
	eventually(t, "the new tail on both streams", func() bool {
		return slices.Contains(a.rec.texts(t), "after") && slices.Contains(b.rec.texts(t), "after")
	})
	if _, logs := s.src.calls(); logs != 2 {
		t.Errorf("%d upstream streams, want 2: one per read", logs)
	}
}
