## 1. The rewrite (archived changes, docs)

- [x] 1.1 Run `enh cite-rewrite --write` once and commit its result by place
- [x] 1.2 Clear the worklist of 188 sites by hand and fix the tool's wrong edits
- [x] 1.3 `task check` green, then commit `docs(openspec): clear the 0030 worklist of the archived changes`

## 2. Rules

- [x] 2.1 `openspec/config.yaml`: `references:`, the read-an-entry context line, the `Delivers` rule, the comment rule; remove the `enhancement.yaml` and delivery log rules
- [x] 2.2 `AGENTS.md`: the same rules in words
- [x] 2.3 `task check` green, then commit `docs(openspec): point to the enhancements store and set the citation rule`

## 3. Close

- [x] 3.1 `enh cite-lint` reports 0 findings and `enh cite-rewrite --dry-run` reports 0 edits at the final head
- [ ] 3.2 Review, fixes, archive, then commit `docs(openspec): archive align-enhancement-store`
