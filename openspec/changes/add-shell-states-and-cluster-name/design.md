## Context

The pages are server-rendered HTML with htmx boosts and one `EventSource` per tab. A change on a
followed topic marks it dirty; 400 ms later `refresh()` in `static/portal.js` fetches the current
page once and replaces every `[data-follow]` region that follows a dirty topic, by id, from that
one document (`portal.js:245-345`). The header has a live mark `#live` (`role="status"`
`aria-live="polite"`), written only by `setLive` (`portal.js:224-231`). Everything else on the
screen is whatever the server wrote at render.

What is missing, each traced to source on the base `e4fd2e9`:

| Gap | Today | Evidence |
| --- | --- | --- |
| Relative times freeze | The `time` partial prints `<time datetime title>` with the server's relative text; no script touches it | `templates/partials.html:13-15`, `internal/ui/view.go:223-249` |
| A slow request shows nothing | `includeIndicatorStyles` is false and no `hx-indicator` is set, so no element carries `htmx-request` | `templates/layout.html:9`, no `hx-indicator` in `templates/` |
| A lost stream is a few words | `setLive(false, "reconnecting")` and `"offline"` change the live mark and nothing else | `portal.js:336-338,553` |
| An expired session is a few words | `sse:expired` closes the stream and sets the mark | `portal.js:544-549` |
| After a reconnect the regions are not re-fetched | `sse:open` clears `primed`, so the first snapshot of each topic is ignored as "what the page already shows" | `portal.js:489,507` |
| The panel is outside the refresh | `#detail` has no `data-follow`; `refresh()` only replaces elements that follow a topic | `templates/owner.html:208-212`, `portal.js:276-345` |
| The header shows the context only | `header()` copies `c.Context`; the entry name in the document is dropped | `internal/ui/ui.go:213-228`; `api/v1alpha1/types.go:381-384` |

The server already sends a `heartbeat` event every 15 seconds on every open stream
(`internal/stream/stream.go:90,144`, `broker.go:145,183`). The page does not listen for it: the
hidden `#stream-events` element lists the events it hooks (`templates/layout.html:21`).

This change is written against the designed behaviour of four changes that are merged as plans
(proposal, "Builds on"). Where a decision here depends on one of them, it says so.

## Goals / Non-Goals

**Goals**

- A relative time on screen is never older than 30 seconds against the clock it was computed from.
- A request that runs longer than 1 second shows that it is running, and ends the indication when
  the last such request ends.
- A page that is not receiving changes says so in words, with the time of the data it shows, and
  offers the way out. When it receives changes again, it is true again before the words go.
- The header names the cluster entry as portal:D18:R1 says.
- The details panel never contradicts the page beside it without saying so.

**Non-Goals**

- A new live region, a toast, a modal, or any announcement of ticking times. The one announcer for
  stream and session events stays `#live`.
- A new read, a new field of the read API, a new stream event, a new storage in the browser.
- The `Tip` (E1), per-region "No longer live" text for a closed topic, a stall watchdog for a
  half-open connection, and the visible absolute time beside every relative time (Open Questions).
- Any change to what the details panel shows. Only when it is replaced.

## Decisions

### The cluster entry name (portal:D18:R1)

The header's `header` view type gains `Entry string`. `header()` sets it from
`c.ClusterEntry` when the document's `Source` is `kubeconfig`, `Entry` is not empty, and it
differs from the first word the pill shows (the context, or the unknown word). In every other case
`Entry` is empty and the pill renders exactly as it does today (the common case: kind and most
kubeconfigs name context and cluster entry alike).

```go
type header struct {
	OK       bool
	Cluster  string // the context, "in cluster" or the unknown word
	Entry    string // the cluster entry's name, only when it adds something
	Context  string // keys what the browser remembers; unchanged
	Username string
	Version  string
}
```

The pill, when `Entry` is set:

```html
<span class="conn-cluster">
  <span class="mono" title="The kubeconfig context the portal reads with">{{.Header.Cluster}}</span>
  <span class="conn-entry"><span class="conn-k">cluster</span> <span class="mono">{{.Header.Entry}}</span></span>
</span>
```

