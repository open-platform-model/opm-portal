package stream

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// perReader is an instance document rendered per subscriber: it counts the
// Pods the reader may see, the way the read API's health roll-up counts only
// what the caller may read. Changing a Pod alice may not read changes bob's
// document and leaves hers as it was.
type perReader struct {
	mu   sync.Mutex
	pods map[string]int // pod name -> version
}

func (r *perReader) set(name string, v int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pods[name] = v
}

// item is apps/blog's document as an item.
func (r *perReader) item() Item {
	const name = "blog"
	return Item{
		Event: EventUpsert,
		Attrs: authz.Attributes{Verb: "get", Resource: instancesGVR, Namespace: "apps", Name: name},
		Render: func(_ context.Context, who authz.Identity) (json.RawMessage, error) {
			r.mu.Lock()
			defer r.mu.Unlock()
			seen := map[string]int{}
			for pod, v := range r.pods {
				if who.Username == "bob" || pod == "web-0" {
					seen[pod] = v
				}
			}
			return json.Marshal(struct {
				Name string         `json:"name"`
				Pods map[string]int `json:"pods"`
			}{name, seen})
		},
	}
}

// TestAChangeASubscriberCannotSeeSendsThemNothing: the stream compares each
// subscriber's rendered document with the last one written to that
// subscriber and writes only a difference, so a change to something alice
// may not read reaches bob and sends alice nothing: no event, no id, and so
// no signal of when it happened.
func TestAChangeASubscriberCannotSeeSendsThemNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice", "bob")
		e.policy.set("alice", allowNamespaces("apps"))
		e.policy.set("bob", allowNamespaces("apps"))
		blog := mustTopic(t, "instance:apps/blog")
		doc := &perReader{pods: map[string]int{"web-0": 1, "db-0": 1}}
		e.prod.seed(blog, "apps/blog", doc.item())

		a := e.open(session("alice"), "instance:apps/blog")
		b := e.open(session("bob"), "instance:apps/blog")
		for _, sv := range []*served{a, b} {
			if got := eventNames(sv.rec.take()); !slices.Equal(got, []string{"open()", "snapshot(instance:apps/blog)"}) {
				t.Fatalf("first events = %v", got)
			}
		}

		// db-0 changes: only bob may see it.
		doc.set("db-0", 2)
		e.prod.upsert(t, blog, doc.item())
		synctest.Wait()
		if got := a.rec.take(); len(got) != 0 {
			t.Errorf("alice received %v for a change she cannot see", eventNames(got))
		}
		if got := eventNames(b.rec.take()); !slices.Equal(got, []string{"upsert(instance:apps/blog)"}) {
			t.Errorf("bob received %v, want one upsert", got)
		}

		// A re-render with nothing changed (a periodic refresh) sends
		// nobody anything.
		e.prod.upsert(t, blog, doc.item())
		synctest.Wait()
		if got := append(a.rec.take(), b.rec.take()...); len(got) != 0 {
			t.Errorf("an unchanged re-render sent %v", eventNames(got))
		}

		// web-0 changes: both see it, and alice's id follows her snapshot's
		// with no gap, so the change she was not sent left no trace.
		doc.set("web-0", 2)
		e.prod.upsert(t, blog, doc.item())
		synctest.Wait()
		evs := a.rec.take()
		if got := eventNames(evs); !slices.Equal(got, []string{"upsert(instance:apps/blog)"}) {
			t.Fatalf("alice received %v, want one upsert", got)
		}
		if got := seqOf(t, evs[0].ID); got != 2 {
			t.Errorf("alice's upsert has id %d, want 2 (right after her snapshot)", got)
		}
		if got := eventNames(b.rec.take()); !slices.Equal(got, []string{"upsert(instance:apps/blog)"}) {
			t.Errorf("bob received %v, want one upsert", got)
		}
	})
}

