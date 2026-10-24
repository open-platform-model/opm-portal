package readmodel

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

const testSvcUID types.UID = "svc-uid"

// sliceOf builds an EndpointSlice of addressType owned (as controller) by
// owner, holding eps.
func sliceOf(addressType string, owner types.UID, eps ...map[string]any) unstructured.Unstructured {
	s := unstructured.Unstructured{Object: map[string]any{
		"apiVersion":  "discovery.k8s.io/v1",
		"kind":        "EndpointSlice",
		"addressType": addressType,
	}}
	s.SetName("slice")
	s.SetNamespace("default")
	if owner != "" {
		yes := true
		s.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "v1", Kind: "Service", Name: "svc", UID: owner, Controller: &yes}})
	}
	items := make([]any, 0, len(eps))
	for _, e := range eps {
		items = append(items, e)
	}
	if len(items) > 0 {
		s.Object["endpoints"] = items
	}
	return s
}

// endpoint builds one endpoint. ready is "true", "false" or "" (unset).
func endpoint(uid, addr, ready string) map[string]any {
	e := map[string]any{}
	if uid != "" {
		e["targetRef"] = map[string]any{"kind": "Pod", "name": "pod-" + uid, "uid": uid}
	}
	if addr != "" {
		e["addresses"] = []any{addr}
	}
	switch ready {
	case "true":
		e["conditions"] = map[string]any{"ready": true}
	case "false":
		e["conditions"] = map[string]any{"ready": false}
	default:
		e["conditions"] = map[string]any{"serving": true}
	}
	return e
}

func TestCountReady(t *testing.T) {
	tests := []struct {
		name      string
		slices    []unstructured.Unstructured
		wantReady int64
		wantFound bool
	}{
		{"ready true", []unstructured.Unstructured{sliceOf("IPv4", testSvcUID, endpoint("a", "10.0.0.1", "true"))}, 1, true},
		{"ready unset counts as ready", []unstructured.Unstructured{sliceOf("IPv4", testSvcUID, endpoint("a", "10.0.0.1", ""))}, 1, true},
		{"no conditions at all counts as ready", []unstructured.Unstructured{sliceOf("IPv4", testSvcUID, map[string]any{"addresses": []any{"10.0.0.1"}})}, 1, true},
		{"ready false", []unstructured.Unstructured{sliceOf("IPv4", testSvcUID, endpoint("a", "10.0.0.1", "false"))}, 0, true},
		{"terminating is ready false", []unstructured.Unstructured{sliceOf("IPv4", testSvcUID, map[string]any{
			"addresses": []any{"10.0.0.1"}, "targetRef": map[string]any{"uid": "a"},
			"conditions": map[string]any{"ready": false, "serving": true, "terminating": true},
		})}, 0, true},
		{"ready true with serving false counts as recorded (publishNotReadyAddresses)", []unstructured.Unstructured{sliceOf("IPv4", testSvcUID, map[string]any{
			"addresses": []any{"10.0.0.1"}, "targetRef": map[string]any{"uid": "a"},
			"conditions": map[string]any{"ready": true, "serving": false, "terminating": false},
		})}, 1, true},
		{"a mixed slice", []unstructured.Unstructured{sliceOf("IPv4", testSvcUID,
			endpoint("a", "10.0.0.1", "true"), endpoint("b", "10.0.0.2", "false"), endpoint("c", "10.0.0.3", ""))}, 2, true},
		{"two slices of one type add up", []unstructured.Unstructured{
			sliceOf("IPv4", testSvcUID, endpoint("a", "10.0.0.1", "true")),
			sliceOf("IPv4", testSvcUID, endpoint("b", "10.0.0.2", "true")),
		}, 2, true},
		{"an IPv4 and an IPv6 slice of the same Pods count 2", []unstructured.Unstructured{
			sliceOf("IPv4", testSvcUID, endpoint("a", "10.0.0.1", "true"), endpoint("b", "10.0.0.2", "true")),
			sliceOf("IPv6", testSvcUID, endpoint("a", "fd00::1", "true"), endpoint("b", "fd00::2", "true")),
		}, 2, true},
		{"an FQDN slice owned by the Service adds nothing", []unstructured.Unstructured{
			sliceOf("IPv4", testSvcUID, endpoint("a", "10.0.0.1", "true")),
			sliceOf("FQDN", testSvcUID, endpoint("x", "one.example", "true"), endpoint("y", "two.example", "true")),
		}, 1, true},
		{"an FQDN slice alone is not found", []unstructured.Unstructured{
			sliceOf("FQDN", testSvcUID, endpoint("x", "one.example", "true")),
		}, 0, false},
		{"one endpoint in two slices of one type counts once", []unstructured.Unstructured{
			sliceOf("IPv4", testSvcUID, endpoint("a", "10.0.0.1", "true")),
			sliceOf("IPv4", testSvcUID, endpoint("a", "10.0.0.1", "true")),
		}, 1, true},
		{"the same endpoint ready in one copy and not in the other is not counted", []unstructured.Unstructured{
			sliceOf("IPv4", testSvcUID, endpoint("a", "10.0.0.1", "true")),
			sliceOf("IPv4", testSvcUID, endpoint("a", "10.0.0.1", "false")),
		}, 0, true},
		{"the conflict does not depend on the order of the copies", []unstructured.Unstructured{
			sliceOf("IPv4", testSvcUID, endpoint("a", "10.0.0.1", "false")),
			sliceOf("IPv4", testSvcUID, endpoint("a", "10.0.0.1", "true")),
		}, 0, true},
		{"an endpoint with no target is keyed by its address", []unstructured.Unstructured{
			sliceOf("IPv4", testSvcUID, endpoint("", "10.0.0.1", "true")),
			sliceOf("IPv4", testSvcUID, endpoint("", "10.0.0.1", "true"), endpoint("", "10.0.0.2", "true")),
		}, 2, true},
		{"an endpoint with neither target nor address counts on its own", []unstructured.Unstructured{
			sliceOf("IPv4", testSvcUID, map[string]any{}, map[string]any{}),
		}, 2, true},
		{"a placeholder slice with no endpoints is 0 and found", []unstructured.Unstructured{sliceOf("IPv4", testSvcUID)}, 0, true},
		{"no slice is not found", nil, 0, false},
		{"a slice with the label and no owner is skipped", []unstructured.Unstructured{sliceOf("IPv4", "", endpoint("a", "10.0.0.1", "true"))}, 0, false},
		{"a slice owned by another uid is skipped", []unstructured.Unstructured{sliceOf("IPv4", "other-uid", endpoint("a", "10.0.0.1", "true"))}, 0, false},
		{"a slice with no address type is skipped", []unstructured.Unstructured{sliceOf("", testSvcUID, endpoint("a", "10.0.0.1", "true"))}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ready, found := countReady(tt.slices, testSvcUID)
			if ready != tt.wantReady || found != tt.wantFound {
				t.Errorf("countReady = (%d, %t), want (%d, %t)", ready, found, tt.wantReady, tt.wantFound)
			}
		})
	}
}

// A slice whose owner reference is not a controller reference is skipped
// even when it names the Service.
func TestCountReadyNeedsAControllerReference(t *testing.T) {
	s := sliceOf("IPv4", testSvcUID, endpoint("a", "10.0.0.1", "true"))
	no := false
	s.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "v1", Kind: "Service", Name: "svc", UID: testSvcUID, Controller: &no}})
	if ready, found := countReady([]unstructured.Unstructured{s}, testSvcUID); ready != 0 || found {
		t.Errorf("countReady = (%d, %t), want (0, false)", ready, found)
	}
}
