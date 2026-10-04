package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"

	"github.com/open-platform-model/opm-portal/internal/authz"
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

// A list topic follows the read model's list rule: it needs the list grant a
// GET list needs, cluster-wide for "instances" and on the namespace for
// "instances:<ns>", and carries only the items within that grant's scope,
// with no review per item (0030:D7:R2).
func TestAListTopicFollowsTheListGrant(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice", "bob")
		// alice may list instances in team-a only.
		e.policy.set("alice", func(a authorizationv1.ResourceAttributes) (bool, error) {
			return a.Namespace == "team-a" && a.Verb == "list", nil
		})
		e.policy.set("bob", func(authorizationv1.ResourceAttributes) (bool, error) { return true, nil })
		all := mustTopic(t, "instances")
		teamA := mustTopic(t, "instances:team-a")
		for _, tp := range []Topic{all, teamA} {
			e.prod.upsert(t, tp, listItem("team-a", "one", 1))
		}
		e.prod.upsert(t, all, listItem("team-b", "two", 1))

		a := e.open(session("alice"), "instances", "instances:team-a")
		if n := e.policy.count(); n != 2 {
			t.Errorf("%d reviews for alice's two topics, want 2", n)
		}
		b := e.open(session("bob"), "instances")
		for i := 2; i <= 50; i++ {
			e.prod.upsert(t, all, listItem("team-b", "two", i))
			e.prod.upsert(t, all, listItem("team-a", "one", i))
			e.prod.upsert(t, teamA, listItem("team-a", "one", i))
		}
		// A producer that strays outside the topic's namespace is not
		// trusted: the item is left out, and still nothing is reviewed.
		e.prod.upsert(t, teamA, listItem("team-b", "two", 51))
		synctest.Wait()

		// Only the topic decisions (alice's two, bob's one) were asked,
		// however many items went out.
		if n := e.policy.count(); n != 3 {
			t.Errorf("%d reviews sent, want 3 (one per topic, none per item)", n)
		}
		aliceSaw, closed := delivered(t, a.rec.take())
		// The same denial a GET list of every namespace gives her.
		if want := []string{"instances " + string(authz.CodeForbidden)}; !slices.Equal(closed, want) {
			t.Errorf("alice's closings = %v, want %v", closed, want)
		}
		if want := slices.Repeat([]string{"instances:team-a team-a/one"}, 50); !slices.Equal(aliceSaw, want) {
			t.Errorf("alice saw %v, want team-a/one 50 times on instances:team-a", aliceSaw)
		}
		bobSaw, _ := delivered(t, b.rec.take())
		if want := 2 + 2*49; len(bobSaw) != want {
			t.Errorf("bob saw %d items, want %d", len(bobSaw), want)
		}
	})
}

