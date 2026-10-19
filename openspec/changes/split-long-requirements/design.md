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

### Batch B1: web-ui (20) and change-stream (8)

Forms used: c2 (REMOVED plus ADDED under new names) for 26 requirements; b (MODIFIED parent with a shorter body and every old scenario kept, plus ADDED splits) for `Applied and health are two badges, never one` and `Installed lists instances and packages together`, because those two names are cited outside `openspec/` (see "Known hits" above). No requirement used form a or d. Every new body is 500 characters or less (the largest is 500).

How the tables read:

- First table of each requirement: every old scenario, its new requirement, and whether its text changed. All 179 old scenarios of the 28 requirements kept their WHEN and THEN byte for byte (scenario check, scoped per requirement: 179 `same`, no `DIFF`, no `LOST`).
- Second table: each new or kept body, by its opening words, with the old clause it carries. Every clause of an old body is in exactly one new body; the citation (`Source:` line) is repeated in each block that carries a clause of the old body.
- Third table, where present: scenarios that are new. Each restates a clause of its own requirement, with WHEN and THEN using only words and values of the old body. A new block needs at least one scenario; where no old scenario fitted the behavior, one restatement was written. No denied or degraded case was invented (DECISIONS.md 2026-10-19 14:00); the old scenarios carry those cases and moved with their behavior.

Wording changes beyond the split (no rule changes):

| Place | Change | Why |
| --- | --- | --- |
| `A decision is cached ...` (change-stream, requirement `A revocation reaches the stream within one decision lifetime`) | "A decision SHALL be cached for at most its lifetime"; the old sentence had no keyword | An ADDED block needs SHALL or MUST; the cache bound was already a rule of the old requirement |
| `The portal accepts exactly the named topics` | lists `log:<namespace>/<pod>/<container>` itself | The rule "exactly these names" stays whole in one block; the `/previous` segment is in its own requirement |
| Bodies that start with a subject such as "The Applied card SHALL", "The Events tab SHALL" | the old text said "the card", "the tab" or "it" | A split body must name its subject; the verb and its strength are unchanged |
| Tabs: Service Endpoints fact | "the fact SHALL read" where the old bullet said "the fact reads" | The bullet sat under "The panel SHALL show"; the new block needs its own keyword |
| Header: cluster naming | "SHALL name the cluster by the kubeconfig context and, when ... or "in cluster"" | The old parenthesis moved into its own sentence |

The plain item 3 command of the guide compares by scenario name over the whole spec. In `web-ui`, `Without script`, `The F1 package`, `Platform forbidden` and `A claim of unknown standing` also name scenarios of requirements that this unit does not rewrite, so the plain command reports a false `DIFF` for them. The unit therefore ran a variant that reads the old text from the one requirement and the new text from the blocks that requirement's Migration line names (or from its MODIFIED block). Both give the same result on every other name.

Known hits stay valid: `docs/DESIGN.md` names two requirements that are not long and are not touched; the comment in `cmd/opm-portal/m1_e2e_test.go` names `Installed lists instances and packages together`, which keeps its name (form b); `docs/design/evidence/05-canvas-gap-report/` quotes `Applied and health are two badges, never one`, which keeps its name.

#### Spec `web-ui`

**Pages read only through the read API** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The UI reads through the API | Every fact on a page comes from a read API document, The UI reads through the API | no, byte for byte |
| No session | A problem document renders as a region of the page, No session | no, byte for byte |
| The import rule | The UI imports only the read API types, The import rule | no, byte for byte |
| A condition's meaning | Every fact on a page comes from a read API document, A condition's meaning | no, byte for byte |

| Old clause | New place |
| --- | --- |
| Every fact a page shows SHALL come from a read API document fetched through the read API's own... | Every fact on a page comes from a read API document |
| Among this module's packages, `internal/ui` SHALL import only `api/v1alpha1`, and a test SHALL... | The UI imports only the read API types |
| A problem document SHALL render as a region of the page: `forbidden` as locked, `not_readable_b... | A problem document renders as a region of the page |

**The Platform page is the landing page** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The F1 platform | The Catalogs tab shows each catalog's source and claimants, The F1 platform | no, byte for byte |
| Accepted and active apart | The Providers tab shows each registration's standing, Accepted and active apart | no, byte for byte |
| A count hands off | The Installed card counts by health and applied state, A count hands off | no, byte for byte |
| Instances forbidden | The Platform page says what the caller could not read, Instances forbidden | no, byte for byte |
| A refusal on the status block | The status block counts refusals and removal blocks by reason, A refusal on the status block | no, byte for byte |
| Registrations forbidden on the status block | The Platform page says what the caller could not read, Registrations forbidden on the status block | no, byte for byte |
| Registrations forbidden while Ready is not True | The Platform page says what the caller could not read, Registrations forbidden while Ready is not True | no, byte for byte |
| Removal blocked keeps acceptance and activation | A removal-blocked registration is not shown as refused, Removal blocked keeps acceptance and activation | no, byte for byte |
| An event's resource filters the feed | The Platform events merge the Platform's and its registrations' events, An event's resource filters the feed | no, byte for byte |

| Old clause | New place |
| --- | --- |
| `/` SHALL show the Platform: an identity card (name, type, and the controller version labelled... | The Platform page opens on an identity card |
| `/` SHALL show a status block with the applied state of `Ready` as a large state word in its to... | The status block shows the applied state of Ready |
| The status block SHALL list the reasons that need attention, each a count and a reason linking... | The status block counts refusals and removal blocks by reason |
| The status block SHALL show the `Ready` reason (`BuildFailed`, `GenerateFailed`, `ContractColli... | The status block names why Ready is not True |
| The status block SHALL show `ContractsFulfilled`'s reason when it is `False`, as information wi... | Unfulfilled provider contracts show as information |
| `/` SHALL show an Installed card with one column of count rows per health state and one per app... | The Installed card counts by health and applied state |
| `/` SHALL show a Providers tab with filters. A Providers row SHALL show the registration's name... | The Providers tab shows each registration's standing |
| A removal-blocked registration SHALL show as removal blocked while still accepted and active, n... | A removal-blocked registration is not shown as refused |
| `/` SHALL show a Catalogs tab with filters. A Catalogs row SHALL show the path linking to the C... | The Catalogs tab shows each catalog's source and claimants |
| `/` SHALL show the recent events of the Platform and of each readable registration, merged newe... | The Platform events merge the Platform's and its registrations' events |
| When the registrations the caller may read are not all of them, or a list behind the Installed... | The Platform page says what the caller could not read |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| The identity card | The Platform page opens on an identity card | the clause of that requirement; WHEN and THEN use only words of the old body |
| The status word | The status block shows the applied state of Ready | the clause of that requirement; WHEN and THEN use only words of the old body |
| The Ready reason | The status block names why Ready is not True | the clause of that requirement; WHEN and THEN use only words of the old body |
| Contracts unfulfilled on the status block | Unfulfilled provider contracts show as information | the clause of that requirement; WHEN and THEN use only words of the old body |

