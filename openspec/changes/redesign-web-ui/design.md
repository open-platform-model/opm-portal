## Context

The web UI (changes `add-htmx-ui` and `refine-htmx-ui`) serves a Platform page built around the
platform graph, separate Instances and Packages lists, and one long page per instance or package.
The owner reviewed a redesign on a design canvas (boards in
`docs/design/evidence/03-ui-canvas/`, mock data) and decided four questions on 2026-10-05. Those
answers and the supervisor's rulings are recorded as portal:D14 to portal:D18 in
`docs/DESIGN.md`; this document says how the change builds them.

Every claim about controller output below names opm-operator source at 277ca18 (the module the
portal pins is older, v1.0.0-beta.6, and the field shapes cited are unchanged between them) or the
F1 capture under `testdata/clusters/f1/`.

## Goals / Non-Goals

**Goals:**

- The canvas's views, cut down to what the cluster records: a header naming cluster, reader and
  version; Platform; one Installed list; instance and package pages with summary cards and tabs;
  a Provider tab; a Catalog page.
- Filters that survive a visit and hand-off links that mean the same thing to everyone
  (portal:D14).
- Providers found by inventory, instance or package (portal:D15), with the controller's verdict
  unedited.
- Every new read stated with its review; no new kind, no write, no Secret, no values.

**Non-Goals:**

- Catalog definitions, descriptions, documentation links, transformers (portal:OQ25), a Platform
  health (portal:OQ22), the transformer behind each object (portal:OQ5).
- Controller changes: package providers (portal:OQ23), package render contracts (portal:OQ24).
- Renaming wire names (`operatorVersion`, owner `operator`): the controller rename's portal change.
- A `packages` stream topic: the Installed list follows `instances` live, as the Packages list
  followed nothing before; packages refresh on navigation.
- In-cluster mode.

## Decisions

### Read API additions (section 1)

All additive under `v1alpha1`. Shapes, with the Go wire types they extend in
`api/v1alpha1/types.go`:

```go
// Cluster is the connection the portal reads with: one per path cluster.
type Cluster struct {
	TypeMeta
	Name string `json:"name"` // the path cluster, "default"
	Mode string `json:"mode"` // local or in-cluster
	// Source is kubeconfig or in-cluster.
	Source string `json:"source"`
	// Context and ClusterEntry are the kubeconfig context and the name of
	// its cluster entry; absent when Source is in-cluster.
	Context      string    `json:"context,omitempty"`
	ClusterEntry string    `json:"clusterEntry,omitempty"`
	ReadingAs    ReadingAs `json:"readingAs"`
	// KubernetesVersion is the API server's gitVersion, absent unless
	// KubernetesVersionAccess is ok.
	KubernetesVersion       string `json:"kubernetesVersion,omitempty"`
	KubernetesVersionAccess string `json:"kubernetesVersionAccess"`
}

// ReadingAs is the caller's own identity, as the portal reads for it.
type ReadingAs struct {
	Username string `json:"username"`
}

// SourceArtifact is what a ModulePackage's last reconcile fetched.
type SourceArtifact struct {
	Revision string `json:"revision,omitempty"`
	Digest   string `json:"digest,omitempty"`
}

// ProviderClaim is one TransformerRegistration an owner's inventory holds,
// with the controller's standing when the caller may read it.
type ProviderClaim struct {
	Registration string `json:"registration"`
	Access       string `json:"access"` // ok, forbidden or notReadable
	Accepted     bool   `json:"accepted,omitempty"`
	Active       bool   `json:"active,omitempty"`
	Verdict      string `json:"verdict,omitempty"`
	Reason       string `json:"reason,omitempty"`
	// ProviderRefMatches: the registration's spec.providerRef names this
	// owner's namespace and name.
	ProviderRefMatches bool `json:"providerRefMatches"`
}

// Added fields:
// InstanceSummary: ProviderOf []ProviderClaim `json:"providerOf,omitempty"`
//                  RenderContracts []string `json:"renderContracts"` (moved up from Instance;
//                  Instance embeds InstanceSummary, so the Instance document is unchanged)
// PackageSummary:  ProviderOf []ProviderClaim `json:"providerOf,omitempty"`
//                  Interval string `json:"interval,omitempty"` (spec.interval as written)
//                  SourceArtifact *SourceArtifact `json:"sourceArtifact,omitempty"`
// Registration:    Conditions []Condition `json:"conditions,omitempty"`
//                  HeldBy []ObjectRef `json:"heldBy,omitempty"`
//                  HeldByPartial bool `json:"heldByPartial,omitempty"`
// HistoryEntry:    Outcome string `json:"outcome"` // Succeeded, Failed or Unknown
```

