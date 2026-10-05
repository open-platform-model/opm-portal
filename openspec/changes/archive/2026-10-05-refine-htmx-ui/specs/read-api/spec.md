## ADDED Requirements

### Requirement: Conditions carry their tone and the portal's explanation

Every condition a document serves SHALL carry `tone`, how the condition reads for its type:
`normal`, `abnormal`, `progressing`, `informational` or `unknown`. `Stalled=True` SHALL be
`abnormal`, `Reconciling=True` `progressing`, `ContractsFulfilled=False` and `Drifted=True`
`informational`, `Ready=False` `abnormal`; an `Unknown` status or a type the portal does not know
SHALL be `unknown`. A condition whose reason the portal explains SHALL carry its `meaning` and,
when a person can act on it, its `nextStep`; a reason the portal does not know SHALL carry
neither. Source: 0030:D2:R1, 0030:D3:R8.

#### Scenario: A refused registration's platform

- **WHEN** a client reads the F1 platform, whose `ContractsFulfilled` is `False`
- **THEN** that condition's `tone` is `informational` and its `meaning` explains unfulfilled
  contracts

#### Scenario: An unknown reason

- **WHEN** a condition carries a reason the portal does not know
- **THEN** it has no `meaning` and no `nextStep`, and its message is served unchanged
