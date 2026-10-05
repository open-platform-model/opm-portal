## MODIFIED Requirements

### Requirement: The instance page shows the record, the feed and the logs

The instance page SHALL show:

- the instance's graph;
- its inventory objects and runtime children with their health, in the Resources tab;
- its conditions, with each known reason's meaning and next step beside the controller's
  message, in a Conditions fold below the tab panels;
- its status history, newest first, behind "Show history";
- the render contracts as plain text, labelled as the contracts the render used and not the
  instance's provider demand;
- its recent events.

The Events tab SHALL open on the events of the instance and of every object and runtime child its
inventory reaches that the caller may read, merged newest first, with repeats folded into one line
with a count and the latest time. When some reached objects' events could not be read, the tab
SHALL say that they are left out, never show their absence as no events. Configuration
components SHALL be folded in the Resources tab into the same groups the graph folds them into,
each with its count and worst health, and a group SHALL be open when any member is not healthy.
ReplicaSets with zero desired replicas SHALL be folded apart from the live ones. The feed SHALL be
labelled as recent activity that expires after about an hour, and the page SHALL say that render
warnings are kept only as events.

The Logs tab SHALL offer each container of each Pod an inventory reaches through one Pod and
container picker. It SHALL show the picked container, or the first one when none is picked, in
one pane that follows its log topic as soon as the tab opens, and SHALL render log text as text
only. The details panel of a Pod SHALL link to its log. When the picked container waits with a
reason under which no instance of it has started (or, where the Pod does not say per container,
its Pod does), the tab SHALL say the container has not started, so there are no logs yet, and
link to that Pod's events. When the picked container is in `CrashLoopBackOff`, the tab SHALL say
it is restarting and point to its previous instance's logs.

The package page SHALL show the same for a ModulePackage, with its source, interval, prune
setting, source revision and dependencies. Source: portal:D4:R3, portal:D9:R2/R3/R5/R6,
portal:D10, portal:D16:R1.

#### Scenario: podinfo's page

- **WHEN** a signed-in browser opens `/instances/default/podinfo` on the F1 capture
- **THEN** across its tabs it shows the podinfo component with its Service and Deployment, the
  ReplicaSet and two Pods below, the conditions with their meanings, the history, the render
  contracts as text, the events feed with its expiry note, and a log choice for each Pod
  container

#### Scenario: cert-manager's configuration components

- **WHEN** a signed-in browser opens `/instances/cert-manager/cert-manager?tab=resources` on the
  F1 capture
- **THEN** its configuration components are folded into the same closed groups the graph shows,
  each with its count and health, and its workload components are listed one by one

#### Scenario: Every reached object's events on arrival

- **WHEN** a signed-in browser opens `/instances/default/podinfo?tab=events` on the F1 capture
- **THEN** with no resource filter the list holds the instance's own events and the Deployment's
  `ScalingReplicaSet` event, newest first

#### Scenario: Events of one namespace not readable

- **WHEN** the caller may list events in the instance's namespace but not in `default`, where the
  events about the instance's cluster-scoped objects live, and opens its Events tab
- **THEN** the tab lists the events it could read and says that some objects' events could not
  be read and are left out

#### Scenario: Logs open on arrival

- **WHEN** a browser with script opens podinfo's Logs tab
- **THEN** the first Pod's first container is picked and its pane follows its log without a
  further click

#### Scenario: A container that has not started

- **WHEN** the picked container belongs to a Pod waiting with `ImagePullBackOff`
- **THEN** the Logs tab says the container has not started, so there are no logs yet, and links
  to the Events tab filtered to that Pod

#### Scenario: A crash-looping container

- **WHEN** the picked container waits with `CrashLoopBackOff`
- **THEN** the Logs tab says the container is restarting and points to its previous instance's
  logs, and does not say it has not started

### Requirement: Instance and package pages summarize Applied, Health and Provider standing

