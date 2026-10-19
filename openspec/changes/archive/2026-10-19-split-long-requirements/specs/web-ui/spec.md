## REMOVED Requirements

### Requirement: Pages read only through the read API
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Every fact on a page comes from a read API document", "The UI imports only the read API types", "A problem document renders as a region of the page".

### Requirement: The Platform page is the landing page
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The Platform page opens on an identity card", "The status block shows the applied state of Ready", "The status block counts refusals and removal blocks by reason", "The status block names why Ready is not True", "Unfulfilled provider contracts show as information", "The Installed card counts by health and applied state", "The Providers tab shows each registration's standing", "A removal-blocked registration is not shown as refused", "The Catalogs tab shows each catalog's source and claimants", "The Platform events merge the Platform's and its registrations' events", "The Platform page says what the caller could not read".

### Requirement: The instance page shows the record, the feed and the logs
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The instance page shows its graph, resources and conditions", "The instance page shows its history, contracts and events", "The package page shows the same for a ModulePackage", "The Resources tab folds configuration components and idle ReplicaSets", "The Events tab merges the events of everything the inventory reaches", "The Events tab says when events could not be read", "The feed is labelled as recent activity", "The Logs tab follows one container's log", "The Logs tab says when a container has not started or is restarting".

### Requirement: Graphs are server-rendered, accessible SVG
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Each graph renders server-side as SVG", "Graph nodes are named and reachable with the keyboard", "A graph node shows three lines", "A node's fill, outline and mark carry its state", "A graph can be selected, fitted, panned and zoomed", "A graph at phone width does not scroll the page".

### Requirement: Pages update live over one stream per tab
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "A tab holds one stream across navigation", "A change on a followed topic re-renders the open page".

### Requirement: Pages carry a strict policy and their own assets
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "UI pages carry a strict Content-Security-Policy", "Script sets positions and sizes only through CSSOM or SVG attributes", "Untrusted text renders only as text", "Assets are served from the binary at pinned versions", "Browser storage holds only the theme and the filter queries", "Pages follow the chosen theme and hold their width at 360 px".

### Requirement: In-cluster pages show no operator text
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "In-cluster pages omit the operator's messages", "In-cluster pages keep reasons, standings and other writers' notes".

### Requirement: An expired session stops the page's stream
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "An expired session closes the stream and marks the live mark", "An expired session shows a banner with the time of the data".

### Requirement: The header names the cluster, the reader and the version
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Every page carries a sticky header", "The header names the cluster and its entry name", "A version the document does not carry reads unknown", "The header survives a failed Cluster document", "The header slims on scroll".

### Requirement: List filters live in the URL and are remembered per browser
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "List filters read the URL query", "Installed offers its filters as removable chips", "An unknown filter value is ignored with a note", "Only the list views remember their filters", "A filter in the URL wins over the remembered filters", "A remembered view opens with its stored filters", "A stored query is checked and only the shown filters are stored", "Stored filters are kept per context", "The uses filter matches render contracts and leaves packages out".

### Requirement: Instance and package pages are organised in tabs
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Instance and package pages offer their tabs in order", "The details panel rests on the node the graph rests on", "Clear selection returns the details panel to the resting node", "The details panel shows a node's kind, state and object facts", "A Service's Endpoints fact reads the recorded ready count", "A Service's Endpoints fact says when endpoints could not be read", "A Service with no recorded slice has no Endpoints fact", "The details panel of a component says it is not a Kubernetes object", "The details panel of the item's own node shows identity facts", "Panel facts are a fixed list for nodes the caller may read", "An image reference is shown without userinfo", "The details panel keeps its links", "Resources is a table of inventory objects and runtime children", "Resources rows are marked and link to the graph", "Resources folds configuration and filters by reason", "A package's Resources start with its source", "No tab makes the page scroll sideways at 360 px", "Events filter by resource", "Events filter by type and reason", "Events are a table with the regarded resource", "The Logs tab picks a Pod and container", "YAML opens on the selected object and shows no Secret".

### Requirement: Graphs can be explored without leaving the page
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "A graph shows a card beside a node", "Activating a graph node selects it and dims nothing", "A focus parameter spotlights a node and its chain", "Clear selection removes the selection and the spotlight", "A graph opens in full screen", "A graph zooms, fits and pans", "A group node expands in place", "Opening or closing a group keeps the scroll position", "Escape steps back one thing per press", "Wheel zoom does not animate and motion stops under reduced motion".

### Requirement: Providers are shown whatever their kind
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "A holder of a registration shows a Provider pill, card and tab", "The provider standing is the registration's own", "A disagreeing providerRef is shown beside the holder".

### Requirement: The Provider tab shows the claim and who uses what it provides
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The Provider tab opens with a sentence about the provider", "The Provider tab shows each registration's standing once", "The Provider tab shows a grid of facts", "The Contracts fact counts what is provided and in use", "The Provider tab shows the registration's conditions", "The Provider tab tabulates what the registration provides", "A claim not yet active says that nothing is provided yet", "A claim whose removal is blocked says so", "A claim of unknown standing says it is not known", "Provider tab rows are cards below 760 px", "Each contract lists its first three users", "A contract nobody uses says so", "An incomplete list of users says it may be incomplete", "The Provider tab renders locked what the Platform document holds".

### Requirement: The Catalog page shows one catalog as the cluster records it
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The Catalog page shows any catalog the Platform knows", "The Catalog page shows the catalog's origin and source", "The Catalog page shows the catalog's enablement", "The Catalog page shows whether the catalog is resolved", "An unresolved catalog shows the time since it was refused", "Each refusal reason links to the filtered Events tab", "The Catalog page lists the registrations that claim it", "The Catalog page tabulates the events of the Platform and its claimants", "The Catalog page says what is not recorded", "The Catalog page renders locked when the Platform is unreadable", "An unnamed catalog path renders not found".

### Requirement: Pages use the words a platform team uses
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Page text names kinds in a platform team's words", "The controller version and owner are worded as controller", "The Installed card counts providers".

### Requirement: Locked things render locked on every page
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Anything the API reports as unreadable renders locked", "A forbidden instance or package list is a locked group", "A Secret in an inventory is never read".

### Requirement: Instance and package pages summarize Applied, Health and Provider standing
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The identity card names the kind, the name and the module or source", "The identity card carries the namespace, owner and applier", "The identity card takes one column on a narrow screen", "The page shows an Applied card, a Health card and a Provider card", "Each card is a state block", "The Applied card shows the applied state as its word", "The Applied card composes its summary from recorded facts", "The Applied card shows the reason's meaning and message", "The Applied card marks a Reconciling condition", "The Applied card counts Warning events by reason", "The Applied card draws one dot per history entry", "The Applied card offers the history", "The Health card shows the health state as its word", "The Health card marks a partial or stale health", "The Health card counts objects by health reason", "The Provider card shows each held registration's standing", "The Provider card summary is not a verdict the portal composed", "The Provider card has a fixed sentence when the controller said nothing", "The Provider card reads Unknown for a standing it does not know", "A non-success provider reason links to the Provider tab".

## MODIFIED Requirements

### Requirement: Applied and health are two badges, never one
Every instance and package SHALL show its applied state and its health as two separate badges,
neither derived from the other. Both badges SHALL share one square shape: an uppercase word in a
bordered rectangle coloured by its state's tone, with no prefix and no dot. Each badge SHALL carry
its axis ("Applied" or "Health") in its accessible name. Source: portal:D3:R1/R5/R6/R8,
portal:D2:R3, portal:D19:R3.

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

### Requirement: Installed lists instances and packages together
`/installed` SHALL list the ModuleInstances and ModulePackages the caller may read in one table: name and namespace, a kind chip with its icon (instance and package drawn apart), the applied badge, the health badge, the object count and the owner (`controller` or `cli` for an instance, `controller` for a package). Each row SHALL be a link to the item's page, and a row whose health is Degraded SHALL be tinted apart from the others. Source: portal:D17:R1, portal:D7:R2, portal:D15:R2/R4.

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

## ADDED Requirements

### Requirement: Every fact on a page comes from a read API document
Every fact a page shows SHALL come from a read API document fetched through the read API's own
handler chain, in-process, with the caller's session, so a client holding the same identity can read
the same fact with the same filtering. Source: portal:D2:R1.

#### Scenario: The UI reads through the API

- **WHEN** a signed-in browser requests `/instances/default/podinfo`
- **THEN** the page is built from the `Instance`, `Graph` and `EventList` documents the read API
  serves for that session

#### Scenario: A condition's meaning

- **WHEN** a page shows a condition with a reason the portal explains
- **THEN** the meaning and next step it shows are the `meaning` and `nextStep` of that condition
  in the read API's document

### Requirement: The UI imports only the read API types
Among this module's packages, `internal/ui` SHALL import only `api/v1alpha1`, and a test SHALL fail
when a file of `internal/ui` imports any other package of the module. Source: portal:D2:R1.

#### Scenario: The import rule

- **WHEN** a file under `internal/ui` imports `internal/readmodel` or `internal/health`
- **THEN** the UI's test suite fails

### Requirement: A problem document renders as a region of the page
A problem document SHALL render as a region of the page: `forbidden` as locked,
`not_readable_by_portal` and `upstream_unavailable` as degraded with the code, `not_found` as a
not-found page with status `404`, `unauthenticated` as a sign-in page with status `401`. Source:
portal:D2:R1.

#### Scenario: No session

- **WHEN** a browser without the session requests `/`
- **THEN** the response is `401` with a page telling the user to open a new launch link
- **AND** no access review is sent

### Requirement: The Platform page opens on an identity card
`/` SHALL show the Platform: an identity card (name, type, and the controller version labelled "Controller"; the context, reader and Kubernetes version are in the header). The page SHALL show no Platform health and no platform graph. Source: portal:D4:R4/R7, portal:D3:R8, portal:D17, portal:D9:R2/R4, portal:D19:R3/R5.

#### Scenario: The identity card
- **WHEN** a signed-in browser opens `/`
- **THEN** the page shows an identity card with the Platform's name, type and controller version labelled "Controller", and shows no Platform health and no platform graph

### Requirement: The status block shows the applied state of Ready
`/` SHALL show a status block with the applied state of `Ready` as a large state word in its tone, the time since that state ("since", from the deciding condition), and the line of resolved catalogs, subscriptions and active claims. Source: portal:D4:R4/R7, portal:D3:R8, portal:D17, portal:D9:R2/R4, portal:D19:R3/R5.

#### Scenario: The status word
- **WHEN** a signed-in browser opens `/`
- **THEN** the status block shows the applied state of `Ready` as a large state word in its tone, with its "since" time and the line of resolved catalogs, subscriptions and active claims

### Requirement: The status block counts refusals and removal blocks by reason
The status block SHALL list the reasons that need attention, each a count and a reason linking to
its cause: per refusal reason, the number of registrations refused with it, linking to the Providers
tab filtered to refused; per removal-blocked reason, the same, linking to it filtered to removal
blocked. Source: portal:D4:R4/R7, portal:D3:R8, portal:D17, portal:D9:R2/R4, portal:D19:R3/R5.

#### Scenario: A refusal on the status block

