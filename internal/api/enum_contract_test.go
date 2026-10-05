package api

import (
	"os"
	"slices"
	"testing"

	"sigs.k8s.io/yaml"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/graph"
	"github.com/open-platform-model/opm-portal/internal/health"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
)

// strs converts a typed constant set to its wire values.
func strs[T ~string](vs ...T) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}

// passedThrough are enumerated fields whose values the portal copies from
// the operator's status without naming them in Go.
var passedThrough = map[string]bool{
	"Catalog/source":      true,
	"GraphCatalog/source": true,
}

// TestEnumsMatchTheGoConstants (portal:D2:R2): every x-extensible-enum in the
// OpenAPI document lists exactly the values the Go code can put on the wire,
// so a value added in Go is documented, and the API gate (hack/api-breaking.sh)
// then refuses its later removal from the document.
func TestEnumsMatchTheGoConstants(t *testing.T) {
	states := strs(health.Healthy, health.Progressing, health.Degraded, health.Missing, health.Unknown)
	access := []string{v1.AccessOK, v1.AccessForbidden, v1.AccessNotReadable, v1.AccessWithheld}
	verdicts := strs(health.VerdictAccepted, health.VerdictRefused, health.VerdictPending, health.VerdictRemovalBlocked, health.VerdictUnknown)
	want := map[string][]string{
		"Reconcile/state": strs(health.AppliedStateApplied, health.AppliedStateReconciling, health.AppliedStateFailed,
			health.AppliedStateStalled, health.AppliedStateSuspended, health.AppliedStateManagedExternally, health.AppliedStateUnknown),
		"Condition/tone": strs(health.ToneNormal, health.ToneAbnormal, health.ToneProgressing, health.ToneInformational,
			health.ToneUnknown),
		"Health/state":                      states,
		"ObjectHealth/state":                states,
		"GraphHealth/state":                 states,
		"InventoryObject/access":            access,
		"InstanceList/access":               access,
		"PackageList/access":                access,
		"Platform/registrationsAccess":      access,
		"GraphPlatform/registrationsAccess": access,
		"GraphNode/access":                  strs(health.AccessOK, health.AccessForbidden, health.AccessNotReadable),
		"InstanceSummary/owner":             strs(readmodel.OwnerOperator, readmodel.OwnerCLI),
		"Registration/verdict":              verdicts,
		"GraphRegistration/verdict":         verdicts,
		"Graph/scope":                       strs(graph.ScopeInstance, graph.ScopePackage, graph.ScopePlatform),
		"GraphGroup/kind":                   strs(graph.GroupConfiguration, graph.GroupPods, graph.GroupMore),
		"GraphNode/kind": strs(graph.KindPlatform, graph.KindCatalog, graph.KindRegistration, graph.KindInstance, graph.KindPackage,
			graph.KindModule, graph.KindSource, graph.KindComponent, graph.KindObject, graph.KindRuntime, graph.KindGroup),
		"GraphEdge/kind": strs(graph.EdgeResolves, graph.EdgeContributes, graph.EdgeProvidedBy, graph.EdgeInstantiates,
			graph.EdgeSourcedFrom, graph.EdgeDependsOn, graph.EdgeHasComponent, graph.EdgeOwns, graph.EdgeControls),
		"Problem/code": {v1.CodeUnauthenticated, v1.CodeForbidden, v1.CodeNotFound, v1.CodeBadRequest,
			v1.CodeMethodNotAllowed, v1.CodeTooManyStreams, v1.CodeNotReadableByPortal, v1.CodeUpstreamUnavailable},
	}

	raw, err := os.ReadFile(openAPIPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]struct {
					Enum []string `json:"x-extensible-enum"`
				} `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for schema, s := range doc.Components.Schemas {
		for prop, p := range s.Properties {
			if p.Enum == nil {
				continue
			}
			key := schema + "/" + prop
			seen[key] = true
			if passedThrough[key] {
				continue
			}
			w, ok := want[key]
			if !ok {
				t.Errorf("%s: x-extensible-enum has no Go constant set in this test", key)
				continue
			}
			got := slices.Sorted(slices.Values(p.Enum))
			if w := slices.Sorted(slices.Values(w)); !slices.Equal(got, w) {
				t.Errorf("%s: document lists %v, Go has %v", key, got, w)
			}
		}
	}
	for key := range want {
		if !seen[key] {
			t.Errorf("%s: no x-extensible-enum in the document", key)
		}
	}
}
