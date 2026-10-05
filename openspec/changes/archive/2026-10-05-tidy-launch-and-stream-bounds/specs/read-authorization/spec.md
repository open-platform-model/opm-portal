## MODIFIED Requirements

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
