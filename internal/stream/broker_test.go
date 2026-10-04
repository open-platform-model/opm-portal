package stream

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
)

// seqOf returns the sequence an event id carries.
func seqOf(t *testing.T, id string) uint64 {
	t.Helper()
	parts := strings.Split(id, ".")
	if len(parts) != 3 {
		t.Fatalf("event id %q is not epoch.stream.seq", id)
	}
	n, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSnapshotThenChanges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		blog := mustTopic(t, "instance:apps/blog")
		e.prod.upsert(t, blog, instItem("apps", "blog", 1))

		sv := e.open(session("alice"), "instance:apps/blog")
		e.prod.upsert(t, blog, instItem("apps", "blog", 2))
		e.prod.upsert(t, blog, instItem("apps", "blog", 3))
		synctest.Wait()

		evs := sv.rec.take()
		if got := eventNames(evs); !slices.Equal(got, []string{"open()", "snapshot(instance:apps/blog)", "upsert(instance:apps/blog)", "upsert(instance:apps/blog)"}) {
			t.Fatalf("events = %v", got)
		}
		if evs[0].Retry != "3000" || !strings.Contains(evs[0].Data, sv.ID()) || evs[0].ID != "" {
			t.Errorf("open event = %+v", evs[0])
		}
		if got := evs[1].versions(t); !slices.Equal(got, []int{1}) {
			t.Errorf("snapshot versions = %v", got)
		}
		if evs[2].versions(t)[0] != 2 || evs[3].versions(t)[0] != 3 {
			t.Errorf("upserts = %s, %s", evs[2].Data, evs[3].Data)
		}
		var last uint64
		for _, ev := range evs[1:] {
			s := seqOf(t, ev.ID)
			if s <= last {
				t.Errorf("event ids do not increase: %d after %d", s, last)
			}
			last = s
		}
	})
}

func TestChangeRacingTheSnapshotIsNotLost(t *testing.T) {
	for _, publishFirst := range []bool{false, true} {
		t.Run("publish before snapshot read="+strconv.FormatBool(publishFirst), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e := newEnv(t, Options{}, "alice")
				e.policy.set("alice", allowNamespaces("apps"))
				blog := mustTopic(t, "instance:apps/blog")
				e.prod.upsert(t, blog, instItem("apps", "blog", 1))
				racer := func() { e.prod.upsert(t, blog, instItem("apps", "blog", 2)) }
				if publishFirst {
					// The change lands between registration and the snapshot read.
					e.prod.duringSnapshot = nil
					s, err := e.b.Open(context.Background(), session("alice"), []Topic{blog}, "")
					if err != nil {
						t.Fatal(err)
					}
					racer()
					sv := e.serve(s)
					assertLastVersion(t, sv.rec.take(), 2)
					return
				}
				// The change lands after the snapshot was read.
				e.prod.duringSnapshot = racer
				sv := e.open(session("alice"), "instance:apps/blog")
				assertLastVersion(t, sv.rec.take(), 2)
			})
		})
	}
}

func assertLastVersion(t *testing.T, evs []sse, want int) {
	t.Helper()
	got := -1
	for _, ev := range evs {
		if ev.Event == EventSnapshot || ev.Event == EventUpsert {
			if vs := ev.versions(t); len(vs) > 0 {
				got = vs[len(vs)-1]
			}
		}
	}
	if got != want {
		t.Errorf("client ends at version %d, want %d (events %v)", got, want, eventNames(evs))
	}
}

