package readmodel

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TestObjectKeepsWhatHealthDrops: the YAML read keeps a Deployment's pod
// template and removes the last-applied annotation and managed fields.
func TestObjectKeepsWhatHealthDrops(t *testing.T) {
	objs := loadF1(t)
	dep := find(t, objs, "Deployment", "podinfo-podinfo")
	ann := dep.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}
	ann[lastAppliedAnnotation] = `{"spec":{"values":{"password":"x"}}}`
	dep.SetAnnotations(ann)
	dep.SetManagedFields(nil)
	_ = unstructured.SetNestedSlice(dep.Object, []any{map[string]any{"manager": "kubectl"}}, "metadata", "managedFields")

	e := newEnv(t, objs, allowAll, allowAll)
	kind := ResolvedKind{Resource: deployments, Namespaced: true}
	ref := ObjectRef{Group: "apps", Kind: "Deployment", Namespace: "default", Name: "podinfo-podinfo"}
	obj, err := e.m.Object(t.Context(), alice, e.grant(t, "get", deployments, "default", "podinfo-podinfo"), kind, ref)
	if err != nil {
		t.Fatalf("Object: %v", err)
	}
	u := &unstructured.Unstructured{Object: obj}
	if _, found, _ := unstructured.NestedMap(obj, "spec", "template"); !found {
		t.Error("the pod template was dropped")
	}
	if _, ok := u.GetAnnotations()[lastAppliedAnnotation]; ok {
		t.Error("the last-applied annotation was served")
	}
	if _, found, _ := unstructured.NestedFieldNoCopy(obj, "metadata", "managedFields"); found {
		t.Error("managed fields were served")
	}
}

// TestObjectRefusesSecretsAndUncoveredGrants: a Secret is refused before
// any review or read, and a grant for another object reads nothing.
func TestObjectRefusesSecretsAndUncoveredGrants(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	secrets := schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	before, readerBefore := e.reviews.Count(), e.readerR.Count()
	_, err := e.m.Object(t.Context(), alice, e.grant(t, "get", deployments, "default", "podinfo-podinfo"),
		ResolvedKind{Resource: secrets, Namespaced: true}, ObjectRef{Kind: "Secret", Namespace: "default", Name: "podinfo"})
	if !errors.Is(err, ErrWithheld) {
		t.Fatalf("Secret: %v, want ErrWithheld", err)
	}
	if e.reviews.Count() != before+1 || e.readerR.Count() != readerBefore {
		t.Error("a review was sent for the Secret")
	}
	_, err = e.m.Object(t.Context(), alice, e.grant(t, "get", deployments, "default", "other"),
		ResolvedKind{Resource: deployments, Namespaced: true}, ObjectRef{Group: "apps", Kind: "Deployment", Namespace: "default", Name: "podinfo-podinfo"})
	if !errors.Is(err, ErrNotCovered) {
		t.Fatalf("uncovered grant: %v, want ErrNotCovered", err)
	}
	for _, line := range clusterReads(e.client) {
		if strings.HasPrefix(line, "get deployments") || strings.Contains(line, "secrets") {
			t.Errorf("read made for a refused request: %s", line)
		}
	}
}

// TestPodsNameTheirContainers: each Pod child carries its containers.
func TestPodsNameTheirContainers(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	d := e.instance(t, "default", "podinfo")
	dep := objectNamed(t, &d, "Deployment", "podinfo-podinfo")
	for _, c := range dep.Children {
		want := []string(nil)
		if c.Ref.Kind == kindPod {
			want = []string{"podinfo"}
		}
		if !slices.Equal(c.Containers, want) {
			t.Errorf("%s %s containers = %v, want %v", c.Ref.Kind, c.Ref.Name, c.Containers, want)
		}
	}
}
