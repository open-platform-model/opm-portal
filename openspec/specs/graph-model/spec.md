# graph-model Specification

## Purpose
How the portal derives its relationship graphs (one instance or package, and the platform) from
read-model views: the nodes and edges it may draw and the one source each edge comes from, the
stable node ids, the rules that collapse a large graph, and the deterministic layout.

## Requirements

### Requirement: No edge from an instance to a contract

A graph SHALL NOT contain a contract node or an edge from an instance to a contract. An instance
node SHALL carry the contracts its render used as text, labelled as the render's contracts.
Source: portal:D4:R3.

#### Scenario: cert-manager's contracts

- **WHEN** the instance graph of `cert-manager/cert-manager` is built from the F1 capture
- **THEN** the instance node lists 15 render contracts and no node or edge names a contract

### Requirement: Registrations show acceptance and activation apart

A registration node SHALL carry `accepted` and `active` as two values, its verdict, and the
reason and message of its Ready condition, as `internal/health` read them from the status
fields. A blocked removal SHALL keep its verdict `RemovalBlocked`, never refused.
Source: portal:D4:R4/R7.

#### Scenario: Refused claim

- **WHEN** the platform graph is built from the F1 capture
- **THEN** node `treg:default.refused-claim-fixture` is neither accepted nor active, its verdict
  is `Refused` with reason `CatalogUnresolved` and the controller's message
- **AND** node `treg:default.backup-provider` is accepted and active with verdict `Accepted`

### Requirement: Node ids are stable

Node ids SHALL have the form `<prefix>:<parts>`, built from kind, group, namespace and name only,
never from a UID, so that the id of an object survives a portal restart and a delete and
recreate of the object. Source: portal:D4:R6.

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
SHALL count the dropped nodes by kind. The summary SHALL carry the worst health state of the
dropped nodes and count them by health state and by access, so a Degraded node is never hidden
behind a count. A cap below two SHALL be two: the root and the summary. No edge SHALL point at a
dropped node.

#### Scenario: Small cap

- **WHEN** the expanded cert-manager graph is built with a cap of 30
- **THEN** it holds 30 nodes, one of them the summary, and the summary's counts add up to the
  nodes dropped

#### Scenario: Cap of one

- **WHEN** the cert-manager graph is built with a cap of 1
- **THEN** it holds the root and the summary

#### Scenario: Capped broken rollout

- **WHEN** the image-break podinfo graph is built with a cap that drops every Pod
- **THEN** the summary is Degraded and counts one Degraded Pod

### Requirement: Unreadable items are shown as such

An object the caller may not read SHALL be a node marked with its access and no health, and a
platform whose registrations the caller may not list SHALL say so on its node instead of showing
none. Source: portal:D7:R3.

#### Scenario: Registrations forbidden

- **WHEN** the platform view says the caller may not list registrations
- **THEN** the platform graph holds the platform and its catalogs, no registration, and the
  platform node marks registrations forbidden

### Requirement: Configuration components are grouped per kind family

Components that hold no workload (no Deployment, StatefulSet, DaemonSet, ReplicaSet, Job, CronJob
or Pod entry) SHALL be sorted into kind families by the API group of their objects: CRDs when every
object is in `apiextensions.k8s.io`, RBAC when every object is in `rbac.authorization.k8s.io`,
Webhook configuration when every object is in `admissionregistration.k8s.io`, and Other
configuration otherwise. Source: portal:D4:R5 (as amended 2026-10-06).

#### Scenario: A family by API group

- **WHEN** cert-manager's instance graph is built from the F1 capture
- **THEN** its CRD components fall in the CRDs family, its ClusterRole, ClusterRoleBinding, Role and
  RoleBinding components in the RBAC family, and its two webhook configuration components in the
  Webhook configuration family

### Requirement: A provider's claim stays out of every family

A component that holds an `opmodel.dev` TransformerRegistration SHALL NOT be put in a family, so
the claim stays in view. Source: portal:D4:R5, portal:D4:R4.

#### Scenario: A provider's claim stays in view