func TestTopicsAddedAndRemovedOnAnOpenStream(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		blog, web := mustTopic(t, "instance:apps/blog"), mustTopic(t, "instance:apps/web")
		e.prod.upsert(t, web, instItem("apps", "web", 1))
		sv := e.open(session("alice"), "instance:apps/blog")
		sv.rec.take()

		if err := e.b.Subscribe(context.Background(), session("alice"), sv.ID(), web); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if got := eventNames(sv.rec.take()); !slices.Equal(got, []string{"snapshot(instance:apps/web)"}) {
			t.Fatalf("after Subscribe: %v", got)
		}
		// Subscribing again changes nothing.
		if err := e.b.Subscribe(context.Background(), session("alice"), sv.ID(), web); err != nil {
			t.Fatal(err)
		}
		if err := e.b.Unsubscribe(session("alice"), sv.ID(), blog); err != nil {
			t.Fatal(err)
		}
		e.prod.upsert(t, blog, instItem("apps", "blog", 2))
		e.prod.upsert(t, web, instItem("apps", "web", 2))
		synctest.Wait()
		if got := eventNames(sv.rec.take()); !slices.Equal(got, []string{"upsert(instance:apps/web)"}) {
			t.Fatalf("after Unsubscribe: %v", got)
		}
	})
}

func TestAnotherSessionCannotChangeAStream(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice", "bob")
		e.policy.set("alice", allowNamespaces("apps"))
		e.policy.set("bob", allowNamespaces("apps"))
		sv := e.open(session("alice"), "instance:apps/blog")
		web := mustTopic(t, "instance:apps/web")
		errForeign := e.b.Subscribe(context.Background(), session("bob"), sv.ID(), web)
		errMissing := e.b.Subscribe(context.Background(), session("bob"), "no-such-stream", web)
		if !errors.Is(errForeign, ErrNoStream) || errForeign.Error() != errMissing.Error() {
			t.Errorf("foreign = %v, missing = %v; want the same ErrNoStream", errForeign, errMissing)
		}
		if err := e.b.Unsubscribe(session("bob"), sv.ID(), web); !errors.Is(err, ErrNoStream) {
			t.Errorf("foreign Unsubscribe = %v", err)
		}
	})
}

func TestActivateAndReleaseAreRefcounted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice", "bob")
		e.policy.set("alice", allowNamespaces("apps"))
		e.policy.set("bob", allowNamespaces("apps"))
		blog := mustTopic(t, "instance:apps/blog")
		a := e.open(session("alice"), "instance:apps/blog")
		b := e.open(session("bob"), "instance:apps/blog")
		if act, rel := e.prod.counts(blog); act != 1 || rel != 0 {
			t.Fatalf("after two subscribers: activations=%d releases=%d", act, rel)
		}
		if err := e.b.Unsubscribe(session("alice"), a.ID(), blog); err != nil {
			t.Fatal(err)
		}
		if act, rel := e.prod.counts(blog); act != 1 || rel != 0 {
			t.Fatalf("after one left: activations=%d releases=%d", act, rel)
		}
		if err := e.b.Unsubscribe(session("bob"), b.ID(), blog); err != nil {
			t.Fatal(err)
		}
		if act, rel := e.prod.counts(blog); act != 1 || rel != 1 {
			t.Fatalf("after both left: activations=%d releases=%d", act, rel)
		}
		// Publishing to a topic nobody follows records nothing.
		e.prod.upsert(t, blog, instItem("apps", "blog", 9))

		web := mustTopic(t, "instance:apps/web")
		e.open(session("alice"), "instance:apps/web")
		e.b.Close()
		synctest.Wait()
		if act, rel := e.prod.counts(web); act != 1 || rel != 1 {
			t.Fatalf("after Close: activations=%d releases=%d", act, rel)
		}
	})
}

