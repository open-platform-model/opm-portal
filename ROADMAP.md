# opm-portal roadmap

The plan and progress of the portal, from the local read-only milestone to the marketplace and
beyond. The design and its decisions live in [docs/DESIGN.md](docs/DESIGN.md), cited as
`portal:Dn`; each change is planned as an OpenSpec change under `openspec/changes/`.

Last updated: 2026-10-11 (phone-width test and contrast fix added; canvas alignment planned).

## Where it stands

| Area | State |
| --- | --- |
| Now: V1, the local read-only portal | M1 done and hardened; first release (0.1.0) pending |
| Future: in-cluster mode with OIDC sign-in | Not scheduled; OIDC removed, authorizer and safeguards dormant on main |
| Future: the marketplace (V2) | Not scheduled; waits on enhancement 0027 |
| Future: advanced features | Not scheduled; ideas, ranked |

## Now

V1 shows what OPM runs in a cluster, from the Platform down to Pods, and changes nothing
(portal:D1). The read API under `/api/v1alpha1` is the product; the HTMX UI is its first consumer
(portal:D2). Only local mode is built.

### V1 M1: local mode (done)

A binary on the user's machine reads with their kubeconfig behind a loopback launch token
(portal:D5). Merged:

| PR | What landed |
| --- | --- |
| 1 | Repository bootstrap: Taskfile, CI, release-please, OpenSpec config |
| 6 | Workload health and the applied axis, computed apart (portal:D3) |
| 7 | Throwaway kind fixture cluster and the F1 capture that seeds every golden suite |
| 8 | The authorization seam every read goes through (portal:D7) |
| 9 | The server-sent-events stream broker |
| 10 | The watched read model over the OPM kinds, inventory objects and runtime children |
| 11 | Release signing key read only in the main-only release environment |
| 12 | The graph model from recorded state only (portal:D4) |
| 16 | Bounded pod log streaming for Pods an inventory reaches (portal:D10) |
| 17 | The read API, its OpenAPI contract and the breaking-change gate |
| 19 | The portal docs bundle published to opmodel.dev |
| 20 | Local mode: `opm-portal serve` with launch token, Host check and self reviews (portal:D5) |
| 22 | The web UI: Platform, instance and package pages, graph, events, logs, YAML, live refresh |

Exit evidence shown: every page renders from the read API over the F1 capture; the scripted image
break turns health Degraded while Applied stays (portal:D3:R2/R3); goldens come from captured
cluster state; F1 holds an accepted, active registration on a released operator (v1.0.0-beta.6).

### V1 M1 hardening (done)

| PR | What landed |
| --- | --- |
| 25 | The M1 e2e suite and browser tests run nightly (issue 21 item 1, issue 23 item 2) |
| 26 | Stream revalidation bounded by time; streams end at session expiry; the spent launch page is deleted (issue 13, issue 21 items 2 and 5) |

### Also merged

| PR | What landed |
| --- | --- |
| 29 | The design and this plan moved into the repo when enhancement 0030 was withdrawn |
| 35 | OIDC sign-in removed and kept as a future plan (owner decision 2026-10-05) |
| 34 | Every `0030:` citation outside archived changes rewritten as `portal:` (issues 31 and 32) |
| 36 | Local mode in a Pod as a single-user test tool: `deploy/` with a read-only role, reached by port-forward, checked nightly by `e2e:pod` (portal:D13) |
| 39 | Web UI redesign (`redesign-web-ui`): Platform and Installed views, a theme choice and remembered filters, summary cards and tabs on instance and package pages, the Provider tab and the Catalog page, built to recorded data (portal:D14 to D18); the `Cluster` document, provider holders and package sources in the read API; local mode on `127.0.0.1:7878` |


### Next in V1

1. **First release, 0.1.0** (PR 18), held until the portal's part of the opm-operator to
   opm-controller rename merges (it renames `operatorVersion` and the owner value `operator` in the
   read API).