// delivered splits evs into the items delivered, as "topic ns/name", and the
// closings, as "topic code".
func delivered(t *testing.T, evs []sse) (items, closed []string) {
	t.Helper()
	for _, ev := range evs {
		switch ev.Event {
		case EventSnapshot, EventUpsert:
			for _, n := range ev.names(t) {
				items = append(items, ev.topic()+" "+n)
			}
		case EventClosed:
			closed = append(closed, ev.topic()+" "+ev.code())
		}
	}
	return items, closed
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
				if a.Resource == "pods" {
					return false, errors.New("webhook timed out")
				}
				return true, nil
			})
			one := mustTopic(t, "instance:team-a/one")
			sv := e.open(session("alice"), "instance:team-a/one")
			sv.rec.take()
			// An item on an object topic that reveals another read is
			// reviewed on its own.
			pod := instItem("team-a", "one", 1)
			pod.Attrs = authz.Attributes{Verb: "get", Resource: podsGVR, Namespace: "team-a", Name: "one-0"}
			e.prod.upsert(t, one, pod)
			e.prod.upsert(t, one, instItem("team-a", "one", 2))
			synctest.Wait()
			evs := sv.rec.take()
			if got := eventNames(evs); !slices.Equal(got, []string{"closed(instance:team-a/one)"}) || evs[0].code() != CodeUpstreamUnavailable {
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

// Event ids count the stream's own events only: items left out for the
// reader, items a producer strays with, other topics and other streams leave
// no gap (0030:D7:R2).
func TestEventIDsRevealNothingPublishedElsewhere(t *testing.T) {
	t.Run("items the reader may not read", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			e := newEnv(t, Options{}, "alice", "bob")
			// alice may read instance team-a/one and the pod web-0 only.
			e.policy.set("alice", func(a authorizationv1.ResourceAttributes) (bool, error) {
				switch a.Resource {
				case instancesGVR.Resource:
					return a.Namespace == "team-a" && a.Name == "one", nil
				case podsGVR.Resource:
					return a.Namespace == "team-a" && a.Name == "web-0", nil
				}
				return false, nil
			})
			e.policy.set("bob", func(authorizationv1.ResourceAttributes) (bool, error) { return true, nil })
			one := mustTopic(t, "instance:team-a/one")
			a := e.open(session("alice"), "instance:team-a/one")
			e.open(session("bob"), "instance:team-a/one")
			for v := 1; v <= 5; v++ {
				// Five changes on the topic; alice may read the first, third
				// and fifth, which reveal web-0, and not the two of db-0.
				name := "web-0"
				if v%2 == 0 {
					name = "db-0"
				}
				e.prod.upsert(t, one, podItem("team-a", name, v))
			}
			synctest.Wait()

			var ids []uint64
			var vs []int
			for _, ev := range a.rec.take() {
				if ev.ID != "" {
					ids = append(ids, seqOf(t, ev.ID))
				}
				if ev.Event == EventUpsert {
					vs = append(vs, ev.versions(t)...)
				}
			}
			if want := []int{1, 3, 5}; !slices.Equal(vs, want) {
				t.Errorf("alice saw versions %v, want %v", vs, want)
			}
			// The snapshot, then her three changes.
			if want := []uint64{1, 2, 3, 4}; !slices.Equal(ids, want) {
				t.Errorf("alice's event ids = %v, want %v", ids, want)
			}
		})
	})
	t.Run("a stray list item, other topics and other streams", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			e := newEnv(t, Options{}, "alice", "bob")
			e.policy.set("alice", allowNamespaces("team-a"))
			e.policy.set("bob", func(authorizationv1.ResourceAttributes) (bool, error) { return true, nil })
			all := mustTopic(t, "instances")
			teamA := mustTopic(t, "instances:team-a")
			other := mustTopic(t, "instance:team-b/two")
			e.prod.upsert(t, all, listItem("team-b", "two", 1))

			a := e.open(session("alice"), "instances:team-a")
			e.open(session("bob"), "instances", "instance:team-b/two")
			e.open(session("bob"), "platform")
			e.prod.upsert(t, all, listItem("team-b", "two", 2))
			e.prod.upsert(t, other, instItem("team-b", "two", 2))
			e.prod.upsert(t, teamA, listItem("team-a", "one", 1))
			e.prod.upsert(t, teamA, listItem("team-b", "two", 3)) // a producer fault, left out
			e.prod.upsert(t, all, listItem("team-b", "two", 3))
			e.prod.upsert(t, teamA, listItem("team-a", "one", 2))
			synctest.Wait()

			var ids []uint64
			for _, ev := range a.rec.take() {
				if ev.ID != "" {
					ids = append(ids, seqOf(t, ev.ID))
				}
			}
			if want := []uint64{1, 2, 3}; !slices.Equal(ids, want) {
				t.Errorf("alice's event ids = %v, want %v", ids, want)
			}
		})
	})
}

