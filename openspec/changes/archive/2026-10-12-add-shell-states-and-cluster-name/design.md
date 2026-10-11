## Context

The pages are server-rendered HTML with htmx boosts and one `EventSource` per tab. A change on a
followed topic marks it dirty; 400 ms later `refresh()` in `static/portal.js` fetches the current
page once and replaces every `[data-follow]` region that follows a dirty topic, by id, from that
one document (`portal.js:413-493`). The header has a live mark `#live` (`role="status"`
`aria-live="polite"`), written only by `setLive` (`portal.js:373-380`). Everything else on the
screen is whatever the server wrote at render.

What is missing, each traced to source on `main` at `360382a` (E0 to E4 merged), as section 1 re-read it:

| Gap | Today | Evidence |
| --- | --- | --- |
| Relative times freeze | The `time` partial prints `<time datetime title>` with the server's relative text; no script touches it | `templates/partials.html:106-108`, `internal/ui/view.go:271-302` |
| A slow request shows nothing | `includeIndicatorStyles` is false and no `hx-indicator` is set, so no element carries `htmx-request` | `templates/layout.html:9`, no `hx-indicator` in `templates/` |
| A lost stream is a few words | `setLive(false, "Reconnecting")`, `"Offline"`, `"Topics refused"` and `"Signed out"` change the live mark and nothing else | `portal.js:348,352,444,486,702` |
| An expired session is a few words | `sse:expired` closes the stream and sets the mark | `portal.js:693-698` |
| After a reconnect the regions are not re-fetched | `sse:open` clears `primed`, so the first snapshot of each topic is ignored as "what the page already shows" | `portal.js:634,654-657` |
| The panel is outside the refresh | `#detail` has no `data-follow`; `refresh()` only replaces elements that follow a topic | `templates/owner.html:206-209`, `portal.js:425-493` |
| The header shows the context only | `header()` copies `c.Context`; the entry name in the document is dropped | `internal/ui/ui.go:213-227`; `api/v1alpha1/types.go:404-407` |

The server already sends a `heartbeat` event every 15 seconds on every open stream
(`internal/stream/stream.go:90,144`, `broker.go:145,183`). The page does not listen for it: the
hidden `#stream-events` element lists the events it hooks (`templates/layout.html:21`).

This change builds on the five canvas changes now merged and built on `main` (E0 to E4, PRs 43, 44,
47, 48 and 49; proposal, "Builds on"). Section 1 re-read the files they changed; where a decision
here depends on one of them, it says so.

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

The pill on `main` is one span, `<span class="conn-cluster mono" title="The kubeconfig context the
portal reads with">` (`templates/layout.html`, `portal.css:322`, a bordered 999 px pill). When
`Entry` is set, the span keeps `conn-cluster` and its `title`, loses `mono`, and holds two parts:

```html
<span class="conn-cluster conn-two" title="Context {{.Header.Cluster}}, cluster {{.Header.Entry}}">
  <span class="conn-name mono">{{.Header.Cluster}}</span>
  <span class="conn-entry"><span class="conn-k">cluster</span> <span class="conn-name mono">{{.Header.Entry}}</span></span>
</span>
```

The words "context" and "cluster" are not both shown as labels: the first value is the context
and the second carries the visible word "cluster" because that is the kubeconfig's own term for
the entry. The tip holds the full values, "Context X" for one name and "Context X, cluster Y" for
two (a fixed sentence about the kubeconfig would add nothing the reader can use). In a Pod there
is no kubeconfig context, so that pill's tip reads "In cluster: the credentials of the Pod" and
does not call the mode a context (T4.69). `data-context`
on `<html>` keeps the context alone, so what the browser remembers per cluster does not change
(portal:D14).

**The pill never grows the header** (supervisor ruling T4.64, applied in T4.67). Each name is one line,
cut with an ellipsis inside the pill (`white-space: nowrap; overflow: hidden; text-overflow:
ellipsis`), never wrapped. The text of both names stays in the page, so a screen reader reads the
full values and the tip shows them. At 760 px and below the pill takes its own row (`flex: 0 1
100%`), drops the 999 px radius for 12 px, and the two parts stack, so 90 character names and short
names make the same header height (within 2 px, asserted in three engines, with and without the
banner). Above 760 px a name is cut at the pill's cap (about 44 characters for one name, 76 for
two). In-cluster the pill reads "in cluster" with no entry. When the document cannot be fetched,
the pill reads "unknown" in the degraded style with no entry.

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
// htmx:afterRequest   -> open.delete(evt.detail.xhr); if (open.size === 0) endLoading()
//                        (also on htmx:sendError, htmx:responseError and htmx:timeout, as a
//                        second delete of the same key)
```

`htmx:afterRequest` is the end event of every path: a success, an error answer, a send error, a
timeout, and an abort, which also fires `htmx:sendAbort`. A request replaced through `hx-sync` is
aborted the same way, so it also ends with `afterRequest`. `htmx:abort` is not an end event: it is
the event htmx listens for in order to abort a request, and it carries no `xhr`. A counter goes
wrong when one request fires two end events (an error fires `responseError` and then
`afterRequest`). A map keyed by the request makes a second delete a no-op, which is the carried
nit of the plan: the hook counts open requests and ends only when none is open (`design/T2.4.md`
S3, nit a).

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
  (`portal.js:442`), so it never starts the timer. A pending log or topic POST uses `fetch` too.
- **Announcement.** None added. `aria-busy` on the polite region defers its announcements until
  it clears; the arriving panel is announced once, as today.

### The banner (RegionBanner)

The banner is the last row of the sticky header, so a reader scrolled down a long page (Resources,
Events) still sees it. It sits inside `<header class="masthead">` after `.conn`, inside `#app`, so a
boosted navigation (which swaps `#main` and `#nav` only) does not remove it:

