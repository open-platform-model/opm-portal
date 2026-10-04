package authz

import (
	"context"
	"errors"
	"time"
)

// Authorizer decides one read. Check returns a valid Grant, or a zero Grant
// and a *DenialError; it never returns a valid Grant together with an error.
type Authorizer interface {
	Check(ctx context.Context, who Identity, req Attributes) (Grant, error)
}

// decider asks a backend whether who may make req. An error means the
// backend could not decide; Checker turns it into a denial.
type decider interface {
	decide(ctx context.Context, who Identity, req Attributes) (allowed bool, err error)
}

// Options tune a Checker. Zero fields take their defaults.
type Options struct {
	// Timeout bounds one access review. Default 5 seconds.
	Timeout time.Duration
}

const defaultTimeout = 5 * time.Second

// Checker is the package's Authorizer. Every backend runs behind the same
// guards, in this order: identity, attributes, then the backend. Check is the
// only place a Grant is issued.
type Checker struct {
	backend decider
	timeout time.Duration
}

var _ Authorizer = (*Checker)(nil)

func newChecker(backend decider, opts Options) *Checker {
	c := &Checker{backend: backend, timeout: opts.Timeout}
	if c.timeout <= 0 {
		c.timeout = defaultTimeout
	}
	return c
}

// Check authorizes req for who. It refuses an unauthenticated identity and
// any non-read, Secret or malformed request before the backend is asked, and
// it turns every backend failure into a CodeUnavailable denial.
func (c *Checker) Check(ctx context.Context, who Identity, req Attributes) (Grant, error) {
	if !who.Authenticated() {
		// No Kubernetes call is made for an identity that names no one (0030:D6:R2).
		return Grant{}, &DenialError{Code: CodeUnauthenticated, Attributes: req}
	}
	if err := req.validate(); err != nil {
		return Grant{}, err
	}
	if c == nil || c.backend == nil {
		return Grant{}, &DenialError{Code: CodeUnavailable, Attributes: req, cause: errors.New("no authorizer configured")}
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	allowed, err := c.backend.decide(ctx, who, req)
	if err != nil {
		if d, ok := errors.AsType[*DenialError](err); ok {
			return Grant{}, d
		}
		return Grant{}, &DenialError{Code: CodeUnavailable, Attributes: req, cause: err}
	}
	if !allowed {
		return Grant{}, &DenialError{Code: CodeForbidden, Attributes: req}
	}
	return issue(who, req), nil
}
