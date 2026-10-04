package stream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

var (
	platformsGVR = schema.GroupVersionResource{Group: "opmodel.dev", Version: "v1alpha1", Resource: "platforms"}
	packagesGVR  = schema.GroupVersionResource{Group: "opmodel.dev", Version: "v1alpha1", Resource: "modulepackages"}
	regsGVR      = schema.GroupVersionResource{Group: "opmodel.dev", Version: "v1alpha1", Resource: "transformerregistrations"}
	eventsGVR    = schema.GroupVersionResource{Group: "events.k8s.io", Version: "v1", Resource: "events"}
	podsGVR      = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
)

func user(name string) authz.Identity {
	return authz.Identity{Username: name, Groups: []string{"system:authenticated"}}
}

func session(name string) Session { return Session{Key: "session-" + name, Identity: user(name)} }

// rule decides one access review: allowed, or an error to fail it with.
type rule func(a authorizationv1.ResourceAttributes) (bool, error)

// allowNamespaces allows every read inside the given namespaces ("" is
// cluster-wide and cluster-scoped reads).
func allowNamespaces(ns ...string) rule {
	return func(a authorizationv1.ResourceAttributes) (bool, error) {
		for _, n := range ns {
			if a.Namespace == n {
				return true, nil
			}
		}
		return false, nil
	}
}

// policy is a fake cluster's RBAC: one rule per user, changeable at any
// time. It counts the reviews it answers.
type policy struct {
	mu      sync.Mutex
	rules   map[string]rule
	reviews int
}

func (p *policy) set(username string, r rule) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rules[username] = r
}

func (p *policy) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.reviews
}

func (p *policy) decide(username string, a authorizationv1.ResourceAttributes) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reviews++
	r := p.rules[username]
	if r == nil {
		return false, nil
	}
	return r(a)
}

// multiAuthz serves several identities, each through its own local Checker
// on a fake cluster, the way an in-cluster authorizer serves many users.
// Every grant comes from a real authz.Checker.
type multiAuthz struct {
	checkers map[string]*authz.Checker
}

func (m *multiAuthz) Check(ctx context.Context, who authz.Identity, req authz.Attributes) (authz.Grant, error) {
	c := m.checkers[who.Username]
	if c == nil {
		var none authz.Grant
		return none, &authz.DenialError{Code: authz.CodeUnauthenticated, Attributes: req}
	}
	return c.Check(ctx, who, req)
}

func newAuthz(t *testing.T, p *policy, usernames ...string) *multiAuthz {
	t.Helper()
	m := &multiAuthz{checkers: map[string]*authz.Checker{}}
	for _, name := range usernames {
		cs := fake.NewClientset()
		cs.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
			review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
			allowed, err := p.decide(name, *review.Spec.ResourceAttributes)
			if err != nil {
				return true, nil, err
			}
			out := review.DeepCopy()
			out.Status.Allowed = allowed
			return true, out, nil
		})
		c, err := authz.NewLocal(cs.AuthorizationV1().SelfSubjectAccessReviews(), user(name), authz.Options{})
		if err != nil {
			t.Fatal(err)
		}
		m.checkers[name] = c
	}
	return m
}

// fakeProducer serves topics from in-memory state. It updates its state
// before publishing, as the producer contract requires.
type fakeProducer struct {
	mu          sync.Mutex
	b           *Broker
	state       map[Topic]map[string]Item
	activations map[Topic]int
	releases    map[Topic]int
	snapshotErr error
	// duringSnapshot runs inside Snapshot after the state was read.
	duringSnapshot func()
	// snapshotWait runs inside Snapshot with its context, before the state
	// is read.
	snapshotWait func(ctx context.Context)
	// attributeCalls counts Attributes calls.
	attributeCalls int
}

func newFakeProducer() *fakeProducer {
	return &fakeProducer{
		state:       map[Topic]map[string]Item{},
		activations: map[Topic]int{},
		releases:    map[Topic]int{},
	}
}

func (p *fakeProducer) Attributes(t Topic) ([]authz.Attributes, bool) {
	p.mu.Lock()
	p.attributeCalls++
	p.mu.Unlock()
	return p.reads(t)
}

