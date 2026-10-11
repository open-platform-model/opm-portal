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

`/` SHALL show the Platform: an identity card (name, type, controller version; the context,
reader and Kubernetes version are in the header); a status card with the applied state of `Ready` and its reason (`Generated`,
or `BuildFailed`, `GenerateFailed`, `ContractCollisions`, `OverSubscribedContracts`,
`ComparablePredicates`), each reason linking to its condition with the meaning, next step and
message, and `ContractsFulfilled` beside it as information; an Installed card with the counts per
health state and per applied state, each linking to Installed filtered by it, and the numbers of
instances, packages and of those that hold a registration; Providers and Catalogs tabs with
filters; and the recent events of the Platform and of each readable registration, merged newest
first, with a resource filter. A Providers row SHALL show the registration's name, its catalog and
version linking to the Catalog page, its provided contracts, its holder ("Installed as") with kind
and link, its acceptance and its activation as two separate pills, and its refusal or
removal-blocked reason and message. A removal-blocked registration SHALL show as removal blocked
while still accepted and active, never as refused. A Catalogs row SHALL show the path linking to
the Catalog page, version, source, claimants and enablement. A Platform reporting unfulfilled
provider contracts SHALL show it as information, not as a failure. The page SHALL show no Platform
health and no platform graph. When the registrations the caller may read are not all of them, or a
list behind the Installed counts is forbidden, the page SHALL say so with the locked style.
Source: portal:D4:R4/R7, portal:D3:R8, portal:D17, portal:D9:R4.

#### Scenario: The F1 platform

- **WHEN** a signed-in browser opens `/` on the F1 capture
- **THEN** its Catalogs tab shows the `opmodel.dev/catalogs/opm` subscription with its version
  and the backup catalog contributed by the accepted, active `backup-provider` claim, and its
  Providers tab shows the refused claim with its verdict Refused, its reason and its message

#### Scenario: Accepted and active apart

- **WHEN** a registration is accepted and not active
- **THEN** its row shows an "accepted" pill and an "inactive" pill, each on its own

#### Scenario: A count hands off

- **WHEN** the user follows the Degraded count on the Installed card
- **THEN** the browser opens `/installed?health=Degraded`

#### Scenario: Instances forbidden

- **WHEN** the caller may not list ModuleInstances
- **THEN** the Installed card shows the instance counts locked, not zero

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

The instance page SHALL show the instance's graph, its inventory objects and runtime children with
their health in the Resources tab, its conditions with each known reason's meaning and next step
beside the controller's message, its status history newest first behind "Show history", the render
contracts as plain text labelled as the contracts the render used and not the instance's provider
demand, and its recent events with repeats folded into one line with a count and the latest time.
Configuration components SHALL be grouped in the Resources tab as in the graph, with a count and
their worst health, and the group SHALL be open when any member is not healthy. ReplicaSets with
zero desired replicas SHALL be folded apart from the live ones. The feed SHALL be labelled as
recent activity that expires after about an hour, and the page SHALL say that render warnings are
kept only as events. The Logs tab SHALL offer each container of each Pod an inventory reaches,
following its log topic, rendering log text as text only, and each Pod row SHALL link to its log.
The package page SHALL show the same for a ModulePackage, with its source, interval, source
revision and dependencies. Source: portal:D4:R3, portal:D9:R2/R3/R5, portal:D10, portal:D16:R1.

#### Scenario: podinfo's page

- **WHEN** a signed-in browser opens `/instances/default/podinfo` on the F1 capture
- **THEN** across its tabs it shows the podinfo component with its Service and Deployment, the
  ReplicaSet and two Pods below, the conditions with their meanings, the history, the render
  contracts as text, the events feed with its expiry note, and one log entry per Pod container

#### Scenario: cert-manager's configuration components

- **WHEN** a signed-in browser opens `/instances/cert-manager/cert-manager?tab=resources` on the
  F1 capture
