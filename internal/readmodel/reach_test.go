package readmodel

import (
	"errors"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// f1Pod is one of the podinfo Deployment's Pods in the F1 capture.
const f1Pod = "podinfo-podinfo-d9585d794-4lg6h"

// logGrant asks alice's checker for get pods/log on default/pod.
func (e *env) logGrant(t testing.TB, pod string) authz.Grant {
	t.Helper()
	g, err := e.caller.Check(t.Context(), alice, podLogRead("default", pod))
	if err != nil {
		t.Fatalf("grant get pods/log default/%s: %v", pod, err)
	}
	return g
}

func TestAPodOfAnInstanceIsReached(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	got, err := e.m.ReachPod(t.Context(), alice, e.logGrant(t, f1Pod), "default", f1Pod)
	if err != nil {
		t.Fatal(err)
	}
	want := PodReach{
		Owner: ObjectRef{Group: opmGroup, Version: opmVersion, Kind: "ModuleInstance", Namespace: "default", Name: "podinfo"},
		Via:   ObjectRef{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "default", Name: "podinfo-podinfo"},
	}
	if got != want {
		t.Fatalf("ReachPod = %+v, want %+v", got, want)
	}
}

func TestAPodNoReadableInventoryReachesIsRefused(t *testing.T) {
	objs := loadF1(t)
	stray := find(t, objs, "Pod", f1Pod).DeepCopy()
	stray.SetName("stray")
	stray.SetUID("stray-uid")
	refs := stray.GetOwnerReferences()
	refs[0].UID = "no-such-owner"
	stray.SetOwnerReferences(refs)
	unlabeled := find(t, objs, "Pod", f1Pod).DeepCopy()
	unlabeled.SetName("unlabeled")
	unlabeled.SetUID("unlabeled-uid")
	unlabeled.SetLabels(nil)

	denyDeployments := func(_ string, ra authorizationv1.ResourceAttributes) bool {
		return ra.Resource != "deployments"
	}
	tests := []struct {
		name   string
		pod    string
		caller rule
	}{
		{"stray Pod", "stray", allowAll},
		{"unlabeled Pod", "unlabeled", allowAll},
		{"missing Pod", "no-such-pod", allowAll},
		{"Deployment forbidden", f1Pod, denyDeployments},
		{"Pods not listable", f1Pod, denyResources("pods")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, append(loadF1(t), stray, unlabeled), func(who string, ra authorizationv1.ResourceAttributes) bool {
				// The log read itself is always allowed: the refusal must come
				// from the reach, not the grant.
				return ra.Subresource == "log" || tc.caller(who, ra)
			}, allowAll)
			_, err := e.m.ReachPod(t.Context(), alice, e.logGrant(t, tc.pod), "default", tc.pod)
			if !errors.Is(err, ErrNotReachable) {
				t.Fatalf("ReachPod = %v, want ErrNotReachable", err)
			}
		})
	}
}

func TestReachPodReadsNothingWithoutTheLogGrant(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	before := len(clusterReads(e.client))
	for name, g := range map[string]authz.Grant{
		"zero grant":      {},
		"get pod grant":   e.grant(t, "get", pods, "default", f1Pod),
		"other pod's log": e.logGrant(t, "other"),
	} {
		reviews := e.reviews.Count()
		if _, err := e.m.ReachPod(t.Context(), alice, g, "default", f1Pod); !errors.Is(err, ErrNotCovered) {
			t.Errorf("%s: ReachPod = %v, want ErrNotCovered", name, err)
		}
		if n := e.reviews.Count() - reviews; n != 0 {
			t.Errorf("%s: %d reviews sent", name, n)
		}
	}
	if after := len(clusterReads(e.client)); after != before {
		t.Errorf("ReachPod read the cluster without a grant: %v", clusterReads(e.client)[before:])
	}
}

func TestReachPodSaysWhenItCannotRead(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	g := e.logGrant(t, f1Pod)
	e.reviews.SetFail(true)
	if _, err := e.m.ReachPod(t.Context(), alice, g, "default", f1Pod); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ReachPod with failing reviews = %v, want ErrUnavailable", err)
	}
}
