# opm-portal

A Go web portal over OPM clusters. It serves a versioned, read-only JSON/SSE API over what the OPM
controller records in a cluster, and an HTMX UI as the API's first consumer.

## Language

### OPM kinds as the portal reads them

**ModuleInstance**:
The cluster object the controller reconciles from one module; the portal's main subject, listed and
shown with its status and inventory. It is never the `#ModuleInstance` schema definition.
_Avoid_: deployment, app, release

**ModulePackage**:
The cluster object that reads a module package from a Flux source and reconciles it; shown beside
ModuleInstance with the same status and inventory, but records no render contracts.
_Avoid_: bundle, chart

**Module**:
The module a ModuleInstance was rendered from, shown as a reference. The portal never loads or
renders a module.
_Avoid_: app, `#Module` (the schema definition, not what the portal handles)

**Component**:
One group of objects in an inventory, named as the controller recorded it. The portal adds a health
value to it and nothing else.
_Avoid_: service, workload, `#Component` (the schema definition)

**Platform**:
The cluster object that holds the subscriptions, the resolved registry and the generated platform;
the root of the platform graph.
_Avoid_: cluster, environment, `#Platform` (the schema definition)

**Catalog**:
One entry of the Platform's resolved registry, with its version and whether it is enabled.
_Avoid_: repository, registry (the whole resolved set)

**Subscription**:
One catalog that the Platform's spec subscribes to.
_Avoid_: dependency, import

### What the controller recorded

**Inventory**:
The list of objects the controller recorded that a ModuleInstance or ModulePackage owns, grouped by
Component. Every graph node below an instance comes from it.
_Avoid_: resources, manifest, rendered output

**Render contracts**:
The contracts an instance's last successful render used, from `status.requiredContracts`. Shown
under the label "Uses"; never an edge and never the instance's provider demand.
_Avoid_: requires, dependencies, demand

**Condition**:
A status condition as the controller wrote it, with its message kept as written.
_Avoid_: status (alone), state

**History**:
The controller's status history entries of an instance or package; with Conditions it is the
record, while events are only a feed.
_Avoid_: audit log, changelog

**Event feed**:
Recent Kubernetes events about one object, folded so a repeat shows once with a count. It expires
and is never state.
_Avoid_: log, history

### Status and health

**Applied**:
The controller's `Ready=True`: every apply succeeded. It says nothing about the workload and is never
labelled healthy.
_Avoid_: healthy, ready, deployed, running

**Reconcile state**:
The controller's reported state for an object: Applied, Reconciling, Failed, Stalled, Suspended,
ManagedExternally or Unknown.
_Avoid_: status, phase

**Health**:
The portal's own workload health, computed from live objects and rolled up worst-of through
components to the instance. Unreadable objects are excluded and the result is marked partial.
_Avoid_: readiness, status, Applied

**Pod rule**:
The one rule beyond kstatus: a container waiting with an image-pull, crash-loop, container-config
or invalid-image reason marks the Pod's owning workload Degraded.
_Avoid_: pod check, probe

**Managed externally**:
The reconcile state of a CLI-owned instance, shown in a neutral style. Its health is still computed
from the inventory the CLI wrote.
_Avoid_: unmanaged, orphaned

**Tone**:
How a condition reads, given its type and status: normal, abnormal, progressing, informational or
unknown.
_Avoid_: severity, level, color

### Providers and registrations

**TransformerRegistration**:
The cluster-scoped object that claims a catalog's transformers and contracts. The portal shows
its standing as the controller wrote it and never computes it.
_Avoid_: `#ComponentTransformer` (the schema definition), plugin, extension

**Provider**:
Whoever holds a TransformerRegistration in its inventory, whether a ModuleInstance or a
ModulePackage. Found by inventory, not by kind.
_Avoid_: owner, source of the registration, producer

**Provider claim**:
One TransformerRegistration that an instance's or package's inventory holds, with the controller's
verdict on it.
_Avoid_: registration (alone), provides

**Verdict**:
The controller's judgement of a registration: Accepted, Refused, Pending, RemovalBlocked or
Unknown. Acceptance and activation are read from `status.accepted` and `status.active`, never
inferred from `Stalled` and `Ready`.
_Avoid_: result, outcome

### Sources and runtime

**Source**:
The Flux object a ModulePackage's `spec.sourceRef` names, read as the caller; its Ready condition,
artifact revision and presence are shown.
_Avoid_: repo, origin, artifact

**Runtime child**:
A ReplicaSet, Pod or Job found below an inventory workload by ownerReferences.
_Avoid_: replica, sub-resource

### Graphs

**Graph**:
A laid-out set of nodes and edges for one instance, one package or the Platform, derived only from
recorded state.
_Avoid_: diagram, topology, dependency tree

**Edge source**:
The field of one Kubernetes object that an edge was drawn from. Each edge kind has exactly one.
_Avoid_: link reason, relation

**Group node**:
One node standing in for a set of nodes: configuration of one kind family, many Pods, or nodes past
the cap.
_Avoid_: cluster, folder, collapsed node

### Access

**Local mode** / **In-cluster mode**:
Where the portal runs. Local mode reads with the user's kubeconfig; in-cluster mode, a future plan,
authorizes each read for the signed-in user, then reads with the portal's own ServiceAccount.
_Avoid_: dev mode, standalone, server mode

**Identity**:
The principal a read is authorized for: the kubeconfig's user locally, the signed-in person in
cluster. The empty identity is refused.
_Avoid_: user (alone), account, token

**Grant**:
Proof that the authorizer allowed one identity one read for a limited time. Every read path
checks it before it looks anything up.
_Avoid_: permission, ticket, role

**Launch token**:
The one-time secret in local mode's start link, exchanged for a session cookie. It is kept only as
a digest and never logged.
_Avoid_: login token, API key, password

**Tier**:
A level of the read model's cache, by what is watched and when: the four OPM kinds, inventory
kinds, runtime children, per-object events.
_Avoid_: layer, cache level
