## 1. Batch B1: web-ui (20) and change-stream (8)

- [x] 1.1 Read the 28 long requirements of `web-ui` and `change-stream` (`openspec validate <spec> --type spec --strict --json`) and choose a form for each, by the table in `design.md`
- [x] 1.2 Write `specs/web-ui/spec.md` and `specs/change-stream/spec.md` as deltas; grep the repo for each old header before a rename, split or REMOVED
- [x] 1.3 Add the B1 ledger to `design.md`, with the clause table of every requirement that moves a clause
- [x] 1.4 Run the scenario check (checklist item 3) per rewritten requirement: no `DIFF`, no `LOST`
- [x] 1.5 Run the gate in a scratch copy with `N=28`: `GATE PASS`
- [x] 1.6 `task check` green, then commit `docs(openspec): split the long requirements of web-ui and change-stream`

## 2. Batch B2: read-api (16), kind-e2e-environment (7), pod-logs (5)

- [x] 2.1 Read the 28 long requirements of the three specs and choose a form for each
- [x] 2.2 Write `specs/read-api/spec.md`, `specs/kind-e2e-environment/spec.md` and `specs/pod-logs/spec.md` as deltas; grep for each old header first
- [x] 2.3 Add the B2 ledger and clause tables to `design.md`
- [x] 2.4 Run the scenario check per rewritten requirement: no `DIFF`, no `LOST`
- [x] 2.5 Run the gate in a scratch copy with `N=56`: `GATE PASS`
- [x] 2.6 `task check` green (unit suite and openspec:check only, DECISIONS.md 2026-10-19 15:05), then commit `docs(openspec): split the long requirements of read-api, kind-e2e-environment and pod-logs`

## 3. Batch B3: eight specs (29)

Specs: `read-model` (6), `read-authorization` (5), `build-and-release` (4), `local-mode` (4), `graph-model` (3), `health-evaluation` (3), `deploy-manifest` (2), `portal-docs` (2).

- [x] 3.1 Read the 29 long requirements of the eight specs and choose a form for each
- [x] 3.2 Write the eight delta files under `specs/<spec>/spec.md`; grep for each old header first
- [x] 3.3 Add the B3 ledger and clause tables to `design.md`
- [x] 3.4 Run the scenario check per rewritten requirement: no `DIFF`, no `LOST`
- [x] 3.5 Run the gate in a scratch copy with `N=85`: `GATE PASS`
- [x] 3.6 `task check` green (unit suite and openspec:check only, DECISIONS.md 2026-10-19 15:05), then commit `docs(openspec): split the long requirements of eight portal specs`

## 4. Strict gate back, and the final gate

- [ ] 4.1 `Taskfile.yml`: `openspec:check` runs `openspec validate --all --strict --no-interactive`; the `desc` drops "--strict is off until the long-requirement rewrite lands"
- [ ] 4.2 `AGENTS.md` (command table) and `openspec/config.yaml` (validation gate 5): `task openspec:check` runs `openspec validate --all --strict`; drop the "off until" text
- [ ] 4.3 `.github/workflows/lint.yml`: rename the step "OpenSpec" to "OpenSpec (strict)" and drop the comment that says validation runs without `--strict`; run `actionlint`
- [ ] 4.4 `ROADMAP.md`: add the change to the done table, one line, and update "Last updated"
- [ ] 4.5 Final gate (`LAST=1`, no warnings expected) in a scratch copy: `openspec validate split-long-requirements --strict` exits 0, `openspec validate --specs --strict` exits 0 after the archive, no "Archive would refuse" line, no new ERROR item; run it once more over the committed tree and read both exit codes
- [ ] 4.6 `task check` green with `--strict` on, then commit `ci(openspec): validate with --strict again`
