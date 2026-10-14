# Releases and the docs bundle

Covers the release line, the gates on a release PR, the immutable-tag recovery steps and the `opm-portal` docs bundle with its pins. Read it before you pick a commit type, touch a release workflow, change a pin, or edit `docs/site/`.

The workspace files `RELEASING.md` (policy, "Pin classes", "Gates") and `references/releases.md` own the cross-repo rules. This file holds what is specific to this repo.

## The release line

- release-please runs on `main` as the `opm-release-please` App. Tags are `vX.Y.Z` with no component. The first release was `0.1.0`.
- The line stays below `1.0.0` until the read API declares stability. A breaking change (`!` in the PR title) bumps the minor version, never the major.
- Squash merges carry the PR title only (`squash_merge_commit_message: BLANK`). The PR title is therefore the CHANGELOG entry. Write it as the entry you want.
- The migration note of a breaking change goes in the PR body, which the CHANGELOG entry links.

## The commit type decides the release

| Type | Releases |
| --- | --- |
| `feat`, `fix`, `perf`, `revert`, `deps`, `refactor` | yes |
| `docs`, `test`, `ci`, `build`, `chore` | no |

Recommended scopes: `api`, `ui`, `authz`, `auth`, `readmodel`, `graph`, `health`, `stream`, `logs`, `version`, `release`.

## Outside the release cascade

This repo is a leaf of the cascade. Dependabot ignores `github.com/open-platform-model/*`. An OPM Go pin moves by hand as a `fix(deps)` PR.

## Gates on a release PR

- `task deps:release-check` (gate G1) runs `hack/release-pin-check.sh`. It runs as a step of the `Lint` job on `release-please--*` PRs. It fails on a `go.mod` `replace`, and on a pseudo-version or untagged OPM Go pin.
- Keep the job name `Lint` unique. The `main` ruleset requires a check with that name.
- `task api:breaking` fails on a breaking change to `openapi/v1alpha1.yaml` unless the PR title carries `!`. It compares against `BASE=<ref>` (default `origin/main`); `PR_TITLE` is optional. The `Lint` job runs it on every pull request. Install the pinned `oasdiff` with `task api:install-oasdiff`. Keep that pin equal to the one in `.github/workflows/lint.yml`.

## Release tags are immutable

The workspace rule is in `references/releases.md` at the workspace root. What it means here:

- A release is a draft until the `publish-release` job of `release.yml` publishes it. That job runs `.github/scripts/release-guard.sh publish` after the binaries, `checksums.txt` and the signed image exist.
- `:vX.Y.Z` on GHCR is written only when absent. `.github/scripts/image-tag-guard.sh` probes the tag and reuses an existing image only when its revision label matches.
- Recover a failed run with "Re-run failed jobs", never "Re-run all jobs". "Re-run all jobs" re-runs release-please and strands the draft.
- After a publish, a bad release is fixed by releasing the next version.

## The docs bundle

The `opm-portal` docs bundle is built from `docs/site/` by `opm-docs`, the docs-kit tool, as `docs-kit.cue` configures. opmodel.dev pulls it into its `/docs/` tree.

| Command | What it does |
| --- | --- |
| `task docs:bundle` | Builds the bundle of the work tree into `out/opm-portal/` (a local preview of edge) |
| `task docs:bundle:check` | Runs `docs:pins:check`, then builds and lints the bundle into a temporary directory |
| `task docs:pins:check` | Refuses a docs-kit `publish.yml` ref under `.github/workflows/` that names another release than `.opm-docs-version`. Offline. The `Lint` job runs it |
| `task tools:opm-docs` | Installs or reuses `.bin/opm-docs`, the checksum-verified docs-kit release that `.opm-docs-version` names |

Rules:

- `.opm-docs-version` and every `publish.yml@` ref are pins of one release. Bump them together in one PR. The workflow pins `publish.yml` by docs-kit release tag, never by SHA, because the signing certificate names it at that ref.
- Pages under `docs/site/` follow `STYLE.md` ("Site Pages") and `VOICE.md` at the workspace root.
- Pages stay under the two paths the bundle owns: `operating/portal/` and `reference/portal/`.
- `TestReadAPIReferenceListsEveryPath` fails until `reference/portal/read-api.md` lists every path in `openapi/v1alpha1.yaml`. A new API path therefore needs a docs edit in the same PR.

## Pitfalls

- Re-running all jobs of a failed release: it strands the draft release. Re-run failed jobs only.
- Bumping `.opm-docs-version` without the `publish.yml@` refs: `docs:pins:check` fails.
- Adding a `replace` or a pseudo-version OPM pin on a branch that later becomes a release PR: G1 fails.
- A page outside `operating/portal/` and `reference/portal/`: the bundle owns only those two paths.
- Using `docs` as the type for a change that must release (for example a user-visible fix in a template): it will not release.

## Check before you finish

- [ ] The PR title is the CHANGELOG entry you want, with `!` for a breaking change.
- [ ] `task docs:bundle:check` passes after any change under `docs/site/` or to `docs-kit.cue`.
- [ ] `task api:breaking` passes, or the title carries `!`.
- [ ] `task deps:release-check` passes on a release PR.
- [ ] No tag was moved, deleted or re-created.
