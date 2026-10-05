## 1. Identity mapping (internal/auth)

- [ ] 1.1 `internal/auth/claims.go`: `claimMapper` from username and groups claims and prefixes; the fail-closed rules (empty raw username before prefixing, `system:` username, `email_verified` false, groups shape, `system:` groups dropped before and after prefixing, `system:authenticated` added); `ErrUnmappedIdentity`; prefix validation (`checkPrefixes`)
- [ ] 1.2 Table tests: every refusal, the mapping of groups, prefix validation including `sys` and the trusted-issuer exception, refusals naming no claim value
- [ ] 1.3 `task check` green, then commit `feat(auth): map OIDC claims to a fail-closed identity`

## 2. Bearer tokens and the fake issuer (internal/auth, internal/auth/oidctest)

- [ ] 2.1 `go.mod`: add `github.com/coreos/go-oidc/v3` v3.21.0 and `golang.org/x/oauth2` as direct dependencies
- [ ] 2.2 `internal/auth/oidctest`: in-process issuer over `httptest` TLS with discovery, keys, authorize (auto-approves the configured claims) and token (checks client, redirect URI and the `S256` verifier) endpoints, key rotation, request counters, `Sign` for bearer tokens
- [ ] 2.3 `internal/auth/keyset.go`: cached `oidc.KeySet` with a refresh lower bound, stale-cache refresh, capped body, public signing keys only
- [ ] 2.4 `internal/auth/oidc.go`: `OIDCConfig`, `NewOIDC` (validation, discovery, algorithms, first key fetch), `Authenticate` for bearer tokens (header shape, size cap, verification, mapping, HMAC session key)
- [ ] 2.5 Tests: configuration refusals before any issuer request; bearer accept; wrong audience, issuer, expired, foreign key, `none` and `HS256` refused; key-ID burst fetches once; rotation after the interval; CVE-2026-23990 regression through `internal/api` with zero reviews and zero reads
- [ ] 2.6 `task check` green, then commit `feat(auth): accept bearer tokens from the configured OIDC issuer`

## 3. Browser sign-in, sessions and the front door (internal/auth)

- [ ] 3.1 `internal/auth/oidc_session.go`: sealed login cookie (AES-GCM, per-process key), session store (digest-keyed, absolute lifetime, bounded), `Authenticate` for cookies
- [ ] 3.2 `internal/auth/oidc_handler.go`: `Handler` with headers plus HSTS, cross-origin protection, `/auth/login`, `/auth/callback`, `POST /auth/logout`, the navigation redirect; shared `refuse`
- [ ] 3.3 Tests: full sign-in through the fake issuer; forged state, missing cookie, issuer error, nonce mismatch, unmapped identity; open-redirect return paths; logout revokes and redirects to end-session; expiry; session cap; cross-origin logout refused; headers on every response; no credential in logs
- [ ] 3.4 `internal/auth/doc.go`, `AGENTS.md` layout and purpose lines
- [ ] 3.5 `task check` green, then commit `feat(auth): sign browsers in with OIDC code flow and PKCE`
