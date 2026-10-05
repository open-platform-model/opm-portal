package readmodel

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"

	"github.com/open-platform-model/opm-portal/internal/health"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

var (
	backupProviderRef = ObjectRef{Group: opmGroup, Version: opmVersion, Kind: "ModuleInstance", Namespace: "default", Name: "backup-provider"}
	packageHolderRef  = ObjectRef{Group: opmGroup, Version: opmVersion, Kind: "ModulePackage",
		Namespace: readmodeltest.PackageHolderNamespace, Name: readmodeltest.PackageHolderName}
)

// sameClaims compares claims field by field, following ProviderRefMatches.
func sameClaims(a, b []ProviderClaim) bool {
	return slices.EqualFunc(a, b, func(x, y ProviderClaim) bool {
		px, py := x.ProviderRefMatches, y.ProviderRefMatches
		x.ProviderRefMatches, y.ProviderRefMatches = nil, nil
		return x == y && (px == nil) == (py == nil) && (px == nil || *px == *py)
	})
}

func ptr[T any](v T) *T { return &v }

func (e *env) platform(t *testing.T) PlatformView {
	t.Helper()
	v, err := e.m.Platform(t.Context(), alice, e.grant(t, "get", platforms, "", "cluster"))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func registrationNamed(t *testing.T, v PlatformView, name string) RegistrationView {
	t.Helper()
	for i := range v.Registrations {
		if v.Registrations[i].Name == name {
			return v.Registrations[i]
		}
	}
	t.Fatalf("no registration %s among %+v", name, v.Registrations)
	return RegistrationView{}
}

// TestF1ProviderJoin: on F1, backup-provider holds its accepted claim, the
// claim applied by hand has no holder, and both sides of the join show the
// registration's own standing.
func TestF1ProviderJoin(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	v := e.platform(t)
	accepted := registrationNamed(t, v, "default.backup-provider")
	if !slices.Equal(accepted.HeldBy, []ObjectRef{backupProviderRef}) || accepted.HeldByPartial {
		t.Errorf("default.backup-provider held by %+v (partial %v); want backup-provider, complete", accepted.HeldBy, accepted.HeldByPartial)
	}
	if i := slices.IndexFunc(accepted.Conditions, func(c health.Condition) bool { return c.Type == "Ready" }); i < 0 || accepted.Conditions[i].Reason != "Accepted" {
		t.Errorf("default.backup-provider conditions = %+v; want Ready/Accepted", accepted.Conditions)
	}
	refused := registrationNamed(t, v, "default.refused-claim-fixture")
	if refused.HeldBy != nil {
		t.Errorf("the refused claim, applied by hand, is held by %+v", refused.HeldBy)
	}

	want := []ProviderClaim{{
		Registration: "default.backup-provider",
		Access:       health.AccessOK,
		Standing:     accepted.Standing,
		// spec.providerRef names default/backup-provider, the holder.
		ProviderRefMatches: ptr(true),
	}}
	d := e.instance(t, "default", "backup-provider")
	if !sameClaims(d.ProviderOf, want) {
		t.Errorf("backup-provider providerOf = %+v; want %+v", d.ProviderOf, want)
	}
	if s := d.ProviderOf[0].Standing; !s.Accepted || !s.Active || s.Verdict != health.VerdictAccepted {
		t.Errorf("standing = %+v; want accepted, active, Accepted", s)
	}
}

// TestF1InstanceListJoin: list items carry the same claims, and their
// render contracts.
func TestF1InstanceListJoin(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	accepted := registrationNamed(t, e.platform(t), "default.backup-provider")
	want := []ProviderClaim{{Registration: "default.backup-provider", Access: health.AccessOK, Standing: accepted.Standing, ProviderRefMatches: ptr(true)}}
	list, err := e.m.ListInstances(t.Context(), alice, e.grant(t, "list", moduleInstances, "", ""), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range list {
		switch it.Ref.Name {
		case "backup-provider":
			if !sameClaims(it.ProviderOf, want) {
				t.Errorf("list item providerOf = %+v; want %+v", it.ProviderOf, want)
			}
		default:
			if it.ProviderOf != nil {
				t.Errorf("%s providerOf = %+v; it holds no registration", it.Ref.Name, it.ProviderOf)
			}
		}
		if it.Ref.Name == "backup-consumer" && !slices.Contains(it.RenderContracts, "opmodel.dev/catalogs/opm/traits/backup@v1alpha1") {
			t.Errorf("backup-consumer list item render contracts = %v", it.RenderContracts)
		}
	}
}

// TestF1HistoryOutcomes: pkg/podinfo's three entries carry a message and no
// phase, the instances' carry phase complete.
func TestF1HistoryOutcomes(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	p, err := e.m.Package(t.Context(), alice, e.grant(t, "get", modulePackages, "pkg", "podinfo"), "pkg", "podinfo")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.History) != 3 {
		t.Fatalf("pkg/podinfo history = %+v", p.History)
	}
	for _, h := range p.History {
		if h.Outcome != OutcomeFailed {
			t.Errorf("pkg/podinfo entry %d outcome = %s; want Failed", h.Sequence, h.Outcome)
		}
	}
	for _, name := range []string{"podinfo", "backup-provider"} {
		d := e.instance(t, "default", name)
		if len(d.History) == 0 {
			t.Fatalf("%s has no history", name)
		}
		for _, h := range d.History {
			if h.Outcome != OutcomeSucceeded {
				t.Errorf("%s entry %d (phase %q) outcome = %s; want Succeeded", name, h.Sequence, h.Phase, h.Outcome)
			}
		}
	}
}

func TestOutcomeOf(t *testing.T) {
	for _, tt := range []struct {
		phase, message string
		want           Outcome
	}{
		{"complete", "", OutcomeSucceeded},
		{"", "getting OCIRepository: not found", OutcomeFailed},
		{"", "", OutcomeUnknown},
		{"complete", "a success entry with a message", OutcomeSucceeded},
		{"applying", "", OutcomeUnknown},
		{"applying", "a phase the operator never writes", OutcomeUnknown},
	} {
		if got := outcomeOf(tt.phase, tt.message); got != tt.want {
			t.Errorf("outcomeOf(%q, %q) = %s; want %s", tt.phase, tt.message, got, tt.want)
		}
	}
}

// TestF1PackageIntervalAndSource: F1 runs no Flux, so pkg/podinfo has its
// interval and no source artifact; a package that recorded one carries its
// revision and digest, never its fetch URL.
func TestF1PackageIntervalAndSource(t *testing.T) {
	objs := loadF1(t)
	fetched := find(t, objs, "ModulePackage", "podinfo").DeepCopy()
	fetched.SetName("fetched")
	if err := unstructured.SetNestedField(fetched.Object, map[string]any{
		"artifactRevision": "6.7.1@sha256:aa",
		"artifactDigest":   "sha256:bb",
		"artifactURL":      "http://source-controller.flux-system.svc/ocirepository/pkg/podinfo/aa.tar.gz",
	}, "status", "source"); err != nil {
		t.Fatal(err)
	}
	unstructured.RemoveNestedField(fetched.Object, "spec", "interval")
	e := newEnv(t, append(objs, fetched), allowAll, allowAll)
	list, err := e.m.ListPackages(t.Context(), alice, e.grant(t, "list", modulePackages, "", ""), "pkg")
	if err != nil || len(list) != 2 {
		t.Fatalf("ListPackages = %+v, %v", list, err)
	}
	got := map[string]PackageItem{}
	for _, it := range list {
		got[it.Ref.Name] = it
	}
	if p := got["podinfo"]; p.Interval != "1m" || p.SourceArtifact != nil {
		t.Errorf("pkg/podinfo interval %q, source artifact %+v; want 1m and none", p.Interval, p.SourceArtifact)
	}
	if p := got["fetched"]; p.Interval != "" || p.SourceArtifact == nil ||
		*p.SourceArtifact != (SourceArtifact{Revision: "6.7.1@sha256:aa", Digest: "sha256:bb"}) {
		t.Errorf("fetched interval %q, source artifact %+v", p.Interval, p.SourceArtifact)
	}
}

// TestPackageHolder: a package's claim is joined like an instance's, and
// the registration's own refusal is shown unchanged. The package holder is
// constructed from F1 (readmodeltest.WithPackageHolder).
func TestPackageHolder(t *testing.T) {
	e := newEnv(t, readmodeltest.WithPackageHolder(t, loadF1(t)), allowAll, allowAll)
	claim := registrationNamed(t, e.platform(t), readmodeltest.PackageHolderClaim)
	if !slices.Equal(claim.HeldBy, []ObjectRef{packageHolderRef}) || claim.HeldByPartial {
		t.Errorf("pkg.provider held by %+v (partial %v); want pkg/provider", claim.HeldBy, claim.HeldByPartial)
	}
	if claim.Provider.Kind != "ModuleInstance" || claim.Provider.Namespace != "pkg" || claim.Provider.Name != "provider" {
		t.Errorf("provider = %+v; want the ModuleInstance pkg/provider spec.providerRef names", claim.Provider)
	}
	if claim.Standing.Verdict != health.VerdictRefused || claim.Standing.Reason != "ProviderMismatch" || claim.Standing.Accepted {
		t.Errorf("standing = %+v; want Refused ProviderMismatch", claim.Standing)
	}
	p, err := e.m.Package(t.Context(), alice, e.grant(t, "get", modulePackages, "pkg", "provider"), "pkg", "provider")
	if err != nil {
		t.Fatal(err)
	}
	want := []ProviderClaim{{Registration: readmodeltest.PackageHolderClaim, Access: health.AccessOK, Standing: claim.Standing, ProviderRefMatches: ptr(false)}}
	if !sameClaims(p.ProviderOf, want) {
		t.Errorf("pkg/provider providerOf = %+v; want %+v (providerRefMatches false: the reference names a ModuleInstance)", p.ProviderOf, want)
	}
}

// TestProviderOfWhenRegistrationsAreForbidden: the claim keeps its name,
// from the owner's inventory, and nothing else.
func TestProviderOfWhenRegistrationsAreForbidden(t *testing.T) {
	e := newEnv(t, loadF1(t), denyResources("transformerregistrations"), allowAll)
	d := e.instance(t, "default", "backup-provider")
	want := []ProviderClaim{{Registration: "default.backup-provider", Access: health.AccessForbidden}}
	if !sameClaims(d.ProviderOf, want) {
		t.Errorf("providerOf = %+v; want %+v", d.ProviderOf, want)
	}
}

// listOnlyIn returns a rule that allows every read but a list of resource
// outside namespace.
func listOnlyIn(resource, namespace string) rule {
	return func(_ string, ra authorizationv1.ResourceAttributes) bool {
		return ra.Resource != resource || ra.Verb != "list" || ra.Namespace == namespace
	}
}

func TestHoldersForACallerWhoCannotLookEverywhere(t *testing.T) {
	for _, tt := range []struct {
		name string
		rule rule
		want []ObjectRef
	}{
		// A namespace the caller may not list hides its holder.
		{"instances listable only in web", listOnlyIn("moduleinstances", "web"), nil},
		// A holder found is no proof that none is missing.
		{"packages listable only in default", listOnlyIn("modulepackages", "default"), []ObjectRef{backupProviderRef}},
		// A holder in a namespace the caller may list is included.
		{"instances listable only in default", listOnlyIn("moduleinstances", "default"), []ObjectRef{backupProviderRef}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t, loadF1(t), tt.rule, allowAll)
			r := registrationNamed(t, e.platform(t), "default.backup-provider")
			if !slices.Equal(r.HeldBy, tt.want) || !r.HeldByPartial {
				t.Errorf("held by %+v (partial %v); want %+v, partial", r.HeldBy, r.HeldByPartial, tt.want)
			}
		})
	}
}

