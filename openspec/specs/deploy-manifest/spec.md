# deploy-manifest Specification

## Purpose
Defines the manifest under `deploy/` that runs local mode in a Pod as a single-user test tool:
which objects it holds, the read-only role the Pod reads through, the restricted Pod with no
network path but `kubectl port-forward`, the image it names, and the tests that hold all of it.

## Requirements

### Requirement: The manifest names the image of its own release

The Deployment's `image:` line SHALL name `ghcr.io/open-platform-model/opm-portal:v<version>` and
SHALL carry the `x-release-please-version` marker, and `release-please-config.json` SHALL list
`deploy/deployment.yaml` as a `generic` extra file, so each release PR moves the tag with
`internal/version`. A Go test SHALL fail when the tag is not `"v" + version.Version` or the marker
is not on the image line.

#### Scenario: A release PR

- **WHEN** release-please opens the release PR for version 0.2.0
- **THEN** it rewrites the image line to `ghcr.io/open-platform-model/opm-portal:v0.2.0` with the
  marker kept, beside `internal/version/version.go`

#### Scenario: A hand edit drifts

- **WHEN** the image tag differs from `"v" + version.Version`
- **THEN** the manifest test fails naming both values

### Requirement: The repository ships deploy/, a kustomization with five objects and no network path
The repository SHALL ship `deploy/`, a kustomization of plain YAML holding exactly a Namespace
`opm-portal`, a ServiceAccount `opm-portal`, a ClusterRole `opm-portal-reader`, a ClusterRoleBinding
of that role to that ServiceAccount, and a Deployment of one replica running `opm-portal serve
--addr 127.0.0.1:8090` as that ServiceAccount. It SHALL hold no Service, no Ingress, no Gateway
route and no probe, so `kubectl port-forward` is the only way to reach the portal. Source:
portal:D13:R2.

#### Scenario: Apply the manifest

- **WHEN** a cluster administrator runs `kubectl apply -k deploy/`
- **THEN** the five objects exist and the Deployment rolls out one Pod
- **AND** `kubectl -n opm-portal get services,ingresses` lists nothing

### Requirement: The Pod and the Namespace are locked down
The Pod SHALL run as non-root with the `RuntimeDefault` seccomp profile, and the container SHALL
have a read-only root filesystem, no privilege escalation, every capability dropped, and memory
requests and limits; the Pod SHALL share no host namespace, mount no volume and override no command,
and the Namespace SHALL enforce the `restricted` Pod Security Standard. Source: portal:D13:R2.

#### Scenario: The Pod is locked down

- **WHEN** the shipped Deployment starts
- **THEN** the Pod SHALL run as non-root with the `RuntimeDefault` seccomp profile, and the container SHALL have a read-only root filesystem, no privilege escalation, every capability dropped, and memory requests and limits

### Requirement: A Go test parses deploy/ and fails on a network path or a loosened setting
A Go test SHALL parse every `*.yaml` and `*.yml` file under `deploy/` and SHALL fail on a Service or
Ingress object, a probe, a bind address other than `127.0.0.1`, container arguments other than the
shipped ones, or a missing or loosened security setting; it SHALL also fail when the kustomization
carries a field other than `apiVersion`, `kind` and `resources`, lists a resource that is not a file
in `deploy/`, or leaves a manifest file unlisted. Source: portal:D13:R2.

#### Scenario: A Service is added

- **WHEN** a change adds a Service to `deploy/` or binds the container to `0.0.0.0:8090`
- **THEN** the manifest test fails naming the object or the address

#### Scenario: The security context is loosened

- **WHEN** a change removes `readOnlyRootFilesystem` or sets `allowPrivilegeEscalation: true`
- **THEN** the manifest test fails naming the setting

### Requirement: Comments in the deploy files carry no design citation
The repository SHALL ship `deploy/`, a kustomization of plain YAML. Comments in the files SHALL
carry no design citation. Source: portal:D13:R2.

#### Scenario: A comment in a deploy file

- **WHEN** a comment is written in a file of `deploy/`
- **THEN** it SHALL carry no design citation

### Requirement: Role grants on the OPM kinds, events, pods, replicasets and jobs
The ClusterRole `opm-portal-reader` SHALL list every rule explicitly and SHALL grant only the verbs
`get`, `list` and `watch`: on the four OPM kinds of `opmodel.dev`; on `events.k8s.io` `events` only
the verb the portal uses, `list`; on `pods` and `get` on `pods/log`; on `apps` `replicasets` and
`batch` `jobs`. Source: portal:D13:R3.

#### Scenario: The shipped role

- **WHEN** the manifest test reads `deploy/clusterrole.yaml`
- **THEN** every verb is `get`, `list` or `watch`, and no rule names `secrets`, `impersonate` or
  `*`

### Requirement: Role grants on endpointslices
The ClusterRole `opm-portal-reader` SHALL list every rule explicitly and SHALL grant only the verbs
`get`, `list` and `watch`: on `discovery.k8s.io` `endpointslices` only `list`. Source:
portal:D13:R3.

#### Scenario: The EndpointSlices rule

- **WHEN** the manifest test reads `deploy/clusterrole.yaml`
- **THEN** it finds one rule for `discovery.k8s.io` `endpointslices`, and the rule's verbs are
  exactly `list`

#### Scenario: A wider EndpointSlices rule

- **WHEN** the manifest test reads a role whose `endpointslices` rule also grants `get` or `watch`
- **THEN** the test fails and names the rule

#### Scenario: A write on EndpointSlices

- **WHEN** the manifest test checks a role granting `create` on `endpointslices`
- **THEN** the check returns an error naming the verb

### Requirement: Role grants on the non-Secret kinds the catalog renders
The ClusterRole `opm-portal-reader` SHALL list every rule explicitly and SHALL grant only the verbs
`get`, `list` and `watch`: on the non-Secret kinds the OPM catalog's transformers render, a list
maintained by hand against catalog 4.6.0's transformers. An inventory object of a kind the role does
not grant SHALL show as not readable, never omitted. An automated check that the role covers every
kind the pinned catalog renders stays with the in-cluster plan. Source: portal:D13:R3.

#### Scenario: A provider kind

- **WHEN** an instance's inventory names a cert-manager Certificate and the portal runs in the Pod
- **THEN** the object shows as not readable by the portal, and the rest of the instance renders

### Requirement: Role grants nothing else
The ClusterRole `opm-portal-reader` SHALL grant only the verbs `get`, `list` and `watch`. It SHALL
NOT grant any other verb, `secrets`, `impersonate`, a `*` in any group, resource or verb, any
subresource but `pods/log` (no `exec`, `attach`, `portforward` or `proxy`), a non-resource URL, or
an aggregation rule, and its one binding SHALL name only the ServiceAccount `opm-portal/opm-portal`,
never a group or a built-in role such as `view`. Source: portal:D13:R3.

#### Scenario: The role grants no Secrets

- **WHEN** the manifest test reads `deploy/clusterrole.yaml`
- **THEN** the role SHALL NOT grant `secrets`

### Requirement: A Go test fails on a role that grants more
A Go test SHALL fail on any of these, as listed in requirements "Role grants on the OPM kinds,
events, pods, replicasets and jobs", "Role grants on endpointslices", "Role grants on the non-Secret
kinds the catalog renders" and "Role grants nothing else", and SHALL show it does by refusing a
table of denied roles and bindings. Source: portal:D13:R3.

#### Scenario: A denied role

- **WHEN** the manifest test checks a role granting `create` on `pods`, `get` on `secrets`,
  `impersonate` on `users`, or `*` as a resource
- **THEN** each check returns an error naming the offending rule
