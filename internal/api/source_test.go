package api

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

const (
	fluxPackage      = base + "/packages/pkg/podinfo"
	fluxPackageGraph = fluxPackage + "/graph"
	crossPackage     = base + "/packages/" + readmodeltest.CrossPackageNamespace + "/" + readmodeltest.CrossPackage
)

// fluxObjs is F1 plus the constructed sources and packages (portal:D20).
func fluxObjs(t testing.TB) []*unstructured.Unstructured {
	t.Helper()
	return readmodeltest.WithCrossNamespaceSource(t, readmodeltest.WithUnversionedSourcePackage(t, readmodeltest.WithSources(t, loadF1(t))))
}

// fluxServed makes the fake cluster serve the Flux source kinds.
func fluxServed(t testing.TB) *env {
	t.Helper()
	return fluxEnv(t, fluxObjs(t), readmodeltest.AllowAll)
}

func fluxEnv(t testing.TB, objs []*unstructured.Unstructured, rule readmodeltest.Rule, opts ...func(*Config)) *env {
	t.Helper()
	return newEnvWithDiscovery(t, objs, rule, readmodeltest.AllowAll, readmodeltest.Discovery(readmodeltest.KindsWithFlux()...), opts...)
}

// sourceNode returns the source node of a package graph.
func sourceNode(t testing.TB, g v1.Graph) v1.GraphNode {
	t.Helper()
	for i := range g.Nodes {
		if g.Nodes[i].Kind == "source" {
			return g.Nodes[i]
		}
	}
	t.Fatal("no source node")
	return v1.GraphNode{}
}

// sourceGets returns the get actions the cluster received for Flux sources.
func sourceGets(e *env) []string {
	var out []string
	for _, a := range readmodeltest.Actions(e.client) {
		if strings.HasPrefix(a, "get ") && (strings.Contains(a, "ocirepositories") || strings.Contains(a, "gitrepositories") || strings.Contains(a, "buckets")) {
			out = append(out, a)
		}
	}
	return out
}

// fluxReviews returns the access reviews asked about the Flux group.
func fluxReviews(e *env) []authorizationv1.ResourceAttributes {
	var out []authorizationv1.ResourceAttributes
	asked := e.reviews.Asked()
	for i := range asked {
		if asked[i].Group == readmodeltest.FluxGroup {
			out = append(out, asked[i])
		}
	}
	return out
}

// naming returns objs with ModulePackage pkg/podinfo's sourceRef set.
func naming(t testing.TB, objs []*unstructured.Unstructured, kind, name string) []*unstructured.Unstructured {
	t.Helper()
	pkg := readmodeltest.Find(t, objs, "ModulePackage", "podinfo")
	for field, v := range map[string]string{"kind": kind, "name": name} {
		if err := unstructured.SetNestedField(pkg.Object, v, "spec", "sourceRef", field); err != nil {
			t.Fatal(err)
		}
	}
	return objs
}

// TestPackageSourceKindNotServed: F1 runs no Flux, so its package's source
// kind is not served: the state says so, the source node is missing, and no
// review and no read is made for it.
func TestPackageSourceKindNotServed(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
	doc := decode[v1.Package](t, e.get(t, fluxPackage))
	if doc.SourceState == nil || *doc.SourceState != (v1.SourceState{Access: "ok", State: "kindNotServed"}) {
		t.Fatalf("sourceState = %+v, want ok and kindNotServed", doc.SourceState)
	}
	n := sourceNode(t, decode[v1.Graph](t, e.get(t, fluxPackageGraph)))
	if n.Access != "ok" || !n.Missing || n.Health != nil || n.SourceState == nil || n.SourceState.State != "kindNotServed" {
		t.Errorf("source node = %+v", n)
	}
	if rs := fluxReviews(e); len(rs) != 0 {
		t.Errorf("reviews for the Flux group: %v", rs)
	}
	if gets := sourceGets(e); len(gets) != 0 {
		t.Errorf("the cluster received %v", gets)
	}
}

// stateCase is one source and what the document and the node say about it.
type stateCase struct {
	name, kind, source string
	state              string
	health             string // "" for none
	missing            bool
	reason, message    string
}

