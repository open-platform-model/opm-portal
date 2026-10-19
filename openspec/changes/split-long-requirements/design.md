## Context

The validator measures the body of a requirement: the lines between `### Requirement:` and the first line that starts with `#`, trimmed, with blank lines, fenced lines and `**Key**:` lines dropped, joined by newline. Scenarios do not count. More than 500 UTF-16 code units is a WARNING. The check runs on main specs and on ADDED blocks, not on MODIFIED blocks. A MODIFIED block that stays over 500 therefore passes `validate <change> --strict` and fails `validate --all --strict` after the archive.

Baseline on `origin/main` (`fceb69b`, OpenSpec 1.14.1): 13 main specs, 85 requirements over 500, and `openspec validate --all --strict --no-interactive 2>&1 | grep -c WARNING` prints 85 (one line per long requirement). No spec has a placeholder Purpose (no "TBD"), so no Purpose text is needed. No change is open besides this one (`openspec/changes/` holds only `archive/`), so no rewrite collides with an open delta.

## Research & Decisions

### The rewrite forms

**Context**: A rewrite must make the body itself shorter. Three loopholes pass the length check and remove nothing, and are forbidden: text moved into `**Key**:` lines, under a sub-heading of level 5 or deeper, or into a fenced block.
**Explored**: The approved guide (swarm `design/T2.1.md`, sections 2 to 4) and its runs in scratch copies of `main`.
**Options considered**:
1. **a. MODIFIED, shorter body, more scenarios.** Keeps the header and every scenario name. Fits examples, lists, formats and rationale around one behavior.
2. **b. MODIFIED plus ADDED.** The parent keeps its header, its scenarios and a shorter body; split-off behaviors become ADDED requirements with new names and new scenarios. A live scenario never moves from the parent to an ADDED block.
3. **c2. REMOVED plus ADDED under different names.** The only form that may rename or move scenarios, so the ledger is mandatory. New blocks land at the end of the spec.
4. **d. RENAMED plus MODIFIED.** Rename only; the portal needs it for no requirement now.
**Decision**: Choose per requirement by what the excess is.

| The excess is | Form |
| --- | --- |
| Examples, lists, formats, rationale around one behavior | a |
| Several behaviors, and the old scenarios fit the parent that stays | b |
| Several behaviors, and the old scenarios belong to different behaviors | c2 |

**Rationale**: Almost every long portal requirement states several rules (the inventory marks most as "several behaviors"), so b and c2 are the common remedy and a is the second. The kind labels in the inventory are a heuristic: the unit reads the text. Where both b and c2 fit, prefer b: it keeps the header and every scenario in place and needs no stale-reference hunt. Use c2 when a scenario would otherwise sit under the wrong behavior. Never use a pair the guide marks unsafe (MODIFIED plus ADDED on one name, and the others of guide section 3).

### Portal rules for new requirements

**Context**: `openspec/config.yaml` rules for specs: RFC 2119 keywords; a citation `portal:DN:Rn` in the body (first sentence or trailing `Source:` line), never in the header; observable behavior; scenarios "including the denied and degraded cases (no permission, unknown kind)".
**Options considered**:
1. Add a denied or degraded scenario to every new ADDED requirement. This can invent a rule, which the rewrite forbids.
2. Add one only when the old text states that case; otherwise record the gap in the unit's report.
**Decision**: Option 2. Every new body keeps SHALL or MUST. A citation stays in the block that carries the rule it supports; a `Source:` line counts toward 500, and `**Source**:` is a forbidden loophole. A new scenario uses only words and values of the old body; a scenario that would add a case absent from the old text is a question for the human, not a rewrite.
**Rationale**: "No rule lost or changed" outranks the style rule. The old scenarios already carry the denied and degraded cases of the portal's specs; the split moves them with their WHEN and THEN unchanged.

### Batch order

**Context**: Three batches of 28, 28 and 29 requirements, in whole specs, one commit each. B4 (`portal-docs`, 2) is merged into B3 by the guide (section 8), so B3 is 29 and a little over the cap of 28.
**Decision**: B1 (`web-ui`, `change-stream`), then B2 (`read-api`, `kind-e2e-environment`, `pod-logs`), then B3 (the eight smaller specs), then the gate section.
**Rationale**: The largest specs come first while the context is fresh and the rules are the same ones. No open change touches these specs, so the order has no dependency. The gate section is last because `--strict` can pass only after the last long requirement is gone; every earlier section ends green without it (Principle VIII).

## The scenario ledger

For every rewritten requirement the unit adds a table to this file under "Ledger", one `### Batch` heading per batch:

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |

