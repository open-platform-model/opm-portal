package readmodel

import (
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// recorder collects the changes a listener is told about.
type recorder struct {
	mu  sync.Mutex
	got []Change
}

func (r *recorder) add(c Change) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, c)
}

// wait returns once want was reported, failing the test after a timeout.
func (r *recorder) wait(t *testing.T, want Change) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		for _, c := range r.got {
			if c == want {
				r.mu.Unlock()
				return
			}
		}
		r.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	t.Fatalf("no change %+v among %+v", want, r.got)
}

func (r *recorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = nil
}

var deployments = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

func TestChangesNameTheOwningInstance(t *testing.T) {
	e := newUnstartedEnv(t, loadF1(t), allowAll, allowAll)
	rec := &recorder{}
	remove := e.m.OnChange(rec.add)
	defer remove()
	if err := e.m.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	podinfo := Change{Kind: ChangeInstance, Namespace: "default", Name: "podinfo"}
	// The initial list reports every OPM object, and a registration also
	// reports the Platform.
	rec.wait(t, podinfo)
	rec.wait(t, Change{Kind: ChangePlatform, Name: "cluster"})

	// Reading the instance starts the Deployment informer (tier 2).
	e.instance(t, "default", "podinfo")
	rec.reset()
	dep, err := e.client.Resource(deployments).Namespace("default").Get(t.Context(), "podinfo-podinfo", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	dep.SetAnnotations(map[string]string{"touched": "yes"})
	if _, err := e.client.Resource(deployments).Namespace("default").Update(t.Context(), dep, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	rec.wait(t, podinfo)

	// A runtime child names its instance by the name label.
	release, err := e.m.HoldChildren(t.Context(), alice, e.grant(t, "watch", pods, "default", ""), "default")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	rec.reset()
	pod := find(t, loadF1(t), "Pod", "podinfo-podinfo-d9585d794-4lg6h")
	pod.SetAnnotations(map[string]string{"touched": "yes"})
	if _, err := e.client.Resource(pods).Namespace("default").Update(t.Context(), pod, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	rec.wait(t, podinfo)

	rec.reset()
	if err := e.client.Resource(moduleInstances).Namespace("default").Delete(t.Context(), "podinfo", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	deleted := podinfo
	deleted.Deleted = true
	rec.wait(t, deleted)
}

func TestRemovedListenerHearsNothing(t *testing.T) {
	e := newUnstartedEnv(t, loadF1(t), allowAll, allowAll)
	rec := &recorder{}
	e.m.OnChange(rec.add)()
	if err := e.m.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.got) != 0 {
		t.Errorf("a removed listener heard %+v", rec.got)
	}
}

func TestResolveKind(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	k, err := e.m.ResolveKind("apps", "Deployment")
	if err != nil || k.Resource != deployments || !k.Namespaced {
		t.Errorf("ResolveKind(apps, Deployment) = %+v, %v", k, err)
	}
	if k, err := e.m.ResolveKind("", "Namespace"); err != nil || k.Namespaced {
		t.Errorf("ResolveKind(Namespace) = %+v, %v", k, err)
	}
	if _, err := e.m.ResolveKind("example.com", "Widget"); err == nil {
		t.Error("an unknown kind resolved")
	}
}
