# 04-ui-redesign-screenshots: the redesigned web UI over F1

Status: Snapshot (2026-10-05)

## What this is

Twelve screenshots of the web UI as the OpenSpec change `redesign-web-ui` builds it, taken to
compare the pages with the owner-reviewed canvas ([03-ui-canvas](../03-ui-canvas/)) and to check
the light and dark themes and the phone width. They show the F1 capture
(`testdata/clusters/f1/`), served without a cluster.

## How they were made

- The pages were served by `OPM_PORTAL_UI_DEV=127.0.0.1:18080 go test ./internal/ui -run
  TestDevServe -timeout 0` over the F1 capture, reading as the test caller `alice`.
- A Playwright script drove Chromium from the pinned Playwright 1.63 image
  (`mcr.microsoft.com/playwright/python:v1.63.0-noble`, the image `task test:browser` uses), on
  2026-10-05, at a 1440 x 900 viewport with `color_scheme` light and dark, and at 360 x 900 light.
  Each page was opened, left until the network was idle, and captured full page; the hover card
  was captured after hovering the first graph node, and the scrolled header after a 1200 px wheel
  scroll.
- The run covered twelve pages in each of the three modes, 36 page shots, plus the hover card and
  the scrolled header: 38 in all. The twelve pages were `/`, `/?tab=catalogs`, `/installed`,
  `/installed?uses=opmodel.dev/catalogs/opm/traits/backup@v1alpha1`, `/instances/default/podinfo`
  on its Graph, Resources, Events and YAML tabs, `/instances/cert-manager/cert-manager`,
  `/instances/default/backup-provider?tab=provider`, `/packages/pkg/podinfo` and
  `/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0`. The twelve files here are the
  set the change cites; the other 26 were not committed.

| File | Page |
| --- | --- |
| `light-platform.png`, `dark-platform.png` | `/`: identity, status, Installed counts, Providers tab, recent events |
| `light-installed.png`, `dark-installed.png` | `/installed` |
| `phone-installed.png`, `phone-platform.png` | the same at 360 px |
| `light-instance-graph.png`, `dark-instance-graph.png` | `/instances/default/podinfo`: cards and the Graph tab |
| `light-instance-hover.png` | a node's hover card on the Graph tab |
| `light-provider.png` | `/instances/default/backup-provider?tab=provider` |
| `light-catalog.png` | `/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0` |
| `light-scrolled-header.png` | the header slimmed after scrolling |

## Read these with care

F1 is a capture, not a live cluster: times are relative to the test clock, and nothing moves.
The live behaviour (the stream, the image break) is recorded in the change's `design.md`.
