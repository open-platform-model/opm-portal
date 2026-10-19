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
exists. Source: portal:D7:R1.

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

### Requirement: Held objects carry no values or apply annotation

Every object the read model holds or returns SHALL have no `metadata.managedFields` and no
`kubectl.kubernetes.io/last-applied-configuration` annotation, and no ModuleInstance or
ModulePackage it holds or returns SHALL carry `spec.values`. No view SHALL embed a raw custom
resource status. Source: portal:D8:R2/R3, portal:D2:R4.

#### Scenario: Client-side applied instance

- **WHEN** the cluster serves a ModuleInstance with `spec.values`, managed fields and the
  last-applied annotation holding those values
- **THEN** neither the held object nor any view built from it contains the values, the managed
  fields or the annotation

### Requirement: Secrets are never read

The read model SHALL NOT get, list or watch Secrets, at any tier. An inventory entry naming a
Secret SHALL be shown as withheld, SHALL NOT make the instance's health partial, and SHALL NOT
start a watch. Source: portal:D8:R1, portal:D3:R4.

#### Scenario: Inventory names a Secret

- **WHEN** an instance's inventory names a Secret and the caller asks for its detail
- **THEN** the entry is withheld, no request for any Secret reaches the cluster, and the health
  is not partial on its account

### Requirement: Instance views are answered from held state

Once the kinds an instance's inventory names are held, the instance detail and the instance list
SHALL be answered without one Kubernetes read per inventory object, and the reading client SHALL
run with a raised request rate. Source: portal:D3:R9.

#### Scenario: Second read of cert-manager

- **WHEN** the cert-manager instance (42 inventory entries) is read twice
- **THEN** the second read makes no list, get or watch request to the cluster

### Requirement: Inventory kinds are watched while in use

The read model SHALL start watching an inventory kind on the first read that needs it, limited to
objects carrying the module-instance label, and SHALL stop watching it once no read has used it
for the idle period. Source: portal:D3:R9.

#### Scenario: Kind idles out

- **WHEN** no read has needed Deployments for longer than the idle period
- **THEN** the Deployment watch is stopped, and the next read that needs Deployments starts it
  again and is answered from fresh state

### Requirement: Objects that cannot be watched are polled and say so

When the reader may get an inventory kind but may not list and watch it in the scope it needs,
the read model SHALL refresh those objects by reading them at most every 30 seconds, and each
such entry SHALL report that it is not live and when it was last evaluated. Source: portal:D3:R5.

#### Scenario: Reader may only get Services

- **WHEN** the reader may get but not list or watch Services and the caller reads instance
  `default/podinfo`
- **THEN** its Service entry is shown with its health, marked not live, with the time it was
  read, and the instance's health says it is not live

### Requirement: Runtime children are watched only while someone looks

ReplicaSets, Pods and Jobs below inventory workloads SHALL be watched in a namespace only while
at least one caller holds interest in that namespace; without interest they SHALL be read on
demand when a view needs them. If they cannot be read, the workloads that can own Pods SHALL be
marked as having unread children and the health SHALL be partial. Source: portal:D3:R2/R4.

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
the latest occurrence time. Source: portal:D9:R3/R4.

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
active and its verdict as separate values. Source: portal:D3:R1/R6, portal:D4:R4.

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

### Requirement: Package views carry their dependencies

A package item SHALL carry the ModulePackages its `spec.dependsOn` names, as written, without
reading them.

#### Scenario: Package without dependencies

- **WHEN** the caller reads package `pkg/podinfo` from the F1 capture
- **THEN** it carries no dependencies and its source `OCIRepository` `podinfo-release`

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

### Requirement: Pod children name their containers

A runtime child that is a Pod SHALL carry the names of its init containers, then its
containers, in spec order.

#### Scenario: podinfo's Pods

- **WHEN** the caller reads instance `default/podinfo` from the F1 capture
- **THEN** each of its two Pods carries container `podinfo`

### Requirement: Package views carry their interval and source artifact

A package view SHALL carry `spec.interval` as written and the `artifactRevision` and
`artifactDigest` of `status.source` when the controller recorded them, and nothing in their place
when it did not. Source: portal:D2:R4.

#### Scenario: No Flux on the fixture cluster

- **WHEN** the caller reads package `pkg/podinfo` from the F1 capture
- **THEN** its interval is `1m` and it carries no source artifact

### Requirement: The server version is read once, like discovery

When it starts, the read model SHALL read the API server's version from `/version` once, as its
reader, the way it reads the discovery documents: without an access review. It SHALL hold the
`gitVersion`. A failed read SHALL leave no version held and SHALL not stop the read model from
starting. The read model SHALL read no non-resource path other than `/version` and the discovery
documents. Source: portal:D18:R3.

#### Scenario: Version read

