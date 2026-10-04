package authz

import (
	"errors"
	"fmt"
)

// ErrNoGrant is returned by Grant.Covers when the grant is the zero value or
// was issued for a different read.
var ErrNoGrant = errors.New("no authorization grant covers this read")

// Grant is proof that Check allowed one identity one read. Read paths take a
// Grant and call Covers before reading. Its fields are unexported, so only
// this package can fill one in, and only Checker.Check does (0030:D7). The
// zero Grant is invalid and covers nothing.
type Grant struct {
	sealed *grantData
}

type grantData struct {
	identity Identity
	attrs    Attributes
}

// issue is the package's only constructor of a non-zero Grant. It is called
// from Checker.Check alone, after an allow.
func issue(who Identity, req Attributes) Grant {
	return Grant{sealed: &grantData{identity: who.clone(), attrs: req}}
}

// Valid reports whether g was issued by Check.
func (g Grant) Valid() bool { return g.sealed != nil }

// Identity returns a copy of the identity the grant was issued for, or the
// zero Identity for an invalid grant.
func (g Grant) Identity() Identity {
	if g.sealed == nil {
		return Identity{}
	}
	return g.sealed.identity.clone()
}

// Attributes returns the read the grant was issued for, or the zero
// Attributes for an invalid grant.
func (g Grant) Attributes() Attributes {
	if g.sealed == nil {
		return Attributes{}
	}
	return g.sealed.attrs
}

// Covers returns nil when g permits the read req. Verb, resource and
// subresource must match exactly. A grant with an empty namespace covers
// every namespace, and one with an empty name covers every object, as RBAC
// answers an access review with those fields empty. Anything else, and the
// zero Grant, returns an error wrapping ErrNoGrant.
func (g Grant) Covers(req Attributes) error {
	if g.sealed == nil {
		return ErrNoGrant
	}
	have := g.sealed.attrs
	switch {
	case have.Verb != req.Verb,
		have.Resource != req.Resource,
		have.Subresource != req.Subresource,
		have.Namespace != "" && have.Namespace != req.Namespace,
		have.Name != "" && have.Name != req.Name:
		return fmt.Errorf("grant for %s does not cover %s: %w", have, req, ErrNoGrant)
	}
	return nil
}
