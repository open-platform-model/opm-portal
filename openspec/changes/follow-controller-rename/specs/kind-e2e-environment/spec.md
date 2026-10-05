## ADDED Requirements

### Requirement: The F1 fixture set covers every state the portal renders

`task e2e:up` SHALL apply the fixture set F1 and wait until it settles: cert-manager
(`opmodel.dev/modules/cert_manager@v2`) under an applier ServiceAccount allowed to write
cluster-scoped kinds, the controller's podinfo test module, a ModulePackage on a cluster without
Flux, the backup provider and backup consumer test modules (`testing.opmodel.dev/...`), and a
CLI-owned `web_app` instance applied with `opm instance apply`, and a deliberate refusal
fixture: a TransformerRegistration applied by hand, labelled
`e2e.opmodel.dev/fixture: deliberate-refusal`, naming a catalog published nowhere and providing
no contract, so golden suites keep a refused registration whichever controller is installed. It
SHALL fail when cert-manager, podinfo, backup-provider or the CLI-owned instance does not become
ready, or when the deliberate refusal fixture gets no verdict or is accepted, and SHALL record,
without failing, the state of the ModulePackage, the backup claim and the backup consumer.

#### Scenario: Released controller refuses the backup claim

- **WHEN** the installed controller is built on a library older than v1.0.0-beta.2
- **THEN** `task e2e:up` succeeds, prints the claim's `Ready` reason, and the capture's
  `meta.yaml` marks the claim as not accepted and active

#### Scenario: Controller accepts the backup claim

- **WHEN** the installed controller accepts the claim
- **THEN** the capture's `meta.yaml` records `accepted: true` and `active: true` for it with no
  note

#### Scenario: Deliberate refusal fixture

- **WHEN** `task e2e:up` applies the claim `default.refused-claim-fixture`
- **THEN** the controller refuses it (`Ready=False`, no `status.accepted` or `status.active`), and
  the capture's `meta.yaml` records it with `deliberateRefusal: true` and a note saying it is
  refused on purpose

#### Scenario: Deliberate refusal fixture accepted

- **WHEN** the installed controller accepts `default.refused-claim-fixture`
- **THEN** `task e2e:up` exits non-zero and prints the claim's conditions

#### Scenario: A required fixture never becomes ready

- **WHEN** the podinfo ModuleInstance is not `Ready=True` within its timeout
- **THEN** `task e2e:up` exits non-zero and prints that instance's conditions

## MODIFIED Requirements

### Requirement: The fixture cluster is created from pinned released artifacts

