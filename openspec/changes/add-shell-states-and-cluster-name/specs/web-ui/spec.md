## MODIFIED Requirements

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

## ADDED Requirements

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