- Forms a and b: every old name stays under the same requirement, byte for byte. Form c2: every old name reappears in one of the new requirements, WHEN and THEN byte for byte. OpenSpec does not check scenarios for c2, so a scenario that is in no new requirement disappears with no error. The unit proves it by name (see "Checks per unit").
- A clause that leaves a body (an example, a list, a format, a default) gets a row in a second table, "Old clause, new place". A clause that adds or changes a condition stays in the body. A separate behavior goes to b or c2.
- A new name must not break references. The units grep the repo for the old header text, outside `openspec/changes/archive/`, before a rename, split or REMOVED. Known hits on the 249 headers today: `docs/DESIGN.md` names the two `web-ui` requirements "Muted text meets WCAG 2.2 AA contrast in both themes" and "Text inputs and selects have a boundary of at least 3:1"; a comment in `cmd/opm-portal/m1_e2e_test.go` names "Installed lists instances and packages together"; `AGENTS.md` and `internal/health/rollup.go` hold the texts of "An authorization error is a denial" and "Secrets are never read" as prose, not as references. Where a long requirement named in `docs/DESIGN.md` or a test comment is split by c2, that unit keeps the name on the requirement that holds the rule, or reports the stale reference. `docs/DESIGN.md` keeps every decision number and is not edited for a header that stays.

## Checks per unit

Each batch section runs, in the worktree and in a scratch copy of `openspec/`, never in the worktree itself:

1. **Scenario check** (guide section 7, item 3), once per rewritten requirement, with `SPEC`, `CH=split-long-requirements` and `REQ` set. It prints `same:`, `new:`, `DIFF:` or `LOST:` per scenario and exits 1 on any `DIFF` or `LOST`. A `new:` line is checked against the clause table.
2. **Loophole grep** over the delta: no `**Key**:` line (except `**Reason**:` and `**Migration**:` in REMOVED), no heading of level 5 or deeper, no fenced block in a body.
3. **The gate** (guide section 6), in one shell call, from a copy of `openspec/` that holds the uncommitted delta, with `WT=<worktree>`, `CH=split-long-requirements`, `S=<scratch folder>`:

   | Section | `N` (requirements rewritten so far) | `LAST` |
   | --- | --- | --- |
   | 1 | 28 | unset |
   | 2 | 56 | unset |
   | 3 | 85 | unset |
   | 4 | 85 | 1 |

   It validates the change with `--strict`, archives the change in the copy, runs `validate --all --strict` and compares: no new ERROR item (a set comparison against `main`), no "Archive would refuse" line, and the WARNING count falls by `N` from the `main` baseline of 85 (the gate prints before and after). With `LAST=1` it requires `openspec validate --specs --strict` to exit 0 after the archive. It needs `jq`. For the batch's specs the unit also runs `openspec validate <spec> --type spec --strict --json` and reads it: no WARNING or ERROR for a rewritten requirement.
4. **Repo checks**: `task check` (fmt, vet, lint, openspec, test, capture check) green before each commit.

The final gate passes when `openspec validate split-long-requirements --strict` exits 0, `openspec validate --specs --strict` exits 0 after the archive, and `validate --all --strict` has no warning in `main`'s specs. This repo has no other open change, so `validate --all --strict` is also expected to exit 0 there; the unit reports its exit code as observed.

## The strict gate

`task openspec:check` returns to `openspec validate --all --strict --no-interactive`. The same unit edits the four texts that say `--strict` is off: the `desc` in `Taskfile.yml`, the command-table line in `AGENTS.md`, validation gate 5 in `openspec/config.yaml`, and the comment above the step in `lint.yml`. The `lint.yml` step name returns to "OpenSpec (strict)". It is a step of the required `Lint` job, so the rename changes no required-check name. The pin stays 1.14.1 in `Taskfile.yml` and `lint.yml`, in lockstep.

## Risks

- **Silent loss.** A split can drop a clause with no tool noticing. Guards: the ledger, the clause table, the byte-equal scenario check, and the reviewer's own pass over `git show main:openspec/specs/<spec>/spec.md`.
- **Long MODIFIED.** A MODIFIED that stays over 500 passes change validation. Guard: the gate runs `validate --all --strict` after the archive in the copy.
- **Size of the diff.** 85 requirements over 13 specs, `web-ui` alone with 20. Each section is its own commit so that a reviewer can read one batch at a time.
- **Late collision.** If another change edits a spec of a later batch before this one lands, its MODIFIED block can go stale. Today `openspec/changes/` holds no other change. A unit that sees an "Archive would refuse" line reports it and does not edit the other change.

## Ledger

Empty at the proposal gate. Each implement unit adds a `### Batch B1`, `### Batch B2` or `### Batch B3` heading with its tables.
