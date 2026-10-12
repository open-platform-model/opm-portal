## ADDED Requirements

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
- **THEN** the Service carries no count, which is not the count 0

### Requirement: Only slices that reach the Service are counted, and ready is the Kubernetes meaning

A slice SHALL count only when it carries the label `kubernetes.io/service-name` with the Service's
name and its controller owner reference reaches that Service by the rule that attaches runtime
children to inventory objects. An endpoint SHALL be ready unless its `ready` condition is false.
The count SHALL be the largest sum over the address types of the Service. Source: portal:D7.

#### Scenario: A slice that does not reach the Service

- **WHEN** a slice carries the Service's name in the label but has no controller owner reference, or
  one that names another object
- **THEN** it adds nothing to the count

#### Scenario: A dual-stack Service

- **WHEN** a Service has an IPv4 slice and an IPv6 slice that each list the same two ready endpoints
- **THEN** the count is 2

### Requirement: The EndpointSlice read keeps a number and nothing else

The read model SHALL reduce a Service's EndpointSlices to one integer when it reads them, SHALL hold
and log no slice, address, port, hostname, node name, zone or Pod reference, and SHALL not attach a
slice to any view as a runtime child. Source: portal:D8.

#### Scenario: Nothing else is kept

- **WHEN** a slice holds addresses, a hostname, a node name, a zone and Pod names, and the caller
  reads the instance
- **THEN** no view, document or log line contains any of those values

#### Scenario: A read that fails

- **WHEN** the list of slices fails or times out
- **THEN** the Service carries no count, the failure is logged without slice content, and the
  health of every object is unchanged

### Requirement: The EndpointSlice read has its own failure and cache state

Slices SHALL be listed on demand and the count kept for the lifetime of an on-demand list of
runtime children, and the review for the caller SHALL be made again for each read that serves the
count. A denial for EndpointSlices SHALL not change the access, health or children of any other
kind. Source: portal:D3:R2.

#### Scenario: Pods stay readable

- **WHEN** the caller may list Pods, ReplicaSets and Jobs but not EndpointSlices
- **THEN** the instance carries its runtime children as before, and only the Service lacks a count
