## Context

The web UI (changes `add-htmx-ui` and `refine-htmx-ui`) serves a Platform page built around the
platform graph, separate Instances and Packages lists, and one long page per instance or package.
The owner reviewed a redesign on a design canvas (boards in
`docs/design/evidence/03-ui-canvas/`, mock data) and answered its questions on 2026-10-05. Those
answers and the supervisor's rulings are recorded as portal:D14 to portal:D18 in
`docs/DESIGN.md`; this document says how the change builds them.

Every claim about controller output below names opm-operator source at 277ca18 or the F1 capture
under `testdata/clusters/f1/`. The portal imports no controller module; F1 was taken from a
fixture cluster running opm-operator v1.0.0-beta.6 (`test/e2e/versions.env`), and the field shapes
cited are the same at both.

## Goals / Non-Goals

**Goals:**

- The canvas's views, cut down to what the cluster records: a header naming cluster, reader and
  version; Platform; one Installed list; instance and package pages with summary cards and tabs;
  a Provider tab; a Catalog page.
- Filters that survive a visit and a restart, and hand-off links that mean the same thing to
  everyone (portal:D14).
- Providers found by inventory, instance or package (portal:D15), with the controller's verdict
  unedited.
- Every new read stated with its authorization; no new kind, no write, no Secret, no values.

**Non-Goals** (each a canvas feature left out, portal:D17):

- Catalog contents (portal:OQ25): the Catalog page's Definitions, Contracts, Transformers and Used
  by tabs, definition descriptions, popovers and documentation links, and on the Provider tab a
  provided contract's kind and contract status and the transformers a provider ships.
- A catalog's registry and digest, which the Platform does not record, and a YAML tab on the
  Catalog page: raw YAML is served only for inventory objects (portal:D8).
- A health for the Platform itself (portal:OQ22); the transformer behind each object
  (portal:OQ5).
- On graph hover cards, an object's own applied age and its warning-event count: the inventory
  records no time per entry, and a count would need an events read per node.