- **WHEN** a signed-in browser opens `/` on the F1 capture
- **THEN** the status block reads "Applied" with a "since" time, and shows the count 1 with the
  reason `CatalogUnresolved` linking to `/?tab=providers&pstatus=refused#parts`

### Requirement: The status block names why Ready is not True
The status block SHALL show the `Ready` reason (`BuildFailed`, `GenerateFailed`,
`ContractCollisions`, `OverSubscribedContracts`, `ComparablePredicates`) when `Ready` is not `True`,
linking to its condition with the meaning, next step and message. Source: portal:D4:R4/R7,
portal:D3:R8, portal:D17, portal:D9:R2/R4, portal:D19:R3/R5.

#### Scenario: The Ready reason
- **WHEN** the Platform's `Ready` is not `True`
- **THEN** the status block shows the `Ready` reason, linking to its condition with the meaning, next step and message

### Requirement: Unfulfilled provider contracts show as information
The status block SHALL show `ContractsFulfilled`'s reason when it is `False`, as information without
a count, linking to the contracts note. A Platform reporting unfulfilled provider contracts SHALL
show it as information, not as a failure. Source: portal:D4:R4/R7, portal:D3:R8, portal:D17,
portal:D9:R2/R4, portal:D19:R3/R5.

#### Scenario: Contracts unfulfilled on the status block
- **WHEN** the Platform's `ContractsFulfilled` is `False`
- **THEN** the status block shows its reason as information without a count, linking to the contracts note, and not as a failure

### Requirement: The Installed card counts by health and applied state
`/` SHALL show an Installed card with one column of count rows per health state and one per applied state, each row a badge and its count linking to Installed filtered by it, and the numbers of instances, packages and of those that are providers. Source: portal:D4:R4/R7, portal:D3:R8, portal:D17, portal:D9:R2/R4, portal:D19:R3/R5.

#### Scenario: A count hands off

- **WHEN** the user follows the Degraded count on the Installed card
- **THEN** the browser opens `/installed?health=Degraded`

### Requirement: The Providers tab shows each registration's standing
`/` SHALL show a Providers tab with filters. A Providers row SHALL show the registration's name linking to its holder's Provider tab when it has one, its catalog and version linking to the Catalog page, its provided contracts, its holder ("Installed as") with kind and link, its acceptance and its activation as two separate badges, and its refusal or removal-blocked reason and message. Source: portal:D4:R4/R7, portal:D3:R8, portal:D17, portal:D9:R2/R4, portal:D19:R3/R5.

#### Scenario: Accepted and active apart

- **WHEN** a registration is accepted and not active
- **THEN** its row shows an "Accepted" badge and an "Inactive" badge, each on its own

### Requirement: A removal-blocked registration is not shown as refused
A removal-blocked registration SHALL show as removal blocked while still accepted and active, never
as refused. Source: portal:D4:R4/R7, portal:D3:R8, portal:D17, portal:D9:R2/R4, portal:D19:R3/R5.

#### Scenario: Removal blocked keeps acceptance and activation

- **WHEN** a registration's verdict is `RemovalBlocked`
- **THEN** its row shows "Accepted", "Active" and "Removal blocked" badges with its reason and
  message, and no "Refused" badge

### Requirement: The Catalogs tab shows each catalog's source and claimants
`/` SHALL show a Catalogs tab with filters. A Catalogs row SHALL show the path linking to the Catalog page, version, source ("subscription", "contributed" or "claimed only"), claimants linking to their holders' Provider tabs, and enablement as a badge. Source: portal:D4:R4/R7, portal:D3:R8, portal:D17, portal:D9:R2/R4, portal:D19:R3/R5.

#### Scenario: The F1 platform

- **WHEN** a signed-in browser opens `/` on the F1 capture
- **THEN** its Catalogs tab shows the `opmodel.dev/catalogs/opm` subscription with its version
  and the backup catalog contributed by the accepted, active `backup-provider` claim, and its
  Providers tab shows the refused claim with its verdict Refused, its reason and its message

### Requirement: The Platform events merge the Platform's and its registrations' events
`/` SHALL show the recent events of the Platform and of each readable registration, merged newest first, as a table with a resource filter whose options carry their counts. An events row SHALL show type, reason, the object it is about as a link that filters the feed to that object, the note with the repeat count and reporting controller, and the age. Source: portal:D4:R4/R7, portal:D3:R8, portal:D17, portal:D9:R2/R4, portal:D19:R3/R5.

#### Scenario: An event's resource filters the feed

- **WHEN** the user follows the resource link of an event about the TransformerRegistration
  `default.refused-claim-fixture`
- **THEN** the browser opens `/` with `eresource=registration:default.refused-claim-fixture` and
  the table holds only that registration's events

### Requirement: The Platform page says what the caller could not read
When the registrations the caller may read are not all of them, or a list behind the Installed
counts is forbidden, the page SHALL say so with the locked style, whatever other reasons the status
block shows, and SHALL never show a refusal count or an active-claims count of zero for
registrations it could not read. Source: portal:D4:R4/R7, portal:D3:R8, portal:D17, portal:D9:R2/R4,
portal:D19:R3/R5.

#### Scenario: Instances forbidden

- **WHEN** the caller may not list ModuleInstances
- **THEN** the Installed card shows the instance counts locked, not zero

#### Scenario: Registrations forbidden on the status block

- **WHEN** the caller may get the Platform but may not list TransformerRegistrations
- **THEN** the status block says the refusals are locked and shows no refusal count, and its
  summary shows no "0 claims"

#### Scenario: Registrations forbidden while Ready is not True

- **WHEN** the caller may get the Platform but may not list TransformerRegistrations, and the
  Platform's `Ready` is `False`
- **THEN** the status block shows the `Ready` reason and, in the locked style, that the refusals
  are locked

### Requirement: A badge stands where visible text names its axis
Wherever an applied badge and a health badge are shown together, each SHALL stand where visible text
names its axis: a column head, a group label, a card's heading or eyebrow, or a label beside the
badge. A health stamp shown alone on a per-object line, which carries one axis only, SHALL name it
in its accessible name. Source: portal:D3:R1/R5/R6/R8, portal:D2:R3, portal:D19:R3.

#### Scenario: A health stamp alone
- **WHEN** a health stamp is shown alone on a per-object line, which carries one axis only
- **THEN** the stamp's accessible name names its axis

### Requirement: The applied badge reads the controller's state words
`Ready=True` SHALL be shown as "Applied" and never as healthy, and the applied badge SHALL read the controller's state words (Applied, Reconciling, Failed, Stalled, Suspended, Managed externally), never "Ready" or "Not ready". `ManagedExternally` SHALL render in a neutral style without an error colour. Source: portal:D3:R1/R5/R6/R8, portal:D2:R3, portal:D19:R3.

#### Scenario: Ready is shown as Applied
- **WHEN** an instance's `Ready` is `True`
- **THEN** its applied badge reads "Applied" and the page does not show it as healthy

### Requirement: A partial or stale health is marked apart
A partial health SHALL say partial and be marked visually apart from a full health by more than
colour; a health with an object refreshed by polling SHALL say it is not live and show when it was
evaluated. An enum value the UI does not know SHALL render as "unknown", never as an error. Source:
portal:D3:R1/R5/R6/R8, portal:D2:R3, portal:D19:R3.

#### Scenario: A health refreshed by polling
- **WHEN** a health has an object refreshed by polling
- **THEN** its badge says it is not live and shows when it was evaluated

### Requirement: The red family marks only failures
The red family SHALL be used only for degraded, failed, stalled and refused states, broken edges,
Warning events and a lost stream; reasons, versions, kinds and other decoration SHALL not use it. A
condition SHALL be coloured by its `tone`, never by its status alone. Source: portal:D3:R1/R5/R6/R8,
portal:D2:R3, portal:D19:R3.

#### Scenario: Decoration is not red
- **WHEN** a page shows a reason, a version or a kind
- **THEN** it does not use the red family

#### Scenario: A condition is coloured by its tone
- **WHEN** a page shows a condition
- **THEN** the condition is coloured by its `tone`, not by its status alone

### Requirement: The instance page shows its graph, resources and conditions
The instance page SHALL show: the instance's graph; its inventory objects and runtime children with
their health, in the Resources tab; its conditions, with each known reason's meaning and next step
beside the controller's message, in a Conditions fold below the tab panels. Source: portal:D4:R3,
portal:D9:R2/R3/R5/R6, portal:D10, portal:D16:R1.

#### Scenario: podinfo's page

- **WHEN** a signed-in browser opens `/instances/default/podinfo` on the F1 capture
- **THEN** across its tabs it shows the podinfo component with its Service and Deployment, the
  ReplicaSet and two Pods below, the conditions with their meanings, the history, the render
  contracts as text, the events feed with its expiry note, and a log choice for each Pod
  container

### Requirement: The instance page shows its history, contracts and events
The instance page SHALL show: its status history, newest first, behind "Show history"; the render
contracts as plain text, labelled as the contracts the render used and not the instance's provider
demand; its recent events. Source: portal:D4:R3, portal:D9:R2/R3/R5/R6, portal:D10, portal:D16:R1.

#### Scenario: History, contracts and events
- **WHEN** a signed-in browser opens an instance page
- **THEN** the page shows the status history newest first behind "Show history", the render contracts as plain text labelled as the contracts the render used and not the instance's provider demand, and the instance's recent events

### Requirement: The package page shows the same for a ModulePackage
The package page SHALL show the same as the instance page for a ModulePackage (its graph, Resources,
Conditions, status history, render contracts and recent events), with its source, interval, prune
setting, source revision and dependencies. Source: portal:D4:R3, portal:D9:R2/R3/R5/R6, portal:D10,
portal:D16:R1.

#### Scenario: The package page record
- **WHEN** a signed-in browser opens the page of a ModulePackage
- **THEN** the page shows the same as the instance page, with the package's source, interval, prune setting, source revision and dependencies

### Requirement: The Resources tab folds configuration components and idle ReplicaSets
Configuration components SHALL be folded in the Resources tab into the same groups the graph folds
them into, each with its count and worst health, and a group SHALL be open when any member is not
healthy. ReplicaSets with zero desired replicas SHALL be folded apart from the live ones. Source:
portal:D4:R3, portal:D9:R2/R3/R5/R6, portal:D10, portal:D16:R1.

#### Scenario: cert-manager's configuration components

- **WHEN** a signed-in browser opens `/instances/cert-manager/cert-manager?tab=resources` on the
  F1 capture
- **THEN** its configuration components are folded into the same closed groups the graph shows,
  each with its count and health, and its workload components are listed one by one

### Requirement: The Events tab merges the events of everything the inventory reaches
The Events tab SHALL open on the events of the instance and of every object and runtime child its
inventory reaches that the caller may read, merged newest first, with repeats folded into one line
with a count and the latest time. Source: portal:D4:R3, portal:D9:R2/R3/R5/R6, portal:D10,
portal:D16:R1.

#### Scenario: Every reached object's events on arrival

- **WHEN** a signed-in browser opens `/instances/default/podinfo?tab=events` on the F1 capture
- **THEN** with no resource filter the list holds the instance's own events and the Deployment's
  `ScalingReplicaSet` event, newest first

#### Scenario: A long event note

