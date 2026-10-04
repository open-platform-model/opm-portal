## Purpose

Defines the throwaway kind cluster the portal's e2e tier and golden captures run on: how it is
created from released artifacts only, which fixtures it carries, and what a capture of it may
and may not contain.

## ADDED Requirements

### Requirement: The fixture cluster is created from pinned released artifacts

`task e2e:up` SHALL create a kind cluster named `opm-portal-e2e` from the node image pinned in
`test/e2e/versions.env`, with podman as the default provider and docker when `E2E_PROVIDER=docker`,
and SHALL record the provider so `task e2e:capture` and `task e2e:down` use it when
`E2E_PROVIDER` is unset. It SHALL refuse to run when the `kind` on `PATH` is not the pinned
release. It SHALL download the `opm` CLI release pinned in `versions.env`, SHALL compare the
archive's sha256 with the pinned value for the host's platform before every extraction, including
from a cached archive, and SHALL stop with a non-zero exit on a mismatch. It SHALL install the
opm-operator release `OPM_OPERATOR_VERSION` names (the CLI's embedded version when it is empty)
without the CLI's Platform, and SHALL apply a cluster Platform subscribed to the catalog version
`versions.env` pins. Container images a fixture module chooses itself (cert-manager's three
images share one digest field, podinfo's is set by its test module) are pinned by tag only; the
CLI-owned instance's image is pinned by digest.

#### Scenario: Fresh environment

- **WHEN** a developer runs `task e2e:up` with no cluster named `opm-portal-e2e`
- **THEN** the cluster exists, the operator Deployment is available, the cluster Platform is
  `Ready=True`, and the kubeconfig is written to `.e2e/kubeconfig`

#### Scenario: Tampered download

- **WHEN** the downloaded CLI archive's sha256 differs from the pinned value
- **THEN** `task e2e:up` exits non-zero naming the mismatch, before any cluster object is applied

#### Scenario: Catalog pin

- **WHEN** a newer release of the pinned catalog is published
- **THEN** the cluster Platform still subscribes to the version `versions.env` pins

#### Scenario: kind differs from the pin

- **WHEN** the `kind` on `PATH` reports a version other than `KIND_VERSION`
- **THEN** `task e2e:up` exits non-zero, naming both versions, before creating a cluster

#### Scenario: Cluster already exists

- **WHEN** a cluster named `opm-portal-e2e` already exists
- **THEN** `task e2e:up` exits non-zero and tells the developer to run `task e2e:down` first

### Requirement: The scripts touch only the fixture cluster

Every `kubectl` and `opm` call the e2e scripts make SHALL pass the kubeconfig `.e2e/kubeconfig`
and the context `kind-opm-portal-e2e` explicitly, and the scripts SHALL ignore the `KUBECONFIG`
environment variable. Before applying or reading anything, a script SHALL check that the
context's API server is on a loopback address and SHALL exit non-zero otherwise. `task e2e:down`
SHALL delete only the cluster named `opm-portal-e2e`.

#### Scenario: Developer's own context is untouched

- **WHEN** a developer's `KUBECONFIG` points at another cluster and they run `task e2e:up`
- **THEN** no object is applied to that cluster and that kubeconfig file is not modified

#### Scenario: Non-loopback server

- **WHEN** `.e2e/kubeconfig` names a server that is not on a loopback address
- **THEN** `task e2e:capture` exits non-zero without reading any object

### Requirement: The F1 fixture set covers the states the portal renders

`task e2e:up` SHALL apply the fixture set F1 and wait until it settles: cert-manager
(`opmodel.dev/modules/cert_manager@v2`) under an applier ServiceAccount allowed to write
cluster-scoped kinds, the operator's podinfo test module, a ModulePackage on a cluster without
Flux, the backup provider and backup consumer test modules (`testing.opmodel.dev/...`), and a
CLI-owned `web_app` instance applied with `opm instance apply`. It SHALL fail when cert-manager,
podinfo, backup-provider or the CLI-owned instance does not become ready, and SHALL record,
without failing, the state of the ModulePackage, the backup claim and the backup consumer.

#### Scenario: Released operator refuses the backup claim

- **WHEN** the installed operator is built on a library older than v1.0.0-beta.2
- **THEN** `task e2e:up` succeeds, prints the claim's `Ready` reason, and the capture's
  `meta.yaml` marks the claim as not accepted and active

#### Scenario: Operator accepts the backup claim

- **WHEN** the installed operator accepts the claim
- **THEN** the capture's `meta.yaml` records `accepted: true` and `active: true` for it with no
  note

#### Scenario: A required fixture never becomes ready

- **WHEN** the podinfo ModuleInstance is not `Ready=True` within its timeout
- **THEN** `task e2e:up` exits non-zero and prints that instance's conditions

### Requirement: A capture holds only what the portal may serve

`task e2e:capture` SHALL write the four OPM kinds, the objects their inventories name, the
ReplicaSets and Pods labelled with an instance name, the events of the fixture namespaces and a
`meta.yaml` into `testdata/clusters/f1/`, each as a list sorted by kind, namespace and name,
leaving out events about Nodes. It SHALL NOT get, list or watch Secrets, and SHALL skip any inventory entry of kind `Secret`. Every
captured object SHALL lack `metadata.managedFields` and the
`kubectl.kubernetes.io/last-applied-configuration` annotation, and every ModuleInstance and
ModulePackage SHALL lack `spec.values`. CustomResourceDefinitions SHALL be captured without
`spec.versions[].schema`. `task e2e:capture:check` SHALL check every file under
`testdata/clusters/`, recursively and whatever its extension (Markdown excepted), and every
document in it, a List by its items and any other object as itself. A file that breaks any of
these rules, or does not parse, SHALL fail it. The capture runs it last, and `task check` and
the `Test` check run it on every pull request. Source: 0030:D8.

#### Scenario: Clean capture

- **WHEN** `task e2e:capture` runs against a settled F1 cluster
- **THEN** `testdata/clusters/f1/` holds `meta.yaml` and the six object lists, and `task e2e:capture:check` exits 0

#### Scenario: Values left in a capture

- **WHEN** a capture file holds a ModuleInstance with `spec.values`
- **THEN** `task e2e:capture:check` exits non-zero and names the file and the object

#### Scenario: Hand-edited capture in a pull request

- **WHEN** a pull request adds `spec.values` to an instance in `testdata/clusters/f1/`
- **THEN** the `Test` check fails

#### Scenario: Secret in a capture

- **WHEN** any file under `testdata/clusters/` holds an object of kind `Secret`, whether in a
  List, as a bare object, as one document of several, or in a `.yml` or `.json` file
- **THEN** `task e2e:capture:check` exits non-zero

#### Scenario: Unparseable capture file

- **WHEN** a file under `testdata/clusters/` does not parse as YAML
- **THEN** `task e2e:capture:check` exits non-zero and names the file

### Requirement: The environment runs on a schedule, never on pull requests

The repository SHALL run `task e2e:up` and `task e2e:capture` on docker kind in a workflow
triggered by manual dispatch and a nightly schedule only, SHALL upload the capture as a workflow
artifact, and SHALL delete the cluster whether or not the earlier steps succeeded. It SHALL fail
when the capture's `meta.yaml`, apart from the capture time and the provider, differs from the
committed one. No pull request check SHALL depend on this workflow.

#### Scenario: Nightly run

- **WHEN** the nightly schedule fires
- **THEN** the workflow creates the cluster, captures it, uploads the capture and deletes the
  cluster

#### Scenario: Verdict drift

- **WHEN** a nightly capture records a different `Ready` reason, claim verdict, operator or
  catalog version than the committed `meta.yaml`
- **THEN** the workflow run fails and its summary shows the difference

#### Scenario: Pull request

- **WHEN** a pull request is opened
- **THEN** the e2e workflow does not run
