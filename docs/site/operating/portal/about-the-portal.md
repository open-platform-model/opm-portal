---
title: "The OPM portal"
description: "What the portal shows about a cluster, where each fact comes from, and why applied state and health are two values."
type: explanation
weight: 10
---

The OPM portal is a read-only view of what a cluster runs under OPM. It starts at the Platform, with its catalogs and the providers that claim them, lists everything installed (every ModuleInstance and ModulePackage) in one place, and goes down through each of them to the objects it applied and the Pods those objects run. Every page's header names the cluster context the portal reads, the user it reads as and the cluster's Kubernetes version. Everything it shows comes from what the OPM operator and the Kubernetes API server already record. The portal adds two things of its own: an access check before every read, and a health value computed from live objects.

Where `kubectl get` shows one kind at a time, the portal joins the kinds OPM spreads a deployment over: the instance, its inventory, the workloads in the inventory and the Pods below them. The comparison stops at the edge of OPM. The portal shows only objects an OPM inventory reaches, so it is not a general cluster browser, and it never edits, restarts or deletes anything.

This page assumes you know what a ModuleInstance is and that you run the operator or the `opm` CLI. To start the portal, see [Run the portal locally](/docs/operating/portal/run-the-portal-locally/).

## How it works

### A read API, with the web UI as its first client

The portal's contract is a versioned read API under `/api/v1alpha1`: JSON documents over `GET`, and one server-sent-events stream per browser tab that carries changes to the same documents. The documents are shaped for the portal (an instance, a package, the Platform, a graph, a list of events), never a copy of a custom resource's status. The web UI renders only from these documents, so a script or another tool that reads the API with the same identity sees what the UI shows. The resources are listed in [Portal read API](/docs/reference/portal/read-api/).

### Two values for every instance: Applied and Health

Each instance and package carries two values side by side, and the portal never merges them.

**Applied** comes from the operator's conditions. The operator sets `Ready=True` when every object of the render was applied; it does not wait for the workloads to come up. The portal therefore shows `Ready=True` as Applied, never as healthy. Other applied states are Reconciling, Failed, Stalled and Suspended. An instance the `opm` CLI owns shows Managed externally, because the operator leaves it alone; that is not a fault.

**Health** is the portal's own computation from live objects. Each object in the inventory is judged by the standard Kubernetes status rules, the ones `kstatus` implements. One rule is added: a Pod whose container is waiting with `ErrImagePull`, `ImagePullBackOff`, `CrashLoopBackOff`, `CreateContainerConfigError` or `InvalidImageName` marks the workload that owns it Degraded. The results roll up worst first, from objects to components to the instance, as Healthy, Progressing, Degraded, Missing or Unknown.

### The Platform page, Installed and providers

The Platform page shows the Platform's identity, a status block with the operator's applied word and one line of what it resolved, counts of what is installed by health and by applied state, and two tabs. The status block names each reason that needs attention, such as a refused claim, with a count that links to the providers it names. When you may not list the registrations, the block says it cannot count claims and shows no refusal count, never a zero. **Providers** lists the transformer registrations: each claim, its catalog, what it provides, the instance or package that holds it and the operator's verdict, accepted and active shown apart. **Catalogs** lists the catalogs the Platform subscribes to, holds in its resolved registry, or a provider claims. Each count, provider and catalog links on: to **Installed** filtered by it, to the holder's **Provider** tab, or to a page for the catalog. That page shows one catalog as the cluster records it: where it comes from (the Platform's subscription, a provider's registration, or a claim only), whether the resolved registry holds it, since when and why it is not resolved, the registrations that claim it and their recent events.

A provider is any instance or package whose inventory holds a TransformerRegistration. The portal shows the operator's verdict on the claim as it is. Today the operator accepts a claim only from the ModuleInstance its `providerRef` names, so a package that ships one shows as a refused provider, with the operator's reason.

