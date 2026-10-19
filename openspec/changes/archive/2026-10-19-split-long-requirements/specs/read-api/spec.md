## REMOVED Requirements

### Requirement: Resources live under a versioned, cluster-parameterized path
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Instance resources live under the versioned cluster path", "Package resources live under the versioned cluster path", "The cluster, platform and stream resources live under the versioned cluster path", "Every read API resource is GET only except the stream topics resource", "Only the cluster default is served in milestone 1", "A method a resource does not take is answered 405", "A malformed namespace or name is answered 400 before any review".

### Requirement: Documents are portal-shaped and versioned
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Every document carries apiVersion and kind and embeds no raw status", "An instance and a package carry applied state and health as two blocks", "Enumerated values are documented as open".

### Requirement: Errors are problem documents
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Every error is a problem document with a code", "The error codes of a problem document are a documented open set", "No problem document carries a credential, an identity or a review's error text".

### Requirement: Events are served only about objects an inventory reaches
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "An events resource serves the feed about its owner or one named object", "An events resource authorizes the owner, the object and the events before any lookup", "The named object's kind is resolved after get on the owner, from the held discovery", "An object no inventory reaches is refused with the forbidden document", "Event lines carry type, reason, note, controller, object, count and time".

### Requirement: Graphs are served as documents
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Graph resources serve Graph documents with nodes, edges and layout", "The expand and showScaledDown parameters change the graph's nodes", "The platform graph authorizes each registration's provider instance".

### Requirement: The change stream carries the GET documents
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The stream resource opens an event stream on the named topics", "Each topic carries the document its GET returns, rendered for the subscriber", "A deleted or missing object's topic carries a Removed document", "A change to an OPM object, an inventory object or a runtime child reaches the topics showing it", "A topic the broker does not serve is refused and a stream beyond the cap gets 429".

### Requirement: The OpenAPI document is the contract
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The OpenAPI document lists every route and matches the wire types", "A breaking change to the OpenAPI document fails Lint unless the title marks it", "A removed enumerated value fails Lint and a test compares values with the Go constants".

### Requirement: A stream's topics change through one request
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "A topics request attaches the added topics, detaches the removed ones and answers 204", "A topics request of another type or shape or with an unparsable topic is answered 400", "A topics request for a stream that is not the session's open stream is answered 404", "A topics request without the session is answered 401", "A topics request reads and creates nothing in the cluster", "In local mode a cross-origin topics request is refused by the front door".

### Requirement: One object an inventory reaches is served stripped
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "An object resource serves an Object document with the object's reference", "A served object lacks managed fields, the apply annotation and values", "The object reads are authorized on the owner and then on the object before any lookup", "A kind the cluster does not serve and an object no inventory reaches are refused", "A core Secret is refused before any review about it".

### Requirement: Conditions carry their tone and the portal's explanation
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Every condition carries a tone", "A condition's tone follows its type and status", "A condition carries its meaning and next step only for a reason the portal explains".

### Requirement: In-cluster mode serves no operator text
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "In-cluster mode serves no condition, reconcile, history or registration message", "In-cluster mode serves no operator-reported event note", "In-cluster mode serves no health message of an opmodel.dev object or status message in an Object", "In-cluster mode keeps every reason, state, tone, meaning, next step, outcome and providerOf standing", "In-cluster mode serves text other writers wrote as written", "In-cluster mode serves events left alike once their notes are dropped as one line", "In-cluster mode fails a document type the omission does not know", "The OpenAPI document says which fields are absent in-cluster".

### Requirement: The cluster document names the connection
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The Cluster document names the cluster, mode, source, context and caller", "The Cluster document carries the API server version when the read model holds it", "The Cluster document carries no server URL or credential and makes no cluster read", "A Cluster request without a principal is answered 401", "The Cluster document is served unchanged in-cluster".

### Requirement: Packages carry their interval and source artifact
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "A package carries its interval and source artifact as recorded", "A package's source artifact fetch URL is not served", "A package document carries its serviceAccountName when set".

### Requirement: Registrations carry their conditions and holders
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "A registration carries its conditions with tone, meaning and next step", "A registration carries the instances and packages that hold it", "heldByPartial is true whenever the caller cannot see every holder", "A registration's provider keeps naming the ModuleInstance its providerRef names".

### Requirement: Instances and packages name the registrations they hold
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "An instance or package carries one providerOf entry per registration it holds", "providerRefMatches is true only for a ModuleInstance the providerRef names", "A registration the cluster does not hold carries verdict Unknown", "A registration the caller may not read carries its name and access only".

### Requirement: Joined fields follow changes on both sides
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "A change to a registration reaches the topics of the owners that hold it", "A change to a holder reaches the platform topic".

## ADDED Requirements

### Requirement: Instance resources live under the versioned cluster path
The portal SHALL serve the instance resources `instances`, `instances/{namespace}/{name}`,
`instances/{namespace}/{name}/graph`, `instances/{namespace}/{name}/events` and
`instances/{namespace}/{name}/object` under `/api/v1alpha1/clusters/{cluster}`, with `GET` only.
Source: portal:D2:R2.

#### Scenario: The default cluster

