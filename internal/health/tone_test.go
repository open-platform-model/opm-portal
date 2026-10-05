package health

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConditionTone(t *testing.T) {
	cases := []struct {
		typ    string
		status metav1.ConditionStatus
		want   Tone
	}{
		{"Ready", metav1.ConditionTrue, ToneNormal},
		{"Ready", metav1.ConditionFalse, ToneAbnormal},
		{"Ready", metav1.ConditionUnknown, ToneUnknown},
		{"Stalled", metav1.ConditionTrue, ToneAbnormal},
		{"Stalled", metav1.ConditionFalse, ToneNormal},
		{"Reconciling", metav1.ConditionTrue, ToneProgressing},
		{"Reconciling", metav1.ConditionFalse, ToneNormal},
		{"ContractsFulfilled", metav1.ConditionFalse, ToneInformational},
		{"ContractsFulfilled", metav1.ConditionTrue, ToneNormal},
		{"Drifted", metav1.ConditionTrue, ToneInformational},
		{"Active", metav1.ConditionFalse, ToneInformational},
		{"ModuleResolved", metav1.ConditionTrue, ToneNormal},
		{"SomethingNew", metav1.ConditionTrue, ToneUnknown},
	}
	for _, c := range cases {
		if got := ConditionTone(Condition{Type: c.typ, Status: c.status}); got != c.want {
			t.Errorf("%s=%s: tone %q, want %q", c.typ, c.status, got, c.want)
		}
	}
}
