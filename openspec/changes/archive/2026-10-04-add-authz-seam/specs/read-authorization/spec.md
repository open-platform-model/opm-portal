## Purpose

Defines how the portal decides whether a caller may make one Kubernetes read before anything is
looked up, what it refuses without asking the cluster, and what a refusal reveals.

## ADDED Requirements

### Requirement: Every read is authorized before anything is looked up

The portal SHALL decide every Kubernetes read on the caller's identity and the read's attributes
(verb, group, version, resource, subresource, namespace, name) before it reads the object, and a
read path SHALL NOT be able to read without the proof of an allow for that read. The decision
SHALL be made without reading the object, so a caller without access receives the same refusal
for an object that exists and for one that does not, and the refusal SHALL NOT name the object
or the caller. Source: 0030:D7:R1.

#### Scenario: Existing and missing objects are refused alike

- **WHEN** a caller the cluster denies asks to get Deployment `exists`, which exists, and
  Deployment `missing`, which does not, in the same namespace
- **THEN** both requests are refused with the same code and the same message
- **AND** neither message contains the object's name or the caller's username
- **AND** the only call made to the cluster is the access review, never a read of a Deployment

#### Scenario: A read path cannot forge an allow

- **WHEN** code outside the authorization package tries to construct the proof of an allow
- **THEN** the build fails
- **AND** the zero value of the proof permits no read

#### Scenario: A proof covers only its own read

- **WHEN** a read path holds the proof of an allow for getting Pod `p` in namespace `a`
- **THEN** the proof covers that read
- **AND** it does not cover Pod `p`'s log, another Pod, another namespace, or another verb

### Requirement: An empty identity is refused before any cluster call

The portal SHALL refuse, with an unauthenticated denial and without any Kubernetes call made on
its behalf, every read for an identity whose username is empty, blank or `system:anonymous`,
whatever groups, UID or extra values it carries. Source: 0030:D6:R2, applied in both milestones.

#### Scenario: Groups without a username

- **WHEN** a read is requested for an identity with no username and the group `system:masters`
- **THEN** the read is refused as unauthenticated
- **AND** the cluster receives no request

#### Scenario: Anonymous identity

- **WHEN** a read is requested for the username `system:anonymous`
- **THEN** the read is refused as unauthenticated
- **AND** the cluster receives no request

### Requirement: Reads the portal never makes are refused without asking

The portal SHALL refuse, without asking the cluster, any request whose verb is not `get`, `list`
or `watch`, any request on core `secrets` whatever the caller's RBAC (Source: 0030:D8:R1), and any
request with an empty verb or resource or a wildcard in any attribute.

#### Scenario: Write verb

- **WHEN** a request asks to `delete` a Deployment
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

### Requirement: An authorization error is a denial

The portal SHALL treat an access review that fails, times out, or reports an evaluation error
without allowing as a denial with an `unavailable` code, never as an allow. An explicit deny SHALL
win over an allow, and a review with no opinion SHALL be a denial. The refusal's message SHALL NOT
carry the failure's own text, which can name the API server, users, roles or objects. Source:
0030:D6:R4.

#### Scenario: Review call fails

- **WHEN** the cluster answers the access review with an error or does not answer within the
  review timeout
- **THEN** the read is refused as unavailable

#### Scenario: Evaluation error without an allow

- **WHEN** the access review's status carries an evaluation error and is not allowed
- **THEN** the read is refused as unavailable
- **AND** the refusal's message does not carry the evaluation error's text

#### Scenario: No opinion

- **WHEN** the access review is neither allowed nor denied and carries no evaluation error
- **THEN** the read is refused as forbidden

### Requirement: Local mode asks the cluster as the kubeconfig's identity

In local mode the portal SHALL decide each read with a SelfSubjectAccessReview carrying the
read's exact attributes, sent with the user's kubeconfig, so the user's RBAC is the boundary. It
SHALL serve only the identity it was started for and refuse any other identity without a
cluster call. Source: 0030:D5:R1.

#### Scenario: Exact attributes are reviewed

- **WHEN** a read of Pod `web-0`'s `log` subresource in namespace `team-a` is requested
- **THEN** the cluster receives one SelfSubjectAccessReview for verb `get`, version `v1`,
  resource `pods`, subresource `log`, namespace `team-a`, name `web-0`

#### Scenario: Another identity

- **WHEN** a read is requested for an identity other than the one local mode was started for
- **THEN** the read is refused as unauthenticated
- **AND** the cluster receives no request

### Requirement: Decisions are cached briefly per identity and request

The portal SHALL reuse an allow or a deny for the same identity and the same attributes for at
most a short time-to-live (30 seconds by default), SHALL treat identities that differ only in the
order of their groups or extra values as the same, and SHALL NOT cache an unavailable outcome.

#### Scenario: Repeat within the time-to-live

- **WHEN** the same identity makes the same request twice within the time-to-live
- **THEN** the cluster receives one access review

#### Scenario: Repeat after the time-to-live

- **WHEN** the same request repeats after the time-to-live has passed
- **THEN** the cluster receives a new access review

#### Scenario: Failure is not remembered

- **WHEN** an access review fails and the same request is made again
- **THEN** the cluster receives a second access review
- **AND** an allow from it grants the read

#### Scenario: Another identity or request

- **WHEN** a request differs from a cached one in its identity, namespace, name or subresource
- **THEN** the cluster receives a new access review for it
