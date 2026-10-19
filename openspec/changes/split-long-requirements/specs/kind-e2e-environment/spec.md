## REMOVED Requirements

### Requirement: The fixture cluster is created from pinned released artifacts
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The fixture cluster is a kind cluster named in the opm-portal-e2e family", "The kind cluster uses the pinned node image and a recorded provider", "The environment refuses a kind that is not the pinned release", "The opm CLI download is compared with its pinned sha256", "The operator and the cluster Platform are installed at pinned versions", "Fixture images are pinned by tag, the CLI-owned instance's image by digest".

### Requirement: The scripts touch only the fixture cluster
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Every e2e script call names the fixture cluster's kubeconfig and context", "An e2e script checks that the API server is on a loopback address", "task e2e:down deletes only the fixture cluster it names".

### Requirement: The F1 fixture set covers the states the portal renders
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "task e2e:up applies the fixture set F1 and waits until it settles", "The deliberate refusal fixture is a registration no operator accepts", "task e2e:up fails when a required fixture is not ready", "task e2e:up fails when the deliberate refusal fixture gets no verdict or is accepted", "task e2e:up records the state of the package, the claim and the consumer without failing".

### Requirement: A capture holds only what the portal may serve
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "A capture writes the OPM kinds, their inventories and the fixture events", "A capture never reads a Secret", "A captured object lacks managed fields, the apply annotation and values", "The capture check reads every file and every document under testdata/clusters", "The capture check fails a file that breaks a rule or does not parse", "The capture check has a test with a scratch case for each rule", "The capture runs the check last and every pull request runs it".

### Requirement: Milestone 1 reads are checked end to end on the fixture cluster
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "task e2e:m1 checks the Platform's registrations", "task e2e:m1 checks the instance list and the web/web page", "task e2e:m1 checks what a namespace-scoped reader lists and gets", "task e2e:m1 checks what a namespace-scoped reader sees of podinfo and on the stream", "task e2e:m1 breaks podinfo's image and checks the stream reports Degraded", "task e2e:m1 reverts the patch and waits for podinfo to settle".

### Requirement: The environment runs on a schedule, never on pull requests
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The e2e workflow is triggered by dispatch and a nightly schedule only", "The e2e workflow uploads the capture and deletes the cluster", "The e2e workflow fails when the capture differs from the committed meta", "The e2e workflow runs the local, m1 and pod tests on the cluster it created", "The e2e workflow runs the browser tests in a parallel job", "A failing test uploads its output, a cluster dump and the screenshots", "The e2e workflow holds only contents read and uses no secrets".

### Requirement: The portal image is checked in a Pod on the fixture cluster
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "task e2e:pod builds the portal image, loads it and waits for the rollout", "task e2e:pod runs TestPod and checks the Pod log", "TestPod checks a launch and the instance list through a port-forward", "TestPod checks a forward from another local port is refused", "task e2e:pod touches only the fixture cluster".

## ADDED Requirements

### Requirement: The fixture cluster is a kind cluster named in the opm-portal-e2e family
`task e2e:up` SHALL create a kind cluster named `opm-portal-e2e`, or the name `E2E_CLUSTER` gives,
which SHALL be `opm-portal-e2e` or start with `opm-portal-e2e-`.

#### Scenario: Fresh environment

- **WHEN** a developer runs `task e2e:up` with no cluster named `opm-portal-e2e`
- **THEN** the cluster exists, the operator Deployment is available, the cluster Platform is
  `Ready=True`, and the kubeconfig is written to `.e2e/kubeconfig`

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

### Requirement: The kind cluster uses the pinned node image and a recorded provider
`task e2e:up` SHALL create the kind cluster from the node image pinned in `test/e2e/versions.env`,
with podman as the default provider and docker when `E2E_PROVIDER=docker`, and SHALL record the
provider so `task e2e:capture` and `task e2e:down` use it when `E2E_PROVIDER` is unset.

#### Scenario: The recorded provider

- **WHEN** `task e2e:up` creates the cluster with `E2E_PROVIDER=docker`
- **THEN** the cluster is created from the node image pinned in `test/e2e/versions.env` with docker, and the provider is recorded so `task e2e:capture` and `task e2e:down` use it when `E2E_PROVIDER` is unset