```json
{"apiVersion":"portal.opmodel.dev/v1alpha1","kind":"Cluster","name":"default","mode":"local",
 "source":"kubeconfig","context":"kind-opm-portal-e2e","clusterEntry":"kind-opm-portal-e2e",
 "readingAs":{"username":"kubernetes-admin"},
 "kubernetesVersion":"v1.36.1","kubernetesVersionAccess":"ok"}
```

`GET /api/v1alpha1/clusters/{cluster}` serves the `Cluster` document. Errors: `401
unauthenticated` without a principal; `404 not_found` for a cluster other than `default`, before
any review (as every route); `405` for another method. A denied or failed `/version` review is not
an error: `kubernetesVersionAccess` is `forbidden` or `notReadable` and the version is absent.

`SourceArtifact` takes `status.source.artifactRevision` and `artifactDigest`
(opm-operator `api/v1alpha1/common_types.go:83-99`, `modulepackage_types.go:88-90`).
`status.source.artifactURL` is not served: it is a fetch URL inside the cluster, of no use to a
reader. `Interval` is `spec.interval` as written (`modulepackage_types.go:40-48`); when absent the
page says "not set", never the controller's default. The F1 package has `interval: 1m` and no
`status.source`, because the fixture cluster runs no Flux; the revision path is tested by a unit
fixture.

`HistoryEntry.outcome` reads the controller's two entry shapes: a success entry carries `phase:
complete` and no message, a failure entry carries a message and no phase
(`internal/status/history.go:16-57`; callers `internal/reconcile/moduleinstance.go:344-347`,
`modulepackage.go:257-259`). `Succeeded` when the phase is `complete`, `Failed` when the phase is
empty and the message is not, `Unknown` otherwise. It is computed before the in-cluster omission
drops history messages, so in-cluster pages still tell a failed attempt from a successful one.
It is a reading of recorded shapes, not an inference from events (portal:D9:R1). The F1 capture
holds both shapes live: `pkg/podinfo`'s three entries carry no phase and the source error as their
message, and the instances' entries carry `phase: complete`
(`testdata/clusters/f1/modulepackages.yaml`, `moduleinstances.yaml`).

### The holder join, kind-agnostic (section 1)

The read model holds every ModuleInstance and ModulePackage with its inventory. A registration's
holders are the owners whose `status.inventory.entries` hold `group: opmodel.dev, kind:
TransformerRegistration, name: <registration>` (entry shape `{group, kind, namespace, name, v,
component}`, `api/v1alpha1/common_types.go:131-147`). The join is a scan of held state, rebuilt with the views; it reads nothing new.

```go
// Holders returns the owners whose inventory holds registration name, among
// those the caller may read, and whether that set may be incomplete.
func (m *Model) Holders(ctx context.Context, who authz.Identity, name string) ([]ObjectRef, bool)

