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
- Linux or macOS, on `amd64` or `arm64`.

## Steps

1. Install `opm-portal` from the [portal's releases page](https://github.com/open-platform-model/opm-portal/releases). Each release has one archive per system, named `opm-portal-<os>-<arch>.tar.gz`, and a `checksums.txt`. For Linux on `amd64`:

   ```sh
   curl -fsSLO https://github.com/open-platform-model/opm-portal/releases/latest/download/opm-portal-linux-amd64.tar.gz
   curl -fsSLO https://github.com/open-platform-model/opm-portal/releases/latest/download/checksums.txt
   grep ' opm-portal-linux-amd64.tar.gz$' checksums.txt | sha256sum --check
   tar -xzf opm-portal-linux-amd64.tar.gz opm-portal
   sudo install opm-portal /usr/local/bin/
   ```

   On macOS, use `darwin` for the system and `shasum -a 256 --check` in place of `sha256sum --check`. Use `arm64` for the architecture on an ARM machine.

   To build from source instead, with Go at the version in the portal's `go.mod`, run `go install github.com/open-platform-model/opm-portal/cmd/opm-portal@latest`. Go writes the binary to `$(go env GOPATH)/bin`.

1. Start the portal with your kubeconfig and the context to read:

   ```sh
   opm-portal serve --kubeconfig ~/.kube/config --context my-cluster --open
   ```

   Without `--kubeconfig`, the portal reads `$KUBECONFIG`, then `~/.kube/config`. Without `--context`, it uses the kubeconfig's current context.

   The portal listens on `127.0.0.1` on a free port and prints a link to open once:

   ```text
   Open this link once to sign in: http://127.0.0.1:<port>/launch?token=<token>
   ```

   `--open` opens that link in your default browser. Without `--open`, copy the link into a browser on the same machine. To pick the port or another loopback address, pass `--addr`, such as `--addr 127.0.0.1:8080`. The portal refuses to start on an address that is not loopback.

   The link answers with a short page that gives the browser a session cookie and moves it on to the read API. The token works only once: the browser that opened it may open it again while its session lasts, and any other browser is refused. To open the portal in another browser, stop the portal and start it again.

1. If the portal warns at startup that your kubeconfig's user may not list and watch a kind cluster-wide, your access is limited to some namespaces. The warning's message reads:

   ```text
   the kubeconfig's user may not list and watch this kind cluster-wide, so it reads as forbidden; pass --namespaces with the namespaces you may read
   ```

   Stop the portal and start it again with the namespaces you may read:

   ```sh
   opm-portal serve --context my-cluster --namespaces team-a,team-b --open
   ```

   `--namespaces` limits the ModuleInstances and ModulePackages the portal reads to those namespaces. A warning about a kind in a namespace you named means your user may not read that kind there either. A warning about a cluster-scoped kind, such as Platform or TransformerRegistration, stays after the restart, and that kind reads as forbidden unless your user may list it cluster-wide.

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

If you started the portal with `--namespaces`, the cluster-wide list stays `forbidden` even when the portal works. Open the list for one of your namespaces instead:

```text
http://127.0.0.1:<port>/api/v1alpha1/clusters/default/instances?namespace=team-a
```

Check your access in that namespace with:

```sh
kubectl --context my-cluster auth can-i list moduleinstances.opmodel.dev -n team-a
```

A request from a browser that never opened the launch link is refused with the problem code `unauthenticated`.

## Related

- [The OPM portal](/docs/operating/portal/about-the-portal/): what the portal shows.
- [The portal's security model](/docs/operating/portal/portal-security/): why it listens on loopback only and what the launch token protects.
- [Portal read API](/docs/reference/portal/read-api/): every resource you can read.
