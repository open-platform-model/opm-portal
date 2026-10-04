# graph-model Specification

## Purpose
How the portal derives its relationship graphs (one instance or package, and the platform) from
read-model views: the nodes and edges it may draw and the one source each edge comes from, the
stable node ids, the rules that collapse a large graph, and the deterministic layout.

## Requirements

### Requirement: Every node and edge is traceable to a recorded field

A graph SHALL be built only from read-model views; no graph SHALL be produced by rendering a
module or by a read of its own. Every edge SHALL belong to one edge kind, and every edge of a kind
SHALL be drawn from that kind's one source, which the edge names: Platform `status.registry` for
platform to catalog; the `status.registry` entries with source `Registration`, joined on the
registration's `spec.catalog`, for registration to catalog; the registration's
`spec.providerRef` for registration to provider instance; `spec.module` for instance to module;
`spec.sourceRef` and `spec.dependsOn` for package to source and to package; `status.inventory`
for owner to component and component to object; controller `metadata.ownerReferences` for object
to runtime child. Source: 0030:D4:R1.

#### Scenario: cert-manager's edges

- **WHEN** the instance graph of `cert-manager/cert-manager` is built from the F1 capture
- **THEN** every edge has one of the kinds `instantiates`, `hasComponent`, `owns` or `controls`,
  and names its source

#### Scenario: Refused claim's catalog

- **WHEN** the platform graph is built from the F1 capture
- **THEN** registration `default.refused-claim-fixture` has no `contributes` edge, because its
  `spec.catalog` is in no registry entry, and its node still names that catalog as text
- **AND** registration `default.backup-provider` contributes to
  `testing.opmodel.dev/catalogs/operator/backup@v0`

### Requirement: No edge from an instance to a contract

A graph SHALL NOT contain a contract node or an edge from an instance to a contract. An instance
node SHALL carry the contracts its render used as text, labelled as the render's contracts.
Source: 0030:D4:R3.

#### Scenario: cert-manager's contracts

- **WHEN** the instance graph of `cert-manager/cert-manager` is built from the F1 capture
- **THEN** the instance node lists 15 render contracts and no node or edge names a contract

### Requirement: A provider disagreement is shown, not resolved

Every registration SHALL have an edge to the instance its `spec.providerRef` names. The edge
SHALL be verified only when that instance was read and its inventory holds a
TransformerRegistration of the registration's name; otherwise it SHALL be unverified with the
reason: the provider was not found, could not be read, or does not hold the registration in its
inventory. A provider that could not be read SHALL still be drawn, marked with its access.
Source: 0030:D4:R2.

#### Scenario: Accepted claim

- **WHEN** the platform graph is built from the F1 capture
- **THEN** the edge from `default.backup-provider` to instance `default/backup-provider` is
  verified

#### Scenario: Provider that does not exist

- **WHEN** the platform graph is built from the F1 capture
- **THEN** the edge from `default.refused-claim-fixture` to instance
  `default/refused-claim-fixture` is unverified with reason `ProviderNotFound`

#### Scenario: Provider the caller may not read

- **WHEN** the provider lookup for `default/backup-provider` is forbidden
- **THEN** the edge is unverified with reason `ProviderUnreadable` and the provider node is
  marked forbidden

### Requirement: Registrations show acceptance and activation apart

A registration node SHALL carry `accepted` and `active` as two values, its verdict, and the
reason and message of its Ready condition, as `internal/health` read them from the status
fields. A blocked removal SHALL keep its verdict `RemovalBlocked`, never refused.
Source: 0030:D4:R4/R7.

#### Scenario: Refused claim

- **WHEN** the platform graph is built from the F1 capture
- **THEN** node `treg:default.refused-claim-fixture` is neither accepted nor active, its verdict
  is `Refused` with reason `CatalogUnresolved` and the operator's message
- **AND** node `treg:default.backup-provider` is accepted and active with verdict `Accepted`

### Requirement: Configuration components are grouped by default

When an instance or package has two or more components that hold no workload (no Deployment,
StatefulSet, DaemonSet, ReplicaSet, Job, CronJob or Pod entry), they SHALL be shown as one group
node whose health is the worst of theirs, partial when any is, with their objects hidden. The
group SHALL be shown expanded when the graph is asked to expand its id. Source: 0030:D4:R5.

#### Scenario: cert-manager collapsed

- **WHEN** the instance graph of `cert-manager/cert-manager` is built with default options
- **THEN** it holds the components `cainjector`, `controller` and `webhook` and one
  configuration group of 17 components, and 19 nodes in all

#### Scenario: cert-manager expanded

- **WHEN** the same graph is built asking to expand the configuration group's id
- **THEN** all 20 components and their 42 inventory objects are nodes

### Requirement: Node ids are stable

Node ids SHALL have the form `<prefix>:<parts>`, built from kind, group, namespace and name only,
never from a UID, so that the id of an object survives a portal restart and a delete and
recreate of the object. Source: 0030:D4:R6.

#### Scenario: Recreated Deployment

- **WHEN** podinfo's Deployment is recreated with a new UID and the graph is built again
- **THEN** its node id is unchanged

### Requirement: Runtime children appear only below inventory objects

Runtime children SHALL be drawn only below the inventory object their controller chain reaches.
A ReplicaSet with zero desired replicas SHALL be hidden with its descendants, and its parent
SHALL carry how many were hidden, unless the graph is asked to show them. More than five Pods
under one parent SHALL be one Pod group with counts by health, unless the graph is asked to
expand it.

#### Scenario: Broken rollout

- **WHEN** the instance graph of podinfo is built from the experiment 01 image-break samples
- **THEN** the Deployment controls two ReplicaSets, the new one controls one Pod Degraded with
  `ImagePullBackOff`, and the Deployment and instance are Degraded while the instance is Applied

#### Scenario: ReplicaSet scaled to zero

- **WHEN** a Deployment's old ReplicaSet has zero desired replicas
- **THEN** it is not a node and the Deployment node says one scaled-down ReplicaSet is hidden

### Requirement: A node cap replaces what it drops with a summary

When a graph would hold more nodes than its cap, nodes SHALL be dropped from the last column
backwards, never the graph's root, until one fewer than the cap remain, and one summary node
SHALL count the dropped nodes by kind. No edge SHALL point at a dropped node.

#### Scenario: Small cap

- **WHEN** the expanded cert-manager graph is built with a cap of 30
- **THEN** it holds 30 nodes, one of them the summary, and the summary's counts add up to the
  nodes dropped

### Requirement: Layout is deterministic

Each scope SHALL place nodes in a fixed column per node kind, order each column by the barycenter
of its neighbours with a stable tie-break, and give every node integer coordinates and every edge
a route. Building the same graph from the same views SHALL produce identical layouts.

#### Scenario: Golden stability

- **WHEN** the F1 graphs are built twice, in separate runs
- **THEN** their JSON and their SVG are byte-identical to the committed goldens

### Requirement: Unreadable items are shown as such

An object the caller may not read SHALL be a node marked with its access and no health, and a
platform whose registrations the caller may not list SHALL say so on its node instead of showing
none. Source: 0030:D7:R3.

#### Scenario: Registrations forbidden

- **WHEN** the platform view says the caller may not list registrations
- **THEN** the platform graph holds the platform and its catalogs, no registration, and the
  platform node marks registrations forbidden
