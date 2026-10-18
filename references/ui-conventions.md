# UI conventions

Covers `internal/ui`, the pages the portal serves: the Content-Security-Policy, the API-only data path, vendored assets, untrusted text, the dev server and the browser tests.

## The data path

`internal/ui` reads only through the read API. It calls the API handler in process. A test fails if `internal/ui` imports `internal/readmodel`. The reason is Principle III of `CONSTITUTION.md`: the UI is the first consumer of `/api/v1alpha1`, and it never reads anything the API does not expose. Tests build the API with the helper package under `internal/api/`.

## Content-Security-Policy

Pages run under a CSP that allows `'self'` only. Therefore:

- No inline `<script>`.
- No inline `style` attribute and no inline `<style>` block.
- No `hx-on` attribute. htmx evaluates it as script.
- No htmx trigger filters. They need `eval`.

Put script in the page script and style in the CSS file. A page that needs a dynamic style uses a class.

## Untrusted text

Condition messages, event notes, labels, annotations and log lines come from the cluster, so anyone who can write to the cluster controls them. Render them through `html/template` only, never `text/template`. The CSP above is the second layer; do not rely on it alone.

## Vendored assets

Vendored files live in `internal/ui/static/vendor/`. Each has its SHA-256 in `CHECKSUMS` in that directory, and `internal/ui`'s tests check it. The file header of `CHECKSUMS` says each file was extracted from the npm tarball named after it, after the tarball's sha512 matched the registry integrity value.

Upgrade one asset by replacing the file, its `CHECKSUMS` line and the name the layout template loads, all in one diff. A partial upgrade fails the tests.

The portal has no frontend build step. CSS is hand-written.

## Pages and goldens

Each page has a golden file of its `<main>` (or of a fragment) in `internal/ui/testdata/golden/`. Update with `go test ./internal/ui -update`, then read the diff before you commit.

## Look at the pages without a cluster

```bash
task ui:dev
```

This serves the pages over the F1 capture at `127.0.0.1:18080`; set `UI_DEV_ADDR` to change the address. Optional variables, set in the environment:

- `OPM_PORTAL_UI_DEV_BROKEN=1`: the image-break sample.
- `OPM_PORTAL_UI_DEV_DENY=<resource>`: show that resource locked.
- `OPM_PORTAL_UI_DEV_HOLDER`: add the package that holds a refused claim.
- `OPM_PORTAL_UI_DEV_FOR`: how long it serves (default 10 minutes).

## Browser tests (`test/browser/`, build tag `browser`)

`task test:browser` runs the browser tests of `cmd/opm-portal/` in Chromium, Firefox and WebKit. It uses the Playwright image (podman by default; `OPM_PORTAL_CONTAINER_ENGINE=docker` otherwise). It pulls the image by its pinned digest and installs the matching Playwright package, so it needs the network. `OPM_PORTAL_BROWSER_SHOTS=<directory>` saves a screenshot of each failing browser there.

A Go client ignores SameSite, so only this tier catches a launch whose cookie a browser withholds. Run `task test:browser` after a change to the code in the right column:

| Area | Checks | Run after a change to |
| --- | --- | --- |
| Launch | A local session starts from the `--open` page (file://) and from the printed link | `internal/auth` or the launch code of `cmd/opm-portal/` |
| Logs | A logs region refreshed under a tailed, focused pane keeps scroll offset, focus and tail | The page script's refresh or log code |
| Expired stream | A stream ended with the `expired` event says so and does not reconnect | The stream or page script |
| Theme and filters | Over the F1 capture, a stored Dark theme paints dark first; a stored Installed filter opens filtered on a full load and on a boosted navigation | `prefs.js`, the page script's filter code |
| Graph | Instance graph: Clear selection, the Resources hand-off, a group's fit and Whole graph, and the focus, zoom and full screen a live refresh keeps | The page script's graph code |
| Owner page | The details panel of an owner page and its Resources hand-off | The owner page or its details panel |
| Phone width | Every F1 page stays within 360 px wide | A template or CSS that changes page width |
| Info tip | The info tip: dismiss, hover, tap, tab away | The info tip code |
| Tab count | A tab count follows live changes | The tab count code |
| Filter forms | Filter forms apply as you type and announce the count | The filter forms |
| Loading state | A slow request shows a loading state; relative times keep counting | The loading or time code |
| Lost stream | The banner of a lost stream shows, and the page resyncs | Stream or banner code |

The browser tests in `cmd/opm-portal/` and the scripts in `test/browser/` are the source of the exact steps; this table is a summary.

## Pitfalls

- Treating a green `go test` as proof that a launch works in a browser. Only `test:browser` sees SameSite.
- Adding `hx-on:...` or an inline handler "just for one button". The CSP blocks it and the page breaks only in a browser, so a Go test can pass.
- Upgrading htmx by replacing the file only. The checksum test fails.
- Reading `internal/readmodel` from a handler in `internal/ui` because it is faster. Add the field to the read API instead, additively.
- Rendering a cluster string with `text/template` or `template.HTML`.

## Check before you finish

- [ ] No inline script, no inline style, no `hx-on`, no trigger filter in any template.
- [ ] Every cluster string reaches the page through `html/template`.
- [ ] `go test ./internal/ui` passes.
- [ ] You ran `task test:browser` for any change in the browser table above.
- [ ] A vendored asset upgrade changed the file, `CHECKSUMS` and the layout in one diff.