// TestServerVersionIsReadOnceAtStart: one request to /version, through
// discovery, and no access review for it.
func TestServerVersionIsReadOnceAtStart(t *testing.T) {
	disc := newDiscovery(clusterKinds...)
	e := newEnv(t, loadF1(t), allowAll, allowAll, func(c *Config) { c.Discovery = disc })
	e.platform(t)
	e.instance(t, "default", "podinfo")
	if got := e.m.ServerVersion(); got != readmodeltest.ServerVersion {
		t.Errorf("ServerVersion = %q; want %q", got, readmodeltest.ServerVersion)
	}
	if n := versionReads(disc); n != 1 {
		t.Errorf("/version read %d times; want once", n)
	}
	for _, ra := range slices.Concat(e.reviews.Asked(), e.readerR.Asked()) {
		if ra.Resource == "version" || ra.Resource == "" {
			t.Errorf("a review was sent for a non-resource read: %+v", ra)
		}
	}
}

func TestServerVersionReadFailureHoldsNothing(t *testing.T) {
	disc := newDiscovery(clusterKinds...)
	disc.PrependReactor("get", "version", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("connection refused")
	})
	e := newEnv(t, loadF1(t), allowAll, allowAll, func(c *Config) { c.Discovery = disc })
	if got := e.m.ServerVersion(); got != "" {
		t.Errorf("ServerVersion = %q after a failed read; want none", got)
	}
	e.platform(t)
}

