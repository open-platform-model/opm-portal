# web-ui Specification

## Purpose
Defines the portal's web pages: what the Platform, instance, package and object pages show, how
unreadable things render locked, how graphs, live updates and logs behave, and the policy and
assets every page is served with.

## Requirements

### Requirement: Pages read only through the read API

Every fact a page shows SHALL come from a read API document fetched through the read API's own
handler chain, in-process, with the caller's session, so a client holding the same identity can
read the same fact with the same filtering. Among this module's packages, `internal/ui` SHALL
import only `api/v1alpha1`, and a test SHALL fail when a file of `internal/ui` imports any other
package of the module. A problem document SHALL render as a region of the page: `forbidden` as
locked, `not_readable_by_portal` and `upstream_unavailable` as degraded with the code,
`not_found` as a not-found page with status `404`, `unauthenticated` as a sign-in page with
status `401`. Source: portal:D2:R1.

#### Scenario: The UI reads through the API

- **WHEN** a signed-in browser requests `/instances/default/podinfo`
- **THEN** the page is built from the `Instance`, `Graph` and `EventList` documents the read API
  serves for that session

#### Scenario: No session

- **WHEN** a browser without the session requests `/`
- **THEN** the response is `401` with a page telling the user to open a new launch link
- **AND** no access review is sent

#### Scenario: The import rule

- **WHEN** a file under `internal/ui` imports `internal/readmodel` or `internal/health`
- **THEN** the UI's test suite fails

#### Scenario: A condition's meaning

- **WHEN** a page shows a condition with a reason the portal explains
- **THEN** the meaning and next step it shows are the `meaning` and `nextStep` of that condition
  in the read API's document

### Requirement: The Platform page is the landing page

`/` SHALL show the Platform: an identity card (name, type, and the controller version labelled
"Controller"; the context, reader and Kubernetes version are in the header); a status block with
the applied state of `Ready` as a large state word in its tone, the time since that state
("since", from the deciding condition), the line of resolved catalogs, subscriptions and active
claims, and the reasons that need attention, each a count and a reason linking to its cause: per
refusal reason, the number of registrations refused with it, linking to the Providers tab filtered
to refused; per removal-blocked reason, the same, linking to it filtered to removal blocked; the
`Ready` reason (`BuildFailed`, `GenerateFailed`, `ContractCollisions`, `OverSubscribedContracts`,
`ComparablePredicates`) when `Ready` is not `True`, linking to its condition with the meaning,
next step and message; and `ContractsFulfilled`'s reason when it is `False`, as information
without a count, linking to the contracts note; an Installed card with one column of count rows
per health state and one per applied state, each row a badge and its count linking to Installed
filtered by it, and the numbers of instances, packages and of those that are providers; Providers
and Catalogs tabs with filters; and the recent events of the Platform and of each readable
registration, merged newest first, as a table with a resource filter whose options carry their
counts. A Providers row SHALL show the registration's name linking to its holder's Provider tab
when it has one, its catalog and version linking to the Catalog page, its provided contracts, its
holder ("Installed as") with kind and link, its acceptance and its activation as two separate
badges, and its refusal or removal-blocked reason and message. A removal-blocked registration
SHALL show as removal blocked while still accepted and active, never as refused. A Catalogs row
SHALL show the path linking to the Catalog page, version, source ("subscription", "contributed" or
"claimed only"), claimants linking to their holders' Provider tabs, and enablement as a badge. An
events row SHALL show type, reason, the object it is about as a link that filters the feed to that
object, the note with the repeat count and reporting controller, and the age. A Platform reporting
unfulfilled provider contracts SHALL show it as information, not as a failure. The page SHALL show
no Platform health and no platform graph. When the registrations the caller may read are not all
of them, or a list behind the Installed counts is forbidden, the page SHALL say so with the locked
style, whatever other reasons the status block shows, and SHALL never show a refusal count or an
active-claims count of zero for registrations it could not read. Source: portal:D4:R4/R7,
portal:D3:R8, portal:D17, portal:D9:R2/R4, portal:D19:R3/R5.

#### Scenario: The F1 platform

- **WHEN** a signed-in browser opens `/` on the F1 capture
- **THEN** its Catalogs tab shows the `opmodel.dev/catalogs/opm` subscription with its version
  and the backup catalog contributed by the accepted, active `backup-provider` claim, and its
  Providers tab shows the refused claim with its verdict Refused, its reason and its message

#### Scenario: Accepted and active apart

- **WHEN** a registration is accepted and not active
- **THEN** its row shows an "Accepted" badge and an "Inactive" badge, each on its own

#### Scenario: A count hands off

- **WHEN** the user follows the Degraded count on the Installed card
- **THEN** the browser opens `/installed?health=Degraded`

#### Scenario: Instances forbidden

- **WHEN** the caller may not list ModuleInstances
- **THEN** the Installed card shows the instance counts locked, not zero

#### Scenario: A refusal on the status block

- **WHEN** a signed-in browser opens `/` on the F1 capture
- **THEN** the status block reads "Applied" with a "since" time, and shows the count 1 with the
  reason `CatalogUnresolved` linking to `/?tab=providers&pstatus=refused#parts`

#### Scenario: Registrations forbidden on the status block

- **WHEN** the caller may get the Platform but may not list TransformerRegistrations
- **THEN** the status block says the refusals are locked and shows no refusal count, and its
  summary shows no "0 claims"

#### Scenario: Registrations forbidden while Ready is not True

- **WHEN** the caller may get the Platform but may not list TransformerRegistrations, and the
  Platform's `Ready` is `False`
- **THEN** the status block shows the `Ready` reason and, in the locked style, that the refusals
  are locked

#### Scenario: Removal blocked keeps acceptance and activation

- **WHEN** a registration's verdict is `RemovalBlocked`
- **THEN** its row shows "Accepted", "Active" and "Removal blocked" badges with its reason and
  message, and no "Refused" badge

#### Scenario: An event's resource filters the feed

- **WHEN** the user follows the resource link of an event about the TransformerRegistration
  `default.refused-claim-fixture`
- **THEN** the browser opens `/` with `eresource=registration:default.refused-claim-fixture` and
  the table holds only that registration's events

### Requirement: Applied and health are two badges, never one

Every instance and package SHALL show its applied state and its health as two separate badges,
neither derived from the other. Both badges SHALL share one square shape: an uppercase word in a
bordered rectangle coloured by its state's tone, with no prefix and no dot. Each badge SHALL carry
its axis ("Applied" or "Health") in its accessible name. Wherever an applied badge and a health
badge are shown together, each SHALL stand where visible text names its axis: a column head, a
group label, a card's heading or eyebrow, or a label beside the badge. A health stamp shown alone
on a per-object line, which carries one axis only, SHALL name it in its accessible name. `Ready=True` SHALL be shown as "Applied" and never as healthy, and the
applied badge SHALL read the controller's state words (Applied, Reconciling, Failed, Stalled,
Suspended, Managed externally), never "Ready" or "Not ready". `ManagedExternally` SHALL render in
a neutral style without an error colour. A partial health SHALL say partial and be marked visually
apart from a full health by more than colour; a health with an object refreshed by polling SHALL
say it is not live and show when it was evaluated. An enum value the UI does not know SHALL render
as "unknown", never as an error. The red family SHALL be used only for degraded, failed, stalled
and refused states, broken edges, Warning events and a lost stream; reasons, versions, kinds and
other decoration SHALL not use it. A condition SHALL be coloured by its `tone`, never by its
status alone. Source: portal:D3:R1/R5/R6/R8, portal:D2:R3, portal:D19:R3.

#### Scenario: The image-break sample

- **WHEN** the instance page of the image-break podinfo sample is rendered
- **THEN** it shows the Applied badge and the Degraded badge side by side

#### Scenario: A CLI-owned instance

- **WHEN** the instance list shows `web/web`, which the CLI applied
- **THEN** its applied badge reads "Managed externally" in the neutral style and its health badge
  is computed

#### Scenario: An unknown state

- **WHEN** a document carries a health state the UI does not know
- **THEN** the badge reads "unknown"

#### Scenario: An informational condition

- **WHEN** the Platform reports `ContractsFulfilled=False`
- **THEN** its condition is marked informational, not as a failure

#### Scenario: One shape, two named axes

- **WHEN** the Installed list on the F1 capture is rendered
- **THEN** every applied and health badge is the same square uppercase badge with no "APPLY"
  prefix and no dot
- **AND** each sits under the column head that names its axis, and its accessible name names the
  axis

#### Scenario: The details panel names both axes

- **WHEN** the details panel shows the ModuleInstance `default/podinfo`, whose node carries both
  axes
- **THEN** its applied and health badges each stand beside a visible "Applied" or "Health" label

#### Scenario: One axis on an object

- **WHEN** the details panel shows podinfo's `podinfo-podinfo` Deployment, which carries health
  only
- **THEN** its health badge stands beside a visible "Health" label and no applied badge is shown

#### Scenario: A failed apply keeps its word

- **WHEN** the F1 package `pkg/podinfo` is shown, whose applied state is Failed
- **THEN** its applied badge reads "Failed, retrying" in the degraded tone, not "Not ready"

#### Scenario: A partial health without colour

- **WHEN** a health is partial because an object could not be read
- **THEN** its badge says "partial" and is drawn with a hatched fill and a dashed border, apart
  from a full health of the same state

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

#### Scenario: Events of a caller who cannot list the Pods

- **WHEN** a caller who may get the instance and list events in `default` but may not list Pods
  opens `/instances/default/podinfo?tab=events` on the F1 capture
- **THEN** no ReplicaSet, Pod or Job is offered in the Resource choice or listed in the table, the
  Events tab carries no count, "All resources" says its count covers what could be read, and no
  count in the filters includes an event about one of them

#### Scenario: A long event note

- **WHEN** an event in the merged feed carries a note of 2,000 code points
- **THEN** the table shows the note as the API served it, capped, and the page clips it no further

#### Scenario: Events of both operator writers in-cluster

- **WHEN** an in-cluster browser opens the Events tab of an instance whose feed holds an event from
  `opm-controller` and an event with no controller that regards the instance, beside the kubelet's
  events about its Pods
- **THEN** neither of the first two shows a note, and the kubelet's events show theirs

### Requirement: Objects can be viewed as YAML without values

Selecting an object an inventory reaches SHALL show it as YAML from the read API's object
resource, without `spec.values`, managed fields or the last-applied annotation. Source:
portal:D8:R2/R3, portal:D2:R4.

#### Scenario: podinfo's Deployment as YAML

- **WHEN** the user selects the `podinfo-podinfo` Deployment's YAML view
- **THEN** the panel shows the Deployment's YAML with its pod template and without
  `managedFields` or the last-applied annotation

### Requirement: Graphs are server-rendered, accessible SVG

