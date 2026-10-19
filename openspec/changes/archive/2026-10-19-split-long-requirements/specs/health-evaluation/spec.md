## REMOVED Requirements

### Requirement: Health rolls up worst-of and says when it is partial
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Health rolls up worst-of, leaves out what the reader cannot read and says when it is partial", "Unread children below a workload make it, its component and the instance partial", "A result carries the oldest evaluation time and says when it is not live".

### Requirement: Applied state is read from the operator's conditions
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Applied state, first four matches", "Applied state, remaining matches", "Ready=True is Applied and never Healthy".

### Requirement: Registration verdicts come from the status fields
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "A TransformerRegistration verdict is read from status.accepted and status.active", "For a registration the verdict decides how it is shown".

## ADDED Requirements

### Requirement: Health rolls up worst-of, leaves out what the reader cannot read and says when it is partial
The portal SHALL roll object health up to each component and to the instance as the worst state by
the order Degraded, Missing, Progressing, Unknown, Healthy. An object the reader may not read, or
could not read, SHALL be excluded from the roll-up and SHALL mark the result partial. A Secret SHALL
never be evaluated, SHALL be excluded, and SHALL NOT mark the result partial. A result with nothing
counted SHALL be Unknown. Source: portal:D3:R4/R5.

#### Scenario: Forbidden object

- **WHEN** one of an instance's objects is forbidden to the reader and the rest are Healthy
- **THEN** the instance is Healthy and partial, and the forbidden object is counted as forbidden

#### Scenario: Secret in the inventory

- **WHEN** an instance's inventory names a Secret and its other objects are Healthy
- **THEN** the Secret is withheld and not evaluated, and the instance is Healthy and not partial

#### Scenario: Nothing readable

- **WHEN** every object of an instance is forbidden, or the inventory is empty
- **THEN** the instance is Unknown

### Requirement: Unread children below a workload make it, its component and the instance partial
The portal SHALL roll object health up to each component and to the instance as the worst state by
the order Degraded, Missing, Progressing, Unknown, Healthy. When the runtime children below the
inventory workloads could not be read, each readable workload that can own Pods SHALL be marked as
having unread children, and its component and the instance SHALL be partial. Source:
portal:D3:R4/R5.

#### Scenario: Pods not readable

- **WHEN** the podinfo objects captured one minute after the image break are evaluated and the
  reader may read the Deployment but not the Pods below it
- **THEN** the Deployment is Progressing and marked as having unread children, and its
  component and the instance are partial

### Requirement: A result carries the oldest evaluation time and says when it is not live
The portal SHALL roll object health up to each component and to the instance as the worst state by
the order Degraded, Missing, Progressing, Unknown, Healthy. Each result SHALL carry the oldest
evaluation time among the objects it counts, and SHALL say it is not live when any counted object
was refreshed by polling. Source: portal:D3:R4/R5.

#### Scenario: Polled object

- **WHEN** one counted object was refreshed by polling
- **THEN** the instance's result is not live and carries that object's evaluation time if it
  is the oldest

### Requirement: Applied state, first four matches
The portal SHALL read a ModuleInstance's, ModulePackage's, Platform's or TransformerRegistration's
applied state from its conditions and spec as exactly one of Applied, Reconciling, Failed, Stalled,
Suspended, ManagedExternally or Unknown, by the first match of: a CLI owner or `ManagedExternally`
reason is ManagedExternally; suspension is Suspended; `Stalled=True` is Stalled; `Ready=False` is
Failed, marked as retrying while `Reconciling=True`. Source: portal:D3:R1/R6/R7/R8.

#### Scenario: CLI-owned instance

- **WHEN** the captured CLI-owned instance (`spec.owner: cli`, `Ready=Unknown/ManagedExternally`)
  is read
- **THEN** its applied state is ManagedExternally, which is not a fault, and its health is
  still computed from its inventory objects

#### Scenario: Apply failed and retrying

- **WHEN** the captured cert-manager instance with `Reconciling=True` and
  `Ready=False/ApplyFailed` is read
- **THEN** its applied state is Failed with reason `ApplyFailed`, marked as retrying

#### Scenario: Package without a source

- **WHEN** the captured ModulePackage with `Ready=False/SourceNotReady` is read
- **THEN** its applied state is Failed with reason `SourceNotReady`

### Requirement: Applied state, remaining matches
The portal SHALL read a ModuleInstance's, ModulePackage's, Platform's or TransformerRegistration's
applied state from its conditions and spec when none of the matches listed in requirement "Applied
state, first four matches" applies, by the first match of: `Reconciling=True` or `Ready=Unknown` is
Reconciling; `Ready=True` whose `observedGeneration` is older than the object's generation is
Reconciling; `Ready=True` is Applied; anything else is Unknown. Source: portal:D3:R1/R6/R7/R8.

#### Scenario: Spec edited, operator not caught up

- **WHEN** the captured podinfo ModuleInstance is read with its generation one past the
  `observedGeneration` of its `Ready=True` condition
- **THEN** its applied state is Reconciling

#### Scenario: Unknown kind

- **WHEN** an object of a kind other than the four operator kinds is read
- **THEN** its applied state is Unknown

### Requirement: Ready=True is Applied and never Healthy
`Ready=True` SHALL be Applied and never Healthy, and the applied state SHALL NOT be derived from
health nor health from it. Failure counters SHALL never change either axis.
`ContractsFulfilled=False` and `Drifted=True` SHALL be informational notes that never change the
state. Source: portal:D3:R1/R6/R7/R8.

#### Scenario: Ready instance with a broken rollout

- **WHEN** the podinfo ModuleInstance captured one minute after the image break
  (`Ready=True/ReconciliationSucceeded`, drift counter 2) is read
- **THEN** its applied state is Applied, while its health from the same capture is Degraded

#### Scenario: Fresh Platform with unfulfilled contracts

- **WHEN** the captured fresh Platform (`Ready=True/Generated`,
  `ContractsFulfilled=False/UnfulfilledContracts`) is read
- **THEN** its applied state is Applied and it carries one informational note naming
  `UnfulfilledContracts`

### Requirement: A TransformerRegistration verdict is read from status.accepted and status.active
The portal SHALL read a TransformerRegistration's acceptance and activation from `status.accepted`
and `status.active`, an absent field meaning false, and SHALL report a verdict of Accepted, Refused,
Pending, RemovalBlocked or Unknown. A registration whose `Ready` reason is `DependentsRemain` SHALL
be RemovalBlocked and keep its accepted and active values, never Refused. Source: portal:D4:R4/R7.

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
- **THEN** its verdict is RemovalBlocked with the operator's message, and it stays accepted and
  active

### Requirement: For a registration the verdict decides how it is shown
For a TransformerRegistration the verdict, not the applied state, SHALL decide how it is shown: a
refusal and a blocked removal are both Stalled on the applied axis. Source: portal:D4:R4/R7.

#### Scenario: Applied state does not separate refusal from removal

- **WHEN** the captured refused claim and the captured removal-blocked claim are read on both
  axes
- **THEN** both applied states are Stalled, and the verdicts are Refused and RemovalBlocked
