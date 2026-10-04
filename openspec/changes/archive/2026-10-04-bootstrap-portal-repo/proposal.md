## Why

`open-platform-model/opm-portal` exists with a seed README and a `main` ruleset that requires no
check, so no portal change can be planned, gated or released yet. This change gives the repo the
guides, the build, the CI gates and the release pipeline the sibling repos have, before the first
line of portal code, so every later change (the authorization seam, the read model, the read API)
lands under the same rules.

## What Changes

- Repository guides: `LICENSE` (Apache-2.0, the operator's text), `README.md` (what the portal is,
  that nothing is built yet, a link to enhancement 0030), `AGENTS.md` (attribution, bare-`@`,
  250-word PR rules verbatim from the siblings; the portal's security rules; routing to
  OpenSpec), `CONSTITUTION.md` (the reader-friendly form of `openspec/config.yaml`).
- Go module `github.com/open-platform-model/opm-portal` (`go 1.26.0`, as the cli) with no
  dependencies, an `internal/version` package whose version release-please rewrites, and a
  `cmd/opm-portal` binary that prints its version and starts no server.
- `Taskfile.yml` (`build`, `fmt`, `vet`, `lint`, `test`, `openspec:check`, `deps:release-check`,
  `check`), a cli-style `.golangci.yml`, `.gitignore` (with `.claude/worktrees/`) and
  `.dockerignore`.
- GitHub workflows: a `Lint` job (the check the `main` ruleset will require; it carries the G1
  release-pin gate on release PRs and the strict OpenSpec validation), a `Test` job, the
  PR-title check copied from the cli, and Dependabot ignoring `github.com/open-platform-model/*`.
- Release pipeline: release-please on the 0.x line (first release `0.1.0`; `docs`, `test`, `ci`,
  `build`, `chore` hidden; `deps` and `refactor` shown) as the `opm-release-please` App;
  goreleaser binaries for linux and darwin on amd64 and arm64 with `checksums.txt`; a distroless
  nonroot image published to `ghcr.io/open-platform-model/opm-portal` on release only, signed,
  with an SBOM and provenance; the draft release published last, after every asset is attached.

Not here: any server, API or UI code (later changes), the docs-kit bundle and `docs.yml` (change
`publish-portal-docs`, after the docs placement decision), PR image builds, `install.yaml` (the
in-cluster milestone), and joining the release cascade.

## Capabilities

### New Capabilities

- `build-and-release`: what the repo's binary reports about its version, which checks a pull
  request must pass, and what a release produces (version line, binaries, checksums, image).

### Modified Capabilities

None (the repo has no specs yet).

## Impact

- New files only; the seed `README.md` is rewritten.
- SemVer class: none. The PR is titled `chore:`, so release-please cuts no release; the first
  `feat` after it releases `0.1.0`.
- Owner follow-ups outside the repo tree: install the `opm-release-please` App and set
  `RELEASE_APP_CLIENT_ID` / `RELEASE_APP_PRIVATE_KEY`, require the `Lint` check in the `main`
  ruleset, add the repo to the tag rulesets and enable immutable releases before the first
  release, and make the GHCR package public after the first image push.
- Enhancement link: the portal is designed in enhancement 0030; this change implements no 0030
  decision, so it carries no `enhancement.yaml`.
