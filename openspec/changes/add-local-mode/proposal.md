## Why

`main` has every part of the portal's read path (authorization, read model, graph, health, the
change stream, pod logs and the `/api/v1alpha1` handlers) but the binary only prints its version:
nobody can use any of it. Milestone 1 of enhancement 0030 is the portal running on the user's
machine against their kubeconfig, with their RBAC as the boundary and no login code (0030:D5).
This change is that runnable binary.

## What Changes

- New subcommand `opm-portal serve` with `--kubeconfig`, `--context`, `--addr` (default
  `127.0.0.1:0`), `--namespaces` and `--open`. It refuses any non-loopback address before it
  touches a cluster (0030:D5:R2).
- Startup: load the kubeconfig, raise client-go's request rate on the one REST config every
  client is built from (dynamic, discovery and the review client, QPS 50 and burst 100), learn the
  kubeconfig's identity with a SelfSubjectReview and refuse to start when it is empty or anonymous,
  then build the local authorizer, the read model, the logs producer, the read API and the
  local-mode HTTP layer, and serve until `SIGINT` or `SIGTERM`, then shut down in order.
- New package `internal/auth`, local mode's front door (0030:D5:R3/R4):
  - a one-time launch token, printed in the URL on standard output (never in a log line) and,
    with `--open`, handed to the browser through a private redirect file instead of a command
    line other local users could read;
  - the token exchanged for a session cookie (`HttpOnly`, `SameSite=Strict`, host-only,
    `Path=/`); local mode serves plain HTTP, so the cookie carries no `Secure` flag and therefore
    no `__Host-` prefix, which requires it;
  - a `Host` allowlist (`127.0.0.1:<port>`, `localhost:<port>`, `[::1]:<port>`) refusing
    everything else, against DNS rebinding;
  - `http.CrossOriginProtection` on every non-safe method;
  - a strict Content-Security-Policy and security headers on every response, refusals included;
  - the `Authenticate` hook: the SelfSubjectReview identity, only for a request with a valid
    session.
- `internal/api`: `Config.Producers` routes topic kinds the read API does not produce itself
  through a `stream.Mux` beside its own producer, and `Server.Broker` exposes the broker a producer
  publishes to. Local mode routes `log:` topics to the logs producer, so the change stream serves
  pod logs. The OpenAPI document's stream description names the log topics (additive).
- An e2e test that runs the built binary against the kind fixture cluster, and README and
  `AGENTS.md` updates saying how to run it.

## Capabilities

### New Capabilities

- `local-mode`: how the portal runs on a user's machine: loopback only, launch token and session
  cookie, `Host` allowlist, cross-origin protection, response headers, the kubeconfig identity,
  and shutdown.

### Modified Capabilities

- `build-and-release`: the binary gains the `serve` subcommand; only `version` still opens no
  listener.
- `read-api`: the change stream serves log topics through the producer local mode routes to it.

## Impact

- Packages: new `internal/auth` (imports `internal/authz` for the identity type only);
  `cmd/opm-portal` wires every package; `internal/api` gains `Config.Producers` and
  `Server.Broker`. No UI page: after the launch the browser lands on the instance list document,
  until `internal/ui` exists.
- Principle V: the portal reads as the kubeconfig's identity, through that identity's RBAC. It
  creates exactly two kinds of object, both review APIs the API server stores nothing for:
  `authentication.k8s.io` `selfsubjectreviews` once at startup, and `authorization.k8s.io`
  `selfsubjectaccessreviews` before each read (0030:D5:R6). No Secret is read. An empty or
  anonymous kubeconfig identity stops the binary before it listens. The launch token and the
  session cookie never appear in a log line, an error or an API response.
- Principle VII: no new dependency. client-go's `clientcmd` (already a dependency through
  client-go) loads the kubeconfig; the standard library supplies flags, the cross-origin check and
  the server.
- SemVer: MINOR after 1.0 (a new command). On the 0.x line it ships as `feat(local)` and cuts a
  minor release.
- Enhancement link: completes 0030:D5 (R1 to R7) and makes 0030:D10 observable: the logs producer
  landed its requirements behind the broker, and this change serves its topics on the read API.
