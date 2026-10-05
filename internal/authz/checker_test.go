package authz

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	deployments = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	pods        = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	secrets     = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	alice       = Identity{Username: "alice", Groups: []string{"dev", "system:authenticated"}}
)

// fakeDecider records how often the backend was asked.
type fakeDecider struct {
	calls   atomic.Int32
	allowed bool
	err     error
	block   bool
}

func (f *fakeDecider) decide(ctx context.Context, _ Identity, _ Attributes) (bool, error) {
	f.calls.Add(1)
	if f.block {
		<-ctx.Done()
		return false, ctx.Err()
	}
	return f.allowed, f.err
}

func getDeployment(ns, name string) Attributes {
	return Attributes{Verb: "get", Resource: deployments, Namespace: ns, Name: name}
}

func requireDenial(t *testing.T, g Grant, err error, want Code) *DenialError {
	t.Helper()
	if g.Valid() {
		t.Fatalf("got a valid grant alongside %v", err)
	}
	d, ok := errors.AsType[*DenialError](err)
	if !ok {
		t.Fatalf("error %v (%T) is not a *DenialError", err, err)
	}
	if d.Code != want {
		t.Fatalf("denial code = %q, want %q (%v)", d.Code, want, d)
	}
	return d
}

// TestEmptyIdentityNeverReachesTheBackend is the regression test for the
// empty-identity class of bug (Flux Operator CVE-2026-23990): a principal
// with no username, whatever groups it claims, is refused and no review is
// sent on its behalf.
func TestEmptyIdentityNeverReachesTheBackend(t *testing.T) {
	cases := map[string]Identity{
		"zero":                     {},
		"blank username":           {Username: "  \t"},
		"groups only":              {Groups: []string{"system:masters"}},
		"uid and extra only":       {UID: "1234", Extra: map[string][]string{"scopes": {"all"}}},
		"anonymous":                {Username: "system:anonymous"},
		"anonymous with a group":   {Username: "system:anonymous", Groups: []string{"system:unauthenticated"}},
		"empty with masters group": {Username: "", Groups: []string{"system:masters", "system:authenticated"}},
	}
	for name, who := range cases {
		t.Run(name, func(t *testing.T) {
			backend := &fakeDecider{allowed: true}
			g, err := newChecker(backend, Options{}).Check(t.Context(), who, getDeployment("team-a", "web"))
			requireDenial(t, g, err, CodeUnauthenticated)
			if n := backend.calls.Load(); n != 0 {
				t.Fatalf("backend asked %d times for an unauthenticated identity", n)
			}
		})
	}
}

func TestRefusedAttributesNeverReachTheBackend(t *testing.T) {
	cases := []struct {
		name string
		req  Attributes
		want Code
	}{
		{"create", Attributes{Verb: "create", Resource: deployments, Namespace: "a"}, CodeForbidden},
		{"update", Attributes{Verb: "update", Resource: deployments, Namespace: "a", Name: "web"}, CodeForbidden},
		{"patch", Attributes{Verb: "patch", Resource: deployments, Namespace: "a", Name: "web"}, CodeForbidden},
		{"delete", Attributes{Verb: "delete", Resource: deployments, Namespace: "a", Name: "web"}, CodeForbidden},
		{"deletecollection", Attributes{Verb: "deletecollection", Resource: deployments, Namespace: "a"}, CodeForbidden},
		{"impersonate", Attributes{Verb: "impersonate", Resource: schema.GroupVersionResource{Version: "v1", Resource: "users"}}, CodeForbidden},
		{"escalate", Attributes{Verb: "escalate", Resource: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}}, CodeForbidden},
		{"get secret", Attributes{Verb: "get", Resource: secrets, Namespace: "a", Name: "db"}, CodeForbidden},
		{"list secrets", Attributes{Verb: "list", Resource: secrets, Namespace: "a"}, CodeForbidden},
		{"watch secrets cluster-wide", Attributes{Verb: "watch", Resource: secrets}, CodeForbidden},
		{"get pods/exec", Attributes{Verb: "get", Resource: pods, Subresource: "exec", Namespace: "a", Name: "p"}, CodeForbidden},
		{"get pods/attach", Attributes{Verb: "get", Resource: pods, Subresource: "attach", Namespace: "a", Name: "p"}, CodeForbidden},
		{"get pods/portforward", Attributes{Verb: "get", Resource: pods, Subresource: "portforward", Namespace: "a", Name: "p"}, CodeForbidden},
		{"get pods/proxy", Attributes{Verb: "get", Resource: pods, Subresource: "proxy", Namespace: "a", Name: "p"}, CodeForbidden},
		{"get nodes/proxy", Attributes{Verb: "get", Resource: schema.GroupVersionResource{Version: "v1", Resource: "nodes"}, Subresource: "proxy", Name: "n"}, CodeForbidden},
		{"get services/proxy", Attributes{Verb: "get", Resource: schema.GroupVersionResource{Version: "v1", Resource: "services"}, Subresource: "proxy", Namespace: "a", Name: "s"}, CodeForbidden},
		{"get deployments/scale", Attributes{Verb: "get", Resource: deployments, Subresource: "scale", Namespace: "a", Name: "web"}, CodeForbidden},
		{"empty verb", Attributes{Resource: deployments}, CodeInvalid},
		{"empty resource", Attributes{Verb: "get"}, CodeInvalid},
		{"wildcard verb", Attributes{Verb: "*", Resource: deployments}, CodeInvalid},
		{"wildcard resource", Attributes{Verb: "get", Resource: schema.GroupVersionResource{Resource: "*"}}, CodeInvalid},
		{"wildcard group", Attributes{Verb: "list", Resource: schema.GroupVersionResource{Group: "*", Resource: "secrets"}}, CodeInvalid},
		{"wildcard subresource", Attributes{Verb: "get", Resource: pods, Subresource: "*"}, CodeInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			backend := &fakeDecider{allowed: true}
			g, err := newChecker(backend, Options{}).Check(t.Context(), alice, tc.req)
			requireDenial(t, g, err, tc.want)
			if n := backend.calls.Load(); n != 0 {
				t.Fatalf("backend asked %d times for a refused request", n)
			}
		})
	}
}