2. **Canvas alignment** (planned 2026-10-06; four OpenSpec changes closing the verified gaps
   between the owner's canvas and the live pages,
   [evidence 05](docs/design/evidence/05-canvas-gap-report/); portal:D19, D20, D4:R5 as amended,
   D9:R6, D14:R7):
   - `align-shell-and-tokens` first, the gate for the other three: fixes the empty details panel
     after a graph node is selected, and lands the flat look (no grid, shadows or heading marks,
     an 1840 px column), tone and kind chip tokens in light and dark, one square badge for both
     axes with the Applied words kept, underline tabs with counts, a worded live mark and the
     shared state block and tooltip (portal:D19). Open deviation for the owner's review: the
     Catalog page's Events tab carries no count where the canvas shows one, since counting it
     would add a read on every tab (portal:D19:R4, a planner reading, not a ruling).
   - `align-platform-installed-catalog`, after the gate: Platform and Catalog state blocks with
     reason counts, filter selects that list the values present with counts and apply as you type
     (portal:D14:R7), Installed counted across namespaces with a package's source in the Module
     filter, and events tables.
   - `align-owner-pages`, after the gate, beside `align-graph`: Applied, Health and Provider as
     state blocks, Resources and Events as tables, the details panel at rest, the Events tab merged
     over the inventory (`scope=all`, portal:D9:R6), Logs and YAML picked with a select, and
     `prune` on packages.
   - `align-graph`, after the gate, beside `align-owner-pages`: configuration grouped per kind
     family and opened one at a time in a frame (portal:D4:R5), three-line nodes, elbow edges, a
     legend, selection apart from the spotlight, registration standing on its node, and the
     package's source read as the caller (portal:D20). It and `align-owner-pages` share files,
     no requirement, and one function, `ownerTabsOf`, where both read the `node` and `focus`
     parameters; the split is in both proposals.
3. **A `packages` stream topic**, so the Installed list follows package changes live as it
   follows instances; until then package rows refresh on navigation.
4. **Follow-ups, none blocking:** issue 23 items 1 and 3 (provider health on the Platform page,
   graph defaults from real use); issue 21 item 3 (replace the meta-refresh hand-off page) and
   item 4 (the loopback cookie risk, accepted and documented).
5. **Phone-width test and contrast fix** (`fix-contrast-and-add-phone-check`, before the canvas
   alignment): `TestBrowserPhone` holds every F1 page to 360 px wide in three browsers; light muted
   text and the border of filter fields pass WCAG 2.2 AA, held by a token-pair test and recorded as
   portal:D19:R6. Two light-theme pairs, `--healthy` and `--degraded` on their fills, fail today and
   stay in a pending list that `align-shell-and-tokens` closes with its `--<tone>-ink` tokens.

## Future plans

Nothing here is scheduled. The order below is the order the owner set, not a timeline; each plan
starts as an OpenSpec change only when the owner schedules it.

### 1. In-cluster mode with OIDC sign-in (milestone 2)

The same binary would run in-cluster: OIDC sign-in, a SubjectAccessReview for the
signed-in user before every read, reads as the portal's own read-only ServiceAccount (portal:D6,
portal:D11). On 2026-10-05 the owner decided "Remove OIDC, but keet it as future plans"; portal:D6's
Status says which parts are planned and which exist.

Dormant on main (merged, constructed by no command):

| PR | What | Decisions |
| --- | --- | --- |
| 24 | In-cluster authorizer behind the seam: people and the portal's own reader, no self review | portal:D6:R1/R3/R7/R9/R10 |
| 27 | Hide operator message text in-cluster; diff the stream per subscriber (local mode uses the diff too) | portal:D8:R5, portal:D2:R7 |

Removed: OIDC sign-in, sessions and bearer tokens with the fail-closed identity mapping (PR 28,
portal:D6:R2/R3/R5/R6/R8 on the authentication side), by the change `remove-oidc-sessions`. The code,
tests and spec are preserved in the archived change `2026-10-05-add-oidc-sessions` and in git
history at f5a8eaa.

