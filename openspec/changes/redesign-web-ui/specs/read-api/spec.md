## ADDED Requirements

### Requirement: The cluster document names the connection

`GET /api/v1alpha1/clusters/{cluster}` SHALL serve a `Cluster` document: the path cluster's
`name`, the server's `mode`, its `source` (`kubeconfig` or `in-cluster`), the kubeconfig
`context` and the name of its `clusterEntry` when the source is `kubeconfig`, `readingAs` with the
requesting caller's own username, and the API server's `kubernetesVersion` as the read model
holds it, absent when the read model holds none. It SHALL carry no server URL, kubeconfig user
entry or credential, and no other identity than the caller's, and SHALL make no cluster read per
request. A request without a principal SHALL be answered `401` with code `unauthenticated`. In
`in-cluster` mode the document SHALL be served unchanged: it carries no operator text. Source:
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

### Requirement: Packages carry their interval and source artifact

A package document and a package list item SHALL carry `interval`, the package's `spec.interval`
as written, when it is set, and `sourceArtifact` with the `revision` and `digest` the controller
recorded in `status.source`, when it recorded them. Neither SHALL be filled with a default or a
guess. The source artifact's fetch URL SHALL not be served. Source: portal:D2:R4.

#### Scenario: The F1 package

- **WHEN** a client reads package `pkg/podinfo` from the F1 capture
- **THEN** it has `interval` `1m` and no `sourceArtifact`

### Requirement: Registrations carry their conditions and holders

Each registration in the platform document SHALL carry its `conditions` with `tone`, `meaning`
and `nextStep` as every condition does, and `heldBy`: the ModuleInstances and ModulePackages the
caller may read whose inventory holds a TransformerRegistration of that name, whatever their
kind. `heldByPartial` SHALL be true whenever the caller lacks a cluster-wide list of
ModuleInstances or of ModulePackages, whether or not `heldBy` is empty. `provider` SHALL keep naming the ModuleInstance `spec.providerRef` names. Source:
portal:D15:R1/R2, portal:D4:R2.

#### Scenario: The accepted claim's holder

- **WHEN** a client reads the F1 platform
- **THEN** registration `default.backup-provider` has `heldBy` naming ModuleInstance
  `default/backup-provider` and a `Ready` condition with reason `Accepted`

#### Scenario: A claim nobody holds

- **WHEN** a client reads the F1 platform
- **THEN** registration `default.refused-claim-fixture`, applied by hand, has no `heldBy`

#### Scenario: A package holder

- **WHEN** a package's inventory holds TransformerRegistration `pkg.provider` and the controller
  refused it with `ProviderMismatch`
- **THEN** that registration's `heldBy` names the ModulePackage, its `provider` still names a
  ModuleInstance, and its verdict is `Refused` with reason `ProviderMismatch`

#### Scenario: Instances not listable everywhere

- **WHEN** a caller who may list ModuleInstances only in namespace `default` reads the platform
- **THEN** each registration has `heldByPartial: true`

### Requirement: Instances and packages name the registrations they hold

An instance or package, in its document and its list item, SHALL carry `providerOf`: one entry
per TransformerRegistration its inventory holds, with the registration's name, the caller's
`access` to it, and, when readable, its `accepted`, `active`, `verdict` and `reason`, and
`providerRefMatches`: true only when the owner is a ModuleInstance whose namespace and name the
registration's `spec.providerRef` names, false otherwise (always false for a ModulePackage, since
the reference names a ModuleInstance). A registration the caller may not read SHALL carry its name
and `access: forbidden` only, with no `providerRefMatches`. Source:
portal:D15:R1/R2/R4.

#### Scenario: The F1 provider

- **WHEN** a client reads instance `default/backup-provider` from the F1 capture
- **THEN** `providerOf` holds `default.backup-provider`, accepted, active, verdict `Accepted`,
  `providerRefMatches: true`

#### Scenario: Registrations forbidden

- **WHEN** a caller who may not list TransformerRegistrations reads the same instance
- **THEN** `providerOf` holds `default.backup-provider` with `access: forbidden`, no standing and
  no `providerRefMatches`

#### Scenario: A package holder

- **WHEN** a package `pkg/provider` holds TransformerRegistration `pkg.provider`, whose
  `spec.providerRef` names `pkg/provider`
- **THEN** its `providerOf` entry has `providerRefMatches: false`

### Requirement: Instance list items carry their render contracts

An instance list item SHALL carry `renderContracts`, the contracts the instance's last successful
render used as the controller recorded them in `status.requiredContracts`, so a client can filter
by them without reading every instance. The instance document SHALL carry the same field as
before. Package items SHALL carry none. Source: portal:D16:R1/R2.

#### Scenario: backup-consumer in the list

- **WHEN** a client reads the F1 instance list
- **THEN** `default/backup-consumer` carries `renderContracts` holding
  `opmodel.dev/catalogs/opm/traits/backup@v1alpha1`

### Requirement: History entries carry their outcome