func (p *fakeProducer) reads(t Topic) ([]authz.Attributes, bool) {
	switch t.Kind() {
	case KindPlatform:
		return []authz.Attributes{{Verb: "get", Resource: platformsGVR, Name: "cluster"}}, true
	case KindInstances:
		// A list topic needs the list grant a GET list needs: cluster-wide
		// for "instances", on the namespace for "instances:<ns>".
		return []authz.Attributes{{Verb: "list", Resource: instancesGVR, Namespace: t.Namespace()}}, true
	case KindInstance:
		return []authz.Attributes{{Verb: "get", Resource: instancesGVR, Namespace: t.Namespace(), Name: t.Name()}}, true
	case KindPackage:
		return []authz.Attributes{{Verb: "get", Resource: packagesGVR, Namespace: t.Namespace(), Name: t.Name()}}, true
	case KindRegistration:
		return []authz.Attributes{{Verb: "get", Resource: regsGVR, Name: t.Name()}}, true
	case KindEvents:
		ref, _ := t.Ref()
		attrs, _ := p.reads(ref)
		return append(attrs, authz.Attributes{Verb: "list", Resource: eventsGVR, Namespace: ref.Namespace()}), true
	case KindLog:
		// Logs are not served.
	}
	return nil, false
}

func (p *fakeProducer) Snapshot(ctx context.Context, t Topic) ([]Item, error) {
	p.mu.Lock()
	wait := p.snapshotWait
	p.mu.Unlock()
	if wait != nil {
		wait(ctx)
	}
	p.mu.Lock()
	if p.snapshotErr != nil {
		p.mu.Unlock()
		return nil, p.snapshotErr
	}
	var items []Item
	for _, k := range sortedKeys(p.state[t]) {
		items = append(items, p.state[t][k])
	}
	hook := p.duringSnapshot
	p.mu.Unlock()
	if hook != nil {
		hook()
	}
	return items, nil
}

func (p *fakeProducer) Activate(t Topic) func() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activations[t]++
	var once sync.Once
	return func() {
		once.Do(func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			p.releases[t]++
		})
	}
}

func (p *fakeProducer) counts(t Topic) (activations, releases int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.activations[t], p.releases[t]
}

// upsert sets an item's state and publishes it.
func (p *fakeProducer) upsert(t *testing.T, topic Topic, it Item) {
	t.Helper()
	p.mu.Lock()
	if p.state[topic] == nil {
		p.state[topic] = map[string]Item{}
	}
	p.state[topic][itemKey(it)] = it
	b := p.b
	p.mu.Unlock()
	if b != nil {
		if err := b.Publish(topic, it); err != nil {
			t.Fatal(err)
		}
	}
}

// itemKey names the object an item describes: from its payload when it
// carries one (a list item's read names no object), else from its read.
func itemKey(it Item) string {
	var obj struct{ Name, Namespace string }
	if it.Data != nil && json.Unmarshal(it.Data, &obj) == nil && obj.Name != "" {
		return obj.Namespace + "/" + obj.Name
	}
	return it.Attrs.Namespace + "/" + it.Attrs.Name
}

func sortedKeys(m map[string]Item) []string { return slices.Sorted(maps.Keys(m)) }

// instItem is an upsert of instance ns/name at version v.
func instItem(ns, name string, v int) Item {
	return Item{
		Event: EventUpsert,
		Attrs: authz.Attributes{Verb: "get", Resource: instancesGVR, Namespace: ns, Name: name},
		Data:  json.RawMessage(fmt.Sprintf(`{"name": %q, "namespace": %q, "v": %d}`, name, ns, v)),
	}
}

// listItem is an upsert of instance ns/name at version v on a list topic:
// the read it reveals is the list of its namespace, which the topic's list
// grant covers.
func listItem(ns, name string, v int) Item {
	it := instItem(ns, name, v)
	it.Attrs = authz.Attributes{Verb: "list", Resource: instancesGVR, Namespace: ns}
	return it
}

func mustTopic(t *testing.T, s string) Topic {
	t.Helper()
	tp, err := ParseTopic(s)
	if err != nil {
		t.Fatal(err)
	}
	return tp
}

// sse is one parsed server-sent event.
type sse struct {
	ID, Event, Data, Retry string
}

func (e sse) topic() string {
	var d struct {
		Topic string `json:"topic"`
	}
	_ = json.Unmarshal([]byte(e.Data), &d)
	return d.Topic
}

func (e sse) code() string {
	var d struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal([]byte(e.Data), &d)
	return d.Code
}

// versions returns the "v" of a snapshot's items or of an upsert's item.
func (e sse) versions(t *testing.T) []int {
	t.Helper()
	var d struct {
		Items []struct {
			V int `json:"v"`
		} `json:"items"`
		Item *struct {
			V int `json:"v"`
		} `json:"item"`
	}
	if err := json.Unmarshal([]byte(e.Data), &d); err != nil {
		t.Fatalf("event data %q: %v", e.Data, err)
	}
	if d.Item != nil {
		return []int{d.Item.V}
	}
	out := []int{}
	for _, it := range d.Items {
		out = append(out, it.V)
	}
	return out
}

