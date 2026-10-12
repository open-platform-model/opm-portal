## Why

A comment that cites an enhancement by decision number, or by a lone id, breaks the comment rule in `openspec/config.yaml`. Nothing fails a pull request that adds one. The shared `cite-lint` check of the `.github` repo does.

## What Changes

- A task `cite-lint` runs the shared `enh cite-lint` tool, built at one pinned commit of the enhancements repo, on the whole repo.
- A new workflow `cite-lint.yml` calls the reusable workflow of the `.github` repo on every pull request and on every push to `main`.
- `AGENTS.md` lists the task and the workflow.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
None. The change has no spec delta, and `.openspec.yaml` declares `skip_specs: true`.

## Impact

CI and the task file only. No code, API resource or UI page changes. The OpenSpec CLI pin stays at 1.12.0: 13 live specs fail `--strict` under 1.14.1 at the base commit. No release: the commits are `ci`, `build` and `docs`. SemVer after 1.0: PATCH.

## Enhancement

None.