- **WHEN** a client sends `GET /api/v1alpha1/clusters/default/instances`
- **THEN** the response is an `InstanceList` document

### Requirement: Package resources live under the versioned cluster path
The portal SHALL serve the package resources `packages`, `packages/{namespace}/{name}`,
`packages/{namespace}/{name}/graph`, `packages/{namespace}/{name}/events` and
`packages/{namespace}/{name}/object` under `/api/v1alpha1/clusters/{cluster}`, with `GET` only.
Source: portal:D2:R2.

#### Scenario: The package list

- **WHEN** a client sends `GET /api/v1alpha1/clusters/default/packages`
- **THEN** the response is a document of the package resource `packages`

### Requirement: The cluster, platform and stream resources live under the versioned cluster path
The portal SHALL serve the cluster itself, `platform`, `platform/graph`, `platform/events`,
`platform/registrations/{name}/events` and `stream` under `/api/v1alpha1/clusters/{cluster}`, with
`GET` only. Source: portal:D2:R2.

#### Scenario: The cluster itself

- **WHEN** a client sends `GET /api/v1alpha1/clusters/default`
- **THEN** the response is a `Cluster` document

### Requirement: Every read API resource is GET only except the stream topics resource
Every read API resource SHALL be served under `/api/v1alpha1/clusters/{cluster}` with `GET` only.
The one exception SHALL be `stream/{stream}/topics`, which takes `POST` only. Source: portal:D2:R2.

#### Scenario: The topics resource

- **WHEN** a client sends a request to `stream/{stream}/topics`
- **THEN** the resource takes `POST` only

### Requirement: Only the cluster default is served in milestone 1
In milestone 1 the only cluster served SHALL be `default`. Any other cluster name SHALL be answered
`404` with code `not_found` before any authorization review. Source: portal:D2:R2.

#### Scenario: Another cluster

- **WHEN** a client sends `GET /api/v1alpha1/clusters/prod/instances`
- **THEN** the response is `404` with code `not_found`
- **AND** no authorization review is sent

### Requirement: A method a resource does not take is answered 405
A method a resource does not take SHALL be answered `405` with code `method_not_allowed`. Source:
portal:D2:R2.

#### Scenario: A write method

- **WHEN** a client sends `POST /api/v1alpha1/clusters/default/instances`
- **THEN** the response is `405` with code `method_not_allowed`

### Requirement: A malformed namespace or name is answered 400 before any review
A namespace that is not a DNS-1123 label, or a name that is not a DNS-1123 subdomain, SHALL be
answered `400` with code `bad_request` before any authorization review. Source: portal:D2:R2.

#### Scenario: A malformed name

- **WHEN** a client sends `GET /api/v1alpha1/clusters/default/instances/Apps/blog`
- **THEN** the response is `400` with code `bad_request`
- **AND** no authorization review is sent

### Requirement: Every document carries apiVersion and kind and embeds no raw status
Every document SHALL carry `apiVersion: portal.opmodel.dev/v1alpha1` and a `kind`. No document SHALL
embed a raw custom-resource status. Source: portal:D2:R3/R4, portal:D3:R1.

#### Scenario: A document's identity

- **WHEN** a client reads any document
- **THEN** it carries `apiVersion: portal.opmodel.dev/v1alpha1` and a `kind`, and embeds no raw custom-resource status

### Requirement: An instance and a package carry applied state and health as two blocks
An instance and a package SHALL carry the operator's applied state (`reconcile`: state, reason,
message, since) and the portal's workload health (`health`: state, per-state and per-access counts,
`partial`, `evaluatedAt`, `live`) as two separate blocks, never one merged status. Source:
portal:D2:R3/R4, portal:D3:R1.

#### Scenario: Both axes on an applied, degraded instance

- **WHEN** a client reads an instance the operator applied whose Pod waits on `ErrImagePull`
- **THEN** its `reconcile.state` is `Applied` and its `health.state` is `Degraded`

#### Scenario: A CLI-owned instance

- **WHEN** a client reads instance `web/web`, which the CLI applied
- **THEN** its `owner` is `cli`, its `reconcile.state` is `ManagedExternally`, and its `health`
  is computed from its inventory

### Requirement: Enumerated values are documented as open
Enumerated values SHALL be documented as open: a client is told to treat an unknown value as unknown
and to ignore unknown fields. Source: portal:D2:R3/R4, portal:D3:R1.

#### Scenario: An unknown value

- **WHEN** a client reads a document with an enumerated value it does not know
- **THEN** the documentation tells it to treat the value as unknown and to ignore unknown fields

### Requirement: Every error is a problem document with a code
Every error SHALL be an RFC 9457 `application/problem+json` document with `type`, `title`, `status`,
`detail` and a `code`. Source: portal:D2:R6.

#### Scenario: A problem document

- **WHEN** the read API answers with an error
- **THEN** the answer is an RFC 9457 `application/problem+json` document with `type`, `title`, `status`, `detail` and a `code`

### Requirement: The error codes of a problem document are a documented open set
The `code` of a problem document SHALL be `unauthenticated` (401), `forbidden` (403), `not_found`
(404), `bad_request` (400), `method_not_allowed` (405), `too_many_streams` (429),
`not_readable_by_portal` (503: the portal holds no readable copy of the kind) or
`upstream_unavailable` (503: an authorization review or the API server failed). The set SHALL be
documented as open. Source: portal:D2:R6.

