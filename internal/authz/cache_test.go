package authz

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
)

// clock is a settable time source for the decision cache.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func checkerWithClock(backend decider, opts Options) (*Checker, *clock) {
	c := newChecker(backend, opts)
	clk := &clock{t: time.Unix(1_800_000_000, 0)}
	c.cache.now = clk.now
	return c, clk
}

func TestCacheReusesDecisionsWithinTheTTL(t *testing.T) {
	for _, allowed := range []bool{true, false} {
		t.Run(fmt.Sprintf("allowed=%v", allowed), func(t *testing.T) {
			backend := &fakeDecider{allowed: allowed}
			c, clk := checkerWithClock(backend, Options{})
			req := getDeployment("team-a", "web")
			for range 3 {
				g, err := c.Check(t.Context(), alice, req)
				if g.Valid() != allowed || (err == nil) != allowed {
					t.Fatalf("Check = %v, %v; want allowed=%v", g, err, allowed)
				}
			}
			if n := backend.calls.Load(); n != 1 {
				t.Fatalf("backend asked %d times within the TTL, want 1", n)
			}
			clk.advance(defaultTTL - time.Second)
			_, _ = c.Check(t.Context(), alice, req)
			if n := backend.calls.Load(); n != 1 {
				t.Fatalf("backend asked again before the TTL ran out")
			}
			clk.advance(time.Second)
			_, _ = c.Check(t.Context(), alice, req)
			if n := backend.calls.Load(); n != 2 {
				t.Fatalf("backend asked %d times after the TTL, want 2", n)
			}
		})
	}
}

func TestCacheHonoursAConfiguredTTL(t *testing.T) {
	backend := &fakeDecider{allowed: true}
	c, clk := checkerWithClock(backend, Options{TTL: 5 * time.Second})
	req := getDeployment("team-a", "web")
	_, _ = c.Check(t.Context(), alice, req)
	clk.advance(5 * time.Second)
	_, _ = c.Check(t.Context(), alice, req)
	if n := backend.calls.Load(); n != 2 {
		t.Fatalf("backend asked %d times across a 5s TTL, want 2", n)
	}
}

func TestCacheNeverStoresFailures(t *testing.T) {
	rc := newReviewCluster(t)
	rc.err = errors.New("connection refused")
	c := rc.checker(t, alice)
	req := getDeployment("team-a", "web")
	g, err := c.Check(t.Context(), alice, req)
	requireDenial(t, g, err, CodeUnavailable)

	rc.err = nil
	rc.status = authorizationv1.SubjectAccessReviewStatus{EvaluationError: "webhook down"}
	g, err = c.Check(t.Context(), alice, req)
	requireDenial(t, g, err, CodeUnavailable)

	rc.status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}
	g, err = c.Check(t.Context(), alice, req)
	if err != nil || !g.Valid() {
		t.Fatalf("Check after recovery = %v, %v; want a grant", g, err)
	}
	if len(rc.seen) != 3 {
		t.Fatalf("reviews sent = %d, want 3 (failures must not be cached)", len(rc.seen))
	}
	if n := c.cache.len(); n != 1 {
		t.Fatalf("cache holds %d entries, want only the allow", n)
	}
}

func TestCacheNeverStoresGuardRefusals(t *testing.T) {
	c := newChecker(&fakeDecider{allowed: true}, Options{})
	_, _ = c.Check(t.Context(), Identity{}, getDeployment("a", "web"))
	_, _ = c.Check(t.Context(), alice, Attributes{Verb: "delete", Resource: deployments, Namespace: "a"})
	_, _ = c.Check(t.Context(), Identity{Username: "bob"}, Attributes{Verb: "get", Resource: secrets, Namespace: "a"})
	if n := c.cache.len(); n != 0 {
		t.Fatalf("cache holds %d guard refusals", n)
	}
}

func TestCacheKeySeparatesIdentityAndRequest(t *testing.T) {
	rc := newReviewCluster(t)
	rc.status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}
	base := getDeployment("team-a", "web")
	variants := []Attributes{
		base,
		getDeployment("team-b", "web"),
		getDeployment("team-a", "db"),
		getDeployment("team-a", ""),
		{Verb: "list", Resource: deployments, Namespace: "team-a"},
		{Verb: "get", Resource: pods, Namespace: "team-a", Name: "web"},
		{Verb: "get", Resource: pods, Subresource: "log", Namespace: "team-a", Name: "web"},
	}
	c := rc.checker(t, alice)
	for _, req := range variants {
		if _, err := c.Check(t.Context(), alice, req); err != nil {
			t.Fatal(err)
		}
	}
	if len(rc.seen) != len(variants) {
		t.Fatalf("reviews sent = %d, want one per distinct request (%d)", len(rc.seen), len(variants))
	}

	// Group order is not part of the key; the groups themselves are.
	backend := &fakeDecider{allowed: true}
	shared := newChecker(backend, Options{})
	for _, who := range []Identity{
		{Username: "u", Groups: []string{"a", "b"}},
		{Username: "u", Groups: []string{"b", "a"}},
		{Username: "u", Groups: []string{"a"}},
		{Username: "v", Groups: []string{"a", "b"}},
	} {
		if _, err := shared.Check(t.Context(), who, base); err != nil {
			t.Fatal(err)
		}
	}
	if n := backend.calls.Load(); n != 3 {
		t.Fatalf("backend asked %d times for 3 distinct identities", n)
	}
}

func TestCacheBound(t *testing.T) {
	backend := &fakeDecider{allowed: true}
	c, clk := checkerWithClock(backend, Options{MaxEntries: 2})
	check := func(name string) {
		t.Helper()
		if _, err := c.Check(t.Context(), alice, getDeployment("a", name)); err != nil {
			t.Fatal(err)
		}
	}
	check("one")
	check("two")
	check("three") // full: not stored
	if n := c.cache.len(); n != 2 {
		t.Fatalf("cache holds %d entries, want its bound 2", n)
	}
	check("three")
	if n := backend.calls.Load(); n != 4 {
		t.Fatalf("backend asked %d times, want 4 (the uncached request asks again)", n)
	}

	clk.advance(defaultTTL)
	check("four") // expired entries make room
	if n := c.cache.len(); n != 1 {
		t.Fatalf("cache holds %d entries after expiry, want 1", n)
	}
}

func TestCacheConcurrentChecks(t *testing.T) {
	backend := &fakeDecider{allowed: true}
	c := newChecker(backend, Options{})
	var wg sync.WaitGroup
	for i := range 64 {
		wg.Go(func() {
			who := Identity{Username: fmt.Sprintf("user-%d", i%4)}
			g, err := c.Check(t.Context(), who, getDeployment("a", fmt.Sprintf("obj-%d", i%8)))
			if err != nil || !g.Valid() {
				t.Errorf("Check = %v, %v", g, err)
			}
		})
	}
	wg.Wait()
	if n := c.cache.len(); n > 8 {
		t.Fatalf("cache holds %d entries for 8 distinct pairs", n)
	}
}