The page of an instance or package SHALL show an identity card. The card SHALL carry the kind,
the name and, under the name, the module path and version, or for a package "from" its source
kind, namespace and name, with the namespace defaulting to the package's own. It SHALL also carry
the namespace, the owner and the applier ServiceAccount, each of the last two with an
explanation reachable by pointer and keyboard. A package's card SHALL add its path, interval,
prune setting and source revision.

The page SHALL show an Applied card and a Health card, and a Provider card when the item holds a
TransformerRegistration. Each card SHALL be a state block: an eyebrow naming the axis and its
source, the recorded time of the state when there is one, an icon and a large state word in the
state's tone, a one-line summary, and reason counts with the count first, each linking to its
evidence, or a line saying there are none.

The Applied card:

- SHALL show the applied state as its word. It SHALL show "Reconciling" instead while the
  `Reconciling` condition is `True` on an applied item, with that condition's time.
- SHALL compose its summary only from recorded facts: the inventory count, the fetched revision,
  the run of failed attempts and whether the controller retries. It SHALL name a retry interval
  only where the controller retries at that interval.
- SHALL show the reason's meaning and, in local mode, the controller's message beneath the
  summary.
- SHALL show a `Reconciling` mark with its reason and since time while that condition is `True`
  and the word is not "Reconciling".
- SHALL count the Warning events about the item itself by reason, labelled as the last hour of
  events, each count linking to the Events tab filtered by that reason.
- SHALL draw one dot per `status.history` entry, oldest first, marked and coloured by the entry's
  outcome, with no dot for an attempt the history does not record.
- SHALL show the number of attempts and a Show history control. The opened history SHALL show
  each entry's outcome and SHALL say that at most ten attempts are kept and that reconciles that
  change nothing are not recorded.

The Health card:

- SHALL show the health state as its word.
- SHALL say how many resources are unhealthy or rolling out out of how many were counted, or that
  nothing was applied, so there is nothing to check.
- SHALL show its partial mark, and when it is not live, the time it was evaluated. A health with
  no counted object SHALL show no time and SHALL not be called "not live".
- SHALL count objects and runtime children by health reason, each linking to the Resources tab
  with `reason` set to it.

The Provider card SHALL show, per held registration, Active, Accepted and not active, Refused,
Removal blocked or Pending as the controller recorded it, or a locked standing when the caller
may not read registrations. Its summary SHALL be the contracts an active registration provides,
or else the controller's message in local mode and the reason's meaning in-cluster, never a
verdict the portal composed. A non-success reason SHALL link to the Provider tab.

An interval the package does not set SHALL read "not set"; a revision the package has not
recorded SHALL read "not recorded". Source: portal:D3:R1/R4/R5, portal:D9:R1/R2,
portal:D15:R2/R3/R4, portal:D8:R5/R6.

#### Scenario: The image-break podinfo

- **WHEN** the page of the image-break podinfo sample is rendered
- **THEN** the Applied card reads Applied and the Health card reads Degraded with a summary
  counting the unhealthy resources and one count for the Pod's waiting reason, linking to
  `tab=resources&reason=<that reason>`, which lists that Pod

#### Scenario: A failed attempt in the history

- **WHEN** an instance's newest history entry has no phase and a message
- **THEN** its dot is drawn and marked failed, and in local mode its tooltip carries the message

#### Scenario: The F1 package

- **WHEN** a browser opens `/packages/pkg/podinfo` on the F1 capture
- **THEN** the identity card reads "from OCIRepository pkg/podinfo-release" under the name, with
  interval "every 1 min", prune on and revision "not recorded"
- **AND** the Applied card reads Failed with reason `SourceNotReady`, a summary naming the three
  failed attempts, a `Reconciling` mark with reason `Progressing`, and three failed dots and no
  other dot

#### Scenario: Rolling out

- **WHEN** an applied instance's `Reconciling` condition turns `True`
- **THEN** its Applied card reads Reconciling in the progressing tone, with the condition's time
  and reason

#### Scenario: Nothing applied yet

