package readmodel

import (
	"errors"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	platformRef     = ObjectRef{Group: opmGroup, Version: opmVersion, Kind: "Platform", Name: "cluster"}
	refusedClaimRef = ObjectRef{Group: opmGroup, Version: opmVersion, Kind: "TransformerRegistration", Name: "default.refused-claim-fixture"}
)

func (e *env) events(t *testing.T, about ObjectRef) []Event {
	t.Helper()
	got, err := e.m.Events(t.Context(), alice, e.grant(t, "list", events, EventNamespace(about), ""), about)
	if err != nil {
		t.Fatalf("Events(%+v) = %v", about, err)
	}
	return got
}

// TestClusterScopedEventsLiveInDefault: the Platform's and the
// registrations' events are read from namespace default (portal:D9:R4), and
// the Platform's two identical Generated events fold into one line.
func TestClusterScopedEventsLiveInDefault(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	got := e.events(t, platformRef)
	if len(got) != 1 {
		t.Fatalf("platform events = %+v, want one folded line", got)
	}
	gen := got[0]
	if gen.Reason != "Generated" || gen.Count != 2 || gen.ReportingController != "opm-controller" ||
		!gen.LastSeen.Equal(time.Date(2026, 10, 4, 18, 4, 23, 340984000, time.UTC)) {
		t.Errorf("Generated line = %+v, want count 2 at the later event's time", gen)
	}
	if !strings.Contains(strings.Join(clusterReads(e.client), "\n"), "list events default/") {
		t.Errorf("platform events not read from default: %v", clusterReads(e.client))
	}

	claim := e.events(t, refusedClaimRef)
	if len(claim) != 1 || claim[0].Reason != "CatalogUnresolved" || claim[0].Type != "Warning" || claim[0].Regarding.Name != refusedClaimRef.Name {
		t.Errorf("refused claim events = %+v", claim)
	}
}

func TestInstanceEventsStayOnTheirObject(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	got := e.events(t, ObjectRef{Group: opmGroup, Version: opmVersion, Kind: "ModuleInstance", Namespace: "default", Name: "podinfo"})
	reasons := make([]string, 0, len(got))
	for i := range got {
		if got[i].Regarding.Name != "podinfo" || got[i].Regarding.Kind != "ModuleInstance" {
			t.Errorf("event about another object: %+v", got[i])
		}
		reasons = append(reasons, got[i].Reason)
	}
	if strings.Join(reasons, ",") != "NoOp,ReconciliationSucceeded,Applied" {
		t.Errorf("reasons newest first = %v", reasons)
	}
	cli := e.events(t, ObjectRef{Group: opmGroup, Version: opmVersion, Kind: "ModuleInstance", Namespace: "web", Name: "web"})
	if len(cli) != 1 || cli[0].Reason != "ManagedExternally" {
		t.Errorf("CLI-owned instance events = %+v", cli)
	}
}

// TestKubeletEventsUseTheDeprecatedFields: kubelet events have no event
// time; their time and count come from the deprecated fields.
func TestKubeletEventsUseTheDeprecatedFields(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	got := e.events(t, ObjectRef{Version: "v1", Kind: "Pod", Namespace: "cert-manager", Name: "cert-manager-cainjector-5d69c59bd-htjjq"})
	var pulling *Event
	for i := range got {
		if got[i].Reason == "Pulling" {
			pulling = &got[i]
		}
	}
	if pulling == nil {
		t.Fatalf("no Pulling line in %+v", got)
	}
	if pulling.Count != 1 || !pulling.LastSeen.Equal(time.Date(2026, 10, 4, 18, 4, 6, 0, time.UTC)) ||
		pulling.FieldPath != "spec.containers{cert-manager-cainjector}" {
		t.Errorf("Pulling = %+v", pulling)
	}
}

// TestFoldCountsEveryWayARepeatIsRecorded (portal:D9:R3): two separate
// events, a series of three and a deprecated count of four, all with the
// same object, type, reason and note, fold into one line of nine.
func TestFoldCountsEveryWayARepeatIsRecorded(t *testing.T) {
	mk := func(name string, extra map[string]any) *unstructured.Unstructured {
		obj := map[string]any{
			"apiVersion": "events.k8s.io/v1", "kind": "Event",
			"metadata": map[string]any{"name": name, "namespace": "a", "creationTimestamp": "2026-10-04T10:00:00Z"},
			"type":     "Warning", "reason": "BackOff", "note": "Back-off restarting failed container",
			"regarding": map[string]any{"apiVersion": "v1", "kind": "Pod", "namespace": "a", "name": "p", "uid": "u1"},
		}
		for k, v := range extra {
			obj[k] = v
		}
		return &unstructured.Unstructured{Object: obj}
	}
	evs := []*unstructured.Unstructured{
		mk("one", nil),
		mk("two", map[string]any{"eventTime": "2026-10-04T10:01:00.000001Z"}),
		mk("series", map[string]any{"series": map[string]any{"count": int64(3), "lastObservedTime": "2026-10-04T10:05:00.000000Z"}}),
		mk("kubelet", map[string]any{"deprecatedCount": int64(4), "deprecatedLastTimestamp": "2026-10-04T10:03:00Z"}),
		mk("other-note", map[string]any{"note": "something else"}),
	}
	got := foldEvents(evs)
	if len(got) != 2 {
		t.Fatalf("lines = %+v, want two", got)
	}
	if got[0].Count != 9 || !got[0].LastSeen.Equal(time.Date(2026, 10, 4, 10, 5, 0, 0, time.UTC)) {
		t.Errorf("folded line = %+v, want count 9 at 10:05", got[0])
	}
	if got[1].Note != "something else" || got[1].Count != 1 {
		t.Errorf("other line = %+v", got[1])
	}
}

func TestEventsRefuseAnUncoveredGrant(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	before := len(clusterReads(e.client))
	// A grant for the Platform's own namespace does not exist: its events
	// are in default, and a grant for kube-system does not cover them.
	g := e.grant(t, "list", events, "kube-system", "")
	if _, err := e.m.Events(t.Context(), alice, g, platformRef); !errors.Is(err, ErrNotCovered) {
		t.Fatalf("Events = %v, want ErrNotCovered", err)
	}
	if after := len(clusterReads(e.client)); after != before {
		t.Fatalf("a refused read listed events")
	}
}

func TestEventsUnavailableToTheReader(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, denyResources("events"))
	g := e.grant(t, "list", events, "default", "")
	if _, err := e.m.Events(t.Context(), alice, g, platformRef); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Events = %v, want ErrUnavailable", err)
	}
}
