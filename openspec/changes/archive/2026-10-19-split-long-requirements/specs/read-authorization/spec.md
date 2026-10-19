## REMOVED Requirements

### Requirement: Every read is authorized before anything is looked up
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "A read path reads only with the proof of an allow, which covers one identity and one read", "A refused read does not tell an existing object from a missing one".

### Requirement: Decisions are cached briefly per identity and request
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "An allow or a deny is reused for a short time-to-live for the same identity and attributes", "A grant expires with the decision it was issued from".

### Requirement: In-cluster mode asks the cluster as the signed-in user
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "In-cluster mode decides each read for a person with a SubjectAccessReview", "In-cluster mode creates only subjectaccessreviews", "In-cluster mode refuses an identity with no username and does not answer it as the portal's ServiceAccount".

### Requirement: In-cluster mode refuses system identities for people
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "In-cluster mode refuses a person with a system username or group", "In-cluster mode treats every identity as a person unless the portal built it as its own ServiceAccount".

### Requirement: Background reads hold a grant for the portal's ServiceAccount
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Background reads hold a grant issued from a review of the portal's ServiceAccount, named by the deployment or the mounted token", "The review of the portal's ServiceAccount is sent when the reader first needs the grant and again once the decision expires".

## ADDED Requirements

### Requirement: A read path reads only with the proof of an allow, which covers one identity and one read
The portal SHALL decide every Kubernetes read on the caller's identity and the read's attributes
(verb, group, version, resource, subresource, namespace, name) before it reads the object, and a
read path SHALL NOT be able to read without the proof of an allow for that read. The proof SHALL
cover only the identity it was issued to and only its own read, and SHALL stop covering any read
when the decision it was issued from expires. Source: portal:D7:R1.

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

### Requirement: A refused read does not tell an existing object from a missing one
The portal SHALL decide every Kubernetes read on the caller's identity and the read's attributes
(verb, group, version, resource, subresource, namespace, name) before it reads the object. The
decision SHALL be made without reading the object, so a caller without access receives the same
refusal for an object that exists and for one that does not, and the refusal SHALL NOT name the
object or the caller. Source: portal:D7:R1.

#### Scenario: Existing and missing objects are refused alike

- **WHEN** a caller the cluster denies asks to get Deployment `exists`, which exists, and
  Deployment `missing`, which does not, in the same namespace
- **THEN** both requests are refused with the same code and the same message
- **AND** neither message contains the object's name or the caller's username
- **AND** the only call made to the cluster is the access review, never a read of a Deployment

### Requirement: An allow or a deny is reused for a short time-to-live for the same identity and attributes
The portal SHALL reuse an allow or a deny for the same identity and the same attributes for at most
a short time-to-live (30 seconds by default), counted from when the access review answers, SHALL
treat identities that differ only in the order of their groups or extra values as the same, and
SHALL NOT cache an unavailable outcome.

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

### Requirement: A grant expires with the decision it was issued from
A grant SHALL expire with the decision it was issued from, so a revocation is seen by new checks and
by grants already held within one time-to-live plus one review (about 35 seconds with the defaults:
a 30 s time-to-live and a 5 s review timeout).

#### Scenario: A grant expires with its decision

- **WHEN** a grant is held and the decision it was issued from expires
- **THEN** the grant SHALL expire with the decision it was issued from

### Requirement: In-cluster mode decides each read for a person with a SubjectAccessReview
In-cluster the portal SHALL decide each read for a person with a SubjectAccessReview carrying that
person's username, UID, groups and extra values and the read's exact attributes, so the person's
RBAC is the boundary. Source: portal:D6:R1.

#### Scenario: Exact identity and attributes are reviewed

- **WHEN** a person `oidc:alice` with groups `oidc:team-a` and `system:authenticated` and one
  extra value asks to get Pod `web-0`'s `log` subresource in namespace `team-a`
- **THEN** the cluster receives one SubjectAccessReview whose user, UID, groups and extra are the
  person's and whose resource attributes are verb `get`, version `v1`, resource `pods`,
  subresource `log`, namespace `team-a`, name `web-0`

#### Scenario: Review fails

- **WHEN** the SubjectAccessReview errors, times out, or reports an evaluation error without
  allowing
- **THEN** the read is refused as unavailable and the next check sends a new review

