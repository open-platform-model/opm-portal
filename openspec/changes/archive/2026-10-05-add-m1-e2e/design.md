## Context

The fixture cluster (`task e2e:up`, fixture set F1) carries every state milestone 1 renders: a
cluster-scoped module, an accepted and active provider claim with its consumer, a deliberately
refused claim, a ModulePackage without Flux and a CLI-owned instance. Two hand-run tests read it
through the built binary today: `TestLocalMode` (the launch and the front door) and
`TestLocalModeNamespaces`. Nothing reads the views milestone 1 promises off a live cluster, and
the nightly E2E workflow only builds and captures the cluster.

## Goals / Non-Goals

**Goals:**

- Assert the milestone 1 views through the binary's read API and stream on a live cluster.
- Run that test, `task e2e:local` and `task test:browser` nightly, with artifacts on failure.
- Close the gap between `TestBrowserLogs`' hand-copied page and the owner template.

**Non-Goals:**

- Running any of it on pull requests (registries outside the repo; a 2.4 GB browser image).
- Milestone 2 (OIDC, SubjectAccessReview): its e2e is a later change.

## Decisions

- The milestone 1 test is `TestM1` and `TestM1NamespaceReader` in `cmd/opm-portal` (build tag
  `e2e`), run by `task e2e:m1` through `test/e2e/m1.sh`, beside `task e2e:local`. It reuses
  that test's helpers: start the binary, launch a session, read with it.
- The image break patches the podinfo ModuleInstance's `spec.values.image.tag` to a tag that
  does not exist, the experiment 01 break, and reverts it (merge patch `image: null`) in a
  cleanup that waits until the Deployment has rolled back and only running Pods remain, so a
  later test (or `e2e:local`'s log follow) reads a settled fixture. It runs last in `TestM1`.
- "Within seconds" is measured against the cluster, not the patch: a poller lists podinfo's
  Pods every 250 ms and records when a container first waits on `ErrImagePull` or
  `ImagePullBackOff`; the stream MUST say Degraded no later than 10 s after that. The time from
  patch to pull failure depends on the registry, not on the portal.
- "Applied stays" is asserted on the document that turns Degraded and on every upsert for 5 s
  after it (never Failed or Stalled, still Degraded), and on the instance list. It is not
  asserted on every document after the patch: the live run showed a passing `Reconciling`
  about 100 ms after the patch while the operator applies it, which is the operator's truth.
- The namespace-scoped identity is the ServiceAccount `TestLocalModeNamespaces` already creates
  (OPM kinds in `default` only), run with `--namespaces default`.
- `TestBrowserLogs` keeps its synthetic page (it needs a fixed render sequence and fixed ids),
  and a unit test in `internal/ui` holds the owner template to the shape that page copies:
  `section#logs > div.logs > details.log`, each pane with its summary, tools and `pre.log-pane`.
  Rendering the browser page through `ui.New` would need a fake API that changes between
  renders; the unit test is the lighter tier and fails on the regression issue 23 names.
- The nightly workflow keeps one `E2E` job for everything that needs the cluster: up, capture,
  the verdict comparison, `e2e:local`, `e2e:m1`, then down. The test steps run when the cluster
  came up even if the comparison failed, so one drift does not hide the other results. The
  browser tests need no cluster, so they run in a parallel `Browser` job with docker.
- The Playwright image is pinned by its index digest beside its tag. It is not cached: on hosted
  runners a pull from MCR takes about as long as restoring a multi-gigabyte tarball from the
  Actions cache, and the cache would take a large share of the repository's 10 GB quota.
- On failure the run uploads the test output, a dump of the fixture cluster's OPM objects, Pods
  and events (`task e2e:dump`, never Secrets or values), and the browser screenshots the Playwright
  scripts save when `OPM_PORTAL_BROWSER_SHOTS` names a directory.

## Risks / Trade-offs

- [The test writes to the fixture cluster] → only the throwaway `opm-portal-e2e*` cluster
  through its own kubeconfig, which `lib.sh` checks is on loopback; the patch is reverted.
- [A slow registry makes the pull failure late] → the bound is measured from the cluster's own
  report, so only the portal's lag can fail it.
- [A failed revert leaves podinfo broken] → the cleanup fails the test; the cluster is
  throwaway and `e2e:down` deletes it.
