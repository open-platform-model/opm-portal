## 1. Go module, version binary and task gates

- [x] 1.1 Add `go.mod` (`module github.com/open-platform-model/opm-portal`, `go 1.26.0`, no requirements) and verify `go mod tidy` leaves it unchanged
- [x] 1.2 Add `internal/version` (`Version` constant annotated `x-release-please-version`, `Full()`) with tests for the semver shape, the annotation line and the `+g` suffix; verify `go test ./internal/version` passes
- [x] 1.3 Add `cmd/opm-portal` (no args, `version`, `--version` print `opm-portal v<version>`, exit 0; anything else prints usage to stderr, exit 2) with a table test; verify `go run ./cmd/opm-portal` prints the line
- [x] 1.4 Add `Taskfile.yml` (`build`, `run`, `fmt`, `vet`, `lint`, `lint:fix`, `test`, `tidy`, `clean`, `openspec:install`, `openspec:check`, `check`), `.golangci.yml` (cli rules), `.gitignore` (with `.claude/worktrees/`, `/bin`, `/dist`); verify `task build` writes `bin/opm-portal`
- [x] 1.5 `task check` green, then commit `chore(build): add the Go module, version binary and task gates`

## 2. Repository guides

- [x] 2.1 Add `LICENSE` as a byte copy of opm-operator's; verify with `cmp`
- [x] 2.2 Rewrite `README.md`: what the portal is, status (nothing built), link to enhancement 0030, build commands; verify every command it names exists in `Taskfile.yml`
- [x] 2.3 Add `AGENTS.md`: the attribution, bare-`@` and 250-word sections verbatim from opm-operator (verify with `diff`), then purpose, entrypoint, layout, security rules (read-only V1, no Secret data, fail closed on empty identity, no tokens in logs), registry, release, commands, enhancement references, OpenSpec routing, verification checklist
- [x] 2.4 Add `CONSTITUTION.md` mirroring the principles of `openspec/config.yaml`; verify the eight principle titles match
- [x] 2.5 `task check` green, then commit `docs: add the licence and repository guides`

## 3. Pull request checks

- [x] 3.1 Add `hack/release-pin-check.sh` (G1 without the operator's `cue.mod` clauses) behind a new `task deps:release-check`; verify it prints `release-pin-check: ok` on this tree and fails on a scratch copy whose `go.mod` has a `replace`
- [x] 3.2 Add `.github/workflows/lint.yml` (job `name: Lint`: G1 on `release-please--*` heads, `task openspec:check`, golangci-lint v2.11.3) and `.github/workflows/test.yml` (job `name: Test`, `task test`); verify no other job is named `Lint`
- [x] 3.3 Add `.github/workflows/pr-title.yml` copied from the cli and `.github/dependabot.yml` from opm-operator (ignoring `github.com/open-platform-model/*` and docs-kit)
- [x] 3.4 `actionlint` and `task check` green, then commit `ci: add the Lint, Test and PR-title checks`

## 4. Release pipeline

- [x] 4.1 Add `release-please-config.json` (0.x: `bump-minor-pre-major`, `initial-version` `0.1.0`, draft, tags `vX.Y.Z`, `extra-files` the version file, opm-operator's `changelog-sections`) and `.release-please-manifest.json` (`0.0.0`); verify both parse with `jq`
- [x] 4.2 Add `.goreleaser.yml` (linux and darwin, amd64 and arm64, archives `opm-portal-<os>-<arch>`, `checksums.txt`, attach to the existing draft without publishing); verify with `goreleaser check` when available, otherwise by review against the cli config
- [x] 4.3 Add `Dockerfile` (golang builder on `$BUILDPLATFORM`, distroless `static:nonroot`, `USER 65532:65532`) and `.dockerignore`; verify an image builds and runs `opm-portal` when podman or docker is available
- [x] 4.4 Add `.github/scripts/image-tag-guard.sh` (copied) and `.github/scripts/release-guard.sh` (asset list: the four archives and `checksums.txt`) and `.github/workflows/release.yml` (release-please, image-release, binaries, publish-release); verify the guard scripts pass `bash -n` and the workflow passes `actionlint`
- [x] 4.5 `actionlint` and `task check` green, then commit `ci(release): add release-please, goreleaser binaries and the release image`