### Requirement: In-cluster mode creates only subjectaccessreviews
In-cluster the only object the portal SHALL create is `authorization.k8s.io` `subjectaccessreviews`:
the portal SHALL NOT send a SelfSubjectAccessReview or a SelfSubjectReview, which would answer for
the portal's own ServiceAccount. Source: portal:D6:R9.

#### Scenario: No self review is ever sent

- **WHEN** in-cluster reads are checked for a person, for the portal's ServiceAccount, for an
  empty identity and for a refused system identity, allowed and denied
- **THEN** every request the cluster receives is a create of `subjectaccessreviews`
- **AND** no `selfsubjectaccessreviews` or `selfsubjectreviews` is ever created

### Requirement: In-cluster mode refuses an identity with no username and does not answer it as the portal's ServiceAccount
In-cluster an identity with an empty, blank or anonymous username SHALL be refused without any call,
and SHALL NOT be answered as the portal's ServiceAccount. Source: portal:D6:R2.

#### Scenario: Empty claims do not fall through to the portal's identity

- **WHEN** a sign-in yields an identity with no username but with groups, a UID and extra values
- **THEN** every read for it is refused as unauthenticated
- **AND** the cluster receives no request, and no grant is issued for the portal's ServiceAccount

### Requirement: In-cluster mode refuses a person with a system username or group
In-cluster the portal SHALL refuse as unauthenticated, without any call, a person whose username
starts with `system:` or who carries any group starting with `system:` other than
`system:authenticated`, so a person can never be answered with the portal's ServiceAccount's access
or a privileged group's. Source: portal:D6:R3.

#### Scenario: System username

- **WHEN** a person's identity names the username `system:serviceaccount:opm-portal:opm-portal`
  without the ServiceAccount's groups, or `system:admin`
- **THEN** the read is refused as unauthenticated
- **AND** the cluster receives no request

#### Scenario: System group

- **WHEN** a person carries the group `system:masters`
- **THEN** the read is refused as unauthenticated
- **AND** the cluster receives no request

### Requirement: In-cluster mode treats every identity as a person unless the portal built it as its own ServiceAccount
In-cluster every identity SHALL be a person unless the portal built it as its own ServiceAccount;
matching the ServiceAccount's username and groups does not make an identity the ServiceAccount.
Source: portal:D6:R3.

#### Scenario: Claims that spell the ServiceAccount

- **WHEN** a person's identity names the username `system:serviceaccount:opm-portal:opm-portal`
  with exactly the ServiceAccount's groups, and the ServiceAccount already holds an allow for
  the same read
- **THEN** the read is refused as unauthenticated
- **AND** the cluster receives no request
- **AND** the access log records the denial

### Requirement: Background reads hold a grant issued from a review of the portal's ServiceAccount, named by the deployment or the mounted token
In-cluster the read model's informers, polls and on-demand lists SHALL read only under a grant
issued from a SubjectAccessReview naming the portal's own ServiceAccount, with the groups the API
server gives it. The ServiceAccount SHALL be named by the deployment or read from the mounted
ServiceAccount token's subject. Source: portal:D6:R10 (partial).

#### Scenario: The ServiceAccount is reviewed by name

- **WHEN** the reader asks to list and watch ModuleInstances cluster-wide
- **THEN** the cluster receives SubjectAccessReviews whose user is
  `system:serviceaccount:<namespace>:<name>` and whose groups are `system:serviceaccounts`,
  `system:serviceaccounts:<namespace>` and `system:authenticated`

#### Scenario: Not a ServiceAccount

- **WHEN** the in-cluster authorizer is built with a reader that is not a ServiceAccount
  username, or a token whose subject is not one
- **THEN** it refuses to build

### Requirement: The review of the portal's ServiceAccount is sent when the reader first needs the grant and again once the decision expires
The SubjectAccessReview SHALL be sent when the reader first needs the grant (at startup for the
watched kinds) and, for polls and on-demand lists, again once the cached decision expires, so a
revoked role reaches them within one time-to-live plus one review. Stopping a running informer when
its review expires, as portal:D6:R10 requires, is deferred to the change that builds the in-cluster
command. Source: portal:D6:R10 (partial).

#### Scenario: Renewed per time-to-live

- **WHEN** the reader asks for the same read twice within the time-to-live, and again after it
- **THEN** the cluster receives one review for the first two and a new review for the third
