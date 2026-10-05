## ADDED Requirements

### Requirement: The server runs in a declared mode

The read API server SHALL be configured with its mode, `local` or `in-cluster`, and SHALL refuse
to start with no mode or any other value. `opm-portal serve` SHALL run it in `local` mode.

#### Scenario: No mode

- **WHEN** the server is built without a mode
- **THEN** it refuses to start and names the missing mode

### Requirement: In-cluster mode serves no operator text

In `in-cluster` mode no document, whether served by a `GET` or on the change stream, SHALL carry
text the operator wrote: a condition's `message`, a reconcile `message`, a history entry's
`message`, a registration's `message` or `activeMessage`, an event's `note`, the health
`message` of an inventory object or graph node of an `opmodel.dev` kind, or, in an `Object` of
an `opmodel.dev` kind, `status.conditions[].message` and `status.history[].message`. Every
reason, state, `tone`, `meaning` and `nextStep` SHALL stay. Events left alike once their notes
are dropped (same type, reason, reporting controller, regarded object and field path) SHALL be
served as one line, their counts summed and its time the latest, so the number of lines does not
tell how many distinct notes were dropped. A document type the omission does not
know SHALL fail with `upstream_unavailable` rather than be served. The OpenAPI document SHALL say,
on each of those fields, that it is absent in-cluster. Source: owner answer to 0030:OQ8.

#### Scenario: A failed instance in-cluster

- **WHEN** a client of an in-cluster server reads an instance whose `Ready` condition is `False`
  with reason `RenderFailed` and a message
- **THEN** the condition carries its type, status, reason, tone, meaning and next step, and no
  `message`; the reconcile state carries its reason and no `message`

#### Scenario: Events in-cluster

- **WHEN** a client of an in-cluster server reads an `EventList` or follows its `events:` topic
- **THEN** every event carries its type, reason, count and times, and no `note`

#### Scenario: Events differing only in their note in-cluster

- **WHEN** a client of an in-cluster server reads the `EventList` of an instance with two
  `Applied` events whose notes differ
- **THEN** it carries one `Applied` line with count 2 and the later time

#### Scenario: An OPM object's YAML in-cluster

- **WHEN** a client of an in-cluster server reads the `Object` of the TransformerRegistration
  `default.backup-provider` through the instance that reaches it
- **THEN** its `status.conditions` keep their type, status and reason and carry no `message`

#### Scenario: Local mode

- **WHEN** a client of a local server reads the same documents
- **THEN** every message and note is served as the operator and the API server wrote it

## MODIFIED Requirements

### Requirement: Values, Secrets and the apply annotation are never served

No document SHALL contain an instance's or package's `spec.values`, Secret data, or the
`kubectl.kubernetes.io/last-applied-configuration` annotation. In `local` mode condition, history
and event messages SHALL be served as the operator and the API server wrote them; in
`in-cluster` mode they SHALL be omitted. Source: 0030:D8:R2/R3.

#### Scenario: No values in any document

- **WHEN** every resource is read for every F1 instance, package and the platform
- **THEN** no response contains a `values` field or the last-applied annotation

### Requirement: Conditions carry their tone and the portal's explanation

Every condition a document serves SHALL carry `tone`, how the condition reads for its type:
`normal`, `abnormal`, `progressing`, `informational` or `unknown`. `Stalled=True` SHALL be
`abnormal`, `Reconciling=True` `progressing`, `ContractsFulfilled=False` and `Drifted=True`
`informational`, `Ready=False` `abnormal`; an `Unknown` status or a type the portal does not know
SHALL be `unknown`. A condition whose reason the portal explains SHALL carry its `meaning` and,
when a person can act on it, its `nextStep`; a reason the portal does not know SHALL carry
neither. Source: 0030:D2:R1, 0030:D3:R8.

#### Scenario: A refused registration's platform

- **WHEN** a client reads the F1 platform, whose `ContractsFulfilled` is `False`
- **THEN** that condition's `tone` is `informational` and its `meaning` explains unfulfilled
  contracts

#### Scenario: An unknown reason

- **WHEN** a condition carries a reason the portal does not know
- **THEN** it has no `meaning` and no `nextStep`, and in `local` mode its message is served
  unchanged
