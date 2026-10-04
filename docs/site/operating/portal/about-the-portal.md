---
title: "The OPM portal"
description: "What the portal shows about a cluster, where each fact comes from, and why applied state and health are two values."
type: explanation
weight: 10
---

The OPM portal is a read-only view of what a cluster runs under OPM. It starts at the Platform, with its catalogs and transformer registrations, and goes down through every ModuleInstance and ModulePackage to the objects each one applied and the Pods those objects run. Everything it shows comes from what the OPM operator and the Kubernetes API server already record. The portal adds two things of its own: an access check before every read, and a health value computed from live objects.

Where `kubectl get` shows one kind at a time, the portal joins the kinds OPM spreads a deployment over: the instance, its inventory, the workloads in the inventory and the Pods below them. The comparison stops at the edge of OPM. The portal shows only objects an OPM inventory reaches, so it is not a general cluster browser, and it never edits, restarts or deletes anything.

This page assumes you know what a ModuleInstance is and that you run the operator or the `opm` CLI. To start the portal, see [Run the portal locally](/docs/operating/portal/run-the-portal-locally/).

> [!IMPORTANT]
> **No web UI yet**
>
> The web UI this page mentions is not built. The read API carries every value this page describes.

## How it works

### A read API, with the web UI as its first client

The portal's contract is a versioned read API under `/api/v1alpha1`: JSON documents over `GET`, and one server-sent-events stream per browser tab that carries changes to the same documents. The documents are shaped for the portal (an instance, a package, the Platform, a graph, a list of events), never a copy of a custom resource's status. The web UI renders only from these documents, so a script or another tool that reads the API with the same identity sees what the UI shows. The resources are listed in [Portal read API](/docs/reference/portal/read-api/).

### Two values for every instance: Applied and Health

Each instance and package carries two values side by side, and the portal never merges them.

**Applied** comes from the operator's conditions. The operator sets `Ready=True` when every object of the render was applied; it does not wait for the workloads to come up. The portal therefore shows `Ready=True` as Applied, never as healthy. Other applied states are Reconciling, Failed, Stalled and Suspended. An instance the `opm` CLI owns shows Managed externally, because the operator leaves it alone; that is not a fault.

**Health** is the portal's own computation from live objects. Each object in the inventory is judged by the standard Kubernetes status rules, the ones `kstatus` implements. One rule is added: a Pod whose container is waiting with `ErrImagePull`, `ImagePullBackOff`, `CrashLoopBackOff`, `CreateContainerConfigError` or `InvalidImageName` marks the workload that owns it Degraded. The results roll up worst first, from objects to components to the instance, as Healthy, Progressing, Degraded, Missing or Unknown.

### Every graph edge comes from a recorded field

The portal draws two graphs. The Platform graph joins the Platform's catalogs, the transformer registrations and the instances that provide them. An instance graph runs from the module through the instance and its components to the inventory objects, then to the ReplicaSets, Pods and Jobs found through the owner references below those objects. Each kind of edge has exactly one source field. The portal never renders a module to find an edge, and where two sources disagree, such as a registration's provider and the provider's inventory, the graph shows the disagreement.

### Events and logs are read when you look

Events about an object are read when you open it, and repeats are folded into one entry with a count. The API server deletes events after a while, an hour by default, so the event list is a recent feed, not a history; the operator's conditions and status history are the record. Pod logs follow one container at a time, only for Pods an inventory reaches, and the portal bounds each log stream itself.

## Why it is built this way

### Why Applied is not healthy

The operator's `Ready` is correct for what it says: the apply succeeded. A rollout can still be broken. When an instance's image tag does not exist, the new Pod cannot pull it while the old replicas keep serving. The Deployment stays available, and the standard status rules call it in progress until its progress deadline passes, about ten minutes later. A single status badge would show green for those ten minutes. The Pod rule makes the instance Degraded within seconds of the Pod reporting its waiting reason, while it still shows Applied.

### Why the API comes first

Standalone Kubernetes UIs tend to be abandoned when their UI is the only interface. A read API that other tools can build on outlives any one UI, and making the portal's own UI a real client of it is what keeps the API complete.

### Why the portal keeps a watched view

Answering one instance page by reading each inventory object on request took seven to nine seconds for an instance with about forty objects. The portal therefore watches the four OPM kinds, and the kinds an inventory names while a page needs them, and answers from that view. A kind your identity may get but not list and watch is polled instead, and the page says how old the answer is.

## Common mistakes

### Applied does not mean it is running

Applied says the operator applied every object. Read Health to learn whether the workloads run.

### An empty list is not an empty cluster

A list you may not read comes back empty and says it is forbidden, with no count of what it hides. A missing permission and a missing object look the same on purpose; see [The portal's security model](/docs/operating/portal/portal-security/).

### Partial health is not healthy

When the portal or you may not read some objects of an instance, its health is computed from the rest and marked partial. A partial Healthy says nothing about the objects left out.

### The portal does not show every object

An object no OPM inventory reaches, such as a Deployment you applied with `kubectl`, is not served, and a request for it is refused like a forbidden one.

## What enforces this

- The portal refuses every Kubernetes verb other than `get`, `list` and `watch` before it sends a request. Its only creates are access reviews the API server answers and does not store.
- The portal refuses a request for an object no inventory reaches, with the same answer a missing permission gets.
- The API server enforces your access: every read is checked for your identity before it is made, against an answer at most 30 seconds old.
- The read API changes only by addition within `v1alpha1`: the portal's CI refuses a change to its OpenAPI document that removes or renames something, unless the change is marked breaking.

> [!NOTE]
> **Direction**
>
> The web UI, and an in-cluster mode in which people sign in with OIDC and every read is checked for the signed-in user, are designed in [enhancement 0030](/enhancements/0030/), a draft. Neither is built.
