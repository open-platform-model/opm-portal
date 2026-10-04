## Context

Enhancement 0030, experiment 01 captured a released operator on throwaway podman kind clusters
(2026-10-04): CLI v1.0.0-beta.7 installed operator v1.0.0-beta.5, the Platform subscribed
`opmodel.dev/catalogs/opm@v4` 4.5.2, and the inputs were cert-manager, podinfo, two hand-applied
registrations, a ModulePackage without Flux and a CLI-owned `web_app` instance. A second cluster
ran the backup provider fixtures from opm-operator PR 212, then unpublished. Since then PR 212
merged and its `Publish Fixtures` run put `backup`, `backup_provider` and `backup_consumer`
v0.1.0 on GHCR under `testing.opmodel.dev` (checked with `skopeo list-tags`, 2026-10-04).

Observations from that capture that shape this change (experiment 01 `README.md`):

- The operator's own ServiceAccount cannot patch CRDs, so cert-manager goes `ApplyFailed`
  unless `spec.serviceAccountName` names a cluster-admin applier (observation 1).
- The released beta.5 operator refuses the claim the backup provider renders,
  `CatalogUnresolved`, because library v1.0.0-beta.1 rejects the bare `0.1.0` the opm catalog's
  transformer writes; library v1.0.0-beta.2 fixed it, and only an unreleased operator build has
  it (observation 9).
- The last-applied annotation copies `spec.values` (observation 14).
- Events about the cluster-scoped Platform and TransformerRegistration land in `default`
  (observation 5).

## Goals / Non-Goals

**Goals:**

- One command builds the fixture cluster from released artifacts only, on podman locally and on
  docker in CI, and one command removes it.
- The committed capture is what later golden suites load, and it holds nothing the portal may
  not serve.

**Non-Goals:**

- e2e assertions through the portal's API (the milestone 1 part of this change line, after the
  read API and UI exist).
- Flux and a reconciling ModulePackage, the hand-applied refusal claims of experiment 01, and the
  scripted image break; each can join F1 in a later change.
- Pulling from a local registry: everything resolves from GHCR.

## Decisions

### Layout

```text
test/e2e/versions.env              pins: kind, node image, opm CLI version + sha256 per platform
test/e2e/lib.sh                    shared env, guards, k() / opm() wrappers, wait helper
test/e2e/up.sh, down.sh            create, install, apply F1, settle / delete
test/e2e/capture.sh                snapshot into testdata/clusters/f1 (or $1)
test/e2e/check-capture.sh          refuse a capture that breaks the strip rules
test/e2e/fixtures/f1/*.yaml        operator-owned fixtures, applied in name order
test/e2e/fixtures/f1/web/          CLI-owned instance package (opm instance apply)
testdata/clusters/f1/              committed capture + README.md
.e2e/                              gitignored: kubeconfig, opm binary, CUE cache
```

### Guards

`lib.sh` fixes the cluster name `opm-portal-e2e`, the context `kind-opm-portal-e2e` and the
kubeconfig `.e2e/kubeconfig`. `k()` and `opm()` pass `--kubeconfig` and `--context` on every
call and the scripts unset `KUBECONFIG`, so a developer's current context is never read or
written. Before any write, `up.sh` checks that the kubeconfig's current context is the expected
one and that its server is a loopback address; `capture.sh` checks the same before reading.
`up.sh` refuses to run when a cluster of that name already exists.

### CLI download and pins

`versions.env` holds `OPM_CLI_VERSION` and `OPM_CLI_SHA256_<os>_<arch>` for the four release
archives, copied from the release's `checksums.txt`. `up.sh` downloads
`opm-<os>-<arch>.tar.gz` from the cli GitHub release into `.e2e/bin/`, compares its sha256 with
the pinned value and stops on a mismatch, before extracting. Pinning the digest, rather than
trusting the `checksums.txt` downloaded beside the archive, means a replaced release asset is
caught. The operator is the CLI's embedded pin unless `OPM_OPERATOR_VERSION` is set, which
passes `--version` to `opm operator install`. `OPM_REGISTRY` is set to the GHCR mapping and
`CUE_CACHE_DIR` to `.e2e/cue-cache`, so a developer's registry mapping and CUE module cache
take no part. The CLI still writes the platform module it generates under `~/.opm/cache`; it
has no setting to move that.

### Provider