#### Scenario: Authorization unavailable

- **WHEN** the authorization review for a request fails
- **THEN** the response is `503` with code `upstream_unavailable`, and nothing is served

### Requirement: No problem document carries a credential, an identity or a review's error text
No problem document SHALL carry a credential, an identity or a review's error text. Source:
portal:D2:R6.

#### Scenario: A review fails

- **WHEN** the authorization review for a request fails
- **THEN** the problem document carries no credential, no identity and no review's error text

### Requirement: An events resource serves the feed about its owner or one named object
An events resource SHALL serve the folded recent-activity feed about its owner (the instance, the
package, the Platform, or one TransformerRegistration) or, for an instance or package, about one
object named by the `group`, `kind`, `namespace` and `name` query parameters. Source: portal:D7:R4,
portal:D9:R3/R4.

#### Scenario: An inventory object's events

- **WHEN** a caller reads `instances/default/podinfo/events` for Deployment `default/podinfo-podinfo`
- **THEN** the feed holds its `ScalingReplicaSet` event

#### Scenario: A registration's events

- **WHEN** a caller reads `platform/registrations/default.refused-claim-fixture/events`
- **THEN** the feed holds its `CatalogUnresolved` warning, read from namespace `default`

### Requirement: An events resource authorizes the owner, the object and the events before any lookup
An events resource SHALL authorize get on the owner, get on the named object, and list of events in
the namespace the events live in, before any lookup. Source: portal:D7:R4, portal:D9:R3/R4.

#### Scenario: Three checks before the lookup

- **WHEN** a caller reads an events resource for an owner and a named object
- **THEN** get on the owner, get on the named object, and list of events in the namespace the events live in are authorized before any lookup

### Requirement: The named object's kind is resolved after get on the owner, from the held discovery
The kind of the object an events resource names SHALL be resolved only after get on the owner is
allowed, from the discovery the portal already holds, and a request SHALL never refresh that
discovery. Source: portal:D7:R4, portal:D9:R3/R4.

#### Scenario: A kind the cluster does not serve

- **WHEN** any caller names an object of a kind discovery does not know
- **THEN** the response is the same `403` document a forbidden read gets, and no discovery
  request is sent

### Requirement: An object no inventory reaches is refused with the forbidden document
An object that is neither the owner, nor in its inventory, nor a runtime child below an inventory
object SHALL be refused by an events resource with the same `403` document as a forbidden read.
Source: portal:D7:R4, portal:D9:R3/R4.

#### Scenario: An object no inventory reaches

- **WHEN** a caller with every permission reads `instances/cert-manager/cert-manager/events` for
  Lease `cert-manager/cert-manager-controller`
- **THEN** the response is the same `403` document a forbidden read gets

### Requirement: Event lines carry type, reason, note, controller, object, count and time
The lines of an events resource SHALL carry type, reason, note, reporting controller, the object
regarded, a count and the latest occurrence time. Source: portal:D7:R4, portal:D9:R3/R4.

#### Scenario: An event line

- **WHEN** a caller reads an events resource
- **THEN** each line carries type, reason, note, reporting controller, the object regarded, a count and the latest occurrence time

### Requirement: Graph resources serve Graph documents with nodes, edges and layout
The graph resources SHALL serve the instance, package and platform graphs as `Graph` documents:
nodes with stable ids, kind, label, access, health, applied state and position; edges with kind, the
field they were drawn from, verification and route; and the layout's size and columns. Source:
portal:D4.

#### Scenario: A Graph document

- **WHEN** a client reads a graph resource
- **THEN** the response is a `Graph` document with nodes (stable ids, kind, label, access, health, applied state and position), edges (kind, the field they were drawn from, verification and route) and the layout's size and columns

### Requirement: The expand and showScaledDown parameters change the graph's nodes
The `expand` query parameter of a graph resource, repeatable, SHALL expand the group node it names,
and `showScaledDown=true` SHALL show ReplicaSets scaled to zero. Source: portal:D4.

#### Scenario: Expanded and scaled-down nodes

- **WHEN** a client reads a graph resource with `expand` naming a group node, or with `showScaledDown=true`
- **THEN** the group node it names is expanded, and ReplicaSets scaled to zero are shown

### Requirement: The platform graph authorizes each registration's provider instance
The platform graph SHALL authorize get on each registration's provider instance for the caller and
show a provider the caller may not read as such. Source: portal:D4.

#### Scenario: The platform graph with a forbidden provider

- **WHEN** a caller who may read the Platform and the registrations, but not ModuleInstances in
  namespace `default`, reads `platform/graph`
- **THEN** the provider node of `default.backup-provider` carries `access: forbidden` and its
  `providedBy` edge is unverified

### Requirement: The stream resource opens an event stream on the named topics
The `stream` resource SHALL open a server-sent-events stream on the topics named by its `topics`
parameter, for the request's principal and session. Source: portal:D2:R5.

#### Scenario: An opened stream

