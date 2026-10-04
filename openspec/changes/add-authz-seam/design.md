## Context

The repo holds only the version binary (see proposal.md, Why). The portal's design puts one
authorization package between identity and every read: `auth -> authz -> readmodel -> ...`
(Principle II). Milestone 1 authorizes with SelfSubjectAccessReviews under the user's kubeconfig;
milestone 2 (change `add-sar-authorizer`) will plug a SubjectAccessReview backend into the same
seam. The decisions it implements are 0030:D7 (authorize before lookup), the empty-identity and
deny-on-error rules of 0030:D6, and the kubeconfig boundary of 0030:D5.

## Goals / Non-Goals

**Goals:**

- A read path cannot read without a proof of an allow, and the type system, not review, enforces
  it.
- Every refusal the portal can decide alone (empty identity, write verb, Secret, malformed
  request) happens before a cluster call, in one place that every backend shares.
- One backend (local SSAR) with the cache and deny-on-error behaviour M2 will reuse.

**Non-Goals:**

- Resolving who the kubeconfig is at startup. `NewLocal` takes the identity from its caller;
  `add-local-mode` decides how to obtain it (see Open Questions).
- List filtering, per-item access states and the inventory reach check (0030:D7:R2-R4).
- An HTTP surface, an access log, identity mapping.

## Decisions

### Package shape

```go
package authz

type Identity struct {
    Username string
    UID      string
    Groups   []string
    Extra    map[string][]string
}
func (Identity) Authenticated() bool

type Attributes struct {
    Verb        string
    Resource    schema.GroupVersionResource
    Subresource string
    Namespace   string // "" = all namespaces or cluster-scoped
    Name        string // "" = all objects
}

type Authorizer interface {
    Check(ctx context.Context, who Identity, req Attributes) (Grant, error) // error is *Denial
}

type Grant struct{ sealed *grantData }      // unexported field; zero value is invalid
func (Grant) Valid() bool
func (Grant) Covers(req Attributes) error   // wraps ErrNoGrant when it does not
func (Grant) Identity() Identity            // deep copy
func (Grant) Attributes() Attributes

type Code string // unauthenticated | forbidden | invalid | unavailable
type Denial struct { Code Code; Attributes Attributes; cause error }

type Options struct{ Timeout, TTL time.Duration; MaxEntries int }
type Checker struct{ ... }                  // the one Authorizer implementation
func NewLocal(reviews authorizationv1client.SelfSubjectAccessReviewInterface,
    self Identity, opts Options) (*Checker, error)
```

`Checker.Check` runs, in order: identity guard, attribute guard, cache lookup, backend
(`decider`, unexported) under a timeout, then `issue` on an allow. Backends implement an
unexported `decide(ctx, who, req) (bool, error)`; they cannot construct a `Grant` and every error
they return becomes a denial.

### Research & Decisions

#### How to make a Grant unforgeable

**Context**: 0030:D7 asks that no read path can skip authorization. In Go another package can
always write the zero value of an exported struct type, so "cannot construct" has to mean "cannot
construct a valid one".
**Explored**: a sealed interface (an interface with an unexported method), a struct with
unexported fields, a pointer type with a nil check. A spike compiled `testdata/forge` against the
struct form: Go 1.26 reports `cannot refer to unexported field sealed in struct literal of type
authz.Grant` and `undefined: authz.issue`.
**Options considered**:
1. Sealed interface - another package can embed the interface in its own struct and satisfy it
   with a nil embedded value; the forgery compiles.
2. Struct with one unexported pointer field - no other package can fill it in; the zero value is
   detectable (`sealed == nil`).
**Decision**: option 2, with three tests: reflection asserts every field is unexported; a test
builds `testdata/forge` and requires the compiler to refuse both forgeries; an AST scan of the
whole module refuses any `authz.Grant` literal, `new(authz.Grant)` or type declared from it outside
the package, and inside it refuses a filled-in literal outside `issue` and a call to `issue`
outside `(*Checker).Check`. The scanner has its own test that it flags each pattern.
**Rationale**: the compiler is the proof; the scan keeps the package itself honest, where the
compiler cannot help.

#### Where the empty-identity guard sits

