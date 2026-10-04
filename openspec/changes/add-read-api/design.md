## Context

See proposal.md for why. On `main`: `internal/authz` (a `Grant` only `Checker.Check` issues; the
local checker sends SelfSubjectAccessReviews), `internal/readmodel` (every exported read takes the
caller's identity and a grant and calls `Grant.Covers` first), `internal/health`,
`internal/graph` (pure functions of views; the platform builder takes provider lookups the
caller made) and `internal/stream` (a broker that authorizes every topic and item for the
subscriber, with one send funnel that re-validates grants before each write; a `Producer`
interface the read side implements). This change serves all of it over HTTP.

Evidence: the F1 capture in `testdata/clusters/f1/` (opm-operator v1.0.0-beta.6), with the
image-break samples from enhancement 0030 experiment 01 in `internal/health/testdata/`.

Supervisor rulings this design follows (not owner decisions): the API owns the 0030:D7:R4 gate
for events; `Catalog.Registrations` is exposed as `claimants` and the contributing registration
is the graph's accepted-and-active join; messages are verbatim in milestone 1 with the caveat
below; paths are cluster-parameterized with only `default` served; the platform graph's provider
lookups are assembled here; the node cap and Pod-group thresholds are left to `add-htmx-ui`.

## Goals / Non-Goals

**Goals:**

- Every resource the owner asked for, as JSON under `/api/v1alpha1/clusters/{cluster}/`, plus
  the change stream carrying the same documents.
- Authorization before lookup in one place per handler, with one forbidden document.
- An OpenAPI document a test holds to the code, and a CI gate that keeps the version additive.

**Non-Goals:**

- The local server, launch token, cookie and Host check (`add-local-mode`); `cmd/opm-portal`
  wiring.
- Changing an open stream's topics over HTTP (needs CSRF protection: `add-htmx-ui`).
- `/whoami`, YAML views, pod logs.

## Decisions

### D-a: Wire types mirror the views; mapping lives in internal/api

`api/v1alpha1` holds structs with explicit lowerCamelCase JSON tags and string-typed open enums,
and no functions (Principle II). `internal/api` maps `readmodel` views and `graph.Graph` onto
them. The graph package's own JSON tags are not the contract: the wire `Graph` is a separate
type, so an internal rename cannot change the API.

Shapes (abridged):

```jsonc
// GET /api/v1alpha1/clusters/default/instances/default/podinfo
{
  "apiVersion": "portal.opmodel.dev/v1alpha1", "kind": "Instance",
  "ref": {"group": "opmodel.dev", "version": "v1alpha1", "kind": "ModuleInstance",
          "namespace": "default", "name": "podinfo"},
  "uid": "…", "module": {"path": "…", "version": "0.1.11"}, "owner": "operator",
  "reconcile": {"state": "Applied", "reason": "ReconciliationSucceeded", "message": "…",
                "since": "…", "retrying": false, "notes": []},
  "health": {"state": "Healthy", "counts": {"healthy": 3, "progressing": 0, "degraded": 0,
             "missing": 0, "unknown": 0, "forbidden": 0, "notReadable": 0, "withheld": 0},
             "partial": false, "evaluatedAt": "…", "live": true},
  "inventoryCount": 3, "lastAppliedAt": "…", "serviceAccountName": "",
  "conditions": [{"type": "Ready", "status": "True", "reason": "…", "message": "…",
                  "lastTransitionTime": "…", "observedGeneration": 1}],
  "history": [{"action": "reconcile", "phase": "complete", "sequence": 3, "…": "…"}],
  "lastApplied": {"source": "sha256:…", "config": "sha256:…", "render": "sha256:…"},
  "renderContracts": ["…"],
  "components": [{"name": "podinfo", "health": {…}, "objects": [
    {"ref": {…}, "access": "ok", "health": {"state": "Healthy", "reason": "", "message": ""},
     "evaluatedAt": "…", "live": true, "children": [{"ref": {…}, "owner": {…},
     "health": {…}, "replicas": 2}]}]}]
}
```

`InstanceList` and `PackageList` are `{apiVersion, kind, access, items}`; an item is the detail
without conditions, history, digests, contracts and components. `Platform` carries
`subscriptions`, `catalogs` (`catalog, version, enabled, source, claimants[], contributedBy`),
`registrations` (claim, `accepted`, `active`, `verdict`, reasons and messages, `reconcile`,
`provider`) and `registrationsAccess`. `EventList` is `{apiVersion, kind, regarding, items}`,
each item `{type, reason, note, reportingController, regarding, fieldPath, count, lastSeen}`.
`Graph` is `{apiVersion, kind, scope, root, layout, nodes, edges}`. `Removed` is
`{apiVersion, kind, ref}`. Times are RFC 3339 and omitted when unknown, never a zero time.

The operator's axis is named `reconcile` on the wire, as enhancement 0030's design names the
block; its states are the applied axis (`Applied`, `Reconciling`, `Failed`, `Stalled`,
`Suspended`, `ManagedExternally`, `Unknown`).

### D-b: Routes, authorization and lookup order

One table lists every route: pattern, the wire type its 200 returns, and the handler. The
OpenAPI test reads the same table. Handlers run in this order: cluster check, path validation,
authorization of every read the response will serve, then the read model.

| Resource | Reads authorized for the caller, before lookup |
| --- | --- |
| `instances[?namespace=]` | list `opmodel.dev/moduleinstances` in the scope |
| `instances/{ns}/{name}`, `/graph` | get `moduleinstances` `ns/name` |
| `instances/{ns}/{name}/events` | get `moduleinstances` `ns/name`; get the named object (its resource resolved through discovery); list `events.k8s.io/events` in the object's events namespace |
| `packages…` | the same on `modulepackages` |
| `platform` | get `platforms` `cluster` (registrations are authorized inside the view) |
| `platform/graph` | get `platforms` `cluster`; then, per distinct provider, get `moduleinstances` `ns/name` as a lookup, not a gate |
| `platform/events` | get `platforms` `cluster`; list `events` in `default` |
| `platform/registrations/{name}/events` | get `transformerregistrations` `name`; list `events` in `default` |
| `stream` | the broker authorizes each topic (the producer names the same reads as the GET) |

Inside a view, the read model authorizes each inventory object and the runtime children for the
caller (unchanged). The platform graph's provider lookups: a denial gives
`ProviderLookup{Access: forbidden}`, an unavailable review `notReadable`, an allowed get on a
missing instance `Access: ok, Instance: nil`. No new Kubernetes verb or resource is read; the
only new checks are get on the object whose events are asked for and get on each provider.

Mapping read-model and authz errors:

| Error | Status, code |
| --- | --- |
| `authz` denial `forbidden` / `invalid`, `readmodel.ErrNotCovered`, unknown events kind, object not reached | 403 `forbidden` |
| `authz` denial `unauthenticated` | 401 `unauthenticated` |
| `authz` denial `unavailable` | 503 `upstream_unavailable` |
| `readmodel.ErrNotFound` | 404 `not_found` |
| `readmodel.ErrUnavailable` | 503 `not_readable_by_portal` |

The forbidden document is one constant (`detail`: "The request was refused: you may not read
this, or the portal does not serve it."), so a caller cannot tell which check failed or whether
the object exists (0030:D7:R1). Problem `type` is `about:blank` with the status's reason phrase
as `title`; `code` is the machine-readable part. A problem's `instance` is the request path,
which the client sent.

### D-c: The identity seam

```go
type Principal struct {
    Identity authz.Identity
    Session  string // stream ownership and caps; never logged
}
type Config struct {
    Authorizer   authz.Authorizer
    Model        *readmodel.Model
    Authenticate func(*http.Request) (Principal, error)
    Reader       authz.Identity // holds runtime-children watches for live topics
    Broker       stream.Options
    Logger       *slog.Logger
}
func New(cfg Config) (*Server, error)   // Server is an http.Handler; Close stops the producer
```

The server wraps the mux: `Authenticate` runs first, and a failure, an unauthenticated identity
or an empty session is a 401 before routing to a handler. Local mode supplies `Authenticate`
from its session cookie; tests supply a fixed principal.

### D-d: The stream producer over the read model

Each topic carries one document, the one its GET returns (0030:D2:R5): `Snapshot` returns one
`Item` whose `Render(ctx, who)` runs the GET's own code path for `who` (authorize, read, map).
List topics carry the whole `InstanceList` under the list attributes the broker requires. A
missing object renders as `Removed`. The topics served: `platform`, `instances[:ns]`,
`instance:`, `package:`, and `events:` of those plus `registration:`; `registration:` itself is
not served (no GET returns a lone registration).

Changes come from a read-model change feed (below). The producer keeps the set of active topics
and a dirty set; a flusher publishes one upsert per dirty topic every `Coalesce` (250 ms), so a
burst of Pod updates is one message. A deleted ModuleInstance or ModulePackage publishes a
delete carrying `Removed` as static data (gated by the broker on get of that object). Events are
not watched (tier 4), so active `events:` topics, and topics whose objects are polled, are
refreshed every `Refresh` (30 s); an owner's change also refreshes its events topic.

An active `instance:` or `package:` topic holds the runtime-children watch of its namespace
(`Model.HoldChildren`), as the reader identity, so a Pod's waiting reason reaches the topic
within the coalescing delay; without it a Pod change would surface only on the next refresh.

**Known leak, milestone 2:** change detection is per topic, not per subscriber, so a subscriber
receives an unchanged re-render when something it cannot see changed (a timing signal, no
content). In local mode the reader is the subscriber; milestone 2 must revisit it.

### D-e: The read-model change feed

`Model.OnChange(func(Change)) (remove func())`. `startWatch`, the one informer constructor,
registers an event handler that maps the object to the OPM objects whose views it feeds:

- an OPM kind: itself (`Deleted` on a delete);
- a runtime child (ReplicaSet, Pod, Job): every held ModuleInstance or ModulePackage named by its
  `module-instance.opmodel.dev/name` label (children carry no uuid or namespace label: capture,
  observation 8), in any namespace; an extra notification costs one re-render;
- any other (tier 2, selected by the uuid label): every held ModuleInstance or ModulePackage
  whose `status.inventory` names it. The label's value is not the instance's UID and is recorded
  in status only for the CLI-owned instance (F1: `web/web` `status.instanceUUID`), so the
  inventory is the join;
- a TransformerRegistration also reports the Platform, whose view lists registrations.

A `Change` carries kind, namespace, name and `Deleted`: no object content, so the feed reveals
nothing without a grant. Listeners run on the informer's goroutine and must not block; the
producer only marks a topic dirty.

`Model.ResolveKind(group, kind string) (Kind, error)` exposes the existing discovery-cached
resolver for the events handler's authorization.

### D-f: The contributing registration

`graph.Contributor(catalog readmodel.Catalog, regs []readmodel.RegistrationView) string` returns
the accepted, active registration claiming the catalog at the resolved version, when the registry
entry's source is `Registration`; the platform builder uses it for its `contributes` edge, so the
document and the graph cannot disagree.

### D-g: OpenAPI and the breaking-change gate

`openapi/v1alpha1.yaml` is written by hand, `openapi: 3.1.0`, using only constructs `oasdiff`
v1.33.0 parses (no type arrays; optional fields are absent, not null). Open enums use
`x-extensible-enum`, so adding a value is not reported as breaking. `internal/api`'s contract
test decodes the document with `sigs.k8s.io/yaml` and checks: every route table entry is a
`get` path with a 200 whose schema `$ref`s the wire type's name, and every path is in the table;
for every wire type, the schema's properties equal its JSON fields, each field's type matches
(string, integer, boolean, array with item type, `$ref` by Go type name), and `required` lists
exactly the fields without `omitempty`.

`hack/api-breaking.sh BASE_REF` runs `oasdiff breaking --fail-on ERR` on the base branch's copy
(`git show`) and the head's; no base copy means nothing to compare. The `Lint` job runs it on
pull requests after `go install`ing oasdiff at a pinned version; the PR title reaches the script
through an environment variable, never interpolated into the shell, and a title matching
`^[a-z]+(\([^)]*\))?!:` lets a breaking diff pass with a notice.

## Research & Decisions

### One document per topic, rendered per subscriber

**Context**: `Producer.Snapshot` has no identity, but every read-model read needs the caller's
grant, and health in a list depends on what the caller may read.
**Explored**: `internal/stream/item.go` (`Render(ctx, who)`), `broker.go` (list topics must be
exactly one list read, items outside it dropped).
**Options considered**:
1. One item per instance on list topics, enumerated as the reader - leaks names the subscriber
   may not list into the producer's choice of items, and needs the reader's grant in the API.
2. One item per topic carrying the GET's document, rendered for the subscriber - same shape as
   the GET by construction; a list change re-renders the whole list.
**Decision**: 2.
**Rationale**: Exact parity with the GET (0030:D2:R5), no identity in `Snapshot`, and list sizes
in V1 are small.

### Events gate: same denial as forbidden

**Context**: 0030:D7:R4 says refuse as "not in inventory"; the supervisor ruled the API returns
the forbidden denial instead.
**Options considered**: 1. 404 `not_in_inventory` - tells an allowed caller the object exists
outside OPM. 2. The forbidden document.
**Decision**: 2, per the ruling. `not_in_inventory` is not in this version's code list; logs may
add it.

### List denial: empty list, not 403

**Context**: 0030:D7:R2 wants an empty list with no count; Principle IV wants degraded reads
shown as degraded.
**Decision**: `200`, `items: []`, `access: forbidden`. The stream refuses a list topic outright,
as the broker already does.

### Messages verbatim (0030:OQ8)

Condition messages, history messages and event notes are served as written. Whether the
operator's embedded kernel redacts marked secret paths in them is open (0030:OQ8). In local mode
the portal reveals nothing the user's kubeconfig cannot already read with `kubectl`; milestone 2
must decide before the in-cluster release. The OpenAPI description states the caveat in plain
words.

## Risks / Trade-offs

- [Re-render per subscriber on every change] -> coalescing (250 ms) and small V1 sizes; measure in
  `add-local-mode` against F1.
- [Hand-written OpenAPI drifts] -> the contract test fails on any route or field mismatch.
- [oasdiff and 3.1] -> only 3.0-compatible constructs; the gate is run locally against this PR.
- [Timing signal in milestone 2] -> recorded in D-d.