func (e sse) names(t *testing.T) []string {
	t.Helper()
	var d struct {
		Items []struct {
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
		} `json:"items"`
		Item *struct {
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
		} `json:"item"`
	}
	if err := json.Unmarshal([]byte(e.Data), &d); err != nil {
		t.Fatalf("event data %q: %v", e.Data, err)
	}
	if d.Item != nil {
		return []string{d.Item.Namespace + "/" + d.Item.Name}
	}
	out := []string{}
	for _, it := range d.Items {
		out = append(out, it.Namespace+"/"+it.Name)
	}
	return out
}

// parseSSE splits a complete server-sent-events body into events.
func parseSSE(body string) []sse {
	var out []sse
	for block := range strings.SplitSeq(body, "\n\n") {
		if block == "" {
			continue
		}
		var e sse
		for line := range strings.SplitSeq(block, "\n") {
			field, value, _ := strings.Cut(line, ": ")
			switch field {
			case "id":
				e.ID = value
			case "event":
				e.Event = value
			case "data":
				e.Data = value
			case "retry":
				e.Retry = value
			}
		}
		out = append(out, e)
	}
	return out
}

// recorder is an http.ResponseWriter that parses what is flushed. While
// gate is set, Write blocks until it is closed, like a client that stopped
// reading.
type recorder struct {
	mu      sync.Mutex
	header  http.Header
	pending bytes.Buffer
	events  []sse
	taken   int
	gate    chan struct{}
}

func newRecorder() *recorder { return &recorder{header: http.Header{}} }

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(int) {}

func (r *recorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	gate := r.gate
	r.mu.Unlock()
	if gate != nil {
		<-gate
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pending.Write(p)
}

func (r *recorder) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	body := r.pending.String()
	cut := strings.LastIndex(body, "\n\n")
	if cut < 0 {
		return
	}
	r.events = append(r.events, parseSSE(body[:cut+2])...)
	r.pending.Reset()
	r.pending.WriteString(body[cut+2:])
}

// take returns the events flushed since the last take.
func (r *recorder) take() []sse {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.events[r.taken:]
	r.taken = len(r.events)
	return out
}

func (r *recorder) block() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gate = make(chan struct{})
}

func (r *recorder) unblock() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.gate != nil {
		close(r.gate)
		r.gate = nil
	}
}

// lastID returns the id of the last flushed event that carried one.
func (r *recorder) lastID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.events) - 1; i >= 0; i-- {
		if r.events[i].ID != "" {
			return r.events[i].ID
		}
	}
	return ""
}

// env is a broker over a fake producer and fake RBAC, inside a synctest
// bubble.
type env struct {
	t      *testing.T
	policy *policy
	prod   *fakeProducer
	b      *Broker
}

func newEnv(t *testing.T, opts Options, usernames ...string) *env {
	t.Helper()
	p := &policy{rules: map[string]rule{}}
	prod := newFakeProducer()
	b := New(prod, newAuthz(t, p, usernames...), opts)
	prod.b = b
	t.Cleanup(func() {
		b.Close()
		synctest.Wait()
	})
	return &env{t: t, policy: p, prod: prod, b: b}
}

// served is a stream being served into a recorder.
type served struct {
	*Stream
	rec    *recorder
	done   chan error
	cancel context.CancelFunc
}

// serve starts serving s and waits until the bubble is quiet.
func (e *env) serve(s *Stream) *served {
	rec := newRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	sv := &served{Stream: s, rec: rec, done: make(chan error, 1), cancel: cancel}
	go func() { sv.done <- s.Serve(ctx, rec) }()
	synctest.Wait()
	return sv
}

// disconnect drops the client's connection and waits for Serve to return.
func (sv *served) disconnect() error {
	sv.cancel()
	return <-sv.done
}

// ended reports how the stream's Serve returned, or nil while it runs.
func (sv *served) ended() error {
	select {
	case err := <-sv.done:
		return err
	default:
		return nil
	}
}

// open opens and serves a stream for sess on topics.
func (e *env) open(sess Session, topics ...string) *served {
	e.t.Helper()
	return e.resume(sess, "", topics...)
}

// resume opens and serves a stream for sess with a Last-Event-ID.
func (e *env) resume(sess Session, lastEventID string, topics ...string) *served {
	e.t.Helper()
	ts := make([]Topic, 0, len(topics))
	for _, s := range topics {
		ts = append(ts, mustTopic(e.t, s))
	}
	s, err := e.b.Open(context.Background(), sess, ts, lastEventID)
	if err != nil {
		e.t.Fatalf("Open(%v): %v", topics, err)
	}
	return e.serve(s)
}

// eventNames lists the event names of evs, for failure messages and
// comparisons.
func eventNames(evs []sse) []string {
	out := make([]string, 0, len(evs))
	for _, e := range evs {
		out = append(out, e.Event+"("+e.topic()+")")
	}
	return out
}
