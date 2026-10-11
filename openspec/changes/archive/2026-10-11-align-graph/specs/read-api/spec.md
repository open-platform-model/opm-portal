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
`noArtifact`, `unknown`, `notFound`, `kindNotServed` or `unsupportedKind`, an extensible set), and,
when the object was read, its Ready condition's `reason` and `message` and its artifact `revision`.
Package list items SHALL NOT carry it. Source: portal:D4:R1, portal:D7:R3, portal:D20.

#### Scenario: The F1 package

- **WHEN** a client reads package `pkg/podinfo` from the F1 capture
- **THEN** its `sourceState` has `state` `kindNotServed`, and the document is served with status
  200

#### Scenario: A source that is not ready

- **WHEN** the package's OCIRepository has `Ready=False` with reason `GitOperationFailed` and a
  message
- **THEN** `sourceState` has `state` `notReady`, that reason and that message, and no URL

### Requirement: A source lookup never fails the document or serves the fetch URL

A failed source lookup SHALL NOT fail the `Package` document or its `Graph`; the failure is the
`state`. The source artifact's fetch URL SHALL NOT be served. Source: portal:D18:R3, portal:D20.

#### Scenario: A lookup that fails

- **WHEN** the source lookup finds the kind not served, or the caller refused
- **THEN** the package document is still served with status 200, and carries no artifact URL

### Requirement: A package's source kind comes from its sourceRef kind alone

The source's kind SHALL be chosen by `spec.sourceRef.kind` alone, in `source.toolkit.fluxcd.io`, as
the controller chooses it. A kind the cluster does not serve SHALL be `kindNotServed`, found through
discovery without an access review or a read of the object. Source: portal:D18:R3, portal:D20.

#### Scenario: A sourceRef without an apiVersion

- **WHEN** a package's `spec.sourceRef` names kind `OCIRepository` with no `apiVersion`, on a
  cluster that serves it
- **THEN** the source is read as an OCIRepository in `source.toolkit.fluxcd.io`, not reported as
  unsupported

### Requirement: A source is read as the caller

A caller who may not get the source SHALL see its `sourceState` with `access` `forbidden` and no
state read from the object, and the source node of its graph with the same `access`; the source
SHALL be authorized in the sourceRef's namespace, not the package's. Source: portal:D7:R3,
portal:D20:R2.

#### Scenario: A source the caller may not read

- **WHEN** a caller who may get ModulePackages but not OCIRepositories in namespace `pkg` reads the
  package
- **THEN** its `sourceState` has `access` `forbidden` and no state read from the object, and the
  source node of its graph carries `access` `forbidden`

#### Scenario: A source in another namespace

- **WHEN** a package in `pkg-a` names the OCIRepository `pkg-b/podinfo-release`, and a caller who
  may get ModulePackages and OCIRepositories in `pkg-a` but not OCIRepositories in `pkg-b` reads
  the package
- **THEN** its `sourceState` has `access` `forbidden` and no state read from the object, the source
  node of its graph carries `access` `forbidden`, and the cluster receives no `get` for the source

### Requirement: Served Flux source text is cut to a cap

A served Ready `message` SHALL be at most 1024 Unicode code points, and a served `reason` and
`revision` at most 256 each, in local mode and in-cluster; a longer value SHALL be cut to the cap
less one code point and end with an ellipsis. The cap SHALL hold in the `Package` document, in the
source node's `sourceState`, and in the health message of the source node. Source: portal:D20:R6.

#### Scenario: A message longer than the cap

- **WHEN** a client reads a package whose OCIRepository has `Ready=False` and a Ready message of
  5000 code points, from a local server
- **THEN** `sourceState.message` and the source node's health message hold the first 1023 code
  points and an ellipsis

#### Scenario: A reason and a revision longer than the cap

- **WHEN** a client reads a package whose OCIRepository has a Ready reason of 600 code points and an
  artifact revision of 600 code points
- **THEN** `sourceState.reason` and `sourceState.revision` each hold their first 255 code points and
  an ellipsis, and a value of exactly 256 code points is served unchanged

#### Scenario: In-cluster

- **WHEN** a client of an in-cluster server reads the package, whose OCIRepository has a Ready
  message, a Ready reason and an artifact revision each longer than its cap
- **THEN** `sourceState.message`, `sourceState.reason` and `sourceState.revision` are served as
  source-controller wrote them, since the operator did not write them, each cut to its cap with an
  ellipsis as in a local server