- **WHEN** a client opens the `stream` resource with a `topics` parameter
- **THEN** a server-sent-events stream opens on the topics it names, for the request's principal and session

### Requirement: Each topic carries the document its GET returns, rendered for the subscriber
Each topic of the `stream` resource SHALL carry exactly one document, the one its `GET` returns:
`platform` the `Platform`, `instances` and `instances:<ns>` the `InstanceList`, `instance:` the
`Instance`, `package:` the `Package`, and `events:` the `EventList` of its object. Snapshot and
upsert messages SHALL carry that document rendered for the subscriber. Source: portal:D2:R5.

#### Scenario: A topic's document

- **WHEN** a client follows `instance:default/podinfo`
- **THEN** each snapshot and upsert message carries the `Instance` document, rendered for the subscriber

### Requirement: A deleted or missing object's topic carries a Removed document
When the object a topic of the `stream` resource shows is deleted, or does not exist at the
snapshot, that topic SHALL carry a `Removed` document naming it. Source: portal:D2:R5.

#### Scenario: A deleted instance

- **WHEN** a client follows `instance:default/podinfo` and the instance is deleted
- **THEN** the stream delivers a delete carrying a `Removed` document naming it, on the
  instance topic and on its events topic, once: later refreshes do not repeat it

#### Scenario: Deleted and recreated before the next publish

- **WHEN** a followed instance is deleted and created again within one coalescing window
- **THEN** the stream delivers the instance as it now is, not a `Removed` document

### Requirement: A change to an OPM object, an inventory object or a runtime child reaches the topics showing it
A change to an OPM object, to an inventory object or to a runtime child below one SHALL reach the
topics showing it. Source: portal:D2:R5.

#### Scenario: An instance topic follows a health change

- **WHEN** a client follows `instance:default/podinfo` and one of its Pods starts waiting on
  `ErrImagePull`
- **THEN** the stream delivers an upsert whose `Instance` document has `health.state` `Degraded`

### Requirement: A topic the broker does not serve is refused and a stream beyond the cap gets 429
Topics the broker does not serve SHALL be refused with `400` and code `bad_request`; a stream beyond
the session's cap with `429` and code `too_many_streams`. Source: portal:D2:R5.

#### Scenario: A topic the broker does not serve

- **WHEN** a client asks the `stream` resource for a topic the broker does not serve
- **THEN** it is refused with `400` and code `bad_request`

#### Scenario: A stream beyond the cap

- **WHEN** a client opens a stream beyond the session's cap
- **THEN** it is refused with `429` and code `too_many_streams`

### Requirement: The OpenAPI document lists every route and matches the wire types
The repository SHALL hold an OpenAPI 3.1 document of the read API. Every served route SHALL appear
in it with the document its success response returns, and every wire type SHALL match its schema by
field names and types; a test SHALL fail otherwise. Source: portal:D2:R2.

#### Scenario: A route or a wire type that differs

- **WHEN** a served route is missing from the OpenAPI document, or a wire type differs from its schema by a field name or type
- **THEN** a test fails

### Requirement: A breaking change to the OpenAPI document fails Lint unless the title marks it
A pull request that changes the OpenAPI document in a way `oasdiff breaking` reports as an error
SHALL fail the required `Lint` check unless its title marks a breaking change with `!`. Source:
portal:D2:R2.

#### Scenario: A field renamed without a breaking title

- **WHEN** a pull request titled `feat(api): rename a field` removes a response property
- **THEN** the `Lint` check fails and names the breaking change

#### Scenario: A field added

- **WHEN** a pull request adds an optional response property
- **THEN** the `Lint` check's API gate passes

### Requirement: A removed enumerated value fails Lint and a test compares values with the Go constants
Because `oasdiff` ignores `x-extensible-enum`, the required `Lint` check SHALL fail when a value the
base OpenAPI document lists for an enumerated field is missing from the head, and a test SHALL fail
when an enumerated field's values differ from the Go constants the server writes. Source:
portal:D2:R2.

#### Scenario: An enumerated value removed

- **WHEN** a pull request titled `feat(api): x` removes `ManagedExternally` from
  `Reconcile.state`
- **THEN** the `Lint` check fails and names the removed value

### Requirement: A topics request attaches the added topics, detaches the removed ones and answers 204
`POST /api/v1alpha1/clusters/{cluster}/stream/{stream}/topics` with `Content-Type: application/json`
and a body `{"add": [topic...], "remove": [topic...]}` SHALL attach the added topics to the
session's open stream `{stream}`, each authorized as at open (a denied topic is closed on the
stream), then detach the removed ones, and SHALL answer `204`.

#### Scenario: Moving a stream to another page

- **WHEN** a client holding stream `S` on topic `platform` posts
  `{"add": ["instance:default/podinfo"], "remove": ["platform"]}` to `stream/S/topics`
- **THEN** the response is `204`, `S` delivers a snapshot of `instance:default/podinfo`, and no
  further `platform` messages

### Requirement: A topics request of another type or shape or with an unparsable topic is answered 400
A `POST` to `stream/{stream}/topics` with a body of another type or shape, or a topic that does not
parse, SHALL be answered `400` with code `bad_request`.

#### Scenario: A form post

