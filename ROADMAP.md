# opm-portal roadmap

The plan and progress of the portal, from the local read-only milestone to the marketplace and
beyond. The design and its decisions live in [docs/DESIGN.md](docs/DESIGN.md), cited as
`portal:Dn`; each change is planned as an OpenSpec change under `openspec/changes/`.

Last updated: 2026-10-05.

## Where it stands

| Milestone | State |
| --- | --- |
| V1 M1: local mode | Done; hardening merged, first release (0.1.0) pending |
| V1 M2: in-cluster | In progress: wave 1 (authorizer, OIDC sign-in, message hiding) merged; next the in-cluster command and manifests |
| V2: marketplace | Not started; waits on enhancement 0027 |
| Beyond V2 | Ideas, ranked |

## V1: read-only portal

V1 shows what OPM runs in a cluster, from the Platform down to Pods, and changes nothing
(portal:D1). The read API under `/api/v1alpha1` is the product; the HTMX UI is its first consumer
(portal:D2).

### M1: local mode (done)

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

### M1 hardening

Merged:

| PR | What landed |
| --- | --- |
| 25 | The M1 e2e suite and browser tests run nightly (issue 21 item 1, issue 23 item 2) |
| 26 | Stream revalidation bounded by time; streams end at session expiry; the spent launch page is deleted (issue 13, issue 21 items 2 and 5) |

Open: the first release, 0.1.0 (PR 18), now unblocked.

Still open: issue 23 items 1 and 3 (provider health on the Platform page, graph defaults from real
use), issue 13 items 3 and 4.

### M2: in-cluster mode (in progress)

The same binary runs in-cluster: OIDC sign-in, a SubjectAccessReview for the signed-in user before
every read, reads as the portal's own read-only ServiceAccount (portal:D6, portal:D11).

Wave 1, merged:

| PR | What landed | Decisions |
| --- | --- | --- |
| 24 | In-cluster authorizer behind the seam: people and the portal's own reader, no self review | portal:D6:R1/R3/R7/R9/R10 |
| 27 | Operator message text hidden in-cluster; the stream diffed per subscriber | portal:D8:R5, portal:D2:R7 |
| 28 | OIDC sign-in, sessions and bearer tokens, fail closed on empty or `system:` identity, prefixes required | portal:D6:R2/R3/R5/R6/R8 |

Also merged: the design and this plan moved into the repo when enhancement 0030 was withdrawn
(PR 29), and every `0030:` citation outside archived changes rewritten as `portal:` (issues 31
and 32).

Next, in order:

1. **In-cluster `serve`**: the mode wired end to end, health endpoints, per-user access log
   (portal:D6:R7), with a Dex issuer on kind for the e2e suite.
2. **The portal's ClusterRole and install manifest**, with the catalog-coverage check
   (portal:D11:R1/R2/R3).
3. **Operator viewer roles** in opm-operator, unaggregated (portal:D11:R4/R6); released before M2
   exits.
4. **Kubernetes floor**: a standing CI job on a 1.34 cluster that exercises the event field
   selectors, and the floor documented (portal:D12). Cross-repo: the owner's 1.34 answer also puts
   opm-operator on that floor, declared, with its own CI job on 1.34; tracked here until an
   operator issue carries it.

M2 exit gate:

- With a test OIDC issuer on kind: two users with different namespace RBAC see different instance
  sets; a user without Platform read sees the hidden notice (portal:D11:R5); a bearer-token client
  sees what the browser sees (portal:D6:R5).
- Empty claims, `system:` groups, a failing review and a wrong-issuer or wrong-audience token each
  get the refusal portal:D6 requires, with zero Kubernetes calls for empty identity or a rejected
  token (portal:D6:R2/R8).
- The role passes the no-write, no-impersonate, no-Secret check and the catalog-coverage check
  (portal:D11:R1/R2); the authorize-before-lookup test is green (portal:D7:R1).
- A workspace `security-audit` pass with no critical finding.

Evidence still missing from the design (not blocking M2): a ModulePackage reconciling from a real
Flux source, and a removal-blocked registration on a released operator.

## V2: marketplace

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

Milestones, each gated:

| Milestone | Delivers | Gate |
| --- | --- | --- |
| V2.0 evidence | Experiments on secret encoding, field hints, unions, card and index | none |
| M3 browse and preview | Card and asset gate, index module, admin registry browser, author preview | 0027:OQ17 answered; core release |
| M4 tenant catalog | Encoder in library, presentation on definitions, offering pages, forms rendered without submit | 0027 accepted with its OQ18 to OQ22 answered |
| M5 order | Dry run and submit as the user, review and status pages, edit and delete | 0027:OQ24 to OQ27 answered; write identity decided (portal:OQ1) |

## Beyond V2

Ranked by value per effort:

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
  PR number, and add what it uncovered under "Next" or "Still open".
- A decision or open question changes in [docs/DESIGN.md](docs/DESIGN.md), not here; this file
  cites it.
- Keep one line per change. The OpenSpec change, the PR and `git log` hold the detail.
- Update "Last updated" with each edit.