// TestReadSubresourcesReachTheBackend: the object, its status and a Pod's
// log are the reads the portal makes, so they are asked about.
func TestReadSubresourcesReachTheBackend(t *testing.T) {
	for _, req := range []Attributes{
		getDeployment("a", "web"),
		{Verb: "get", Resource: deployments, Subresource: "status", Namespace: "a", Name: "web"},
		{Verb: "get", Resource: pods, Subresource: "log", Namespace: "a", Name: "p"},
	} {
		backend := &fakeDecider{allowed: true}
		if _, err := newChecker(backend, Options{}).Check(t.Context(), alice, req); err != nil {
			t.Errorf("Check(%s) = %v, want a grant", req, err)
		}
		if n := backend.calls.Load(); n != 1 {
			t.Errorf("backend asked %d times for %s, want 1", n, req)
		}
	}
}

func TestCheckOutcomes(t *testing.T) {
	apiErr := errors.New("connection refused")
	cases := []struct {
		name    string
		backend *fakeDecider
		want    Code // empty: allowed
	}{
		{"allowed", &fakeDecider{allowed: true}, ""},
		{"denied", &fakeDecider{}, CodeForbidden},
		{"backend error is a denial", &fakeDecider{allowed: true, err: apiErr}, CodeUnavailable},
		{"backend denial passes through", &fakeDecider{err: &DenialError{Code: CodeUnauthenticated}}, CodeUnauthenticated},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := getDeployment("team-a", "web")
			g, err := newChecker(tc.backend, Options{}).Check(t.Context(), alice, req)
			if tc.want != "" {
				d := requireDenial(t, g, err, tc.want)
				if errors.Is(tc.backend.err, apiErr) && !errors.Is(d, apiErr) {
					t.Errorf("denial does not wrap the backend error: %v", d)
				}
				if strings.Contains(d.Error(), apiErr.Error()) {
					t.Errorf("denial text %q carries the backend error's text", d)
				}
				return
			}
			if err != nil || !g.Valid() {
				t.Fatalf("Check = %v, %v; want a valid grant", g, err)
			}
			if got := g.Attributes(); got != req {
				t.Errorf("grant attributes = %+v, want %+v", got, req)
			}
			if got := g.Identity(); got.key() != alice.key() {
				t.Errorf("grant identity = %+v, want %+v", got, alice)
			}
		})
	}
}

func TestCheckTimesOutAsUnavailable(t *testing.T) {
	backend := &fakeDecider{block: true}
	start := time.Now()
	g, err := newChecker(backend, Options{Timeout: 20 * time.Millisecond}).Check(t.Context(), alice, getDeployment("a", "web"))
	d := requireDenial(t, g, err, CodeUnavailable)
	if !errors.Is(d, context.DeadlineExceeded) {
		t.Errorf("denial does not wrap the deadline: %v", d)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Check took %v; the timeout did not apply", elapsed)
	}
}

func TestUnconfiguredCheckerFailsClosed(t *testing.T) {
	var nilChecker *Checker
	for name, c := range map[string]*Checker{"nil": nilChecker, "zero": {}} {
		t.Run(name, func(t *testing.T) {
			g, err := c.Check(t.Context(), alice, getDeployment("a", "web"))
			requireDenial(t, g, err, CodeUnavailable)
		})
	}
}

