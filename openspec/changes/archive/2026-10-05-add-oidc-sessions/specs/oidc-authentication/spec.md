## ADDED Requirements

### Requirement: The OIDC authenticator refuses an unsafe configuration

Constructing the OIDC authenticator SHALL fail, before any network call, when the issuer URL is
not `https`, the client ID is empty, the redirect URL is not absolute, is plain `http` on a
non-loopback host, or has a path other than `/auth/callback`, the username prefix is empty, or a
groups claim is configured with an empty groups prefix, unless the configuration declares that
the API server trusts the same issuer with the same prefixes. It SHALL also fail for any prefix
that is a prefix of `system:` or starts with `system:`. It SHALL then fail when discovery or the
first fetch of the issuer's signing keys fails, or when the discovered issuer differs from the
configured one. Source: 0030:D6:R6.

#### Scenario: Empty username prefix

- **WHEN** the authenticator is constructed with username prefix `""` and no declaration that the
  API server trusts the issuer
- **THEN** construction fails with an error naming the username prefix
- **AND** no request reaches the issuer

#### Scenario: Prefixes allowed empty when the API server trusts the issuer

- **WHEN** the configuration declares the API server trusts the issuer and both prefixes are empty
- **THEN** construction succeeds against a reachable issuer

#### Scenario: A prefix that could form a system name

- **WHEN** the groups prefix is `sys` or `system:oidc:`
- **THEN** construction fails

#### Scenario: Unreachable issuer

- **WHEN** the issuer's discovery document cannot be fetched
- **THEN** construction fails and no authenticator is returned

### Requirement: Claims map to an identity that fails closed

The authenticator SHALL map a verified token's claims to an identity as follows, and SHALL refuse
the token, with no Kubernetes call made on its behalf, when any rule refuses it. The username
claim SHALL be a string that is non-empty after trimming, checked before the username prefix is
applied. A raw or mapped username starting with `system:` SHALL be refused. When the username
claim is `email`, a token with `email_verified` false SHALL be refused. A groups claim, when
configured, SHALL be absent, a string, or a list of strings, and any other shape SHALL be
refused. Every group that starts with `system:`, before or after the groups prefix is applied,
SHALL be dropped, and `system:authenticated` SHALL be added to every mapped identity. Source:
0030:D6:R2/R3.

#### Scenario: Empty username claim (CVE-2026-23990 regression)

- **WHEN** a correctly signed bearer token for the configured audience carries an empty,
  whitespace-only or missing username claim, with or without groups
- **THEN** the read API answers `401` with code `unauthenticated`
- **AND** no access review and no Kubernetes read is made for the request

#### Scenario: Identity provider asserts system groups

- **WHEN** a token for user `alice` carries groups `["system:masters", "dev", ""]` with username
  prefix `oidc:` and groups prefix `oidc:`
- **THEN** the identity is username `oidc:alice` with groups `oidc:dev` and
  `system:authenticated` only

#### Scenario: System username

- **WHEN** a token's username claim is `system:admin`
- **THEN** the token is refused

#### Scenario: Malformed groups claim

- **WHEN** the groups claim is a number or a list holding a non-string
- **THEN** the token is refused

### Requirement: Bearer tokens are accepted only from the configured issuer and audience

A request with an `Authorization` header SHALL be authenticated only from that header: it SHALL
be accepted when the header is `Bearer` followed by a JWT signed by one of the issuer's signing
keys with an asymmetric algorithm, whose `iss` is the configured issuer, whose `aud` names the
configured audience, and which has not expired. Any other header or token SHALL be refused with
no Kubernetes call made on its behalf, and the request SHALL NOT fall back to a session cookie.
A bearer client SHALL map to the same identity a browser session for the same claims maps to.
Source: 0030:D6:R5/R8.

#### Scenario: Wrong audience, wrong issuer, expired, bad signature

- **WHEN** a bearer token names another audience, comes from another issuer, has expired, is
  signed by a key the issuer does not publish, or uses `alg: none` or `HS256`
- **THEN** the read API answers `401` and no access review is made

#### Scenario: Same view as the browser

- **WHEN** a valid bearer token and a browser session carry the same claims
- **THEN** both authenticate as the same identity