Open items from the reviews of PRs 24 and 28 ([issue 33](https://github.com/open-platform-model/opm-portal/issues/33)),
to settle when the plan resumes:

- End a session at the ID token's expiry, not a fixed 8 hours.
- Make the per-user session bound configurable and state it in the spec.
- Decide what the global session cap does when full: evict across users or refuse new sign-ins.
- Bound the background grant's scope: a ClusterRole test pinning exactly the verbs and resources
  the informers need.
- Access log volume: one line per decision, cached ones included; deduplicate or sample.

What it would take, in order:

1. **OIDC sign-in**, re-applied from the archived change with the session items above settled
   (portal:D6:R2/R3/R5/R6/R8).
2. **In-cluster `serve`**: the mode wired end to end, health endpoints, per-user access log
   (portal:D6:R7).
3. **The portal's ClusterRole and install manifest**, with the catalog-coverage check
   (portal:D11:R1/R2/R3).
4. **Operator viewer roles** in opm-operator, unaggregated (portal:D11:R4/R6); released before
   in-cluster mode ships.
5. **Kubernetes floor**: a standing CI job on a 1.34 cluster that exercises the event field
   selectors, and the floor documented (portal:D12). Cross-repo: the owner's 1.34 answer also puts
   opm-operator on that floor, declared, with its own CI job on 1.34; tracked here until an
   operator issue carries it.

Exit gate:

- With a test OIDC issuer on kind: two users with different namespace RBAC see different instance
  sets; a user without Platform read sees the hidden notice (portal:D11:R5); a bearer-token client
  sees what the browser sees (portal:D6:R5).
- Empty claims, `system:` groups, a failing review and a wrong-issuer or wrong-audience token each
  get the refusal portal:D6 requires, with zero Kubernetes calls for empty identity or a rejected
  token (portal:D6:R2/R8).
- The role passes the no-write, no-impersonate, no-Secret check and the catalog-coverage check
  (portal:D11:R1/R2); the authorize-before-lookup test is green (portal:D7:R1).
- A workspace `security-audit` pass with no critical finding.

Evidence still missing from the design (not blocking this plan): a ModulePackage reconciling from a
real Flux source, and a removal-blocked registration on a released operator.

### 2. The marketplace (V2)

Writes arrive with V2: a tenant browses offerings, fills a form generated from the module's
configuration, and orders an instance as themselves. **V2 cannot start before enhancement 0027
(self-service kinds) is accepted**: an order is an instance of a 0027 served kind, never a
ModuleInstance that names a module.

The marketplace contract questions live in enhancement 0027, OQ17 to OQ29 (presentation block on
the definition, schema agreement, secret and union encoding, required and computed fields,
admission, outputs, what a UI may create, under which identity, per-field errors, literal secrets,
prefill). The direction the portal brings to them, as candidates and not decisions:

- **A layered presentation contract.** The module author owns a small card (title, summary,
  category, icon path, links) in the module file, form hints and help text on `#config` fields;
  the platform team overrides presentation on the 0027 definition, field by field and inert; a
  publisher's pipeline builds an index module listing every card.
- **Forms from a purpose-built CUE walker**, because the stock CUE OpenAPI encoder failed on 19 of
  20 real modules; validation runs through the 0027 projection, then an API-server dry run as the
  user.
- **Order flow**: form, validate, review page with secrets redacted, create as the user, status
  page built on the V1 views.

Evidence: the withdrawn enhancement 0031 (module presentation contract) and 0027's experiments.

What it would take, in order, each step gated:

| Step | Delivers | Gate |
| --- | --- | --- |
| Evidence | Experiments on secret encoding, field hints, unions, card and index | none |
| Browse and preview | Card and asset gate, index module, admin registry browser, author preview | 0027:OQ17 answered; core release |
| Tenant catalog | Encoder in library, presentation on definitions, offering pages, forms rendered without submit | 0027 accepted with its OQ18 to OQ22 answered |
| Order | Dry run and submit as the user, review and status pages, edit and delete | 0027:OQ24 to OQ27 answered; write identity decided (portal:OQ1) |

### 3. Advanced features

Left out of the web UI redesign until a source exists, not ranked below:

- **A Platform health** (portal:OQ22): the Platform page shows no health for the Platform itself.
- **Catalog contents** (portal:OQ25): definitions, their descriptions and documentation links,
  and the transformers a catalog ships, on the Catalog page and the Provider tab.
- **Controller follow-ups**: the operator accepting a ModulePackage as a provider (portal:OQ23,
  [opm-operator#254](https://github.com/open-platform-model/opm-operator/issues/254)), and
  packages recording the contracts their render used (portal:OQ24).

Beyond the marketplace, ranked by value per effort:

1. **MCP server over the read API**: list offerings, describe a form, dry run, read status. A thin
   adapter.
2. **`opm module preview`**: renders card, detail page and form locally, with a completeness score.
3. **Contract fulfilment matrix**: per Platform, demanded versus provided versus active. Needs the
   operator to record demand (portal:OQ18). The first step of "define a platform".
4. **Upgrade assistant**: consumer-schema diff, change class, values that would break, render diff.
5. **Provider ecosystem view**: registrations, contracts, dependants. Cheap after item 3.
6. **Platform builder**: compose a Platform from catalogs, pick providers, see fulfilment, apply or
   open a PR. The end goal; high effort.
7. **GitOps PR mode** for orders and definition edits; also a write path that needs no
   impersonation.
8. **Headlamp plugin**, and 9. **Backstage adapter**: always adapters over the read API, never a
   base.
10. **Drift view** (needs per-entry content hashes from the operator); 11. **quota and cost
    hints**; 12. **multi-cluster hub** (portal:OQ15).

Not planned: a bespoke package format, a central OPM-hosted marketplace, multi-page wizards.

## How this file is maintained

- The PR that lands a change updates this file in the same diff: move the change to done with its
  PR number, and add what it uncovered under "Next in V1".
- A decision or open question changes in [docs/DESIGN.md](docs/DESIGN.md), not here; this file
  cites it.
- Keep one line per change. The OpenSpec change, the PR and `git log` hold the detail.
- Update "Last updated" with each edit.
