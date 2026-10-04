package stream

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// admitting serves log topics on top of a fake producer and admits them
// through a changeable decision, counting the calls.
type admitting struct {
	*fakeProducer
	mu       sync.Mutex
	decision func(t Topic) error
	asked    []string
	followed []string
}

func (a *admitting) Follow(t Topic) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.followed = append(a.followed, t.String())
}

func (a *admitting) follows() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.followed)
}

func (a *admitting) Attributes(t Topic) ([]authz.Attributes, bool) {
	if t.Kind() == KindLog {
		return []authz.Attributes{{Verb: "get", Resource: podsGVR, Subresource: "log", Namespace: t.Namespace(), Name: t.Name()}}, true
	}
	return a.fakeProducer.Attributes(t)
}

func (a *admitting) Admit(_ context.Context, who authz.Identity, t Topic, grants []authz.Grant) error {
	a.mu.Lock()
	a.asked = append(a.asked, who.Username+" "+t.String())
	decide := a.decision
	a.mu.Unlock()
	attrs, _ := a.Attributes(t)
	if len(grants) != len(attrs) {
		return errors.New("grants do not match the topic's reads")
	}
	for i := range attrs {
		if err := grants[i].Covers(who, attrs[i]); err != nil {
			return err
		}
	}
	if decide == nil {
		return nil
	}
	return decide(t)
}

func (a *admitting) set(decide func(Topic) error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.decision = decide
}

func (a *admitting) calls() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.asked)
}

// newAdmitEnv is newEnv with a producer that serves and admits log topics.
func newAdmitEnv(t *testing.T, opts Options, usernames ...string) (*env, *admitting) {
	t.Helper()
	p := &policy{rules: map[string]rule{}}
	prod := newFakeProducer()
	ad := &admitting{fakeProducer: prod}
	b := New(ad, newAuthz(t, p, usernames...), opts)
	prod.b = b
	t.Cleanup(func() {
		b.Close()
		synctest.Wait()
	})
	return &env{t: t, policy: p, prod: prod, b: b}, ad
}

func refuseAll(Topic) error { return ErrNotAdmitted }

func TestAnUnadmittedTopicIsClosedLikeADenial(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, ad := newAdmitEnv(t, Options{}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		ad.set(func(t Topic) error {
			if t.Name() == "stray" {
				return ErrNotAdmitted
			}
			return nil
		})
		sv := e.open(session("alice"), "log:apps/stray/c", "log:other/denied/c", "log:apps/web-0/c")
		evs := sv.rec.take()
		if got := eventNames(evs); !slices.Equal(got, []string{"open()", "closed(log:apps/stray/c)", "closed(log:other/denied/c)", "snapshot(log:apps/web-0/c)"}) {
			t.Fatalf("events = %v", got)
		}
		refused := strings.Replace(evs[1].Data, "apps/stray", "X", 1)
		denied := strings.Replace(evs[2].Data, "other/denied", "X", 1)
		if refused != denied || evs[1].code() != CodeForbidden {
			t.Errorf("an unadmitted topic closes unlike a denied one: %s vs %s", evs[1].Data, evs[2].Data)
		}
		if act, _ := e.prod.counts(mustTopic(t, "log:apps/stray/c")); act != 0 {
			t.Errorf("an unadmitted topic was activated %d times", act)
		}
		// Admission runs only after the topic's reads are allowed.
		if got := ad.calls(); !slices.Equal(got, []string{"alice log:apps/stray/c", "alice log:apps/web-0/c"}) {
			t.Errorf("Admit calls = %v, want none for the denied topic", got)
		}
	})
}

func TestAnAdmissionErrorClosesTheTopic(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{"unavailable", errors.New("read model not synced"), CodeUpstreamUnavailable},
		{"wrapped refusal", errorsJoin(ErrNotAdmitted), CodeForbidden},
		{"unauthenticated", &authz.DenialError{Code: authz.CodeUnauthenticated}, CodeUnauthenticated},
		{"forbidden denial", &authz.DenialError{Code: authz.CodeForbidden}, CodeForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e, ad := newAdmitEnv(t, Options{}, "alice")
				e.policy.set("alice", allowNamespaces("apps"))
				ad.set(func(Topic) error { return tc.err })
				evs := e.open(session("alice"), "log:apps/web-0/c").rec.take()
				if got := eventNames(evs); !slices.Equal(got, []string{"open()", "closed(log:apps/web-0/c)"}) || evs[1].code() != tc.code {
					t.Fatalf("events = %v (%s), want closed with %s", got, evs[len(evs)-1].Data, tc.code)
				}
			})
		})
	}
}

func errorsJoin(err error) error { return errors.Join(errors.New("pod not reachable"), err) }

