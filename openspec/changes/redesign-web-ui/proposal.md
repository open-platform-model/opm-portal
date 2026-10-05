## Why

The owner reviewed a redesign of the web UI on a design canvas (boards kept in
`docs/design/evidence/03-ui-canvas/`) and answered four questions about it on 2026-10-05. The
current pages split what is installed over two lists, show the Platform as a graph rather than as
rows a platform team can filter, put every instance section on one long page, follow only the
operating system's theme, forget every filter, and never say which cluster, identity or
Kubernetes version the portal reads. They also name kinds ("TransformerRegistration",
"operator") where the canvas uses the words a platform team uses ("Providers", "controller",
"Installed").

The canvas also draws things the cluster does not record (catalog definitions and descriptions,
transformers, a Platform health, run-level history results), and the controller does not yet do
everything the canvas assumes (a ModulePackage cannot be an accepted provider; packages record no
contracts). This change builds the canvas down to recorded data, states each gap, and records the
owner's answers as portal decisions.

## What Changes

- **Decisions** (already in `docs/DESIGN.md` on this branch): portal:D14 (the browser keeps a
  theme choice and per-view filters; filters in the URL win; local mode listens on a fixed default
  port so they survive a restart), portal:D15 (a provider is any instance or package whose
  inventory holds a TransformerRegistration; the controller's verdict is shown as it is),
  portal:D16 ("Uses" is the contracts an instance's render used, never an edge; "Used by" is the
  intersection the controller uses to count dependents), portal:D17 (scope: Platform and
  Installed, the user's words, nothing unrecorded), portal:D18 (every page names the cluster, the
  reader and the server version). portal:OQ22 to portal:OQ25 opened, portal:OQ18 narrowed;
  portal:D1 amended to list the non-resource reads (discovery and `/version`) and portal:D2 the
  new document. Principle VII amended in `openspec/config.yaml` and `CONSTITUTION.md` for
  portal:D14.
- **Read API, additive** (section 1): a `Cluster` document at `/api/v1alpha1/clusters/{cluster}`
  (context, cluster entry, reading-as username, Kubernetes version); `Package` gains `interval`
  and the source artifact's `revision` and `digest`; `Registration` gains its `conditions` and
  `heldBy` (the instances and packages whose inventory holds it); instance and package summaries
  gain `providerOf` (the registrations they hold, with the controller's standing); instance
  summaries carry `renderContracts`; a history entry gains `outcome` (`Succeeded`, `Failed`,
  `Unknown`), derived from the controller's two entry shapes before any in-cluster omission. The
  read model reads `/version` once at start, as it reads discovery. The change stream routes
  registration changes to their holders' topics and holder changes to `platform`.
- **Local mode**: `--addr` defaults to `127.0.0.1:7878` instead of a random port, and a taken port
  fails with a message naming `--addr` (portal:D14:R6). The `deploy/` manifest keeps
  `--addr 127.0.0.1:8090`.
- **Web UI** (sections 2 to 5): a sticky header that slims on scroll, with Platform and Installed,
  the cluster and reading-as marks, the live mark and a Light/Dark/System theme switch; one
  Installed list of instances and packages with filters (search, kind, provider, uses, namespace,
  health, applied, owner, module) shown as removable chips, kept in the URL and, for list views,
  remembered per browser; a Platform page with identity and status cards, an Installed card whose counts link to
  filtered views, Providers and Catalogs tabs, and a merged recent-events feed with a resource
  filter; instance and package pages with Applied and Health summary cards (reason counts,
  attempt dots from `status.history`), tabs Graph, Resources, Events, Logs and YAML, and graph
  interactions (hover cards, spotlight from Resources, full screen, zoom slider with Fit and
  Ctrl-wheel, group accordion with zoom-to-group and a way back); a Provider pill, block and tab on
  any instance or package that holds a registration; a Catalog page.
- `/instances` and `/packages` redirect to `/installed` with a kind filter.
- Docs pages under `docs/site/` that describe the pages, and `ROADMAP.md`.

Not in this change: catalog definitions, descriptions, documentation links and transformers
(owner: "Defer those tabs"; portal:OQ25), and every other canvas feature design.md lists under
Non-Goals; a Platform health (portal:OQ22); controller acceptance of package providers
(portal:OQ23, opm-operator#254) and package render contracts (portal:OQ24), which are controller
changes; renaming `operatorVersion` and the owner value `operator` on the wire, which is the
controller rename's portal change; in-cluster mode.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-ui`: new shell, theme and remembered filters; Installed replaces Instances and Packages;
  Platform, instance, package, provider and catalog pages as above; graph interactions; the
  in-cluster omission covers the new fields.
- `read-api`: the `Cluster` resource and the additive fields above; the in-cluster omission
  names registration condition messages, and keeps history outcomes and provider standings.
- `read-model`: the holder join for registrations, kind-agnostic; package interval and source
  revision; the server version read once at start, like discovery.
- `local-mode`: the fixed default address; local mode hands the read API its kubeconfig context
  and cluster entry names, or the in-cluster source.

## Impact

- Packages: `internal/readmodel` (holder join, package fields, server version), `api/v1alpha1`
  and `internal/api` (new resource and fields, change-stream routing, OpenAPI, goldens, in-cluster
  omission), `cmd/opm-portal` (default address, context names into the API config), `internal/ui`
  (templates, view, graph, CSS, `portal.js`, a new `/static/prefs.js`, tests). `internal/authz`,
  `internal/graph` and `internal/health` do not change.
- API: additive only; `task api:breaking` stays green. One new path, so
  `docs/site/reference/portal/read-api.md` lists it (`TestReadAPIReferenceListsEveryPath`).
- Principle V: one new read, the API server's `/version`, a non-resource `get` made once at start
  without an access review, as discovery is today; it is open to every authenticated identity
  through the built-in `system:public-info-viewer` role and reveals nothing about any object
  (portal:D18:R3). No new create, no write, no new kind, no Secret, no values. Every other new
  field comes from objects the portal already reads (ModulePackages, TransformerRegistrations,
  inventories). The `Cluster` document carries the caller's own username and two kubeconfig names,
  never a server URL or credential (portal:D18:R1). The `deploy/` role needs no new rule. Browser
  storage holds filter queries and the theme; any process that later serves pages on
  `127.0.0.1:7878` can read them, which `portal-security.md` states.
- Principle VII: amended by the owner (portal:D14) to allow browser-kept display preferences. No
  new dependency; no JavaScript build step. The second script is a few lines served from
  `/static`, needed because a deferred script cannot set the theme before first paint.
- SemVer: MINOR after 1.0 (additive API, new pages, a new default address; `--addr` keeps every
  old setting possible). On the 0.x line it ships as one PR titled
  `feat(ui): redesign the web UI around Platform and Installed`, which cuts a minor release.
- Ordering: the controller rename's portal change (PORTAL-1) renames `operatorVersion` and the
  owner enum and recaptures the goldens; whichever lands second rebases its goldens. Release PR 18
  (0.1.0) is held for that rename, not for this change.
- Decisions: implements portal:D14 to portal:D18; amends portal:D1 and portal:D2; portal:D18
  names `/version` beside discovery as the reads portal:D5:R7's review does not precede; keeps
  portal:D3, portal:D4:R3, portal:D8:R5 and portal:D9 as they are.
