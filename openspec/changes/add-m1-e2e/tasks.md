## 1. Milestone 1 e2e test (cmd/opm-portal, test/e2e)

- [x] 1.1 `e2e_test.go`: share `fixtureCluster`, `buildPortal` and `restConfig` with the new test
- [x] 1.2 `m1_e2e_test.go`: `TestM1` (claims, the two axes, the CLI-owned instance, the image break with its revert) and `TestM1NamespaceReader` (own namespace only, forbidden elsewhere, locked objects, stream topics)
- [x] 1.3 `test/e2e/m1.sh` and `task e2e:m1`; run it on a throwaway podman cluster, then `task e2e:local` after it
- [x] 1.4 `task check` and `golangci-lint run --build-tags e2e,browser ./cmd/...` green, then commit `test(e2e): check the milestone 1 reads end to end on the fixture cluster`

## 2. Logs region shape (internal/ui)

- [x] 2.1 Unit test: the owner page's `#logs` section has `.logs` as a direct child, each log pane a direct child of it with its summary, tools and pane, on the F1 and image-break captures
- [x] 2.2 `task check` green, then commit `test(ui): hold the logs region to the shape the page script refreshes`

## 3. Nightly workflow (.github/workflows, test, AGENTS.md)

- [ ] 3.1 Pin the Playwright image by digest; screenshots on failure into `OPM_PORTAL_BROWSER_SHOTS`
- [ ] 3.2 `test/e2e/dump.sh`: OPM objects, Pods and events of the fixture cluster, no Secrets
- [ ] 3.3 `e2e.yml`: `e2e:local` and `e2e:m1` in the `E2E` job, a parallel `Browser` job, artifacts on failure; `actionlint`
- [ ] 3.4 `AGENTS.md`: `task e2e:m1` and the nightly coverage
- [ ] 3.5 `task check` and `actionlint` green, then commit `ci(e2e): run the local, milestone 1 and browser tests nightly`
