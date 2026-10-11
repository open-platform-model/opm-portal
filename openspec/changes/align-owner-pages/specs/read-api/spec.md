## ADDED Requirements

### Requirement: An owner's events can be read merged over its inventory

The events resource of an instance or package SHALL accept `scope`. Its values are `owner`, the
default, which serves today's feed, and `all`. With `scope=all` it SHALL serve one `EventList` with
`scope` `all`, holding the folded events about the owner and about every object and runtime child
its inventory reaches that the caller may read, merged newest first. Core Secrets SHALL be left
out. Source: portal:D9:R3/R6.

#### Scenario: podinfo's merged feed

- **WHEN** a caller with every permission reads `instances/default/podinfo/events?scope=all` on
  the F1 capture
- **THEN** the list carries `scope` `all` and holds the instance's own events, the Deployment's
  `ScalingReplicaSet` event and the ReplicaSet's and Pods' events, newest first, with no
  `partial`

### Requirement: An unknown scope is refused

The events resource SHALL answer `400 bad_request` to any `scope` other than `owner` and `all`, and
to `scope=all` together with an object named by `group`, `kind`, `namespace` and `name`: one request
is either one object or everything. Source: portal:D9:R6.

#### Scenario: An unknown scope

- **WHEN** a caller reads `instances/default/podinfo/events?scope=everything`, or
  `?scope=all&kind=Deployment&name=podinfo-podinfo`
- **THEN** the response is `400` with a `bad_request` problem

### Requirement: The merged feed is read with one list per namespace and kind

The merged feed SHALL be read with one list per event namespace and regarded kind, never one read
per object. Events about a cluster-scoped object SHALL be read from namespace `default`. Source:
portal:D9:R3/R4/R6.

#### Scenario: Cluster-scoped objects' events

- **WHEN** a caller reads `instances/cert-manager/cert-manager/events?scope=all`
- **THEN** the events about its cluster-scoped objects are read from namespace `default`, after a
  review of list events there

### Requirement: The merged feed authorizes the owner before it looks anything up

The server SHALL review get on the owner and list of events in the owner's own namespace before it
looks the owner up, then read the owner and its inventory from held state, then review list of
events in each other event namespace one at a time. A denial of either first review SHALL be the
same `403` document a forbidden read gets, a missing owner `404`, and the portal's reader denied
the list in the owner's namespace `503` `not_readable_by_portal`. Source: portal:D9:R6, portal:D7:R4.

#### Scenario: The owner's namespace not listable

- **WHEN** the caller may get instance `default/podinfo` but may not list events in `default`,
  and reads its events with `scope=all`
- **THEN** the response is the same `403` document a forbidden read gets, and the instance is not
  looked up

#### Scenario: A missing owner

- **WHEN** a caller who may get instances and list events in `default` reads
  `instances/default/nothing/events?scope=all`
- **THEN** the response is `404` with a `not_found` problem, as for the owner's own feed

### Requirement: The merged feed leaves out what the caller cannot read

An object the caller may not read, or whose events' namespace the caller may not list or the server
could not list, SHALL be left out of the merged feed, and the list SHALL carry `partial: true`. A
caller SHALL see no event about an object or runtime child that the caller's own view of the owner
does not list, and no count of the feed SHALL include one. In-cluster, each line SHALL omit what a
one-object feed omits. Source: portal:D7:R4, portal:D8:R5.

#### Scenario: One namespace not listable

- **WHEN** the caller may list events in `cert-manager` but not in `default`, and reads
  `instances/cert-manager/cert-manager/events?scope=all`
- **THEN** the response is `200` with the events from `cert-manager` and `partial: true`, and no
  list runs in `default`

#### Scenario: Operator notes in-cluster

- **WHEN** a client of an in-cluster server reads a merged feed holding the operator's events and
  the kubelet's
- **THEN** the operator's lines carry no `note`, and the kubelet's carry theirs as written

#### Scenario: Both writers of an operator note in-cluster

