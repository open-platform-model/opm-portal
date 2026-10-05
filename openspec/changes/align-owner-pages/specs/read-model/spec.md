## ADDED Requirements

### Requirement: Events are read merged over an inventory

The read model SHALL read the events about a set of reached objects with one list from
`events.k8s.io/v1` per event namespace and regarded kind. Each list SHALL be selected by the
regarded kind, filtered to the set by group, kind, namespace and name, folded as one object's
feed is, and merged newest first. Events about a cluster-scoped object SHALL be read from
namespace `default`. A namespace the caller's grant does not cover SHALL not be listed: its
objects are skipped and counted. The lists SHALL run under the read timeout, a bounded number at a
time. Source: portal:D9:R3/R4/R6.

#### Scenario: podinfo's objects

- **WHEN** the events about podinfo's Deployment, ReplicaSet and Pods are read merged on the F1
  capture
- **THEN** one list runs per regarded kind in namespace `default`, and the result holds each
  object's folded events and no event about another object

#### Scenario: A namespace without a grant

- **WHEN** the set holds objects in a namespace the caller's grants do not cover
- **THEN** no list runs there, and those objects are reported as skipped

### Requirement: Package views carry their prune setting

A package view SHALL carry `spec.prune` as written when it is set, and nothing when it is not.
Source: portal:D2:R4.

#### Scenario: The F1 package

- **WHEN** the caller reads package `pkg/podinfo` from the F1 capture
- **THEN** its prune setting is `true`