### Requirement: The environment refuses a kind that is not the pinned release
`task e2e:up` SHALL refuse to run when the `kind` on `PATH` is not the pinned release.

#### Scenario: kind differs from the pin

- **WHEN** the `kind` on `PATH` reports a version other than `KIND_VERSION`
- **THEN** `task e2e:up` exits non-zero, naming both versions, before creating a cluster

### Requirement: The opm CLI download is compared with its pinned sha256
`task e2e:up` SHALL download the `opm` CLI release pinned in `versions.env`, SHALL compare the
archive's sha256 with the pinned value for the host's platform before every extraction, including
from a cached archive, and SHALL stop with a non-zero exit on a mismatch.

#### Scenario: Tampered download

- **WHEN** the downloaded CLI archive's sha256 differs from the pinned value
- **THEN** `task e2e:up` exits non-zero naming the mismatch, before any cluster object is applied

### Requirement: The operator and the cluster Platform are installed at pinned versions
`task e2e:up` SHALL install the opm-operator release `OPM_OPERATOR_VERSION` names (the CLI's
embedded version when it is empty) without the CLI's Platform, and SHALL apply a cluster Platform
subscribed to the catalog version `versions.env` pins.

#### Scenario: Catalog pin

- **WHEN** a newer release of the pinned catalog is published
- **THEN** the cluster Platform still subscribes to the version `versions.env` pins

