package health

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestObject_CapturedHealthyObjectsAreHealthy(t *testing.T) {
	for _, file := range []string{
		"objects-podinfo-phase3-healthy.yaml",
		"objects-cert-manager-phase3-healthy.yaml",
		"objects-web-cli-owned.yaml",
	} {
		t.Run(file, func(t *testing.T) {
			objs := loadCapture(t, file)
			if len(objs) == 0 {
				t.Fatal("empty capture")
			}
			for _, o := range objs {
				if got := Object(o); got.State != Healthy {
					t.Errorf("%s %s: got %s (%s), want Healthy", o.GetKind(), o.GetName(), got.State, got.Message)
				}
			}
		})
	}
}

func TestObject_ImageBreak(t *testing.T) {
	tests := []struct {
		file       string
		kind, name string
		want       State
		reason     string
	}{
		// One minute in: kstatus alone says the Deployment is in progress.
		{"objects-podinfo-phase4-broken-1min.yaml", "Deployment", "podinfo-podinfo", Progressing, "InProgress"},
		{"objects-podinfo-phase4-broken-1min.yaml", "Pod", "podinfo-podinfo-794bd8c7fb-cc5ql", Degraded, "ImagePullBackOff"},
		{"objects-podinfo-phase4-broken-1min.yaml", "Pod", "podinfo-podinfo-d9585d794-5m9tx", Healthy, "Current"},
		// After the progress deadline kstatus agrees.
		{"objects-podinfo-phase5-broken-deadline.yaml", "Deployment", "podinfo-podinfo", Degraded, "Failed"},
		{"objects-podinfo-phase5-broken-deadline.yaml", "Pod", "podinfo-podinfo-794bd8c7fb-cc5ql", Degraded, "ImagePullBackOff"},
	}
	for _, tt := range tests {
		t.Run(tt.file+"/"+tt.name, func(t *testing.T) {
			got := Object(find(t, loadCapture(t, tt.file), tt.kind, tt.name))
			if got.State != tt.want || got.Reason != tt.reason {
				t.Fatalf("got %s/%s (%s), want %s/%s", got.State, got.Reason, got.Message, tt.want, tt.reason)
			}
		})
	}
}

func TestObject_ProgressDeadlineMessage(t *testing.T) {
	got := Object(find(t, loadCapture(t, "objects-podinfo-phase5-broken-deadline.yaml"), "Deployment", "podinfo-podinfo"))
	if got.Message != "Progress deadline exceeded" {
		t.Fatalf("message %q", got.Message)
	}
}

func waitingPod(field, reason string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"name": "p", "namespace": "ns", "generation": int64(1)},
		"status": map[string]any{
			"phase": "Pending",
			field: []any{map[string]any{
				"name":  "app",
				"state": map[string]any{"waiting": map[string]any{"reason": reason, "message": "details"}},
			}},
		},
	}}
}

func TestObject_PodRule(t *testing.T) {
	tests := []struct {
		field, reason string
		want          State
	}{
		{"containerStatuses", "ErrImagePull", Degraded},
		{"containerStatuses", "ImagePullBackOff", Degraded},
		{"containerStatuses", "CrashLoopBackOff", Degraded},
		{"containerStatuses", "CreateContainerConfigError", Degraded},
		{"containerStatuses", "InvalidImageName", Degraded},
		{"initContainerStatuses", "CrashLoopBackOff", Degraded},
		// Not in the rule: kstatus decides, and a Pending Pod is in progress.
		{"containerStatuses", "ContainerCreating", Progressing},
		{"containerStatuses", "PodInitializing", Progressing},
	}
	for _, tt := range tests {
		t.Run(tt.field+"/"+tt.reason, func(t *testing.T) {
			got := Object(waitingPod(tt.field, tt.reason))
			if got.State != tt.want {
				t.Fatalf("got %s (%s), want %s", got.State, got.Message, tt.want)
			}
			if tt.want == Degraded && (got.Reason != tt.reason || !strings.Contains(got.Message, "container app")) {
				t.Fatalf("reason %q message %q", got.Reason, got.Message)
			}
		})
	}
}

func TestObject_NilIsMissing(t *testing.T) {
	if got := Object(nil); got.State != Missing {
		t.Fatalf("got %s", got.State)
	}
}

func TestObject_KstatusErrorIsUnknown(t *testing.T) {
	// A generation decoded as float64 is what kstatus refuses.
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"name": "d", "generation": 1.0},
	}}
	got := Object(u)
	if got.State != Unknown || !strings.Contains(got.Message, "computing status") {
		t.Fatalf("got %s (%s)", got.State, got.Message)
	}
}

func TestStateRank_WorstFirstOrder(t *testing.T) {
	order := []State{Degraded, Missing, Progressing, Unknown, Healthy}
	for i := 0; i+1 < len(order); i++ {
		if !order[i].worse(order[i+1]) {
			t.Errorf("%s should be worse than %s", order[i], order[i+1])
		}
	}
}
