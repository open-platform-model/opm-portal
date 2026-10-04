package api

import (
	"maps"
	"os"
	"regexp"
	"slices"
	"testing"
)

const readAPIReferencePath = "../../docs/site/reference/portal/read-api.md"

// referenceRow matches a resource row of the reference page's table:
// | `GET /clusters/{cluster}/instances` | ... |
var referenceRow = regexp.MustCompile("(?m)^\\| `GET (/[^`]*)` \\|")

// TestReadAPIReferenceListsEveryPath: the published read API reference names
// exactly the OpenAPI document's paths, so a route added to or removed from
// the contract fails here until the page follows.
func TestReadAPIReferenceListsEveryPath(t *testing.T) {
	doc := loadOpenAPI(t)
	raw, err := os.ReadFile(readAPIReferencePath)
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, m := range referenceRow.FindAllStringSubmatch(string(raw), -1) {
		if listed[m[1]] {
			t.Errorf("%s lists %s twice", readAPIReferencePath, m[1])
		}
		listed[m[1]] = true
	}
	if len(listed) == 0 {
		t.Fatalf("%s has no resource rows (| `GET <path>` | ...)", readAPIReferencePath)
	}
	for _, p := range slices.Sorted(maps.Keys(doc.Paths)) {
		if !listed[p] {
			t.Errorf("%s does not list %s, which openapi/v1alpha1.yaml has", readAPIReferencePath, p)
		}
	}
	for _, p := range slices.Sorted(maps.Keys(listed)) {
		if _, ok := doc.Paths[p]; !ok {
			t.Errorf("%s lists %s, which openapi/v1alpha1.yaml does not have", readAPIReferencePath, p)
		}
	}
}