// TestDenialReadsTheSameForAnyName: the refusal for an object that exists
// and one that does not must be indistinguishable.
func TestDenialReadsTheSameForAnyName(t *testing.T) {
	c := newChecker(&fakeDecider{}, Options{})
	_, errA := c.Check(t.Context(), alice, getDeployment("team-a", "exists"))
	_, errB := c.Check(t.Context(), alice, getDeployment("team-a", "missing"))
	if errA.Error() != errB.Error() {
		t.Fatalf("denials differ:\n%v\n%v", errA, errB)
	}
	for _, leak := range []string{"exists", "alice"} {
		if strings.Contains(errA.Error(), leak) {
			t.Errorf("denial %q names %q", errA, leak)
		}
	}
}

func TestGrantCopiesTheIdentity(t *testing.T) {
	who := Identity{Username: "bob", Groups: []string{"dev"}, Extra: map[string][]string{"k": {"v"}}}
	g, err := newChecker(&fakeDecider{allowed: true}, Options{}).Check(t.Context(), who, getDeployment("a", "web"))
	if err != nil {
		t.Fatal(err)
	}
	who.Groups[0] = "system:masters"
	who.Extra["k"][0] = "changed"
	got := g.Identity()
	if got.Groups[0] != "dev" || got.Extra["k"][0] != "v" {
		t.Fatalf("grant identity changed with the caller's copy: %+v", got)
	}
	got.Groups[0] = "system:masters"
	if g.Identity().Groups[0] != "dev" {
		t.Fatal("grant identity changed through its accessor")
	}
}

func TestZeroGrant(t *testing.T) {
	var g Grant
	if g.Valid() {
		t.Fatal("zero Grant is valid")
	}
	if err := g.Covers(alice, getDeployment("a", "web")); !errors.Is(err, ErrNoGrant) {
		t.Fatalf("zero Grant Covers = %v, want ErrNoGrant", err)
	}
	if err := (Grant{}).Covers(Identity{}, Attributes{}); !errors.Is(err, ErrNoGrant) {
		t.Fatalf("zero Grant covers zero attributes: %v", err)
	}
	if g.Identity().Authenticated() || g.Attributes() != (Attributes{}) || !g.Expires().IsZero() {
		t.Fatal("zero Grant reports an identity, attributes or an expiry")
	}
}

// scopeCases are the scope rules both Attributes.Covers and Grant.Covers
// apply.
var scopeCases = []struct {
	name    string
	granted Attributes
	req     Attributes
	covers  bool
}{
	{"exact", getDeployment("a", "web"), getDeployment("a", "web"), true},
	{"other name", getDeployment("a", "web"), getDeployment("a", "db"), false},
	{"other namespace", getDeployment("a", "web"), getDeployment("b", "web"), false},
	{"any name covers one", getDeployment("a", ""), getDeployment("a", "web"), true},
	{"any namespace covers one", getDeployment("", ""), getDeployment("b", "web"), true},
	{"one name does not cover all", getDeployment("a", "web"), getDeployment("a", ""), false},
	{"one namespace does not cover all", getDeployment("a", ""), getDeployment("", ""), false},
	{"other verb", Attributes{Verb: "list", Resource: deployments, Namespace: "a"}, Attributes{Verb: "watch", Resource: deployments, Namespace: "a"}, false},
	{"other version", getDeployment("a", "web"), Attributes{Verb: "get", Resource: schema.GroupVersionResource{Group: "apps", Version: "v1beta1", Resource: "deployments"}, Namespace: "a", Name: "web"}, false},
	{"pod does not cover its log", Attributes{Verb: "get", Resource: pods, Namespace: "a", Name: "p"}, Attributes{Verb: "get", Resource: pods, Subresource: "log", Namespace: "a", Name: "p"}, false},
	{"log covers log", Attributes{Verb: "get", Resource: pods, Subresource: "log", Namespace: "a", Name: "p"}, Attributes{Verb: "get", Resource: pods, Subresource: "log", Namespace: "a", Name: "p"}, true},
}

func TestAttributesCovers(t *testing.T) {
	for _, tc := range scopeCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.granted.Covers(tc.req); got != tc.covers {
				t.Fatalf("Covers = %v, want %v", got, tc.covers)
			}
		})
	}
}

func TestGrantCovers(t *testing.T) {
	for _, tc := range scopeCases {
		t.Run(tc.name, func(t *testing.T) {
			g, err := newChecker(&fakeDecider{allowed: true}, Options{}).Check(t.Context(), alice, tc.granted)
			if err != nil {
				t.Fatal(err)
			}
			err = g.Covers(alice, tc.req)
			if tc.covers && err != nil {
				t.Fatalf("Covers = %v, want nil", err)
			}
			if !tc.covers && !errors.Is(err, ErrNoGrant) {
				t.Fatalf("Covers = %v, want ErrNoGrant", err)
			}
		})
	}
}

