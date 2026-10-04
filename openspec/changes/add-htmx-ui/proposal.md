## Why

`opm-portal serve` runs the whole read path, but a person who launches it lands on a JSON
document. The owner's first ask is a page: "showing what is installed in the cluster and status +
events logs, and ability to view the resources, with a generated DAG showing the relationships",
and the owner wants to use OPM to define and build platforms, so the Platform (its catalogs,
claimants and contracts) is the landing page, not an afterthought. The read API was built to have
the web UI as its first consumer (0030:D2); this change is that consumer.

## What Changes

- New package `internal/ui`: server-rendered pages from `html/template`, with htmx and its SSE
  extension vendored at pinned versions (checksums recorded) and served from the binary through
  `go:embed`. No CDN, no bundler, no build step. Plain CSS with colour tokens for light and dark
  (`prefers-color-scheme`), two vendored OFL fonts, and a layout that works at phone width
  without horizontal scroll.
- Pages, every fact read through the read API's handler chain in-process, under the caller's
  session (0030:D2):
  - **Platform** (the landing page, `/`): catalog subscriptions and the resolved registry with
    versions and sources, registrations with separate accepted and active pills, verdicts,
    removal-blocked shown as its own state, refusal reasons, the catalogs each registration
    claims and provides, the Platform's conditions (an unfulfilled contract shown as
    information), and the platform graph.
  - **Instances** (`/instances`, filter by namespace): Applied and Health as two separate badges,
    ManagedExternally neutral, rows the caller may not read shown locked.
  - **Instance** (`/instances/{ns}/{name}`): the graph, components with their objects and health,
    conditions with the reason's meaning and next step, the status history timeline, the events
    feed with folded repeats labelled as recent activity that expires after about an hour, the
    render contracts as plain text, a logs panel per Pod container on the log topics, and a YAML
    view of a selected object with values and the last-applied annotation stripped.
  - **Packages** and **Package** (`/packages`, `/packages/{ns}/{name}`): the same shape for
    ModulePackages.
  - Every object, row, node or section the identity may not read renders **locked**, with no
    name of anything hidden beyond what the API document carries (0030:D5:R7).
- A server-rendered SVG graph from the API's laid-out graph documents: CSS classes for health
  and access, keyboard-focusable nodes, a node detail panel fetched with `hx-get`, collapse groups
  expandable through the API's `expand` parameter, and a hand-written pan and zoom.
- Live updates: one `EventSource` per tab on the existing stream, which survives htmx-boosted
  navigation; each page declares its topics and the tab adds and removes them through a new
  CSRF-protected request on the read API. Changes re-fetch the affected page regions.
- Read API, additive:
  - `POST /clusters/{cluster}/stream/{stream}/topics` adds and removes topics on an open stream
    (JSON body, `204`), behind the front door's cross-origin protection.
  - `GET /clusters/{cluster}/{instances|packages}/{namespace}/{name}/object?group&kind&namespace&name`
    serves one object an inventory reaches as an `Object` document, authorized like the events
    of that object, stripped of managed fields, values and the last-applied annotation; a Secret
    is never read.
  - A Pod's runtime child carries its `containers`, so a client can name a log topic.
- Local mode: `GET /launch` answers with the UI's landing page itself, rendered in place under
  the new session, instead of the meta-refresh hand-off page (issue 21, item 3). A redirect is
  still not used: it would lose the `SameSite=Strict` cookie on a launch started from the
  `--open` page.
- The UI's own Content-Security-Policy widens the front door's `default-src 'none'` only to
  `'self'` for scripts, styles, fonts, images and connections, with no `'unsafe-inline'` and no
  `'unsafe-eval'`.
- A test forbids `internal/ui` importing `internal/readmodel`.

## Capabilities

### New Capabilities

- `web-ui`: the HTML pages, what each shows, the locked rendering, the graph, live updates, the
  vendored assets and the page security policy.

### Modified Capabilities

- `read-api`: the topic-change request, the object resource and Pod containers.
- `read-model`: one stripped object read on demand for the object resource, and Pod containers on
  runtime children.
- `local-mode`: the launch answers with the landing page, and UI pages carry their own
  Content-Security-Policy.

## Impact

- Packages: new `internal/ui` (imports `api/v1alpha1` and the standard library only; reads go
  through an `http.Handler`, the read API); `internal/api` gains two routes and the `Object`
  document; `internal/readmodel` gains `Object` and Pod containers; `internal/auth` serves the
  landing in place; `cmd/opm-portal` mounts the UI beside `/api/`.
- API: additive only (two resources, one wire type, one optional field); `task api:breaking`
  stays green.
- Principle V: no new kind is read except the inventory objects the object resource gets on
  demand, as the reader identity after the caller's own access review, for objects an inventory
  reaches; a Secret is refused before any review. The only new write is the topic-change request
  on the portal's own stream; it changes nothing in the cluster.
- Principle VII: two vendored front-end assets the constitution already names (htmx, its SSE
  extension), one hand-written script for the stream, the graph and the log panel, and two
  vendored font files. No Go dependency.
- SemVer: MINOR after 1.0 (new pages and additive API). On the 0.x line it ships as `feat(ui)`
  and cuts a minor release.
- Enhancement link: completes 0030:D5 (R7, locked nodes shown up front), 0030:D4 (R3 contracts as
  text, R4 and R7 shown on pages) and 0030:D3 (both axes shown apart, the scripted image break
  measured on the live UI). See `enhancement.yaml`.