```html
<div class="region-banner" id="region-banner" hidden>
  <p class="rb-text"><span class="rb-lead"></span>Showing data read
    <time class="rb-time" data-relative aria-live="off" datetime="" title=""></time>.<span class="rb-word"></span></p>
  <a class="rb-reload" href="" hx-boost="false">Reload</a>
</div>
```

It has no `role` and no `aria-live` of its own. `href=""` resolves to the current address with
its query, and `hx-boost="false"` makes it a full page load, which is what a closed stream needs (a
boosted swap would keep the old, closed `EventSource`). Without script the banner stays hidden,
which is right: the states it describes only arise with script.

One function sets both the live mark and the banner, so they cannot disagree:

```js
// state: "live" | "lost" | "expired" | "signedout"
function setStream(state, word) {      // word: the live mark's text, e.g. "Reconnecting"
  setLive(state === "live", word);
  showBanner(state, word);             // after a loss, hides it only by the rule of the next section
}
```

| State | Live mark (E1's words) | Banner text | Reload link |
| --- | --- | --- | --- |
| lost (`Reconnecting`, `Offline`, `Topics refused`) | unchanged | `Showing data read 42s ago. <Word>.` | yes |
| expired | `Session expired, reload` | `Session expired. Showing data read 42s ago.` | yes |
| signed out | `Signed out` | `Signed out. Showing data read 42s ago.` | yes |

The words around the time are the live mark's own, so the page has one vocabulary. The script
writes `.rb-lead` ("Session expired. " or "Signed out. ", empty for a lost stream) and `.rb-word`
(" Reconnecting.", empty for expired and signed out) with `textContent`, each with its own
spacing.

**Data time.** `lastRead` is the time of the data the page shows. It starts at page load and, while
the stream is healthy, follows the last message the stream delivered, from any event the hidden
hook lists, `heartbeat` included (the layout adds `sse:heartbeat` to the list; the handler only
records the time). With a heartbeat every 15 seconds, the stated time is at most about 15 seconds
earlier than the true last contact, and never later.

`lastRead` freezes from the first sign of a loss (`htmx:sseError`, the `error` event, the `catch`
in `refresh()`, `sse:expired`) until a resync succeeds. In that window the reopened stream
delivers the `open` event, snapshots and heartbeats, and none of them moves `lastRead`: the regions
on screen are still the render from before the outage, and the banner must not say "data read just
now" over them. A resync that returned 200 and replaced its regions sets `lastRead` to the moment
of its answer and ends the freeze. The time the banner states is therefore always the time of the
data on screen.
**The time's form is the `time` partial's.** The banner prints the read time the way every other
time on the pages is printed (`templates/partials.html:106-108`, `internal/ui/view.go:271-288`):
relative text, the ISO time in `datetime` and the absolute time in `title`
(`2006-01-02 15:04:05 UTC`). The script sets `datetime` from `new Date(lastRead + skew)`, in UTC,
and `title` from the same instant in the `title` form, with `setAttribute`; the text comes from
the table of "Relative times keep counting". The element carries `data-relative`, so the tick
rewrites it with the others and the banner keeps counting ("42s ago", then "1m ago") while the
stream stays down. No new time form exists, and no time zone enters the text, so the browser test
does not depend on the browser's time zone. The absolute time shows as a tooltip only, as it does
everywhere else on the pages; a visible absolute time is Open Question 1.

**Announcement.** `#live` keeps announcing: its text changes in `setLive`, as today. The banner
adds nothing a screen reader hears, because a second announcement of the same outage would read it
twice (WCAG 4.1.3; `design/T2.3.md` section 11, `RegionBanner`). A reader who moves to the banner
finds the same words with the time. The `time` inside it is marked `aria-live="off"`, as the
`time` partial is.

**Look.** The banner is a full-width row of the header with its own background: the neutral or
progressing tone tokens of `align-shell-and-tokens` for a lost stream and the degraded tone for
expired and signed out. The words carry the state and the tone only supports them. The pair `-ink`
on `-tint` and the bar's accent on the header are added to the contrast pair test of
`fix-contrast-and-add-phone-check`. Forced colours: a 1 px border in `CanvasText`. The row is
capped at `max-height: 8rem` with `overflow-y: auto`: the text is at most about 90 characters, so
at 360 px it takes two or three lines beside a Reload link that is 44 px high, and the cap only
bites at large text sizes. The header slims after 48 px of scroll (`.masthead.compact`); the
banner row keeps its size, because its words and its link are the point of the row.

**The header's height is a variable, and the page keeps clear of it (WCAG 2.2 2.4.11).** A sticky
header that grows by a banner can cover a focused element: Shift+Tab to a link above the viewport,
or a fragment link, scrolls it to the top edge, under the header. The base has no
`scroll-padding`. So `html` carries `scroll-padding-top: calc(var(--masthead-h, 4.5rem) + 8px)`,
and the sticky details panel takes its offset from the same variable
(`#detail { top: calc(var(--masthead-h, 72px) + 8px) }`, today `80px` at `portal.css:387`, and its
`max-height` subtracts the same). The script sets `--masthead-h` from the header's border-box height
(`offsetHeight`, or `borderBoxSize` from the observer; `contentRect` leaves out the padding and read
40 of 68 px in the spike) with
`document.documentElement.style.setProperty` (CSSOM, never a `style` attribute), at load and from a
`ResizeObserver` on `.masthead`, so it follows the slimmed header after a scroll of 48 px, the wrapped
header on a phone, a text size of 200 percent and the banner coming and going. Without script the
variable keeps its fallback, which is the header without a banner. Whether CSSOM `setProperty`
passes `style-src 'self'` in all three engines is Assumption 6 of the spike; if it does not, the
fallback is a class on `html` (`has-banner`) with fixed paddings in rem for the compact header, the
full header and the banner cap, which is coarser and covers the same cases.
### Reconnecting: the page is true again before the banner goes

`sse:open` today clears `primed` so the first snapshot of every topic after it is treated as
"the page already shows this" and dropped (`portal.js:634,654-657`). That holds at page load.
It does not hold after a reconnect: the page may have missed changes during the outage, and if the
server could not resume the stream (a new stream id) nothing re-renders the regions. Clearing the
banner then would claim freshness the page does not have.

Decision: the script remembers whether a stream has opened before (`everOpened`). The first open
of a page sets "Live" as today. After a loss, the page is `resyncing` from the first sign of the
new connection until its resync settles, and the rules are these:

- `htmx:sseOpen` (the connection opened, `portal.js:705`) and the server's `open` event
  (`portal.js:626`) do not set "Live" and do not hide the banner. The live mark keeps the word of
  the loss ("Reconnecting") and the banner stays, so a reader never sees "Live" or an empty header
  before the page is true again. They do not move `lastRead` either (see "Data time").
- On the server's `open` event that is not the first, the script keeps `primed` cleared (so
  snapshots do not hide changes) and marks every topic the page follows dirty (`pageTopics()`),
  so one `refresh()` re-renders all followed regions from one fetch. This is the resync: the
  first refresh that starts after that `open` event. A refresh already in flight does not count;
  the dirty topics wait for the next one through the existing `schedule()` after `inflight`.