### Requirement: Signing keys are cached and refetched with a lower bound

The authenticator SHALL cache the issuer's signing keys. A token whose key ID is not cached SHALL
cause a fetch of the key set only when the configured refresh interval has passed since the last
fetch; otherwise the token SHALL be refused without a fetch. A key the issuer rotates in SHALL be
accepted once a fetch after the interval returns it.

#### Scenario: A burst of tokens with unknown key IDs

- **WHEN** many bearer tokens with unknown key IDs arrive within one refresh interval
- **THEN** the issuer's key endpoint is fetched at most once for them, and each is refused

#### Scenario: Key rotation

- **WHEN** the issuer starts signing with a new key and a token signed with it arrives after the
  refresh interval
- **THEN** the key set is fetched again and the token is accepted

### Requirement: Browsers sign in with the authorization code flow and PKCE

`GET /auth/login` SHALL redirect the browser to the issuer's authorization endpoint with a fresh
`state`, a fresh `nonce` and an `S256` PKCE challenge, and SHALL set a short-lived, sealed,
`Secure`, `HttpOnly` login cookie binding them to the browser. `GET /auth/callback` SHALL create a
session only when the login cookie is present and unexpired, `state` matches it, the code
exchange with the PKCE verifier succeeds, the returned ID token verifies against the client ID,
its `nonce` matches, and its claims map to an identity. Every other callback SHALL be refused
with a plain-text page naming no token, code or claim value, and SHALL create no session. A
`return` parameter SHALL be honoured only when it is a path on the portal itself.

#### Scenario: Successful sign-in

- **WHEN** a browser follows `/auth/login?return=/instances` through the issuer and back
- **THEN** the callback answers `303` to `/instances` with a session cookie
- **AND** requests carrying that cookie authenticate as the mapped identity

#### Scenario: Forged or replayed callback

- **WHEN** a callback arrives without the login cookie, with a `state` other than the cookie's,
  with an `error` parameter, or with a code whose ID token carries another nonce
- **THEN** the portal answers `400` or `403` and sets no session cookie

#### Scenario: Open redirect

- **WHEN** a browser starts sign-in with `return=//evil.example` or `return=https://evil.example`
- **THEN** a successful sign-in lands on `/`

### Requirement: Sessions are server-side and revocable

A session SHALL be held in the portal's memory, keyed by a digest of a random cookie value. Its
cookie SHALL be `__Host-` prefixed, `Secure`, `HttpOnly`, `SameSite=Lax`, host-only and `Path=/`.
A session SHALL end at its absolute lifetime, and `POST /auth/logout` SHALL delete it and clear
the cookie, after which the cookie authenticates nothing. The number of sessions SHALL be bounded.

#### Scenario: Logout revokes the cookie

- **WHEN** a signed-in browser posts to `/auth/logout` from the portal's origin
- **THEN** the session is deleted and the old cookie value is refused on the next request
- **AND** the browser is sent to the issuer's end-session endpoint when it advertises one

#### Scenario: Expired session

- **WHEN** a request carries a session cookie past the session lifetime
- **THEN** it is unauthenticated

### Requirement: The in-cluster front door protects every response

Every response of the OIDC front door, refusals included, SHALL carry the same security headers
as local mode and `Strict-Transport-Security`. A non-safe cross-origin request SHALL be refused.
A browser navigation without a live session or an `Authorization` header SHALL be redirected to
`/auth/login` with its path as the return path; other unauthenticated requests SHALL reach the
read API, which answers `401`. No token, cookie value, authorization code, client secret or
`Authorization` header SHALL appear in a log line, an error or a response body.

#### Scenario: Cross-site logout

- **WHEN** a page on another origin posts to `/auth/logout`
- **THEN** the portal answers `403` and the session survives

#### Scenario: Navigation without a session

- **WHEN** a browser navigates to `/instances` with no session
- **THEN** the portal answers `303` to `/auth/login?return=%2Finstances`

#### Scenario: No credentials in logs

- **WHEN** a sign-in, a refused callback and refused bearer tokens are logged
- **THEN** no log line contains a token, a cookie value, the code or the client secret
