## Context

`main` serves `/api/v1alpha1` (documents, graphs, events, one SSE stream with log topics) behind
local mode's front door (`internal/auth`: launch token, session cookie, Host allowlist,
`http.CrossOriginProtection`, `Content-Security-Policy: default-src 'none'`). The launch answers
with a meta-refresh page that moves on to the instance list JSON. There is no HTML.

Sources: 0030:D2 (the UI consumes the API in-process from the same documents), 0030:D3 (two
axes), 0030:D4 (graphs, contracts as text, accepted and active apart), 0030:D5:R7 (locked shown
up front), 0030:D8 (no Secret data, no values, no last-applied), 0030:D9 (events feed), 0030:D10
(logs). Owner: "Go API + HTMX on top"; "Keep the seam". Where this design and 0030 disagree, 0030
wins.

## Goals / Non-Goals

**Goals:**

- A person who launches the portal lands on the Platform and can reach every instance, package,
  object, event and log line the read API serves to them, and nothing else.
- Every fact on a page comes from a read API document fetched through the API's own handler
  chain, so the API stays complete (0030:D2:R1).
- Pages stay under a CSP with no `'unsafe-inline'` and no `'unsafe-eval'`; untrusted text is only
  ever rendered by `html/template` or assigned as `textContent`.
- Readable at phone width; dark and light follow the OS.

**Non-Goals:**

- Writes, forms that change anything, module presentation (icons, cards), multi-cluster, OIDC.
- A client-side application: no client templates, no client state beyond the stream's topic set,
  the log panes and the graph's pan and zoom.

## Decisions

### The UI calls the read API's handler chain in-process

```go
// internal/ui
type Config struct {
    API      http.Handler // the read API, authenticating itself
    Cluster  string       // "default"
    Logger   *slog.Logger
}
func New(cfg Config) (*Handler, error)

// fetch runs GET path through cfg.API with the caller's cookies and decodes the 200 body
// into out, or returns the problem document.
func (h *Handler) fetch(r *http.Request, path string, out any) *v1.Problem
```

`fetch` builds a `GET` request on the incoming request's context, copies its `Cookie` header and
`Host`, serves it through `cfg.API` into an in-memory response writer, and decodes JSON into the
`api/v1alpha1` type, or the problem document on any other status. The UI never sees the read
model, the authorizer or a principal: authentication, authorization, classification and
stripping are the API's. A test parses every file of `internal/ui` (tests included) and fails on
an import of `internal/readmodel` or anything below it; the UI's tests build their read API
through a new `internal/api/apitest` helper.

**Research & Decisions**

**Context**: 0030:D2 says the UI renders from the API's documents in-process.
**Options considered**:
1. Exported Go methods on `api.Server` returning DTOs - fast, but a second entry point beside the
   HTTP handlers that can drift (a check added to one path and not the other).
2. The UI calls `ServeHTTP` of the API with a synthesized `GET` - one path, every rule the API
   tests prove holds for the UI; costs a JSON encode and decode per document.
**Decision**: 2. The cost is microseconds against a local page; the guarantee is structural.

### Pages and fragments

| Path | API documents | Topics |
| --- | --- | --- |
| `/` Platform | `platform`, `platform/graph` | `platform` |
| `/instances[?namespace=]` | `instances` | `instances` or `instances:<ns>` |
| `/instances/{ns}/{name}` | `instances/{ns}/{name}`, `/graph`, `/events` | `instance:…`, `events:instance:…` |
| `/packages[?namespace=]` | `packages` | none (no list topic for packages) |
| `/packages/{ns}/{name}` | `packages/{ns}/{name}`, `/graph`, `/events` | `package:…`, `events:package:…` |
| `/{instances,packages}/{ns}/{name}/object?…` | `…/object` | none |
| `/{instances,packages}/{ns}/{name}/node?id=…` | `…/graph` | none |
| `/{…}/events` fragment | `…/events`, optionally about one object | none |
| `/platform/node?id=…`, `/platform/registrations/{name}/events` | `platform/graph`, events | none |
| `/static/…` | embedded assets, no session needed | none |

A page fetched with `HX-Request` and a target returns the same HTML; htmx selects the region it
swaps (`hx-select`), so every region has one renderer. Graph `expand` parameters pass through to
the API's `expand`.

