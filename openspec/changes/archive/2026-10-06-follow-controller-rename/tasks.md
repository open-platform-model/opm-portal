## 1. Rename the wire fields (api/v1alpha1, internal/readmodel, internal/graph, internal/api, openapi)

- [x] 1.1 `internal/readmodel`: `OwnerController Owner = "controller"`, `ownerOf` returns it for any owner but `cli`; `PlatformView.ControllerVersion` read from `status.controllerVersion` only
- [x] 1.2 `internal/graph` and `internal/api`: `ControllerVersion` on the graph platform and in the converters; `api/v1alpha1`: `Platform.ControllerVersion` and `GraphPlatform.ControllerVersion` (`json:"controllerVersion,omitempty"`)
- [x] 1.3 `openapi/v1alpha1.yaml`: owner `x-extensible-enum: [controller, cli]`, `controllerVersion` on Platform and GraphPlatform; `enum_contract_test` names `readmodel.OwnerController`
- [x] 1.4 Regenerate the goldens with `-update`
- [x] 1.5 `task check` green and `PR_TITLE='feat(api)!: ...' task api:breaking` passes, then commit `feat(api)!: name the controller in owner and version fields`

## 2. Call the product the controller in pages, explanations and code (internal/ui, internal/health, internal/api, internal/readmodel, internal/graph)

- [x] 2.1 `internal/ui` templates and CSS comments: "Controller says", "the controller", "the controller's record"; the Platform page's "Controller version" reads `ControllerVersion`
- [x] 2.2 `internal/health`: explanation text and the "not an OPM controller kind" message; `OPM_CONTROLLER_SRC` for the reason-source test
- [x] 2.3 Identifiers, comments and test names across `internal/` and `api/` (`omitControllerText`, `controllerReported`, `opmController`, `controllerGroup`, `controllerKinds`); OpenAPI descriptions; regenerate the goldens
- [x] 2.4 The Installed list shows the read API's owner as served: drop `ownerText`, which mapped the old owner value to "controller"
- [x] 2.5 `task check` green, then commit `refactor: call the product the controller in pages and code`

## 3. Install and read the controller in the e2e cluster (test/e2e, cmd/opm-portal)

- [x] 3.1 `versions.env`: `OPM_CONTROLLER_VERSION=0.1.0`, the `opm_controller` module version `opm controller install --version` takes; the cli pin marked `TODO(cutover)` until the cli release with `opm controller` exists
- [x] 3.2 `lib.sh`, `up.sh`: the `OPM_CONTROLLER_VERSION` override and `opm controller install`; `capture.sh`: Deployment `opm-controller-manager` in `opm-system`, `meta.yaml` key `controller:` read from `status.controllerVersion`, default owner `controller`; `dump.sh`: `controller.log` from the same Deployment
- [x] 3.3 F1 fixtures and `cmd/opm-portal/m1_e2e_test.go`: `testing.opmodel.dev/*/controller/*` and owner `controller`; Taskfile descriptions
- [x] 3.4 F1 fixtures pin podinfo `v0.1.12` and backup_provider and backup_consumer `v0.1.1`, the versions the controller fixtures publish
- [x] 3.5 `task check` green, then commit `test(e2e): install the controller under its new names`

## 4. Docs and specs (docs, README, ROADMAP, AGENTS, CONSTITUTION, openspec)

- [x] 4.1 `docs/site` pages, `docs/DESIGN.md`, README, AGENTS, CONSTITUTION and `openspec/config.yaml` say "controller"; human and upstream "operator" stay
- [x] 4.2 `docs/design/evidence` prose renamed
- [x] 4.3 `health-evaluation` main spec Purpose line (a delta cannot carry it)
- [x] 4.4 `task check` and `task docs:bundle:check` green, ROADMAP updated, then commit `docs: call the product the controller`

## 5. Pin the wire reads (internal/readmodel, internal/ui)

- [x] 5.1 `TestOwnerOf` (cli, controller, absent)
- [x] 5.2 `task check` green, then commit `test(readmodel): pin the owner and controller version reads`

## 6. Keep captured evidence verbatim (internal/health/testdata, testdata/clusters/f1, docs/design/evidence)

- [x] 6.1 Every captured file stays as main holds it: `internal/health/testdata/*.yaml` (provenance headers, `operatorVersion` keys), `testdata/clusters/f1/**` (`meta.yaml` included), the evidence samples, inputs and outputs with their file names (`events-operator-*.yaml`) and header lines, and the test literals of captured fixture paths
- [x] 6.2 `TestPlatformViewControllerVersion` (read model) and `TestPlatformNamesTheControllerVersion` (API document, platform graph and page) set `status.controllerVersion` on the captured Platform; the captured F1 Platform shows none, and `TestPlatformStatusLinksItsReasons` expects "not recorded"; regenerate the goldens with `-update`
- [x] 6.3 `docs/design/evidence/README.md` says the captures predate the rename and keep every captured name; the 01 capture README names what was installed and cited by its name then (opm-operator v1.0.0-beta.5, its PRs and issues)
- [x] 6.4 The web-ui requirement "Pages use the words a platform team uses" shows the owner values as served; the deltas are derived from the main specs the redesign left
- [x] 6.5 `task check` green
