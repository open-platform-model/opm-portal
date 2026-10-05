## ADDED Requirements

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
access or a privileged group's. Source: 0030:D6:R3.

#### Scenario: System username

- **WHEN** a person's identity names the username `system:serviceaccount:opm-portal:opm-portal`
  without the ServiceAccount's groups, or `system:admin`
- **THEN** the read is refused as unauthenticated
- **AND** the cluster receives no request

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
