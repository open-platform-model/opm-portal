---
title: "Run the portal locally"
description: "Start the portal on your machine with your kubeconfig and open it in your browser."
type: how-to
weight: 20
---

Run the portal locally to see what a cluster runs under OPM with your own access, without installing anything in the cluster. The portal reads as the identity your kubeconfig authenticates as, so it shows what your RBAC lets you read and nothing more.

> [!IMPORTANT]
> **No web UI yet**
>
> The portal's web UI is not built. The browser shows the read API's JSON.

## Before you begin

- A kubeconfig for a cluster that runs OPM: the operator, or instances applied with the `opm` CLI.
- Read access, as that kubeconfig's identity, to the OPM resources you want to see: `get`, `list` and `watch` on `moduleinstances`, `modulepackages`, `platforms` and `transformerregistrations` in the group `opmodel.dev`, and on the objects their inventories name. The portal shows a resource you may not read as locked.
- To build from source: Go, at the version in the portal's `go.mod`.

## Steps

1. Install `opm-portal`.

   Build it from source on `main`:

   ```sh
   go install github.com/open-platform-model/opm-portal/cmd/opm-portal@main
   ```

   Go writes the binary to `$(go env GOPATH)/bin`. Make sure that directory is on your `PATH`.

1. Start the portal with your kubeconfig and the context to read:

   ```sh
   opm-portal serve --kubeconfig ~/.kube/config --context my-cluster --open
   ```

   Without `--kubeconfig`, the portal reads `$KUBECONFIG`, then `~/.kube/config`. Without `--context`, it uses the kubeconfig's current context. To read ModuleInstances and ModulePackages in some namespaces only, add `--namespaces team-a,team-b`.

   The portal listens on `127.0.0.1` on a free port and prints a link to open once:

   ```text
   Open this link once to sign in: http://127.0.0.1:<port>/launch?token=<token>
   ```

   `--open` opens that link in your default browser. Without `--open`, copy the link into a browser on the same machine.

   The browser trades the launch token for a session cookie, and the token works only once. To open the portal in another browser, stop the portal and start it again.

1. Leave the command running while you use the portal. Press Ctrl-C to stop it; the session ends with it. A session also ends 12 hours after the launch. To get a new link, restart the portal.

## Check that it worked

After the launch, the browser lands on the instance list of the read API:

```text
http://127.0.0.1:<port>/api/v1alpha1/clusters/default/instances
```

The response is a JSON `InstanceList`. Its `access` field is `ok` when your identity may list ModuleInstances, and each item shows an instance's applied state and health. An `access` of `forbidden` with no items means your identity may not list ModuleInstances in that scope: check your access with:

```sh
kubectl --context my-cluster auth can-i list moduleinstances.opmodel.dev --all-namespaces
```

A request from a browser that never opened the launch link is refused with the problem code `unauthenticated`.

## Related

- [The OPM portal](/docs/operating/portal/about-the-portal/): what the portal shows.
- [The portal's security model](/docs/operating/portal/portal-security/): why it listens on loopback only and what the launch token protects.
- [Portal read API](/docs/reference/portal/read-api/): every resource you can read.
