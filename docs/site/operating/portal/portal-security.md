---
title: "The portal's security model"
description: "What the portal reads, as whom, how local mode keeps other users and web pages out, and what it never shows."
type: explanation
weight: 30
---

The portal reads a cluster as you and shows you only what you could read yourself. In local mode, the only mode built, it runs on your machine and reads with your kubeconfig, so your RBAC is the whole boundary: the portal adds no credential, holds no role of its own and installs nothing in the cluster. What it adds is a set of rules about who may reach it on your machine, what it reads, and what it never shows, even to you.

This page assumes you know Kubernetes RBAC. To start the portal, see [Run the portal locally](/docs/operating/portal/run-the-portal-locally/).

## How it works

### Only your machine can reach it

Local mode listens on a loopback address only and refuses to start on any other address. A port on loopback is still open to every process and every user on the same machine, so loopback alone is not enough. The portal prints a link that carries a one-time launch token. The first request with the token receives a session cookie and a page that moves the browser on, and the token then works for no other browser. Every later request must carry that cookie. The session lasts at most 12 hours. A request without it is refused with `unauthenticated` before anything is read. Another user on a shared machine, or a process that finds the port, has neither the token nor the cookie.

### Other web pages cannot use it

A web page you visit in the same browser can send requests to a loopback address. Two defences stop it from reading the cluster through the portal. The portal refuses every request whose `Host` header names anything other than its own loopback address and port, which defeats DNS rebinding, where an attacker's domain is made to resolve to `127.0.0.1`. And the portal answers only `GET`, refuses any other method from another origin, never sets a permissive cross-origin header, and sends every response, refusals included, with a Content-Security-Policy that allows no script, style or other content to load, so another site's page can neither read its responses nor run script in them.

### Every read is checked before it is made

The portal learns who your kubeconfig authenticates as with a SelfSubjectReview when it starts, and refuses to start when the answer is an empty or anonymous user. Before each read, it asks the API server whether that identity may make it, with a SelfSubjectAccessReview for the exact verb, resource, namespace and name. The API server answers both reviews in its response and stores nothing. Only on an allow does the portal read. A resource you may not read is shown as locked, without being read. A review that fails or times out is never taken as an allow: nothing is read, the request is answered with `upstream_unavailable`, and an item inside a page is marked not readable.

Each answer is cached for up to 30 seconds, for that identity and that exact request. A permission granted or revoked in the cluster therefore takes effect in the portal within 30 seconds, also for a change stream or log stream already open.

The check runs on the request's attributes before anything is looked up. When you may not read a kind in a namespace, the portal gives the same `forbidden` answer whether the object exists or not, and a list you may not read is empty, with no count of what it hides. Inside a page you may read, an object you may not read is marked forbidden instead of failing the page.

### Nothing is written

The portal sends `get`, `list` and `watch` requests, and refuses any other verb before it sends it. The two reviews above are its only creates. It never reads a subresource that opens a channel into a workload, such as `exec`, `attach`, `portforward` or `proxy`.

### No Secret data, and no values

The portal never reads a Secret, in any mode. An inventory entry that names a Secret shows as withheld, and does not make the instance's health partial.

The portal does not show an instance's or a package's `spec.values`, and removes the `kubectl.kubernetes.io/last-applied-configuration` annotation from every object it serves, because a client-side apply copies the full values into it. Values are hidden as a whole because nothing in the stored values marks which of them are secret: the marks live in the module's schema.

Hiding the values does not hide what they became. A value a module renders into an object that is not a Secret, such as a ConfigMap entry or a container's environment variable, is readable to anyone who may read that object, in the portal as with `kubectl`. To keep a value out of reach, the module has to render it into a Secret.

### Messages are shown as written

Condition messages, status history and event notes are shown exactly as the operator and the API server wrote them. The portal does not remove secret values from them. In local mode this reveals nothing new: anyone who can open the portal can read the same text with `kubectl` and your kubeconfig. All cluster text, log lines included, is treated as untrusted and escaped before it reaches a page.

## Why it is built this way

### Why check first instead of reading and handling the refusal

The portal could send every read and turn the API server's refusal into a locked node. Checking first lets a page show what is locked before anything is read. It also keeps one place where access is decided, apart from the reads.

### Why a launch token on loopback

Notebook servers use the same defence for the same reason: loopback keeps the network out but not the other processes and users of the machine. A token that works once, traded for a cookie, ties the portal to the browser you opened it in.

## Common mistakes

### Hidden values are not hidden secrets

The portal hides `spec.values`, but a password a module renders into a ConfigMap or an environment variable is shown to anyone who may read that object. Read what your modules render before you rely on the portal for secrecy.

### Locked does not mean missing

A locked or forbidden item may exist. The portal answers the same for both on purpose, so a refusal tells you nothing about whether the object is there.

### The portal does not widen your access

The portal reads as you. If `kubectl auth can-i` says no, the portal shows the resource as locked.

## What enforces this

- The portal refuses to listen on any address other than loopback, refuses a request without the session cookie or after the session ends, and refuses a request whose `Host` is not its own address and port.
- The portal refuses any verb other than `get`, `list` and `watch`, any read of `secrets`, and any `exec`, `attach`, `portforward` or `proxy` subresource, before a request is sent.
- The API server decides every SelfSubjectAccessReview, from your RBAC.
- The portal's read model drops `spec.values` and the last-applied annotation before it stores an object, so no response can carry them.
- Nothing stops a module from rendering a value into a ConfigMap or an environment variable. That is the module author's choice.

> [!NOTE]
> **Direction**
>
> An in-cluster mode, with OIDC sign-in and a SubjectAccessReview for the signed-in user before every read, is designed in [enhancement 0030](/enhancements/0030/), a draft, and is not built. It would swap the self review for a review of the signed-in user in the one place access is decided, without changing a read path. It would send no self review: inside a cluster, a SelfSubjectAccessReview answers for the portal's own ServiceAccount, and a check that answers for the portal would let anyone read what the portal may read. Whether the operator removes secret values from condition and event messages is an open question there, to be answered before that mode is released.
