## Why

Milestone 2 of enhancement 0030 runs the portal in-cluster for many people, so it must know who
each request is before it can ask a SubjectAccessReview on their behalf (0030:D6). Local mode
borrows the kubeconfig's identity; in-cluster there is no such identity, and the bug class this
must avoid is concrete: CVE-2026-23990 let empty OIDC claims in the Flux Operator UI fall through
to the server's own identity. This change adds the identity half of milestone 2 in `internal/auth`:
OIDC sign-in for browsers, bearer JWTs for programmatic clients, and an identity mapping that fails
closed. The in-cluster command that wires it is a later change.

## What Changes

- New `auth.OIDC`, constructed by `auth.NewOIDC(ctx, OIDCConfig)`: OIDC discovery against the
  configured issuer, refusing to construct on an invalid configuration (0030:D6:R6).
- Browser sign-in through the authorization code flow with PKCE (`S256`), `state` and `nonce`,
  carried between `/auth/login` and `/auth/callback` in a short-lived sealed cookie.
- Server-side, in-memory sessions behind a random `__Host-` cookie (`Secure`, `HttpOnly`,
  `SameSite=Lax`); `POST /auth/logout` deletes the session and, when the issuer advertises one,
  sends the browser to its end-session endpoint.
- Bearer JWTs in `Authorization`: accepted only when signed by the issuer's keys, naming the
  configured audience and unexpired; anything else is refused with no Kubernetes call
  (0030:D6:R8). Signing keys are cached and refetched on an unknown key at most once per interval.
- Identity mapping from configurable username and groups claims and prefixes: an empty username
  claim is refused before prefixing, a `system:` username is refused, every `system:` group is
  stripped and `system:authenticated` is added (0030:D6:R2/R3). A regression test for
  CVE-2026-23990 drives empty and missing claims through the read API and asserts zero reviews and
  zero reads.
- `OIDC.Authenticate` keeps local mode's hook contract (identity, session key, error);
  `OIDC.Handler` is the in-cluster front door: the same security headers as local mode plus HSTS,
  `http.CrossOriginProtection`, the three auth routes, and a redirect to sign-in for a browser
  navigation without a session.
- `internal/auth/oidctest`: an in-process OIDC issuer (discovery, keys, authorize, token) for tests.

## Capabilities

### New Capabilities

- `oidc-authentication`: how the in-cluster portal learns who a request is: configuration checks,
  browser sign-in and sessions, bearer tokens, identity mapping, key caching, and the front door.

### Modified Capabilities

None. Local mode is unchanged.

## Impact

- Packages: `internal/auth` gains the OIDC authenticator; new test-support package
  `internal/auth/oidctest`. `internal/api`, `internal/authz` and `cmd/opm-portal` are unchanged:
  no `serve --mode in-cluster` flag yet.
- Principle V: no Kubernetes call is added. The authenticator only decides who a request is; a
  request it refuses never reaches the read API, and an identity it returns always names a
  non-`system:` user and carries `system:authenticated`. Reads stay as they are; the
  SubjectAccessReview backend is a separate change. Tokens, cookies, the client secret and
  `Authorization` headers never appear in a log line, an error or a response.
- Principle VII: two dependencies. `github.com/coreos/go-oidc/v3` verifies ID tokens (issuer,
  audience, expiry, algorithm) and performs discovery; hand-written JWT validation is the riskier
  choice in a security boundary. `golang.org/x/oauth2`, already an indirect dependency, runs the
  code exchange and PKCE. `go-jose/v4` arrives with go-oidc and verifies signatures in the
  portal's own rate-limited key set.
- SemVer: MINOR after 1.0 (a new capability, nothing changed). On the 0.x line it ships as
  `feat(auth)`.
- Enhancement link: lands 0030:D6:R2/R3/R5/R6/R8 on the authentication side. D6 is not claimed:
  R1, R4, R7 and R9 belong to the SubjectAccessReview authorizer and the in-cluster command.
