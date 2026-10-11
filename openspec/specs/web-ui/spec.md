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

Every instance and package SHALL show its applied state and its health as two separate badges of
different shapes. `Ready=True` SHALL be shown as "Applied" and never as healthy.
`ManagedExternally` SHALL render in a neutral style without an error colour. A partial health
SHALL say partial and be marked visually apart from a full health; a health with an object
refreshed by polling SHALL say it is not live and show when it was evaluated. An enum value the
UI does not know SHALL render as "unknown", never as an error. The red family SHALL be used only
for degraded, failed, stalled and refused states, broken edges, Warning events and a lost stream; reasons, versions, kinds and
other decoration SHALL not use it. A condition SHALL be coloured by its `tone`, never by its
status alone. Source: portal:D3:R1/R5/R6/R8, portal:D2:R3.

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
not reconnect, and SHALL show in the live indicator that the session expired and the page must be
reloaded. The page SHALL keep showing what it last rendered.

#### Scenario: The session expires while a page is open

- **WHEN** the session of an open page expires
- **THEN** the live indicator reads "session expired, reload"
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

### Requirement: Muted text meets WCAG 2.2 AA contrast in both themes

Text drawn with the muted ink token SHALL have a contrast ratio of at least 4.5:1 against each
surface token it is drawn on (page, card, secondary card, field, the neutral fill and the degraded
fill), in the light and the dark theme. The ratio is computed from the token hex values in `portal.css` with the WCAG relative
luminance formula and is not rounded. Source: portal:D19:R6.

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
- **THEN** every token in the contrast test's pair list has the same value in both blocks, or the
  test fails

#### Scenario: A token the test cannot read

- **WHEN** a listed token holds a value that is not a six-digit hex colour
- **THEN** the contrast test fails and names the token instead of skipping the pair
