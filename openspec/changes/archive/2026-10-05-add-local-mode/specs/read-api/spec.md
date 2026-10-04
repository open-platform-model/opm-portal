## ADDED Requirements

### Requirement: The change stream serves the topic kinds the mode routes to it

The read API SHALL serve the topics of its own documents itself and SHALL route any other topic
kind to the producer the serving mode configures for it, on the same stream and under the same
authorization. Local mode SHALL route `log:` topics to the pod-log producer, so a client follows a
container's log on the `stream` resource like any other topic. A mode that configures a producer
for a kind the read API serves itself SHALL fail to start. Source: 0030:D10:R2.

#### Scenario: A log topic on the read API's stream

- **WHEN** a local-mode client allowed `get pods/log` opens the `stream` resource with topic
  `log:default/<pod>/podinfo` for a Pod of the `podinfo` instance
- **THEN** the topic attaches and delivers a snapshot, then `log` messages

#### Scenario: A log topic without a log producer

- **WHEN** the read API runs with no producer routed for `log:` and a client asks for a log topic
- **THEN** the request is refused with `400` and code `bad_request`

#### Scenario: A producer that claims an API kind

- **WHEN** a mode routes `instance:` topics to a producer of its own
- **THEN** the read API refuses to start