- **WHEN** an event in the merged feed carries a note of 2,000 code points
- **THEN** the table shows the note as the API served it, capped, and the page clips it no further

#### Scenario: Events of both operator writers in-cluster

- **WHEN** an in-cluster browser opens the Events tab of an instance whose feed holds an event from
  `opm-controller` and an event with no controller that regards the instance, beside the kubelet's
  events about its Pods
- **THEN** neither of the first two shows a note, and the kubelet's events show theirs

### Requirement: The Events tab says when events could not be read
When some reached objects' events could not be read, the Events tab SHALL say that they are left
out, never show their absence as no events. Source: portal:D4:R3, portal:D9:R2/R3/R5/R6, portal:D10,
portal:D16:R1.

#### Scenario: Events of one namespace not readable

- **WHEN** the caller may list events in the instance's namespace but not in `default`, where the
  events about the instance's cluster-scoped objects live, and opens its Events tab
- **THEN** the tab lists the events it could read and says that some objects' events could not
  be read and are left out

#### Scenario: Events of a caller who cannot list the Pods

- **WHEN** a caller who may get the instance and list events in `default` but may not list Pods
  opens `/instances/default/podinfo?tab=events` on the F1 capture
- **THEN** no ReplicaSet, Pod or Job is offered in the Resource choice or listed in the table, the
  Events tab carries no count, "All resources" says its count covers what could be read, and no
  count in the filters includes an event about one of them

### Requirement: The feed is labelled as recent activity
The feed SHALL be labelled as recent activity that expires after about an hour, and the page SHALL
say that render warnings are kept only as events. Source: portal:D4:R3, portal:D9:R2/R3/R5/R6,
portal:D10, portal:D16:R1.

#### Scenario: The feed note
- **WHEN** a signed-in browser opens the Events tab of an instance page
- **THEN** the feed is labelled as recent activity that expires after about an hour, and the page says that render warnings are kept only as events

### Requirement: The Logs tab follows one container's log
The Logs tab SHALL offer each container of each Pod an inventory reaches through one Pod and
container picker. It SHALL show the picked container, or the first one when none is picked, in one
pane that follows its log topic as soon as the tab opens, and SHALL render log text as text only.
The details panel of a Pod SHALL link to its log. Source: portal:D4:R3, portal:D9:R2/R3/R5/R6,
portal:D10, portal:D16:R1.

#### Scenario: Logs open on arrival

- **WHEN** a browser with script opens podinfo's Logs tab
- **THEN** the first Pod's first container is picked and its pane follows its log without a
  further click

### Requirement: The Logs tab says when a container has not started or is restarting
When the picked container waits with a reason under which no instance of it has started (or, where
the Pod does not say per container, its Pod does), the Logs tab SHALL say the container has not
started, so there are no logs yet, and link to that Pod's events. When the picked container is in
`CrashLoopBackOff`, the tab SHALL say it is restarting and point to its previous instance's logs.
Source: portal:D4:R3, portal:D9:R2/R3/R5/R6, portal:D10, portal:D16:R1.

#### Scenario: A container that has not started

- **WHEN** the picked container belongs to a Pod waiting with `ImagePullBackOff`
- **THEN** the Logs tab says the container has not started, so there are no logs yet, and links
  to the Events tab filtered to that Pod

#### Scenario: A crash-looping container

- **WHEN** the picked container waits with `CrashLoopBackOff`
- **THEN** the Logs tab says the container is restarting and points to its previous instance's
  logs, and does not say it has not started

### Requirement: Each graph renders server-side as SVG
Each graph SHALL render server-side as SVG from the read API's laid-out graph document, with no edge
from an instance to a contract. Edges SHALL be drawn as orthogonal elbows through the document's
route points, without arrowheads. Graphs SHALL appear on instance and package pages; the Platform
page SHALL not draw one. The instance page SHALL not draw the module node; the module's path and
version stay on the page's identity card. Source: portal:D4:R1/R3/R5, portal:D3:R1, portal:D17,
portal:D19.

#### Scenario: Elbow edges

- **WHEN** any instance graph is rendered
- **THEN** every edge path is made of horizontal and vertical segments and carries no arrowhead

### Requirement: Graph nodes are named and reachable with the keyboard
Nodes SHALL carry CSS classes for kind, health and access, an accessible name, and a title with
their full label, and SHALL be reachable with the keyboard in document order. A label too long for
its node SHALL keep the part that tells nodes apart. Source: portal:D4:R1/R3/R5, portal:D3:R1,
portal:D17, portal:D19.

#### Scenario: Keyboard focus

- **WHEN** the user tabs into podinfo's graph
- **THEN** focus moves through the nodes in column order and each announces its kind, name and
  health

### Requirement: A graph node shows three lines
A node SHALL show three lines: its kind, its name, and a status line (its health reason or state, a
ReplicaSet's desired count, a component's object count, a group's object and kind counts, a
registration's standing or a source's state). Source: portal:D4:R1/R3/R5, portal:D3:R1, portal:D17,
portal:D19.

#### Scenario: A ReplicaSet's status line

- **WHEN** the image-break podinfo graph is rendered
- **THEN** the new ReplicaSet's node shows its kind, its name and its desired replica count, and is
  drawn dashed

### Requirement: A node's fill, outline and mark carry its state
A node's fill and outline SHALL carry its health, except that a TransformerRegistration node's fill
and outline SHALL carry the claim's standing (a source node's health is mapped from the source's
state); a separate square mark on the node SHALL carry its applied state, and is the only applied
mark; nodes the cluster made below an inventory object SHALL be drawn dashed. Source:
portal:D4:R1/R3/R5, portal:D3:R1, portal:D17, portal:D19.

#### Scenario: Two axes on a node

- **WHEN** an instance node is applied and degraded
- **THEN** its fill and outline are drawn as degraded and its applied mark as applied

### Requirement: A graph can be selected, fitted, panned and zoomed
Activating a node SHALL show its detail panel and mark the node as selected; activating a group node
SHALL show it expanded. A graph SHALL open fitted to its frame, SHALL pan with the pointer and SHALL
zoom with a zoom slider, Fit and Ctrl/Cmd-wheel. Source: portal:D4:R1/R3/R5, portal:D3:R1,
portal:D17, portal:D19.

#### Scenario: Selecting and zooming a graph
- **WHEN** a user activates a node of a graph, or activates a group node, or zooms the graph
- **THEN** the detail panel of the node shows and the node is marked as selected, the group node shows expanded, and a graph opened fitted to its frame pans with the pointer and zooms with the zoom slider, Fit and Ctrl/Cmd-wheel

### Requirement: A graph at phone width does not scroll the page
At a 360 px wide viewport the page SHALL NOT scroll sideways; the graph's frame SHALL take the
graph's extra width. Source: portal:D4:R1/R3/R5, portal:D3:R1, portal:D17, portal:D19.

#### Scenario: A graph at phone width

- **WHEN** a browser 360 px wide opens cert-manager's Graph tab
- **THEN** the page does not scroll sideways, and the graph's frame scrolls to reach the part of
  the graph beyond its width

### Requirement: A tab holds one stream across navigation
A tab SHALL hold one `EventSource` on the read API's stream across navigation, and each page SHALL
add the topics it needs and remove the ones it no longer needs through the topic-change request.

#### Scenario: Navigating keeps the stream

- **WHEN** the user moves from `/` to `/instances/default/podinfo`
- **THEN** the same stream drops `platform` and adds `instance:default/podinfo` and
  `events:instance:default/podinfo`

### Requirement: A change on a followed topic re-renders the open page
A change on a followed topic SHALL re-render every region of the open page tab that shows it, and
the summary cards, from one fetch of the page, so the regions of one refresh come from the same
moment. A refresh SHALL keep the open page tab, the `focus` parameter, the selected node, the zoom,
an open hover card and open groups; a page tab not shown SHALL be rendered fresh when opened.

#### Scenario: An image break on the open page

- **WHEN** podinfo's image is changed to a tag that does not exist while its page is open on the
  Graph tab
- **THEN** without a reload the graph shows the new ReplicaSet and the Pod waiting on the image,
  the Health card reads Degraded with a count for the waiting reason and the Applied card still
  reads Applied, and the Graph tab stays open with its zoom
- **AND** opening the Resources tab then lists the new ReplicaSet and the waiting Pod

### Requirement: UI pages carry a strict Content-Security-Policy
UI pages SHALL be served with
`Content-Security-Policy: default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'`,
with no inline script, no inline style attribute and no `eval`. Source: portal:D14:R1/R4.

#### Scenario: The page policy

- **WHEN** any UI page is served
- **THEN** it carries that policy, and no element in it has an inline `style` attribute, an
  inline script or an event-handler attribute

### Requirement: Script sets positions and sizes only through CSSOM or SVG attributes
Script SHALL set positions and sizes only through CSSOM properties or SVG attributes, never a
`style` attribute. Source: portal:D14:R1/R4.

#### Scenario: No style attribute from script
- **WHEN** a page script sets a position or a size
- **THEN** it does so through CSSOM properties or SVG attributes and never through a `style` attribute

### Requirement: Untrusted text renders only as text
Untrusted text (messages, notes, labels, log lines) SHALL be rendered only through `html/template`
or as text nodes, and a test SHALL fail when a page renders such text as markup or when the UI's
code converts a string to a trusted template type or assigns markup in a page script. Source:
portal:D14:R1/R4.

#### Scenario: Hostile cluster text

- **WHEN** a condition message, history entry, event note or annotation holds
  `<script>alert(1)</script>`
- **THEN** every page and fragment shows it as text, escaped

### Requirement: Assets are served from the binary at pinned versions
Scripts, styles and fonts SHALL be served from the binary, vendored at pinned versions whose SHA-256
a test checks. Source: portal:D14:R1/R4.

#### Scenario: A tampered asset

- **WHEN** a vendored file differs from its recorded checksum
- **THEN** the UI's test suite fails

### Requirement: Browser storage holds only the theme and the filter queries
Browser storage SHALL hold only the theme choice and each list view's filter query, every access
guarded so a throwing storage changes nothing else. Source: portal:D14:R1/R4.

#### Scenario: What the browser stores

- **WHEN** a session has visited every page and used every filter and the theme menu
- **THEN** browser storage holds only the theme key and one filter query per list view, and no
  document field, username or token

### Requirement: Pages follow the chosen theme and hold their width at 360 px
Pages SHALL follow the theme chosen in the browser, or the OS light or dark preference under System,
and SHALL not scroll horizontally at a 360 px wide viewport. Source: portal:D14:R1/R4.

#### Scenario: Theme and phone width
- **WHEN** a page is served to a browser with a chosen theme, or under System, at a 360 px wide viewport
- **THEN** the page follows the chosen theme, or the OS light or dark preference under System, and does not scroll horizontally

### Requirement: In-cluster pages omit the operator's messages
Pages served over an `in-cluster` read API SHALL show no condition message (the conditions of a
registration included), reconcile message, history message, registration message or note of an event
the operator reported, because the read API does not serve them. Source: owner answer to portal:OQ8
(portal:D8:R5); its scope is the supervisor ruling recorded under portal:D8.

#### Scenario: The broken podinfo page in-cluster

- **WHEN** the instance page of the image-break sample is rendered over an in-cluster read API
- **THEN** it shows the applied and health badges, each condition's reason, meaning and next
  step, and no message text the operator wrote

