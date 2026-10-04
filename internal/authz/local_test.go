package authz

import (
	"errors"
	"slices"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// reviewCluster is a fake cluster whose SelfSubjectAccessReview answers come
// from status, or fail with err, and which records every review it sees.
type reviewCluster struct {
	*fake.Clientset
	status  authorizationv1.SubjectAccessReviewStatus
	err     error
	nilResp bool
	seen    []authorizationv1.ResourceAttributes
}

func newReviewCluster(t *testing.T, objects ...runtime.Object) *reviewCluster {
	t.Helper()
	rc := &reviewCluster{Clientset: fake.NewClientset(objects...)}
	rc.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
		rc.seen = append(rc.seen, *review.Spec.ResourceAttributes)
		if rc.err != nil {
			return true, nil, rc.err
		}
		if rc.nilResp {
			return true, nil, nil
		}
		out := review.DeepCopy()
		out.Status = rc.status
		return true, out, nil
	})
	return rc
}

func (rc *reviewCluster) checker(t *testing.T, self Identity) *Checker {
	t.Helper()
	c, err := NewLocal(rc.AuthorizationV1().SelfSubjectAccessReviews(), self, Options{})
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	return c
}

func TestNewLocalRefusesWhatCannotBeServed(t *testing.T) {
	reviews := fake.NewClientset().AuthorizationV1().SelfSubjectAccessReviews()
	if _, err := NewLocal(nil, alice, Options{}); err == nil {
		t.Error("NewLocal accepted no review client")
	}
	for _, self := range []Identity{{}, {Username: " "}, {Username: "system:anonymous"}, {Groups: []string{"system:masters"}}} {
		if _, err := NewLocal(reviews, self, Options{}); err == nil {
			t.Errorf("NewLocal accepted unauthenticated identity %+v", self)
		}
	}
}

func TestLocalSendsTheExactAttributes(t *testing.T) {
	rc := newReviewCluster(t)
	rc.status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}
	req := Attributes{Verb: "get", Resource: pods, Subresource: "log", Namespace: "team-a", Name: "web-0"}
	g, err := rc.checker(t, alice).Check(t.Context(), alice, req)
	if err != nil || !g.Valid() {
		t.Fatalf("Check = %v, %v; want a grant", g, err)
	}
	want := authorizationv1.ResourceAttributes{Verb: "get", Version: "v1", Resource: "pods", Subresource: "log", Namespace: "team-a", Name: "web-0"}
	if len(rc.seen) != 1 || rc.seen[0] != want {
		t.Fatalf("reviews sent = %+v, want one %+v", rc.seen, want)
	}
}

func TestLocalVerdicts(t *testing.T) {
	cases := []struct {
		name    string
		status  authorizationv1.SubjectAccessReviewStatus
		err     error
		nilResp bool
		want    Code // empty: allowed
	}{
		{"allowed", authorizationv1.SubjectAccessReviewStatus{Allowed: true}, nil, false, ""},
		{"allowed despite a partial evaluation error", authorizationv1.SubjectAccessReviewStatus{Allowed: true, EvaluationError: "webhook timeout"}, nil, false, ""},
		{"denied", authorizationv1.SubjectAccessReviewStatus{Denied: true}, nil, false, CodeForbidden},
		{"no opinion", authorizationv1.SubjectAccessReviewStatus{}, nil, false, CodeForbidden},
		{"deny wins over allow", authorizationv1.SubjectAccessReviewStatus{Allowed: true, Denied: true}, nil, false, CodeForbidden},
		{"evaluation error without allow", authorizationv1.SubjectAccessReviewStatus{EvaluationError: "webhook timeout"}, nil, false, CodeUnavailable},
		{"review forbidden by the API", authorizationv1.SubjectAccessReviewStatus{}, apierrors.NewForbidden(authorizationv1.Resource("selfsubjectaccessreviews"), "", errors.New("no")), false, CodeUnavailable},
		{"API server error", authorizationv1.SubjectAccessReviewStatus{Allowed: true}, apierrors.NewInternalError(errors.New("etcd down")), false, CodeUnavailable},
		{"API timeout", authorizationv1.SubjectAccessReviewStatus{Allowed: true}, apierrors.NewTimeoutError("slow", 1), false, CodeUnavailable},
		{"empty response is no opinion", authorizationv1.SubjectAccessReviewStatus{Allowed: true}, nil, true, CodeForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rc := newReviewCluster(t)
			rc.status, rc.err, rc.nilResp = tc.status, tc.err, tc.nilResp
			g, err := rc.checker(t, alice).Check(t.Context(), alice, getDeployment("team-a", "web"))
			if tc.want == "" {
				if err != nil || !g.Valid() {
					t.Fatalf("Check = %v, %v; want a grant", g, err)
				}
				return
			}
			requireDenial(t, g, err, tc.want)
		})
	}
}