- Controller changes: package providers (portal:OQ23,
  [opm-operator#254](https://github.com/open-platform-model/opm-operator/issues/254)), package
  render contracts (portal:OQ24).
- Renaming wire names (`operatorVersion`, owner `operator`): the controller rename's portal change.
- A `packages` stream topic: the Installed list follows `instances` live, as the Packages list
  followed nothing before; package rows refresh on navigation.
- In-cluster mode.

Kept from today rather than dropped: the details panel's Open, Expand, YAML and Events links
(`internal/ui/templates/panel-body.html:34-38`); a Pod's panel gains a Logs link to the Logs tab.

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
	// KubernetesVersion is the API server's gitVersion, read once at start;
	// absent when that read failed.
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
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
	// ProviderRefMatches is set only when Access is ok: true when the
	// owner is a ModuleInstance and the registration's spec.providerRef
	// names its namespace and name. Always false for a ModulePackage, since
	// the reference names a ModuleInstance.
	ProviderRefMatches *bool `json:"providerRefMatches,omitempty"`
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
 "readingAs":{"username":"kubernetes-admin"},"kubernetesVersion":"v1.36.1"}
```

`GET /api/v1alpha1/clusters/{cluster}` serves the `Cluster` document. It reads nothing from the
cluster per request: the context names come from the mode's configuration, the username from the
request's principal, the version from the read model. Errors: `401 unauthenticated` without a
principal; `404 not_found` for a cluster other than `default`, before any review (as every route);
`405` for another method. In-cluster, `omitOperatorText` (`internal/api/mode.go:46-84`) gains a
`Cluster` case that passes it through (it carries no operator text), so the document is not refused
as an unknown type.

`SourceArtifact` takes `status.source.artifactRevision` and `artifactDigest`
(opm-operator `api/v1alpha1/common_types.go:83-99`, `modulepackage_types.go:88-90`).
`status.source.artifactURL` is not served: it is a fetch URL inside the cluster, of no use to a
reader. `Interval` is `spec.interval` as written (`modulepackage_types.go:40-48`); when absent the
page says "not set", never the controller's default. The F1 package has `interval: 1m` and no
`status.source`, because the fixture cluster runs no Flux; the revision path is tested by a unit
fixture.

`HistoryEntry.outcome` reads the controller's two entry shapes: a success entry carries `phase:
complete` and no message, a failure entry carries a message and no phase
(`internal/status/history.go:16-57`). Every failure entry is written through `NewFailureEntry`, at
`internal/reconcile/moduleinstance.go:179` and `:347` and `internal/reconcile/modulepackage.go:259`
and `:432`; every success entry through `NewSuccessEntry` with phase `complete`, at
`moduleinstance.go:344` and `modulepackage.go:257`. `Succeeded` when the phase is `complete`,
`Failed` when the phase is empty and the message is not, `Unknown` otherwise. It is computed before
the in-cluster omission drops history messages, so in-cluster pages still tell a failed attempt
from a successful one. It is a reading of recorded shapes, not an inference from events
(portal:D9:R1). The F1 capture holds both shapes live: `pkg/podinfo`'s three entries carry no phase
and the source error as their message, and the instances' entries carry `phase: complete`
(`testdata/clusters/f1/modulepackages.yaml`, `moduleinstances.yaml`).

### The holder join, kind-agnostic (section 1)

The read model holds every ModuleInstance and ModulePackage with its inventory. A registration's
holders are the owners whose `status.inventory.entries` hold `group: opmodel.dev, kind:
TransformerRegistration, name: <registration>` (entry shape `{group, kind, namespace, name, v,
component}`, `api/v1alpha1/common_types.go:131-147`). The join is a scan of held state, rebuilt
with the views; it reads nothing new.

```go
// Holders returns the owners whose inventory holds registration name, among
// those the caller may read, and whether the caller lacks a cluster-wide
// list of ModuleInstances or ModulePackages.
func (m *Model) Holders(ctx context.Context, who authz.Identity, name string) ([]ObjectRef, bool)

// ProviderOf returns the registrations owner's inventory holds, each with
// the caller's access to it and the controller's standing.
func (m *Model) ProviderOf(ctx context.Context, who authz.Identity, owner ObjectRef, inv []InventoryEntry) []ProviderClaim
```

Authorization: `ProviderOf` asks `list transformerregistrations` (cluster scope) for the caller
once per request, through the cached decisions the platform view already uses; denied, each claim
carries `access: forbidden` and its name only (the name is in the owner's inventory, which the
caller is reading). `Holders` includes an owner only when the caller may `list` its kind in its
namespace. `heldByPartial` is true whenever the caller lacks cluster-wide `list` on ModuleInstances
or on ModulePackages, whether or not a holder was found, so an empty `heldBy` never reads as
"nobody holds it" when the caller could not look everywhere.

As built (section 1): the two joins are unexported (`holders`, `claims.of` in
`internal/readmodel/providers.go`) and fill the views (`InstanceItem.ProviderOf`,
`PackageItem.ProviderOf`, `RegistrationView.HeldBy` and `HeldByPartial`), because every exported
read of the model takes a grant (`TestEveryReadTakesAGrant`). `heldByPartial` is also true when
the model itself does not hold every namespace (`--namespaces`, or a scope the reader may not
watch). An inventory entry naming a registration the model does not hold (not created yet, or
deleted) gives a claim with `access: ok`, verdict `Unknown` and `providerRefMatches: false`,
never a guessed standing. A change routed through a join is marked `Joined` on the read model's
`Change`, so the producer marks the document topics and leaves the events topics alone.

`Registration.provider` keeps meaning what it meant: `spec.providerRef` as the controller reads it,
kind ModuleInstance (`api/v1alpha1/common_types.go:48`). `heldBy` is the new, kind-agnostic answer.
`ProviderClaim.providerRefMatches` says, for a readable registration, whether the reference names
this owner. The catalog's transformer stamps the reference from the rendering context's name and
namespace (`catalog_opm/src/transformers/transformer_registration_transformer.cue:80`), but the
reference names a ModuleInstance, so for a package it is false even when namespace and name are
equal: the controller looks for a ModuleInstance of that name, finds none (or one whose inventory
lacks the claim), and refuses with `ProviderMismatch`
(`internal/controller/transformerregistration_controller.go:175-180`, `:369-401`). A page shows the
holder, the reference and the refusal side by side (portal:D15:R3, portal:D4:R2).

Catalog rows already join `claimants` and `contributedBy` from the registrations, because
`status.registry` does not name the registration that contributed an entry. The controller issue
[opm-operator#230](https://github.com/open-platform-model/opm-operator/issues/230) would record it
on the Platform; until then the join stays as it is.

`internal/graph` does not change: the platform graph's provider lookup stays instance-only and the
Platform page no longer shows that graph (portal:D17).

### Change stream routing for the joins (section 1)

`providerOf` and `heldBy` join two kinds of object, so a change to one must refresh topics showing
the other. Today a registration change marks only `events:registration:<name>` and, through
`changesFor`, `platform` (`internal/api/producer.go:301-320`, `internal/readmodel/changes.go:122-123`);
an instance or package change marks only its own topics and the instance lists. The producer
gains two routes:

- A TransformerRegistration change also marks the `instance:` or `package:` topic of each held
  owner whose inventory holds it, and `instances` and `instances:<namespace>` for instance holders.
- A ModuleInstance or ModulePackage change also marks `platform` when its inventory holds a
  TransformerRegistration, before or after the change (a holder that drops a claim must refresh
  `heldBy` too).

A test drives each route through the producer over F1 (accepting and refusing
`default.backup-provider`; removing it from backup-provider's inventory) and asserts the topics
marked. The per-subscriber diff (portal:D2:R7) still decides what is sent.

### The server version, like discovery (section 1)

The read model already reads the discovery documents `/api` and `/apis` through
`restmapper.NewDeferredDiscoveryRESTMapper` (`internal/readmodel/kinds.go:59-63`) with no access
review: they are non-resource paths every authenticated identity may read. It reads
`discovery.ServerVersion()` (`GET /version`) the same way, once at `Start`, and holds `gitVersion`.
A failed read holds nothing and does not stop the start; the `Cluster` document then has no
`kubernetesVersion` and pages say "unknown". The authorization seam does not change.

Verbs and resources this change reads: `get` on the non-resource path `/version`, once per start,
without a review (portal:D18:R3). Everything else is already read: `get`, `list`, `watch` on
ModuleInstances, ModulePackages, TransformerRegistrations and the Platform, and `list` on
`events.k8s.io` events, with the grants they take today. The only creates stay
`selfsubjectaccessreviews` and `selfsubjectreviews` (portal:D5:R6). The `deploy/` role needs no new
rule: `/version` is open to every authenticated identity through `system:public-info-viewer`, which
section 5's `TestPod` confirms by finding a version in the `Cluster` document.

### Local mode: a fixed default port and its context names (section 1)

`--addr` defaults to `127.0.0.1:7878` instead of `127.0.0.1:0` (`cmd/opm-portal/serve.go:42`), so
the browser origin, and with it the stored theme and filters, survives a restart (portal:D14:R6).
The listener moves up: `serve` binds right after `loopbackAddr` accepts the address, before
`connect` loads the kubeconfig and sends the SelfSubjectReview (today the listen follows them,
`serve.go:226-231`). A taken port therefore fails before any cluster call, and `serve` exits 1
with `127.0.0.1:7878 is in use; pass --addr 127.0.0.1:<port> to use another port` in place of the
bare listen error. The loopback checks are unchanged.

The e2e helpers start the portal with `--addr 127.0.0.1:0`, so a nightly run never collides with a
portal a developer has open on 7878 (`cmd/opm-portal/e2e_test.go:91`, `:137`, `:158`;
`m1_e2e_test.go:55`, `:76`); `TestLocalMode` keeps one start on the default address to test it. `deploy/deployment.yaml:32` passes `--addr 127.0.0.1:8090` and keeps it, so the Pod
path is not affected; `--open` and the printed launch URL already use the bound address.

`cmd/opm-portal` already resolves the context it loaded (`configSource`, `serve.go:180-194`). It
passes the context name, the context's cluster entry name, or `source: in-cluster`, to
`api.Config.Connection`. The read API never sees the kubeconfig itself. The username is the
principal's identity, already resolved per request.

### The shell and theme (section 2)

The masthead becomes sticky: brand, nav (Platform, Installed), the cluster chip (`context`, or "in
cluster"), "reading as <username>", the Kubernetes version ("unknown" when absent), the live mark,
and a theme menu (Light, Dark, System). The header is the only place the version and context show.
When the `Cluster` fetch fails, the header still renders its nav, live mark and theme menu, and the
cluster, reader and version read "unknown" in the degraded style. An `IntersectionObserver` on a
sentinel 48 px down the page toggles a `compact` class that slims the header;
`prefers-reduced-motion` turns the transition off.

Theme: `portal.css` defines every colour as a custom property on `:root`, redefines them under
`@media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) { ... } }` and again under
`:root[data-theme="dark"]`. A new `/static/prefs.js`, loaded in `<head>` without `defer` before the
stylesheet, reads `localStorage["opm-portal.theme"]` in a `try` block and sets
`document.documentElement.dataset.theme` to `light` or `dark`, or nothing for System. It is the
only script that runs before first paint, so a stored choice never flashes the other theme. The
page policy does not change: `script-src 'self'` admits it, and it sets an attribute, not a style.

### Filters: URL first, browser second (section 2)

Each view has a fixed set of filter parameters with fixed values; the URL query is their source of
truth (portal:D14). Only the list views remember theirs: Installed and the Platform page's two tabs.

| View | Parameters | Remembered |
| --- | --- | --- |
| `/installed` | `q`, `kind` (`instance`, `package`), `provider` (`yes`, `no`), `uses` (a contract), `namespace`, `health`, `applied`, `owner` (`controller`, `cli`), `module` | yes, key `opm-portal.filters.installed:<context>` |
| `/` Providers tab | `pq`, `pstatus` (`active`, `accepted`, `refused`, `blocked`, `pending`), `provides` | yes, key `opm-portal.filters.providers:<context>` |
| `/` Catalogs tab | `cq`, `csource` (`subscription`, `registration`, `claim`), `claimed` (`yes`, `no`) | yes, key `opm-portal.filters.catalogs:<context>` |
| `/` events | `eresource` (`platform` or `registration:<name>`) | no |
| instance and package Events tab | `resource`, `type`, `reason` | no |
| instance and package Resources tab | `reason` (a health reason) | no |

`<context>` is the kubeconfig context name, or `in-cluster`: the layout renders it into a
`data-context` attribute on `<html>` from the `Cluster` document, and `prefs.js` reads it there.
One origin can serve different clusters (the same port, another `--context`), and filters naming
one cluster's namespaces and modules are not restored against another. When the `Cluster`
document could not be fetched the attribute is absent and nothing is restored or stored.

`tab` and `focus` are navigation, not filters: they are never stored and never count as "the URL
carries a filter". The Health card's reason counts link to `tab=resources&reason=<health reason>`.

`internal/ui` filters server-side from the query, so every filtered view is a plain link that
works without script, and each active filter renders as a chip whose link drops that parameter.
An unknown value of an enumerated parameter (`health=Bogus`) is ignored, shown as a dismissible
note "ignored filter health=Bogus", and never stored; free-text and name parameters (`q`, `uses`,
`namespace`, `module`, `provides`) are matched as given. The filter form is a `GET` form
(`form-action 'self'`).

Remembering (`prefs.js` and `portal.js`):

1. On a full page load, `prefs.js` checks whether the path is `/installed` or `/` and the query
   carries none of a remembered view's parameters; if so, it reads that view's key, keeps only the
   view's own parameters with valid values (an enumerated value outside its set is dropped), and if
   anything is left calls `location.replace` with the current path and query plus the stored
   parameters, before anything paints. A stored query that validates to nothing is removed.
2. On a boosted navigation (`hx-boost` swaps `#main`, so the head script does not run),
   `portal.js` handles `htmx:configRequest` for requests htmx makes for a boosted link or form only
   (`evt.detail.boosted`; never region refreshes, panel fetches or the stream's topic `POST`), and
   applies the same check and rewrite to the request path. htmx 2.0.11 then requests and pushes
   the rewritten URL, so no `HX-Push-Url` header is needed (spike, "Boosted filter restore"
   below). Only a boosted link is rewritten: a boosted `GET` form carries its own filter fields as
   request parameters, which are an explicit choice and are never merged with stored ones.
3. After every render of a remembered view, `portal.js` writes the view's current parameters to its
   key, or removes the key when there are none. "Clear filters" is a link to the path without that
   view's parameters that removes the key first.

Every storage access is in `try`/`catch`; on failure the page behaves as if nothing is stored.
What is written is the query the viewer already has in the URL bar, never a document field
(portal:D14:R1).

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

- **Identity card**: name, type (`spec.type`) and controller version (`operatorVersion`). The
  context, reader and Kubernetes version are in the header (portal:D18).
- **Status card**: the `Ready` state with its reason and the portal's meaning. The Platform's
  `Ready` reasons are `Generated` (true), and `BuildFailed`, `GenerateFailed`,
  `ContractCollisions`, `OverSubscribedContracts`, `ComparablePredicates` (false)
  (`internal/status/conditions.go:81-127`; `api/v1alpha1/platform_types.go:182-206`);
  `ContractsFulfilled` is shown beside it as information: `UnfulfilledContracts`,
  `ContractsFulfilled`, `NoContractsDefined` (`conditions.go:134-148`;
  `platform_types.go:208-215`) (portal:D3:R8). A subscription the controller cannot resolve shows
  on the Platform as `Ready=False/BuildFailed`; `CatalogUnresolved` is a TransformerRegistration
  reason (`conditions.go:171`) and is linked only from a registration. Each reason links to its row
  in the conditions list, which shows meaning, next step and message. No Platform health block
  (portal:D17:R4).
- **Installed card**: counts per health state and per applied state over both lists, each a link
  to `/installed` with that filter; one line "N installed: I instances, P packages; K of them hold
  a registration". A list the caller cannot read makes its counts locked, not zero.
- **Providers | Catalogs tabs** with the filters above. A provider row: registration name, catalog
  and version (plain text until section 5 links them to the Catalog page), provided contracts,
  holder ("Installed as": kind chip and a link to the holder's page; section 5 points it at the
  holder's Provider tab), acceptance and activation as two pills (portal:D4:R4), and the refusal or
  blocked reason. A catalog row: path, version, source, claimants (links to their holders),
  enabled. The note that a provider-fulfilled contract without a provider is information stays.
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
- **Applied card**: the applied badge, reason and meaning; a `Reconciling` mark with its reason and
  since time while that condition is `True`; the counts of Warning events in the current feed by
  reason, each linking to the Events tab filtered by that reason, labelled as the last hour of
  events (portal:D9:R1/R2: counts of the feed, never a state); and the attempt dots.
- **Attempt dots**: one dot per `status.history` entry, oldest left, coloured by `outcome`, with
  action, outcome, finish time and, in local mode, the message in its title. There is no running
  dot: `Reconciling=True` is set at the start of every package reconcile
  (`internal/reconcile/modulepackage.go:279`, through `MarkReconciling`,
  `internal/status/conditions.go:250-253`) and stays beside `Ready=False` while retries continue,
  as F1's `pkg/podinfo` shows, so it says "the controller is working on it", not "an attempt is
  running"; the Applied card shows it as the condition mark above. Under the dots: "at most ten
  attempts are kept; reconciles that change nothing are not recorded"
  (`internal/status/history.go:9-13`, `:59-66`). "Show history" opens the history table that exists
  today.
- **Health card**: the health badge, partial and live marks as today, and the counts of objects
  and runtime children by health reason (`ImagePullBackOff`, `CrashLoopBackOff`, ...), computed
  in `internal/ui` from the `Instance` or `Package` document's per-object and per-child health.
  Each links to `tab=resources&reason=<reason>`, because health reasons are container waiting
  reasons, not event reasons.
- **Provider card**, only when `providerOf` is not empty: per claim, Active, Accepted and not
  active, Refused, Removal blocked or Pending, with the reason linking to the Provider tab.
- **Tabs** as `?tab=graph|resources|events|logs|yaml|provider`, server-rendered, boosted links.
  The details panel shows only on Graph and Resources. Resources lists kind, name, origin
  (inventory, made by the cluster below an inventory object), health, reason and age where the
  document has one, filtered by `reason`; a row's link opens Graph with `focus=<node id>`. Events,
  Logs and YAML are today's regions moved into tabs, with Events gaining the `resource`, `type` and
  `reason` filters.

Graph interactions, all in `portal.js` over the server-rendered SVG:

- **Hover cards**: the server renders one card per node in a hidden list beside the SVG (kind,
  name, health and applied marks, health reason, origin, access), and the script shows the card on
  pointer hover or keyboard focus, positioned through CSSOM properties (`el.style.left`), never a
  `style` attribute, which the policy forbids.
- **Focus spotlight**: `focus=<id>` dims every node and edge not adjacent to it by a class;
  "Clear selection" drops the parameter.
- **Full screen**: the Fullscreen API on the graph region, with a class-based fallback.
- **Zoom**: an `<input type="range">` bound to the existing zoom scale, beside Fit; Ctrl/Cmd-wheel
  and pointer pan exist. The −, 1:1 and + buttons are gone (supervisor ruling 2026-10-05).
- **Groups**: a group node's accordion expands it through the graph resource's `expand` parameter
  (exists), then fits the view to the group's members; a "Whole graph" button removes the
  expansion and fits again.
- **Locked edges** stay as they are: `edgeOf` (`internal/ui/graph.go:221-224`) draws the locked
  style only for an unverified edge with a locked end, and only `platform/graph` produces one (its
  provider lookup); instance and package graphs look up no access for `dependsOn` or source edges
  (`internal/graph/instance.go:104-121`). With the platform graph off the Platform page, no page
  draws a locked edge; a `dependsOn` to a package the caller may not read is drawn as today.

Live refresh keeps the open tab, the `focus` parameter, the selected node, zoom, the open card and
open groups (the refresh already keeps open groups, selection and zoom, `portal.js:196-236`). A
change refreshes the regions of the open tab and the summary cards; a tab not shown is rendered
fresh when opened.

### Provider tab and Catalog page (section 5)

The Provider tab, on any instance or package with `providerOf`: per held registration, its name,
claimed catalog (link) and version, whether the catalog is in the Platform's resolved registry and
contributed by this registration (`contributedBy`), the registration's conditions table (type,
status, reason, meaning, message in local mode, since `lastTransitionTime`), and per provided
contract, "Used by": the first three readable instances whose `renderContracts` contain it, "and N
more", and a link "All N in Installed" to `/installed?uses=<contract>` (portal:D16:R3). The
registration's conditions and catalog come from the `Platform` document; when the caller may not
read the Platform, those parts render locked and the tab keeps the name and standing from
`providerOf`. The graph already includes the registration as an inventory object.

The Catalog page, `/catalog?path=<catalog path>`, for any catalog the Platform subscribes to,
holds in its registry, or a readable registration claims: path, version, origin (Subscribed, From a
provider, or Claimed only), source (the subscription, or the contributing registration and its
holder), enablement; a Resolved block (in `status.registry`, or not, with the Platform's `Ready`
reason, or the claiming registration's refusal reason such as `CatalogUnresolved`); the Platform's
`ContractsFulfilled` labelled as platform-wide; tabs Claims (registrations claiming it, holders,
standing) and Events (Platform and claimants). The page says that definitions and transformers
are not shown because the cluster does not record them (portal:D17:R3). It reads no new API
resource: everything comes from the `Platform` document and the registrations' `heldBy`. When the
caller may not read the Platform, the page renders locked with status `403`, saying nothing about
whether the catalog exists; it is `404` only for a readable Platform that names no such catalog.

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
   loses the facts, and none come from the Platform object.
2. A `Cluster` document at the cluster path, shown in the header - every page reads it; one more
   in-process fetch per page.
**Decision**: Option 2 (portal:D18). The Platform identity card keeps only Platform facts.
**Rationale**: The facts describe the connection, not the Platform, and must show on every page,
including to a caller who may not read the Platform.

### Kubernetes version source and its authorization

**Context**: The canvas shows a Kubernetes version; nothing the portal reads carries it.
**Explored**: discovery `ServerVersion()` (`GET /version`); Node `status.nodeInfo.kubeletVersion`;
how the read model reads discovery today (`internal/readmodel/kinds.go:59-63`, no review);
`internal/authz/attributes.go:15-21` (resource attributes only).
**Options considered**:
1. `/version` behind a review - extend `authz.Attributes` with a non-resource path, refuse every
   other path without asking, send `nonResourceAttributes` reviews, and change `Covers` and its
   tests. Keeps portal:D5:R7 literally, at the cost of a second attribute shape in the seam, to guard a
   fact every authenticated identity may read.
2. `/version` read like discovery, once at start, without a review - no seam change; the same
   treatment `/api` and `/apis` already get. One more exception to portal:D5:R7, of the same kind.
3. Nodes - a new kind with a `list` grant on every node; kubelets may differ from the server.
**Decision**: Option 2 (supervisor ruling 2026-10-05; portal:D18:R3).
**Rationale**: The value is public through `system:public-info-viewer`, reveals nothing about any
object, and is read once; the seam stays as small as it is.

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
**Decision**: Option 2, URL wins, stored only for list views, restored only when the URL has none
of the view's filters.
**Rationale**: Principle VII as amended allows display preferences in the browser and nothing
more; filtering stays on the server, so no page needs client-side component state, and hand-offs
stay exact.

### Local mode's default port

**Context**: Browser storage is per origin, including the port; local mode picked a free port on
every run, so every restart lost the stored theme and filters.
**Options considered**: keep the random port (nothing survives a restart); a fixed default port
with `--addr` to override (owner's choice); keep preferences server-side (rejected under
portal:D14).
**Decision**: `127.0.0.1:7878` by default, failing clearly when taken (portal:D14:R6).
**Rationale**: Owner answer 2026-10-05 ("Fixed default port (Recommended)"). The cost: another
local process that later binds 7878 serves pages on the same origin and can read the stored filter
queries; `portal-security.md` says so.

### Boosted filter restore (spike)

**Context**: Remembered filters must be restored on a boosted navigation, where the head script
does not run, without painting the unfiltered view and with the filtered URL in the address bar.
The question: if `portal.js` rewrites the path of a boosted request in `htmx:configRequest`, which
URL does htmx 2.0.11 push, and which requests count as boosted?
**Explored**: the vendored `internal/ui/static/vendor/htmx-2.0.11.min.js` (minified names in
brackets), read without a browser:
- `issueAjaxRequest` builds the request config as `{boosted: getInternalData(elt).boosted, ...,
  path, ...}` and fires `htmx:configRequest` with it as `evt.detail` [`const $=re(r).boosted; ...
  const C={boosted:$, ..., path:n, ...}; if(!ae(r,"htmx:configRequest",C))`]. After the event it
  reads the path back from the detail [`n=C.path`], splits off a `#` anchor, appends the
  parameters to it for a `GET` (with `&` when the path already has a query), and opens the request
  on that final path [`g.open(t.toUpperCase(),T,true)`]. So a rewritten `evt.detail.path`,
  query included, is the URL requested.
- `determineHistoryUpdates` [the function holding `HX-Push-Url`]: with no `HX-Push`,
  `HX-Push-Url` or `HX-Replace-Url` response header and no `hx-push-url` or `hx-replace-url`
  attribute (the portal sets none), a boosted element pushes `responsePath || finalRequestPath`
  [`else if(u){a="push";f=s||i}`]. `responsePath` is the `pathname + search` of the XHR's
  `responseURL` [`function Nn`], which is the rewritten URL, or its target after a server redirect.
  So the rewritten URL is pushed, and the anchor is appended when the rewritten path has none.
- Which elements are boosted: `boostElement` [`function ht`] sets the internal `boosted` flag only
  on an `HTMLAnchorElement` with a same-host, non-`#` `href` and an empty or `_self` target, or on
  a `FORM` whose method is not `dialog`; `processNode` [`function Mt`] calls it only under
  `hx-boost="true"` and only when the element has no explicit `hx-get`, `hx-post` and so on.
- The other requests on a page: region refreshes and the topics `POST` go through `window.fetch`
  (`portal.js` `refresh` and `post`), which fires no htmx event at all. The panel fetch is
  `htmx.ajax("GET", panel, {source: a})` from a graph node, an SVG `<a>` (`SVGAElement`, not
  `HTMLAnchorElement`), so its source is never boosted. The panel's YAML and Events links carry an
  explicit `hx-get`, so they are not boosted either. None of them sees `evt.detail.boosted`.
- A boosted `GET` form drops the query from its `action` [`o=o.replace(/\?[^#]+/,"")`] and sends
  its fields as parameters, which htmx appends to whatever path the handler leaves.
**Options considered**:
1. Rewrite `evt.detail.path` in `htmx:configRequest` - the rewritten URL is requested and pushed;
   no server change.
2. The server answers with `HX-Push-Url` - needs `internal/ui` to know the stored filters, which
   live only in the browser.
**Decision**: Option 1, for boosted links only: the handler acts when `evt.detail.boosted` is true,
the verb is `get` and `evt.detail.elt` is an `A`. A boosted form submit is left alone, because its
fields are the viewer's explicit filters and would otherwise be appended to the restored query.
A tab link (`?tab=`, section 4) is a boosted link and is restored like any other, since `tab` and
`focus` never count as filters. `HX-Push-Url` is not needed.
**Rationale**: The code path is short and has no branch for a rewritten path. Confidence: high
for the request and push behaviour, which follows directly from the source above; not run in a
browser, so section 2's `TestBrowserTheme` (a stored filter opens filtered) is the live check.

### History outcome and Reconciling

**Context**: The canvas draws attempt dots as Applied, Failed or Running. `status.history` has no
result field, no per-entry reason, no running entries, and no-ops are not recorded.
**Decision**: `outcome` from the two entry shapes, computed before the in-cluster omission; no
running dot; `Reconciling=True` shown as a condition mark with reason and since.
**Rationale**: In-cluster mode drops history messages (portal:D8:R5), so a page cannot tell the
shapes apart after the omission. `Reconciling=True` persists beside `Ready=False` across retries,
so a running dot would claim an attempt in flight the controller never recorded.

### Package providers

**Context**: Owner: a package can be a provider (portal:D15). The controller refuses its claims.
**Decision**: The UI is kind-agnostic through `heldBy` and `providerOf`; the refusal is shown as
the controller wrote it; the controller change is portal:OQ23
([opm-operator#254](https://github.com/open-platform-model/opm-operator/issues/254)), not a task
here.
**Rationale**: Principle IV; a portal that hid or relabelled the refusal would disagree with
`kubectl`.

## Risks / Trade-offs

- **Package-provider path has no live evidence.** F1 has one ModulePackage, `SourceNotReady`
  because the fixture cluster runs no Flux, and it holds no claim. The path is covered by unit
  tests over read-model fixtures derived from the F1 instance capture, labelled as constructed,
  with the refusal reason and message taken from controller source. A live capture waits on a Flux
  source in the fixture cluster (the gap ROADMAP already names). → Mitigation: the tests assert
  only joins and that the registration's own status is shown unchanged.
- **A second unreviewed non-resource read.** `/version` joins `/api` and `/apis` as reads without a
  review. → Read once, at start, a public value; portal:D18 records the exception.
- **A fixed port can be taken.** Another tool on 7878 stops `serve` from starting. → A clear
  message naming `--addr`; and anything else on that origin can read the stored queries, stated in
  `portal-security.md`.
- **Goldens churn with the controller rename.** PORTAL-1 recaptures F1 and renames
  `operatorVersion`; whichever lands second regenerates its goldens.
- **The Platform page loses the graph.** A reader who used it to see provider edges now reads rows.
  → `platform/graph` stays in the API; the owner confirmed it on 2026-10-05.
- **Browser storage is shared by everyone on one browser profile.** Filters can name namespaces
  and modules. → Only the query the person already had in the URL bar is stored.

## Migration Plan

Additive API; old UI paths redirect. Local mode's default address changes from a random port to
`127.0.0.1:7878`; a script that parsed the printed URL keeps working, and one that needs a random
port passes `--addr 127.0.0.1:0`. Rollback is reverting the PR.

## Open Questions

Recorded in `docs/DESIGN.md`: portal:OQ22 (Platform health), portal:OQ23 (controller accepts
package providers, opm-operator#254), portal:OQ24 (package render contracts), portal:OQ25 (catalog
contents), and portal:OQ18 narrowed. None blocks this change.
