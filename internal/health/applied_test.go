package health

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestReadApplied_Captured(t *testing.T) {
	tests := []struct {
		file     string
		want     AppliedState
		reason   string
		retrying bool
		notes    []string
	}{
		{"mi-podinfo-healthy.yaml", AppliedStateApplied, "ReconciliationSucceeded", false, nil},
		// One minute after the image break: the operator still says applied.
		{"mi-podinfo-image-broken.yaml", AppliedStateApplied, "ReconciliationSucceeded", false, nil},
		{"mi-cert-manager-phase3-healthy.yaml", AppliedStateApplied, "ReconciliationSucceeded", false, nil},
		{"mi-backup-consumer-ready.yaml", AppliedStateApplied, "ReconciliationSucceeded", false, nil},
		{"mi-apply-failed.yaml", AppliedStateFailed, "ApplyFailed", true, nil},
		{"mi-cli-owned.yaml", AppliedStateManagedExternally, "ManagedExternally", false, nil},
		{"mp-source-not-ready.yaml", AppliedStateFailed, "SourceNotReady", true, nil},
		{"platform-fresh.yaml", AppliedStateApplied, "Generated", false, []string{"UnfulfilledContracts"}},
		{"platform-registration-entry.yaml", AppliedStateApplied, "Generated", false, []string{"UnfulfilledContracts"}},
		{"treg-catalog-unresolved.yaml", AppliedStateStalled, "CatalogUnresolved", false, nil},
		{"treg-accepted-active.yaml", AppliedStateApplied, "Accepted", false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			got := ReadApplied(loadCapture(t, tt.file)[0])
			if got.State != tt.want || got.Reason != tt.reason || got.Retrying != tt.retrying {
				t.Fatalf("got %+v, want %s/%s retrying=%v", got, tt.want, tt.reason, tt.retrying)
			}
			if got.Since.IsZero() {
				t.Errorf("since not set")
			}
			var notes []string
			for _, n := range got.Notes {
				notes = append(notes, n.Reason)
			}
			if !reflect.DeepEqual(notes, tt.notes) {
				t.Errorf("notes %v, want %v", notes, tt.notes)
			}
		})
	}
}

func TestReadApplied_NeverHealthAndNotAFault(t *testing.T) {
	if AppliedStateManagedExternally.IsFault() || AppliedStateSuspended.IsFault() || AppliedStateApplied.IsFault() {
		t.Error("neutral states reported as faults")
	}
	if !AppliedStateFailed.IsFault() || !AppliedStateStalled.IsFault() {
		t.Error("failures not reported as faults")
	}
}

// operatorObject builds a ModuleInstance with the given conditions and spec.
func operatorObject(kind string, spec map[string]any, conds ...map[string]any) *unstructured.Unstructured {
	raw := make([]any, 0, len(conds))
	for _, c := range conds {
		if _, ok := c["lastTransitionTime"]; !ok {
			c["lastTransitionTime"] = "2026-10-04T13:41:11Z"
		}
		raw = append(raw, c)
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "opmodel.dev/v1alpha1",
		"kind":       kind,
		"metadata":   map[string]any{"name": "x", "namespace": "ns"},
		"spec":       spec,
		"status":     map[string]any{"conditions": raw},
	}}
}

func cond(typ, status, reason string) map[string]any {
	return map[string]any{"type": typ, "status": status, "reason": reason, "message": reason + " message"}
}

func TestReadApplied_Synthetic(t *testing.T) {
	tests := []struct {
		name   string
		obj    *unstructured.Unstructured
		want   AppliedState
		reason string
	}{
		{"stalled", operatorObject("ModuleInstance", nil, cond("Stalled", "True", "ImpersonationFailed"), cond("Ready", "False", "ImpersonationFailed")), AppliedStateStalled, "ImpersonationFailed"},
		{"suspended by spec", operatorObject("ModulePackage", map[string]any{"suspend": true}, cond("Ready", "True", "ReconciliationSucceeded")), AppliedStateSuspended, "ReconciliationSucceeded"},
		{"suspended by reason", operatorObject("ModuleInstance", nil, cond("Ready", "False", "Suspended")), AppliedStateSuspended, "Suspended"},
		{"reconciling", operatorObject("ModuleInstance", nil, cond("Reconciling", "True", "Progressing"), cond("Ready", "Unknown", "Progressing")), AppliedStateReconciling, "Progressing"},
		{"ready unknown alone", operatorObject("TransformerRegistration", nil, cond("Ready", "Unknown", "ProviderInventoryPending")), AppliedStateReconciling, "ProviderInventoryPending"},
		{"cli owner without conditions", operatorObject("ModuleInstance", map[string]any{"owner": "cli"}), AppliedStateManagedExternally, ""},
		{"no conditions", operatorObject("ModuleInstance", nil), AppliedStateUnknown, ""},
		{"unknown kind", operatorObject("Widget", nil, cond("Ready", "True", "Fine")), AppliedStateUnknown, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReadApplied(tt.obj)
			if got.State != tt.want || got.Reason != tt.reason {
				t.Fatalf("got %+v, want %s/%s", got, tt.want, tt.reason)
			}
		})
	}
}