// checkDocument holds the Package document's source state to tc.
func (tc stateCase) checkDocument(t *testing.T, doc v1.Package) {
	t.Helper()
	s := doc.SourceState
	if s == nil || s.Access != "ok" || s.State != tc.state {
		t.Fatalf("sourceState = %+v, want ok and %s", s, tc.state)
	}
	if tc.state == "notReady" && (s.Reason != tc.reason || s.Message != tc.message) {
		t.Errorf("reason %q message %q", s.Reason, s.Message)
	}
	if tc.state == "ready" && s.Revision != readmodeltest.SourceRevision {
		t.Errorf("revision = %q", s.Revision)
	}
	if tc.health != "" && doc.Health.State != "Unknown" {
		t.Errorf("package health = %q, want Unknown (the source is not counted)", doc.Health.State)
	}
}

// checkNode holds the source node to tc: its access, missing and health
// follow the state, and its state is the document's.
func (tc stateCase) checkNode(t *testing.T, n v1.GraphNode, doc v1.Package) {
	t.Helper()
	got := ""
	if n.Health != nil {
		got = n.Health.State
	}
	if n.Access != "ok" || n.Missing != tc.missing || got != tc.health {
		t.Errorf("node access %q missing %v health %q; want ok, %v, %q", n.Access, n.Missing, got, tc.missing, tc.health)
	}
	if n.SourceState == nil || *n.SourceState != *doc.SourceState {
		t.Errorf("node state %+v differs from the document's %+v", n.SourceState, doc.SourceState)
	}
	if tc.state == "notReady" && (n.Health.Reason != tc.reason || n.Health.Message != tc.message) {
		t.Errorf("node health reason %q message %q", n.Health.Reason, n.Health.Message)
	}
}

// TestPackageSourceStates reads each constructed source and holds the
// document and the source node to the table of the design (the node's
// health, access and missing follow the state).
func TestPackageSourceStates(t *testing.T) {
	for _, tc := range []stateCase{
		{"ready", "OCIRepository", readmodeltest.SourceReady, "ready", "Healthy", false, "", ""},
		{"not ready", "OCIRepository", readmodeltest.SourceNotReady, "notReady", "Degraded", false,
			readmodeltest.SourceNotReadyReason, readmodeltest.SourceNotReadyMessage},
		{"ready with no artifact", "OCIRepository", readmodeltest.SourceNoArtifact, "noArtifact", "Unknown", false, "", ""},
		{"not found", "OCIRepository", "no-such-source", "notFound", "", true, "", ""},
		{"a kind the cluster serves, absent", "GitRepository", "no-such-source", "notFound", "", true, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := fluxEnv(t, naming(t, fluxObjs(t), tc.kind, tc.source), readmodeltest.AllowAll)
			doc := decode[v1.Package](t, e.get(t, fluxPackage))
			tc.checkDocument(t, doc)
			tc.checkNode(t, sourceNode(t, decode[v1.Graph](t, e.get(t, fluxPackageGraph))), doc)
		})
	}
}

// TestPackageSourceNeverServesTheArtifactURL: no field of the document or the
// graph carries the artifact's in-cluster URL.
func TestPackageSourceNeverServesTheArtifactURL(t *testing.T) {
	e := fluxServed(t)
	for _, path := range []string{fluxPackage, fluxPackageGraph} {
		if body := e.get(t, path).body; strings.Contains(string(body), "source-controller.flux-system") {
			t.Errorf("GET %s serves the artifact URL", path)
		}
	}
}

// TestPackageSourceWithoutAPIVersion: the controller chooses the group from
// the kind alone, so a sourceRef with no apiVersion reads its source, and the
// source node names the group the portal read.
func TestPackageSourceWithoutAPIVersion(t *testing.T) {
	e := fluxServed(t)
	path := base + "/packages/pkg/" + readmodeltest.UnversionedPackage
	doc := decode[v1.Package](t, e.get(t, path))
	if doc.SourceState == nil || doc.SourceState.State != "ready" {
		t.Fatalf("sourceState = %+v, want ready", doc.SourceState)
	}
	n := sourceNode(t, decode[v1.Graph](t, e.get(t, path+"/graph")))
	if n.Ref == nil || n.Ref.Group != readmodeltest.FluxGroup || n.Ref.Version != "v1" {
		t.Errorf("source node ref = %+v, want group %s, version v1", n.Ref, readmodeltest.FluxGroup)
	}
}

