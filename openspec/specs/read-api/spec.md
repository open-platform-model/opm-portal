# read-api Specification

## Purpose
Defines the portal's versioned read API, `/api/v1alpha1`: which resources it serves, the
documents and errors it returns, the order in which it authorizes and looks up, what it never
serves, the documents its change stream carries, and the OpenAPI contract that holds it stable.

## Requirements

### Requirement: Resources live under a versioned, cluster-parameterized path

The portal SHALL serve every read API resource under `/api/v1alpha1/clusters/{cluster}/`, with
`GET` only: `instances`, `instances/{namespace}/{name}`, `instances/{namespace}/{name}/graph`,
`instances/{namespace}/{name}/events`, `packages`, `packages/{namespace}/{name}`,
`packages/{namespace}/{name}/graph`, `packages/{namespace}/{name}/events`, `platform`,
`platform/graph`, `platform/events`, `platform/registrations/{name}/events` and `stream`. In
milestone 1 the only cluster served SHALL be `default`. Any other cluster name SHALL be answered
`404` with code `not_found` before any authorization review. A method other than `GET` SHALL be
answered `405` with code `method_not_allowed`. A namespace that is not a DNS-1123 label, or a
name that is not a DNS-1123 subdomain, SHALL be answered `400` with code `bad_request` before any
authorization review. Source: 0030:D2:R2.

#### Scenario: The default cluster

- **WHEN** a client sends `GET /api/v1alpha1/clusters/default/instances`
- **THEN** the response is an `InstanceList` document

#### Scenario: Another cluster

- **WHEN** a client sends `GET /api/v1alpha1/clusters/prod/instances`
- **THEN** the response is `404` with code `not_found`
- **AND** no authorization review is sent

#### Scenario: A write method

- **WHEN** a client sends `POST /api/v1alpha1/clusters/default/instances`
- **THEN** the response is `405` with code `method_not_allowed`

#### Scenario: A malformed name

- **WHEN** a client sends `GET /api/v1alpha1/clusters/default/instances/Apps/blog`
- **THEN** the response is `400` with code `bad_request`
- **AND** no authorization review is sent

### Requirement: Documents are portal-shaped and versioned

Every document SHALL carry `apiVersion: portal.opmodel.dev/v1alpha1` and a `kind`. No document
SHALL embed a raw custom-resource status. An instance and a package SHALL carry the operator's
applied state (`reconcile`: state, reason, message, since) and the portal's workload health
(`health`: state, per-state and per-access counts, `partial`, `evaluatedAt`, `live`) as two
separate blocks, never one merged status. Enumerated values SHALL be documented as open: a
client is told to treat an unknown value as unknown and to ignore unknown fields. Source:
0030:D2:R3/R4, 0030:D3:R1.

#### Scenario: Both axes on an applied, degraded instance

- **WHEN** a client reads an instance the operator applied whose Pod waits on `ErrImagePull`
- **THEN** its `reconcile.state` is `Applied` and its `health.state` is `Degraded`

#### Scenario: A CLI-owned instance

- **WHEN** a client reads instance `web/web`, which the CLI applied
- **THEN** its `owner` is `cli`, its `reconcile.state` is `ManagedExternally`, and its `health`
  is computed from its inventory

### Requirement: Every request is authorized before anything is looked up

Every resource SHALL authorize each Kubernetes read it is about to serve, for the caller's
identity, before it looks anything up. A caller who may not make one of those reads SHALL
receive `403` with code `forbidden` and a detail that names neither the object nor the check
that failed, identical whether or not the object exists. A caller who may make them and asks for
an object that does not exist SHALL receive `404` with code `not_found`. Source: 0030:D7:R1.

#### Scenario: Forbidden, existing and missing alike

- **WHEN** a caller who may not get ModuleInstances in namespace `default` reads
  `instances/default/podinfo`, which exists, and `instances/default/nothing`, which does not
- **THEN** both responses are byte-for-byte the same `403` problem document with code `forbidden`

#### Scenario: Allowed and missing

- **WHEN** a caller who may get ModuleInstances in namespace `default` reads
  `instances/default/nothing`
- **THEN** the response is `404` with code `not_found`

### Requirement: Lists hold only what the caller may list

A list resource SHALL authorize `list` on its kind in its scope: cluster-wide, or the namespace a
`namespace` query parameter names. When that is denied it SHALL answer `200` with an empty
`items` and `access: forbidden`, and no count or name of a hidden item. Source: 0030:D7:R2,
0030:D5:R5.

#### Scenario: A namespace-scoped list

- **WHEN** a caller who may list ModuleInstances only in namespace `default` reads
  `instances?namespace=default`
- **THEN** the list holds the instances in `default` and its `access` is `ok`

#### Scenario: No list access