func TestStreamsChangeOnlyForTheIdentityThatOpenedThem(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice", "mallory")
		e.policy.set("alice", allowNamespaces("apps"))
		e.policy.set("mallory", allowNamespaces("apps"))
		sv := e.open(session("alice"), "instance:apps/blog")
		web := mustTopic(t, "instance:apps/web")
		// The same session key now names another principal.
		relogin := Session{Key: session("alice").Key, Identity: user("mallory")}
		if err := e.b.Subscribe(context.Background(), relogin, sv.ID(), web); !errors.Is(err, ErrNoStream) {
			t.Errorf("Subscribe under another identity = %v", err)
		}
		if err := e.b.Unsubscribe(relogin, sv.ID(), mustTopic(t, "instance:apps/blog")); !errors.Is(err, ErrNoStream) {
			t.Errorf("Unsubscribe under another identity = %v", err)
		}
		if n := e.policy.count(); n != 1 {
			t.Errorf("%d reviews sent, want 1 (alice's own topic)", n)
		}
	})
}

func TestTheCapIsCheckedBeforeTheProducerIsAsked(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{MaxTopicsPerStream: 2}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		three := []Topic{mustTopic(t, "instance:apps/a"), mustTopic(t, "instance:apps/b"), mustTopic(t, "instance:apps/c")}
		if _, err := e.b.Open(context.Background(), session("alice"), three, ""); !errors.Is(err, ErrTooManyTopics) {
			t.Fatalf("three topics: %v", err)
		}
		if e.prod.attributeCalls != 0 {
			t.Errorf("Attributes called %d times for a refused request", e.prod.attributeCalls)
		}
	})
}

// A reader allowed single instances by name, and not the list of their
// namespace, is refused the list topic as a GET list refuses it: names are
// not asked one by one (0030:D7:R2).
func TestAListTopicIsRefusedWithoutTheListGrant(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		e.policy.set("alice", func(a authorizationv1.ResourceAttributes) (bool, error) {
			return a.Namespace == "team-b" && a.Name == "two", nil
		})
		ns := mustTopic(t, "instances:team-b")
		e.prod.upsert(t, ns, listItem("team-b", "one", 1))
		e.prod.upsert(t, ns, listItem("team-b", "two", 1))
		sv := e.open(session("alice"), "instances:team-b")
		e.prod.upsert(t, ns, listItem("team-b", "two", 2))
		synctest.Wait()
		evs := sv.rec.take()
		if got := eventNames(evs); !slices.Equal(got, []string{"open()", "closed(instances:team-b)"}) || evs[1].code() != CodeForbidden {
			t.Errorf("events = %v %+v", got, evs)
		}
		if n := e.policy.count(); n != 1 {
			t.Errorf("%d reviews sent, want 1 (the topic's list read)", n)
		}
		if act, _ := e.prod.counts(ns); act != 0 {
			t.Errorf("the refused topic was activated %d times", act)
		}
	})
}

// An identity the authorizer does not serve is refused a list topic when it
// asks for it, as it is refused a GET list.
func TestAListTopicClosesForAnIdentityTheAuthorizerDoesNotServe(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		all := mustTopic(t, "instances")
		e.prod.upsert(t, all, listItem("apps", "blog", 1))
		evs := e.open(session("carol"), "instances").rec.take()
		if got := eventNames(evs); !slices.Equal(got, []string{"open()", "closed(instances)"}) || evs[1].code() != CodeUnauthenticated {
			t.Errorf("events = %v %+v", got, evs)
		}
	})
}