### Requirement: Fixture images are pinned by tag, the CLI-owned instance's image by digest
Container images a fixture module chooses itself (cert-manager's three images share one digest
field, podinfo's is set by its test module) SHALL be pinned by tag only; the CLI-owned instance's
image SHALL be pinned by digest.

#### Scenario: Image pins

- **WHEN** a fixture module chooses a container image itself
- **THEN** the image is pinned by tag only, and the CLI-owned instance's image is pinned by digest

### Requirement: Every e2e script call names the fixture cluster's kubeconfig and context
Every `kubectl` and `opm` call the e2e scripts make SHALL pass the fixture cluster's kubeconfig
(`.e2e/kubeconfig` for `opm-portal-e2e`, `.e2e/clusters/<name>/kubeconfig` for any other) and the
context `kind-<name>` explicitly, and the scripts SHALL ignore the `KUBECONFIG` environment
variable.

#### Scenario: Developer's own context is untouched

- **WHEN** a developer's `KUBECONFIG` points at another cluster and they run `task e2e:up`
- **THEN** no object is applied to that cluster and that kubeconfig file is not modified

### Requirement: An e2e script checks that the API server is on a loopback address
Before applying or reading anything, an e2e script SHALL check that the context's API server is on a
loopback address and SHALL exit non-zero otherwise.

#### Scenario: Non-loopback server

- **WHEN** `.e2e/kubeconfig` names a server that is not on a loopback address
- **THEN** `task e2e:capture` exits non-zero without reading any object

### Requirement: task e2e:down deletes only the fixture cluster it names
`task e2e:down` SHALL delete only the fixture cluster it names.

#### Scenario: Down names one cluster

- **WHEN** a developer runs `task e2e:down` for a fixture cluster
- **THEN** only the fixture cluster it names is deleted

### Requirement: task e2e:up applies the fixture set F1 and waits until it settles
`task e2e:up` SHALL apply the fixture set F1 and wait until it settles: cert-manager
(`opmodel.dev/modules/cert_manager@v2`) under an applier ServiceAccount allowed to write
cluster-scoped kinds, the operator's podinfo test module, a ModulePackage on a cluster without Flux,
the backup provider and backup consumer test modules (`testing.opmodel.dev/...`), a CLI-owned
`web_app` instance applied with `opm instance apply`, and a deliberate refusal fixture.

#### Scenario: The fixture set F1

- **WHEN** `task e2e:up` applies the fixture set F1
- **THEN** it waits until the set settles: cert-manager under an applier ServiceAccount allowed to write cluster-scoped kinds, the operator's podinfo test module, a ModulePackage on a cluster without Flux, the backup provider and backup consumer test modules, a CLI-owned `web_app` instance applied with `opm instance apply`, and a deliberate refusal fixture

### Requirement: The deliberate refusal fixture is a registration no operator accepts
The fixture set F1 SHALL hold a deliberate refusal fixture: a TransformerRegistration applied by
hand, labelled `e2e.opmodel.dev/fixture: deliberate-refusal`, naming a catalog published nowhere and
providing no contract, so golden suites keep a refused registration whichever operator is installed.

#### Scenario: Deliberate refusal fixture

- **WHEN** `task e2e:up` applies the claim `default.refused-claim-fixture`
- **THEN** the operator refuses it (`Ready=False`, no `status.accepted` or `status.active`), and
  the capture's `meta.yaml` records it with `deliberateRefusal: true` and a note saying it is
  refused on purpose

### Requirement: task e2e:up fails when a required fixture is not ready
`task e2e:up` SHALL fail when cert-manager, podinfo, backup-provider or the CLI-owned instance of
the fixture set F1 does not become ready.

#### Scenario: A required fixture never becomes ready

- **WHEN** the podinfo ModuleInstance is not `Ready=True` within its timeout
- **THEN** `task e2e:up` exits non-zero and prints that instance's conditions

### Requirement: task e2e:up fails when the deliberate refusal fixture gets no verdict or is accepted
`task e2e:up` SHALL fail when the deliberate refusal fixture of the fixture set F1 gets no verdict
or is accepted.

#### Scenario: Deliberate refusal fixture accepted

- **WHEN** the installed operator accepts `default.refused-claim-fixture`
- **THEN** `task e2e:up` exits non-zero and prints the claim's conditions

### Requirement: task e2e:up records the state of the package, the claim and the consumer without failing
`task e2e:up` SHALL record, without failing, the state of the ModulePackage, the backup claim and
the backup consumer of the fixture set F1.

#### Scenario: Released operator refuses the backup claim

- **WHEN** the installed operator is built on a library older than v1.0.0-beta.2
- **THEN** `task e2e:up` succeeds, prints the claim's `Ready` reason, and the capture's
  `meta.yaml` marks the claim as not accepted and active

#### Scenario: Operator accepts the backup claim

- **WHEN** the installed operator accepts the claim
- **THEN** the capture's `meta.yaml` records `accepted: true` and `active: true` for it with no
  note

### Requirement: A capture writes the OPM kinds, their inventories and the fixture events
`task e2e:capture` SHALL write the four OPM kinds, the objects their inventories name, the
ReplicaSets and Pods labelled with an instance name, the events of the fixture namespaces and a
`meta.yaml` into `testdata/clusters/f1/`, each as a list sorted by kind, namespace and name, leaving
out events about Nodes. Source: portal:D8.

#### Scenario: Clean capture

- **WHEN** `task e2e:capture` runs against a settled F1 cluster
- **THEN** `testdata/clusters/f1/` holds `meta.yaml` and the six object lists, and `task e2e:capture:check` exits 0

### Requirement: A capture never reads a Secret
`task e2e:capture` SHALL NOT get, list or watch Secrets, and SHALL skip any inventory entry of kind
`Secret`. Source: portal:D8.

#### Scenario: A Secret in an inventory

- **WHEN** an inventory names an entry of kind `Secret`
- **THEN** `task e2e:capture` skips the entry and does not get, list or watch Secrets

### Requirement: A captured object lacks managed fields, the apply annotation and values
Every object `task e2e:capture` writes SHALL lack `metadata.managedFields` and the
`kubectl.kubernetes.io/last-applied-configuration` annotation, and every ModuleInstance and
ModulePackage SHALL lack `spec.values`. CustomResourceDefinitions SHALL be captured without
`spec.versions[].schema`. Source: portal:D8.

#### Scenario: A captured object

- **WHEN** `task e2e:capture` writes an object
- **THEN** the object lacks `metadata.managedFields` and the `kubectl.kubernetes.io/last-applied-configuration` annotation, a ModuleInstance or ModulePackage lacks `spec.values`, and a CustomResourceDefinition lacks `spec.versions[].schema`

### Requirement: The capture check reads every file and every document under testdata/clusters
`task e2e:capture:check` SHALL check every file under `testdata/clusters/`, recursively and whatever
its extension (Markdown excepted), and every document in it to any depth, treating every map with
both `kind` and `metadata` as an object, and any map of kind `Secret` carrying `data` or
`stringData` as a Secret. Source: portal:D8.

#### Scenario: Secret in a capture

- **WHEN** any file under `testdata/clusters/` holds an object of kind `Secret`, whether in a
  List, as a bare object, as one document of several, in a top-level YAML sequence or JSON
  array, in a List nested inside a List, alongside an `items` key, or in a `.yml` or `.json`
  file
- **THEN** `task e2e:capture:check` exits non-zero

### Requirement: The capture check fails a file that breaks a rule or does not parse
A file under `testdata/clusters/` that breaks any of the rules of a capture (no Secret, no
`metadata.managedFields`, no last-applied annotation, no `spec.values` on a ModuleInstance or
ModulePackage, no `spec.versions[].schema` on a CustomResourceDefinition), or does not parse, SHALL
fail `task e2e:capture:check`. Source: portal:D8.

#### Scenario: Values left in a capture

- **WHEN** a capture file holds a ModuleInstance with `spec.values`
- **THEN** `task e2e:capture:check` exits non-zero and names the file and the object

#### Scenario: Unparseable capture file

- **WHEN** a file under `testdata/clusters/` does not parse as YAML
- **THEN** `task e2e:capture:check` exits non-zero and names the file

### Requirement: The capture check has a test with a scratch case for each rule
`test/e2e/check-capture_test.sh` SHALL run the check against scratch cases for each rule before
`task e2e:capture:check` checks the committed captures. Source: portal:D8.

#### Scenario: A scratch case per rule

- **WHEN** `task e2e:capture:check` runs
- **THEN** `test/e2e/check-capture_test.sh` has run the check against scratch cases for each rule before the committed captures are checked

### Requirement: The capture runs the check last and every pull request runs it
`task e2e:capture` SHALL run `task e2e:capture:check` last, and `task check` and the `Test` check
SHALL run it on every pull request. Source: portal:D8.

#### Scenario: Hand-edited capture in a pull request

- **WHEN** a pull request adds `spec.values` to an instance in `testdata/clusters/f1/`
- **THEN** the `Test` check fails

### Requirement: task e2e:m1 checks the Platform's registrations
`task e2e:m1` SHALL run the built binary in local mode against the fixture cluster and SHALL fail
unless, through the read API and its stream, the Platform shows `default.backup-provider` accepted
and active and contributing the backup catalog, and `default.refused-claim-fixture` refused with its
reason and message. Source: portal:D3:R2/R3, portal:D4:R4, portal:D5:R5/R7.

#### Scenario: The Platform's registrations

- **WHEN** `task e2e:m1` runs the built binary in local mode against the fixture cluster
- **THEN** it fails unless the Platform shows `default.backup-provider` accepted and active and contributing the backup catalog, and `default.refused-claim-fixture` refused with its reason and message

### Requirement: task e2e:m1 checks the instance list and the web/web page
`task e2e:m1` SHALL run the built binary in local mode against the fixture cluster and SHALL fail
unless, through the read API and its stream, the instance list shows each F1 instance's applied
state and health as two values (the operator's instances Applied and Healthy, the CLI-owned
`web/web` ManagedExternally and Healthy) and the `web/web` page reads "Managed externally". Source:
portal:D3:R2/R3, portal:D4:R4, portal:D5:R5/R7.

#### Scenario: The instance list and the web/web page

- **WHEN** `task e2e:m1` runs the built binary in local mode against the fixture cluster
- **THEN** it fails unless the instance list shows each F1 instance's applied state and health as two values (the operator's instances Applied and Healthy, the CLI-owned `web/web` ManagedExternally and Healthy), and the `web/web` page reads "Managed externally"