The words "context" and "cluster" are not both shown as labels: the first value is the context
(its `title` says so, as today) and the second carries the visible word "cluster" because that is
the kubeconfig's own term for the entry. `data-context` on `<html>` keeps the context alone, so
what the browser remembers per cluster does not change (portal:D14). The pill wraps: at 760 px and
below `.conn-cluster` drops the 999 px radius for 12 px and the two parts stack; `.conn .mono`
already breaks anywhere (`portal.css:239`), so a name of 90 characters (an EKS ARN) wraps inside
the 328 px column. In-cluster the pill reads "in cluster" with no entry. When the document cannot
be fetched, the pill reads "unknown" in the degraded style with no entry.

**Research & Decisions entry** is below ("Where the cluster entry name comes from").

### Relative times keep counting

The `time` partial gains `data-relative` and `aria-live="off"`:

```html
<time datetime="{{.ISO}}" title="{{.Absolute}}" data-relative aria-live="off">{{.Relative}}</time>
```

The script rewrites `time[data-relative]` every 30 seconds. `aria-live="off"` makes the text
change inside the details panel (`#detail` is `aria-live="polite"`) not announced; it is a
nested override and needs a screen-reader pass (Unverified).

The words are the server's, by one table. `relative(d)` in `view.go` is the source; the script
repeats it:

| Age | Text |
| --- | --- |
| below 0 | `just now` |
| under 60 s | `<n>s ago` (whole seconds) |
| under 60 min | `<n>m ago` (whole minutes) |
| under 48 h | `<n>h ago` (whole hours) |
| otherwise | `<n>d ago` (whole days, floor of hours / 24) |

Two copies of a table can drift. One file, `internal/ui/testdata/relative-time.json`, lists pairs
of age in seconds and text, including 0, 59, 60, 3599, 3600, 172799, 172800 and a negative age.
The Go test of `relative` reads it; the browser test sets `datetime` attributes at those ages and
checks the script's text against the same rows.

**The clock.** The script must not trust the browser clock against a server timestamp. The layout
adds `<meta name="opm-portal-now" content="{{.Now}}">`, with the handler's clock (`Config.Now`),
and the script computes `skew = serverNow - Date.now()` once at load. All ages use
`Date.now() + skew`. A browser clock that is five minutes off therefore shows the same age as the
server did. Boosted navigation does not reload the head, and a clock skew does not change within
a session, so one reading at load is enough.

