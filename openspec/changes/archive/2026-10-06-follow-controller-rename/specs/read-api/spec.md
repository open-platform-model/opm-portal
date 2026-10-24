## ADDED Requirements

### Requirement: Owner and version fields use the controller's names

An instance summary's `owner` SHALL be `cli` when the instance's `spec.owner` is `cli`, and
`controller` when it is `controller` or absent. Any other `spec.owner` value SHALL be served as
the instance holds it and SHALL NOT be mapped to `controller` or `cli`. The OpenAPI document
SHALL list the known owner values as `[controller, cli]`. The Platform document and the platform of a
graph SHALL carry `controllerVersion`, read from the Platform's `status.controllerVersion` and from
no other field; when that field is absent, `controllerVersion` SHALL be absent.

#### Scenario: An instance the controller applies

- **WHEN** a client lists instances and one has `spec.owner: controller`, and another has no
  `spec.owner`
- **THEN** both summaries carry `owner: controller`

#### Scenario: An owner value the portal does not know

- **WHEN** a client lists instances and one has a `spec.owner` that is neither `controller` nor
  `cli`
- **THEN** its summary carries that value as written, and not `controller`

#### Scenario: A CLI-owned instance

- **WHEN** a client lists instances and one has `spec.owner: cli`
- **THEN** its summary carries `owner: cli` and its reconcile state is `ManagedExternally`

#### Scenario: The Platform reports its controller version

- **WHEN** a client reads the Platform whose `status.controllerVersion` is `v1.0.0-beta.9`
- **THEN** the document carries `controllerVersion: v1.0.0-beta.9`, and the platform of the
  platform graph carries the same value

#### Scenario: The Platform reports no controller version

- **WHEN** a client reads a Platform whose status has no `controllerVersion`
- **THEN** the document carries no `controllerVersion` field and no version from any other field

## MODIFIED Requirements

### Requirement: Documents are portal-shaped and versioned

Every document SHALL carry `apiVersion: portal.opmodel.dev/v1alpha1` and a `kind`. No document
SHALL embed a raw custom-resource status. An instance and a package SHALL carry the controller's
applied state (`reconcile`: state, reason, message, since) and the portal's workload health
(`health`: state, per-state and per-access counts, `partial`, `evaluatedAt`, `live`) as two
separate blocks, never one merged status. Enumerated values SHALL be documented as open: a
client is told to treat an unknown value as unknown and to ignore unknown fields. Source:
portal:D2:R3/R4, portal:D3:R1.

#### Scenario: Both axes on an applied, degraded instance

- **WHEN** a client reads an instance the controller applied whose Pod waits on `ErrImagePull`
- **THEN** its `reconcile.state` is `Applied` and its `health.state` is `Degraded`

#### Scenario: A CLI-owned instance

- **WHEN** a client reads instance `web/web`, which the CLI applied
- **THEN** its `owner` is `cli`, its `reconcile.state` is `ManagedExternally`, and its `health`
  is computed from its inventory

### Requirement: Values, Secrets and the apply annotation are never served

No document SHALL contain an instance's or package's `spec.values`, Secret data, or the
`kubectl.kubernetes.io/last-applied-configuration` annotation. In `local` mode condition, history
and event messages SHALL be served as the controller and the API server wrote them; in
`in-cluster` mode the text the controller wrote SHALL be omitted, as the in-cluster requirement
lists. Source: portal:D8:R2/R3/R5/R6.

#### Scenario: No values in any document

- **WHEN** every resource is read for every F1 instance, package and the platform
- **THEN** no response contains a `values` field or the last-applied annotation

### Requirement: The platform document names claimants and the contributing registration

Each catalog in the platform document SHALL list as `claimants` the readable registrations that
claim it, and SHALL name as `contributedBy` the one accepted and active registration the
registry's contribution is joined to, by the same rule the platform graph draws its
`contributes` edge. Source: portal:D4:R4.

#### Scenario: The accepted claim contributed the backup catalog

- **WHEN** a caller reads the F1 platform
- **THEN** catalog `testing.opmodel.dev/catalogs/controller/backup@v0` has `contributedBy`
  `default.backup-provider`

### Requirement: In-cluster mode serves no controller text

