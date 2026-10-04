## Context

`main` holds the read path: `internal/authz` (`NewLocal`, a `Checker` over
SelfSubjectAccessReviews), `internal/readmodel` (`New`, `Start`, `TuneConfig`), `internal/logs`
(a `stream.Producer` for `log:` topics), `internal/stream` (`Broker`, `Mux`) and `internal/api`
(the handlers behind an `Authenticate` hook, building its own broker over its own producer). The
binary prints its version. Supervisor rulings: the read-model producer and the logs producer are
joined through `stream.Mux` in this change, and `TuneConfig` applies to the dynamic client and to
the review client alike.

Sources: 0030:D5 (local mode, R1 to R7), 0030:D6:R2 (an empty identity is refused before any
Kubernetes call), 0030:D10 (logs). Where this design and 0030 disagree, 0030 wins.

## Goals / Non-Goals

**Goals:**

- `opm-portal serve` gives the user a working read API in their browser with one command, as the
  kubeconfig's identity and nobody else's.
- Nothing but the browser that opened the launch link can read through the portal: not another
  local user, not another local process, not a web page in another tab (DNS rebinding, CSRF).
- Every refusal happens before any cluster call, and no secret reaches a log.

**Non-Goals:**

- HTML pages (`internal/ui`), TLS, in-cluster mode, OIDC, multiple kubeconfig contexts at once.
- Re-issuing a launch link without a restart.

## Decisions

### The command: `serve` with five flags, loopback checked before anything else

```text
opm-portal serve [--kubeconfig PATH] [--context NAME] [--addr HOST:PORT]
                 [--namespaces NS[,NS...]] [--open]
```

- `--kubeconfig` empty follows client-go's loading rules (`KUBECONFIG`, then `~/.kube/config`);
  `--context` empty uses the current context.
- `--addr` defaults to `127.0.0.1:0` (a free port). The host MUST be a loopback IP literal
  (`127.0.0.0/8`, `::1`) or `localhost`, which binds `127.0.0.1`. An empty host (`:8080`), any
  other name, and any non-loopback IP are refused with exit code 2 before the kubeconfig is read
  (0030:D5:R2). After `net.Listen` the bound address is checked again.
- `--namespaces` limits the namespaced OPM kinds to those namespaces, so a user who cannot list
  them cluster-wide can still use the portal on the namespaces they can read (0030:D5:R5). The
  read model already takes `Namespaces`; the flag was not in the task's list but R5 is not
  reachable without it.
- `--open` opens the launch link in the default browser (below).
- Parse errors exit 2; a startup failure (kubeconfig, identity, listen) exits 1; a clean shutdown
  exits 0.

`run` becomes `run(ctx, args, stdout, stderr) int`; `main` gives it a context cancelled on
`SIGINT` or `SIGTERM`.

### One REST config, tuned once, for every client

```go
cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{CurrentContext: ctxName}).ClientConfig()
readmodel.TuneConfig(cfg)                 // QPS 50, burst 100 where unset
cs  := kubernetes.NewForConfig(cfg)       // reviews, discovery, pod logs
dyn := dynamic.NewForConfig(cfg)          // read model
```

Every client is built from the tuned config, so the review client (SelfSubjectAccessReviews,
sent before each read) is not throttled at client-go's default QPS 5 while the dynamic client runs
at 50. Exec credential plugins work as they do for `kubectl`. An error loading the kubeconfig is
reported with its path and client-go's message, never its content.

### Identity from a SelfSubjectReview, failing closed

```go
func selfIdentity(ctx context.Context, reviews authenticationv1client.SelfSubjectReviewInterface) (authz.Identity, error)
```

One `authentication.k8s.io/v1` SelfSubjectReview at startup, with a 10 s timeout; its
`status.userInfo` becomes the `authz.Identity` (username, UID, groups, extra). An error, or an
identity `authz.Identity.Authenticated` refuses (empty, blank, `system:anonymous`), stops the
binary with exit 1 before it listens or reads (0030:D6:R2). Local mode does not strip `system:`
names or groups: they are the user's own credential's, unlike an identity provider's claims in
0030:D6:R3.

**Authorization**: the startup review is `create` on `selfsubjectreviews`; every read after it is
preceded by `create` on `selfsubjectaccessreviews` through `authz.NewLocal`. Those are the only
creates (0030:D5:R6). The reads themselves are those of the read model, the read API and the logs
producer: `get`, `list`, `watch` on the OPM kinds and inventory kinds, `list` on events, `get` on
`pods` and `pods/log`; never `secrets`.

### Wiring