- **THEN** its configuration components are one closed group with their count and health, and
  its workload components are listed one by one

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
document order. A node's box outline SHALL carry its health only, and a separate mark on the node
SHALL carry its applied state. A label too long for its node SHALL keep the part that tells nodes
apart. Activating a node SHALL show its detail panel and mark the node as selected; activating a
group node SHALL show it expanded. A graph SHALL open fitted to its frame, SHALL pan with the
pointer and SHALL zoom with a zoom slider, Fit and Ctrl/Cmd-wheel. Graphs SHALL appear on instance and package pages; the Platform
page SHALL not draw one. Source: portal:D4:R1/R3/R5, portal:D3:R1, portal:D17.

#### Scenario: Keyboard focus

- **WHEN** the user tabs into podinfo's graph
- **THEN** focus moves through the nodes in column order and each announces its kind, name and
  health

#### Scenario: Two axes on a node

- **WHEN** an instance node is applied and degraded
- **THEN** its outline is drawn as degraded and its applied mark as applied

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
rendered.

#### Scenario: The session expires while a page is open

- **WHEN** the session of an open page expires
- **THEN** the live mark reads "Session expired, reload"
- **AND** the page makes no further stream request

### Requirement: The header names the cluster, the reader and the version

Every page SHALL carry a sticky header with the main navigation (Platform, Installed), the
cluster the portal reads (the kubeconfig context, or "in cluster"), "reading as" with the
caller's username, the Kubernetes version, the live mark and the theme menu, the facts read from
the read API's `Cluster` document. The header SHALL be the one place these facts show. A version
the document does not carry SHALL read "unknown". When the `Cluster` document cannot be fetched,
the header SHALL still render its navigation, live mark and theme menu, with the cluster, reader
and version reading "unknown" in the degraded style. The header SHALL slim after the page scrolls
48 px and SHALL not animate under `prefers-reduced-motion`. Source: portal:D18, portal:D17:R1.

#### Scenario: The F1 header

- **WHEN** a signed-in browser opens any page on the F1 capture in local mode
- **THEN** the header shows Platform and Installed, the context name, "reading as" with the
  kubeconfig's username, the Kubernetes version and the live mark

#### Scenario: Version not read

- **WHEN** the `Cluster` document carries no `kubernetesVersion`
- **THEN** the header's version reads "unknown", not empty

#### Scenario: The cluster document fails

- **WHEN** the in-process fetch of the `Cluster` document fails
- **THEN** the page renders with its navigation and theme menu, and the header's cluster, reader
  and version read "unknown" in the degraded style

#### Scenario: In a Pod

- **WHEN** the portal runs local mode in a Pod with in-cluster credentials
- **THEN** the header reads "in cluster" in place of a context name

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
name and namespace, a kind chip (instance and package drawn apart), a provider badge when the
item holds a TransformerRegistration, the module path and version or the package's source and
path, the applied badge, the health badge, the object count and the owner (`controller` or `cli`
for an instance, `controller` for a package). A kind the caller may not list SHALL render locked
for that kind only, with no count, while the other kind's rows show. `/instances` and `/packages`
SHALL redirect with `308` to `/installed` with `kind=instance` or `kind=package`, keeping a
`namespace` parameter. Source: portal:D17:R1, portal:D7:R2.

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

The page of an instance or package SHALL show an identity card (kind, name, namespace, module path
and version, or source, path, interval and source revision, owner, applier ServiceAccount), an
Applied card and a Health card, and a Provider card when it holds a TransformerRegistration. The
Applied card SHALL show the applied badge with its reason and meaning, the counts of Warning
events in the current feed by reason, each linking to the Events tab filtered by that reason and
labelled as the last hour of events, a `Reconciling` mark with its reason and since time while
that condition is `True`, and one dot per `status.history` entry, oldest first, coloured by the
entry's outcome; it SHALL show no dot for an attempt the history does not record, and SHALL say
that at most ten attempts are kept and that reconciles that change nothing are not recorded. The
Health card SHALL show the health badge, its partial and live marks, and the counts of objects and
runtime children by health reason, each linking to the Resources tab with `reason` set to it. The Provider card SHALL show, per held
registration, Active, Accepted and not active, Refused, Removal blocked or Pending with its
reason, or a locked standing when the caller may not read registrations. An interval the package
does not set SHALL read "not set"; a revision the package has not recorded SHALL read "not
recorded". Source: portal:D3:R1, portal:D9:R1/R2, portal:D15:R2/R4.