- The resync settles in one of four ways. The fetch answered 200 and the regions are replaced:
  only then `setStream("live", "live")` runs, the live mark reads "Live" and the banner hides.
  The fetch answered 401: `setStream("signedout")` runs and the banner stays, reading "Signed
  out" (the same 401 path as `portal.js:443-446`, which settles `refresh()` without an error and
  must not be read as success). The fetch failed or answered another status than 200 or 401: the
  `catch` sets "Offline" and the banner stays. The page moved on while the fetch ran
  (`portal.js:452-458`): the topics are marked dirty again and the banner stays until the next
  resync settles.
- **A failed resync is tried again.** While the page is still `resyncing`, the next stream event of
  any kind (`open`, a topic snapshot, a heartbeat) marks every followed topic dirty again and
  `schedule()` runs one more `refresh()`. A heartbeat comes every 15 seconds, so the wait is
  bounded and the page never stays "Offline" on a stream that is open and delivering. A fetch
  answered 401 is not retried: the session is gone, and the Reload link is the way out.
- A loss before the first `open` of a page (the first connection fails once) needs no resync: the
  page was rendered by the server a moment ago, and the first `open` sets "Live" and hides the
  banner.
- When no region follows any topic, there is nothing to re-render: the banner hides and the mark
  reads "Live" on the server's `open` event.