**Applied and health are two badges, never one** (form b)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The image-break sample | Applied and health are two badges, never one, The image-break sample | no, byte for byte |
| A CLI-owned instance | Applied and health are two badges, never one, A CLI-owned instance | no, byte for byte |
| An unknown state | Applied and health are two badges, never one, An unknown state | no, byte for byte |
| An informational condition | Applied and health are two badges, never one, An informational condition | no, byte for byte |
| One shape, two named axes | Applied and health are two badges, never one, One shape, two named axes | no, byte for byte |
| The details panel names both axes | Applied and health are two badges, never one, The details panel names both axes | no, byte for byte |
| One axis on an object | Applied and health are two badges, never one, One axis on an object | no, byte for byte |
| A failed apply keeps its word | Applied and health are two badges, never one, A failed apply keeps its word | no, byte for byte |
| A partial health without colour | Applied and health are two badges, never one, A partial health without colour | no, byte for byte |

| Old clause | New place |
| --- | --- |
| Every instance and package SHALL show its applied state and its health as two separate badges,... | kept in `Applied and health are two badges, never one` (MODIFIED) |
| Wherever an applied badge and a health badge are shown together, each SHALL stand where visible... | A badge stands where visible text names its axis |
| `Ready=True` SHALL be shown as "Applied" and never as healthy, and the applied badge SHALL read... | The applied badge reads the controller's state words |
| A partial health SHALL say partial and be marked visually apart from a full health by more than... | A partial or stale health is marked apart |
| The red family SHALL be used only for degraded, failed, stalled and refused states, broken edge... | The red family marks only failures |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A health stamp alone | A badge stands where visible text names its axis | the clause of that requirement; WHEN and THEN use only words of the old body |
| Ready is shown as Applied | The applied badge reads the controller's state words | the clause of that requirement; WHEN and THEN use only words of the old body |
| A health refreshed by polling | A partial or stale health is marked apart | the clause of that requirement; WHEN and THEN use only words of the old body |
| Decoration is not red | The red family marks only failures | the clause of that requirement; WHEN and THEN use only words of the old body |
| A condition is coloured by its tone | The red family marks only failures | the clause of that requirement; WHEN and THEN use only words of the old body |

**The instance page shows the record, the feed and the logs** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| podinfo's page | The instance page shows its graph, resources and conditions, podinfo's page | no, byte for byte |
| cert-manager's configuration components | The Resources tab folds configuration components and idle ReplicaSets, cert-manager's configuration components | no, byte for byte |
| Every reached object's events on arrival | The Events tab merges the events of everything the inventory reaches, Every reached object's events on arrival | no, byte for byte |
| Events of one namespace not readable | The Events tab says when events could not be read, Events of one namespace not readable | no, byte for byte |
| Logs open on arrival | The Logs tab follows one container's log, Logs open on arrival | no, byte for byte |
| A container that has not started | The Logs tab says when a container has not started or is restarting, A container that has not started | no, byte for byte |
| A crash-looping container | The Logs tab says when a container has not started or is restarting, A crash-looping container | no, byte for byte |
| Events of a caller who cannot list the Pods | The Events tab says when events could not be read, Events of a caller who cannot list the Pods | no, byte for byte |
| A long event note | The Events tab merges the events of everything the inventory reaches, A long event note | no, byte for byte |
| Events of both operator writers in-cluster | The Events tab merges the events of everything the inventory reaches, Events of both operator writers in-cluster | no, byte for byte |

