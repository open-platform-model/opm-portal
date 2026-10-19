## Why

OpenSpec 1.14.1 raises "Requirement text is very long (>500 characters)" from INFO to WARNING. `openspec validate --strict` fails on any warning, and no config key or flag turns this one off: the limit is a constant in the validator. The warning is right on the merits. A long requirement usually holds several behaviors, and one behavior per requirement makes each one testable by its own scenarios.

The portal has 85 such requirements in 13 main specs. The 1.14.1 bump (archived as `2026-10-19-bump-openspec-1-14-1`) took `--strict` out of `task openspec:check` so that the pin could land. This change rewrites the 85 requirements so that `--strict` can pass again, with no rule lost or changed, and then puts `--strict` back.

## What Changes

Spec text is rewritten batch by batch, in whole specs. Each batch is one section of `tasks.md` and one commit. The counts are requirements over 500 characters on `origin/main` at `fceb69b`, measured with the 1.14.1 validator per spec; they match the swarm inventory (85 in 13 specs).

- **Batch B1 (28).** `web-ui` (20), `change-stream` (8).
- **Batch B2 (28).** `read-api` (16), `kind-e2e-environment` (7), `pod-logs` (5).
- **Batch B3 (29).** `read-model` (6), `read-authorization` (5), `build-and-release` (4), `local-mode` (4), `graph-model` (3), `health-evaluation` (3), `deploy-manifest` (2), and `portal-docs` (2, the small batch B4 merged in).
- **Gate.** The last section puts `--strict` back into `openspec:check` and its text: `Taskfile.yml` (command and `desc`), `AGENTS.md`, `openspec/config.yaml` (validation gate 5) and the comment in `.github/workflows/lint.yml`. It renames the `lint.yml` step back to "OpenSpec (strict)". The step stays a step of the required `Lint` job, not a check of its own.

Each rewrite is a split or a move of detail into scenarios. The form is chosen per requirement (see `design.md`). No requirement is deleted without its rules reappearing, and every old scenario keeps its name and its WHEN and THEN text.

Release class: none. Spec text, task, CI and doc files only; no Go code changes. SemVer as after 1.0: PATCH. On the 0.x line it does not release (types `docs`, `ci`).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

Thirteen existing capabilities change in wording, not in behavior: `web-ui`, `change-stream`, `read-api`, `kind-e2e-environment`, `pod-logs`, `read-model`, `read-authorization`, `build-and-release`, `local-mode`, `graph-model`, `health-evaluation`, `deploy-manifest`, `portal-docs`. The delta files are written in the implement units, one batch at a time; this change has none yet.

## Enhancement

None.

## Impact

- **Files:** `openspec/specs/*/spec.md` of the 13 specs (through the archive), `Taskfile.yml`, `AGENTS.md`, `openspec/config.yaml`, `.github/workflows/lint.yml`, `ROADMAP.md`.
- **Packages, API resources, UI pages:** none. Principle V is not touched: no new read, no change of identity.
- **Decisions:** none added or altered; `docs/DESIGN.md` does not change. Citations (`portal:Dn:Rn`) stay in requirement bodies.
- **Risks:** a split can drop or weaken a rule without any tool noticing, because OpenSpec does not compare scenario text. The scenario ledger and the byte-equal check in `design.md` guard against it. A requirement that stays over 500 inside a MODIFIED block passes `validate <change>` and fails later; the gate catches it.
