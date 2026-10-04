## 1. Guard the release key and the release builds

- [x] 1.1 `.github/workflows/release.yml`: `release-please` declares `environment: release` and `permissions: {}`; `binaries` sets `cache: false` on `actions/setup-go`; `image-release` drops `cache-from` and `cache-to`. Verify: `grep -n "RELEASE_APP_PRIVATE_KEY\|environment:\|cache" .github/workflows/release.yml` shows the key only in the job that declares `environment: release` and no `type=gha`.
- [x] 1.2 `.github/CODEOWNERS` naming the owners on `/.github/`, `/Taskfile*.yml`, `/release-please-config.json` and `/.release-please-manifest.json`. Verify: every path exists in the tree.
- [x] 1.3 `actionlint` and `task check` green, then commit `ci: read the release key only in the main-only release environment`.

## 2. Review follow-ups (PR 11)

- [x] 2.1 The App token step requests only `permission-contents`, `permission-pull-requests` and `permission-issues` write; the `binaries`, `image-release` and `publish-release` checkouts set `persist-credentials: false`; CODEOWNERS adds `/.goreleaser.yml`, `/Dockerfile` and `/.dockerignore` and states review is enforced only by the ruleset; dependabot ignores `open-platform-model/.github*`. The delta and main spec name the token scope and the checkouts. Verify: `actionlint` and `task check` green.
