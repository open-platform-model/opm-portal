## ADDED Requirements

### Requirement: Local mode names its connection to the read API

`opm-portal serve` SHALL give the read API the name of the kubeconfig context it loaded and the
name of that context's cluster entry, or, when client-go fell back to the in-cluster
configuration, the `in-cluster` source with no context. It SHALL pass no server URL, user entry or
credential. Source: portal:D18:R1.

#### Scenario: A named context

- **WHEN** `opm-portal serve --context kind-opm-portal-e2e` starts
- **THEN** the read API's `Cluster` document names context `kind-opm-portal-e2e` and that
  context's cluster entry

#### Scenario: In a Pod

- **WHEN** the shipped Deployment starts with no kubeconfig
- **THEN** the `Cluster` document has `source: in-cluster` and no context
