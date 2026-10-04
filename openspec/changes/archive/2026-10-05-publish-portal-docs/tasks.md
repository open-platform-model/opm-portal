## 1. Adopt docs-kit with the portal pages

- [x] 1.1 Pin docs-kit: `.opm-docs-version` (`v0.7.0`), `.tasks/opm-docs.sh` copied unchanged from opm-operator, `/out/` in `.gitignore`
- [x] 1.2 `docs-kit.cue`: one bundle `opm-portal`, placement `docs` at `/docs/` owning `operating/portal/` and `reference/portal/`, version from `v` tags, one `markdown` source `docs/site`
- [x] 1.3 `Taskfile.yml`: `tools:opm-docs`, `docs:bundle`, `docs:pins:check`, `docs:bundle:check`
- [x] 1.4 `.github/workflows/docs.yml` (check, edge, dispatch), `publish-docs` in `release.yml`, the pin check step in `lint.yml`; `actionlint` clean
- [x] 1.5 `docs/site/operating/portal/`: `_index.md`, `about-the-portal.md` (explanation: what the portal is), `run-the-portal-locally.md` (how-to against `opm-portal serve --kubeconfig --context --open`, opening with the not-yet-released alert), `portal-security.md` (explanation: loopback, launch token, `Host` check, SelfSubjectAccessReview per read, no Secret data, values hidden and what they became, messages verbatim)
- [x] 1.6 `docs/site/reference/portal/`: `_index.md` and `read-api.md` (resources, stream topics, problem codes, link to `openapi/v1alpha1.yaml`)
- [x] 1.7 `task docs:bundle:check` and `task check` green, then commit `docs: publish the portal pages as a docs-kit bundle`

## 2. Hold the reference to the OpenAPI document

- [x] 2.1 `internal/api`: `TestReadAPIReferenceListsEveryPath` compares the reference page's resource table with the OpenAPI paths, both ways
- [x] 2.2 `task check` green, then commit `test(api): hold the read API reference to the OpenAPI paths`

## 3. Repository guides

- [x] 3.1 `AGENTS.md` (layout, commands) and `README.md` name the docs bundle and its tasks
- [x] 3.2 `task docs:bundle:check` and `task check` green, then commit `docs: name the docs bundle in the repository guides`
