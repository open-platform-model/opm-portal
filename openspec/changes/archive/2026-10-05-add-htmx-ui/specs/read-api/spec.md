## MODIFIED Requirements

### Requirement: Resources live under a versioned, cluster-parameterized path

The portal SHALL serve every read API resource under `/api/v1alpha1/clusters/{cluster}/`, with
`GET` only: `instances`, `instances/{namespace}/{name}`, `instances/{namespace}/{name}/graph`,
`instances/{namespace}/{name}/events`, `instances/{namespace}/{name}/object`, `packages`,
`packages/{namespace}/{name}`, `packages/{namespace}/{name}/graph`,
`packages/{namespace}/{name}/events`, `packages/{namespace}/{name}/object`, `platform`,
`platform/graph`, `platform/events`, `platform/registrations/{name}/events` and `stream`. The one
exception SHALL be `stream/{stream}/topics`, which takes `POST` only. In milestone 1 the only
cluster served SHALL be `default`. Any other cluster name SHALL be answered `404` with code
`not_found` before any authorization review. A method a resource does not take SHALL be answered
`405` with code `method_not_allowed`. A namespace that is not a DNS-1123 label, or a name that is
not a DNS-1123 subdomain, SHALL be answered `400` with code `bad_request` before any
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

## ADDED Requirements

### Requirement: A stream's topics change through one request

`POST /api/v1alpha1/clusters/{cluster}/stream/{stream}/topics` with `Content-Type:
application/json` and a body `{"add": [topic...], "remove": [topic...]}` SHALL attach the added
topics to the session's open stream `{stream}`, each authorized as at open (a denied topic is
closed on the stream), then detach the removed ones, and SHALL answer `204`. A body of another
type or shape, or a topic that does not parse, SHALL be answered `400` with code `bad_request`;
a stream that is not this session's open stream `404` with `not_found`; a request without the
session `401` with `unauthenticated`. The request SHALL read and create nothing in the cluster.
In local mode a cross-origin request SHALL be refused by the front door before it reaches the
read API.

#### Scenario: Moving a stream to another page

- **WHEN** a client holding stream `S` on topic `platform` posts
  `{"add": ["instance:default/podinfo"], "remove": ["platform"]}` to `stream/S/topics`
- **THEN** the response is `204`, `S` delivers a snapshot of `instance:default/podinfo`, and no
  further `platform` messages

#### Scenario: Another session's stream

- **WHEN** a client posts to the topics of a stream another session opened
- **THEN** the response is `404` with code `not_found`

#### Scenario: A form post

- **WHEN** a client posts `application/x-www-form-urlencoded` to a stream's topics
- **THEN** the response is `400` with code `bad_request` and the topics are unchanged

### Requirement: One object an inventory reaches is served stripped

`GET …/instances/{namespace}/{name}/object` and `GET …/packages/{namespace}/{name}/object`, with
the object named by the `group`, `kind`, `namespace` and `name` query parameters, SHALL serve an
`Object` document carrying the object's reference (with the version its kind is served at) and
the object as the cluster returns it, without `metadata.managedFields`, the
`kubectl.kubernetes.io/last-applied-configuration` annotation, or, on a ModuleInstance or
ModulePackage, `spec.values`. The reads SHALL be authorized as the events about one object are,
before any lookup: `get` on the owner, then `get` on the object; a kind the cluster does not
serve, an object the owner's inventory does not reach, and a core Secret SHALL each be refused
`403` with code `forbidden`, a Secret before any review about it. Source: 0030:D7:R4,
0030:D8:R1/R2/R3, 0030:D2:R4.

#### Scenario: podinfo's Deployment

- **WHEN** an allowed caller requests the `object` of `apps` `Deployment`
  `default/podinfo-podinfo` under instance `default/podinfo`
- **THEN** the response is an `Object` document with the Deployment, its pod template included,
  and no managed fields or last-applied annotation

#### Scenario: An object no inventory reaches

- **WHEN** a caller allowed to read every Deployment requests one that podinfo's inventory does
  not name
- **THEN** the response is `403` with code `forbidden`

#### Scenario: A Secret

- **WHEN** a caller requests the `object` of a core `Secret`
- **THEN** the response is `403` with code `forbidden` and no review or read of the Secret is
  made

### Requirement: Pods name their containers

A runtime child that is a Pod SHALL carry `containers`: the names of its init containers, then
its containers, in spec order, so a client can name the Pod's log topics.

#### Scenario: podinfo's Pods

- **WHEN** a caller reads instance `default/podinfo` from the F1 capture
- **THEN** each Pod child carries `containers` `["podinfo"]`
