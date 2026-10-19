# 05: canvas vs live portal gap report

This is a snapshot from 2026-10-06. It compares the owner's design canvas (`../03-ui-canvas/`) with the live portal at main `ad8b218`. That commit is the merged redesign, PR 39.

## How it was made

- Six agents compared one board group each: Sonnet, reading the canvas board source together with 1920 px screenshots of the live portal in light and dark (F1 and F1-broken, served by `TestDevServe`).
- Six Opus-high verifiers then rejected false gaps, reclassified the rest and added missed ones.
- The result is 174 verified gaps:
  - 52 visual
  - 27 behaviour
  - 23 layout
  - 22 copy
  - 20 missing UI
  - 23 already ruled out
  - 5 live-only
  - 2 needing the controller or the registry
- `gaps.md` groups them by the change that addresses them: A shell and tokens, B Platform/Installed/Catalog, C owner pages, D graph, X not built.
- `gaps.json` holds the raw verified records.

## Owner decisions from this comparison (2026-10-06)

- Follow the canvas's flat look, 1840 px column.
- Group graph objects per kind, amending portal:D4:R5.
- Merge every reached object's events in the Events tab.
- Read the package's source object to show its state.