- **WHEN** the read model starts against the F1 fixture cluster
- **THEN** it holds the server's `gitVersion`, `v1.36.1`, after one request to `/version`

#### Scenario: Version read fails

- **WHEN** the request to `/version` fails at start
- **THEN** the read model starts and holds no version

### Requirement: Events are read merged over an inventory

The read model SHALL read the events about a set of reached objects with one list from
`events.k8s.io/v1` per event namespace and regarded kind. Each list SHALL be selected by the
regarded kind, filtered to the set by group, kind, namespace and name, folded as one object's
feed is, and merged newest first. Source: portal:D9:R3/R6.

#### Scenario: podinfo's objects

- **WHEN** the events about podinfo's Deployment, ReplicaSet and Pods are read merged on the F1
  capture
- **THEN** one list runs per regarded kind in namespace `default`, and the result holds each
  object's folded events and no event about another object

### Requirement: Merged event reads are scoped by namespace and bounded

Events about a cluster-scoped object SHALL be read from namespace `default`. A namespace the
caller's grant does not cover SHALL not be listed: its objects are skipped and counted. The lists
SHALL run under the read timeout, a bounded number at a time. Source: portal:D9:R4/R6.

#### Scenario: A namespace without a grant

- **WHEN** the set holds objects in a namespace the caller's grants do not cover
- **THEN** no list runs there, and those objects are reported as skipped

### Requirement: Package views carry their prune setting

A package view SHALL carry `spec.prune` as written when it is set, and nothing when it is not.
Source: portal:D2:R2.

#### Scenario: The F1 package

- **WHEN** the caller reads package `pkg/podinfo` from the F1 capture
- **THEN** its prune setting is `true`

### Requirement: A package's source is read on demand for the caller

The read model SHALL read the source a ModulePackage's `spec.sourceRef` names only when its kind is
OCIRepository, GitRepository or Bucket, always in `source.toolkit.fluxcd.io` whatever the
sourceRef's `apiVersion` says, in the sourceRef's namespace or else the package's. A kind outside
the three SHALL be reported as unsupported without a review or a read. Source: portal:D1,
portal:D8:R1, portal:D20.

#### Scenario: A kind the controller does not read

- **WHEN** a package's `spec.sourceRef` names kind `ConfigMap`
- **THEN** the result is unsupported, and the cluster receives no access review and no read

### Requirement: A source is read with one reviewed get

The read model SHALL read the source with one `get` for a caller whose grant covers that `get` in
the source's namespace, after the reader identity's own `get` review. It SHALL NOT list or watch
source objects, follow a source's `secretRef`, or read a core Secret, which it SHALL refuse before
any review or read. A reader denied the `get` SHALL make the source not readable. Source:
portal:D7:R3, portal:D8:R1, portal:D20.

#### Scenario: A Secret named as a source

- **WHEN** a package's `spec.sourceRef` names a core `Secret`
- **THEN** the read is refused without a review or a read

#### Scenario: The reader may not read the source

- **WHEN** the caller may get the OCIRepository but the reader identity's own `get` review is
  denied
- **THEN** the source is not readable and nothing is read

#### Scenario: A source in another namespace

- **WHEN** a package in `pkg-a` names the OCIRepository `pkg-b/podinfo-release` in its `sourceRef`,
  and the caller's grant covers `get` in `pkg-a` only
- **THEN** the grant does not cover the source, nothing is read, and the cluster receives no
  `get` for it

### Requirement: A source read returns the Ready state and the revision

The read model SHALL return the source's Ready condition status, reason and message and
`status.artifact.revision`, and SHALL NOT return `status.artifact.url`. A Ready source with no
`status.artifact` SHALL be reported as having no artifact, not as ready. Source: portal:D20.

#### Scenario: A ready OCIRepository

- **WHEN** a caller allowed to get OCIRepositories in `pkg` reads `pkg/podinfo-release`, which has
  `Ready=True` and artifact revision `v6.7.0@sha256:4f1c9a2e`
- **THEN** the result is ready with that revision and carries no artifact URL

#### Scenario: Ready without an artifact

- **WHEN** the OCIRepository has `Ready=True` and no `status.artifact`
- **THEN** the result says the source has no artifact, not that it is ready

### Requirement: A source that does not exist is reported as not found

An object that does not exist SHALL be reported as not found. Source: portal:D20:R2.

#### Scenario: A source that does not exist

- **WHEN** a caller allowed to get OCIRepositories in `pkg` reads `pkg/absent`, which does not
  exist
- **THEN** the result is not found

### Requirement: Source kind discovery is re-checked on a timer

Discovery of the source kinds SHALL be re-checked by the server on a timer, never on a request, so
that a kind installed after the portal starts is found. Source: portal:D20:R3, portal:D18:R3.

#### Scenario: Flux installed after the portal starts

- **WHEN** the cluster starts serving OCIRepositories after the portal started
- **THEN** within one discovery timer period, a lookup finds the kind served

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