#### Scenario: Attempt dots and the Provider tab in-cluster

- **WHEN** backup-provider's page is rendered over an in-cluster read API
- **THEN** its attempt dots keep their outcomes, its registration conditions show type, status,
  reason and meaning, and no message text the operator wrote appears

### Requirement: In-cluster pages keep reasons, standings and other writers' notes
In-cluster pages SHALL still show every reason, state, history outcome, provider standing and the
portal's meaning and next step, and SHALL show the kubelet's and other controllers' event notes and
the health messages of rendered workloads as local mode does. Source: owner answer to portal:OQ8
(portal:D8:R5); its scope is the supervisor ruling recorded under portal:D8.

#### Scenario: A Pod's events in-cluster

- **WHEN** the events of a podinfo Pod are rendered over an in-cluster read API
- **THEN** the kubelet's and the scheduler's notes are shown as they wrote them

### Requirement: An expired session closes the stream and marks the live mark
When the stream ends with an `expired` event, the page SHALL close its `EventSource`, so it does not
reconnect, and SHALL show in the live mark that the session expired and the page must be reloaded,
in the live mark's capitalised wording. The page SHALL keep showing what it last rendered.

#### Scenario: The session expires while a page is open

- **WHEN** the session of an open page expires
- **THEN** the live mark reads "Session expired, reload"
- **AND** the banner reads "Session expired. Showing data read" followed by the age of the data on
  screen, the last message the stream delivered before the session expired, as a `time` element (relative text, with the instant in
  `datetime` and in UTC in `title`), with a Reload link
- **AND** the page makes no further stream request

### Requirement: An expired session shows a banner with the time of the data
When the stream ends with an `expired` event, the page SHALL show a visible banner in the sticky
header that says the session expired, states the time of the data the page shows in the form of the
`time` partial (relative text, the ISO time in `datetime`, the absolute time in UTC in `title`), and
carries a Reload link that loads the page afresh. The banner SHALL be neither a live region nor
given a role: the live mark SHALL stay the one announcer.

#### Scenario: Reload after the session expired

- **WHEN** the user activates the banner's Reload link
- **THEN** the browser loads the current address as a new page, with a new stream

### Requirement: Every page carries a sticky header
Every page SHALL carry a sticky header with the main navigation (Platform, Installed), the cluster
the portal reads, "reading as" with the caller's username, the Kubernetes version, the live mark and
the theme menu, the facts read from the read API's `Cluster` document. The header SHALL be the one
place these facts show. Source: portal:D18, portal:D17:R1.

#### Scenario: The F1 header

- **WHEN** a signed-in browser opens any page on the F1 capture in local mode
- **THEN** the header shows Platform and Installed, the context name, "reading as" with the
  kubeconfig's username, the Kubernetes version and the live mark

### Requirement: The header names the cluster and its entry name
The header SHALL name the cluster by the kubeconfig context and, when the `Cluster` document carries
a cluster entry name that differs from the context name, that name beside it with the word
"cluster"; or "in cluster". The cluster entry name SHALL come from the `Cluster` document alone: the
header SHALL read nothing else for it. Source: portal:D18, portal:D17:R1.

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

#### Scenario: In a Pod

- **WHEN** the portal runs local mode in a Pod with in-cluster credentials
- **THEN** the header reads "in cluster" in place of a context name, with no cluster entry name

### Requirement: A version the document does not carry reads unknown
A version the `Cluster` document does not carry SHALL read "unknown". Source: portal:D18,
portal:D17:R1.

#### Scenario: Version not read

- **WHEN** the `Cluster` document carries no `kubernetesVersion`
- **THEN** the header's version reads "unknown", not empty

### Requirement: The header survives a failed Cluster document
When the `Cluster` document cannot be fetched, the header SHALL still render its navigation, live
mark and theme menu, with the cluster, reader and version reading "unknown" in the degraded style,
and no cluster entry name. Source: portal:D18, portal:D17:R1.

#### Scenario: The cluster document fails

- **WHEN** the in-process fetch of the `Cluster` document fails
- **THEN** the page renders with its navigation and theme menu, and the header's cluster, reader
  and version read "unknown" in the degraded style, with no cluster entry name

### Requirement: The header slims on scroll
The header SHALL slim after the page scrolls 48 px and SHALL not animate under
`prefers-reduced-motion`. Source: portal:D18, portal:D17:R1.

#### Scenario: The header slims
- **WHEN** the page scrolls 48 px
- **THEN** the header slims, and does not animate under `prefers-reduced-motion`

### Requirement: An Installed row shows a provider badge with a tooltip
The Installed table SHALL show a provider badge reading "Provider" when the item holds a
TransformerRegistration, coloured by its standing and opening, on hover, on keyboard focus and on a
tap, a tooltip that names the registration and its standing in words, that Escape closes and that
the pointer can enter. Source: portal:D17:R1, portal:D7:R2, portal:D15:R2/R4.

#### Scenario: The provider badge
- **WHEN** an Installed item holds a TransformerRegistration
- **THEN** its row shows a badge reading "Provider", coloured by the registration's standing, with a tooltip that names the registration and its standing in words

### Requirement: An Installed row shows the module or the package source
The Installed table SHALL show the module path with its version on a line below, or the package's
source as `<Kind> <namespace>/<name>` with its source revision's short digest on a line below or "no
revision recorded" when the controller recorded none, and its path only when it is neither empty nor
`.`. Source: portal:D17:R1, portal:D7:R2, portal:D15:R2/R4.

#### Scenario: A module's path and version
- **WHEN** the Installed list shows an instance
- **THEN** its row shows the module path with its version on a line below

### Requirement: An Installed list with no match offers to clear the filters
When filters match nothing, the Installed list SHALL say so and offer a button that clears them.
Source: portal:D17:R1, portal:D7:R2, portal:D15:R2/R4.

#### Scenario: Filters match nothing
- **WHEN** the filters of the Installed list match nothing
- **THEN** the list says so and offers a button that clears them

### Requirement: A kind the caller may not list renders locked
A kind the caller may not list SHALL render locked in the Installed list for that kind only, with no
count, while the other kind's rows show. Source: portal:D17:R1, portal:D7:R2, portal:D15:R2/R4.

#### Scenario: A kind forbidden
- **WHEN** the caller may not list ModuleInstances but may list ModulePackages
- **THEN** the instances show locked with no count and the packages' rows show

### Requirement: The old list addresses redirect to Installed
`/instances` and `/packages` SHALL redirect with `308` to `/installed` with `kind=instance` or `kind=package`, keeping a `namespace` parameter. Source: portal:D17:R1, portal:D7:R2, portal:D15:R2/R4.

#### Scenario: An old packages link
- **WHEN** a browser requests `/packages`
- **THEN** the response is `308` to `/installed?kind=package`

### Requirement: List filters read the URL query
The Installed list, the Platform page's Providers and Catalogs tabs and its events, and the Events
and Resources tabs of instance and package pages SHALL filter from their URL query, server-side, so
a filtered view is a plain link that works without script. Source: portal:D14:R1/R2/R3,
portal:D16:R2.

#### Scenario: Without script

- **WHEN** a browser with script disabled opens `/installed?kind=package`
- **THEN** the list holds only packages

### Requirement: Installed offers its filters as removable chips
Installed SHALL offer search, kind, provider, uses, namespace, health, applied, owner and module;
each active filter SHALL show as a chip whose link removes it. Source: portal:D14:R1/R2/R3,
portal:D16:R2.

#### Scenario: Active filters show as chips
- **WHEN** a browser opens `/installed?health=Degraded`
- **THEN** the filter shows as a chip whose link removes it

### Requirement: An unknown filter value is ignored with a note
An enumerated filter value the view does not know SHALL be ignored with a note naming it, and never
stored. Source: portal:D14:R1/R2/R3, portal:D16:R2.

#### Scenario: An unknown filter value

- **WHEN** a browser opens `/installed?health=Bogus`
- **THEN** the list is unfiltered by health, a note says the filter `health=Bogus` was ignored,
  and nothing about it is stored

### Requirement: Only the list views remember their filters
Only the list views SHALL remember their filters: Installed and the Providers and Catalogs tabs,
each on its own; the Platform events and instance and package pages SHALL not. `tab` and `focus` are
not filters: they SHALL never be stored and SHALL not count as a filter in the URL. Source:
portal:D14:R1/R2/R3, portal:D16:R2.

#### Scenario: A tab is not a filter

- **WHEN** a browser that remembered `pstatus=refused` for the Providers tab opens `/?tab=providers`
- **THEN** the page shows `/?tab=providers&pstatus=refused`

### Requirement: A filter in the URL wins over the remembered filters
When the URL carries any of a remembered view's filter parameters, the view SHALL show exactly
those. Source: portal:D14:R1/R2/R3, portal:D16:R2.

#### Scenario: A link wins over the remembered filters

- **WHEN** a browser that remembered `health=Healthy` for Installed opens
  `/installed?health=Degraded`
- **THEN** the list shows only Degraded items, one chip reads Degraded, and Installed now
  remembers `health=Degraded`

### Requirement: A remembered view opens with its stored filters
When the URL carries none of a remembered view's filter parameters and the browser holds valid
filters for that view, the view SHALL open with them and the URL SHALL show them, without painting
the unfiltered view first, on a full load and on a boosted navigation alike; no other request SHALL
be rewritten. Source: portal:D14:R1/R2/R3, portal:D16:R2.

#### Scenario: Remembered filters restored

- **WHEN** a browser that remembered `namespace=default` for Installed follows the Installed
  link in the header
- **THEN** the page shows `/installed?namespace=default` and was never painted unfiltered

### Requirement: A stored query is checked and only the shown filters are stored
A stored query SHALL be checked against the view's parameters and values, and what does not fit
SHALL be dropped. After rendering, a remembered view SHALL store the filters it shows and nothing
else. Source: portal:D14:R1/R2/R3, portal:D16:R2.

#### Scenario: A stale stored query

- **WHEN** the stored Installed query holds `health=Bogus&namespace=default` and the browser opens
  `/installed`
- **THEN** the page shows `/installed?namespace=default`

### Requirement: Stored filters are kept per context
Stored filters SHALL be kept per kubeconfig context (or the in-cluster source), so filters stored
while reading one context are never restored while reading another. Source: portal:D14:R1/R2/R3,
portal:D16:R2.

#### Scenario: Another context

- **WHEN** a browser remembered `namespace=team-a` for Installed while the portal read context
  `prod`, and the portal is restarted on the same address with `--context staging`
- **THEN** `/installed` opens unfiltered

### Requirement: The uses filter matches render contracts and leaves packages out
The `uses` filter SHALL match an instance's render contracts, SHALL leave packages out and SHALL say
that packages do not record what they use. Source: portal:D14:R1/R2/R3, portal:D16:R2.

#### Scenario: Uses leaves packages out

- **WHEN** a browser opens `/installed?uses=opmodel.dev/catalogs/opm/traits/backup@v1alpha1` on
  the F1 capture
- **THEN** the list holds `default/backup-consumer` and no package, and says packages are not
  included because they do not record what they use