func versionReads(d *fakediscovery.FakeDiscovery) int {
	n := 0
	for _, a := range d.Actions() {
		if a.GetVerb() == "get" && a.GetResource().Resource == "version" {
			n++
		}
	}
	return n
}

// TestChangesFollowTheProviderJoins: a registration's change reaches its
// holders, joined, whether or not it carries the instance label; a holder
// that drops its claim reaches the Platform, joined.
func TestChangesFollowTheProviderJoins(t *testing.T) {
	objs := loadF1(t)
	// Without its uuid label the registration reaches backup-provider only
	// through the join, not through the label path tier 2 objects take.
	find(t, objs, "TransformerRegistration", "default.backup-provider").SetLabels(nil)
	e := newUnstartedEnv(t, objs, allowAll, allowAll)
	rec := &recorder{}
	remove := e.m.OnChange(rec.add)
	defer remove()
	if err := e.m.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	rec.wait(t, Change{Kind: ChangeInstance, Namespace: "default", Name: "backup-provider"})

	rec.reset()
	reg, err := e.client.Resource(registrations).Get(t.Context(), "default.backup-provider", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(reg.Object, false, "status", "active"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.client.Resource(registrations).Update(t.Context(), reg, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	rec.wait(t, Change{Kind: ChangeInstance, Namespace: "default", Name: "backup-provider", Joined: true})
	rec.wait(t, Change{Kind: ChangePlatform, Name: "cluster"})

	rec.reset()
	inst, err := e.client.Resource(moduleInstances).Namespace("default").Get(t.Context(), "backup-provider", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedSlice(inst.Object, []any{}, "status", "inventory", "entries"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.client.Resource(moduleInstances).Namespace("default").Update(t.Context(), inst, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	rec.wait(t, Change{Kind: ChangeInstance, Namespace: "default", Name: "backup-provider"})
	rec.wait(t, Change{Kind: ChangePlatform, Name: "cluster", Joined: true})
}

// TestProviderOfARegistrationTheClusterDoesNotHold: the inventory names a
// registration there is none of; the claim is readable, its verdict
// Unknown, and nothing is said about its providerRef.
func TestProviderOfARegistrationTheClusterDoesNotHold(t *testing.T) {
	objs := loadF1(t)
	kept := objs[:0]
	for _, o := range objs {
		if o.GetKind() != "TransformerRegistration" || o.GetName() != "default.backup-provider" {
			kept = append(kept, o)
		}
	}
	e := newEnv(t, kept, allowAll, allowAll)
	d := e.instance(t, "default", "backup-provider")
	want := []ProviderClaim{{Registration: "default.backup-provider", Access: health.AccessOK,
		Standing: health.Registration{Verdict: health.VerdictUnknown}}}
	if !sameClaims(d.ProviderOf, want) {
		t.Errorf("providerOf = %+v; want %+v with no providerRefMatches", d.ProviderOf, want)
	}
}

// TestProviderOfWhenTheReaderMayNotWatchRegistrations: the caller may list
// them, but the model holds none, so the claim is not readable.
func TestProviderOfWhenTheReaderMayNotWatchRegistrations(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, denyResources("transformerregistrations"))
	d := e.instance(t, "default", "backup-provider")
	want := []ProviderClaim{{Registration: "default.backup-provider", Access: health.AccessNotReadable}}
	if !sameClaims(d.ProviderOf, want) {
		t.Errorf("providerOf = %+v; want %+v", d.ProviderOf, want)
	}
}

// TestHoldersWhenTheModelCannotListOwners: the reader may not watch
// ModulePackages, so their list fails and the answer is partial, while
// the instance holder is still found.
func TestHoldersWhenTheModelCannotListOwners(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, denyResources("modulepackages"))
	r := registrationNamed(t, e.platform(t), "default.backup-provider")
	if !slices.Equal(r.HeldBy, []ObjectRef{backupProviderRef}) || !r.HeldByPartial {
		t.Errorf("held by %+v (partial %v); want backup-provider, partial", r.HeldBy, r.HeldByPartial)
	}
}

// TestHoldersWithConfiguredNamespaces: a model that holds only some
// namespaces cannot say nobody else holds a registration.
func TestHoldersWithConfiguredNamespaces(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll, func(c *Config) { c.Namespaces = []string{"default", "pkg"} })
	r := registrationNamed(t, e.platform(t), "default.backup-provider")
	if !slices.Equal(r.HeldBy, []ObjectRef{backupProviderRef}) || !r.HeldByPartial {
		t.Errorf("held by %+v (partial %v); want backup-provider, partial", r.HeldBy, r.HeldByPartial)
	}
}

// TestServerVersionThroughTheRESTClient: a real discovery client reads
// /version once with the request's context, and a failed answer holds
// nothing.
func TestServerVersionThroughTheRESTClient(t *testing.T) {
	for _, tt := range []struct {
		status int
		want   string
	}{{http.StatusOK, "v1.36.1"}, {http.StatusInternalServerError, ""}} {
		var reads atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/version" {
				http.NotFound(w, r)
				return
			}
			reads.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(tt.status)
			_, _ = w.Write([]byte(`{"major":"1","minor":"36","gitVersion":"v1.36.1"}`))
		}))
		d, err := discovery.NewDiscoveryClientForConfig(&rest.Config{Host: srv.URL})
		if err != nil {
			t.Fatal(err)
		}
		info, err := serverVersion(t.Context(), d)
		got := ""
		if err == nil {
			got = info.GitVersion
		}
		if got != tt.want || reads.Load() != 1 {
			t.Errorf("status %d: version %q (%v) after %d reads; want %q after one", tt.status, got, err, reads.Load(), tt.want)
		}
		srv.Close()
	}
}
