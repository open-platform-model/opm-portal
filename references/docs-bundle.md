# The opm-portal docs bundle

Covers the `opm-portal` docs bundle: how `docs/site/` becomes a bundle, the `opm-docs` pins and the rules for the pages.

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

- Bumping `.opm-docs-version` without the `publish.yml@` refs: `docs:pins:check` fails.
- A page outside `operating/portal/` and `reference/portal/`: the bundle owns only those two paths.
- Adding an API path without a line in `reference/portal/read-api.md`: `TestReadAPIReferenceListsEveryPath` fails.

## Check before you finish

- [ ] `task docs:bundle:check` passes after any change under `docs/site/` or to `docs-kit.cue`.
- [ ] `.opm-docs-version` and every `publish.yml@` ref name the same release.
- [ ] Every new page follows `STYLE.md` ("Site Pages") and `VOICE.md`.
