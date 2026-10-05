## Context

`main` serves local mode: `internal/auth.Local` admits one browser through a launch token and
answers `Authenticate(r) (authz.Identity, string, error)`, which `cmd/opm-portal` adapts into
`api.Config.Authenticate`. The read API refuses any request whose identity is not
`Authenticated()` or whose session key is empty, before any review (0030:D6:R2). Milestone 2 needs
a second implementation of the same hook for many users signing in through an OIDC issuer.

Sources: 0030:D6 (R2, R3, R5, R6, R8), 0030:D7 (refusals before lookup), and the risk entry for
CVE-2026-23990 in 0030's risks file. Where this design and 0030 disagree, 0030 wins. A parallel
change adds the SubjectAccessReview authorizer; a later one builds `serve --mode in-cluster`.

## Goals / Non-Goals

**Goals:**

- One type, `auth.OIDC`, with the same `Authenticate` contract as `auth.Local` and a `Handler`
  front door, constructible from configuration and testable without a network beyond loopback.
- Every rejected credential, and every identity that maps to no one, stops in `internal/auth`:
  the read API never sees it, so no review and no read is made for it.
- No token, cookie, client secret or `Authorization` header in a log line, error or response.

**Non-Goals:**

- The in-cluster command, its flags, TLS serving, the install manifest (later change).
- The SubjectAccessReview authorizer and the per-user access log, 0030:D6:R1/R7 (other changes).
- Refresh tokens, sessions shared across replicas, UserInfo lookups, distributed claims.

## Decisions

### Configuration and construction

```go
type OIDCConfig struct {
    IssuerURL    string   // https only; discovery at IssuerURL/.well-known/openid-configuration
    ClientID     string   // required; the audience of browser ID tokens
    ClientSecret string   // optional (public client); never logged
    RedirectURL  string   // absolute; path MUST be CallbackPath; https unless the host is loopback
    Scopes       []string // default openid, email, profile; openid always added
    Audience     string   // audience a bearer token must name; default ClientID

    UsernameClaim  string // default "sub"
    UsernamePrefix string
    GroupsClaim    string // empty: no groups are read
    GroupsPrefix   string
    // APIServerTrustsIssuer declares the API server trusts this issuer with
    // the same prefixes; only then may a prefix be empty (0030:D6:R6).
    APIServerTrustsIssuer bool

    PostLogoutRedirectURL string        // sent to the end-session endpoint, if any
    SessionTTL            time.Duration // default 8h, absolute
    MaxSessions           int           // default 10000
    KeyRefreshInterval    time.Duration // default 30s: least time between key fetches
    HTTPClient            *http.Client  // for the issuer; default 10s timeout
    Now                   func() time.Time
    Logger                *slog.Logger
}

func NewOIDC(ctx context.Context, cfg OIDCConfig) (*OIDC, error)
func (o *OIDC) Authenticate(r *http.Request) (authz.Identity, string, error)
func (o *OIDC) Handler(next http.Handler) http.Handler
```

`NewOIDC` MUST refuse, before any network call: a non-https issuer, an empty client ID, a
redirect URL that is not absolute, not https (loopback hosts excepted, for development) or whose
path is not `/auth/callback`, an empty username prefix, or an empty groups prefix when a groups
claim is set, unless `APIServerTrustsIssuer` is true, and any prefix that is a prefix of
`system:` or starts with it (so `sys` plus a group `tem:masters` cannot form `system:masters`). It
then runs discovery, fetches the signing keys once and fails on either error, so a portal that
cannot verify tokens never starts.

### Identity mapping fails closed

```go
func (m claimMapper) identity(claims map[string]any) (authz.Identity, error)
```

1. The username claim MUST be a JSON string whose trimmed value is non-empty; otherwise
   `ErrUnmappedIdentity`. This check runs on the raw claim, before the prefix: a prefix would turn
   an empty claim into a non-empty name such as `oidc:`, which is the CVE-2026-23990 shape.