- **WHEN** an instance has two configuration components, one holding a TransformerRegistration and
  a ConfigMap and the other two ConfigMaps
- **THEN** the component holding the TransformerRegistration is an ordinary node, and the other
  forms an Other configuration group only if its objects number two or more

### Requirement: A family of two or more objects is one group node

A family whose components hold two or more inventory objects in all SHALL be shown as one group
node, naming its family, with its objects hidden, its members' names listed sorted and its objects
counted by kind, so a reordered inventory yields the same group. A family with fewer objects SHALL
show its components as ordinary nodes. Source: portal:D4:R5.

#### Scenario: cert-manager at rest

- **WHEN** the instance graph of `cert-manager/cert-manager` is built from the F1 capture with
  default options
- **THEN** it holds the components `cainjector`, `controller`, `webhook` and `namespace`, a CRDs
  group of 1 component and 6 objects, an RBAC group of 13 components and 26 objects, and a Webhook
  configuration group of 2 components and 2 objects, and 23 nodes in all

#### Scenario: A reordered inventory

- **WHEN** the same graph is built from an inventory in another order
- **THEN** it holds the same groups with the same members, sorted

### Requirement: A configuration group's health is the worst of its members'

A configuration group's health SHALL be the worst of its members', partial when any member is and
not live when any member is. Configuration objects keep their kstatus health, so a group of
members that are all Current is Healthy. Source: portal:D3:R1.

#### Scenario: Configuration health kept

- **WHEN** an RBAC group's members are all Current under kstatus
- **THEN** the group's health is Healthy, read from its members, and not left empty

### Requirement: A configuration group is shown expanded when asked

A configuration group SHALL be shown expanded when the graph is asked to expand its id, and only
the groups asked for SHALL be shown expanded. Source: portal:D4:R5.

#### Scenario: RBAC expanded

- **WHEN** the same graph is built asking to expand the RBAC group's id
- **THEN** its 13 components and their 26 objects are nodes, the CRDs and Webhook configuration
  groups stay folded, and the graph holds 61 nodes

#### Scenario: Every group expanded

- **WHEN** the same graph is built asking to expand all three groups
- **THEN** all 20 components and their 42 inventory objects are nodes

### Requirement: An expanded group is laid out as one band

Each expanded configuration group SHALL occupy a band of consecutive rows, on a row grid shared by
every column, across the components column and every column to its right, holding its members and
the objects they own and no other node, with half a row's margin above and below. The band's
placement SHALL be deterministic. Source: portal:D4:R5.

#### Scenario: Two groups open

- **WHEN** the same graph is built with the RBAC and CRDs groups both expanded
- **THEN** each group has its own band and the two bands do not overlap

### Requirement: A band holds nothing but its group and is never crossed

No node that is neither a member nor an object of a member SHALL lie within the rectangle a band
spans, and no edge between two nodes outside the band SHALL cross it. Source: portal:D4:R5.

#### Scenario: RBAC open beside the workloads

- **WHEN** cert-manager's graph is built with the RBAC group expanded
- **THEN** its 13 members occupy consecutive rows of the components column, their 26 objects sit
  beside them, neither `controller` nor any of its objects lies inside the band, and no edge of
  `controller` crosses it

### Requirement: Package graphs read from the package

A package graph SHALL place the package in its first column and its source and the packages its
`spec.dependsOn` names in the components column, ahead of the components, so that every edge from
the package runs toward later columns. Its column titles SHALL name what each column holds; a
runtime column SHALL be titled by the kinds it holds ("ReplicaSets", "Pods", "Jobs"), or
"Runtime" when it holds more than one kind. Source: portal:D4:R1.

#### Scenario: The F1 package

- **WHEN** the graph of package `pkg/podinfo` is built from the F1 capture
- **THEN** the package node is in the first column, its `OCIRepository podinfo-release` source
  node is in the next one, and the `sourcedFrom` edge runs from left to right

#### Scenario: A StatefulSet's runtime column

- **WHEN** an instance graph holds a StatefulSet whose Pods are its runtime children, and no
  Deployment
