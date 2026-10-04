# read-model Specification

## Purpose
What the portal holds in memory about one cluster, how it keeps that state fresh, what it strips
before holding it, and how each view it builds is authorized and degraded per item.

## Requirements

### Requirement: Every read is covered by the caller's grant

The read model SHALL serve a view only to a caller whose grant, issued by the authorizer for that
caller, covers the view's own read (verb, resource, namespace, name) at the time of the call. A
zero grant, a grant issued to another identity, an expired grant or a grant for another read
SHALL be refused before anything is looked up, with the same refusal whether or not the object
exists. Source: 0030:D7:R1.

#### Scenario: Grant for another read

- **WHEN** a caller asks for instance `default/podinfo` holding a grant for `get` on
  ModuleInstance `default/backup-consumer`
- **THEN** the read is refused as not covered, and no view is returned

#### Scenario: Grant issued to someone else

- **WHEN** a caller presents a grant issued to another identity for the same read
- **THEN** the read is refused as not covered

#### Scenario: Missing and existing objects read alike

- **WHEN** a caller without a covering grant asks for an instance that exists and one that does
  not
- **THEN** both refusals are identical

### Requirement: Views mark unreadable items instead of failing

Inside a view the caller may read, every further object (an inventory entry, a runtime child, a
registration) SHALL be authorized for that caller on its own before it is shown. An item the
caller may not read SHALL be marked forbidden and shown without its content; an item the portal
could not read SHALL be marked not readable; neither SHALL fail the view. A list SHALL contain
only items within the namespace scope of the caller's list grant, with no count or name of
others. Source: 0030:D7:R2/R3.

#### Scenario: Inventory kind the caller may not read

- **WHEN** a caller who may read instance `cert-manager/cert-manager` but not ClusterRoles asks
  for its detail
- **THEN** every ClusterRole entry is marked forbidden, carries no object content, and the
  instance's health is partial
- **AND** every other entry is shown with its health

#### Scenario: Namespace-scoped list

- **WHEN** a caller holding a grant to list ModuleInstances in namespace `default` lists them
- **THEN** the list holds `backup-consumer`, `backup-provider` and `podinfo`, and nothing from
  `cert-manager` or `web`

#### Scenario: Registrations the caller may not list

- **WHEN** a caller may read the Platform but not TransformerRegistrations
- **THEN** the platform view shows its catalogs and marks the registrations forbidden, rather
  than showing none

### Requirement: Held objects carry no values or apply annotation

Every object the read model holds or returns SHALL have no `metadata.managedFields` and no
`kubectl.kubernetes.io/last-applied-configuration` annotation, and no ModuleInstance or
ModulePackage it holds or returns SHALL carry `spec.values`. No view SHALL embed a raw custom
resource status. Source: 0030:D8:R2/R3, 0030:D2:R4.

#### Scenario: Client-side applied instance

- **WHEN** the cluster serves a ModuleInstance with `spec.values`, managed fields and the
  last-applied annotation holding those values
- **THEN** neither the held object nor any view built from it contains the values, the managed
  fields or the annotation

### Requirement: Secrets are never read

The read model SHALL NOT get, list or watch Secrets, at any tier. An inventory entry naming a
Secret SHALL be shown as withheld, SHALL NOT make the instance's health partial, and SHALL NOT
start a watch. Source: 0030:D8:R1, 0030:D3:R4.

#### Scenario: Inventory names a Secret

- **WHEN** an instance's inventory names a Secret and the caller asks for its detail
- **THEN** the entry is withheld, no request for any Secret reaches the cluster, and the health
  is not partial on its account

### Requirement: Instance views are answered from held state

Once the kinds an instance's inventory names are held, the instance detail and the instance list
SHALL be answered without one Kubernetes read per inventory object, and the reading client SHALL
run with a raised request rate. Source: 0030:D3:R9.

#### Scenario: Second read of cert-manager

- **WHEN** the cert-manager instance (42 inventory entries) is read twice
- **THEN** the second read makes no list, get or watch request to the cluster

### Requirement: Inventory kinds are watched while in use

