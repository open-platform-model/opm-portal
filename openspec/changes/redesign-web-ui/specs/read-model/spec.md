## ADDED Requirements

### Requirement: Registrations are joined to the owners that hold them

The read model SHALL answer, from held state and without a read of its own, which
ModuleInstances and ModulePackages hold a TransformerRegistration of a given name in their
`status.inventory`, whatever the owner's kind, and which registrations a given owner's inventory
holds. For a caller it SHALL include an owner only when the caller may list that owner's kind in
its namespace, and SHALL mark the answer incomplete whenever the caller lacks a cluster-wide list of
ModuleInstances or of ModulePackages, whether or not a holder was found. A registration's standing SHALL
be attached only when the caller may list TransformerRegistrations, and SHALL be the
registration's own `status.accepted`, `status.active` and verdict reason. Source:
portal:D15:R1/R2/R4, portal:D7:R2.

#### Scenario: The F1 join

- **WHEN** the caller reads the holders of `default.backup-provider` from the F1 capture
- **THEN** the answer is ModuleInstance `default/backup-provider`, complete

#### Scenario: A package holder

- **WHEN** a held ModulePackage's inventory holds TransformerRegistration `pkg.provider`
- **THEN** the package is a holder of `pkg.provider`, and the registration's standing is read
  from its own status, unchanged

#### Scenario: A namespace the caller may not list

- **WHEN** the caller may list ModuleInstances only in namespace `web`
- **THEN** the holders of `default.backup-provider` are empty and marked incomplete

#### Scenario: Incomplete even with a holder

- **WHEN** the caller may list ModuleInstances cluster-wide but ModulePackages only in namespace
  `default`
- **THEN** the holders of `default.backup-provider` name `default/backup-provider` and are marked
  incomplete

### Requirement: Package views carry their interval and source artifact

A package view SHALL carry `spec.interval` as written and the `artifactRevision` and
`artifactDigest` of `status.source` when the controller recorded them, and nothing in their place
when it did not. Source: portal:D2:R4.

#### Scenario: No Flux on the fixture cluster

- **WHEN** the caller reads package `pkg/podinfo` from the F1 capture
- **THEN** its interval is `1m` and it carries no source artifact

### Requirement: The server version is read once, like discovery

When it starts, the read model SHALL read the API server's version from `/version` once, as its
reader, the way it reads the discovery documents: without an access review. It SHALL hold the
`gitVersion`. A failed read SHALL leave no version held and SHALL not stop the read model from
starting. The read model SHALL read no non-resource path other than `/version` and the discovery
documents. Source: portal:D18:R3.

#### Scenario: Version read

- **WHEN** the read model starts against the F1 fixture cluster
- **THEN** it holds the server's `gitVersion`, `v1.36.1`, after one request to `/version`

#### Scenario: Version read fails

- **WHEN** the request to `/version` fails at start
- **THEN** the read model starts and holds no version