func TestAReconnectIsAdmittedAgain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, ad := newAdmitEnv(t, Options{}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		web := mustTopic(t, "log:apps/web-0/c")
		sv := e.open(session("alice"), web.String())
		e.prod.upsert(t, web, Item{Event: EventLog, Attrs: authz.Attributes{Verb: "get", Resource: podsGVR, Subresource: "log", Namespace: "apps", Name: "web-0"}, Data: json.RawMessage(`{"seq":1}`)})
		synctest.Wait()
		if got := eventNames(sv.rec.take()); !slices.Equal(got, []string{"open()", "snapshot(log:apps/web-0/c)", "log(log:apps/web-0/c)"}) {
			t.Fatalf("events = %v", got)
		}
		last := sv.rec.lastID()
		_ = sv.disconnect()
		ad.set(refuseAll)
		evs := e.resume(session("alice"), last).rec.take()
		if got := eventNames(evs); !slices.Equal(got, []string{"open()", "closed(log:apps/web-0/c)"}) || evs[1].code() != CodeForbidden {
			t.Fatalf("events = %v", got)
		}
		if n := len(ad.calls()); n != 2 {
			t.Errorf("Admit called %d times, want once on open and once on reconnect", n)
		}
		if _, rel := e.prod.counts(web); rel != 1 {
			t.Errorf("released %d times, want 1", rel)
		}
	})
}

func TestLogTopicCaps(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, _ := newAdmitEnv(t, Options{MaxLogTopicsPerSession: 2, MaxLogTopics: 3}, "alice", "bob", "carol")
		for _, u := range []string{"alice", "bob", "carol"} {
			e.policy.set(u, allowNamespaces("apps"))
		}
		a := e.open(session("alice"), "log:apps/a/c", "log:apps/b/c", "instance:apps/blog")
		reviews := e.policy.count()

		// A third log topic in the session, on this stream or another one.
		if err := e.b.Subscribe(context.Background(), session("alice"), a.ID(), mustTopic(t, "log:apps/c/c")); !errors.Is(err, ErrTooManyTopics) {
			t.Errorf("third log topic on the stream: %v", err)
		}
		if _, err := e.b.Open(context.Background(), session("alice"), []Topic{mustTopic(t, "log:apps/c/c")}, ""); !errors.Is(err, ErrTooManyTopics) {
			t.Errorf("third log topic on a second stream: %v", err)
		}
		// Other topics are not log topics.
		if err := e.b.Subscribe(context.Background(), session("alice"), a.ID(), mustTopic(t, "instance:apps/web")); err != nil {
			t.Errorf("instance topic: %v", err)
		}
		if n := e.policy.count() - reviews; n != 1 {
			t.Errorf("%d reviews sent, want only the instance topic's", n)
		}

		// The process serves a/c and b/c: a shared topic does not count again.
		e.open(session("bob"), "log:apps/a/c", "log:apps/c/c")
		if _, err := e.b.Open(context.Background(), session("carol"), []Topic{mustTopic(t, "log:apps/d/c")}, ""); !errors.Is(err, ErrTooManyTopics) {
			t.Errorf("a fourth log topic in the process: %v", err)
		}
		e.open(session("carol"), "log:apps/b/c")

		// The attached topics keep streaming.
		b := mustTopic(t, "log:apps/b/c")
		e.prod.upsert(t, b, Item{Event: EventLog, Attrs: authz.Attributes{Verb: "get", Resource: podsGVR, Subresource: "log", Namespace: "apps", Name: "b"}, Data: json.RawMessage(`{"seq":1}`)})
		synctest.Wait()
		if got := eventNames(a.rec.take()); !slices.Contains(got, "log(log:apps/b/c)") {
			t.Errorf("alice's events = %v, want the log line", got)
		}

		// Leaving frees the session's room.
		if err := e.b.Unsubscribe(session("alice"), a.ID(), b); err != nil {
			t.Fatal(err)
		}
		if err := e.b.Subscribe(context.Background(), session("alice"), a.ID(), mustTopic(t, "log:apps/c/c")); err != nil {
			t.Errorf("log topic after leaving one: %v", err)
		}
	})
}

func TestMuxRoutesByKind(t *testing.T) {
	inst := newFakeProducer()
	ad := &admitting{fakeProducer: newFakeProducer()}
	ad.set(refuseAll)
	m := Mux{KindInstance: inst, KindLog: ad}

	blog := mustTopic(t, "instance:apps/blog")
	logTopic := mustTopic(t, "log:apps/web-0/c")
	if attrs, ok := m.Attributes(blog); !ok || attrs[0].Resource != instancesGVR {
		t.Errorf("instance topic attributes = %v, %v", attrs, ok)
	}
	if attrs, ok := m.Attributes(logTopic); !ok || attrs[0].Subresource != "log" {
		t.Errorf("log topic attributes = %v, %v", attrs, ok)
	}
	if _, ok := m.Attributes(mustTopic(t, "platform")); ok {
		t.Error("an unrouted kind is served")
	}
	if _, err := m.Snapshot(context.Background(), mustTopic(t, "platform")); err == nil {
		t.Error("an unrouted kind has a snapshot")
	}
	m.Activate(mustTopic(t, "platform"))()
	release := m.Activate(blog)
	release()
	if act, rel := inst.counts(blog); act != 1 || rel != 1 {
		t.Errorf("instance producer activations %d releases %d", act, rel)
	}
	if err := m.Admit(context.Background(), user("alice"), blog, nil); err != nil {
		t.Errorf("a producer that is no Admitter refused: %v", err)
	}
	if err := m.Admit(context.Background(), user("alice"), logTopic, nil); err == nil {
		t.Error("the log producer's Admit was not asked")
	}
	m.Follow(blog)
	m.Follow(mustTopic(t, "platform"))
	m.Follow(logTopic)
	if got := ad.follows(); !slices.Equal(got, []string{logTopic.String()}) {
		t.Errorf("Follow reached the log producer with %v", got)
	}
}

