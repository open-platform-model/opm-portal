# Proposal

## Why

Agents and the owner use several words for one portal concept: "Ready" and "healthy", "provider"
and "owner", "uses" and "requires". The design record defines the words, but no short file lists
them. A root `GLOSSARY.md` gives each concept one word and names the loose words to avoid.

## What Changes

- Add `GLOSSARY.md` at the repository root with the portal's domain terms, each with a definition
  and an `_Avoid_` line, backed by `docs/DESIGN.md`, `api/v1alpha1/` and the `internal/` packages.
- Add one line to `AGENTS.md` and one row to `CONTEXT.md` that route to the glossary.

## Capabilities

### New Capabilities

None. This change adds documentation only and changes no requirement.

### Modified Capabilities

None.

## Enhancement

None.

## Impact

Documentation only. No code, no API, no release: `docs` commits do not release in this repository. After 1.0 this would be PATCH.
