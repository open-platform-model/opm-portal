package authz

import (
	"encoding/base64"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const portalSA = "system:serviceaccount:opm-portal:opm-portal"

// person is a signed-in user as identity mapping hands it over: prefixed
// name and groups, system:authenticated added.
var person = Identity{
	Username: "oidc:alice",
	UID:      "a1b2",
	Groups:   []string{"oidc:team-a", "system:authenticated"},
	Extra:    map[string][]string{"oidc.example/acr": {"mfa"}},
}

func portalReader(t *testing.T) Identity {
	t.Helper()
	r, err := ServiceAccountIdentity(portalSA)
	if err != nil {
		t.Fatalf("ServiceAccountIdentity: %v", err)
	}
	return r
}

// sarCluster is a fake cluster that answers SubjectAccessReviews from
// answer (or fails with err) and records every review it sees. Any self
// review fails the test; Actions on the embedded clientset records every
// request made at all.
type sarCluster struct {
	*fake.Clientset
	mu     sync.Mutex
	answer func(spec authorizationv1.SubjectAccessReviewSpec) authorizationv1.SubjectAccessReviewStatus
	err    error
	seen   []authorizationv1.SubjectAccessReviewSpec
}

func newSARCluster(t *testing.T) *sarCluster {
	t.Helper()
	sc := &sarCluster{Clientset: fake.NewClientset()}
	sc.answer = func(authorizationv1.SubjectAccessReviewSpec) authorizationv1.SubjectAccessReviewStatus {
		return authorizationv1.SubjectAccessReviewStatus{Allowed: true}
	}
	sc.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		sc.mu.Lock()
		defer sc.mu.Unlock()
		sc.seen = append(sc.seen, *review.Spec.DeepCopy())
		if sc.err != nil {
			return true, nil, sc.err
		}
		out := review.DeepCopy()
		out.Status = sc.answer(review.Spec)
		return true, out, nil
	})
	for _, self := range []string{"selfsubjectaccessreviews", "selfsubjectreviews"} {
		sc.PrependReactor("create", self, func(action k8stesting.Action) (bool, runtime.Object, error) {
			t.Errorf("in-cluster mode sent a %s", action.GetResource().Resource)
			return true, nil, errors.New("self reviews are forbidden in-cluster")
		})
	}
	return sc
}

func (sc *sarCluster) checker(t *testing.T, opts Options) *Checker {
	t.Helper()
	c, err := NewInCluster(sc.AuthorizationV1().SubjectAccessReviews(), portalReader(t), opts)
	if err != nil {
		t.Fatalf("NewInCluster: %v", err)
	}
	return c
}

func (sc *sarCluster) reviews() []authorizationv1.SubjectAccessReviewSpec {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return slices.Clone(sc.seen)
}

func TestServiceAccountIdentity(t *testing.T) {
	got, err := ServiceAccountIdentity(portalSA)
	if err != nil {
		t.Fatalf("ServiceAccountIdentity: %v", err)
	}
	want := Identity{Username: portalSA, Groups: []string{"system:serviceaccounts", "system:serviceaccounts:opm-portal", "system:authenticated"}, reader: true}
	if got.key() != want.key() {
		t.Fatalf("ServiceAccountIdentity = %+v, want %+v", got, want)
	}
	for _, bad := range []string{
		"", "opm-portal", "system:serviceaccount:", "system:serviceaccount:opm-portal",
		"system:serviceaccount::opm-portal", "system:serviceaccount:opm-portal:", "system:serviceaccount:a:b:c",
		"system:serviceaccount: a:b", "system:admin", "oidc:system:serviceaccount:a:b",
	} {
		if _, err := ServiceAccountIdentity(bad); err == nil {
			t.Errorf("ServiceAccountIdentity(%q) accepted a non-ServiceAccount username", bad)
		}
	}
}

func jwt(payload string) []byte {
	enc := base64.RawURLEncoding.EncodeToString
	return []byte(enc([]byte(`{"alg":"RS256"}`)) + "." + enc([]byte(payload)) + ".c2ln")
}

func TestServiceAccountFromToken(t *testing.T) {
	got, err := ServiceAccountFromToken(append(jwt(`{"sub":"`+portalSA+`","aud":["https://kubernetes.default.svc"]}`), '\n'))
	if err != nil || got != portalSA {
		t.Fatalf("ServiceAccountFromToken = %q, %v; want %q", got, err, portalSA)
	}
	cases := map[string][]byte{
		"empty":            nil,
		"not a JWT":        []byte("opaque-token"),
		"bad base64":       []byte("a.!!!.c"),
		"not JSON":         jwt(`sub=x`),
		"no sub":           jwt(`{"iss":"kubernetes"}`),
		"empty sub":        jwt(`{"sub":""}`),
		"a user, not a SA": jwt(`{"sub":"alice"}`),
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ServiceAccountFromToken(token); err == nil {
				t.Fatal("ServiceAccountFromToken accepted it")
			}
		})
	}
}

