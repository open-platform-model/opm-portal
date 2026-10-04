## 1. Grant seam and fail-closed guards (internal/authz)

- [x] 1.1 Add `k8s.io/apimachinery` at v0.36.4 (the pin the cli and opm-operator use) and `Identity` (with `Authenticated` and a canonical key), `Attributes` (with validation and a name-free `String`), `Denial` and its codes, and `Grant` (one unexported field, `issue`, `Valid`, `Covers`, copying accessors); verify `go build ./...`
- [x] 1.2 Add `Authorizer`, the unexported `decider` and `Checker.Check` (identity guard, attribute guard, backend under a timeout, errors become denials); verify with a table test over a fake decider that empty, blank, groups-only and anonymous identities, write verbs, Secrets and wildcard attributes reach no backend, that a backend error or timeout is `unavailable`, and that a nil or zero `Checker` denies
- [x] 1.3 Add the seal tests: every `Grant` field unexported (reflection), `testdata/forge` fails to compile naming `sealed` and `issue`, and the module AST scan (with its own forgery cases); verify `go test ./internal/authz` passes and fails when a forgery is added outside the package
- [x] 1.4 Add the package documentation (`doc.go`); verify `go doc ./internal/authz` prints it
- [x] 1.5 `task check` green, then commit `chore(authz): add the grant seam with fail-closed guards`

## 2. Local SelfSubjectAccessReview backend (internal/authz)

- [x] 2.1 Add `k8s.io/client-go` and `k8s.io/api` at v0.36.4; verify `go mod tidy` leaves `go.mod` unchanged afterwards
- [x] 2.2 Add `NewLocal` and the `selfReviewer` backend (exact attributes, one-identity binding, verdict mapping); verify with fake-clientset reactor tests for allowed, denied, no opinion, deny-over-allow, evaluation error with and without allow, API errors and timeouts
- [x] 2.3 Add the regression tests against the fake cluster: an empty identity and a foreign identity record zero API actions, and a denied caller gets the same denial for an existing and a missing Deployment with only review actions recorded; verify `go test ./internal/authz` passes
- [x] 2.4 `task check` green, then commit `chore(authz): add the local SelfSubjectAccessReview authorizer`

## 3. Decision cache (internal/authz)

- [x] 3.1 Add the decision cache (TTL default 30 s, `MaxEntries` default 4096, injectable clock) and wire it into `Checker.Check` between the guards and the backend; verify with tests that allow and deny are reused within the TTL and re-asked after it, that unavailable outcomes are never stored, that identity, namespace, name and subresource separate entries while group order does not, that a full cache drops expired entries and otherwise stops storing, and that concurrent checks pass under `go test -race`
- [x] 3.2 `task check` green, then commit `chore(authz): cache authorization decisions per identity and request`

## 4. Review fixes (PR 8)

- [x] 4.1 Claim no 0030 decision in `enhancement.yaml` (0030:D7 is delivered only in part); verify the proposal's enhancement link agrees
- [x] 4.2 Leave the cause out of `DenialError.Error()` and keep it behind `Unwrap`; verify a test that an evaluation error's text does not reach the message
- [x] 4.3 Bind a `Grant` to its identity (`Covers(who, req)`) and expire it with its decision; verify tests for another identity, reordered groups, expiry of a fresh, a cached and an uncached grant
- [x] 4.4 Refuse every subresource but none, `status` and `log` before any review; verify cases for exec, attach, portforward and the proxy subresources
- [x] 4.5 Flag `sealed` assignments, `grantData` literals outside `issue` and any reference to `issue` outside `Check` in the seal scan; verify a forgery case for each
- [x] 4.6 Run `task test` under `-race`; verify `task check` is green
- [x] 4.7 `task check` green, then commit `docs(openspec): record the review fixes and the SSAR principle question`

## 5. Principle V amendment and re-check nits (PR 8)

- [x] 5.1 Flag any write through an issued grant's sealed data (`g.sealed.expires`, an index into it, an increment) and any copy of the sealed pointer in the seal scan; update the forge testdata to call `issue` with its four-argument signature; verify a forgery case for each pattern and that testdata/forge still fails to compile
- [x] 5.2 Amend Principle V (`CONSTITUTION.md`, `AGENTS.md` Security Rules, `openspec/config.yaml`) so the only allowed writes are `create` on `subjectaccessreviews`, `selfsubjectaccessreviews` and `selfsubjectreviews`, per the owner's decision; record it as resolved in design.md's Open Questions
- [x] 5.3 Align design.md's decision sentence with the no-claim `enhancement.yaml` and the proposal's commit-type sentence with the squash title
- [x] 5.4 `task check` green, then commit `docs(openspec): amend Principle V for the review APIs and close the re-check nits`