- **WHEN** a client posts `application/x-www-form-urlencoded` to a stream's topics
- **THEN** the response is `400` with code `bad_request` and the topics are unchanged

### Requirement: A topics request for a stream that is not the session's open stream is answered 404
A `POST` to `stream/{stream}/topics` for a stream that is not this session's open stream SHALL be
answered `404` with `not_found`.

#### Scenario: Another session's stream

- **WHEN** a client posts to the topics of a stream another session opened
- **THEN** the response is `404` with code `not_found`

### Requirement: A topics request without the session is answered 401
A `POST` to `stream/{stream}/topics` without the session SHALL be answered `401` with
`unauthenticated`.

#### Scenario: No session

- **WHEN** a client posts to `stream/{stream}/topics` without the session
- **THEN** the response is `401` with `unauthenticated`

### Requirement: A topics request reads and creates nothing in the cluster
A `POST` to `stream/{stream}/topics` SHALL read and create nothing in the cluster.

#### Scenario: Nothing in the cluster

- **WHEN** a client posts to `stream/{stream}/topics`
- **THEN** nothing is read or created in the cluster

### Requirement: In local mode a cross-origin topics request is refused by the front door
In local mode a cross-origin `POST` to `stream/{stream}/topics` SHALL be refused by the front door
before it reaches the read API.

#### Scenario: A cross-origin post

- **WHEN** in local mode a cross-origin request posts to `stream/{stream}/topics`
- **THEN** the front door refuses it before it reaches the read API

### Requirement: An object resource serves an Object document with the object's reference
`GET …/instances/{namespace}/{name}/object` and `GET …/packages/{namespace}/{name}/object`, with the
object named by the `group`, `kind`, `namespace` and `name` query parameters, SHALL serve an
`Object` document carrying the object's reference (with the version its kind is served at) and the
object as the cluster returns it. Source: portal:D7:R4, portal:D8:R1/R2/R3, portal:D2:R4.

#### Scenario: podinfo's Deployment

- **WHEN** an allowed caller requests the `object` of `apps` `Deployment`
  `default/podinfo-podinfo` under instance `default/podinfo`
- **THEN** the response is an `Object` document with the Deployment, its pod template included,
  and no managed fields or last-applied annotation

### Requirement: A served object lacks managed fields, the apply annotation and values
The object an `Object` document carries SHALL lack `metadata.managedFields` and the
`kubectl.kubernetes.io/last-applied-configuration` annotation and, on a ModuleInstance or
ModulePackage, `spec.values`. Source: portal:D7:R4, portal:D8:R1/R2/R3, portal:D2:R4.

#### Scenario: A stripped ModuleInstance

- **WHEN** a caller requests the `object` of a ModuleInstance
- **THEN** the `Object` document lacks `metadata.managedFields`, the `kubectl.kubernetes.io/last-applied-configuration` annotation and `spec.values`

### Requirement: The object reads are authorized on the owner and then on the object before any lookup
The `object` reads of an instance or package SHALL be authorized as the events about one object are,
before any lookup: `get` on the owner, then `get` on the object. Source: portal:D7:R4,
portal:D8:R1/R2/R3, portal:D2:R4.

#### Scenario: Owner, then object

- **WHEN** a caller requests the `object` of an object under an instance
- **THEN** `get` on the owner is authorized, then `get` on the object, before any lookup

### Requirement: A kind the cluster does not serve and an object no inventory reaches are refused
A kind the cluster does not serve and an object the owner's inventory does not reach SHALL each be
refused `403` with code `forbidden` by an `object` read. Source: portal:D7:R4, portal:D8:R1/R2/R3,
portal:D2:R4.

#### Scenario: An object no inventory reaches

- **WHEN** a caller allowed to read every Deployment requests one that podinfo's inventory does
  not name
- **THEN** the response is `403` with code `forbidden`

#### Scenario: An object of a kind the cluster does not serve

- **WHEN** a caller requests the `object` of a kind the cluster does not serve
- **THEN** the response is `403` with code `forbidden`

### Requirement: A core Secret is refused before any review about it
A core Secret SHALL be refused `403` with code `forbidden` by an `object` read, a Secret before any
review about it. Source: portal:D7:R4, portal:D8:R1/R2/R3, portal:D2:R4.

#### Scenario: A Secret

- **WHEN** a caller requests the `object` of a core `Secret`
- **THEN** the response is `403` with code `forbidden` and no review or read of the Secret is
  made

### Requirement: Every condition carries a tone
Every condition a document serves SHALL carry `tone`, how the condition reads for its type:
`normal`, `abnormal`, `progressing`, `informational` or `unknown`. Source: portal:D2:R1,
portal:D3:R8.

#### Scenario: A condition's tone

- **WHEN** a client reads a document that serves a condition
- **THEN** the condition carries `tone`, one of `normal`, `abnormal`, `progressing`, `informational` or `unknown`

### Requirement: A condition's tone follows its type and status
A condition's `tone` SHALL be `abnormal` for `Stalled=True`, `progressing` for `Reconciling=True`,
`informational` for `ContractsFulfilled=False` and `Drifted=True`, `abnormal` for `Ready=False`, and
`unknown` for an `Unknown` status or a type the portal does not know. Source: portal:D2:R1,
portal:D3:R8.

