# health-evaluation Specification

## Purpose
How the portal derives workload health from live objects, how it reads applied state and
registration verdicts from the operator's status, and how it explains an operator reason, kept
as two axes that never merge.

## Requirements


### Requirement: Object health follows the Kubernetes status rules

The portal SHALL compute each readable object's health from the standard Kubernetes status
rules (kstatus) as one of Healthy, Progressing, Degraded, Missing or Unknown: current is
Healthy, in progress and terminating are Progressing, failed is Degraded, an object the reader
found absent is Missing, and anything the rules cannot decide is Unknown, never a guess.
Source: 0030:D3.

#### Scenario: Healthy instance objects

- **WHEN** the captured healthy podinfo and cert-manager objects are evaluated (Deployments,
  ReplicaSets, Pods, Services, CRDs, webhooks, RBAC, Namespace, ServiceAccounts)
- **THEN** every one of them is Healthy

#### Scenario: Progress deadline exceeded

- **WHEN** the podinfo Deployment captured after `ProgressDeadlineExceeded` is evaluated
- **THEN** it is Degraded with the message "Progress deadline exceeded"

#### Scenario: Object read and not found

- **WHEN** an inventory entry was read without error and the object does not exist
- **THEN** the entry is Missing

#### Scenario: Undecidable object

- **WHEN** the status rules return an error for an object
- **THEN** the object is Unknown and the error is its message

### Requirement: A Pod waiting on a broken container degrades its workload

A Pod with a container or init container waiting with reason `ErrImagePull`,
`ImagePullBackOff`, `CrashLoopBackOff`, `CreateContainerConfigError` or `InvalidImageName` SHALL
be Degraded, and the inventory object that owns it through its chain of controller owner
references SHALL be Degraded with the Pod's reason, even while that object's own status reports
it available. An owner reference not marked controller SHALL NOT be followed.
Source: 0030:D3:R2/R3.

#### Scenario: Image break one minute in

- **WHEN** the podinfo objects captured one minute after the image break are evaluated, where
  the new Pod waits in `ImagePullBackOff`, the Deployment is `Available=True` and the status
  rules alone say the Deployment is in progress
- **THEN** the Deployment is Degraded with reason `ImagePullBackOff`, and so are its component
  and the instance

#### Scenario: Pod owned by nothing in the inventory

- **WHEN** a Pod in `CrashLoopBackOff` has no owner chain that reaches an inventory object
- **THEN** no inventory object's health changes

#### Scenario: Plain owner reference

- **WHEN** a Pod in `CrashLoopBackOff` names an inventory Deployment in an owner reference that
  is not marked controller
- **THEN** the Deployment's health does not change

#### Scenario: Children are not counted twice

- **WHEN** the healthy podinfo instance is evaluated with its ReplicaSet and Pods as children
- **THEN** the instance counts only its two inventory objects

### Requirement: Health rolls up worst-of and says when it is partial

The portal SHALL roll object health up to each component and to the instance as the worst
state by the order Degraded, Missing, Progressing, Unknown, Healthy. An object the reader may
not read, or could not read, SHALL be excluded from the roll-up and SHALL mark the result
partial. When the runtime children below the inventory workloads could not be read, each
readable workload that can own Pods SHALL be marked as having unread children, and its
component and the instance SHALL be partial. A Secret SHALL never be evaluated, SHALL be excluded, and SHALL NOT mark the result
partial. A result with nothing counted SHALL be Unknown. Each result SHALL carry the oldest
evaluation time among the objects it counts, and SHALL say it is not live when any counted
object was refreshed by polling. Source: 0030:D3:R4/R5.

#### Scenario: Forbidden object

- **WHEN** one of an instance's objects is forbidden to the reader and the rest are Healthy
- **THEN** the instance is Healthy and partial, and the forbidden object is counted as forbidden

#### Scenario: Pods not readable

- **WHEN** the podinfo objects captured one minute after the image break are evaluated and the
  reader may read the Deployment but not the Pods below it
- **THEN** the Deployment is Progressing and marked as having unread children, and its
  component and the instance are partial

#### Scenario: Secret in the inventory

- **WHEN** an instance's inventory names a Secret and its other objects are Healthy
- **THEN** the Secret is withheld and not evaluated, and the instance is Healthy and not partial

#### Scenario: Nothing readable

- **WHEN** every object of an instance is forbidden, or the inventory is empty
- **THEN** the instance is Unknown

#### Scenario: Polled object

- **WHEN** one counted object was refreshed by polling
- **THEN** the instance's result is not live and carries that object's evaluation time if it
  is the oldest

### Requirement: Applied state is read from the operator's conditions

The portal SHALL read a ModuleInstance's, ModulePackage's, Platform's or
TransformerRegistration's applied state from its conditions and spec as exactly one of Applied,
Reconciling, Failed, Stalled, Suspended, ManagedExternally or Unknown, by the first match of:
a CLI owner or `ManagedExternally` reason is ManagedExternally; suspension is Suspended;
`Stalled=True` is Stalled; `Ready=False` is Failed, marked as retrying while
`Reconciling=True`; `Reconciling=True` or `Ready=Unknown` is Reconciling; `Ready=True` is
Applied; anything else is Unknown. `Ready=True` SHALL be Applied and never Healthy, and the
applied state SHALL NOT be derived from health nor health from it. Failure counters SHALL never
change either axis. `ContractsFulfilled=False` and `Drifted=True` SHALL be informational notes
that never change the state. Source: 0030:D3:R1/R6/R7/R8.

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

#### Scenario: Unknown kind

- **WHEN** an object of a kind other than the four operator kinds is read
- **THEN** its applied state is Unknown

### Requirement: Registration verdicts come from the status fields

The portal SHALL read a TransformerRegistration's acceptance and activation from
`status.accepted` and `status.active`, an absent field meaning false, and SHALL report a verdict
of Accepted, Refused, Pending, RemovalBlocked or Unknown. A registration whose `Ready` reason is
`DependentsRemain` SHALL be RemovalBlocked and keep its accepted and active values, never
Refused. Source: 0030:D4:R4/R7.

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

### Requirement: Every operator condition reason has an explanation

The portal SHALL hold, for every reason the operator writes into a condition, a short meaning
and, where a person can act, a next step. A reason the portal does not know SHALL be shown
raw, with no explanation and no error. Explanations SHALL NOT cite enhancement decisions.

#### Scenario: Known refusal reason

- **WHEN** the explanation for `ProvidesMismatch` is requested
- **THEN** a meaning and a next step are returned

#### Scenario: Unknown reason

- **WHEN** the explanation for a reason the operator does not write is requested
- **THEN** no explanation is returned

#### Scenario: Operator adds a reason

- **WHEN** the copied list of operator reasons gains a reason the table has no row for
- **THEN** the test suite fails, naming the reason