- **THEN** the first runtime column is titled "Pods"

### Requirement: Instance and package graphs carry the standing of held registrations

In an instance or package graph, the node of a TransformerRegistration the owner's inventory holds
SHALL carry the registration's own accepted, active, verdict, reason and message, catalog, version
and provided contracts, as a registration node of the platform graph does, when the caller may read
registrations. When the caller may not, the node SHALL carry no standing. Source: portal:D15:R2/R4,
portal:D4:R4.

#### Scenario: backup-provider

- **WHEN** the instance graph of `default/backup-provider` is built from the F1 capture
- **THEN** its TransformerRegistration node `default.backup-provider` is accepted and active, with
  verdict `Accepted`, catalog `testing.opmodel.dev/catalogs/controller/backup@v0` and version `0.1.0`

#### Scenario: A package holder refused

- **WHEN** the graph of a package whose inventory holds a registration the controller refused with
  `ProviderMismatch` is built
- **THEN** that registration's node is neither accepted nor active, with verdict `Refused` and
  reason `ProviderMismatch`

#### Scenario: Registrations forbidden

- **WHEN** a caller who may not list TransformerRegistrations builds backup-provider's graph
- **THEN** the registration node carries no standing

### Requirement: A package's source node carries the source's state

The source node of a package graph SHALL carry the state of the source object as the caller read
it: whether the cluster serves its kind, whether the caller and the portal could read it, whether
it exists, and its Ready condition's status, reason and message and its artifact revision. Source:
portal:D4:R1, portal:D20.

#### Scenario: Flux not installed

- **WHEN** the graph of package `pkg/podinfo` is built from the F1 capture, whose cluster serves no
  `source.toolkit.fluxcd.io` kinds
- **THEN** its source node's state says the kind is not served

### Requirement: A source node's health is mapped from its state

The source node SHALL carry a health mapped from its state (ready Healthy; not ready Degraded with
the Ready reason and message; ready with no artifact, or unknown, Unknown), which SHALL not count
toward the package's own health. Source: portal:D4:R1, portal:D20.

#### Scenario: Source ready

- **WHEN** the package's OCIRepository has `Ready=True` and an artifact revision
- **THEN** the source node's state is ready, with that revision, and its health is Healthy

### Requirement: A source node marks what the caller could not read or find

A source the caller may not read SHALL be marked with its access and no state or health. A source
that does not exist, or whose kind the cluster does not serve, SHALL be marked missing. Source:
portal:D4:R1, portal:D20.

#### Scenario: Source forbidden

- **WHEN** the caller may get the package but not its OCIRepository
- **THEN** the source node carries access `forbidden`, no state and no health

#### Scenario: Source not found

- **WHEN** the package's sourceRef names an OCIRepository that does not exist and the caller may
  get it
- **THEN** the source node is marked missing with state not found

### Requirement: A graph is built only from read-model views
A graph SHALL be built only from read-model views; no graph SHALL be produced by rendering a module
or by a read of its own. Source: portal:D4:R1.

#### Scenario: A graph is built

- **WHEN** a graph is built
- **THEN** it SHALL be built only from read-model views, and no graph SHALL be produced by rendering a module or by a read of its own

### Requirement: Every edge is drawn from its kind's one source, for platform, registration and provider edges
Every edge SHALL belong to one edge kind, and every edge of a kind SHALL be drawn from that kind's
one source, which the edge names: Platform `status.registry` for platform to catalog; the
`status.registry` entries with source `Registration`, joined on the `spec.catalog` and
`spec.version` of a registration that is accepted and active, for registration to catalog; the
registration's `spec.providerRef` for registration to provider instance. Source: portal:D4:R1.

#### Scenario: Refused claim's catalog

- **WHEN** the platform graph is built from the F1 capture
- **THEN** registration `default.refused-claim-fixture` has no `contributes` edge, because its
  `spec.catalog` is in no registry entry, and its node still names that catalog as text
- **AND** registration `default.backup-provider` contributes to
  `testing.opmodel.dev/catalogs/controller/backup@v0`