func TestSubscriberReceivesOnlyWhatItMayRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice", "bob")
		// alice may list instances everywhere but get them only in team-a.
		e.policy.set("alice", func(a authorizationv1.ResourceAttributes) (bool, error) {
			return a.Verb == "list" || a.Namespace == "team-a", nil
		})
		e.policy.set("bob", func(authorizationv1.ResourceAttributes) (bool, error) { return true, nil })
		all := mustTopic(t, "instances")
		e.prod.upsert(t, all, instItem("team-a", "one", 1))
		e.prod.upsert(t, all, instItem("team-b", "two", 1))

		a := e.open(session("alice"), "instances")
		b := e.open(session("bob"), "instances")
		e.prod.upsert(t, all, instItem("team-b", "two", 2))
		e.prod.upsert(t, all, instItem("team-a", "one", 2))
		e.prod.upsert(t, all, instItem("team-b", "three", 1))
		synctest.Wait()

		var aliceSaw, bobSaw []string
		for _, ev := range a.rec.take() {
			if ev.Event == EventSnapshot || ev.Event == EventUpsert {
				aliceSaw = append(aliceSaw, ev.names(t)...)
			}
		}
		for _, ev := range b.rec.take() {
			if ev.Event == EventSnapshot || ev.Event == EventUpsert {
				bobSaw = append(bobSaw, ev.names(t)...)
			}
		}
		if want := []string{"team-a/one", "team-a/one"}; !slices.Equal(aliceSaw, want) {
			t.Errorf("alice saw %v, want %v", aliceSaw, want)
		}
		if want := []string{"team-a/one", "team-b/two", "team-b/two", "team-a/one", "team-b/three"}; !slices.Equal(bobSaw, want) {
			t.Errorf("bob saw %v, want %v", bobSaw, want)
		}
	})
}

func TestAForbiddenTopicIsClosedNotServed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		e.policy.set("alice", allowNamespaces("other"))
		blog := mustTopic(t, "instance:apps/blog")
		e.prod.upsert(t, blog, instItem("apps", "blog", 1))

		sv := e.open(session("alice"), "instance:apps/blog", "instance:apps/missing")
		e.prod.upsert(t, blog, instItem("apps", "blog", 2))
		synctest.Wait()
		evs := sv.rec.take()
		if got := eventNames(evs); !slices.Equal(got, []string{"open()", "closed(instance:apps/blog)", "closed(instance:apps/missing)"}) {
			t.Fatalf("events = %v", got)
		}
		for _, ev := range evs[1:] {
			if ev.code() != CodeForbidden || ev.ID != "" {
				t.Errorf("closed event %+v", ev)
			}
		}
		existing := strings.Replace(evs[1].Data, "blog", "X", 1)
		missing := strings.Replace(evs[2].Data, "missing", "X", 1)
		if existing != missing {
			t.Errorf("closing differs for an existing and a missing object: %s vs %s", evs[1].Data, evs[2].Data)
		}
		if act, _ := e.prod.counts(blog); act != 0 {
			t.Errorf("a denied topic was activated %d times", act)
		}
	})
}

func TestARevokedPermissionClosesTheTopic(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		blog := mustTopic(t, "instance:apps/blog")
		e.prod.upsert(t, blog, instItem("apps", "blog", 1))
		sv := e.open(session("alice"), "instance:apps/blog")
		sv.rec.take()

		e.policy.set("alice", allowNamespaces())
		// The decision behind the grant lives 30 s; heartbeats re-check it.
		time.Sleep(31 * time.Second)
		synctest.Wait()
		e.prod.upsert(t, blog, instItem("apps", "blog", 2))
		synctest.Wait()

		var got []string
		for _, ev := range sv.rec.take() {
			if ev.Event != EventHeartbeat {
				got = append(got, ev.Event+"("+ev.topic()+")"+ev.code())
			}
		}
		if !slices.Equal(got, []string{"closed(instance:apps/blog)forbidden"}) {
			t.Errorf("events after revocation = %v", got)
		}
		if _, rel := e.prod.counts(blog); rel != 1 {
			t.Errorf("the closed topic was released %d times", rel)
		}
	})
}

func TestARevokedPermissionIsSeenOnTheNextDelivery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{HeartbeatInterval: time.Hour}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		blog := mustTopic(t, "instance:apps/blog")
		sv := e.open(session("alice"), "instance:apps/blog")
		sv.rec.take()
		e.policy.set("alice", allowNamespaces())
		time.Sleep(30 * time.Second)
		e.prod.upsert(t, blog, instItem("apps", "blog", 2))
		synctest.Wait()
		if got := eventNames(sv.rec.take()); !slices.Equal(got, []string{"closed(instance:apps/blog)"}) {
			t.Errorf("events = %v", got)
		}
	})
}

