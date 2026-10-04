package health

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var captureTime = time.Date(2026, 10, 4, 13, 48, 0, 0, time.UTC)

// inputFromCapture builds an Input the way the read model will: one entry per
// inventory entry of the captured ModuleInstance, matched to the captured live
// object, and every other captured object as a runtime child.
func inputFromCapture(t *testing.T, instanceFile, objectsFile string) Input {
	t.Helper()
	instance := loadCapture(t, instanceFile)[0]
	objs := loadCapture(t, objectsFile)
	raw, found, err := unstructured.NestedSlice(instance.Object, "status", "inventory", "entries")
	if err != nil || !found {
		t.Fatalf("%s has no inventory: %v", instanceFile, err)
	}
	in := Input{ChildrenAccess: AccessOK}
	used := map[*unstructured.Unstructured]bool{}
	for _, r := range raw {
		e, ok := r.(map[string]any)
		if !ok {
			t.Fatalf("inventory entry %v", r)
		}
		ref := Ref{
			Group:     stringAt(e, "group"),
			Version:   stringAt(e, "v"),
			Kind:      stringAt(e, "kind"),
			Namespace: stringAt(e, "namespace"),
			Name:      stringAt(e, "name"),
			Component: stringAt(e, "component"),
		}
		var live *unstructured.Unstructured
		for _, o := range objs {
			if o.GroupVersionKind().Group == ref.Group && o.GetKind() == ref.Kind &&
				o.GetNamespace() == ref.Namespace && o.GetName() == ref.Name {
				live = o
			}
		}
		if live == nil {
			t.Fatalf("inventory entry %+v not captured", ref)
		}
		used[live] = true
		in.Entries = append(in.Entries, Entry{Ref: ref, Access: AccessOK, Object: live, EvaluatedAt: captureTime, Live: true})
	}
	for _, o := range objs {
		if !used[o] {
			in.Children = append(in.Children, o)
		}
	}
	return in
}

func TestEvaluate_CapturedInstances(t *testing.T) {
	tests := []struct {
		name               string
		instance, objects  string
		entries            int
		want               State
		deploymentReason   string
		deploymentName     string
		components         int
		wantHealthyObjects int
	}{
		{"cert-manager healthy", "mi-cert-manager-phase3-healthy.yaml", "objects-cert-manager-phase3-healthy.yaml", 42, Healthy, "Current", "cert-manager-webhook", 20, 42},
		{"podinfo healthy", "mi-podinfo-healthy.yaml", "objects-podinfo-phase3-healthy.yaml", 2, Healthy, "Current", "podinfo-podinfo", 1, 2},
		{"podinfo one minute after the break", "mi-podinfo-image-broken.yaml", "objects-podinfo-phase4-broken-1min.yaml", 2, Degraded, "ImagePullBackOff", "podinfo-podinfo", 1, 1},
		{"podinfo after the deadline", "mi-podinfo-image-broken.yaml", "objects-podinfo-phase5-broken-deadline.yaml", 2, Degraded, "Failed", "podinfo-podinfo", 1, 1},
		{"cli-owned web", "mi-cli-owned.yaml", "objects-web-cli-owned.yaml", 2, Healthy, "Current", "web-web", 1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := inputFromCapture(t, tt.instance, tt.objects)
			if len(in.Entries) != tt.entries {
				t.Fatalf("entries %d, want %d", len(in.Entries), tt.entries)
			}
			got := Evaluate(in)
			if got.Instance.State != tt.want || got.Instance.Partial || !got.Instance.Live {
				t.Fatalf("instance %+v, want %s, complete and live", got.Instance, tt.want)
			}
			if got.Instance.Counts.Healthy != tt.wantHealthyObjects {
				t.Errorf("healthy count %d, want %d", got.Instance.Counts.Healthy, tt.wantHealthyObjects)
			}
			if len(got.Components) != tt.components {
				t.Errorf("components %d, want %d", len(got.Components), tt.components)
			}
			if !got.Instance.EvaluatedAt.Equal(captureTime) {
				t.Errorf("evaluatedAt %v", got.Instance.EvaluatedAt)
			}
			for _, o := range got.Objects {
				if o.Ref.Kind == "Deployment" && o.Ref.Name == tt.deploymentName && o.Health.Reason != tt.deploymentReason {
					t.Errorf("deployment reason %q (%s), want %q", o.Health.Reason, o.Health.Message, tt.deploymentReason)
				}
			}
		})
	}
}

