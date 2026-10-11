package readmodel

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	k8stesting "k8s.io/client-go/testing"

	"github.com/open-platform-model/opm-portal/internal/health"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

var (
	ociRepositoriesGVR = schema.GroupVersionResource{Group: readmodeltest.FluxGroup, Version: "v1", Resource: "ocirepositories"}
	configMapsGVR      = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	secretsGVR         = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
)

// fluxEnv is a Model over F1 plus the constructed sources, on a cluster
// that serves the Flux kinds.
func fluxEnv(t testing.TB, callerRule, readerRule rule) *env {
	t.Helper()
	objs := readmodeltest.WithCrossNamespaceSource(t, readmodeltest.WithUnversionedSourcePackage(t, readmodeltest.WithSources(t, loadF1(t))))
	return newEnv(t, objs, callerRule, readerRule, func(c *Config) { c.Discovery = newDiscovery(readmodeltest.KindsWithFlux()...) })
}

// sourceReads returns the reads the cluster received for Flux sources.
func sourceReads(e *env) []string {
	var out []string
	for _, r := range clusterReads(e.client) {
		if strings.Contains(r, "ocirepositories") || strings.Contains(r, "gitrepositories") || strings.Contains(r, "buckets") {
			out = append(out, r)
		}
	}
	return out
}

// sourceReviews counts the reviews asked about anything in the Flux group
// or about core secrets, for the caller and the reader together.
func sourceReviews(e *env) int {
	n := 0
	asked := append(e.reviews.Asked(), e.readerR.Asked()...)
	for i := range asked {
		r := &asked[i]
		if r.Group == readmodeltest.FluxGroup || r.Resource == "secrets" || r.Resource == "configmaps" {
			n++
		}
	}
	return n
}

func ociRef(ns, name string) ObjectRef {
	return ObjectRef{Group: readmodeltest.FluxGroup, Version: "v1", Kind: "OCIRepository", Namespace: ns, Name: name}
}

// TestSourceReadsTheStates reads each constructed source as the caller and
// holds the state to what the cluster says (portal:D20).
func TestSourceReadsTheStates(t *testing.T) {
	e := fluxEnv(t, allowAll, allowAll)
	kind, err := e.m.ResolveKind(readmodeltest.FluxGroup, "OCIRepository")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		want SourceState
	}{
		{readmodeltest.SourceReady, SourceState{
			Access: health.AccessOK, Readiness: SourceReady, Reason: "Succeeded",
			Message:  "stored artifact for digest '" + readmodeltest.SourceRevision + "'",
			Revision: readmodeltest.SourceRevision,
		}},
		{readmodeltest.SourceNotReady, SourceState{
			Access: health.AccessOK, Readiness: SourceNotReady,
			Reason: readmodeltest.SourceNotReadyReason, Message: readmodeltest.SourceNotReadyMessage,
		}},
		{readmodeltest.SourceNoArtifact, SourceState{
			Access: health.AccessOK, Readiness: SourceNoArtifact, Reason: "Succeeded",
			Message: "stored artifact for digest '" + readmodeltest.SourceRevision + "'",
		}},
		{"missing", SourceState{Access: health.AccessOK, Readiness: SourceNotFound}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := e.grant(t, "get", kind.Resource, readmodeltest.SourceNamespace, tc.name)
			got, err := e.m.Source(t.Context(), alice, g, kind, ociRef(readmodeltest.SourceNamespace, tc.name))
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("state = %+v\nwant    %+v", got, tc.want)
			}
		})
	}
}

// TestSourceNeverServesTheArtifactURL: the state holds no field the artifact
// URL could be in.
func TestSourceNeverServesTheArtifactURL(t *testing.T) {
	e := fluxEnv(t, allowAll, allowAll)
	kind, _ := e.m.ResolveKind(readmodeltest.FluxGroup, "OCIRepository")
	g := e.grant(t, "get", kind.Resource, readmodeltest.SourceNamespace, readmodeltest.SourceReady)
	got, err := e.m.Source(t.Context(), alice, g, kind, ociRef(readmodeltest.SourceNamespace, readmodeltest.SourceReady))
	if err != nil {
		t.Fatal(err)
	}
	if all := strings.Join([]string{string(got.Access), string(got.Readiness), got.Reason, got.Message, got.Revision}, " "); strings.Contains(all, "source-controller.flux-system") {
		t.Errorf("state carries the artifact URL: %q", all)
	}
}

