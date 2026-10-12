## ADDED Requirements

### Requirement: A Service's graph node carries a ready endpoint count

The graph node of a Service SHALL carry `endpointsReady`, a non-negative integer, when the read
model holds a ready count for it, and SHALL omit the field otherwise. Source: portal:D2:R5.

#### Scenario: A count of two

- **WHEN** a client reads the graph of instance `default/podinfo` and the Service has two ready
  endpoints
- **THEN** the node `obj:_/Service/default/podinfo-podinfo` carries `endpointsReady` 2

#### Scenario: Zero is not absent

- **WHEN** the Service's slices record no ready endpoint
- **THEN** the node carries `endpointsReady` 0

#### Scenario: Neither field when no slice is recorded

- **WHEN** the cluster records no slice for the Service
- **THEN** the node carries neither `endpointsReady` nor `endpointsUnread`

### Requirement: A Service's graph node says when the count could not be read

The graph node of a Service SHALL carry `endpointsUnread`, the boolean true, when the read model
could not read the count, and SHALL omit it otherwise. The node SHALL never carry both
`endpointsUnread` and `endpointsReady`. Source: portal:D2:R5.

#### Scenario: Unread for a caller without the review

- **WHEN** a client who may read Services but not list EndpointSlices reads the same graph
- **THEN** the node carries `endpointsUnread` true and no `endpointsReady`, and the document is
  otherwise the same

### Requirement: No document carries more than the number

The two fields SHALL be the only values of an EndpointSlice on any document, and no document SHALL
carry the name, address, port or node of an endpoint. Both fields are additive within
`/api/v1alpha1`. Source: portal:D2:R5, portal:D11:R7.

#### Scenario: An EndpointSlice is not an object to fetch

- **WHEN** a client asks `object` for an EndpointSlice of the Service
- **THEN** the portal answers with the same refusal as for any object no inventory reaches

#### Scenario: The contract gate

- **WHEN** `task api:breaking` compares the OpenAPI document with the base
- **THEN** it reports no breaking change, and the pull request title needs no `!`