func TestEvaluate_ImageBreakDegradesComponent(t *testing.T) {
	got := Evaluate(inputFromCapture(t, "mi-podinfo-image-broken.yaml", "objects-podinfo-phase4-broken-1min.yaml"))
	if len(got.Components) != 1 || got.Components[0].Name != "podinfo" || got.Components[0].State != Degraded {
		t.Fatalf("components %+v", got.Components)
	}
	// Children read and none found: kstatus alone says the Deployment
	// progresses, and the result is complete.
	in := inputFromCapture(t, "mi-podinfo-image-broken.yaml", "objects-podinfo-phase4-broken-1min.yaml")
	in.Children = nil
	if got := Evaluate(in).Instance; got.State != Progressing || got.Partial {
		t.Fatalf("without children got %+v, want Progressing and complete", got)
	}
}

// When the reader may read the Deployment but not the Pods below it, the
// broken rollout cannot be seen; the result must say it is partial instead
// of looking merely in progress.
func TestEvaluate_ChildrenUnreadable(t *testing.T) {
	for _, access := range []Access{AccessForbidden, AccessNotReadable, ""} {
		t.Run(string(access), func(t *testing.T) {
			in := inputFromCapture(t, "mi-podinfo-image-broken.yaml", "objects-podinfo-phase4-broken-1min.yaml")
			in.Children, in.ChildrenAccess = nil, access
			got := Evaluate(in)
			if got.Instance.State != Progressing || !got.Instance.Partial {
				t.Fatalf("instance %+v, want Progressing and partial", got.Instance)
			}
			if len(got.Components) != 1 || !got.Components[0].Partial {
				t.Fatalf("components %+v, want the podinfo component partial", got.Components)
			}
			for _, o := range got.Objects {
				if want := o.Ref.Kind == "Deployment"; o.ChildrenUnread != want {
					t.Errorf("%s %s childrenUnread=%v, want %v", o.Ref.Kind, o.Ref.Name, o.ChildrenUnread, want)
				}
			}
		})
	}
	// Children handed in but marked unreadable are not trusted either.
	in := inputFromCapture(t, "mi-podinfo-image-broken.yaml", "objects-podinfo-phase4-broken-1min.yaml")
	in.ChildrenAccess = AccessForbidden
	if got := Evaluate(in).Instance; got.State != Progressing || !got.Partial {
		t.Fatalf("instance %+v, want Progressing and partial", got)
	}
}

// An instance with no workload is complete whatever happened to the children.
func TestEvaluate_ChildrenUnreadableWithoutWorkloads(t *testing.T) {
	in := inputFromCapture(t, "mi-podinfo-healthy.yaml", "objects-podinfo-phase3-healthy.yaml")
	var kept []Entry
	for _, e := range in.Entries {
		if e.Ref.Kind != "Deployment" {
			kept = append(kept, e)
		}
	}
	in.Entries, in.ChildrenAccess = kept, AccessForbidden
	if got := Evaluate(in).Instance; got.State != Healthy || got.Partial {
		t.Fatalf("instance %+v, want Healthy and complete", got)
	}
}

func TestEvaluate_ChildrenAreNotCounted(t *testing.T) {
	got := Evaluate(inputFromCapture(t, "mi-podinfo-healthy.yaml", "objects-podinfo-phase3-healthy.yaml"))
	c := got.Instance.Counts
	if total := c.Healthy + c.Progressing + c.Degraded + c.Missing + c.Unknown; total != 2 {
		t.Fatalf("counted %d objects, want the 2 inventory objects", total)
	}
}

func TestEvaluate_PodOwnedByNothingInTheInventory(t *testing.T) {
	in := inputFromCapture(t, "mi-podinfo-healthy.yaml", "objects-podinfo-phase3-healthy.yaml")
	stray := waitingPod("containerStatuses", "CrashLoopBackOff")
	isController := true
	stray.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "other", UID: "not-in-inventory", Controller: &isController}})
	in.Children = append(in.Children, stray, waitingPod("containerStatuses", "ErrImagePull"))
	if s := Evaluate(in).Instance.State; s != Healthy {
		t.Fatalf("got %s, want Healthy", s)
	}
}

func TestEvaluate_OwnerWalkIsBounded(t *testing.T) {
	in := inputFromCapture(t, "mi-podinfo-healthy.yaml", "objects-podinfo-phase3-healthy.yaml")
	// Two ReplicaSets owning each other: the walk must stop.
	a := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "apps/v1", "kind": "ReplicaSet", "metadata": map[string]any{"name": "a", "uid": "a"}}}
	b := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "apps/v1", "kind": "ReplicaSet", "metadata": map[string]any{"name": "b", "uid": "b"}}}
	isController := true
	a.SetOwnerReferences([]metav1.OwnerReference{{Kind: "ReplicaSet", Name: "b", UID: "b", Controller: &isController}})
	b.SetOwnerReferences([]metav1.OwnerReference{{Kind: "ReplicaSet", Name: "a", UID: "a", Controller: &isController}})
	pod := waitingPod("containerStatuses", "CrashLoopBackOff")
	pod.SetOwnerReferences([]metav1.OwnerReference{{Kind: "ReplicaSet", Name: "a", UID: "a", Controller: &isController}})
	in.Children = append(in.Children, a, b, pod)
	if s := Evaluate(in).Instance.State; s != Healthy {
		t.Fatalf("got %s, want Healthy", s)
	}
}

