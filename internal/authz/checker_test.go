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
	if err := g.Covers(getDeployment("a", "web")); !errors.Is(err, ErrNoGrant) {
		t.Fatalf("zero Grant Covers = %v, want ErrNoGrant", err)
	}
	if err := (Grant{}).Covers(Attributes{}); !errors.Is(err, ErrNoGrant) {
		t.Fatalf("zero Grant covers zero attributes: %v", err)
	}
	if g.Identity().Authenticated() || g.Attributes() != (Attributes{}) {
		t.Fatal("zero Grant reports an identity or attributes")
	}
}

func TestGrantCovers(t *testing.T) {
	cases := []struct {
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
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, err := newChecker(&fakeDecider{allowed: true}, Options{}).Check(t.Context(), alice, tc.granted)
			if err != nil {
				t.Fatal(err)
			}
			err = g.Covers(tc.req)
			if tc.covers && err != nil {
				t.Fatalf("Covers = %v, want nil", err)
			}
			if !tc.covers && !errors.Is(err, ErrNoGrant) {
				t.Fatalf("Covers = %v, want ErrNoGrant", err)
			}
		})
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
	}
	seen := map[string]int{}
	for i, id := range distinct {
		if j, ok := seen[id.key()]; ok {
			t.Errorf("identities %d and %d share key %s", j, i, id.key())
		}
		seen[id.key()] = i
	}
}
