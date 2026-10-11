package readmodel

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

// TestSourceFixtures holds the constructed Flux fixtures to their shape: the
// three OCIRepositories carry the states they are named for and say they are
// constructed, F1's discovery does not serve their kind, and the
// package with no apiVersion reads with none.
func TestSourceFixtures(t *testing.T) {
	ociRepositories := schema.GroupVersionResource{Group: readmodeltest.FluxGroup, Version: "v1", Resource: "ocirepositories"}
	objs := readmodeltest.WithUnversionedSourcePackage(t, readmodeltest.WithSources(t, loadF1(t)))
	e := newEnv(t, objs, allowAll, allowAll)

	for _, tc := range []struct {
		name, ready, reason string
		artifact            bool
	}{
		{readmodeltest.SourceReady, "True", "Succeeded", true},
		{readmodeltest.SourceNotReady, "False", readmodeltest.SourceNotReadyReason, false},
		{readmodeltest.SourceNoArtifact, "True", "Succeeded", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, err := e.client.Resource(ociRepositories).Namespace(readmodeltest.SourceNamespace).Get(t.Context(), tc.name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			conds, _, _ := unstructured.NestedSlice(o.Object, "status", "conditions")
			c, _ := conds[0].(map[string]any)
			_, hasArtifact, _ := unstructured.NestedMap(o.Object, "status", "artifact")
			if c["status"] != tc.ready || c["reason"] != tc.reason || hasArtifact != tc.artifact {
				t.Errorf("Ready %v, reason %v, artifact %v; want %v, %v, %v", c["status"], c["reason"], hasArtifact, tc.ready, tc.reason, tc.artifact)
			}
			if o.GetAnnotations()[readmodeltest.ConstructedAnnotation] != "true" {
				t.Errorf("not annotated %s", readmodeltest.ConstructedAnnotation)
			}
		})
	}

	if _, err := e.m.ResolveKind(readmodeltest.FluxGroup, "OCIRepository"); err == nil {
		t.Error("F1's discovery serves OCIRepository")
	}
	served := newEnv(t, objs, allowAll, allowAll, func(c *Config) { c.Discovery = newDiscovery(readmodeltest.KindsWithFlux()...) })
	if k, err := served.m.ResolveKind(readmodeltest.FluxGroup, "OCIRepository"); err != nil || k.Resource != ociRepositories || !k.Namespaced {
		t.Errorf("KindsWithFlux: %+v, %v; want ocirepositories, namespaced", k, err)
	}

	g := e.grant(t, "get", modulePackages, "pkg", readmodeltest.UnversionedPackage)
	d, err := e.m.Package(t.Context(), alice, g, "pkg", readmodeltest.UnversionedPackage)
	if err != nil {
		t.Fatal(err)
	}
	if want := (SourceRef{Kind: "OCIRepository", Name: readmodeltest.SourceReady}); d.Source != want {
		t.Errorf("source = %+v, want %+v", d.Source, want)
	}
}
