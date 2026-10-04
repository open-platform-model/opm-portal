package readmodel

import (
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// heapInUse returns the live heap after a full collection.
func heapInUse() uint64 {
	runtime.GC()
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapAlloc
}

func timed(fn func()) time.Duration {
	start := time.Now()
	fn()
	return time.Since(start)
}

// TestMeasureF1 records the read model's own cost on the F1 set, through
// the fake cluster: no network, so the numbers are the portal's work. Run
// with -v to see them; design.md records them.
func TestMeasureF1(t *testing.T) {
	objs := loadF1(t)
	trackerBase := heapInUse()
	trackerOnly := newDynamic(objs...)
	trackerCost := int64(heapInUse()) - int64(trackerBase)
	runtime.KeepAlive(trackerOnly)

	base := heapInUse()
	var e *env
	start := timed(func() { e = newEnv(t, objs, allowAll, allowAll) })
	ctx := t.Context()
	getCM := e.grant(t, "get", moduleInstances, "cert-manager", "cert-manager")
	listAll := e.grant(t, "list", moduleInstances, "", "")

	readsBefore := len(clusterReads(e.client))
	cold := timed(func() {
		if _, err := e.m.Instance(ctx, alice, getCM, "cert-manager", "cert-manager"); err != nil {
			t.Fatal(err)
		}
	})
	coldReads := len(clusterReads(e.client)) - readsBefore
	const n = 200
	warm := timed(func() {
		for range n {
			if _, err := e.m.Instance(ctx, alice, getCM, "cert-manager", "cert-manager"); err != nil {
				t.Fatal(err)
			}
		}
	}) / n
	listCold := timed(func() {
		if _, err := e.m.ListInstances(ctx, alice, listAll, ""); err != nil {
			t.Fatal(err)
		}
	})
	listWarm := timed(func() {
		for range n {
			if _, err := e.m.ListInstances(ctx, alice, listAll, ""); err != nil {
				t.Fatal(err)
			}
		}
	}) / n
	held := int64(heapInUse()) - int64(base) - trackerCost
	runtime.KeepAlive(e)

	t.Logf("F1 (fake cluster, no network): Start %v", start)
	t.Logf("Instance cert-manager (42 entries): cold %v with %d cluster requests, warm %v", cold, coldReads, warm)
	t.Logf("ListInstances (5 instances): first after cert-manager %v, warm %v", listCold, listWarm)
	t.Logf("heap held by the warm model: %.1f MiB (fake cluster's own copy, %.1f MiB, subtracted)",
		float64(held)/(1<<20), float64(trackerCost)/(1<<20))
}

// countingTransport counts the requests a client makes, split into access
// reviews and reads.
type countingTransport struct {
	next    http.RoundTripper
	reviews atomic.Int64
	reads   atomic.Int64
}

func (c *countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if strings.Contains(r.URL.Path, "accessreviews") || strings.Contains(r.URL.Path, "selfsubjectreviews") {
		c.reviews.Add(1)
	} else {
		c.reads.Add(1)
	}
	return c.next.RoundTrip(r)
}

func (c *countingTransport) snapshot() (reviews, reads int64) {
	return c.reviews.Load(), c.reads.Load()
}

// liveModel builds a Model against the cluster the kubeconfig names, as the
// kubeconfig's own identity, the way local mode will.
func liveModel(t *testing.T, kubeconfig string, tune bool) (*Model, *countingTransport, authz.Identity, *authz.Checker) {
	t.Helper()
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("loading kubeconfig: %v", err)
	}
	if tune {
		TuneConfig(cfg)
	}
	counter := &countingTransport{}
	cfg.Wrap(func(rt http.RoundTripper) http.RoundTripper { counter.next = rt; return counter })
	cs := kubernetes.NewForConfigOrDie(cfg)
	ssr, err := cs.AuthenticationV1().SelfSubjectReviews().Create(t.Context(), &authenticationv1.SelfSubjectReview{}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("self subject review: %v", err)
	}
	ui := ssr.Status.UserInfo
	who := authz.Identity{Username: ui.Username, UID: ui.UID, Groups: ui.Groups}
	checker, err := authz.NewLocal(cs.AuthorizationV1().SelfSubjectAccessReviews(), who, authz.Options{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := New(Config{
		Dynamic:    dynamic.NewForConfigOrDie(cfg),
		Discovery:  discovery.NewDiscoveryClientForConfigOrDie(cfg),
		Authorizer: checker,
		Reader:     who,
	})
	if err != nil {
		t.Fatal(err)
	}
	return m, counter, who, checker
}

// TestMeasureLive records the same numbers against a live cluster, over the
// network. It runs only when OPM_PORTAL_MEASURE_KUBECONFIG names the
// kubeconfig of the e2e fixture cluster (task e2e:up).
func TestMeasureLive(t *testing.T) {
	kubeconfig := os.Getenv("OPM_PORTAL_MEASURE_KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("OPM_PORTAL_MEASURE_KUBECONFIG not set")
	}
	for _, tune := range []bool{false, true} {
		name := "client-go defaults (QPS 5, burst 10)"
		if tune {
			name = "TuneConfig (QPS 50, burst 100)"
		}
		t.Run(name, func(t *testing.T) { measureLive(t, kubeconfig, tune) })
	}
}

func measureLive(t *testing.T, kubeconfig string, tune bool) {
	ctx := t.Context()
	m, counter, who, checker := liveModel(t, kubeconfig, tune)
	grant := func(verb string, ns, name string) authz.Grant {
		g, err := checker.Check(ctx, who, authz.Attributes{Verb: verb, Resource: moduleInstances, Namespace: ns, Name: name})
		if err != nil {
			t.Fatalf("grant: %v", err)
		}
		return g
	}
	start := timed(func() {
		if err := m.Start(ctx); err != nil {
			t.Fatal(err)
		}
	})
	t.Cleanup(m.Stop)
	getCM, listAll := grant("get", "cert-manager", "cert-manager"), grant("list", "", "")
	r0, q0 := counter.snapshot()
	instance := func() {
		if _, err := m.Instance(ctx, who, getCM, "cert-manager", "cert-manager"); err != nil {
			t.Fatal(err)
		}
	}
	cold := timed(instance)
	r1, q1 := counter.snapshot()
	const n = 20
	warm := timed(func() {
		for range n {
			instance()
		}
	}) / n
	r2, q2 := counter.snapshot()
	listWarm := timed(func() {
		if _, err := m.ListInstances(ctx, who, listAll, ""); err != nil {
			t.Fatal(err)
		}
	})
	t.Logf("Start %v", start)
	t.Logf("Instance cert-manager cold %v (%d reads, %d access reviews), warm %v (%d reads, %d reviews over %d calls)",
		cold, q1-q0, r1-r0, warm, q2-q1, r2-r1, n)
	t.Logf("ListInstances after the cert-manager read: %v", listWarm)
}
