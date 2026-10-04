## Context

The repo holds one seed commit (`README.md`). Its `main` ruleset is live (PR-only, squash only,
admin bypass in pull-request mode) and requires no check until this change adds `Lint`
(workspace `RELEASING.md`, "Rulesets on main"). The workspace already lists the portal as a
releasing repo on the 0.x line, outside the release cascade as a leaf (workspace `RELEASING.md`,
tier table), so its Go pins will move by hand as `fix(deps)`.

Models, read on 2026-10-04: `opm-operator` (licence, `AGENTS.md` shared sections, constitution
shape, `lint.yml`, `release.yml`, `.github/scripts/*`, `release-please-config.json`,
`hack/release-pin-check.sh`, `Dockerfile`, `.dockerignore`, `.gitignore`, `dependabot.yml`,
`internal/version`) and `cli` (`go.mod` directive `go 1.26.0`, `.golangci.yml`, `Taskfile.yml`,
`pr-title.yml`, the `openspec:check` task, `.goreleaser.yml`).

## Goals / Non-Goals

**Goals:**

- `task check` and the CI checks are green on a tree with no portal code, so the next change
  only adds code.
- The release pipeline is complete before the first `feat`, so `0.1.0` ships binaries, an image
  and checksums from its first run.

**Non-Goals:**

- No HTTP server, no `/healthz`: the binary prints its version only. The server arrives with the
  first change that has something to serve.
- No Go dependencies. The operator API module is added by the first change that imports it
  (`go mod tidy` would drop an unused requirement).
- No docs-kit bundle, `docs.yml`, `.opm-docs-version` or `docs:*` tasks (change
  `publish-portal-docs`).
- No `image-pr.yml`: images are pushed on release only.

## Decisions

### Version burned into source, rewritten by release-please

`internal/version` holds `const Version = "0.0.0" // x-release-please-version`, listed in
`extra-files`, and `Full()` appends `+g<rev>[.dirty]` from `debug.ReadBuildInfo`, exactly as
opm-operator does. Alternative: cli-style `-ldflags -X` injection. Rejected because every build
path (goreleaser, Dockerfile, `go install`) would have to cooperate; the source form needs none.

### Release: operator flow plus goreleaser binaries

`release.yml` keeps the operator's jobs and guards: `release-please` (App token),
`image-release` (`image-tag-guard.sh` probe and verify, buildx multi-arch, cosign keyless
signature, SPDX SBOM attestation, build provenance), and `publish-release`
(`release-guard.sh publish`, the single publish point). It adds a `binaries` job that runs
goreleaser against the existing draft (`use_existing_draft`, `draft: true`,
`mode: keep-existing`, `replace_existing_artifacts`), so goreleaser attaches the four archives and
`checksums.txt` and never publishes. `release-guard.sh` checks exactly those five assets.

Options considered:

1. cli flow (goreleaser builds binaries and the image with `dockers_v2`, then publishes). Fewer
   jobs, but it has no version-tag overwrite guard, no SBOM or provenance, and publishes before a
   separate image job could fail.
2. Operator flow plus a goreleaser job (chosen). The image guard and supply-chain steps are
   copied, not new work, and the release stays a draft until both artifact jobs succeed.

The operator's `publish-examples`, `publish-docs` and `install.yaml` steps are dropped. QEMU is
dropped too: the Dockerfile's builder stage runs on `$BUILDPLATFORM` and cross-compiles, and the
final distroless stage has no `RUN`, so no target-platform emulation is needed.

### 0.x versioning

`release-please-config.json` sets `bump-minor-pre-major: true` (a breaking change bumps the
minor), `bump-patch-for-minor-pre-major: false` (a `feat` bumps the minor),
`initial-version: 0.1.0`, no `versioning: prerelease`. The manifest starts at `0.0.0`.
`changelog-sections` are opm-operator's: `deps` and `refactor` shown, `docs` hidden.

### Lint carries every gate

`Lint` (unique job name) runs, in order: the G1 release-pin gate on `release-please--*` heads
(`task deps:release-check`, `hack/release-pin-check.sh` with the operator's `cue.mod` clauses
removed), `task openspec:check` (openspec 1.12.0 from npm), and golangci-lint v2.11.3 through
`golangci-lint-action` with `skip-cache: true` (the cli's reason). One required check carries
all three, because a skipped separate job counts as passing.

### Dockerfile

Operator shape: `golang:1.26` builder on `$BUILDPLATFORM`, `CGO_ENABLED=0`, cache mounts,
`gcr.io/distroless/static:nonroot`, `USER 65532:65532`. It copies `go.*` rather than
`go.mod go.sum`, because a module with no dependencies has no `go.sum`. `.dockerignore`
re-includes only Go sources and module files; a later change that embeds web assets must
re-include them.

## Risks / Trade-offs

- [The release pipeline is untested until the first release] → The jobs are copied from
  pipelines that have released (opm-operator beta.3 onwards, cli goreleaser settings); actionlint
  checks the workflow syntax. The first `0.1.0` run is the proof, and a failed run leaves a
  draft that "Re-run failed jobs" finishes.
- [release-please may compute the first version from the manifest's `0.0.0` instead of
  `initial-version`] → For a `feat`, both paths give `0.1.0`. A first release driven only by a
  `fix` could come out as `0.0.1`; the first portal change is a `feat`.
- [The App, secrets, tag rulesets and immutable releases are owner settings] → Listed in the PR
  and in proposal Impact. Without the App the release-please job fails and nothing is tagged.
- [The GHCR package is created private on first push] → The owner makes it public after the
  first release (workspace bootstrap checklist).
