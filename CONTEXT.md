# opm-portal: where do I go?

Router only. Facts and rules are in `AGENTS.md`.

| Task | Read first | Then |
| --- | --- | --- |
| Change a handler, a wire type or a path of the read API | `openapi/v1alpha1.yaml` | `docs/DESIGN.md`, `CONSTITUTION.md` Principle III, then `references/release.md` (breaking-change gate, docs path list) |
| Change a template, the page script, the CSS or a vendored asset | `references/ui-conventions.md` | `internal/ui/` and its golden files |
| Change what the portal may read, write, show or log (RBAC, Secrets, `spec.values`, identity) | `CONSTITUTION.md` Principle V | `AGENTS.md` Security Rules, `docs/DESIGN.md` |
| Change `internal/auth` or the launch flow | `references/ui-conventions.md` (browser tests) | `docs/DESIGN.md` |
| Change `deploy/` (the Pod manifest) | `references/ui-conventions.md` (`deploy/` section) | `deploy/manifest_test.go`, `references/e2e-fixtures.md` (`e2e:pod`) |
| Start, use or tear down the kind fixture cluster; recapture; a failed nightly E2E run | `references/e2e-fixtures.md` | `test/e2e/`, `.github/workflows/e2e.yml` |
| Update golden files or a file under `testdata/` | `references/e2e-fixtures.md` | `AGENTS.md` Commands |
| See the pages without a cluster | `references/ui-conventions.md` (dev server) | |
| Run or fix a browser test | `references/ui-conventions.md` (browser tests) | `test/browser/` |
| Pick a commit type, write a PR title, fix a release, bump a pin | `references/release.md` | `RELEASING.md` at the workspace root |
| Edit a site page or the docs bundle | `references/release.md` (docs bundle) | `STYLE.md` and `VOICE.md` at the workspace root |
| Start a feature change | `openspec/config.yaml` | `docs/DESIGN.md`, `ROADMAP.md` |
| Cite a portal decision, or an enhancement, in code or text | `AGENTS.md` Citations | `references/enhancements.md` at the workspace root |
| Add or change a design decision | `docs/DESIGN.md` | `openspec/config.yaml` |
| Run, build, lint or test | `Taskfile.yml` | `AGENTS.md` Commands |
| Commit, open a PR or write GitHub text | `references/git-text.md` at the workspace root | |
