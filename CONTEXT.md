# opm-portal: where do I go?

Router only. Facts and rules are in `AGENTS.md`.

| Task | Read first | Then |
| --- | --- | --- |
| Add or change a path, field or handler of the read API | `openapi/` | `CONSTITUTION.md` (read API principle), `docs/DESIGN.md`, `references/docs-bundle.md` (the read-API reference page must list every path) |
| Edit a template, the page script, the CSS or a vendored asset | `references/ui-conventions.md` | `internal/ui/` and its golden files |
| Run or fix a Playwright browser test | `references/ui-conventions.md` | `test/browser/` |
| Change what the portal may read, write, show or log (RBAC, Secrets, `spec.values`, identity) | `CONSTITUTION.md` (read-only and least privilege) | `AGENTS.md`, `docs/DESIGN.md` |
| Change `internal/auth` or the launch token flow | `docs/DESIGN.md` | `references/ui-conventions.md` (the browser test of launch) |
| Change `deploy/` (the Pod manifest, its role) | `references/deploy.md` | `deploy/` |
| Start, use or delete the kind fixture cluster; recapture; read a failed nightly E2E run | `references/e2e-fixtures.md` | `test/e2e/`, `.github/workflows/` |
| Update a golden file or a file under `testdata/` | `references/e2e-fixtures.md` | `AGENTS.md` |
| Preview pages without a cluster | `references/ui-conventions.md` | |
| Edit a workflow under `.github/workflows/`, or the required `Lint` job | `references/release.md` | `RELEASING.md` at the workspace root |
| Fix a failed release, bump an OPM Go pin, or read the release gates | `references/release.md` | `RELEASING.md` at the workspace root |
| Choose a commit type or write a PR title | `CONSTITUTION.md` (versioning and commits) | `references/git-text.md` at the workspace root |
| Edit a page under `docs/site/`, `docs-kit.cue` or the `opm-docs` pin | `references/docs-bundle.md` | `STYLE.md` and `VOICE.md` at the workspace root |
| Start a feature change | `openspec/config.yaml` | `docs/DESIGN.md`, `ROADMAP.md` |
| Add or change a design decision | `docs/DESIGN.md` | `openspec/config.yaml` |
| Cite a portal decision or an enhancement in code or text | `AGENTS.md` | `references/enhancements.md` at the workspace root |
| Run, build, lint or test the code | `Taskfile.yml` | `AGENTS.md` |
