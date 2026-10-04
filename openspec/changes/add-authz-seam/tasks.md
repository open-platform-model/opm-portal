## 1. Grant seam and fail-closed guards (internal/authz)

- [ ] 1.1 Add `k8s.io/apimachinery` at v0.36.4 (the pin the cli and opm-operator use) and `Identity` (with `Authenticated` and a canonical key), `Attributes` (with validation and a name-free `String`), `Denial` and its codes, and `Grant` (one unexported field, `issue`, `Valid`, `Covers`, copying accessors); verify `go build ./...`
- [ ] 1.2 Add `Authorizer`, the unexported `decider` and `Checker.Check` (identity guard, attribute guard, backend under a timeout, errors become denials); verify with a table test over a fake decider that empty, blank, groups-only and anonymous identities, write verbs, Secrets and wildcard attributes reach no backend, that a backend error or timeout is `unavailable`, and that a nil or zero `Checker` denies
- [ ] 1.3 Add the seal tests: every `Grant` field unexported (reflection), `testdata/forge` fails to compile naming `sealed` and `issue`, and the module AST scan (with its own forgery cases); verify `go test ./internal/authz` passes and fails when a forgery is added outside the package
- [ ] 1.4 Add the package documentation (`doc.go`); verify `go doc ./internal/authz` prints it
- [ ] 1.5 `task check` green, then commit `chore(authz): add the grant seam with fail-closed guards`

## 2. Local SelfSubjectAccessReview backend (internal/authz)

- [ ] 2.1 Add `k8s.io/client-go` and `k8s.io/api` at v0.36.4; verify `go mod tidy` leaves `go.mod` unchanged afterwards
- [ ] 2.2 Add `NewLocal` and the `selfReviewer` backend (exact attributes, one-identity binding, verdict mapping); verify with fake-clientset reactor tests for allowed, denied, no opinion, deny-over-allow, evaluation error with and without allow, API errors and timeouts
- [ ] 2.3 Add the regression tests against the fake cluster: an empty identity and a foreign identity record zero API actions, and a denied caller gets the same denial for an existing and a missing Deployment with only review actions recorded; verify `go test ./internal/authz` passes
- [ ] 2.4 `task check` green, then commit `chore(authz): add the local SelfSubjectAccessReview authorizer`

## 3. Decision cache (internal/authz)

- [ ] 3.1 Add the decision cache (TTL default 30 s, `MaxEntries` default 4096, injectable clock) and wire it into `Checker.Check` between the guards and the backend; verify with tests that allow and deny are reused within the TTL and re-asked after it, that unavailable outcomes are never stored, that identity, namespace, name and subresource separate entries while group order does not, that a full cache drops expired entries and otherwise stops storing, and that concurrent checks pass under `go test -race`
- [ ] 3.2 `task check` green, then commit `chore(authz): cache authorization decisions per identity and request`