func TestNewInClusterRefusesWhatCannotBeServed(t *testing.T) {
	reviews := fake.NewClientset().AuthorizationV1().SubjectAccessReviews()
	reader := portalReader(t)
	if _, err := NewInCluster(nil, reader, Options{}); err == nil {
		t.Error("NewInCluster accepted no review client")
	}
	widened := reader.clone()
	widened.Groups = append(widened.Groups, "system:masters")
	// The reader's exact claims, not built by ServiceAccountIdentity.
	spelled := Identity{Username: portalSA, Groups: slices.Clone(reader.Groups)}
	for _, r := range []Identity{{}, {Username: "system:anonymous"}, person, {Username: portalSA}, widened, spelled} {
		if _, err := NewInCluster(reviews, r, Options{}); err == nil {
			t.Errorf("NewInCluster accepted reader %+v", r)
		}
	}
}

func TestInClusterSendsTheExactReview(t *testing.T) {
	sc := newSARCluster(t)
	req := Attributes{Verb: "get", Resource: pods, Subresource: "log", Namespace: "team-a", Name: "web-0"}
	g, err := sc.checker(t, Options{}).Check(t.Context(), person, req)
	if err != nil || g.Covers(person, req) != nil {
		t.Fatalf("Check = %v, %v; want a grant covering the read", g, err)
	}
	seen := sc.reviews()
	if len(seen) != 1 {
		t.Fatalf("reviews sent = %d, want 1", len(seen))
	}
	got := seen[0]
	wantAttrs := authorizationv1.ResourceAttributes{Verb: "get", Version: "v1", Resource: "pods", Subresource: "log", Namespace: "team-a", Name: "web-0"}
	if got.ResourceAttributes == nil || *got.ResourceAttributes != wantAttrs {
		t.Errorf("resource attributes = %+v, want %+v", got.ResourceAttributes, wantAttrs)
	}
	if got.User != person.Username || got.UID != person.UID || !slices.Equal(got.Groups, person.Groups) {
		t.Errorf("review subject = %q %q %v, want the person's", got.User, got.UID, got.Groups)
	}
	if len(got.Extra) != 1 || !slices.Equal([]string(got.Extra["oidc.example/acr"]), []string{"mfa"}) {
		t.Errorf("review extra = %v, want the person's", got.Extra)
	}
	if got.NonResourceAttributes != nil {
		t.Errorf("review carries non-resource attributes %+v", got.NonResourceAttributes)
	}
}

func TestInClusterReviewsTheReaderByName(t *testing.T) {
	sc := newSARCluster(t)
	c := sc.checker(t, Options{TTL: time.Minute})
	clk := &clock{t: time.Unix(1_800_000_000, 0)}
	c.cache.now = clk.now
	reader := portalReader(t)
	list := Attributes{Verb: "list", Resource: deployments}

	for range 2 {
		if _, err := c.Check(t.Context(), reader, list); err != nil {
			t.Fatalf("Check(reader) = %v", err)
		}
	}
	clk.advance(time.Minute)
	g, err := c.Check(t.Context(), reader, list)
	if err != nil {
		t.Fatalf("Check(reader) after the TTL = %v", err)
	}
	if !g.Expires().Equal(clk.now().Add(time.Minute)) {
		t.Errorf("reader grant expires %v, want one TTL from the renewal", g.Expires())
	}

	seen := sc.reviews()
	if len(seen) != 2 {
		t.Fatalf("reviews sent = %d, want one at first and one after the TTL", len(seen))
	}
	for _, s := range seen {
		if s.User != portalSA || !slices.Equal(s.Groups, reader.Groups) || s.UID != "" || len(s.Extra) != 0 {
			t.Errorf("reader review subject = %q %v %q %v, want the ServiceAccount with its own groups", s.User, s.Groups, s.UID, s.Extra)
		}
	}
}

