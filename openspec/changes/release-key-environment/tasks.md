## 1. Guard the release key and the release builds

- [ ] 1.1 `.github/workflows/release.yml`: `release-please` declares `environment: release` and `permissions: {}`; `binaries` sets `cache: false` on `actions/setup-go`; `image-release` drops `cache-from` and `cache-to`. Verify: `grep -n "RELEASE_APP_PRIVATE_KEY\|environment:\|cache" .github/workflows/release.yml` shows the key only in the job that declares `environment: release` and no `type=gha`.
- [ ] 1.2 `.github/CODEOWNERS` naming the owners on `/.github/`, `/Taskfile*.yml`, `/release-please-config.json` and `/.release-please-manifest.json`. Verify: every path exists in the tree.
- [ ] 1.3 `actionlint` and `task check` green, then commit `ci: read the release key only in the main-only release environment`.