### Requirement: task e2e:m1 checks what a namespace-scoped reader lists and gets
`task e2e:m1` SHALL run the built binary in local mode against the fixture cluster and SHALL fail
unless, through the read API and its stream, a ServiceAccount that may read the OPM kinds only in
`default`, run with `--namespaces default`, lists `default`'s instances only and gets forbidden for
every other list, object and the Platform. Source: portal:D3:R2/R3, portal:D4:R4, portal:D5:R5/R7.

#### Scenario: Namespace-scoped reader

- **WHEN** the reader that may read the OPM kinds only in `default` asks for the cluster-wide
  instance list, the `web/web` instance or the Platform
- **THEN** the list answers with access `forbidden` and no items, and the reads answer 403 with
  the code `forbidden`

### Requirement: task e2e:m1 checks what a namespace-scoped reader sees of podinfo and on the stream
`task e2e:m1` SHALL run the built binary in local mode against the fixture cluster and SHALL fail
unless, through the read API and its stream, a ServiceAccount that may read the OPM kinds only in
`default`, run with `--namespaces default`, sees podinfo's inventory objects as forbidden with a
partial health and as locked rows on the page, and has the cluster-wide list topic closed as
forbidden on the stream. Source: portal:D3:R2/R3, portal:D4:R4, portal:D5:R5/R7.

