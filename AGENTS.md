# opm-portal repository guide

A Go web portal over OPM clusters: a versioned JSON/SSE read API (`/api/v1alpha1`) with an HTMX UI as
its first consumer. V1 is read-only: status as the controller reports it, events, pod logs and a
relationship graph, from the Platform down to Pods.

- Milestone 1 runs locally with the user's kubeconfig: `opm-portal serve` puts
  `internal/auth`'s launch token and front door before `internal/api` (under `/api/v1alpha1`) and
  `internal/ui` (every other path). `opm-portal version` only prints the version.
- Milestone 2 (future, `ROADMAP.md`) runs in cluster with OIDC and SubjectAccessReview as the user. Its
  authorizer and safeguards are on `main`, unwired. OIDC was removed.
- Design record: `docs/DESIGN.md` (decisions, open questions, risks). Principles: `CONSTITUTION.md`.
  OpenSpec rules: `openspec/config.yaml`. Plan and progress: `ROADMAP.md`.

## Branch model

`main` is the only long-lived branch and carries the 0.x line. Nothing is pushed to it: every change
lands by PR. Squash merges carry the PR title only, so the title is the CHANGELOG entry.

## Repository rules

- `Taskfile.yml` is the authority for build, lint and test. Do not add a `Makefile`.
- Plan a change as an OpenSpec change under `openspec/changes/`. The archive commit rides the
  implementing PR. The PR that lands a change updates `ROADMAP.md`.
- A change that alters or adds a portal decision edits `docs/DESIGN.md` in the same PR and keeps every
  existing number. Reason: code and specs cite decisions by number (see Citations).
- Packages arrive with the change that needs them, never ahead of it. The planned boundaries are in
  `openspec/config.yaml`, Principle II.
- `/api/v1alpha1` changes only additively (Principle III). `task api:breaking` enforces it.
- Standard library first: `net/http` routing, `html/template`, `log/slog`. A dependency earns its place
  in the change that adds it.
- Claims about controller output are tested against live cluster captures, not hand-written YAML.
- The portal publishes no CUE module and needs no local registry. A fixture it ever publishes lives
  under `testing.opmodel.dev/*`, never under `opmodel.dev/*`.

## Security Rules

A portal renders cluster data to people and, from milestone 2, authenticates them. `CONSTITUTION.md`,
Principle V, holds the binding rules: read-only with the review-API exceptions per mode, no Secret
reads, no `spec.values`, act as the user, fail closed on an empty or unmapped identity, no tokens in
logs. A change that bends a rule needs a decision in `docs/DESIGN.md` first. These rules add to
Principle V and live only here:

- **No write verb in any role.** No ClusterRole or Role grants one, except `create` on the review APIs
  of Principle V (portal:D11). The RBAC then matches the read-only rule.
- **Authorize before the lookup** (portal:D7). Every request is authorized on verb, resource, namespace
  and name before anything is looked up. A caller who may not read a kind gets the same refusal for an
  existing and a missing object (portal:D7:R1), so a refusal does not reveal which objects exist.
- **An authorization error is a denial.** A review that errors or times out denies the read (portal:D6).
- **No secrets in logs, errors or responses.** Cookies and `Authorization` headers join the tokens and
  kubeconfig content of Principle V, and also stay out of every API response.
- **Strip the last-applied annotation.** `kubectl.kubernetes.io/last-applied-configuration` is removed
  from every object served, because a client-side apply copies the full `spec.values` into it.
- **No false comfort on values.** Secret markers live in the module's schema, not in the stored
  values, so there is nothing to mask on (portal:D8). Hiding `spec.values` does not hide what they
  became: a value rendered into a ConfigMap entry or a container's environment shows to anyone who
  may read that object, as in `kubectl`. The portal's documentation says so (portal:D8:R4).
- **Untrusted text everywhere.** Condition messages, event notes, labels, annotations and log lines
  come from whoever can write to the cluster. Render them through `html/template` only, never
  `text/template`, under a strict Content-Security-Policy with no inline script or style.
- Run the workspace `security-audit` skill before each milestone's release.

## Citations

Decisions in `docs/DESIGN.md` are cited as `portal:Dn`. This form is repo-local. Cite once, at the
symbol, and only when the rationale lives in a decision.

