## ADDED Requirements

### Requirement: An owner's events can be read merged over its inventory

The events resource of an instance or package SHALL accept `scope`. Its values are `owner`, the
default, which serves today's feed, and `all`.

With `scope=all`, it SHALL serve one `EventList` with `scope` `all`. The list holds the folded
events about the owner and about every object and runtime child its inventory reaches that the
caller may read, merged newest first. Core Secrets SHALL be left out.

The server SHALL authorize in this order, before any lookup each review covers:

1. get on the owner;
2. the inventory, from held state;
3. list of events in each namespace where those events live, one namespace at a time.

A denial of the owner get, or of the list of events in the owner's own namespace, SHALL be the
same `403` document a forbidden read gets. An object the caller may not read, or whose events'
namespace the caller may not list or the server could not list, SHALL be left out, and the list
SHALL carry `partial: true`. `scope` with any other value, or `scope=all` together with an object
named by the `group`, `kind`, `namespace` and `name` parameters, SHALL be `400 bad_request`. The
merged feed SHALL be read with one list per event namespace and regarded kind, never one read per
object. In-cluster, each line SHALL omit what a one-object feed omits. Source: portal:D9:R3/R4/R6,
portal:D7:R4, portal:D8:R5.

#### Scenario: podinfo's merged feed

- **WHEN** a caller with every permission reads `instances/default/podinfo/events?scope=all` on
  the F1 capture
- **THEN** the list carries `scope` `all` and holds the instance's own events, the Deployment's
  `ScalingReplicaSet` event and the ReplicaSet's and Pods' events, newest first, with no
  `partial`

#### Scenario: Cluster-scoped objects' events

- **WHEN** a caller reads `instances/cert-manager/cert-manager/events?scope=all`
- **THEN** the events about its cluster-scoped objects are read from namespace `default`, after a
  review of list events there

#### Scenario: One namespace not listable

- **WHEN** the caller may list events in `cert-manager` but not in `default`, and reads
  `instances/cert-manager/cert-manager/events?scope=all`
- **THEN** the response is `200` with the events from `cert-manager` and `partial: true`, and no
  list runs in `default`

#### Scenario: The owner's namespace not listable

- **WHEN** the caller may get instance `default/podinfo` but may not list events in `default`,
  and reads its events with `scope=all`
- **THEN** the response is the same `403` document a forbidden read gets

#### Scenario: An unknown scope

- **WHEN** a caller reads `instances/default/podinfo/events?scope=everything`, or
  `?scope=all&kind=Deployment&name=podinfo-podinfo`
- **THEN** the response is `400` with a `bad_request` problem

#### Scenario: Operator notes in-cluster

- **WHEN** a client of an in-cluster server reads a merged feed holding the operator's events and
  the kubelet's
- **THEN** the operator's lines carry no `note`, and the kubelet's carry theirs as written

### Requirement: Packages carry their prune setting

A package document and a package list item SHALL carry `prune`, the package's `spec.prune` as
written, when it is set, and nothing when it is not, never a default. Source: portal:D2:R4.

#### Scenario: The F1 package prunes

- **WHEN** a client reads package `pkg/podinfo` from the F1 capture
- **THEN** it carries `prune: true`

#### Scenario: Prune not set

- **WHEN** a package's spec does not set `prune`
- **THEN** its document carries no `prune` field
