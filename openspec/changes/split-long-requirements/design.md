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


### Batch B2: read-api (16), kind-e2e-environment (7) and pod-logs (5)

Forms used: c2 (REMOVED plus ADDED under new names) for all 28 requirements. A grep of the repo outside `openspec/changes/archive/` found no reference to any of the 28 old headers outside their own spec, so no rename needs a stale-reference fix and form b was not needed. No requirement used form a or d. The 28 old bodies measured 507 to 1516 characters; the 131 new bodies measure 63 to 466 (the lengths are in the unit report).

How the tables read:

- First table of each requirement: every old scenario, its new requirement, and whether its text changed. All 97 old scenarios of the 28 requirements (`read-api` 49, `kind-e2e-environment` 29, `pod-logs` 19) kept their WHEN and THEN byte for byte. The scenario check, scoped per requirement (old text from the one requirement, new text from the blocks its Migration line names), gave 97 `same`, no `DIFF`, no `LOST`.
- Second table: each clause of an old body, the new requirement that carries it, and the opening words of that body. Every clause is in exactly one new body, with its SHALL. The `Source:` line of an old body is repeated whole in each block that carries one of its clauses; an old body with no `Source:` line got none.
- Third table, where present: scenarios that are new. Each restates a clause of its own requirement, with WHEN and THEN using only words and values of the old body. A new block needs at least one scenario; where no old scenario fitted the behavior, one restatement was written. No denied or degraded case was invented (DECISIONS.md 2026-10-19 14:00).

Wording changes beyond the split (no rule changes):

| Place | Change | Why |
| --- | --- | --- |
| `pod-logs`, `A log topic for a Pod no inventory reaches is closed as forbidden` | "SHALL each be closed" became "or ... SHALL be closed" over the same three cases | The block holds only the three cases; "each" had no list to point at once the first case left |
| `pod-logs`, `A container the Pod does not have ends the log topic` | "whose reason says so" became "whose reason says the Pod does not have it" | "so" pointed at the sentence before |
| `pod-logs`, `A log follows with timestamps, ...` | "A stream SHALL never end" became "A log stream SHALL never end" | A split body names its subject |
| `kind-e2e-environment`, `Fixture images are pinned by tag, ...` | "are pinned by tag only" became "SHALL be pinned by tag only" (and "is pinned by digest" became "SHALL be pinned by digest") | An ADDED block needs SHALL or MUST; the pin was already a rule of the old requirement |
| `pod-logs`, `The initial tail of a log ends at its first live line and is delivered once it ends` | The old "the marker delay" is kept; the body adds ", the delay of requirement" and the exact header of the block that holds the delay | The old sentence naming the delay ("within a short delay") is in that block; DECISIONS.md 2026-10-19 16:55 |
| `kind-e2e-environment`, `The capture runs the check last and every pull request runs it` | "The capture runs it last, and ... run it" became "The capture SHALL run the check last, and ... SHALL run it"; "the check" names the check of the body before, no task name is added | The old sentence had no keyword; the rule is unchanged (DECISIONS.md 2026-10-19 16:20) |
| `kind-e2e-environment`, `The capture check fails a file that breaks a rule or does not parse` | "any of these rules" became the five rules of a captured file written out: no Secret, no `metadata.managedFields`, no last-applied annotation, no `spec.values` on a ModuleInstance or ModulePackage, no `spec.versions[].schema` on a CustomResourceDefinition | "these" pointed at the sentences before; the list is the rules those sentences state |
| `read-api`, `A package's source artifact fetch URL is not served` | "SHALL not be served" became "SHALL NOT be served" | RFC 2119 form of the same rule |
| `read-api`, `Joined fields` blocks | The sentence "Each subscriber is still written only a change to its own document." is repeated verbatim in both blocks | It applies to both clauses; it has no keyword in the old text and none was added |
| `read-api`, `A condition's tone follows its type and status` | "`Stalled=True` SHALL be `abnormal`, `Reconciling=True` `progressing`, ..." became "`tone` SHALL be `abnormal` for `Stalled=True`, `progressing` for `Reconciling=True`, ..." | The elliptic list needs its verb and subject in a split body; the same five pairs and the unknown rule |
| `read-api`, resource blocks | The list of 15 paths is split into three blocks (instances, packages, cluster/platform/stream) with the path prefix and "with `GET` only" repeated; the whole-set rule "every read API resource ... with `GET` only" and its one exception stay in their own block | The list alone is over 500 characters |

The plain item 3 command compares by scenario name over the whole spec. In `read-api`, `In-cluster` and `The F1 package` also name scenarios of requirements that this unit does not rewrite, so the plain command reports a false `DIFF` for those two names (and for a new scenario named `A kind the cluster does not serve`, which was renamed to `An object of a kind the cluster does not serve` once the check showed the clash). The unit therefore ran the scoped variant of B1, which gives the same result on every other name. The names `An object no inventory reaches` and `A package holder` occur in two requirements of `read-api` each; both copies are kept, in their old order.

#### Spec `read-api`