// ProviderOf returns the registrations owner's inventory holds, each with
// the caller's access to it and the controller's standing.
func (m *Model) ProviderOf(ctx context.Context, who authz.Identity, owner ObjectRef, inv []InventoryEntry) []ProviderClaim
```

Authorization: `ProviderOf` asks `list transformerregistrations` (cluster scope) for the caller
once per request, through the cached decisions the platform view already uses; denied, each claim
carries `access: forbidden` and its name only (the name is in the owner's inventory, which the
caller is reading). `Holders` includes an owner only when the caller may `list` its kind in its
namespace; when any namespace was refused, `heldByPartial` is true.

`Registration.provider` keeps meaning what it meant: `spec.providerRef` as the controller reads it,
kind ModuleInstance (`api/v1alpha1/common_types.go:48`). `heldBy` is the new, kind-agnostic answer.
`ProviderClaim.providerRefMatches` says whether the two agree, so a page can show a disagreement
(portal:D4:R2). For a package holding a claim, `providerRefMatches` is true when the package's
namespace and name equal the reference (the catalog's transformer stamps the reference from the
rendering context, `catalog_opm/src/transformers/transformer_registration_transformer.cue:80`),
and the registration is refused with `ProviderMismatch` by the controller
(`internal/controller/transformerregistration_controller.go:175-180`, `:369-401`). The page shows
both facts: holder and reference agree, and the controller refused (portal:D15:R3).

`internal/graph` does not change: the platform graph's provider lookup stays instance-only and the
Platform page no longer shows that graph (portal:D17).

### The server version, through the seam (section 1)

`/version` is a non-resource URL. Today `internal/authz` reviews resource attributes only, and its
refusal rules refuse an empty resource without asking. The seam gains one non-resource read:

```go
type Attributes struct {
	Verb        string
	Resource    schema.GroupVersionResource
	Subresource string
	Namespace   string
	Name        string
	// NonResourcePath is set only for a non-resource read; Resource,
	// Subresource, Namespace and Name are then empty.
	NonResourcePath string
}
```

The only non-resource read allowed is `get` on `/version`; any other path or verb is refused
without asking, as a write verb is. A local review sends a SelfSubjectAccessReview with
`spec.nonResourceAttributes {path: /version, verb: get}`; the in-cluster review sends the
SubjectAccessReview equivalent. `Grant.Covers` compares the path. The grant seal scan
(`internal/authz/seal_test.go`) is unchanged in what it forbids.

The read model reads `discovery.ServerVersion()` once at `Start`, as its reader, after an allowed
review, and holds `gitVersion`; a denied or failed review holds nothing, and the `Cluster` document
then says `notReadable`. Per request, the document asks the same review for the caller and serves
the held version only on allow (portal:D18:R3). In local mode the reader is the caller, so the two
answers agree.

Verbs and resources read by this change: `get` on non-resource `/version` (new, once per start).
Everything else is already read: `get`, `list`, `watch` on ModuleInstances, ModulePackages,
TransformerRegistrations and the Platform, and `list` on `events.k8s.io` events, all with the
grants they take today. The only create stays `selfsubjectaccessreviews` (portal:D5:R6).

### Context names into the read API (section 1)

`cmd/opm-portal` already resolves the context it loaded (`configSource`, `serve.go:180-194`). It
passes the context name, the context's cluster entry name, or `source: in-cluster`, to
`api.Config.Connection`. The read API never sees the kubeconfig itself. The username is the
principal's identity, already resolved per request.

### The shell and theme (section 2)

The masthead becomes sticky: brand, nav (Platform, Installed), the cluster chip (`context` or "in
cluster"), "reading as <username>", the live mark, and a theme menu (Light, Dark, System). An
`IntersectionObserver` on a sentinel 48 px down the page toggles a `compact` class that slims it;
`prefers-reduced-motion` turns the transition off.

Theme: `portal.css` defines every colour as a custom property on `:root`, redefines them under
`@media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) { ... } }` and again under
`:root[data-theme="dark"]`. A new `/static/prefs.js`, loaded in `<head>` without `defer` before the
stylesheet, reads `localStorage["opm-portal.theme"]` in a `try` block and sets
`document.documentElement.dataset.theme` to `light` or `dark`, or nothing for System. It is the
only script that runs before first paint, so a stored choice never flashes the other theme. The
page policy does not change: `script-src 'self'` admits it, and it sets an attribute, not a style.

### Filters: URL first, browser second (section 2)

Every list view has a fixed set of filter parameters; the URL query is their source of truth
(portal:D14).

| View | Parameters |
| --- | --- |
| `/installed` | `q`, `kind` (`instance`, `package`), `provider` (`yes`, `no`), `uses` (a contract), `namespace`, `health`, `applied`, `owner` (`controller`, `cli`), `module` |
| `/` Providers tab | `tab=providers`, `pq`, `pstatus` (`active`, `accepted`, `refused`, `blocked`, `pending`), `provides` |
| `/` Catalogs tab | `tab=catalogs`, `cq`, `csource` (`subscription`, `registration`, `claim`), `claimed` (`yes`, `no`) |
| `/` events | `eresource` (`platform` or `registration:<name>`) |
| instance and package Events tab | `tab=events`, `resource`, `type`, `reason` |

`internal/ui` filters server-side from the query, so every filtered view is a plain link that
works without script, and each active filter renders as a chip whose link drops that parameter.
The filter form is a `GET` form (`form-action 'self'`).

Remembering (`prefs.js` and `portal.js`):

1. On a full page load, `prefs.js` checks whether the path is a list view and its query carries
   none of that view's parameters; if so and `localStorage["opm-portal.filters:<path>"]` holds a
   query, it calls `location.replace(path + "?" + stored)` before anything paints.
2. On a boosted navigation (`hx-boost` swaps `#main`, so the head script does not run),
   `portal.js` handles `htmx:configRequest`: the same check, and it rewrites the request path.
   Whether htmx then pushes the rewritten URL is unverified (htmx 2.0.11); the spike settles it,
   with the server's `HX-Push-Url` response header as the fallback.