```go
checker := authz.NewLocal(cs.AuthorizationV1().SelfSubjectAccessReviews(), self, authz.Options{})
model   := readmodel.New(readmodel.Config{Dynamic: dyn, Discovery: cs.Discovery(), Authorizer: checker, Reader: self, Namespaces: ns})
model.Start(ctx)
logsP   := logs.New(logs.Config{Source: logs.ClientSource(cs), Reach: model, Authorizer: checker, Reader: self, Options: logs.Options{Logger: log}})
gate    := auth.NewLocal(auth.LocalConfig{Identity: self, Port: port, Landing: api.Prefix + "/clusters/default/instances", Logger: log})
srv     := api.New(api.Config{Model: model, Authorizer: checker, Reader: self, Authenticate: principal(gate), Producers: stream.Mux{stream.KindLog: logsP}, Logger: log})
logsP.SetPublisher(srv.Broker())
http.Server{Handler: gate.Handler(srv), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute}
```

`cmd/opm-portal/serve.go` splits this into `connect` (kubeconfig, clients, identity, authorizer),
listening, `build` (start the read model, then `wire` the rest) and serving, so the read model is
stopped on every later startup failure.

The reader and the caller are the same identity in local mode, so the read model reads with the
user's RBAC and a caller can never see more than the reader does.

### `internal/api`: extra producers through `stream.Mux`

**Context**: `api.New` builds its broker over its own producer, so the logs producer has no way in.

**Options considered**:

1. Pass a whole `stream.Producer` into `api.Config` and let the caller build the Mux. The caller
   then has to know which kinds the API's own producer serves, and the API's producer is
   unexported.
2. `Config.Producers stream.Mux` for kinds the API does not produce; `New` routes every other
   kind to its own producer and refuses a Mux entry that claims one of them.