// TestGrantIsBoundToItsIdentity: a grant kept in shared state must not
// cover the same read for another caller (0030:D7).
func TestGrantIsBoundToItsIdentity(t *testing.T) {
	req := getDeployment("team-a", "web")
	g, err := newChecker(&fakeDecider{allowed: true}, Options{}).Check(t.Context(), alice, req)
	if err != nil {
		t.Fatal(err)
	}
	reordered := Identity{Username: "alice", Groups: []string{"system:authenticated", "dev"}}
	if err := g.Covers(reordered, req); err != nil {
		t.Fatalf("grant does not cover its own identity with its groups reordered: %v", err)
	}
	others := map[string]Identity{
		"another user":       {Username: "bob", Groups: alice.Groups},
		"same name, a group": {Username: "alice", Groups: []string{"dev"}},
		"same name, a uid":   {Username: "alice", UID: "1", Groups: alice.Groups},
		"empty":              {},
	}
	for name, who := range others {
		t.Run(name, func(t *testing.T) {
			err := g.Covers(who, req)
			if !errors.Is(err, ErrNoGrant) {
				t.Fatalf("Covers = %v, want ErrNoGrant", err)
			}
			if strings.Contains(err.Error(), "alice") || strings.Contains(err.Error(), "web") {
				t.Errorf("error %q names the identity or the object", err)
			}
		})
	}
}

// TestGrantExpiresWithItsDecision: a held grant (a stream, a long request)
// stops covering reads when the decision behind it expires, so a revocation
// reaches it within one TTL.
func TestGrantExpiresWithItsDecision(t *testing.T) {
	c, clk := checkerWithClock(&fakeDecider{allowed: true}, Options{TTL: 10 * time.Second})
	req := getDeployment("team-a", "web")
	g, err := c.Check(t.Context(), alice, req)
	if err != nil {
		t.Fatal(err)
	}
	clk.advance(9 * time.Second)
	if err := g.Covers(alice, req); err != nil {
		t.Fatalf("grant expired early: %v", err)
	}
	// A grant issued from the cached decision expires with that decision,
	// not one TTL after its own issue.
	cachedGrant, err := c.Check(t.Context(), alice, req)
	if err != nil {
		t.Fatal(err)
	}
	if !cachedGrant.Expires().Equal(g.Expires()) {
		t.Fatalf("grant from the cache expires at %v, want the decision's %v", cachedGrant.Expires(), g.Expires())
	}
	clk.advance(time.Second)
	for name, held := range map[string]Grant{"fresh": g, "from cache": cachedGrant} {
		if err := held.Covers(alice, req); !errors.Is(err, ErrNoGrant) {
			t.Errorf("%s grant Covers after its TTL = %v, want ErrNoGrant", name, err)
		}
	}
}

// TestUncachedGrantStillExpires: a decision the full cache could not store
// still gives its grant one TTL.
func TestUncachedGrantStillExpires(t *testing.T) {
	c, clk := checkerWithClock(&fakeDecider{allowed: true}, Options{MaxEntries: 1})
	if _, err := c.Check(t.Context(), alice, getDeployment("a", "one")); err != nil {
		t.Fatal(err)
	}
	req := getDeployment("a", "two")
	g, err := c.Check(t.Context(), alice, req)
	if err != nil {
		t.Fatal(err)
	}
	if n := c.cache.len(); n != 1 {
		t.Fatalf("cache holds %d entries, want 1 (the second decision not stored)", n)
	}
	clk.advance(defaultTTL)
	if err := g.Covers(alice, req); !errors.Is(err, ErrNoGrant) {
		t.Fatalf("uncached grant Covers after the TTL = %v, want ErrNoGrant", err)
	}
}

func TestIdentityKeyIsCanonical(t *testing.T) {
	a := Identity{Username: "u", Groups: []string{"b", "a"}, Extra: map[string][]string{"x": {"2", "1"}, "y": {"z"}}}
	b := Identity{Username: "u", Groups: []string{"a", "b"}, Extra: map[string][]string{"y": {"z"}, "x": {"1", "2"}}}
	if a.key() != b.key() {
		t.Fatalf("order changed the key:\n%s\n%s", a.key(), b.key())
	}
	distinct := []Identity{
		{Username: "u"},
		{Username: "u", UID: "1"},
		{Username: "u", Groups: []string{"a"}},
		{Username: "u", Groups: []string{"a,b"}},
		{Username: "u", Groups: []string{"a", "b"}},
		{Username: "u", Extra: map[string][]string{"a": {"b"}}},
		{Username: "u;g=[]"},
		{Username: "u", reader: true},
		{Username: "reader;u=\"u\""},
	}
	seen := map[string]int{}
	for i, id := range distinct {
		if j, ok := seen[id.key()]; ok {
			t.Errorf("identities %d and %d share key %s", j, i, id.key())
		}
		seen[id.key()] = i
	}
}
