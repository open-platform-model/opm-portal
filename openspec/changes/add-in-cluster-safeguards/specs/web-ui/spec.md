## ADDED Requirements

### Requirement: In-cluster pages show no operator text

Pages served over an `in-cluster` read API SHALL show no condition message, reconcile message,
history message, registration message or event note, because the read API does not serve them;
they SHALL still show every reason, state and the portal's meaning and next step. Source: owner
answer to 0030:OQ8.

#### Scenario: The broken podinfo page in-cluster

- **WHEN** the instance page of the image-break sample is rendered over an in-cluster read API
- **THEN** it shows the applied and health badges, each condition's reason, meaning and next
  step, and no message text the operator wrote
