package health

import (
	"os"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// loadCapture decodes a testdata List the way client-go decodes objects
// (integers as int64), which kstatus requires.
func loadCapture(t *testing.T, name string) []*unstructured.Unstructured {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	js, err := yaml.YAMLToJSON(raw)
	if err != nil {
		t.Fatalf("converting %s to JSON: %v", name, err)
	}
	var list unstructured.UnstructuredList
	if err := list.UnmarshalJSON(js); err != nil {
		t.Fatalf("decoding %s: %v", name, err)
	}
	out := make([]*unstructured.Unstructured, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, &list.Items[i])
	}
	return out
}

// find returns the captured object with the given kind and name.
func find(t *testing.T, objs []*unstructured.Unstructured, kind, name string) *unstructured.Unstructured {
	t.Helper()
	for _, o := range objs {
		if o.GetKind() == kind && o.GetName() == name {
			return o
		}
	}
	t.Fatalf("no %s %s in capture", kind, name)
	return nil
}

// TestCaptureHoldsNoSecrets guards the testdata: no Secret objects, no
// instance values and no last-applied annotation may be committed.
func TestCaptureHoldsNoSecrets(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		for _, o := range loadCapture(t, e.Name()) {
			if o.GetKind() == "Secret" {
				t.Errorf("%s holds Secret %s", e.Name(), o.GetName())
			}
			if len(o.GetAnnotations()) > 0 {
				t.Errorf("%s: %s %s keeps annotations", e.Name(), o.GetKind(), o.GetName())
			}
			if _, found, _ := unstructured.NestedFieldNoCopy(o.Object, "spec", "values"); found {
				t.Errorf("%s: %s %s keeps spec.values", e.Name(), o.GetKind(), o.GetName())
			}
		}
	}
}