#### Scenario: A refused registration's platform

- **WHEN** a client reads the F1 platform, whose `ContractsFulfilled` is `False`
- **THEN** that condition's `tone` is `informational` and its `meaning` explains unfulfilled
  contracts

### Requirement: A condition carries its meaning and next step only for a reason the portal explains
A condition whose reason the portal explains SHALL carry its `meaning` and, when a person can act on
it, its `nextStep`; a reason the portal does not know SHALL carry neither. Source: portal:D2:R1,
portal:D3:R8.

#### Scenario: An unknown reason

- **WHEN** a condition carries a reason the portal does not know
- **THEN** it has no `meaning` and no `nextStep`, and in `local` mode its message is served
  unchanged

### Requirement: In-cluster mode serves no condition, reconcile, history or registration message
In `in-cluster` mode no document, whether served by a `GET` or on the change stream, SHALL carry
text the operator wrote: a condition's `message` (a registration's `conditions` included), a
reconcile `message`, a history entry's `message`, a registration's `message` or `activeMessage`.
Source: owner answer to portal:OQ8 (portal:D8:R5); its scope is the supervisor ruling recorded under
portal:D8.

#### Scenario: A failed instance in-cluster

- **WHEN** a client of an in-cluster server reads an instance whose `Ready` condition is `False`
  with reason `RenderFailed` and a message
- **THEN** the condition carries its type, status, reason, tone, meaning and next step, and no
  `message`; the reconcile state carries its reason and no `message`

#### Scenario: Registration conditions in-cluster

- **WHEN** a client of an in-cluster server reads the platform document
- **THEN** each registration's `conditions` carry type, status, reason, tone and meaning, and no
  `message`

#### Scenario: Local mode

- **WHEN** a client of a local server reads the same documents
- **THEN** every message and note is served as the operator and the API server wrote it

### Requirement: In-cluster mode serves no operator-reported event note
In `in-cluster` mode no document, whether served by a `GET` or on the change stream, SHALL carry the
`note` of an event the operator reported (its `reportingController` is `opm-controller`, or it names
none and regards an `opmodel.dev` object). Source: owner answer to portal:OQ8 (portal:D8:R5); its
scope is the supervisor ruling recorded under portal:D8.

#### Scenario: Events in-cluster

- **WHEN** a client of an in-cluster server reads an `EventList` or follows its `events:` topic
- **THEN** every event the operator reported carries its type, reason, count and times, and no
  `note`

### Requirement: In-cluster mode serves no health message of an opmodel.dev object or status message in an Object
In `in-cluster` mode no document, whether served by a `GET` or on the change stream, SHALL carry the
health `message` of an inventory object or graph node of an `opmodel.dev` kind, or, in an `Object`
of an `opmodel.dev` kind, `status.conditions[].message` and `status.history[].message`. Source:
owner answer to portal:OQ8 (portal:D8:R5); its scope is the supervisor ruling recorded under
portal:D8.

#### Scenario: An OPM object's YAML in-cluster

- **WHEN** a client of an in-cluster server reads the `Object` of the TransformerRegistration
  `default.backup-provider` through the instance that reaches it
- **THEN** its `status.conditions` keep their type, status and reason and carry no `message`

### Requirement: In-cluster mode keeps every reason, state, tone, meaning, next step, outcome and providerOf standing
In `in-cluster` mode every reason, state, `tone`, `meaning`, `nextStep`, history `outcome` and
`providerOf` standing SHALL stay in a document. Source: owner answer to portal:OQ8 (portal:D8:R5);
its scope is the supervisor ruling recorded under portal:D8.

#### Scenario: Reasons and states stay

- **WHEN** a client of an in-cluster server reads an instance whose `Ready` condition is `False` with reason `RenderFailed`
- **THEN** the condition keeps its reason, tone, meaning and next step, and the reconcile state keeps its reason

### Requirement: In-cluster mode serves text other writers wrote as written
In `in-cluster` mode text other writers wrote SHALL be served as written: the notes of events the
kubelet or another controller reported, and the health `message` of an object outside `opmodel.dev`.
Source: owner answer to portal:OQ8 (portal:D8:R5); its scope is the supervisor ruling recorded under
portal:D8.

#### Scenario: The kubelet's events in-cluster

- **WHEN** a client of an in-cluster server reads the `EventList` of a Pod the kubelet reported
  pulling an image for
- **THEN** each of those events carries its `note` as the kubelet wrote it

### Requirement: In-cluster mode serves events left alike once their notes are dropped as one line
In `in-cluster` mode events the operator reported that are left alike once their notes are dropped
(same type, reason, reporting controller, regarded object and field path) SHALL be served as one
line, their counts summed and its time the latest, so the number of lines does not tell how many
distinct notes were dropped. Source: owner answer to portal:OQ8 (portal:D8:R5); its scope is the
supervisor ruling recorded under portal:D8.

#### Scenario: Events differing only in their note in-cluster

- **WHEN** a client of an in-cluster server reads the `EventList` of an instance with two
  `Applied` events whose notes differ
- **THEN** it carries one `Applied` line with count 2 and the later time

