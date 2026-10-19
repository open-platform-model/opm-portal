## REMOVED Requirements

### Requirement: Every node and edge is traceable to a recorded field
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "A graph is built only from read-model views", "Every edge is drawn from its kind's one source, for platform, registration and provider edges", "Every edge is drawn from its kind's one source, for module, package, inventory and runtime child edges", "A dependsOn entry in another namespace is drawn unverified".

### Requirement: A provider disagreement is shown, not resolved
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "A registration's edge to its provider instance is verified or unverified with a reason", "A provider that could not be read or was not looked up is still drawn".

### Requirement: Layout is deterministic
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Each scope places nodes in fixed columns and the same graph gives the same layout", "An expanded configuration group's band is the one exception to barycenter order".

## ADDED Requirements

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
  `testing.opmodel.dev/catalogs/operator/backup@v0`

#### Scenario: Duplicate claim on a contributed catalog

- **WHEN** a second registration claims `testing.opmodel.dev/catalogs/operator/backup@v0` and is
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
A `spec.dependsOn` entry naming another namespace, which the operator refuses, SHALL be drawn
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