// Only controller references are followed: a broken Pod that merely names an
// inventory object as a plain owner does not degrade it.
func TestEvaluate_NonControllerOwnerIsNotFollowed(t *testing.T) {
	in := inputFromCapture(t, "mi-podinfo-healthy.yaml", "objects-podinfo-phase3-healthy.yaml")
	var deployment *unstructured.Unstructured
	for _, e := range in.Entries {
		if e.Ref.Kind == "Deployment" {
			deployment = e.Object
		}
	}
	pod := waitingPod("containerStatuses", "CrashLoopBackOff")
	pod.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: deployment.GetName(), UID: deployment.GetUID()}})
	in.Children = append(in.Children, pod)
	if s := Evaluate(in).Instance.State; s != Healthy {
		t.Fatalf("got %s, want Healthy", s)
	}
	isController := true
	pod.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: deployment.GetName(), UID: deployment.GetUID(), Controller: &isController}})
	if s := Evaluate(in).Instance.State; s != Degraded {
		t.Fatalf("with a controller reference got %s, want Degraded", s)
	}
}

func TestEvaluate_AccessAndCompleteness(t *testing.T) {
	healthy := func() Input {
		return inputFromCapture(t, "mi-podinfo-healthy.yaml", "objects-podinfo-phase3-healthy.yaml")
	}
	secretRef := Ref{Version: "v1", Kind: "Secret", Namespace: "default", Name: "creds", Component: "podinfo"}
	tests := []struct {
		name    string
		mutate  func(in *Input)
		want    State
		partial bool
		check   func(t *testing.T, r Result)
	}{
		{
			name:    "forbidden object",
			mutate:  func(in *Input) { in.Entries[1].Access, in.Entries[1].Object = AccessForbidden, nil },
			want:    Healthy,
			partial: true,
			check: func(t *testing.T, r Result) {
				if r.Instance.Counts.Forbidden != 1 || r.Instance.Counts.Healthy != 1 {
					t.Errorf("counts %+v", r.Instance.Counts)
				}
			},
		},
		{
			name:    "not readable object",
			mutate:  func(in *Input) { in.Entries[0].Access, in.Entries[0].Object = AccessNotReadable, nil },
			want:    Healthy,
			partial: true,
		},
		{
			name: "secret in the inventory is withheld even when handed in",
			mutate: func(in *Input) {
				secret := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]any{"name": "creds"}}}
				in.Entries = append(in.Entries, Entry{Ref: secretRef, Access: AccessOK, Object: secret, Live: true})
			},
			want: Healthy,
			check: func(t *testing.T, r Result) {
				last := r.Objects[len(r.Objects)-1]
				if last.Access != AccessWithheld || r.Instance.Counts.Withheld != 1 {
					t.Errorf("secret result %+v counts %+v", last, r.Instance.Counts)
				}
			},
		},
		{
			name: "every object forbidden",
			mutate: func(in *Input) {
				for i := range in.Entries {
					in.Entries[i].Access, in.Entries[i].Object = AccessForbidden, nil
				}
			},
			want:    Unknown,
			partial: true,
		},
		{
			name:   "empty inventory",
			mutate: func(in *Input) { in.Entries = nil },
			want:   Unknown,
		},
		{
			name:   "read and not found",
			mutate: func(in *Input) { in.Entries[1].Object = nil },
			want:   Missing,
		},
		{
			name: "polled object",
			mutate: func(in *Input) {
				in.Entries[1].Live = false
				in.Entries[1].EvaluatedAt = captureTime.Add(-25 * time.Second)
			},
			want: Healthy,
			check: func(t *testing.T, r Result) {
				if r.Instance.Live || !r.Instance.EvaluatedAt.Equal(captureTime.Add(-25*time.Second)) {
					t.Errorf("instance %+v, want not live and the polled time", r.Instance)
				}
			},
		},
		{
			name:    "unset access counts as not readable",
			mutate:  func(in *Input) { in.Entries[0].Access = "" },
			want:    Healthy,
			partial: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := healthy()
			tt.mutate(&in)
			got := Evaluate(in)
			if got.Instance.State != tt.want || got.Instance.Partial != tt.partial {
				t.Fatalf("instance %+v, want %s partial=%v", got.Instance, tt.want, tt.partial)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}
