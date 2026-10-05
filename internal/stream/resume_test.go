package stream

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

// openBlog opens alice's stream on instance:apps/blog at version 1 and
// returns it with the events taken.
func openBlog(t *testing.T, e *env) (*served, Topic) {
	t.Helper()
	e.policy.set("alice", allowNamespaces("apps"))
	blog := mustTopic(t, "instance:apps/blog")
	e.prod.upsert(t, blog, instItem("apps", "blog", 1))
	sv := e.open(session("alice"), "instance:apps/blog")
	sv.rec.take()
	return sv, blog
}

func versionsOf(t *testing.T, evs []sse, event string) []int {
	t.Helper()
	var out []int
	for _, ev := range evs {
		if ev.Event == event {
			out = append(out, ev.versions(t)...)
		}
	}
	return out
}

func TestResumeWithinTheBuffer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		sv, blog := openBlog(t, e)
		last := sv.rec.lastID()
		if err := sv.disconnect(); !errors.Is(err, context.Canceled) {
			t.Fatalf("Serve after disconnect = %v", err)
		}
		for v := 2; v <= 4; v++ {
			e.prod.upsert(t, blog, instItem("apps", "blog", v))
		}
		again := e.resume(session("alice"), last, "instance:apps/blog")
		evs := again.rec.take()
		if again.ID() != sv.ID() {
			t.Errorf("resumed stream id %s, want %s", again.ID(), sv.ID())
		}
		if got := eventNames(evs); !slices.Equal(got, []string{"open()", "upsert(instance:apps/blog)", "upsert(instance:apps/blog)", "upsert(instance:apps/blog)"}) {
			t.Fatalf("events = %v", got)
		}
		if got := versionsOf(t, evs, EventUpsert); !slices.Equal(got, []int{2, 3, 4}) {
			t.Errorf("replayed versions %v", got)
		}
		if seqOf(t, evs[1].ID) <= seqOf(t, last) {
			t.Errorf("replay id %s is not after %s", evs[1].ID, last)
		}
		// The topic keeps streaming live.
		e.prod.upsert(t, blog, instItem("apps", "blog", 5))
		synctest.Wait()
		if got := versionsOf(t, again.rec.take(), EventUpsert); !slices.Equal(got, []int{5}) {
			t.Errorf("live after resume %v", got)
		}
	})
}

func TestResumePastTheBuffer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{RingSize: 2}, "alice")
		sv, blog := openBlog(t, e)
		last := sv.rec.lastID()
		_ = sv.disconnect()
		for v := 2; v <= 4; v++ {
			e.prod.upsert(t, blog, instItem("apps", "blog", v))
		}
		evs := e.resume(session("alice"), last, "instance:apps/blog").rec.take()
		if got := eventNames(evs); !slices.Equal(got, []string{"open()", "snapshot(instance:apps/blog)"}) {
			t.Fatalf("events = %v", got)
		}
		if got := versionsOf(t, evs, EventSnapshot); !slices.Equal(got, []int{4}) {
			t.Errorf("snapshot versions %v", got)
		}
	})
}

func TestResumeBeforeATopicsSnapshotWasDelivered(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		sv, blog := openBlog(t, e)
		last := sv.rec.lastID()
		_ = sv.disconnect()
		// A topic added while the stream was away has no snapshot yet.
		web := mustTopic(t, "instance:apps/web")
		e.prod.upsert(t, web, instItem("apps", "web", 7))
		if err := e.b.Subscribe(context.Background(), session("alice"), sv.ID(), web); err != nil {
			t.Fatal(err)
		}
		e.prod.upsert(t, blog, instItem("apps", "blog", 2))
		evs := e.resume(session("alice"), last, "instance:apps/blog").rec.take()
		if got := eventNames(evs); !slices.Equal(got, []string{"open()", "upsert(instance:apps/blog)", "snapshot(instance:apps/web)"}) {
			t.Fatalf("events = %v", got)
		}
		if got := versionsOf(t, evs, EventSnapshot); !slices.Equal(got, []int{7}) {
			t.Errorf("web snapshot %v", got)
		}
	})
}

