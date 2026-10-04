## Why

The owner asked to see "what is installed in the cluster and status + events logs, and ability to
view the resources, with a generated DAG showing the relationships", and decided that a versioned
JSON/SSE read API is the durable contract from day one, with the HTMX UI as its first consumer
(0030:D2). Everything below the API is on `main`: authorization (`internal/authz`), the read model
(`internal/readmodel`), health, the graph model and the SSE broker. Nothing serves any of it over
HTTP yet, so neither the UI (`add-htmx-ui`) nor local mode (`add-local-mode`) can start. This is
the portal's first user-visible surface and its first release, 0.1.0.

## What Changes

- New package `api/v1alpha1`: the exported wire types of the read API, and no logic. Documents
  carry `apiVersion: portal.opmodel.dev/v1alpha1` and a `kind`: `InstanceList`, `Instance`
  (components, inventory objects with access and health, runtime children, conditions, history,
  last apply, render contracts), `PackageList`, `Package`, `Platform` (subscriptions, catalogs with
  their claimants and the registration that contributed them, registrations), `EventList`,
  `Graph` (nodes, edges, layout) and `Removed`; `Problem` for errors. An instance or package
  carries the operator's axis (`reconcile`) and the portal's (`health`) apart (0030:D3).
- New package `internal/api`: `GET` handlers on `net/http.ServeMux` under
  `/api/v1alpha1/clusters/{cluster}/` for `instances`, `instances/{ns}/{name}`,
  `instances/{ns}/{name}/graph`, `instances/{ns}/{name}/events`, the same four for `packages`,
  `platform`, `platform/graph`, `platform/events`, `platform/registrations/{name}/events`, and
  `stream`. Only cluster `default` is served in milestone 1; the path keeps room for more.
  - Every handler authorizes the request's reads through `internal/authz` before it looks
    anything up (0030:D7). A forbidden caller gets the same RFC 9457 problem document for an
    object that exists and one that does not; a list the caller may not read is empty and says
    `forbidden`, with no count or name. Errors are `application/problem+json` with an open set
    of `code` values.
  - The events resources serve the feed about the owner or about one object its inventory or
    runtime children reach; any other object is refused with the forbidden denial (0030:D7:R4).
  - The platform graph's provider instances are authorized and read per provider here, not in
    the read model.
  - No document carries `spec.values`, a Secret or the last-applied annotation (0030:D8).
    Condition, history and event messages are shown verbatim; design.md records the caveat.
  - `stream` serves `internal/stream` with a `Producer` implemented here over the read model:
    each topic carries the same document its `GET` returns.
  - An identity seam: the server takes an `Authenticate` function that resolves each request's
    principal and session, and fails closed on an empty identity. Local mode (`add-local-mode`)
    supplies it.
- `openapi/v1alpha1.yaml`: a checked-in OpenAPI 3.1 document. A test holds every route and every
  wire type to it. The `Lint` workflow runs `oasdiff breaking` against the base branch's copy and
  fails unless the PR title carries `!`.
- `internal/readmodel`: a change feed (which OPM object's view may have changed, from the
  informers it already runs) and kind-to-resource resolution for authorizing an events request.
- `internal/graph`: the catalog-contribution join is exported so the API names the contributing
  registration by the same rule the graph draws its edge.

## Capabilities

### New Capabilities

- `read-api`: the `/api/v1alpha1` resources, their documents, errors, authorization order, the
  change stream's documents, and the OpenAPI contract with its breaking-change gate.

### Modified Capabilities

- `read-model`: the read model reports which OPM object's view may have changed.
- `build-and-release`: the `Lint` check also refuses a breaking API change whose PR title has no
  `!`.

## Impact

- Packages: new `api/v1alpha1`, `internal/api`; additions to `internal/readmodel` and
  `internal/graph`. No UI page; no `cmd/opm-portal` wiring (local mode does that).
- API resources: every resource above is new.
- Dependencies: none new in `go.mod` (Principle VII). CI installs `oasdiff`, pinned, in the
  `Lint` job.
- Principle V: no new kind is read. The API reads only through the read model, which reads as
  before. It adds one kind of authorization check (get on an object whose events are asked for,
  and get on each registration's provider instance), both as the caller. Fail closed holds: no
  principal, no review.
- SemVer: MINOR after 1.0 (new API). On the 0.x line the PR title is `feat(api): serve the read
  API`, which cuts 0.1.0.

Not in this change: changing a stream's topics on an open stream over HTTP (the broker supports
it; the CSRF-protected request comes with `add-htmx-ui`), `/whoami`, YAML views, pod logs
(`add-pod-log-streaming`), the local server, launch token and Host check (`add-local-mode`), and
the default node cap and Pod-group thresholds (revisited in `add-htmx-ui`).
