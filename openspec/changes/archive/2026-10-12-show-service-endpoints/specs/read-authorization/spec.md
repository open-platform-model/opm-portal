## ADDED Requirements

### Requirement: A Service's EndpointSlices are read under their own review

The portal SHALL read the EndpointSlices of a Service only for a caller whose own access review
for `list` on `discovery.k8s.io` `endpointslices` in the Service's namespace is allowed, and SHALL
never use the review for the Service itself in its place. The review SHALL run for each caller on
each read that serves the count, before the cache or the cluster is touched. Source: portal:D5:R7,
portal:D7.

#### Scenario: Service allowed, EndpointSlices denied

- **WHEN** the caller may get and list Services but is denied `list` on `endpointslices` in
  `default`, and reads instance `default/podinfo`
- **THEN** the Service carries the unread mark and no ready count, and the portal makes no list of
  `endpointslices`

#### Scenario: Both allowed

- **WHEN** the caller may list Services and `endpointslices` in `default`
- **THEN** the review that was asked names verb `list`, resource `endpointslices` in group
  `discovery.k8s.io`, and namespace `default`

#### Scenario: A warm cache is not an allow

- **WHEN** one caller who may list `endpointslices` has read the count and a second caller who may
  not reads the same instance within the cache lifetime
- **THEN** the second caller gets the unread mark and no count

### Requirement: A review that does not allow serves the unread mark and makes no read

A review that is denied, fails or times out SHALL serve no count and make no read, and the Service
SHALL carry the unread mark in its place. Source: portal:D5:R7, portal:D7.

#### Scenario: A review that fails

- **WHEN** the access review for `endpointslices` errors or times out
- **THEN** the Service carries the unread mark and no count, and the rest of the instance is served