// TestSourceChoosesTheGroupFromTheKind: the controller ignores
// sourceRef.apiVersion, so a package whose sourceRef names none still reads
// its source in source.toolkit.fluxcd.io.
func TestSourceChoosesTheGroupFromTheKind(t *testing.T) {
	e := fluxEnv(t, allowAll, allowAll)
	pg := e.grant(t, "get", modulePackages, "pkg", readmodeltest.UnversionedPackage)
	d, err := e.m.Package(t.Context(), alice, pg, "pkg", readmodeltest.UnversionedPackage)
	if err != nil {
		t.Fatal(err)
	}
	if d.Source.APIVersion != "" {
		t.Fatalf("fixture has apiVersion %q", d.Source.APIVersion)
	}
	kind, err := e.m.ResolveKind(readmodeltest.FluxGroup, d.Source.Kind)
	if err != nil {
		t.Fatal(err)
	}
	ref := ObjectRef{Kind: d.Source.Kind, Namespace: "pkg", Name: d.Source.Name}
	g := e.grant(t, "get", kind.Resource, ref.Namespace, ref.Name)
	got, err := e.m.Source(t.Context(), alice, g, kind, ref)
	if err != nil || got.Readiness != SourceReady {
		t.Errorf("state = %+v, %v; want ready", got, err)
	}
}

// TestSourceReaderDenied: the reader may not get the source, so it is not
// readable by the portal, and the cluster receives no get.
func TestSourceReaderDenied(t *testing.T) {
	e := fluxEnv(t, allowAll, denyResources("ocirepositories"))
	kind, _ := e.m.ResolveKind(readmodeltest.FluxGroup, "OCIRepository")
	g := e.grant(t, "get", kind.Resource, "pkg", readmodeltest.SourceReady)
	got, err := e.m.Source(t.Context(), alice, g, kind, ociRef("pkg", readmodeltest.SourceReady))
	if err != nil {
		t.Fatal(err)
	}
	if got.Access != health.AccessNotReadable || got.Readiness != "" {
		t.Errorf("state = %+v, want notReadable with no readiness", got)
	}
	if reads := sourceReads(e); len(reads) != 0 {
		t.Errorf("the cluster received %v", reads)
	}
}

// TestSourceKindsOutsideTheThree: a kind the controller does not read is
// answered with no review and no read, and a core Secret is refused before
// any review (portal:D8:R1).
func TestSourceKindsOutsideTheThree(t *testing.T) {
	e := fluxEnv(t, allowAll, allowAll)

	cm := ObjectRef{Kind: "ConfigMap", Namespace: "pkg", Name: "x"}
	g := e.grant(t, "get", configMapsGVR, "pkg", "x")
	before := sourceReviews(e)
	readsBefore := len(clusterReads(e.client))
	got, err := e.m.Source(t.Context(), alice, g, ResolvedKind{Resource: configMapsGVR, Namespaced: true}, cm)
	if err != nil || got.Readiness != SourceUnsupported {
		t.Errorf("ConfigMap: %+v, %v; want unsupportedKind", got, err)
	}
	// A kind the portal never reads under a Flux name.
	got, err = e.m.Source(t.Context(), alice, g, ResolvedKind{Resource: ociRepositoriesGVR, Namespaced: true}, ObjectRef{Kind: "Kustomization", Namespace: "pkg", Name: "x"})
	if err != nil || got.Readiness != SourceUnsupported {
		t.Errorf("Kustomization: %+v, %v; want unsupportedKind", got, err)
	}

	// The authorizer refuses a Secret grant itself; the Model refuses first.
	_, err = e.m.Source(t.Context(), alice, g, ResolvedKind{Resource: secretsGVR, Namespaced: true}, ObjectRef{Kind: "Secret", Namespace: "pkg", Name: "x"})
	if !errors.Is(err, ErrWithheld) {
		t.Errorf("Secret: err = %v, want ErrWithheld", err)
	}
	if n := sourceReviews(e); n != before {
		t.Errorf("a review was asked for the Secret or the kinds outside the three")
	}
	if len(clusterReads(e.client)) != readsBefore {
		t.Errorf("the cluster received a read: %v", clusterReads(e.client)[readsBefore:])
	}
}

