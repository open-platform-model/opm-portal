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

// Covers reports whether a, the read a decision was made for, covers req
// by scope alone, apart from identity and expiry. Verb, resource and
// subresource must match exactly. An empty namespace covers every namespace,
// and an empty name covers every object, as RBAC answers an access review
// with those fields empty. Grant.Covers applies this test, and so does any
// caller deciding whether a read falls within one already granted.
func (a Attributes) Covers(req Attributes) bool {
	return a.Verb == req.Verb &&
		a.Resource == req.Resource &&
		a.Subresource == req.Subresource &&
		(a.Namespace == "" || a.Namespace == req.Namespace) &&
		(a.Name == "" || a.Name == req.Name)
}

// readVerbs are the only verbs the portal ever asks for.
var readVerbs = map[string]bool{"get": true, "list": true, "watch": true}

// readSubresources are the only subresources the portal ever reads: the
// object itself, its status, and a Pod's log (portal:D10, portal:D11). Others
// are refused because a get on them is not a read: pods/exec, pods/attach
// and pods/portforward, and the proxy subresources (nodes/proxy is an exec
// path), are authorized as get when upgraded to a stream.
var readSubresources = map[string]bool{"": true, "status": true, "log": true}

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
	case !readSubresources[a.Subresource]:
		// A get on exec, attach, portforward or proxy opens a stream into
		// the workload or node; the portal never asks for one.
		return &DenialError{Code: CodeForbidden, Attributes: a}
	case a.Resource.Group == "" && a.Resource.Resource == "secrets":
		// The portal never reads Secret data, whatever the caller's RBAC (portal:D8:R1).
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
// from it reads the same for an existing and a missing object (portal:D7:R1).
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
