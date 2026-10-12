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

### Requirement: The count is computed for detail views only

The count and the unread mark SHALL be computed only when the Instance or Package detail view is
built. The instance list, the package list and the admission of a log topic SHALL ask no review of
`endpointslices` and make no list of it. Source: portal:D3:R2.

#### Scenario: Lists and log admission

- **WHEN** a client reads the instance list or the package list, or the portal admits a log topic
  for a Pod
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