3. After every render of a list view, `portal.js` writes the view's current filter query (only its
   own parameters) to its key, or removes the key when there is none. "Clear filters" is a link to
   the bare path that removes the key first.

Every storage access is in `try`/`catch`; on failure the page behaves as if nothing is stored.
Values are written as the query string the page already shows; nothing from a document is stored.

### Installed (section 2)

`/installed` merges the `InstanceList` and the `PackageList` the caller may read into one table:
name and namespace, a kind chip (instance solid, package dashed), a provider badge from
`providerOf` (accepted and active, accepted and not active, refused, removal blocked, locked), the
module path and version or the package's source and path, the applied badge, the health badge, the
object count, and the owner: `controller` or `cli` for an instance (wire `operator` shows as
"controller"), and `controller` for every package, which has no owner field and is reconciled only
by the controller. A list the caller may not read renders locked for its kind only. The `uses`
filter matches `renderContracts`; with it set, packages are left out and a line says packages do
not record what they use (portal:D16:R2). `/instances` and `/packages` answer `308` to `/installed`
with `kind=instance` or `kind=package` and their `namespace` parameter (portal:D17:R1).

A contract is shown by its last two path segments with the version (`traits/backup@v1alpha1`) and
the full name in its `title`; the portal adds no kind, description or link to it (portal:OQ25).

### Platform page (section 3)

- **Identity card**: cluster (context and cluster entry from the `Cluster` document), type
  (`spec.type`), controller version (`operatorVersion`), Kubernetes version, context.
- **Status card**: the `Ready` state with its reason and the portal's meaning. The Platform's
  `Ready` reasons are `Generated` (true), and `BuildFailed`, `GenerateFailed`,
  `ContractCollisions`, `OverSubscribedContracts`, `ComparablePredicates` (false)
  (`internal/status/conditions.go:81-127`; `api/v1alpha1/platform_types.go:182-206`);
  `ContractsFulfilled` is shown beside it as information: `UnfulfilledContracts`,
  `ContractsFulfilled`, `NoContractsDefined` (`conditions.go:134-148`;
  `platform_types.go:208-215`) (portal:D3:R8). A
  subscription the controller cannot resolve shows on the Platform as `Ready=False/BuildFailed`;
  `CatalogUnresolved` is a TransformerRegistration reason (`conditions.go:171`) and is linked only
  from a registration. Each reason links to its row in the conditions list, which shows meaning,
  next step and message. No Platform health block (portal:D17:R4).
- **Installed card**: counts per health state and per applied state over both lists, each a link
  to `/installed` with that filter; one line "N installed: I instances, P packages; K of them hold
  a registration". A list the caller cannot read makes its counts locked, not zero.
- **Providers | Catalogs tabs** with the filters above. A provider row: registration name (link to
  the holder's Provider tab, or plain text with no readable holder), catalog and version (link to
  the Catalog page), provided contracts, holder ("Installed as": kind chip and link, from
  `heldBy`), acceptance and activation as two pills (portal:D4:R4), and the refusal or blocked
  reason. A catalog row: path (link), version, source, claimants (links), enabled. The note that a
  provider-fulfilled contract without a provider is information stays.
- **Recent events**: the Platform's feed and each readable registration's feed, fetched through
  the read API and merged newest first, with the `eresource` filter grouped as Platform and
  Providers. The page follows `platform`, `instances`, `events:platform` and
  `events:registration:<name>` for each readable registration. The feed keeps its expiry label
  (portal:D9:R2).

The platform graph leaves the page; `platform/graph` stays in the read API (portal:D17).

### Instance and package pages (section 4)

- **Identity card**: kind, name, namespace, module path and version (instance) or source, path,
  interval and revision (package), owner, and the applier ServiceAccount (`serviceAccountName`)
  with a title explaining it.
- **Applied card**: the applied badge, reason and meaning; the counts of Warning events in the
  current feed by reason, each linking to the Events tab filtered by that reason, labelled as the
  last hour of events (portal:D9:R1/R2: counts of the feed, never a state); and the attempt dots.
