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
status `401`. Source: 0030:D2:R1.

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

`/` SHALL show the Platform: its applied state and conditions, its catalog subscriptions with
versions, the resolved registry with each catalog's version, enablement, source, claimants and the
registration that contributed it, and each readable TransformerRegistration with its catalog,
version, provided contracts and provider, its acceptance and its activation as two separate
pills, its verdict, and its refusal or removal-blocked reason and message. A removal-blocked
registration SHALL show as removal blocked while still accepted and active, never as refused. A
Platform reporting unfulfilled provider contracts SHALL show it as information, not as a failure.
The page SHALL show the platform graph and the Platform's recent events. When the registrations
the caller may read are not all of them, the page SHALL say so with the locked style. Source:
0030:D4:R4/R7, 0030:D3:R8.

#### Scenario: The F1 platform

- **WHEN** a signed-in browser opens `/` on the F1 capture
- **THEN** it shows the `opmodel.dev/catalogs/opm` subscription with its version, the backup
  catalog contributed by the accepted, active `backup-provider` claim, and the refused claim
  with its verdict Refused, its reason and its message

#### Scenario: Accepted and active apart

- **WHEN** a registration is accepted and not active
- **THEN** its row shows an "accepted" pill and an "inactive" pill, each on its own

### Requirement: Applied and health are two badges, never one

Every instance and package SHALL show its applied state and its health as two separate badges of
different shapes. `Ready=True` SHALL be shown as "Applied" and never as healthy.
`ManagedExternally` SHALL render in a neutral style without an error colour. A partial health
SHALL say partial and be marked visually apart from a full health; a health with an object
refreshed by polling SHALL say it is not live and show when it was evaluated. An enum value the
UI does not know SHALL render as "unknown", never as an error. The red family SHALL be used only
for degraded, failed, stalled and refused states, broken edges, Warning events and a lost stream; reasons, versions, kinds and
other decoration SHALL not use it. A condition SHALL be coloured by its `tone`, never by its
status alone. Source: 0030:D3:R1/R5/R6/R8, 0030:D2:R3.

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

### Requirement: Unreadable things render locked

Every object, row, node, list or section the read API reports as forbidden, not readable or
withheld SHALL render locked: a distinct locked style, a lock mark and text saying the user may
not read it or the portal cannot, showing only what the API document carries, with no link to a
detail it cannot open. A forbidden instance or package list SHALL render as a locked list with no
count. A Secret in an inventory SHALL show that Secret data is never read, with no YAML action.
An edge the portal cannot confirm because its far end is locked or unreadable SHALL be drawn in
the locked style, apart from an edge confirmed broken. Source: 0030:D5:R7, 0030:D7:R3.

#### Scenario: A forbidden kind inside an instance

- **WHEN** the caller may not read Services and opens `/instances/default/podinfo`
- **THEN** podinfo's Service row and its graph node render locked, and the Deployment renders
  normally

#### Scenario: A forbidden list

- **WHEN** the caller may not list ModuleInstances and opens `/instances`
- **THEN** the page shows a locked list and no count

#### Scenario: An edge to a locked provider

- **WHEN** the caller may not read the provider instance of an accepted registration
- **THEN** the edge from the registration to the locked provider is drawn in the locked style,
  not as unverified

### Requirement: The instance page shows the record, the feed and the logs

The instance page SHALL show the instance's graph, its components with their objects and health,
its conditions with each known reason's meaning and next step beside the operator's message, its
status history newest first, the render contracts as plain text labelled as the contracts the
render used and not the instance's provider demand, and its recent events with repeats folded
into one line with a count and the latest time. Configuration components SHALL be grouped in
the components list as in the graph, with a count and their worst health, and the group SHALL
be open when any member is not healthy. ReplicaSets with zero desired replicas SHALL be folded
apart from the live ones. The feed SHALL be labelled as recent activity that expires after about
an hour, and the page SHALL say that render warnings are kept only as events. A logs panel SHALL
offer each container of each Pod an inventory reaches, following its log topic, rendering log
text as text only, and each Pod row SHALL link to its log. The package page SHALL show the same
for a ModulePackage, with its source and dependencies. Source: 0030:D4:R3, 0030:D9:R2/R3/R5,
0030:D10.

#### Scenario: podinfo's page

- **WHEN** a signed-in browser opens `/instances/default/podinfo` on the F1 capture
- **THEN** it shows the podinfo component with its Service and Deployment, the ReplicaSet and two
  Pods below, the conditions with their meanings, the history, the render contracts as text, the
  events feed with its expiry note, and one log pane entry per Pod container

#### Scenario: cert-manager's configuration components

- **WHEN** a signed-in browser opens `/instances/cert-manager/cert-manager` on the F1 capture
- **THEN** its configuration components are one closed group with their count and health, and
  its workload components are listed one by one

### Requirement: Objects can be viewed as YAML without values

Selecting an object an inventory reaches SHALL show it as YAML from the read API's object
resource, without `spec.values`, managed fields or the last-applied annotation. Source:
0030:D8:R2/R3, 0030:D2:R4.

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
group node SHALL show it expanded. A graph SHALL open fitted to its frame and SHALL pan and zoom
with pointer, wheel and buttons. Source: 0030:D4:R1/R3/R5, 0030:D3:R1.

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
request. A change on a followed topic SHALL re-render every region that shows it from one fetch
of the page, so the regions of one refresh come from the same moment.

#### Scenario: Navigating keeps the stream

- **WHEN** the user moves from `/` to `/instances/default/podinfo`
- **THEN** the same stream drops `platform` and adds `instance:default/podinfo` and
  `events:instance:default/podinfo`

#### Scenario: An image break on the open page

- **WHEN** podinfo's image is changed to a tag that does not exist while its page is open
- **THEN** without a reload the page shows the new ReplicaSet and the Pod waiting on the image
  in the components and the graph, the instance Degraded and still Applied

### Requirement: Pages carry a strict policy and their own assets

UI pages SHALL be served with
`Content-Security-Policy: default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'`,
with no inline script, no inline style attribute and no `eval`. Untrusted text (messages, notes,
labels, log lines) SHALL be rendered only through `html/template` or as text nodes, and a test
SHALL fail when a page renders such text as markup or when the UI's code converts a string to a
trusted template type or assigns markup in the page script. Scripts, styles and fonts SHALL be
served from the binary, vendored at pinned versions whose SHA-256 a test checks. Pages SHALL
follow the OS light or dark preference and SHALL not scroll horizontally at a 360 px wide
viewport.

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

### Requirement: An expired session stops the page's stream

When the stream ends with an `expired` event, the page SHALL close its `EventSource`, so it does
not reconnect, and SHALL show in the live indicator that the session expired and the page must be
reloaded. The page SHALL keep showing what it last rendered.

#### Scenario: The session expires while a page is open

- **WHEN** the session of an open page expires
- **THEN** the live indicator reads "session expired, reload"
- **AND** the page makes no further stream request
