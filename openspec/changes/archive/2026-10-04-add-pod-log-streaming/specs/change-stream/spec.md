## ADDED Requirements

### Requirement: Topics follow a fixed grammar

The portal SHALL accept exactly these topic names on a stream: `platform`, `instances`,
`instances:<namespace>`, `instance:<namespace>/<name>`, `package:<namespace>/<name>`,
`registration:<name>`, `events:<object topic>`, where an object topic is `platform`,
`instance:…`, `package:…` or `registration:…`, and `log:<namespace>/<pod>/<container>` with an
optional trailing `/previous` segment that asks for the previous container's logs. Namespaces and
containers SHALL be DNS-1123 labels and names and Pods DNS-1123 subdomains. Any other name, a
wildcard, an empty segment, or a fourth log segment other than `previous` SHALL be refused as a
bad request before the stream opens.

#### Scenario: A well-formed topic is accepted

- **WHEN** a client opens a stream with topics `instance:apps/blog` and `events:instance:apps/blog`
- **THEN** the stream opens with both topics attached

#### Scenario: A malformed topic is refused

- **WHEN** a client opens a stream with topic `instance:apps` or `instance:Apps/blog` or `pods:*`
- **THEN** the request is refused as a bad request
- **AND** no stream is opened and no authorization review is sent

#### Scenario: A log topic is served

- **WHEN** a client opens a stream with topic `log:apps/blog-0/server` or
  `log:apps/blog-0/server/previous`
- **THEN** the topic is authorized and attached like any other topic

#### Scenario: A malformed log topic is refused

- **WHEN** a client opens a stream with topic `log:apps/blog-0/server/current` or `log:apps/blog-0`
- **THEN** the request is refused as a bad request and no authorization review is sent

### Requirement: Log topics are capped per session and per process

A session SHALL follow at most four log topics across all of its streams, and the process SHALL
serve at most fifty distinct log topics at once. A request that would exceed either cap SHALL be
refused as too many topics before any authorization review is sent, and the topics already
attached SHALL keep streaming. A log topic another subscriber already follows SHALL NOT count
again towards the process cap.

#### Scenario: A fifth log topic in one session

- **WHEN** a session follows four log topics and attaches a fifth
- **THEN** the request is refused as too many topics
- **AND** the four log topics keep streaming

#### Scenario: A shared log topic counts once in the process

- **WHEN** the process serves fifty log topics and another session attaches one of them
- **THEN** the topic attaches

## REMOVED Requirements

### Requirement: Topics have a fixed grammar

**Reason**: Log topics are now served, so the requirement's reserved-and-refused clause and its
"A log topic is reserved" scenario no longer hold.
**Migration**: Replaced by "Topics follow a fixed grammar", which keeps every other topic form
and scenario and serves `log:` topics with an optional `previous` segment.
