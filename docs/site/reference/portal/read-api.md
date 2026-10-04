---
title: "Portal read API"
description: "Every resource of the portal's read API, the topics of its change stream, and its problem codes."
type: reference
weight: 10
---

This page lists every resource of the OPM portal's read API, version `v1alpha1`, the topics of its change stream and the problem codes it answers with. The full contract, with every document's fields, is the OpenAPI document [`openapi/v1alpha1.yaml`](https://github.com/open-platform-model/opm-portal/blob/main/openapi/v1alpha1.yaml) in the portal's repository.

> [!IMPORTANT]
> **Not in a release yet**
>
> No `opm-portal` release serves this API yet. The resources below are built and tested on `main`.

## Rules for every resource

- Every resource lives below `/api/v1alpha1`, and every method is `GET`. Any other method is refused with `method_not_allowed`.
- Every path names the cluster. The portal serves one cluster, `default`; any other is refused.
- Every document carries `apiVersion: portal.opmodel.dev/v1alpha1` and a `kind`.
- Every request is authorized for the caller before anything is looked up. A caller who may not make a read receives the same `forbidden` problem whether or not the object exists. A list the caller may not read is empty, its `access` field is `forbidden`, and it carries no count.
- An item the caller may not read inside a readable document is marked by its `access` field: `forbidden` (the caller may not read it), `notReadable` (the portal may not read it) or `withheld` (a Secret, never read).
- Within `v1alpha1` fields and enumerated values are only added. Clients ignore fields they do not know and treat every enumerated string as open, showing an unknown value as unknown.
- No document carries an instance's or package's `spec.values`, Secret data or the `kubectl.kubernetes.io/last-applied-configuration` annotation. Condition and history messages and event notes are served exactly as the operator and the API server wrote them.

## Resources

Paths are below `/api/v1alpha1`.

| Resource | Returns |
| --- | --- |
| `GET /clusters/{cluster}/instances` | `InstanceList`: the ModuleInstances the caller may list, each with its applied state and health. `?namespace=` lists one namespace. |
| `GET /clusters/{cluster}/instances/{namespace}/{name}` | `Instance`: one ModuleInstance with its components and inventory. |
| `GET /clusters/{cluster}/instances/{namespace}/{name}/graph` | `Graph`: the instance's relationship graph. `?expand=` opens a group node, `?showScaledDown=true` shows ReplicaSets scaled to zero. |
| `GET /clusters/{cluster}/instances/{namespace}/{name}/events` | `EventList`: recent events about the instance, or, with `?group=&kind=&namespace=&name=`, about one object it reaches. |
| `GET /clusters/{cluster}/packages` | `PackageList`: the ModulePackages the caller may list. `?namespace=` lists one namespace. |
| `GET /clusters/{cluster}/packages/{namespace}/{name}` | `Package`: one ModulePackage with its components and inventory. |
| `GET /clusters/{cluster}/packages/{namespace}/{name}/graph` | `Graph`: the package's relationship graph. |
| `GET /clusters/{cluster}/packages/{namespace}/{name}/events` | `EventList`: recent events about the package, or about one object it reaches. |
| `GET /clusters/{cluster}/platform` | `Platform`: the Platform, its catalogs and its transformer registrations. |
| `GET /clusters/{cluster}/platform/graph` | `Graph`: catalogs, registrations and the instances that provide them. |
| `GET /clusters/{cluster}/platform/events` | `EventList`: recent events about the Platform. |
| `GET /clusters/{cluster}/platform/registrations/{name}/events` | `EventList`: recent events about one TransformerRegistration. |
| `GET /clusters/{cluster}/stream` | A server-sent-events stream of the topics named in `?topics=`. |

## Change stream topics

`GET /clusters/{cluster}/stream?topics=<topic>,<topic>` follows several topics on one connection. Each topic carries the document its `GET` returns, rendered for the caller.

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

A `snapshot` event carries `{topic, items: [document]}`; `upsert`, `delete` and `k8sevent` events carry `{topic, item: document}`. A `delete`, and a snapshot of an object that does not exist, carry a `Removed` document. A topic the caller may not follow is closed with a `closed` event naming a problem code. To resume after a disconnect, reconnect with the `Last-Event-ID` header.

## Problem codes

Errors are RFC 9457 problem documents, media type `application/problem+json`, with a `code`. The set of codes may grow.

| Status | `code` | Meaning |
| --- | --- | --- |
| 400 | `bad_request` | A path or query value is malformed. |
| 401 | `unauthenticated` | The request names no authenticated user. |
| 403 | `forbidden` | The caller may not make this read, or the portal does not serve the object. The same for an object that does not exist. |
| 404 | `not_found` | The object does not exist, and the caller may read its kind there. |
| 405 | `method_not_allowed` | The method is not `GET`. |
| 429 | `too_many_streams` | The session holds as many streams as it may. |
| 503 | `not_readable_by_portal` | The portal's reading identity may not list and watch the kind, or its cache has not synced yet. |
| 503 | `upstream_unavailable` | An access review or the Kubernetes API failed. Try again. |
