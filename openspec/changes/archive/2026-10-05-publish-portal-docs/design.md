## Context

docs-kit (`open-platform-model/docs-kit`, release `v0.7.0`) builds each repository's site pages
into a signed OCI bundle in that repository's CI; opmodel.dev pulls bundles and builds the site
from them. The operator and the cli adopted it with the same set of files: `.opm-docs-version`,
`docs-kit.cue`, `.tasks/opm-docs.sh`, the `docs:*` tasks, `docs.yml`, a `publish-docs` job in
`release.yml`, and a pin check in the `Lint` job. The portal has none of them and no `docs/site/`.

The portal has no release yet (release PR `0.1.0` is open), and `opm-portal serve` is being added
in parallel by `add-local-mode`. The pages must describe local mode honestly before either lands.

## Goals / Non-Goals

**Goals:**

- The portal's pages build, lint and publish exactly as the other adopters' do.
- Four pages, one per type the task needs: what the portal is, how to run it locally, the read
  API reference, and the security model.
- The reference cannot drift from `openapi/v1alpha1.yaml` without a failing test.

**Non-Goals:**

- The site change that pulls the bundle (`opmodel.dev`); a site version names it separately.
- A docs-kit extractor for OpenAPI. The reference is authored and held to the document by a test.
- Pages for in-cluster mode, the web UI or the change stream's log topics, which are not built
  or not reachable yet.

## Decisions

### Placement in the docs tree

**Context**: a docs bundle merges into a site version's `/docs/` tree, whose top-level sections
(`start`, `concepts`, `authoring`, `operating`, `extending`, `embedding`, `reference`,
`diagnostics`) and their `_index.md` pages belong to opm and the site. Concepts pages live in
core by rule (`STYLE.md`, "Site Pages").
**Options considered**:

1. A new top-level `portal/` section. Self-contained, but it adds a sidebar section the site
   does not have, which is the site's decision, and the task rules out a new tab.
2. Pages spread over `start/`, `operating/` and `reference/` beside the other bundles' pages, as
   the operator does. No new section, but four portal pages scattered over three sections, and
   no section index of its own to group them.
3. `operating/portal/` for the explanations and the how-to, and `reference/portal/` for the
   reference, each with its own `_index.md`, both owned by the bundle.

**Decision**: option 3. The reader of every portal page holds a cluster, which is what
"Deploying and operating" is for, and the reference goes where every reference goes.
**Rationale**: owned paths make docs-kit refuse another bundle's page under them (C15, C16), and
the two section indexes are the portal's own, so no shared `_index.md` is touched. The concept
page is an explanation in `operating/portal/`, not a Concepts page, because it explains a tool,
not an OPM schema concept.

### The read API reference is authored and tested

**Context**: `STYLE.md` says every fact derivable from CUE, cobra or a CRD is generated.
OpenAPI is none of those, and docs-kit has no OpenAPI source kind.
**Options considered**:

1. Link only: one paragraph and a link to `openapi/v1alpha1.yaml`. Cannot drift, but a reader
   learns nothing on the site.
2. A docs-kit `openapi` extractor. The right end state, but a docs-kit change of its own.
3. An authored table of resources and problem codes, plus the link, with a test in
   `internal/api` (where the OpenAPI contract tests already live) that compares the table's paths
   with the document's.

**Decision**: option 3.
**Rationale**: the page is useful today, and the test turns the transcription risk into a
failing check. The test reads the page's table rows with a fixed pattern (`` | `GET <path>` | ``)
and the document's `paths` keys, and reports both differences.

```go
// TestReadAPIReferenceListsEveryPath: the reference page's resource table
// names exactly the OpenAPI document's paths.
func TestReadAPIReferenceListsEveryPath(t *testing.T)
```

### Writing against a command `main` does not have yet

**Context**: the how-to must use `opm-portal serve --kubeconfig <file> --context <name> --open`,
which `add-local-mode` adds. A release's docs bundle is built from its tag and never changes, so
an alert about releases would stay in the bundle of the release that makes it false.
**Decision**: this change merges after `add-local-mode`, so every page describes what `main`
has. The pages are written against that change's `serve.go` and `internal/auth`, and the read
API reference lists its `log:` topics and front-door refusals. The only alert names what is not
built: the web UI. In-cluster mode, and why it sends no self review, stay in `Direction` notes.
**Rationale**: `STYLE.md` lets pages state only what is true today and keeps future work in
direction notes. Wording the alert about the web UI keeps it true in any release's bundle until
the change that builds the UI removes it (spec: "Behavior that is not built is marked as not
built").

### Installing from a release

**Context**: a release's docs bundle never changes. A how-to that installs with
`go install ...@main` would install unreleased code, and one that names a version would break or
go stale as releases move on.
**Decision**: the how-to downloads the archive for the reader's system from the releases page
through `releases/latest/download/`, checks it against `checksums.txt` (the assets
`.goreleaser.yml` builds) and names no version. A source build is a one-line note pinned to
`latest`.
**Rationale**: the commands name no version or branch, so they never break and never install
unreleased code, and the reader gets a binary the release workflow built and checksummed. They
install the newest release, which can be newer than the pages of an older release's bundle.

### Pins and workflow

`.opm-docs-version` is `v0.7.0`, the newest docs-kit release and the operator's pin.
`publish.yml` is referenced by tag, not by SHA: the signing certificate names the workflow at
that ref and the site trusts only docs-kit's `v*` tags. `.tasks/opm-docs.sh` is copied unchanged
from the operator, so the adopters' copies stay identical. `docs.yml` mirrors the operator's,
with `project: opm-portal`; there is no backfill floor, because no portal release exists.
`publish-docs` in `release.yml` follows `binaries` and `image-release`, as the operator's follows
its image, and `publish-release` does not wait for it, so a docs failure never strands a draft.

Authorization: none. The change reads no Kubernetes resource.

## Risks / Trade-offs

- [This change merges before `add-local-mode`] -> the how-to and the security page describe a
  command `main` lacks, and the next `edge` push publishes them; the PR says to merge it after.
- [The `serve` flags change after merge] -> the how-to is wrong until fixed; the change that
  moves a flag updates the page.
- [The GHCR package starts private] -> the site cannot pull it; making it public is an owner step
  named in the PR.
- [Edge publishes on every push to `main`] -> each push writes one bundle; docs-kit's `edge` tag
  moves, older edge builds stay by digest only, as for every adopter.
