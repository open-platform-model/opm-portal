## Why

The web-ui spec says pages must not scroll sideways at a 360 px wide viewport, and no test checks it:
`cmd/opm-portal/browser_test.go` sets no viewport. Two sets of colours also fall below WCAG 2.2 AA on
`main` today. Light-theme `--muted` text reads 4.17:1 on the page background and 4.39:1 on
`--surface-2` (4.5:1 needed). The border of an input or select, `--line-strong`, reads 2.18:1 on
`--field` in the light theme and 1.71:1 in the dark one (3:1 needed). Both figures come from the
WCAG formula over the token hex values (design.md, Evidence). A phone user and a low-vision user
meet these defects on every page, and the next UI changes would build on them. The owner's ruling
of 2026-10-10 puts the contrast fix first, and ruling 4 of 2026-10-11 puts the AA floor and the
360 px rule above any canvas value.

## What Changes

- A browser test, `TestBrowserPhone`, opens every page of the F1 capture, the not-found page included, at a 360 px wide viewport in
  Chromium, Firefox and WebKit and fails when a page has a scroll width above 360 px. The test proves
  it can fail by detecting a 500 px element it injects. Its name joins the `-run` pattern of
  `test:browser` in `Taskfile.yml`, which the nightly `E2E` workflow also runs.
- Light `--muted` goes from `#6a6f7a` to `#5f6672`. A new `--control-border` token (`#8c8269` light,
  `#6a7587` dark) draws the border of the filter inputs and selects. Both candidates are
  re-measured in a browser before they ship. No other token changes.
- A pure Go test reads the hex values from `portal.css` and holds a list of token pairs to 4.5:1
  (text) or 3:1 (control boundaries) in both themes. Later changes extend the list. Two light-theme
  pairs that fail today and need new tokens (`--healthy` and `--degraded` on their fills) sit in a
  pending list that the test reports and that the change `align-shell-and-tokens` closes.
- Decided here (both asked by the brief):
  1. No page fails the phone test today, so the test carries no named exception. The measurement is
     in design.md, Evidence. If the implementation run disagrees, it stops and reports.
  2. The contrast floor is a requirement of the web-ui spec delta in this change, and the change
     records it in `docs/DESIGN.md` as `portal:D19:R6`. PR 40 merged on 2026-10-11 and put
     `portal:D19` on `main`; the owner answered yes (design/T3.4 NQ8, 2026-10-11) to recording the
     floor as D19's next free requirement. The change edits `docs/DESIGN.md` for that requirement
     only, in section 2 (task 2.5).
- Not changed: the read API and its goldens, page layout, copy, templates and components, any other
  token, the strict CSP (no inline style, no `style` attribute), the JS build (none), dependencies
  (none).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-ui`: two ADDED requirements. A test holds the 360 px width of every F1 page in three
  browsers. Muted text and the boundary of text inputs and selects meet WCAG 2.2 AA in both themes,
  held by a token-pair test.

## Impact

- Files: `internal/ui/static/portal.css` (one token changed, one added in three places, one border
  rule), `internal/ui/contrast_test.go` (new), `test/browser/phone.py` (new),
  `cmd/opm-portal/browser_test.go`, `Taskfile.yml` (line 90 and the task description), `AGENTS.md`
  (one sentence), `docs/DESIGN.md` (the D19 requirement R6 only), `ROADMAP.md`. No template, handler, API type or golden changes.
- Principle V: unchanged. No new read, verb or identity. The test serves the F1 capture in process.
- Security: no new trust boundary. No UI text is added, so the hostile-text suite is unaffected.
  The CSP is untouched; the phone script runs in the pinned Playwright image like the other browser
  tests.
- SemVer: PATCH after 1.0 (a fix to colour tokens); on the 0.x line it releases as a patch. Section
  1 commits as `test(ui)` (no release), section 2 as `fix(ui)`. The PR title is
  `fix(ui): pass WCAG 2.2 AA contrast and test the 360 px width`, the highest class of its commits.
- Complexity: one new Python script and one new Go test file, both in the shape of the existing
  browser tests and the existing `internal/ui` tests. No dependency. `task test:browser` needs the
  network and a container engine, as it does today.
- Shared files: `ROADMAP.md` and `Taskfile.yml:90` conflict with later changes; both resolve by
  adding a line (design/T3.4 section 3.2).
- Delivery: one PR for the change.
