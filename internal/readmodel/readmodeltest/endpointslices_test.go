package readmodeltest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const f1Capture = "../../../testdata/clusters/f1"

func TestWithEndpointSlicesPointsAtTheF1Service(t *testing.T) {
	f1 := LoadCapture(t, f1Capture)
	svc := findService(t, f1, PodinfoServiceNamespace, PodinfoServiceName)
	before := len(f1)

	got := WithEndpointSlices(t, f1)

	if len(f1) != before {
		t.Fatalf("the input grew from %d to %d objects", before, len(f1))
	}
	slices := make([]*unstructured.Unstructured, 0, len(got)-before)
	for _, o := range got[before:] {
		if o.GetKind() != "EndpointSlice" {
			t.Fatalf("an added object is a %s", o.GetKind())
		}
		slices = append(slices, o)
	}
	if len(slices) == 0 || len(got) != before+len(slices) {
		t.Fatalf("got %d objects for %d in F1 and %d slices", len(got), before, len(slices))
	}
	for _, sl := range slices {
		checkOwnedBy(t, sl, svc)
	}
	for _, o := range f1 {
		if o.GetKind() == "EndpointSlice" {
			t.Fatal("F1 holds a slice, so WithEndpointSlices would add a second copy")
		}
	}
}

// checkOwnedBy fails unless sl is a slice of the Service svc that svc owns as controller.
func checkOwnedBy(t *testing.T, sl, svc *unstructured.Unstructured) {
	t.Helper()
	if sl.GetNamespace() != PodinfoServiceNamespace || sl.GetLabels()[ServiceNameLabel] != PodinfoServiceName {
		t.Errorf("slice %s/%s has label %q", sl.GetNamespace(), sl.GetName(), sl.GetLabels()[ServiceNameLabel])
	}
	refs := sl.GetOwnerReferences()
	if len(refs) != 1 || refs[0].Kind != "Service" || refs[0].Controller == nil || !*refs[0].Controller {
		t.Fatalf("owner references = %+v, want one controller Service", refs)
	}
	if refs[0].UID != svc.GetUID() || refs[0].Name != svc.GetName() {
		t.Errorf("owner = %s %s, want F1's Service %s %s", refs[0].Name, refs[0].UID, svc.GetName(), svc.GetUID())
	}
}

func TestLoadEndpointSlicesRefuses(t *testing.T) {
	_, here := filepath.Split(endpointSlicesFile)
	good, err := os.ReadFile(filepath.Join("..", "testdata", here))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(item map[string]any, items *[]any)
		want   string
	}{
		{"a Secret", func(_ map[string]any, items *[]any) {
			*items = append(*items, map[string]any{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]any{"name": "s", "namespace": "default"}})
		}, "Secret may not be recorded"},
		{"another kind", func(_ map[string]any, items *[]any) {
			*items = append(*items, map[string]any{"apiVersion": "v1", "kind": "Pod", "metadata": map[string]any{"name": "p"}})
		}, "only EndpointSlice"},
		{"managedFields", func(item map[string]any, _ *[]any) {
			item["metadata"].(map[string]any)["managedFields"] = []any{map[string]any{"manager": "kube-controller-manager", "operation": "Update"}}
		}, "managedFields"},
		{"the last-applied annotation", func(item map[string]any, _ *[]any) {
			m := item["metadata"].(map[string]any)
			m["annotations"] = map[string]any{lastAppliedAnnotation: "{}"}
		}, lastAppliedAnnotation},
		{"no owner reference", func(item map[string]any, _ *[]any) {
			delete(item["metadata"].(map[string]any), "ownerReferences")
		}, "no controller owner reference"},
		{"no service-name label", func(item map[string]any, _ *[]any) {
			delete(item["metadata"].(map[string]any)["labels"].(map[string]any), ServiceNameLabel)
		}, "label is missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var list map[string]any
			if err := yaml.Unmarshal(good, &list); err != nil {
				t.Fatal(err)
			}
			items := list["items"].([]any)
			tt.mutate(items[0].(map[string]any), &items)
			list["items"] = items
			raw, err := yaml.Marshal(list)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "slices.yaml")
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err = loadEndpointSlices(path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("loadEndpointSlices = %v, want an error containing %q", err, tt.want)
			}
		})
	}
	t.Run("the recorded file loads", func(t *testing.T) {
		if _, err := loadEndpointSlices(filepath.Join("..", "testdata", here)); err != nil {
			t.Fatal(err)
		}
	})
}

