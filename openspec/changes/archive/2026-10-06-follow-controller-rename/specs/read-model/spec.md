## MODIFIED Requirements

### Requirement: Views show the controller's verdicts on their own axis

The instance, package and platform views SHALL carry the applied state read from the controller's
conditions beside, never merged with, the workload health; a registration SHALL carry accepted,
active and its verdict as separate values. Source: portal:D3:R1/R6, portal:D4:R4.

#### Scenario: CLI-owned instance

- **WHEN** the caller reads instance `web/web`, owned by the CLI
- **THEN** its owner is the CLI, its applied state is managed externally, and its health is
  computed from its two inventory objects

#### Scenario: Apply failed before any inventory

- **WHEN** the caller reads an instance whose apply failed before an inventory was recorded
- **THEN** its applied state is failed and retrying, and its inventory is empty, not unreadable

#### Scenario: Accepted and refused claims

- **WHEN** the caller reads the platform view
- **THEN** `default.backup-provider` is accepted and active, `default.refused-claim-fixture`
  is refused with reason `CatalogUnresolved`, and the registry lists both the subscribed and
  the registered catalog

## RENAMED Requirements

- FROM: `### Requirement: Views show the operator's verdicts on their own axis`
- TO: `### Requirement: Views show the controller's verdicts on their own axis`