**Resources live under a versioned, cluster-parameterized path** (form c2; old body 1062 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The default cluster | Instance resources live under the versioned cluster path, The default cluster | no, byte for byte |
| The cluster itself | The cluster, platform and stream resources live under the versioned cluster path, The cluster itself | no, byte for byte |
| Another cluster | Only the cluster default is served in milestone 1, Another cluster | no, byte for byte |
| A write method | A method a resource does not take is answered 405, A write method | no, byte for byte |
| A malformed name | A malformed namespace or name is answered 400 before any review, A malformed name | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The portal SHALL serve every read API resource under `/api/v1alpha1/clusters/{cluster}`, with `... | Instance resources live under the versioned cluster path | The portal SHALL serve the instance resources `instances`, `instances/... |
| `packages`, `packages/{namespace}/{name}`, `packages/{namespace}/{name}/graph`, `packages/{name... | Package resources live under the versioned cluster path | The portal SHALL serve the package resources `packages`, `packages/{na... |
| the cluster itself, ... `platform`, `platform/graph`, `platform/events`, `platform/registration... | The cluster, platform and stream resources live under the versioned cluster path | The portal SHALL serve the cluster itself, `platform`, `platform/graph... |
| The portal SHALL serve every read API resource under `/api/v1alpha1/clusters/{cluster}`, with `... | Every read API resource is GET only except the stream topics resource | Every read API resource SHALL be served under `/api/v1alpha1/clusters/... |
| The one exception SHALL be `stream/{stream}/topics`, which takes `POST` only. | Every read API resource is GET only except the stream topics resource | Every read API resource SHALL be served under `/api/v1alpha1/clusters/... |
| In milestone 1 the only cluster served SHALL be `default`. Any other cluster name SHALL be answ... | Only the cluster default is served in milestone 1 | In milestone 1 the only cluster served SHALL be `default`. Any other c... |
| A method a resource does not take SHALL be answered `405` with code `method_not_allowed`. | A method a resource does not take is answered 405 | A method a resource does not take SHALL be answered `405` with code `m... |
| A namespace that is not a DNS-1123 label, or a name that is not a DNS-1123 subdomain, SHALL be... | A malformed namespace or name is answered 400 before any review | A namespace that is not a DNS-1123 label, or a name that is not a DNS-... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| The package list | Package resources live under the versioned cluster path | the clause of that requirement; WHEN and THEN use only words of the old body |
| The topics resource | Every read API resource is GET only except the stream topics resource | the clause of that requirement; WHEN and THEN use only words of the old body |

**Documents are portal-shaped and versioned** (form c2; old body 585 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Both axes on an applied, degraded instance | An instance and a package carry applied state and health as two blocks, Both axes on an applied, degraded instance | no, byte for byte |
| A CLI-owned instance | An instance and a package carry applied state and health as two blocks, A CLI-owned instance | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| Every document SHALL carry `apiVersion: portal.opmodel.dev/v1alpha1` and a `kind`. | Every document carries apiVersion and kind and embeds no raw status | Every document SHALL carry `apiVersion: portal.opmodel.dev/v1alpha1` a... |
| No document SHALL embed a raw custom-resource status. | Every document carries apiVersion and kind and embeds no raw status | Every document SHALL carry `apiVersion: portal.opmodel.dev/v1alpha1` a... |
| An instance and a package SHALL carry the operator's applied state (`reconcile`: state, reason,... | An instance and a package carry applied state and health as two blocks | An instance and a package SHALL carry the operator's applied state (`r... |
| Enumerated values SHALL be documented as open: a client is told to treat an unknown value as un... | Enumerated values are documented as open | Enumerated values SHALL be documented as open: a client is told to tre... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A document's identity | Every document carries apiVersion and kind and embeds no raw status | the clause of that requirement; WHEN and THEN use only words of the old body |
| An unknown value | Enumerated values are documented as open | the clause of that requirement; WHEN and THEN use only words of the old body |

**Errors are problem documents** (form c2; old body 585 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Authorization unavailable | The error codes of a problem document are a documented open set, Authorization unavailable | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| Every error SHALL be an RFC 9457 `application/problem+json` document with `type`, `title`, `sta... | Every error is a problem document with a code | Every error SHALL be an RFC 9457 `application/problem+json` document w... |
| The codes SHALL be `unauthenticated` (401), `forbidden` (403), `not_found` (404), `bad_request`... | The error codes of a problem document are a documented open set | The `code` of a problem document SHALL be `unauthenticated` (401), `fo... |
| No problem document SHALL carry a credential, an identity or a review's error text. | No problem document carries a credential, an identity or a review's error text | No problem document SHALL carry a credential, an identity or a review'... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A problem document | Every error is a problem document with a code | the clause of that requirement; WHEN and THEN use only words of the old body |
| A review fails | No problem document carries a credential, an identity or a review's error text | the clause of that requirement; WHEN and THEN use only words of the old body |

**Events are served only about objects an inventory reaches** (form c2; old body 918 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| An inventory object's events | An events resource serves the feed about its owner or one named object, An inventory object's events | no, byte for byte |
| A registration's events | An events resource serves the feed about its owner or one named object, A registration's events | no, byte for byte |
| A kind the cluster does not serve | The named object's kind is resolved after get on the owner, from the held discovery, A kind the cluster does not serve | no, byte for byte |
| An object no inventory reaches | An object no inventory reaches is refused with the forbidden document, An object no inventory reaches | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| An events resource SHALL serve the folded recent-activity feed about its owner (the instance, t... | An events resource serves the feed about its owner or one named object | An events resource SHALL serve the folded recent-activity feed about i... |
| It SHALL authorize get on the owner, get on the named object, and list of events in the namespa... | An events resource authorizes the owner, the object and the events before any lookup | An events resource SHALL authorize get on the owner, get on the named... |
| the named object's kind SHALL be resolved only after get on the owner is allowed, from the disc... | The named object's kind is resolved after get on the owner, from the held discovery | The kind of the object an events resource names SHALL be resolved only... |
| An object that is neither the owner, nor in its inventory, nor a runtime child below an invento... | An object no inventory reaches is refused with the forbidden document | An object that is neither the owner, nor in its inventory, nor a runti... |
| Lines SHALL carry type, reason, note, reporting controller, the object regarded, a count and th... | Event lines carry type, reason, note, controller, object, count and time | The lines of an events resource SHALL carry type, reason, note, report... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| Three checks before the lookup | An events resource authorizes the owner, the object and the events before any lookup | the clause of that requirement; WHEN and THEN use only words of the old body |
| An event line | Event lines carry type, reason, note, controller, object, count and time | the clause of that requirement; WHEN and THEN use only words of the old body |

**Graphs are served as documents** (form c2; old body 595 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The platform graph with a forbidden provider | The platform graph authorizes each registration's provider instance, The platform graph with a forbidden provider | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The graph resources SHALL serve the instance, package and platform graphs as `Graph` documents:... | Graph resources serve Graph documents with nodes, edges and layout | The graph resources SHALL serve the instance, package and platform gra... |
| The `expand` query parameter, repeatable, SHALL expand the group node it names, and `showScaled... | The expand and showScaledDown parameters change the graph's nodes | The `expand` query parameter of a graph resource, repeatable, SHALL ex... |
| The platform graph SHALL authorize get on each registration's provider instance for the caller... | The platform graph authorizes each registration's provider instance | The platform graph SHALL authorize get on each registration's provider... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A Graph document | Graph resources serve Graph documents with nodes, edges and layout | the clause of that requirement; WHEN and THEN use only words of the old body |
| Expanded and scaled-down nodes | The expand and showScaledDown parameters change the graph's nodes | the clause of that requirement; WHEN and THEN use only words of the old body |

**The change stream carries the GET documents** (form c2; old body 898 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A deleted instance | A deleted or missing object's topic carries a Removed document, A deleted instance | no, byte for byte |
| Deleted and recreated before the next publish | A deleted or missing object's topic carries a Removed document, Deleted and recreated before the next publish | no, byte for byte |
| An instance topic follows a health change | A change to an OPM object, an inventory object or a runtime child reaches the topics showing it, An instance topic follows a health change | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The `stream` resource SHALL open a server-sent-events stream on the topics named by its `topics... | The stream resource opens an event stream on the named topics | The `stream` resource SHALL open a server-sent-events stream on the to... |
| Each topic SHALL carry exactly one document, the one its `GET` returns: `platform` the `Platfor... | Each topic carries the document its GET returns, rendered for the subscriber | Each topic of the `stream` resource SHALL carry exactly one document,... |
| Snapshot and upsert messages SHALL carry that document rendered for the subscriber. | Each topic carries the document its GET returns, rendered for the subscriber | Each topic of the `stream` resource SHALL carry exactly one document,... |
| When the object is deleted, or does not exist at the snapshot, the topic SHALL carry a `Removed... | A deleted or missing object's topic carries a Removed document | When the object a topic of the `stream` resource shows is deleted, or... |
| A change to an OPM object, to an inventory object or to a runtime child below one SHALL reach t... | A change to an OPM object, an inventory object or a runtime child reaches the topics showing it | A change to an OPM object, to an inventory object or to a runtime chil... |
| Topics the broker does not serve SHALL be refused with `400` and code `bad_request`; a stream b... | A topic the broker does not serve is refused and a stream beyond the cap gets 429 | Topics the broker does not serve SHALL be refused with `400` and code... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| An opened stream | The stream resource opens an event stream on the named topics | the clause of that requirement; WHEN and THEN use only words of the old body |
| A topic's document | Each topic carries the document its GET returns, rendered for the subscriber | the clause of that requirement; WHEN and THEN use only words of the old body |
| A topic the broker does not serve | A topic the broker does not serve is refused and a stream beyond the cap gets 429 | the clause of that requirement; WHEN and THEN use only words of the old body |
| A stream beyond the cap | A topic the broker does not serve is refused and a stream beyond the cap gets 429 | the clause of that requirement; WHEN and THEN use only words of the old body |

**The OpenAPI document is the contract** (form c2; old body 713 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A field renamed without a breaking title | A breaking change to the OpenAPI document fails Lint unless the title marks it, A field renamed without a breaking title | no, byte for byte |
| A field added | A breaking change to the OpenAPI document fails Lint unless the title marks it, A field added | no, byte for byte |
| An enumerated value removed | A removed enumerated value fails Lint and a test compares values with the Go constants, An enumerated value removed | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The repository SHALL hold an OpenAPI 3.1 document of the read API. Every served route SHALL app... | The OpenAPI document lists every route and matches the wire types | The repository SHALL hold an OpenAPI 3.1 document of the read API. Eve... |
| A pull request that changes the document in a way `oasdiff breaking` reports as an error SHALL... | A breaking change to the OpenAPI document fails Lint unless the title marks it | A pull request that changes the OpenAPI document in a way `oasdiff bre... |
| Because `oasdiff` ignores `x-extensible-enum`, the same check SHALL fail when a value the base... | A removed enumerated value fails Lint and a test compares values with the Go constants | Because `oasdiff` ignores `x-extensible-enum`, the required `Lint` che... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A route or a wire type that differs | The OpenAPI document lists every route and matches the wire types | the clause of that requirement; WHEN and THEN use only words of the old body |

**A stream's topics change through one request** (form c2; old body 757 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Moving a stream to another page | A topics request attaches the added topics, detaches the removed ones and answers 204, Moving a stream to another page | no, byte for byte |
| A form post | A topics request of another type or shape or with an unparsable topic is answered 400, A form post | no, byte for byte |
| Another session's stream | A topics request for a stream that is not the session's open stream is answered 404, Another session's stream | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| `POST /api/v1alpha1/clusters/{cluster}/stream/{stream}/topics` with `Content-Type: application/... | A topics request attaches the added topics, detaches the removed ones and answers 204 | `POST /api/v1alpha1/clusters/{cluster}/stream/{stream}/topics` with `C... |
| A body of another type or shape, or a topic that does not parse, SHALL be answered `400` with c... | A topics request of another type or shape or with an unparsable topic is answered 400 | A `POST` to `stream/{stream}/topics` with a body of another type or sh... |
| a stream that is not this session's open stream `404` with `not_found`; | A topics request for a stream that is not the session's open stream is answered 404 | A `POST` to `stream/{stream}/topics` for a stream that is not this ses... |
| a request without the session `401` with `unauthenticated`. | A topics request without the session is answered 401 | A `POST` to `stream/{stream}/topics` without the session SHALL be answ... |
| The request SHALL read and create nothing in the cluster. | A topics request reads and creates nothing in the cluster | A `POST` to `stream/{stream}/topics` SHALL read and create nothing in... |
| In local mode a cross-origin request SHALL be refused by the front door before it reaches the r... | In local mode a cross-origin topics request is refused by the front door | In local mode a cross-origin `POST` to `stream/{stream}/topics` SHALL... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| No session | A topics request without the session is answered 401 | the clause of that requirement; WHEN and THEN use only words of the old body |
| Nothing in the cluster | A topics request reads and creates nothing in the cluster | the clause of that requirement; WHEN and THEN use only words of the old body |
| A cross-origin post | In local mode a cross-origin topics request is refused by the front door | the clause of that requirement; WHEN and THEN use only words of the old body |

**One object an inventory reaches is served stripped** (form c2; old body 860 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| podinfo's Deployment | An object resource serves an Object document with the object's reference, podinfo's Deployment | no, byte for byte |
| An object no inventory reaches | A kind the cluster does not serve and an object no inventory reaches are refused, An object no inventory reaches | no, byte for byte |
| A Secret | A core Secret is refused before any review about it, A Secret | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| `GET …/instances/{namespace}/{name}/object` and `GET …/packages/{namespace}/{name}/object`, wit... | An object resource serves an Object document with the object's reference | `GET …/instances/{namespace}/{name}/object` and `GET …/packages/{names... |
| without `metadata.managedFields`, the `kubectl.kubernetes.io/last-applied-configuration` annota... | A served object lacks managed fields, the apply annotation and values | The object an `Object` document carries SHALL lack `metadata.managedFi... |
| The reads SHALL be authorized as the events about one object are, before any lookup: `get` on t... | The object reads are authorized on the owner and then on the object before any lookup | The `object` reads of an instance or package SHALL be authorized as th... |
| a kind the cluster does not serve, an object the owner's inventory does not reach, ... SHALL ea... | A kind the cluster does not serve and an object no inventory reaches are refused | A kind the cluster does not serve and an object the owner's inventory... |
| and a core Secret SHALL each be refused `403` with code `forbidden`, a Secret before any review... | A core Secret is refused before any review about it | A core Secret SHALL be refused `403` with code `forbidden` by an `obje... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A stripped ModuleInstance | A served object lacks managed fields, the apply annotation and values | the clause of that requirement; WHEN and THEN use only words of the old body |
| Owner, then object | The object reads are authorized on the owner and then on the object before any lookup | the clause of that requirement; WHEN and THEN use only words of the old body |
| An object of a kind the cluster does not serve | A kind the cluster does not serve and an object no inventory reaches are refused | the clause of that requirement; WHEN and THEN use only words of the old body |

**Conditions carry their tone and the portal's explanation** (form c2; old body 604 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A refused registration's platform | A condition's tone follows its type and status, A refused registration's platform | no, byte for byte |
| An unknown reason | A condition carries its meaning and next step only for a reason the portal explains, An unknown reason | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| Every condition a document serves SHALL carry `tone`, how the condition reads for its type: `no... | Every condition carries a tone | Every condition a document serves SHALL carry `tone`, how the conditio... |
| `Stalled=True` SHALL be `abnormal`, `Reconciling=True` `progressing`, `ContractsFulfilled=False... | A condition's tone follows its type and status | A condition's `tone` SHALL be `abnormal` for `Stalled=True`, `progress... |
| A condition whose reason the portal explains SHALL carry its `meaning` and, when a person can a... | A condition carries its meaning and next step only for a reason the portal explains | A condition whose reason the portal explains SHALL carry its `meaning`... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A condition's tone | Every condition carries a tone | the clause of that requirement; WHEN and THEN use only words of the old body |

**In-cluster mode serves no operator text** (form c2; old body 1516 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A failed instance in-cluster | In-cluster mode serves no condition, reconcile, history or registration message, A failed instance in-cluster | no, byte for byte |
| Registration conditions in-cluster | In-cluster mode serves no condition, reconcile, history or registration message, Registration conditions in-cluster | no, byte for byte |
| Local mode | In-cluster mode serves no condition, reconcile, history or registration message, Local mode | no, byte for byte |
| Events in-cluster | In-cluster mode serves no operator-reported event note, Events in-cluster | no, byte for byte |
| An OPM object's YAML in-cluster | In-cluster mode serves no health message of an opmodel.dev object or status message in an Object, An OPM object's YAML in-cluster | no, byte for byte |
| The kubelet's events in-cluster | In-cluster mode serves text other writers wrote as written, The kubelet's events in-cluster | no, byte for byte |
| Events differing only in their note in-cluster | In-cluster mode serves events left alike once their notes are dropped as one line, Events differing only in their note in-cluster | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| In `in-cluster` mode no document, whether served by a `GET` or on the change stream, SHALL carr... | In-cluster mode serves no condition, reconcile, history or registration message | In `in-cluster` mode no document, whether served by a `GET` or on the... |
| the `note` of an event the operator reported (its `reportingController` is `opm-controller`, or... | In-cluster mode serves no operator-reported event note | In `in-cluster` mode no document, whether served by a `GET` or on the... |
| the health `message` of an inventory object or graph node of an `opmodel.dev` kind, or, in an `... | In-cluster mode serves no health message of an opmodel.dev object or status message in an Object | In `in-cluster` mode no document, whether served by a `GET` or on the... |
| Every reason, state, `tone`, `meaning`, `nextStep`, history `outcome` and `providerOf` standing... | In-cluster mode keeps every reason, state, tone, meaning, next step, outcome and providerOf standing | In `in-cluster` mode every reason, state, `tone`, `meaning`, `nextStep... |
| Text other writers wrote SHALL be served as written: the notes of events the kubelet or another... | In-cluster mode serves text other writers wrote as written | In `in-cluster` mode text other writers wrote SHALL be served as writt... |
| Events the operator reported that are left alike once their notes are dropped (same type, reaso... | In-cluster mode serves events left alike once their notes are dropped as one line | In `in-cluster` mode events the operator reported that are left alike... |
| A document type the omission does not know SHALL fail with `upstream_unavailable` rather than b... | In-cluster mode fails a document type the omission does not know | In `in-cluster` mode a document type the omission does not know SHALL... |
| The OpenAPI document SHALL say, on each of those fields, that it is absent in-cluster. | The OpenAPI document says which fields are absent in-cluster | The OpenAPI document SHALL say, on each field whose operator text is o... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| Reasons and states stay | In-cluster mode keeps every reason, state, tone, meaning, next step, outcome and providerOf standing | the clause of that requirement; WHEN and THEN use only words of the old body |
| An unknown document type in-cluster | In-cluster mode fails a document type the omission does not know | the clause of that requirement; WHEN and THEN use only words of the old body |
| An omitted field in the OpenAPI document | The OpenAPI document says which fields are absent in-cluster | the clause of that requirement; WHEN and THEN use only words of the old body |

**The cluster document names the connection** (form c2; old body 775 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Local mode on the fixture cluster | The Cluster document names the cluster, mode, source, context and caller, Local mode on the fixture cluster | no, byte for byte |
| Version not read | The Cluster document carries the API server version when the read model holds it, Version not read | no, byte for byte |
| Nothing else about the kubeconfig | The Cluster document carries no server URL or credential and makes no cluster read, Nothing else about the kubeconfig | no, byte for byte |
| In-cluster | The Cluster document is served unchanged in-cluster, In-cluster | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| `GET /api/v1alpha1/clusters/{cluster}` SHALL serve a `Cluster` document: the path cluster's `na... | The Cluster document names the cluster, mode, source, context and caller | `GET /api/v1alpha1/clusters/{cluster}` SHALL serve a `Cluster` documen... |
| and the API server's `kubernetesVersion` as the read model holds it, absent when the read model... | The Cluster document carries the API server version when the read model holds it | The `Cluster` document SHALL carry the API server's `kubernetesVersion... |
| It SHALL carry no server URL, kubeconfig user entry or credential, and no other identity than t... | The Cluster document carries no server URL or credential and makes no cluster read | The `Cluster` document SHALL carry no server URL, kubeconfig user entr... |
| A request without a principal SHALL be answered `401` with code `unauthenticated`. | A Cluster request without a principal is answered 401 | A request for the `Cluster` document without a principal SHALL be answ... |
| In `in-cluster` mode the document SHALL be served unchanged: it carries no operator text. | The Cluster document is served unchanged in-cluster | In `in-cluster` mode the `Cluster` document SHALL be served unchanged:... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| No principal | A Cluster request without a principal is answered 401 | the clause of that requirement; WHEN and THEN use only words of the old body |

**Packages carry their interval and source artifact** (form c2; old body 538 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The F1 package | A package carries its interval and source artifact as recorded, The F1 package | no, byte for byte |
| A package with an applier | A package document carries its serviceAccountName when set, A package with an applier | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| A package document and a package list item SHALL carry `interval`, the package's `spec.interval... | A package carries its interval and source artifact as recorded | A package document and a package list item SHALL carry `interval`, the... |
| Neither SHALL be filled with a default or a guess. | A package carries its interval and source artifact as recorded | A package document and a package list item SHALL carry `interval`, the... |
| The source artifact's fetch URL SHALL not be served. | A package's source artifact fetch URL is not served | The fetch URL of a package's source artifact SHALL NOT be served. Sour... |
| A package document SHALL also carry `serviceAccountName`, the package's `spec.serviceAccountNam... | A package document carries its serviceAccountName when set | A package document SHALL also carry `serviceAccountName`, the package'... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| The fetch URL | A package's source artifact fetch URL is not served | the clause of that requirement; WHEN and THEN use only words of the old body |

**Registrations carry their conditions and holders** (form c2; old body 600 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The accepted claim's holder | A registration carries the instances and packages that hold it, The accepted claim's holder | no, byte for byte |
| A claim nobody holds | A registration carries the instances and packages that hold it, A claim nobody holds | no, byte for byte |
| A package holder | A registration carries the instances and packages that hold it, A package holder | no, byte for byte |
| Instances not listable everywhere | heldByPartial is true whenever the caller cannot see every holder, Instances not listable everywhere | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| Each registration in the platform document SHALL carry its `conditions` with `tone`, `meaning`... | A registration carries its conditions with tone, meaning and next step | Each registration in the platform document SHALL carry its `conditions... |
| and `heldBy`: the ModuleInstances and ModulePackages the caller may read whose inventory holds... | A registration carries the instances and packages that hold it | Each registration in the platform document SHALL carry `heldBy`: the M... |
| `heldByPartial` SHALL be true whenever the caller lacks a cluster-wide list of ModuleInstances... | heldByPartial is true whenever the caller cannot see every holder | `heldByPartial` of a registration SHALL be true whenever the caller la... |
| `provider` SHALL keep naming the ModuleInstance `spec.providerRef` names. | A registration's provider keeps naming the ModuleInstance its providerRef names | The `provider` of a registration SHALL keep naming the ModuleInstance... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A registration's conditions | A registration carries its conditions with tone, meaning and next step | the clause of that requirement; WHEN and THEN use only words of the old body |
| The provider | A registration's provider keeps naming the ModuleInstance its providerRef names | the clause of that requirement; WHEN and THEN use only words of the old body |

**Instances and packages name the registrations they hold** (form c2; old body 782 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The F1 provider | An instance or package carries one providerOf entry per registration it holds, The F1 provider | no, byte for byte |
| A package holder | providerRefMatches is true only for a ModuleInstance the providerRef names, A package holder | no, byte for byte |
| Registrations forbidden | A registration the caller may not read carries its name and access only, Registrations forbidden | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| An instance or package, in its document and its list item, SHALL carry `providerOf`: one entry... | An instance or package carries one providerOf entry per registration it holds | An instance or package, in its document and its list item, SHALL carry... |
| `providerRefMatches`: true only when the owner is a ModuleInstance whose namespace and name the... | providerRefMatches is true only for a ModuleInstance the providerRef names | The `providerRefMatches` of a `providerOf` entry SHALL be true only wh... |
| A registration the inventory names but the cluster does not hold SHALL carry verdict `Unknown`... | A registration the cluster does not hold carries verdict Unknown | A `providerOf` entry for a registration the inventory names but the cl... |
| A registration the caller may not read SHALL carry its name and `access: forbidden` only, with... | A registration the caller may not read carries its name and access only | A `providerOf` entry for a registration the caller may not read SHALL... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A registration the cluster does not hold | A registration the cluster does not hold carries verdict Unknown | the clause of that requirement; WHEN and THEN use only words of the old body |

**Joined fields follow changes on both sides** (form c2; old body 515 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A claim becomes active | A change to a registration reaches the topics of the owners that hold it, A claim becomes active | no, byte for byte |
| A holder drops its claim | A change to a holder reaches the platform topic, A holder drops its claim | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| A change to a TransformerRegistration SHALL reach the `instance:` or `package:` topic of every... | A change to a registration reaches the topics of the owners that hold it | A change to a TransformerRegistration SHALL reach the `instance:` or `... |
| Each subscriber is still written only a change to its own document. | A change to a registration reaches the topics of the owners that hold it | A change to a TransformerRegistration SHALL reach the `instance:` or `... |
| A change to a ModuleInstance or ModulePackage whose inventory holds a TransformerRegistration,... | A change to a holder reaches the platform topic | A change to a ModuleInstance or ModulePackage whose inventory holds a... |
| Each subscriber is still written only a change to its own document. | A change to a holder reaches the platform topic | A change to a ModuleInstance or ModulePackage whose inventory holds a... |


#### Spec `kind-e2e-environment`

**The fixture cluster is created from pinned released artifacts** (form c2; old body 1184 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Fresh environment | The fixture cluster is a kind cluster named in the opm-portal-e2e family, Fresh environment | no, byte for byte |
| Second cluster beside the default | The fixture cluster is a kind cluster named in the opm-portal-e2e family, Second cluster beside the default | no, byte for byte |
| Cluster name outside the family | The fixture cluster is a kind cluster named in the opm-portal-e2e family, Cluster name outside the family | no, byte for byte |
| Cluster already exists | The fixture cluster is a kind cluster named in the opm-portal-e2e family, Cluster already exists | no, byte for byte |
| kind differs from the pin | The environment refuses a kind that is not the pinned release, kind differs from the pin | no, byte for byte |
| Tampered download | The opm CLI download is compared with its pinned sha256, Tampered download | no, byte for byte |
| Catalog pin | The operator and the cluster Platform are installed at pinned versions, Catalog pin | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| `task e2e:up` SHALL create a kind cluster named `opm-portal-e2e`, or the name `E2E_CLUSTER` giv... | The fixture cluster is a kind cluster named in the opm-portal-e2e family | `task e2e:up` SHALL create a kind cluster named `opm-portal-e2e`, or t... |
| from the node image pinned in `test/e2e/versions.env`, | The kind cluster uses the pinned node image and a recorded provider | `task e2e:up` SHALL create the kind cluster from the node image pinned... |
| with podman as the default provider and docker when `E2E_PROVIDER=docker`, and SHALL record the... | The kind cluster uses the pinned node image and a recorded provider | `task e2e:up` SHALL create the kind cluster from the node image pinned... |
| It SHALL refuse to run when the `kind` on `PATH` is not the pinned release. | The environment refuses a kind that is not the pinned release | `task e2e:up` SHALL refuse to run when the `kind` on `PATH` is not the... |
| It SHALL download the `opm` CLI release pinned in `versions.env`, SHALL compare the archive's s... | The opm CLI download is compared with its pinned sha256 | `task e2e:up` SHALL download the `opm` CLI release pinned in `versions... |
| It SHALL install the opm-operator release `OPM_OPERATOR_VERSION` names (the CLI's embedded vers... | The operator and the cluster Platform are installed at pinned versions | `task e2e:up` SHALL install the opm-operator release `OPM_OPERATOR_VER... |
| Container images a fixture module chooses itself (cert-manager's three images share one digest... | Fixture images are pinned by tag, the CLI-owned instance's image by digest | Container images a fixture module chooses itself (cert-manager's three... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| The recorded provider | The kind cluster uses the pinned node image and a recorded provider | the clause of that requirement; WHEN and THEN use only words of the old body |
| Image pins | Fixture images are pinned by tag, the CLI-owned instance's image by digest | the clause of that requirement; WHEN and THEN use only words of the old body |

**The scripts touch only the fixture cluster** (form c2; old body 507 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Developer's own context is untouched | Every e2e script call names the fixture cluster's kubeconfig and context, Developer's own context is untouched | no, byte for byte |
| Non-loopback server | An e2e script checks that the API server is on a loopback address, Non-loopback server | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| Every `kubectl` and `opm` call the e2e scripts make SHALL pass the fixture cluster's kubeconfig... | Every e2e script call names the fixture cluster's kubeconfig and context | Every `kubectl` and `opm` call the e2e scripts make SHALL pass the fix... |
| Before applying or reading anything, a script SHALL check that the context's API server is on a... | An e2e script checks that the API server is on a loopback address | Before applying or reading anything, an e2e script SHALL check that th... |
| `task e2e:down` SHALL delete only the fixture cluster it names. | task e2e:down deletes only the fixture cluster it names | `task e2e:down` SHALL delete only the fixture cluster it names.... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| Down names one cluster | task e2e:down deletes only the fixture cluster it names | the clause of that requirement; WHEN and THEN use only words of the old body |

**The F1 fixture set covers the states the portal renders** (form c2; old body 986 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Deliberate refusal fixture | The deliberate refusal fixture is a registration no operator accepts, Deliberate refusal fixture | no, byte for byte |
| A required fixture never becomes ready | task e2e:up fails when a required fixture is not ready, A required fixture never becomes ready | no, byte for byte |
| Deliberate refusal fixture accepted | task e2e:up fails when the deliberate refusal fixture gets no verdict or is accepted, Deliberate refusal fixture accepted | no, byte for byte |
| Released operator refuses the backup claim | task e2e:up records the state of the package, the claim and the consumer without failing, Released operator refuses the backup claim | no, byte for byte |
| Operator accepts the backup claim | task e2e:up records the state of the package, the claim and the consumer without failing, Operator accepts the backup claim | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| `task e2e:up` SHALL apply the fixture set F1 and wait until it settles: cert-manager (`opmodel.... | task e2e:up applies the fixture set F1 and waits until it settles | `task e2e:up` SHALL apply the fixture set F1 and wait until it settles... |
| a deliberate refusal fixture: a TransformerRegistration applied by hand, labelled `e2e.opmodel.... | The deliberate refusal fixture is a registration no operator accepts | The fixture set F1 SHALL hold a deliberate refusal fixture: a Transfor... |
| It SHALL fail when cert-manager, podinfo, backup-provider or the CLI-owned instance does not be... | task e2e:up fails when a required fixture is not ready | `task e2e:up` SHALL fail when cert-manager, podinfo, backup-provider o... |
| or when the deliberate refusal fixture gets no verdict or is accepted, | task e2e:up fails when the deliberate refusal fixture gets no verdict or is accepted | `task e2e:up` SHALL fail when the deliberate refusal fixture of the fi... |
| and SHALL record, without failing, the state of the ModulePackage, the backup claim and the bac... | task e2e:up records the state of the package, the claim and the consumer without failing | `task e2e:up` SHALL record, without failing, the state of the ModulePa... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| The fixture set F1 | task e2e:up applies the fixture set F1 and waits until it settles | the clause of that requirement; WHEN and THEN use only words of the old body |

**A capture holds only what the portal may serve** (form c2; old body 1333 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Clean capture | A capture writes the OPM kinds, their inventories and the fixture events, Clean capture | no, byte for byte |
| Secret in a capture | The capture check reads every file and every document under testdata/clusters, Secret in a capture | no, byte for byte |
| Values left in a capture | The capture check fails a file that breaks a rule or does not parse, Values left in a capture | no, byte for byte |
| Unparseable capture file | The capture check fails a file that breaks a rule or does not parse, Unparseable capture file | no, byte for byte |
| Hand-edited capture in a pull request | The capture runs the check last and every pull request runs it, Hand-edited capture in a pull request | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| `task e2e:capture` SHALL write the four OPM kinds, the objects their inventories name, the Repl... | A capture writes the OPM kinds, their inventories and the fixture events | `task e2e:capture` SHALL write the four OPM kinds, the objects their i... |
| It SHALL NOT get, list or watch Secrets, and SHALL skip any inventory entry of kind `Secret`. | A capture never reads a Secret | `task e2e:capture` SHALL NOT get, list or watch Secrets, and SHALL ski... |
| Every captured object SHALL lack `metadata.managedFields` and the `kubectl.kubernetes.io/last-a... | A captured object lacks managed fields, the apply annotation and values | Every object `task e2e:capture` writes SHALL lack `metadata.managedFie... |
| `task e2e:capture:check` SHALL check every file under `testdata/clusters/`, recursively and wha... | The capture check reads every file and every document under testdata/clusters | `task e2e:capture:check` SHALL check every file under `testdata/cluste... |
| A file that breaks any of these rules, or does not parse, SHALL fail it. | The capture check fails a file that breaks a rule or does not parse | A file under `testdata/clusters/` that breaks any of the rules of a ca... |
| `test/e2e/check-capture_test.sh` SHALL run the check against scratch cases for each rule before... | The capture check has a test with a scratch case for each rule | `test/e2e/check-capture_test.sh` SHALL run the check against scratch c... |
| The capture runs it last, and `task check` and the `Test` check run it on every pull request. | The capture runs the check last and every pull request runs it | The capture SHALL run the check last, and `task check` and the `Test` check SHALL run it on every... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A Secret in an inventory | A capture never reads a Secret | the clause of that requirement; WHEN and THEN use only words of the old body |
| A captured object | A captured object lacks managed fields, the apply annotation and values | the clause of that requirement; WHEN and THEN use only words of the old body |
| A scratch case per rule | The capture check has a test with a scratch case for each rule | the clause of that requirement; WHEN and THEN use only words of the old body |

**Milestone 1 reads are checked end to end on the fixture cluster** (form c2; old body 1369 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Namespace-scoped reader | task e2e:m1 checks what a namespace-scoped reader lists and gets, Namespace-scoped reader | no, byte for byte |
| Image break | task e2e:m1 breaks podinfo's image and checks the stream reports Degraded, Image break | no, byte for byte |
| Health stops following the stream | task e2e:m1 breaks podinfo's image and checks the stream reports Degraded, Health stops following the stream | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| `task e2e:m1` SHALL run the built binary in local mode against the fixture cluster and SHALL fa... | task e2e:m1 checks the Platform's registrations | `task e2e:m1` SHALL run the built binary in local mode against the fix... |
| the instance list shows each F1 instance's applied state and health as two values (the operator... | task e2e:m1 checks the instance list and the web/web page | `task e2e:m1` SHALL run the built binary in local mode against the fix... |
| and a ServiceAccount that may read the OPM kinds only in `default`, run with `--namespaces defa... | task e2e:m1 checks what a namespace-scoped reader lists and gets | `task e2e:m1` SHALL run the built binary in local mode against the fix... |
| sees podinfo's inventory objects as forbidden with a partial health and as locked rows on the p... | task e2e:m1 checks what a namespace-scoped reader sees of podinfo and on the stream | `task e2e:m1` SHALL run the built binary in local mode against the fix... |
| It SHALL patch podinfo's image tag to one that does not exist and SHALL fail unless the stream... | task e2e:m1 breaks podinfo's image and checks the stream reports Degraded | `task e2e:m1` SHALL patch podinfo's image tag to one that does not exi... |
| It SHALL revert the patch and wait for podinfo to settle before it ends. | task e2e:m1 reverts the patch and waits for podinfo to settle | `task e2e:m1` SHALL revert the patch to podinfo's image tag and wait f... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| The Platform's registrations | task e2e:m1 checks the Platform's registrations | the clause of that requirement; WHEN and THEN use only words of the old body |
| The instance list and the web/web page | task e2e:m1 checks the instance list and the web/web page | the clause of that requirement; WHEN and THEN use only words of the old body |
| The reader and podinfo's inventory | task e2e:m1 checks what a namespace-scoped reader sees of podinfo and on the stream | the clause of that requirement; WHEN and THEN use only words of the old body |
| The revert | task e2e:m1 reverts the patch and waits for podinfo to settle | the clause of that requirement; WHEN and THEN use only words of the old body |

**The environment runs on a schedule, never on pull requests** (form c2; old body 1008 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Pull request | The e2e workflow is triggered by dispatch and a nightly schedule only, Pull request | no, byte for byte |
| Nightly run | The e2e workflow uploads the capture and deletes the cluster, Nightly run | no, byte for byte |
| Verdict drift | The e2e workflow fails when the capture differs from the committed meta, Verdict drift | no, byte for byte |
| A failing local-mode test | The e2e workflow runs the local, m1 and pod tests on the cluster it created, A failing local-mode test | no, byte for byte |
| A launch regression | The e2e workflow runs the browser tests in a parallel job, A launch regression | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The repository SHALL run `task e2e:up` and `task e2e:capture` on docker kind in a workflow trig... | The e2e workflow is triggered by dispatch and a nightly schedule only | The repository SHALL run `task e2e:up` and `task e2e:capture` on docke... |
| No pull request check SHALL depend on this workflow. | The e2e workflow is triggered by dispatch and a nightly schedule only | The repository SHALL run `task e2e:up` and `task e2e:capture` on docke... |
| SHALL upload the capture as a workflow artifact, and SHALL delete the cluster whether or not th... | The e2e workflow uploads the capture and deletes the cluster | The e2e workflow SHALL upload the capture as a workflow artifact, and... |
| It SHALL fail when the capture's `meta.yaml`, apart from the capture time, the provider and the... | The e2e workflow fails when the capture differs from the committed meta | The e2e workflow SHALL fail when the capture's `meta.yaml`, apart from... |
| On the same cluster it SHALL run `task e2e:local`, `task e2e:m1` and then `task e2e:pod` whenev... | The e2e workflow runs the local, m1 and pod tests on the cluster it created | On the same cluster the e2e workflow SHALL run `task e2e:local`, `task... |
| and in a parallel job it SHALL run `task test:browser` with docker and the Playwright image pin... | The e2e workflow runs the browser tests in a parallel job | In a parallel job the e2e workflow SHALL run `task test:browser` with... |
| When a test fails it SHALL upload the test output, a dump of the fixture cluster's OPM objects,... | A failing test uploads its output, a cluster dump and the screenshots | When a test fails the e2e workflow SHALL upload the test output, a dum... |
| The workflow SHALL hold only the `contents: read` permission and SHALL use no secrets. | The e2e workflow holds only contents read and uses no secrets | The e2e workflow SHALL hold only the `contents: read` permission and S... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A failing test | A failing test uploads its output, a cluster dump and the screenshots | the clause of that requirement; WHEN and THEN use only words of the old body |
| Permissions of the workflow | The e2e workflow holds only contents read and uses no secrets | the clause of that requirement; WHEN and THEN use only words of the old body |

**The portal image is checked in a Pod on the fixture cluster** (form c2; old body 903 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The image runs in the fixture cluster | task e2e:pod builds the portal image, loads it and waits for the rollout, The image runs in the fixture cluster | no, byte for byte |
| A manifest that does not run | task e2e:pod builds the portal image, loads it and waits for the rollout, A manifest that does not run | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| `task e2e:pod` SHALL build the portal image from the repository's `Dockerfile` with the e2e pro... | task e2e:pod builds the portal image, loads it and waits for the rollout | `task e2e:pod` SHALL build the portal image from the repository's `Doc... |
| It SHALL then run `TestPod` (build tag `e2e`), which SHALL fail unless: the Pod log names the S... | task e2e:pod runs TestPod and checks the Pod log | After the Deployment has rolled out, `task e2e:pod` SHALL run `TestPod... |
| with `kubectl port-forward` from local port 8090 to the Pod's port 8090, a launch with the toke... | TestPod checks a launch and the instance list through a port-forward | `TestPod` (build tag `e2e`) SHALL fail unless, with `kubectl port-forw... |
| and a forward from local port 8091 to port 8090 is refused with `403`. | TestPod checks a forward from another local port is refused | `TestPod` (build tag `e2e`) SHALL fail unless a forward from local por... |
| It SHALL touch only the fixture cluster, through the kubeconfig and context the other e2e tasks... | task e2e:pod touches only the fixture cluster | `task e2e:pod` SHALL touch only the fixture cluster, through the kubec... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| The Pod log | task e2e:pod runs TestPod and checks the Pod log | the clause of that requirement; WHEN and THEN use only words of the old body |
| A launch through the forward | TestPod checks a launch and the instance list through a port-forward | the clause of that requirement; WHEN and THEN use only words of the old body |
| A second forward | TestPod checks a forward from another local port is refused | the clause of that requirement; WHEN and THEN use only words of the old body |
| Only the fixture cluster | task e2e:pod touches only the fixture cluster | the clause of that requirement; WHEN and THEN use only words of the old body |


#### Spec `pod-logs`

**Logs are served only for Pods an OPM inventory reaches** (form c2; old body 561 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A Pod of an instance | A log topic attaches only for a Pod an inventory reaches, A Pod of an instance | no, byte for byte |
| A Pod outside every inventory | A log topic for a Pod no inventory reaches is closed as forbidden, A Pod outside every inventory | no, byte for byte |
| A Pod whose owning instance the caller may not read | A log topic for a Pod no inventory reaches is closed as forbidden, A Pod whose owning instance the caller may not read | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| A log topic SHALL attach only when its Pod is a runtime child of an inventory object the caller... | A log topic attaches only for a Pod an inventory reaches | A log topic SHALL attach only when its Pod is a runtime child of an in... |
| A topic for a Pod no inventory reaches, a Pod reached only through objects the caller may not r... | A log topic for a Pod no inventory reaches is closed as forbidden | A log topic for a Pod no inventory reaches, for a Pod reached only thr... |
| No upstream log stream SHALL be opened for a topic that did not attach. | No upstream log stream opens for a log topic that did not attach | No upstream log stream SHALL be opened for a log topic that did not at... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A topic that did not attach | No upstream log stream opens for a log topic that did not attach | the clause of that requirement; WHEN and THEN use only words of the old body |

**Pod log access is checked before the stream starts and before each delivery** (form c2; old body 588 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| No permission, no upstream read | A log topic's read is authorized before the lookup and before the upstream stream opens, No permission, no upstream read | no, byte for byte |
| Reconnect re-checks | A log topic's authorization is renewed before delivery and on reconnect, Reconnect re-checks | no, byte for byte |
| Revoked mid-stream | A revoked permission closes a log topic within one decision lifetime plus one heartbeat, Revoked mid-stream | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| A log topic's read SHALL be `get` on the Pod's `log` subresource. | A log topic's read is authorized before the lookup and before the upstream stream opens | A log topic's read SHALL be `get` on the Pod's `log` subresource. It S... |
| It SHALL be authorized for the caller before the reachability lookup and before any upstream st... | A log topic's read is authorized before the lookup and before the upstream stream opens | A log topic's read SHALL be `get` on the Pod's `log` subresource. It S... |
| The reading identity SHALL also be allowed the read before the upstream stream opens. | A log topic's read is authorized before the lookup and before the upstream stream opens | A log topic's read SHALL be `get` on the Pod's `log` subresource. It S... |
| re-checked before every delivery once the decision behind it has expired, | A log topic's authorization is renewed before delivery and on reconnect | A log topic's read, `get` on the Pod's `log` subresource, SHALL be re-... |
| and authorized again, with the reachability, on every reconnect. | A log topic's authorization is renewed before delivery and on reconnect | A log topic's read, `get` on the Pod's `log` subresource, SHALL be re-... |
| A revoked permission SHALL close the topic with `forbidden` within one decision lifetime plus o... | A revoked permission closes a log topic within one decision lifetime plus one heartbeat | A revoked permission SHALL close a log topic with `forbidden` within o... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| An expired decision | A log topic's authorization is renewed before delivery and on reconnect | the clause of that requirement; WHEN and THEN use only words of the old body |

**Log output is bounded by the portal and marked** (form c2; old body 1274 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| An oversize line | A log line longer than the line cap is cut and marked truncated, An oversize line | no, byte for byte |
| A burst over the rate | Log lines beyond the rate are dropped and counted by a rate-limited marker, A burst over the rate | no, byte for byte |
| A burst followed by silence | Log lines beyond the rate are dropped and counted by a rate-limited marker, A burst followed by silence | no, byte for byte |
| A quiet container's tail | The initial tail of a log ends at its first live line and is delivered once it ends, A quiet container's tail | no, byte for byte |
| A large initial tail | An initial tail larger than the tail byte cap keeps its newest lines and announces the skipped ones, A large initial tail | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The portal SHALL follow a container's log with timestamps and a capped initial tail, and SHALL... | A log follows with timestamps, a capped initial tail and no byte limit of the API server | The portal SHALL follow a container's log with timestamps and a capped... |
| A stream SHALL never end because of a byte limit. | A log follows with timestamps, a capped initial tail and no byte limit of the API server | The portal SHALL follow a container's log with timestamps and a capped... |
| A line longer than the line cap SHALL be cut and marked `truncated` with the number of bytes cu... | A log line longer than the line cap is cut and marked truncated | A log line longer than the line cap SHALL be cut and marked `truncated... |
| Lines beyond the per-topic line rate or byte rate SHALL be dropped, and a `rate-limited` marker... | Log lines beyond the rate are dropped and counted by a rate-limited marker | Log lines beyond the per-topic line rate or byte rate SHALL be dropped... |
| The initial tail SHALL be at most the requested tail lines, ending at the first line stamped af... | The initial tail of a log ends at its first live line and is delivered once it ends | The initial tail of a followed container's log SHALL be at most the re... |
| The initial tail SHALL be delivered once it ends, and no later than the marker delay after its... | The initial tail of a log ends at its first live line and is delivered once it ends | The initial tail of a followed container's log SHALL be at most the re... |
| An initial tail larger than the tail byte cap SHALL keep its newest lines within the cap and sk... | An initial tail larger than the tail byte cap keeps its newest lines and announces the skipped ones | An initial tail larger than the tail byte cap SHALL keep its newest li... |
| The stamps are the node's clock and the stream's opening is the portal's, so clock skew can cou... | Clock skew leaves a log line bounded and counted | The stamps of log lines are the node's clock and the stream's opening... |
| Log lines SHALL NOT appear in the portal's own logs. | Log lines never appear in the portal's own logs | Log lines SHALL NOT appear in the portal's own logs. Source: portal:D1... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A followed container | A log follows with timestamps, a capped initial tail and no byte limit of the API server | the clause of that requirement; WHEN and THEN use only words of the old body |
| Clock skew | Clock skew leaves a log line bounded and counted | the clause of that requirement; WHEN and THEN use only words of the old body |
| The portal's own logs | Log lines never appear in the portal's own logs | the clause of that requirement; WHEN and THEN use only words of the old body |

**Containers are chosen explicitly and a stopped container ends its stream** (form c2; old body 679 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A container the Pod does not have | A container the Pod does not have ends the log topic, A container the Pod does not have | no, byte for byte |
| A crash-looping container | The previous form reads the last terminated instance once, A crash-looping container | no, byte for byte |
| A container that stops | A stopped container ends its log topic with a logend message, A container that stops | no, byte for byte |
| A container that has not started | A container that has not started ends a live log topic as waiting, A container that has not started | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| A log topic SHALL name its container, which SHALL be one of the Pod's containers, init containe... | A log topic names one container of the Pod | A log topic SHALL name its container, which SHALL be one of the Pod's... |
| a container the Pod does not have SHALL end the topic with a `logend` message whose reason says... | A container the Pod does not have ends the log topic | A log topic that names a container the Pod does not have SHALL end wit... |
| The `previous` form SHALL read the last terminated instance of the container once, without foll... | The previous form reads the last terminated instance once | The `previous` form of a log topic SHALL read the last terminated inst... |
| When the followed container stops, or the previous output has been read, the topic SHALL delive... | A stopped container ends its log topic with a logend message | When the followed container of a log topic stops, or the previous outp... |
| A live topic for a container that has not started yet SHALL end with a `logend` whose reason sa... | A container that has not started ends a live log topic as waiting | A live log topic for a container that has not started yet SHALL end wi... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A named container | A log topic names one container of the Pod | the clause of that requirement; WHEN and THEN use only words of the old body |

**One upstream stream per log topic, closed with its last subscriber** (form c2; old body 888 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Two tabs, one upstream | All subscribers of one log topic share one upstream log stream, Two tabs, one upstream | no, byte for byte |
| The last subscriber leaves | The upstream log stream closes when the log topic's last subscriber leaves, The last subscriber leaves | no, byte for byte |
| Following again after the end | A new subscription after the read ended starts a new read, Following again after the end | no, byte for byte |
| A reconnect after the end | A reconnect that resumes a stream starts no new read, A reconnect after the end | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| All subscribers of one log topic SHALL share one upstream log stream. | All subscribers of one log topic share one upstream log stream | All subscribers of one log topic SHALL share one upstream log stream.... |
| The upstream stream SHALL be closed when the topic's last subscriber leaves, whether by detachi... | The upstream log stream closes when the log topic's last subscriber leaves | The upstream log stream of a log topic SHALL be closed when the topic'... |
| A subscriber that joins a topic already streaming SHALL receive the recent lines in its snapsho... | A subscriber that joins a streaming log topic receives the recent lines, numbered | A subscriber that joins a log topic already streaming SHALL receive th... |
| each log message SHALL carry a sequence number, increasing along the topic, so a client can dis... | A subscriber that joins a streaming log topic receives the recent lines, numbered | A subscriber that joins a log topic already streaming SHALL receive th... |
| A new subscription to a topic whose read has ended, while others still hold it, SHALL start a n... | A new subscription after the read ended starts a new read | A new subscription to a log topic whose read has ended, while others s... |
| A reconnect that resumes a stream SHALL NOT start a new read: for a resumed subscription the `l... | A reconnect that resumes a stream starts no new read | A reconnect that resumes a stream SHALL NOT start a new read: for a re... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A subscriber joins a streaming topic | A subscriber that joins a streaming log topic receives the recent lines, numbered | the clause of that requirement; WHEN and THEN use only words of the old body |


### Batch B3: eight specs (29)

Forms used: c2 (REMOVED plus ADDED under new names) for all 29 requirements. A grep of the repo outside `openspec/changes/archive/` found no reference to any of the 29 old headers outside their own spec (the prose of `AGENTS.md` and `internal/health/rollup.go` names two rules, not headers), so no rename needs a stale-reference fix and form b was not needed. No requirement used form a or d. The 29 old bodies measured 502 to 1483 characters; the 81 new bodies measure 77 to 499 (the lengths are in the unit report). The new bodies are built from whole sentences of the old body (DECISIONS.md 2026-10-19 15:50): a sentence is copied unchanged, and the only added words are listed below.

How the tables read:

- First table of each requirement: every old scenario, its new requirement, and whether its text changed. All 105 old scenarios of the 29 requirements kept their WHEN and THEN byte for byte (scenario check, scoped per requirement: 105 `same`, no `DIFF`, no `LOST`).
- Second table: each clause of an old body, the new requirement that carries it, and the opening words of that body. The `Source:` line of an old body is repeated whole in each block that carries one of its clauses; an old body with no `Source:` line got none.
- Third table, where present: scenarios that are new. Each restates a clause of its own requirement, with WHEN and THEN using only words of the old body. A new block needs at least one scenario; where no old scenario fitted the behavior, one restatement was written. The only negative new scenario is `Standing is not attached for a caller who may not list registrations`, the plain negation of "only when the caller may list TransformerRegistrations". No other denied or degraded case was invented (DECISIONS.md 2026-10-19 14:00).

Wording changes beyond the split (no rule changes). A first clause of a sentence repeated as context, a subject copied from the same old requirement and a cut of a long sentence at a list boundary use only words of the old text:

| Place | Change | Why |
| --- | --- | --- |
| `read-model`, `The answer counts only what the caller may get, read and list` | "The answer SHALL count" became "The read model's answer SHALL count" | "The answer" pointed at the sentence before |
| `read-model`, `A deletion of an OPM object is reported as a deletion` | The first clause of the first sentence, up to "whose view may have changed", is repeated before the deletion sentence | "the OPM object" is defined there |
| `read-model`, the read-on-demand and registration blocks | "It SHALL keep", "It SHALL refuse" and "For a caller it SHALL include" became "The read model SHALL ..." | A split body names its subject |
| `read-authorization`, `A refused read does not tell an existing object from a missing one` | The first clause of the first sentence, up to "before it reads the object", is repeated before the second sentence | "The decision" and "the object" point at it |
| `read-authorization`, the in-cluster blocks | "The only object it SHALL create ... it SHALL NOT send" became "In-cluster the only object the portal SHALL create ... the portal SHALL NOT send"; "An identity with an empty ..." became "In-cluster an identity ..."; "Every identity is a person" became "In-cluster every identity SHALL be a person" | A split body names its subject and its condition ("In-cluster" opens the old requirement); the last sentence had no keyword |
| `read-authorization`, `In-cluster mode treats every identity as a person ...` | See the strength table | The old sentence had no keyword |
| `read-authorization`, `The portal's ServiceAccount is named by ...` | "The ServiceAccount SHALL" became "The portal's own ServiceAccount SHALL" | "the portal's own ServiceAccount" is the old words of the first sentence |
| `build-and-release`, `The binary runs local mode ...` | "`serve` SHALL run" became "For the `opm-portal` binary, `serve` SHALL run" | A split body names its subject |
| `local-mode`, `Local mode refuses ...` and `Local mode binds ...` | "It SHALL" became "`opm-portal serve` SHALL" | The subject of the first sentence of the old body |
| `local-mode`, launch blocks | `A GET /launch spends the token` repeats the first clause of the first sentence ("At startup the portal SHALL print one launch URL on standard output carrying a random token") before the `GET /launch` sentence; `A GET /launch from a browser ...` adds "as listed in requirement" and the exact header of that block after "the same way", and "It SHALL NOT redirect" became "A `GET /launch` SHALL NOT redirect" | "that token" points at the first sentence; "the same way" points at the block before (DECISIONS.md 2026-10-19 16:55) |
| `local-mode`, Pod blocks | The 555-character first sentence is cut at a list boundary in two: the part up to "an empty or anonymous one", and the condition clause plus "it SHALL print ... exactly as on a user's machine"; "that ServiceAccount" became "the Pod's ServiceAccount" (old words of the first sentence) | A sentence over 500 characters; "that" pointed at the first sentence |
| `health-evaluation`, the roll-up blocks | The first sentence ("The portal SHALL roll object health up ...") is repeated as context in the two blocks whose sentences speak of "the roll-up", "the result" and "the instance" | A split body names its subject |
| `health-evaluation`, `Applied state, first four matches` and `Applied state, remaining matches` | The 647-character first sentence is cut at the list of matches: the first block holds the matches from "a CLI owner" to "marked as retrying while `Reconciling=True`", the second block holds the matches from "`Reconciling=True` or `Ready=Unknown`" to "anything else is Unknown" and adds "when none of the matches listed in requirement" plus the exact header of the first block, "applies" | The list is read in order, so the second block must say that the first block comes first (DECISIONS.md 2026-10-19 16:55) |
| `graph-model`, the edge blocks | The 691-character edge sentence is cut at the semicolons of its list; each block repeats the opening clause ("Every edge SHALL belong to one edge kind ... which the edge names:") | A sentence over 500 characters |
| `deploy-manifest`, `Comments in the deploy files ...` | The first clause of the first sentence ("The repository SHALL ship `deploy/`, a kustomization of plain YAML") is repeated before the comment sentence | "the files" points at `deploy/` |
| `deploy-manifest`, the role blocks | The first sentence is cut at its list of grants, each block repeating its head ("The ClusterRole `opm-portal-reader` SHALL list every rule explicitly and SHALL grant only the verbs `get`, `list` and `watch`:"); the block `Role grants nothing else` repeats "SHALL grant only the verbs `get`, `list` and `watch`" after the subject; the block `A Go test fails on a role that grants more` adds "as listed in requirements" and the exact headers of the four role blocks after "any of these" | "other verb" and "these" point at the first sentence and the second (DECISIONS.md 2026-10-19 16:55); the first sentence with its Source line is over 500 |
| `deploy-manifest`, `Role grants on the non-Secret kinds ...` | The sentence "An automated check that the role covers every kind the pinned catalog renders stays with the in-cluster plan." stays word for word with no keyword | It states where a check lives; the old text had no keyword and none was added |
| `read-authorization`, `Background reads ...` | The sentence "Stopping a running informer ... is deferred to the change that builds the in-cluster command." stays word for word with no keyword | Same reason |
| `portal-docs`, security page blocks | "It SHALL state" became "The security page SHALL state"; the `Source:` line, which sat in the middle of the old body, is repeated in each of the three blocks | A split body names its subject (14:48 ruling on `Source:`) |
| `portal-docs`, `The how-to builds from source with go install at latest` | "A note on building from source SHALL" became "A note on building from source in the how-to for running the portal locally SHALL"; "The commands install" became "The commands of the how-to for running the portal locally SHALL install" | A split body names its subject; the last sentence had no keyword |
| `graph-model`, `An expanded configuration group's band is the one exception to barycenter order` | See the strength table | The old sentence had no keyword |

The plain item 3 command compares by scenario name over the whole spec. Scenario names that occur twice inside one old spec would report a false `DIFF`; the unit ran the scoped variant of B1 and B2 (old text from the one requirement, new text from the blocks its Migration line names), and also ran the plain command for each requirement (output in the unit report).

#### Spec `read-model`

**Views mark unreadable items instead of failing** (form c2; old body 503 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Inventory kind the caller may not read | Every further object in a view is authorized and marked, never failing the view, Inventory kind the caller may not read | no, byte for byte |
| Registrations the caller may not list | Every further object in a view is authorized and marked, never failing the view, Registrations the caller may not list | no, byte for byte |
| Namespace-scoped list | A list holds only items within the caller's list grant, Namespace-scoped list | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| Inside a view the caller may read, every further object (an inventory entry, a runtime child, a... | Every further object in a view is authorized and marked, never failing the view | Inside a view the caller may read, every further object (an inventory... |
| An item the caller may not read SHALL be marked forbidden and shown without its content; an ite... | Every further object in a view is authorized and marked, never failing the view | Inside a view the caller may read, every further object (an inventory... |
| A list SHALL contain only items within the namespace scope of the caller's list grant, with no... | A list holds only items within the caller's list grant | A list SHALL contain only items within the namespace scope of the call... |

**Instance views carry the runtime children below each inventory object** (form c2; old body 550 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| podinfo's Deployment | An inventory object carries the runtime children whose controller chain reaches it, podinfo's Deployment | no, byte for byte |
| Children unreadable | A child that reaches no inventory object is attached to none, and unreadable children are reported, Children unreadable | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| When the runtime children of an instance's namespaces could be read, each inventory object in t... | An inventory object carries the runtime children whose controller chain reaches it | When the runtime children of an instance's namespaces could be read, e... |
| A child whose chain reaches no inventory object SHALL NOT be attached to any. | A child that reaches no inventory object is attached to none, and unreadable children are reported | A child whose chain reaches no inventory object SHALL NOT be attached... |
| When the children could not be read, no object SHALL carry children and the workloads SHALL kee... | A child that reaches no inventory object is attached to none, and unreadable children are reported | A child whose chain reaches no inventory object SHALL NOT be attached... |

**A Pod's reach from an inventory is answered for the caller** (form c2; old body 923 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A Pod below an inventory Deployment | The read model answers which inventory object a Pod is a runtime child of, A Pod below an inventory Deployment | no, byte for byte |
| A grant that does not cover the Pod's log | The read model answers which inventory object a Pod is a runtime child of, A grant that does not cover the Pod's log | no, byte for byte |
| A Pod reached only through an object the caller may not read | The answer counts only what the caller may get, read and list, A Pod reached only through an object the caller may not read | no, byte for byte |
| A Pod whose owner the caller may not read | The answer counts only what the caller may get, read and list, A Pod whose owner the caller may not read | no, byte for byte |
| A Pod no inventory reaches | A Pod that is not reached gets one refusal, and a check that cannot be decided is unavailable, A Pod no inventory reaches | no, byte for byte |
| The owner check cannot be decided | A Pod that is not reached gets one refusal, and a check that cannot be decided is unavailable, The owner check cannot be decided | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The read model SHALL answer, for a caller and one Pod, the inventory object of a ModuleInstance... | The read model answers which inventory object a Pod is a runtime child of | The read model SHALL answer, for a caller and one Pod, the inventory o... |
| The caller's grant SHALL cover `get` on the Pod's `log` subresource before anything is looked u... | The read model answers which inventory object a Pod is a runtime child of | The read model SHALL answer, for a caller and one Pod, the inventory o... |
| The answer SHALL count only owners the caller may get, inventory objects the caller may read an... | The answer counts only what the caller may get, read and list | The read model's answer SHALL count only owners the caller may get, in... |
| A Pod that does not exist, that no inventory reaches, or that is reached only through objects t... | A Pod that is not reached gets one refusal, and a check that cannot be decided is unavailable | A Pod that does not exist, that no inventory reaches, or that is reach... |

**The read model reports which OPM object changed** (form c2; old body 521 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| An inventory object changes | The read model reports the OPM object whose view may have changed, An inventory object changes | no, byte for byte |
| The instance is deleted | A deletion of an OPM object is reported as a deletion, The instance is deleted | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The read model SHALL report, to a registered listener, the kind, namespace and name of every OP... | The read model reports the OPM object whose view may have changed | The read model SHALL report, to a registered listener, the kind, names... |
| A report SHALL carry no object content, so a listener learns nothing it could serve without a g... | The read model reports the OPM object whose view may have changed | The read model SHALL report, to a registered listener, the kind, names... |
| A deletion of the OPM object itself SHALL be reported as a deletion. | A deletion of an OPM object is reported as a deletion | The read model SHALL report, to a registered listener, the kind, names... |

**One inventory object is read on demand, stripped** (form c2; old body 523 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A Deployment with its template | One object is read on demand for a caller whose grant covers get, and is returned stripped, A Deployment with its template | no, byte for byte |
| A Secret | A core Secret is refused before any review or read, A Secret | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The read model SHALL read one object by kind, namespace and name on demand for a caller whose g... | One object is read on demand for a caller whose grant covers get, and is returned stripped | The read model SHALL read one object by kind, namespace and name on de... |
| It SHALL keep the fields the held copies drop for memory, such as a Deployment's pod template. | One object is read on demand for a caller whose grant covers get, and is returned stripped | The read model SHALL read one object by kind, namespace and name on de... |
| It SHALL refuse a core Secret before any review or read. | A core Secret is refused before any review or read | The read model SHALL refuse a core Secret before any review or read. S... |

**Registrations are joined to the owners that hold them** (form c2; old body 819 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The F1 join | The read model answers which owners hold a registration and which registrations an owner holds, The F1 join | no, byte for byte |
| A package holder | The read model answers which owners hold a registration and which registrations an owner holds, A package holder | no, byte for byte |
| A namespace the caller may not list | An owner is included only when the caller may list its kind and the answer is marked incomplete otherwise, A namespace the caller may not list | no, byte for byte |
| Incomplete even with a holder | An owner is included only when the caller may list its kind and the answer is marked incomplete otherwise, Incomplete even with a holder | no, byte for byte |
| A portal limited to namespaces | An owner is included only when the caller may list its kind and the answer is marked incomplete otherwise, A portal limited to namespaces | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The read model SHALL answer, from held state and without a read of its own, which ModuleInstanc... | The read model answers which owners hold a registration and which registrations an owner holds | The read model SHALL answer, from held state and without a read of its... |
| For a caller it SHALL include an owner only when the caller may list that owner's kind in its n... | An owner is included only when the caller may list its kind and the answer is marked incomplete otherwise | For a caller the read model SHALL include an owner only when the calle... |
| A registration's standing SHALL be attached only when the caller may list TransformerRegistrati... | A registration's standing is attached only when the caller may list registrations | A registration's standing SHALL be attached only when the caller may l... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| Standing is attached for a caller who may list registrations | A registration's standing is attached only when the caller may list registrations | the clause of that requirement; WHEN and THEN use only words of the old body |
| Standing is not attached for a caller who may not list registrations | A registration's standing is attached only when the caller may list registrations | the clause of that requirement; WHEN and THEN use only words of the old body |


#### Spec `read-authorization`

**Every read is authorized before anything is looked up** (form c2; old body 673 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A read path cannot forge an allow | A read path reads only with the proof of an allow, which covers one identity and one read, A read path cannot forge an allow | no, byte for byte |
| A proof covers only its own read | A read path reads only with the proof of an allow, which covers one identity and one read, A proof covers only its own read | no, byte for byte |
| A held proof expires with its decision | A read path reads only with the proof of an allow, which covers one identity and one read, A held proof expires with its decision | no, byte for byte |
| Existing and missing objects are refused alike | A refused read does not tell an existing object from a missing one, Existing and missing objects are refused alike | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The portal SHALL decide every Kubernetes read on the caller's identity and the read's attribute... | A read path reads only with the proof of an allow, which covers one identity and one read | The portal SHALL decide every Kubernetes read on the caller's identity... |
| The proof SHALL cover only the identity it was issued to and only its own read, and SHALL stop... | A read path reads only with the proof of an allow, which covers one identity and one read | The portal SHALL decide every Kubernetes read on the caller's identity... |
| The decision SHALL be made without reading the object, so a caller without access receives the... | A refused read does not tell an existing object from a missing one | The portal SHALL decide every Kubernetes read on the caller's identity... |

**Decisions are cached briefly per identity and request** (form c2; old body 580 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Repeat within the time-to-live | An allow or a deny is reused for a short time-to-live for the same identity and attributes, Repeat within the time-to-live | no, byte for byte |
| Repeat after the time-to-live | An allow or a deny is reused for a short time-to-live for the same identity and attributes, Repeat after the time-to-live | no, byte for byte |
| Failure is not remembered | An allow or a deny is reused for a short time-to-live for the same identity and attributes, Failure is not remembered | no, byte for byte |
| Another identity or request | An allow or a deny is reused for a short time-to-live for the same identity and attributes, Another identity or request | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The portal SHALL reuse an allow or a deny for the same identity and the same attributes for at... | An allow or a deny is reused for a short time-to-live for the same identity and attributes | The portal SHALL reuse an allow or a deny for the same identity and th... |
| A grant SHALL expire with the decision it was issued from, so a revocation is seen by new check... | A grant expires with the decision it was issued from | A grant SHALL expire with the decision it was issued from, so a revoca... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A grant expires with its decision | A grant expires with the decision it was issued from | the clause of that requirement; WHEN and THEN use only words of the old body |

**In-cluster mode asks the cluster as the signed-in user** (form c2; old body 637 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Exact identity and attributes are reviewed | In-cluster mode decides each read for a person with a SubjectAccessReview, Exact identity and attributes are reviewed | no, byte for byte |
| Review fails | In-cluster mode decides each read for a person with a SubjectAccessReview, Review fails | no, byte for byte |
| No self review is ever sent | In-cluster mode creates only subjectaccessreviews, No self review is ever sent | no, byte for byte |
| Empty claims do not fall through to the portal's identity | In-cluster mode refuses an identity with no username and does not answer it as the portal's ServiceAccount, Empty claims do not fall through to the portal's identity | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| In-cluster the portal SHALL decide each read for a person with a SubjectAccessReview carrying t... | In-cluster mode decides each read for a person with a SubjectAccessReview | In-cluster the portal SHALL decide each read for a person with a Subje... |
| The only object it SHALL create is `authorization.k8s.io` `subjectaccessreviews`: it SHALL NOT... | In-cluster mode creates only subjectaccessreviews | In-cluster the only object the portal SHALL create is `authorization.k... |
| An identity with an empty, blank or anonymous username SHALL be refused without any call, and S... | In-cluster mode refuses an identity with no username and does not answer it as the portal's ServiceAccount | In-cluster an identity with an empty, blank or anonymous username SHAL... |

**In-cluster mode refuses system identities for people** (form c2; old body 502 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| System username | In-cluster mode refuses a person with a system username or group, System username | no, byte for byte |
| System group | In-cluster mode refuses a person with a system username or group, System group | no, byte for byte |
| Claims that spell the ServiceAccount | In-cluster mode treats every identity as a person unless the portal built it as its own ServiceAccount, Claims that spell the ServiceAccount | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| In-cluster the portal SHALL refuse as unauthenticated, without any call, a person whose usernam... | In-cluster mode refuses a person with a system username or group | In-cluster the portal SHALL refuse as unauthenticated, without any cal... |
| Every identity is a person unless the portal built it as its own ServiceAccount; matching the S... | In-cluster mode treats every identity as a person unless the portal built it as its own ServiceAccount | In-cluster every identity SHALL be a person unless the portal built it... |

**Background reads hold a grant for the portal's ServiceAccount** (form c2; old body 739 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The ServiceAccount is reviewed by name | Background reads hold a grant issued from a review of the portal's ServiceAccount, The ServiceAccount is reviewed by name | no, byte for byte |
| Renewed per time-to-live | Background reads hold a grant issued from a review of the portal's ServiceAccount, Renewed per time-to-live | no, byte for byte |
| Not a ServiceAccount | The portal's ServiceAccount is named by the deployment or the mounted token, Not a ServiceAccount | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| In-cluster the read model's informers, polls and on-demand lists SHALL read only under a grant... | Background reads hold a grant issued from a review of the portal's ServiceAccount | In-cluster the read model's informers, polls and on-demand lists SHALL... |
| The review SHALL be sent when the reader first needs the grant (at startup for the watched kind... | Background reads hold a grant issued from a review of the portal's ServiceAccount | In-cluster the read model's informers, polls and on-demand lists SHALL... |
| Stopping a running informer when its review expires, as portal:D6:R10 requires, is deferred to... | The portal's ServiceAccount is named by the deployment or the mounted token | Stopping a running informer when its review expires, as portal:D6:R10... |
| The ServiceAccount SHALL be named by the deployment or read from the mounted ServiceAccount tok... | The portal's ServiceAccount is named by the deployment or the mounted token | Stopping a running informer when its review expires, as portal:D6:R10... |


#### Spec `build-and-release`

**The binary reports its version** (form c2; old body 567 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Version with no arguments | The binary prints its version and exits 0, Version with no arguments | no, byte for byte |
| Version subcommand and flag | The binary prints its version and exits 0, Version subcommand and flag | no, byte for byte |
| Unknown argument | The binary runs local mode for serve and refuses any other argument, Unknown argument | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The `opm-portal` binary SHALL print `opm-portal v<version>` on standard output and exit 0 when... | The binary prints its version and exits 0 | The `opm-portal` binary SHALL print `opm-portal v<version>` on standar... |
| `<version>` SHALL be the version of the release the source was tagged with, optionally followed... | The binary prints its version and exits 0 | The `opm-portal` binary SHALL print `opm-portal v<version>` on standar... |
| Reporting the version SHALL NOT open a network listener or contact a cluster. | The binary prints its version and exits 0 | The `opm-portal` binary SHALL print `opm-portal v<version>` on standar... |
| `serve` SHALL run local mode (the `local-mode` capability). | The binary runs local mode for serve and refuses any other argument | For the `opm-portal` binary, `serve` SHALL run local mode (the `local-... |
| Any other argument SHALL print a usage line on standard error and exit 2. | The binary runs local mode for serve and refuses any other argument | For the `opm-portal` binary, `serve` SHALL run local mode (the `local-... |

**Pull requests pass the Lint and Test checks** (form c2; old body 777 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Ordinary PR | Every pull request runs a Lint check and a Test check, Ordinary PR | no, byte for byte |
| Release PR with an unreleased OPM pin | Lint fails a release-please pull request with an unreleased OPM pin, Release PR with an unreleased OPM pin | no, byte for byte |
| Breaking API change with a breaking title | Lint fails a breaking API change unless the pull request title carries a bang, Breaking API change with a breaking title | no, byte for byte |
| Base branch without the document | Lint fails a breaking API change unless the pull request title carries a bang, Base branch without the document | no, byte for byte |
| Non-conventional title | A pull request title is a Conventional Commit with a lowercase subject, Non-conventional title | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| Every pull request SHALL run a check named exactly `Lint` and a check named `Test`. | Every pull request runs a Lint check and a Test check | Every pull request SHALL run a check named exactly `Lint` and a check... |
| `Lint` SHALL run golangci-lint and validate every OpenSpec main spec and active change under `-... | Every pull request runs a Lint check and a Test check | Every pull request SHALL run a check named exactly `Lint` and a check... |
| The repository SHALL have no other job named `Lint`. | Every pull request runs a Lint check and a Test check | Every pull request SHALL run a check named exactly `Lint` and a check... |
| On a release-please pull request (`release-please--*` head branch) `Lint` SHALL also fail when... | Lint fails a release-please pull request with an unreleased OPM pin | On a release-please pull request (`release-please--*` head branch) `Li... |
| On every pull request `Lint` SHALL compare the read API's OpenAPI document with the base branch... | Lint fails a breaking API change unless the pull request title carries a bang | On every pull request `Lint` SHALL compare the read API's OpenAPI docu... |
| A pull request title SHALL be a Conventional Commit with a lowercase subject. | A pull request title is a Conventional Commit with a lowercase subject | A pull request title SHALL be a Conventional Commit with a lowercase s... |

**A release is published only with all its assets** (form c2; old body 605 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Image job fails | A release is a draft until its archives and its signed image exist, Image job fails | no, byte for byte |
| Image runs as nonroot | A release is a draft until its archives and its signed image exist, Image runs as nonroot | no, byte for byte |
| Version image tag already holds another commit | A version image tag is never overwritten and no image is pushed outside a release, Version image tag already holds another commit | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| A release SHALL be created as a draft. | A release is a draft until its archives and its signed image exist | A release SHALL be created as a draft. Before it is published it SHALL... |
| Before it is published it SHALL carry the archives `opm-portal-linux-amd64.tar.gz`, `opm-portal... | A release is a draft until its archives and its signed image exist | A release SHALL be created as a draft. Before it is published it SHALL... |
| The draft SHALL be published only after both have succeeded. | A release is a draft until its archives and its signed image exist | A release SHALL be created as a draft. Before it is published it SHALL... |
| A version image tag SHALL never be overwritten with a different image. | A version image tag is never overwritten and no image is pushed outside a release | A version image tag SHALL never be overwritten with a different image.... |
| No image SHALL be pushed outside a release. | A version image tag is never overwritten and no image is pushed outside a release | A version image tag SHALL never be overwritten with a different image.... |

**The release key is read only on main** (form c2; old body 732 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A push to main | The release key is read in one job that runs in the release environment, A push to main | no, byte for byte |
| A workflow on another branch asks for the key | The release key is read in one job that runs in the release environment, A workflow on another branch asks for the key | no, byte for byte |
| A release builds without a cache | A job that builds or publishes a release artifact restores no Actions cache, A release builds without a cache | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The release workflow SHALL read `RELEASE_APP_PRIVATE_KEY` in exactly one job, `release-please`,... | The release key is read in one job that runs in the release environment | The release workflow SHALL read `RELEASE_APP_PRIVATE_KEY` in exactly o... |
| The `release-please` job acts only through the App token and SHALL declare no `GITHUB_TOKEN` pe... | The release key is read in one job that runs in the release environment | The release workflow SHALL read `RELEASE_APP_PRIVATE_KEY` in exactly o... |
| Every workflow SHALL declare its token permissions explicitly. | Every workflow declares its token permissions and a release job does not persist a write token | Every workflow SHALL declare its token permissions explicitly. A relea... |
| A release job that holds a write token SHALL check out without persisting it. | Every workflow declares its token permissions and a release job does not persist a write token | Every workflow SHALL declare its token permissions explicitly. A relea... |
| No job that builds or publishes a release artifact (`binaries`, `image-release`) SHALL restore... | A job that builds or publishes a release artifact restores no Actions cache | No job that builds or publishes a release artifact (`binaries`, `image... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A release job that holds a write token | Every workflow declares its token permissions and a release job does not persist a write token | the clause of that requirement; WHEN and THEN use only words of the old body |


#### Spec `local-mode`

**Local mode listens on loopback only** (form c2; old body 717 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Default address | Local mode listens on the address --addr names, 127.0.0.1:7878 by default, Default address | no, byte for byte |
| The Pod keeps its own address | Local mode listens on the address --addr names, 127.0.0.1:7878 by default, The Pod keeps its own address | no, byte for byte |
| A non-loopback address | Local mode refuses an address that is not loopback, A non-loopback address | no, byte for byte |
| The default port is taken | Local mode binds the address before it reads the kubeconfig, The default port is taken | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| `opm-portal serve` SHALL listen on the address `--addr` names, `127.0.0.1:7878` by default, so... | Local mode listens on the address --addr names, 127.0.0.1:7878 by default | `opm-portal serve` SHALL listen on the address `--addr` names, `127.0.... |
| `--addr 127.0.0.1:0` SHALL still pick a free port. | Local mode listens on the address --addr names, 127.0.0.1:7878 by default | `opm-portal serve` SHALL listen on the address `--addr` names, `127.0.... |
| It SHALL refuse, with exit code 2 and before it reads the kubeconfig, an address whose host is... | Local mode refuses an address that is not loopback | `opm-portal serve` SHALL refuse, with exit code 2 and before it reads... |
| It SHALL bind the address before it reads the kubeconfig or makes any cluster call; when the ad... | Local mode binds the address before it reads the kubeconfig | `opm-portal serve` SHALL bind the address before it reads the kubeconf... |

**Local mode reads as the kubeconfig's identity and fails closed** (form c2; old body 617 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| An anonymous kubeconfig | Local mode learns the kubeconfig's identity at startup and reads only after a review, An anonymous kubeconfig | no, byte for byte |
| The fixture cluster's admin | Local mode learns the kubeconfig's identity at startup and reads only after a review, The fixture cluster's admin | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| At startup the portal SHALL learn the kubeconfig's identity with one SelfSubjectReview and SHAL... | Local mode learns the kubeconfig's identity at startup and reads only after a review | At startup the portal SHALL learn the kubeconfig's identity with one S... |
| Every read SHALL be preceded by a SelfSubjectAccessReview for that identity and the read's exac... | Local mode learns the kubeconfig's identity at startup and reads only after a review | At startup the portal SHALL learn the kubeconfig's identity with one S... |
| In local mode the only create requests the portal sends SHALL be `selfsubjectreviews` and `self... | Local mode creates only two kinds of review | In local mode the only create requests the portal sends SHALL be `self... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| Only review objects are created | Local mode creates only two kinds of review | the clause of that requirement; WHEN and THEN use only words of the old body |

**A browser is admitted only through a one-time launch token** (form c2; old body 1483 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The launch page is removed once the token is spent | The portal prints one launch URL at startup and removes the launch page when the token is spent, The launch page is removed once the token is spent | no, byte for byte |
| Launch and read | A GET /launch spends the token, Launch and read | no, byte for byte |
| Launch from the --open page | A GET /launch from a browser that holds the live session answers the same way and does not redirect, Launch from the --open page | no, byte for byte |
| The token is used twice | A missing, wrong or spent token is refused with the same body, The token is used twice | no, byte for byte |
| A request without the session | A request without a live session is refused before any review or read, A request without the session | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| At startup the portal SHALL print one launch URL on standard output carrying a random token, an... | The portal prints one launch URL at startup and removes the launch page when the token is spent | At startup the portal SHALL print one launch URL on standard output ca... |
| The portal SHALL remove that page and its directory as soon as the token is spent, through eith... | The portal prints one launch URL at startup and removes the launch page when the token is spent | At startup the portal SHALL print one launch URL on standard output ca... |
| A `GET /launch` carrying that token SHALL spend it, set a session cookie and answer with the UI... | A GET /launch spends the token | At startup the portal SHALL print one launch URL on standard output ca... |
| A `GET /launch` from a browser that already holds the live session SHALL answer the same way, w... | A GET /launch from a browser that holds the live session answers the same way and does not redirect | A `GET /launch` from a browser that already holds the live session SHA... |
| It SHALL NOT redirect: a browser treats a redirect as part of the navigation that reached `/lau... | A GET /launch from a browser that holds the live session answers the same way and does not redirect | A `GET /launch` from a browser that already holds the live session SHA... |
| A missing, wrong or already-spent token SHALL be refused with `403` and the same body for each. | A missing, wrong or spent token is refused with the same body | A missing, wrong or already-spent token SHALL be refused with `403` an... |
| A request that does not carry a live session SHALL be refused before any authorization review o... | A request without a live session is refused before any review or read | A request that does not carry a live session SHALL be refused before a... |
| `--open` SHALL open the launch URL in the default browser without putting the token on any proc... | --open opens the launch URL without the token on a command line | `--open` SHALL open the launch URL in the default browser without putt... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| The browser is opened with the launch URL | --open opens the launch URL without the token on a command line | the clause of that requirement; WHEN and THEN use only words of the old body |

**Local mode runs unchanged in a Pod with in-cluster credentials** (form c2; old body 987 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| An identity the cluster does not name | In a Pod local mode binds loopback and fails closed on an empty or anonymous identity, An identity the cluster does not name | no, byte for byte |
| The Pod starts in local mode | In a Pod local mode prints the launch URL and applies the launch token and the browser protections, The Pod starts in local mode | no, byte for byte |
| The token is spent | In a Pod local mode prints the launch URL and applies the launch token and the browser protections, The token is spent | no, byte for byte |
| Port-forward to the bound port | A browser reaches the portal only through kubectl port-forward to the bound port, Port-forward to the bound port | no, byte for byte |
| Port-forward from another local port | A browser reaches the portal only through kubectl port-forward to the bound port, Port-forward from another local port | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| When `opm-portal serve` finds no kubeconfig and client-go loads the in-cluster configuration, t... | In a Pod local mode binds loopback and fails closed on an empty or anonymous identity | When `opm-portal serve` finds no kubeconfig and client-go loads the in... |
| SHALL print the launch URL on standard output, which becomes the container log, and SHALL apply... | In a Pod local mode prints the launch URL and applies the launch token and the browser protections | When `opm-portal serve` finds no kubeconfig and client-go loads the in... |
| Every read SHALL be made as that ServiceAccount after a SelfSubjectAccessReview for it. | In a Pod every read is made as the ServiceAccount and the log line says in-cluster | Every read SHALL be made as the Pod's ServiceAccount after a SelfSubje... |
| The startup log line naming the identity SHALL carry `source=in-cluster` in place of an empty c... | In a Pod every read is made as the ServiceAccount and the log line says in-cluster | Every read SHALL be made as the Pod's ServiceAccount after a SelfSubje... |
| A browser reaches the portal only through `kubectl port-forward` to the bound port, and the `Ho... | A browser reaches the portal only through kubectl port-forward to the bound port | A browser reaches the portal only through `kubectl port-forward` to th... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| The startup log line | In a Pod every read is made as the ServiceAccount and the log line says in-cluster | the clause of that requirement; WHEN and THEN use only words of the old body |


#### Spec `graph-model`

**Every node and edge is traceable to a recorded field** (form c2; old body 975 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Refused claim's catalog | Every edge is drawn from its kind's one source, for platform, registration and provider edges, Refused claim's catalog | no, byte for byte |
| Duplicate claim on a contributed catalog | Every edge is drawn from its kind's one source, for platform, registration and provider edges, Duplicate claim on a contributed catalog | no, byte for byte |
| cert-manager's edges | Every edge is drawn from its kind's one source, for module, package, inventory and runtime child edges, cert-manager's edges | no, byte for byte |
| Dependency in another namespace | A dependsOn entry in another namespace is drawn unverified, Dependency in another namespace | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| A graph SHALL be built only from read-model views; no graph SHALL be produced by rendering a mo... | A graph is built only from read-model views | A graph SHALL be built only from read-model views; no graph SHALL be p... |
| Every edge SHALL belong to one edge kind, and every edge of a kind SHALL be drawn from that kin... | Every edge is drawn from its kind's one source, for platform, registration and provider edges | Every edge SHALL belong to one edge kind, and every edge of a kind SHA... |
| `spec.module` for instance to module; `spec.sourceRef` and `spec.dependsOn` for package to sour... | Every edge is drawn from its kind's one source, for module, package, inventory and runtime child edges | Every edge SHALL belong to one edge kind, and every edge of a kind SHA... |
| A `spec.dependsOn` entry naming another namespace, which the operator refuses, SHALL be drawn u... | A dependsOn entry in another namespace is drawn unverified | A `spec.dependsOn` entry naming another namespace, which the operator... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A graph is built | A graph is built only from read-model views | the clause of that requirement; WHEN and THEN use only words of the old body |

**A provider disagreement is shown, not resolved** (form c2; old body 584 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Accepted claim | A registration's edge to its provider instance is verified or unverified with a reason, Accepted claim | no, byte for byte |
| Provider that does not exist | A registration's edge to its provider instance is verified or unverified with a reason, Provider that does not exist | no, byte for byte |
| Provider the caller may not read | A provider that could not be read or was not looked up is still drawn, Provider the caller may not read | no, byte for byte |
| Provider not looked up | A provider that could not be read or was not looked up is still drawn, Provider not looked up | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| Every registration SHALL have an edge to the instance its `spec.providerRef` names. | A registration's edge to its provider instance is verified or unverified with a reason | Every registration SHALL have an edge to the instance its `spec.provid... |
| The edge SHALL be verified only when that instance was read and its inventory holds a Transform... | A registration's edge to its provider instance is verified or unverified with a reason | Every registration SHALL have an edge to the instance its `spec.provid... |
| A provider that could not be read SHALL still be drawn, marked with its access; one that was no... | A provider that could not be read or was not looked up is still drawn | A provider that could not be read SHALL still be drawn, marked with it... |

**Layout is deterministic** (form c2; old body 641 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Golden stability | Each scope places nodes in fixed columns and the same graph gives the same layout, Golden stability | no, byte for byte |
| A band keeps its members in name order | An expanded configuration group's band is the one exception to barycenter order, A band keeps its members in name order | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| Each scope SHALL place nodes in a fixed column per node kind, order each column by the barycent... | Each scope places nodes in fixed columns and the same graph gives the same layout | Each scope SHALL place nodes in a fixed column per node kind, order ea... |
| Building the same graph from the same views SHALL produce identical layouts. | Each scope places nodes in fixed columns and the same graph gives the same layout | Each scope SHALL place nodes in a fixed column per node kind, order ea... |
| An expanded configuration group's band is the one exception to barycenter order: its members ar... | An expanded configuration group's band is the one exception to barycenter order | An expanded configuration group's band SHALL be the one exception to b... |


#### Spec `health-evaluation`

**Health rolls up worst-of and says when it is partial** (form c2; old body 819 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Forbidden object | Health rolls up worst-of, leaves out what the reader cannot read and says when it is partial, Forbidden object | no, byte for byte |
| Secret in the inventory | Health rolls up worst-of, leaves out what the reader cannot read and says when it is partial, Secret in the inventory | no, byte for byte |
| Nothing readable | Health rolls up worst-of, leaves out what the reader cannot read and says when it is partial, Nothing readable | no, byte for byte |
| Pods not readable | Unread children below a workload make it, its component and the instance partial, Pods not readable | no, byte for byte |
| Polled object | A result carries the oldest evaluation time and says when it is not live, Polled object | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The portal SHALL roll object health up to each component and to the instance as the worst state... | Health rolls up worst-of, leaves out what the reader cannot read and says when it is partial | The portal SHALL roll object health up to each component and to the in... |
| An object the reader may not read, or could not read, SHALL be excluded from the roll-up and SH... | Health rolls up worst-of, leaves out what the reader cannot read and says when it is partial | The portal SHALL roll object health up to each component and to the in... |
| A Secret SHALL never be evaluated, SHALL be excluded, and SHALL NOT mark the result partial. | Health rolls up worst-of, leaves out what the reader cannot read and says when it is partial | The portal SHALL roll object health up to each component and to the in... |
| A result with nothing counted SHALL be Unknown. | Health rolls up worst-of, leaves out what the reader cannot read and says when it is partial | The portal SHALL roll object health up to each component and to the in... |
| When the runtime children below the inventory workloads could not be read, each readable worklo... | Unread children below a workload make it, its component and the instance partial | The portal SHALL roll object health up to each component and to the in... |
| Each result SHALL carry the oldest evaluation time among the objects it counts, and SHALL say i... | A result carries the oldest evaluation time and says when it is not live | The portal SHALL roll object health up to each component and to the in... |

**Applied state is read from the operator's conditions** (form c2; old body 955 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| CLI-owned instance | Applied state, first four matches, CLI-owned instance | no, byte for byte |
| Apply failed and retrying | Applied state, first four matches, Apply failed and retrying | no, byte for byte |
| Package without a source | Applied state, first four matches, Package without a source | no, byte for byte |
| Spec edited, operator not caught up | Applied state, remaining matches, Spec edited, operator not caught up | no, byte for byte |
| Unknown kind | Applied state, remaining matches, Unknown kind | no, byte for byte |
| Ready instance with a broken rollout | Ready=True is Applied and never Healthy, Ready instance with a broken rollout | no, byte for byte |
| Fresh Platform with unfulfilled contracts | Ready=True is Applied and never Healthy, Fresh Platform with unfulfilled contracts | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The portal SHALL read a ModuleInstance's, ModulePackage's, Platform's or TransformerRegistratio... | Applied state, first four matches | The portal SHALL read a ModuleInstance's, ModulePackage's, Platform's... |
| `Reconciling=True` or `Ready=Unknown` is Reconciling; `Ready=True` whose `observedGeneration` i... | Applied state, remaining matches | The portal SHALL read a ModuleInstance's, ModulePackage's, Platform's... |
| `Ready=True` SHALL be Applied and never Healthy, and the applied state SHALL NOT be derived fro... | Ready=True is Applied and never Healthy | `Ready=True` SHALL be Applied and never Healthy, and the applied state... |
| Failure counters SHALL never change either axis. | Ready=True is Applied and never Healthy | `Ready=True` SHALL be Applied and never Healthy, and the applied state... |
| `ContractsFulfilled=False` and `Drifted=True` SHALL be informational notes that never change th... | Ready=True is Applied and never Healthy | `Ready=True` SHALL be Applied and never Healthy, and the applied state... |

**Registration verdicts come from the status fields** (form c2; old body 564 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Refused claim | A TransformerRegistration verdict is read from status.accepted and status.active, Refused claim | no, byte for byte |
| Accepted and active claim | A TransformerRegistration verdict is read from status.accepted and status.active, Accepted and active claim | no, byte for byte |
| Removal blocked | A TransformerRegistration verdict is read from status.accepted and status.active, Removal blocked | no, byte for byte |
| Applied state does not separate refusal from removal | For a registration the verdict decides how it is shown, Applied state does not separate refusal from removal | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The portal SHALL read a TransformerRegistration's acceptance and activation from `status.accept... | A TransformerRegistration verdict is read from status.accepted and status.active | The portal SHALL read a TransformerRegistration's acceptance and activ... |
| A registration whose `Ready` reason is `DependentsRemain` SHALL be RemovalBlocked and keep its... | A TransformerRegistration verdict is read from status.accepted and status.active | The portal SHALL read a TransformerRegistration's acceptance and activ... |
| For a TransformerRegistration the verdict, not the applied state, SHALL decide how it is shown:... | For a registration the verdict decides how it is shown | For a TransformerRegistration the verdict, not the applied state, SHAL... |


#### Spec `deploy-manifest`

**The manifest runs local mode and opens no network path** (form c2; old body 1362 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| Apply the manifest | The repository ships deploy/, a kustomization with five objects and no network path, Apply the manifest | no, byte for byte |
| A Service is added | A Go test parses deploy/ and fails on a network path or a loosened setting, A Service is added | no, byte for byte |
| The security context is loosened | A Go test parses deploy/ and fails on a network path or a loosened setting, The security context is loosened | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The repository SHALL ship `deploy/`, a kustomization of plain YAML holding exactly a Namespace... | The repository ships deploy/, a kustomization with five objects and no network path | The repository SHALL ship `deploy/`, a kustomization of plain YAML hol... |
| It SHALL hold no Service, no Ingress, no Gateway route and no probe, so `kubectl port-forward`... | The repository ships deploy/, a kustomization with five objects and no network path | The repository SHALL ship `deploy/`, a kustomization of plain YAML hol... |
| The Pod SHALL run as non-root with the `RuntimeDefault` seccomp profile, and the container SHAL... | The Pod and the Namespace are locked down | The Pod SHALL run as non-root with the `RuntimeDefault` seccomp profil... |
| A Go test SHALL parse every `*.yaml` and `*.yml` file under `deploy/` and SHALL fail on a Servi... | A Go test parses deploy/ and fails on a network path or a loosened setting | A Go test SHALL parse every `*.yaml` and `*.yml` file under `deploy/`... |
| Comments in the files SHALL carry no design citation. | Comments in the deploy files carry no design citation | The repository SHALL ship `deploy/`, a kustomization of plain YAML. Co... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| The Pod is locked down | The Pod and the Namespace are locked down | the clause of that requirement; WHEN and THEN use only words of the old body |
| A comment in a deploy file | Comments in the deploy files carry no design citation | the clause of that requirement; WHEN and THEN use only words of the old body |

**The role only reads, and never Secrets** (form c2; old body 1188 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| The shipped role | Role grants on the OPM kinds, events, pods, replicasets and jobs, The shipped role | no, byte for byte |
| The EndpointSlices rule | Role grants on endpointslices, The EndpointSlices rule | no, byte for byte |
| A wider EndpointSlices rule | Role grants on endpointslices, A wider EndpointSlices rule | no, byte for byte |
| A write on EndpointSlices | Role grants on endpointslices, A write on EndpointSlices | no, byte for byte |
| A provider kind | Role grants on the non-Secret kinds the catalog renders, A provider kind | no, byte for byte |
| A denied role | A Go test fails on a role that grants more, A denied role | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The ClusterRole `opm-portal-reader` SHALL list every rule explicitly and SHALL grant only the v... | Role grants on the OPM kinds, events, pods, replicasets and jobs | The ClusterRole `opm-portal-reader` SHALL list every rule explicitly a... |
| on `discovery.k8s.io` `endpointslices` only `list` | Role grants on endpointslices | The ClusterRole `opm-portal-reader` SHALL list every rule explicitly a... |
| on the non-Secret kinds the OPM catalog's transformers render, a list maintained by hand agains... | Role grants on the non-Secret kinds the catalog renders | The ClusterRole `opm-portal-reader` SHALL list every rule explicitly a... |
| An inventory object of a kind the role does not grant SHALL show as not readable, never omitted... | Role grants on the non-Secret kinds the catalog renders | The ClusterRole `opm-portal-reader` SHALL list every rule explicitly a... |
| An automated check that the role covers every kind the pinned catalog renders stays with the in... | Role grants on the non-Secret kinds the catalog renders | The ClusterRole `opm-portal-reader` SHALL list every rule explicitly a... |
| It SHALL NOT grant any other verb, `secrets`, `impersonate`, a `*` in any group, resource or ve... | Role grants nothing else | The ClusterRole `opm-portal-reader` SHALL grant only the verbs `get`,... |
| A Go test SHALL fail on any of these, and SHALL show it does by refusing a table of denied role... | A Go test fails on a role that grants more | A Go test SHALL fail on any of these, as listed in requirements "Role... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| The role grants no Secrets | Role grants nothing else | the clause of that requirement; WHEN and THEN use only words of the old body |


#### Spec `portal-docs`

**The security page states what the portal reads and shows** (form c2; old body 869 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A reader asks whether a password set in values is safe | The security page states that Secret data and spec.values are never read, A reader asks whether a password set in values is safe | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The security page SHALL state that local mode listens on loopback only, admits a browser only t... | The security page states how local mode is closed to other readers | The security page SHALL state that local mode listens on loopback only... |
| It SHALL state that the portal never reads Secret data and serves no instance's or package's `s... | The security page states that Secret data and spec.values are never read | The security page SHALL state that the portal never reads Secret data... |
| It SHALL state that local mode shows condition messages and event notes verbatim, as the operat... | The security page states that local mode shows condition messages and event notes verbatim | The security page SHALL state that local mode shows condition messages... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| Local mode is closed to other readers | The security page states how local mode is closed to other readers | the clause of that requirement; WHEN and THEN use only words of the old body |
| A reader looks up condition messages | The security page states that local mode shows condition messages and event notes verbatim | the clause of that requirement; WHEN and THEN use only words of the old body |

**The local-mode how-to installs from a release** (form c2; old body 504 characters)

| Old scenario name | New place (requirement, scenario name) | Changed text? |
| --- | --- | --- |
| A reader follows the how-to in an older release's bundle | The how-to installs opm-portal from the GitHub releases page and names no version or branch, A reader follows the how-to in an older release's bundle | no, byte for byte |

| Old clause | New requirement | New sentence (opening words of its body) |
| --- | --- | --- |
| The how-to for running the portal locally SHALL install `opm-portal` from the portal's GitHub r... | The how-to installs opm-portal from the GitHub releases page and names no version or branch | The how-to for running the portal locally SHALL install `opm-portal` f... |
| Its commands SHALL name no version or branch, so they never break and never install unreleased... | The how-to installs opm-portal from the GitHub releases page and names no version or branch | The how-to for running the portal locally SHALL install `opm-portal` f... |
| A note on building from source SHALL use `go install` with `latest`, never a branch. | The how-to builds from source with go install at latest | A note on building from source in the how-to for running the portal lo... |
| The commands install the newest release, which can be newer than the pages of an older release'... | The how-to builds from source with go install at latest | A note on building from source in the how-to for running the portal lo... |

| New scenario | In requirement | Restates |
| --- | --- | --- |
| A note on building from source | The how-to builds from source with go install at latest | the clause of that requirement; WHEN and THEN use only words of the old body |


## Strength table

Every plain sentence that gained SHALL, MUST or MAY in a split block, by unit (DECISIONS.md 2026-10-19 16:20, 17:05). A restated scenario that repeats the SHALL of its own block is not listed. The human judges the rows.

| Spec, block | Sentence | Old text | New text |
| --- | --- | --- | --- |
| `change-stream`, `A revocation reaches the stream within one decision lifetime` (B1) | The cache bound | "A decision is cached for at most its lifetime" | "A decision SHALL be cached for at most its lifetime" |
| `web-ui`, Service Endpoints fact (B1) | The fact in the Tabs block | "the fact reads" | "the fact SHALL read" |
| `kind-e2e-environment`, `Fixture images are pinned by tag, ...` (B2) | Two pin sentences | "are pinned by tag only", "is pinned by digest" | "SHALL be pinned by tag only", "SHALL be pinned by digest" |
| `kind-e2e-environment`, `The capture runs the check last and every pull request runs it` (B2) | Two run sentences | "The capture runs it last, and ... run it" | "The capture SHALL run the check last, and ... SHALL run it" |
| `read-authorization`, `In-cluster mode treats every identity as a person unless the portal built it as its own ServiceAccount` (B3) | The person rule | "Every identity is a person unless the portal built it as its own ServiceAccount;" | "In-cluster every identity SHALL be a person unless the portal built it as its own ServiceAccount;" |
| `graph-model`, `An expanded configuration group's band is the one exception to barycenter order` (B3) | The band rule | "An expanded configuration group's band is the one exception to barycenter order:" | "An expanded configuration group's band SHALL be the one exception to barycenter order:" |
| `portal-docs`, `The how-to builds from source with go install at latest` (B3) | The newest-release sentence | "The commands install the newest release, which can be newer than ..." | "The commands of the how-to for running the portal locally SHALL install the newest release, which can be newer than ..." |

Two B3 sentences have no keyword in the old text and none was added: the deferral of the informer stop (`read-authorization`, `The portal's ServiceAccount is named by the deployment or the mounted token`) and the plan for the automated role check (`deploy-manifest`, `Role grants on the non-Secret kinds the catalog renders`).