#### Scenario: The image-break podinfo

- **WHEN** the page of the image-break podinfo sample is rendered
- **THEN** the Applied card reads Applied and the Health card reads Degraded with one count for
  the Pod's waiting reason, linking to `tab=resources&reason=<that reason>`, which lists that Pod

#### Scenario: A failed attempt in the history

- **WHEN** an instance's newest history entry has no phase and a message
- **THEN** its dot is drawn failed, and in local mode its title carries the message

#### Scenario: The F1 package

- **WHEN** a browser opens `/packages/pkg/podinfo` on the F1 capture
- **THEN** the identity card shows source `OCIRepository` `podinfo-release`, interval `1m` and
  revision "not recorded", and the Applied card reads Failed with reason `SourceNotReady`, a
  `Reconciling` mark with reason `Progressing`, and three failed dots and no other dot

### Requirement: Instance and package pages are organised in tabs

Below the cards the page SHALL offer the tabs Graph, Resources, Events, Logs and YAML, and
Provider when the item holds a registration, each a link carrying `tab=` that the server renders.
The details panel SHALL show only on Graph and Resources. Resources SHALL list each inventory
object and runtime child with kind, name, origin (in the inventory, or made by the cluster below an
inventory object), health and reason, filtered by a `reason` parameter, with configuration
components grouped as in the graph, and each row SHALL link to the Graph tab focused on its node.
The details panel SHALL keep its Open, Expand, YAML and Events links, and SHALL add a Logs link for
a Pod. Events SHALL filter by resource (grouped
by kind), type and reason. Logs SHALL pick a Pod and container and follow its log as the logs
panel does today. YAML SHALL pick an object grouped by kind and show no Secret and no values.
Source: portal:D8:R2, portal:D9:R3, portal:D10.

#### Scenario: From Resources to the graph

- **WHEN** the user follows the `podinfo-podinfo` Deployment's row in podinfo's Resources tab
- **THEN** the Graph tab opens with that node focused and the details panel showing it

#### Scenario: Events filtered by reason

- **WHEN** a browser opens podinfo's page with `tab=events&type=Warning`
- **THEN** only Warning events are listed, folded with their counts

#### Scenario: A Secret in YAML

- **WHEN** the YAML picker lists an inventory that holds a Secret
- **THEN** the Secret is listed as never read and cannot be opened

### Requirement: Graphs can be explored without leaving the page

A graph SHALL show a card for a node on pointer hover and on keyboard focus (kind, name, health
and applied marks, health reason, origin, access), positioned without any `style` attribute. A
`focus` parameter SHALL spotlight its node and dim what is not adjacent to it, and "Clear
selection" SHALL remove it. The graph SHALL open in full screen and back. It SHALL zoom with a
zoom slider, Fit and Ctrl/Cmd-wheel, and pan with the pointer. A group
node SHALL expand in place through the graph's `expand` parameter and fit the view to its
members, and a "Whole graph" control SHALL collapse it and fit the whole graph again. Source:
portal:D4:R5.

#### Scenario: cert-manager's RBAC group

- **WHEN** the user expands cert-manager's configuration group
- **THEN** the view fits its members, and "Whole graph" returns to the collapsed, fitted graph

#### Scenario: Keyboard card

- **WHEN** the user tabs onto podinfo's Deployment node
- **THEN** its card shows kind, name, health and origin, and is announced with the node

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