#### Scenario: The reader and podinfo's inventory

- **WHEN** `task e2e:m1` runs the built binary in local mode against the fixture cluster
- **THEN** it fails unless a ServiceAccount that may read the OPM kinds only in `default`, run with `--namespaces default`, sees podinfo's inventory objects as forbidden with a partial health and as locked rows on the page, and has the cluster-wide list topic closed as forbidden on the stream

### Requirement: task e2e:m1 breaks podinfo's image and checks the stream reports Degraded
`task e2e:m1` SHALL patch podinfo's image tag to one that does not exist and SHALL fail unless the
stream reports podinfo Degraded within 10 seconds of the cluster first reporting a Pod that cannot
pull its image, with its applied state Applied, and for 5 seconds afterwards never reports it Failed
or Stalled or other than Degraded. Source: portal:D3:R2/R3, portal:D4:R4, portal:D5:R5/R7.

#### Scenario: Image break

- **WHEN** `task e2e:m1` patches podinfo's image tag to a missing tag
- **THEN** the `instance:default/podinfo` topic carries an Instance with health Degraded and
  applied state Applied within 10 seconds of the Pod's `ErrImagePull`
- **AND** after the test, podinfo's Deployment has rolled back and its Pods are running

#### Scenario: Health stops following the stream

- **WHEN** a change makes the portal stop delivering podinfo's Pod changes on the stream
- **THEN** `task e2e:m1` fails, either because no Degraded document arrives or because it
  arrives more than 10 seconds after the cluster reported the broken Pod

### Requirement: task e2e:m1 reverts the patch and waits for podinfo to settle
`task e2e:m1` SHALL revert the patch to podinfo's image tag and wait for podinfo to settle before it
ends. Source: portal:D3:R2/R3, portal:D4:R4, portal:D5:R5/R7.

#### Scenario: The revert

- **WHEN** `task e2e:m1` has patched podinfo's image tag
- **THEN** it reverts the patch and waits for podinfo to settle before it ends

### Requirement: The e2e workflow is triggered by dispatch and a nightly schedule only
The repository SHALL run `task e2e:up` and `task e2e:capture` on docker kind in a workflow triggered
by manual dispatch and a nightly schedule only. No pull request check SHALL depend on this workflow.

#### Scenario: Pull request

- **WHEN** a pull request is opened
- **THEN** the e2e workflow does not run

### Requirement: The e2e workflow uploads the capture and deletes the cluster
The e2e workflow SHALL upload the capture as a workflow artifact, and SHALL delete the cluster
whether or not the earlier steps succeeded.

#### Scenario: Nightly run

- **WHEN** the nightly schedule fires
- **THEN** the workflow creates the cluster, captures it, uploads the capture and deletes the
  cluster

### Requirement: The e2e workflow fails when the capture differs from the committed meta
The e2e workflow SHALL fail when the capture's `meta.yaml`, apart from the capture time, the
provider and the cluster name, differs from the committed one.

#### Scenario: Verdict drift

- **WHEN** a nightly capture records a different `Ready` reason, claim verdict, operator or
  catalog version than the committed `meta.yaml`
- **THEN** the workflow run fails and its summary shows the difference

### Requirement: The e2e workflow runs the local, m1 and pod tests on the cluster it created
On the same cluster the e2e workflow SHALL run `task e2e:local`, `task e2e:m1` and then `task
e2e:pod` whenever the cluster was created, even after a verdict difference or an earlier test
failure.

#### Scenario: A failing local-mode test