In `in-cluster` mode no document, whether served by a `GET` or on the change stream, SHALL carry
text the controller wrote: a condition's `message` (a registration's `conditions` included), a
reconcile `message`, a history entry's `message`, a registration's `message` or `activeMessage`,
the `note` of an event the controller reported (its `reportingController` is `opm-controller`, or it
names none and regards an `opmodel.dev` object), the health `message` of an inventory object or
graph node of an `opmodel.dev` kind, or, in an `Object` of an `opmodel.dev` kind,
`status.conditions[].message` and `status.history[].message`. Every reason, state, `tone`,
`meaning`, `nextStep`, history `outcome` and `providerOf` standing SHALL stay. Text other writers
wrote SHALL be served as written: the notes of events the kubelet or another controller reported,
and the health `message` of an object outside `opmodel.dev`. Events the controller reported that are
left alike once their notes are dropped (same type, reason, reporting controller, regarded object
and field path) SHALL be served as one line, their counts summed and its time the latest, so the
number of lines does not tell how many distinct notes were dropped. A document type the omission
does not know SHALL fail with `upstream_unavailable` rather than be served. The OpenAPI document
SHALL say, on each of those fields, that it is absent in-cluster. Source: owner answer to
portal:OQ8 (portal:D8:R5); its scope is the supervisor ruling recorded under portal:D8.

#### Scenario: A failed instance in-cluster

- **WHEN** a client of an in-cluster server reads an instance whose `Ready` condition is `False`
  with reason `RenderFailed` and a message
- **THEN** the condition carries its type, status, reason, tone, meaning and next step, and no
  `message`; the reconcile state carries its reason and no `message`

#### Scenario: Events in-cluster

- **WHEN** a client of an in-cluster server reads an `EventList` or follows its `events:` topic
- **THEN** every event the controller reported carries its type, reason, count and times, and no
  `note`

#### Scenario: The kubelet's events in-cluster

- **WHEN** a client of an in-cluster server reads the `EventList` of a Pod the kubelet reported
  pulling an image for
- **THEN** each of those events carries its `note` as the kubelet wrote it

#### Scenario: Events differing only in their note in-cluster

- **WHEN** a client of an in-cluster server reads the `EventList` of an instance with two
  `Applied` events whose notes differ
- **THEN** it carries one `Applied` line with count 2 and the later time

#### Scenario: An OPM object's YAML in-cluster

- **WHEN** a client of an in-cluster server reads the `Object` of the TransformerRegistration
  `default.backup-provider` through the instance that reaches it
- **THEN** its `status.conditions` keep their type, status and reason and carry no `message`

#### Scenario: Local mode

- **WHEN** a client of a local server reads the same documents
- **THEN** every message and note is served as the controller and the API server wrote it

#### Scenario: Registration conditions in-cluster

- **WHEN** a client of an in-cluster server reads the platform document
- **THEN** each registration's `conditions` carry type, status, reason, tone and meaning, and no
  `message`

### Requirement: The cluster document names the connection

`GET /api/v1alpha1/clusters/{cluster}` SHALL serve a `Cluster` document: the path cluster's
`name`, the server's `mode`, its `source` (`kubeconfig` or `in-cluster`), the kubeconfig
`context` and the name of its `clusterEntry` when the source is `kubeconfig`, `readingAs` with the
requesting caller's own username, and the API server's `kubernetesVersion` as the read model
holds it, absent when the read model holds none. It SHALL carry no server URL, kubeconfig user
entry or credential, and no other identity than the caller's, and SHALL make no cluster read per
request. A request without a principal SHALL be answered `401` with code `unauthenticated`. In
`in-cluster` mode the document SHALL be served unchanged: it carries no controller text. Source:
portal:D18:R1/R2/R3.

#### Scenario: Local mode on the fixture cluster

- **WHEN** a local-mode client reads `/api/v1alpha1/clusters/default` against a kubeconfig whose
  context is `kind-opm-portal-e2e`
- **THEN** the document has `source: kubeconfig`, `context: kind-opm-portal-e2e`, `readingAs`
  with the kubeconfig's username and the API server's `kubernetesVersion`

#### Scenario: Version not read

- **WHEN** the read model could not read `/version` at start
- **THEN** the document has no `kubernetesVersion`, and the response is still `200`

#### Scenario: Nothing else about the kubeconfig

- **WHEN** any client reads the `Cluster` document
- **THEN** it contains no server URL, certificate, token or user entry

#### Scenario: In-cluster

- **WHEN** a client of an in-cluster server reads the `Cluster` document
- **THEN** it is served, not refused as a document type the omission does not know

## RENAMED Requirements

- FROM: `### Requirement: In-cluster mode serves no operator text`
- TO: `### Requirement: In-cluster mode serves no controller text`
