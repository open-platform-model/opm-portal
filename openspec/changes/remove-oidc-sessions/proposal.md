## Why

The owner expected the current work to add local mode only, not OIDC. Asked on 2026-10-05 what to
do with the OIDC sign-in merged as PR 28 (`feat(auth): authenticate with OIDC for in-cluster
mode`, squash commit f5a8eaa), the owner answered: "Remove OIDC, but keet it as future plans".

Nothing constructs in-cluster mode today: no command wires the OIDC authenticator, so it is about
3,400 lines of security-boundary code, two dependencies and a main spec that ship with every
release and protect nothing. This change removes it and keeps in-cluster mode with OIDC sign-in as
a future plan in `docs/DESIGN.md` (portal:D6) and `ROADMAP.md`.

## What Changes

- Revert f5a8eaa in code: delete `internal/auth`'s OIDC authenticator (`claims.go`, `keyset.go`,
  `oidc.go`, `oidc_session.go`, `oidc_handler.go` and their tests) and the test issuer
  `internal/auth/oidctest`. `internal/auth` goes back to local mode only (`doc.go`, `handler.go`).
- `go.mod`: drop `github.com/coreos/go-oidc/v3` and `github.com/go-jose/go-jose/v4`;
  `golang.org/x/oauth2` returns to an indirect dependency (client-go uses it).
- The empty-identity regression test that drove OIDC tokens through the read API in in-cluster
  mode (`internal/auth/oidc_api_test.go`, edited by PR 27) moves to `internal/api` and drives fake
  identities instead, so the fail-closed check of portal:D6:R2 at the API keeps its coverage.
- Remove the `oidc-authentication` main spec. The archived change
  `2026-10-05-add-oidc-sessions` stays as history.
- `docs/DESIGN.md`: portal:D6's OIDC sign-in and identity mapping (R2/R3 mapping half, R5, R6, R8)
  are marked a future plan, text and numbers kept, with the owner decision recorded.
  `ROADMAP.md`: M2 becomes "Future plans: in-cluster mode with OIDC sign-in", listing what stays
  dormant on main, what was removed, and the open items of issue 33.
- Stays on main, dormant: the in-cluster SubjectAccessReview authorizer (PR 24) and the in-cluster
  safeguards (PR 27: the API mode setting, operator-text hiding, the per-subscriber stream diff,
  which local mode also uses).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `oidc-authentication`: removed in full; every requirement goes, and with them the main spec.

## Impact

- Packages: `internal/auth` (OIDC files and `oidctest` deleted), `internal/api` (one test file
  added). `cmd/opm-portal`, `internal/authz`, `internal/readmodel`, `internal/stream` and the read
  API are unchanged. No API resource or UI page changes.
- Principle V: no read, identity or review changes. Local mode authenticates exactly as before.
  The removed code was never reachable from a command.
- Principle VII: two dependencies leave.
- SemVer: PATCH after 1.0 (the removed package is internal, no public surface changes). On the 0.x
  line it ships as `revert`, which releases; the PR body names the reverted commit so
  release-please drops PR 28's entry from the 0.1.0 changelog.
- Decisions: portal:D6 (status of its OIDC requirements). No enhancement is implemented, so the
  change carries no `enhancement.yaml`.