- **WHEN** a client of an in-cluster server reads a merged feed that holds an event whose
  `reportingController` is `opm-controller`, an event with no `reportingController` that regards
  the instance, and the kubelet's events about its Pods
- **THEN** the first two carry no `note`, and the kubelet's carry theirs as written

### Requirement: Runtime children are left out together

The runtime children of an owner (ReplicaSets, Pods and Jobs) are read all or none for each owner.
A caller who may not list all three kinds in some namespace of the owner SHALL see no runtime child
of the owner in any namespace and no event about one. Source: portal:D3:R2, portal:D7:R4.

#### Scenario: A caller who cannot list the Pods

- **WHEN** user A, who may get the instance and its Deployment and list events in `default`, may
  list ReplicaSets and Jobs there but has no verb on Pods, reads
  `instances/default/podinfo/events?scope=all`, and user B, who may do everything, reads the same
- **THEN** A's list holds no event about a ReplicaSet, a Pod or a Job, no Pod name and no Pod event
  note appears anywhere in its body, and it carries `partial: true`
- **AND** each count in A's list equals B's count without the items about a ReplicaSet, a Pod or a
  Job
- **AND** A reading one Pod's events through the object parameters gets the same `403` document a
  forbidden read gets

#### Scenario: A caller who may list the Pods but not get them

- **WHEN** a caller who may get the instance and its Deployment, may list events, ReplicaSets, Pods
  and Jobs in `default`, and may not get a Pod, reads `instances/default/podinfo/events?scope=all`
- **THEN** every Pod that the list carries an event about is among the Pods in that caller's own
  document of the instance, and no event about any other Pod appears

### Requirement: Unread runtime children make the merged feed partial

The merged feed SHALL carry `partial: true` when the caller's view of the owner marks an inventory
object of a kind that has runtime children (Deployment, ReplicaSet, StatefulSet, DaemonSet, Job,
CronJob) as having unread children. Source: portal:D3:R2, portal:D7:R4.

#### Scenario: A workload with unread children

- **WHEN** a caller who may not list the Pods of `default` reads the merged feed of an instance
  whose inventory holds a Deployment
- **THEN** the list carries `partial: true`

#### Scenario: Children read in full

- **WHEN** a caller who may list ReplicaSets, Pods and Jobs in every namespace of the owner reads
  its merged feed
- **THEN** the list carries no `partial` for that reason

### Requirement: Served event notes are capped

The API SHALL serve no event note longer than 512 Unicode code points. A longer note SHALL be
served as its first 511 code points followed by an ellipsis, cut between code points, in every
events feed, the merged feed and the stream snapshot included. A note of 512 code points or fewer
SHALL be served as written. The cut SHALL follow the read model's fold of events by note, so that
folding sees the whole note.

#### Scenario: A long note

- **WHEN** an event about the instance carries a note of 2,000 code points and a client reads the
  merged feed
- **THEN** the item's `note` is its first 511 code points and an ellipsis, and a client of the
  per-object feed gets the same text

#### Scenario: A note at the limit

- **WHEN** an event carries a note of exactly 512 code points
- **THEN** it is served unchanged

#### Scenario: A note of multi-byte characters

- **WHEN** an event carries a note of 600 code points that each take three bytes
- **THEN** the served note is valid text of 512 code points, cut between two code points

#### Scenario: Notes that differ only after the cut

- **WHEN** two events about one object share their first 511 code points and differ after them
- **THEN** the read model folds them as two lines, and a local-mode API serves two lines whose
  notes are equal
- **AND** an in-cluster API serves one line with their counts added, because it folds the served
  lines again without their notes

### Requirement: Packages carry their prune setting

A package document and a package list item SHALL carry `prune`, the package's `spec.prune` as
written, when it is set, and nothing when it is not, never a default. Source: portal:D2:R2.

#### Scenario: The F1 package prunes

- **WHEN** a client reads package `pkg/podinfo` from the F1 capture
- **THEN** it carries `prune: true`

#### Scenario: Prune not set

- **WHEN** a package's spec does not set `prune`
- **THEN** its document carries no `prune` field
