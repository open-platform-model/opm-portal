# opm-portal

A web portal for [Open Platform Model](https://opmodel.dev) clusters: a versioned JSON and
server-sent-events read API (`/api/v1alpha1`) with an HTMX UI as its first consumer.

## Status

Milestone 1 runs: `opm-portal serve` serves the web pages and the read API on your machine,
reading the cluster as your kubeconfig's user. To install a release, download it from the
[GitHub releases page](https://github.com/open-platform-model/opm-portal/releases) as
[the how-to for running the portal locally](docs/site/operating/portal/run-the-portal-locally.md)
describes.

The design is enhancement
[0030](https://github.com/open-platform-model/enhancements/tree/main/0030) in the OPM
enhancements repo.

> **Direction (enhancement 0030).** The first version is read-only. It shows what a cluster runs
> under OPM, from the Platform down to Pods: status as the operator reports it, events, pod logs
> and a relationship graph. It first runs on your machine with your kubeconfig, then in-cluster
> with OIDC login, where every read is authorized as the signed-in user. It creates, edits and
> deletes nothing.

## Run it locally

```bash
task build
./bin/opm-portal serve --open                      # current kubeconfig context
./bin/opm-portal serve --context kind-dev          # another context
./bin/opm-portal serve --namespaces team-a,team-b  # only these namespaces
```

`serve` prints one link, `Open this link once to sign in: http://127.0.0.1:<port>/launch?token=...`,
and with `--open` opens it in your default browser. The link works once: it trades its token for
a session cookie and answers with the Platform page. Stop the portal with Ctrl-C; restart it for
a new link.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--kubeconfig` | `$KUBECONFIG`, then `~/.kube/config` | the kubeconfig to read the cluster with |
| `--context` | the current context | the kubeconfig context |
| `--addr` | `127.0.0.1:0` (a free port) | the address to listen on; only loopback addresses are accepted |
| `--namespaces` | all | read ModuleInstances and ModulePackages only in these namespaces, for users who cannot list them cluster-wide |
| `--open` | off | open the link in the default browser |

## Pages

| Page | Shows |
| --- | --- |
| `/` Platform | catalog subscriptions and the resolved registry with versions, registrations with separate *accepted* and *active* pills and their verdicts, contracts without a provider (information, not a failure), conditions with what each reason means and what to do, the platform graph, recent events |
| `/instances`, `/packages` | every ModuleInstance or ModulePackage you may list, filterable by namespace, each with two badges: **Applied** (what the operator applied) and **Health** (what the portal sees running) |
| `/instances/<ns>/<name>`, `/packages/<ns>/<name>` | the relationship graph, components with their objects and Pods, conditions, the contracts the render used (as text: they are not the instance's provider demand), the operator's history, recent events, live logs per container, and a YAML view of any object |

Applied and Health are never merged: an instance whose rollout is broken shows **Applied** and
**Degraded** side by side. Anything you may not read shows **locked**, with nothing about it
beyond what the read API says. Events are recent activity: Kubernetes keeps them for about an
hour. Pages update live over one stream per browser tab, and need no script to read (logs and
live updates do).

The YAML view shows an object as the cluster serves it, without managed fields, the
`kubectl.kubernetes.io/last-applied-configuration` annotation or an instance's `spec.values`.
**A value a module rendered into a non-Secret object (a ConfigMap entry, a container's
environment) is visible there to anyone who may read that object, as it is in `kubectl`.** Keep
a value out of reach by rendering it into a Secret; the portal never reads Secrets.

What it does with your access:

- **Reads as you.** At startup it asks the cluster who your kubeconfig authenticates as (a
  SelfSubjectReview) and refuses to start for an empty or anonymous user. Before every read it asks
  whether you may make it (a SelfSubjectAccessReview), and shows what you may not read as
  forbidden instead of reading it. Those two reviews are the only objects it creates; the API
  server stores neither. It never reads a Secret and never shows an instance's `spec.values`.
  A value that became part of another object, such as a ConfigMap entry or a container's
  environment, shows to anyone who may read that object, as it does in `kubectl`.
- **Listens on loopback only** and answers only requests addressed to `127.0.0.1:<port>`,
  `localhost:<port>`, `[::1]:<port>` or the loopback IP `--addr` names, so a web page that
  rebinds its own name to 127.0.0.1 gets nothing.
- **Admits one browser.** Requests without the session cookie get `401`. Cross-site writes are
  refused. Every response carries a Content-Security-Policy that allows nothing to load; pages
  widen it only to the portal's own scripts, styles, fonts and connections, with no inline script
  or style. htmx, its SSE extension and the fonts are built into the binary at pinned versions;
  nothing loads from the internet.
- **Logs** go to standard error and never hold the link's token, the cookie or kubeconfig
  content. The link is printed to standard output only. `--open` hands it to the browser through
  a private file, not a command line other local users could read, and removes that file as soon
  as the link's token is spent, or when the portal stops if it never is.

The session cookie is `opm-portal-<port>` with `HttpOnly`, `SameSite=Strict`, `Path=/`, no
`Domain` and a 12-hour `Max-Age`. The portal serves plain HTTP on loopback, so the cookie has no
`Secure` flag and cannot use the `__Host-` prefix, which requires it. Browsers send a loopback
cookie to every port on that address, so a program serving another port on 127.0.0.1 could
receive it if your browser visits that program; treat the portal like any other local
developer server.

## Build

Requires Go (the version in `go.mod`), [Task](https://taskfile.dev) and
[golangci-lint](https://golangci-lint.run) v2.

```bash
task build              # bin/opm-portal
./bin/opm-portal        # prints the version (serve runs it, above)
task check              # fmt, vet, lint, openspec, test, capture check
```

`task openspec:check` needs the `openspec` CLI (`task openspec:install`, needs npm).

`task e2e:up` builds a throwaway kind cluster with the released opm-operator and a set of test
modules, `task e2e:capture` snapshots it into `testdata/clusters/f1/`, `task e2e:local` runs the
built binary in local mode against it, `task e2e:m1` checks the milestone 1 views through it
(including a scripted image break it reverts), and `task e2e:down` deletes it. The `E2E`
workflow runs them, and the browser tests, every night. They need kind (podman by default, `E2E_PROVIDER=docker` otherwise), kubectl, yq and
jq, and never touch a cluster other than `opm-portal-e2e` (or the `opm-portal-e2e-<suffix>`
cluster `E2E_CLUSTER` names).

## Documentation

The user documentation lives in [`docs/site/`](docs/site/) and is published to opmodel.dev as a
docs-kit bundle by the `Docs` workflow. `task docs:bundle:check` builds and lints it locally.

## Contributing

Read [`AGENTS.md`](AGENTS.md) and [`CONSTITUTION.md`](CONSTITUTION.md) first. Changes are planned
as OpenSpec changes under `openspec/changes/`.

## Licence

Apache-2.0, see [`LICENSE`](LICENSE).