func TestInClusterVerdicts(t *testing.T) {
	cases := []struct {
		name   string
		status authorizationv1.SubjectAccessReviewStatus
		err    error
		want   Code // empty: allowed
	}{
		{"allowed", authorizationv1.SubjectAccessReviewStatus{Allowed: true}, nil, ""},
		{"denied", authorizationv1.SubjectAccessReviewStatus{Denied: true}, nil, CodeForbidden},
		{"no opinion", authorizationv1.SubjectAccessReviewStatus{}, nil, CodeForbidden},
		{"deny wins over allow", authorizationv1.SubjectAccessReviewStatus{Allowed: true, Denied: true}, nil, CodeForbidden},
		{"evaluation error without allow", authorizationv1.SubjectAccessReviewStatus{EvaluationError: "webhook names oidc:bob"}, nil, CodeUnavailable},
		{"review forbidden by the API", authorizationv1.SubjectAccessReviewStatus{}, apierrors.NewForbidden(authorizationv1.Resource("subjectaccessreviews"), "", errors.New("no")), CodeUnavailable},
		{"API timeout", authorizationv1.SubjectAccessReviewStatus{Allowed: true}, apierrors.NewTimeoutError("slow", 1), CodeUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sc := newSARCluster(t)
			sc.answer = func(authorizationv1.SubjectAccessReviewSpec) authorizationv1.SubjectAccessReviewStatus {
				return tc.status
			}
			sc.err = tc.err
			g, err := sc.checker(t, Options{}).Check(t.Context(), person, getDeployment("team-a", "web"))
			if tc.want == "" {
				if err != nil || !g.Valid() {
					t.Fatalf("Check = %v, %v; want a grant", g, err)
				}
				return
			}
			d := requireDenial(t, g, err, tc.want)
			if strings.Contains(d.Error(), "bob") {
				t.Errorf("denial %q carries the review's own text", d.Error())
			}
		})
	}
}

func TestInClusterFailureIsNeverCached(t *testing.T) {
	sc := newSARCluster(t)
	sc.err = apierrors.NewInternalError(errors.New("etcd down"))
	c := sc.checker(t, Options{})
	g, err := c.Check(t.Context(), person, getDeployment("team-a", "web"))
	requireDenial(t, g, err, CodeUnavailable)

	sc.mu.Lock()
	sc.err = nil
	sc.mu.Unlock()
	if g, err := c.Check(t.Context(), person, getDeployment("team-a", "web")); err != nil || !g.Valid() {
		t.Fatalf("Check after recovery = %v, %v; want a grant", g, err)
	}
	if n := len(sc.reviews()); n != 2 {
		t.Fatalf("reviews sent = %d, want 2 (a failure is asked again)", n)
	}
}

func TestInClusterCacheKeyIsTheFullIdentity(t *testing.T) {
	sc := newSARCluster(t)
	sc.answer = func(s authorizationv1.SubjectAccessReviewSpec) authorizationv1.SubjectAccessReviewStatus {
		// Only the person with the acr=mfa extra value may read.
		return authorizationv1.SubjectAccessReviewStatus{Allowed: slices.Equal([]string(s.Extra["oidc.example/acr"]), []string{"mfa"})}
	}
	c := sc.checker(t, Options{})
	req := getDeployment("team-a", "web")
	if _, err := c.Check(t.Context(), person, req); err != nil {
		t.Fatalf("Check(person) = %v", err)
	}
	other := person.clone()
	other.Extra = map[string][]string{"oidc.example/acr": {"pwd"}}
	g, err := c.Check(t.Context(), other, req)
	requireDenial(t, g, err, CodeForbidden)
	if n := len(sc.reviews()); n != 2 {
		t.Fatalf("reviews sent = %d, want 2 (identities differing in extra values share no decision)", n)
	}
}

func TestInClusterRefusesSystemIdentities(t *testing.T) {
	reader := portalReader(t)
	saName := Identity{Username: portalSA, Groups: []string{"system:authenticated"}}
	saGroups := person.clone()
	saGroups.Groups = reader.Groups
	cases := map[string]Identity{
		"the ServiceAccount's name without its groups": saName,
		"another ServiceAccount":                       {Username: "system:serviceaccount:kube-system:default", Groups: []string{"system:authenticated"}},
		"a system user":                                {Username: "system:admin"},
		"a padded system user":                         {Username: " system:admin"},
		"system:masters":                               {Username: "oidc:alice", Groups: []string{"system:masters", "system:authenticated"}},
		"the ServiceAccount groups":                    saGroups,
		"a padded system group":                        {Username: "oidc:alice", Groups: []string{" system:masters"}},
	}
	for name, who := range cases {
		t.Run(name, func(t *testing.T) {
			sc := newSARCluster(t)
			g, err := sc.checker(t, Options{}).Check(t.Context(), who, getDeployment("team-a", "web"))
			requireDenial(t, g, err, CodeUnauthenticated)
			if n := len(sc.Actions()); n != 0 {
				t.Fatalf("cluster saw %d requests for a refused identity, want 0", n)
			}
		})
	}
}

