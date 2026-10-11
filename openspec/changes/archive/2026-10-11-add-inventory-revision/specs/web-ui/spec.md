## ADDED Requirements

### Requirement: The details panel shows an instance's inventory and its revision

The details panel SHALL show, for the node that is the instance page's own instance, "Inventory",
its object count ("1 object", "2 objects"), and "Inventory revision", the revision the API serves.
Inventory SHALL show when the count is above zero or a revision is served; the revision SHALL show
only when the API serves one. Neither SHALL show for a node the caller may not read, for a package,
or for any other node. Source: portal:D2:R1, portal:D17.

#### Scenario: podinfo at rest

- **WHEN** a browser opens `/instances/default/podinfo` with no `node` or `focus`
- **THEN** the panel shows `Inventory` `2 objects` and `Inventory revision` `1`

#### Scenario: One object

- **WHEN** the instance's inventory holds one object, as `default/backup-provider`'s does
- **THEN** the panel reads `1 object`

#### Scenario: Another node selected

- **WHEN** the user selects podinfo's Deployment node
- **THEN** the panel shows the Deployment and neither fact, and Clear selection brings both back

#### Scenario: Applied, nothing rendered

- **WHEN** an instance records revision 2 and an empty inventory
- **THEN** the panel shows `Inventory` `0 objects` and `Inventory revision` `2`

#### Scenario: Never applied

- **WHEN** an instance has recorded no inventory
- **THEN** the panel shows neither fact

#### Scenario: A package panel

- **WHEN** a browser opens `/packages/pkg/podinfo`
- **THEN** the panel shows its source facts, with its `Revision` as the source's, and no
  `Inventory revision`

#### Scenario: A locked node

- **WHEN** the panel opens on a node whose access is not ok
- **THEN** it shows the locked form with no facts, and neither fact

#### Scenario: Phone width

- **WHEN** the instance page is opened at 360 px wide
- **THEN** both facts are readable in the panel and the page does not scroll sideways