Each graph SHALL render server-side as SVG from the read API's laid-out graph document, with no
edge from an instance to a contract. Nodes SHALL carry CSS classes for kind, health and access, an
accessible name, and a title with their full label, and SHALL be reachable with the keyboard in
document order. A node SHALL show three lines: its kind, its name, and a status line (its health
reason or state, a ReplicaSet's desired count, a component's object count, a group's object and
kind counts, a registration's standing or a source's state). A node's fill and outline SHALL carry
its health, except that a TransformerRegistration node's fill and outline SHALL carry the claim's
standing (a source node's health is mapped from the source's state); a separate square mark on the
node SHALL carry its applied state, and is the only applied mark; nodes the cluster made below an
inventory object SHALL be drawn dashed. Edges SHALL be drawn as orthogonal
elbows through the document's route points, without arrowheads. A label too long for its node
SHALL keep the part that tells nodes apart. Activating a node SHALL show its detail panel and mark
the node as selected; activating a group node SHALL show it expanded. A graph SHALL open fitted to
its frame, SHALL pan with the pointer and SHALL zoom with a zoom slider, Fit and Ctrl/Cmd-wheel.
At a 360 px wide viewport the page SHALL NOT scroll sideways; the graph's frame SHALL take the
graph's extra width. Graphs SHALL appear on instance and package pages; the Platform page SHALL not draw one. The
instance page SHALL not draw the module node; the module's path and version stay on the page's
identity card. Source: portal:D4:R1/R3/R5, portal:D3:R1, portal:D17, portal:D19.

#### Scenario: Keyboard focus

- **WHEN** the user tabs into podinfo's graph
- **THEN** focus moves through the nodes in column order and each announces its kind, name and
  health

#### Scenario: Two axes on a node

- **WHEN** an instance node is applied and degraded
- **THEN** its fill and outline are drawn as degraded and its applied mark as applied

#### Scenario: A ReplicaSet's status line

- **WHEN** the image-break podinfo graph is rendered
- **THEN** the new ReplicaSet's node shows its kind, its name and its desired replica count, and is
  drawn dashed

#### Scenario: Elbow edges

- **WHEN** any instance graph is rendered
- **THEN** every edge path is made of horizontal and vertical segments and carries no arrowhead

#### Scenario: A graph at phone width

- **WHEN** a browser 360 px wide opens cert-manager's Graph tab
- **THEN** the page does not scroll sideways, and the graph's frame scrolls to reach the part of
  the graph beyond its width

### Requirement: Pages update live over one stream per tab

A tab SHALL hold one `EventSource` on the read API's stream across navigation, and each page
SHALL add the topics it needs and remove the ones it no longer needs through the topic-change
request. A change on a followed topic SHALL re-render every region of the open page tab that shows
it, and the summary cards, from one fetch of the page, so the regions of one refresh come from the
same moment. A refresh SHALL keep the open page tab, the `focus` parameter, the selected node, the
zoom, an open hover card and open groups; a page tab not shown SHALL be rendered fresh when opened.

#### Scenario: Navigating keeps the stream

- **WHEN** the user moves from `/` to `/instances/default/podinfo`
- **THEN** the same stream drops `platform` and adds `instance:default/podinfo` and
  `events:instance:default/podinfo`

#### Scenario: An image break on the open page

- **WHEN** podinfo's image is changed to a tag that does not exist while its page is open on the
  Graph tab
- **THEN** without a reload the graph shows the new ReplicaSet and the Pod waiting on the image,
  the Health card reads Degraded with a count for the waiting reason and the Applied card still
  reads Applied, and the Graph tab stays open with its zoom
- **AND** opening the Resources tab then lists the new ReplicaSet and the waiting Pod

### Requirement: Pages carry a strict policy and their own assets

UI pages SHALL be served with
`Content-Security-Policy: default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'`,
with no inline script, no inline style attribute and no `eval`. Script SHALL set positions and
sizes only through CSSOM properties or SVG attributes, never a `style` attribute. Untrusted text
(messages, notes, labels, log lines) SHALL be rendered only through `html/template` or as text
nodes, and a test SHALL fail when a page renders such text as markup or when the UI's code converts
a string to a trusted template type or assigns markup in a page script. Scripts, styles and fonts
SHALL be served from the binary, vendored at pinned versions whose SHA-256 a test checks. Browser
storage SHALL hold only the theme choice and each list view's filter query, every access guarded
so a throwing storage changes nothing else. Pages SHALL follow the theme chosen in the browser, or
the OS light or dark preference under System, and SHALL not scroll horizontally at a 360 px wide
viewport. Source: portal:D14:R1/R4.

#### Scenario: The page policy

- **WHEN** any UI page is served
- **THEN** it carries that policy, and no element in it has an inline `style` attribute, an
  inline script or an event-handler attribute

#### Scenario: A tampered asset

- **WHEN** a vendored file differs from its recorded checksum
- **THEN** the UI's test suite fails

#### Scenario: Hostile cluster text

- **WHEN** a condition message, history entry, event note or annotation holds
  `<script>alert(1)</script>`
- **THEN** every page and fragment shows it as text, escaped

#### Scenario: What the browser stores

- **WHEN** a session has visited every page and used every filter and the theme menu
- **THEN** browser storage holds only the theme key and one filter query per list view, and no
  document field, username or token

### Requirement: In-cluster pages show no operator text

Pages served over an `in-cluster` read API SHALL show no condition message (the conditions of a
registration included), reconcile message, history message, registration message or note of an
event the operator reported, because the read API does not serve them; they SHALL still show every
reason, state, history outcome, provider standing and the portal's meaning and next step, and
SHALL show the kubelet's and other controllers' event notes and the health messages of rendered
workloads as local mode does. Source: owner answer to portal:OQ8 (portal:D8:R5); its scope is the
supervisor ruling recorded under portal:D8.

#### Scenario: The broken podinfo page in-cluster

- **WHEN** the instance page of the image-break sample is rendered over an in-cluster read API
- **THEN** it shows the applied and health badges, each condition's reason, meaning and next
  step, and no message text the operator wrote

#### Scenario: A Pod's events in-cluster

- **WHEN** the events of a podinfo Pod are rendered over an in-cluster read API
- **THEN** the kubelet's and the scheduler's notes are shown as they wrote them

#### Scenario: Attempt dots and the Provider tab in-cluster

- **WHEN** backup-provider's page is rendered over an in-cluster read API
- **THEN** its attempt dots keep their outcomes, its registration conditions show type, status,
  reason and meaning, and no message text the operator wrote appears

### Requirement: An expired session stops the page's stream

When the stream ends with an `expired` event, the page SHALL close its `EventSource`, so it does
not reconnect, and SHALL show in the live mark that the session expired and the page must be
reloaded, in the live mark's capitalised wording. The page SHALL keep showing what it last
rendered, and SHALL show a visible banner in the sticky header that says the session expired,
states the time of the data the page shows in the form of the `time` partial (relative
text, the ISO time in `datetime`, the absolute time in UTC in `title`), and carries a Reload link
that loads the page afresh. The banner SHALL be neither a live region nor given a role: the live
mark SHALL stay the one announcer.

#### Scenario: The session expires while a page is open

- **WHEN** the session of an open page expires
- **THEN** the live mark reads "Session expired, reload"
- **AND** the banner reads "Session expired. Showing data read" followed by the age of the data on
  screen, the last message the stream delivered before the session expired, as a `time` element (relative text, with the instant in
  `datetime` and in UTC in `title`), with a Reload link
- **AND** the page makes no further stream request

#### Scenario: Reload after the session expired

- **WHEN** the user activates the banner's Reload link
- **THEN** the browser loads the current address as a new page, with a new stream

### Requirement: The header names the cluster, the reader and the version

Every page SHALL carry a sticky header with the main navigation (Platform, Installed), the
cluster the portal reads (the kubeconfig context and, when the `Cluster` document carries a
cluster entry name that differs from the context name, that name beside it with the word
"cluster"; or "in cluster"), "reading as" with the caller's username, the Kubernetes version, the
live mark and the theme menu, the facts read from the read API's `Cluster` document. The header
SHALL be the one place these facts show. A version the document does not carry SHALL read
"unknown". When the `Cluster` document cannot be fetched, the header SHALL still render its
navigation, live mark and theme menu, with the cluster, reader and version reading "unknown" in
the degraded style, and no cluster entry name. The header SHALL slim after the page scrolls 48 px
and SHALL not animate under `prefers-reduced-motion`. The cluster entry name SHALL come from the
`Cluster` document alone: the header SHALL read nothing else for it. Source: portal:D18,
portal:D17:R1.

#### Scenario: The F1 header

- **WHEN** a signed-in browser opens any page on the F1 capture in local mode
- **THEN** the header shows Platform and Installed, the context name, "reading as" with the
  kubeconfig's username, the Kubernetes version and the live mark

#### Scenario: A cluster entry that differs from the context

- **WHEN** the `Cluster` document carries the context `admin@prod-east` and the cluster entry
  `prod-east`
- **THEN** the cluster pill shows `admin@prod-east` and, beside it, the word "cluster" with
  `prod-east`
- **AND** the root element's `data-context` still carries `admin@prod-east` alone

#### Scenario: A cluster entry that equals the context

- **WHEN** the `Cluster` document carries the same name for the context and the cluster entry
- **THEN** the pill shows that name once, with no "cluster" word

#### Scenario: The longest entry name at 360 px

- **WHEN** the context and the cluster entry are each 90 characters with no space, on a 360 px
  wide screen
- **THEN** each name is cut to one line with an ellipsis inside the pill, the full values are in
  the pill's tip and in its text, the header is as tall as with short names (within 2 px), with
  and without the banner, and the page does not scroll sideways

#### Scenario: Version not read

- **WHEN** the `Cluster` document carries no `kubernetesVersion`
- **THEN** the header's version reads "unknown", not empty

#### Scenario: The cluster document fails

- **WHEN** the in-process fetch of the `Cluster` document fails
- **THEN** the page renders with its navigation and theme menu, and the header's cluster, reader
  and version read "unknown" in the degraded style, with no cluster entry name

#### Scenario: In a Pod

- **WHEN** the portal runs local mode in a Pod with in-cluster credentials
- **THEN** the header reads "in cluster" in place of a context name, with no cluster entry name

### Requirement: The theme is chosen per browser

The header's theme menu SHALL offer Light, Dark and System. System SHALL follow the operating
system's preference; Light and Dark SHALL override it. The choice SHALL be kept in the browser's
`localStorage` and applied before first paint by a script served from `/static` and loaded
without `defer` in the page head, so a page never paints in the other theme first. With storage
unavailable or throwing, pages SHALL render in System. Source: portal:D14:R4/R5.

#### Scenario: Dark chosen on a light system

- **WHEN** a browser that chose Dark loads any page on a system set to light
- **THEN** the first paint is in the dark theme

#### Scenario: Storage blocked

- **WHEN** `localStorage` throws on access
- **THEN** every page renders, follows the system preference, and the theme menu still switches
  the theme for the open page

### Requirement: Installed lists instances and packages together