- **Attempt dots**: one dot per `status.history` entry, oldest left, coloured by `outcome`, with
  action, outcome, finish time and, in local mode, the message in its title; a running dot is added
  only while the `Reconciling` condition is `True`. Under the dots: "at most ten attempts are kept;
  reconciles that change nothing are not recorded" (`internal/status/history.go:9-13`, `:59-66`).
  "Show history" opens the history table that exists today.
- **Health card**: the health badge, partial and live marks as today, and the counts of objects
  and runtime children by health reason (`ImagePullBackOff`, `CrashLoopBackOff`, ...), computed
  in `internal/ui` from the `Instance` or `Package` document's per-object and per-child health.
  Each links to the Resources tab filtered to those objects, because health reasons are container
  waiting reasons, not event reasons.
- **Provider card**, only when `providerOf` is not empty: per claim, Active, Accepted and not
  active, Refused, Removal blocked or Pending, with the reason linking to the Provider tab.
- **Tabs** as `?tab=graph|resources|events|logs|yaml|provider`, server-rendered, boosted links.
  The details panel shows only on Graph and Resources. Resources lists kind, name, origin
  (inventory, made by the cluster below an inventory object), health, reason and age where the
  document has one; a row's link opens Graph with `focus=<node id>`. Events, Logs and YAML are
  today's regions moved into tabs, with Events gaining the `resource`, `type` and `reason`
  filters.

Graph interactions, all in `portal.js` over the server-rendered SVG:

- **Hover cards**: the server renders one card per node in a hidden list beside the SVG (kind,
  name, health and applied marks, health reason, origin, access), and the script shows the card on
  pointer hover or keyboard focus, positioned through CSSOM properties (`el.style.left`), never a
  `style` attribute, which the policy forbids.
- **Focus spotlight**: `focus=<id>` dims every node and edge not adjacent to it by a class;
  "Clear selection" drops the parameter.
- **Full screen**: the Fullscreen API on the graph region, with a class-based fallback.
- **Zoom**: an `<input type="range">` bound to the existing zoom scale, beside Fit; Ctrl-wheel and
  pointer pan exist.
- **Groups**: a group node's accordion expands it through the graph resource's `expand` parameter
  (exists), then fits the view to the group's members; a "Whole graph" button removes the
  expansion and fits again.

### Provider tab and Catalog page (section 5)

The Provider tab, on any instance or package with `providerOf`: per held registration, its name,
claimed catalog (link) and version, whether the catalog is in the Platform's resolved registry and
contributed by this registration (`contributedBy`), the registration's conditions table (type,
status, reason, meaning, message in local mode, since `lastTransitionTime`), and per provided
contract, "Used by": the first three readable instances whose `renderContracts` contain it, "and N
more", and a link "All N in Installed" to `/installed?uses=<contract>` (portal:D16:R3). The graph
already includes the registration as an inventory object.