- **WHEN** a package whose inventory names no object is rendered
- **THEN** its Health card reads Unknown with "Nothing applied yet, so nothing to check", shows no
  time, and does not say "not live"

#### Scenario: A refused provider in-cluster

- **WHEN** a package holding a claim the controller refused is rendered over an in-cluster read
  API
- **THEN** its Provider card reads Refused with the reason's meaning as its summary and no
  message text the controller wrote

#### Scenario: Platform forbidden on the card

- **WHEN** the caller may read `default/backup-provider` and its registration but not the
  Platform
- **THEN** the Provider card reads Active from the claim, with no time and no list of provided
  contracts, and the page is not an error

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
- for a component, that a component is not a Kubernetes object and has no events or YAML of its
  own, and what it renders;
- for the item's own node, its identity facts and an Events link, and no link to the page already
  open.

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
Source: portal:D8:R2, portal:D9:R3/R6, portal:D10, portal:D14:R7.

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

### Requirement: The Provider tab shows the claim and who uses what it provides

The Provider tab SHALL open with a sentence, per kind, saying the item is also a provider that
registers a catalog with the platform. For a refused claim, the sentence SHALL add, from the
controller's verdict, that it provides nothing.

Per held registration, the tab SHALL show:

- the registration's name and its standing once: acceptance and activation as two marks, and the
  reason only when it is not a success reason;
- a grid of facts: the registration, its claimed catalog (linking to the Catalog page) and
  version, whether that catalog is in the Platform's resolved registry and contributed by this
  registration, how many contracts it provides and how many are in use (or "none" for a refused
  claim, and no in-use count when a list of users is incomplete or locked), the `providerRef`,
  and in local mode the controller's message;
- its "Registration conditions" (type, a status badge toned by the condition's tone, reason,
  meaning and since, and in local mode the message);
- a table of what it provides, where each contract is a pill showing its short path, with the full
  path as its title, and with the readable instances whose render contracts contain it.

For each contract the table SHALL list the first three users and the count of the rest. It SHALL
link to Installed filtered by that contract as "Show in Installed" when there are three or fewer,
and "All N in Installed" otherwise. When no readable instance uses the contract and the list is
complete, the tab SHALL say no installed module uses it yet and offer no link. When the caller
could not list instances everywhere, the list SHALL say it may be incomplete.

The registration's conditions, catalog and registry state come from the Platform document. When
the caller may not read the Platform, those parts SHALL render locked, and the tab SHALL keep the
registration's name and standing. Source: portal:D16:R3, portal:D4:R4, portal:D7:R3,
portal:D15:R2/R3.

#### Scenario: Who uses the backup trait

- **WHEN** a browser opens backup-provider's Provider tab on the F1 capture
- **THEN** contract `opmodel.dev/catalogs/opm/traits/backup@v1alpha1` lists
  `default/backup-consumer` and links to
  `/installed?uses=opmodel.dev/catalogs/opm/traits/backup@v1alpha1` as "Show in Installed", and
  the Contracts fact reads "1 provided, 1 in use"

#### Scenario: Platform forbidden

- **WHEN** the caller may read `default/backup-provider` and list TransformerRegistrations but may
  not get the Platform, and opens its Provider tab
- **THEN** the tab names `default.backup-provider` with its standing, and its conditions and
  catalog render locked; the page is not an error

#### Scenario: Users not fully readable

- **WHEN** the caller may not list ModuleInstances in every namespace and opens backup-provider's
  Provider tab
- **THEN** the Contracts fact reads "1 provided; use not fully readable", not an in-use count

#### Scenario: A contract nobody uses

- **WHEN** a registration provides a contract that no readable instance's render used, and the
  caller may list instances everywhere
- **THEN** the contract's row says no installed module uses it yet and offers no Installed link

#### Scenario: The standing said once

- **WHEN** a browser opens backup-provider's Provider tab on the F1 capture
- **THEN** its registration shows "accepted" and "active" once each, and no success reason beside
  them