The read model SHALL start watching an inventory kind on the first read that needs it, limited to
objects carrying the module-instance label, and SHALL stop watching it once no read has used it
for the idle period. Source: 0030:D3:R9.

#### Scenario: Kind idles out

- **WHEN** no read has needed Deployments for longer than the idle period
- **THEN** the Deployment watch is stopped, and the next read that needs Deployments starts it
  again and is answered from fresh state

### Requirement: Objects that cannot be watched are polled and say so

When the reader may get an inventory kind but may not list and watch it in the scope it needs,
the read model SHALL refresh those objects by reading them at most every 30 seconds, and each
such entry SHALL report that it is not live and when it was last evaluated. Source: 0030:D3:R5.

#### Scenario: Reader may only get Services

- **WHEN** the reader may get but not list or watch Services and the caller reads instance
  `default/podinfo`
- **THEN** its Service entry is shown with its health, marked not live, with the time it was
  read, and the instance's health says it is not live

### Requirement: Runtime children are watched only while someone looks

ReplicaSets, Pods and Jobs below inventory workloads SHALL be watched in a namespace only while
at least one caller holds interest in that namespace; without interest they SHALL be read on
demand when a view needs them. If they cannot be read, the workloads that can own Pods SHALL be
marked as having unread children and the health SHALL be partial. Source: 0030:D3:R2/R4.

#### Scenario: Interest released

- **WHEN** the last caller holding interest in namespace `default` releases it
- **THEN** the namespace's runtime-children watches stop

#### Scenario: Pods visible to the Pod rule

- **WHEN** the caller reads instance `default/podinfo` and its Pods are readable
- **THEN** the Pods reach the health evaluation and the Deployment's health reflects them

### Requirement: Events are read on demand about one object

Events SHALL be read only when asked for, from `events.k8s.io/v1`, selected by the object they
regard. Events about the Platform and about a TransformerRegistration SHALL be read from
namespace `default`. Repeats about the same object with the same type, reason and note SHALL be
returned once, with a count that sums separate events, event series and deprecated counts, and
the latest occurrence time. Source: 0030:D9:R3/R4.

#### Scenario: Platform events

- **WHEN** the caller reads the events about Platform `cluster`
- **THEN** they come from namespace `default` and include its `Generated` events

#### Scenario: Kubelet events with no event time

- **WHEN** events about a Pod were recorded by the kubelet with `eventTime: null` and a
  deprecated count
- **THEN** each line's time is the deprecated last timestamp and its count is the deprecated
  count

### Requirement: Views show the operator's verdicts on their own axis

The instance, package and platform views SHALL carry the applied state read from the operator's
conditions beside, never merged with, the workload health; a registration SHALL carry accepted,
active and its verdict as separate values. Source: 0030:D3:R1/R6, 0030:D4:R4.

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

### Requirement: A kind the reader cannot list is reported as unavailable

When the reader cannot list and watch one of the four OPM kinds in a configured scope, views of
that kind SHALL report it as unavailable, never as empty. Source: Principle IV.

#### Scenario: Reader cannot watch ModulePackages

- **WHEN** the reader may not list ModulePackages and a caller lists packages
- **THEN** the read reports the kind unavailable instead of returning an empty list

### Requirement: Instance views carry the runtime children below each inventory object

When the runtime children of an instance's namespaces could be read, each inventory object in the
instance or package view SHALL carry the ReplicaSets, Pods and Jobs whose chain of controller
owner references reaches it, each with its direct controller owner, its health, and for a
ReplicaSet its desired replica count. A child whose chain reaches no inventory object SHALL NOT
be attached to any. When the children could not be read, no object SHALL carry children and the
workloads SHALL keep saying their children are unread. Source: 0030:D4:R1.

#### Scenario: podinfo's Deployment

- **WHEN** the caller reads instance `default/podinfo` from the F1 capture
- **THEN** its Deployment carries one ReplicaSet with 2 desired replicas, owned by the
  Deployment, and two Pods owned by that ReplicaSet, each Healthy

#### Scenario: Children unreadable

- **WHEN** the caller may not list Pods in `default` and reads instance `default/podinfo`
- **THEN** no inventory object carries children and the Deployment says its children are unread

