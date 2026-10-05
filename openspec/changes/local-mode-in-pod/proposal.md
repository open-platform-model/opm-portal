## Why

The owner asked how much work is left before the portal can be used in a cluster, still with
local auth only, as a tool to try it out, and then said "go". A manual run on kind (2026-10-05)
showed the stock image already works in a Pod: bound to `127.0.0.1:8090`, client-go falls back
to the in-cluster config, the SelfSubjectReview names the Pod's ServiceAccount, the launch link
appears in `kubectl logs`, and `kubectl port-forward 8090:8090` reaches the loopback-bound server.
What is missing is everything around it: a manifest to apply, a read-only role, a published image
the manifest names, a recorded decision that this is allowed and what it trusts, and a test that
keeps it working.

## What Changes

- New decision portal:D13 in `docs/DESIGN.md`: local mode may run in a Pod as a single-user test
  tool, reading as its ServiceAccount. D5's decision text, R1 and alternatives are amended in
  place to point at D13, and the numbering header names D13. `AGENTS.md`, `CONSTITUTION.md` and
  `openspec/config.yaml` get a one-line carve-out to the "never fall back to the portal's own
  ServiceAccount" rule: it still binds in-cluster mode (portal:D6).
- New `deploy/` directory: a `kustomization.yaml` and plain YAML for a Namespace `opm-portal`, a
  ServiceAccount, a ClusterRole `opm-portal-reader` with an explicit read-only rule list, its
  ClusterRoleBinding, and a one-replica Deployment running `serve --addr 127.0.0.1:8090` under a
  restricted security context, with no Service, Ingress or probe. Its image line carries the
  `x-release-please-version` marker, and release-please's `extra-files` names the file.
- A Go test parses `deploy/*.yaml` and refuses a write verb, Secrets, impersonate, a wildcard, a
  Service or Ingress, a non-loopback bind and a loosened security context; a second test holds the
  image tag to `internal/version`.
- The startup log names `source=in-cluster` instead of `context=""` when no kubeconfig was loaded
  and client-go used the in-cluster config. This is the only Go behaviour change.
- A new e2e task `task e2e:pod` builds the image with the e2e provider, loads it into the fixture
  cluster, applies a test overlay of `deploy/` and waits for the rollout; `TestPod` (build tag
  `e2e`) port-forwards `8090:8090`, launches with the token from the Pod log, checks the instance
  list against the cluster's, and checks that a forward from another local port is refused. The
  nightly E2E workflow runs it after `e2e:m1`.
- README section "Try it in a cluster"; ROADMAP line.

Not in this change: in-cluster mode (portal:D6), OIDC, SubjectAccessReview, per-user access, a
Service or Ingress, an automated catalog-coverage check of the role (portal:D11:R2 stays with the
in-cluster plan), and a docs-bundle page (no existing page covers running the portal in a
cluster, and the image is not released yet).

## Capabilities

### New Capabilities

- `deploy-manifest`: the shipped manifest that runs local mode in a Pod: what objects it holds,
  the read-only role, the restricted Pod, the image pin and the tests that hold them.

### Modified Capabilities

- `local-mode`: local mode may run in a Pod with in-cluster credentials, reached only through a
  port-forward to the bound port; the startup log names the in-cluster source.
- `kind-e2e-environment`: `task e2e:pod` checks the image in a Pod on the fixture cluster, and the
  nightly workflow runs it.

## Impact

- Packages: `cmd/opm-portal` (startup log line). New Go test package `deploy` holding only tests.
  No API resource or UI page changes.
- New files: `deploy/` (user-copyable, no `portal:` citations), `test/e2e/pod/` overlay,
  `test/e2e/pod.sh`, `cmd/opm-portal/pod_e2e_test.go`.
- Principle V: the portal reads as a different identity here, the Pod's ServiceAccount, through a
  role that grants only `get`, `list` and `watch` (and `get` on `pods/log`), no Secrets, no
  impersonate, no write verb and no wildcard; a test enforces it. Local mode's self reviews are
  allowed to every authenticated identity by `system:basic-user`, so the role carries no
  `create`. Everyone who holds the launch token reads what the ServiceAccount may read; getting
  the token needs `pods/log` and reaching the portal needs `pods/portforward` in the `opm-portal`
  namespace, so those grants are the boundary (portal:D13). An empty identity still fails closed.
  Nothing else changes: no Secret data, no values, no credential in a log.
- Principle VII: no new dependency. The manifest test uses `sigs.k8s.io/yaml`, already in
  `go.mod`; the e2e test drives `kubectl` as a subprocess.
- SemVer: MINOR after 1.0 (a new install artifact). On the 0.x line it ships as
  `feat(deploy): run local mode in a Pod as a test tool` and cuts a minor release.
- Decisions: adds portal:D13; amends portal:D5 (decision text, R1, alternatives).