// closeTopic for a subscription the stream no longer holds (the topic was
// removed and added again meanwhile) writes nothing and keeps the new one.
func TestClosingAReplacedSubscriptionChangesNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		blog := mustTopic(t, "instance:apps/blog")
		s, err := e.b.Open(context.Background(), session("alice"), []Topic{blog}, "")
		if err != nil {
			t.Fatal(err)
		}
		e.b.mu.Lock()
		stale := s.st.subs[blog]
		fresh := &subscription{topic: blog, attrs: stale.attrs, grants: stale.grants, snapshotSeq: stale.snapshotSeq}
		s.st.subs[blog] = fresh
		e.b.topics[blog].subs[s.st] = fresh
		e.b.mu.Unlock()

		rec := newRecorder()
		wr := &writer{w: rec, rc: http.NewResponseController(rec), timeout: time.Second}
		if err := s.closeTopic(context.Background(), wr, stale, CodeForbidden); err != nil {
			t.Fatal(err)
		}
		if got := rec.take(); len(got) != 0 {
			t.Errorf("wrote %v for a replaced subscription", eventNames(got))
		}
		e.b.mu.Lock()
		defer e.b.mu.Unlock()
		if s.st.subs[blog] != fresh || len(s.st.pending) != 0 {
			t.Errorf("the replacing subscription was dropped or a closing kept: subs=%v pending=%v", s.st.subs, s.st.pending)
		}
		if _, rel := e.prod.counts(blog); rel != 0 {
			t.Errorf("the topic was released %d times", rel)
		}
	})
}

// podItem is an upsert of pod ns/name at version v on an object topic: the
// read it reveals is that pod's, which the topic's grant does not cover.
func podItem(ns, name string, v int) Item {
	return Item{
		Event: EventUpsert,
		Attrs: authz.Attributes{Verb: "get", Resource: podsGVR, Namespace: ns, Name: name},
		Data:  json.RawMessage(fmt.Sprintf(`{"name": %q, "namespace": %q, "v": %d}`, name, ns, v)),
	}
}

// slowListItems seeds n list items of team-a on topic, each rendered by
// render.
func slowListItems(e *env, topic Topic, n int, render func(ctx context.Context, i int) (json.RawMessage, error)) {
	for i := range n {
		e.prod.seed(topic, fmt.Sprintf("team-a/i%d", i), Item{
			Event:  EventUpsert,
			Attrs:  authz.Attributes{Verb: "list", Resource: instancesGVR, Namespace: "team-a"},
			Render: func(ctx context.Context, _ authz.Identity) (json.RawMessage, error) { return render(ctx, i) },
		})
	}
}

func listOnly(ns string) rule {
	return func(a authorizationv1.ResourceAttributes) (bool, error) {
		return a.Verb == "list" && a.Namespace == ns, nil
	}
}

func denyAll(authorizationv1.ResourceAttributes) (bool, error) { return false, nil }

// nonHeartbeats lists evs as "event(topic)code", leaving heartbeats out.
func nonHeartbeats(evs []sse) []string {
	var out []string
	for _, ev := range evs {
		if ev.Event != EventHeartbeat {
			out = append(out, ev.Event+"("+ev.topic()+")"+ev.code())
		}
	}
	return out
}

// A list grant that expires while the snapshot is taken is asked again: the
// snapshot carries every item the reader may list, or the topic closes with
// the denial; it never arrives cut short.
func TestAGrantExpiringDuringASlowSnapshotIsAskedAgain(t *testing.T) {
	for _, revoked := range []bool{false, true} {
		t.Run(fmt.Sprintf("revoked=%v", revoked), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e := newEnv(t, Options{HeartbeatInterval: time.Hour}, "alice")
				e.policy.set("alice", listOnly("team-a"))
				teamA := mustTopic(t, "instances:team-a")
				e.prod.seed(teamA, "a", listItem("team-a", "one", 1))
				e.prod.seed(teamA, "b", listItem("team-a", "two", 1))
				e.prod.snapshotWait = func(context.Context) {
					if revoked {
						e.policy.set("alice", denyAll)
					}
					// Past the 30 s the topic's grant lives.
					time.Sleep(31 * time.Second)
				}
				sv := e.open(session("alice"), "instances:team-a")
				time.Sleep(time.Minute)
				synctest.Wait()

				evs := sv.rec.take()
				if revoked {
					if got, want := nonHeartbeats(evs), []string{"open()", "closed(instances:team-a)forbidden"}; !slices.Equal(got, want) {
						t.Errorf("events = %v, want %v", got, want)
					}
					return
				}
				items, closed := delivered(t, evs)
				if want := []string{"instances:team-a team-a/one", "instances:team-a team-a/two"}; !slices.Equal(items, want) || len(closed) != 0 {
					t.Errorf("items = %v, closings = %v; want %v and none", items, closed, want)
				}
			})
		})
	}
}

