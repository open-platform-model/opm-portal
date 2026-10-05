## ADDED Requirements

### Requirement: Registrations are joined to the owners that hold them

The read model SHALL answer, from held state and without a read of its own, which
ModuleInstances and ModulePackages hold a TransformerRegistration of a given name in their
`status.inventory`, whatever the owner's kind, and which registrations a given owner's inventory
holds. For a caller it SHALL include an owner only when the caller may list that owner's kind in
its namespace, and SHALL say when it left an owner's namespace out. A registration's standing SHALL
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

### Requirement: Package views carry their interval and source artifact

A package view SHALL carry `spec.interval` as written and the `artifactRevision` and
`artifactDigest` of `status.source` when the controller recorded them, and nothing in their place
when it did not. Source: portal:D2:R4.

#### Scenario: No Flux on the fixture cluster

- **WHEN** the caller reads package `pkg/podinfo` from the F1 capture
- **THEN** its interval is `1m` and it carries no source artifact

### Requirement: The server version is read once, after a review

When it starts, the read model SHALL read the API server's version from `/version` as its reader,
only after the reader's review of `get` on the non-resource path `/version` allowed it, and SHALL
hold the `gitVersion`. A denied or failed review, or a failed read, SHALL leave no version held
and SHALL not stop the read model from starting. Source: portal:D18:R3.

#### Scenario: Review allowed

- **WHEN** the read model starts with a reader allowed `get /version`
- **THEN** it holds the server's `gitVersion`, and the only request to `/version` came after the
  review

#### Scenario: Review denied

- **WHEN** the reader's review of `get /version` is denied
- **THEN** the read model starts, holds no version, and sends no request to `/version`