// assertNothingAskedFor fails on a review or a get after the first n
// reviews that names a kind the controller does not read.
func assertNothingAskedFor(t *testing.T, e *env, n int, kind string) {
	t.Helper()
	asked := e.reviews.Asked()
	for i := n; i < len(asked); i++ {
		r := &asked[i]
		if strings.EqualFold(r.Resource, kind+"s") || r.Resource == "secrets" || r.Resource == "configmaps" || r.Group == readmodeltest.FluxGroup {
			t.Errorf("a review was asked for %+v", *r)
		}
	}
	for _, a := range readmodeltest.Actions(e.client) {
		if strings.HasPrefix(a, "get ") && (strings.Contains(a, "secrets") || strings.Contains(a, "configmaps") || strings.Contains(a, "helmcharts")) {
			t.Errorf("the cluster received %s", a)
		}
	}
}

// TestPackageSourceUnsupportedKind: a kind the controller does not read gets
// no review and no read, and its node has no health.
func TestPackageSourceUnsupportedKind(t *testing.T) {
	for _, kind := range []string{"HelmChart", "Secret", "ConfigMap"} {
		t.Run(kind, func(t *testing.T) {
			e := fluxEnv(t, naming(t, fluxObjs(t), kind, "x"), readmodeltest.AllowAll)
			before := len(e.reviews.Asked())
			doc := decode[v1.Package](t, e.get(t, fluxPackage))
			if doc.SourceState == nil || *doc.SourceState != (v1.SourceState{Access: "ok", State: "unsupportedKind"}) {
				t.Fatalf("sourceState = %+v", doc.SourceState)
			}
			n := sourceNode(t, decode[v1.Graph](t, e.get(t, fluxPackageGraph)))
			if n.Health != nil || n.Missing || n.Access != "ok" {
				t.Errorf("node = %+v", n)
			}
			assertNothingAskedFor(t, e, before, kind)
		})
	}
}

// TestPackageSourceForbidden (SR2): a caller allowed to get packages but not
// OCIRepositories sees the source locked, the review names the source, and
// the cluster receives no get for it.
func TestPackageSourceForbidden(t *testing.T) {
	e := fluxEnv(t, fluxObjs(t), readmodeltest.DenyResources("ocirepositories"))
	doc := decode[v1.Package](t, e.get(t, fluxPackage))
	if doc.SourceState == nil || *doc.SourceState != (v1.SourceState{Access: "forbidden"}) {
		t.Fatalf("sourceState = %+v, want forbidden with no state", doc.SourceState)
	}
	if doc.Reconcile.State == "" {
		t.Error("the package document lost its own state")
	}
	n := sourceNode(t, decode[v1.Graph](t, e.get(t, fluxPackageGraph)))
	if n.Access != "forbidden" || n.Missing || n.Health != nil {
		t.Errorf("node access %q missing %v health %v; want forbidden (locked), not missing, no health", n.Access, n.Missing, n.Health)
	}
	want := authorizationv1.ResourceAttributes{Verb: "get", Group: readmodeltest.FluxGroup, Version: "v1", Resource: "ocirepositories", Namespace: "pkg", Name: readmodeltest.SourceReady}
	if rs := fluxReviews(e); len(rs) == 0 || !slices.ContainsFunc(rs, func(r authorizationv1.ResourceAttributes) bool { return sameReview(r, want) }) {
		t.Errorf("reviews for the Flux group = %+v, want one like %+v", rs, want)
	}
	if gets := sourceGets(e); len(gets) != 0 {
		t.Errorf("the cluster received %v", gets)
	}
}

func sameReview(a, b authorizationv1.ResourceAttributes) bool {
	return a.Verb == b.Verb && a.Group == b.Group && a.Resource == b.Resource && a.Namespace == b.Namespace && a.Name == b.Name
}

