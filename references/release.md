# Releases and release workflows

Covers how this repo releases: release-please, the gates on a release PR, the immutable-tag recovery steps, the OPM Go pin and the workflows under `.github/workflows/`.

The workspace files `RELEASING.md` (policy, "Pin classes", "Gates") and `references/releases.md` own the cross-repo rules. `CONSTITUTION.md`, Principle VI, owns the 0.x line, the breaking-change rule and the commit-type table. This file holds what is specific to this repo.

## Release mechanics

- release-please runs on `main` as the `opm-release-please` App. Tags are `vX.Y.Z` with no component. The first release was `0.1.0`.
- The PR title is the CHANGELOG entry (Principle VI). Write it as the entry you want.

## Outside the release cascade

This repo is a leaf of the cascade. Dependabot ignores `github.com/open-platform-model/*`. An OPM Go pin moves by hand as a `fix(deps)` PR.

## Gates on a release PR

- `task deps:release-check` (gate G1) runs `hack/release-pin-check.sh`. It runs as a step of the `Lint` job on `release-please--*` PRs. It fails on a `go.mod` `replace`, and on a pseudo-version or untagged OPM Go pin.
- Keep the job name `Lint` unique. The `main` ruleset requires a check with that name, so a second job called `Lint` breaks the required check.
- A workflow change needs `actionlint`. The `E2E` workflow and `Cite lint` have their own notes in their file headers.
- `task api:breaking` fails on a breaking change to `openapi/v1alpha1.yaml` unless the PR title carries `!`. It compares against `BASE=<ref>` (default `origin/main`); `PR_TITLE` is optional. The `Lint` job runs it on every pull request. Install the pinned `oasdiff` with `task api:install-oasdiff`. Keep that pin equal to the one in `.github/workflows/lint.yml`.

## Release tags are immutable

The workspace rule is in `references/releases.md` at the workspace root. What it means here:

- A release is a draft until the `publish-release` job of `release.yml` publishes it. That job runs `.github/scripts/release-guard.sh publish` after the binaries, `checksums.txt` and the signed image exist.
- `:vX.Y.Z` on GHCR is written only when absent. `.github/scripts/image-tag-guard.sh` probes the tag and reuses an existing image only when its revision label matches.
- Recover a failed run with "Re-run failed jobs", never "Re-run all jobs". "Re-run all jobs" re-runs release-please and strands the draft.
- After a publish, a bad release is fixed by releasing the next version.

## Pitfalls

- Re-running all jobs of a failed release: it strands the draft release. Re-run failed jobs only.
- Adding a `replace` or a pseudo-version OPM pin on a branch that later becomes a release PR: G1 fails.
- Using `docs`, `test`, `ci`, `build` or `chore` for a change that must release: it will not release.

## Check before you finish

- [ ] The PR title is the CHANGELOG entry you want, with `!` for a breaking change.
- [ ] `actionlint` passes after any workflow change.
- [ ] `task api:breaking` passes, or the title carries `!`.
- [ ] `task deps:release-check` passes on a release PR.
- [ ] No tag was moved, deleted or re-created.
