package readmodel

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-platform-model/opm-portal/internal/health"
)

// Field readers over operator custom resources. They read what the operator
// wrote and copy it into views; a field that is absent or of another type
// reads as its zero value, never as a guess.

func str(obj map[string]any, fields ...string) string {
	v, found, err := unstructured.NestedString(obj, fields...)
	if err != nil || !found {
		return ""
	}
	return v
}

func i64(obj map[string]any, fields ...string) (int64, bool) {
	v, found, err := unstructured.NestedInt64(obj, fields...)
	if err != nil || !found {
		return 0, false
	}
	return v, true
}

func num(obj map[string]any, fields ...string) int64 {
	v, _ := i64(obj, fields...)
	return v
}

func boolean(obj map[string]any, fields ...string) (value, found bool) {
	v, found, err := unstructured.NestedBool(obj, fields...)
	return v, found && err == nil
}

func strs(obj map[string]any, fields ...string) []string {
	v, found, err := unstructured.NestedStringSlice(obj, fields...)
	if err != nil || !found {
		return nil
	}
	return v
}

// object returns the map at the field path, or nil.
func object(obj map[string]any, fields ...string) map[string]any {
	raw, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if err != nil || !found {
		return nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return m
}

// maps returns the maps in the list at the field path, skipping anything
// else.
func maps(obj map[string]any, fields ...string) []map[string]any {
	raw, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if err != nil || !found {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func timestamp(obj map[string]any, fields ...string) time.Time {
	s := str(obj, fields...)
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func refOf(u *unstructured.Unstructured) ObjectRef {
	gvk := u.GroupVersionKind()
	return ObjectRef{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind, Namespace: u.GetNamespace(), Name: u.GetName()}
}

// conditions reads status.conditions in the operator's order.
func conditions(u *unstructured.Unstructured) []health.Condition {
	raw := maps(u.Object, "status", "conditions")
	out := make([]health.Condition, 0, len(raw))
	for _, c := range raw {
		out = append(out, health.Condition{
			Type:               str(c, "type"),
			Status:             metav1.ConditionStatus(str(c, "status")),
			Reason:             str(c, "reason"),
			Message:            str(c, "message"),
			LastTransitionTime: timestamp(c, "lastTransitionTime"),
			ObservedGeneration: num(c, "observedGeneration"),
		})
	}
	return out
}

// history reads status.history in the operator's order (newest first).
func history(u *unstructured.Unstructured) []HistoryEntry {
	raw := maps(u.Object, "status", "history")
	out := make([]HistoryEntry, 0, len(raw))
	for _, h := range raw {
		out = append(out, HistoryEntry{
			Action:          str(h, "action"),
			Phase:           str(h, "phase"),
			Sequence:        num(h, "sequence"),
			StartedAt:       timestamp(h, "startedAt"),
			FinishedAt:      timestamp(h, "finishedAt"),
			Message:         str(h, "message"),
			InventoryCount:  num(h, "inventoryCount"),
			InventoryDigest: str(h, "inventoryDigest"),
			Digests: Digests{
				Source: str(h, "sourceDigest"),
				Config: str(h, "configDigest"),
				Render: str(h, "renderDigest"),
			},
		})
	}
	return out
}

func lastApplied(u *unstructured.Unstructured) Digests {
	return Digests{
		Source: str(u.Object, "status", "lastAppliedSourceDigest"),
		Config: str(u.Object, "status", "lastAppliedConfigDigest"),
		Render: str(u.Object, "status", "lastAppliedRenderDigest"),
	}
}

// inventory reads status.inventory.entries. An object with no inventory
// (an apply that failed before recording one) has none, which is not an
// unreadable inventory.
func inventory(u *unstructured.Unstructured) []health.Ref {
	raw := maps(u.Object, "status", "inventory", "entries")
	out := make([]health.Ref, 0, len(raw))
	for _, e := range raw {
		out = append(out, health.Ref{
			Group:     str(e, "group"),
			Version:   str(e, "v"),
			Kind:      str(e, "kind"),
			Namespace: str(e, "namespace"),
			Name:      str(e, "name"),
			Component: str(e, "component"),
		})
	}
	return out
}

// inventoryCount is the operator's count, or the number of entries when it
// wrote none.
func inventoryCount(u *unstructured.Unstructured, entries []health.Ref) int64 {
	if n, found := i64(u.Object, "status", "inventory", "count"); found {
		return n
	}
	return int64(len(entries))
}

func ownerOf(u *unstructured.Unstructured) Owner {
	if str(u.Object, "spec", "owner") == string(OwnerCLI) {
		return OwnerCLI
	}
	return OwnerOperator
}
