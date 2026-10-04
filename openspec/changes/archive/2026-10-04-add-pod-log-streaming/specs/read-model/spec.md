## ADDED Requirements

### Requirement: A Pod's reach from an inventory is answered for the caller

The read model SHALL answer, for a caller and one Pod, the inventory object of a ModuleInstance or
ModulePackage that the Pod is a runtime child of, through the same controller chain the views'
runtime children follow (a ReplicaSet, a Job, or the inventory workload itself). The answer SHALL
count only inventory objects the caller may read and runtime children the caller may list. The
caller's grant SHALL cover `get` on the Pod's `log` subresource before anything is looked up. A
Pod that does not exist, that no inventory reaches, or that is reached only through objects the
caller may not read SHALL get one and the same refusal; a read the reading identity cannot make
SHALL be reported as unavailable. Source: 0030:D10:R1, 0030:D7:R1.

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

#### Scenario: A grant that does not cover the Pod's log

- **WHEN** a caller's grant does not cover `get pods/log` on the Pod
- **THEN** the read model refuses before it reads anything