The Provider tab SHALL show, per held registration, its name, its claimed catalog (linking to the
Catalog page) and version, whether that catalog is in the Platform's resolved registry and
contributed by this registration, its conditions (type, status, reason, meaning, since, and in
local mode the message), and per provided contract the readable instances whose render contracts
contain it: the first three, the count of the rest, and a link to Installed filtered by that
contract. When the caller could not list instances everywhere, the list SHALL say it may be
incomplete. The registration's conditions, catalog and registry state come from the Platform
document; when the caller may not read the Platform, those parts SHALL render locked and the tab
SHALL keep the registration's name and standing. Source: portal:D16:R3, portal:D4:R4,
portal:D7:R3.

#### Scenario: Who uses the backup trait

- **WHEN** a browser opens backup-provider's Provider tab on the F1 capture
- **THEN** contract `opmodel.dev/catalogs/opm/traits/backup@v1alpha1` lists
  `default/backup-consumer` and links to
  `/installed?uses=opmodel.dev/catalogs/opm/traits/backup@v1alpha1`

#### Scenario: Platform forbidden

- **WHEN** the caller may read `default/backup-provider` and list TransformerRegistrations but may
  not get the Platform, and opens its Provider tab
- **THEN** the tab names `default.backup-provider` with its standing, and its conditions and
  catalog render locked; the page is not an error

### Requirement: The Catalog page shows one catalog as the cluster records it

`/catalog?path=<catalog>` SHALL show any catalog the Platform subscribes to, holds in its resolved
registry, or a readable registration claims: its path, version, origin (subscribed, from a
provider, or claimed only), source (the subscription, or the contributing registration and its
holder), enablement, whether it is resolved (with the Platform's `Ready` reason, or the claiming
registration's refusal reason), the Platform's `ContractsFulfilled` labelled as platform-wide, the
registrations that claim it with their holders and standing, and the recent events of the Platform
and its claimants. It SHALL show no definition, description, documentation link or transformer,
and SHALL say these are not recorded. When the caller may not read the Platform, the page SHALL
render locked with status `403`, saying nothing about whether the catalog exists. A path that a
readable Platform and the readable registrations do not name SHALL render not found with status
`404`. Source: portal:D17:R3, portal:D3:R8, portal:D7:R1.

#### Scenario: The contributed backup catalog

- **WHEN** a browser opens `/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0` on the
  F1 capture
- **THEN** it shows version `0.1.0`, origin from a provider, contributed by
  `default.backup-provider` held by `default/backup-provider`, and resolved

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

### Requirement: Pages use the words a platform team uses

Page text SHALL name kinds in the words a platform team uses: "Providers" for
TransformerRegistrations and the instances and packages that hold them, "Installed" for
ModuleInstances and ModulePackages, and "controller" for the opm-operator. The read API's
`operatorVersion` SHALL be labelled "Controller version", and its owner value `operator` SHALL
read "controller". The YAML view shows objects as the cluster serves them and is exempt. Source:
portal:D17:R2.

#### Scenario: The owner column

- **WHEN** the Installed list shows an instance whose owner is `operator` in the read API
- **THEN** its owner cell reads "controller"

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
(inventory objects and runtime children) and Events on instance and package pages (the folded
lines of the feed the tab opens with). Source: portal:D19:R4, portal:D7:R2/R3.

#### Scenario: The F1 Platform tabs

- **WHEN** a signed-in browser opens `/` on the F1 capture
- **THEN** the tabs read "Providers 2" and "Catalogs 3", with Providers underlined as current

#### Scenario: podinfo's tabs

- **WHEN** a signed-in browser opens `/instances/default/podinfo` on the F1 capture
- **THEN** the Resources tab carries the number of its inventory objects and runtime children,
  the Events tab the number of folded lines its feed opens with, and Graph, Logs and YAML carry
  none

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