Each history entry SHALL carry `outcome`: `Succeeded` when the controller recorded phase
`complete`, `Failed` when it recorded no phase and a message, and `Unknown` otherwise. The outcome
SHALL be decided before any in-cluster omission of the message. The value set SHALL be documented
as open. Source: portal:D9:R1.

#### Scenario: The F1 package's failed attempts

- **WHEN** a client reads package `pkg/podinfo` from the F1 capture, whose three history entries
  carry no phase and the source error as their message
- **THEN** each entry's `outcome` is `Failed`

#### Scenario: A successful attempt

- **WHEN** a client reads instance `default/podinfo` from the F1 capture
- **THEN** its history entries with phase `complete` have `outcome` `Succeeded`

#### Scenario: Failed attempts in-cluster

- **WHEN** a client of an in-cluster server reads package `pkg/podinfo`
- **THEN** each entry's `outcome` is `Failed` and carries no `message`

### Requirement: Joined fields follow changes on both sides

A change to a TransformerRegistration SHALL reach the `instance:` or `package:` topic of every
owner whose inventory holds it, and the `instances` and `instances:<namespace>` topics for instance
holders, so `providerOf` refreshes. A change to a ModuleInstance or ModulePackage whose inventory
holds a TransformerRegistration, before or after the change, SHALL reach the `platform` topic, so
`heldBy` refreshes. Each subscriber is still written only a change to its own document. Source:
portal:D2:R5/R7, portal:D15.

#### Scenario: A claim becomes active

- **WHEN** a client follows `instance:default/backup-provider` and `default.backup-provider` turns
  active
- **THEN** the stream delivers an upsert whose `providerOf` entry is active

#### Scenario: A holder drops its claim

- **WHEN** a client follows `platform` and backup-provider's inventory stops holding
  `default.backup-provider`
- **THEN** the stream delivers a `Platform` whose registration `default.backup-provider` has no
  `heldBy`

## MODIFIED Requirements

### Requirement: Resources live under a versioned, cluster-parameterized path

The portal SHALL serve every read API resource under `/api/v1alpha1/clusters/{cluster}`, with
`GET` only: the cluster itself, `instances`, `instances/{namespace}/{name}`,
`instances/{namespace}/{name}/graph`, `instances/{namespace}/{name}/events`,
`instances/{namespace}/{name}/object`, `packages`, `packages/{namespace}/{name}`,
`packages/{namespace}/{name}/graph`, `packages/{namespace}/{name}/events`,
`packages/{namespace}/{name}/object`, `platform`, `platform/graph`, `platform/events`,
`platform/registrations/{name}/events` and `stream`. The one exception SHALL be
`stream/{stream}/topics`, which takes `POST` only. In milestone 1 the only cluster served SHALL be
`default`. Any other cluster name SHALL be answered `404` with code `not_found` before any
authorization review. A method a resource does not take SHALL be answered `405` with code
`method_not_allowed`. A namespace that is not a DNS-1123 label, or a name that is not a DNS-1123
subdomain, SHALL be answered `400` with code `bad_request` before any authorization review.
Source: portal:D2:R2.

#### Scenario: The default cluster

- **WHEN** a client sends `GET /api/v1alpha1/clusters/default/instances`
- **THEN** the response is an `InstanceList` document

#### Scenario: Another cluster

- **WHEN** a client sends `GET /api/v1alpha1/clusters/prod/instances`
- **THEN** the response is `404` with code `not_found`
- **AND** no authorization review is sent

#### Scenario: A write method

- **WHEN** a client sends `POST /api/v1alpha1/clusters/default/instances`
- **THEN** the response is `405` with code `method_not_allowed`

#### Scenario: A malformed name

- **WHEN** a client sends `GET /api/v1alpha1/clusters/default/instances/Apps/blog`
- **THEN** the response is `400` with code `bad_request`
- **AND** no authorization review is sent

#### Scenario: The cluster itself

- **WHEN** a client sends `GET /api/v1alpha1/clusters/default`
- **THEN** the response is a `Cluster` document

### Requirement: In-cluster mode serves no operator text

In `in-cluster` mode no document, whether served by a `GET` or on the change stream, SHALL carry
text the operator wrote: a condition's `message` (a registration's `conditions` included), a
reconcile `message`, a history entry's `message`, a registration's `message` or `activeMessage`,
the `note` of an event the operator reported (its `reportingController` is `opm-controller`, or it
names none and regards an `opmodel.dev` object), the health `message` of an inventory object or
graph node of an `opmodel.dev` kind, or, in an `Object` of an `opmodel.dev` kind,
`status.conditions[].message` and `status.history[].message`. Every reason, state, `tone`,
`meaning`, `nextStep`, history `outcome` and `providerOf` standing SHALL stay. Text other writers
wrote SHALL be served as written: the notes of events the kubelet or another controller reported,
and the health `message` of an object outside `opmodel.dev`. Events the operator reported that are
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
- **THEN** every event the operator reported carries its type, reason, count and times, and no
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
- **THEN** every message and note is served as the operator and the API server wrote it

#### Scenario: Registration conditions in-cluster

- **WHEN** a client of an in-cluster server reads the platform document
- **THEN** each registration's `conditions` carry type, status, reason, tone and meaning, and no
  `message`
