# opm-portal

A web portal for [Open Platform Model](https://opmodel.dev) clusters: a versioned JSON and
server-sent-events read API (`/api/v1alpha1`) with an HTMX UI as its first consumer.

## Status

Nothing is built yet. This repository holds the build, the checks and the release pipeline; the
`opm-portal` binary only prints its version. There is no release.

The design is enhancement
[0030](https://github.com/open-platform-model/enhancements/tree/main/0030) in the OPM
enhancements repo.

> **Direction (enhancement 0030).** The first version is read-only. It shows what a cluster runs
> under OPM, from the Platform down to Pods: status as the operator reports it, events, pod logs
> and a relationship graph. It first runs on your machine with your kubeconfig, then in-cluster
> with OIDC login, where every read is authorized as the signed-in user. It creates, edits and
> deletes nothing.

## Build

Requires Go (the version in `go.mod`), [Task](https://taskfile.dev) and
[golangci-lint](https://golangci-lint.run) v2.

```bash
task build              # bin/opm-portal
./bin/opm-portal        # prints the version
task check              # fmt, vet, lint, openspec, test, capture check
```

`task openspec:check` needs the `openspec` CLI (`task openspec:install`, needs npm).

`task e2e:up` builds a throwaway kind cluster with the released opm-operator and a set of test
modules, `task e2e:capture` snapshots it into `testdata/clusters/f1/`, and `task e2e:down`
deletes it. They need kind (podman by default, `E2E_PROVIDER=docker` otherwise), kubectl, yq and
jq, and never touch a cluster other than `opm-portal-e2e`.

## Contributing

Read [`AGENTS.md`](AGENTS.md) and [`CONSTITUTION.md`](CONSTITUTION.md) first. Changes are planned
as OpenSpec changes under `openspec/changes/`.

## Licence

Apache-2.0, see [`LICENSE`](LICENSE).