### Requirement: Inventory Services carry the ready count of their EndpointSlices

When the caller and the portal's reader may list `endpointslices` in a Service's namespace, each
readable Service of an instance or package inventory SHALL carry the number of ready endpoints that
the cluster records in its EndpointSlices. Source: portal:D4, portal:D7.

#### Scenario: podinfo's Service

- **WHEN** the caller reads instance `default/podinfo` and the Service's slice lists two ready
  endpoints
- **THEN** the Service carries a ready count of 2

#### Scenario: A Service with no ready endpoint

- **WHEN** the Service's only slice lists no endpoint, or only endpoints whose `ready` is false
- **THEN** the Service carries a ready count of 0

#### Scenario: No slice

- **WHEN** no slice reaches the Service
- **THEN** the Service carries no count and no unread mark, which is not the count 0

### Requirement: A read that was not made is marked unread, never counted as zero

When the caller or the portal's reader may not list `endpointslices` in the Service's namespace, or
the list fails, times out or returns a continue token, the Service SHALL carry an unread mark and no
count. The mark and a count SHALL never be set together. Source: portal:D3:R2, portal:D7.

#### Scenario: A read that was not made

- **WHEN** the caller is denied `list` on `endpointslices`, or the portal's reader is, or the list
  errors, times out or returns a continue token
- **THEN** the Service carries the unread mark and no count

#### Scenario: A read that fails

- **WHEN** the list of slices fails or times out
- **THEN** the Service carries the unread mark and no count, the failure is logged without slice
  content, and the health of every object is unchanged

### Requirement: The count is computed for the graph views only

The count and the unread mark SHALL be computed only when the instance graph or the package graph
is built. The Instance and Package documents and their stream topics, the owner events endpoints,
the provider lookup of the Platform graph, the instance list, the package list and the admission of
a log topic SHALL ask no review of `endpointslices` and make no list of it. Source: portal:D3:R2.

#### Scenario: Paths that do not show the count

- **WHEN** a client reads an Instance or Package document, opens its stream topic, reads the events
  of an owner or of a named object, reads the Platform graph, reads the instance list or the package
  list, or the portal admits a log topic for a Pod
- **THEN** no review of `endpointslices` is asked and no list of `endpointslices` is made

### Requirement: Only slices that reach the Service are counted

A slice SHALL count only when it carries the label `kubernetes.io/service-name` with the Service's
name, its controller owner reference reaches that Service by the rule that attaches runtime children
to inventory objects, and its address type is IPv4 or IPv6. Source: portal:D7.

#### Scenario: A slice that does not reach the Service

- **WHEN** a slice carries the Service's name in the label but has no controller owner reference, or
  one that names another object
- **THEN** it adds nothing to the count

#### Scenario: An FQDN slice

- **WHEN** a slice of address type FQDN, owned by the Service, lists ready endpoints
- **THEN** it adds nothing to the count

### Requirement: Ready is the Kubernetes meaning, and an endpoint counts once

An endpoint SHALL be ready unless its `ready` condition is false. Within one address type an endpoint
SHALL count once, keyed by its target reference UID or else its first address, and as ready only if
no copy of it has `ready` false. The count SHALL be the larger of the sums of the two address types.
Source: portal:D7.

#### Scenario: A dual-stack Service

- **WHEN** a Service has an IPv4 slice and an IPv6 slice that each list the same two ready endpoints
- **THEN** the count is 2

#### Scenario: An endpoint in two slices

- **WHEN** one ready endpoint is listed in two slices of the same address type
- **THEN** it is counted once, and not at all if one copy has `ready` false

### Requirement: The EndpointSlice read keeps a number and nothing else

The read model SHALL reduce a Service's EndpointSlices to one integer when it reads them, SHALL hold
and log no slice, address, port, hostname, node name, zone or Pod reference, and SHALL not attach a
slice to any view as a runtime child. Source: portal:D11:R7.

#### Scenario: Nothing else is kept

- **WHEN** a slice holds addresses, a hostname, a node name, a zone and Pod names, and the caller
  reads the instance
- **THEN** no view, document or log line contains any of those values

### Requirement: The EndpointSlice read has its own failure and cache state

Slices SHALL be listed on demand and the count kept for the lifetime of an on-demand list of
runtime children, and the review for the caller SHALL be made again for each read that serves the
count. A denial for EndpointSlices SHALL not change the access, health or children of any other
kind. Source: portal:D3:R2.

#### Scenario: Pods stay readable

- **WHEN** the caller may list Pods, ReplicaSets and Jobs but not EndpointSlices
- **THEN** the instance carries its runtime children as before, and only the Service lacks a count

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

### Requirement: An owner is included only when the caller may list its kind and the answer is marked incomplete otherwise
For a caller the read model SHALL include an owner only when the caller may list that owner's kind
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