### Requirement: Instance and package pages offer their tabs in order
Below the cards the page SHALL offer the tabs Graph, Provider when the item holds a registration,
Resources, Events, Logs and YAML, in that order. Each tab SHALL be a link carrying `tab=` that the
server renders, and SHALL keep the `node` and `focus` parameters. Source: portal:D8:R2,
portal:D9:R3/R6, portal:D10, portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: Tab order with a provider

- **WHEN** a browser opens `/instances/default/backup-provider` on the F1 capture
- **THEN** the tabs read Graph, Provider, Resources, Events, Logs and YAML, in that order

### Requirement: The details panel rests on the node the graph rests on
The details panel SHALL show only on Graph and Resources, beside the tab's region where the page is
wide enough. With neither `node` nor `focus` set, it SHALL show the details of the node the graph
rests on (the item's own node, unless the graph's resting rule picks a held registration whose claim
needs attention), without spotlighting it. Source: portal:D8:R2, portal:D9:R3/R6, portal:D10,
portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: The panel at rest

- **WHEN** a signed-in browser opens `/instances/default/podinfo` with no `focus`
- **THEN** the details panel shows the ModuleInstance `default/podinfo` with its identity facts
  and an Events link, and no node is spotlighted

### Requirement: Clear selection returns the details panel to the resting node
Clear selection SHALL return the details panel to the node the graph rests on. The panel SHALL never
be an empty placeholder. Source: portal:D8:R2, portal:D9:R3/R6, portal:D10, portal:D14:R7,
portal:D7, portal:D11:R7.

#### Scenario: Clear selection returns to the resting node

- **WHEN** the user selects podinfo's Deployment node and then Clear selection, with script on
- **THEN** the details panel shows the node the graph rests on again, not an empty placeholder

### Requirement: The details panel shows a node's kind, state and object facts
The panel SHALL show the node's kind, name and state badges, its health message in a box toned by
its health, and its facts: for a Deployment, StatefulSet, DaemonSet, ReplicaSet, Pod or Service, the
replica, image, strategy, node, restart, start, type and port facts its object document carries.
Source: portal:D8:R2, portal:D9:R3/R6, portal:D10, portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: Object facts in the panel

- **WHEN** the panel opens on podinfo's Deployment
- **THEN** it shows the desired replicas and the container image from the Deployment object, and
  no environment variable

### Requirement: A Service's Endpoints fact reads the recorded ready count
For a Service the panel SHALL also show an Endpoints fact. When its graph node carries
`endpointsReady` the fact SHALL read "N ready" and nothing more: the count the cluster records in
the Service's EndpointSlices, which is not proof that the Service works. Source: portal:D8:R2,
portal:D9:R3/R6, portal:D10, portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: A Service's ready endpoints

- **WHEN** the panel opens on podinfo's Service and its graph node carries `endpointsReady` 2
- **THEN** the facts list ends with an Endpoints fact that reads "2 ready"

#### Scenario: A Service with no ready endpoint

- **WHEN** the panel opens on a Service whose graph node carries `endpointsReady` 0
- **THEN** the Endpoints fact reads "0 ready", in the same words and without a tone

#### Scenario: Nothing but a number

- **WHEN** the panel opens on a Service whose slices hold addresses, a hostname, a node name and
  Pod names
- **THEN** none of those strings appears anywhere in the page

#### Scenario: Endpoints at 360 px

- **WHEN** a browser 360 px wide opens the panel on a Service whose Endpoints fact reads "2 ready"
  or "Could not be read"
- **THEN** the page does not scroll sideways

### Requirement: A Service's Endpoints fact says when endpoints could not be read
When the Service's graph node carries `endpointsUnread` the Endpoints fact SHALL read "Could not be
read", the words of the Pods line, and never "0 ready". Source: portal:D8:R2, portal:D9:R3/R6,
portal:D10, portal:D14:R7, portal:D7, portal:D11:R7.

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

### Requirement: A Service with no recorded slice has no Endpoints fact
When the Service's graph node carries neither `endpointsReady` nor `endpointsUnread`, because the
cluster records no slice, the Endpoints fact SHALL be left out. Source: portal:D8:R2,
portal:D9:R3/R6, portal:D10, portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: A Service with no recorded slice

- **WHEN** the caller and the portal may list EndpointSlices and the cluster records none for the
  Service
- **THEN** the panel shows the Service's other facts and no Endpoints line

### Requirement: The details panel of a component says it is not a Kubernetes object
For a component, the panel SHALL say that a component is not a Kubernetes object and has no events
or YAML of its own, and what it renders. Source: portal:D8:R2, portal:D9:R3/R6, portal:D10,
portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: A component's panel
- **WHEN** the details panel opens on a component
- **THEN** it says the component is not a Kubernetes object and has no events or YAML of its own, and what it renders

### Requirement: The details panel of the item's own node shows identity facts
For the item's own node, the panel SHALL show its identity facts and an Events link, and no link to
the page already open. Source: portal:D8:R2, portal:D9:R3/R6, portal:D10, portal:D14:R7, portal:D7,
portal:D11:R7.

#### Scenario: The item's own node
- **WHEN** the details panel opens on the item's own node
- **THEN** it shows its identity facts and an Events link, and no link to the page already open

### Requirement: Panel facts are a fixed list for nodes the caller may read
The facts SHALL be a fixed list and SHALL appear only for a node the caller may read: a node that is
locked, forbidden or not readable SHALL carry none, and no object SHALL be read for it. Source:
portal:D8:R2, portal:D9:R3/R6, portal:D10, portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: A node the caller cannot read

- **WHEN** the panel opens on a node whose access for the caller is not ok
- **THEN** it shows no per-kind fact, and no request for the object is made

#### Scenario: Object facts not readable

- **WHEN** the panel opens on an object whose document the read API refuses
- **THEN** the panel shows the node's graph facts and says details from the object could not be
  read, and is not an error page

### Requirement: An image reference is shown without userinfo
An image reference SHALL be shown without any userinfo before its host. Source: portal:D8:R2,
portal:D9:R3/R6, portal:D10, portal:D14:R7, portal:D7, portal:D11:R7.

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

### Requirement: The details panel keeps its links
The panel SHALL keep its Open, Expand, YAML and Events links, and SHALL add a Logs link for a Pod.
Source: portal:D8:R2, portal:D9:R3/R6, portal:D10, portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: The panel's links
- **WHEN** the details panel opens on a Pod
- **THEN** it shows its Open, Expand, YAML and Events links and a Logs link

### Requirement: Resources is a table of inventory objects and runtime children
Resources SHALL be a table with one row per inventory object and runtime child: kind, name,
component, origin (applied from the inventory, or made by the cluster below an inventory object),
health, and details (the health reason or desired replicas and the message). Source: portal:D8:R2,
portal:D9:R3/R6, portal:D10, portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: The Resources columns
- **WHEN** a browser opens the Resources tab
- **THEN** the table has one row per inventory object and runtime child, with its kind, name, component, origin, health and details

### Requirement: Resources rows are marked and link to the graph
A Degraded or Missing row SHALL be marked apart, and the row of the `node` or `focus` object SHALL
be marked as current. Each row SHALL link to the Graph tab focused on its node. Source:
portal:D8:R2, portal:D9:R3/R6, portal:D10, portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: From Resources to the graph

- **WHEN** the user follows the `podinfo-podinfo` Deployment's row in podinfo's Resources tab
- **THEN** the Graph tab opens with that node focused and the details panel showing it

### Requirement: Resources folds configuration and filters by reason
Configuration components SHALL fold as in the graph. The Resources table SHALL be filtered by a
`reason` parameter shown as a removable chip. Source: portal:D8:R2, portal:D9:R3/R6, portal:D10,
portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: A reason filter
- **WHEN** a browser opens the Resources tab with a `reason` parameter
- **THEN** the table is filtered by it and the filter shows as a removable chip

### Requirement: A package's Resources start with its source
On a package page the first row of the Resources table SHALL be the package's source, with origin
"source". With nothing applied, the Resources tab SHALL read "Nothing applied yet. The graph fills
in once an apply succeeds." on an instance, and "Nothing applied yet. The package has never fetched
its source." on a package that has no source artifact. Source: portal:D8:R2, portal:D9:R3/R6,
portal:D10, portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: The package's source row

- **WHEN** a browser opens `/packages/pkg/podinfo?tab=resources` on the F1 capture
- **THEN** the first row names `OCIRepository` `pkg/podinfo-release` with origin "source", and the
  tab says the package has never fetched its source

### Requirement: No tab makes the page scroll sideways at 360 px
At 360 px wide, no tab SHALL make the page scroll sideways, and the Resources and Events tables
SHALL lay out each row as a card below 760 px. Source: portal:D8:R2, portal:D9:R3/R6, portal:D10,
portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: Owner tabs at 360 px

- **WHEN** a browser 360 px wide opens the Graph, Resources, Events, Logs and YAML tabs of
  podinfo
- **THEN** the page does not scroll sideways on any of them, and each displayed row of the
  Resources and Events tables is a card

### Requirement: Events filter by resource
Events SHALL filter by resource (grouped by kind, with "All resources" and the item itself as
choices, and otherwise only objects that have events in the feed), each choice with its count; on a
feed that left objects out, "All resources" SHALL say its count covers what could be read. Source:
portal:D8:R2, portal:D9:R3/R6, portal:D10, portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: Only objects with events are offered

- **WHEN** a browser opens podinfo's Events tab and one reached object has no event in the feed
- **THEN** the Resource filter does not offer that object

### Requirement: Events filter by type and reason
Events SHALL filter by type and by reason, each choice with its count. The reason SHALL be shown as
a chip only when set. Source: portal:D8:R2, portal:D9:R3/R6, portal:D10, portal:D14:R7, portal:D7,
portal:D11:R7.

#### Scenario: Events filtered by reason

- **WHEN** a browser opens podinfo's page with `tab=events&type=Warning`
- **THEN** only Warning events are listed, folded with their counts, and the type choice reads
  Warning with its count

### Requirement: Events are a table with the regarded resource
"Showing N of M" SHALL sit with the filters. The events SHALL be a table, and each line SHALL show
type, reason, the regarded resource (a link that filters to it when the item reaches it), message
and age. Source: portal:D8:R2, portal:D9:R3/R6, portal:D10, portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: A resource link filters the feed

- **WHEN** the user follows the Deployment's name in a line of podinfo's Events tab
- **THEN** the tab lists only the Deployment's events, and the Resource choice reads its name

### Requirement: The Logs tab picks a Pod and container
Logs SHALL pick a Pod and container and follow its log as the logs panel does today. Source:
portal:D8:R2, portal:D9:R3/R6, portal:D10, portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: Logs pick a container
- **WHEN** a browser opens the Logs tab
- **THEN** it picks a Pod and container and follows its log as the logs panel does today

### Requirement: YAML opens on the selected object and shows no Secret
YAML SHALL pick an object grouped by kind, open on the selected (`node`) object, else the focused
(`focus`) object, else the first readable object, and show no Secret and no values. Source:
portal:D8:R2, portal:D9:R3/R6, portal:D10, portal:D14:R7, portal:D7, portal:D11:R7.

#### Scenario: A Secret in YAML

- **WHEN** the YAML picker lists an inventory that holds a Secret
- **THEN** the Secret is listed as never read and cannot be picked

