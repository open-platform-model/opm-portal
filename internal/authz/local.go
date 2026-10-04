package authz

import (
	"context"
	"errors"
	"fmt"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	authorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

// NewLocal returns a Checker for local mode. It asks the cluster with
// SelfSubjectAccessReviews sent through reviews, so every answer is about the
// identity the kubeconfig authenticates as (0030:D5:R1). self is that
// identity, as the caller resolved it; the Checker serves self alone and
// refuses any other identity before a review is sent.
//
// reviews is the only client the Checker holds, so it cannot look anything
// up: it authorizes on attributes alone.
func NewLocal(reviews authorizationv1client.SelfSubjectAccessReviewInterface, self Identity, opts Options) (*Checker, error) {
	if reviews == nil {
		return nil, errors.New("local authorizer: no access review client")
	}
	if !self.Authenticated() {
		return nil, errors.New("local authorizer: the kubeconfig identity has no username")
	}
	return newChecker(&selfReviewer{reviews: reviews, self: self.key()}, opts), nil
}

// selfReviewer decides through SelfSubjectAccessReviews.
type selfReviewer struct {
	reviews authorizationv1client.SelfSubjectAccessReviewInterface
	self    string
}

func (s *selfReviewer) decide(ctx context.Context, who Identity, req Attributes) (bool, error) {
	if who.key() != s.self {
		// A self review answers for the kubeconfig's user only; answering it
		// for anyone else would lend them the kubeconfig's access.
		return false, &DenialError{Code: CodeUnauthenticated, Attributes: req}
	}
	review := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: resourceAttributes(req),
		},
	}
	resp, err := s.reviews.Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("creating self subject access review: %w", err)
	}
	if resp == nil {
		return false, errors.New("creating self subject access review: empty response")
	}
	return verdict(resp.Status)
}

func resourceAttributes(req Attributes) *authorizationv1.ResourceAttributes {
	return &authorizationv1.ResourceAttributes{
		Verb:        req.Verb,
		Group:       req.Resource.Group,
		Version:     req.Resource.Version,
		Resource:    req.Resource.Resource,
		Subresource: req.Subresource,
		Namespace:   req.Namespace,
		Name:        req.Name,
	}
}

// verdict reads an access review's status, failing closed: an explicit deny
// wins over an allow, no opinion is a deny, and an evaluation error without
// an allow is an error, so it is neither allowed nor cached (0030:D6:R4).
func verdict(st authorizationv1.SubjectAccessReviewStatus) (bool, error) {
	switch {
	case st.Denied:
		return false, nil
	case st.Allowed:
		return true, nil
	case st.EvaluationError != "":
		return false, fmt.Errorf("access review could not be evaluated: %s", st.EvaluationError)
	default:
		return false, nil
	}
}