func TestResumeIsReauthorized(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		sv, blog := openBlog(t, e)
		last := sv.rec.lastID()
		_ = sv.disconnect()
		e.policy.set("alice", allowNamespaces())
		time.Sleep(30 * time.Second) // the cached allow expires
		e.prod.upsert(t, blog, instItem("apps", "blog", 2))
		evs := e.resume(session("alice"), last, "instance:apps/blog").rec.take()
		if got := eventNames(evs); !slices.Equal(got, []string{"open()", "closed(instance:apps/blog)"}) || evs[1].code() != CodeForbidden {
			t.Fatalf("events = %v", got)
		}
		if _, rel := e.prod.counts(blog); rel != 1 {
			t.Errorf("released %d times", rel)
		}
	})
}

func TestResumeThatOpensAFreshStream(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, e *env, id string) (sess Session, lastID string)
	}{
		{"another session", func(t *testing.T, e *env, id string) (Session, string) {
			e.policy.set("bob", allowNamespaces("apps"))
			return session("bob"), id
		}},
		{"same key, another identity", func(t *testing.T, e *env, id string) (Session, string) {
			e.policy.set("bob", allowNamespaces("apps"))
			return Session{Key: session("alice").Key, Identity: user("bob")}, id
		}},
		{"another epoch", func(t *testing.T, e *env, id string) (Session, string) {
			parts := strings.SplitN(id, ".", 2)
			return session("alice"), "XXXXXXXX." + parts[1]
		}},
		{"after the resume window", func(t *testing.T, e *env, id string) (Session, string) {
			time.Sleep(61 * time.Second)
			return session("alice"), id
		}},
		{"a malformed id", func(t *testing.T, e *env, id string) (Session, string) {
			return session("alice"), id + ".x"
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e := newEnv(t, Options{HeartbeatInterval: time.Hour}, "alice", "bob")
				sv, blog := openBlog(t, e)
				web := mustTopic(t, "instance:apps/web")
				if err := e.b.Subscribe(context.Background(), session("alice"), sv.ID(), web); err != nil {
					t.Fatal(err)
				}
				synctest.Wait()
				last := sv.rec.lastID()
				_ = sv.disconnect()
				e.prod.upsert(t, blog, instItem("apps", "blog", 2))
				sess, id := tc.setup(t, e, last)
				again := e.resume(sess, id, "instance:apps/blog")
				evs := again.rec.take()
				if again.ID() == sv.ID() {
					t.Error("the old stream was resumed")
				}
				if got := eventNames(evs); !slices.Equal(got, []string{"open()", "snapshot(instance:apps/blog)"}) {
					t.Errorf("events = %v (want only the URL's topic, from a snapshot)", got)
				}
			})
		})
	}
}

func TestAReconnectTakesOverAnAttachedStream(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		sv, blog := openBlog(t, e)
		last := sv.rec.lastID()
		e.prod.upsert(t, blog, instItem("apps", "blog", 2))
		synctest.Wait()
		// The old connection is still attached when the reconnect arrives.
		again := e.resume(session("alice"), last, "instance:apps/blog")
		if err := sv.ended(); !errors.Is(err, ErrReplaced) {
			t.Errorf("old Serve = %v, want ErrReplaced", err)
		}
		if got := versionsOf(t, again.rec.take(), EventUpsert); !slices.Equal(got, []int{2}) {
			t.Errorf("replayed %v", got)
		}
		e.prod.upsert(t, blog, instItem("apps", "blog", 3))
		synctest.Wait()
		if got := versionsOf(t, sv.rec.take(), EventUpsert); !slices.Equal(got, []int{2}) {
			t.Errorf("old connection saw %v after the takeover", got)
		}
		if got := versionsOf(t, again.rec.take(), EventUpsert); !slices.Equal(got, []int{3}) {
			t.Errorf("new connection saw %v", got)
		}
	})
}

