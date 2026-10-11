## ADDED Requirements

### Requirement: Instance views carry the inventory revision

An instance view SHALL carry the inventory revision, the integer the controller recorded in
`status.inventory.revision`, when it is at least 1. A missing field, a value below 1 and a value
of another type SHALL leave the view with no revision. The view SHALL never fill it with a default
or compute it. Source: portal:D2:R4.

#### Scenario: A revision recorded

- **WHEN** the model reads an instance whose `status.inventory.revision` is `3`
- **THEN** its view carries revision 3

#### Scenario: No revision recorded

- **WHEN** the model reads an instance with no `status.inventory`, or with an inventory that holds
  no `revision`
- **THEN** its view carries no revision, and its inventory count is as before

#### Scenario: A value the controller does not write

- **WHEN** the model reads an instance whose `status.inventory.revision` is `0`, `-2` or the string
  `"3"`
- **THEN** its view carries no revision