**Decision**: Option 2, plus `Server.Broker() *stream.Broker` so a producer that publishes (the
logs producer's `SetPublisher`) can be given the broker. The API's own kinds are `platform`,
`instances`, `instance`, `package`, `registration` and `events`.

### `internal/auth`: local mode's front door

```go
type LocalConfig struct {
    Identity   authz.Identity // the SelfSubjectReview identity; required, Authenticated
    Host       string         // the bound loopback IP, for the launch URL; default 127.0.0.1
    Port       int            // the bound port; the Host allowlist and the cookie name use it
    Landing    string         // where a successful launch's page moves on to; default "/"
    SessionTTL time.Duration  // default 12h
    Now        func() time.Time
    Logger     *slog.Logger
}
func NewLocal(cfg LocalConfig) (*Local, error)
func (l *Local) LaunchURL() string                                    // http://<host>:<port>/launch?token=...
func (l *Local) Authenticate(r *http.Request) (authz.Identity, string, error) // identity, session key
func (l *Local) Handler(next http.Handler) http.Handler
```

`Handler` runs, in order, on every request:

1. **Security headers**, so refusals carry them too:
   `Content-Security-Policy: default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'`,
   `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `X-Frame-Options: DENY`,
   `Cross-Origin-Opener-Policy: same-origin`, `Cross-Origin-Resource-Policy: same-origin`. The
   only HTML is the launch's hand-off page (step 4), which loads nothing, so nothing is allowed; the UI change widens the policy to `'self'`
   for its own scripts and styles and nothing more.
2. **Host allowlist**: `r.Host`, lowercased, MUST be exactly `127.0.0.1:<port>`,
   `localhost:<port>`, `[::1]:<port>` or the bound address (another `127.0.0.0/8` IP when
   `--addr` names one); anything else is `403` before any other step
   (0030:D5:R4). A DNS-rebinding page's requests carry the attacker's host name, so they stop
   here.
3. **Cross-origin protection**: `http.NewCrossOriginProtection().Handler`, which refuses a
   non-safe method whose `Sec-Fetch-Site` or `Origin` says it came from another origin. The read
   API is GET only and answers any other method `405`; this keeps a later form or `POST` safe by
   default.
4. **Launch**: `GET /launch?token=T`. When the request already carries a valid session it gets
   the hand-off page whatever the token. Otherwise the token is compared in constant time
   with the one-time launch token; on a match the token is spent, a session is created, the
   cookie is set and the response is `200` with `Cache-Control: no-store` and a hand-off page
   that moves on to `Landing` through a meta refresh and a link. Not a `303`: found in review,
   a browser treats a redirect as part of the navigation that reached `/launch`, and when the
   `--open` page (`file://`) started it, Chromium, Firefox and WebKit all withhold the new
   `SameSite=Strict` cookie from the landing request. The refresh starts from the portal's own
   origin, so the landing request is same-site. `TestBrowserLaunch` (`task test:browser`)
   drives both launches in the three browsers. A wrong, missing or spent token is `403` with
   the same body for each.
5. `next`.

`Authenticate` returns the identity and a session key for a request carrying a live session
cookie, and an error otherwise; the read API turns the error into `401 unauthenticated` before
anything else runs (0030:D5:R3). The session key handed to the broker is a separate random id,
not the cookie value, so a broker log line can never carry the credential.

**Token and session**: `crypto/rand.Text()` (26 base32 characters, 130 bits) for the launch token
and for the session cookie value; only their SHA-256 digests are kept. One launch, one session:
the token is single use, and a session lasts `SessionTTL` (12 h) from the launch.

### Cookie flags on plain-HTTP loopback

**Context**: the task asks for a `__Host-` style cookie, `Secure` where possible. A `__Host-`
cookie MUST carry `Secure`, and a `Secure` cookie set over `http://` is accepted only by some
browsers on some loopback names (Chromium and Firefox treat `localhost` as a secure context;
Safari has not consistently, and `127.0.0.1` varies).

**Options considered**:

1. `__Host-` with `Secure` over plain HTTP. Works in some browsers, silently fails to log in in
   others.
2. Serve TLS on loopback with a generated certificate. Every browser warns on a self-signed
   certificate, and teaching users to click through that warning is worse.
3. The `__Host-` rules without the prefix: host-only (no `Domain`), `Path=/`, `HttpOnly`,
   `SameSite=Strict`, no `Secure`.

**Decision**: Option 3. The cookie is `opm-portal-<port>=<value>; Path=/; Max-Age=43200;
HttpOnly; SameSite=Strict`. The port in the name keeps two portals on one machine from
overwriting each other's cookie, because browsers do not separate cookies by port.

**Residual risk** (documented in the README): browsers send a loopback host's cookies to every
port on it, so a local process serving another port on `127.0.0.1` receives the cookie if the
user's browser visits it. A process that can do that can already read the user's files, the
kubeconfig included, when it runs as the same user; across users it needs the victim to browse to
it. TLS would not close this either.

### `--open` without the token on a command line

Passing the URL to `xdg-open` or `open` puts the token in a process's arguments, readable by any
local user through the process list. Local mode instead writes a small HTML file holding a
`<meta http-equiv="refresh">` to the launch URL into a fresh `os.MkdirTemp` directory (`0700`,
file `0600`) and opens the file's path. The directory is removed at shutdown. If the browser
cannot be started, the printed URL is the fallback. A browser that cannot read the temporary
directory (some sandboxed packages) also falls back to the printed URL.

### Output and logs

- Standard output carries one line with the launch URL, the only place the token is written.
- Standard error carries `log/slog` text logs: the listen address, the identity's username, the
  context name, startup failures and shutdown. Never the token, the cookie, a session id, the
  kubeconfig's content or an `Authorization` header.

### Shutdown

On `SIGINT` or `SIGTERM`: close the read API (its broker ends every stream, which returns the
long-lived SSE handlers), `http.Server.Shutdown` with a 5 s deadline, then stop the read model and
remove the `--open` file. A second signal is not handled specially: `Shutdown`'s deadline bounds
the wait.

## Research & Decisions

### Where the HTTP layer lives

**Context**: Principle II gives `cmd/opm-portal` flags and wiring, and `internal/auth` identity
("the local launch token in M1").
**Explored**: the constitution's package list; the read API's `Authenticate` seam.
**Options considered**:
1. All of it in `cmd/opm-portal`. Untestable without running the binary, and `main` grows a
   security layer.
2. `internal/auth` owns the launch token, the session and the request gate in front of the API
   (Host, cross-origin, headers), because each refusal is about who may reach the portal.
**Decision**: Option 2.
**Rationale**: the gate and the session are one unit to test, and M2's OIDC front door replaces
the same package's piece without touching `cmd` beyond wiring. `internal/auth` does not import
`internal/api` (the dependency runs the other way in Principle II), so `cmd` adapts its result
into an `api.Principal`.

### Launch token transport

**Context**: the token must reach exactly one browser.
**Explored**: Jupyter's launch flow (token in URL, opened through a redirect file); 0030:D5's
rejected alternative "loopback without a launch token".
**Options considered**:
1. Token in the URL, reusable for the process lifetime. Simple; a URL left in shell history or
   browser history stays a key.
2. One-time token exchanged for a cookie. A leaked URL is dead after the first use.
**Decision**: Option 2.
**Rationale**: the task asks for it, and it limits the token's exposure to the seconds between
start and first click.
