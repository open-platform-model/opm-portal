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