func TestReadApplied_DriftIsANote(t *testing.T) {
	got := ReadApplied(operatorObject("ModuleInstance", nil, cond("Ready", "True", "ReconciliationSucceeded"), cond("Drifted", "True", "DriftDetected")))
	if got.State != AppliedStateApplied || len(got.Notes) != 1 || got.Notes[0].Reason != "DriftDetected" {
		t.Fatalf("got %+v", got)
	}
}

func TestReadApplied_FailureCountersChangeNothing(t *testing.T) {
	for _, file := range []string{"mi-podinfo-image-broken.yaml", "mi-apply-failed.yaml", "mp-source-not-ready.yaml"} {
		t.Run(file, func(t *testing.T) {
			obj := loadCapture(t, file)[0]
			before := ReadApplied(obj)
			if err := unstructured.SetNestedMap(obj.Object, map[string]any{"reconcile": int64(99), "apply": int64(99), "drift": int64(99)}, "status", "failureCounters"); err != nil {
				t.Fatal(err)
			}
			if after := ReadApplied(obj); !reflect.DeepEqual(before, after) {
				t.Fatalf("failure counters changed the applied axis: %+v to %+v", before, after)
			}
		})
	}
}

func TestReadRegistration_Captured(t *testing.T) {
	tests := []struct {
		file             string
		index            int
		accepted, active bool
		verdict          Verdict
		reason           string
		activeReason     string
	}{
		{"treg-refusals.yaml", 0, false, false, VerdictRefused, "ProvidesMismatch", ""},
		{"treg-refusals.yaml", 1, false, false, VerdictRefused, "ProviderMismatch", ""},
		{"treg-catalog-unresolved.yaml", 0, false, false, VerdictRefused, "CatalogUnresolved", ""},
		{"treg-rendered-refused-beta5.yaml", 0, false, false, VerdictRefused, "CatalogUnresolved", ""},
		{"treg-accepted-active.yaml", 0, true, true, VerdictAccepted, "Accepted", "ProviderReady"},
		{"treg-removal-blocked.yaml", 0, true, true, VerdictRemovalBlocked, "DependentsRemain", "ProviderReady"},
	}
	for _, tt := range tests {
		t.Run(tt.file+"/"+tt.reason, func(t *testing.T) {
			got := ReadRegistration(loadCapture(t, tt.file)[tt.index])
			if got.Accepted != tt.accepted || got.Active != tt.active || got.Verdict != tt.verdict ||
				got.Reason != tt.reason || got.ActiveReason != tt.activeReason {
				t.Fatalf("got %+v", got)
			}
			if got.Message == "" {
				t.Error("message not carried")
			}
		})
	}
}

func TestReadRegistration_Synthetic(t *testing.T) {
	pending := operatorObject("TransformerRegistration", nil, cond("Ready", "Unknown", "ProviderInventoryPending"))
	if got := ReadRegistration(pending); got.Verdict != VerdictPending {
		t.Errorf("pending: %+v", got)
	}
	inactive := operatorObject("TransformerRegistration", nil, cond("Ready", "True", "Accepted"), cond("Active", "False", "ProviderNotReady"))
	inactive.Object["status"].(map[string]any)["accepted"] = true
	if got := ReadRegistration(inactive); got.Verdict != VerdictAccepted || got.Active || got.ActiveReason != "ProviderNotReady" {
		t.Errorf("accepted inactive: %+v", got)
	}
	if got := ReadRegistration(operatorObject("TransformerRegistration", nil)); got.Verdict != VerdictUnknown {
		t.Errorf("no conditions: %+v", got)
	}
	if got := ReadRegistration(operatorObject("ModuleInstance", nil)); got.Verdict != VerdictUnknown {
		t.Errorf("wrong kind: %+v", got)
	}
}