// TestInClusterClaimsSpellingTheReaderAreAPerson is the case identity
// mapping could produce from claims that name the portal's ServiceAccount
// with its exact groups: the identity was not built by
// ServiceAccountIdentity, so it is a person, refused with no request, and
// logged, even after the reader holds a cached allow for the same read.
func TestInClusterClaimsSpellingTheReaderAreAPerson(t *testing.T) {
	sc := newSARCluster(t)
	log, buf := accessLogger()
	c := sc.checker(t, Options{AccessLog: log})
	req := Attributes{Verb: "list", Resource: deployments}
	if _, err := c.Check(t.Context(), portalReader(t), req); err != nil {
		t.Fatalf("Check(reader) = %v", err)
	}
	before := len(sc.Actions())

	// The groups in another order, as claims might carry them.
	spelled := Identity{Username: portalSA, Groups: []string{"system:authenticated", "system:serviceaccounts:opm-portal", "system:serviceaccounts"}}
	g, err := c.Check(t.Context(), spelled, req)
	requireDenial(t, g, err, CodeUnauthenticated)
	if after := len(sc.Actions()); after != before {
		t.Fatalf("cluster saw %d requests for claims spelling the reader, want 0", after-before)
	}
	if err := (Grant{}).Covers(spelled, req); err == nil {
		t.Fatal("a zero grant covers the spelled identity")
	}
	rg, err := c.Check(t.Context(), portalReader(t), req)
	if err != nil {
		t.Fatalf("Check(reader) = %v", err)
	}
	if err := rg.Covers(spelled, req); err == nil {
		t.Fatal("the reader's grant covers claims spelling the reader")
	}

	want := accessLine{Msg: "access", User: portalSA, Verb: "list", Group: "apps", Version: "v1", Resource: "deployments", Decision: "deny", Code: string(CodeUnauthenticated)}
	if lines := accessLines(t, buf); len(lines) != 1 || lines[0] != want {
		t.Fatalf("access log = %+v, want only %+v (no reader line)", lines, want)
	}
}

// TestInClusterEmptyClaimsNeverBecomeThePortal is the regression test for
// CVE-2026-23990's shape: a sign-in whose claims map to no username fell
// through to the server's own identity. Here such an identity is refused
// before any call, even after the portal's ServiceAccount holds a cached
// allow for the same read.
func TestInClusterEmptyClaimsNeverBecomeThePortal(t *testing.T) {
	sc := newSARCluster(t)
	c := sc.checker(t, Options{})
	req := Attributes{Verb: "list", Resource: deployments}
	if _, err := c.Check(t.Context(), portalReader(t), req); err != nil {
		t.Fatalf("Check(reader) = %v", err)
	}
	before := len(sc.Actions())

	empties := []Identity{
		{},
		{Username: "", UID: "a1b2", Groups: []string{"oidc:team-a", "system:authenticated"}, Extra: map[string][]string{"k": {"v"}}},
		{Username: "   ", Groups: portalReader(t).Groups},
		{Username: "system:anonymous", Groups: []string{"system:unauthenticated"}},
	}
	for _, who := range empties {
		g, err := c.Check(t.Context(), who, req)
		requireDenial(t, g, err, CodeUnauthenticated)
	}
	if after := len(sc.Actions()); after != before {
		t.Fatalf("cluster saw %d requests for empty identities, want 0", after-before)
	}
	for _, s := range sc.reviews() {
		if s.User != portalSA {
			t.Errorf("a review named %q", s.User)
		}
	}
}

// TestInClusterSendsNoSelfReview records every request the fake cluster
// receives across every route and outcome: each is a create of
// subjectaccessreviews, never a self review (0030:D6:R9).
func TestInClusterSendsNoSelfReview(t *testing.T) {
	sc := newSARCluster(t)
	sc.answer = func(s authorizationv1.SubjectAccessReviewSpec) authorizationv1.SubjectAccessReviewStatus {
		return authorizationv1.SubjectAccessReviewStatus{Allowed: s.ResourceAttributes.Namespace == "team-a"}
	}
	c := sc.checker(t, Options{})
	reader := portalReader(t)
	for _, who := range []Identity{person, reader, {}, {Username: "system:admin"}} {
		for _, ns := range []string{"team-a", "team-b"} {
			_, _ = c.Check(t.Context(), who, Attributes{Verb: "get", Resource: pods, Subresource: "log", Namespace: ns, Name: "web-0"})
			_, _ = c.Check(t.Context(), who, Attributes{Verb: "watch", Resource: deployments, Namespace: ns})
		}
	}
	actions := sc.Actions()
	if len(actions) == 0 {
		t.Fatal("no request recorded")
	}
	for _, a := range actions {
		if a.GetVerb() != "create" || a.GetResource().Group != "authorization.k8s.io" || a.GetResource().Resource != "subjectaccessreviews" {
			t.Errorf("in-cluster mode sent %s %s/%s, want only create subjectaccessreviews", a.GetVerb(), a.GetResource().Group, a.GetResource().Resource)
		}
	}
}