// TestAReconnectWritesNoDocumentTheClientHolds: a client that resumes from
// the event its topic's last document was written in still holds that
// document, so a replayed item equal to it is not written: its replay would
// tell the client when a change it cannot see happened. A different
// document is written as usual.
func TestAReconnectWritesNoDocumentTheClientHolds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		blog := mustTopic(t, "instance:apps/blog")
		e.prod.upsert(t, blog, instItem("apps", "blog", 1))
		sv := e.open(session("alice"), "instance:apps/blog")
		e.prod.upsert(t, blog, instItem("apps", "blog", 2))
		synctest.Wait()
		last := sv.rec.lastID()
		_ = sv.disconnect()

		// The same document again while the stream is away.
		e.prod.upsert(t, blog, instItem("apps", "blog", 2))
		back := e.resume(session("alice"), last)
		synctest.Wait()
		if got := eventNames(back.rec.take()); !slices.Equal(got, []string{"open()"}) {
			t.Fatalf("after the reconnect: %v, want nothing but the open event", got)
		}

		// A different document is still written, with the id right after
		// the client's last one.
		e.prod.upsert(t, blog, instItem("apps", "blog", 3))
		synctest.Wait()
		evs := back.rec.take()
		if got := eventNames(evs); !slices.Equal(got, []string{"upsert(instance:apps/blog)"}) {
			t.Fatalf("after a change: %v, want one upsert", got)
		}
		if got, want := seqOf(t, evs[0].ID), seqOf(t, last)+1; got != want {
			t.Errorf("the upsert has id %d, want %d", got, want)
		}
	})
}

// TestAReconnectFromBeforeTheLastDocumentWritesIt: a client that resumes
// from an event before the one its topic's last document was written in may
// not hold that document, so the first one replayed is written whatever it
// holds, and only later equal ones are left out.
func TestAReconnectFromBeforeTheLastDocumentWritesIt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		blog := mustTopic(t, "instance:apps/blog")
		e.prod.upsert(t, blog, instItem("apps", "blog", 1))
		sv := e.open(session("alice"), "instance:apps/blog")
		synctest.Wait()
		snapshotID := sv.rec.lastID()
		e.prod.upsert(t, blog, instItem("apps", "blog", 2))
		synctest.Wait()
		_ = sv.disconnect()

		e.prod.upsert(t, blog, instItem("apps", "blog", 2))
		back := e.resume(session("alice"), snapshotID)
		synctest.Wait()
		if got := eventNames(back.rec.take()); !slices.Equal(got, []string{"open()", "upsert(instance:apps/blog)"}) {
			t.Errorf("after the reconnect: %v, want the missed upsert written once", got)
		}
	})
}

// TestLogLinesAreNeverCompared: a log line is a record, not a document, so
// two equal lines are both written.
func TestLogLinesAreNeverCompared(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		// An object topic whose producer publishes log events stands in for
		// a log topic: the broker decides on the event, not the topic kind.
		blog := mustTopic(t, "instance:apps/blog")
		e.prod.seed(blog, "apps/blog", instItem("apps", "blog", 1))
		sv := e.open(session("alice"), "instance:apps/blog")
		line := Item{
			Event: EventLog,
			Attrs: authz.Attributes{Verb: "get", Resource: instancesGVR, Namespace: "apps", Name: "blog"},
			Data:  json.RawMessage(`{"text":"same"}`),
		}
		for range 2 {
			if err := e.b.Publish(blog, line); err != nil {
				t.Fatal(err)
			}
		}
		synctest.Wait()
		var logs int
		for _, ev := range sv.rec.take() {
			if ev.Event == EventLog {
				logs++
			}
		}
		if logs != 2 {
			t.Errorf("%d log events, want 2", logs)
		}
	})
}