func TestHeartbeatsOnAQuietStream(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		sv, _ := openBlog(t, e)
		time.Sleep(45 * time.Second)
		synctest.Wait()
		evs := sv.rec.take()
		if got := eventNames(evs); !slices.Equal(got, []string{"heartbeat()", "heartbeat()", "heartbeat()"}) {
			t.Fatalf("events = %v", got)
		}
		if evs[0].ID != "" || evs[0].Data != "{}" {
			t.Errorf("heartbeat %+v", evs[0])
		}
	})
}

func TestAnIdleStreamIsClosed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		sv, blog := openBlog(t, e)
		if err := e.b.Unsubscribe(session("alice"), sv.ID(), blog); err != nil {
			t.Fatal(err)
		}
		time.Sleep(29 * time.Minute)
		synctest.Wait()
		if err := sv.ended(); err != nil {
			t.Fatalf("closed early: %v", err)
		}
		time.Sleep(2 * time.Minute)
		synctest.Wait()
		if err := sv.ended(); !errors.Is(err, ErrIdle) {
			t.Fatalf("Serve = %v, want ErrIdle", err)
		}
		if err := e.b.Subscribe(context.Background(), session("alice"), sv.ID(), blog); !errors.Is(err, ErrNoStream) {
			t.Errorf("an idle stream is still registered: %v", err)
		}
	})
}

// A stream ends when its session does: its last message is the expired
// event, and it is discarded, so a reconnect with its last event id opens a
// fresh stream with a fresh snapshot.
func TestAStreamEndsWhenItsSessionExpires(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		blog := mustTopic(t, "instance:apps/blog")
		e.prod.seed(blog, "blog", instItem("apps", "blog", 1))
		sess := session("alice")
		sess.Expires = time.Now().Add(time.Minute)
		sv := e.open(sess, "instance:apps/blog")
		last := sv.rec.lastID()

		time.Sleep(59 * time.Second)
		synctest.Wait()
		if err := sv.ended(); err != nil {
			t.Fatalf("ended before the session: %v", err)
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if err := sv.ended(); !errors.Is(err, ErrSessionExpired) {
			t.Fatalf("Serve = %v, want ErrSessionExpired", err)
		}
		evs := sv.rec.take()
		if got, want := nonHeartbeats(evs), []string{"open()", "snapshot(instance:apps/blog)", "expired()unauthenticated"}; !slices.Equal(got, want) {
			t.Fatalf("events = %v, want %v", got, want)
		}
		if got := evs[len(evs)-1]; got.ID != "" || got.Data != `{"code":"unauthenticated"}` {
			t.Errorf("expired event = %+v", got)
		}
		// Nothing published after the end reaches anyone.
		e.prod.upsert(t, blog, instItem("apps", "blog", 2))
		synctest.Wait()
		if evs := sv.rec.take(); len(evs) != 0 {
			t.Errorf("events after the end: %v", eventNames(evs))
		}
		if err := e.b.Subscribe(context.Background(), sess, sv.ID(), blog); !errors.Is(err, ErrNoStream) {
			t.Errorf("an expired stream is still registered: %v", err)
		}
		// A renewed session cannot resume it either.
		renewed := session("alice")
		renewed.Expires = time.Now().Add(time.Hour)
		again := e.resume(renewed, last, "instance:apps/blog")
		if again.ID() == sv.ID() {
			t.Error("a reconnect resumed the expired stream")
		}
	})
}

// A session that has already expired opens no stream, and no review is
// sent for it.
func TestAnExpiredSessionOpensNoStream(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		sess := session("alice")
		sess.Expires = time.Now()
		if _, err := e.b.Open(context.Background(), sess, []Topic{mustTopic(t, "instance:apps/blog")}, ""); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("Open = %v, want ErrUnauthenticated", err)
		}
		if n := e.policy.count(); n != 0 {
			t.Errorf("%d reviews sent for an expired session", n)
		}
	})
}