If the server resumes the stream with the missed events (it receives `Last-Event-ID`,
`internal/stream/handler.go:63`, matched in `internal/stream/broker.go:474`), the extra refresh is redundant and harmless: one fetch. Whether
the server resumes or starts anew is an Assumption of the spike, not a precondition of the
design. The spike also confirms that a refresh in flight at the `open` event delays the resync and
does not hide the banner.

### The details panel follows the page, or says it is old

The panel holds one of three things: a node panel (the page render holds it for the address's
`node` or `focus`, or the resting node), a YAML view, or an Events view. The YAML and Events views
are fragments fetched into `#detail` by their links (`panel-body.html:40`); the address does
not hold them.

**A node panel follows.** `#detail` carries `data-topics="<owner topic>"` (the value the owner
cards follow, `owner.html:4`) and the script sets `data-follow` from it exactly while the panel is
a node panel. `refresh()` then replaces `#detail` by id from the one fetch, as it replaces every
other followed region, and `keepOpen` carries open `details` across. Focus is carried by `focusKey` and `refocus`, which
today restore only an element with an `id` or a graph node (`portal.js` `focusKey`, `refocus`),
and the panel links (Open page, Expand this group, YAML, Events, Logs) have no `id`
(`templates/panel-body.html:40`). So this change gives each panel link a stable key,
`data-focus-key="panel-open"`, `"panel-expand"`, `"panel-yaml"`, `"panel-events"` and
`"panel-logs"` (one panel on a page, so each is unique), and `focusKey` records
`data-focus-key` beside the `id` and the graph node, and `refocus` finds the element with the
same `data-focus-key`. There are two Clear selection controls
(`data-clear-selection`): the pill at the top of a node panel (`panel-body.html:4`, inside
`#detail`) takes the key `panel-clear`, and the toolbar link of the graph (`partials.html:212`,
inside `#graph`, which `refresh()` already replaces) takes `clear-<graph id>`, for the same reason. A keyboard reader on the YAML link then keeps
focus through each refresh. The key is an attribute of the template, never text from the cluster. The replacement is the page render for the current address, which holds the selected
node's panel because a click writes `node` to the address (E4) and the resting panel when no
parameter is set (E3, E4).

**A YAML or Events view does not follow, and says so.** The two fragments mark their root: `<div class="fragment" data-view="yaml">`, with a hidden note
and a reload link as ordinary template text. The note prints the time through the existing `time`
partial, so it has the one form every time on the pages has:

```html
<p class="stale-note" hidden>Read {{template "time" .Read}}. The object may have changed since.
<a href="{{.Reload}}" hx-get="{{.Reload}}" hx-target="#detail" hx-select="unset"
hx-swap="innerHTML">Reload this view</a></p>
```

which renders as `Read <time datetime="2026-10-11T10:42:07Z" title="2026-10-11 10:42:07 UTC"
data-relative aria-live="off">4m ago</time>`. The read time is the handler's clock at render
(`Config.Now`), so the server writes it and the page script does not guess it. The `time` carries
`data-relative`, so the tick keeps the note's "4m ago" true while the note is hidden or shown; it
is the same element the rest of the pages tick, in the same polite region with the same
`aria-live="off"`. After a refresh in which a followed
topic was dirty, the script removes `hidden` from the note of the fragment in `#detail`, if any.
This is the only DOM change; the view's text, scroll position and any selection in it stay.

**The rule in the script.** After any swap into `#detail`, after `resetDetail`, and after a
region refresh: `follow = !#detail.querySelector("[data-view]")`; set or remove `data-follow`
accordingly. Three cases need care:

- A request into `#detail` is open when a refresh would replace it: `refresh()` skips `#detail`
  and marks the topic dirty again, so the replacement runs after the request settles. (htmx
  resolves a target selector when the request starts; replacing the element under it would drop
  the response.)
- The reader's focus is inside the panel (a link, the Clear selection pill): the `focusKey` and
  `refocus` pair, extended as above, restores it by `data-focus-key`; the spike checks that a
  replaced `aside` does the same as a replaced section.
- The selected node is gone: `markSelected` already clears the selection and resets the panel.

**Announcement.** A replaced `aside` is a new live region inserted with its content, which screen
readers do not announce as a change. The panel's refresh is silent, as the summary cards' is.
This rests on screen-reader behaviour that this change does not test (Unverified); the stale note
is the one new text, and it is a short sentence inside the polite region, which is wanted.

