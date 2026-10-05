## Context

PR 28 added an OIDC authenticator to `internal/auth` for milestone 2. Two later merges touched it or
its surroundings: PR 27 set `api.ModeInCluster` in `internal/auth/oidc_api_test.go`, and PR 29 added
`docs/DESIGN.md` and `ROADMAP.md`, which describe OIDC as in review. The owner decided on
2026-10-05 to remove OIDC and keep it as a future plan.

## Goals / Non-Goals

**Goals:**

- The tree holds no OIDC code, no OIDC dependency and no OIDC main spec.
- Local mode behaves exactly as before.
- The design record and roadmap keep the OIDC design as a future plan, with every number intact.

**Non-Goals:**

- Removing the in-cluster SubjectAccessReview authorizer (PR 24) or the in-cluster safeguards
  (PR 27). They stay on main, dormant; the per-subscriber stream diff also serves local mode.
- Rewriting the archived change `2026-10-05-add-oidc-sessions`: it is history.

## Decisions

- The removal MUST start from `git revert --no-commit f5a8eaa`, so the deletion matches what PR 28
  added. The archived change directory and the main spec are restored from the revert: the first
  is history, the second is removed through this change's REMOVED delta at archive.
- `go mod tidy` decides the dependency set; it drops go-oidc and go-jose and keeps
  `golang.org/x/oauth2` as indirect.
- The DESIGN.md edit SHALL keep portal:D6's text and requirement numbers (numbers are permanent)
  and add a status line naming which parts are a future plan.

## Research & Decisions

### The in-cluster empty-identity test

**Context**: `internal/auth/oidc_api_test.go` was the regression test for the empty-identity bug
class (CVE-2026-23990): signed tokens whose claims map to no user, sent to the read API in
in-cluster mode, must get 401 with no review and no read for the caller or the portal's reader. It
needs the OIDC authenticator, and the revert conflicts on it because PR 27 edited it.
**Explored**: `internal/api/authz_test.go` `TestUnauthenticatedIsRefusedBeforeAnyReview` covers
empty and anonymous principals, but in local mode and only counting the caller's reviews.
**Options considered**:
1. Delete the test - loses the in-cluster, reader-side and no-read assertions.
2. Move it to `internal/api` with an `Authenticate` hook that returns fake identities - keeps every
   API-level assertion; the claim-shape cases (a non-string `sub`, a `system:` username) were the
   OIDC mapper's job and go with it.
**Decision**: Option 2, as `internal/api/incluster_identity_test.go`
(`TestEmptyIdentityNeverReachesTheClusterInCluster`).
**Rationale**: portal:D6:R2 at the API (`Identity.Authenticated` in `Server.ServeHTTP`) stays on
main with the in-cluster mode, so its test stays too.

## Risks / Trade-offs

- [Re-adding OIDC later costs a re-implementation] → The archived change and `git show f5a8eaa`
  hold the full code, tests and spec; issue 33's open items are listed on the roadmap.
