package authz

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Attributes describe one Kubernetes read: the same fields a
// SubjectAccessReview's resource attributes carry. An empty Namespace means
// every namespace (or a cluster-scoped resource); an empty Name means every
// object.
type Attributes struct {
	Verb        string
	Resource    schema.GroupVersionResource
	Subresource string
	Namespace   string
	Name        string
}

// readVerbs are the only verbs the portal ever asks for.
var readVerbs = map[string]bool{"get": true, "list": true, "watch": true}

const wildcard = "*"

// validate refuses attributes the portal must never authorize, before any
// review is sent.
func (a Attributes) validate() error {
	switch {
	case a.Verb == "" || a.Resource.Resource == "":
		return &DenialError{Code: CodeInvalid, Attributes: a}
	case containsWildcard(a.Verb, a.Resource.Group, a.Resource.Version, a.Resource.Resource,
		a.Subresource, a.Namespace, a.Name):
		return &DenialError{Code: CodeInvalid, Attributes: a}
	case !readVerbs[a.Verb]:
		// The portal is read-only: no grant is ever issued for a write verb.
		return &DenialError{Code: CodeForbidden, Attributes: a}
	case a.Resource.Group == "" && a.Resource.Resource == "secrets":
		// The portal never reads Secret data, whatever the caller's RBAC (0030:D8:R1).
		return &DenialError{Code: CodeForbidden, Attributes: a}
	}
	return nil
}

func containsWildcard(fields ...string) bool {
	for _, f := range fields {
		if strings.Contains(f, wildcard) {
			return true
		}
	}
	return false
}

// key returns a canonical, unambiguous encoding of a.
func (a Attributes) key() string {
	parts := []string{a.Verb, a.Resource.Group, a.Resource.Version, a.Resource.Resource,
		a.Subresource, a.Namespace, a.Name}
	for i, p := range parts {
		parts[i] = strconv.Quote(p)
	}
	return strings.Join(parts, "/")
}

// String renders the request without its object name, so a message built
// from it reads the same for an existing and a missing object (0030:D7:R1).
func (a Attributes) String() string {
	resource := a.Resource.Resource
	if a.Resource.Group != "" {
		resource = a.Resource.Group + "/" + resource
	}
	if a.Subresource != "" {
		resource += "/" + a.Subresource
	}
	if a.Namespace == "" {
		return fmt.Sprintf("%s %s", a.Verb, resource)
	}
	return fmt.Sprintf("%s %s in namespace %s", a.Verb, resource, a.Namespace)
}