// TestTheComparisonIsPerSubscriber: alice reconnecting does not make bob
// receive anything again, and a topic alice removed and added again starts
// from its snapshot.
func TestTheComparisonIsPerSubscriber(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{HeartbeatInterval: time.Hour}, "alice", "bob")
		allow := func(authorizationv1.ResourceAttributes) (bool, error) { return true, nil }
		e.policy.set("alice", allow)
		e.policy.set("bob", allow)
		blog := mustTopic(t, "instance:apps/blog")
		e.prod.upsert(t, blog, instItem("apps", "blog", 1))
		a := e.open(session("alice"), "instance:apps/blog")
		b := e.open(session("bob"), "instance:apps/blog")
		_, _ = a.rec.take(), b.rec.take()

		if err := e.b.Unsubscribe(session("alice"), a.ID(), blog); err != nil {
			t.Fatal(err)
		}
		if err := e.b.Subscribe(context.Background(), session("alice"), a.ID(), blog); err != nil {
			t.Fatal(err)
		}
		e.prod.upsert(t, blog, instItem("apps", "blog", 1))
		synctest.Wait()
		if got := eventNames(a.rec.take()); !slices.Equal(got, []string{"snapshot(instance:apps/blog)"}) {
			t.Errorf("alice after re-adding the topic: %v, want only its snapshot", got)
		}
		if got := b.rec.take(); len(got) != 0 {
			t.Errorf("bob received %v for an unchanged document", eventNames(got))
		}
	})
}

// TestADeleteIsAlwaysWritten: a delete is never left out, even when an
// upsert already carried its Removed document. The client drops the topic's
// object on the delete event, and when the object went is its own, which
// every subscriber of the topic may see.
func TestADeleteIsAlwaysWritten(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		blog := mustTopic(t, "instance:apps/blog")
		e.prod.upsert(t, blog, instItem("apps", "blog", 1))
		sv := e.open(session("alice"), "instance:apps/blog")
		synctest.Wait()
		sv.rec.take()

		removed := json.RawMessage(`{"kind": "Removed", "name": "blog", "namespace": "apps"}`)
		gone := Item{Event: EventUpsert, Attrs: instItem("apps", "blog", 0).Attrs, Data: removed}
		e.prod.upsert(t, blog, gone)
		synctest.Wait()
		gone.Event = EventDelete
		if err := e.b.Publish(blog, gone); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if got := eventNames(sv.rec.take()); !slices.Equal(got, []string{"upsert(instance:apps/blog)", "delete(instance:apps/blog)"}) {
			t.Errorf("events = %v, want the upsert and the delete", got)
		}
	})
}

// TestAResumeThatFallsBackToASnapshotComparesWithIt: a resume the stream
// cannot replay (the ring lost part of the gap, or the stream no longer
// remembers the client's event id) writes a fresh snapshot, and later items
// are compared with that snapshot's document: an equal one is not written,
// a different one is.
func TestAResumeThatFallsBackToASnapshotComparesWithIt(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts Options
		// live reports whether the changes reach the stream before it
		// disconnects (so its event ids move on) or only after.
		live bool
	}{
		{name: "the ring lost the gap", opts: Options{RingSize: 2}},
		{name: "the event id is forgotten", opts: Options{QueueSize: 2}, live: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e := newEnv(t, tc.opts, "alice")
				sv, blog := openBlog(t, e)
				first := sv.rec.lastID()
				if !tc.live {
					_ = sv.disconnect()
				}
				for v := 2; v <= 4; v++ {
					e.prod.upsert(t, blog, instItem("apps", "blog", v))
					synctest.Wait()
				}
				if tc.live {
					_ = sv.disconnect()
				}
				back := e.resume(session("alice"), first, "instance:apps/blog")
				synctest.Wait()
				if got := eventNames(back.rec.take()); !slices.Equal(got, []string{"open()", "snapshot(instance:apps/blog)"}) {
					t.Fatalf("after the resume: %v, want a snapshot", got)
				}

				e.prod.upsert(t, blog, instItem("apps", "blog", 4))
				synctest.Wait()
				if got := back.rec.take(); len(got) != 0 {
					t.Errorf("an item equal to the snapshot sent %v", eventNames(got))
				}
				e.prod.upsert(t, blog, instItem("apps", "blog", 5))
				synctest.Wait()
				if got := eventNames(back.rec.take()); !slices.Equal(got, []string{"upsert(instance:apps/blog)"}) {
					t.Errorf("after a change: %v, want one upsert", got)
				}
			})
		})
	}
}