func TestASlowConsumerIsEvicted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{QueueSize: 2}, "alice", "bob")
		slow, blog := openBlog(t, e)
		e.policy.set("bob", allowNamespaces("apps"))
		fast := e.open(session("bob"), "instance:apps/blog")
		fast.rec.take()

		slow.rec.block()
		for v := 2; v <= 6; v++ {
			e.prod.upsert(t, blog, instItem("apps", "blog", v))
			synctest.Wait()
		}
		if got := versionsOf(t, fast.rec.take(), EventUpsert); !slices.Equal(got, []int{2, 3, 4, 5, 6}) {
			t.Errorf("the fast subscriber saw %v", got)
		}
		slow.rec.unblock()
		synctest.Wait()
		if err := slow.ended(); !errors.Is(err, ErrSlowConsumer) {
			t.Fatalf("slow Serve = %v, want ErrSlowConsumer", err)
		}
		seen := versionsOf(t, slow.rec.take(), EventUpsert)
		again := e.resume(session("alice"), slow.rec.lastID(), "instance:apps/blog")
		seen = append(seen, versionsOf(t, again.rec.take(), EventUpsert)...)
		if !slices.Equal(seen, []int{2, 3, 4, 5, 6}) {
			t.Errorf("before and after the eviction the slow client saw %v", seen)
		}
	})
}

func TestStreamCaps(t *testing.T) {
	t.Run("a third tab is refused", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			e := newEnv(t, Options{}, "alice")
			one, blog := openBlog(t, e)
			two := e.open(session("alice"), "instance:apps/blog")
			two.rec.take()
			reviews := e.policy.count()
			_, err := e.b.Open(context.Background(), session("alice"), []Topic{blog}, "")
			if !errors.Is(err, ErrTooManyStreams) {
				t.Fatalf("third Open = %v", err)
			}
			if e.policy.count() != reviews {
				t.Error("a refused stream sent reviews")
			}
			e.prod.upsert(t, blog, instItem("apps", "blog", 2))
			synctest.Wait()
			for _, sv := range []*served{one, two} {
				if got := versionsOf(t, sv.rec.take(), EventUpsert); !slices.Equal(got, []int{2}) {
					t.Errorf("an open stream saw %v", got)
				}
			}
		})
	})
	t.Run("a reloaded tab is not refused", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			e := newEnv(t, Options{}, "alice")
			one, blog := openBlog(t, e)
			e.open(session("alice"), "instance:apps/blog")
			_ = one.disconnect()
			e.open(session("alice"), "instance:apps/blog")
			if err := e.b.Subscribe(context.Background(), session("alice"), one.ID(), blog); !errors.Is(err, ErrNoStream) {
				t.Errorf("the disconnected stream was kept: %v", err)
			}
		})
	})
	t.Run("the process cap", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			e := newEnv(t, Options{MaxStreams: 2}, "alice", "bob", "carol")
			for _, u := range []string{"alice", "bob", "carol"} {
				e.policy.set(u, allowNamespaces("apps"))
			}
			blog := mustTopic(t, "instance:apps/blog")
			a := e.open(session("alice"), "instance:apps/blog")
			e.open(session("bob"), "instance:apps/blog")
			if _, err := e.b.Open(context.Background(), session("carol"), []Topic{blog}, ""); !errors.Is(err, ErrTooManyStreams) {
				t.Fatalf("third stream in the process = %v", err)
			}
			_ = a.disconnect()
			e.open(session("carol"), "instance:apps/blog")
		})
	})
}

func TestClosedBroker(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		sv, blog := openBlog(t, e)
		e.b.Close()
		synctest.Wait()
		if err := sv.ended(); !errors.Is(err, ErrClosed) {
			t.Errorf("Serve = %v, want ErrClosed", err)
		}
		if _, err := e.b.Open(context.Background(), session("alice"), []Topic{blog}, ""); !errors.Is(err, ErrClosed) {
			t.Errorf("Open after Close = %v", err)
		}
		if err := e.b.Publish(blog, instItem("apps", "blog", 2)); err != nil {
			t.Errorf("Publish after Close = %v", err)
		}
	})
}

