## Context

The controller renames two fields of its custom resources with no alias: `spec.owner` takes
`cli` or `controller`, and the Platform reports `status.controllerVersion`. The portal reads both
(`internal/readmodel/parse.go` `ownerOf`, `internal/readmodel/platform.go`) and serves them in
`/api/v1alpha1`. Everything else is a name: the product in prose and labels, the namespace and
Deployment the e2e scripts read, the CLI noun, the fixture paths.

## Goals / Non-Goals

**Goals:** the portal's code, read API, pages, scripts and docs use the controller's names, and
leave no alias or fallback for the old ones.

**Non-Goals:** recapturing F1 (needs the released controller and cli; done at cutover); rewriting
what a live cluster wrote into captured files; changing reason keys or behavior.

## Decisions

### Wire shape

```go
const (
    OwnerController Owner = "controller"
    OwnerCLI        Owner = "cli"
)
type PlatformView struct { /* ... */ ControllerVersion string }
// api/v1alpha1
type Platform struct { /* ... */ ControllerVersion string `json:"controllerVersion,omitempty"` }
type GraphPlatform struct { Type string; ControllerVersion string `json:"controllerVersion,omitempty"`; /* ... */ }
```

`ownerOf` returns `OwnerCLI` for `spec.owner: cli` and `OwnerController` otherwise, as before.
The Platform reader reads `status.controllerVersion` only. The OpenAPI document lists
`x-extensible-enum: [controller, cli]` and the `controllerVersion` property. No error case
changes.

## Research & Decisions

### Rename in place or open a new API version

**Context**: Principle III says removing or renaming a field needs a new API version.
**Options considered**:
1. `v1alpha2` beside `v1alpha1` - keeps the rule, but serves the old names for a version nobody
   ever received, and the owner's rename decision forbids aliases and traces.
2. Rename in `v1alpha1` - one breaking change on a pre-release, unreleased API.
**Decision**: rename in `v1alpha1`, marked `feat(api)!:`.
**Rationale**: the portal has never released (release PR for 0.1.0 is held for this change), so
no client holds the old names; `task api:breaking` still guards the break and passes only with
the `!` title.

### Captured data

**Context**: `testdata/clusters/f1/**`, `internal/health/testdata/**` and the design evidence
samples are what a live cluster wrote; AGENTS.md requires claims about controller output to rest
on live captures, not hand-written YAML.
**Options considered**:
1. Rewrite every old name in the captures - the files would claim namespaces, image references,
   UIDs and digests no cluster ever wrote.
2. Rename only the schema keys the code reads, keep the rest until the F1 recapture.
3. Keep every captured file verbatim, including keys, `meta.yaml`, sample file names and the
   provenance header lines, and pin the new read with a test that sets the new key.
**Decision**: option 3, the owner's rename decision that captured evidence stays verbatim. The
F1 Platform carries `status.operatorVersion`, which the portal no
longer reads, so the captured platform shows no controller version and the goldens carry none
until the recapture at cutover replaces F1. `TestPlatformViewControllerVersion` (read model) and
`TestPlatformNamesTheControllerVersion` (API document, platform graph and page) set
`status.controllerVersion` on the captured Platform to pin the read. The provenance headers of
`internal/health/testdata` and the evidence samples keep naming opm-operator, the name the
captured release shipped under, and the capture README names what was installed and cited by its
name then (opm-operator v1.0.0-beta.5, its PRs and issues); the evidence README says the captures
predate the rename.
**Rationale**: no fabricated cluster output, and no captured record rewritten to a name it never
carried; the tests still cover the renamed read.

### e2e controller version

**Context**: `up.sh` passed `--version v1.0.0-beta.6`, an image tag, while `opm operator install
--version` takes the module version (`X.Y.Z` pins, `vN` floats).
**Decision**: `OPM_CONTROLLER_VERSION=0.1.0`, the first `opm_controller` module version, passed
to `opm controller install --version`. The cli pin (`OPM_CLI_VERSION` and its four sha256 values)
cannot be known until the cli release with `opm controller` exists; it is marked
`TODO(cutover)` and moved then.
The F1 fixtures pin the module versions the controller's fixture set publishes under
`testing.opmodel.dev/modules/controller` (podinfo 0.1.12, backup_provider and backup_consumer
0.1.1); the older versions exist only under the retired operator paths.

### The redesigned pages

**Context**: the web UI redesign (change `redesign-web-ui`, archived on main before this change)
labels the version field "Controller version" and maps the owner value of the old name to
"controller" on the Installed list, with the web-ui requirement "Pages use the words a platform
team uses" saying so.
**Decision**: the label reads `ControllerVersion`; the mapping goes, because the wire value is now
`controller` and the pages show `controller` and `cli` as the read API serves them. The web-ui
requirement is restated to match.
**Rationale**: a mapping from a value the read API no longer serves is dead code.

## Risks / Trade-offs

- [Captured data keeps old names until the recapture] -> listed in the cutover notes with the
  exact commands; the residue is confined to captured files and goldens derived from them.
- [The e2e scripts call `opm controller` while the pinned cli is still the old release] -> the e2e
  workflow runs on a schedule, never on pull requests; the cli pin moves at cutover before the
  recapture.