// TestPackageSourceInAnotherNamespace (AG-A2): a caller allowed to get
// ModulePackages and OCIRepositories in pkg-a but not OCIRepositories in
// pkg-b reads the package with its source locked; the review names pkg-b and
// the source's name, and the cluster receives no get for it. A caller also
// allowed in pkg-b reads the source as ready.
func TestPackageSourceInAnotherNamespace(t *testing.T) {
	notB := func(_ string, ra authorizationv1.ResourceAttributes) bool {
		return ra.Resource != "ocirepositories" || ra.Namespace != readmodeltest.CrossSourceNamespace
	}
	e := fluxEnv(t, fluxObjs(t), notB)
	doc := decode[v1.Package](t, e.get(t, crossPackage))
	if doc.SourceState == nil || *doc.SourceState != (v1.SourceState{Access: "forbidden"}) {
		t.Fatalf("sourceState = %+v, want forbidden", doc.SourceState)
	}
	n := sourceNode(t, decode[v1.Graph](t, e.get(t, crossPackage+"/graph")))
	if n.Access != "forbidden" || n.Health != nil || n.Missing {
		t.Errorf("node = access %q health %v missing %v; want locked", n.Access, n.Health, n.Missing)
	}
	if n.Ref == nil || n.Ref.Namespace != readmodeltest.CrossSourceNamespace {
		t.Errorf("node ref = %+v, want namespace pkg-b", n.Ref)
	}
	assertReviewedInPackB(t, e)
	if gets := sourceGets(e); len(gets) != 0 {
		t.Errorf("the cluster received %v", gets)
	}

	allowed := fluxServed(t)
	doc = decode[v1.Package](t, allowed.get(t, crossPackage))
	if doc.SourceState == nil || doc.SourceState.State != "ready" {
		t.Errorf("with get in pkg-b: sourceState = %+v, want ready", doc.SourceState)
	}
	if gets := sourceGets(allowed); !slices.Contains(gets, "get ocirepositories pkg-b/"+readmodeltest.SourceReady) {
		t.Errorf("the cluster received %v, want a get in pkg-b", gets)
	}
}

// assertReviewedInPackB holds the source's access review to its group,
// resource, namespace and name, and never the package's namespace.
func assertReviewedInPackB(t *testing.T, e *env) {
	t.Helper()
	want := authorizationv1.ResourceAttributes{Verb: "get", Group: readmodeltest.FluxGroup, Resource: "ocirepositories",
		Namespace: readmodeltest.CrossSourceNamespace, Name: readmodeltest.SourceReady}
	rs := fluxReviews(e)
	for i := range rs {
		if rs[i].Namespace == readmodeltest.CrossPackageNamespace {
			t.Errorf("the source was reviewed in the package's namespace: %+v", rs[i])
		}
	}
	if !slices.ContainsFunc(rs, func(r authorizationv1.ResourceAttributes) bool { return sameReview(r, want) }) {
		t.Errorf("reviews for the Flux group = %+v, want one like %+v", rs, want)
	}
}

// TestPackageSourceReaderDenied: the portal's own reader may not get the
// source, so it is not readable by the portal, whoever the caller is.
func TestPackageSourceReaderDenied(t *testing.T) {
	e := newEnvWithDiscovery(t, fluxObjs(t), readmodeltest.AllowAll, readmodeltest.DenyResources("ocirepositories"),
		readmodeltest.Discovery(readmodeltest.KindsWithFlux()...))
	doc := decode[v1.Package](t, e.get(t, fluxPackage))
	if doc.SourceState == nil || *doc.SourceState != (v1.SourceState{Access: "notReadable"}) {
		t.Fatalf("sourceState = %+v, want notReadable", doc.SourceState)
	}
	n := sourceNode(t, decode[v1.Graph](t, e.get(t, fluxPackageGraph)))
	if n.Access != "notReadable" || n.Health != nil {
		t.Errorf("node = %+v", n)
	}
	if gets := sourceGets(e); len(gets) != 0 {
		t.Errorf("the cluster received %v", gets)
	}
}