**The spike shows the page render holds the panel,** so the fallback below is not built (see "Live
checks", assumption 2). One difference to look at in the screenshots of task 4.7: the panel the page
renders inside `#detail` has no `<div class="fragment">` wrapper, which the `/node` fragment has;
`portal.css:420` gives `.fragment` only `min-width: 0`.

**Fallback if the page render did not hold the panel.** If the node panel could
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
| `static/portal.js` | `setLive` callers (through `setStream`), the `sse:open`, `sse:expired`, `htmx:sseError` handlers, the `catch` in `refresh()`, the region loop in `refresh()` for `#detail`, `focusKey` and `refocus` (one more key), and new blocks (tick, loading, banner, resync) | E1: the words in `setLive`; E3/E4: `resetDetail`, `openNode`, `markSelected`; E2: filter code |
| `templates/layout.html` | the `meta` for the server time, `sse:heartbeat` in the hook list, the banner element (the last child of the header), the cluster pill markup | E1: the live mark's initial text |
| `templates/partials.html` | the `time` define; `data-focus-key` on the Clear selection pill | E1: `state-block`, `tip`; E2/E3: `events-table`, `history`; E4: the graph toolbar |
| `templates/owner.html` | `data-topics` on the `detail` define; the `reveal` class removed | E3: the rest of `detail` (`data-root-panel`) |
| `templates/panel-body.html` | `data-focus-key` on the five panel links | E3, E4: the rest of the body (`panel.go` fields) |
| `templates/catalog.html`, `installed.html`, `message.html`, `platform.html` | the `reveal` class removed (SH-A5 removed the CSS; E5 lands last) | none |
| `templates/object.html`, `events.html` | the fragment root's `data-view` and note | E3: the Events view uses `events-table` |
| `static/portal.css` | `.region-banner`, `.rb-*`, `.detail-loading`, `.masthead::after`, `.conn-entry`, `time[data-relative]`, `html` `scroll-padding-top`, the `top` and `max-height` of `#detail` | E1: tokens |
| `internal/ui/refresh_test.go`, `internal/ui/testdata/golden/` | the `reveal` class in the fixture string at `refresh_test.go:67`; goldens regenerated | none |
| `Taskfile.yml:90` | names added to the `-run` pattern | E0, E2, E3 add names too; a conflict resolves by keeping every name |

Merge `main` into the branch first (`PROTOCOL.md`), re-run the goldens with `-update` and read
the diff.

## Research & Decisions

### Where the cluster entry name comes from

**Context**: `design/T3.4.md` section 3.8 left "where the cluster entry name comes from, and
whether it needs a read" open, and asked for the effect on Principle V if it does.

**Explored**: `api/v1alpha1/types.go:396-409` (the `Cluster` document has `Context` and
`ClusterEntry`, both absent in-cluster); `internal/api/handlers.go:116-134` (`getCluster` copies
both from `s.cfg.Connection` and "reads nothing from the cluster and sends no review");
`cmd/opm-portal/serve.go:218-232` (`configSource` fills `ClusterEntry` from
`raw.Contexts[contextName].Cluster`, and names no server URL, user entry or credential);
`internal/ui/ui.go:213-227` (the header reads the document and drops the entry).

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

### The data time is the last message, in the `time` partial's form

**Context**: "Showing data read ..." needs a time. The banner and the stale note follow the
existing `time` partial rather than a new UTC form, so the pages keep one convention.

**Decision**: the time of the last stream message (heartbeat included), printed as a `time`
element: relative text, ISO in `datetime`, `2006-01-02 15:04:05 UTC` in `title`.

**Rationale**: It is the last moment the page knew it was current, bounded by the 15 s heartbeat.
Every other time on the pages already has this form, so the page has one convention, the tick
keeps the banner's text true, and the tooltip carries the absolute time in UTC, so the test does
not depend on a time zone.

## Assumptions

These were read from source. Section 1 ran them in the browser harness
(`cmd/opm-portal/browser_test.go`) and wrote the answers under "Live checks" below before any code
of sections 2 to 4. All six held.

1. **A refresh leaves `#detail` alone.** After a region refresh, a YAML or Events view in `#detail`
   is unchanged, and a node panel is also unchanged (stale). Traced: `#detail` has no
   `data-follow` (`owner.html:207`) and `refresh()` replaces only followed regions
   (`portal.js:425-461`). This is the question `design/T2.3.md` section 18 left open.
2. **A replaced `#detail` holds.** Replacing the `aside` from the page render keeps the focus that
   `refocus` restores, keeps a `details` open state, and the graph's `markSelected` still finds its
   node. An htmx request into `#detail` that started before the replacement is the unsafe case.
3. **A reconnect with a new stream id re-renders nothing today.** After `htmx:sseOpen` the server
   either resumes (backlog events reach `changed()`) or starts a new stream (the first snapshot of
   each topic is dropped as `primed`). Traced for the second case at `portal.js:654-657`.
4. **Every htmx request ends with `htmx:afterRequest` carrying its `xhr`,** including a boosted
   navigation, an `htmx.ajax` into `#detail`, a request aborted, and a request replaced by a newer
   one on the same element through `hx-sync`. Read from the vendored htmx 2.0.11
   (`internal/ui/static/vendor/htmx-2.0.11.min.js`): an abort ends with `htmx:afterRequest` and
   `htmx:sendAbort`. The spike confirms each kind.
5. **`heartbeat` reaches the page when its name is in the hook list.** The `htmx-ext-sse` 2.2.4
   extension dispatches `sse:<name>` for the names in `hx-trigger`.
6. **A script can set a custom property on `html` under the page policy.** `style.setProperty`
   on the root element, and a `ResizeObserver` on `.masthead`, work with `style-src 'self'` in all
   three engines, and `scroll-padding-top` and the sticky `#detail` offset follow the property.

If assumption 6 fails, the `has-banner` class fallback of "The header's height" replaces the
variable. If assumption 1 fails (a refresh does replace `#detail`), section 4 shrinks to the YAML and Events
note. If assumption 3 shows the server always resumes, the resync stays as a cheap guarantee. If
assumption 4 fails for a case, the registry also clears on `htmx:afterSettle` for its target; if
that is not enough, the proposal stops and asks.

## Risks / Trade-offs

- **A replaced `#detail` loses a text selection.** A reader copying a message from the panel
  loses the selection at the next refresh. The cards have the same behaviour and the 400 ms
  debounce limits it. Accepted.
- **`aria-live="off"` on a `time` inside a polite region** may not be honoured by every screen
  reader. If it is not, a time in the panel is announced every 30 seconds. The panel shows no
  relative time at rest (the node panel has none; the Events view has them). Mitigation if found:
  skip ticking elements inside `[aria-live]` regions. Not assessed here.
- **Clock skew handling is one reading.** A device that sleeps and wakes with a corrected clock
  moves the skew. The tick re-reads `meta` only at load. A refreshed fragment carries fresh server
  relative text, so an error self-heals at the next refresh.
- **The banner makes the sticky header taller.** It is the last row of the header, so a reader
  scrolled down a long page sees it, with the data time and the Reload link. The price is that the
  header covers up to the banner's height (capped at 8rem, about 3 lines plus the link at 360 px)
  of the page while the banner shows, for the whole outage. The base had no `scroll-padding-top`,
  so a focused element could sit under the header already; this change adds the variable and the
  padding of "The header's height", so the old case is fixed too. The page below shifts down once
  when the banner appears; the browser test checks that a heading in view does not jump more than
  the banner's height, that the banner is in view after a scroll of 3000 px, and that a link
  reached by Shift+Tab or a fragment link is not covered by the header, with and without the
  banner.
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
   the history table, which `align-shell-and-tokens` and `align-owner-pages` own. Not scheduled in
   track 1; it can become a later change on the owner's ask.
2. **A stall watchdog.** Show the banner when no message has arrived for a multiple of the
   heartbeat. Needs the interval on the page (a `meta` or a field in the `open` event). Not
   scheduled in track 1; it can become a later change on the owner's ask.
3. **"No longer live" on a closed region.** `sse:closed` adds a class and a `title`
   (`portal.js:676-688`); a sighted keyboard or touch reader sees nothing. Not scheduled in track 1; it can become a later
   change on the owner's ask.

## Deviations

What the built change does differently from the text above. Each was accepted by the supervisor
(`DECISIONS.md`, the rows for T4.61, T4.64, T4.65 and T4.66), and none changes a requirement.

- **A loading line is removed by `paintLoading`, not only by the swap** (T4.61). One function
  repaints the bar, the busy state and the line from the registry, so an abort or a 500 leaves no
  stale line. The page's `Now` meta is RFC 3339 at second precision, which the 30 s tick does not
  outgrow.
- **Task 2.10's second half is not built** (T4.61). The bar and the line do not change the scroll
  width (`loading.py`, step 7), so the Graph pages are not added to `TestBrowserPhone`.
- **`conn-two`** (T4.64). The two-name pill carries a second class, so the 12 px radius and the
  stacking apply to it alone, with no `:has()`.
- **Recovery after a failed topic POST** (T4.64). A refused or offline POST sets a flag, and the
  next POST that succeeds, while the page is lost and the stream is connected, starts the resync,
  so the banner does not wait for the stream to reconnect.
- **A non-200 is "Offline" only for the resync fetch** (T4.64). Outside a resync the refresh fetch
  keeps its old handling, so the 404 page of a deleted object does not raise the banner.
- **`phone_banner.py` runs as `TestBrowserPhone/<engine>/banner`**, not as new entries of
  `phonePages` (T4.64). Its "200 percent" leg sets the root font size to 200 percent, not a browser
  zoom: only rem sizes follow it. The banner and the "cluster" label are in rem since T4.67, and
  the leg asserts that the header grows by at least 8 px, so it fails when the header stops
  following the text size. Other header text is still in px (an old rule of `portal.css`, outside
  this change).
- **`TestBrowserExpired` expects two streams** (T4.64): the page's and the one the Reload link opens.
- **`scroll-padding-top` is the header's height plus 8 px** (T4.64, ruled). The spec says "at least
  the header's height".
- **Task 4.5 is built in `stale.py` only** (T4.65). `TestBrowserGraph` serves the capture with the
  real broker and cannot change the cluster; the stale site lets the script move the page renders
  and send frames. `graph.py` is not edited.
- **One focus key for Expand group and Collapse group** (T4.65): `panel-expand`, because one of the
  two shows at a time; the note's Reload link carries `stale-reload`; the Events link of the
  owner's own node carries `panel-events`. The design named five keys.
- **`data-view` and the note only on a document that did not fail** (T4.65). A failed view is a
  problem block, which the next refresh may replace.
- **`heldTopics` is marked again when a request ends, whatever the answer** (T4.65), so a YAML view
  loaded in the meantime gets its note at once (conservative: that read may be newer than the
  change).
- **The pill's tip is the value, not a fixed sentence, and the names are cut, not wrapped**
  (T4.67, from the T4.64 ruling). The sections above say so.
- **The in-cluster pill's tip does not say "Context"** (T4.69, from the T4.68 fix-check). In a
  Pod the portal reads with the Pod's own credentials and has no context name.

## Live checks

Section 1 (task T4.59), 2026-10-11, on `main` at `360382a` merged into the branch. Browsers:
Chromium, Firefox and WebKit of the Playwright image `mcr.microsoft.com/playwright/python:v1.63.0-noble`
(the digest pinned at `cmd/opm-portal/browser_test.go:35`), driven by a scratch script and a scratch
Go test (neither committed) over the real UI and the real read API on the F1 capture. The scratch
server proxied the stream so that a test could cut it, refuse reconnects and drop `Last-Event-ID`,
and it changed the Deployment `default/podinfo-podinfo` through the fake dynamic client, so every
stream event below came from the real broker. Page: `/instances/default/podinfo?tab=graph`, with
the Deployment node selected (the address then carries `node=obj:apps/Deployment/default/podinfo-podinfo`).
The three engines gave the same result in every row unless a row says otherwise.

**1. A refresh leaves `#detail` alone: holds.**
Steps: select the Deployment node, change the object (replicas 2 to 11, ready 1), wait for the
refresh. Seen: one page fetch; `#graph` was replaced; `#detail` was the same element, with the same
content apart from htmx's transient `htmx-added` class. The panel read "Healthy / Deployment is
available / Replicas: 2" while a fresh render of the same address read "Progressing / Replicas:
2/11": the panel contradicted the graph beside it, as the proposal says. A YAML view and an Events
view (which holds one `time` element) stayed the same element with the same content after the same
change (the YAML view's HTML differed in one engine run from the transient class only). Conclusion:
section 4 builds as designed. `#detail` carries `aria-live="polite"` and no `data-follow`.

**2. A replaced `#detail` holds, with two findings: holds.**
Steps: set `data-follow` on `#detail` by hand and change the object, so the existing `refresh()`
replaces the `aside` by id from the page render. Seen: the replacement equals a fresh render of the
address; the selected node stays selected (`aria-current`); the panel holds no `details`, so there is
no open state to carry. Focus on the panel's YAML link is lost to `body`, in all three engines: the
panel links have no `id`, so the `data-focus-key` extension of section 4 is needed as designed. An
htmx request into `#detail` that started 0.5 s before the replacement ended with its
`htmx:afterRequest` but its response never appeared in the page (the panel showed the node panel,
not the YAML): the response is lost, so `refresh()` must skip `#detail` while a request into it is
open, as designed. The page render holds the panel for the address, so the `data-panel-url` fallback
is not built. The page render's panel has no `<div class="fragment">` wrapper (see the note under
"The details panel follows the page").

**3. A reconnect: holds, and shows both cases.**
Steps: cut the stream, refuse reconnects for 1.5 s while the object changes, then allow them. The
live mark read "Reconnecting" during the loss. (a) The server resumed (`Last-Event-ID` sent): the
page saw `htmx:sseError`, `htmx:sseOpen`, `sse:open` (no data, the connection event), `htmx:sseOpen`
again, `sse:open` with the stream id, then `sse:upsert` of the instance topic: the backlog reached
`changed()` and one page fetch followed. (b) The stream was new (`Last-Event-ID` dropped): the page
saw the same opens, then two `sse:snapshot` events (instance and events topics), and made no fetch:
the snapshots are dropped as the first of their topic (`primed`), so the regions stayed as they were
while the live mark read "Live". In both cases `htmx:sseOpen` fires twice per reconnect and the mark
read "Live" before any fetch. The resync of "Reconnecting" is needed as designed, and both
`htmx:sseOpen` events and the data-less `sse:open` must leave the mark alone after a loss.

**4. Every request ends with exactly one `htmx:afterRequest` carrying its `xhr`: holds.**
Steps: a boosted navigation, `htmx.ajax` into `#detail`, an answer of 500, a network failure
(`route.abort`), `htmx:abort` triggered in flight, two clicks on one element with
`hx-sync="this:replace"`, and two overlapping `htmx.ajax` calls into `#detail`. Seen: each `xhr` got
one `beforeRequest` and one `afterRequest`. An answer of 500 gave no `responseError` (the page's
`responseHandling` swaps it); a network failure gave `sendError` after `afterRequest`; `htmx:abort`
carries no `xhr` (a separate event with an empty request); the aborted request ended with
`afterRequest`, and with `sendAbort` after it in Chromium and Firefox but not in WebKit. The registry
therefore keys on `afterRequest` alone, and `sendAbort` is not an end event.

**5. `sse:heartbeat` reaches the page when the hook lists it: holds.**
Steps: wait 17.5 s on an open stream, first with the base hook list, then with `sse:heartbeat`
added to `#stream-events` (scratch edit of `layout.html`, restored). Seen: no heartbeat event
without the name; one with it, in all three engines, inside the interval.

**6. A script can set a custom property on `html` under the page policy: holds.**
Steps: from the page, `document.documentElement.style.setProperty("--masthead-h", "88px")`,
`style.cssText` and `setAttribute("style", ...)`, with a `securitypolicyviolation` listener, and a
`ResizeObserver` on `#masthead`. Seen: `setProperty` took effect and raised no violation;
`setAttribute("style", ...)` was blocked (`style-src-attr`), as the policy says; `ResizeObserver`
exists and fired. `contentRect` height was 40 px against a border box of 68 px, so the script
reads the border box. The CSS side (`scroll-padding-top` and the `#detail` offset following the
property) is not run: it needs the section 3 CSS, and task 3.8 tests it.

**What the spike did not run.** Screen-reader behaviour of the nested `aria-live="off"` and of a
replaced live region; the 2.4.11 case itself (section 3); a half-open connection. The scratch
script and server are in the session scratchpad of task T4.59 and were not committed.

Section 4 (task T4.65), 2026-10-11, screenshots of the Playwright image's Chromium (scratch script over
`serveStaleSite`, not committed) at 1920 px in light and dark and at 360 px, with the Deployment node
selected, its YAML open in the panel, the pages then rendered over the image-break capture and a topic
upsert sent, then the stream dropped. Seen: the note is a bordered block in the lost-stream tone
(pale blue in light, deep blue in dark) above the YAML text, reading "Read 0s ago. The object may have
changed since." with "Reload this view" as an underlined link of 44 px height on its own line in the
420 px panel; the YAML text did not move. The banner sits as the last row of the sticky header, light
tint over the dark header, "Showing data read 2s ago. Reconnecting." with "Reload" at the right edge,
and the live mark reads "Reconnecting". At 360 px the banner takes the full header width under the
tabs, and Reload is on its own line. Not captured: the loading bar (it needs a held request, which
`TestBrowserLoading` drives), and the pill with a long entry name (`TestBrowserPhone/<engine>/banner`
measures it). One gap: the page render's panel has no `.fragment` wrapper, as the earlier note says,
and nothing in the screenshots showed a difference. The details panel's group switch
(Expand or Collapse group) shares one focus key, `panel-expand`, because one of the two is present at a
time; the design named five keys.

Section 4 fixes (task T4.67), 2026-10-12, screenshots of the Playwright image's Chromium (scratch
script over a site with a 90 character context and a different 90 character entry, and over the F1
capture, not committed), after the pill fix, in light and dark, at 360 px and at 1280 or 1440 px.
Seen, long-entry pill: at 360 px the pill is its own row under the brand, two lines inside one
rounded box, the context on the first and the word "cluster" in small uppercase letters with the entry on the
second, each cut with "..." at the box's right edge; "reading as alice", "Kubernetes v1.36.1" and the live mark
sit on the next row, and the header is 208 px tall (305.5 px before the fix), with the banner 293 px
(390.5 px before). At 1280 px the pill is one row of two parts split by a thin rule, each cut with
"...", with the same row's other items after it. With the banner at 360 px (dark) the banner is the
last row, "Showing data read 0s ago. Reconnecting." with "Reload" on its own line, and the pill above
is unchanged. Seen, loading bar: with a node panel request held, a brass line of 2 px on the foot of
the header (partly drawn in the capture because it is animated: about 230 of 1440 px at 1440 px, and
across the tabs row at 360 px), in light and dark. No gap remains for task 4.7.