**Context**: 0030:D6:R2 (the Flux Operator CVE-2026-23990 class: empty claims fell through to the
server's own identity). Each backend could check, and one would forget.
**Options considered**:
1. Each backend checks the identity - duplicated, easy to miss in M2.
2. Exported `Authorizer` implementations are all `*Checker`, which checks before any backend runs.
**Decision**: option 2. Backends are unexported `decider`s; the only exported constructor,
`NewLocal`, returns a `*Checker`. An identity is unauthenticated when its trimmed username is empty
or `system:anonymous`, regardless of groups, UID or extra. `NewLocal` also refuses an
unauthenticated `self`. A nil or zero `*Checker` denies as unavailable instead of panicking or
allowing.
**Rationale**: the guard is part of the seam's type, not of a backend's diligence.
`system:anonymous` is included because an anonymous principal is the same "no one" the CVE class
exploits; local mode with an anonymous kubeconfig has nothing to act as.

#### Local backend serves one identity

**Context**: a SelfSubjectAccessReview answers for whoever the client authenticates as. If a
future wiring bug passed another principal's identity to the local Checker, the answer would be
the kubeconfig user's.
**Decision**: `NewLocal` binds the canonical key of `self`; `decide` refuses any other identity
with `CodeUnauthenticated` and sends nothing. Equality is on the canonical key (username, UID,
sorted groups, sorted extra), so group order does not matter.

#### Refusals decided without the cluster

**Decision**: verbs other than `get`, `list`, `watch` are `forbidden` (the portal is read-only,
Principle V); core `secrets` is `forbidden` whatever RBAC says (0030:D8:R1); an empty verb or
resource or a `*` in any attribute is `invalid`. None reaches a backend or the cache.
**Rationale**: these are portal policy, not RBAC questions; asking the cluster would only add a
way to get them wrong.

#### Verdict mapping

**Decision**: `Denied` wins over `Allowed`; `Allowed` grants (even with a partial
`EvaluationError`, as the API server itself would); `EvaluationError` without `Allowed` is an
error and so `unavailable`; neither flag is `forbidden`. A transport or API error, a timeout
(`Options.Timeout`, default 5 s) or a nil response is `unavailable`. `unavailable` wraps its cause
for logs; the read API will map it to its own problem code.
**Rationale**: 0030:D6:R4 and the architecture's "error, timeout or evaluationError without
allowed is a deny".

#### What a denial says

**Decision**: `Denial.Error()` is `authorization <code>: <verb> <group/resource[/sub]>[ in
namespace <ns>]` plus the cause for `unavailable`. It never names the object or the identity.
**Rationale**: 0030:D7:R1 (identical for existing and missing objects) and the security rule that
no principal or credential appears in an error.

#### Covers semantics

**Decision**: verb, group/version/resource and subresource must match exactly; a grant with an
empty namespace covers any namespace and one with an empty name covers any name, matching how RBAC
answers a review with those fields empty. Anything else wraps `ErrNoGrant`.

#### Decision cache

**Decision**: a mutex-guarded map from `identity.key() + attributes.key()` (both built from
`strconv.Quote`d fields, so no two distinct inputs share a key) to `{allowed, expires}`. TTL
default 30 s, `MaxEntries` default 4096. Only allow and forbidden outcomes from a backend are
stored; guard refusals never reach the cache and backend errors are never stored. On insert into a
full map, expired entries are dropped first; if it is still full, the decision is not cached
(correctness never depends on the cache). The clock is injectable for tests.
**Alternatives**: an LRU (more code for no V1 need); no cache (one review per read per request,
which the architecture's per-node locked state cannot afford).

### Authorization

The change reads no object. Its only cluster call is `create` on `authorization.k8s.io/v1`
`selfsubjectaccessreviews`, made with the user's own kubeconfig and never stored.

## Risks / Trade-offs

- [A deny is cached for up to 30 s, so a newly granted permission shows late] -> short TTL; the
  UI will say "as of" where it matters.
- [The module scan is a textual check and can be fooled by reflection or `unsafe`] -> the
  compiler test is the proof for ordinary code; `unsafe` is visible in review and lint.
- [The forge test shells out to `go build`] -> it skips with `-short` or without a `go` binary;
  CI runs it.

## Open Questions

- How `add-local-mode` obtains the kubeconfig identity it passes to `NewLocal`. The natural
  source is a `SelfSubjectReview` (`authentication.k8s.io/v1`), another create-only review that is
  never stored; whether it falls inside Principle V's review exception, which today names
  SubjectAccessReviews only, is for that change (and the owner) to settle. It changes nothing
  here: `NewLocal` takes the identity either way.
