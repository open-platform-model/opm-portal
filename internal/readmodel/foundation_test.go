package readmodel

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/health"
)

func TestTuneConfig(t *testing.T) {
	cfg := &rest.Config{}
	TuneConfig(cfg)
	if cfg.QPS != tunedQPS || cfg.Burst != tunedBurst {
		t.Fatalf("tuned to QPS %v burst %d, want %d and %d", cfg.QPS, cfg.Burst, tunedQPS, tunedBurst)
	}
	explicit := &rest.Config{QPS: 7, Burst: 9}
	TuneConfig(explicit)
	if explicit.QPS != 7 || explicit.Burst != 9 {
		t.Fatalf("explicit QPS and burst changed to %v and %d", explicit.QPS, explicit.Burst)
	}
	TuneConfig(nil)
}

func TestNewRefusesAnIncompleteConfig(t *testing.T) {
	checker, _ := newChecker(t, alice, allowAll, authz.Options{})
	full := Config{Dynamic: newDynamic(), Discovery: newDiscovery(), Authorizer: checker, Reader: alice}
	if _, err := New(full); err != nil {
		t.Fatalf("New(full config) = %v", err)
	}
	cases := map[string]func(c *Config){
		"no dynamic client": func(c *Config) { c.Dynamic = nil },
		"no discovery":      func(c *Config) { c.Discovery = nil },
		"no authorizer":     func(c *Config) { c.Authorizer = nil },
		"empty reader":      func(c *Config) { c.Reader = authz.Identity{} },
		"anonymous reader":  func(c *Config) { c.Reader = authz.Identity{Username: "system:anonymous"} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := full
			mutate(&c)
			if _, err := New(c); err == nil {
				t.Fatal("New accepted the config")
			}
		})
	}
}

// TestStripKeepsHealth runs health on every captured object before and
// after stripping: the dropped fields must never change an answer.
func TestStripKeepsHealth(t *testing.T) {
	objs := loadF1(t)
	objs = append(objs, loadList(t, applyFailedSample)...)
	for _, o := range objs {
		stripped := o.DeepCopy()
		strip(stripped)
		before, after := health.Object(o), health.Object(stripped)
		if before != after {
			t.Errorf("%s %s/%s: health %+v before stripping, %+v after", o.GetKind(), o.GetNamespace(), o.GetName(), before, after)
		}
	}
}

func TestStripRemovesValuesAndTheApplyAnnotation(t *testing.T) {
	const secretValue = "hunter2-do-not-leak"
	for _, kind := range []string{"ModuleInstance", "ModulePackage"} {
		t.Run(kind, func(t *testing.T) {
			u := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "opmodel.dev/v1alpha1",
				"kind":       kind,
				"metadata": map[string]any{
					"name": "x", "namespace": "y",
					"managedFields": []any{map[string]any{"manager": "kubectl", "fieldsV1": map[string]any{"f:spec": map[string]any{}}}},
					"annotations": map[string]any{
						lastAppliedAnnotation: `{"spec":{"values":{"password":"` + secretValue + `"}}}`,
						"keep":                "me",
					},
				},
				"spec": map[string]any{"values": map[string]any{"password": secretValue}, "module": map[string]any{"path": "p"}},
			}}
			strip(u)
			js, err := json.Marshal(u.Object)
			if err != nil {
				t.Fatal(err)
			}
			for _, leak := range []string{secretValue, "managedFields", lastAppliedAnnotation, `"values"`} {
				if strings.Contains(string(js), leak) {
					t.Errorf("stripped object still holds %q: %s", leak, js)
				}
			}
			if u.GetAnnotations()["keep"] != "me" {
				t.Errorf("other annotations lost: %v", u.GetAnnotations())
			}
			if path, _, _ := unstructured.NestedString(u.Object, "spec", "module", "path"); path != "p" {
				t.Errorf("spec.module lost")
			}
		})
	}
}

func TestStripKeepsValuesOfOtherKinds(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1", "kind": "Thing",
		"metadata": map[string]any{"name": "x"},
		"spec":     map[string]any{"values": "kept"},
	}}
	strip(u)
	if v, _, _ := unstructured.NestedString(u.Object, "spec", "values"); v != "kept" {
		t.Fatalf("spec.values of a non-OPM kind removed")
	}
}

