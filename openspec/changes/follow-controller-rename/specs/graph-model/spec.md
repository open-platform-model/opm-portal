## MODIFIED Requirements

### Requirement: Every node and edge is traceable to a recorded field

A graph SHALL be built only from read-model views; no graph SHALL be produced by rendering a
module or by a read of its own. Every edge SHALL belong to one edge kind, and every edge of a kind
SHALL be drawn from that kind's one source, which the edge names: Platform `status.registry` for
platform to catalog; the `status.registry` entries with source `Registration`, joined on the
`spec.catalog` and `spec.version` of a registration that is accepted and active, for registration
to catalog; the registration's
`spec.providerRef` for registration to provider instance; `spec.module` for instance to module;
`spec.sourceRef` and `spec.dependsOn` for package to source and to package; `status.inventory`
for owner to component and component to object; controller `metadata.ownerReferences` for object
to runtime child. A `spec.dependsOn` entry naming another namespace, which the controller refuses,
SHALL be drawn unverified with reason `ForeignNamespace`. Source: portal:D4:R1.

#### Scenario: cert-manager's edges

- **WHEN** the instance graph of `cert-manager/cert-manager` is built from the F1 capture
- **THEN** every edge has one of the kinds `instantiates`, `hasComponent`, `owns` or `controls`,
  and names its source

#### Scenario: Refused claim's catalog

- **WHEN** the platform graph is built from the F1 capture
- **THEN** registration `default.refused-claim-fixture` has no `contributes` edge, because its
  `spec.catalog` is in no registry entry, and its node still names that catalog as text
- **AND** registration `default.backup-provider` contributes to
  `testing.opmodel.dev/catalogs/controller/backup@v0`

#### Scenario: Dependency in another namespace

- **WHEN** a package's `spec.dependsOn` names `shared` in namespace `other`
- **THEN** its `dependsOn` edge to `mp:other/shared` is unverified with reason `ForeignNamespace`

#### Scenario: Duplicate claim on a contributed catalog

- **WHEN** a second registration claims `testing.opmodel.dev/catalogs/controller/backup@v0` and is
  refused as a duplicate, accepted but not active, or claims another version
- **THEN** it has no `contributes` edge, and `default.backup-provider` still contributes

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
