## 1. Conditions from the read API (internal/health, api/v1alpha1, internal/api, internal/ui)

- [x] 1.1 `internal/health`: `ConditionTone` with its table and test
- [x] 1.2 `api/v1alpha1` and `internal/api`: `Condition.tone`, `meaning`, `nextStep`; OpenAPI and enum contract; API goldens
- [x] 1.3 `internal/ui`: render meaning, next step and the stripe from the condition; drop the `internal/health` import; the import test allows only `api/v1alpha1` among the module's packages
- [x] 1.4 `task check` green, then commit `feat(api): serve a condition's tone, meaning and next step`

## 2. Live refresh from one fetch (internal/ui/static)

- [x] 2.1 `portal.js`: per-topic debounce, one page fetch, swap every following region from it, keep open groups, selection and zoom
- [x] 2.2 `task check` green, then commit `fix(ui): refresh every followed region from one fetch`

## 3. Visual fixes (internal/ui)

- [x] 3.1 Graph: health on the outline, applied stamp, selection mark, fit on load, middle-ellipsis labels with titles, locked edges, edge token, toolbar wrap
- [x] 3.2 Colour: accent out of the red family; condition stripes by tone; partial health marked
- [x] 3.3 Pages: configuration components grouped, zero-replica ReplicaSets folded, refs break at separators, compact opaque masthead on a phone, log disclosure marks and Pod log links, panel scroll at any width, repeated messages dropped, kicker and stat labels
- [x] 3.4 UI goldens updated and reviewed
- [x] 3.5 `task check` green, then commit `fix(ui): keep the two axes apart and fix the review's visual findings`

## 4. Escaping held by tests (internal/ui)

- [x] 4.1 Render every F1 page and fragment over a capture whose free text is hostile and assert no raw markup
- [x] 4.2 Source check: no trusted-type conversion in `internal/ui`, no markup assignment in `portal.js`
- [x] 4.3 `task check` green, then commit `test(ui): fail when a page stops escaping cluster text`

## 5. Live evidence and docs (test, docs/site)

- [x] 5.1 Throwaway cluster `opm-portal-e2e-ui2`: re-run the scripted image break on the open page, record when each region shows it, re-take every screenshot light, dark and at phone width, delete the cluster
- [x] 5.2 Record the evidence in design.md; update the portal pages under `docs/site/` where they describe what changed
- [x] 5.3 `task check` green, then commit `docs(ui): record the image break on the open page`
