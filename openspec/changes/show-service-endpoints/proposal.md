## Why

The Service side panel of the instance graph cannot say whether a Service has ready endpoints. It
shows the Service's type and ports and stops there (`internal/ui/panelfacts.go`, `factsOf`). A
Service whose Pods are Running but not ready, or whose selector matches nothing, looks the same
as a healthy one. The cluster records the answer in the Service's EndpointSlices, but the portal
reads no EndpointSlice: the kind is outside the ClusterRole (`deploy/clusterrole.yaml`) and
outside the runtime children the portal walks (`internal/readmodel/children.go`). The earlier
`align-owner-pages` change left the Endpoints fact out for exactly this reason: a new kind needs
its own decision under Principle V.

The owner took that decision on 2026-10-12 (OD12, "Service endpoints (G14)": "Add the read"). This
change builds it: the panel shows "Endpoints N ready", counted from the Service's EndpointSlices,
and nothing else about them.

## What Changes

- **The panel shows one new fact on a Service.** `Endpoints` with the value `N ready`, for
  example `2 ready`, and `0 ready` when the cluster records slices with no ready endpoint. When
  the cluster records no slice for the Service, or the caller or the portal may not read them, the
  panel shows no Endpoints line at all. It never shows a name, an address, a port or a node of an
  endpoint (portal:D8).
- **The read has its own review.** The portal reads a Service's EndpointSlices only for a caller
  who passes an access review for `list` on `discovery.k8s.io` `endpointslices` in the Service's
  namespace. The review for the Service does not stand in for it. A caller who may read the
  Service but not its EndpointSlices sees no readiness fact and no sign that one exists. This is
  the new authorization case of the plan's security requirement SR5.
- **One new ClusterRole rule.** `deploy/clusterrole.yaml` gains `list` on `discovery.k8s.io`
  `endpointslices`, and nothing else: no `get`, no `watch`, no wildcard, no Secret (SR10). The role
  test holds the rule to exactly that.
- **One additive field on the wire.** The graph node of a Service gains `endpointsReady`, an
  integer that is absent when nothing is shown. `/api/v1alpha1` changes only additively
  (Principle III); `task api:breaking` passes without a `!`.
- **No amendment of `portal:D7`.** The slices are read as runtime children of the Service, as
  ReplicaSets are children of a Deployment: counted only when their controller owner reference
  reaches an inventory Service, by the rule `reachInventory` already applies. design.md,
  "D7: slices as runtime children", gives the reason and the one assumption it rests on, which
  section 1 checks on a live cluster before any code. `docs/DESIGN.md` D11 is edited in place to
  record the new rule and the owner answer; that edit adds a requirement and changes no policy.

Section 1 is a spike. It reads a live EndpointSlice of a fixture Service and writes the finding
into design.md. If the controller does not set the owner reference the design relies on, the
change stops there and goes back to the supervisor, because the slices would then not be runtime
children and `portal:D7` would need an amendment the owner decides.

## Principle V

- **Read-only holds.** The only new verb is `list` on `endpointslices`. The portal creates, updates,
  patches and deletes nothing, and the review APIs it creates (`selfsubjectaccessreviews` locally)
  are the same ones as before.
- **No Secret data.** EndpointSlices hold addresses, ports and Pod references, not Secret data.
  The portal reduces a slice to one integer at the read boundary and keeps neither the slice nor
  any field of it (portal:D8).
- **Acts as the user, fails closed.** The caller's review runs first, per caller, on every read
  that serves the count. A denied, failed or timed-out review serves no count. The portal's own
  read is preceded by its reader review, as for every runtime child. An empty identity is denied
  before either (portal:D5, portal:D7).
- **A degraded read is not shown as empty.** Principle IV says a missing permission is "shown as
  such, never as empty". The panel does not show `0 ready` for a read it could not make: it shows
  no Endpoints line, which is not a claim of zero. design.md, "Absent, not locked", gives the
  reason: for a caller without the review, a distinct "locked" mark would itself tell that
  slices exist in that namespace.

## The deploy/ widening (SR10)

Run from `deploy/`, the portal acts as its own ServiceAccount for whoever holds the launch token
(portal:D13). The new rule therefore lets every launch-token holder see the ready count of every
inventory Service in every namespace, whatever their own RBAC says about EndpointSlices. The
caller's review still runs, but in this mode it asks as the ServiceAccount and so passes. This is
a reviewed widening of what a token holder sees, limited to one integer for each Service that an
OPM inventory reaches. The README and the portal-security page say so in the same change. In local
mode on a user's machine the rule does not exist: the user's kubeconfig RBAC is the boundary.

## Threat statement

*From `agent-swarm/opm-portal-v1/design/T2.4.md` section 8, "G14, EndpointSlices", in the
repository's words.*

- **New data and boundary.** The ready count of the EndpointSlices of a Service crosses from the
  cluster API into the panel. The kind `endpointslices` in group `discovery.k8s.io` is new to the
  ClusterRole.
- **Who writes it.** The EndpointSlice controller of the control plane, from Pod readiness; and
  anyone with `create` or `update` on `endpointslices` in that namespace. The panel therefore shows
  the count as the cluster's record, not as proof of health.
- **Who reads it.** A caller who passes the review for `list` on `endpointslices` in the Service's
  namespace. In `deploy/` mode, every launch-token holder, because the rule is in the role.
- **Threat statement.** A caller who may read Services but not EndpointSlices can see endpoint
  readiness if the portal reuses the Service's review, which leads to disclosure of workload
  topology, negatively impacting namespace isolation. Response: mitigate, by a separate review on
  `discovery.k8s.io` `endpointslices`, reads through the runtime-children rule, and a ready count
  only. Test: a new authorization case (tasks 3.4 and 4.4).
- **How `portal:D5`, `portal:D7` and `portal:D8` stay true.** D5: the review names the verb,
  resource and namespace of the slices. D7: the slices are runtime children of an inventory
  Service, and no request can name a slice. D8: no names, no addresses, a number only.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `read-authorization`: adds the rule that a Service's EndpointSlices are read under their own
  review, never the Service's.
- `read-model`: adds the ready count of an inventory Service, read as runtime children and
  reduced to a number.
- `read-api`: adds `endpointsReady` to the Service's graph node.
- `web-ui`: adds the Endpoints fact to the Service panel, in the fixed list of facts.
- `deploy-manifest`: adds `list` on `endpointslices` to what the role grants, and holds it to that.

## Impact

- **Packages:** `internal/readmodel` (the review, the read, the count, the cache),
  `internal/graph` (one field), `api/v1alpha1` and `openapi/v1alpha1.yaml` (one field),
  `internal/api` (the conversion, goldens), `internal/ui` (one fact), `deploy/` (one rule and its
  test), `test/browser` (one panel check), `docs/site`, `README.md`, `docs/DESIGN.md`,
  `ROADMAP.md`.
- **API resources:** the instance and package graph documents gain one optional field. No new path,
  no new error code, no new stream topic.
- **UI pages:** the details panel of a Service node, on the Graph and Resources tabs.
- **SemVer, as after 1.0:** MINOR (an additive field and a new fact). On the 0.x line it ships as
  `feat`, which releases, and carries no `!`.
- **Complexity (Principle VII):** one read path, no new dependency, no new background watch. The
  cost is one label-selected list per inventory Service per ten seconds (design.md, "Reading").
- **Not in this change:** a watch on EndpointSlices, a per-endpoint view, the Endpoints count on
  the Resources tab or the hover card, and any readiness for another kind.
