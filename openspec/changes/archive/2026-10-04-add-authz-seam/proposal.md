## Why

Every later read path (read model, API, streams, logs) must take its authorization decision
before it looks anything up, and a shared cache serving many users is exactly where a
cross-tenant leak happens (0030:D7). If the seam arrives after the first read path, each handler
has to be audited for it; if it arrives first, every read path takes a `Grant` and calls
`Covers` with the caller and the read before reading, and a grant covers only the caller it was
issued to, only its own read, and only while its decision lasts. This change lands the seam before
any code reads the cluster.

## What Changes

- New package `internal/authz`: an `Identity`, the read `Attributes` (verb, group, version,
  resource, subresource, namespace, name), a `Grant` that only `Checker.Check` can construct and
  that covers only its own identity and read until its decision expires, a typed `*DenialError`
  with a closed set of codes, and the `Authorizer` interface.
- Fail-closed guards that run before any backend call: an empty, blank or anonymous identity is
  refused with no Kubernetes call (the 0030:D6:R2 rule, applied in both milestones), any verb other
  than `get`, `list` or `watch` is refused, any subresource other than none, `status` or `log` is
  refused (exec, attach, portforward and proxy open streams), core `secrets` are refused whatever
  the caller's RBAC (0030:D8:R1), and malformed or wildcard attributes are refused.
- Deny on error: a failed, timed-out or unevaluable access review is a denial, never an allow,
  and is never cached (0030:D6:R4).
- The local-mode backend: SelfSubjectAccessReviews sent with the user's kubeconfig (0030:D5:R1),
  serving exactly the one identity it was built for.
- A decision cache with a 30-second TTL keyed by the canonical identity and attributes.
- Tests: the empty-identity regression modelled on Flux Operator CVE-2026-23990, identical
  denials for existing and missing objects, and a compile-time proof that no other package can
  build a `Grant`.
- Dependencies: `k8s.io/client-go`, `k8s.io/api` and `k8s.io/apimachinery` at v0.36.4, the pin the
  cli and opm-operator use.

## Capabilities

### New Capabilities

- `read-authorization`: how the portal decides whether a caller may make one Kubernetes read,
  what it refuses before asking the cluster, and what a denial reveals.

### Modified Capabilities

None.

## Impact

- Packages: new `internal/authz`. No API resource, UI page or binary behaviour changes; the
  package has no caller yet (the read model, change `add-read-model`, is the first).
- Principle V: the change adds the only cluster call the portal makes so far, `create` on
  `authorization.k8s.io` `selfsubjectaccessreviews`, with the user's own kubeconfig. A review is
  answered in the response and never stored. It reads no object, no Secret, and refuses an empty
  identity before any call.
- Principle VII: client-go is the dependency the constitution names for cluster reads; it earns
  its place here with the typed review client and the fake clientset the tests need.
- SemVer: MINOR after 1.0 (new internal capability, nothing removed). Every commit is `chore` or
  `docs` because nothing user-visible changes, so the 0.x line cuts no release for it.
- Enhancement link: lands the seam for 0030:D7 (the R1 refusal at Check); the M1 halves of
  0030:D5 and 0030:D6 are guards and backends only. No decision is claimed (`enhancement.yaml`):
  the change that completes 0030:D7 with a read path claims it.

Not in this change: the in-cluster SubjectAccessReview backend and identity mapping (changes
`add-sar-authorizer`, `add-oidc-sessions`), resolving the kubeconfig's identity at startup and
the launch token (`add-local-mode`), list filtering and per-item access states (0030:D7:R2/R3,
`add-read-model` and `add-read-api`), and the not-in-inventory refusal (0030:D7:R4).