| Old clause | New place |
| --- | --- |
| The instance page SHALL show: the instance's graph; its inventory objects and runtime children... | The instance page shows its graph, resources and conditions |
| The instance page SHALL show: its status history, newest first, behind "Show history"; the rend... | The instance page shows its history, contracts and events |
| The package page SHALL show the same as the instance page for a ModulePackage (its graph, Resou... | The package page shows the same for a ModulePackage |
| Configuration components SHALL be folded in the Resources tab into the same groups the graph fo... | The Resources tab folds configuration components and idle ReplicaSets |
| The Events tab SHALL open on the events of the instance and of every object and runtime child i... | The Events tab merges the events of everything the inventory reaches |
| When some reached objects' events could not be read, the Events tab SHALL say that they are lef... | The Events tab says when events could not be read |
| The feed SHALL be labelled as recent activity that expires after about an hour, and the page SH... | The feed is labelled as recent activity |
| The Logs tab SHALL offer each container of each Pod an inventory reaches through one Pod and co... | The Logs tab follows one container's log |
| When the picked container waits with a reason under which no instance of it has started (or, wh... | The Logs tab says when a container has not started or is restarting |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| History, contracts and events | The instance page shows its history, contracts and events | the clause of that requirement; WHEN and THEN use only words of the old body |
| The package page record | The package page shows the same for a ModulePackage | the clause of that requirement; WHEN and THEN use only words of the old body |
| The feed note | The feed is labelled as recent activity | the clause of that requirement; WHEN and THEN use only words of the old body |

**Graphs are server-rendered, accessible SVG** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Keyboard focus | Graph nodes are named and reachable with the keyboard, Keyboard focus | no, byte for byte |
| Two axes on a node | A node's fill, outline and mark carry its state, Two axes on a node | no, byte for byte |
| A ReplicaSet's status line | A graph node shows three lines, A ReplicaSet's status line | no, byte for byte |
| Elbow edges | Each graph renders server-side as SVG, Elbow edges | no, byte for byte |
| A graph at phone width | A graph at phone width does not scroll the page, A graph at phone width | no, byte for byte |

| Old clause | New place |
| --- | --- |
| Each graph SHALL render server-side as SVG from the read API's laid-out graph document, with no... | Each graph renders server-side as SVG |
| Nodes SHALL carry CSS classes for kind, health and access, an accessible name, and a title with... | Graph nodes are named and reachable with the keyboard |
| A node SHALL show three lines: its kind, its name, and a status line (its health reason or stat... | A graph node shows three lines |
| A node's fill and outline SHALL carry its health, except that a TransformerRegistration node's... | A node's fill, outline and mark carry its state |
| Activating a node SHALL show its detail panel and mark the node as selected; activating a group... | A graph can be selected, fitted, panned and zoomed |
| At a 360 px wide viewport the page SHALL NOT scroll sideways; the graph's frame SHALL take the... | A graph at phone width does not scroll the page |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| Selecting and zooming a graph | A graph can be selected, fitted, panned and zoomed | the clause of that requirement; WHEN and THEN use only words of the old body |

**Pages update live over one stream per tab** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Navigating keeps the stream | A tab holds one stream across navigation, Navigating keeps the stream | no, byte for byte |
| An image break on the open page | A change on a followed topic re-renders the open page, An image break on the open page | no, byte for byte |

| Old clause | New place |
| --- | --- |
| A tab SHALL hold one `EventSource` on the read API's stream across navigation, and each page SH... | A tab holds one stream across navigation |
| A change on a followed topic SHALL re-render every region of the open page tab that shows it, a... | A change on a followed topic re-renders the open page |

**Pages carry a strict policy and their own assets** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The page policy | UI pages carry a strict Content-Security-Policy, The page policy | no, byte for byte |
| A tampered asset | Assets are served from the binary at pinned versions, A tampered asset | no, byte for byte |
| Hostile cluster text | Untrusted text renders only as text, Hostile cluster text | no, byte for byte |
| What the browser stores | Browser storage holds only the theme and the filter queries, What the browser stores | no, byte for byte |

| Old clause | New place |
| --- | --- |
| UI pages SHALL be served with `Content-Security-Policy: default-src 'none'; script-src 'self';... | UI pages carry a strict Content-Security-Policy |
| Script SHALL set positions and sizes only through CSSOM properties or SVG attributes, never a `... | Script sets positions and sizes only through CSSOM or SVG attributes |
| Untrusted text (messages, notes, labels, log lines) SHALL be rendered only through `html/templa... | Untrusted text renders only as text |
| Scripts, styles and fonts SHALL be served from the binary, vendored at pinned versions whose SH... | Assets are served from the binary at pinned versions |
| Browser storage SHALL hold only the theme choice and each list view's filter query, every acces... | Browser storage holds only the theme and the filter queries |
| Pages SHALL follow the theme chosen in the browser, or the OS light or dark preference under Sy... | Pages follow the chosen theme and hold their width at 360 px |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| No style attribute from script | Script sets positions and sizes only through CSSOM or SVG attributes | the clause of that requirement; WHEN and THEN use only words of the old body |
| Theme and phone width | Pages follow the chosen theme and hold their width at 360 px | the clause of that requirement; WHEN and THEN use only words of the old body |

**In-cluster pages show no operator text** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The broken podinfo page in-cluster | In-cluster pages omit the operator's messages, The broken podinfo page in-cluster | no, byte for byte |
| A Pod's events in-cluster | In-cluster pages keep reasons, standings and other writers' notes, A Pod's events in-cluster | no, byte for byte |
| Attempt dots and the Provider tab in-cluster | In-cluster pages omit the operator's messages, Attempt dots and the Provider tab in-cluster | no, byte for byte |

| Old clause | New place |
| --- | --- |
| Pages served over an `in-cluster` read API SHALL show no condition message (the conditions of a... | In-cluster pages omit the operator's messages |
| In-cluster pages SHALL still show every reason, state, history outcome, provider standing and t... | In-cluster pages keep reasons, standings and other writers' notes |

**An expired session stops the page's stream** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The session expires while a page is open | An expired session closes the stream and marks the live mark, The session expires while a page is open | no, byte for byte |
| Reload after the session expired | An expired session shows a banner with the time of the data, Reload after the session expired | no, byte for byte |

| Old clause | New place |
| --- | --- |
| When the stream ends with an `expired` event, the page SHALL close its `EventSource`, so it doe... | An expired session closes the stream and marks the live mark |
| When the stream ends with an `expired` event, the page SHALL show a visible banner in the stick... | An expired session shows a banner with the time of the data |

**The header names the cluster, the reader and the version** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The F1 header | Every page carries a sticky header, The F1 header | no, byte for byte |
| A cluster entry that differs from the context | The header names the cluster and its entry name, A cluster entry that differs from the context | no, byte for byte |
| A cluster entry that equals the context | The header names the cluster and its entry name, A cluster entry that equals the context | no, byte for byte |
| The longest entry name at 360 px | The header names the cluster and its entry name, The longest entry name at 360 px | no, byte for byte |
| Version not read | A version the document does not carry reads unknown, Version not read | no, byte for byte |
| The cluster document fails | The header survives a failed Cluster document, The cluster document fails | no, byte for byte |
| In a Pod | The header names the cluster and its entry name, In a Pod | no, byte for byte |

| Old clause | New place |
| --- | --- |
| Every page SHALL carry a sticky header with the main navigation (Platform, Installed), the clus... | Every page carries a sticky header |
| The header SHALL name the cluster by the kubeconfig context and, when the `Cluster` document ca... | The header names the cluster and its entry name |
| A version the `Cluster` document does not carry SHALL read "unknown". | A version the document does not carry reads unknown |
| When the `Cluster` document cannot be fetched, the header SHALL still render its navigation, li... | The header survives a failed Cluster document |
| The header SHALL slim after the page scrolls 48 px and SHALL not animate under `prefers-reduced... | The header slims on scroll |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| The header slims | The header slims on scroll | the clause of that requirement; WHEN and THEN use only words of the old body |

**Installed lists instances and packages together** (form b)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The F1 Installed list | Installed lists instances and packages together, The F1 Installed list | no, byte for byte |
| Packages forbidden | Installed lists instances and packages together, Packages forbidden | no, byte for byte |
| An old link | Installed lists instances and packages together, An old link | no, byte for byte |
| The provider tooltip by keyboard | Installed lists instances and packages together, The provider tooltip by keyboard | no, byte for byte |
| The provider tooltip by tap and Escape | Installed lists instances and packages together, The provider tooltip by tap and Escape | no, byte for byte |
| A package with no recorded revision | Installed lists instances and packages together, A package with no recorded revision | no, byte for byte |
| A whole row opens the item | Installed lists instances and packages together, A whole row opens the item | no, byte for byte |

| Old clause | New place |
| --- | --- |
| `/installed` SHALL list the ModuleInstances and ModulePackages the caller may read in one table... | kept in `Installed lists instances and packages together` (MODIFIED) |
| The Installed table SHALL show a provider badge reading "Provider" when the item holds a Transf... | An Installed row shows a provider badge with a tooltip |
| The Installed table SHALL show the module path with its version on a line below, or the package... | An Installed row shows the module or the package source |
| When filters match nothing, the Installed list SHALL say so and offer a button that clears them... | An Installed list with no match offers to clear the filters |
| A kind the caller may not list SHALL render locked in the Installed list for that kind only, wi... | A kind the caller may not list renders locked |
| `/instances` and `/packages` SHALL redirect with `308` to `/installed` with `kind=instance` or... | The old list addresses redirect to Installed |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| The provider badge | An Installed row shows a provider badge with a tooltip | the clause of that requirement; WHEN and THEN use only words of the old body |
| A module's path and version | An Installed row shows the module or the package source | the clause of that requirement; WHEN and THEN use only words of the old body |
| Filters match nothing | An Installed list with no match offers to clear the filters | the clause of that requirement; WHEN and THEN use only words of the old body |
| A kind forbidden | A kind the caller may not list renders locked | the clause of that requirement; WHEN and THEN use only words of the old body |
| An old packages link | The old list addresses redirect to Installed | the clause of that requirement; WHEN and THEN use only words of the old body |

**List filters live in the URL and are remembered per browser** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A link wins over the remembered filters | A filter in the URL wins over the remembered filters, A link wins over the remembered filters | no, byte for byte |
| Remembered filters restored | A remembered view opens with its stored filters, Remembered filters restored | no, byte for byte |
| Uses leaves packages out | The uses filter matches render contracts and leaves packages out, Uses leaves packages out | no, byte for byte |
| Without script | List filters read the URL query, Without script | no, byte for byte |
| An unknown filter value | An unknown filter value is ignored with a note, An unknown filter value | no, byte for byte |
| A stale stored query | A stored query is checked and only the shown filters are stored, A stale stored query | no, byte for byte |
| Another context | Stored filters are kept per context, Another context | no, byte for byte |
| A tab is not a filter | Only the list views remember their filters, A tab is not a filter | no, byte for byte |

| Old clause | New place |
| --- | --- |
| The Installed list, the Platform page's Providers and Catalogs tabs and its events, and the Eve... | List filters read the URL query |
| Installed SHALL offer search, kind, provider, uses, namespace, health, applied, owner and modul... | Installed offers its filters as removable chips |
| An enumerated filter value the view does not know SHALL be ignored with a note naming it, and n... | An unknown filter value is ignored with a note |
| Only the list views SHALL remember their filters: Installed and the Providers and Catalogs tabs... | Only the list views remember their filters |
| When the URL carries any of a remembered view's filter parameters, the view SHALL show exactly... | A filter in the URL wins over the remembered filters |
| When the URL carries none of a remembered view's filter parameters and the browser holds valid... | A remembered view opens with its stored filters |
| A stored query SHALL be checked against the view's parameters and values, and what does not fit... | A stored query is checked and only the shown filters are stored |
| Stored filters SHALL be kept per kubeconfig context (or the in-cluster source), so filters stor... | Stored filters are kept per context |
| The `uses` filter SHALL match an instance's render contracts, SHALL leave packages out and SHAL... | The uses filter matches render contracts and leaves packages out |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| Active filters show as chips | Installed offers its filters as removable chips | the clause of that requirement; WHEN and THEN use only words of the old body |

**Instance and package pages summarize Applied, Health and Provider standing** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The image-break podinfo | The Health card counts objects by health reason, The image-break podinfo | no, byte for byte |
| A failed attempt in the history | The Applied card draws one dot per history entry, A failed attempt in the history | no, byte for byte |
| The F1 package | The identity card names the kind, the name and the module or source, The F1 package | no, byte for byte |
| Rolling out | The Applied card shows the applied state as its word, Rolling out | no, byte for byte |
| Nothing applied yet | The Health card marks a partial or stale health, Nothing applied yet | no, byte for byte |
| A refused provider in-cluster | The Provider card summary is not a verdict the portal composed, A refused provider in-cluster | no, byte for byte |
| A pending claim with no controller text | The Provider card has a fixed sentence when the controller said nothing, A pending claim with no controller text | no, byte for byte |
| A claim of unknown standing | The Provider card reads Unknown for a standing it does not know, A claim of unknown standing | no, byte for byte |
| A blocked removal | The Provider card has a fixed sentence when the controller said nothing, A blocked removal | no, byte for byte |
| Platform forbidden on the card | The Provider card summary is not a verdict the portal composed, Platform forbidden on the card | no, byte for byte |

| Old clause | New place |
| --- | --- |
| The page of an instance or package SHALL show an identity card. The card SHALL carry the kind,... | The identity card names the kind, the name and the module or source |
| The identity card SHALL also carry the namespace, the owner and the applier ServiceAccount, eac... | The identity card carries the namespace, owner and applier |
| Below 520 px wide, the identity card's facts SHALL take one column. | The identity card takes one column on a narrow screen |
| The page SHALL show an Applied card and a Health card, and a Provider card when the item holds... | The page shows an Applied card, a Health card and a Provider card |
| Each card SHALL be a state block: an eyebrow naming the axis and its source, the recorded time... | Each card is a state block |
| The Applied card SHALL show the applied state as its word. It SHALL show "Reconciling" instead... | The Applied card shows the applied state as its word |
| The Applied card SHALL compose its summary only from recorded facts: the inventory count, the f... | The Applied card composes its summary from recorded facts |
| The Applied card SHALL show the reason's meaning and, in local mode, the controller's message b... | The Applied card shows the reason's meaning and message |
| The Applied card SHALL show a `Reconciling` mark with its reason and since time while that cond... | The Applied card marks a Reconciling condition |
| The Applied card SHALL count the Warning events about the item itself by reason, labelled as th... | The Applied card counts Warning events by reason |
| The Applied card SHALL draw one dot per `status.history` entry, oldest first, marked and colour... | The Applied card draws one dot per history entry |
| The Applied card SHALL show the number of attempts and a Show history control. The opened histo... | The Applied card offers the history |
| The Health card SHALL show the health state as its word, and SHALL say how many resources are u... | The Health card shows the health state as its word |
| The Health card SHALL show its partial mark, and when it is not live, the time it was evaluated... | The Health card marks a partial or stale health |
| The Health card SHALL count objects and runtime children by health reason, each linking to the... | The Health card counts objects by health reason |
| The Provider card SHALL show, per held registration, Active, Accepted and not active, Refused,... | The Provider card shows each held registration's standing |
| The Provider card's summary SHALL be the contracts an active registration provides, or else the... | The Provider card summary is not a verdict the portal composed |
| When the controller wrote no message and the reason has no known meaning, the Provider card's s... | The Provider card has a fixed sentence when the controller said nothing |
| A standing the page does not know SHALL read Unknown on the Provider card. | The Provider card reads Unknown for a standing it does not know |
| A non-success reason on the Provider card SHALL link to the Provider tab. | A non-success provider reason links to the Provider tab |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| Namespace, owner and applier | The identity card carries the namespace, owner and applier | the clause of that requirement; WHEN and THEN use only words of the old body |
| Identity card below 520 px | The identity card takes one column on a narrow screen | the clause of that requirement; WHEN and THEN use only words of the old body |
| The three cards | The page shows an Applied card, a Health card and a Provider card | the clause of that requirement; WHEN and THEN use only words of the old body |
| The state blocks | Each card is a state block | the clause of that requirement; WHEN and THEN use only words of the old body |
| An Applied summary from recorded facts | The Applied card composes its summary from recorded facts | the clause of that requirement; WHEN and THEN use only words of the old body |
| Meaning and message under the summary | The Applied card shows the reason's meaning and message | the clause of that requirement; WHEN and THEN use only words of the old body |
| A Reconciling mark | The Applied card marks a Reconciling condition | the clause of that requirement; WHEN and THEN use only words of the old body |
| Warning events on the Applied card | The Applied card counts Warning events by reason | the clause of that requirement; WHEN and THEN use only words of the old body |
| The opened history | The Applied card offers the history | the clause of that requirement; WHEN and THEN use only words of the old body |
| The Health summary | The Health card shows the health state as its word | the clause of that requirement; WHEN and THEN use only words of the old body |
| Standing as recorded | The Provider card shows each held registration's standing | the clause of that requirement; WHEN and THEN use only words of the old body |
| A reason links to the Provider tab | A non-success provider reason links to the Provider tab | the clause of that requirement; WHEN and THEN use only words of the old body |

**Instance and package pages are organised in tabs** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| From Resources to the graph | Resources rows are marked and link to the graph, From Resources to the graph | no, byte for byte |
| Events filtered by reason | Events filter by type and reason, Events filtered by reason | no, byte for byte |
| A Secret in YAML | YAML opens on the selected object and shows no Secret, A Secret in YAML | no, byte for byte |
| The panel at rest | The details panel rests on the node the graph rests on, The panel at rest | no, byte for byte |
| Clear selection returns to the resting node | Clear selection returns the details panel to the resting node, Clear selection returns to the resting node | no, byte for byte |
| Tab order with a provider | Instance and package pages offer their tabs in order, Tab order with a provider | no, byte for byte |
| The package's source row | A package's Resources start with its source, The package's source row | no, byte for byte |
| Object facts in the panel | The details panel shows a node's kind, state and object facts, Object facts in the panel | no, byte for byte |
| An image reference with userinfo | An image reference is shown without userinfo, An image reference with userinfo | no, byte for byte |
| A password that holds a slash | An image reference is shown without userinfo, A password that holds a slash | no, byte for byte |
| A digest reference | An image reference is shown without userinfo, A digest reference | no, byte for byte |
| A digest reference with userinfo | An image reference is shown without userinfo, A digest reference with userinfo | no, byte for byte |
| A sha384 digest reference | An image reference is shown without userinfo, A sha384 digest reference | no, byte for byte |
| A node the caller cannot read | Panel facts are a fixed list for nodes the caller may read, A node the caller cannot read | no, byte for byte |
| Owner tabs at 360 px | No tab makes the page scroll sideways at 360 px, Owner tabs at 360 px | no, byte for byte |
| Object facts not readable | Panel facts are a fixed list for nodes the caller may read, Object facts not readable | no, byte for byte |
| YAML on arrival | YAML opens on the selected object and shows no Secret, YAML on arrival | no, byte for byte |
| YAML follows a graph click | YAML opens on the selected object and shows no Secret, YAML follows a graph click | no, byte for byte |
| Only objects with events are offered | Events filter by resource, Only objects with events are offered | no, byte for byte |
| A resource link filters the feed | Events are a table with the regarded resource, A resource link filters the feed | no, byte for byte |
| A Service's ready endpoints | A Service's Endpoints fact reads the recorded ready count, A Service's ready endpoints | no, byte for byte |
| A Service with no ready endpoint | A Service's Endpoints fact reads the recorded ready count, A Service with no ready endpoint | no, byte for byte |
| A Service whose endpoints the caller may not list | A Service's Endpoints fact says when endpoints could not be read, A Service whose endpoints the caller may not list | no, byte for byte |
| Endpoints the portal could not read | A Service's Endpoints fact says when endpoints could not be read, Endpoints the portal could not read | no, byte for byte |
| A Service with no recorded slice | A Service with no recorded slice has no Endpoints fact, A Service with no recorded slice | no, byte for byte |
| Endpoints at 360 px | A Service's Endpoints fact reads the recorded ready count, Endpoints at 360 px | no, byte for byte |
| Nothing but a number | A Service's Endpoints fact reads the recorded ready count, Nothing but a number | no, byte for byte |

| Old clause | New place |
| --- | --- |
| Below the cards the page SHALL offer the tabs Graph, Provider when the item holds a registratio... | Instance and package pages offer their tabs in order |
| The details panel SHALL show only on Graph and Resources, beside the tab's region where the pag... | The details panel rests on the node the graph rests on |
| Clear selection SHALL return the details panel to the node the graph rests on. The panel SHALL... | Clear selection returns the details panel to the resting node |
| The panel SHALL show the node's kind, name and state badges, its health message in a box toned... | The details panel shows a node's kind, state and object facts |
| For a Service the panel SHALL also show an Endpoints fact. When its graph node carries `endpoin... | A Service's Endpoints fact reads the recorded ready count |
| When the Service's graph node carries `endpointsUnread` the Endpoints fact SHALL read "Could no... | A Service's Endpoints fact says when endpoints could not be read |
| When the Service's graph node carries neither `endpointsReady` nor `endpointsUnread`, because t... | A Service with no recorded slice has no Endpoints fact |
| For a component, the panel SHALL say that a component is not a Kubernetes object and has no eve... | The details panel of a component says it is not a Kubernetes object |
| For the item's own node, the panel SHALL show its identity facts and an Events link, and no lin... | The details panel of the item's own node shows identity facts |
| The facts SHALL be a fixed list and SHALL appear only for a node the caller may read: a node th... | Panel facts are a fixed list for nodes the caller may read |
| An image reference SHALL be shown without any userinfo before its host. | An image reference is shown without userinfo |
| The panel SHALL keep its Open, Expand, YAML and Events links, and SHALL add a Logs link for a P... | The details panel keeps its links |
| Resources SHALL be a table with one row per inventory object and runtime child: kind, name, com... | Resources is a table of inventory objects and runtime children |
| A Degraded or Missing row SHALL be marked apart, and the row of the `node` or `focus` object SH... | Resources rows are marked and link to the graph |
| Configuration components SHALL fold as in the graph. The Resources table SHALL be filtered by a... | Resources folds configuration and filters by reason |
| On a package page the first row of the Resources table SHALL be the package's source, with orig... | A package's Resources start with its source |
| At 360 px wide, no tab SHALL make the page scroll sideways, and the Resources and Events tables... | No tab makes the page scroll sideways at 360 px |
| Events SHALL filter by resource (grouped by kind, with "All resources" and the item itself as c... | Events filter by resource |
| Events SHALL filter by type and by reason, each choice with its count. The reason SHALL be show... | Events filter by type and reason |
| "Showing N of M" SHALL sit with the filters. The events SHALL be a table, and each line SHALL s... | Events are a table with the regarded resource |
| Logs SHALL pick a Pod and container and follow its log as the logs panel does today. | The Logs tab picks a Pod and container |
| YAML SHALL pick an object grouped by kind, open on the selected (`node`) object, else the focus... | YAML opens on the selected object and shows no Secret |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A component's panel | The details panel of a component says it is not a Kubernetes object | the clause of that requirement; WHEN and THEN use only words of the old body |
| The item's own node | The details panel of the item's own node shows identity facts | the clause of that requirement; WHEN and THEN use only words of the old body |
| The panel's links | The details panel keeps its links | the clause of that requirement; WHEN and THEN use only words of the old body |
| The Resources columns | Resources is a table of inventory objects and runtime children | the clause of that requirement; WHEN and THEN use only words of the old body |
| A reason filter | Resources folds configuration and filters by reason | the clause of that requirement; WHEN and THEN use only words of the old body |
| Logs pick a container | The Logs tab picks a Pod and container | the clause of that requirement; WHEN and THEN use only words of the old body |

**Graphs can be explored without leaving the page** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| cert-manager's RBAC group | A group node expands in place, cert-manager's RBAC group | no, byte for byte |
| Keyboard card | A graph shows a card beside a node, Keyboard card | no, byte for byte |
| A click does not dim | Activating a graph node selects it and dims nothing, A click does not dim | no, byte for byte |
| Hand-off spotlight | A focus parameter spotlights a node and its chain, Hand-off spotlight | no, byte for byte |
| Escape steps back | Escape steps back one thing per press, Escape steps back | no, byte for byte |
| Full screen survives a group | A graph opens in full screen, Full screen survives a group | no, byte for byte |
| Wheel zoom at the pointer | A graph zooms, fits and pans, Wheel zoom at the pointer | no, byte for byte |

| Old clause | New place |
| --- | --- |
| A graph SHALL show a card beside a node on pointer hover and on keyboard focus (kind, namespace... | A graph shows a card beside a node |
| Activating a node SHALL select it through a `node` parameter, which shows its panel and dims no... | Activating a graph node selects it and dims nothing |
| A `focus` parameter SHALL spotlight its node: select it, scroll it into the centre of the view... | A focus parameter spotlights a node and its chain |
| A "Clear selection" control SHALL remove the selection and the spotlight. | Clear selection removes the selection and the spotlight |
| The graph SHALL open in full screen and back, refitting to the space it has. Full screen SHALL... | A graph opens in full screen |
| The graph SHALL zoom with a zoom slider from 30 % to 200 %, Fit (marked pressed while the view... | A graph zooms, fits and pans |
| A group node SHALL expand in place through the graph's `expand` parameter and fit the view to t... | A group node expands in place |
| Opening or closing a group, and the group controls, SHALL keep the page's scroll position. | Opening or closing a group keeps the scroll position |
| Escape SHALL step back one thing per press: an open card, a selection, an open group, full scre... | Escape steps back one thing per press |
| Wheel zoom SHALL not animate, and motion SHALL stop under the reduced-motion preference. | Wheel zoom does not animate and motion stops under reduced motion |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| Clear selection on a spotlight | Clear selection removes the selection and the spotlight | the clause of that requirement; WHEN and THEN use only words of the old body |
| A group keeps the scroll position | Opening or closing a group keeps the scroll position | the clause of that requirement; WHEN and THEN use only words of the old body |
| Reduced motion | Wheel zoom does not animate and motion stops under reduced motion | the clause of that requirement; WHEN and THEN use only words of the old body |

**Providers are shown whatever their kind** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The F1 provider | A holder of a registration shows a Provider pill, card and tab, The F1 provider | no, byte for byte |
| A package holding a refused claim | The provider standing is the registration's own, A package holding a refused claim | no, byte for byte |
| Registrations forbidden | The provider standing is the registration's own, Registrations forbidden | no, byte for byte |

| Old clause | New place |
| --- | --- |
| An instance or a package whose inventory holds a TransformerRegistration SHALL show a Provider... | A holder of a registration shows a Provider pill, card and tab |
| The standing shown SHALL be the registration's own accepted, active, verdict and reason, never... | The provider standing is the registration's own |
| When the registration's `spec.providerRef` and the holder disagree, both SHALL be shown. | A disagreeing providerRef is shown beside the holder |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| providerRef and holder disagree | A disagreeing providerRef is shown beside the holder | the clause of that requirement; WHEN and THEN use only words of the old body |

**The Provider tab shows the claim and who uses what it provides** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Who uses the backup trait | Each contract lists its first three users, Who uses the backup trait | no, byte for byte |
| Platform forbidden | The Provider tab renders locked what the Platform document holds, Platform forbidden | no, byte for byte |
| Users not fully readable | The Contracts fact counts what is provided and in use, Users not fully readable | no, byte for byte |
| A contract nobody uses | A contract nobody uses says so, A contract nobody uses | no, byte for byte |
| A claim that provides nothing yet | A claim not yet active says that nothing is provided yet, A claim that provides nothing yet | no, byte for byte |
| A blocked removal still provides | A claim whose removal is blocked says so, A blocked removal still provides | no, byte for byte |
| A claim of unknown standing | A claim of unknown standing says it is not known, A claim of unknown standing | no, byte for byte |
| The Provider tab at 360 px | Provider tab rows are cards below 760 px, The Provider tab at 360 px | no, byte for byte |
| The standing said once | The Provider tab shows each registration's standing once, The standing said once | no, byte for byte |

| Old clause | New place |
| --- | --- |
| The Provider tab SHALL open with a sentence, per kind, saying the item is also a provider that... | The Provider tab opens with a sentence about the provider |
| Per held registration, the Provider tab SHALL show the registration's name and its standing onc... | The Provider tab shows each registration's standing once |
| Per held registration, the tab SHALL show a grid of facts: the registration, its claimed catalo... | The Provider tab shows a grid of facts |
| The grid SHALL also show how many contracts the registration provides and how many are in use (... | The Contracts fact counts what is provided and in use |
| Per held registration, the tab SHALL show its "Registration conditions" (type, a status badge t... | The Provider tab shows the registration's conditions |
| Per held registration, the tab SHALL show a table of what it provides, where each contract is a... | The Provider tab tabulates what the registration provides |
| The Contracts fact SHALL have a text for every standing. A claim that is accepted but not activ... | A claim not yet active says that nothing is provided yet |
| A claim whose removal is blocked SHALL give its counts in the Contracts fact and say the remova... | A claim whose removal is blocked says so |
| A claim of unknown standing SHALL say in the Contracts fact that it is not known. | A claim of unknown standing says it is not known |
| Below 760 px wide, each row of the Provider tab's table of what the registration provides SHALL... | Provider tab rows are cards below 760 px |
| For each contract the Provider tab's table of what the registration provides SHALL list the fir... | Each contract lists its first three users |
| When no readable instance uses the contract and the list is complete, the tab SHALL say no inst... | A contract nobody uses says so |
| When the caller could not list instances everywhere, the list of users SHALL say it may be inco... | An incomplete list of users says it may be incomplete |
| The registration's conditions, catalog and registry state come from the Platform document. When... | The Provider tab renders locked what the Platform document holds |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| The Provider tab's opening sentence | The Provider tab opens with a sentence about the provider | the clause of that requirement; WHEN and THEN use only words of the old body |
| The grid of facts | The Provider tab shows a grid of facts | the clause of that requirement; WHEN and THEN use only words of the old body |
| Registration conditions | The Provider tab shows the registration's conditions | the clause of that requirement; WHEN and THEN use only words of the old body |
| What it provides | The Provider tab tabulates what the registration provides | the clause of that requirement; WHEN and THEN use only words of the old body |
| A list that may be incomplete | An incomplete list of users says it may be incomplete | the clause of that requirement; WHEN and THEN use only words of the old body |

**The Catalog page shows one catalog as the cluster records it** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The contributed backup catalog | The Catalog page shows any catalog the Platform knows, The contributed backup catalog | no, byte for byte |
| A claimed catalog that does not resolve | The Catalog page shows whether the catalog is resolved, A claimed catalog that does not resolve | no, byte for byte |
| An unknown catalog | An unnamed catalog path renders not found, An unknown catalog | no, byte for byte |
| Platform forbidden | The Catalog page renders locked when the Platform is unreadable, Platform forbidden | no, byte for byte |
| A refusal reason opens the events | Each refusal reason links to the filtered Events tab, A refusal reason opens the events | no, byte for byte |
| Events filtered by type | The Catalog page tabulates the events of the Platform and its claimants, Events filtered by type | no, byte for byte |
| A Type choice keeps the reason | The Catalog page tabulates the events of the Platform and its claimants, A Type choice keeps the reason | no, byte for byte |
| Claimants forbidden | The Catalog page lists the registrations that claim it, Claimants forbidden | no, byte for byte |

| Old clause | New place |
| --- | --- |
| `/catalog?path=<catalog>` SHALL show any catalog the Platform subscribes to, holds in its resol... | The Catalog page shows any catalog the Platform knows |
| The Catalog page SHALL show the catalog's origin (subscribed, from a provider, or claimed only)... | The Catalog page shows the catalog's origin and source |
| The Catalog page SHALL show the catalog's enablement, reading "no: not in the registry" for a c... | The Catalog page shows the catalog's enablement |
| The Catalog page SHALL show whether the catalog is resolved, as a state block with the state wo... | The Catalog page shows whether the catalog is resolved |
| For an unresolved catalog the state block SHALL show the time since it was refused: the earlies... | An unresolved catalog shows the time since it was refused |
| For an unresolved catalog the state block SHALL show each reason (the Platform's `Ready` reason... | Each refusal reason links to the filtered Events tab |
| The Catalog page SHALL list the registrations that claim the catalog with their holders and sta... | The Catalog page lists the registrations that claim it |
| The Catalog page SHALL show the recent events of the Platform and its claimants as a table, fil... | The Catalog page tabulates the events of the Platform and its claimants |
| The Catalog page SHALL show no definition, description, documentation link or transformer, and... | The Catalog page says what is not recorded |
| When the caller may not read the Platform, the Catalog page SHALL render locked with status `40... | The Catalog page renders locked when the Platform is unreadable |
| A path that a readable Platform and the readable registrations do not name SHALL render not fou... | An unnamed catalog path renders not found |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| Origin and source | The Catalog page shows the catalog's origin and source | the clause of that requirement; WHEN and THEN use only words of the old body |
| Enablement of a catalog not in the registry | The Catalog page shows the catalog's enablement | the clause of that requirement; WHEN and THEN use only words of the old body |
| Refused since | An unresolved catalog shows the time since it was refused | the clause of that requirement; WHEN and THEN use only words of the old body |
| Not recorded | The Catalog page says what is not recorded | the clause of that requirement; WHEN and THEN use only words of the old body |

**Pages use the words a platform team uses** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The owner column | The controller version and owner are worded as controller, The owner column | no, byte for byte |
| The providers line | The Installed card counts providers, The providers line | no, byte for byte |

| Old clause | New place |
| --- | --- |
| Page text SHALL name kinds in the words a platform team uses: "Providers" for TransformerRegist... | Page text names kinds in a platform team's words |
| The read API's `operatorVersion` SHALL be labelled "Controller", and its owner value `operator`... | The controller version and owner are worded as controller |
| The Installed card SHALL count the items that hold a registration as providers. | The Installed card counts providers |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| The platform team's words | Page text names kinds in a platform team's words | the clause of that requirement; WHEN and THEN use only words of the old body |

**Locked things render locked on every page** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A forbidden kind inside an instance | Anything the API reports as unreadable renders locked, A forbidden kind inside an instance | no, byte for byte |
| A forbidden list in Installed | A forbidden instance or package list is a locked group, A forbidden list in Installed | no, byte for byte |

| Old clause | New place |
| --- | --- |
| Every object, row, node, list, card or section the read API reports as forbidden, not readable... | Anything the API reports as unreadable renders locked |
| A forbidden instance or package list SHALL render as a locked group of the Installed list with... | A forbidden instance or package list is a locked group |
| A Secret in an inventory SHALL show that Secret data is never read, with no YAML action. | A Secret in an inventory is never read |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A Secret in an inventory | A Secret in an inventory is never read | the clause of that requirement; WHEN and THEN use only words of the old body |

#### Spec `change-stream`

**One stream per tab carries many topics** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Adding a topic to an open stream | A stream multiplexes the topics attached to it, Adding a topic to an open stream | no, byte for byte |
| Removing a topic from an open stream | A stream multiplexes the topics attached to it, Removing a topic from an open stream | no, byte for byte |
| Another session cannot change a stream | Only the opening session and identity can change a stream's topics, Another session cannot change a stream | no, byte for byte |
| Another identity in the same session cannot change a stream | Only the opening session and identity can change a stream's topics, Another identity in the same session cannot change a stream | no, byte for byte |
| Asking again for a denied topic counts it once | A stream carries at most a configured number of topics, Asking again for a denied topic counts it once | no, byte for byte |
| A detached topic's unsent closing is dropped | A stream carries at most a configured number of topics, A detached topic's unsent closing is dropped | no, byte for byte |

| Old clause | New place |
| --- | --- |
| A stream SHALL carry every topic attached to it, multiplexed, with each message naming its topi... | A stream multiplexes the topics attached to it |
| The first message on a stream SHALL name the stream's identifier, which only the session that o... | Only the opening session and identity can change a stream's topics |
| A stream SHALL carry at most a configured number of topics; a topic whose closed message has no... | A stream carries at most a configured number of topics |

**Every subscription and every delivery is authorized for the subscriber** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A subscriber receives only what it may read | Any other topic delivers only what the subscriber may read, A subscriber receives only what it may read | no, byte for byte |
| A list topic sends no review per item | A list topic carries the items within its grant with no review per item, A list topic sends no review per item | no, byte for byte |
| A decision that expires while a message is built is asked again before the write | A topic is authorized before it attaches and again before delivery, A decision that expires while a message is built is asked again before the write | no, byte for byte |
| A message is written only right after every decision it used is confirmed | A topic is authorized before it attaches and again before delivery, A message is written only right after every decision it used is confirmed | no, byte for byte |
| Re-validation sends no review past its deadline | Re-validating one message has a deadline, Re-validation sends no review past its deadline | no, byte for byte |
| A reader allowed single names is refused the list topic | The instance list topics follow the read model's list rule, A reader allowed single names is refused the list topic | no, byte for byte |
| A forbidden topic is closed, not served | A denied topic is closed and delivers nothing, A forbidden topic is closed, not served | no, byte for byte |
| A topic denied while the stream is away is closed on reconnect | A closed message that did not leave is sent on the next connection, A topic denied while the stream is away is closed on reconnect | no, byte for byte |
| A closing lost with its connection is sent on the next | A closed message that did not leave is sent on the next connection, A closing lost with its connection is sent on the next | no, byte for byte |
| A revoked permission stops delivery | A revocation reaches the stream within one decision lifetime, A revoked permission stops delivery | no, byte for byte |
| An authorization error delivers nothing | An authorization error closes the topic, An authorization error delivers nothing | no, byte for byte |
| An unauthenticated identity opens no stream | A stream does not open for an unauthenticated identity, An unauthenticated identity opens no stream | no, byte for byte |

| Old clause | New place |
| --- | --- |
| The portal SHALL authorize a topic for the stream's identity, on the reads the topic stands for... | A topic is authorized before it attaches and again before delivery |
| A decision SHALL be cached for at most its lifetime (30 s by default), counted from when its re... | A revocation reaches the stream within one decision lifetime |
| Re-validating one message SHALL take at most one decision lifetime (30 s by default): a message... | Re-validating one message has a deadline |
| The list topics `instances` and `instances:<namespace>` SHALL follow the read model's list rule... | The instance list topics follow the read model's list rule |
| A list topic (`instances` or `instances:<namespace>`) SHALL carry only items within the namespa... | A list topic carries the items within its grant with no review per item |
| On any other topic each change SHALL be delivered only to subscribers allowed to read the objec... | Any other topic delivers only what the subscriber may read |
| A denied topic SHALL be closed on the stream with a `forbidden` code (or `unauthenticated` when... | A denied topic is closed and delivers nothing |
| A closed message not written before its connection ends, including one for a topic denied while... | A closed message that did not leave is sent on the next connection |
| An authorization error SHALL close the topic with an `upstream_unavailable` code and SHALL deli... | An authorization error closes the topic |
| A stream SHALL NOT open for an unauthenticated identity, and no authorization review SHALL be s... | A stream does not open for an unauthenticated identity |

**Each topic starts with a snapshot and continues with changes** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A change racing the snapshot is not lost | No change made after the snapshot is missed, A change racing the snapshot is not lost | no, byte for byte |
| Identifiers reveal nothing left out | Event identifiers strictly increase and reveal nothing left out, Identifiers reveal nothing left out | no, byte for byte |

| Old clause | New place |
| --- | --- |
| The first message for a topic on a stream SHALL be a snapshot carrying the topic's current item... | A topic starts with a snapshot and continues with changes |
| No change made after the snapshot was taken SHALL be missed; a change MAY repeat state the snap... | No change made after the snapshot is missed |
| Every snapshot and change SHALL carry an event identifier, and identifiers SHALL strictly incre... | Event identifiers strictly increase and reveal nothing left out |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A snapshot first | A topic starts with a snapshot and continues with changes | the clause of that requirement; WHEN and THEN use only words of the old body |

**A reconnecting stream resumes or re-reads** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Resume within the buffer | A reconnecting stream resumes from its buffer, Resume within the buffer | no, byte for byte |
| Resume past the buffer | A topic the buffer no longer covers gets a fresh snapshot, Resume past the buffer | no, byte for byte |
| An identifier from another session | An identifier that cannot be resumed opens a new stream, An identifier from another session | no, byte for byte |

| Old clause | New place |
| --- | --- |
| When a client reconnects with the `Last-Event-ID` of the stream it lost, within the resume wind... | A reconnecting stream resumes from its buffer |
| When the buffer no longer covers the gap, or the client had not yet received a topic's snapshot... | A topic the buffer no longer covers gets a fresh snapshot |
| An identifier from another session, another process lifetime, or past the resume window SHALL o... | An identifier that cannot be resumed opens a new stream |

**Streams are kept alive and bounded** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Heartbeats on a quiet stream | A stream sends a heartbeat while it is open, Heartbeats on a quiet stream | no, byte for byte |
| A slow consumer is evicted | A slow consumer is closed without delaying others, A slow consumer is evicted | no, byte for byte |
| A third tab in local mode | Streams are capped per session and per process, A third tab in local mode | no, byte for byte |
| A reloaded tab is not refused | Streams are capped per session and per process, A reloaded tab is not refused | no, byte for byte |

| Old clause | New place |
| --- | --- |
| A stream SHALL send a heartbeat at a fixed interval while it is open. | A stream sends a heartbeat while it is open |
| A stream whose client cannot keep up, so its queue of undelivered messages fills, SHALL be clos... | A slow consumer is closed without delaying others |
| A stream with no topics for the idle timeout SHALL be closed. | A stream with no topics is closed after the idle timeout |
| The portal SHALL refuse a new stream as too many streams when the session already holds its cap... | Streams are capped per session and per process |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| An idle stream | A stream with no topics is closed after the idle timeout | the clause of that requirement; WHEN and THEN use only words of the old body |

**Topics follow a fixed grammar** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A well-formed topic is accepted | The portal accepts exactly the named topics, A well-formed topic is accepted | no, byte for byte |
| A malformed topic is refused | A malformed topic is refused before the stream opens, A malformed topic is refused | no, byte for byte |
| A log topic is served | A log topic names a Pod and a container, A log topic is served | no, byte for byte |
| A malformed log topic is refused | A malformed topic is refused before the stream opens, A malformed log topic is refused | no, byte for byte |

| Old clause | New place |
| --- | --- |
| The portal SHALL accept exactly these topic names on a stream: `platform`, `instances`, `instan... | The portal accepts exactly the named topics |
| A log topic SHALL be named `log:<namespace>/<pod>/<container>` with an optional trailing `/prev... | A log topic names a Pod and a container |
| Namespaces and containers SHALL be DNS-1123 labels and names and Pods DNS-1123 subdomains. Any... | A malformed topic is refused before the stream opens |

**A subscriber is written only a change to its own document** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A change the subscriber cannot see | A subscriber is written an item only when it differs from its last document, A change the subscriber cannot see | no, byte for byte |
| A periodic refresh with nothing changed | A subscriber is written an item only when it differs from its last document, A periodic refresh with nothing changed | no, byte for byte |
| A reconnect from the last event received | After a reconnect the last document is compared when the client holds it, A reconnect from the last event received | no, byte for byte |
| A reconnect from before the last document | After a reconnect the last document is compared when the client holds it, A reconnect from before the last document | no, byte for byte |
| A delete after an upsert that carried its Removed document | A delete, a snapshot and a closed event are always written, A delete after an upsert that carried its Removed document | no, byte for byte |
| A reconnect that falls back to a snapshot | A snapshot after a reconnect becomes the compared document, A reconnect that falls back to a snapshot | no, byte for byte |
| Equal log lines | Log lines are never compared, Equal log lines | no, byte for byte |

| Old clause | New place |
| --- | --- |
| In every mode, the stream SHALL compare each `upsert` or `k8sevent` item it is about to write t... | A subscriber is written an item only when it differs from its last document |
| A `delete`, a snapshot and a `closed` event SHALL always be written. A snapshot's only item, an... | A delete, a snapshot and a closed event are always written |
| After a reconnect, the document last written SHALL still be compared with when the client's `La... | After a reconnect the last document is compared when the client holds it |
| A reconnect that falls back to a snapshot SHALL write it, and later items SHALL be compared wit... | A snapshot after a reconnect becomes the compared document |
| Log lines SHALL never be compared. | Log lines are never compared |

**A stream ends when its session expires** (form c2)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A session expires under an open stream | A stream ends with an expired event when its session expires, A session expires under an open stream | no, byte for byte |
| A message in progress at the session's end is not written | A stream ends with an expired event when its session expires, A message in progress at the session's end is not written | no, byte for byte |
| An expired session opens no stream | A stream does not open for an expired session, An expired session opens no stream | no, byte for byte |

| Old clause | New place |
| --- | --- |
| A stream SHALL end when the session that opened it expires. Its last message SHALL be an `expir... | A stream ends with an expired event when its session expires |
| A stream SHALL NOT open for a session that has already expired, and no authorization review SHA... | A stream does not open for an expired session |
| Control events (`open`, `heartbeat`, `closed`, `expired`) SHALL carry only the stream's own fie... | Control events carry only the stream's own fields |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A control event carries no topic data | Control events carry only the stream's own fields | the clause of that requirement; WHEN and THEN use only words of the old body |