func TestATopicDeniedWhileDetachedIsClosedOnReconnect(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		sv, _ := openBlog(t, e)
		last := sv.rec.lastID()
		_ = sv.disconnect()
		secret := mustTopic(t, "instance:secret/db")
		if err := e.b.Subscribe(context.Background(), session("alice"), sv.ID(), secret, secret); err != nil {
			t.Fatal(err)
		}
		if err := e.b.Subscribe(context.Background(), session("alice"), sv.ID(), secret); err != nil {
			t.Fatal(err)
		}
		evs := e.resume(session("alice"), last, "instance:apps/blog").rec.take()
		if got := eventNames(evs); !slices.Equal(got, []string{"open()", "closed(instance:secret/db)"}) || evs[1].code() != CodeForbidden {
			t.Fatalf("events = %v", got)
		}
	})
}

func TestAnOldEventIDResumesWithSnapshots(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{QueueSize: 2}, "alice")
		sv, blog := openBlog(t, e)
		first := sv.rec.lastID()
		for v := 2; v <= 4; v++ {
			e.prod.upsert(t, blog, instItem("apps", "blog", v))
			synctest.Wait()
		}
		_ = sv.disconnect()
		// The stream no longer remembers where id 1 stood in the ring.
		evs := e.resume(session("alice"), first, "instance:apps/blog").rec.take()
		if got := eventNames(evs); !slices.Equal(got, []string{"open()", "snapshot(instance:apps/blog)"}) {
			t.Fatalf("events = %v", got)
		}
		if got := seqOf(t, evs[1].ID); got != 5 {
			t.Errorf("event id after resume = %d, want 5 (ids continue along the stream)", got)
		}
	})
}

func TestAResumeIgnoresTheURLTopics(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{MaxTopicsPerStream: 1}, "alice")
		sv, _ := openBlog(t, e)
		last := sv.rec.lastID()
		_ = sv.disconnect()
		ts := []Topic{mustTopic(t, "log:apps/blog-0/server"), mustTopic(t, "instance:apps/a")}
		again, err := e.b.Open(context.Background(), session("alice"), ts, last)
		if err != nil || again.ID() != sv.ID() {
			t.Fatalf("resume = %v, %v", again, err)
		}
	})
}

func TestEndingAConnectionCancelsItsWork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		e.prod.snapshotWait = func(ctx context.Context) { <-ctx.Done() }
		sv := e.open(session("alice"), "instance:apps/blog")
		e.b.Close()
		synctest.Wait()
		if err := sv.ended(); !errors.Is(err, ErrClosed) {
			t.Fatalf("Serve after Close = %v, want ErrClosed", err)
		}
	})
}

// A client that goes away during a snapshot keeps its topic for the resume.
func TestADisconnectDuringASnapshotKeepsTheTopic(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{}, "alice")
		e.policy.set("alice", allowNamespaces("apps"))
		blog := mustTopic(t, "instance:apps/blog")
		e.prod.upsert(t, blog, instItem("apps", "blog", 1))
		e.prod.snapshotWait = func(ctx context.Context) { <-ctx.Done() }
		s, err := e.b.Open(context.Background(), session("alice"), []Topic{blog}, "")
		if err != nil {
			t.Fatal(err)
		}
		sv := e.serve(s)
		if err := sv.disconnect(); !errors.Is(err, context.Canceled) {
			t.Fatalf("Serve after disconnect = %v", err)
		}
		e.prod.mu.Lock()
		e.prod.snapshotWait = nil
		e.prod.mu.Unlock()
		// The client got no event id, so it names the stream with one the
		// stream never issued, which resumes it with fresh snapshots.
		again := e.resume(session("alice"), e.b.epoch+"."+sv.ID()+".0", "instance:apps/blog")
		if got := eventNames(again.rec.take()); !slices.Equal(got, []string{"open()", "snapshot(instance:apps/blog)"}) {
			t.Fatalf("events = %v", got)
		}
		if _, rel := e.prod.counts(blog); rel != 0 {
			t.Errorf("topic released %d times", rel)
		}
	})
}