`/installed` SHALL list the ModuleInstances and ModulePackages the caller may read in one table:
name and namespace, a kind chip with its icon (instance and package drawn apart), a provider badge
reading "Provider" when the item holds a TransformerRegistration, coloured by its standing and
opening, on hover, on keyboard focus and on a tap, a tooltip that names the registration and its
standing in words, that Escape closes and that the pointer can enter, the module path with its version on a line below, or the package's source as
`<Kind> <namespace>/<name>` with its source revision's short digest on a line below or "no
revision recorded" when the controller recorded none, and its path only when it is neither empty
nor `.`, the applied badge, the health badge, the object count and the owner (`controller` or
`cli` for an instance, `controller` for a package). Each row SHALL be a link to the item's page,
and a row whose health is Degraded SHALL be tinted apart from the others. When filters match
nothing, the list SHALL say so and offer a button that clears them. A kind the caller may not list
SHALL render locked for that kind only, with no count, while the other kind's rows show.
`/instances` and `/packages` SHALL redirect with `308` to `/installed` with `kind=instance` or
`kind=package`, keeping a `namespace` parameter. Source: portal:D17:R1, portal:D7:R2,
portal:D15:R2/R4.

#### Scenario: The F1 Installed list

- **WHEN** a signed-in browser opens `/installed` on the F1 capture
- **THEN** it lists the instances `cert-manager/cert-manager`, `default/podinfo`,
  `default/backup-provider`, `default/backup-consumer` and `web/web` and the package
  `pkg/podinfo`, with `web/web` owned by `cli` and `default/backup-provider` carrying an accepted,
  active provider badge

#### Scenario: Packages forbidden

- **WHEN** the caller may list ModuleInstances but not ModulePackages
- **THEN** the instances are listed and the packages show as a locked group with no count

#### Scenario: An old link

- **WHEN** a browser requests `/instances?namespace=team-a`
- **THEN** the response is `308` to `/installed?kind=instance&namespace=team-a`

#### Scenario: The provider tooltip by keyboard

- **WHEN** a keyboard user tabs to the provider badge of `default/backup-provider`
- **THEN** a tooltip shows that registration `default.backup-provider` is accepted and active,
  and the standing is also in the badge's text for assistive technology

#### Scenario: The provider tooltip by tap and Escape

- **WHEN** a touch user taps the provider badge of `default/backup-provider`, and later taps it
  again, or a keyboard user opens the tooltip and presses Escape
- **THEN** the first tap opens the tooltip without leaving the page, and the second tap or Escape
  closes it while focus stays on the badge

#### Scenario: A package with no recorded revision

- **WHEN** the Installed list shows the F1 package `pkg/podinfo`
- **THEN** its source cell reads its source kind with `pkg/` and the source name on one line and
  "no revision recorded" on the next, and shows no "path ."

#### Scenario: A whole row opens the item

- **WHEN** the user clicks the Owner cell of the `default/podinfo` row
- **THEN** the browser opens `/instances/default/podinfo`

### Requirement: List filters live in the URL and are remembered per browser

The Installed list, the Platform page's Providers and Catalogs tabs and its events, and the Events
and Resources tabs of instance and package pages SHALL filter from their URL query, server-side,
so a filtered view is a plain link that works without script. Installed SHALL offer search, kind,
provider, uses, namespace, health, applied, owner and module; each active filter SHALL show as a
chip whose link removes it. An enumerated filter value the view does not know SHALL be ignored
with a note naming it, and never stored. Only the list views SHALL remember their filters:
Installed and the Providers and Catalogs tabs, each on its own; the Platform events and instance
and package pages SHALL not. `tab` and `focus` are not filters: they SHALL never be stored and
SHALL not count as a filter in the URL. When the URL carries any of a remembered view's filter
parameters, the view SHALL show exactly those. When it carries none and the browser holds valid
filters for that view, the view SHALL open with them and the URL SHALL show them, without painting
the unfiltered view first, on a full load and on a boosted navigation alike; no other request
SHALL be rewritten. A stored query SHALL be checked against the view's parameters and values, and
what does not fit SHALL be dropped. Stored filters SHALL be kept per kubeconfig context (or the
in-cluster source), so filters stored while reading one context are never restored while reading
another. After rendering, a remembered view SHALL store the filters it shows and nothing else. The `uses` filter SHALL match an instance's render contracts, SHALL leave
packages out and SHALL say that packages do not record what they use. Source:
portal:D14:R1/R2/R3, portal:D16:R2.

#### Scenario: A link wins over the remembered filters

- **WHEN** a browser that remembered `health=Healthy` for Installed opens
  `/installed?health=Degraded`
- **THEN** the list shows only Degraded items, one chip reads Degraded, and Installed now
  remembers `health=Degraded`

#### Scenario: Remembered filters restored

- **WHEN** a browser that remembered `namespace=default` for Installed follows the Installed
  link in the header
- **THEN** the page shows `/installed?namespace=default` and was never painted unfiltered

#### Scenario: Uses leaves packages out

- **WHEN** a browser opens `/installed?uses=opmodel.dev/catalogs/opm/traits/backup@v1alpha1` on
  the F1 capture
- **THEN** the list holds `default/backup-consumer` and no package, and says packages are not
  included because they do not record what they use

#### Scenario: Without script

- **WHEN** a browser with script disabled opens `/installed?kind=package`
- **THEN** the list holds only packages

#### Scenario: An unknown filter value

- **WHEN** a browser opens `/installed?health=Bogus`
- **THEN** the list is unfiltered by health, a note says the filter `health=Bogus` was ignored,
  and nothing about it is stored

#### Scenario: A stale stored query

- **WHEN** the stored Installed query holds `health=Bogus&namespace=default` and the browser opens
  `/installed`
- **THEN** the page shows `/installed?namespace=default`

#### Scenario: Another context

- **WHEN** a browser remembered `namespace=team-a` for Installed while the portal read context
  `prod`, and the portal is restarted on the same address with `--context staging`
- **THEN** `/installed` opens unfiltered

#### Scenario: A tab is not a filter

- **WHEN** a browser that remembered `pstatus=refused` for the Providers tab opens `/?tab=providers`
- **THEN** the page shows `/?tab=providers&pstatus=refused`

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
verdict the portal composed. When the controller wrote no message and the reason has no known
meaning, the summary SHALL be a fixed sentence for the recorded standing, so no standing shows its
word over an empty summary, and a standing the page does not know SHALL read Unknown. A non-success
reason SHALL link to the Provider tab. Below 520 px wide, the identity card's facts SHALL take one
column.

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

#### Scenario: A pending claim with no controller text

- **WHEN** a registration is Pending and its conditions carry no message and no known reason
- **THEN** the Provider card reads Pending with the summary "The controller has not decided on this
  claim yet." in both modes

#### Scenario: A claim of unknown standing

- **WHEN** the Platform records a verdict the page does not know for a held registration
- **THEN** the Provider card reads Unknown and says the portal does not know the claim's standing

#### Scenario: A blocked removal

- **WHEN** a held registration's removal is blocked while modules still use what it provides, and
  the conditions carry no message
- **THEN** the Provider card reads Removal blocked and says the claim is accepted and active and
  its removal waits until no module uses what it provides

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

### Requirement: Graphs can be explored without leaving the page

A graph SHALL show a card beside a node on pointer hover and on keyboard focus (kind, namespace and
name, health and applied marks, health reason and message, its origin, for a child of the cluster
the object it was created from, access, and a hint to click for details or to expand), positioned
without any `style` attribute. Activating a node SHALL select it through a `node` parameter, which
shows its panel and dims nothing. A `focus` parameter SHALL spotlight its node: select it, scroll
it into the centre of the view and dim every node and edge outside its whole upstream and
downstream chain; pointer hover and keyboard focus SHALL dim the same chain while they last. A
"Clear selection" control SHALL remove the selection and the spotlight. The graph SHALL open in
full screen and back, refitting to the space it has. It SHALL zoom with a zoom slider from 30 % to
200 %, Fit (marked pressed while the view is fitted) and Ctrl/Cmd-wheel anchored at the pointer,
and pan with the pointer; a fit SHALL use both the frame's width and height, and the frame's
height SHALL follow the fitted graph. A group node SHALL expand in place through the graph's
`expand` parameter and fit the view to the group's frame at no less than 60 %, and an "Overview"
control and the Escape key SHALL collapse it and fit the whole graph again. Full screen SHALL cover
the page's window and SHALL stay on while groups open and close. Escape SHALL step back one thing
per press: an open card, a selection, an open group, full screen. Opening or closing a group, and
the group controls, SHALL keep the page's scroll position. Wheel zoom SHALL not animate, and motion
SHALL stop under the reduced-motion preference. Source: portal:D4:R5, portal:D19.

#### Scenario: cert-manager's RBAC group

- **WHEN** the user expands cert-manager's RBAC group
- **THEN** the view fits the group's frame, and "Overview" returns to the collapsed, fitted graph

#### Scenario: Keyboard card

- **WHEN** the user tabs onto podinfo's Deployment node
- **THEN** its card shows kind, name, health and origin, and is announced with the node

#### Scenario: A click does not dim

- **WHEN** the user clicks podinfo's Service node
- **THEN** the details panel shows the Service, the address carries `node` and no `focus`, and no
  node is dimmed

#### Scenario: Hand-off spotlight

- **WHEN** the user follows the image-break Pod's row from the Resources tab
- **THEN** the Graph tab opens with the Pod centred in the view and its ReplicaSet, Deployment,
  component and instance lit, and podinfo's Service dimmed

#### Scenario: Escape steps back

- **WHEN** the user has opened the RBAC group in full screen and selected one of its objects, and
  presses Escape three times
- **THEN** the first press clears the selection, the second collapses the group, and the third
  leaves full screen

#### Scenario: Full screen survives a group

- **WHEN** the user opens cert-manager's graph in full screen and opens, then collapses, its RBAC
  group
- **THEN** the graph is still in full screen

#### Scenario: Wheel zoom at the pointer

- **WHEN** the user zooms with Ctrl and the wheel over a node
- **THEN** that node stays under the pointer

### Requirement: Providers are shown whatever their kind

An instance or a package whose inventory holds a TransformerRegistration SHALL show a Provider
pill, the Provider card and a Provider tab. The standing shown SHALL be the registration's own
accepted, active, verdict and reason, never one the portal computed; a package whose claim the
controller refused SHALL show as a refused provider with the controller's reason and, in local
mode, its message. When the registration's `spec.providerRef` and the holder disagree, both SHALL
be shown. Source: portal:D15:R1/R2/R3, portal:D4:R2.

#### Scenario: The F1 provider

- **WHEN** a browser opens `/instances/default/backup-provider` on the F1 capture
- **THEN** it shows the Provider pill, a Provider card reading Active, and a Provider tab for
  `default.backup-provider`

#### Scenario: A package holding a refused claim

- **WHEN** a package's inventory holds a registration the controller refused with
  `ProviderMismatch`
- **THEN** the package shows a Provider pill and a Provider card reading Refused with reason
  `ProviderMismatch`, and no text saying the refusal is wrong

#### Scenario: Registrations forbidden

- **WHEN** the caller may read `default/backup-provider` but may not list TransformerRegistrations
- **THEN** the Provider card names `default.backup-provider` with a locked standing

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

The Contracts fact SHALL have a text for every standing. A claim that is accepted but not active,
or pending, SHALL say how many contracts it lists and that none is provided yet. A claim of unknown
standing SHALL say that it is not known. A claim whose removal is blocked SHALL give its counts and
say the removal is blocked.

