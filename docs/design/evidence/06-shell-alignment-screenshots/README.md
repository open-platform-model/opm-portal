# 06-shell-alignment-screenshots: the aligned shell over F1

Status: Snapshot (2026-10-11)

## What this is

Twenty-one screenshots of the web UI as the OpenSpec change `align-shell-and-tokens` leaves it:
the flat look and tokens, one badge shape, underline tabs with counts, the worded live mark, and
the details panel after a node selection. They are the evidence for the change's "Live checks"
(`openspec/changes/align-shell-and-tokens/design.md`, or its archived copy). They follow
[04-ui-redesign-screenshots](../04-ui-redesign-screenshots/), which shows the same pages before
the change. They show the F1 capture (`testdata/clusters/f1/`), served without a cluster.

## How they were made

- The pages were served by `OPM_PORTAL_UI_DEV=127.0.0.1:18110 go test ./internal/ui -run
  TestDevServe -timeout 0` over the F1 capture, reading as the test caller `alice`.
- A Playwright script drove Chromium from the pinned Playwright 1.63 image
  (`mcr.microsoft.com/playwright/python:v1.63.0-noble`, the image `task test:browser` uses), on
  2026-10-11. Each page was opened, left until the network was idle, and captured full page.
  - Light and dark: 1920 x 1080, `color_scheme` light and dark.
  - Phone: 360 x 800, light.
  - Forced colours: 1920 x 1080, `forced_colors="active"` (the page reported
    `matchMedia('(forced-colors: active)')` true).
  - Node selection: the `podinfo-podinfo` Deployment node was focused and activated with Enter,
    and the capture was taken once `#detail .frag-h` read `podinfo-podinfo`.
- The run covered twelve pages in each of the light, dark and phone modes, plus the node selection
  in each, the hover card in light and three pages in forced colours. These 21 files are the set the
  change cites; the other 22 of the 43 captured were not committed. Every page and node-selection capture had a
  scroll width of 1920 at the wide size and 360 at the phone size, so no page scrolled sideways.
- The forced-colours capture of the instance page first showed an underline under every tab (a
  transparent border is painted in forced colours). The branch fixed that, and commit 44fb224 later
  changed the rule again: the other tabs now draw no bottom border and take 3 px of padding. The
  forced-colours files here predate 44fb224, so they do not show the shipped tab strip. The rule is
  held by `TestPortalCSSMarksOnlyTheCurrentTabInForcedColours`, a CSS text test.

| File | Page |
| --- | --- |
| `light-platform.png`, `dark-platform.png`, `phone-platform.png` | `/`: identity, status, Installed counts, Providers and Catalogs tabs with counts |
| `light-installed.png`, `dark-installed.png`, `phone-installed.png` | `/installed` |
| `light-instance-graph.png`, `dark-instance-graph.png` | `/instances/default/podinfo`: Applied and Health cards, tabs with counts, the Graph tab |
| `light-instance-resources.png`, `dark-instance-resources.png`, `phone-instance-resources.png` | the Resources tab of the same instance |
| `light-instance-node-selected.png`, `dark-instance-node-selected.png`, `phone-instance-node-selected.png` | the Graph tab after a node selection: the details panel filled |
| `light-instance-hover.png` | a node's hover card with its Health label |
| `light-provider.png` | `/instances/default/backup-provider?tab=provider` |
| `light-catalog.png` | `/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0` |
| `light-package.png` | `/packages/pkg/podinfo` |
| `forced-installed.png`, `forced-instance-graph.png`, `forced-instance-resources.png` | forced colours, taken before 44fb224: badges keep a border; they do not show the final tab rule |

## Read these with care

F1 is a capture, not a live cluster: times are relative to the test clock, and nothing moves.
No page shows a state block or a tip yet; the follow-on changes add them, and the CSS tests and
the browser case `TestBrowserTip` cover them until then. A full-page capture of a sticky header
draws the header once, where the scroll position was.