#### Scenario: Duplicate claim on a contributed catalog

- **WHEN** a second registration claims `testing.opmodel.dev/catalogs/controller/backup@v0` and is
  refused as a duplicate, accepted but not active, or claims another version
- **THEN** it has no `contributes` edge, and `default.backup-provider` still contributes

### Requirement: Every edge is drawn from its kind's one source, for module, package, inventory and runtime child edges
Every edge SHALL belong to one edge kind, and every edge of a kind SHALL be drawn from that kind's
one source, which the edge names: `spec.module` for instance to module; `spec.sourceRef` and
`spec.dependsOn` for package to source and to package; `status.inventory` for owner to component and
component to object; controller `metadata.ownerReferences` for object to runtime child. Source:
portal:D4:R1.

#### Scenario: cert-manager's edges

- **WHEN** the instance graph of `cert-manager/cert-manager` is built from the F1 capture
- **THEN** every edge has one of the kinds `instantiates`, `hasComponent`, `owns` or `controls`,
  and names its source

### Requirement: A dependsOn entry in another namespace is drawn unverified
A `spec.dependsOn` entry naming another namespace, which the controller refuses, SHALL be drawn
unverified with reason `ForeignNamespace`. Source: portal:D4:R1.

#### Scenario: Dependency in another namespace

- **WHEN** a package's `spec.dependsOn` names `shared` in namespace `other`
- **THEN** its `dependsOn` edge to `mp:other/shared` is unverified with reason `ForeignNamespace`

### Requirement: A registration's edge to its provider instance is verified or unverified with a reason
Every registration SHALL have an edge to the instance its `spec.providerRef` names. The edge SHALL
be verified only when that instance was read and its inventory holds a TransformerRegistration of
the registration's name; otherwise it SHALL be unverified with the reason: the provider was not
found, could not be read, does not hold the registration in its inventory, or was not looked up by
the caller. Source: portal:D4:R2.

#### Scenario: Accepted claim

- **WHEN** the platform graph is built from the F1 capture
- **THEN** the edge from `default.backup-provider` to instance `default/backup-provider` is
  verified

#### Scenario: Provider that does not exist

- **WHEN** the platform graph is built from the F1 capture
- **THEN** the edge from `default.refused-claim-fixture` to instance
  `default/refused-claim-fixture` is unverified with reason `ProviderNotFound`

### Requirement: A provider that could not be read or was not looked up is still drawn
A provider that could not be read SHALL still be drawn, marked with its access; one that was not
looked up SHALL be drawn with no access, never a guessed one. Source: portal:D4:R2.

#### Scenario: Provider the caller may not read

- **WHEN** the provider lookup for `default/backup-provider` is forbidden
- **THEN** the edge is unverified with reason `ProviderUnreadable` and the provider node is
  marked forbidden

#### Scenario: Provider not looked up

- **WHEN** the caller passes no lookup for `default/backup-provider`
- **THEN** the edge is unverified with reason `ProviderNotLookedUp` and the provider node carries
  no access

### Requirement: Each scope places nodes in fixed columns and the same graph gives the same layout
Each scope SHALL place nodes in a fixed column per node kind, order each column by the barycenter of
its neighbours with a stable tie-break, and give every node integer coordinates and every edge a
route. Building the same graph from the same views SHALL produce identical layouts. Source:
portal:D4:R5.

#### Scenario: Golden stability

- **WHEN** the F1 graphs are built twice, in separate runs
- **THEN** their JSON and their SVG are byte-identical to the committed goldens

### Requirement: An expanded configuration group's band is the one exception to barycenter order
An expanded configuration group's band SHALL be the one exception to barycenter order: its members
are ordered by name, their objects follow in member order, and every other node of those columns is
placed above or below the band in its barycenter order; once a graph holds a band, every column is
placed on one row grid shared by all columns. Source: portal:D4:R5.

#### Scenario: A band keeps its members in name order

- **WHEN** cert-manager's graph is built with the RBAC group expanded, from the inventory in two
  different orders
- **THEN** the RBAC members sit in the same rows, ordered by name, in both