A problem document becomes a region, never a failed page: `forbidden` renders the locked panel,
`not_readable_by_portal` and `upstream_unavailable` render a degraded panel naming the code,
`not_found` a not-found page with `404`, `unauthenticated` a sign-in page with `401` telling the
user to open a new launch link.

### Locked rendering (0030:D5:R7)

Anything the API reports with `access` `forbidden`, `notReadable` or `withheld`, a list with
access `forbidden`, a problem `forbidden`, or a graph node whose access is not `ok`, renders with
the `locked` class: a hatched surface, a lock glyph and the words "Locked: you may not read this"
(or "The portal cannot read this" for `notReadable`). It shows exactly what the document carries
(an inventory reference), never more, and offers no link to a detail it cannot open. A Secret in
an inventory shows "Secret data is never read" and no YAML action.

### Two axes, never merged (0030:D3)

Each instance and package shows two badges with different shapes so they cannot be read as one:
**Applied** (operator state; `Applied`, `Reconciling`, `Failed`, `Stalled`, `Suspended`,
`ManagedExternally` neutral, `Unknown`) as a squared stamp, **Health** (`Healthy`, `Progressing`,
`Degraded`, `Missing`, `Unknown`) as a dot and word. A partial health says "partial", a polled one
says "not live" with the evaluation time. Any enum value the UI does not know renders as
"unknown" with the raw value in a title (0030:D2:R3).

### Graph rendering

The SVG is drawn server-side from the API's `Graph` document: `viewBox="0 0 width height"`, one
`<path d="M x0 y0 C x1 y1, x2 y2, x3 y3">` per edge from its four route points, one `<a>` per node
with a `<rect>` and its label, classes `node kind-<kind> health-<state> access-<access>`, and
`aria-label` naming kind, label and state. Nodes are focusable SVG links whose `href` is the page
with `?node=<id>` (the panel renders server-side without script); `portal.js` intercepts
activation and loads the node panel into `#node-panel` with `htmx.ajax`. Group nodes link to the
page with `expand=<id>` added. Pan and zoom: `portal.js` sets a `transform` attribute on the
graph's viewport group on wheel, drag and the zoom buttons (no style attribute, so no CSP
exception).

### Live updates: one EventSource per tab