- **WHEN** the same caller reads `instances` cluster-wide
- **THEN** the response is `200` with no items and `access: forbidden`

### Requirement: Partial access is reported per item

Inside a readable document, an inventory object the caller may not read SHALL be marked
`forbidden` and one the portal could not read `notReadable`, and neither SHALL fail the request.
When the caller may not list TransformerRegistrations, the platform document SHALL say
`registrationsAccess: forbidden` rather than show none. Source: 0030:D7:R3, 0030:D11:R5.

#### Scenario: A forbidden kind inside an instance

- **WHEN** a caller who may read instance `cert-manager/cert-manager` but not ClusterRoles reads it
- **THEN** each ClusterRole in its inventory carries `access: forbidden` and no health
- **AND** the instance's `health.partial` is true

### Requirement: Errors are problem documents

Every error SHALL be an RFC 9457 `application/problem+json` document with `type`, `title`,
`status`, `detail` and a `code`. The codes SHALL be `unauthenticated` (401), `forbidden` (403),
`not_found` (404), `bad_request` (400), `method_not_allowed` (405), `too_many_streams` (429),
`not_readable_by_portal` (503: the portal holds no readable copy of the kind) and
`upstream_unavailable` (503: an authorization review or the API server failed). The set SHALL be
documented as open. No problem document SHALL carry a credential, an identity or a review's
error text. Source: 0030:D2:R6.

#### Scenario: Authorization unavailable

- **WHEN** the authorization review for a request fails
- **THEN** the response is `503` with code `upstream_unavailable`, and nothing is served

### Requirement: Requests carry an authenticated principal

Every request SHALL be resolved to a principal (an identity and a session) by the mode that
serves the API before any handler runs. A request with no principal, or whose identity has an
empty or anonymous username, SHALL be answered `401` with code `unauthenticated` and SHALL cause
no authorization review and no read. Source: 0030:D6:R2.

#### Scenario: Empty identity

- **WHEN** a request resolves to an identity with an empty username
- **THEN** the response is `401` with code `unauthenticated`
- **AND** no authorization review is sent

### Requirement: Values, Secrets and the apply annotation are never served

No document SHALL contain an instance's or package's `spec.values`, Secret data, or the
`kubectl.kubernetes.io/last-applied-configuration` annotation. Condition, history and event
messages SHALL be served as the operator and the API server wrote them. Source: 0030:D8:R2/R3.

#### Scenario: No values in any document

- **WHEN** every resource is read for every F1 instance, package and the platform
- **THEN** no response contains a `values` field or the last-applied annotation

### Requirement: Events are served only about objects an inventory reaches

An events resource SHALL serve the folded recent-activity feed about its owner (the instance,
the package, the Platform, or one TransformerRegistration) or, for an instance or package, about
one object named by the `group`, `kind`, `namespace` and `name` query parameters. It SHALL
authorize get on the owner, get on the named object, and list of events in the namespace the
events live in, before any lookup; the named object's kind SHALL be resolved only after get on
the owner is allowed, from the discovery the portal already holds, and a request SHALL never
refresh that discovery. An object that is neither the owner, nor in its inventory,
nor a runtime child below an inventory object SHALL be refused with the same `403` document as a
forbidden read. Lines SHALL carry type, reason, note, reporting controller, the object regarded,
a count and the latest occurrence time. Source: 0030:D7:R4, 0030:D9:R3/R4.

#### Scenario: An inventory object's events

- **WHEN** a caller reads `instances/default/podinfo/events` for Deployment `default/podinfo-podinfo`
- **THEN** the feed holds its `ScalingReplicaSet` event

#### Scenario: An object no inventory reaches

- **WHEN** a caller with every permission reads `instances/cert-manager/cert-manager/events` for
  Lease `cert-manager/cert-manager-controller`
- **THEN** the response is the same `403` document a forbidden read gets

#### Scenario: A kind the cluster does not serve

- **WHEN** any caller names an object of a kind discovery does not know
- **THEN** the response is the same `403` document a forbidden read gets, and no discovery
  request is sent

#### Scenario: A registration's events

- **WHEN** a caller reads `platform/registrations/default.refused-claim-fixture/events`
- **THEN** the feed holds its `CatalogUnresolved` warning, read from namespace `default`

### Requirement: Graphs are served as documents

The graph resources SHALL serve the instance, package and platform graphs as `Graph` documents:
nodes with stable ids, kind, label, access, health, applied state and position; edges with kind,
the field they were drawn from, verification and route; and the layout's size and columns. The
`expand` query parameter, repeatable, SHALL expand the group node it names, and
`showScaledDown=true` SHALL show ReplicaSets scaled to zero. The platform graph SHALL authorize
get on each registration's provider instance for the caller and show a provider the caller may
not read as such. Source: 0030:D4.

