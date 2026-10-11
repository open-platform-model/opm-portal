## ADDED Requirements

### Requirement: Instances carry their inventory revision

An instance document and an instance list item SHALL carry `inventoryRevision`, the integer the
controller recorded in `status.inventory.revision`, when it recorded one of at least 1, and
nothing when it did not, never a default. The field SHALL be optional in the OpenAPI document on
both schemas, and SHALL be served unchanged in local and in-cluster mode, since it is a number and
not controller text. Package documents and items SHALL carry none. Source: portal:D2:R1/R2/R3.

#### Scenario: The F1 instances

- **WHEN** a client reads instance `default/podinfo`, and the instance list, from the F1 capture
- **THEN** the document and its list item carry `inventoryRevision` `1` beside `inventoryCount` `2`,
  and so do the other four capture instances

#### Scenario: An instance never applied

- **WHEN** a client reads an instance whose status holds no inventory
- **THEN** its document and list item carry no `inventoryRevision`, and `inventoryCount` is `0`

#### Scenario: In-cluster

- **WHEN** a client of an in-cluster server reads an instance that recorded revision 3
- **THEN** the document carries `inventoryRevision` `3`

#### Scenario: A package

- **WHEN** a client reads package `pkg/podinfo`, or any package list item
- **THEN** it carries no `inventoryRevision`

#### Scenario: A caller who may not read the instance

- **WHEN** a caller who may not get instance `default/podinfo` reads it
- **THEN** the response is the same `403` document as before and names no revision

#### Scenario: The revision changes

- **WHEN** a client follows `instance:default/podinfo` and the controller records revision 2
- **THEN** the stream delivers an upsert whose document carries `inventoryRevision` `2`

#### Scenario: A field added to the contract

- **WHEN** the pull request adds the optional property to the OpenAPI document
- **THEN** the `Lint` check's API gate passes, and the wire-type test finds the property on both
  `Instance` and `InstanceSummary`
