## 1. Kind environment and the F1 fixture set

- [x] 1.1 Add `test/e2e/versions.env` (kind version, pinned node image, `OPM_CLI_VERSION` and the four `OPM_CLI_SHA256_<os>_<arch>` values from the release's `checksums.txt`, optional `OPM_OPERATOR_VERSION`)
- [x] 1.2 Add `test/e2e/lib.sh` (fixed cluster, context and `.e2e/kubeconfig`; `KUBECONFIG` unset; `k()`/`opm_k()` wrappers with explicit `--kubeconfig`/`--context`; loopback-server guard; polling wait helper) and `test/e2e/down.sh`
- [x] 1.3 Add the F1 fixtures under `test/e2e/fixtures/f1/` (cert-manager with its applier, podinfo, ModulePackage without Flux, backup provider and consumer copied from opm-operator `main`, CLI-owned `web/` instance package)
- [x] 1.4 Add `test/e2e/up.sh` (existing-cluster refusal, podman or docker create with the pinned image, checksum-verified CLI download, operator install, F1 apply in order, settle rules, verdict summary); verify `bash -n` and `shellcheck` when available, and that a forged pin makes the download step exit non-zero
- [x] 1.5 Add `e2e:up`, `e2e:down` to `Taskfile.yml` and `/.e2e/` to `.gitignore`
- [x] 1.6 `task check` green, then commit `test(e2e): add a scripted kind environment with the F1 fixture set`

## 2. Capture into testdata

- [x] 2.1 Add `test/e2e/capture.sh` (OPM kinds, inventory objects without Secrets, labelled ReplicaSets and Pods, fixture-namespace events, `meta.yaml` with versions and verdicts; strip `managedFields`, last-applied, MI/MP `spec.values`; sorted lists) and `test/e2e/check-capture.sh`; add `e2e:capture` and `e2e:capture:check` tasks
- [x] 2.2 Verify `check-capture.sh` fails on a scratch file holding a Secret, and on one holding a ModuleInstance with `spec.values`
- [x] 2.3 Run `task e2e:up` on a new throwaway podman cluster `opm-portal-e2e`, then `task e2e:capture`, then `task e2e:down`; confirm no kind cluster remains
- [x] 2.4 Add `testdata/clusters/f1/README.md` (how the capture was made, its versions, the claim verdict and why)
- [x] 2.5 `task check` green, then commit `test(e2e): capture the F1 fixture cluster into testdata`

## 3. Scheduled CI run

- [x] 3.1 Add `.github/workflows/e2e.yml` (job `E2E`; `workflow_dispatch` and a nightly `schedule` only; pinned actions; kind from the `KIND_VERSION` pin; `task e2e:up E2E_PROVIDER=docker`, `task e2e:capture`, artifact upload, `task e2e:down` under `if: always()`); add a `task e2e:capture:check` step to the `Test` job
- [x] 3.2 Update `AGENTS.md` (layout, commands) and `README.md` where they list tasks
- [x] 3.3 `actionlint` and `task check` green, then commit `ci(e2e): run the kind environment and capture nightly and on demand`

## 4. Review fixes

- [x] 4.1 `check-capture.sh`: walk `testdata/clusters/` recursively (any extension, Markdown excepted), check every document as a List's items or a single object, fail on unparseable files; verify against a bare Secret, a `.yml` List, a nested `.json`, a multi-document file and an unparseable file
- [x] 4.2 Pin the catalog (`--skip-platform` plus a Platform at `OPM_CATALOG_VERSION`), the operator (`OPM_OPERATOR_VERSION=v1.0.0-beta.6`) and the web instance's nginx digest
- [x] 4.3 Record the provider in `.e2e/provider`, refuse a `kind` other than the pin, re-hash the kept CLI archive on every run, clean the download dir on any exit, drop Node events; add `e2e:capture:check` to `task check` and the gate lists
- [x] 4.4 Fail the nightly run on a `meta.yaml` verdict or version change
- [x] 4.5 Recapture on a new throwaway podman cluster, update `testdata/clusters/f1/README.md`, delete the cluster; `task check` green, then commit `test(e2e): recapture F1 on operator v1.0.0-beta.6 and catalog 4.6.0`