#### Scenario: YAML on arrival

- **WHEN** a browser opens podinfo's page with `tab=yaml&focus=<the Deployment's node>`
- **THEN** the Deployment's YAML is shown without picking it

#### Scenario: YAML follows a graph click

- **WHEN** the user clicks podinfo's Deployment node on the Graph tab and then follows the YAML
  tab
- **THEN** the YAML tab opens on the Deployment's YAML

### Requirement: A graph shows a card beside a node
A graph SHALL show a card beside a node on pointer hover and on keyboard focus (kind, namespace and
name, health and applied marks, health reason and message, its origin, for a child of the cluster
the object it was created from, access, and a hint to click for details or to expand), positioned
without any `style` attribute. Source: portal:D4:R5, portal:D19.

#### Scenario: Keyboard card

- **WHEN** the user tabs onto podinfo's Deployment node
- **THEN** its card shows kind, name, health and origin, and is announced with the node

### Requirement: Activating a graph node selects it and dims nothing
Activating a node SHALL select it through a `node` parameter, which shows its panel and dims
nothing. Source: portal:D4:R5, portal:D19.

#### Scenario: A click does not dim

- **WHEN** the user clicks podinfo's Service node
- **THEN** the details panel shows the Service, the address carries `node` and no `focus`, and no
  node is dimmed

### Requirement: A focus parameter spotlights a node and its chain
A `focus` parameter SHALL spotlight its node: select it, scroll it into the centre of the view and
dim every node and edge outside its whole upstream and downstream chain; pointer hover and keyboard
focus SHALL dim the same chain while they last. Source: portal:D4:R5, portal:D19.

#### Scenario: Hand-off spotlight

- **WHEN** the user follows the image-break Pod's row from the Resources tab
- **THEN** the Graph tab opens with the Pod centred in the view and its ReplicaSet, Deployment,
  component and instance lit, and podinfo's Service dimmed

### Requirement: Clear selection removes the selection and the spotlight
A "Clear selection" control SHALL remove the selection and the spotlight. Source: portal:D4:R5,
portal:D19.

#### Scenario: Clear selection on a spotlight
- **WHEN** a node is selected or spotlighted
- **THEN** the "Clear selection" control removes the selection and the spotlight

### Requirement: A graph opens in full screen
The graph SHALL open in full screen and back, refitting to the space it has. Full screen SHALL cover
the page's window and SHALL stay on while groups open and close. Source: portal:D4:R5, portal:D19.

#### Scenario: Full screen survives a group

- **WHEN** the user opens cert-manager's graph in full screen and opens, then collapses, its RBAC
  group
- **THEN** the graph is still in full screen

### Requirement: A graph zooms, fits and pans
The graph SHALL zoom with a zoom slider from 30 % to 200 %, Fit (marked pressed while the view is
fitted) and Ctrl/Cmd-wheel anchored at the pointer, and pan with the pointer; a fit SHALL use both
the frame's width and height, and the frame's height SHALL follow the fitted graph. Source:
portal:D4:R5, portal:D19.

#### Scenario: Wheel zoom at the pointer

- **WHEN** the user zooms with Ctrl and the wheel over a node
- **THEN** that node stays under the pointer

### Requirement: A group node expands in place
A group node SHALL expand in place through the graph's `expand` parameter and fit the view to the
group's frame at no less than 60 %, and an "Overview" control and the Escape key SHALL collapse it
and fit the whole graph again. Source: portal:D4:R5, portal:D19.

#### Scenario: cert-manager's RBAC group

- **WHEN** the user expands cert-manager's RBAC group
- **THEN** the view fits the group's frame, and "Overview" returns to the collapsed, fitted graph

### Requirement: Opening or closing a group keeps the scroll position
Opening or closing a group, and the group controls, SHALL keep the page's scroll position. Source:
portal:D4:R5, portal:D19.

#### Scenario: A group keeps the scroll position
- **WHEN** the user opens or closes a group
- **THEN** the page keeps its scroll position

### Requirement: Escape steps back one thing per press
Escape SHALL step back one thing per press: an open card, a selection, an open group, full screen.
Source: portal:D4:R5, portal:D19.

#### Scenario: Escape steps back

- **WHEN** the user has opened the RBAC group in full screen and selected one of its objects, and
  presses Escape three times
- **THEN** the first press clears the selection, the second collapses the group, and the third
  leaves full screen

### Requirement: Wheel zoom does not animate and motion stops under reduced motion
Wheel zoom SHALL not animate, and motion SHALL stop under the reduced-motion preference. Source:
portal:D4:R5, portal:D19.

#### Scenario: Reduced motion
- **WHEN** the browser sets the reduced-motion preference
- **THEN** graph motion stops, and wheel zoom does not animate

### Requirement: A holder of a registration shows a Provider pill, card and tab
An instance or a package whose inventory holds a TransformerRegistration SHALL show a Provider pill,
the Provider card and a Provider tab. Source: portal:D15:R1/R2/R3, portal:D4:R2.

#### Scenario: The F1 provider

- **WHEN** a browser opens `/instances/default/backup-provider` on the F1 capture
- **THEN** it shows the Provider pill, a Provider card reading Active, and a Provider tab for
  `default.backup-provider`

### Requirement: The provider standing is the registration's own
The standing shown SHALL be the registration's own accepted, active, verdict and reason, never one
the portal computed; a package whose claim the controller refused SHALL show as a refused provider
with the controller's reason and, in local mode, its message. Source: portal:D15:R1/R2/R3,
portal:D4:R2.

#### Scenario: A package holding a refused claim

- **WHEN** a package's inventory holds a registration the controller refused with
  `ProviderMismatch`
- **THEN** the package shows a Provider pill and a Provider card reading Refused with reason
  `ProviderMismatch`, and no text saying the refusal is wrong

#### Scenario: Registrations forbidden

- **WHEN** the caller may read `default/backup-provider` but may not list TransformerRegistrations
- **THEN** the Provider card names `default.backup-provider` with a locked standing

### Requirement: A disagreeing providerRef is shown beside the holder
When the registration's `spec.providerRef` and the holder disagree, both SHALL be shown. Source:
portal:D15:R1/R2/R3, portal:D4:R2.

#### Scenario: providerRef and holder disagree
- **WHEN** the registration's `spec.providerRef` and the holder disagree
- **THEN** the page shows both

### Requirement: The Provider tab opens with a sentence about the provider
The Provider tab SHALL open with a sentence, per kind, saying the item is also a provider that
registers a catalog with the platform. For a refused claim, the sentence SHALL add, from the
controller's verdict, that it provides nothing. Source: portal:D16:R3, portal:D4:R4, portal:D7:R3,
portal:D15:R2/R3.

#### Scenario: The Provider tab's opening sentence
- **WHEN** a browser opens the Provider tab of an item that holds a registration
- **THEN** the tab opens with a sentence, per kind, saying the item is also a provider that registers a catalog with the platform

### Requirement: The Provider tab shows each registration's standing once
Per held registration, the Provider tab SHALL show the registration's name and its standing once:
acceptance and activation as two marks, and the reason only when it is not a success reason. Source:
portal:D16:R3, portal:D4:R4, portal:D7:R3, portal:D15:R2/R3.

#### Scenario: The standing said once

- **WHEN** a browser opens backup-provider's Provider tab on the F1 capture
- **THEN** its registration shows "accepted" and "active" once each, and no success reason beside
  them

### Requirement: The Provider tab shows a grid of facts
Per held registration, the tab SHALL show a grid of facts: the registration, its claimed catalog
(linking to the Catalog page) and version, whether that catalog is in the Platform's resolved
registry and contributed by this registration, the `providerRef`, and in local mode the controller's
message. Source: portal:D16:R3, portal:D4:R4, portal:D7:R3, portal:D15:R2/R3.

#### Scenario: The grid of facts
- **WHEN** a browser opens the Provider tab of an item that holds a registration
- **THEN** the tab shows a grid of facts with the registration, its claimed catalog linking to the Catalog page, its version, whether the catalog is in the Platform's resolved registry and contributed by this registration, and the `providerRef`

### Requirement: The Contracts fact counts what is provided and in use
The grid SHALL also show how many contracts the registration provides and how many are in use (or
"none" for a refused claim, and no in-use count when a list of users is incomplete or locked).
Source: portal:D16:R3, portal:D4:R4, portal:D7:R3, portal:D15:R2/R3.

#### Scenario: Users not fully readable

- **WHEN** the caller may not list ModuleInstances in every namespace and opens backup-provider's
  Provider tab
- **THEN** the Contracts fact reads "1 provided; use not fully readable", not an in-use count

### Requirement: The Provider tab shows the registration's conditions
Per held registration, the tab SHALL show its "Registration conditions" (type, a status badge toned
by the condition's tone, reason, meaning and since, and in local mode the message). Source:
portal:D16:R3, portal:D4:R4, portal:D7:R3, portal:D15:R2/R3.

#### Scenario: Registration conditions
- **WHEN** a browser opens the Provider tab of an item that holds a registration
- **THEN** the tab shows its "Registration conditions" with type, a status badge toned by the condition's tone, reason, meaning and since

### Requirement: The Provider tab tabulates what the registration provides
Per held registration, the tab SHALL show a table of what it provides, where each contract is a pill
showing its short path, with the full path as its title, and with the readable instances whose
render contracts contain it. Source: portal:D16:R3, portal:D4:R4, portal:D7:R3, portal:D15:R2/R3.

#### Scenario: What it provides
- **WHEN** a browser opens the Provider tab of an item that holds a registration
- **THEN** the tab shows a table of what it provides, each contract a pill with its short path and its full path as its title

### Requirement: A claim not yet active says that nothing is provided yet
The Contracts fact SHALL have a text for every standing. A claim that is accepted but not active, or
pending, SHALL say how many contracts it lists and that none is provided yet. Source: portal:D16:R3,
portal:D4:R4, portal:D7:R3, portal:D15:R2/R3.

#### Scenario: A claim that provides nothing yet

- **WHEN** a held registration lists two contracts and is Pending, or accepted and not active
- **THEN** the Contracts fact says it lists two and that none is provided yet, and does not read
  "2 provided, 0 in use"

### Requirement: A claim whose removal is blocked says so
A claim whose removal is blocked SHALL give its counts in the Contracts fact and say the removal is
blocked. Source: portal:D16:R3, portal:D4:R4, portal:D7:R3, portal:D15:R2/R3.

#### Scenario: A blocked removal still provides

- **WHEN** a held registration's removal is blocked and one of its contracts is in use
- **THEN** the Contracts fact gives "1 in use" and says the removal is blocked

### Requirement: A claim of unknown standing says it is not known
A claim of unknown standing SHALL say in the Contracts fact that it is not known. Source:
portal:D16:R3, portal:D4:R4, portal:D7:R3, portal:D15:R2/R3.

#### Scenario: A claim of unknown standing

- **WHEN** the verdict of a held registration is one the page does not know
- **THEN** the Contracts fact says it is not known

### Requirement: Provider tab rows are cards below 760 px
Below 760 px wide, each row of the Provider tab's table of what the registration provides SHALL be a
card. Source: portal:D16:R3, portal:D4:R4, portal:D7:R3, portal:D15:R2/R3.

#### Scenario: The Provider tab at 360 px

- **WHEN** a browser 360 px wide opens backup-provider's Provider tab
- **THEN** the page does not scroll sideways, the facts take one column, and each row of "What it
  provides" is a card

### Requirement: Each contract lists its first three users
For each contract the Provider tab's table of what the registration provides SHALL list the first
three users and the count of the rest. It SHALL link to Installed filtered by that contract as "Show
in Installed" when there are three or fewer, and "All N in Installed" otherwise. Source:
portal:D16:R3, portal:D4:R4, portal:D7:R3, portal:D15:R2/R3.

#### Scenario: Who uses the backup trait

- **WHEN** a browser opens backup-provider's Provider tab on the F1 capture
- **THEN** contract `opmodel.dev/catalogs/opm/traits/backup@v1alpha1` lists
  `default/backup-consumer` and links to
  `/installed?uses=opmodel.dev/catalogs/opm/traits/backup@v1alpha1` as "Show in Installed", and
  the Contracts fact reads "1 provided, 1 in use"

### Requirement: A contract nobody uses says so
When no readable instance uses the contract and the list is complete, the tab SHALL say no installed
module uses it yet and offer no link. Source: portal:D16:R3, portal:D4:R4, portal:D7:R3,
portal:D15:R2/R3.

#### Scenario: A contract nobody uses

- **WHEN** a registration provides a contract that no readable instance's render used, and the
  caller may list instances everywhere
- **THEN** the contract's row says no installed module uses it yet and offers no Installed link

### Requirement: An incomplete list of users says it may be incomplete
When the caller could not list instances everywhere, the list of users SHALL say it may be
incomplete. Source: portal:D16:R3, portal:D4:R4, portal:D7:R3, portal:D15:R2/R3.

#### Scenario: A list that may be incomplete
- **WHEN** the caller could not list instances everywhere and opens the Provider tab
- **THEN** the list of users says it may be incomplete

### Requirement: The Provider tab renders locked what the Platform document holds
The registration's conditions, catalog and registry state come from the Platform document. When the
caller may not read the Platform, those parts SHALL render locked, and the tab SHALL keep the
registration's name and standing. Source: portal:D16:R3, portal:D4:R4, portal:D7:R3,
portal:D15:R2/R3.

#### Scenario: Platform forbidden

- **WHEN** the caller may read `default/backup-provider` and list TransformerRegistrations but may
  not get the Platform, and opens its Provider tab
- **THEN** the tab names `default.backup-provider` with its standing, and its conditions and
  catalog render locked; the page is not an error

### Requirement: The Catalog page shows any catalog the Platform knows
`/catalog?path=<catalog>` SHALL show any catalog the Platform subscribes to, holds in its resolved registry, or a readable registration claims: as its headline, the catalog's short name (its last path segment without the major version, or a longer form when two catalogs share it), with the full path and version on a line below. The breadcrumb SHALL end with the short name. Source: portal:D17:R3, portal:D3:R8, portal:D7:R1, portal:D9:R2, portal:D19:R5.

#### Scenario: The contributed backup catalog

- **WHEN** a browser opens `/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0` on the
  F1 capture
- **THEN** it shows the headline `backup`, version `0.1.0`, origin from a provider, the source
  `default.backup-provider` linking to the Provider tab of `default/backup-provider`, and resolved

### Requirement: The Catalog page shows the catalog's origin and source
The Catalog page SHALL show the catalog's origin (subscribed, from a provider, or claimed only) as a
pill, and its source ("Platform subscription", or the contributing registration, or for a
claimed-only catalog the claiming registrations, each linking to its holder's Provider tab when it
has one holder). Source: portal:D17:R3, portal:D3:R8, portal:D7:R1, portal:D9:R2, portal:D19:R5.

#### Scenario: Origin and source
- **WHEN** a browser opens the Catalog page of a catalog
- **THEN** the page shows its origin as a pill and its source

### Requirement: The Catalog page shows the catalog's enablement
The Catalog page SHALL show the catalog's enablement, reading "no: not in the registry" for a
catalog the registry does not hold. Source: portal:D17:R3, portal:D3:R8, portal:D7:R1, portal:D9:R2,
portal:D19:R5.

#### Scenario: Enablement of a catalog not in the registry
- **WHEN** a browser opens the Catalog page of a catalog the registry does not hold
- **THEN** its enablement reads "no: not in the registry"

### Requirement: The Catalog page shows whether the catalog is resolved
The Catalog page SHALL show whether the catalog is resolved, as a state block with the state word
and a summary of what the registry records, and the Platform's `ContractsFulfilled` labelled as
platform-wide. Source: portal:D17:R3, portal:D3:R8, portal:D7:R1, portal:D9:R2, portal:D19:R5.

#### Scenario: A claimed catalog that does not resolve

- **WHEN** a browser opens the Catalog page of the catalog `default.refused-claim-fixture` claims
- **THEN** it shows the catalog as claimed only and not resolved, with the claim's reason
  `CatalogUnresolved`

### Requirement: An unresolved catalog shows the time since it was refused
For an unresolved catalog the state block SHALL show the time since it was refused: the earliest
time a claiming registration's `Ready` condition turned `False`, or the Platform's `Ready` for a
subscription. Source: portal:D17:R3, portal:D3:R8, portal:D7:R1, portal:D9:R2, portal:D19:R5.

#### Scenario: Refused since
- **WHEN** a browser opens the Catalog page of an unresolved catalog
- **THEN** the state block shows the time since it was refused

### Requirement: Each refusal reason links to the filtered Events tab
For an unresolved catalog the state block SHALL show each reason (the Platform's `Ready` reason
without a count, or the claiming registrations' refusal reasons with their count) linking to the
Events tab filtered by that reason. Source: portal:D17:R3, portal:D3:R8, portal:D7:R1, portal:D9:R2,
portal:D19:R5.

#### Scenario: A refusal reason opens the events

- **WHEN** the user follows the `CatalogUnresolved` reason on the Resolved block of the catalog
  `default.refused-claim-fixture` claims
- **THEN** the Events tab opens filtered to reason `CatalogUnresolved`, holding the claim's
  warning, with its source reading the claiming registration's name, its enablement "no: not in
  the registry", and a "refused since" time on the block

### Requirement: The Catalog page lists the registrations that claim it
The Catalog page SHALL list the registrations that claim the catalog with their holders and
standing. Source: portal:D17:R3, portal:D3:R8, portal:D7:R1, portal:D9:R2, portal:D19:R5.

#### Scenario: Claimants forbidden

- **WHEN** the caller may get the Platform but may not list TransformerRegistrations and opens a
  subscribed catalog
- **THEN** the claims render locked and the Resolved block shows the Platform's own resolution,
  with no refusal count

### Requirement: The Catalog page tabulates the events of the Platform and its claimants
The Catalog page SHALL show the recent events of the Platform and its claimants as a table,
filterable by type and reason, with how many are shown of how many. Source: portal:D17:R3,
portal:D3:R8, portal:D7:R1, portal:D9:R2, portal:D19:R5.

#### Scenario: Events filtered by type

- **WHEN** a browser opens the Events tab of a catalog with `type=Warning`
- **THEN** the table holds only Warning events and says how many it shows of all the events

#### Scenario: A Type choice keeps the reason

- **WHEN** a browser opens a catalog's Events tab with `reason=CatalogUnresolved` and then picks
  type Warning
- **THEN** the address carries both `reason=CatalogUnresolved` and `type=Warning`

### Requirement: The Catalog page says what is not recorded
The Catalog page SHALL show no definition, description, documentation link or transformer, and SHALL
say these are not recorded. Source: portal:D17:R3, portal:D3:R8, portal:D7:R1, portal:D9:R2,
portal:D19:R5.

#### Scenario: Not recorded
- **WHEN** a browser opens the Catalog page of a catalog
- **THEN** the page shows no definition, description, documentation link or transformer, and says these are not recorded

### Requirement: The Catalog page renders locked when the Platform is unreadable
When the caller may not read the Platform, the Catalog page SHALL render locked with status `403`,
saying nothing about whether the catalog exists. Source: portal:D17:R3, portal:D3:R8, portal:D7:R1,
portal:D9:R2, portal:D19:R5.

#### Scenario: Platform forbidden

- **WHEN** a caller who may not get the Platform opens
  `/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0` and
  `/catalog?path=example.com/nothing@v0`
- **THEN** both render the same locked page with status `403`

### Requirement: An unnamed catalog path renders not found
A path that a readable Platform and the readable registrations do not name SHALL render not found
with status `404`. Source: portal:D17:R3, portal:D3:R8, portal:D7:R1, portal:D9:R2, portal:D19:R5.

#### Scenario: An unknown catalog

- **WHEN** a browser opens `/catalog?path=example.com/nothing@v0`
- **THEN** the response is `404` with the not-found page

### Requirement: Page text names kinds in a platform team's words
Page text SHALL name kinds in the words a platform team uses: "Providers" for
TransformerRegistrations and the instances and packages that hold them, "Installed" for
ModuleInstances and ModulePackages, and "controller" for the opm-operator. The YAML view shows
objects as the cluster serves them and is exempt. Source: portal:D17:R2.

#### Scenario: The platform team's words
- **WHEN** a page names TransformerRegistrations, ModuleInstances, ModulePackages or the opm-operator
- **THEN** it uses "Providers", "Installed" and "controller"

### Requirement: The controller version and owner are worded as controller
The read API's `operatorVersion` SHALL be labelled "Controller", and its owner value `operator`
SHALL read "controller". Source: portal:D17:R2.

#### Scenario: The owner column

- **WHEN** the Installed list shows an instance whose owner is `operator` in the read API
- **THEN** its owner cell reads "controller"

### Requirement: The Installed card counts providers
The Installed card SHALL count the items that hold a registration as providers. Source:
portal:D17:R2.

#### Scenario: The providers line

- **WHEN** the Platform page's Installed card counts the F1 capture
- **THEN** its closing line says how many of the installed items are providers

### Requirement: Anything the API reports as unreadable renders locked
Every object, row, node, list, card or section the read API reports as forbidden, not readable or
withheld SHALL render locked: a distinct locked style, a lock mark and text saying the user may not
read it or the portal cannot, showing only what the API document carries, with no link to a detail
it cannot open. Source: portal:D5:R7, portal:D7:R3.

#### Scenario: A forbidden kind inside an instance

- **WHEN** the caller may not read Services and opens `/instances/default/podinfo`
- **THEN** podinfo's Service row and its graph node render locked, and the Deployment renders
  normally

### Requirement: A forbidden instance or package list is a locked group
A forbidden instance or package list SHALL render as a locked group of the Installed list with no
count. Source: portal:D5:R7, portal:D7:R3.

#### Scenario: A forbidden list in Installed

- **WHEN** the caller may not list ModuleInstances and opens `/installed`
- **THEN** the instances show as a locked group with no count

### Requirement: A Secret in an inventory is never read
A Secret in an inventory SHALL show that Secret data is never read, with no YAML action. Source:
portal:D5:R7, portal:D7:R3.

#### Scenario: A Secret in an inventory
- **WHEN** an inventory holds a Secret
- **THEN** the page shows that Secret data is never read, and offers no YAML action

### Requirement: The identity card names the kind, the name and the module or source
The page of an instance or package SHALL show an identity card. The card SHALL carry the kind, the
name and, under the name, the module path and version, or for a package "from" its source kind,
namespace and name, with the namespace defaulting to the package's own. Source: portal:D3:R1/R4/R5,
portal:D9:R1/R2, portal:D15:R2/R3/R4, portal:D8:R5/R6.

#### Scenario: The F1 package

- **WHEN** a browser opens `/packages/pkg/podinfo` on the F1 capture
- **THEN** the identity card reads "from OCIRepository pkg/podinfo-release" under the name, with
  interval "every 1 min", prune on and revision "not recorded"
- **AND** the Applied card reads Failed with reason `SourceNotReady`, a summary naming the three
  failed attempts, a `Reconciling` mark with reason `Progressing`, and three failed dots and no
  other dot

### Requirement: The identity card carries the namespace, owner and applier
The identity card SHALL also carry the namespace, the owner and the applier ServiceAccount, each of
the last two with an explanation reachable by pointer and keyboard. A package's card SHALL add its
path, interval, prune setting and source revision. An interval the package does not set SHALL read
"not set"; a revision the package has not recorded SHALL read "not recorded". Source:
portal:D3:R1/R4/R5, portal:D9:R1/R2, portal:D15:R2/R3/R4, portal:D8:R5/R6.

#### Scenario: Namespace, owner and applier
- **WHEN** a browser opens the page of an instance
- **THEN** the identity card carries the namespace, the owner and the applier ServiceAccount, each of the last two with an explanation reachable by pointer and keyboard

### Requirement: The identity card takes one column on a narrow screen
Below 520 px wide, the identity card's facts SHALL take one column. Source: portal:D3:R1/R4/R5,
portal:D9:R1/R2, portal:D15:R2/R3/R4, portal:D8:R5/R6.

#### Scenario: Identity card below 520 px
- **WHEN** a browser opens an instance page below 520 px wide
- **THEN** the identity card's facts take one column

### Requirement: The page shows an Applied card, a Health card and a Provider card
The page SHALL show an Applied card and a Health card, and a Provider card when the item holds a
TransformerRegistration. Source: portal:D3:R1/R4/R5, portal:D9:R1/R2, portal:D15:R2/R3/R4,
portal:D8:R5/R6.

#### Scenario: The three cards
- **WHEN** a browser opens the page of an item that holds a TransformerRegistration
- **THEN** the page shows an Applied card, a Health card and a Provider card

### Requirement: Each card is a state block
Each card SHALL be a state block: an eyebrow naming the axis and its source, the recorded time of
the state when there is one, an icon and a large state word in the state's tone, a one-line summary,
and reason counts with the count first, each linking to its evidence, or a line saying there are
none. Source: portal:D3:R1/R4/R5, portal:D9:R1/R2, portal:D15:R2/R3/R4, portal:D8:R5/R6.

#### Scenario: The state blocks
- **WHEN** a browser opens the page of an item
- **THEN** each card shows an eyebrow naming its axis and its source, a large state word in the state's tone and a one-line summary

### Requirement: The Applied card shows the applied state as its word
The Applied card SHALL show the applied state as its word. It SHALL show "Reconciling" instead while
the `Reconciling` condition is `True` on an applied item, with that condition's time. Source:
portal:D3:R1/R4/R5, portal:D9:R1/R2, portal:D15:R2/R3/R4, portal:D8:R5/R6.

#### Scenario: Rolling out

- **WHEN** an applied instance's `Reconciling` condition turns `True`
- **THEN** its Applied card reads Reconciling in the progressing tone, with the condition's time
  and reason

### Requirement: The Applied card composes its summary from recorded facts
The Applied card SHALL compose its summary only from recorded facts: the inventory count, the
fetched revision, the run of failed attempts and whether the controller retries. It SHALL name a
retry interval only where the controller retries at that interval. Source: portal:D3:R1/R4/R5,
portal:D9:R1/R2, portal:D15:R2/R3/R4, portal:D8:R5/R6.

#### Scenario: An Applied summary from recorded facts
- **WHEN** a browser opens the page of a failed package
- **THEN** the Applied card's summary names only recorded facts: the inventory count, the fetched revision, the run of failed attempts and whether the controller retries

### Requirement: The Applied card shows the reason's meaning and message
The Applied card SHALL show the reason's meaning and, in local mode, the controller's message
beneath the summary. Source: portal:D3:R1/R4/R5, portal:D9:R1/R2, portal:D15:R2/R3/R4,
portal:D8:R5/R6.

#### Scenario: Meaning and message under the summary
- **WHEN** a browser opens the page of an item whose applied state has a reason, in local mode
- **THEN** the Applied card shows the reason's meaning and the controller's message beneath the summary

### Requirement: The Applied card marks a Reconciling condition
The Applied card SHALL show a `Reconciling` mark with its reason and since time while that condition
is `True` and the word is not "Reconciling". Source: portal:D3:R1/R4/R5, portal:D9:R1/R2,
portal:D15:R2/R3/R4, portal:D8:R5/R6.

#### Scenario: A Reconciling mark
- **WHEN** the `Reconciling` condition is `True` and the Applied card's word is not "Reconciling"
- **THEN** the card shows a `Reconciling` mark with its reason and since time

### Requirement: The Applied card counts Warning events by reason
The Applied card SHALL count the Warning events about the item itself by reason, labelled as the
last hour of events, each count linking to the Events tab filtered by that reason. Source:
portal:D3:R1/R4/R5, portal:D9:R1/R2, portal:D15:R2/R3/R4, portal:D8:R5/R6.

#### Scenario: Warning events on the Applied card
- **WHEN** a browser opens the page of an item with Warning events about itself
- **THEN** the Applied card counts them by reason, labelled as the last hour of events, each count linking to the Events tab filtered by that reason

### Requirement: The Applied card draws one dot per history entry
The Applied card SHALL draw one dot per `status.history` entry, oldest first, marked and coloured by
the entry's outcome, with no dot for an attempt the history does not record. Source:
portal:D3:R1/R4/R5, portal:D9:R1/R2, portal:D15:R2/R3/R4, portal:D8:R5/R6.

#### Scenario: A failed attempt in the history

- **WHEN** an instance's newest history entry has no phase and a message
- **THEN** its dot is drawn and marked failed, and in local mode its tooltip carries the message

### Requirement: The Applied card offers the history
The Applied card SHALL show the number of attempts and a Show history control. The opened history
SHALL show each entry's outcome and SHALL say that at most ten attempts are kept and that reconciles
that change nothing are not recorded. Source: portal:D3:R1/R4/R5, portal:D9:R1/R2,
portal:D15:R2/R3/R4, portal:D8:R5/R6.

#### Scenario: The opened history
- **WHEN** the user opens Show history on the Applied card
- **THEN** the history shows each entry's outcome and says that at most ten attempts are kept and that reconciles that change nothing are not recorded

### Requirement: The Health card shows the health state as its word
The Health card SHALL show the health state as its word, and SHALL say how many resources are
unhealthy or rolling out out of how many were counted, or that nothing was applied, so there is
nothing to check. Source: portal:D3:R1/R4/R5, portal:D9:R1/R2, portal:D15:R2/R3/R4, portal:D8:R5/R6.

#### Scenario: The Health summary
- **WHEN** a browser opens the page of an item with applied objects
- **THEN** the Health card shows the health state as its word and says how many resources are unhealthy or rolling out out of how many were counted

### Requirement: The Health card marks a partial or stale health
The Health card SHALL show its partial mark, and when it is not live, the time it was evaluated. A
health with no counted object SHALL show no time and SHALL not be called "not live". Source:
portal:D3:R1/R4/R5, portal:D9:R1/R2, portal:D15:R2/R3/R4, portal:D8:R5/R6.

#### Scenario: Nothing applied yet

- **WHEN** a package whose inventory names no object is rendered
- **THEN** its Health card reads Unknown with "Nothing applied yet, so nothing to check", shows no
  time, and does not say "not live"

### Requirement: The Health card counts objects by health reason
The Health card SHALL count objects and runtime children by health reason, each linking to the
Resources tab with `reason` set to it. Source: portal:D3:R1/R4/R5, portal:D9:R1/R2,
portal:D15:R2/R3/R4, portal:D8:R5/R6.

#### Scenario: The image-break podinfo

- **WHEN** the page of the image-break podinfo sample is rendered
- **THEN** the Applied card reads Applied and the Health card reads Degraded with a summary
  counting the unhealthy resources and one count for the Pod's waiting reason, linking to
  `tab=resources&reason=<that reason>`, which lists that Pod

### Requirement: The Provider card shows each held registration's standing
The Provider card SHALL show, per held registration, Active, Accepted and not active, Refused,
Removal blocked or Pending as the controller recorded it, or a locked standing when the caller may
not read registrations. Source: portal:D3:R1/R4/R5, portal:D9:R1/R2, portal:D15:R2/R3/R4,
portal:D8:R5/R6.

#### Scenario: Standing as recorded
- **WHEN** a registration the controller recorded as Refused is held
- **THEN** the Provider card shows Refused, as the controller recorded it

### Requirement: The Provider card summary is not a verdict the portal composed
The Provider card's summary SHALL be the contracts an active registration provides, or else the
controller's message in local mode and the reason's meaning in-cluster, never a verdict the portal
composed. Source: portal:D3:R1/R4/R5, portal:D9:R1/R2, portal:D15:R2/R3/R4, portal:D8:R5/R6.

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

### Requirement: The Provider card has a fixed sentence when the controller said nothing
When the controller wrote no message and the reason has no known meaning, the Provider card's
summary SHALL be a fixed sentence for the recorded standing, so no standing shows its word over an
empty summary. Source: portal:D3:R1/R4/R5, portal:D9:R1/R2, portal:D15:R2/R3/R4, portal:D8:R5/R6.

#### Scenario: A pending claim with no controller text

- **WHEN** a registration is Pending and its conditions carry no message and no known reason
- **THEN** the Provider card reads Pending with the summary "The controller has not decided on this
  claim yet." in both modes

#### Scenario: A blocked removal

- **WHEN** a held registration's removal is blocked while modules still use what it provides, and
  the conditions carry no message
- **THEN** the Provider card reads Removal blocked and says the claim is accepted and active and
  its removal waits until no module uses what it provides

### Requirement: The Provider card reads Unknown for a standing it does not know
A standing the page does not know SHALL read Unknown on the Provider card. Source:
portal:D3:R1/R4/R5, portal:D9:R1/R2, portal:D15:R2/R3/R4, portal:D8:R5/R6.

#### Scenario: A claim of unknown standing

- **WHEN** the Platform records a verdict the page does not know for a held registration
- **THEN** the Provider card reads Unknown and says the portal does not know the claim's standing

### Requirement: A non-success provider reason links to the Provider tab
A non-success reason on the Provider card SHALL link to the Provider tab. Source:
portal:D3:R1/R4/R5, portal:D9:R1/R2, portal:D15:R2/R3/R4, portal:D8:R5/R6.

#### Scenario: A reason links to the Provider tab
- **WHEN** a held registration has a non-success reason
- **THEN** the Provider card's reason links to the Provider tab