#### Scenario: The platform graph with a forbidden provider

- **WHEN** a caller who may read the Platform and the registrations, but not ModuleInstances in
  namespace `default`, reads `platform/graph`
- **THEN** the provider node of `default.backup-provider` carries `access: forbidden` and its
  `providedBy` edge is unverified

### Requirement: The platform document names claimants and the contributing registration

Each catalog in the platform document SHALL list as `claimants` the readable registrations that
claim it, and SHALL name as `contributedBy` the one accepted and active registration the
registry's contribution is joined to, by the same rule the platform graph draws its
`contributes` edge. Source: 0030:D4:R4.

#### Scenario: The accepted claim contributed the backup catalog

- **WHEN** a caller reads the F1 platform
- **THEN** catalog `testing.opmodel.dev/catalogs/operator/backup@v0` has `contributedBy`
  `default.backup-provider`

### Requirement: The change stream carries the GET documents

The `stream` resource SHALL open a server-sent-events stream on the topics named by its `topics`
parameter, for the request's principal and session. Each topic SHALL carry exactly one document,
the one its `GET` returns: `platform` the `Platform`, `instances` and `instances:<ns>` the
`InstanceList`, `instance:` the `Instance`, `package:` the `Package`, and `events:` the
`EventList` of its object. Snapshot and upsert messages SHALL carry that document rendered for
the subscriber. When the object is deleted, or does not exist at the snapshot, the topic SHALL
carry a `Removed` document naming it. A change to an OPM object, to an inventory object or to a
runtime child below one SHALL reach the topics showing it. Topics the broker does not serve
SHALL be refused with `400` and code `bad_request`; a stream beyond the session's cap with `429`
and code `too_many_streams`. Source: 0030:D2:R5.

#### Scenario: An instance topic follows a health change

- **WHEN** a client follows `instance:default/podinfo` and one of its Pods starts waiting on
  `ErrImagePull`
- **THEN** the stream delivers an upsert whose `Instance` document has `health.state` `Degraded`

#### Scenario: A deleted instance

- **WHEN** a client follows `instance:default/podinfo` and the instance is deleted
- **THEN** the stream delivers a delete carrying a `Removed` document naming it, on the
  instance topic and on its events topic, once: later refreshes do not repeat it

#### Scenario: Deleted and recreated before the next publish

- **WHEN** a followed instance is deleted and created again within one coalescing window
- **THEN** the stream delivers the instance as it now is, not a `Removed` document

### Requirement: The OpenAPI document is the contract

The repository SHALL hold an OpenAPI 3.1 document of the read API. Every served route SHALL
appear in it with the document its success response returns, and every wire type SHALL match its
schema by field names and types; a test SHALL fail otherwise. A pull request that changes the
document in a way `oasdiff breaking` reports as an error SHALL fail the required `Lint` check
unless its title marks a breaking change with `!`. Because `oasdiff` ignores
`x-extensible-enum`, the same check SHALL fail when a value the base document lists for an
enumerated field is missing from the head, and a test SHALL fail when an enumerated field's
values differ from the Go constants the server writes. Source: 0030:D2:R2.

#### Scenario: A field renamed without a breaking title

- **WHEN** a pull request titled `feat(api): rename a field` removes a response property
- **THEN** the `Lint` check fails and names the breaking change

#### Scenario: An enumerated value removed

- **WHEN** a pull request titled `feat(api): x` removes `ManagedExternally` from
  `Reconcile.state`
- **THEN** the `Lint` check fails and names the removed value

#### Scenario: A field added

- **WHEN** a pull request adds an optional response property
- **THEN** the `Lint` check's API gate passes

### Requirement: The change stream serves the topic kinds the mode routes to it

The read API SHALL serve the topics of its own documents itself and SHALL route any other topic
kind to the producer the serving mode configures for it, on the same stream and under the same
authorization. Local mode SHALL route `log:` topics to the pod-log producer, so a client follows a
container's log on the `stream` resource like any other topic. A mode that configures a producer
for a kind the read API serves itself SHALL fail to start. Source: 0030:D10:R2.

#### Scenario: A log topic on the read API's stream

- **WHEN** a local-mode client allowed `get pods/log` opens the `stream` resource with topic
  `log:default/<pod>/podinfo` for a Pod of the `podinfo` instance
- **THEN** the topic attaches and delivers a snapshot, then `log` messages

#### Scenario: A log topic without a log producer

- **WHEN** the read API runs with no producer routed for `log:` and a client asks for a log topic
- **THEN** the request is refused with `400` and code `bad_request`

#### Scenario: A producer that claims an API kind

- **WHEN** a mode routes `instance:` topics to a producer of its own
- **THEN** the read API refuses to start
