## Context

The tool lints Go and CUE files. The repo holds 207 Go files and 7 CUE files. At the base commit the tool reports 214 files checked and 0 findings with no exclude.

## Decisions

### No exclude

The repo needs none, so the task and the workflow pass none. A later exclude must carry a reason in a comment, and must be written in both places (see next decision).

### Two places for the pin, one note each

The task pins the tool commit in `Taskfile.yml`. The workflow holds its own pin as the default of `enh-ref` in the `.github` workflow, so a repo never passes it. The two commits agree today. A comment in each file names the other. The `.github` pin bump moves the CI tool; the task pin moves by hand in the same PR.

### A separate small workflow

`lint.yml` holds the required `Lint` check, which must not be skipped. A reusable-workflow call is a job of its own and cannot be a step of `Lint`. A new file `cite-lint.yml` calls it on `pull_request` and on `push` to `main`. The check is not required by a ruleset.

### Not part of `task check`

`task cite-lint` builds the tool with `go run`, which needs the Go proxy. `task check` stays offline-capable, so the new task stands apart.

### The workflow pin

The call is pinned by full SHA with the comment `.github main`, as every `.github` reference is. Dependabot ignores `open-platform-model/.github*` already. The caller job has only `name`, `permissions`, `uses` and `with` keys, with `permissions` set to `contents: read`.

## Risks

The tool pin in the task and the one in the workflow can differ after a `.github` bump. The comments name the pair.
