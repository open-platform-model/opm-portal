## 1. Remove the OIDC authenticator (internal/auth, internal/api, go.mod)

- [x] 1.1 `git revert --no-commit f5a8eaa`; restore `openspec/changes/archive/2026-10-05-add-oidc-sessions` (history) and `openspec/specs/oidc-authentication` (removed at archive through this change's delta)
- [x] 1.2 Resolve the conflict on `internal/auth/oidc_api_test.go`: delete it and move its API-level assertions to `internal/api/incluster_identity_test.go` with fake identities
- [x] 1.3 `internal/auth/doc.go`, `internal/auth/handler.go`: local mode only (from the revert)
- [x] 1.4 `go mod tidy`: go-oidc and go-jose dropped, `golang.org/x/oauth2` indirect
- [x] 1.5 `task check` green, then commit `revert(auth): remove the OIDC authenticator`

## 2. Keep in-cluster mode with OIDC as a future plan (docs)

- [ ] 2.1 `docs/DESIGN.md`: portal:D6 status (OIDC sign-in and mapping a future plan, text and numbers kept, owner decision recorded); summary, approach and risks say so
- [ ] 2.2 `ROADMAP.md`: M2 becomes "Future plans: in-cluster mode with OIDC sign-in" (dormant on main, removed, issue 33's open items)
- [ ] 2.3 `AGENTS.md`, `CONSTITUTION.md`, `openspec/config.yaml`, `README.md`: in-cluster mode with OIDC described as a future plan
- [ ] 2.4 `task check` green, then commit `docs: keep in-cluster mode with OIDC sign-in as a future plan`
