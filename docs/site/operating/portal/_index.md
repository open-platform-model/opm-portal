---
title: "The portal"
description: "A read-only web portal and read API that show what OPM runs in a cluster, from the Platform down to Pods."
weight: 60
---

The OPM portal shows what a cluster runs under OPM in one place: the Platform, its catalogs and registrations, every ModuleInstance and ModulePackage, the objects each one applied, and the Pods below them, with events and logs beside them. It reads; it never changes anything in the cluster.

> [!IMPORTANT]
> **No release yet**
>
> `opm-portal` has no release. Its read API is built, but no released binary serves it yet, and the web UI is not built. Each page in this section says which part it describes is not in a release.

## How-to guides

- [Run the portal locally](/docs/operating/portal/run-the-portal-locally/): start the portal on your machine with your kubeconfig and open it in your browser.

## Explanations

- [The OPM portal](/docs/operating/portal/about-the-portal/): what the portal shows about a cluster, where each fact comes from, and why applied state and health are two values.
- [The portal's security model](/docs/operating/portal/portal-security/): what the portal reads, as whom, how local mode keeps other users and web pages out, and what it never shows.

## Reference

- [Portal read API](/docs/reference/portal/read-api/): every resource of the portal's read API, the topics of its change stream, and its problem codes.
