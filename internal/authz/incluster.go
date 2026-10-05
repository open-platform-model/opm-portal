package authz

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	authorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

const (
	systemPrefix         = "system:"
	serviceAccountPrefix = "system:serviceaccount:"
	authenticatedGroup   = "system:authenticated"
)

// NewInCluster returns a Checker for in-cluster mode. It asks the cluster
// with SubjectAccessReviews sent through reviews, naming the identity being
// checked, so every answer is about the signed-in person (0030:D6:R1) and
// never about the portal's own ServiceAccount. It holds no other client: it
// sends no self review and reads no object (0030:D6:R9).
//
// reader is the portal's ServiceAccount, the identity the read model reads
// as, exactly as ServiceAccountIdentity returns it. Its own grants come from reviews naming it,
// renewed when the cached decision expires. Every other identity is a
// person, and a person with a system username or a system group other than
// system:authenticated is refused before a review is sent, so a mapping
// mistake cannot borrow the ServiceAccount's access or a privileged group
// (0030:D6:R3).
func NewInCluster(reviews authorizationv1client.SubjectAccessReviewInterface, reader Identity, opts Options) (*Checker, error) {
	if reviews == nil {
		return nil, errors.New("in-cluster authorizer: no access review client")
	}
	want, err := ServiceAccountIdentity(reader.Username)
	if err != nil {
		return nil, fmt.Errorf("in-cluster authorizer: the reader %w", err)
	}
	if reader.key() != want.key() {
		// The reader is reviewed with exactly the groups the API server
		// gives the ServiceAccount, never with more.
		return nil, errors.New("in-cluster authorizer: the reader is not the identity ServiceAccountIdentity returns")
	}
	return newChecker(&subjectReviewer{reviews: reviews, readerKey: reader.key()}, opts), nil
}

// subjectReviewer decides through SubjectAccessReviews.
type subjectReviewer struct {
	reviews   authorizationv1client.SubjectAccessReviewInterface
	readerKey string
}

func (s *subjectReviewer) decide(ctx context.Context, who Identity, req Attributes) (bool, error) {
	if who.key() != s.readerKey && !isPerson(who) {
		// A person never carries a system name or group; one that does
		// came through a broken mapping and is refused, not reviewed.
		return false, &DenialError{Code: CodeUnauthenticated, Attributes: req}
	}
	review := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: resourceAttributes(req),
			User:               who.Username,
			UID:                who.UID,
			Groups:             who.clone().Groups,
			Extra:              extraValues(who.Extra),
		},
	}
	resp, err := s.reviews.Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("creating subject access review: %w", err)
	}
	if resp == nil {
		return false, errors.New("creating subject access review: empty response")
	}
	return verdict(resp.Status)
}

// isPerson reports whether who may be reviewed as a signed-in person: no
// system username, and no system group but system:authenticated.
func isPerson(who Identity) bool {
	if strings.HasPrefix(strings.TrimSpace(who.Username), systemPrefix) {
		return false
	}
	for _, g := range who.Groups {
		if strings.HasPrefix(strings.TrimSpace(g), systemPrefix) && g != authenticatedGroup {
			return false
		}
	}
	return true
}

func extraValues(extra map[string][]string) map[string]authorizationv1.ExtraValue {
	if len(extra) == 0 {
		return nil
	}
	out := make(map[string]authorizationv1.ExtraValue, len(extra))
	for k, v := range extra {
		out[k] = authorizationv1.ExtraValue(append([]string(nil), v...))
	}
	return out
}

// ServiceAccountIdentity returns the identity the API server gives the
// ServiceAccount username system:serviceaccount:<namespace>:<name>, with the
// groups it is authenticated with, so a review naming it asks the question
// the API server answers for the ServiceAccount's own reads.
func ServiceAccountIdentity(username string) (Identity, error) {
	ns, err := serviceAccountNamespace(username)
	if err != nil {
		return Identity{}, err
	}
	return Identity{
		Username: username,
		Groups:   []string{"system:serviceaccounts", "system:serviceaccounts:" + ns, authenticatedGroup},
	}, nil
}

// serviceAccountNamespace parses system:serviceaccount:<namespace>:<name>
// and returns the namespace.
func serviceAccountNamespace(username string) (string, error) {
	rest, ok := strings.CutPrefix(username, serviceAccountPrefix)
	if !ok {
		return "", fmt.Errorf("%q is not a ServiceAccount username", username)
	}
	ns, name, ok := strings.Cut(rest, ":")
	if !ok || ns == "" || name == "" || strings.Contains(name, ":") ||
		strings.TrimSpace(ns) != ns || strings.TrimSpace(name) != name {
		return "", fmt.Errorf("%q is not a ServiceAccount username", username)
	}
	return ns, nil
}

// ServiceAccountFromToken returns the ServiceAccount username a projected
// ServiceAccount token names in its sub claim. The token is the portal's
// own, mounted into its Pod, and is read only to name the portal, so it is
// not verified; the API server still authenticates every read the portal
// makes with it. The error never carries the token.
func ServiceAccountFromToken(token []byte) (string, error) {
	parts := strings.Split(strings.TrimSpace(string(token)), ".")
	if len(parts) != 3 {
		return "", errors.New("reading the ServiceAccount token: not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errors.New("reading the ServiceAccount token: the payload is not base64url")
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", errors.New("reading the ServiceAccount token: the payload is not JSON")
	}
	if _, err := serviceAccountNamespace(claims.Sub); err != nil {
		return "", fmt.Errorf("reading the ServiceAccount token: the subject %w", err)
	}
	return claims.Sub, nil
}
