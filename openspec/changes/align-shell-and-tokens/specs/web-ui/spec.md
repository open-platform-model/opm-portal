## MODIFIED Requirements

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

### Requirement: An expired session stops the page's stream

When the stream ends with an `expired` event, the page SHALL close its `EventSource`, so it does
not reconnect, and SHALL show in the live mark that the session expired and the page must be
reloaded, in the live mark's capitalised wording. The page SHALL keep showing what it last
rendered.

#### Scenario: The session expires while a page is open

- **WHEN** the session of an open page expires
- **THEN** the live mark reads "Session expired, reload"
- **AND** the page makes no further stream request

## ADDED Requirements

### Requirement: Pages share one flat look

Every page SHALL follow the reviewed canvas's look. The page background SHALL be one flat colour
with no grid or glow. Panels and cards SHALL be flat, with a 1 px border, no shadow and no corner
mark. Section headings SHALL be plain 20 px text with no marker. The page column and the footer
SHALL be at most 1840 px wide. A row of summary cards SHALL share its width in equal columns and
SHALL stack at narrow widths. Body text SHALL be 16 px, a page title at most 40 px, and mono text
13 px. Kickers, labels, table heads and fact labels SHALL use the page's sans face, and fact
labels SHALL be in sentence case. Text links in the page body SHALL be drawn in the accent
colour; the header, tabs, chips, badges, state-block reason links and graph nodes keep their own
colours.
Every colour SHALL be a token on `:root`, redefined for dark mode under the `prefers-color-scheme`
block guarded by `:root:not([data-theme="light"])` and again under `:root[data-theme="dark"]`.
Each status tone SHALL have a border, a text ink, a background and a tint token. The Instance and
Package kind chips SHALL take their colours from their own tokens, so that both read in light and
dark. No page SHALL scroll horizontally at a 360 px wide viewport. The graph pane's own grid is part
of the graph's drawing, not the page background, and is not covered by this requirement. Source:
portal:D19:R1/R2, portal:D17.

#### Scenario: A flat page

- **WHEN** a signed-in browser opens `/` on the F1 capture
- **THEN** the page background has no image, no panel or card casts a shadow or carries a corner
  mark, and no `h2` has a marker

#### Scenario: A wide screen

- **WHEN** `/installed` is shown in a 1920 px wide viewport
- **THEN** the page column is 1840 px wide, with the footer at the same width

#### Scenario: A phone

- **WHEN** `/`, `/installed`, `/instances/default/podinfo` and a Catalog page are shown in a
  360 px wide viewport
- **THEN** none scrolls horizontally, and the summary cards stack in one column

#### Scenario: The Instance chip in dark mode

- **WHEN** a browser that chose Dark opens `/installed` on the F1 capture
- **THEN** each Instance chip is a slate fill with light text, not a light fill with dark text

#### Scenario: Tone tokens in both themes

- **WHEN** the stylesheet is checked
- **THEN** every tone's border, ink, background and tint token is defined on `:root` and
  redefined, with the same values, in both dark blocks

#### Scenario: Header links keep their colours

- **WHEN** any page is shown
- **THEN** links in the page body are drawn in the accent colour and the header's brand,
  navigation and connection marks keep the header's own colours

### Requirement: Tabs are underlined and carry their counts

Every tab strip (the Platform's Providers and Catalogs, the Catalog page's tabs, and the instance
and package pages' tabs) SHALL be drawn as underline tabs: no box around a tab, the current tab
marked by a 3 px accent underline and the ink colour, the others muted. A tab SHALL be a link
carrying `tab=` that works without script. A tab SHALL show, after its label, the unfiltered
number of items it lists when the page already holds that number and it covers everything the tab
lists: Providers, Catalogs, Claims, Resources (inventory objects and runtime children) and Events
on instance and package pages (the folded lines of the feed the tab opens with). A tab whose list
the caller may not read in full, or whose source is a problem, SHALL show no count, never a
partial one. Showing a count SHALL not add a read. A count SHALL update when the page's regions
refresh live, without reloading the page. A region a tab opens SHALL not show a visible
heading of its own, since the selected tab names it, and SHALL keep that heading for assistive
technology. Source: portal:D19:R4, portal:D7:R2/R3.

#### Scenario: The F1 Platform tabs

- **WHEN** a signed-in browser opens `/` on the F1 capture
- **THEN** the tabs read "Providers 2" and "Catalogs 3", with Providers underlined as current

#### Scenario: Registrations not all readable

- **WHEN** the caller may read the Platform but not list TransformerRegistrations
- **THEN** neither the Providers nor the Catalogs tab shows a count, since the Catalogs tab lists
  catalogs that only a registration claims

#### Scenario: Platform forbidden

- **WHEN** the caller may not read the Platform
- **THEN** neither the Providers nor the Catalogs tab shows a count

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

#### Scenario: A count follows the page live

- **WHEN** podinfo's page is open on its Graph tab and a new event about the instance arrives
- **THEN** the Events tab's count updates without a reload, and the tabs keep working as links

#### Scenario: A region named by its tab

- **WHEN** podinfo's Resources tab is open
- **THEN** no visible heading repeats "Resources" above the list, and the region's accessible name
  still starts with "Resources"

### Requirement: Selecting a graph node fills the details panel

With script on, activating a graph node by pointer or keyboard SHALL load that node's details
into the details panel in place, showing its name, its kind, the axes it carries and its links,
and SHALL mark the node as selected. The request SHALL not take its swap selection from the page
around it, so the panel never shows empty after a selection. Without script, a node SHALL stay a
link that opens the page with the node selected and its details rendered by the server. A node
the caller may not read SHALL fill the panel with its locked form. Source: portal:D4,
portal:D19.

#### Scenario: Selecting podinfo's Deployment

- **WHEN** the user activates the `podinfo-podinfo` Deployment node on `/instances/default/podinfo`
  with script on
- **THEN** the details panel shows the name `podinfo-podinfo`, an Object fact naming the kind
  Deployment, and its health badge beside a visible "Health" label, and the node is marked
  selected

#### Scenario: Without script

- **WHEN** a browser with script disabled follows the same node
- **THEN** the page opens with that node selected and the details panel rendered with it

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
or else words that name no time (such as "checked live"), and nothing otherwise; an icon and a large uppercase state word in the state's tone; a summary
line; optional secondary lines under it (a reason's meaning, a partial or locked mark); an
optional caption saying what the reasons count; reason links, each a count and a reason linking
to the view that lists them, or a line saying there are none; and optional footer links. The state word SHALL carry the state without relying on colour. A state
the UI does not know SHALL render in the unknown tone with its word, never in an error tone. When
the block's source is forbidden or not readable, the block SHALL render that source locked or
degraded, with no state word, so a locked source never reads as a state. Every text in the block
SHALL be rendered as text. Source: portal:D3:R1, portal:D2:R3, portal:D7:R3, portal:D19:R5.

#### Scenario: A degraded health block

- **WHEN** a state block is drawn for a Degraded health with one `ImagePullBackOff` reason
- **THEN** it shows the bang icon, the word "Degraded" drawn uppercase in the degraded tone, its
  summary, and a link reading 1 `ImagePullBackOff`

#### Scenario: No recorded time

- **WHEN** a state block's source records no time for its state
- **THEN** the block shows no time, never one borrowed from another field

#### Scenario: A locked source

- **WHEN** a state block's source is `forbidden`
- **THEN** the block shows its eyebrow and the locked region, and no state word

#### Scenario: Hostile reason text

- **WHEN** a reason or summary holds `<script>alert(1)</script>`
- **THEN** the block shows it as text, escaped