func TestStripDropsCRDSchemasAndConfigMapData(t *testing.T) {
	crd := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1", "kind": "CustomResourceDefinition",
		"metadata": map[string]any{"name": "things.example.com"},
		"spec": map[string]any{"versions": []any{
			map[string]any{"name": "v1", "served": true, "schema": map[string]any{"openAPIV3Schema": map[string]any{"type": "object"}}},
		}},
	}}
	strip(crd)
	versions, _, _ := unstructured.NestedSlice(crd.Object, "spec", "versions")
	if len(versions) != 1 || versions[0].(map[string]any)["schema"] != nil || versions[0].(map[string]any)["name"] != "v1" {
		t.Errorf("CRD versions after strip = %v", versions)
	}
	cm := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]any{"name": "c", "namespace": "n"},
		"data":     map[string]any{"k": "v"}, "binaryData": map[string]any{"b": "AA=="},
	}}
	strip(cm)
	if cm.Object["data"] != nil || cm.Object["binaryData"] != nil {
		t.Errorf("ConfigMap data kept: %v", cm.Object)
	}
}

func TestStripTransformPassesNonObjectsThrough(t *testing.T) {
	tomb := cache.DeletedFinalStateUnknown{Key: "a/b"}
	got, err := stripTransform(tomb)
	if err != nil || got != tomb {
		t.Fatalf("stripTransform(tombstone) = %v, %v", got, err)
	}
}

func TestKindResolverResolvesAndRefreshes(t *testing.T) {
	deployments := clusterKinds[6]
	widgets := testKind{schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Widget"}, "widgets", false}
	d := newDiscovery(deployments)
	k := newKindResolver(d)

	got, err := k.resolve(deployments.gvk)
	if err != nil || got.Resource != deployments.gvr() || !got.Namespaced {
		t.Fatalf("resolve(Deployment) = %+v, %v", got, err)
	}
	if _, err := k.resolve(widgets.gvk); err == nil {
		t.Fatal("resolved a kind discovery does not know")
	}
	// A CRD installed later resolves after one refresh.
	setDiscovery(d, deployments, widgets)
	got, err = k.resolve(widgets.gvk)
	if err != nil || got.Resource != widgets.gvr() || got.Namespaced {
		t.Fatalf("resolve(Widget) after install = %+v, %v", got, err)
	}
}

// TestFakeInformerAssumptions pins what the read model's tests rely on: the
// fake dynamic client applies label selectors on list, and an informer's
// transform runs before the object is stored.
func TestFakeInformerAssumptions(t *testing.T) {
	objs := loadF1(t)
	podinfo := find(t, objs, "Deployment", "default", "podinfo-podinfo")
	unlabeled := podinfo.DeepCopy()
	unlabeled.SetName("unlabeled")
	unlabeled.SetLabels(nil)
	withAnnotation := podinfo.DeepCopy()
	withAnnotation.SetAnnotations(map[string]string{lastAppliedAnnotation: "{}"})

	client := newDynamic(withAnnotation, unlabeled)
	gvr := clusterKinds[6].gvr()
	inf := dynamicinformer.NewFilteredDynamicInformer(client, gvr, "", 0, cache.Indexers{},
		func(o *metav1.ListOptions) { o.LabelSelector = "module-instance.opmodel.dev/uuid" })
	if err := inf.Informer().SetTransform(stripTransform); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	defer close(stop)
	go inf.Informer().Run(stop)
	if !cache.WaitForCacheSync(stop, inf.Informer().HasSynced) {
		t.Fatal("informer did not sync")
	}
	time.Sleep(10 * time.Millisecond)
	items := inf.Informer().GetStore().List()
	if len(items) != 1 {
		t.Fatalf("store holds %d objects, want only the labeled one", len(items))
	}
	if ann := items[0].(*unstructured.Unstructured).GetAnnotations(); ann[lastAppliedAnnotation] != "" {
		t.Fatalf("stored object keeps the last-applied annotation: %v", ann)
	}
}