// A denial queued on a connection that then ends is not lost: it stays
// pending until a connection writes it, so the next one sends it first.
func TestADenialQueuedOnAnEndingConnectionReachesTheNext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{QueueSize: 2}, "alice")
		sv, blog := openBlog(t, e)
		sv.rec.block()
		e.prod.upsert(t, blog, instItem("apps", "blog", 2))
		synctest.Wait()
		e.prod.upsert(t, blog, instItem("apps", "blog", 3))
		// The writer is stuck on version 2; version 3 and the closing fill
		// the queue, and the next change evicts the connection.
		if err := e.b.Subscribe(context.Background(), session("alice"), sv.ID(), mustTopic(t, "instance:secret/db")); err != nil {
			t.Fatal(err)
		}
		e.prod.upsert(t, blog, instItem("apps", "blog", 4))
		sv.rec.unblock()
		synctest.Wait()
		if err := sv.ended(); !errors.Is(err, ErrSlowConsumer) {
			t.Fatalf("Serve = %v, want ErrSlowConsumer", err)
		}
		for _, ev := range sv.rec.take() {
			if ev.Event == EventClosed {
				t.Fatalf("the evicted connection wrote %s", ev.Data)
			}
		}
		evs := e.resume(session("alice"), sv.rec.lastID(), "instance:apps/blog").rec.take()
		if len(evs) < 2 || evs[1].Event != EventClosed || evs[1].topic() != "instance:secret/db" || evs[1].code() != CodeForbidden {
			t.Fatalf("events after the resume = %v, want the closing first", eventNames(evs))
		}
		if got := versionsOf(t, evs, EventUpsert); !slices.Equal(got, []int{3, 4}) {
			t.Errorf("replayed %v, want [3 4]", got)
		}
	})
}

// Asking again for a topic whose closing is still pending counts it once
// against the cap, and an allow replaces the closing.
func TestARetriedDenialCountsOnceAgainstTheCap(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{MaxTopicsPerStream: 2}, "alice")
		sv, _ := openBlog(t, e)
		last := sv.rec.lastID()
		_ = sv.disconnect()
		secret := mustTopic(t, "instance:secret/db")
		if err := e.b.Subscribe(context.Background(), session("alice"), sv.ID(), secret); err != nil {
			t.Fatal(err)
		}
		e.policy.set("alice", allowNamespaces("apps", "secret"))
		time.Sleep(31 * time.Second) // the cached denial expires
		if err := e.b.Subscribe(context.Background(), session("alice"), sv.ID(), secret); err != nil {
			t.Fatalf("subscribing again to a pending denied topic: %v", err)
		}
		evs := e.resume(session("alice"), last, "instance:apps/blog").rec.take()
		if got := eventNames(evs); !slices.Equal(got, []string{"open()", "snapshot(instance:secret/db)"}) {
			t.Errorf("events = %v", got)
		}
	})
}

// Removing a topic forgets its pending closing and frees its place.
func TestUnsubscribeForgetsAPendingClosing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(t, Options{MaxTopicsPerStream: 2}, "alice")
		sv, _ := openBlog(t, e)
		last := sv.rec.lastID()
		_ = sv.disconnect()
		secret := mustTopic(t, "instance:secret/db")
		if err := e.b.Subscribe(context.Background(), session("alice"), sv.ID(), secret); err != nil {
			t.Fatal(err)
		}
		if err := e.b.Unsubscribe(session("alice"), sv.ID(), secret); err != nil {
			t.Fatal(err)
		}
		if err := e.b.Subscribe(context.Background(), session("alice"), sv.ID(), mustTopic(t, "instance:apps/web")); err != nil {
			t.Fatalf("the removed closing still counts against the cap: %v", err)
		}
		evs := e.resume(session("alice"), last, "instance:apps/blog").rec.take()
		if got := eventNames(evs); !slices.Equal(got, []string{"open()", "snapshot(instance:apps/web)"}) {
			t.Errorf("events = %v", got)
		}
	})
}
