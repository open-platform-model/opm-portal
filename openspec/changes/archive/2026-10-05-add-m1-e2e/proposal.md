## Why

Milestone 1 is feature-complete, but its delivery evidence (enhancement 0030, "Delivery
evidence", milestone 1) is checked against captures and by hand only: no test reads a live
fixture cluster through the binary and asserts that the Platform shows the accepted and the
refused claim, that the instance list keeps Applied and Health apart, that a scripted image
break turns Health Degraded within seconds while Applied stays, that a CLI-owned instance reads
Managed externally, or that a namespace-scoped identity sees its namespace and locked items
only. `task e2e:local` and `task test:browser` exist but run only by hand, so a change to
`internal/auth` or the launch path can bring back the `--open` 401 with no red check (issue 21,
item 1). And `TestBrowserLogs` serves a hand-copied logs region, so a template change that moves
`.logs` off the region's direct children would silently bring back the scroll reset on refresh
(issue 23, item 2).

## What Changes

- New e2e test `TestM1` (build tag `e2e`) and `task e2e:m1`: runs the built binary in local mode
  against the fixture cluster and asserts the milestone 1 reads through the JSON API and the SSE
  stream, including a scripted image break it reverts when it ends.
- Unit test in `internal/ui`: the owner page's rendered `#logs` section has `.logs` as a direct
  child holding the log panes in the shape the page script and `TestBrowserLogs` rely on.
- The nightly E2E workflow also runs `task e2e:local` and `task e2e:m1` on the fixture cluster,
  and a parallel `Browser` job runs `task test:browser` with docker. The Playwright image is
  pinned by digest. On failure the run uploads the test logs, a dump of the fixture cluster's
  OPM objects, Pods and events, and browser screenshots.

## Capabilities

### Modified Capabilities

- `kind-e2e-environment`: the milestone 1 e2e test, and the nightly workflow running the local,
  milestone 1 and browser tests with artifacts on failure.

## Impact

- Files: `cmd/opm-portal` (`e2e_test.go`, a new `m1_e2e_test.go`, `browser_test.go`),
  `test/e2e` (`m1.sh`, `dump.sh`), `test/browser` (screenshots on failure), `internal/ui` (a
  test), `Taskfile.yml` (`e2e:m1`, `e2e:dump`), `.github/workflows/e2e.yml`, `AGENTS.md`,
  `README.md`. No production code changes.
- API and pages: none. No new dependency.
- Principle V: the portal's reads and verbs are unchanged. The test itself writes to the
  throwaway fixture cluster with the fixture kubeconfig (a ServiceAccount, Role and RoleBinding,
  as `TestLocalModeNamespaces` already does, and a patch of podinfo's `spec.values` that it
  reverts); the portal never does.
- Workflow: least-privilege `contents: read`, no secrets, still never on pull requests.
- SemVer: none (test and CI only); ships as `test(e2e)`, which does not release.
- Enhancement link: produces the milestone 1 delivery evidence of 0030 (0030:D3:R2/R3,
  0030:D4:R4, 0030:D5:R5/R7) without claiming a decision, so it carries no `enhancement.yaml`.
