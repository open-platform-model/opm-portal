// The opm-portal docs bundle, built by docs-kit's opm-docs (docs-kit
// docs/contracts.md C6, C15) and published by docs.yml and release.yml to
// ghcr.io/open-platform-model/docs/opm-portal.
//
// It holds the authored pages under docs/site/ only. They sit in two sections
// the bundle owns, operating/portal/ and reference/portal/, so no other
// bundle can place a page there and this one touches no shared section index.
// The read API reference is authored: internal/api's
// TestReadAPIReferenceListsEveryPath holds it to openapi/v1alpha1.yaml.
bundles: "opm-portal": {
	placement: {kind: "docs", root: "/docs/", owns: ["operating/portal/", "reference/portal/"]}
	version: {from: "tag", prefix: "v"}
	sources: [{kind: "markdown", dir: "docs/site"}]
}