// TestUnavailableDenialHidesItsCause: an evaluation error or a transport
// error can name principals, roles or the API server, so the denial's text
// leaves it out; the cause stays reachable through Unwrap for logs.
func TestUnavailableDenialHidesItsCause(t *testing.T) {
	const evalErr = "webhook: user mallory bound to role cluster-admin"
	rc := newReviewCluster(t)
	rc.status = authorizationv1.SubjectAccessReviewStatus{EvaluationError: evalErr}
	g, err := rc.checker(t, alice).Check(t.Context(), alice, getDeployment("team-a", "web"))
	d := requireDenial(t, g, err, CodeUnavailable)
	for _, leak := range []string{"mallory", "cluster-admin", "webhook"} {
		if strings.Contains(d.Error(), leak) {
			t.Errorf("denial %q carries %q from the evaluation error", d, leak)
		}
	}
	if cause := errors.Unwrap(d); cause == nil || !strings.Contains(cause.Error(), "mallory") {
		t.Errorf("Unwrap = %v, want the evaluation error for logs", cause)
	}
}

// TestLocalServesOnlyItsOwnIdentity: a self review answers for the
// kubeconfig's user, so any other identity is refused without a review.
func TestLocalServesOnlyItsOwnIdentity(t *testing.T) {
	rc := newReviewCluster(t)
	rc.status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}
	c := rc.checker(t, alice)
	others := []Identity{
		{Username: "bob"},
		{Username: "alice"},
		{Username: "alice", Groups: []string{"dev", "system:authenticated", "system:masters"}},
	}
	for _, who := range others {
		g, err := c.Check(t.Context(), who, getDeployment("team-a", "web"))
		requireDenial(t, g, err, CodeUnauthenticated)
	}
	if len(rc.Actions()) != 0 {
		t.Fatalf("API calls for a foreign identity: %v", rc.Actions())
	}
	reordered := Identity{Username: "alice", Groups: []string{"system:authenticated", "dev"}}
	if _, err := c.Check(t.Context(), reordered, getDeployment("team-a", "web")); err != nil {
		t.Fatalf("the same identity with its groups reordered was refused: %v", err)
	}
}

// TestLocalEmptyIdentityMakesNoCall is the empty-identity regression test
// (Flux Operator CVE-2026-23990 class) against the real backend: the fake
// cluster records no request at all.
func TestLocalEmptyIdentityMakesNoCall(t *testing.T) {
	rc := newReviewCluster(t)
	rc.status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}
	c := rc.checker(t, alice)
	for _, who := range []Identity{{}, {Groups: []string{"system:masters"}}, {Username: "system:anonymous"}} {
		g, err := c.Check(t.Context(), who, getDeployment("team-a", "web"))
		requireDenial(t, g, err, CodeUnauthenticated)
	}
	if len(rc.Actions()) != 0 {
		t.Fatalf("API calls for an empty identity: %v", rc.Actions())
	}
}

// TestLocalAuthorizesBeforeLookup: a denied caller gets the same refusal for
// an object that exists and one that does not, and the authorizer never
// reads the object.
func TestLocalAuthorizesBeforeLookup(t *testing.T) {
	existing := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "exists"}}
	rc := newReviewCluster(t, existing)
	c := rc.checker(t, alice)
	_, errExists := c.Check(t.Context(), alice, getDeployment("team-a", "exists"))
	_, errMissing := c.Check(t.Context(), alice, getDeployment("team-a", "missing"))
	dExists := requireDenial(t, Grant{}, errExists, CodeForbidden)
	dMissing := requireDenial(t, Grant{}, errMissing, CodeForbidden)
	if dExists.Error() != dMissing.Error() || dExists.Code != dMissing.Code {
		t.Fatalf("denials differ:\n%v\n%v", dExists, dMissing)
	}
	for _, a := range rc.Actions() {
		if a.GetVerb() != "create" || a.GetResource().Resource != "selfsubjectaccessreviews" {
			t.Errorf("authorizer made a lookup: %s %s", a.GetVerb(), a.GetResource())
		}
	}
	names := []string{rc.seen[0].Name, rc.seen[1].Name}
	if !slices.Equal(names, []string{"exists", "missing"}) {
		t.Errorf("reviews named %v", names)
	}
}
