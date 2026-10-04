## ADDED Requirements

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
