## MODIFIED Requirements

### Requirement: Instance and package pages are organised in tabs

Below the cards the page SHALL offer the tabs Graph, Provider when the item holds a registration,
Resources, Events, Logs and YAML, in that order. Each tab SHALL be a link carrying `tab=` that the
server renders, and SHALL keep the `node` and `focus` parameters.

The details panel SHALL show only on Graph and Resources, beside the tab's region where the page
is wide enough. With neither `node` nor `focus` set, it SHALL show the details of the node the
graph rests on
(the item's own node, unless the graph's resting rule picks a held registration whose claim needs
attention), without spotlighting it, and Clear selection SHALL return it there. The panel SHALL never be an empty placeholder.

The panel SHALL show the node's kind, name and state badges, its health message in a box toned by
its health, and its facts:

- for a Deployment, StatefulSet, DaemonSet, ReplicaSet, Pod or Service, the replica, image,
  strategy, node, restart, start, type and port facts its object document carries;
- for a Service also, an Endpoints fact. When its graph node carries `endpointsReady` the fact
  reads "N ready" and nothing more: the count the cluster records in the Service's EndpointSlices,
  which is not proof that the Service works. When the node carries `endpointsUnread` the fact reads
  "Could not be read", the words of the Pods line, and never "0 ready". When the node carries
  neither, because the cluster records no slice, the fact is left out;
- for a component, that a component is not a Kubernetes object and has no events or YAML of its
  own, and what it renders;
- for the item's own node, its identity facts and an Events link, and no link to the page already
  open.

The facts SHALL be a fixed list and SHALL appear only for a node the caller may read: a node that is
locked, forbidden or not readable SHALL carry none, and no object SHALL be read for it. An image
reference SHALL be shown without any userinfo before its host.

The panel SHALL keep its Open, Expand, YAML and Events links, and SHALL add a Logs link for a Pod.

Resources SHALL be a table with one row per inventory object and runtime child: kind, name,
component, origin (applied from the inventory, or made by the cluster below an inventory object),
health, and details (the health reason or desired replicas and the message). A Degraded or
Missing row SHALL be marked apart, and the row of the `node` or `focus` object SHALL be marked as
current. Each row SHALL link to the Graph tab focused on its node.
Configuration components SHALL fold as in the graph. The table SHALL be filtered by a `reason`
parameter shown as a removable chip. On a package page the first row SHALL be the package's
source, with origin "source". With nothing applied, the tab SHALL read "Nothing applied yet. The
graph fills in once an apply succeeds." on an instance, and "Nothing applied yet. The package has
never fetched its source." on a package that has no source artifact.

At 360 px wide, no tab SHALL make the page scroll sideways, and the Resources and Events tables
SHALL lay out each row as a card below 760 px.

Events SHALL filter by resource (grouped by kind, with "All resources" and the item itself as
choices, and otherwise only objects that have events in the feed), by type and by reason, each
choice with its count; on a feed that left objects out, "All resources" SHALL say its count
covers what could be read. The reason SHALL be shown as a chip
only when set. "Showing N of M" SHALL sit with the filters. The events SHALL be a table, and each
line SHALL show type, reason, the regarded resource (a link that filters to it when the item
reaches it), message and age.

Logs SHALL pick a Pod and container and follow its log as the logs panel does today.

YAML SHALL pick an object grouped by kind, open on the selected (`node`) object, else the
focused (`focus`) object, else the first readable object, and show no Secret and no values.
Source: portal:D8:R2, portal:D9:R3/R6, portal:D10, portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: From Resources to the graph

- **WHEN** the user follows the `podinfo-podinfo` Deployment's row in podinfo's Resources tab
- **THEN** the Graph tab opens with that node focused and the details panel showing it

#### Scenario: Events filtered by reason

- **WHEN** a browser opens podinfo's page with `tab=events&type=Warning`
- **THEN** only Warning events are listed, folded with their counts, and the type choice reads
  Warning with its count

#### Scenario: A Secret in YAML

- **WHEN** the YAML picker lists an inventory that holds a Secret
- **THEN** the Secret is listed as never read and cannot be picked

#### Scenario: The panel at rest

- **WHEN** a signed-in browser opens `/instances/default/podinfo` with no `focus`
- **THEN** the details panel shows the ModuleInstance `default/podinfo` with its identity facts
  and an Events link, and no node is spotlighted

#### Scenario: Clear selection returns to the resting node

- **WHEN** the user selects podinfo's Deployment node and then Clear selection, with script on
- **THEN** the details panel shows the node the graph rests on again, not an empty placeholder

#### Scenario: Tab order with a provider

- **WHEN** a browser opens `/instances/default/backup-provider` on the F1 capture
- **THEN** the tabs read Graph, Provider, Resources, Events, Logs and YAML, in that order

