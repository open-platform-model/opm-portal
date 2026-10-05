package readmodel

import (
	"errors"
	"slices"
	"sync"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// sarRecorder is a fake cluster that allows every SubjectAccessReview and
// records each one.
type sarRecorder struct {
	*fake.Clientset
	mu   sync.Mutex
	seen []authorizationv1.SubjectAccessReviewSpec
}

func newSARRecorder() *sarRecorder {
	r := &sarRecorder{Clientset: fake.NewClientset()}
	r.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		r.mu.Lock()
		r.seen = append(r.seen, *review.Spec.DeepCopy())
		r.mu.Unlock()
		out := review.DeepCopy()
		out.Status.Allowed = true
		return true, out, nil
	})
	return r
}

func (r *sarRecorder) reviews() []authorizationv1.SubjectAccessReviewSpec {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.seen)
}

// TestInClusterReaderHoldsTheServiceAccountsGrant runs the read model on
// the in-cluster authorizer: Start reviews the portal's ServiceAccount by
// name before each OPM informer, the reads inside a person's view are
// reviewed as that person, and the cluster sees only SubjectAccessReviews.
func TestInClusterReaderHoldsTheServiceAccountsGrant(t *testing.T) {
	sa, err := authz.ServiceAccountIdentity("system:serviceaccount:opm-portal:opm-portal")
	if err != nil {
		t.Fatal(err)
	}
	person := authz.Identity{Username: "oidc:alice", Groups: []string{"system:authenticated"}}
	cluster := newSARRecorder()
	az, err := authz.NewInCluster(cluster.AuthorizationV1().SubjectAccessReviews(), sa, authz.Options{})
	if err != nil {
		t.Fatal(err)
	}
	e := newEnv(t, loadF1(t), allowAll, allowAll, func(c *Config) {
		c.Authorizer = az
		c.Reader = sa
	})

	readerAsked := map[string]bool{}
	for _, s := range cluster.reviews() {
		if s.User != sa.Username || !slices.Equal(s.Groups, sa.Groups) {
			t.Errorf("a startup review named %q %v, want the ServiceAccount", s.User, s.Groups)
		}
		readerAsked[s.ResourceAttributes.Verb+" "+s.ResourceAttributes.Resource] = true
	}
	for _, want := range []string{"list moduleinstances", "watch moduleinstances", "list platforms", "watch platforms"} {
		if !readerAsked[want] {
			t.Errorf("Start did not review the ServiceAccount for %s (asked %v)", want, readerAsked)
		}
	}

	g, err := az.Check(t.Context(), person, authz.Attributes{Verb: "get", Resource: platforms})
	if err != nil {
		t.Fatalf("Check(person) = %v", err)
	}
	beforeView := len(cluster.reviews())
	if _, err := e.m.Platform(t.Context(), person, g); err != nil && !errors.Is(err, ErrNotFound) {
		t.Fatalf("Platform(person) = %v", err)
	}
	if !slices.ContainsFunc(cluster.reviews()[beforeView:], func(s authorizationv1.SubjectAccessReviewSpec) bool {
		return s.User == person.Username
	}) {
		t.Error("the reads inside the person's view were not reviewed as the person")
	}
	for _, a := range cluster.Actions() {
		if a.GetVerb() != "create" || a.GetResource().Resource != "subjectaccessreviews" {
			t.Errorf("in-cluster read model sent %s %s", a.GetVerb(), a.GetResource().Resource)
		}
	}
}
