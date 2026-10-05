## ADDED Requirements

### Requirement: Applied state is read from the controller's conditions

The portal SHALL read a ModuleInstance's, ModulePackage's, Platform's or
TransformerRegistration's applied state from its conditions and spec as exactly one of Applied,
Reconciling, Failed, Stalled, Suspended, ManagedExternally or Unknown, by the first match of:
a CLI owner or `ManagedExternally` reason is ManagedExternally; suspension is Suspended;
`Stalled=True` is Stalled; `Ready=False` is Failed, marked as retrying while
`Reconciling=True`; `Reconciling=True` or `Ready=Unknown` is Reconciling; `Ready=True` whose
`observedGeneration` is older than the object's generation is Reconciling; `Ready=True` is
Applied; anything else is Unknown. `Ready=True` SHALL be Applied and never Healthy, and the
applied state SHALL NOT be derived from health nor health from it. Failure counters SHALL never
change either axis. `ContractsFulfilled=False` and `Drifted=True` SHALL be informational notes
that never change the state. Source: portal:D3:R1/R6/R7/R8.

#### Scenario: Ready instance with a broken rollout

- **WHEN** the podinfo ModuleInstance captured one minute after the image break
  (`Ready=True/ReconciliationSucceeded`, drift counter 2) is read
- **THEN** its applied state is Applied, while its health from the same capture is Degraded

#### Scenario: Apply failed and retrying

- **WHEN** the captured cert-manager instance with `Reconciling=True` and
  `Ready=False/ApplyFailed` is read
- **THEN** its applied state is Failed with reason `ApplyFailed`, marked as retrying

#### Scenario: CLI-owned instance

- **WHEN** the captured CLI-owned instance (`spec.owner: cli`, `Ready=Unknown/ManagedExternally`)
  is read
- **THEN** its applied state is ManagedExternally, which is not a fault, and its health is
  still computed from its inventory objects

#### Scenario: Fresh Platform with unfulfilled contracts

- **WHEN** the captured fresh Platform (`Ready=True/Generated`,
  `ContractsFulfilled=False/UnfulfilledContracts`) is read
- **THEN** its applied state is Applied and it carries one informational note naming
  `UnfulfilledContracts`

#### Scenario: Package without a source

- **WHEN** the captured ModulePackage with `Ready=False/SourceNotReady` is read
- **THEN** its applied state is Failed with reason `SourceNotReady`

#### Scenario: Spec edited, controller not caught up

- **WHEN** the captured podinfo ModuleInstance is read with its generation one past the
  `observedGeneration` of its `Ready=True` condition
- **THEN** its applied state is Reconciling

#### Scenario: Unknown kind

- **WHEN** an object of a kind other than the four controller kinds is read
- **THEN** its applied state is Unknown

### Requirement: Every controller condition reason has an explanation

The portal SHALL hold, for every reason the controller writes into a condition, a short meaning
and, where a person can act, a next step. A reason the portal does not know SHALL be shown
raw, with no explanation and no error. Explanations SHALL NOT cite enhancement decisions.

#### Scenario: Known refusal reason

- **WHEN** the explanation for `ProvidesMismatch` is requested
- **THEN** a meaning and a next step are returned

#### Scenario: Unknown reason

- **WHEN** the explanation for a reason the controller does not write is requested
- **THEN** no explanation is returned

#### Scenario: Controller adds a reason

- **WHEN** the copied list of controller reasons gains a reason the table has no row for
- **THEN** the test suite fails, naming the reason

## MODIFIED Requirements

### Requirement: Registration verdicts come from the status fields

The portal SHALL read a TransformerRegistration's acceptance and activation from
`status.accepted` and `status.active`, an absent field meaning false, and SHALL report a verdict
of Accepted, Refused, Pending, RemovalBlocked or Unknown. A registration whose `Ready` reason is
`DependentsRemain` SHALL be RemovalBlocked and keep its accepted and active values, never
Refused. For a TransformerRegistration the verdict, not the applied state, SHALL decide how it
is shown: a refusal and a blocked removal are both Stalled on the applied axis.
Source: portal:D4:R4/R7.

#### Scenario: Refused claim

- **WHEN** a captured refused claim (`Stalled=True` and `Ready=False/ProvidesMismatch`, no
  `accepted` or `active` field) is read
- **THEN** it is not accepted, not active, and its verdict is Refused with reason
  `ProvidesMismatch`

#### Scenario: Accepted and active claim

- **WHEN** the captured claim with `accepted: true`, `active: true`, `Ready=True/Accepted` and
  `Active=True/ProviderReady` is read
- **THEN** it is accepted and active and its verdict is Accepted

#### Scenario: Removal blocked

- **WHEN** the captured claim deleted while a consumer demands its contract
  (`Stalled=True` and `Ready=False/DependentsRemain`, `accepted: true`, `active: true`) is read
- **THEN** its verdict is RemovalBlocked with the controller's message, and it stays accepted and
  active

#### Scenario: Applied state does not separate refusal from removal

- **WHEN** the captured refused claim and the captured removal-blocked claim are read on both
  axes
- **THEN** both applied states are Stalled, and the verdicts are Refused and RemovalBlocked

## REMOVED Requirements

### Requirement: Applied state is read from the operator's conditions

**Reason**: The product is now called the controller; a scenario name changed, so the requirement is restated under a new name.

**Migration**: None. The ADDED requirement below carries the same behavior.

### Requirement: Every operator condition reason has an explanation

**Reason**: The product is now called the controller; a scenario name changed, so the requirement is restated under a new name.

**Migration**: None. The ADDED requirement below carries the same behavior.