// TestPackageSourceReviewUnavailable: a review that cannot be made is not
// readable, and the package still serves.
func TestPackageSourceReviewUnavailable(t *testing.T) {
	e := fluxServed(t)
	e.az.setFail(func(_ authz.Identity, a authz.Attributes) bool { return a.Resource.Resource == "ocirepositories" })
	doc := decode[v1.Package](t, e.get(t, fluxPackage))
	if doc.SourceState == nil || *doc.SourceState != (v1.SourceState{Access: "notReadable"}) {
		t.Fatalf("sourceState = %+v, want notReadable", doc.SourceState)
	}
	if gets := sourceGets(e); len(gets) != 0 {
		t.Errorf("the cluster received %v", gets)
	}
}

// TestPackageWithoutSource: a package that names no source has no state.
func TestPackageWithoutSource(t *testing.T) {
	objs := fluxObjs(t)
	pkg := readmodeltest.Find(t, objs, "ModulePackage", "podinfo")
	unstructured.RemoveNestedField(pkg.Object, "spec", "sourceRef")
	e := fluxEnv(t, objs, readmodeltest.AllowAll)
	if doc := decode[v1.Package](t, e.get(t, fluxPackage)); doc.SourceState != nil {
		t.Errorf("sourceState = %+v, want none", doc.SourceState)
	}
}

// longSources sets the three served fields of the Ready=False source to
// values of the given lengths in code points: the message of
// SourceNotReady, its reason, and the revision of an artifact.
func longSources(t testing.TB, message, reason, revision string) []*unstructured.Unstructured {
	t.Helper()
	objs := naming(t, fluxObjs(t), "OCIRepository", readmodeltest.SourceNotReady)
	src := readmodeltest.Find(t, objs, "OCIRepository", readmodeltest.SourceNotReady)
	conds := []any{map[string]any{"type": "Ready", "status": "False", "reason": reason, "message": message,
		"observedGeneration": int64(1), "lastTransitionTime": "2026-10-04T18:04:00Z"}}
	if err := unstructured.SetNestedSlice(src.Object, conds, "status", "conditions"); err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(src.Object, revision, "status", "artifact", "revision"); err != nil {
		t.Fatal(err)
	}
	return objs
}

func codePoints(s string) int { return utf8.RuneCountInString(s) }

// TestPackageSourceTextIsCut (AG-A1): the Ready message, reason and
// artifact revision are served cut to 1024, 256 and 256 Unicode code points,
// in the package document and in the source node, with an ellipsis; a value
// at its cap, or under it, is served as written.
func TestPackageSourceTextIsCut(t *testing.T) {
	long := func(n int) string { return strings.Repeat("é", n-1) + "x" }
	for _, tc := range []struct {
		name                    string
		message, reason, rev    string
		wantMsg, wantRsn, wantR string
	}{
		{"over the caps", long(5000), long(600), long(600),
			strings.Repeat("é", 1023) + "…", strings.Repeat("é", 255) + "…", strings.Repeat("é", 255) + "…"},
		{"one over", long(1025), long(257), long(257),
			strings.Repeat("é", 1023) + "…", strings.Repeat("é", 255) + "…", strings.Repeat("é", 255) + "…"},
		{"exactly the caps", long(1024), long(256), long(256), long(1024), long(256), long(256)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := fluxEnv(t, longSources(t, tc.message, tc.reason, tc.rev), readmodeltest.AllowAll)
			doc := decode[v1.Package](t, e.get(t, fluxPackage))
			s := doc.SourceState
			if s == nil || s.Message != tc.wantMsg || s.Reason != tc.wantRsn || s.Revision != tc.wantR {
				t.Fatalf("sourceState message %d, reason %d, revision %d code points; want %d, %d, %d (and the cut text)",
					codePoints(s.Message), codePoints(s.Reason), codePoints(s.Revision), codePoints(tc.wantMsg), codePoints(tc.wantRsn), codePoints(tc.wantR))
			}
			n := sourceNode(t, decode[v1.Graph](t, e.get(t, fluxPackageGraph)))
			if n.Health == nil || n.Health.Message != tc.wantMsg || n.Health.Reason != tc.wantRsn {
				t.Errorf("node health = %+v, want the cut message and reason", n.Health)
			}
			if n.SourceState == nil || *n.SourceState != *s {
				t.Errorf("node state differs from the document's")
			}
		})
	}
}

