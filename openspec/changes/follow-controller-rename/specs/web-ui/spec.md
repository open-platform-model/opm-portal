## MODIFIED Requirements

### Requirement: The instance page shows the record, the feed and the logs

The instance page SHALL show the instance's graph, its components with their objects and health,
its conditions with each known reason's meaning and next step beside the controller's message, its
status history newest first, the render contracts as plain text labelled as the contracts the
render used and not the instance's provider demand, and its recent events with repeats folded
into one line with a count and the latest time. Configuration components SHALL be grouped in
the components list as in the graph, with a count and their worst health, and the group SHALL
be open when any member is not healthy. ReplicaSets with zero desired replicas SHALL be folded
apart from the live ones. The feed SHALL be labelled as recent activity that expires after about
an hour, and the page SHALL say that render warnings are kept only as events. A logs panel SHALL
offer each container of each Pod an inventory reaches, following its log topic, rendering log
text as text only, and each Pod row SHALL link to its log. The package page SHALL show the same
for a ModulePackage, with its source and dependencies. Source: portal:D4:R3, portal:D9:R2/R3/R5,
portal:D10.

#### Scenario: podinfo's page

- **WHEN** a signed-in browser opens `/instances/default/podinfo` on the F1 capture
- **THEN** it shows the podinfo component with its Service and Deployment, the ReplicaSet and two
  Pods below, the conditions with their meanings, the history, the render contracts as text, the
  events feed with its expiry note, and one log pane entry per Pod container

#### Scenario: cert-manager's configuration components

- **WHEN** a signed-in browser opens `/instances/cert-manager/cert-manager` on the F1 capture
- **THEN** its configuration components are one closed group with their count and health, and
  its workload components are listed one by one

### Requirement: In-cluster pages show no controller text

Pages served over an `in-cluster` read API SHALL show no condition message, reconcile message,
history message, registration message or note of an event the controller reported, because the
read API does not serve them; they SHALL still show every reason, state and the portal's meaning
and next step, and SHALL show the kubelet's and other controllers' event notes and the health
messages of rendered workloads as local mode does. Source: owner answer to portal:OQ8
(portal:D8:R5); its scope is the supervisor ruling recorded under portal:D8.

#### Scenario: The broken podinfo page in-cluster

- **WHEN** the instance page of the image-break sample is rendered over an in-cluster read API
- **THEN** it shows the applied and health badges, each condition's reason, meaning and next
  step, and no message text the controller wrote

#### Scenario: A Pod's events in-cluster

- **WHEN** the events of a podinfo Pod are rendered over an in-cluster read API
- **THEN** the kubelet's and the scheduler's notes are shown as they wrote them

## RENAMED Requirements

- FROM: `### Requirement: In-cluster pages show no operator text`
- TO: `### Requirement: In-cluster pages show no controller text`
