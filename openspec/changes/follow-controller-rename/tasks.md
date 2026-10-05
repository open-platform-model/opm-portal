## 1. Rename the wire fields (api/v1alpha1, internal/readmodel, internal/graph, internal/api, openapi)

- [x] 1.1 `internal/readmodel`: `OwnerController Owner = "controller"`, `ownerOf` returns it for any owner but `cli`; `PlatformView.ControllerVersion` read from `status.controllerVersion` only
- [x] 1.2 `internal/graph` and `internal/api`: `ControllerVersion` on the graph platform and in the converters; `api/v1alpha1`: `Platform.ControllerVersion` and `GraphPlatform.ControllerVersion` (`json:"controllerVersion,omitempty"`)
- [x] 1.3 `openapi/v1alpha1.yaml`: owner `x-extensible-enum: [controller, cli]`, `controllerVersion` on Platform and GraphPlatform; `enum_contract_test` names `readmodel.OwnerController`
- [x] 1.4 Captured data: rename only the keys the code reads (`status.controllerVersion` in `testdata/clusters/f1/platforms.yaml` and `internal/health/testdata/platform-*.yaml`); regenerate the goldens with `-update`
- [x] 1.5 `task check` green and `PR_TITLE='feat(api)!: ...' task api:breaking` passes, then commit `feat(api)!: name the controller in owner and version fields`

## 2. Call the product the controller in pages, explanations and code (internal/ui, internal/health, internal/api, internal/readmodel, internal/graph)

- [ ] 2.1 `internal/ui` templates and CSS comments: "Controller says", "the controller", "the controller's record", "controller vX"
- [ ] 2.2 `internal/health`: explanation text and the "not an OPM controller kind" message; `OPM_CONTROLLER_SRC` for the reason-source test
- [ ] 2.3 Identifiers, comments and test names across `internal/` and `api/` (`omitControllerText`, `controllerReported`, `opmController`, `controllerGroup`, `controllerKinds`); OpenAPI descriptions; regenerate the goldens
- [ ] 2.4 `task check` green, then commit `refactor: call the product the controller in pages and code`

## 3. Install and read the controller in the e2e cluster (test/e2e, cmd/opm-portal)

- [ ] 3.1 `versions.env`: `OPM_CONTROLLER_VERSION=0.1.0`, the `opm_controller` module version `opm controller install --version` takes; the cli pin marked `TODO(cutover)` until the cli release with `opm controller` exists
- [ ] 3.2 `lib.sh`, `up.sh`: the `OPM_CONTROLLER_VERSION` override and `opm controller install`; `capture.sh`: Deployment `opm-controller-manager` in `opm-controller-system`, `meta.yaml` key `controller:` read from `status.controllerVersion`, default owner `controller`; `dump.sh`: `controller.log` from the same Deployment
- [ ] 3.3 F1 fixtures and `cmd/opm-portal/m1_e2e_test.go`: `testing.opmodel.dev/*/controller/*` and owner `controller`; `testdata/clusters/f1/meta.yaml` keys follow `capture.sh` (`controller:`, `owner: controller`); Taskfile descriptions
- [ ] 3.4 `task check` green, then commit `test(e2e): install the controller under its new names`

## 4. Docs and specs (docs, README, ROADMAP, AGENTS, CONSTITUTION, openspec)

- [ ] 4.1 `docs/site` pages, `docs/DESIGN.md`, README, AGENTS, CONSTITUTION and `openspec/config.yaml` say "controller"; human and upstream "operator" stay
- [ ] 4.2 `docs/design/evidence` prose renamed; the two `events-operator-*.yaml` samples renamed `events-controller-*.yaml`; captured sample content stays as captured, with one note line in `docs/design/evidence/README.md`
- [ ] 4.3 `health-evaluation` main spec Purpose line (a delta cannot carry it)
- [ ] 4.4 `task check` and `task docs:bundle:check` green, ROADMAP updated, then commit `docs: call the product the controller`
