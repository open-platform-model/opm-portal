# read-authorization Specification

## Purpose

Defines how the portal decides whether a caller may make one Kubernetes read before anything is
looked up, what it refuses without asking the cluster, and what a refusal reveals.

## Requirements

### Requirement: Every read is authorized before anything is looked up

The portal SHALL decide every Kubernetes read on the caller's identity and the read's attributes
(verb, group, version, resource, subresource, namespace, name) before it reads the object, and a
read path SHALL NOT be able to read without the proof of an allow for that read. The decision
SHALL be made without reading the object, so a caller without access receives the same refusal
for an object that exists and for one that does not, and the refusal SHALL NOT name the object
or the caller. The proof SHALL cover only the identity it was issued to and only its own read, and
SHALL stop covering any read when the decision it was issued from expires. Source: 0030:D7:R1.

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
- **AND** it does not cover the same read for another identity

#### Scenario: A held proof expires with its decision

- **WHEN** a read path holds the proof of an allow and the decision it was issued from expires
- **THEN** the proof no longer covers the read
- **AND** the read path has to ask again before reading

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
or `watch`, any request whose subresource is not empty, `status` or `log` (a `get` on `exec`,
`attach`, `portforward` or `proxy` opens a stream into a workload or node), any request on core
`secrets` whatever the caller's RBAC (Source: 0030:D8:R1), and any request with an empty verb or
resource or a wildcard in any attribute.

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
most a short time-to-live (30 seconds by default), counted from when the access review answers,
SHALL treat identities that differ only in the order of their groups or extra values as the same,
and SHALL NOT cache an unavailable outcome. A grant SHALL expire with the decision it was issued
from, so a revocation is seen by new checks and by grants already held within one time-to-live
plus one review (about 35 seconds with the defaults: a 30 s time-to-live and a 5 s review
timeout).

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

### Requirement: In-cluster mode asks the cluster as the signed-in user

In-cluster the portal SHALL decide each read for a person with a SubjectAccessReview carrying that
person's username, UID, groups and extra values and the read's exact attributes, so the person's
RBAC is the boundary. Source: 0030:D6:R1. The only object it SHALL create is
`authorization.k8s.io` `subjectaccessreviews`: it SHALL NOT send a SelfSubjectAccessReview or a
SelfSubjectReview, which would answer for the portal's own ServiceAccount. Source: 0030:D6:R9.
An identity with an empty, blank or anonymous username SHALL be refused without any call, and
SHALL NOT be answered as the portal's ServiceAccount. Source: 0030:D6:R2.

#### Scenario: Exact identity and attributes are reviewed

- **WHEN** a person `oidc:alice` with groups `oidc:team-a` and `system:authenticated` and one
  extra value asks to get Pod `web-0`'s `log` subresource in namespace `team-a`
- **THEN** the cluster receives one SubjectAccessReview whose user, UID, groups and extra are the
  person's and whose resource attributes are verb `get`, version `v1`, resource `pods`,
  subresource `log`, namespace `team-a`, name `web-0`

#### Scenario: No self review is ever sent

- **WHEN** in-cluster reads are checked for a person, for the portal's ServiceAccount, for an
  empty identity and for a refused system identity, allowed and denied
- **THEN** every request the cluster receives is a create of `subjectaccessreviews`
- **AND** no `selfsubjectaccessreviews` or `selfsubjectreviews` is ever created

#### Scenario: Empty claims do not fall through to the portal's identity

- **WHEN** a sign-in yields an identity with no username but with groups, a UID and extra values
- **THEN** every read for it is refused as unauthenticated
- **AND** the cluster receives no request, and no grant is issued for the portal's ServiceAccount

#### Scenario: Review fails

- **WHEN** the SubjectAccessReview errors, times out, or reports an evaluation error without
  allowing
- **THEN** the read is refused as unavailable and the next check sends a new review

### Requirement: In-cluster mode refuses system identities for people

In-cluster the portal SHALL refuse as unauthenticated, without any call, a person whose username
starts with `system:` or who carries any group starting with `system:` other than
`system:authenticated`, so a person can never be answered with the portal's ServiceAccount's
access or a privileged group's. Every identity is a person unless the portal built it as its own
ServiceAccount; matching the ServiceAccount's username and groups does not make an identity the
ServiceAccount. Source: 0030:D6:R3.

#### Scenario: System username

- **WHEN** a person's identity names the username `system:serviceaccount:opm-portal:opm-portal`
  without the ServiceAccount's groups, or `system:admin`
- **THEN** the read is refused as unauthenticated
- **AND** the cluster receives no request

#### Scenario: Claims that spell the ServiceAccount

- **WHEN** a person's identity names the username `system:serviceaccount:opm-portal:opm-portal`
  with exactly the ServiceAccount's groups, and the ServiceAccount already holds an allow for
  the same read
- **THEN** the read is refused as unauthenticated
- **AND** the cluster receives no request
- **AND** the access log records the denial

#### Scenario: System group

- **WHEN** a person carries the group `system:masters`
- **THEN** the read is refused as unauthenticated
- **AND** the cluster receives no request

### Requirement: Background reads hold a grant for the portal's ServiceAccount

In-cluster the read model's informers, polls and on-demand lists SHALL read only under a grant
issued from a SubjectAccessReview naming the portal's own ServiceAccount, with the groups the API
server gives it. The review SHALL be sent when the reader first needs the grant (at startup for
the watched kinds) and again once the cached decision expires, so a revoked role is seen within
one time-to-live. The ServiceAccount SHALL be named by the deployment or read from the mounted
ServiceAccount token's subject.

#### Scenario: The ServiceAccount is reviewed by name

- **WHEN** the reader asks to list and watch ModuleInstances cluster-wide
- **THEN** the cluster receives SubjectAccessReviews whose user is
  `system:serviceaccount:<namespace>:<name>` and whose groups are `system:serviceaccounts`,
  `system:serviceaccounts:<namespace>` and `system:authenticated`

#### Scenario: Renewed per time-to-live

- **WHEN** the reader asks for the same read twice within the time-to-live, and again after it
- **THEN** the cluster receives one review for the first two and a new review for the third

#### Scenario: Not a ServiceAccount

- **WHEN** the in-cluster authorizer is built with a reader that is not a ServiceAccount
  username, or a token whose subject is not one
- **THEN** it refuses to build

### Requirement: Every decision about a person is logged

When an access log is configured, the portal SHALL write one structured line per decision about
a person, allowed or denied, from the cache or not, naming the username, the read's attributes,
the decision and the denial code. The line SHALL NOT carry groups, extra values, tokens, the
review's error text or any object content. Decisions about the portal's ServiceAccount SHALL NOT
be logged. Source: 0030:D6:R7.

#### Scenario: Allowed and denied reads are logged

- **WHEN** a person is allowed one read and denied another, and repeats the first within the
  time-to-live
- **THEN** the access log holds three lines naming the person, each read, `allow` or `deny`, and
  whether it came from the cache

#### Scenario: Nothing secret reaches the log

- **WHEN** a review fails with an error whose text names another user
- **THEN** the access log line says `deny` with code `unavailable`
- **AND** it carries neither the error's text nor the person's groups or extra values
