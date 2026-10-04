## 1. Read API: route other producers through the stream Mux (internal/api, openapi)

- [x] 1.1 `internal/api/server.go`: `Config.Producers stream.Mux`; `New` routes the API's own kinds (`platform`, `instances`, `instance`, `package`, `registration`, `events`) to its producer and refuses a `Producers` entry for one of them; `Server.Broker()`
- [x] 1.2 `openapi/v1alpha1.yaml`: name the `log:` topics and their `log` and `logend` events in the stream description (additive)
- [x] 1.3 Tests: a fake `log:` producer routed through `Producers` delivers its snapshot on the API stream; without it a log topic is `400 bad_request`; a `Producers` entry for `instance` makes `New` fail
- [x] 1.4 `task check` green, then commit `feat(api): route log topics to a mode's producer`

## 2. Local front door (internal/auth)

- [x] 2.1 `internal/auth/local.go`: `LocalConfig`, `NewLocal` (identity must be authenticated, port required), the one-time launch token and the session (digests only, `crypto/rand.Text`), `LaunchURL`, `Authenticate`
- [x] 2.2 `internal/auth/handler.go`: `Handler` with security headers, the Host allowlist, `http.CrossOriginProtection`, `GET /launch`, then the next handler
- [x] 2.3 Unit tests: token exchange (success, wrong, missing, spent, method), a live session redirects whatever the token, cookie flags and name, session expiry, Host allowlist (three loopback forms pass, others and wrong port refused), cross-origin POST refused and same-origin passed, headers on every response including refusals, no token or cookie in log output
- [x] 2.4 `task check` green, then commit `feat(auth): admit a local browser through a one-time launch token`

## 3. The serve command (cmd/opm-portal)

- [x] 3.1 `cmd/opm-portal/main.go`: `run(ctx, ...)` with the `serve` subcommand and its flags; loopback address check before the kubeconfig is read; usage line
- [x] 3.2 `cmd/opm-portal/serve.go`: kubeconfig loading, `TuneConfig` on the one REST config every client is built from, `selfIdentity` through a SelfSubjectReview failing closed, the wiring of authz, read model, logs producer, read API and the gate, listen and re-check, the launch URL on stdout, `--open` through a private redirect file, graceful shutdown
- [x] 3.3 Tests: flag errors and the non-loopback refusal exit 2 without reading the kubeconfig; `selfIdentity` refuses an empty and an anonymous user and a failed review, and maps user info; the bound-address re-check; the `--open` file is private and holds the URL; updated `TestRun`
- [x] 3.4 `task check` green, then commit `feat(local): serve the read API locally against a kubeconfig`

## 4. End to end and docs (cmd/opm-portal, Taskfile, README, AGENTS.md)

- [x] 4.1 `cmd/opm-portal/e2e_test.go` (build tag `e2e`): build the binary, run `serve` against the fixture cluster's kubeconfig, launch, then fetch the instance list and the `default/podinfo` graph with the session cookie, and get `401` without it and `403` for a foreign `Host`; `task e2e:local`
- [x] 4.2 Run it against a throwaway podman cluster `opm-portal-e2e-local` (`E2E_CLUSTER=opm-portal-e2e-local task e2e:up`), then delete the cluster
- [x] 4.3 README: how to run local mode, what it reads and creates, the cookie flags on plain HTTP and the residual cross-port risk; `AGENTS.md`: layout, commands, the binary's modes; `Taskfile.yml` `run` description
- [x] 4.4 `task check` green, then commit `test(e2e): run the local-mode binary against the fixture cluster`
