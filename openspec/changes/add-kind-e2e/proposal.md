## Why

Every claim the portal makes about operator output is to be tested against a live cluster
capture, never hand-written YAML (`AGENTS.md`, "Testing Style"). The only capture so far
(enhancement 0030, experiment 01) was taken by hand with scratch scripts, kept a trimmed sample
in the enhancements repo, and cannot be rerun when the operator pin moves. The read model, graph,
health and read API changes need a reproducible fixture cluster and a committed capture to seed
their golden suites, and the milestone 1 e2e (a later part of this change line) needs the same
cluster to run against.

## What Changes

- A scripted, throwaway kind environment for the portal: `task e2e:up` creates the cluster
  `opm-portal-e2e` (podman locally, as the owner chose; docker in CI) with its kubeconfig in the
  gitignored `.e2e/` directory, installs the released opm-operator through the released `opm`
  CLI (version and per-platform sha256 pinned in `test/e2e/versions.env`, download verified
  before use), applies the fixture set F1 and waits for it to settle. `task e2e:down` deletes
  the cluster. Every `kubectl` and `opm` call names the kubeconfig and context explicitly, and
  the scripts refuse any other context.
- Fixture set F1 under `test/e2e/fixtures/f1/`: cert-manager
  (`opmodel.dev/modules/cert_manager@v2`, with an applier ServiceAccount bound to cluster-admin,
  because the operator's own ServiceAccount cannot patch CRDs), the operator's podinfo test
  module, a ModulePackage on a cluster without Flux (the `SourceNotReady` state), the backup
  provider and consumer test modules from opm-operator (`testing.opmodel.dev/...`), and a
  CLI-owned `web_app` instance applied with `opm instance apply`.
- `task e2e:capture`: snapshots the four OPM kinds, the non-Secret objects their inventories
  name, the ReplicaSets and Pods below them, and the events in the fixture namespaces into
  `testdata/clusters/f1/`, with `managedFields`, the last-applied annotation and every instance's
  and package's `spec.values` removed, and never a Secret. A check script refuses a capture that
  breaks any of those rules. `meta.yaml` records the versions and the verdict each registration
  got.
- One committed capture from a run on a new throwaway podman cluster, deleted afterwards.
- A CI workflow `E2E` (manual dispatch and a nightly schedule, not on pull requests, because it
  pulls from GHCR) that runs `e2e:up` and `e2e:capture` on docker kind and uploads the capture as
  an artifact.

The backup claim is accepted and active only on an operator built on library v1.0.0-beta.2 or
later, and no released operator is (opm-operator release PR 208 is open). The environment does
not fail on that: it records the verdict the released operator gives, and `meta.yaml` marks a
refused claim as such.

## Capabilities

### New Capabilities

- `kind-e2e-environment`: the throwaway fixture cluster, its pins and guards, the F1 fixture
  set, and the capture rules (what is captured, what is stripped, what is never read).

### Modified Capabilities

None.

## Impact

- New: `test/e2e/` (scripts, pins, fixtures), `testdata/clusters/f1/`,
  `.github/workflows/e2e.yml`; `Taskfile.yml` gains `e2e:*` tasks; `.gitignore` gains `/.e2e/`.
- No Go package, API resource or UI page changes. No Go dependency.
- Principle V: the capture reads as the cluster-admin kubeconfig kind writes, on a throwaway
  cluster only. It never gets, lists or watches Secrets (the inventory walk skips the kind), and
  it strips `spec.values` and the last-applied annotation, so the committed testdata holds what
  the portal itself may serve. The portal binary is unchanged.
- SemVer: none (test and CI only); ships under a `test:` title, so no release.
- Complexity (Principle VII): shell scripts plus `kubectl` and `yq`, which GitHub's Ubuntu
  runners carry; no new Go code. The nightly job is the only way to notice a released operator
  or fixture change that breaks the environment, without pulling from GHCR on every pull request.
- Implements no 0030 decision; it produces the captures the 0030 implementations test against,
  so it carries no `enhancement.yaml`.
