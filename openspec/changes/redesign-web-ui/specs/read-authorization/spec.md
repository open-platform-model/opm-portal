## ADDED Requirements

### Requirement: One non-resource read is reviewed like a resource read

The portal SHALL decide a read of the non-resource path `/version` with verb `get` as it decides a
resource read: on the caller's identity, before the read, through the same proof of an allow,
cache and denial rules. Local mode SHALL send a SelfSubjectAccessReview with non-resource
attributes `{path: /version, verb: get}`; in-cluster mode SHALL send the SubjectAccessReview
equivalent. A proof for that read SHALL cover only that path and verb for that identity, and no
resource read SHALL be covered by it. Source: portal:D18:R3, portal:D5:R7.

#### Scenario: The version review in local mode

- **WHEN** the reader asks to read `/version`
- **THEN** the cluster receives one SelfSubjectAccessReview with non-resource attributes path
  `/version` and verb `get`

#### Scenario: A version proof covers nothing else

- **WHEN** a read path holds the proof of an allow for `get /version`
- **THEN** it does not cover `get /healthz`, `list` on `/version`, or any resource read

## MODIFIED Requirements

### Requirement: Reads the portal never makes are refused without asking

The portal SHALL refuse, without asking the cluster, any request whose verb is not `get`, `list`
or `watch`, any request whose subresource is not empty, `status` or `log` (a `get` on `exec`,
`attach`, `portforward` or `proxy` opens a stream into a workload or node), any request on core
`secrets` whatever the caller's RBAC (Source: portal:D8:R1), and any request with an empty verb or
resource or a wildcard in any attribute. The one exception to the empty-resource rule SHALL be a
`get` on the non-resource path `/version`; any other non-resource path, any other verb on
`/version`, and a request naming both a non-resource path and a resource SHALL be refused without
asking.

#### Scenario: Write verb

- **WHEN** a request asks to `delete` a Deployment
- **THEN** it is refused as forbidden
- **AND** the cluster receives no request

#### Scenario: Streaming subresource

- **WHEN** a request asks to `get` Pod `p`'s `exec` subresource, or Node `n`'s `proxy` subresource
- **THEN** it is refused as forbidden
- **AND** the cluster receives no request

#### Scenario: Secret read by a caller who may read Secrets

- **WHEN** a caller the cluster would allow asks to `get` a Secret
- **THEN** it is refused as forbidden
- **AND** the cluster receives no request

#### Scenario: Wildcard resource

- **WHEN** a request names the resource `*`
- **THEN** it is refused as invalid
- **AND** the cluster receives no request

#### Scenario: Another non-resource path

- **WHEN** a request asks to `get` the non-resource path `/metrics` or `/healthz`
- **THEN** it is refused as forbidden
- **AND** the cluster receives no request