func TestAnAuthorizationErrorDeliversNothing(t *testing.T) {
	failing := func(authorizationv1.ResourceAttributes) (bool, error) {
		return false, errors.New("webhook timed out")
	}
	t.Run("on subscribe", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			e := newEnv(t, Options{}, "alice")
			e.policy.set("alice", failing)
			blog := mustTopic(t, "instance:apps/blog")
			e.prod.upsert(t, blog, instItem("apps", "blog", 1))
			sv := e.open(session("alice"), "instance:apps/blog")
			evs := sv.rec.take()
			if got := eventNames(evs); !slices.Equal(got, []string{"open()", "closed(instance:apps/blog)"}) || evs[1].code() != CodeUpstreamUnavailable {
				t.Errorf("events = %v %+v", got, evs)
			}
		})
	})
	t.Run("on an item", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			e := newEnv(t, Options{}, "alice")
			e.policy.set("alice", func(a authorizationv1.ResourceAttributes) (bool, error) {
				if a.Namespace == "team-b" {
					return false, errors.New("webhook timed out")
				}
				return true, nil
			})
			all := mustTopic(t, "instances")
			sv := e.open(session("alice"), "instances")
			sv.rec.take()
			e.prod.upsert(t, all, instItem("team-b", "two", 1))
			e.prod.upsert(t, all, instItem("team-a", "one", 1))
			synctest.Wait()
			evs := sv.rec.take()
			if got := eventNames(evs); !slices.Equal(got, []string{"closed(instances)"}) || evs[0].code() != CodeUpstreamUnavailable {
				t.Errorf("events = %v", got)
			}
		})
	})
	t.Run("snapshot failure", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			e := newEnv(t, Options{}, "alice")
			e.policy.set("alice", allowNamespaces("apps"))
			e.prod.snapshotErr = errors.New("cache not synced")
			sv := e.open(session("alice"), "instance:apps/blog")
			evs := sv.rec.take()
			if got := eventNames(evs); !slices.Equal(got, []string{"open()", "closed(instance:apps/blog)"}) || evs[1].code() != CodeUpstreamUnavailable {
				t.Errorf("events = %v", got)
			}
		})
	})
}

func TestAnUnauthenticatedSessionOpensNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		blog := mustTopic(t, "instance:apps/blog")
		for _, s := range []Session{
			{Key: "k"},
			{Key: "k", Identity: user(" ")},
			{Key: "k", Identity: user("system:anonymous")},
			{Identity: user("alice")},
		} {
			if _, err := e.b.Open(context.Background(), s, []Topic{blog}, ""); !errors.Is(err, ErrUnauthenticated) {
				t.Errorf("Open(%+v) = %v, want ErrUnauthenticated", s, err)
			}
		}
		if n := e.policy.count(); n != 0 {
			t.Errorf("%d reviews sent for unauthenticated sessions", n)
		}
	})
}

func TestTopicsThatCannotAttach(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{MaxTopicsPerStream: 2}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		logTopic := mustTopic(t, "log:apps/blog-0/server")
		if _, err := e.b.Open(context.Background(), session("alice"), []Topic{logTopic}, ""); !errors.Is(err, ErrTopicNotServed) {
			t.Errorf("log topic: %v", err)
		}
		var zero Topic
		if _, err := e.b.Open(context.Background(), session("alice"), []Topic{zero}, ""); !errors.Is(err, ErrTopicNotServed) {
			t.Errorf("zero topic: %v", err)
		}
		three := []Topic{mustTopic(t, "instance:apps/a"), mustTopic(t, "instance:apps/b"), mustTopic(t, "instance:apps/c")}
		if _, err := e.b.Open(context.Background(), session("alice"), three, ""); !errors.Is(err, ErrTooManyTopics) {
			t.Errorf("three topics: %v", err)
		}
		sv := e.open(session("alice"), "instance:apps/a", "instance:apps/a", "instance:apps/b")
		if err := e.b.Subscribe(context.Background(), session("alice"), sv.ID(), three[2]); !errors.Is(err, ErrTooManyTopics) {
			t.Errorf("third topic on an open stream: %v", err)
		}
		if n := e.policy.count(); n != 2 {
			t.Errorf("%d reviews sent, want 2 (the two attached topics)", n)
		}
	})
}