Filters live in the page address, so a filtered view is a link you can share. Each filter that offers choices lists the values present in the rows you may read, with the number of rows each stands for, and applies as you choose or type; without script, the Apply button shows. The Module filter on Installed matches a module path, and for a package its source. A reader who may list only some namespaces sees no counts for the lists locked to them and types a namespace instead of picking one. Your browser also remembers the theme you pick and the filters you last used on Installed and the Platform's two tabs, per cluster context; see [The portal's security model](/docs/operating/portal/portal-security/).

### The instance and package pages

An instance or package page opens with an identity card and three blocks: **Applied**, **Health** and, when the item holds a registration, **Provider**. Each block shows one state word, a summary, the reasons behind it and when it began. Applied also keeps a strip of the last attempts. A block is a state, never a verdict on the others: a page can read Applied and Degraded together.

Below the blocks, tabs open the views. **Graph** and **Resources** share a details panel. At rest the panel shows the owner itself; select a node or a row and it shows that object, and Clear selection brings the owner back. **Resources** is a table of the objects the inventory applied and what the cluster made below them, with the old ReplicaSets and each configuration group folded. **Events**, **Logs** and **YAML** each take one choice from a select. **Provider** appears for an owner that holds a registration and shows its claim, its conditions and who uses what it provides.

The **Events** tab merges the owner's own events with those of every object its inventory reaches and every Pod or ReplicaSet below them, so a failed image pull shows on the instance without a filter. Narrow it by resource or by type, or follow a resource name in a row. A note says when the feed is partial: some objects, or the events of some namespaces, are ones you may not read and are left out. The tab and its count refresh with the owner's own events; an event about a child alone shows the next time the page reads. **Logs** follows one container, the one you pick, in one open pane. **YAML** shows one object you may read, and a Secret is never offered.

### Every graph edge comes from a recorded field

Each instance and package page draws a graph. An instance graph starts at the instance and a package graph at the package, with its source first. Both run through the components to the inventory objects, then to the ReplicaSets, Pods and Jobs found through the owner references below those objects. The module is on the identity card, not in the graph. Each kind of edge has exactly one source field. The portal never renders a module to find an edge. The read API also serves a Platform graph of catalogs, registrations and the instances that provide them; the Platform page shows the same joins as rows instead, and where a registration's provider and the provider's inventory disagree, both are shown.

### The graph groups configuration and shows what you pick

Configuration components that hold RBAC rules, CustomResourceDefinitions or webhook configurations are folded into one box per kind family, so the workloads stay readable. A group box reads "Group", its family, and the count of objects by kind. Click a group to open it: its members move out of the box into a frame with a header and a Collapse pill, and the view fits the frame. One configuration group is open at a time, so opening another closes the first. Expand all opens every group, Collapse all closes them, and Overview, or the Esc key, goes back to the graph with nothing open. The Group configuration switch reads "Grouped" or "Ungrouped"; ungrouped shows every configuration component as a box of its own, with no frames.

A click on a box selects it: the details panel on the right shows it, and nothing dims. A link from the Resources tab spotlights a node instead: the graph dims everything outside the node's chain, its ancestors and its descendants, and centres the node. Hovering or focusing a box shows a card with its facts and dims the same chain while it lasts. A box that is not selected and not spotlit has the health of its object in its fill and outline, a square for "applied by the controller", and a dashed outline when the cluster made it. The legend under the graph names each mark.

Fit sizes the whole graph to the pane, and the slider or Ctrl and scroll zooms from 30 to 200 percent. The full screen button fills the window; Esc steps back one thing at a time: a card, a selection, an open group, then full screen. Motion follows your operating system's reduced-motion setting.

On a package graph the first node after the package is the source its `spec.sourceRef` names, read as you. It shows ready, not ready, or "No artifact yet". It shows "Kind not installed" until Flux serves the source kind; that clears within five minutes of installing Flux. The source box is locked when you may not read it, and the portal reads only the three Flux source kinds.

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
> An in-cluster mode, in which people sign in with OIDC and every read is checked for the signed-in user, is a future plan with no date: the portal's [roadmap](https://github.com/open-platform-model/opm-portal/blob/main/ROADMAP.md) lists it under future plans and its [design record](https://github.com/open-platform-model/opm-portal/blob/main/docs/DESIGN.md) holds the decisions. It is not built, and `opm-portal serve` runs local mode only.
