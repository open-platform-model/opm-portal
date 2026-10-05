package authz

import (
	"context"
	"errors"
	"log/slog"
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
	// TTL is how long an allow or deny is reused for the same identity
	// and request. Default 30 seconds.
	TTL time.Duration
	// MaxEntries bounds the decision cache. Default 4096.
	MaxEntries int
	// AccessLog, when set, receives one line per decision about a person,
	// allowed or denied, cached or not (0030:D6:R7). Nil logs nothing.
	AccessLog *slog.Logger
}

const defaultTimeout = 5 * time.Second

// Checker is the package's Authorizer. Every backend runs behind the same
// guards, in this order: identity, attributes, the decision cache, then the
// backend. Check is the only place a Grant is issued.
type Checker struct {
	backend decider
	timeout time.Duration
	cache   *decisionCache
	// accessLog records decisions about people; quiet is the key of the
	// identity it leaves out, the in-cluster reader, which is no person.
	accessLog *slog.Logger
	quiet     string
}

var _ Authorizer = (*Checker)(nil)

func newChecker(backend decider, opts Options) *Checker {
	c := &Checker{backend: backend, timeout: opts.Timeout, cache: newDecisionCache(opts.TTL, opts.MaxEntries), accessLog: opts.AccessLog}
	if c.timeout <= 0 {
		c.timeout = defaultTimeout
	}
	return c
}

// Check authorizes req for who. It refuses an unauthenticated identity and
// any non-read, Secret or malformed request before the backend is asked, and
// it turns every backend failure into a CodeUnavailable denial.
func (c *Checker) Check(ctx context.Context, who Identity, req Attributes) (Grant, error) {
	d, cached, err := c.check(ctx, who, req)
	c.logAccess(ctx, who, req, cached, err)
	if err != nil {
		return Grant{}, err
	}
	// The grant lasts as long as the decision behind it, so a held grant
	// sees a revocation within one TTL, like a new Check does.
	return issue(who, req, d.expires, c.cache.clock()), nil
}

// check decides req for who and reports whether the decision came from the
// cache. It returns an allowed decision or a *DenialError, never both.
func (c *Checker) check(ctx context.Context, who Identity, req Attributes) (cachedDecision, bool, error) {
	if !who.Authenticated() {
		// No Kubernetes call is made for an identity that names no one (0030:D6:R2).
		return cachedDecision{}, false, &DenialError{Code: CodeUnauthenticated, Attributes: req}
	}
	if err := req.validate(); err != nil {
		return cachedDecision{}, false, err
	}
	if c == nil || c.backend == nil {
		return cachedDecision{}, false, &DenialError{Code: CodeUnavailable, Attributes: req, cause: errors.New("no authorizer configured")}
	}

	key := cacheKey(who, req)
	d, cached := c.cache.get(key)
	if !cached {
		allowed, err := c.decide(ctx, who, req)
		if err != nil {
			// A failure is never cached: the next check asks again (0030:D6:R4).
			return cachedDecision{}, false, err
		}
		d = c.cache.put(key, allowed)
	}
	if !d.allowed {
		return cachedDecision{}, cached, &DenialError{Code: CodeForbidden, Attributes: req}
	}
	return d, cached, nil
}

// logAccess writes one access log line for a decision about a person: who,
// which read, the decision and its code. It never writes groups, extra
// values or a denial's cause, whose text the portal does not control, and
// attributes carry no values.
func (c *Checker) logAccess(ctx context.Context, who Identity, req Attributes, cached bool, err error) {
	if c == nil || c.accessLog == nil || (c.quiet != "" && who.key() == c.quiet) {
		return
	}
	decision, code := "allow", ""
	if err != nil {
		decision, code = "deny", string(CodeUnavailable)
		if d, ok := errors.AsType[*DenialError](err); ok {
			code = string(d.Code)
		}
	}
	c.accessLog.LogAttrs(ctx, slog.LevelInfo, "access",
		slog.String("user", who.Username),
		slog.String("verb", req.Verb),
		slog.String("group", req.Resource.Group),
		slog.String("version", req.Resource.Version),
		slog.String("resource", req.Resource.Resource),
		slog.String("subresource", req.Subresource),
		slog.String("namespace", req.Namespace),
		slog.String("name", req.Name),
		slog.String("decision", decision),
		slog.String("code", code),
		slog.Bool("cached", cached),
	)
}

// decide asks the backend under the review timeout and turns any failure
// into a denial.
func (c *Checker) decide(ctx context.Context, who Identity, req Attributes) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	allowed, err := c.backend.decide(ctx, who, req)
	if err != nil {
		if d, ok := errors.AsType[*DenialError](err); ok {
			return false, d
		}
		return false, &DenialError{Code: CodeUnavailable, Attributes: req, cause: err}
	}
	return allowed, nil
}
