## REMOVED Requirements

### Requirement: Views mark unreadable items instead of failing
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Every further object in a view is authorized and marked, never failing the view", "A list holds only items within the caller's list grant".

### Requirement: Instance views carry the runtime children below each inventory object
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "An inventory object carries the runtime children whose controller chain reaches it", "A child that reaches no inventory object is attached to none, and unreadable children are reported".

### Requirement: A Pod's reach from an inventory is answered for the caller
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The read model answers which inventory object a Pod is a runtime child of", "The answer about a Pod counts only what the caller may get, read and list", "A Pod that is not reached gets one refusal, and a check that cannot be decided is unavailable".

### Requirement: The read model reports which OPM object changed
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The read model reports the OPM object whose view may have changed", "A deletion of an OPM object is reported as a deletion".

### Requirement: One inventory object is read on demand, stripped
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "One object is read on demand for a caller whose grant covers get, and is returned stripped", "A core Secret is refused before any review or read".

### Requirement: Registrations are joined to the owners that hold them
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The read model answers which owners hold a registration and which registrations an owner holds", "An owner that holds a registration is included only when the caller may list its kind and the answer is marked incomplete otherwise", "A registration's standing is attached only when the caller may list registrations".

## ADDED Requirements

### Requirement: Every further object in a view is authorized and marked, never failing the view
Inside a view the caller may read, every further object (an inventory entry, a runtime child, a
registration) SHALL be authorized for that caller on its own before it is shown. An item the caller
may not read SHALL be marked forbidden and shown without its content; an item the portal could not
read SHALL be marked not readable; neither SHALL fail the view. Source: portal:D7:R2/R3.

#### Scenario: Inventory kind the caller may not read

- **WHEN** a caller who may read instance `cert-manager/cert-manager` but not ClusterRoles asks
  for its detail
- **THEN** every ClusterRole entry is marked forbidden, carries no object content, and the
  instance's health is partial
- **AND** every other entry is shown with its health

#### Scenario: Registrations the caller may not list

- **WHEN** a caller may read the Platform but not TransformerRegistrations
- **THEN** the platform view shows its catalogs and marks the registrations forbidden, rather
  than showing none

### Requirement: A list holds only items within the caller's list grant
A list SHALL contain only items within the namespace scope of the caller's list grant, with no count
or name of others. Source: portal:D7:R2/R3.

#### Scenario: Namespace-scoped list

- **WHEN** a caller holding a grant to list ModuleInstances in namespace `default` lists them
- **THEN** the list holds `backup-consumer`, `backup-provider` and `podinfo`, and nothing from
  `cert-manager` or `web`

### Requirement: An inventory object carries the runtime children whose controller chain reaches it
When the runtime children of an instance's namespaces could be read, each inventory object in the
instance or package view SHALL carry the ReplicaSets, Pods and Jobs whose chain of controller owner
references reaches it, each with its direct controller owner, its health, and for a ReplicaSet its
desired replica count. Source: portal:D4:R1.

#### Scenario: podinfo's Deployment

- **WHEN** the caller reads instance `default/podinfo` from the F1 capture
- **THEN** its Deployment carries one ReplicaSet with 2 desired replicas, owned by the
  Deployment, and two Pods owned by that ReplicaSet, each Healthy

### Requirement: A child that reaches no inventory object is attached to none, and unreadable children are reported
A child whose chain reaches no inventory object SHALL NOT be attached to any. When the children
could not be read, no object SHALL carry children and the workloads SHALL keep saying their children
are unread. Source: portal:D4:R1.

#### Scenario: Children unreadable

- **WHEN** the caller may not list Pods in `default` and reads instance `default/podinfo`
- **THEN** no inventory object carries children and the Deployment says its children are unread

### Requirement: The read model answers which inventory object a Pod is a runtime child of
The read model SHALL answer, for a caller and one Pod, the inventory object of a ModuleInstance or
ModulePackage that the Pod is a runtime child of, through the same controller chain the views'
runtime children follow (a ReplicaSet, a Job, or the inventory workload itself). The caller's grant
SHALL cover `get` on the Pod's `log` subresource before anything is looked up. Source:
portal:D10:R1, portal:D7:R1.

#### Scenario: A Pod below an inventory Deployment

- **WHEN** a caller who may read the `podinfo` instance's Deployment asks about one of its Pods
- **THEN** the answer names that Deployment and the instance

#### Scenario: A grant that does not cover the Pod's log

- **WHEN** a caller's grant does not cover `get pods/log` on the Pod
- **THEN** the read model refuses before it reads anything

### Requirement: The answer about a Pod counts only what the caller may get, read and list
The read model's answer, for a caller and one Pod, SHALL count only owners the caller may get,
inventory objects the caller may read and runtime children the caller may list; an owner SHALL be
authorized before its inventory is read. Source: portal:D10:R1, portal:D7:R1.

#### Scenario: A Pod reached only through an object the caller may not read

- **WHEN** a caller who may not get the Deployment asks about one of its Pods
- **THEN** the read model refuses with the not-reachable refusal

#### Scenario: A Pod whose owner the caller may not read

- **WHEN** a caller who may read the Deployment but may not get the `podinfo` ModuleInstance asks
  about one of the Deployment's Pods
- **THEN** the read model refuses with the not-reachable refusal, as it does for a Pod no
  inventory reaches

### Requirement: A Pod that is not reached gets one refusal, and a check that cannot be decided is unavailable
A Pod that does not exist, that no inventory reaches, or that is reached only through objects the
caller may not read SHALL get one and the same refusal; a read the reading identity cannot make, and
a caller check the authorizer cannot decide, the owner's included, SHALL be reported as unavailable
rather than refused. Source: portal:D10:R1, portal:D7:R1.