### Requirement: In-cluster mode fails a document type the omission does not know
In `in-cluster` mode a document type the omission does not know SHALL fail with
`upstream_unavailable` rather than be served. Source: owner answer to portal:OQ8 (portal:D8:R5); its
scope is the supervisor ruling recorded under portal:D8.

#### Scenario: An unknown document type in-cluster

- **WHEN** a client of an in-cluster server reads a document of a type the omission does not know
- **THEN** it fails with `upstream_unavailable` rather than being served

### Requirement: The OpenAPI document says which fields are absent in-cluster
The OpenAPI document SHALL say, on each field whose operator text is omitted in `in-cluster` mode,
that it is absent in-cluster. Source: owner answer to portal:OQ8 (portal:D8:R5); its scope is the
supervisor ruling recorded under portal:D8.

#### Scenario: An omitted field in the OpenAPI document

- **WHEN** a field's operator text is omitted in `in-cluster` mode
- **THEN** the OpenAPI document says on that field that it is absent in-cluster

### Requirement: The Cluster document names the cluster, mode, source, context and caller
`GET /api/v1alpha1/clusters/{cluster}` SHALL serve a `Cluster` document: the path cluster's `name`,
the server's `mode`, its `source` (`kubeconfig` or `in-cluster`), the kubeconfig `context` and the
name of its `clusterEntry` when the source is `kubeconfig`, and `readingAs` with the requesting
caller's own username. Source: portal:D18:R1/R2/R3.

#### Scenario: Local mode on the fixture cluster

- **WHEN** a local-mode client reads `/api/v1alpha1/clusters/default` against a kubeconfig whose
  context is `kind-opm-portal-e2e`
- **THEN** the document has `source: kubeconfig`, `context: kind-opm-portal-e2e`, `readingAs`
  with the kubeconfig's username and the API server's `kubernetesVersion`

### Requirement: The Cluster document carries the API server version when the read model holds it
The `Cluster` document SHALL carry the API server's `kubernetesVersion` as the read model holds it,
absent when the read model holds none. Source: portal:D18:R1/R2/R3.

#### Scenario: Version not read

- **WHEN** the read model could not read `/version` at start
- **THEN** the document has no `kubernetesVersion`, and the response is still `200`

### Requirement: The Cluster document carries no server URL or credential and makes no cluster read
The `Cluster` document SHALL carry no server URL, kubeconfig user entry or credential, and no other
identity than the caller's, and serving it SHALL make no cluster read per request. Source:
portal:D18:R1/R2/R3.

#### Scenario: Nothing else about the kubeconfig

- **WHEN** any client reads the `Cluster` document
- **THEN** it contains no server URL, certificate, token or user entry

### Requirement: A Cluster request without a principal is answered 401
A request for the `Cluster` document without a principal SHALL be answered `401` with code
`unauthenticated`. Source: portal:D18:R1/R2/R3.

#### Scenario: No principal

- **WHEN** a client requests the `Cluster` document without a principal
- **THEN** the response is `401` with code `unauthenticated`

### Requirement: The Cluster document is served unchanged in-cluster
In `in-cluster` mode the `Cluster` document SHALL be served unchanged: it carries no operator text.
Source: portal:D18:R1/R2/R3.

#### Scenario: In-cluster

- **WHEN** a client of an in-cluster server reads the `Cluster` document
- **THEN** it is served, not refused as a document type the omission does not know

### Requirement: A package carries its interval and source artifact as recorded
A package document and a package list item SHALL carry `interval`, the package's `spec.interval` as
written, when it is set, and `sourceArtifact` with the `revision` and `digest` the controller
recorded in `status.source`, when it recorded them. Neither SHALL be filled with a default or a
guess. Source: portal:D2:R4.

#### Scenario: The F1 package

- **WHEN** a client reads package `pkg/podinfo` from the F1 capture
- **THEN** it has `interval` `1m`, no `sourceArtifact` and no `serviceAccountName`

### Requirement: A package's source artifact fetch URL is not served
The fetch URL of a package's source artifact SHALL NOT be served. Source: portal:D2:R4.

#### Scenario: The fetch URL

- **WHEN** a client reads a package document
- **THEN** the source artifact's fetch URL is not served

### Requirement: A package document carries its serviceAccountName when set
A package document SHALL also carry `serviceAccountName`, the package's `spec.serviceAccountName`,
when it is set; absent, the controller applies as its own identity. Source: portal:D2:R4.

#### Scenario: A package with an applier

- **WHEN** a package sets `spec.serviceAccountName` to `podinfo-applier`
- **THEN** its document carries `serviceAccountName` `podinfo-applier`

### Requirement: A registration carries its conditions with tone, meaning and next step
Each registration in the platform document SHALL carry its `conditions` with `tone`, `meaning` and
`nextStep` as every condition does. Source: portal:D15:R1/R2, portal:D4:R2.

#### Scenario: A registration's conditions

- **WHEN** a client reads the platform document
- **THEN** each registration carries its `conditions` with `tone`, `meaning` and `nextStep`