2. When the username claim is `email` and the token carries `email_verified: false`, refuse (the
   API server's own OIDC authenticator does the same).
3. A raw username starting with `system:` is refused, and so is the mapped one.
4. The groups claim, when configured, MUST be absent, a string, or an array of strings; anything
   else is refused. Empty strings and raw `system:` groups are dropped, each remaining group is
   prefixed, mapped groups starting with `system:` are dropped again, then duplicates go, and
   `system:authenticated` is appended (0030:D6:R3).
5. UID and Extra stay empty.

The result is always `Authenticated()`. A refusal names the rule, never a claim value.

### Bearer tokens

`Authenticate` reads `Authorization` first. A header present but not `Bearer <token>`, a token
over 16 KiB, or a token that fails verification returns `ErrInvalidToken`; a request with a
header never falls back to its cookie. Verification uses go-oidc's `IDTokenVerifier` with
`ClientID: Audience`, which checks issuer, audience, expiry and `nbf`, and restricts algorithms to
the asymmetric ones discovery advertises (RS256 when it advertises none; never `HS*` or `none`).
The session key is `bearer:` plus an HMAC-SHA-256 of the token under a per-process random key:
stable for one token (so its streams count against one session), unlinkable to the token outside
the process. A bearer client maps through the same claim mapper as a browser, so both see the
same identity (0030:D6:R5).

### Signing keys: the portal's own cached key set

go-oidc's `RemoteKeySet` fetches the JWKS again for every token whose key ID it does not know,
with no lower bound, so an unauthenticated caller can make the portal hammer the issuer. The
portal implements `oidc.KeySet` itself (`keySet`, about eighty lines over go-jose): keys are
cached; an unknown key ID triggers a fetch only when `KeyRefreshInterval` has passed since the
last one; a cache older than one hour is refreshed before use and kept if that refresh fails; the
response body is capped at 1 MiB and keys that are not public signing keys are skipped.

### Browser sign-in: code flow with PKCE

```text
GET  /auth/login?return=/path  -> 302 issuer authorize (state, nonce, S256 challenge)
                                  Set-Cookie __Host-opm-portal-login (sealed, 10 min)
GET  /auth/callback?code&state -> exchange with verifier, verify ID token, check nonce,
                                  map identity, create session
                                  Set-Cookie __Host-opm-portal-session; 303 to return path
POST /auth/logout              -> delete session, clear cookie;
                                  303 to end_session_endpoint, or 200 "signed out"
```

- `return` MUST be a local path (`/` first, not `//` or `/\`); anything else becomes `/`.
- The callback MUST refuse a missing or unsealable login cookie, an expired login, a `state` that
  does not match it (constant time), an `error` from the issuer, a missing `id_token`, a failed
  verification, a nonce mismatch, or an identity the mapper refuses; each answer is a plain-text
  `400` or `403` naming no value, and the login cookie is cleared either way.
- The front door redirects a `GET` that is a browser navigation (`Sec-Fetch-Mode: navigate`, or
  no such header and an `Accept` naming `text/html`) and carries neither a live session nor an
  `Authorization` header to `/auth/login?return=<path>`. API clients and htmx requests get the
  read API's `401` as before.

### Front door

Every response, refusals included, carries local mode's headers (`securityHeaders`) plus
`Strict-Transport-Security: max-age=31536000`. `http.CrossOriginProtection` wraps everything, so
`POST /auth/logout` and any later non-safe method refuse a cross-origin request. There is no
`Host` allowlist: in-cluster the portal sits behind an ingress and probes use the pod address,
and the `Secure` host-only cookie plus cross-origin protection carry what the allowlist did
locally.

## Research & Decisions

### Where sessions live

**Context**: The task asks for an encrypted cookie or a server store, justified.
**Explored**: 0030's design ("stateless apart from sessions") and operational notes ("no state
beyond in-memory sessions"); local mode's digest-keyed session.
**Options considered**:
1. Encrypted cookie holding the mapped identity - survives restarts and scales across replicas;
   but logout cannot revoke a stolen cookie before it expires, the cookie grows with the groups
   list (4 KiB limit), and the sealing key needs secret management across replicas.
2. Server-side in-memory store keyed by the SHA-256 of a random cookie value - logout revokes at
   once, the cookie is 26 random characters, nothing to manage; sessions are lost on restart and
   a second replica needs sticky sessions.
**Decision**: Option 2, bounded twice. Each mapped username holds at most ten sessions; at that
bound a new sign-in ends that user's own session closest to expiry. `MaxSessions` bounds the whole
store (expired sessions are swept on insert; at the cap the session closest to expiry is evicted,
whoever holds it).
**Rationale**: Revocation on logout is a security property; restart re-login is a cost the
operational notes already accept, and V1 runs one replica.

### Where the login transaction lives

**Context**: `state`, `nonce` and the PKCE verifier must survive the round trip to the issuer.
**Options considered**:
1. Server map keyed by state - anyone unauthenticated can fill it by hitting `/auth/login`.
2. AES-GCM sealed cookie under a per-process random key, ten minutes, bound to the browser.
**Decision**: Option 2. **Rationale**: no memory an unauthenticated caller can grow; the cookie
also binds the callback to the browser that started it (login CSRF). Two tabs signing in at once
overwrite one cookie, so the first callback fails and that tab starts again.

### SameSite for the session cookie

**Context**: The callback is reached by a cross-site redirect from the issuer.
**Options considered**: `Strict` (the cookie set there is withheld from the redirect that
follows, as local mode found with its launch) or `Lax` (sent on top-level `GET` navigations).
**Decision**: `Lax` for both cookies. **Rationale**: every read is a `GET` whose response another
site cannot read, and every non-safe method passes `http.CrossOriginProtection`.

### Groups prefix when no groups claim is read

**Context**: 0030:D6:R6 asks for non-empty username and groups prefixes.
**Decision**: The groups prefix is required when `GroupsClaim` is set; with no groups claim the
portal reads no groups and a prefix would apply to nothing. The only group then is
`system:authenticated`.

### Dependencies

**Context**: Principle VII. **Options considered**: hand-written JWT and JWKS handling on the
standard library; `coreos/go-oidc/v3` plus `x/oauth2`.
**Decision**: go-oidc v3.21.0 and x/oauth2 (already indirect). **Rationale**: they are what the
Kubernetes API server's own OIDC support builds on; the portal adds only the key set, to bound
refetches.

## Authorization

This change sends no Kubernetes request. It reads nothing and creates nothing; its only network
peers are the configured issuer's discovery, key, token and (by redirect) authorize and
end-session endpoints.

## Risks / Trade-offs

- [Sessions are per process] -> a restart signs everyone out; one replica in V1, sticky sessions
  later if needed.
- [Group membership is fixed for the session] -> `SessionTTL` (8 hours) bounds how long a removed
  group still reaches the authorizer; no refresh token is kept.
- [Logout ends only the portal session when the issuer has no end-session endpoint] -> the next
  navigation signs in again silently; the signed-out page says so.
- [The global session cap evicts across users] -> one account can push out only its own sessions,
  but an attacker holding `MaxSessions / 10` accounts the mapper accepts (1,000 at the default) can
  still sign everyone else out, and each sign-in at the cap scans every session under one mutex.
  The accounts need no Kubernetes rights; an issuer that lets anyone register widens this.
- [A key rotated within `KeyRefreshInterval` of the last fetch] -> tokens signed with it are
  refused for at most that interval.

## Migration Plan

None: nothing wires `NewOIDC` yet. The in-cluster command constructs it.