// A list grant that expires between the renders of one snapshot is asked
// again before the next item, as above.
func TestAGrantExpiringDuringSlowRendersIsAskedAgain(t *testing.T) {
	for _, revoked := range []bool{false, true} {
		t.Run(fmt.Sprintf("revoked=%v", revoked), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e := newEnv(t, Options{HeartbeatInterval: time.Hour}, "alice")
				e.policy.set("alice", listOnly("team-a"))
				teamA := mustTopic(t, "instances:team-a")
				var renders int
				slowListItems(e, teamA, 3, func(_ context.Context, i int) (json.RawMessage, error) {
					// Three renders of 16 s cross the 30 s the grant lives.
					time.Sleep(16 * time.Second)
					renders++
					if revoked && renders == 1 {
						e.policy.set("alice", denyAll)
					}
					return json.RawMessage(fmt.Sprintf(`{"name": "i%d", "namespace": "team-a", "v": 1}`, i)), nil
				})
				sv := e.open(session("alice"), "instances:team-a")
				time.Sleep(time.Minute)
				synctest.Wait()

				evs := sv.rec.take()
				if revoked {
					if got, want := nonHeartbeats(evs), []string{"open()", "closed(instances:team-a)forbidden"}; !slices.Equal(got, want) {
						t.Errorf("events = %v, want %v", got, want)
					}
					if renders != 2 {
						t.Errorf("%d renders, want 2 (none after the denial)", renders)
					}
					return
				}
				items, closed := delivered(t, evs)
				if want := []string{"instances:team-a team-a/i0", "instances:team-a team-a/i1", "instances:team-a team-a/i2"}; !slices.Equal(items, want) || len(closed) != 0 {
					t.Errorf("items = %v, closings = %v; want %v and none", items, closed, want)
				}
			})
		})
	}
}

// Nothing is written under a grant that expired while it was being made: a
// snapshot or render that outlives the topic's grant is gated again right
// before the write, and a revoked grant closes the topic.
func TestNothingIsWrittenUnderAGrantThatExpiredBeforeTheWrite(t *testing.T) {
	render := func(e *env, d time.Duration, revoke bool) func(ctx context.Context, _ authz.Identity) (json.RawMessage, error) {
		return func(context.Context, authz.Identity) (json.RawMessage, error) {
			if revoke {
				e.policy.set("alice", denyAll)
			}
			time.Sleep(d)
			return json.RawMessage(`{"name": "one", "namespace": "team-a", "v": 1}`), nil
		}
	}
	listed := func(r func(context.Context, authz.Identity) (json.RawMessage, error)) Item {
		return Item{Event: EventUpsert, Attrs: authz.Attributes{Verb: "list", Resource: instancesGVR, Namespace: "team-a"}, Render: r}
	}
	cases := []struct {
		name  string
		setup func(t *testing.T, e *env, teamA Topic)
		live  func(t *testing.T, e *env, teamA Topic)
	}{
		{"an empty snapshot", func(_ *testing.T, e *env, _ Topic) {
			e.prod.snapshotWait = func(context.Context) {
				e.policy.set("alice", denyAll)
				// Past the 30 s the topic's grant lives, with no item to
				// gate the topic again.
				time.Sleep(31 * time.Second)
			}
		}, nil},
		{"the last render of a snapshot", func(_ *testing.T, e *env, teamA Topic) {
			// The second render starts within the grant's 30 s and ends
			// past them.
			e.prod.seed(teamA, "a", listed(render(e, 20*time.Second, false)))
			e.prod.seed(teamA, "b", listed(render(e, 20*time.Second, true)))
		}, nil},
		{"the render of a live item", nil, func(t *testing.T, e *env, teamA Topic) {
			time.Sleep(25 * time.Second)
			e.prod.upsert(t, teamA, listed(render(e, 10*time.Second, true)))
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e := newEnv(t, Options{HeartbeatInterval: time.Hour}, "alice")
				e.policy.set("alice", listOnly("team-a"))
				teamA := mustTopic(t, "instances:team-a")
				if c.setup != nil {
					c.setup(t, e, teamA)
				}
				sv := e.open(session("alice"), "instances:team-a")
				synctest.Wait()
				if c.live != nil {
					c.live(t, e, teamA)
				}
				time.Sleep(time.Minute)
				synctest.Wait()

				evs := nonHeartbeats(sv.rec.take())
				want := []string{"open()", "closed(instances:team-a)forbidden"}
				if c.live != nil {
					want = []string{"open()", "snapshot(instances:team-a)", "closed(instances:team-a)forbidden"}
				}
				if !slices.Equal(evs, want) {
					t.Errorf("events = %v, want %v", evs, want)
				}
				if n := e.policy.count(); n != 2 {
					t.Errorf("%d reviews sent, want 2 (the expired grant asked again)", n)
				}
			})
		})
	}
}

