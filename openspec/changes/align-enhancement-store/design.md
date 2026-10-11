## Context

The `enh cite-rewrite` tool (enhancements `p5-rewrite`, built at `fdb2ec9e`) rewrites citations by form. For this repo it removed the withdrawn entry's token from 162 places and deleted 14 `enhancement.yaml` files. It listed 188 sites for hand work.

## Decisions

### A citation that a sentence needs becomes `portal:Dn`

A sentence such as "the old head D7 asks that no read path can skip authorization" uses the citation as a noun. Deleting it breaks the sentence. `docs/DESIGN.md` says the old numbers and the `portal:` numbers are the same, so `portal:D7` keeps both the reason and a working reference. The alternative, deleting every citation, loses the link to the design record. A `Source:` list and a trailing parenthesis carry no sentence, so they are dropped, as the tool does.

### A bare id of the withdrawn entry becomes words

The two ids of the withdrawn entries are free again after the migration, so a bare id would later name another entry. The text now says "the withdrawn portal enhancement".

### Anchors of headings

Three archived headings lost their citation (`### Locked rendering`). No link points at them (checked with `git grep`).

## Risks

A wrong edit in archived text changes history. The review samples the diff and checks that no code line changed.
