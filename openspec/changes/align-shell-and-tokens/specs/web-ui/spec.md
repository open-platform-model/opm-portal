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

### Requirement: Muted text meets WCAG 2.2 AA contrast in both themes

Text drawn with the muted ink token SHALL have a contrast ratio of at least 4.5:1 against each
surface token it is drawn on (page, card, secondary card, field, the neutral fill and the degraded
fill), in the light and the dark theme. Text in a tone's ink SHALL have the same ratio on that
tone's background and tint, and so SHALL links in the accent colours on the page and surface
colours, and the muted and secondary ink on every tint. A canvas value that fails SHALL change to
one that passes, never the layout. The ratio is computed from the token hex values in `portal.css`
with the WCAG relative luminance formula and is not rounded. Source: portal:D19:R6, portal:D19:R7.

#### Scenario: Muted text on the page and on cards

- **WHEN** the light theme draws muted text on the page background, on a card, on a secondary card,
  on a field, or on the neutral or degraded fill
- **THEN** each pair has a ratio of at least 4.5:1

#### Scenario: A token pair falls below its floor

- **WHEN** a change lowers the ratio of a listed token pair under its floor in either theme
- **THEN** the contrast test fails and names the pair, the theme and the measured ratio

#### Scenario: Tone ink on its background and tint

- **WHEN** the contrast test checks each tone's ink, including the locked ink, on its background
  and its tint, in the light and the dark theme
- **THEN** every pair is at least 4.5:1, and no rule draws text in a tone's border token on its fill

#### Scenario: Links and secondary ink

- **WHEN** the contrast test checks the accent and deep accent on the page and surface colours,
  and the muted and secondary ink on every tint, in both themes
- **THEN** every pair is at least 4.5:1

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
page's regions refresh live, without reloading the page. Source: portal:D19:R4, portal:D7:R2/R3.

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

An info tip SHALL show its box when its trigger has the pointer or keyboard focus. The tip SHALL be
dismissable, hoverable and persistent: Escape closes it without moving focus, the pointer can move
onto the box without closing it, and the box stays until the user moves away or dismisses it. A tap
or Enter on the trigger SHALL open a closed tip and close an open one. The box holds text only.
Source: WCAG 2.2 1.4.13, portal:D19.

#### Scenario: Opens on focus

- **WHEN** keyboard focus reaches a tip's trigger
- **THEN** the box is shown, and the trigger's `aria-describedby` names it

#### Scenario: Escape closes the tip

- **WHEN** a tip is open and the user presses Escape
- **THEN** the box is hidden, focus stays on the trigger, and no other layer closes with it

#### Scenario: Enter opens it again

- **WHEN** the user has closed a tip with Escape and presses Enter on its trigger
- **THEN** the box is shown again

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
