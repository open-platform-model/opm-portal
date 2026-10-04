package logs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"sync"
	"testing"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

// f1Dir is the committed capture of the e2e fixture cluster.
const f1Dir = "../../testdata/clusters/f1"

// f1Pod and f1Container are one of the podinfo Deployment's Pods in F1.
const (
	f1Pod       = "podinfo-podinfo-d9585d794-4lg6h"
	f1Container = "podinfo"
)

var (
	alice  = authz.Identity{Username: "alice", Groups: []string{"system:authenticated"}}
	bob    = authz.Identity{Username: "bob", Groups: []string{"system:authenticated"}}
	reader = authz.Identity{Username: "portal-reader", Groups: []string{"system:authenticated"}}
)

// f1PodObject returns a copy of the F1 Pod as a typed Pod.
func f1PodObject(t testing.TB) *corev1.Pod {
	t.Helper()
	f1PodOnce.Do(func() {
		u := readmodeltest.Find(t, readmodeltest.LoadCapture(t, f1Dir), "Pod", f1Pod)
		var pod corev1.Pod
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &pod); err != nil {
			t.Fatal(err)
		}
		f1PodCached = &pod
	})
	if f1PodCached == nil {
		t.Fatal("the F1 Pod did not load")
	}
	return f1PodCached.DeepCopy()
}

var (
	f1PodOnce   sync.Once
	f1PodCached *corev1.Pod
)

// fakeSource serves one Pod and hands each log stream to the test through
// a pipe.
type fakeSource struct {
	mu       sync.Mutex
	pod      *corev1.Pod // nil: not found
	logsErr  error
	podCalls int
	opts     []corev1.PodLogOptions
	streams  []*fakeStream
	opened   chan *fakeStream
}

func newFakeSource(pod *corev1.Pod) *fakeSource {
	return &fakeSource{pod: pod, opened: make(chan *fakeStream, 16)}
}

func (s *fakeSource) Pod(_ context.Context, namespace, name string) (*corev1.Pod, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.podCalls++
	if s.pod == nil || s.pod.Namespace != namespace || s.pod.Name != name {
		return nil, apierrors.NewNotFound(corev1.Resource("pods"), name)
	}
	return s.pod.DeepCopy(), nil
}

func (s *fakeSource) Logs(_ context.Context, _, _ string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opts = append(s.opts, *opts)
	if s.logsErr != nil {
		return nil, s.logsErr
	}
	r, w := io.Pipe()
	fs := &fakeStream{r: r, w: w}
	s.streams = append(s.streams, fs)
	s.opened <- fs
	return fs, nil
}

func (s *fakeSource) setPod(pod *corev1.Pod) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pod = pod
}

func (s *fakeSource) calls() (pods, logs int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.podCalls, len(s.opts)
}

func (s *fakeSource) open(t testing.TB) *fakeStream {
	t.Helper()
	select {
	case fs := <-s.opened:
		return fs
	case <-time.After(5 * time.Second):
		t.Fatal("no log stream was opened")
		return nil
	}
}

// fakeStream is one upstream log stream.
type fakeStream struct {
	r      *io.PipeReader
	w      *io.PipeWriter
	mu     sync.Mutex
	closed bool
}

func (f *fakeStream) Read(p []byte) (int, error) { return f.r.Read(p) }

func (f *fakeStream) Close() error {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
	return f.r.Close()
}

func (f *fakeStream) isClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

// write writes raw lines; it fails the test when the reader is gone.
func (f *fakeStream) write(t testing.TB, lines ...string) {
	t.Helper()
	for _, l := range lines {
		if _, err := io.WriteString(f.w, l+"\n"); err != nil {
			t.Fatalf("writing a log line: %v", err)
		}
	}
}

// fakeReach answers ReachPod from a fixed decision.
type fakeReach struct {
	mu    sync.Mutex
	err   error
	asked int
}

func (r *fakeReach) ReachPod(_ context.Context, who authz.Identity, g authz.Grant, namespace, pod string) (readmodel.PodReach, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.asked++
	read := authz.Attributes{Verb: "get", Resource: pods, Subresource: "log", Namespace: namespace, Name: pod}
	if err := g.Covers(who, read); err != nil {
		return readmodel.PodReach{}, readmodel.ErrNotCovered
	}
	return readmodel.PodReach{}, r.err
}

// capture is a Publisher that records every item.
type capture struct {
	mu    sync.Mutex
	items []stream.Item
	ch    chan stream.Item
}

func newCapture() *capture { return &capture{ch: make(chan stream.Item, 4096)} }

func (c *capture) Publish(_ stream.Topic, items ...stream.Item) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = append(c.items, items...)
	for i := range items {
		c.ch <- items[i]
	}
	return nil
}

// next waits for the next published message.
func (c *capture) next(t testing.TB) Message {
	t.Helper()
	select {
	case it := <-c.ch:
		return decode(t, it.Data)
	case <-time.After(5 * time.Second):
		t.Fatal("no message was published")
		return Message{}
	}
}

// none checks that nothing more is published for a moment.
func (c *capture) none(t testing.TB) {
	t.Helper()
	select {
	case it := <-c.ch:
		t.Fatalf("unexpected message %s", it.Data)
	case <-time.After(50 * time.Millisecond):
	}
}

func decode(t testing.TB, data []byte) Message {
	t.Helper()
	var m Message
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("message %s: %v", data, err)
	}
	return m
}

// clock is a settable clock.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// switchRule is an access rule a test can change while it runs.
type switchRule struct {
	mu   sync.Mutex
	deny func(username string, resource, subresource string) bool
}

func (s *switchRule) set(deny func(username, resource, subresource string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deny = deny
}

func (s *switchRule) rule() readmodeltest.Rule {
	return func(who string, ra authorizationv1.ResourceAttributes) bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.deny == nil || !s.deny(who, ra.Resource, ra.Subresource)
	}
}

// newAuthorizer returns an authorizer serving each identity through a real
// local Checker whose reviews follow rule.
func newAuthorizer(t testing.TB, rule readmodeltest.Rule, ttl time.Duration, ids ...authz.Identity) readmodeltest.ByIdentity {
	t.Helper()
	by := readmodeltest.ByIdentity{}
	for _, id := range ids {
		c, _ := readmodeltest.NewChecker(t, id, rule, authz.Options{TTL: ttl})
		by[id.Username] = c
	}
	return by
}

func mustTopic(t testing.TB, s string) stream.Topic {
	t.Helper()
	tp, err := stream.ParseTopic(s)
	if err != nil {
		t.Fatal(err)
	}
	return tp
}

// liveTS stamps a line at the clock's time plus d, as the API server does
// with timestamps on.
func liveTS(at time.Time, text string) string {
	return at.UTC().Format(time.RFC3339Nano) + " " + text
}

func eventually(t testing.TB, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

var errBoom = errors.New("boom")

func types(ms []Message) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		s := m.Type
		if m.Marker != "" {
			s += ":" + m.Marker
		}
		if m.Reason != "" {
			s += ":" + m.Reason
		}
		out = append(out, s)
	}
	return slices.Clip(out)
}

// lockedWriter serializes writes to w, so a test can read what was logged.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
