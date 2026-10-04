## Why

`RELEASE_APP_PRIVATE_KEY` is an org secret with no environment, so a workflow run on any branch of this repository can read it on a push event and act as the release App, which may create release tags. The owner decided (release-cascade security pass, 2026-10-04) to move the key, unrotated, into a main-only `release` environment in every repository that reads it; the environment already exists here with a `main` branch policy. The same audit found two release jobs that restore an Actions cache another run may have written (the Go build cache in `binaries`, the BuildKit `type=gha` cache in `image-release`), and no file naming code owners for CI and release config.

## What Changes

- **`.github/workflows/release.yml`**: the `release-please` job, the only reader of the key, runs in `environment: release` and declares `permissions: {}` (it acts only through the App token). `binaries` sets `cache: false` on `actions/setup-go`; `image-release` drops `cache-from`/`cache-to: type=gha`, so release images build from scratch.
- **`.github/CODEOWNERS`**: the owners review `.github/`, `Taskfile.yml`, `release-please-config.json` and `.release-please-manifest.json`.
- Every other workflow already declares its permissions, and none of them publishes, so they keep their caches. `.github/dependabot.yml` already covers GitHub Actions.

Release class: none. Commits are `ci` and `chore`.

## Capabilities

### Modified Capabilities

- `build-and-release`: the release key is read only in the main-only `release` environment, and release artifacts build without an Actions cache.

## Impact

- Files: `.github/workflows/release.yml`, `.github/CODEOWNERS`.
- Release image builds lose the BuildKit cache; both platforms cross-compile in the builder stage, so a release run takes a few minutes longer.
- Risk: until the owner stores the key in the environment, the job reads the org secret as before, so this merges safely first. A wrong `permissions: {}` would show on the next push to `main` as a failed release-please step.