// TestSourceInAnotherNamespace (AG-A2): a grant that covers get in pkg-a
// does not cover the source in pkg-b, and the cluster receives no get for it.
func TestSourceInAnotherNamespace(t *testing.T) {
	e := fluxEnv(t, allowAll, allowAll)
	kind, _ := e.m.ResolveKind(readmodeltest.FluxGroup, "OCIRepository")
	pkgA := e.grant(t, "get", kind.Resource, readmodeltest.CrossPackageNamespace, readmodeltest.SourceReady)
	_, err := e.m.Source(t.Context(), alice, pkgA, kind, ociRef(readmodeltest.CrossSourceNamespace, readmodeltest.SourceReady))
	if !errors.Is(err, ErrNotCovered) {
		t.Fatalf("err = %v, want ErrNotCovered", err)
	}
	if reads := sourceReads(e); len(reads) != 0 {
		t.Errorf("the cluster received %v", reads)
	}
	pkgB := e.grant(t, "get", kind.Resource, readmodeltest.CrossSourceNamespace, readmodeltest.SourceReady)
	got, err := e.m.Source(t.Context(), alice, pkgB, kind, ociRef(readmodeltest.CrossSourceNamespace, readmodeltest.SourceReady))
	if err != nil || got.Readiness != SourceReady {
		t.Errorf("with a grant for pkg-b: %+v, %v; want ready", got, err)
	}
	if reads := sourceReads(e); len(reads) != 1 || reads[0] != "get ocirepositories pkg-b/podinfo-release" {
		t.Errorf("reads = %v, want one get in pkg-b", reads)
	}
}

// TestSourceFailedGetIsNotReadable: a read that fails is a state, not an
// error.
func TestSourceFailedGetIsNotReadable(t *testing.T) {
	e := fluxEnv(t, allowAll, allowAll)
	kind, _ := e.m.ResolveKind(readmodeltest.FluxGroup, "OCIRepository")
	e.client.PrependReactor("get", "ocirepositories", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("connection refused")
	})
	g := e.grant(t, "get", kind.Resource, "pkg", readmodeltest.SourceReady)
	got, err := e.m.Source(t.Context(), alice, g, kind, ociRef("pkg", readmodeltest.SourceReady))
	if err != nil || got.Access != health.AccessNotReadable {
		t.Errorf("state = %+v, %v; want notReadable", got, err)
	}
}

// lockedDiscovery guards a fake discovery's resources with a mutex, so a
// test can change what the cluster serves while the Model's timer reads it.
type lockedDiscovery struct {
	discovery.DiscoveryInterface
	mu sync.Mutex
	d  discovery.DiscoveryInterface
	fn func(kinds ...testKind)
}

func (l *lockedDiscovery) set(kinds ...testKind) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fn(kinds...)
}

func (l *lockedDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.d.ServerGroups()
}

func (l *lockedDiscovery) ServerResourcesForGroupVersion(gv string) (*metav1.APIResourceList, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.d.ServerResourcesForGroupVersion(gv)
}

func (l *lockedDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.d.ServerGroupsAndResources()
}

// TestSourceKindAddedAfterStart: no inventory reaches a Flux source, so
// discovery would never learn a Flux installed after the Model started. The
// timer finds the kind, and a request never refreshes discovery.
func TestSourceKindAddedAfterStart(t *testing.T) {
	fake := newDiscovery(clusterKinds...)
	ld := &lockedDiscovery{DiscoveryInterface: fake, d: fake, fn: func(kinds ...testKind) { setDiscovery(fake, kinds...) }}
	objs := readmodeltest.WithSources(t, loadF1(t))
	e := newEnv(t, objs, allowAll, allowAll, func(c *Config) {
		c.Discovery = ld
		c.SourceKindsInterval = 20 * time.Millisecond
	})
	if _, err := e.m.ResolveKind(readmodeltest.FluxGroup, "OCIRepository"); err == nil {
		t.Fatal("the kind is served before Flux is installed")
	}

	ld.set(readmodeltest.KindsWithFlux()...)
	// A request does not refresh discovery: the lookup keeps failing until the
	// timer has run, and the timer reads discovery once for its own tick.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if k, err := e.m.ResolveKind(readmodeltest.FluxGroup, "OCIRepository"); err == nil {
			if k.Resource != ociRepositoriesGVR {
				t.Fatalf("resolved to %+v", k.Resource)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the kind was not served after the timer ran")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestSourceKindRecheckIsNotOnARequest: with the timer far away, a lookup of
// a kind gained after start still fails, however often it is asked.
func TestSourceKindRecheckIsNotOnARequest(t *testing.T) {
	fake := newDiscovery(clusterKinds...)
	e := newEnv(t, loadF1(t), allowAll, allowAll, func(c *Config) { c.Discovery = fake })
	if _, err := e.m.ResolveKind(readmodeltest.FluxGroup, "OCIRepository"); err == nil {
		t.Fatal("the kind is served before Flux is installed")
	}
	setDiscovery(fake, readmodeltest.KindsWithFlux()...)
	for range 3 {
		if _, err := e.m.ResolveKind(readmodeltest.FluxGroup, "OCIRepository"); err == nil {
			t.Fatal("a request refreshed discovery")
		}
	}
	e.m.recheckSourceKinds()
	if _, err := e.m.ResolveKind(readmodeltest.FluxGroup, "OCIRepository"); err != nil {
		t.Errorf("after the recheck: %v", err)
	}
}
