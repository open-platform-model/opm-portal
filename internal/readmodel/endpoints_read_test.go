package readmodel

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

// lastSliceSelector returns the label selector of the last list of
// EndpointSlices the fake cluster served.
func lastSliceSelector(t testing.TB, c *dynfake.FakeDynamicClient) string {
	t.Helper()
	out := ""
	for _, a := range c.Actions() {
		if l, ok := a.(k8stesting.ListAction); ok && a.GetResource().Resource == "endpointslices" {
			out = l.GetListRestrictions().Labels.String()
		}
	}
	if out == "" {
		t.Fatal("no list of endpointslices with a label selector")
	}
	return out
}

// sliceLists counts the lists of EndpointSlices the fake cluster served.
func sliceLists(c *dynfake.FakeDynamicClient) int {
	n := 0
	for _, a := range clusterReads(c) {
		if strings.HasPrefix(a, "list endpointslices ") {
			n++
		}
	}
	return n
}

// clock is a Config.Now a test moves by hand.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func newClock() *clock { return &clock{now: time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)} }

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// withSlices is F1 plus the recorded EndpointSlice of the podinfo Service.
func withSlices(t testing.TB, opts ...readmodeltest.SliceOption) []*unstructured.Unstructured {
	t.Helper()
	return readmodeltest.WithEndpointSlices(t, loadF1(t), opts...)
}

// podinfoService is F1's podinfo Service.
func podinfoService(t testing.TB, objs []*unstructured.Unstructured) *unstructured.Unstructured {
	t.Helper()
	return find(t, objs, "Service", readmodeltest.PodinfoServiceName)
}

// answer is a reactor that answers every list with obj and err.
func answer(obj runtime.Object, err error) func(k8stesting.Action) (bool, runtime.Object, error) {
	return func(k8stesting.Action) (bool, runtime.Object, error) { return true, obj, err }
}

// sliceListFailures are the ways a list of EndpointSlices can fail, as
// reactors for the fake cluster.
func sliceListFailures() map[string]func(k8stesting.Action) (bool, runtime.Object, error) {
	nextPage := &unstructured.UnstructuredList{Object: map[string]any{
		"apiVersion": "discovery.k8s.io/v1", "kind": "EndpointSliceList",
		"metadata": map[string]any{"continue": "next-page"},
	}}
	return map[string]func(k8stesting.Action) (bool, runtime.Object, error){
		"an error":         answer(nil, errors.New("connection refused")),
		"a timeout":        answer(nil, context.DeadlineExceeded),
		"a continue token": answer(nextPage, nil),
	}
}

// TestReadyEndpointsCachesPerService: one list serves every read inside the
// TTL, the TTL ends it, another Service lists for itself and finds nothing
// of the first one's slice, and the list carries the Service's own label selector.
func TestReadyEndpointsCachesPerService(t *testing.T) {
	objs := withSlices(t)
	clk := newClock()
	e := newEnv(t, objs, allowAll, allowAll, func(c *Config) { c.Now = clk.Now })
	svc := podinfoService(t, objs)

	for range 2 {
		ready, found, err := e.m.readyEndpoints(t.Context(), svc)
		if err != nil || !found || ready != 2 {
			t.Fatalf("readyEndpoints = (%d, %t, %v), want (2, true, nil)", ready, found, err)
		}
	}
	if n := sliceLists(e.client); n != 1 {
		t.Fatalf("lists inside the TTL = %d, want 1", n)
	}

	clk.Advance(e.m.cfg.ChildrenTTL + time.Second)
	if _, _, err := e.m.readyEndpoints(t.Context(), svc); err != nil {
		t.Fatal(err)
	}
	if n := sliceLists(e.client); n != 2 {
		t.Fatalf("lists after the TTL = %d, want 2", n)
	}

	other := svc.DeepCopy()
	other.SetName("other")
	other.SetUID("other-uid")
	ready, found, err := e.m.readyEndpoints(t.Context(), other)
	if err != nil || found || ready != 0 {
		t.Fatalf("readyEndpoints(other) = (%d, %t, %v), want (0, false, nil)", ready, found, err)
	}
	if n := sliceLists(e.client); n != 3 {
		t.Errorf("lists after reading another Service = %d, want 3 (one for each Service)", n)
	}
	if got, want := lastSliceSelector(t, e.client), "kubernetes.io/service-name=other"; got != want {
		t.Errorf("label selector = %q, want %q", got, want)
	}
	if _, _, err := e.m.readyEndpoints(t.Context(), svc); err != nil {
		t.Fatal(err)
	}
	if n := sliceLists(e.client); n != 3 {
		t.Errorf("lists after reading the first Service again = %d, want 3 (its entry is fresh)", n)
	}
}

