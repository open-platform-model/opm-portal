## MODIFIED Requirements

### Requirement: Layout is deterministic

Each scope SHALL place nodes in a fixed column per node kind, order each column by the barycenter
of its neighbours with a stable tie-break, and give every node integer coordinates and every edge
a route. An expanded configuration group's band is the one exception to barycenter order: its
members are ordered by name, their objects follow in member order, and every other node of those
columns is placed above or below the band in its barycenter order; once a graph holds a band,
every column is placed on one row grid shared by all columns. Building the same graph from the
same views SHALL produce identical layouts. Source: portal:D4:R5.

#### Scenario: Golden stability

- **WHEN** the F1 graphs are built twice, in separate runs
- **THEN** their JSON and their SVG are byte-identical to the committed goldens

#### Scenario: A band keeps its members in name order

- **WHEN** cert-manager's graph is built with the RBAC group expanded, from the inventory in two
  different orders
- **THEN** the RBAC members sit in the same rows, ordered by name, in both

## ADDED Requirements

### Requirement: Configuration components are grouped per kind family

Components that hold no workload (no Deployment, StatefulSet, DaemonSet, ReplicaSet, Job, CronJob
or Pod entry) SHALL be sorted into kind families by the API group of their objects: CRDs when every
object is in `apiextensions.k8s.io`, RBAC when every object is in `rbac.authorization.k8s.io`,
Webhook configuration when every object is in `admissionregistration.k8s.io`, and Other
configuration otherwise. A component that holds an `opmodel.dev` TransformerRegistration SHALL NOT
be put in a family. A family whose components hold two or more inventory objects in all SHALL be
shown as one group node, naming its family, with its objects hidden, its members' names listed
sorted, its objects counted by kind, and the worst health of its members, partial when any is, so
a reordered inventory yields the same group; a family with fewer objects SHALL show its components
as ordinary nodes. Each group SHALL be shown expanded when the graph is asked to expand its id.
Source: portal:D4:R5 (as amended 2026-10-06).

#### Scenario: cert-manager at rest

- **WHEN** the instance graph of `cert-manager/cert-manager` is built from the F1 capture with
  default options
- **THEN** it holds the components `cainjector`, `controller`, `webhook` and `namespace`, a CRDs
  group of 1 component and 6 objects, an RBAC group of 13 components and 26 objects, and a Webhook
  configuration group of 2 components and 2 objects, and 23 nodes in all

#### Scenario: RBAC expanded

- **WHEN** the same graph is built asking to expand the RBAC group's id
- **THEN** its 13 components and their 26 objects are nodes, the CRDs and Webhook configuration
  groups stay folded, and the graph holds 61 nodes

#### Scenario: Every group expanded

- **WHEN** the same graph is built asking to expand all three groups
- **THEN** all 20 components and their 42 inventory objects are nodes

#### Scenario: A provider's claim stays in view

- **WHEN** an instance has two configuration components, one holding a TransformerRegistration and
  a ConfigMap and the other two ConfigMaps
- **THEN** the component holding the TransformerRegistration is an ordinary node, and the other
  forms an Other configuration group only if its objects number two or more

#### Scenario: Configuration health kept

- **WHEN** an RBAC group's members are all Current under kstatus
- **THEN** the group's health is Healthy, read from its members, and not left empty

### Requirement: An expanded group is laid out as one band

Each expanded configuration group SHALL occupy a band of consecutive rows, on a row grid shared
by every column, across the components column and every column to its right, holding its members
and the objects they own and no other node, with half a row's margin above and below. No node that
is neither a member nor an object of a member SHALL lie within the rectangle the band spans, and
no edge between two nodes outside the band SHALL cross it. The band's placement SHALL be
deterministic. Source: portal:D4:R5.

#### Scenario: RBAC open beside the workloads

- **WHEN** cert-manager's graph is built with the RBAC group expanded
- **THEN** its 13 members occupy consecutive rows of the components column, their 26 objects sit
  beside them, neither `controller` nor any of its objects lies inside the band, and no edge of
  `controller` crosses it

#### Scenario: Two groups open

- **WHEN** the same graph is built with the RBAC and CRDs groups both expanded
- **THEN** each group has its own band and the two bands do not overlap

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
  verdict `Accepted`, catalog `testing.opmodel.dev/catalogs/operator/backup@v0` and version `0.1.0`

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
it exists, and its Ready condition's status, reason and message and its artifact revision. The node
SHALL carry a health mapped from that state (ready Healthy; not ready Degraded with the Ready
reason and message; ready with no artifact, or unknown, Unknown), which SHALL not count toward the
package's own health. A source the caller may not read SHALL be marked with its access and no
state or health; a source that does not exist, or whose kind the cluster does not serve, SHALL be
marked missing. Source: portal:D4:R1, portal:D20.

#### Scenario: Flux not installed

- **WHEN** the graph of package `pkg/podinfo` is built from the F1 capture, whose cluster serves no
  `source.toolkit.fluxcd.io` kinds
- **THEN** its source node's state says the kind is not served

#### Scenario: Source ready

- **WHEN** the package's OCIRepository has `Ready=True` and an artifact revision
- **THEN** the source node's state is ready, with that revision, and its health is Healthy

#### Scenario: Source forbidden

- **WHEN** the caller may get the package but not its OCIRepository
- **THEN** the source node carries access `forbidden`, no state and no health

#### Scenario: Source not found

- **WHEN** the package's sourceRef names an OCIRepository that does not exist and the caller may
  get it
- **THEN** the source node is marked missing with state not found

## REMOVED Requirements

### Requirement: Configuration components are grouped by default

**Reason**: The owner amended portal:D4:R5 on 2026-10-06 ("Groups per kind"): configuration
components are grouped per kind family, not into one node per owner.

**Migration**: See "Configuration components are grouped per kind family" and "An expanded group is
laid out as one band". Group ids gain a family part (`grp:configuration/<owner>/<family>`); an
`expand` value with the old id names no node and the graph opens with every group folded.