The rewrite sets the element's first text node (`el.firstChild.nodeValue`), and only when the text
differs, so a tick that changes nothing touches nothing. Tabular numerals (`font-variant-numeric:
tabular-nums` on `time[data-relative]`) keep the width from moving; a layout shift check is part
of the browser test (the element's width does not change by more than the glyph difference).
The tick runs while the tab is hidden at no extra cost; it also runs once when the tab becomes
visible, so a returning reader never reads a time that is minutes old.

### The loading state

One registry of open requests, keyed by the request object, not a counter:

```js
var open = new Map();                 // xhr -> target element
// htmx:beforeRequest   -> open.set(evt.detail.xhr, evt.detail.target); startTimer()
// htmx:afterRequest, htmx:sendError, htmx:responseError, htmx:timeout, htmx:abort
//                      -> open.delete(evt.detail.xhr); if (open.size === 0) endLoading()
```

A counter goes wrong when one request fires two end events (an error fires `responseError` and
then `afterRequest`) or none (a request replaced by a newer one on the same element). A map keyed
by the request makes a second delete a no-op, which is the carried nit of the plan: the hook
counts open requests and ends only when none is open (`design/T2.4.md` S3, nit a).

- **Start.** On the first `beforeRequest` of an empty registry a 1000 ms timer starts. When it
  fires and the registry is not empty, `body` gets the class `is-loading`.
- **The details panel.** When any open request's target is `#detail` at that moment, or a request
  into `#detail` begins while `is-loading` is set, `#detail` gets `aria-busy="true"` and the
  script prepends `<p class="detail-loading">Loading...</p>` (text via `textContent`). The
  response's `innerHTML` swap removes the line. `endLoading()` removes it, clears `aria-busy` and
  the class when the registry is empty or no request targets `#detail`.
- **The bar.** `body.is-loading .masthead::after` draws a 2 px bar along the bottom of the header,
  in the accent tone. It moves with a 1.2 s keyframe under `prefers-reduced-motion: no-preference`
  and is a static full-width bar otherwise. It is `pointer-events: none` and holds no text; the
  words are in `#detail` or, for a page navigation, nowhere, which is why it never carries meaning
  alone: the browser's own page transition and the arrival of the page are the outcome.
- **What it does not show.** The stream's background refresh uses `fetch`, not htmx
  (`portal.js:293`), so it never starts the timer. A pending log or topic POST uses `fetch` too.
- **Announcement.** None added. `aria-busy` on the polite region defers its announcements until
  it clears; the arriving panel is announced once, as today.

### The banner (RegionBanner)

A banner sits in the layout between `</header>` and `<main>`, inside `#app` so a boosted
navigation (which swaps `#main`) does not remove it:

```html
<div class="region-banner" id="region-banner" hidden>
  <p class="rb-text"></p>
  <a class="rb-reload" href="" hx-boost="false">Reload</a>
</div>
```

It has no `role` and no `aria-live`. `href=""` resolves to the current address with its query, and
`hx-boost="false"` makes it a full page load, which is what a closed stream needs (a boosted swap
would keep the old, closed `EventSource`). Without script the banner stays hidden, which is right:
the states it describes only arise with script.

One function sets both the live mark and the banner, so they cannot disagree:

```js
// state: "live" | "lost" | "expired" | "signedout"
function setStream(state, word) {      // word: the live mark's text, e.g. "Reconnecting"
  setLive(state === "live", word);
  showBanner(state, word);             // hides it for "live" unless a resync is running
}
```

| State | Live mark (E1's words) | Banner text | Reload link |
| --- | --- | --- | --- |
| lost (`Reconnecting`, `Offline`, `Topics refused`) | unchanged | `Showing data read at 10:42 UTC. <Word>.` | yes |
| expired | `Session expired, reload` | `Session expired. Showing data read at 10:42 UTC.` | yes |
| signed out | `Signed out` | `Signed out. Showing data read at 10:42 UTC.` | yes |

The words after "Showing data read at" are the live mark's own, so the page has one vocabulary.
The time is UTC, to match the absolute times the pages already print (`view.go:228`) and the logs
an operator compares them with; it also makes the test independent of the browser's time zone.

**Data time.** `lastRead` is the time of the last message the stream delivered, from any event the
hidden hook lists, `heartbeat` included (the layout adds `sse:heartbeat` to the list; the handler
only records the time). It starts at page load. With a heartbeat every 15 seconds, the stated time
is at most about 15 seconds earlier than the true last contact, and never later.

**Announcement.** `#live` keeps announcing: its text changes in `setLive`, as today. The banner
adds nothing a screen reader hears, because a second announcement of the same outage would read it
twice (WCAG 4.1.3; `design/T2.3.md` section 11, `RegionBanner`). A reader who moves to the banner
finds the same words with the time.

**Look.** The banner takes the neutral or progressing tone tokens of `align-shell-and-tokens` for
a lost stream and the degraded tone for expired and signed out; the words carry the state and the
tone only supports them. The pair `-ink` on `-tint` and the bar's accent on the header are added
to the contrast pair test of `fix-contrast-and-add-phone-check`. Forced colours: a 1 px border in
`CanvasText`. The banner is in flow (not sticky) so it never covers a heading at 200 percent text
size or at 360 px; the Reload link is 44 px high.

### Reconnecting: the page is true again before the banner goes

`sse:open` today clears `primed` so the first snapshot of every topic after it is treated as
"the page already shows this" and dropped (`portal.js:489,507`). That holds at page load.
It does not hold after a reconnect: the page may have missed changes during the outage, and if the
server could not resume the stream (a new stream id) nothing re-renders the regions. Clearing the
banner then would claim freshness the page does not have.

Decision: the script remembers whether a stream has opened before (`everOpened`). On an `sse:open`
that is not the first, it keeps `primed` cleared (so snapshots do not hide changes) and marks
every topic the page follows dirty (`pageTopics()`), so one `refresh()` re-renders all followed
regions from one fetch. The banner hides when that refresh settles without error; when the fetch
fails, `refresh()`'s catch already sets "Offline" and the banner stays. When no region follows
any topic, the banner hides at once.

If the server resumes the stream with the missed events (it receives `Last-Event-ID`,
`internal/stream/stream.go:87`), the extra refresh is redundant and harmless: one fetch. Whether
the server resumes or starts anew is an Assumption of the spike, not a precondition of the
design.

### The details panel follows the page, or says it is old

The panel holds one of three things: a node panel (the page render holds it for the address's
`node` or `focus`, or the resting node), a YAML view, or an Events view. The YAML and Events views
are fragments fetched into `#detail` by their links (`panel-body.html:27-28`); the address does
not hold them.

**A node panel follows.** `#detail` carries `data-topics="<owner topic>"` (the value the owner
cards follow, `owner.html:4`) and the script sets `data-follow` from it exactly while the panel is
a node panel. `refresh()` then replaces `#detail` by id from the one fetch, as it replaces every
other followed region, and `keepOpen`, `focusKey` and `refocus` already carry open `details` and
focus across. The replacement is the page render for the current address, which holds the selected
node's panel because a click writes `node` to the address (E4) and the resting panel when no
parameter is set (E3, E4).

**A YAML or Events view does not follow, and says so.** The two fragments mark their root:
`<div class="fragment" data-view="yaml" data-read="2026-10-11T10:42:07Z">`, with a hidden note and
a reload link as ordinary template text:

```html
<p class="stale-note" hidden>Read at <time datetime="…">10:42 UTC</time>. The object may have
changed since. <a href="{{.Reload}}" hx-get="{{.Reload}}" hx-target="#detail" hx-select="unset"
hx-swap="innerHTML">Reload this view</a></p>
```

The note's time is absolute, UTC, server-written from the handler's clock at render: no tick and
no relative text, so nothing in it goes stale in a second way. After a refresh in which a followed
topic was dirty, the script removes `hidden` from the note of the fragment in `#detail`, if any.
This is the only DOM change; the view's text, scroll position and any selection in it stay.

**The rule in the script.** After any swap into `#detail`, after `resetDetail`, and after a
region refresh: `follow = !#detail.querySelector("[data-view]")`; set or remove `data-follow`
accordingly. Three cases need care:

- A request into `#detail` is open when a refresh would replace it: `refresh()` skips `#detail`
  and marks the topic dirty again, so the replacement runs after the request settles. (htmx
  resolves a target selector when the request starts; replacing the element under it would drop
  the response.)
- The reader's focus is inside the panel (a link, the Clear selection pill): the existing
  `focusKey`/`refocus` pair restores it by key; the spike checks that a replaced `aside` does the
  same as a replaced section.
- The selected node is gone: `markSelected` already clears the selection and resets the panel.

**Announcement.** A replaced `aside` is a new live region inserted with its content, which screen
readers do not announce as a change. The panel's refresh is silent, as the summary cards' is.
This rests on screen-reader behaviour that this change does not test (Unverified); the stale note
is the one new text, and it is a short sentence inside the polite region, which is wanted.

**Fallback if the spike shows the page render does not hold the panel.** If the node panel can
differ from what the address renders, the node-panel fragment gets `data-panel-url` and the
script re-fetches that URL into `#detail` instead of replacing from the page render. Same rules,
same skip while a request is open, one more request per dirty refresh. The spec does not change.

### Shared files and merge order

E5 lands after E1, E3 and E4. It edits `static/portal.js` in separate places from E1 (the words
`setLive` writes, `openNode`'s `select`), E2 (the filter code), E3 and E4 (`resetDetail`, the
selection code). The functions it adds are new; the lines it edits in existing functions are named
here:

| File | This change edits only | Expected from other changes |
| --- | --- | --- |
| `static/portal.js` | `setLive` callers (through `setStream`), the `sse:open`, `sse:expired`, `htmx:sseError` handlers, the `catch` in `refresh()`, the region loop in `refresh()` for `#detail`, and new blocks (tick, loading, banner, resync) | E1: the words in `setLive`; E3/E4: `resetDetail`, `openNode`, `markSelected`; E2: filter code |
| `templates/layout.html` | the `meta` for the server time, `sse:heartbeat` in the hook list, the banner element, the cluster pill markup | E1: the live mark's initial text |
| `templates/partials.html` | the `time` define | E1: `state-block`, `tip`; E2/E3: `events-table`, `history` |
| `templates/owner.html` | `data-topics` on the `detail` define | E3: the rest of `detail` (`data-root-panel`) |
| `templates/object.html`, `events.html` | the fragment root's `data-view` and note | E3: the Events view uses `events-table` |
| `static/portal.css` | `.region-banner`, `.rb-*`, `.detail-loading`, `.masthead::after`, `.conn-entry`, `time[data-relative]` | E1: tokens |
| `Taskfile.yml:90` | names added to the `-run` pattern | E0, E2, E3 add names too; a conflict resolves by keeping every name |

Merge `main` into the branch first (`PROTOCOL.md`), re-run the goldens with `-update` and read
the diff.

## Research & Decisions

### Where the cluster entry name comes from

**Context**: `design/T3.4.md` section 3.8 left "where the cluster entry name comes from, and
whether it needs a read" open, and asked for the effect on Principle V if it does.

**Explored**: `api/v1alpha1/types.go:373-390` (the `Cluster` document has `Context` and
`ClusterEntry`, both absent in-cluster); `internal/api/handlers.go:116-134` (`getCluster` copies
both from `s.cfg.Connection` and "reads nothing from the cluster and sends no review");
`cmd/opm-portal/serve.go:218-232` (`configSource` fills `ClusterEntry` from
`raw.Contexts[contextName].Cluster`, and names no server URL, user entry or credential);
`internal/ui/ui.go:213-228` (the header reads the document and drops the entry).

**Options considered**:
1. Show the entry the document already carries. No read, no API change, no new data on the wire.
2. Read the entry from the kubeconfig file in the UI. The UI must not read anything the API does
   not expose (Principle III), and the kubeconfig is the API's connection, not the UI's.
3. Add a field to the document. There is nothing to add: the field exists.

**Decision**: Option 1.

**Rationale**: It is the whole of portal:D18:R1 with zero new surface. Principle V has no effect:
no new read, kind, identity or write. The entry name is a name in the user's own kubeconfig, and
D18:R1 already allows it ("never a server URL, kubeconfig user entry or credential").

### One announcer for stream and session events

**Context**: A banner that also announces would read every outage twice.

**Options considered**:
1. A second live region on the banner. Double announcement; WCAG 4.1.3 is met by `#live` already.
2. The banner as a visible-only line beside `#live`'s words. One announcer.
3. Make the banner the announcer and drop `#live`. `#live` is in every page, in the header; moving
   it is a larger change than this one.

**Decision**: Option 2 (`design/T2.3.md` section 11 `RegionBanner`, section 17).

**Rationale**: The words stay in one place that announces; the banner adds what a sighted reader
needs (the time, the way out).

### Follow the panel, or only mark it

**Context**: A stale panel contradicts the graph beside it. Two ways: replace it with the current
render, or leave it and mark it.

**Options considered**:
1. Mark every panel stale with a note and a reload link. No content shift under the reader. But the
   resting panel is the page's default view and any pod event would mark it, so the note becomes
   noise, and the reader does one click to see what the graph already knows.
2. Replace a node panel from the page render; mark a YAML or Events view. A node panel is derived
   from the same document as the cards and the graph, so replacing it keeps the page consistent
   with one fetch. A YAML or Events view is a different request, not held by the address, so it
   cannot be replaced from the page render and the reader chose it.
3. Re-fetch every panel from its own URL. An extra request per refresh and a second path.

**Decision**: Option 2, with option 3 as the stated fallback (see "Fallback").

**Rationale**: It matches `design/T2.3.md` S4 ("follow the owner topic only while the panel shows
a node") and the owner's cards. The reader's view is never replaced by something they did not ask
for.

### The data time is the last message, in UTC

**Context**: "Showing data read at 10:42" needs a time and a zone.

**Decision**: the time of the last stream message (heartbeat included), shown as `HH:MM UTC`.

**Rationale**: It is the last moment the page knew it was current, bounded by the 15 s heartbeat.
The pages print absolute times in UTC; a banner in another zone would make two answers.

## Assumptions

These are read from source and not run. Section 1 runs them in the browser harness
(`cmd/opm-portal/browser_test.go`, `serveExpiringPage` style fake streams) and writes the answers
under "Live checks" below before any code of sections 2 to 4.

1. **A refresh leaves `#detail` alone.** After a region refresh, a YAML or Events view in `#detail`
   is unchanged, and a node panel is also unchanged (stale). Traced: `#detail` has no
   `data-follow` (`owner.html:209`) and `refresh()` replaces only followed regions
   (`portal.js:276-313`). This is the question `design/T2.3.md` section 18 left open.
2. **A replaced `#detail` holds.** Replacing the `aside` from the page render keeps the focus that
   `refocus` restores, keeps a `details` open state, and the graph's `markSelected` still finds its
   node. An htmx request into `#detail` that started before the replacement is the unsafe case.
3. **A reconnect with a new stream id re-renders nothing today.** After `htmx:sseOpen` the server
   either resumes (backlog events reach `changed()`) or starts a new stream (the first snapshot of
   each topic is dropped as `primed`). Traced for the second case at `portal.js:507`.
4. **Every htmx request ends with exactly one of the end events carrying its `xhr`,** including
   a boosted navigation, an `htmx.ajax` into `#detail` and a request replaced by a newer one on the
   same element. `htmx:abort` and `htmx:timeout` are listed because htmx 2.0.11
   (`layout.html:16`) is documented to fire them; the spike confirms.
5. **`heartbeat` reaches the page when its name is in the hook list.** The `htmx-ext-sse` 2.2.4
   extension dispatches `sse:<name>` for the names in `hx-trigger`.

If assumption 1 fails (a refresh does replace `#detail`), section 4 shrinks to the YAML and Events
note. If assumption 3 shows the server always resumes, the resync stays as a cheap guarantee. If
assumption 4 fails for a case, the registry also clears on `htmx:afterSettle` for its target; if
that is not enough, the proposal stops and asks.

## Risks / Trade-offs

- **A replaced `#detail` loses a text selection.** A reader copying a message from the panel
  loses the selection at the next refresh. The cards have the same behaviour and the 400 ms
  debounce limits it. Accepted; recorded so the reviewer does not read it as a bug.
- **`aria-live="off"` on a `time` inside a polite region** may not be honoured by every screen
  reader. If it is not, a time in the panel is announced every 30 seconds. The panel shows no
  relative time at rest (the node panel has none; the Events view has them). Mitigation if found:
  skip ticking elements inside `[aria-live]` regions. Not assessed here.
- **Clock skew handling is one reading.** A device that sleeps and wakes with a corrected clock
  moves the skew. The tick re-reads `meta` only at load. A refreshed fragment carries fresh server
  relative text, so an error self-heals at the next refresh.
- **The banner adds a block above `main`.** The page below shifts down when it appears. It appears
  on a fault, once, and is in flow. It is not sticky, so the layout never covers content. The
  scroll position of the page is kept by the browser's scroll anchoring; the browser test checks
  that a heading in view does not jump more than the banner's height.
- **Re-fetch storm after an outage.** Every client of an outage refreshes once on return. The
  server's page render is the same one the cards already trigger on every change; one fetch per
  client per reconnect is the same cost as a burst of changes.
- **A half-open connection shows "Live" forever.** The browser does not fire `onerror` for a
  connection that stopped delivering. The banner cannot appear. A watchdog on the time since the
  last message would catch it; it needs the heartbeat interval on the page. Left as an open
  question rather than guessed.
- **Shared `portal.js`.** Five changes edit it. The table above names the lines; the merge order
  of `design/T3.4.md` section 3.2 (E2, E3, E4, then this) keeps the rebases small.

## Migration Plan

None. No API, schema, flag or stored value changes. The change ships as one PR of four section
commits; rolling it back is a revert of the PR. A page loaded before the deploy keeps working: the
old script ignores the new markup (the banner is hidden, the `meta` is unused) and the new script
tolerates an older page without them (it does not tick when the meta is missing, and shows no
banner when the element is missing).

## Open Questions

1. **Absolute time beside relative time.** `design/T2.3.md` section 11 wants the absolute time as
   visible text in the detail views, because `title` is reachable by mouse only. This change keeps
   `title` and ticks the relative text. Adding visible absolute times touches the state block and
   the history table, which `align-shell-and-tokens` and `align-owner-pages` own. Not scheduled;
   the supervisor decides whether it is a follow-up change.
2. **A stall watchdog.** Show the banner when no message has arrived for a multiple of the
   heartbeat. Needs the interval on the page (a `meta` or a field in the `open` event). Not
   scheduled.
3. **"No longer live" on a closed region.** `sse:closed` adds a class and a `title`
   (`portal.js:534-537`); a sighted keyboard or touch reader sees nothing. Not scheduled.

## Live checks

To be written by section 1 (the spike): for each of assumptions 1 to 5, the browser, the page, the
steps and what was seen, with the version of Playwright the image pins.