### Requirement: Package views carry their dependencies

A package item SHALL carry the ModulePackages its `spec.dependsOn` names, as written, without
reading them.

#### Scenario: Package without dependencies

- **WHEN** the caller reads package `pkg/podinfo` from the F1 capture
- **THEN** it carries no dependencies and its source `OCIRepository` `podinfo-release`

### Requirement: A Pod's reach from an inventory is answered for the caller

The read model SHALL answer, for a caller and one Pod, the inventory object of a ModuleInstance or
ModulePackage that the Pod is a runtime child of, through the same controller chain the views'
runtime children follow (a ReplicaSet, a Job, or the inventory workload itself). The answer SHALL
count only owners the caller may get, inventory objects the caller may read and runtime children
the caller may list; an owner SHALL be authorized before its inventory is read. The
caller's grant SHALL cover `get` on the Pod's `log` subresource before anything is looked up. A
Pod that does not exist, that no inventory reaches, or that is reached only through objects the
caller may not read SHALL get one and the same refusal; a read the reading identity cannot make,
and a caller check the authorizer cannot decide, the owner's included, SHALL be reported as
unavailable rather than refused. Source: 0030:D10:R1, 0030:D7:R1.

#### Scenario: A Pod below an inventory Deployment

- **WHEN** a caller who may read the `podinfo` instance's Deployment asks about one of its Pods
- **THEN** the answer names that Deployment and the instance

#### Scenario: A Pod no inventory reaches

- **WHEN** a caller asks about a Pod labeled with an instance's name whose controller is no
  inventory object, or about a Pod that does not exist
- **THEN** the read model refuses with the not-reachable refusal, the same for both

#### Scenario: A Pod reached only through an object the caller may not read

- **WHEN** a caller who may not get the Deployment asks about one of its Pods
- **THEN** the read model refuses with the not-reachable refusal

#### Scenario: A Pod whose owner the caller may not read

- **WHEN** a caller who may read the Deployment but may not get the `podinfo` ModuleInstance asks
  about one of the Deployment's Pods
- **THEN** the read model refuses with the not-reachable refusal, as it does for a Pod no
  inventory reaches

#### Scenario: The owner check cannot be decided

- **WHEN** the authorizer cannot decide whether the caller may get the `podinfo` ModuleInstance
  and no other owner reaches the Pod
- **THEN** the read model reports the answer as unavailable, not as the not-reachable refusal

#### Scenario: A grant that does not cover the Pod's log

- **WHEN** a caller's grant does not cover `get pods/log` on the Pod
- **THEN** the read model refuses before it reads anything

### Requirement: The read model reports which OPM object changed

The read model SHALL report, to a registered listener, the kind, namespace and name of every OPM
object (ModuleInstance, ModulePackage, Platform, TransformerRegistration) whose view may have
changed: when the object itself is added, updated or deleted, when an inventory object it owns
changes, and when a runtime child labelled with its name changes. A report SHALL carry no object
content, so a listener learns nothing it could serve without a grant. A deletion of the OPM
object itself SHALL be reported as a deletion.

#### Scenario: An inventory object changes

- **WHEN** Deployment `default/podinfo-podinfo`, which instance `default/podinfo`'s inventory names,
  is updated in a held informer
- **THEN** the listener is told that ModuleInstance `default/podinfo` changed

#### Scenario: The instance is deleted

- **WHEN** ModuleInstance `default/podinfo` is deleted
- **THEN** the listener is told it was deleted

### Requirement: Kinds resolve to resources for authorization

The read model SHALL resolve a group and kind to the resource it is served as, and whether it is
namespaced, through cached discovery, so a caller can authorize a read of that kind before it
looks anything up. An unknown kind SHALL be reported as unresolved, never guessed.

#### Scenario: A known kind

- **WHEN** the caller resolves group `apps`, kind `Deployment`
- **THEN** the result is resource `apps/v1 deployments`, namespaced

#### Scenario: An unknown kind

- **WHEN** the caller resolves group `example.com`, kind `Widget`, which discovery does not know
- **THEN** the result is an error