`task e2e:up` SHALL create a kind cluster named `opm-portal-e2e`, or the name `E2E_CLUSTER` gives,
which SHALL be `opm-portal-e2e` or start with `opm-portal-e2e-`, from the node image pinned in
`test/e2e/versions.env`, with podman as the default provider and docker when `E2E_PROVIDER=docker`,
and SHALL record the provider so `task e2e:capture` and `task e2e:down` use it when
`E2E_PROVIDER` is unset. It SHALL refuse to run when the `kind` on `PATH` is not the pinned
release. It SHALL download the `opm` CLI release pinned in `versions.env`, SHALL compare the
archive's sha256 with the pinned value for the host's platform before every extraction, including
from a cached archive, and SHALL stop with a non-zero exit on a mismatch. It SHALL install the
`opm_controller` module version `OPM_CONTROLLER_VERSION` names, with
`opm controller install --version` (the version the CLI pins when it is empty), without the CLI's
Platform, and SHALL apply a cluster Platform subscribed to the catalog version
`versions.env` pins. Container images a fixture module chooses itself (cert-manager's three
images share one digest field, podinfo's is set by its test module) are pinned by tag only; the
CLI-owned instance's image is pinned by digest.

#### Scenario: Fresh environment

- **WHEN** a developer runs `task e2e:up` with no cluster named `opm-portal-e2e`
- **THEN** the cluster exists, the controller Deployment is available, the cluster Platform is
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

#### Scenario: Second cluster beside the default

- **WHEN** a developer runs `task e2e:up` with `E2E_CLUSTER=opm-portal-e2e-fix`
- **THEN** the cluster `opm-portal-e2e-fix` is created with its kubeconfig and provider under
  `.e2e/clusters/opm-portal-e2e-fix/`, and `.e2e/kubeconfig` is not touched

#### Scenario: Cluster name outside the family

- **WHEN** `E2E_CLUSTER` names a cluster that does not start with `opm-portal-e2e`
- **THEN** every e2e script exits non-zero before calling kind or kubectl

#### Scenario: Cluster already exists

- **WHEN** a cluster named `opm-portal-e2e` already exists
- **THEN** `task e2e:up` exits non-zero and tells the developer to run `task e2e:down` first

### Requirement: Milestone 1 reads are checked end to end on the fixture cluster

`task e2e:m1` SHALL run the built binary in local mode against the fixture cluster and SHALL
fail unless, through the read API and its stream: the Platform shows `default.backup-provider`
accepted and active and contributing the backup catalog, and `default.refused-claim-fixture`
refused with its reason and message; the instance list shows each F1 instance's applied state
and health as two values (the controller's instances Applied and Healthy, the CLI-owned `web/web`
ManagedExternally and Healthy); the `web/web` page reads "Managed externally"; and a
ServiceAccount that may read the OPM kinds only in `default`, run with `--namespaces default`,
lists `default`'s instances only, gets forbidden for every other list, object and the Platform,
sees podinfo's inventory objects as forbidden with a partial health and as locked rows on the
page, and has the cluster-wide list topic closed as forbidden on the stream. It SHALL patch
podinfo's image tag to one that does not exist and SHALL fail unless the stream reports podinfo
Degraded within 10 seconds of the cluster first reporting a Pod that cannot pull its image, with
its applied state Applied, and for 5 seconds afterwards never reports it Failed or Stalled or
other than Degraded. It SHALL revert the
patch and wait for podinfo to settle before it ends. Source: portal:D3:R2/R3, portal:D4:R4,
portal:D5:R5/R7.

#### Scenario: Image break

- **WHEN** `task e2e:m1` patches podinfo's image tag to a missing tag
- **THEN** the `instance:default/podinfo` topic carries an Instance with health Degraded and
  applied state Applied within 10 seconds of the Pod's `ErrImagePull`
- **AND** after the test, podinfo's Deployment has rolled back and its Pods are running

#### Scenario: Namespace-scoped reader

- **WHEN** the reader that may read the OPM kinds only in `default` asks for the cluster-wide
  instance list, the `web/web` instance or the Platform
- **THEN** the list answers with access `forbidden` and no items, and the reads answer 403 with
  the code `forbidden`

#### Scenario: Health stops following the stream

- **WHEN** a change makes the portal stop delivering podinfo's Pod changes on the stream
- **THEN** `task e2e:m1` fails, either because no Degraded document arrives or because it
  arrives more than 10 seconds after the cluster reported the broken Pod

### Requirement: The environment runs on a schedule, never on pull requests

The repository SHALL run `task e2e:up` and `task e2e:capture` on docker kind in a workflow
triggered by manual dispatch and a nightly schedule only, SHALL upload the capture as a workflow
artifact, and SHALL delete the cluster whether or not the earlier steps succeeded. It SHALL fail
when the capture's `meta.yaml`, apart from the capture time, the provider and the cluster name,
differs from the committed one. On the same cluster it SHALL run `task e2e:local`, `task e2e:m1`
and then `task e2e:pod` whenever the cluster was created, even after a verdict difference or an
earlier test failure, and in a parallel job it SHALL run `task test:browser` with docker and the
Playwright image pinned by digest. When a test fails it SHALL upload the test output, a dump of
the fixture cluster's OPM objects, Pods and events that holds no Secret, and the browser
screenshots. The workflow SHALL hold only the `contents: read` permission and SHALL use no
secrets. No pull request check SHALL depend on this workflow.

#### Scenario: Nightly run

- **WHEN** the nightly schedule fires
- **THEN** the workflow creates the cluster, captures it, uploads the capture and deletes the
  cluster

#### Scenario: Verdict drift

- **WHEN** a nightly capture records a different `Ready` reason, claim verdict, controller or
  catalog version than the committed `meta.yaml`
- **THEN** the workflow run fails and its summary shows the difference

#### Scenario: Pull request

- **WHEN** a pull request is opened
- **THEN** the e2e workflow does not run

#### Scenario: A launch regression

- **WHEN** a change to `internal/auth` makes a browser land on the launch without its session
- **THEN** the next nightly run fails in the `Browser` job and uploads the browser screenshots

#### Scenario: A failing local-mode test

- **WHEN** `task e2e:local` fails in the nightly run
- **THEN** the run still runs `task e2e:m1` and `task e2e:pod`, deletes the cluster, and uploads
  the test output and the cluster dump

## REMOVED Requirements

### Requirement: The F1 fixture set covers the states the portal renders

**Reason**: The product is now called the controller; a scenario name changed, so the requirement is restated under a new name.

**Migration**: None. The ADDED requirement below carries the same behavior.