// TestReadyEndpointsDoesNotServeADeletedServicesCount: a Service made again
// under the same name has another uid and reads again, so it does not inherit
// the count of the one before.
func TestReadyEndpointsDoesNotServeADeletedServicesCount(t *testing.T) {
	objs := withSlices(t)
	e := newEnv(t, objs, allowAll, allowAll)
	svc := podinfoService(t, objs)
	if _, _, err := e.m.readyEndpoints(t.Context(), svc); err != nil {
		t.Fatal(err)
	}
	again := svc.DeepCopy()
	again.SetUID("a-new-uid")
	// The recorded slice is owned by the old uid, so the new Service owns no
	// slice: nothing is kept.
	ready, found, err := e.m.readyEndpoints(t.Context(), again)
	if err != nil || found || ready != 0 {
		t.Fatalf("readyEndpoints(new uid) = (%d, %t, %v), want (0, false, nil)", ready, found, err)
	}
	if n := sliceLists(e.client); n != 2 {
		t.Errorf("lists = %d, want 2 (one for each uid)", n)
	}
}

// TestReadyEndpointsFailureIsNotCached: an error, a timeout and a list with
// a continue token each return an error and are not cached, so the next read
// lists again and counts.
func TestReadyEndpointsFailureIsNotCached(t *testing.T) {
	failures := sliceListFailures()
	for name, fail := range failures {
		t.Run(name, func(t *testing.T) {
			objs := withSlices(t)
			e := newEnv(t, objs, allowAll, allowAll)
			svc := podinfoService(t, objs)

			e.client.PrependReactor("list", "endpointslices", fail)
			if ready, found, err := e.m.readyEndpoints(t.Context(), svc); err == nil || ready != 0 || found {
				t.Fatalf("readyEndpoints = (%d, %t, %v), want an error and no count", ready, found, err)
			}
			e.client.ReactionChain = e.client.ReactionChain[1:] // the cluster answers again
			ready, found, err := e.m.readyEndpoints(t.Context(), svc)
			if err != nil || !found || ready != 2 {
				t.Fatalf("readyEndpoints after the failure = (%d, %t, %v), want (2, true, nil)", ready, found, err)
			}
			if n := sliceLists(e.client); n != 2 {
				t.Errorf("lists = %d, want 2 (the failure was not cached)", n)
			}
		})
	}
}

// TestSweepDropsExpiredEndpointCounts: the janitor's sweep forgets a count
// once its TTL has passed and keeps a fresh one.
func TestSweepDropsExpiredEndpointCounts(t *testing.T) {
	objs := withSlices(t)
	clk := newClock()
	e := newEnv(t, objs, allowAll, allowAll, func(c *Config) { c.Now = clk.Now })
	if _, _, err := e.m.readyEndpoints(t.Context(), podinfoService(t, objs)); err != nil {
		t.Fatal(err)
	}
	size := func() int {
		e.m.mu.Lock()
		defer e.m.mu.Unlock()
		return len(e.m.endpointList)
	}
	e.m.sweep()
	if size() != 1 {
		t.Fatalf("entries after a sweep inside the TTL = %d, want 1", size())
	}
	clk.Advance(e.m.cfg.ChildrenTTL + time.Second)
	e.m.sweep()
	if size() != 0 {
		t.Errorf("entries after a sweep past the TTL = %d, want 0", size())
	}
}