// A list topic is served only under the one list read a GET list needs; a
// producer that names anything else does not serve it, and no review is
// sent.
func TestAListTopicNeedsExactlyItsListRead(t *testing.T) {
	list := func(ns string) authz.Attributes {
		return authz.Attributes{Verb: "list", Resource: instancesGVR, Namespace: ns}
	}
	with := func(a authz.Attributes, f func(*authz.Attributes)) authz.Attributes {
		f(&a)
		return a
	}
	cases := []struct {
		name  string
		topic string
		reads []authz.Attributes
		ok    bool
	}{
		{"cluster-wide list", "instances", []authz.Attributes{list("")}, true},
		{"namespace list", "instances:team-a", []authz.Attributes{list("team-a")}, true},
		{"namespace list for the cluster-wide topic", "instances", []authz.Attributes{list("team-a")}, false},
		{"cluster-wide list for a namespace topic", "instances:team-a", []authz.Attributes{list("")}, false},
		{"another namespace", "instances:team-a", []authz.Attributes{list("team-b")}, false},
		{"get, not list", "instances:team-a", []authz.Attributes{with(list("team-a"), func(a *authz.Attributes) { a.Verb = "get" })}, false},
		{"watch, not list", "instances", []authz.Attributes{with(list(""), func(a *authz.Attributes) { a.Verb = "watch" })}, false},
		{"a subresource", "instances:team-a", []authz.Attributes{with(list("team-a"), func(a *authz.Attributes) { a.Subresource = "status" })}, false},
		{"a name", "instances:team-a", []authz.Attributes{with(list("team-a"), func(a *authz.Attributes) { a.Name = "one" })}, false},
		{"two reads", "instances:team-a", []authz.Attributes{list("team-a"), list("team-a")}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e := newEnv(t, Options{}, "alice")
				e.policy.set("alice", func(authorizationv1.ResourceAttributes) (bool, error) { return true, nil })
				e.prod.attrsFor = func(Topic) ([]authz.Attributes, bool) { return c.reads, true }
				_, err := e.b.Open(context.Background(), session("alice"), []Topic{mustTopic(t, c.topic)}, "")
				if c.ok && err != nil {
					t.Errorf("Open = %v, want it served", err)
				}
				if !c.ok {
					if !errors.Is(err, ErrTopicNotServed) {
						t.Errorf("Open = %v, want ErrTopicNotServed", err)
					}
					if n := e.policy.count(); n != 0 {
						t.Errorf("%d reviews sent for a topic not served", n)
					}
				}
			})
		})
	}
}