- **WHEN** `task e2e:local` fails in the nightly run
- **THEN** the run still runs `task e2e:m1` and `task e2e:pod`, deletes the cluster, and uploads
  the test output and the cluster dump

### Requirement: The e2e workflow runs the browser tests in a parallel job
In a parallel job the e2e workflow SHALL run `task test:browser` with docker and the Playwright
image pinned by digest.

#### Scenario: A launch regression

- **WHEN** a change to `internal/auth` makes a browser land on the launch without its session
- **THEN** the next nightly run fails in the `Browser` job and uploads the browser screenshots

### Requirement: A failing test uploads its output, a cluster dump and the screenshots
When a test fails the e2e workflow SHALL upload the test output, a dump of the fixture cluster's OPM
objects, Pods and events that holds no Secret, and the browser screenshots.

#### Scenario: A failing test

- **WHEN** a test fails in the e2e workflow
- **THEN** it uploads the test output, a dump of the fixture cluster's OPM objects, Pods and events that holds no Secret, and the browser screenshots

### Requirement: The e2e workflow holds only contents read and uses no secrets
The e2e workflow SHALL hold only the `contents: read` permission and SHALL use no secrets.

#### Scenario: Permissions of the workflow

- **WHEN** the e2e workflow runs
- **THEN** it holds only the `contents: read` permission and uses no secrets

### Requirement: task e2e:pod builds the portal image, loads it and waits for the rollout
`task e2e:pod` SHALL build the portal image from the repository's `Dockerfile` with the e2e
provider's engine, SHALL load it into the fixture cluster without a registry, SHALL apply the
overlay under `test/e2e/pod/`, which uses `deploy/` unchanged apart from the image name and
`imagePullPolicy: Never`, and SHALL wait for the Deployment to roll out. Source: portal:D13:R1/R2.

#### Scenario: The image runs in the fixture cluster

- **WHEN** a developer runs `task e2e:up` and then `task e2e:pod`
- **THEN** the `opm-portal` Deployment in namespace `opm-portal` is available, and `TestPod`
  passes

#### Scenario: A manifest that does not run

- **WHEN** a change makes the Pod fail to start or the role lose `list` on `moduleinstances`
- **THEN** `task e2e:pod` fails, at the rollout wait or in `TestPod`'s instance list

### Requirement: task e2e:pod runs TestPod and checks the Pod log
After the Deployment has rolled out, `task e2e:pod` SHALL run `TestPod` (build tag `e2e`), which
SHALL fail unless the Pod log names the ServiceAccount identity and holds a launch URL. Source:
portal:D13:R1/R2.

#### Scenario: The Pod log

- **WHEN** `task e2e:pod` has waited for the Deployment to roll out and runs `TestPod` (build tag `e2e`)
- **THEN** `TestPod` fails unless the Pod log names the ServiceAccount identity and holds a launch URL

### Requirement: TestPod checks a launch and the instance list through a port-forward
`TestPod` (build tag `e2e`) SHALL fail unless, with `kubectl port-forward` from local port 8090 to
the Pod's port 8090, a launch with the token from the Pod log answers `200` with a session and the
instance list names the same instances the fixture cluster holds. Source: portal:D13:R1/R2.

#### Scenario: A launch through the forward

- **WHEN** `TestPod` forwards local port 8090 to the Pod's port 8090 with `kubectl port-forward`
- **THEN** a launch with the token from the log answers `200` with a session and the instance list names the same instances the fixture cluster holds

### Requirement: TestPod checks a forward from another local port is refused
`TestPod` (build tag `e2e`) SHALL fail unless a forward from local port 8091 to port 8090 is refused
with `403`. Source: portal:D13:R1/R2.

#### Scenario: A second forward

- **WHEN** `TestPod` forwards local port 8091 to port 8090
- **THEN** the forward is refused with `403`

### Requirement: task e2e:pod touches only the fixture cluster
`task e2e:pod` SHALL touch only the fixture cluster, through the kubeconfig and context the other
e2e tasks use. Source: portal:D13:R1/R2.

#### Scenario: Only the fixture cluster

- **WHEN** `task e2e:pod` runs
- **THEN** it touches only the fixture cluster, through the kubeconfig and context the other e2e tasks use
