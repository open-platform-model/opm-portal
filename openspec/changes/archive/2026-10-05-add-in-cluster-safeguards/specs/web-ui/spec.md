## ADDED Requirements

### Requirement: In-cluster pages show no operator text

Pages served over an `in-cluster` read API SHALL show no condition message, reconcile message,
history message, registration message or note of an event the operator reported, because the
read API does not serve them; they SHALL still show every reason, state and the portal's meaning
and next step, and SHALL show the kubelet's and other controllers' event notes and the health
messages of rendered workloads as local mode does. Source: owner answer to portal:OQ8
(portal:D8:R5); its scope is the supervisor ruling recorded under portal:D8.

#### Scenario: The broken podinfo page in-cluster

- **WHEN** the instance page of the image-break sample is rendered over an in-cluster read API
- **THEN** it shows the applied and health badges, each condition's reason, meaning and next
  step, and no message text the operator wrote

#### Scenario: A Pod's events in-cluster

- **WHEN** the events of a podinfo Pod are rendered over an in-cluster read API
- **THEN** the kubelet's and the scheduler's notes are shown as they wrote them