### Requirement: A registration carries the instances and packages that hold it
Each registration in the platform document SHALL carry `heldBy`: the ModuleInstances and
ModulePackages the caller may read whose inventory holds a TransformerRegistration of that name,
whatever their kind. Source: portal:D15:R1/R2, portal:D4:R2.

#### Scenario: The accepted claim's holder

- **WHEN** a client reads the F1 platform
- **THEN** registration `default.backup-provider` has `heldBy` naming ModuleInstance
  `default/backup-provider` and a `Ready` condition with reason `Accepted`

#### Scenario: A claim nobody holds

- **WHEN** a client reads the F1 platform
- **THEN** registration `default.refused-claim-fixture`, applied by hand, has no `heldBy`

#### Scenario: A package holder

- **WHEN** a package's inventory holds TransformerRegistration `pkg.provider` and the controller
  refused it with `ProviderMismatch`
- **THEN** that registration's `heldBy` names the ModulePackage, its `provider` still names a
  ModuleInstance, and its verdict is `Refused` with reason `ProviderMismatch`

### Requirement: heldByPartial is true whenever the caller cannot see every holder
`heldByPartial` of a registration SHALL be true whenever the caller lacks a cluster-wide list of
ModuleInstances or of ModulePackages, or the portal does not hold every namespace, whether or not
`heldBy` is empty. Source: portal:D15:R1/R2, portal:D4:R2.

#### Scenario: Instances not listable everywhere

- **WHEN** a caller who may list ModuleInstances only in namespace `default` reads the platform
- **THEN** each registration has `heldByPartial: true`

### Requirement: A registration's provider keeps naming the ModuleInstance its providerRef names
The `provider` of a registration SHALL keep naming the ModuleInstance `spec.providerRef` names.
Source: portal:D15:R1/R2, portal:D4:R2.

#### Scenario: The provider

- **WHEN** a client reads the platform document
- **THEN** each registration's `provider` names the ModuleInstance its `spec.providerRef` names

### Requirement: An instance or package carries one providerOf entry per registration it holds
An instance or package, in its document and its list item, SHALL carry `providerOf`: one entry per
TransformerRegistration its inventory holds, with the registration's name, the caller's `access` to
it, and, when readable, its `accepted`, `active`, `verdict` and `reason`, and `providerRefMatches`.
Source: portal:D15:R1/R2/R4.

#### Scenario: The F1 provider

- **WHEN** a client reads instance `default/backup-provider` from the F1 capture
- **THEN** `providerOf` holds `default.backup-provider`, accepted, active, verdict `Accepted`,
  `providerRefMatches: true`

### Requirement: providerRefMatches is true only for a ModuleInstance the providerRef names
The `providerRefMatches` of a `providerOf` entry SHALL be true only when the owner is a
ModuleInstance whose namespace and name the registration's `spec.providerRef` names, false otherwise
(always false for a ModulePackage, since the reference names a ModuleInstance). Source:
portal:D15:R1/R2/R4.

#### Scenario: A package holder

- **WHEN** a package `pkg/provider` holds TransformerRegistration `pkg.provider`, whose
  `spec.providerRef` names `pkg/provider`
- **THEN** its `providerOf` entry has `providerRefMatches: false`

### Requirement: A registration the cluster does not hold carries verdict Unknown
A `providerOf` entry for a registration the inventory names but the cluster does not hold SHALL
carry verdict `Unknown` and no `providerRefMatches`. Source: portal:D15:R1/R2/R4.

#### Scenario: A registration the cluster does not hold

- **WHEN** an inventory names a registration the cluster does not hold
- **THEN** its `providerOf` entry carries verdict `Unknown` and no `providerRefMatches`

### Requirement: A registration the caller may not read carries its name and access only
A `providerOf` entry for a registration the caller may not read SHALL carry its name and `access:
forbidden` only, with no `providerRefMatches`. Source: portal:D15:R1/R2/R4.

#### Scenario: Registrations forbidden

- **WHEN** a caller who may not list TransformerRegistrations reads the same instance
- **THEN** `providerOf` holds `default.backup-provider` with `access: forbidden`, no standing and
  no `providerRefMatches`

### Requirement: A change to a registration reaches the topics of the owners that hold it
A change to a TransformerRegistration SHALL reach the `instance:` or `package:` topic of every owner
whose inventory holds it, and the `instances` and `instances:<namespace>` topics for instance
holders, so `providerOf` refreshes. Each subscriber is still written only a change to its own
document. Source: portal:D2:R5/R7, portal:D15.

#### Scenario: A claim becomes active

- **WHEN** a client follows `instance:default/backup-provider` and `default.backup-provider` turns
  active
- **THEN** the stream delivers an upsert whose `providerOf` entry is active

### Requirement: A change to a holder reaches the platform topic
A change to a ModuleInstance or ModulePackage whose inventory holds a TransformerRegistration,
before or after the change, SHALL reach the `platform` topic, so `heldBy` refreshes. Each subscriber
is still written only a change to its own document. Source: portal:D2:R5/R7, portal:D15.

#### Scenario: A holder drops its claim

- **WHEN** a client follows `platform` and backup-provider's inventory stops holding
  `default.backup-provider`
- **THEN** the stream delivers a `Platform` whose registration `default.backup-provider` has no
  `heldBy`