#### Scenario: A Pod no inventory reaches

- **WHEN** a caller asks about a Pod labeled with an instance's name whose controller is no
  inventory object, or about a Pod that does not exist
- **THEN** the read model refuses with the not-reachable refusal, the same for both

#### Scenario: The owner check cannot be decided

- **WHEN** the authorizer cannot decide whether the caller may get the `podinfo` ModuleInstance
  and no other owner reaches the Pod
- **THEN** the read model reports the answer as unavailable, not as the not-reachable refusal

### Requirement: The read model reports the OPM object whose view may have changed
The read model SHALL report, to a registered listener, the kind, namespace and name of every OPM
object (ModuleInstance, ModulePackage, Platform, TransformerRegistration) whose view may have
changed: when the object itself is added, updated or deleted, when an inventory object it owns
changes, and when a runtime child labelled with its name changes. A report SHALL carry no object
content, so a listener learns nothing it could serve without a grant.

#### Scenario: An inventory object changes

- **WHEN** Deployment `default/podinfo-podinfo`, which instance `default/podinfo`'s inventory names,
  is updated in a held informer
- **THEN** the listener is told that ModuleInstance `default/podinfo` changed

### Requirement: A deletion of an OPM object is reported as a deletion
The read model SHALL report, to a registered listener, the kind, namespace and name of every OPM
object (ModuleInstance, ModulePackage, Platform, TransformerRegistration) whose view may have
changed. A deletion of the OPM object itself SHALL be reported as a deletion.

#### Scenario: The instance is deleted

- **WHEN** ModuleInstance `default/podinfo` is deleted
- **THEN** the listener is told it was deleted

### Requirement: One object is read on demand for a caller whose grant covers get, and is returned stripped
The read model SHALL read one object by kind, namespace and name on demand for a caller whose grant
covers `get` on it, after the reader identity's own `get` review, and SHALL return it without
`metadata.managedFields`, the `kubectl.kubernetes.io/last-applied-configuration` annotation, or, on
a ModuleInstance or ModulePackage, `spec.values`. The read model SHALL keep the fields the held
copies drop for memory, such as a Deployment's pod template. Source: portal:D8:R1/R2/R3.

#### Scenario: A Deployment with its template

- **WHEN** a caller allowed `get` on Deployments in `default` reads `apps` `Deployment`
  `default/podinfo-podinfo`
- **THEN** the object carries `spec.template` and no managed fields or last-applied annotation

### Requirement: A core Secret is refused before any review or read
The read model SHALL refuse a core Secret before any review or read. Source: portal:D8:R1/R2/R3.

#### Scenario: A Secret

- **WHEN** a caller reads a core `Secret`
- **THEN** the read is refused without a review or a read

### Requirement: The read model answers which owners hold a registration and which registrations an owner holds
The read model SHALL answer, from held state and without a read of its own, which ModuleInstances
and ModulePackages hold a TransformerRegistration of a given name in their `status.inventory`,
whatever the owner's kind, and which registrations a given owner's inventory holds. Source:
portal:D15:R1/R2/R4, portal:D7:R2.

#### Scenario: The F1 join

- **WHEN** the caller reads the holders of `default.backup-provider` from the F1 capture
- **THEN** the answer is ModuleInstance `default/backup-provider`, complete

#### Scenario: A package holder

- **WHEN** a held ModulePackage's inventory holds TransformerRegistration `pkg.provider`
- **THEN** the package is a holder of `pkg.provider`, and the registration's standing is read
  from its own status, unchanged

### Requirement: An owner that holds a registration is included only when the caller may list its kind and the answer is marked incomplete otherwise
For a caller, in the answer which ModuleInstances and ModulePackages hold a TransformerRegistration
of a given name, the read model SHALL include an owner only when the caller may list that owner's kind
in its namespace, and SHALL mark the answer incomplete whenever the caller lacks a cluster-wide list
of ModuleInstances or of ModulePackages, or the portal does not hold every namespace, whether or not
a holder was found. Source: portal:D15:R1/R2/R4, portal:D7:R2.

#### Scenario: A namespace the caller may not list

- **WHEN** the caller may list ModuleInstances only in namespace `web`
- **THEN** the holders of `default.backup-provider` are empty and marked incomplete

#### Scenario: Incomplete even with a holder

- **WHEN** the caller may list ModuleInstances cluster-wide but ModulePackages only in namespace
  `default`
- **THEN** the holders of `default.backup-provider` name `default/backup-provider` and are marked
  incomplete

#### Scenario: A portal limited to namespaces

- **WHEN** the portal was started with `--namespaces default,pkg` and the caller may list
  everything
- **THEN** the holders of `default.backup-provider` name `default/backup-provider` and are marked
  incomplete

### Requirement: A registration's standing is attached only when the caller may list registrations
A registration's standing SHALL be attached only when the caller may list TransformerRegistrations,
and SHALL be the registration's own `status.accepted`, `status.active` and verdict reason. Source:
portal:D15:R1/R2/R4, portal:D7:R2.

#### Scenario: Standing is attached for a caller who may list registrations

- **WHEN** the caller may list TransformerRegistrations
- **THEN** the registration's standing SHALL be its own `status.accepted`, `status.active` and verdict reason

#### Scenario: Standing is not attached for a caller who may not list registrations

- **WHEN** the caller may not list TransformerRegistrations
- **THEN** no standing SHALL be attached to the registration
