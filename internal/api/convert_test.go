package api

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-platform-model/opm-portal/internal/health"
)

// TestConditionCarriesToneAndExplanation: a known reason brings its meaning,
// an unknown one brings none and keeps the operator's message.
func TestConditionCarriesToneAndExplanation(t *testing.T) {
	known := condition(health.Condition{Type: "Stalled", Status: metav1.ConditionTrue, Reason: "RenderFailed", Message: "boom"})
	if known.Tone != "abnormal" || known.Meaning == "" {
		t.Errorf("Stalled=True RenderFailed: tone %q meaning %q", known.Tone, known.Meaning)
	}
	unknown := condition(health.Condition{Type: "Shiny", Status: metav1.ConditionTrue, Reason: "NoSuchReason", Message: "as written"})
	if unknown.Tone != "unknown" || unknown.Meaning != "" || unknown.NextStep != "" || unknown.Message != "as written" {
		t.Errorf("unknown reason: %+v", unknown)
	}
}