Below 760 px wide, each row of the table SHALL be a card. For each contract the table SHALL list the first three users and the count of the rest. It SHALL
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

#### Scenario: A claim that provides nothing yet

- **WHEN** a held registration lists two contracts and is Pending, or accepted and not active
- **THEN** the Contracts fact says it lists two and that none is provided yet, and does not read
  "2 provided, 0 in use"

#### Scenario: A blocked removal still provides

- **WHEN** a held registration's removal is blocked and one of its contracts is in use
- **THEN** the Contracts fact gives "1 in use" and says the removal is blocked

#### Scenario: A claim of unknown standing

- **WHEN** the verdict of a held registration is one the page does not know
- **THEN** the Contracts fact says it is not known

#### Scenario: The Provider tab at 360 px

- **WHEN** a browser 360 px wide opens backup-provider's Provider tab
- **THEN** the page does not scroll sideways, the facts take one column, and each row of "What it
  provides" is a card

#### Scenario: The standing said once

- **WHEN** a browser opens backup-provider's Provider tab on the F1 capture
- **THEN** its registration shows "accepted" and "active" once each, and no success reason beside
  them

### Requirement: The Catalog page shows one catalog as the cluster records it

`/catalog?path=<catalog>` SHALL show any catalog the Platform subscribes to, holds in its resolved
registry, or a readable registration claims: as its headline, the catalog's short name (its last
path segment without the major version, or a longer form when two catalogs share it), with the
full path and version on a line below; its origin (subscribed, from a provider, or claimed only)
as a pill; its source ("Platform subscription", or the contributing registration, or for a
claimed-only catalog the claiming registrations, each linking to its holder's Provider tab when it
has one holder); its enablement, reading "no: not in the registry" for a catalog the registry does
not hold; whether it is resolved, as a state block with the state word, a summary of what the
registry records, and for an unresolved catalog the time since it was refused (the earliest time
a claiming registration's `Ready` condition turned `False`, or the Platform's `Ready` for a
subscription) and each reason (the Platform's `Ready` reason without a count, or the claiming
registrations' refusal reasons with their count)
linking to the Events tab filtered by that reason; the Platform's `ContractsFulfilled` labelled as
platform-wide; the registrations that claim it with their holders and standing; and the recent
events of the Platform and its claimants as a table, filterable by type and reason, with how many
are shown of how many. The breadcrumb SHALL end with the short name. It SHALL show no definition,
description, documentation link or transformer, and SHALL say these are not recorded. When the
caller may not read the Platform, the page SHALL render locked with status `403`, saying nothing
about whether the catalog exists. A path that a readable Platform and the readable registrations
do not name SHALL render not found with status `404`. Source: portal:D17:R3, portal:D3:R8,
portal:D7:R1, portal:D9:R2, portal:D19:R5.

#### Scenario: The contributed backup catalog

- **WHEN** a browser opens `/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0` on the
  F1 capture
- **THEN** it shows the headline `backup`, version `0.1.0`, origin from a provider, the source
  `default.backup-provider` linking to the Provider tab of `default/backup-provider`, and resolved

#### Scenario: A claimed catalog that does not resolve

- **WHEN** a browser opens the Catalog page of the catalog `default.refused-claim-fixture` claims
- **THEN** it shows the catalog as claimed only and not resolved, with the claim's reason
  `CatalogUnresolved`

#### Scenario: An unknown catalog

- **WHEN** a browser opens `/catalog?path=example.com/nothing@v0`
- **THEN** the response is `404` with the not-found page

#### Scenario: Platform forbidden

- **WHEN** a caller who may not get the Platform opens
  `/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0` and
  `/catalog?path=example.com/nothing@v0`
- **THEN** both render the same locked page with status `403`

#### Scenario: A refusal reason opens the events

- **WHEN** the user follows the `CatalogUnresolved` reason on the Resolved block of the catalog
  `default.refused-claim-fixture` claims
- **THEN** the Events tab opens filtered to reason `CatalogUnresolved`, holding the claim's
  warning, with its source reading the claiming registration's name, its enablement "no: not in
  the registry", and a "refused since" time on the block

#### Scenario: Events filtered by type

- **WHEN** a browser opens the Events tab of a catalog with `type=Warning`
- **THEN** the table holds only Warning events and says how many it shows of all the events

#### Scenario: A Type choice keeps the reason

- **WHEN** a browser opens a catalog's Events tab with `reason=CatalogUnresolved` and then picks
  type Warning
- **THEN** the address carries both `reason=CatalogUnresolved` and `type=Warning`

#### Scenario: Claimants forbidden

- **WHEN** the caller may get the Platform but may not list TransformerRegistrations and opens a
  subscribed catalog
- **THEN** the claims render locked and the Resolved block shows the Platform's own resolution,
  with no refusal count

### Requirement: Pages use the words a platform team uses

Page text SHALL name kinds in the words a platform team uses: "Providers" for
TransformerRegistrations and the instances and packages that hold them, "Installed" for
ModuleInstances and ModulePackages, and "controller" for the opm-operator. The read API's
`operatorVersion` SHALL be labelled "Controller", and its owner value `operator` SHALL read
"controller". The Installed card SHALL count the items that hold a registration as providers. The
YAML view shows objects as the cluster serves them and is exempt. Source: portal:D17:R2.

#### Scenario: The owner column

- **WHEN** the Installed list shows an instance whose owner is `operator` in the read API
- **THEN** its owner cell reads "controller"

#### Scenario: The providers line

- **WHEN** the Platform page's Installed card counts the F1 capture
- **THEN** its closing line says how many of the installed items are providers

### Requirement: Locked things render locked on every page

Every object, row, node, list, card or section the read API reports as forbidden, not readable or
withheld SHALL render locked: a distinct locked style, a lock mark and text saying the user may
not read it or the portal cannot, showing only what the API document carries, with no link to a
detail it cannot open. A forbidden instance or package list SHALL render as a locked group of the
Installed list with no count. A Secret in an inventory SHALL show that Secret data is never read,
with no YAML action. Source: portal:D5:R7, portal:D7:R3.

#### Scenario: A forbidden kind inside an instance

- **WHEN** the caller may not read Services and opens `/instances/default/podinfo`
- **THEN** podinfo's Service row and its graph node render locked, and the Deployment renders
  normally

#### Scenario: A forbidden list in Installed

- **WHEN** the caller may not list ModuleInstances and opens `/installed`
- **THEN** the instances show as a locked group with no count

### Requirement: Every F1 page holds its width at a phone viewport

A browser test SHALL open every page of the F1 capture, the not-found page included, at a 360 CSS px
wide viewport in Chromium, Firefox and WebKit, and SHALL fail when a page's scroll width exceeds
360 px. The test SHALL prove that it can fail. A change that adds a page SHALL add it to the test.
The test SHALL also fill the details panel by selecting a graph node and measure again.

#### Scenario: A page fits the phone viewport

- **WHEN** the test opens a page of the F1 capture at 360 px wide, in any of the three browsers
- **THEN** the page's scroll width is 360 px or less and the test passes

#### Scenario: A page scrolls sideways

- **WHEN** a page has content that makes its scroll width exceed 360 px
- **THEN** the test fails and names the page, the browser and the elements that reach past the
  viewport

#### Scenario: The check can fail

- **WHEN** the test adds a 500 px wide element to a page at the 360 px viewport
- **THEN** it measures a scroll width above 360 px in each browser, or the test fails

#### Scenario: A page that needs another set-up

- **WHEN** a page needs another capture or authorizer than the F1 site (the locked page, the
  broken-rollout page and in-cluster mode)
- **THEN** the test does not open it, and a later change adds it with its own set-up

#### Scenario: The test runs where the other browser tests run

- **WHEN** `task test:browser` runs, locally or in the nightly `E2E` workflow
- **THEN** it includes `TestBrowserPhone`

#### Scenario: A selected node on a phone

- **WHEN** the user activates the `podinfo-podinfo` node from the keyboard on
  `/instances/default/podinfo` at 360 px wide
- **THEN** the details panel fills and the page's scroll width is at most 360 px

### Requirement: Muted text meets WCAG 2.2 AA contrast in both themes

Text drawn with the muted ink token SHALL have a contrast ratio of at least 4.5:1 against each
surface token it is drawn on (page, card, secondary card, field, the neutral fill and the degraded
fill), in the light and the dark theme. A canvas value that fails SHALL change to one that passes,
never the layout. The ratio is computed from the token hex values in `portal.css` with the WCAG
relative luminance formula and is not rounded. Source: portal:D19:R6, portal:D19:R7.

#### Scenario: Muted text on the page and on cards

- **WHEN** the light theme draws muted text on the page background, on a card, on a secondary card,
  on a field, or on the neutral or degraded fill
- **THEN** each pair has a ratio of at least 4.5:1

#### Scenario: A token pair falls below its floor

- **WHEN** a change lowers the ratio of a listed token pair under its floor in either theme
- **THEN** the contrast test fails and names the pair, the theme and the measured ratio

### Requirement: Text inputs and selects have a boundary of at least 3:1

The border of a text input or select SHALL have a contrast ratio of at least 3:1 against the fill
of the field and against each surface a field is drawn on, in the light and the dark theme, whether
the field is empty or filled. Borders that only decorate a control that has a text label
are not held to this ratio. Source: portal:D19:R6.

#### Scenario: A filter field on its panel

- **WHEN** a filter input or select is drawn on the filter panel, in either theme
- **THEN** its border has a ratio of at least 3:1 against the field fill and against the panel

#### Scenario: A filled filter field

- **WHEN** a filter input has a value or a select has a chosen option, in either theme
- **THEN** its accent border has a ratio of at least 3:1 against its accent field fill and against
  the panel

#### Scenario: The two dark blocks agree

- **WHEN** the dark theme tokens are set both under the OS preference and under the stored Dark
  choice
- **THEN** every token in the contrast test's pair list, and every tone and kind token, has the
  same value in both blocks, or the test fails

#### Scenario: A token the test cannot read

- **WHEN** a listed token holds a value that is not a six-digit hex colour
- **THEN** the contrast test fails and names the token instead of skipping the pair

### Requirement: Pages share one flat look

Every page SHALL follow the reviewed canvas's look. The page background SHALL be one flat colour
with no grid or glow. Panels and cards SHALL be flat, with a 1 px border, no shadow and no corner
mark. Section headings SHALL be plain 20 px text with no marker. The page column and the footer
SHALL be at most 1840 px wide. The graph pane's own grid is part of the graph's drawing, not the
page background, and is not covered by this requirement. Source: portal:D19:R1.

#### Scenario: A flat page

- **WHEN** a signed-in browser opens `/` on the F1 capture
- **THEN** the page background has no image, no panel or card casts a shadow or carries a corner
  mark, and no `h2` has a marker

#### Scenario: A wide screen

- **WHEN** `/installed` is shown in a 1920 px wide viewport
- **THEN** the page column is 1840 px wide, with the footer at the same width

### Requirement: Summary rows share their width

A row of summary cards SHALL share its width in equal columns and SHALL stack at narrow widths.
Source: portal:D19:R1, portal:D17.

#### Scenario: Summary cards on a phone

- **WHEN** `/`, `/installed`, `/instances/default/podinfo` and a Catalog page are shown in a
  360 px wide viewport
- **THEN** the summary cards stack in one column

### Requirement: Pages share one type scale and link colour

Body text SHALL be 16 px, a page title at most 40 px, and mono text 13 px. Kickers, labels, table
heads and fact labels SHALL use the page's sans face, and fact labels SHALL be in sentence case.
Text links in the page body SHALL be drawn in the accent colour; the header, tabs, chips, badges,
state-block reason links and graph nodes keep their own colours. Source: portal:D19, portal:D17.

#### Scenario: Header links keep their colours

- **WHEN** any page is shown
- **THEN** links in the page body are drawn in the accent colour and the header's brand,
  navigation and connection marks keep the header's own colours

### Requirement: Every colour is a token in both themes

Every colour SHALL be a token on `:root`, redefined for dark mode under the `prefers-color-scheme`
block guarded by `:root:not([data-theme="light"])` and again under `:root[data-theme="dark"]`.
Each status tone SHALL have a border, a text ink, a background and a tint token. The Instance and
Package kind chips SHALL take their colours from their own tokens, so that both read in light and
dark. Source: portal:D19:R2.

#### Scenario: The Instance chip in dark mode

- **WHEN** a browser that chose Dark opens `/installed` on the F1 capture
- **THEN** each Instance chip is a slate fill with light text, not a light fill with dark text

#### Scenario: Tone tokens in both themes

- **WHEN** the stylesheet is checked
- **THEN** every tone's border, ink, background and tint token is defined on `:root` and in both
  dark blocks, and the contrast test's dark-block agreement check covers each of them

### Requirement: Tone ink and link colours meet the same contrast floor

Text in a tone's ink SHALL have a contrast ratio of at least 4.5:1 on that tone's background and
tint, in the light and the dark theme. Links in the accent colours on the page and surface colours,
and the muted and secondary ink on every tint, SHALL meet the same ratio. The ratio is computed as
for muted text. Source: portal:D19:R7.

#### Scenario: Tone ink on its background and tint

- **WHEN** the contrast test checks each tone's ink, including the locked ink, on its background
  and its tint, in the light and the dark theme
- **THEN** every pair is at least 4.5:1, and no rule draws text in a tone's border token on its fill

#### Scenario: Links and secondary ink

- **WHEN** the contrast test checks the accent and deep accent on the page and surface colours,
  and the muted and secondary ink on every tint, in both themes
- **THEN** every pair is at least 4.5:1

### Requirement: Tabs are underlined

Every tab strip (the Platform's Providers and Catalogs, the Catalog page's tabs, and the instance
and package pages' tabs) SHALL be drawn as underline tabs: no box around a tab, the current tab
marked by a 3 px accent underline and the ink colour, the others muted. A tab SHALL be a link
carrying `tab=` that works without script. Source: portal:D19, portal:D7:R2/R3.

#### Scenario: A tab without script

- **WHEN** a browser with script off follows podinfo's Resources tab
- **THEN** the page opens on that tab, drawn with the 3 px accent underline

### Requirement: Tabs carry their counts

A tab SHALL show, after its label, the unfiltered number of items it lists when the page already
holds that number and it covers everything the tab lists: Providers, Catalogs, Claims, Resources
(every row the table lists: inventory objects, their runtime children, and a package's source row)
and Events on instance and package pages (the folded lines of the feed the tab opens with).
Source: portal:D19:R4, portal:D7:R2/R3.

#### Scenario: The F1 Platform tabs

- **WHEN** a signed-in browser opens `/` on the F1 capture
- **THEN** the tabs read "Providers 2" and "Catalogs 3", with Providers underlined as current

#### Scenario: podinfo's tabs

- **WHEN** a signed-in browser opens `/instances/default/podinfo` on the F1 capture
- **THEN** the Resources tab carries the number of its inventory objects and runtime children,
  the Events tab the number of folded lines its feed opens with, and Graph, Logs and YAML carry
  none

#### Scenario: A package's Resources count includes its source row

- **WHEN** a signed-in browser opens `/packages/pkg/podinfo` on the F1 capture, whose inventory
  is empty
- **THEN** the Resources tab reads "Resources 1", the one row its table lists being the package's
  source, and the count stays 1 under a `reason` filter that hides the row

#### Scenario: A filter does not change a count

- **WHEN** a browser opens podinfo's page with `tab=events&type=Warning`
- **THEN** the Events tab's count is the unfiltered number, and the list shows only Warning
  events

#### Scenario: The Catalog page's tabs

- **WHEN** a browser opens `/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0` on the
  F1 capture
- **THEN** the Claims tab carries the number of registrations that claim the catalog and the
  Events tab carries none

### Requirement: A tab count is never partial, adds no read and follows the page live

A tab whose list the caller may not read in full, or whose source is a problem, SHALL show no
count, never a partial one. Showing a count SHALL not add a read. A count SHALL update when the
page's regions refresh live, without reloading the page, and a count that has become unreadable
SHALL be removed by that refresh. A tab that can carry a count SHALL always draw its count element,
empty when it shows none, so the refresh can fill or empty it. Source: portal:D19:R4,
portal:D7:R2/R3.

#### Scenario: Registrations not all readable

- **WHEN** the caller may read the Platform but not list TransformerRegistrations
- **THEN** neither the Providers nor the Catalogs tab shows a count, since the Catalogs tab lists
  catalogs that only a registration claims

#### Scenario: Platform forbidden

- **WHEN** the caller may not read the Platform
- **THEN** neither the Providers nor the Catalogs tab shows a count

#### Scenario: A count follows the page live

- **WHEN** podinfo's page is open on its Graph tab and a new event about the instance arrives
- **THEN** the Events tab's count updates without a reload, and the tabs keep working as links

#### Scenario: A count goes when its source becomes a problem

- **WHEN** podinfo's page shows an Events count and a live refresh finds the events unreadable
- **THEN** the Events tab shows no count, and a later refresh that finds the events readable shows
  the count again

### Requirement: A region a tab opens has no visible heading

A region a tab opens SHALL not show a visible heading of its own, since the selected tab names it,
and SHALL keep that heading for assistive technology. Source: portal:D19.

#### Scenario: A region named by its tab

- **WHEN** podinfo's Resources tab is open
- **THEN** no visible heading repeats "Resources" above the list, and the region's accessible name
  still starts with "Resources"

### Requirement: Selecting a graph node fills the details panel

With script on, activating a graph node by pointer or keyboard SHALL load that node's details into
the details panel in place, showing its name, its kind, the axes it carries and its links, and
SHALL mark the node as selected. Without script, a node SHALL stay a link that opens the page with
the node selected and its details rendered by the server. Source: portal:D4, portal:D19.

#### Scenario: Selecting podinfo's Deployment

- **WHEN** the user activates the `podinfo-podinfo` Deployment node on `/instances/default/podinfo`
  with script on
- **THEN** the details panel shows the name `podinfo-podinfo`, an Object fact naming the kind
  Deployment, and its health badge beside a visible "Health" label, and the node is marked
  selected

#### Scenario: Without script

- **WHEN** a browser with script disabled follows the same node
- **THEN** the page opens with that node selected and the details panel rendered with it

### Requirement: The panel request keeps its own swap selection, and a locked node fills it

The request that loads a node's details SHALL not take its swap selection from the page around
it, so the panel never shows empty after a selection. A node the caller may not read SHALL fill the
panel with its locked form. Source: portal:D4, portal:D19.

#### Scenario: A locked node

- **WHEN** the caller may not read Services and selects podinfo's Service node
- **THEN** the details panel shows the node locked, with the lock mark and no YAML or Events link

### Requirement: The live mark names the stream's state in words

The header's live mark SHALL say the state of the page's stream in capitalised words: "Live"
while the stream is open, "Not live" before it opens, "Reconnecting", "Offline", "Signed out",
"Topics refused", and "Session expired, reload". Its dot SHALL be steady, with no animation, and
coloured as live, lost or idle; the words, not the colour, SHALL carry the state. Source:
portal:D19.

#### Scenario: A live page

- **WHEN** a page's stream is open
- **THEN** the live mark reads "Live" beside a steady dot

#### Scenario: The stream is lost

- **WHEN** the page's stream drops and the browser is offline
- **THEN** the live mark reads "Offline" and the page keeps what it last rendered

### Requirement: Summary cards share one state block

The pages SHALL draw each summary of one axis or standing with one shared state block: an eyebrow
naming the axis and its source; the time the source recorded for the state, when it records one,
or else words that name no time (such as "checked live"), and nothing otherwise; an icon and a
large uppercase state word in the state's tone; and a summary line. Source: portal:D3:R1,
portal:D2:R3, portal:D7:R3, portal:D19:R5.

#### Scenario: A degraded health block

- **WHEN** a state block is drawn for a Degraded health with one `ImagePullBackOff` reason
- **THEN** it shows the bang icon, the word "Degraded" drawn uppercase in the degraded tone, its
  summary, and a link reading 1 `ImagePullBackOff`

#### Scenario: No recorded time

- **WHEN** a state block's source records no time for its state
- **THEN** the block shows no time, never one borrowed from another field

### Requirement: A state block carries its notes, reasons and links

A state block SHALL also carry optional secondary lines under the summary (a reason's meaning, a
partial or locked mark); an optional caption saying what the reasons count; reason links, each a
count and a reason linking to the view that lists them, or a line saying there are none; and
optional footer links. Source: portal:D3:R1, portal:D7:R3.

#### Scenario: Reasons, notes and links

- **WHEN** a state block is drawn with a note, a caption, two reasons and a footer link
- **THEN** it shows the note under the summary, the caption over the reasons, each reason as a
  link with its count, and the footer link

### Requirement: A state block never reads a missing source as a state

The state word SHALL carry the state without relying on colour. A state the UI does not know SHALL
render in the unknown tone with its word, never in an error tone. When the block's source is
forbidden or not readable, the block SHALL render that source locked or degraded, with no state
word, so a locked source never reads as a state. Every text in the block SHALL be rendered as
text. Source: portal:D2:R3, portal:D19:R5.

#### Scenario: A locked source

- **WHEN** a state block's source is `forbidden`
- **THEN** the block shows its eyebrow and the locked region, and no state word

#### Scenario: Hostile reason text

- **WHEN** a reason or summary holds `<script>alert(1)</script>`
- **THEN** the block shows it as text, escaped

### Requirement: An info tip can be dismissed, hovered and opened by tap

An info tip SHALL show its box when its trigger, a native button, has the pointer or keyboard
focus. It SHALL be dismissable, hoverable and persistent: Escape closes it without moving focus, the
pointer can reach the box, and an opened box closes when focus or the pointer leaves. A tap or Enter
on the trigger SHALL open a closed tip and close an open one. Source: WCAG 2.2 1.4.13 and 4.1.2,
portal:D19.

#### Scenario: Opens on focus

- **WHEN** keyboard focus reaches a tip's trigger
- **THEN** the box is shown, and the trigger's `aria-describedby` names it

#### Scenario: Escape closes the tip

- **WHEN** a tip is open and the user presses Escape
- **THEN** the box is hidden, focus stays on the trigger, and no other layer closes with it

#### Scenario: Escape closes one layer

- **WHEN** a tip is shown while the theme menu is open and the user presses Escape twice
- **THEN** the first press hides the tip and leaves the menu open, and the second closes the menu

#### Scenario: Enter opens it again

- **WHEN** the user has closed a tip with Escape and presses Enter on its trigger
- **THEN** the box is shown again

#### Scenario: Moving focus away closes it

- **WHEN** a tip is open and the user presses Tab to leave its trigger
- **THEN** the box is hidden, and it is shown again when focus returns to the trigger

#### Scenario: A dismiss does not stay

- **WHEN** the user closes a tip with Escape, moves focus away and returns
- **THEN** the box is shown

#### Scenario: The box holds text only

- **WHEN** the tip partial renders hostile text
- **THEN** the box shows it escaped and holds no link or control

#### Scenario: The trigger is a control

- **WHEN** a screen reader meets a tip's trigger
- **THEN** it is a button that `aria-describedby` ties to the box

#### Scenario: The pointer moves onto the box

- **WHEN** the pointer rests on a tip's trigger and then moves onto its box
- **THEN** the box stays shown

#### Scenario: A tap on a touch screen

- **WHEN** the user taps a tip's trigger on a touch screen
- **THEN** the box opens, a second tap on the trigger closes it, and a tap outside closes it

#### Scenario: Without script

- **WHEN** script is off and the pointer or focus reaches a tip's trigger
- **THEN** the box is still shown

### Requirement: Badges, the state block and the info tip keep a border in forced-colours mode

In forced-colours mode, the Applied and Health badges, the state block and the info tip's box
SHALL each keep a border drawn in a system colour, and a partial health SHALL stay apart from a
full health by its dashed border. Source: WCAG 2.2 1.4.11, portal:D19:R3.

#### Scenario: Badges keep their border

- **WHEN** the stylesheet is checked
- **THEN** a forced-colours block sets a system-colour border on the Applied badge, the Health
  badge and the partial health badge, and on the state block and the tip's box

#### Scenario: Only the current tab keeps its underline

- **WHEN** forced-colours mode replaces the tab strip's colours
- **THEN** a forced-colours block gives the tabs that are not current no bottom border, so the
  strip's baseline stays whole, and the current tab keeps a system-colour underline

#### Scenario: A partial health without its hatch

- **WHEN** forced-colours mode drops the partial health badge's hatch
- **THEN** the badge's dashed border still tells it from a full health

### Requirement: Pages do not animate in

No element SHALL animate when a page loads or a tab opens, whatever the user's motion preference.
The fading ring on a region that a live refresh replaces stays, and only where the user has not
asked for reduced motion. Source: the owner's decision of 2026-10-11, portal:D19.

#### Scenario: A page loads

- **WHEN** `/installed` loads with motion allowed
- **THEN** no animation runs on any element inside `main`

#### Scenario: No entrance rule remains

- **WHEN** the stylesheet is checked
- **THEN** it holds no `@keyframes rise` and no rule on `.reveal`

### Requirement: Filter forms offer the values present

The filter forms of the Installed list, the Platform's Providers and Catalogs tabs and recent
events, and the Catalog page's Events tab SHALL each draw their fields in their view's parameter
order. A filter whose values are facts of the listed rows (Installed's namespace, uses and
module; the Providers tab's provides) SHALL be a select of the values present in the rows the
caller may read. Source: portal:D14:R1/R2, portal:D19.

#### Scenario: Namespace choices with counts

- **WHEN** a signed-in browser opens `/installed` on the F1 capture
- **THEN** the Namespace filter offers `cert-manager (1)`, `default (3)`, `pkg (1)` and
  `web (1)` after "All namespaces"

### Requirement: Filter selects name their empty choice

On the filter forms of the Installed list, the Platform's Providers and Catalogs tabs and recent
events, and the Catalog page's Events tab, the first option of every select SHALL name the empty
choice in words ("Any kind", "All namespaces", "Anyone", "Any status", "Either"). Source:
portal:D14:R1/R2, portal:D19.

#### Scenario: Provider status choices

- **WHEN** a signed-in browser opens `/?tab=providers` on the F1 capture
- **THEN** the Status filter offers "Any status", then each status with its count, among them
  `Accepted (1)` and `Refused (1)`

### Requirement: Filter selects keep the full contract and a URL value

On the filter forms of the Installed list, the Platform's Providers and Catalogs tabs and recent
events, and the Catalog page's Events tab, a contract SHALL show by its last two path segments
with its version, with the full contract as its value. A value in the URL that is not among the
choices SHALL still apply and show as selected. Source: portal:D14:R1/R2, portal:D19.

#### Scenario: A value not among the choices

- **WHEN** a browser opens `/installed?namespace=team-z` on the F1 capture
- **THEN** the Namespace select shows `team-z` selected, the list is empty, and its chip reads
  "Namespace: team-z"

### Requirement: Filter counts cover only what the caller may list

On the filter forms of the Installed list, the Platform's Providers and Catalogs tabs and recent
events, and the Catalog page's Events tab, every select option SHALL carry its count over the
rows before filtering. A select SHALL offer as choices, and count, only values found in rows the
caller may list, the Uses options included. Source: portal:D14:R7, portal:D7:R2.

#### Scenario: A reader who may list one namespace sees that namespace only

- **WHEN** the caller may list ModuleInstances and ModulePackages only in namespace `default` and
  opens `/installed?namespace=default` on the F1 capture
- **THEN** every option count, the Uses counts included, counts rows in `default` only, the Uses
  options hold no contract that only rows in `cert-manager` or `web` use (such as `crds`), and the
  namespace choices hold `default` only

### Requirement: A list the caller may not read gives no count

On the filter forms of the Installed list, the Platform's Providers and Catalogs tabs and recent
events, and the Catalog page's Events tab, an option whose rows come from a list the caller may
not read, or that failed, SHALL carry no count, never a zero. The form SHALL then say its counts
cover only what the caller may read, and the namespace filter SHALL stay a field the user can
type into, so a reader who may list only some namespaces can name one. Source: portal:D14:R7,
portal:D7:R2.

#### Scenario: A namespace-only reader

- **WHEN** the caller may list ModuleInstances and ModulePackages only in namespace `default` and
  opens `/installed`
- **THEN** both lists render locked, the Namespace filter is a field the user can type into, and
  no option count stands for the locked lists

#### Scenario: Packages forbidden in the Kind filter

- **WHEN** the caller may list ModuleInstances but not ModulePackages and opens `/installed`
- **THEN** the Kind filter's Package option carries no count

#### Scenario: Neither Installed list readable

- **WHEN** the caller may list neither ModuleInstances nor ModulePackages and opens `/installed`
- **THEN** the Health, Applied, Provider and Owner options carry no count, never "(0)"

#### Scenario: Platform forbidden in the Catalogs filters

- **WHEN** the caller may not get the Platform and opens `/?tab=catalogs`
- **THEN** the Source options carry no count

#### Scenario: Registrations forbidden in the Platform filters

- **WHEN** the caller may get the Platform but may not list TransformerRegistrations and opens
  `/?tab=providers`, then `/?tab=catalogs`
- **THEN** the Status and Provides options carry no counts, and the Catalogs tab offers no Claimed
  filter

### Requirement: Filter forms show what they apply

On the filter forms of the Installed list, the Platform's Providers and Catalogs tabs and recent
events, and the Catalog page's Events tab, each active filter's chip SHALL read "Label: value".
The form SHALL always show how many rows it shows of how many, and "Clear all" when a filter is
active. Source: portal:D14:R1/R3.

#### Scenario: Chip, count and Clear all

- **WHEN** a signed-in browser opens `/installed?namespace=default` on the F1 capture
- **THEN** the form shows the chip "Namespace: default", the count "Showing 3 of 6" and "Clear all"

### Requirement: Filter forms keep the URL as their source

On the filter forms of the Installed list, the Platform's Providers and Catalogs tabs and recent
events, and the Catalog page's Events tab, the URL query SHALL stay the filters' source of truth.
Without script the Apply button SHALL show and every filter SHALL work as a plain `GET`. Source:
portal:D14:R1/R3.

#### Scenario: Without script

- **WHEN** a browser with script disabled submits Installed's filter form with search `backup`
- **THEN** the Apply button is visible and the page shows `/installed?q=backup`

### Requirement: Filter forms apply as the user types

On every page, with script, a search or text field of a `data-filters` form, among them the
filter forms of the Installed list, the Platform's Providers and Catalogs tabs and recent events,
and the Catalog page's Events tab, SHALL apply after a short pause in typing, with the user's
focus, caret and any text typed meanwhile kept, and that form's Apply button SHALL be hidden until
it takes keyboard focus.
Source: portal:D14:R1/R2, portal:D19.

#### Scenario: A text field applies when the user tabs out of it

- **WHEN** a browser with script types `BackOff` into a text field that is not a search, in a
  `data-filters` form whose Apply button has a stable id (as the owner page's Events form has),
  and presses Tab
- **THEN** focus is on that form's Apply button, the list applies, and focus is on the new Apply
  button after the swap, because the button has a stable id

#### Scenario: Typing applies the search

- **WHEN** a browser with script types `backup` into Installed's search field and stops typing
- **THEN** the page shows `/installed?q=backup` listing `default/backup-provider` and
  `default/backup-consumer`, with focus and caret still in the search field, and no Apply button
  is visible

### Requirement: A filter's new count is announced once

On the Platform page, the Installed list and the Catalog page, when a filter change (a field, a
chip's remove link or "Clear all") has applied, the page SHALL announce the new count ("Showing n
of M") once, through the header's existing polite status region. A live refresh, a change of the
stream's state and a move to another page SHALL NOT announce it again. No other live region SHALL
be added. Source: portal:D14:R1, WCAG 2.2 success criterion 4.1.3.

#### Scenario: Typing announces the count

- **WHEN** a browser with script types `backup` into Installed's search field and the list applies
- **THEN** the header's status region holds the text "Showing 2 of 6" once, and focus stays in
  the search field

#### Scenario: A form with no count announces nothing

- **WHEN** a browser with script searches the Platform's Providers tab and the response leaves the
  Providers form without a count
- **THEN** the header's status region holds no text: the count announced is the one inside the
  form that made the request, and the Recent events count is not announced in its place

#### Scenario: Removing a chip announces the count

- **WHEN** a browser with script opens `/installed?q=backup` on the F1 capture and removes the
  chip "Search: backup"
- **THEN** the header's status region holds the text "Showing 6 of 6" once

#### Scenario: Clear all announces the count

- **WHEN** a browser with script opens `/installed?namespace=default` on the F1 capture and
  follows "Clear all"
- **THEN** the header's status region holds the text "Showing 6 of 6" once

#### Scenario: A refresh does not announce

- **WHEN** a live refresh changes the rows or the count of an Installed list the user has not
  touched
- **THEN** the header's status region keeps the text it held before

#### Scenario: A change of the stream's state does not repeat the count

- **WHEN** the count "Showing 2 of 6" has been announced and the stream's state then changes to
  "Reconnecting"
- **THEN** the header's status region presents the new state only, and the count is not read again

#### Scenario: A move to another page forgets the count

- **WHEN** the count "Showing 2 of 6" has been announced on Installed and the user opens the
  Platform page from the main navigation
- **THEN** the hidden count is empty, and a later change of the stream's state does not present it

### Requirement: Installed counts every namespace the caller may list

When the caller may list ModuleInstances and ModulePackages in every namespace, Installed SHALL
apply its namespace filter to the full lists, so its total, its filter counts and its namespace
choices cover every namespace. Source: portal:D7:R2, portal:D14:R2/R7.

#### Scenario: The total under a namespace filter

- **WHEN** a signed-in browser opens `/installed?namespace=default` on the F1 capture
- **THEN** the list holds the three instances in `default` and says it shows 3 of 6

### Requirement: Installed counts one namespace when a kind's full list is forbidden

When a kind's full list is forbidden and a namespace filter is set, Installed SHALL list that kind
in the filtered namespace only, SHALL apply the namespace filter to the other kind's rows before
counting, so that the total, the filter counts and the choices all cover that namespace only, and
SHALL label its total as within that namespace. Source: portal:D7:R2, portal:D14:R2/R7.

#### Scenario: A namespace-scoped reader with a namespace filter

- **WHEN** the caller may list ModuleInstances only in namespace `default` and opens
  `/installed?namespace=default`
- **THEN** the instances in `default` are listed and the total is labelled as within `default`

#### Scenario: One kind scoped

- **WHEN** the caller may list ModuleInstances in every namespace but ModulePackages only in
  `default`, and opens `/installed?namespace=default`
- **THEN** the page says "of N in default", where N counts only the instances and packages in
  `default`, and no option count, the Uses counts included, counts a row outside `default`

### Requirement: The module filter matches a package by its source

Installed's module filter SHALL offer, beside the instances' module paths, each package's source
as `<Kind> <namespace>/<name>`, and SHALL match a package whose source is that value. Search SHALL
also match an item's kind word (`instance`, `package`) and a package's source. Source: portal:D17.

#### Scenario: A package by its source

- **WHEN** a browser chooses the F1 package's source in Installed's Module filter
- **THEN** the list holds `pkg/podinfo` only, and no note says packages are excluded

#### Scenario: Searching by kind

- **WHEN** a browser opens `/installed?q=package` on the F1 capture
- **THEN** the list holds `pkg/podinfo`

### Requirement: Configuration groups open one at a time inside a frame

A collapsed configuration group SHALL be drawn as a stacked, hatched node naming its family, its
component count and its object and kind counts. Opening one configuration group SHALL close any
other open configuration group, while open Pod groups stay open. Source: portal:D4:R5.

#### Scenario: Opening CRDs closes RBAC

- **WHEN** cert-manager's RBAC group is open and the user activates the CRDs group
- **THEN** the CRDs group is open inside its frame and the RBAC group is folded again

### Requirement: An open configuration group is drawn inside a dotted frame

An open configuration group SHALL be drawn inside a dotted frame, spanning from the components
column to the last drawn column, whose header names the family and counts its components and
objects and which carries a Collapse control. Collapsing SHALL leave the group selected, with its
panel showing the object kinds it folds and their counts, its health and a control to expand it
again. Source: portal:D4:R5.

#### Scenario: Collapse from the frame

- **WHEN** the user activates the RBAC frame's Collapse control
- **THEN** the group is folded and selected, and the panel counts its ClusterRoles,
  ClusterRoleBindings, Roles and RoleBindings

### Requirement: The graph toolbar groups and counts configuration

The graph toolbar SHALL offer Expand all, which opens every family, Collapse all, and a Group
configuration switch that says in visible text whether configuration is grouped: grouped, families
fold and open as above; ungrouped, every configuration component is shown with no group node and
no frame. The toolbar SHALL say how many boxes are drawn of how many there are. Source: portal:D4:R5.

#### Scenario: Expand all

- **WHEN** the user activates Expand all on cert-manager's graph
- **THEN** all three groups are open, each inside its own frame, and the Group configuration switch
  still reads grouped

#### Scenario: Ungrouped

- **WHEN** the user turns the Group configuration switch off on cert-manager's graph
- **THEN** all 20 components are shown with their objects, with no group node and no frame, and the
  switch reads ungrouped

### Requirement: The graph explains its marks

Below every graph a legend SHALL say what the marks mean: applied by the controller, made by the
cluster, a configuration group and how it opens, that fill and outline carry health, and how to
zoom and step back. A graph that draws a source node SHALL also name the source's origin, read by
the controller and owned by Flux, and say which other boxes are drawn the same way. Source:
portal:D4:R5, portal:D19.

#### Scenario: The legend

- **WHEN** a browser opens podinfo's Graph tab
- **THEN** the legend names the applied mark, the dashed cluster-made boxes, the group box, and the
  Ctrl + scroll and Esc keys, and names no source

#### Scenario: The legend of a package

- **WHEN** a browser opens the Graph tab of `/packages/pkg/podinfo` on the F1 capture
- **THEN** the legend also names the source's origin, "Read by the controller, owned by Flux", says
  that the package's components and the packages it depends on are drawn the same way, and the
  source's own hover card reads the same origin words

### Requirement: An empty inventory shows a placeholder

When an instance or package has applied nothing (no inventory and no last applied time), the graph
SHALL show a dashed placeholder box beside its root saying nothing was applied yet and that the
graph fills in once an apply succeeds, and, for a package with no recorded source artifact, that it
has never fetched its source. The placeholder SHALL not be a node: it has no panel and no link.
Source: portal:D4:R1.

#### Scenario: The F1 package

- **WHEN** a browser opens the Graph tab of `/packages/pkg/podinfo` on the F1 capture
- **THEN** a dashed box beside the package says nothing was applied yet and that the package has
  never fetched its source

### Requirement: Registration and source nodes show their own state

A TransformerRegistration node on an instance or package graph SHALL be toned and labelled by the
registration's own standing (active, accepted but not active, refused with its reason, removal
blocked, pending), and its card SHALL show that standing with the claimed catalog and the provider.
A claim the caller may not read SHALL render locked. Source: portal:D15:R2/R3/R4.

#### Scenario: backup-provider's registration

- **WHEN** a browser opens backup-provider's Graph tab on the F1 capture
- **THEN** the `default.backup-provider` node reads "Accepted · active" in the healthy tone, and its
  panel names the catalog `testing.opmodel.dev/catalogs/operator/backup@v0` and version `0.1.0`

#### Scenario: Locked claim

- **WHEN** a caller who may not list TransformerRegistrations opens backup-provider's Graph tab
- **THEN** the registration node shows no standing and its panel says the standing is locked

### Requirement: A registration's panel shows its standing, catalog and provider

A registration's panel SHALL show the standing, the controller's message in local mode, the catalog
with its version linking to the Catalog page, and the provider and whether `spec.providerRef` names
it. Source: portal:D15:R2/R3, portal:D8:R5.

#### Scenario: In-cluster registration panel

- **WHEN** backup-provider's registration panel is rendered over an in-cluster read API
- **THEN** it shows the standing and reason and no message text the operator wrote

### Requirement: A source node is toned and labelled by the source's state

A package's source node SHALL be toned and labelled by the source's state (ready with its revision,
not ready with its reason, ready with no artifact, unknown, not found, kind not installed, kind not
read), and its panel SHALL show the Ready reason, message and revision, and for a kind the cluster
does not serve, that Flux source-controller is not installed or the package names a kind the
cluster lacks. Source: portal:D20, portal:D8:R5.

#### Scenario: Kind not installed

- **WHEN** a browser opens the Graph tab of `/packages/pkg/podinfo` on the F1 capture
- **THEN** the source node reads "Kind not installed", and its panel says the cluster does not serve
  the OCIRepository kind

#### Scenario: Locked source

- **WHEN** a caller who may get the package but not its OCIRepository opens the Graph tab of
  `/packages/pkg/podinfo` on a cluster that serves the kind
- **THEN** the source node is drawn locked with no state, and its panel says the source is
  locked

### Requirement: The details panel rests on the node that needs attention

On the Graph and Resources tabs, with neither `node` nor `focus` set, the details panel SHALL show
the resting node without a spotlight: the node of the first held TransformerRegistration the caller
may read whose claim is refused, pending, accepted but not active, or removal blocked, and
otherwise the instance's or package's own node. Source: portal:D4:R4, portal:D19.

#### Scenario: cert-manager at rest

- **WHEN** a browser opens cert-manager's Graph tab with no parameters
- **THEN** the details panel shows the ModuleInstance `cert-manager/cert-manager`

#### Scenario: A refused package claim

- **WHEN** a browser opens the Graph tab of a package whose held registration was refused with
  `ProviderMismatch`
- **THEN** the details panel shows that registration with its refusal reason, and no node is dimmed

### Requirement: The resting node is marked apart and a selection returns to it

The resting node SHALL be marked apart from a selection, and Escape SHALL not clear it. Clearing a
selection SHALL return the panel to the resting node. A panel other than the resting node's SHALL
carry a Clear selection control. Source: portal:D4:R4, portal:D19.

#### Scenario: Clearing returns to rest

- **WHEN** the user selects podinfo's Service and then activates Clear selection
- **THEN** the panel shows the ModuleInstance `default/podinfo` again and the address carries no
  `node`

### Requirement: The details panel shows an instance's inventory and its revision

The details panel SHALL show, for the node that is the instance page's own instance, "Inventory",
its object count ("1 object", "2 objects"), and "Inventory revision", the revision the API serves.
Inventory SHALL show when the count is above zero or a revision is served; the revision SHALL show
only when the API serves one. Neither SHALL show for a node the caller may not read, for a package,
or for any other node. Source: portal:D2:R1, portal:D17.

#### Scenario: podinfo at rest

- **WHEN** a browser opens `/instances/default/podinfo` with no `node` or `focus`
- **THEN** the panel shows `Inventory` `2 objects` and `Inventory revision` `1`

#### Scenario: One object

- **WHEN** the instance's inventory holds one object, as `default/backup-provider`'s does
- **THEN** the panel reads `1 object`

#### Scenario: Another node selected

- **WHEN** the user selects podinfo's Deployment node
- **THEN** the panel shows the Deployment and neither fact, and Clear selection brings both back

#### Scenario: Applied, nothing rendered

- **WHEN** an instance records revision 2 and an empty inventory
- **THEN** the panel shows `Inventory` `0 objects` and `Inventory revision` `2`

#### Scenario: Never applied

- **WHEN** an instance has recorded no inventory
- **THEN** the panel shows neither fact

#### Scenario: A package panel

- **WHEN** a browser opens `/packages/pkg/podinfo`
- **THEN** the panel shows its source facts, with its `Revision` as the source's, and no
  `Inventory revision`

#### Scenario: A locked node

- **WHEN** the panel opens on a node whose access is not ok
- **THEN** it shows the locked form with no facts, and neither fact

#### Scenario: Phone width

- **WHEN** the instance page is opened at 360 px wide
- **THEN** both facts are readable in the panel and the page does not scroll sideways

### Requirement: Relative times are rewritten as they age

Every relative time the pages print (`<n>s ago`, `<n>m ago`, `<n>h ago`, `<n>d ago`, `just now`)
SHALL be rewritten by the page script every 30 seconds, and once when a hidden tab becomes
visible, from the element's `datetime` attribute, in the words the server uses for the same age.
The script SHALL measure the age against the server's clock as the page carried it at load, not
the browser's clock alone.

#### Scenario: A time ages without a reload

- **WHEN** a page shows a time stamped 4 minutes before the server's clock and stays open for
  2 minutes
- **THEN** the time reads "6m ago" without any request

#### Scenario: Boundaries use the server's words

- **WHEN** the script computes the age for 0, 59, 60, 3599, 3600, 172799 and 172800 seconds and
  a negative age
- **THEN** the texts are the rows of `internal/ui/testdata/relative-time.json`, the same rows
  the server's `relative` function is tested against

#### Scenario: A browser clock that is wrong

- **WHEN** the browser's clock is 10 minutes behind the server's and the page shows a time stamped
  4 minutes before the server's clock
- **THEN** the time reads "4m ago" at load and "5m ago" 60 seconds later

### Requirement: A rewritten time changes only its text

The rewrite of a relative time SHALL write the element's text and nothing else, SHALL leave an
unchanged text untouched, SHALL not move the layout by more than the difference in glyph width,
and SHALL not be announced by a screen reader. A time the server could not state ("never") SHALL
not be rewritten.

#### Scenario: A time that is not stated

- **WHEN** a field carries no recorded time and the page prints "never"
- **THEN** the script leaves it as it is

#### Scenario: A time in the details panel

- **WHEN** a time shown inside the details panel changes its text on a tick
- **THEN** the change is not announced, because the time element is marked as not live

#### Scenario: A tick that changes nothing

- **WHEN** a tick finds a time whose text is already right
- **THEN** the script does not write to the element

### Requirement: A request open for a second shows a loading bar

Once any request that the page script's htmx sends has been open for 1 second, the page SHALL
show a 2 px bar along the bottom of the header, static under `prefers-reduced-motion`. The bar
SHALL end when no such request is open. A request that ends in an error, an abort or a timeout
SHALL end like a success, and a second end event of one request SHALL change nothing. The stream's
background refreshes SHALL not show the bar.

#### Scenario: A request that ends in under a second

- **WHEN** a boosted navigation answers in 300 ms
- **THEN** no bar appears

#### Scenario: Two requests overlap

- **WHEN** a panel request and a boosted navigation are open and the panel request ends first
- **THEN** the bar stays until the navigation ends

#### Scenario: A request fails

- **WHEN** a request that has shown the bar ends in a server error
- **THEN** the bar is gone and the page shows its error as it does today

#### Scenario: A refresh from the stream

- **WHEN** a followed topic changes and the page re-renders its regions in the background for
  4 seconds
- **THEN** no bar appears

### Requirement: A slow panel request marks the details panel busy

While a request that fills the details panel has been open for 1 second, the panel SHALL carry
`aria-busy="true"` and a visible "Loading..." line, and both SHALL go when the request ends. The
state SHALL not use a `style` attribute.

#### Scenario: A slow panel

- **WHEN** the user selects a graph node and the panel request takes 3 seconds
- **THEN** after 1 second the bar shows, and the details panel carries `aria-busy="true"` and the
  line "Loading..."
- **AND** when the panel arrives, the bar, the line and `aria-busy` are gone

#### Scenario: A panel request fails

- **WHEN** a panel request that has shown the line ends in a server error
- **THEN** the line and `aria-busy` are gone and the panel shows the error as it does today

### Requirement: A lost stream shows a banner

When the page's stream is not open (reconnecting, offline, topics refused, or a signed-out
answer), the page SHALL show a visible banner in the sticky header, so that a reader scrolled down
the page still sees it, with the live mark's word, the time of the data on screen, and a Reload
link. The banner SHALL be neither a live region nor given a role. While the stream is healthy the
page SHALL record the time of every stream message, a heartbeat included, and at load.

#### Scenario: The stream drops

- **WHEN** the stream's connection drops 20 seconds after its last heartbeat at 10:42:07 UTC
- **THEN** the live mark reads "Reconnecting"
- **AND** the banner reads "Showing data read 20s ago. Reconnecting." with a Reload link

#### Scenario: A reader scrolled far down

- **WHEN** the reader has scrolled 3000 px down a Resources page and the stream drops
- **THEN** the banner and its Reload link are inside the viewport, at the top, with the header

#### Scenario: The page makes no announcement of its own

- **WHEN** the banner appears
- **THEN** it has no `role` and no `aria-live`, and the only status text that changes is the
  live mark's

#### Scenario: No script

- **WHEN** a page is served and its script does not run
- **THEN** the banner stays hidden and the page renders as before

### Requirement: The banner and the stale note print their time like every other time

The time in the banner and in the stale note SHALL have the form of the `time` partial: relative
text, the ISO time in `datetime` and the absolute time in UTC in `title`. It SHALL keep counting
like every other relative time.

#### Scenario: The time carries both forms

- **WHEN** the banner shows the last message of 10:42:07 UTC, 20 seconds old
- **THEN** its `time` carries `datetime="2026-10-11T10:42:07Z"` and the `title`
  "2026-10-11 10:42:07 UTC", and reads "20s ago"

#### Scenario: The banner keeps counting

- **WHEN** the banner shows "Showing data read 20s ago." and the stream stays down for 60 more
  seconds
- **THEN** the banner reads "Showing data read 1m ago."

### Requirement: A stream that returns re-renders before the banner goes

When a stream opens again after a loss, the page SHALL re-render every region it follows from one
fetch of the page. The page SHALL hide the banner and set the live mark to "Live" only when that
fetch has returned 200 and the regions are replaced.

#### Scenario: The stream returns

- **WHEN** the stream opens again and the one fetch of the page succeeds
- **THEN** the followed regions show the fetch's content, the live mark reads "Live" and the
  banner is hidden

### Requirement: A resync that has not succeeded keeps the banner

From the moment the connection opens again until the resync has succeeded, the live mark SHALL
keep the word of the loss and the banner SHALL stay. When the fetch fails, the banner SHALL stay
with the live mark's word for the failure. When the fetch is answered as signed out, the banner
SHALL stay and read "Signed out".

#### Scenario: The connection is open and the fetch is pending

- **WHEN** the stream's connection has opened again and the one fetch of the page has not yet
  answered
- **THEN** the live mark still reads "Reconnecting" and the banner is still shown

#### Scenario: The stream returns but the fetch fails

- **WHEN** the stream opens again and the page's fetch fails
- **THEN** the live mark reads "Offline" and the banner stays, with "Offline" as its last word

#### Scenario: The stream returns but the session is gone

- **WHEN** the stream opens again and the page's fetch is answered with 401
- **THEN** the live mark reads "Signed out" and the banner stays, reading "Signed out. Showing
  data read" and the time of the data on screen

### Requirement: The banner's time is the time of the data on screen

From the first sign of a loss until a resync has succeeded, the time the banner states SHALL not
move: the `open` event, the snapshots and the heartbeats of the reopened stream SHALL not change
it. A resync that has succeeded SHALL set it to the moment of its answer.

#### Scenario: The reopened stream delivers a heartbeat

- **WHEN** the stream drops 20 seconds after a heartbeat at 10:42:07 UTC, opens again, and a
  heartbeat arrives while the page's fetch is pending
- **THEN** the banner's `time` still carries `datetime="2026-10-11T10:42:07Z"`

### Requirement: A failed resync is tried again

While the page is resyncing, the next stream event of any kind SHALL mark every followed topic dirty
again, so that one more fetch of the page runs. A fetch answered as signed out SHALL not be tried
again.

#### Scenario: The fetch fails once

- **WHEN** the stream opens again, the page's fetch fails, and a heartbeat arrives 15 seconds later
- **THEN** the page fetches again, and when that fetch returns 200 the regions are replaced, the
  live mark reads "Live" and the banner is hidden

### Requirement: The sticky header never covers a focused element

The page SHALL scroll a focused element or the target of a fragment link clear of the sticky
header: `html` SHALL carry a `scroll-padding-top` at least the header's height, raised while the
banner shows and lowered after it goes, and the sticky details panel SHALL take its offset from
the same height.

#### Scenario: Shift+Tab to a link above the viewport

- **WHEN** the banner shows, the reader has scrolled down a page, and presses Shift+Tab to a link
  above the viewport
- **THEN** the link is scrolled into view below the header and is not covered by it

#### Scenario: The banner goes

- **WHEN** the banner is hidden again
- **THEN** the scroll padding returns to the height of the header without it

#### Scenario: The details panel under a taller header

- **WHEN** the banner shows on an instance page wider than 1100 px
- **THEN** the top edge of the sticky details panel sits below the header, not under it

### Requirement: A node panel is re-rendered with its page

A details panel that shows a graph node, including the resting node, SHALL be re-rendered with
the other regions of its page when a topic the page follows changes, from the same one fetch,
without announcing the panel again. A refresh SHALL not replace the panel while a request into it
is open, and SHALL keep the reader's focus, including focus on one of the panel's own links, the
selected node and the open state of its groups.

#### Scenario: A selected node changes health

- **WHEN** the user has selected podinfo's Deployment node, whose panel reads Progressing, and the
  Deployment becomes Healthy
- **THEN** without a reload the graph and the panel both read Healthy, the node stays selected and
  the address still carries `node`

#### Scenario: Focus on a panel link survives a refresh

- **WHEN** a keyboard user has focus on the panel's YAML link and a followed topic changes
- **THEN** after the refresh the panel is replaced and focus is on the YAML link of the new panel

#### Scenario: A request into the panel is open

- **WHEN** a followed topic changes while a panel request is open
- **THEN** the refresh leaves the panel alone and the panel the request returns is the one shown

#### Scenario: A locked node

- **WHEN** the caller may not read Services, a Service node is selected, and a followed topic
  changes
- **THEN** the panel still shows the node locked, with no YAML or Events link

#### Scenario: The selected node is gone

- **WHEN** the object of the selected node is deleted and a followed topic changes
- **THEN** the selection clears and the panel returns to the resting node

### Requirement: A YAML or Events view says when it is old

A YAML view or an Events view in the details panel SHALL not be re-fetched by a refresh. After a
refresh in which a followed topic changed, the view SHALL show a note with the time it was read,
in the form of the `time` partial (relative text, the ISO time in `datetime`, the
absolute time in UTC in `title`), the words "The object may have changed since", and a link that
reloads the view. The note SHALL be plain text from the template.

#### Scenario: A YAML view after a change

- **WHEN** the user has opened a Deployment's YAML in the panel and a followed topic changes
- **THEN** the YAML text is unchanged, and a note reads "Read" with the age of the read as a
  `time` element (its `title` holds the absolute time in UTC) and "The object may have changed
  since", with a link "Reload this view"

#### Scenario: Reloading the view

- **WHEN** the user activates "Reload this view"
- **THEN** the panel shows the object's current YAML and no note