The Catalog page, `/catalog?path=<catalog path>`, for any catalog the Platform subscribes to,
holds in its registry, or a readable registration claims: path, version, origin (Subscribed, From a
provider, or Claimed only), source (the subscription, or the contributing registration and its
holder), enablement; a Resolved block (in `status.registry`, or not, with the Platform's `Ready`
reason, or the claiming registration's refusal reason such as `CatalogUnresolved`); the Platform's
`ContractsFulfilled` labelled as platform-wide; tabs Claims (registrations claiming it, holders,
standing) and Events (Platform and claimants). The page says that definitions and transformers
are not shown because the cluster does not record them (portal:D17:R3). It reads no new API
resource: everything comes from the `Platform` document and the registrations' `heldBy`.

## Research & Decisions

### Where the cluster, reader and version live

**Context**: The canvas shows cluster, controller version, Kubernetes version and context on the
Platform identity card, and "reading as" in the header. The brief placed version and context in
the Platform document.
**Explored**: `cmd/opm-portal/serve.go:149-194` (context resolution), `internal/readmodel/platform.go`
(Platform view is gated on `get platforms cluster`), `internal/auth/local.go` (identity per
session).
**Options considered**:
1. Platform document fields - one fetch for the Platform page; but a caller denied the Platform
   loses the header facts, and none come from the Platform object.
2. A `Cluster` document at the cluster path - every page's header reads it; one more in-process
   fetch per page.
**Decision**: Option 2 (portal:D18).
**Rationale**: The facts describe the connection, not the Platform, and must show on every page,
including to a caller who may not read the Platform.

### Kubernetes version source

**Context**: The canvas shows a Kubernetes version; nothing the portal reads carries it.
**Explored**: discovery `ServerVersion()` (`GET /version`); Node `status.nodeInfo.kubeletVersion`.
**Options considered**:
1. `/version` - the API server's own answer; a non-resource read the seam cannot review today.
2. Nodes - a new kind with a `list` grant on every node; kubelets may differ from the server.
**Decision**: Option 1, with the seam extended to one non-resource path.
**Rationale**: No new kind; the review keeps portal:D5:R7. Unverified: that the built-in
`system:public-info-viewer` binding lets every authenticated identity, the `deploy/`
ServiceAccount included, `get /version`. The first spike in tasks.md settles it.

### Merged Platform events

**Context**: The canvas shows one recent-events feed for the Platform and its registrations.
**Options considered**:
1. A new merged events resource - one fetch; a new route, new authorization combination.
2. Merge in `internal/ui` from the existing `platform/events` and
   `platform/registrations/{name}/events` - several in-process fetches; no API change.
**Decision**: Option 2.
**Rationale**: Each feed is already served and authorized per object; a client can do the same
merge (portal:D2:R1 holds). Registrations number a handful per cluster.

### Health reason counts

**Context**: The canvas's Health card counts objects by reason.
**Decision**: Derived in `internal/ui` from per-object and per-child `health.reason` in the
document already served; no `health-evaluation` change.
**Rationale**: The counts are a tally of served fields, not a new judgement.

### Filters in the URL and the browser

**Context**: The owner allowed per-browser filters (portal:D14); hand-offs must stay plain links.
**Options considered**:
1. Client-side filtering in script over the full list - needs component state and breaks
   without script.
2. Server-side filtering from the query, script only to restore and remember - every view is a
   link.
**Decision**: Option 2, URL wins, stored only when the URL has none.
**Rationale**: Keeps pages server-rendered (Principle II) and hand-offs exact.

### History outcome

**Context**: The canvas draws attempt dots as Applied, Failed or Running. `status.history` has no
result field, no per-entry reason, no running entries, and no-ops are not recorded.
**Decision**: `outcome` from the two entry shapes, computed before the in-cluster omission; a
running dot only from `Reconciling=True`.
**Rationale**: In-cluster mode drops history messages (portal:D8:R5), so a page cannot tell the
shapes apart after the omission.

### Package providers

**Context**: Owner: a package can be a provider (portal:D15). The controller refuses its claims.
**Decision**: The UI is kind-agnostic through `heldBy` and `providerOf`; the refusal is shown as
the controller wrote it; the controller change is portal:OQ23, not a task here.
**Rationale**: Principle IV; a portal that hid or relabelled the refusal would disagree with
`kubectl`.

## Risks / Trade-offs

- **Package-provider path has no live evidence.** F1 has one ModulePackage, `SourceNotReady`
  because the fixture cluster runs no Flux, and it holds no claim. The path is covered by unit
  tests over read-model fixtures derived from the F1 instance capture, labelled as constructed,
  with the refusal reason and message taken from controller source. A live capture waits on a Flux
  source in the fixture cluster (the gap ROADMAP already names). → Mitigation: the tests assert
  only joins and that the registration's own status is shown unchanged.
- **The seam grows a second attribute shape.** A non-resource path in `Attributes` is a new
  covers branch in security-critical code. → Exactly one path and one verb; everything else is
  refused without asking; table tests for covers across both shapes.
- **Goldens churn with the controller rename.** PORTAL-1 recaptures F1 and renames
  `operatorVersion`; whichever lands second regenerates its goldens.
- **The Platform page loses the graph.** A reader who used it to see provider edges now reads rows.
  → `platform/graph` stays in the API; portal:D17 records the call for the owner's review.
- **Browser storage leaks between people on one browser profile.** Filters can name namespaces and
  modules. → Only the query the person already had in the URL bar is stored; no document data.

## Migration Plan

Additive API; old UI paths redirect. No data to migrate. Rollback is reverting the PR.

## Open Questions

Recorded in `docs/DESIGN.md`: portal:OQ22 (Platform health), portal:OQ23 (controller accepts
package providers), portal:OQ24 (package render contracts), portal:OQ25 (catalog contents), and
portal:OQ18 narrowed. None blocks this change.