The `<body>` carries `hx-ext="sse"`, `sse-connect` (the stream URL with the first page's topics)
and `hx-boost` targeting `#main` with `hx-select="#main"`, so navigation swaps `#main` and the
body's `EventSource` lives on. A hidden element under the body listens for `sse:open`,
`sse:snapshot`, `sse:upsert`, `sse:delete`, `sse:k8sevent`, `sse:log`, `sse:logend` and
`sse:closed`; `portal.js` routes each by its `topic`:

- `open` (carries the stream id): POST the current page's `data-topics` as `add`, and the topics
  the stream was opened with that the page no longer wants as `remove`.
- After each boosted swap: POST the difference between the old and the new page's topics.
- `upsert`, `delete`, `k8sevent`, and a `snapshot` after the topic's first: trigger
  `portal:changed` on every region whose `data-follow` names the topic; regions re-fetch
  themselves (`hx-get` of the page, `hx-select` of the region, `delay:400ms`).
- `log`, `logend`, and a log topic's snapshot: append `textContent` lines to the pane of that
  topic.
- `closed`: mark the following regions as no longer live, with the code.

The `open` event name collides with `EventSource`'s own `open`; the handler ignores events
without data.

### The topic-change request

```http
POST /api/v1alpha1/clusters/default/stream/{stream}/topics
Content-Type: application/json

{"add": ["instance:default/podinfo"], "remove": ["platform"]}
```

`204` on success. `Subscribe` then `Unsubscribe` on the broker for the session that sent it;
a denied topic is closed on the stream, as at open. Problems: `400 bad_request` (a body that is
not JSON of that shape, or not `application/json`, an unparseable topic, too many topics), `401
unauthenticated`, `404 not_found` (no such stream of this session), `405` for any other method.
The front door's `http.CrossOriginProtection` refuses a cross-origin `POST` before it reaches the
API; requiring `application/json` additionally makes a cross-site form unable to send it
without a CORS preflight the portal never grants. Reads no Kubernetes object; creates nothing in
the cluster.

### The object resource

```http
GET /api/v1alpha1/clusters/default/instances/default/podinfo/object?group=apps&kind=Deployment&namespace=default&name=podinfo-podinfo
```

```json
{"apiVersion": "portal.opmodel.dev/v1alpha1", "kind": "Object",
 "ref": {"group": "apps", "version": "v1", "kind": "Deployment", "namespace": "default", "name": "podinfo-podinfo"},
 "object": {"apiVersion": "apps/v1", "kind": "Deployment", "metadata": {}, "spec": {}, "status": {}}}
```

Authorization in order, all before any lookup, exactly as the events of one object (0030:D7):
`get` on the owner; a Secret refused as `forbidden` before anything else about it; the kind
resolved through cached discovery (unknown is `forbidden`); `get` on the object; the owner's
inventory must reach it (else `forbidden`). Then `readmodel.Model.Object` reads it on demand as
the reader, after the reader's own `get` review, and strips `metadata.managedFields`, the
`kubectl.kubernetes.io/last-applied-configuration` annotation and, for ModuleInstance and
ModulePackage, `spec.values` (0030:D8:R2/R3). Unlike the held copies it keeps the bulky fields
health drops (a Deployment's template, a ConfigMap's data): the YAML view shows the object as
`kubectl get -o yaml` would, which is why 0030:D8:R4 documents that values rendered into
non-Secret objects are visible. The UI renders it as YAML with `sigs.k8s.io/yaml` (already a
module dependency).

### Pod containers

`readmodel.RuntimeChild` and `v1.RuntimeChild` gain `Containers []string` (`containers`,
omitempty), the names of a Pod's `spec.initContainers` then `spec.containers`, read from the held
Pod. The logs panel offers one pane per container, following `log:<ns>/<pod>/<container>`.

### Launch: the landing page in place

On a valid token the front door sets the cookie and serves the landing request itself: it clones
the request with path `Landing`, no query, and the new session's cookie in place of any other,
and passes it to the next handler. The page carries `<link rel="canonical" href="/">`, and
`portal.js` replaces the address (`history.replaceState`) so the token-bearing URL leaves the
history. A spent token on a request that carries the live session serves the landing the same
way. A 303 is still not used: a navigation started from the `--open` `file://` page is
cross-site, and a redirect stays part of it, so the browser withholds the new `SameSite=Strict`
cookie (the reason for the earlier hand-off page). `TestBrowserLaunch` keeps proving the launch
and the reload in Chromium, Firefox and WebKit.

### Content-Security-Policy for pages

The front door keeps `default-src 'none'` on every response. UI pages replace it with:

```text
default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self';
connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'
```

htmx is configured by a `<meta name="htmx-config">` with `includeIndicatorStyles: false` (it
would otherwise inject an inline `<style>`), `allowEval: false` and `allowScriptTags: false`.
No `hx-on`, no trigger filters (both need `eval`).

### Vendored assets

| File | Source | SHA-256 |
| --- | --- | --- |
| `htmx-2.0.11.min.js` | npm `htmx.org@2.0.11` `dist/htmx.min.js` | in `internal/ui/static/vendor/CHECKSUMS` |
| `htmx-ext-sse-2.2.4.min.js` | npm `htmx-ext-sse@2.2.4` `dist/sse.min.js` | same |
| `space-grotesk-latin-wght-normal.woff2` | npm `@fontsource-variable/space-grotesk` | same |
| `jetbrains-mono-latin-wght-normal.woff2` | npm `@fontsource-variable/jetbrains-mono` | same |

The npm tarballs' `sha512` integrity was checked against the registry before extraction. A test
recomputes each file's SHA-256 against `CHECKSUMS`. Upgrading means replacing the file and its
line in one diff.

## Risks / Trade-offs

- [Re-fetching a region per change costs a page render] → `delay:400ms` coalesces bursts; the API
  answers from held state (0030:D3:R9).
- [The stream's per-session cap is two] → one `EventSource` per tab; a third tab's stream makes
  room by discarding the oldest detached stream, as the broker already does.
- [The object resource reads on demand] → one `get` per click, as the reader, after both reviews;
  never on page load.
- [Golden HTML churns with markup changes] → goldens cover `#main` only, generated with
  `-update` and reviewed like the API goldens.
