## ADDED Requirements

### Requirement: Graph groups name their family and the kinds they fold

A configuration group node in a `Graph` document SHALL carry `family`, one of `crds`, `rbac`,
`webhooks` and `other` (an extensible set), and `objectKinds`, the inventory objects its members
hold counted by kind and sorted by kind. A client that does not know a family SHALL still find the
group's `kind`, `members` and health. Source: portal:D4:R5.

#### Scenario: cert-manager's RBAC group

- **WHEN** a client reads cert-manager's instance graph from the F1 capture
- **THEN** one group node has `family` `rbac`, 13 `members`, and `objectKinds` counting 10
  ClusterRole, 10 ClusterRoleBinding, 3 Role and 3 RoleBinding objects

#### Scenario: Group ids name the family

- **WHEN** a client reads the same graph with `expand` set to that group's id
- **THEN** the id ends in `/rbac`, and the response holds the 13 components as nodes

### Requirement: Packages carry their source's state

A `Package` document, and the source node of a package's `Graph` document, SHALL carry
`sourceState`: the caller's `access` to the source object, its `state` (`ready`, `notReady`,
`unknown`, `notFound`, `kindNotServed` or `unsupportedKind`, an extensible set), and, when the
object was read, its Ready condition's `reason` and `message` and its artifact `revision`. A failed
source lookup SHALL NOT fail the document. The source artifact's fetch URL SHALL NOT be served.
Package list items SHALL NOT carry it. Source: portal:D4:R1, portal:D7:R3, portal:D20.

#### Scenario: The F1 package

- **WHEN** a client reads package `pkg/podinfo` from the F1 capture
- **THEN** its `sourceState` has `state` `kindNotServed`, and the document is served with status
  200

#### Scenario: A source the caller may not read

- **WHEN** a caller who may get ModulePackages but not OCIRepositories in namespace `pkg` reads the
  package
- **THEN** its `sourceState` has `access` `forbidden` and no state read from the object, and the
  source node of its graph carries `access` `forbidden`

#### Scenario: A source that is not ready

- **WHEN** the package's OCIRepository has `Ready=False` with reason `GitOperationFailed` and a
  message
- **THEN** `sourceState` has `state` `notReady`, that reason and that message, and no URL

#### Scenario: In-cluster

- **WHEN** a client of an in-cluster server reads the package
- **THEN** `sourceState.message` is served as source-controller wrote it, since the operator did
  not write it