func TestWithSentinelsReachesEveryField(t *testing.T) {
	s := Sentinels{Address: "SENT-ADDR", Hostname: "SENT-HOST", NodeName: "SENT-NODE", Zone: "SENT-ZONE", PodName: "SENT-POD", PortName: "SENT-PORT", LabelValue: "SENT-LABEL"}
	got := WithEndpointSlices(t, LoadCapture(t, f1Capture), WithSentinels(s))

	var found int
	addrs := map[string]bool{}
	for _, o := range got {
		if o.GetKind() != "EndpointSlice" {
			continue
		}
		found++
		raw, err := yaml.Marshal(o.Object)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		for _, want := range s.All() {
			if !strings.Contains(text, want) {
				t.Errorf("slice %s holds no %q", o.GetName(), want)
			}
		}
		for k, v := range o.GetLabels() {
			if k != ServiceNameLabel && v != s.LabelValue {
				t.Errorf("label %s = %q, want the sentinel", k, v)
			}
		}
		if o.GetLabels()[ServiceNameLabel] != PodinfoServiceName {
			t.Errorf("the service-name label became %q", o.GetLabels()[ServiceNameLabel])
		}
		eps, _, _ := unstructured.NestedSlice(o.Object, "endpoints")
		for _, e := range eps {
			a := e.(map[string]any)["addresses"].([]any)[0].(string)
			if addrs[a] {
				t.Errorf("address %q is listed twice", a)
			}
			addrs[a] = true
		}
	}
	if found == 0 {
		t.Fatal("no slice was added")
	}
}

func TestWithNoReadyEndpoints(t *testing.T) {
	got := WithEndpointSlices(t, LoadCapture(t, f1Capture), WithNoReadyEndpoints())
	var seen int
	for _, o := range got {
		if o.GetKind() != "EndpointSlice" {
			continue
		}
		eps, _, _ := unstructured.NestedSlice(o.Object, "endpoints")
		for _, e := range eps {
			seen++
			ready, ok, _ := unstructured.NestedBool(e.(map[string]any), "conditions", "ready")
			if !ok || ready {
				t.Errorf("an endpoint of %s has ready = %v (set %v), want false", o.GetName(), ready, ok)
			}
		}
	}
	if seen == 0 {
		t.Fatal("the slice holds no endpoint")
	}
}

func TestRecordedSliceIsReadyAsCaptured(t *testing.T) {
	got := WithEndpointSlices(t, LoadCapture(t, f1Capture))
	var ready int
	for _, o := range got {
		if o.GetKind() != "EndpointSlice" {
			continue
		}
		eps, _, _ := unstructured.NestedSlice(o.Object, "endpoints")
		for _, e := range eps {
			if r, ok, _ := unstructured.NestedBool(e.(map[string]any), "conditions", "ready"); ok && r {
				ready++
			}
		}
	}
	if ready != 2 {
		t.Fatalf("the recorded slice holds %d ready endpoints, want 2 (podinfo has two Pods)", ready)
	}
}

func TestWithServicePackageNamesTheF1Service(t *testing.T) {
	f1 := LoadCapture(t, f1Capture)
	before := len(f1)
	got := WithServicePackage(t, f1)
	if len(got) != before+1 || len(f1) != before {
		t.Fatalf("objects = %d (input %d), want one more than %d and the input unchanged", len(got), len(f1), before)
	}
	pkg := Find(t, got, "ModulePackage", ServicePackageName)
	entries, _, err := unstructured.NestedSlice(pkg.Object, "status", "inventory", "entries")
	if err != nil || len(entries) != 1 {
		t.Fatalf("inventory entries = %v (%v), want the one Service", entries, err)
	}
	e := entries[0].(map[string]any)
	if e["kind"] != "Service" || e["name"] != PodinfoServiceName || e["namespace"] != PodinfoServiceNamespace {
		t.Errorf("entry = %v, want Service %s/%s", e, PodinfoServiceNamespace, PodinfoServiceName)
	}
	if pkg.GetNamespace() != ServicePackageNamespace {
		t.Errorf("namespace = %q, want %q", pkg.GetNamespace(), ServicePackageNamespace)
	}
}
