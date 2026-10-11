## Why

The enhancements now live in one OpenSpec store, `opm-enhancements`, and a citation of one is an id beside a reason in words, never a decision number and never a lone id. The portal still carries 385 citations of its withdrawn enhancement, 14 `enhancement.yaml` files and rules that point at a delivery log that no longer exists.

## What Changes

- The old enhancement head goes from archived changes, `docs/DESIGN.md`, `ROADMAP.md` and the evidence note. Where a sentence needs the citation it reads `portal:Dn`, because the two numberings are the same (`docs/DESIGN.md`, "Numbering"). A trailing `Source:` list is dropped. A bare id of the withdrawn entry reads "the withdrawn portal enhancement".
- The 14 archived `enhancement.yaml` files are deleted.
- Citations of other enhancements (0013, 0015, 0021, 0027) become the id beside the decision title or a reason in words.
- `openspec/config.yaml` gains `references:` for `opm-enhancements`, a context line to read an entry, the `Delivers` proposal rule and the comment rule. The `enhancement.yaml` and delivery log rules go. `AGENTS.md` says the same.
- The `portal:Dn` head is unchanged.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
None. The live specs hold no enhancement citation, so no spec text changes. The change has no spec delta, and `.openspec.yaml` declares `skip_specs: true`.

## Impact

Comments, rule text and archived text only. No code, API resource, UI page or test changes, and no effect on Principle V. No release: the commits are `docs`. SemVer after 1.0: PATCH.

## Enhancement

None.
