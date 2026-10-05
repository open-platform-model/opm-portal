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

## MODIFIED Requirements

### Requirement: Local mode listens on loopback only

`opm-portal serve` SHALL listen on the address `--addr` names, `127.0.0.1:7878` by default, so the
browser origin, and the theme and filters stored for it, stay the same across restarts. It SHALL
refuse, with exit code 2 and before it reads the kubeconfig, an address whose host is empty, a
name other than `localhost`, or an IP address outside the loopback ranges, and SHALL check the
bound address again after listening. When the address is in use it SHALL exit 1 with a message
naming the address and saying to pass `--addr` for another port. `--addr 127.0.0.1:0` SHALL still
pick a free port. Source: portal:D5:R2, portal:D14:R6.

#### Scenario: Default address

- **WHEN** a user runs `opm-portal serve` with no `--addr`
- **THEN** the portal listens on `127.0.0.1:7878` and the printed launch link names that port

#### Scenario: A non-loopback address

- **WHEN** a user runs `opm-portal serve --addr 0.0.0.0:8080`, `--addr :8080`,
  `--addr 192.168.1.10:8080` or `--addr example.com:8080`
- **THEN** standard error says the address is not loopback
- **AND** the exit code is 2 and the kubeconfig is not read

#### Scenario: The default port is taken

- **WHEN** another process listens on `127.0.0.1:7878` and a user runs `opm-portal serve` with no
  `--addr`
- **THEN** standard error says `127.0.0.1:7878` is in use and to pass `--addr` for another port
- **AND** the exit code is 1 and no launch link is printed

#### Scenario: The Pod keeps its own address

- **WHEN** the shipped Deployment starts `opm-portal serve --addr 127.0.0.1:8090`
- **THEN** the portal listens on `127.0.0.1:8090`, not the default
