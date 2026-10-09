## Why

The OPM Kubernetes controller is renamed: repository, image, namespace `opm-system`,
Deployment `opm-controller-manager`, CLI noun `opm controller`, test fixtures under
`testing.opmodel.dev/*/controller/*`. Its custom resources change two fields hard, with no alias:
`spec.owner` takes `cli` or `controller`, and the Platform reports `status.controllerVersion`. The
owner decided the rename leaves no migration path, no aliases and no traces (enhancement 0032,
"The operator becomes the controller"). The portal reads
those fields, names the product on every page and in its read API, and installs it in its e2e
cluster, so it must follow before its first release (0.1.0, held until this merges).

## What Changes

- **BREAKING** Read API: `InstanceSummary.owner` takes `[controller, cli]` (was the old product
  name); `Platform.controllerVersion` and `GraphPlatform.controllerVersion` replace the old
  version field. The read model reads `status.controllerVersion` only, with no fallback to the
  old field. Go: `readmodel.OwnerController`, `ControllerVersion` fields, `api/v1alpha1` types,
  `enum_contract_test`.
- Pages, health explanations, API descriptions, comments and test names call the product "the
  controller" (labels such as "Controller says", "the controller's record"). Reason keys are
  unchanged. The Platform page's "Controller version" reads `controllerVersion`, and the
  Installed list shows the owner as the read API serves it, with no mapping from the old value.
- e2e: `versions.env`, `lib.sh`, `up.sh`, `capture.sh` and `dump.sh` install and read the
  controller under its new names. `OPM_CONTROLLER_VERSION` is the `opm_controller` module version
  `opm controller install --version` takes, fixing the old pin, which named an image tag where the
  flag takes a module version. The F1 fixtures name `testing.opmodel.dev/*/controller/*`.
- Captured data stays verbatim: the F1 capture, `internal/health/testdata`, the design
  evidence and the test literals of captured fixture paths keep every name, key and provenance
  line as captured, until the F1 recapture at
  cutover. The F1 Platform wrote `status.operatorVersion`, so it shows no controller version
  until then; a test sets `status.controllerVersion` to pin the read.
- Docs: README, ROADMAP, AGENTS, CONSTITUTION, `openspec/config.yaml`, `docs/DESIGN.md`, the
  design evidence prose and `docs/site` pages.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `read-api`: owner and version fields use the controller's names; prose and one requirement
  title rename.
- `read-model`, `graph-model`, `health-evaluation`, `web-ui`, `portal-docs`: prose and
  requirement or scenario names say "controller".
- `kind-e2e-environment`: installs with `opm controller install --version <module version>`;
  scenario names say "controller".

## Impact

- Packages: `api/v1alpha1`, `internal/readmodel`, `internal/graph`, `internal/api`,
  `internal/health`, `internal/ui`, `cmd/opm-portal` (e2e test), `test/e2e`, `openapi/`, docs.
  No new read, no new verb, no new dependency; `go.mod` does not depend on the controller.
- Principle V: unchanged. The portal reads the same objects with the same grants.
- Principle III: the change removes and renames fields inside `v1alpha1` instead of opening a new
  API version. The portal has never released, so no client holds the old names, and the owner's
  rename decision rules out a compatibility version (design.md).
- SemVer: MAJOR after 1.0 (a removed enum value and a renamed response field); on the 0.x line it
  ships as `feat(api)!:` and bumps the minor version. `task api:breaking` passes only with that
  PR title.
- Coupling: merges after the controller release that renames the fields and the cli release that
  has `opm controller`; the e2e pins and the F1 recapture wait for those releases.
