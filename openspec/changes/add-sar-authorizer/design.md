## Context

`internal/authz` decides every read through `Checker.Check`: the identity guard, the attribute
guard, the decision cache, then a backend under a timeout (`add-authz-seam`). The only backend is
`selfReviewer` (SelfSubjectAccessReviews, local mode). The read model, the logs producer and the
read API take one `Authorizer` and one reader identity, and ask that `Authorizer` for the
reader's grant before every informer, poll and on-demand list (`add-read-model`, Open
Questions: "whether the portal reviews its own ServiceAccount that way or trusts its role is for
`add-sar-authorizer` to decide"). The owner decided it on 2026-10-05: a SubjectAccessReview naming
the portal's own ServiceAccount, at startup and per TTL.

## Goals / Non-Goals

**Goals:**

- An in-cluster backend that asks as the person (0030:D6:R1) through `subjectaccessreviews` only
  (0030:D6:R9), behind the unchanged seam.
- The reader's grant from a review naming the portal's ServiceAccount, renewed by the cache TTL.
- No path on which a person is answered as the portal's ServiceAccount: an empty identity, a
  system username and a system group are refused before any call.
- A per-user access log line for every decision about a person (0030:D6:R7).

**Non-Goals:**

- OIDC sign-in, bearer tokens, identity mapping, prefixes (0030:D6:R3/R5/R6/R8): the in-cluster
  wiring change. That change also adds the `--service-account` flag and a request id; this change
  adds no flag and no command.
- Hiding condition and event messages in-cluster (owner decision on 0030:OQ8) and the
  per-subscriber change comparison (supervisor ruling on 0030:OQ20): read API and stream changes.

## Decisions

### Package shape

```go
// NewInCluster returns a Checker that decides through SubjectAccessReviews.
// reader is the portal's ServiceAccount identity (ServiceAccountIdentity).
func NewInCluster(reviews authorizationv1client.SubjectAccessReviewInterface, reader Identity, opts Options) (*Checker, error)

// ServiceAccountIdentity builds the identity the API server gives a
// ServiceAccount: system:serviceaccount:<ns>:<name>, with the groups
// system:serviceaccounts, system:serviceaccounts:<ns> and system:authenticated.
func ServiceAccountIdentity(username string) (Identity, error)

// ServiceAccountFromToken returns the sub claim of a projected
// ServiceAccount token, without verifying it.
func ServiceAccountFromToken(token []byte) (string, error)

type Options struct {
    Timeout    time.Duration
    TTL        time.Duration
    MaxEntries int
    // AccessLog, when set, receives one line per decision about a person.
    AccessLog *slog.Logger
}
```

`subjectReviewer.decide(ctx, who, req)`:

1. `who.key() == reader.key()`: send a SubjectAccessReview with the reader's username, groups
   and extra.
2. Otherwise `who` is a person. A username starting `system:` or a group starting `system:`
   other than `system:authenticated` is a `CodeUnauthenticated` denial; no review is sent.
3. Send a SubjectAccessReview with `User`, `UID`, `Groups`, `Extra` from `who` and
   `ResourceAttributes` from `req` (the helper local mode uses), and read its status with
   `verdict`, so deny wins, no opinion is a deny, and an evaluation error without an allow is
   unavailable and uncached.

The empty-identity guard is `Checker.Check`'s, unchanged: it runs before route 1, so an empty
identity can never match the reader or reach a review (the CVE-2026-23990 shape: empty claims
falling through to the server's identity). `NewInCluster` refuses a reader that is not a
ServiceAccount username, and copies it, so the caller cannot change it afterwards.

### Background grant: review the ServiceAccount, renewed by the TTL

**Context**: every read must hold a grant; the informers read as the ServiceAccount, and
in-cluster only `subjectaccessreviews` may be created (0030:D6:R9).
**Options considered**:
1. Trust the ServiceAccount's role: issue the reader a grant with no review. Adds an exception
   to "only a decision issues a grant" and hides a missing role behind failed reads.
2. A SubjectAccessReview naming the ServiceAccount (owner's choice).
3. A SelfSubjectAccessReview from the ServiceAccount: forbidden in-cluster by 0030:D6:R9.
**Decision**: option 2, through the same `Checker`. The read model already asks for the reader's
grant before it starts each informer (at startup) and before each poll and on-demand list; the
cache reuses an allow for one TTL and then asks again, so the review recurs per TTL with no new
code in the read model. A running informer is not stopped when its startup grant expires: its
watch runs with the ServiceAccount's token, so the API server enforces a revocation on it.
**Rationale**: no new grant path, and a deployment without the ServiceAccount's role shows the
kinds as forbidden at startup (the read model's denied scopes) instead of failing silently.

### Two routes in one Checker

**Context**: the read model, the logs producer and the read API ask one `Authorizer` for both
people and the reader.
**Options considered**:
1. One `Checker`, routed on the identity's full key, with people barred from system names and
   groups (chosen). A person can match the reader only with a `system:serviceaccount:` username
   and the ServiceAccount groups, both of which the person route refuses and identity mapping
   strips (0030:D6:R3).
2. A second `Authorizer` field for the reader in three packages' configs. Stronger separation,
   but it touches the read model, the logs producer and the read API for a case route 1 already
   refuses twice over.
**Decision**: option 1.

### Access log in the Checker

**Context**: 0030:D6:R7 asks for one record per authorized and per denied read, per person,
because the API server's audit log sees only the ServiceAccount.
**Decision**: `Checker.Check` logs one `Info` line, message `access`, after it decides, with
`user`, `verb`, `group`, `version`, `resource`, `subresource`, `namespace`, `name`, `decision`
(`allow` or `deny`), `code` (the denial code, empty on allow) and `cached`. It is written for
every outcome, the empty identity included, and skipped for the reader. It never carries groups,
extra values, the review's error text or anything read; attributes hold no values. Local mode may
pass a logger too, but its wiring does not, since the API server's audit log already names the
kubeconfig's user.
**Rationale**: the `Checker` is the one place every decision passes; logging there cannot miss a
read path.

### Authorization

The change reads no object. Its only cluster call is `create` on `authorization.k8s.io/v1`
`subjectaccessreviews`, which the API server evaluates and does not store (0030:D6:R9). The
portal's ServiceAccount needs `create` on `subjectaccessreviews` (the in-cluster role ships with
the wiring change).

## Risks / Trade-offs

- [The token's `sub` is read unverified] -> it is the portal's own mounted token and names only
  the portal; a wrong name makes the reader's reviews answer for another subject, while the reads
  themselves still run as the real ServiceAccount, so the API server stays the boundary. The flag
  overrides it.
- [The access log grows with every per-item check of a view] -> it is the record 0030:D6:R7
  asks for; cached decisions are marked so they can be filtered.
- [A person with no `system:authenticated` group is reviewed without it] -> the review then
  under-reports access, which fails closed; identity mapping adds it (0030:D6:R3).
