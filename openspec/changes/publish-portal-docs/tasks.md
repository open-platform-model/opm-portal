## 1. Adopt docs-kit with the first page

- [ ] 1.1 Pin docs-kit: `.opm-docs-version` (`v0.7.0`), `.tasks/opm-docs.sh` copied unchanged from opm-operator, `/out/` in `.gitignore`
- [ ] 1.2 `docs-kit.cue`: one bundle `opm-portal`, placement `docs` at `/docs/` owning `operating/portal/` and `reference/portal/`, version from `v` tags, one `markdown` source `docs/site`
- [ ] 1.3 `Taskfile.yml`: `tools:opm-docs`, `docs:bundle`, `docs:pins:check`, `docs:bundle:check`
- [ ] 1.4 `docs/site/operating/portal/_index.md` and `about-the-portal.md` (explanation: what the portal is)
- [ ] 1.5 `.github/workflows/docs.yml` (check, edge, dispatch), `publish-docs` in `release.yml`, the pin check step in `lint.yml`; `actionlint` clean
- [ ] 1.6 `task docs:bundle:check` and `task check` green, then commit `ci(docs): publish the portal docs as a docs-kit bundle`

## 2. Local mode and security pages

- [ ] 2.1 `docs/site/operating/portal/run-the-portal-locally.md` (how-to against `opm-portal serve --kubeconfig --context --open`, opening with the not-yet-released alert)
- [ ] 2.2 `docs/site/operating/portal/portal-security.md` (explanation: loopback, launch token, `Host` check, SelfSubjectAccessReview per read, no Secret data, values hidden and what they became, messages verbatim)
- [ ] 2.3 `task docs:bundle:check` and `task check` green, then commit `docs: add the local mode and security pages`

## 3. Read API reference

- [ ] 3.1 `docs/site/reference/portal/_index.md` and `read-api.md` (resources, stream topics, problem codes, link to `openapi/v1alpha1.yaml`)
- [ ] 3.2 `internal/api`: `TestReadAPIReferenceListsEveryPath` compares the page's resource table with the OpenAPI paths, both ways
- [ ] 3.3 `task docs:bundle:check` and `task check` green, then commit `docs: add the read API reference`

## 4. Repository guides

- [ ] 4.1 `AGENTS.md` (layout, commands) and `README.md` name the docs bundle and its tasks
- [ ] 4.2 `task docs:bundle:check` and `task check` green, then commit `docs: name the docs bundle in the repository guides`