#### Scenario: The package's source row

- **WHEN** a browser opens `/packages/pkg/podinfo?tab=resources` on the F1 capture
- **THEN** the first row names `OCIRepository` `pkg/podinfo-release` with origin "source", and the
  tab says the package has never fetched its source

#### Scenario: Object facts in the panel

- **WHEN** the panel opens on podinfo's Deployment
- **THEN** it shows the desired replicas and the container image from the Deployment object, and
  no environment variable

#### Scenario: An image reference with userinfo

- **WHEN** the panel opens on a Deployment whose container image is
  `user:secret@registry.example/app:1`
- **THEN** the panel shows `registry.example/app:1`, and neither the user nor the password appears
  anywhere in the page

#### Scenario: A password that holds a slash

- **WHEN** the panel opens on a Deployment whose container image is
  `user:ab/cd@registry.example/app:1`
- **THEN** the panel shows `registry.example/app:1`, and neither the user nor any part of the
  password appears anywhere in the page

#### Scenario: A digest reference

- **WHEN** the panel opens on a Deployment whose container image is
  `registry.example/app@sha256:` followed by a digest
- **THEN** the panel shows the reference whole

#### Scenario: A digest reference with userinfo

- **WHEN** the panel opens on a Deployment whose container image is
  `user:secret@registry.example/app@sha256:` followed by a digest
- **THEN** the panel shows `registry.example/app@sha256:` and the digest, with no userinfo

#### Scenario: A sha384 digest reference

- **WHEN** the panel opens on a Deployment whose container image is
  `registry.example/app@sha384:` followed by a digest
- **THEN** the panel shows the reference whole, repository included

#### Scenario: A node the caller cannot read

- **WHEN** the panel opens on a node whose access for the caller is not ok
- **THEN** it shows no per-kind fact, and no request for the object is made

#### Scenario: Owner tabs at 360 px

- **WHEN** a browser 360 px wide opens the Graph, Resources, Events, Logs and YAML tabs of
  podinfo
- **THEN** the page does not scroll sideways on any of them, and each displayed row of the
  Resources and Events tables is a card

#### Scenario: Object facts not readable

- **WHEN** the panel opens on an object whose document the read API refuses
- **THEN** the panel shows the node's graph facts and says details from the object could not be
  read, and is not an error page

#### Scenario: YAML on arrival

- **WHEN** a browser opens podinfo's page with `tab=yaml&focus=<the Deployment's node>`
- **THEN** the Deployment's YAML is shown without picking it

#### Scenario: YAML follows a graph click

- **WHEN** the user clicks podinfo's Deployment node on the Graph tab and then follows the YAML
  tab
- **THEN** the YAML tab opens on the Deployment's YAML

#### Scenario: Only objects with events are offered

- **WHEN** a browser opens podinfo's Events tab and one reached object has no event in the feed
- **THEN** the Resource filter does not offer that object

#### Scenario: A resource link filters the feed

- **WHEN** the user follows the Deployment's name in a line of podinfo's Events tab
- **THEN** the tab lists only the Deployment's events, and the Resource choice reads its name

#### Scenario: A Service's ready endpoints

- **WHEN** the panel opens on podinfo's Service and its graph node carries `endpointsReady` 2
- **THEN** the facts list ends with an Endpoints fact that reads "2 ready"

#### Scenario: A Service with no ready endpoint

- **WHEN** the panel opens on a Service whose graph node carries `endpointsReady` 0
- **THEN** the Endpoints fact reads "0 ready", in the same words and without a tone

#### Scenario: A Service whose endpoints the caller may not list

- **WHEN** the caller may read Services but not EndpointSlices and the panel opens on podinfo's
  Service
- **THEN** the panel shows the Service's other facts and an Endpoints fact that reads "Could not be
  read", no count, and nothing on the page names EndpointSlices or says what is denied

#### Scenario: Endpoints the portal could not read

- **WHEN** the portal's reader may not list EndpointSlices, or the list fails or times out, and
  the panel opens on podinfo's Service
- **THEN** the Endpoints fact reads "Could not be read", the same words as for a denied caller,
  and it does not read "0 ready"

#### Scenario: A Service with no recorded slice

- **WHEN** the caller and the portal may list EndpointSlices and the cluster records none for the
  Service
- **THEN** the panel shows the Service's other facts and no Endpoints line

#### Scenario: Endpoints at 360 px

- **WHEN** a browser 360 px wide opens the panel on a Service whose Endpoints fact reads "2 ready"
  or "Could not be read"
- **THEN** the page does not scroll sideways

#### Scenario: Nothing but a number

- **WHEN** the panel opens on a Service whose slices hold addresses, a hostname, a node name and
  Pod names
- **THEN** none of those strings appears anywhere in the page
