## 1. Kind environment and the F1 fixture set

- [ ] 1.1 Add `test/e2e/versions.env` (kind version, pinned node image, `OPM_CLI_VERSION` and the four `OPM_CLI_SHA256_<os>_<arch>` values from the release's `checksums.txt`, optional `OPM_OPERATOR_VERSION`)
- [ ] 1.2 Add `test/e2e/lib.sh` (fixed cluster, context and `.e2e/kubeconfig`; `KUBECONFIG` unset; `k()`/`opm()` wrappers with explicit `--kubeconfig`/`--context`; loopback-server guard; polling wait helper) and `test/e2e/down.sh`
- [ ] 1.3 Add the F1 fixtures under `test/e2e/fixtures/f1/` (cert-manager with its applier, podinfo, ModulePackage without Flux, backup provider and consumer copied from opm-operator `main`, CLI-owned `web/` instance package)
- [ ] 1.4 Add `test/e2e/up.sh` (existing-cluster refusal, podman or docker create with the pinned image, checksum-verified CLI download, operator install, F1 apply in order, settle rules, verdict summary); verify `bash -n` and `shellcheck` when available, and that a forged pin makes the download step exit non-zero
- [ ] 1.5 Add `e2e:up`, `e2e:down` to `Taskfile.yml` and `/.e2e/` to `.gitignore`
- [ ] 1.6 `task check` green, then commit `test(e2e): add a scripted kind environment with the F1 fixture set`

## 2. Capture into testdata

- [ ] 2.1 Add `test/e2e/capture.sh` (OPM kinds, inventory objects without Secrets, labelled ReplicaSets and Pods, fixture-namespace events, `meta.yaml` with versions and verdicts; strip `managedFields`, last-applied, MI/MP `spec.values`; sorted lists) and `test/e2e/check-capture.sh`; add `e2e:capture` and `e2e:capture:check` tasks
- [ ] 2.2 Verify `check-capture.sh` fails on a scratch file holding a Secret, and on one holding a ModuleInstance with `spec.values`
- [ ] 2.3 Run `task e2e:up` on a new throwaway podman cluster `opm-portal-e2e`, then `task e2e:capture`, then `task e2e:down`; confirm no kind cluster remains
- [ ] 2.4 Add `testdata/clusters/f1/README.md` (how the capture was made, its versions, the claim verdict and why)
- [ ] 2.5 `task check` green, then commit `test(e2e): capture the F1 fixture cluster into testdata`

## 3. Scheduled CI run

- [ ] 3.1 Add `.github/workflows/e2e.yml` (job `E2E`; `workflow_dispatch` and a nightly `schedule` only; pinned actions; kind from the `KIND_VERSION` pin; `task e2e:up E2E_PROVIDER=docker`, `task e2e:capture`, artifact upload, `task e2e:down` under `if: always()`)
- [ ] 3.2 Update `AGENTS.md` (layout, commands) and `README.md` where they list tasks
- [ ] 3.3 `actionlint` and `task check` green, then commit `ci(e2e): run the kind environment and capture nightly and on demand`
