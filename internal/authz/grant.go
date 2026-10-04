package authz

import (
	"errors"
	"fmt"
	"time"
)

// ErrNoGrant is returned by Grant.Covers when the grant is the zero value,
// was issued to a different identity or for a different read, or has
// expired. A read path that gets it calls Check again before reading.
var ErrNoGrant = errors.New("no authorization grant covers this read")

// Grant is proof that Check allowed one identity one read, for as long as
// the decision behind it lasts. Read paths take a Grant and call Covers, with
// the caller's identity and the read they are about to make, before every
// read; Covers is the enforcement. Grant's fields are unexported, so only
// this package can fill one in, and only Checker.Check does (0030:D7). The
// zero Grant is invalid and covers nothing.
type Grant struct {
	sealed *grantData
}

type grantData struct {
	identity Identity
	idKey    string
	attrs    Attributes
	expires  time.Time
	now      func() time.Time
}

// issue is the package's only constructor of a non-zero Grant. It is called
// from Checker.Check alone, after an allow. The grant stops covering reads
// at expires, as told by now.
func issue(who Identity, req Attributes, expires time.Time, now func() time.Time) Grant {
	return Grant{sealed: &grantData{
		identity: who.clone(),
		idKey:    who.key(),
		attrs:    req,
		expires:  expires,
		now:      now,
	}}
}

// Valid reports whether g was issued by Check. An expired grant is still
// Valid; Covers refuses it.
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

// Expires returns when the grant stops covering reads, or the zero time for
// an invalid grant.
func (g Grant) Expires() time.Time {
	if g.sealed == nil {
		return time.Time{}
	}
	return g.sealed.expires
}

// Covers returns nil when g permits who to make the read req now. who must
// be the identity the grant was issued to (compared on its canonical key, so
// group and extra-value order does not matter), and the grant must not have
// expired. Verb, resource and subresource must match exactly. A grant with
// an empty namespace covers every namespace, and one with an empty name
// covers every object, as RBAC answers an access review with those fields
// empty. Anything else, and the zero Grant, returns an error wrapping
// ErrNoGrant. The error never names the identity or the object.
func (g Grant) Covers(who Identity, req Attributes) error {
	if g.sealed == nil {
		return ErrNoGrant
	}
	if who.key() != g.sealed.idKey {
		// A grant held in shared state must not lend one caller's access
		// to another (0030:D7).
		return fmt.Errorf("grant was issued to another identity: %w", ErrNoGrant)
	}
	if !g.sealed.now().Before(g.sealed.expires) {
		return fmt.Errorf("grant has expired: %w", ErrNoGrant)
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