`E2E_PROVIDER=podman` (default) runs
`KIND_EXPERIMENTAL_PROVIDER=podman systemd-run --scope --user kind create cluster ...`, the form
rootless podman needs for cgroup delegation and the one experiment 01 used.
`E2E_PROVIDER=docker` runs plain `kind create cluster`. Both pass the pinned
`kindest/node:v1.36.1@sha256:...` image (kind v0.32.0's default), so the two providers run the
same Kubernetes.

### Settle rules

`up.sh` waits with a polling helper (no `kubectl wait --for=jsonpath` value-less forms, which
differ across kubectl versions):

| Fixture | Settled when | Fails `up` |
| --- | --- | --- |
| cert-manager MI | `Ready=True`, Deployments `Available` | yes |
| podinfo MI | `Ready=True`, Deployment `Available` | yes |
| ModulePackage `pkg/podinfo` | `Ready` reason `SourceNotReady` | no, recorded |
| backup-provider MI | `Ready=True` | yes |
| claim `default.backup-provider` | any `Ready` condition at its generation | no, recorded |
| backup-consumer MI | any `Ready` condition at its generation | no, recorded |
| CLI-owned `web/web` | `opm instance apply --wait` succeeds | yes |

The claim and the consumer depend on the operator's library, so `up.sh` prints their verdict
and does not judge it. `capture.sh` writes the verdict into `meta.yaml`, with a note when the
claim is not accepted and active.

### Capture shape

Each file is a `kind: List` sorted by kind, namespace and name:

- `platforms.yaml`, `moduleinstances.yaml`, `modulepackages.yaml`,
  `transformerregistrations.yaml`: the four OPM kinds, all namespaces.
- `objects.yaml`: every object an instance's `status.inventory.entries` names, fetched by
  `kind.version.group` (plain kind for the core group), skipping `Secret`; then the ReplicaSets
  and Pods labelled `module-instance.opmodel.dev/name`, which carry no uuid label (observation 8).
- `events.yaml`: `events.k8s.io/v1` events in `default`, `cert-manager`, `pkg` and `web`.
- `meta.yaml`: capture time, kind and node image, CLI version, operator image, Platform catalog
  version, and per instance and registration the `Ready` status and reason, `accepted`,
  `active`.

Every item loses `metadata.managedFields` and the
`kubectl.kubernetes.io/last-applied-configuration` annotation; ModuleInstances and
ModulePackages lose `spec.values`. CustomResourceDefinitions lose `spec.versions[].schema`:
cert-manager's six CRD schemas were 472 KB of a 1.17 MB `objects.yaml` in the first run, and the
portal reads a CRD's metadata and conditions, never its schema. `check-capture.sh` fails on any `Secret`, any remaining
`managedFields`, last-applied annotation or MI/MP `spec.values`, and `capture.sh` runs it last.

### Authorization

The scripts read, as kind's cluster-admin kubeconfig, on the throwaway cluster: `get`/`list` on
`platforms`, `moduleinstances`, `modulepackages`, `transformerregistrations` (`opmodel.dev`),
`get` on each inventory object, `list` on `replicasets` and `pods`, `list` on
`events.events.k8s.io`, `get` on the operator Deployment. No verb on `secrets`. Writes
(`up.sh` only): the kind cluster itself, the operator install, and the fixture objects.

### CI

`.github/workflows/e2e.yml`, job `E2E` (never `Lint`), on `workflow_dispatch` and a nightly
`schedule`, never on `pull_request`. It installs kind from the `KIND_VERSION` pin with
`go install`, runs `task e2e:up E2E_PROVIDER=docker` and `task e2e:capture`, uploads
`testdata/clusters/f1/` as an artifact, and runs `task e2e:down` under `if: always()`.
`permissions: contents: read`; GHCR pulls are anonymous.

## Research & Decisions

### Where the scripts live

**Context**: the plan puts the kind e2e and its fixtures under `test/e2e/`; the repo has
`hack/` for helper scripts.
**Explored**: opm-operator keeps `hack/` for release and lint helpers and `test/` for test tiers.
**Options considered**:
1. `hack/e2e/`: groups with helpers, but splits the environment from the Go e2e tests that
   follow.
2. `test/e2e/` (chosen): the Go e2e package of the next part lands beside the environment it
   runs on.
**Decision**: `test/e2e/`.
**Rationale**: one place for the e2e tier.

### Checksum source

**Context**: the task asks for a checksum-verified CLI download.
**Explored**: experiment 01 compared the archive with the release's `checksums.txt`.
**Options considered**:
1. Download `checksums.txt` beside the archive: catches corruption, not a replaced asset.
2. Pin the sha256 in the repo (chosen): a replaced asset fails; moving the pin is one reviewed
   diff.
**Decision**: pinned sha256 per platform.
**Rationale**: the pin file is reviewed; a downloaded checksum is not.

### Verdict of the backup claim

**Context**: the claim is accepted and active only on an operator built on library
v1.0.0-beta.2 or later; the released beta.5 operator refuses it `CatalogUnresolved`
(experiment 01, phase 7).
**Options considered**:
1. Build the operator from `main` for the capture: tests an unreleased build, the thing
   0030's graduation gate rules out as evidence.
2. Leave the backup fixtures out until a release: loses the refused-claim and
   consumer-without-provider states.
3. Apply them and record the verdict (chosen).
**Decision**: record, do not judge; `meta.yaml` notes a claim that is not accepted and active.
**Rationale**: the refused state is itself a state the portal must render, and rerunning after
operator release 1.0.0-beta.6 (or a CLI that embeds it) captures the accepted state with no
script change.

## Risks / Trade-offs

- [GHCR or the cli release is unreachable] → `up` fails before applying anything; the nightly
  job reports it, no pull request is blocked.
- [A later operator changes status shapes] → the capture is a snapshot of named versions
  (`meta.yaml`); a pin move is a reviewed recapture.
- [Rendered values reach non-Secret objects, such as a ConfigMap or a container env] → the same
  holds in `kubectl` and for the portal (0030:D8:R4); the fixtures carry no secret material.
