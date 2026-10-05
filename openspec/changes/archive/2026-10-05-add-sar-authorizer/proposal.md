## Why

Milestone 2 runs the portal in-cluster: people sign in, and the portal reads with its own
ServiceAccount only after the cluster has said the signed-in person may make that read
(0030:D6). `internal/authz` has the seam, the guards and the cache, but its only backend is the
local one, which asks with SelfSubjectAccessReviews. In-cluster a self review would answer for
the portal's own ServiceAccount, not the person, so it is forbidden there (0030:D6:R9). This
change adds the in-cluster backend behind the same seam, so the in-cluster wiring can plug it in
without touching a read path.

The read model's informers, polls and on-demand lists also need a grant, for the identity they
read as. In-cluster that identity is the portal's ServiceAccount. The owner decided (2026-10-05,
"SAR for its own SA") that the portal sends a SubjectAccessReview naming its own ServiceAccount at
startup and per TTL, and the background reads hold that grant: no exception to "every read holds
a grant", and every answer a user sees is still gated by a review as that user.

## What Changes

- `authz.NewInCluster(reviews, reader, opts)`: a `Checker` that decides through
  `authorization.k8s.io/v1` `subjectaccessreviews`, carrying the identity's username, UID, groups
  and extra values and the read's exact attributes. It holds a SubjectAccessReview client and
  nothing else: it can send no self review and read no object.
- The same guards and cache as local mode: the empty-identity guard runs before any call, an
  error, a timeout or an evaluation error without an allow is a denial and is never cached, and
  the cache key is the full identity.
- Two routes, decided on the identity's full key before any review:
  - the reader (the portal's ServiceAccount): reviewed as that ServiceAccount, with the groups the
    API server gives it, so the grant asks the same question the API server answers for the read;
  - everyone else is a person: a `system:` username, or any `system:` group other than
    `system:authenticated`, is refused as unauthenticated with no call made, so a mapping slip
    can never borrow the ServiceAccount's access or a privileged group (0030:D6:R3).
- `authz.ServiceAccountIdentity(username)` builds the reader identity from
  `system:serviceaccount:<namespace>:<name>` (the value a deployment flag will carry), and
  `authz.ServiceAccountFromToken(token)` reads that username from the in-cluster token's `sub`
  claim.
- `Options.AccessLog`: when set, one structured line per decision for a person (allowed or
  denied, cached or not): the username, verb, group, version, resource, subresource, namespace,
  name, decision and denial code. Never groups, extra values, tokens, the review's error text or
  any object content (0030:D6:R7). The reader's own decisions are not logged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `read-authorization`: an in-cluster backend that asks as the signed-in user, refuses system
  identities for people, reviews the portal's ServiceAccount for background reads, never sends a
  self review, and writes a per-user access log.

## Impact

- Packages: `internal/authz` only. No read path, API resource or UI page changes: the read model,
  the logs producer and the read API already take an `Authorizer` and a reader identity.
- Not wired yet: no command builds the in-cluster `Checker` until OIDC sign-in lands, so this
  change adds no flag. The in-cluster wiring passes the reader from a `--service-account` flag or
  from the mounted token, and the access log logger.
- Principle V: in-cluster the portal creates only `subjectaccessreviews` (0030:D6:R9), checked by
  a test that records every action the fake cluster sees. The portal reads nothing new and no
  Secret. An empty identity, and an identity that names a system user or carries a system group,
  is refused before any call, never answered as the portal's ServiceAccount. The access log
  carries no token, value or review error text.
- Principle VII: no new dependency. The token's claim is read with `encoding/base64` and
  `encoding/json`; it is the portal's own mounted token, used only to name itself, so it is not
  verified.
- SemVer: MINOR after 1.0 (a new authorizer). On the 0.x line it ships as `feat(authz)` and cuts
  a minor release.
- Enhancement link: lands 0030:D6 R1, R2, R4, R7 and R9 on the authorizer side and R3's last line
  of defence; D6 is not claimed until OIDC sign-in and identity mapping land.
