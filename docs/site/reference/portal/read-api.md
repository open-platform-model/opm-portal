---
title: "Portal read API"
description: "Every resource of the portal's read API, the topics of its change stream, and its problem codes."
type: reference
weight: 10
---

This page lists every resource of the OPM portal's read API, version `v1alpha1`, the topics of its change stream and the problem codes it answers with. The full contract, with every document's fields, is the OpenAPI document [`openapi/v1alpha1.yaml`](https://github.com/open-platform-model/opm-portal/blob/main/openapi/v1alpha1.yaml) in the portal's repository.

## Rules for every resource

- Every resource lives below `/api/v1alpha1`, and every method is `GET`, except the request that changes an open stream's topics, which is `POST`. Any other method is refused with `method_not_allowed`.
- Every path names the cluster. The portal serves one cluster, `default`; any other is answered with `not_found`.
- Every document carries `apiVersion: portal.opmodel.dev/v1alpha1` and a `kind`.
- Every request is authorized for the caller before anything is looked up. A caller who may not make a read receives the same `forbidden` problem whether or not the object exists. A list the caller may not read is empty, its `access` field is `forbidden`, and it carries no count.
- An item the caller may not read inside a readable document is marked by its `access` field: `forbidden` (the caller may not read it), `notReadable` (the portal could not read it: the access review or the read failed, or the kind is not served) or `withheld` (a Secret, never read).
- Within `v1alpha1` fields and enumerated values are only added. Clients ignore fields they do not know and treat every enumerated string as open, showing an unknown value as unknown.
- No document carries an instance's or package's `spec.values`, Secret data or the `kubectl.kubernetes.io/last-applied-configuration` annotation. Condition and history messages and event notes are served exactly as the operator and the API server wrote them.
- Every condition carries a `tone`, how it reads for its type, because its status alone does not say it: `Stalled=True` is `abnormal`, `Reconciling=True` `progressing`, `ContractsFulfilled=False` `informational`, and an `Unknown` status or a type the portal does not know is `unknown`. A condition whose reason the portal knows also carries its `meaning` and, when you can act on it, its `nextStep`.

## Resources

Paths are below `/api/v1alpha1`.

| Resource | Returns |
| --- | --- |
| `GET /clusters/{cluster}/instances` | `InstanceList`: the ModuleInstances the caller may list, each with its applied state and health. `?namespace=` lists one namespace. |
| `GET /clusters/{cluster}/instances/{namespace}/{name}` | `Instance`: one ModuleInstance with its components and inventory. |
| `GET /clusters/{cluster}/instances/{namespace}/{name}/graph` | `Graph`: the instance's relationship graph. `?expand=` opens a group node, `?showScaledDown=true` shows ReplicaSets scaled to zero. |
| `GET /clusters/{cluster}/instances/{namespace}/{name}/events` | `EventList`: recent events about the instance, or, with `?group=&kind=&namespace=&name=`, about one object it reaches. |
| `GET /clusters/{cluster}/instances/{namespace}/{name}/object` | `Object`: one object the instance reaches, named by `?group=&kind=&namespace=&name=`, as the cluster serves it, without managed fields, the last-applied annotation or values. A Secret, and an object no inventory reaches, are refused with `forbidden`. |
| `GET /clusters/{cluster}/packages` | `PackageList`: the ModulePackages the caller may list. `?namespace=` lists one namespace. |
| `GET /clusters/{cluster}/packages/{namespace}/{name}` | `Package`: one ModulePackage with its components and inventory. |
| `GET /clusters/{cluster}/packages/{namespace}/{name}/graph` | `Graph`: the package's relationship graph. |
| `GET /clusters/{cluster}/packages/{namespace}/{name}/events` | `EventList`: recent events about the package, or about one object it reaches. |
| `GET /clusters/{cluster}/packages/{namespace}/{name}/object` | `Object`: one object the package reaches, as for an instance. |
| `GET /clusters/{cluster}/platform` | `Platform`: the Platform, its catalogs and its transformer registrations. |
| `GET /clusters/{cluster}/platform/graph` | `Graph`: catalogs, registrations and the instances that provide them. |
| `GET /clusters/{cluster}/platform/events` | `EventList`: recent events about the Platform. |
| `GET /clusters/{cluster}/platform/registrations/{name}/events` | `EventList`: recent events about one TransformerRegistration. |
| `GET /clusters/{cluster}/stream` | A server-sent-events stream of the topics named in `?topics=`. |
| `POST /clusters/{cluster}/stream/{stream}/topics` | Adds and removes topics on an open stream of this session, named by the id its `open` event carries. The body is JSON, `{"add": [topic], "remove": [topic]}`, with `Content-Type: application/json`; the answer is `204`. An added topic the caller may not follow is closed on the stream. |

## Change stream topics

`GET /clusters/{cluster}/stream?topics=<topic>,<topic>` follows several topics on one connection. Each topic carries the document its `GET` returns, rendered for the caller. The stream's first event, `open`, carries `{stream: <id>}`; post to `stream/<id>/topics` to change the topics while the stream stays open.

| Topic | Document |
| --- | --- |
| `platform` | `Platform` |
| `instances` | `InstanceList`, cluster-wide |
| `instances:<namespace>` | `InstanceList` of one namespace |
| `instance:<namespace>/<name>` | `Instance` |
| `package:<namespace>/<name>` | `Package` |
| `events:platform` | `EventList` of the Platform |
| `events:instance:<namespace>/<name>` | `EventList` of an instance |
| `events:package:<namespace>/<name>` | `EventList` of a package |
| `events:registration:<name>` | `EventList` of a registration |
| `log:<namespace>/<pod>/<container>` | The log of one container of a Pod an OPM inventory reaches |
| `log:<namespace>/<pod>/<container>/previous` | The log of that container's previous, terminated instance |

A `snapshot` event carries `{topic, items: [document]}`; `upsert`, `delete` and `k8sevent` events carry `{topic, item: document}`. A `delete`, and a snapshot of an object that does not exist, carry a `Removed` document. A topic the caller may not follow is closed with a `closed` event naming a problem code. When the caller's session ends, the stream's last event is `expired`, carrying `{code: unauthenticated}`, and the response ends; the stream cannot be resumed, so a client stops and signs in again. To resume after a disconnect, reconnect with the `Last-Event-ID` header.

A log topic is served where the serving mode routes it; local mode does. Its snapshot carries the recent messages, and `log` and `logend` events carry `{topic, item: message}`. A Pod in an `Instance` or `Package` names its containers in `containers`, the names its log topics take. A message has a `seq`, a `type` (`line`, `marker` or `end`), its `container`, and, as its type needs, `time`, `text`, `marker` (`truncated`, `rate-limited` or `skipped`), `cut`, `dropped` or `reason`. These enumerations are open. Log text is untrusted. A session follows at most four log topics at once.

## Problem codes

Every error the read API answers is an RFC 9457 problem document, media type `application/problem+json`, with a `code`. The set of codes may grow.

| Status | `code` | Meaning |
| --- | --- | --- |
| 400 | `bad_request` | A path or query value is malformed, or a topic is malformed, not served or beyond the stream's limit. |
| 401 | `unauthenticated` | The request names no authenticated user. |
| 403 | `forbidden` | The caller may not make this read, or the portal does not serve the object. The same for an object that does not exist. |
| 404 | `not_found` | The object does not exist, and the caller may read its kind there; or the cluster is not `default`; or the path is not a read API resource. |
| 405 | `method_not_allowed` | The resource does not take the method: every resource takes `GET`, and a stream's topics take `POST`. |
| 429 | `too_many_streams` | The session, or the portal, holds as many streams as it may. |
| 503 | `not_readable_by_portal` | The portal's reading identity may not list and watch the kind, or its cache has not synced yet. |
| 503 | `upstream_unavailable` | An access review or the Kubernetes API failed, or the portal is shutting down. Try again. |

## Refusals before the read API

In local mode, the portal's front door answers some requests before the read API sees them. These answers have a plain-text body, not a problem document:

| Status | Request |
| --- | --- |
| 403 | Any request whose `Host` header is not `127.0.0.1`, `localhost`, `[::1]` or the address the portal listens on, with the portal's port. |
| 403 | A request with a method other than `GET`, `HEAD` or `OPTIONS` that comes from another origin. |
| 403 | A `GET /launch` whose token is missing, wrong or already used, from a browser without the session. A browser with the session is sent on whatever the token. |
| 405 | A `/launch` request with a method other than `GET`. |

A `GET /launch` that is not refused answers `200` with the Platform page, under the new session; its script then moves the browser once to `/`. A request without the session cookie reaches the read API and is answered with `unauthenticated`.