- `portal:D6/D7` for several decisions; `portal:D9:R2` for one requirement; `portal:D9:R1/R2` for several under
  one decision; `portal:OQ20` for an open question. A bare `D9` names nothing, because numbers restart per source.
- Cite an enhancement by id beside a reason in words, such as "(enhancement 0027, the self-service
  kinds)". Never a decision number of an enhancement, and never a lone id. `task cite-lint` checks
  this. Never write a `0030:` citation: that enhancement was withdrawn and its numbers moved into
  `docs/DESIGN.md` as `portal:Dn`.
- Never cite a section, slice, phase, task or design-doc-local number (`§8.1`, `slice C2`). They are
  not stable. A requirement number (`R2`) is.
- No citation in scaffold templates, generated files, fixtures a user copies, or user-facing strings
  (pages, API errors, log lines). They reach people who do not read the design record.
- No `Was:` rename history. `git log` owns it.

## Layout

```text
cmd/opm-portal/   main: version, and serve (local mode's flags and wiring)
api/v1alpha1/     wire types of the read API, no logic
internal/         auth, authz, readmodel, health, graph, stream, logs, api, ui, version
deploy/           kustomization that runs local mode in a Pod as a test tool (portal:D13), with its tests
openapi/          the read API contract, held to the code by internal/api's tests
docs/DESIGN.md    the design record
docs/design/      evidence/: live captures and research that DESIGN.md cites
docs/site/        site pages, published as the opm-portal docs bundle (docs-kit.cue)
hack/             release-pin gate, API breaking-change gate
test/e2e/         throwaway kind fixture cluster: pins, scripts, fixture set F1
test/browser/     Playwright scripts for the browser tests
testdata/         committed cluster captures for golden suites (generated)
openspec/         config, main specs, changes
.github/          workflows (Lint, Test, PR Title, Release, E2E, Docs, Cite lint) and release guard scripts
```

## Commands

| Command | What it does | When |
| --- | --- | --- |
| `task check` | fmt, vet, lint, openspec, test, capture check | Before committing Go changes |
| `task build` / `task run -- serve --open` | Build `bin/opm-portal` / run local mode against the current kubeconfig context | Local use |
| `task fmt`, `task vet`, `task lint` (`lint:fix`) | gofmt and goimports, `go vet` (with `e2e,browser` tags), golangci-lint | Parts of `check` |
| `task test` | `go test -race ./...` (needs cgo and a C compiler) | Part of `check` |
| `go test ./<package> -run <Name>` | One test | Iterating |
| `task openspec:check` | `openspec validate --all --strict` (`task openspec:install` first) | Before committing an OpenSpec artifact |
| `task cite-lint` | Reject enhancement citations by decision number or lone id (needs the Go proxy) | After editing comments |
| `task deps:release-check` | Release-pin gate G1 | On release PRs |
| `task api:breaking` | Fail on a breaking `openapi/` change unless the title has `!` | After an API change |
| `task docs:bundle:check` | Build and lint the docs bundle | After editing `docs/site/` or `docs-kit.cue` |
| `task e2e:up` / `e2e:down` | Create or delete the fixture cluster | Before and after e2e work |
| `task e2e:local`, `e2e:m1`, `e2e:pod` | Run the e2e tests against it | After changes to serve, reads or `deploy/` |
| `task e2e:capture` / `e2e:capture:check` | Recapture the cluster / verify the committed captures | After a pin move / always |
| `task test:browser` | Browser tests in Chromium, Firefox and WebKit (needs the network) | After changes to auth or the page script |
| `actionlint` | Lint workflows | After a workflow change |

## Style

- Go: the `go` directive in `go.mod` sets the version (CI reads it). golangci-lint v2 with the gofmt
  and goimports formatters; CI pins the version in the Lint workflow.
- Import groups: stdlib, third-party, local module. No unused helpers, no speculative abstractions.
- Names: lowercase package names; explicit lowerCamelCase JSON tags; concrete structs over
  `map[string]any` in API types.
- Wrap errors with context: `fmt.Errorf("listing module instances: %w", err)`. Error messages are
  lowercase unless a proper noun or identifier. No silent error swallowing.
- Log with `log/slog`, balanced key/value pairs, never a credential.
- Tests: table-driven, standard `testing` package. Use the lightest tier that proves the behavior:
  unit, then envtest, then kind e2e.
- Comments say what the code does and why, in their own words. Default is no decision citation.

@CONTEXT.md