func TestOnlyANewSubscriptionToAnActiveTopicFollowsIt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, ad := newAdmitEnv(t, Options{}, "alice", "bob")
		e.policy.set("alice", allowNamespaces("apps"))
		e.policy.set("bob", allowNamespaces("apps"))
		web := mustTopic(t, "log:apps/web-0/c")
		e.open(session("alice"), web.String())
		synctest.Wait()
		if got := ad.follows(); len(got) != 0 {
			t.Fatalf("the activating subscription followed: %v", got)
		}

		bob := e.open(session("bob"), web.String())
		synctest.Wait()
		if got := ad.follows(); !slices.Equal(got, []string{web.String()}) {
			t.Fatalf("follows after bob opened = %v", got)
		}

		// A reconnect resumes bob's subscription: it is no new follow.
		last := bob.rec.lastID()
		_ = bob.disconnect()
		bob = e.resume(session("bob"), last)
		synctest.Wait()
		if got := ad.follows(); len(got) != 1 {
			t.Fatalf("follows after a reconnect = %v, want none added", got)
		}

		// Unsubscribing and subscribing again is.
		if err := e.b.Unsubscribe(session("bob"), bob.ID(), web); err != nil {
			t.Fatal(err)
		}
		if err := e.b.Subscribe(context.Background(), session("bob"), bob.ID(), web); err != nil {
			t.Fatal(err)
		}
		if got := ad.follows(); len(got) != 2 {
			t.Errorf("follows after a re-follow = %v, want 2", got)
		}
	})
}

func TestARefusedAttachDoesNotFollow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, ad := newAdmitEnv(t, Options{MaxLogTopicsPerSession: 1}, "alice", "bob")
		e.policy.set("alice", allowNamespaces("apps"))
		e.policy.set("bob", allowNamespaces("apps"))
		web := mustTopic(t, "log:apps/web-0/c")
		e.open(session("alice"), web.String())
		bob := e.open(session("bob"), "instance:apps/blog")
		other := e.open(session("bob"), "instance:apps/blog")

		// While bob's subscription to web is being admitted, his other
		// stream takes the session's one log topic, so the cap re-checked
		// after admission refuses web.
		var once sync.Once
		ad.set(func(t Topic) error {
			if t == web {
				once.Do(func() {
					_ = e.b.Subscribe(context.Background(), session("bob"), other.ID(), mustTopic(e.t, "log:apps/db-0/c"))
				})
			}
			return nil
		})
		if err := e.b.Subscribe(context.Background(), session("bob"), bob.ID(), web); !errors.Is(err, ErrTooManyTopics) {
			t.Fatalf("Subscribe = %v, want ErrTooManyTopics from the re-check", err)
		}
		if got := ad.follows(); slices.Contains(got, web.String()) {
			t.Errorf("a refused attach followed: %v", got)
		}
	})
}

func TestOnlyLogTopicRingsAreBoundedByBytes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, _ := newAdmitEnv(t, Options{LogRingBytes: 10}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		blog := mustTopic(t, "instance:apps/blog")
		web := mustTopic(t, "log:apps/web-0/c")
		sv := e.open(session("alice"), blog.String(), web.String())
		synctest.Wait()
		last := sv.rec.lastID()
		_ = sv.disconnect()
		logAttrs := authz.Attributes{Verb: "get", Resource: podsGVR, Subresource: "log", Namespace: "apps", Name: "web-0"}
		for v := 1; v <= 3; v++ {
			e.prod.upsert(t, blog, instItem("apps", "blog", v))
			e.prod.upsert(t, web, Item{Event: EventLog, Attrs: logAttrs, Data: json.RawMessage(`{"seq":` + strings.Repeat("1", 20) + `}`)})
		}
		got := eventNames(e.resume(session("alice"), last).rec.take())
		want := []string{"open()", "upsert(instance:apps/blog)", "upsert(instance:apps/blog)", "upsert(instance:apps/blog)", "snapshot(log:apps/web-0/c)"}
		if !slices.Equal(got, want) {
			t.Errorf("events = %v, want the object topic replayed and the log topic snapshotted", got)
		}
	})
}
