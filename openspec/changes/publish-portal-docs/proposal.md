## Why

Enhancement 0030 puts the portal's documentation on opmodel.dev, and 0030:D8:R4 requires that
documentation to say that values rendered into non-Secret objects stay readable to anyone who
may read those objects. Every other OPM repository publishes its pages as a docs-kit bundle that
the site pulls; the portal publishes nothing, so the site has nothing to pull and the security
posture of local mode (0030:D5) is written down only in the enhancement and the code.

## What Changes

- Adopt docs-kit as the operator and the cli do: a repo-root `.opm-docs-version` (`v0.7.0`), a
  `docs-kit.cue` declaring one bundle, `opm-portal`, placed in a site version's `/docs/` tree
  (not a tab), `.tasks/opm-docs.sh` (identical to the other adopters'), the `docs:*` tasks, and a
  `docs.yml` workflow calling docs-kit's `publish.yml` pinned to the same release: `check` on
  pull requests, `edge` on pushes to `main`, `release` and `revision` on dispatch.
- `release.yml` gains a `publish-docs` job that publishes the release's bundle once release-please
  cuts a release. The `Lint` job refuses a `publish.yml` ref that names another release than
  `.opm-docs-version` (offline, no install).
- Four authored pages under `docs/site/`, in the page dialect of the workspace `STYLE.md`
  ("Site Pages") and the voice of `VOICE.md`:
  - `operating/portal/` (a section the bundle owns): what the portal is (explanation), run the
    portal locally (how-to, written against `opm-portal serve --kubeconfig --context --open`,
    which the parallel `add-local-mode` change adds, and marked as not yet released), and the
    portal's security model (explanation: loopback only, launch token, `Host` check,
    SelfSubjectAccessReview per read, no Secret data, values hidden, messages shown verbatim).
  - `reference/portal/read-api.md`: the read API's resources and problem codes, linking
    `openapi/v1alpha1.yaml` as the full contract. A test holds the page's resource table to the
    OpenAPI document's paths, so a new or removed route fails until the page follows.
- `AGENTS.md` and `README.md` name the docs bundle and its tasks.

## Capabilities

### New Capabilities

- `portal-docs`: the portal's documentation bundle: what it holds, how and when it is published,
  how its docs-kit pins agree, and what the read API reference and the security page must state.

### Modified Capabilities

None.

## Impact

- Files: `docs-kit.cue`, `.opm-docs-version`, `.tasks/opm-docs.sh`, `Taskfile.yml`, `.gitignore`
  (`/out/`), `.github/workflows/docs.yml` (new), `release.yml` and `lint.yml`, `docs/site/`, one
  test in `internal/api` (the package that already holds the OpenAPI contract tests).
- No Go code, no API resource and no UI page changes. Principle V: unaffected; the pages describe
  the posture, the code keeps it.
- Principle VII: no Go dependency. The workflow adds docs-kit's reusable workflow, pinned by its
  immutable release tag as every adopter pins it (docs-kit README, "Using the workflow").
- Owner steps after merge: GHCR creates `ghcr.io/open-platform-model/docs/opm-portal` private on
  the first `edge` push, and it must be made public before the site can pull it. The site change
  that pulls the bundle is separate (opmodel.dev).
- SemVer: PATCH after 1.0 (documentation and CI only). The PR title is `docs:`, which cuts no
  release on the 0.x line.
- Enhancement link: lands 0030:D8:R4 (the documentation statement) and documents 0030:D5. No
  decision is claimed in `enhancement.yaml`: D8's YAML views and pages, and D5's local mode
  itself, are other changes'.