// TestInClusterSourceTextIsServedUpToItsCap: source-controller wrote the
// Ready message, reason and revision, not the operator, so an in-cluster
// server serves each of the three, cut to its cap, in the document and the
// graph (SR3).
func TestInClusterSourceTextIsServedUpToItsCap(t *testing.T) {
	long := func(n int) string { return strings.Repeat("é", n-1) + "x" }
	e := fluxEnv(t, longSources(t, long(5000), long(600), long(600)), readmodeltest.AllowAll, inCluster)
	doc := decode[v1.Package](t, e.get(t, fluxPackage))
	s := doc.SourceState
	if s == nil {
		t.Fatal("no sourceState")
	}
	for _, f := range []struct {
		name string
		got  string
		cap  int
	}{{"message", s.Message, 1024}, {"reason", s.Reason, 256}, {"revision", s.Revision, 256}} {
		if codePoints(f.got) != f.cap || !strings.HasSuffix(f.got, "…") {
			t.Errorf("in-cluster %s has %d code points (suffix %v), want %d ending in an ellipsis", f.name, codePoints(f.got), strings.HasSuffix(f.got, "…"), f.cap)
		}
	}
	n := sourceNode(t, decode[v1.Graph](t, e.get(t, fluxPackageGraph)))
	if n.Health == nil || codePoints(n.Health.Message) != 1024 || codePoints(n.Health.Reason) != 256 {
		t.Errorf("in-cluster node health = %+v", n.Health)
	}

	// A source served as source-controller wrote it, under its cap, is served
	// whole.
	e = fluxEnv(t, naming(t, fluxObjs(t), "OCIRepository", readmodeltest.SourceNotReady), readmodeltest.AllowAll, inCluster)
	s = decode[v1.Package](t, e.get(t, fluxPackage)).SourceState
	if s == nil || s.Message != readmodeltest.SourceNotReadyMessage || s.Reason != readmodeltest.SourceNotReadyReason {
		t.Errorf("in-cluster state = %+v, want the message and reason as written", s)
	}
	ready := decode[v1.Package](t, fluxEnv(t, fluxObjs(t), readmodeltest.AllowAll, inCluster).get(t, fluxPackage)).SourceState
	if ready == nil || ready.Revision != readmodeltest.SourceRevision {
		t.Errorf("in-cluster ready state = %+v, want the revision as written", ready)
	}
}

// TestSourceCapConstants holds the caps to the values of portal:D20:R6.
func TestSourceCapConstants(t *testing.T) {
	if maxSourceMessage != 1024 || maxSourceReason != 256 || maxSourceRevision != 256 {
		t.Errorf("caps = %d, %d, %d; want 1024, 256, 256", maxSourceMessage, maxSourceReason, maxSourceRevision)
	}
	if got := readmodel.CapText(strings.Repeat("a", 2000), maxSourceMessage); codePoints(got) != 1024 {
		t.Errorf("cut to %d", codePoints(got))
	}
}

// TestTheStreamCarriesThePackageSource: a package topic's snapshot is the GET
// document, source state included.
func TestTheStreamCarriesThePackageSource(t *testing.T) {
	e := fluxEnv(t, fluxObjs(t), readmodeltest.AllowAll, fastStream)
	c, res := openStream(t, newHTTPServer(t, e), "package:pkg/podinfo")
	if c == nil {
		t.Fatalf("stream refused: %d %s", res.status, res.body)
	}
	m := c.next(t, func(ev sse, _ message) bool { return ev.event == stream.EventSnapshot })
	if len(m.Items) != 1 {
		t.Fatalf("snapshot with %d items", len(m.Items))
	}
	if got, want := compact(t, m.Items[0]), compact(t, e.get(t, fluxPackage).body); got != want {
		t.Errorf("snapshot differs from GET:\n%s\n%s", got, want)
	}
	if !strings.Contains(string(m.Items[0]), `"sourceState"`) {
		t.Error("the snapshot carries no sourceState")
	}
}
