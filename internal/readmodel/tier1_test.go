package readmodel

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/health"
)

func TestStartRefusesASecondStart(t *testing.T) {
	e := newEnv(t, nil, allowAll, allowAll)
	if err := e.m.Start(t.Context()); err == nil {
		t.Fatal("second Start succeeded")
	}
	e.m.Stop()
	e.m.Stop()
	if _, err := e.m.ListInstances(t.Context(), alice, e.grant(t, "list", moduleInstances, "", ""), ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ListInstances after Stop = %v, want ErrUnavailable", err)
	}
}

func TestPlatformView(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	v, err := e.m.Platform(t.Context(), alice, e.grant(t, "get", platforms, "", "cluster"))
	if err != nil {
		t.Fatal(err)
	}
	if v.Name != "cluster" || v.Type != "kubernetes" || v.OperatorVersion != "v1.0.0-beta.6" {
		t.Errorf("platform = %s %s %s", v.Name, v.Type, v.OperatorVersion)
	}
	if v.Applied.State != health.AppliedStateApplied {
		t.Errorf("applied = %+v, want Applied", v.Applied)
	}
	if len(v.Applied.Notes) != 1 || v.Applied.Notes[0].Reason != "UnfulfilledContracts" {
		t.Errorf("notes = %+v, want the unfulfilled-contracts note", v.Applied.Notes)
	}
	checkCatalogs(t, v)
	checkRegistrations(t, v)
}

// checkCatalogs: the subscribed catalog in spec, and both the subscribed
// and the registered catalog in the resolved registry.
func checkCatalogs(t *testing.T, v PlatformView) {
	t.Helper()
	if len(v.Subscriptions) != 1 || v.Subscriptions[0].Catalog != "opmodel.dev/catalogs/opm@v4" ||
		v.Subscriptions[0].Version != "4.6.0" || v.Subscriptions[0].Enable == nil || !*v.Subscriptions[0].Enable {
		t.Errorf("subscriptions = %+v", v.Subscriptions)
	}
	wantCatalogs := []Catalog{
		{Catalog: "opmodel.dev/catalogs/opm@v4", Version: "4.6.0", Enabled: true, Source: "Subscription"},
		{Catalog: "testing.opmodel.dev/catalogs/operator/backup@v0", Version: "0.1.0", Enabled: true, Source: "Registration",
			Registrations: []string{"default.backup-provider"}},
	}
	if len(v.Catalogs) != len(wantCatalogs) {
		t.Fatalf("catalogs = %+v", v.Catalogs)
	}
	for i, want := range wantCatalogs {
		got := v.Catalogs[i]
		if got.Catalog != want.Catalog || got.Version != want.Version || got.Enabled != want.Enabled ||
			got.Source != want.Source || !slices.Equal(got.Registrations, want.Registrations) {
			t.Errorf("catalog %d = %+v, want %+v", i, got, want)
		}
	}
}

// checkRegistrations: the accepted claim and the deliberate refusal of F1,
// acceptance, activation and verdict read apart (0030:D4:R4).
func checkRegistrations(t *testing.T, v PlatformView) {
	t.Helper()
	if v.RegistrationsAccess != health.AccessOK || len(v.Registrations) != 2 {
		t.Fatalf("registrations = %v %+v", v.RegistrationsAccess, v.Registrations)
	}
	accepted, refused := v.Registrations[0], v.Registrations[1]
	if accepted.Name != "default.backup-provider" || !accepted.Standing.Accepted || !accepted.Standing.Active ||
		accepted.Standing.Verdict != health.VerdictAccepted || accepted.Provider.Name != "backup-provider" ||
		!slices.Equal(accepted.Provides, []string{"opmodel.dev/catalogs/opm/traits/backup@v1alpha1"}) {
		t.Errorf("accepted claim = %+v", accepted)
	}
	if refused.Name != "default.refused-claim-fixture" || refused.Standing.Accepted || refused.Standing.Active ||
		refused.Standing.Verdict != health.VerdictRefused || refused.Standing.Reason != "CatalogUnresolved" {
		t.Errorf("refused claim = %+v", refused)
	}
}

func TestPlatformMarksRegistrationsTheCallerMayNotList(t *testing.T) {
	e := newEnv(t, loadF1(t), denyResources("transformerregistrations"), allowAll)
	v, err := e.m.Platform(t.Context(), alice, e.grant(t, "get", platforms, "", "cluster"))
	if err != nil {
		t.Fatal(err)
	}
	if v.RegistrationsAccess != health.AccessForbidden || v.Registrations != nil {
		t.Fatalf("registrations = %v %+v, want forbidden and none", v.RegistrationsAccess, v.Registrations)
	}
	if len(v.Catalogs) != 2 || v.Catalogs[1].Registrations != nil {
		t.Fatalf("catalogs = %+v, want both, naming no registration", v.Catalogs)
	}
}

func TestPackageView(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	p, err := e.m.Package(t.Context(), alice, e.grant(t, "get", modulePackages, "pkg", "podinfo"), "pkg", "podinfo")
	if err != nil {
		t.Fatal(err)
	}
	if p.Applied.State != health.AppliedStateFailed || p.Applied.Reason != "SourceNotReady" || !p.Applied.Retrying {
		t.Errorf("applied = %+v, want Failed SourceNotReady retrying", p.Applied)
	}
	if p.Source.Kind != "OCIRepository" || p.Source.Name != "podinfo-release" || p.Path != "." {
		t.Errorf("source = %+v path %q", p.Source, p.Path)
	}
	if len(p.History) != 3 || p.History[0].Sequence != 3 || p.InventoryCount != 0 {
		t.Errorf("history = %d entries (first %+v), inventory %d", len(p.History), p.History, p.InventoryCount)
	}
	list, err := e.m.ListPackages(t.Context(), alice, e.grant(t, "list", modulePackages, "", ""), "")
	if err != nil || len(list) != 1 || list[0].Ref.Name != "podinfo" {
		t.Fatalf("ListPackages = %+v, %v", list, err)
	}
}

func TestInstanceViews(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	cases := []struct {
		ns, name  string
		owner     Owner
		applied   health.AppliedState
		inventory int64
		contracts int
		history   int
	}{
		{"cert-manager", "cert-manager", OwnerOperator, health.AppliedStateApplied, 42, 15, 1},
		{"default", "podinfo", OwnerOperator, health.AppliedStateApplied, 2, 7, 1},
		{"web", "web", OwnerCLI, health.AppliedStateManagedExternally, 2, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := e.m.Instance(t.Context(), alice, e.grant(t, "get", moduleInstances, tc.ns, tc.name), tc.ns, tc.name)
			if err != nil {
				t.Fatal(err)
			}
			if d.Owner != tc.owner || d.Applied.State != tc.applied || d.InventoryCount != tc.inventory ||
				len(d.RenderContracts) != tc.contracts || len(d.History) != tc.history {
				t.Errorf("instance = owner %s applied %s inventory %d contracts %d history %d",
					d.Owner, d.Applied.State, d.InventoryCount, len(d.RenderContracts), len(d.History))
			}
			if d.Applied.State.IsFault() {
				t.Errorf("applied state %s shown as a fault", d.Applied.State)
			}
		})
	}
}

// TestApplyFailedInstance: the operator could not apply, retries, and has
// recorded no inventory; the instance is Failed and its inventory is empty,
// not unreadable.
func TestApplyFailedInstance(t *testing.T) {
	e := newEnv(t, loadList(t, applyFailedSample), allowAll, allowAll)
	d, err := e.m.Instance(t.Context(), alice, e.grant(t, "get", moduleInstances, "cert-manager", "cert-manager"), "cert-manager", "cert-manager")
	if err != nil {
		t.Fatal(err)
	}
	if d.Applied.State != health.AppliedStateFailed || d.Applied.Reason != "ApplyFailed" || !d.Applied.Retrying {
		t.Errorf("applied = %+v, want Failed ApplyFailed retrying", d.Applied)
	}
	if d.InventoryCount != 0 || d.Components != nil || !d.LastAppliedAt.IsZero() {
		t.Errorf("inventory %d components %v lastAppliedAt %v, want none", d.InventoryCount, d.Components, d.LastAppliedAt)
	}
	if len(d.RenderContracts) == 0 {
		t.Errorf("render contracts missing although the operator recorded them")
	}
}

func TestListInstancesStaysInTheGrantedNamespace(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	names := func(items []InstanceItem) []string {
		out := make([]string, 0, len(items))
		for _, i := range items {
			out = append(out, i.Ref.Namespace+"/"+i.Ref.Name)
		}
		return out
	}
	got, err := e.m.ListInstances(t.Context(), alice, e.grant(t, "list", moduleInstances, "default", ""), "default")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"default/backup-consumer", "default/backup-provider", "default/podinfo"}; !slices.Equal(names(got), want) {
		t.Errorf("default = %v, want %v", names(got), want)
	}
	got, err = e.m.ListInstances(t.Context(), alice, e.grant(t, "list", moduleInstances, "", ""), "")
	if err != nil || len(got) != 5 {
		t.Errorf("all namespaces = %v, %v; want 5", names(got), err)
	}
	// A grant for one namespace does not cover another, or all of them.
	g := e.grant(t, "list", moduleInstances, "default", "")
	for _, ns := range []string{"web", ""} {
		if _, err := e.m.ListInstances(t.Context(), alice, g, ns); !errors.Is(err, ErrNotCovered) {
			t.Errorf("ListInstances(%q) with a default-only grant = %v, want ErrNotCovered", ns, err)
		}
	}
}

func TestReadsRefuseUncoveredGrants(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	bob := authz.Identity{Username: "bob", Groups: []string{"system:authenticated"}}
	bobChecker, _ := newChecker(t, bob, allowAll, authz.Options{})
	bobGrant, err := bobChecker.Check(t.Context(), bob, authz.Attributes{Verb: "get", Resource: moduleInstances, Namespace: "default", Name: "podinfo"})
	if err != nil {
		t.Fatal(err)
	}
	shortLived, _ := newChecker(t, alice, allowAll, authz.Options{TTL: time.Millisecond})
	expired, err := shortLived.Check(t.Context(), alice, authz.Attributes{Verb: "get", Resource: moduleInstances, Namespace: "default", Name: "podinfo"})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	var zero authz.Grant
	cases := map[string]authz.Grant{
		"zero grant":             zero,
		"issued to someone else": bobGrant,
		"expired":                expired,
		"another instance":       e.grant(t, "get", moduleInstances, "default", "backup-consumer"),
		"another kind":           e.grant(t, "get", modulePackages, "default", "podinfo"),
		"list, not get":          e.grant(t, "list", moduleInstances, "default", ""),
		"another namespace":      e.grant(t, "get", moduleInstances, "web", ""),
		"watch, not get":         e.grant(t, "watch", moduleInstances, "default", ""),
	}
	before := len(clusterReads(e.client))
	for name, g := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := e.m.Instance(t.Context(), alice, g, "default", "podinfo")
			if !errors.Is(err, ErrNotCovered) || !errors.Is(err, authz.ErrNoGrant) {
				t.Fatalf("Instance = %v, want ErrNotCovered wrapping authz.ErrNoGrant", err)
			}
			if strings.Contains(err.Error(), "podinfo") || strings.Contains(err.Error(), "alice") {
				t.Errorf("refusal %q names the object or the caller", err)
			}
		})
	}
	if after := len(clusterReads(e.client)); after != before {
		t.Errorf("refused reads reached the cluster: %v", clusterReads(e.client)[before:])
	}
	var zeroGrant authz.Grant
	if _, err := e.m.Platform(t.Context(), alice, zeroGrant); !errors.Is(err, ErrNotCovered) {
		t.Errorf("Platform with no grant = %v", err)
	}
	if _, err := e.m.ListPackages(t.Context(), alice, zeroGrant, ""); !errors.Is(err, ErrNotCovered) {
		t.Errorf("ListPackages with no grant = %v", err)
	}
	if _, err := e.m.Package(t.Context(), alice, zeroGrant, "pkg", "podinfo"); !errors.Is(err, ErrNotCovered) {
		t.Errorf("Package with no grant = %v", err)
	}
}

// TestMissingAndExistingReadAlike: an uncovered read is refused the same
// way whether or not the object exists (0030:D7:R1), and a covered read of a
// missing object is not found.
func TestMissingAndExistingReadAlike(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	g := e.grant(t, "get", moduleInstances, "default", "backup-consumer")
	_, errExists := e.m.Instance(t.Context(), alice, g, "default", "podinfo")
	_, errMissing := e.m.Instance(t.Context(), alice, g, "default", "no-such-instance")
	if errExists == nil || errMissing == nil || errExists.Error() != errMissing.Error() {
		t.Fatalf("refusals differ:\n%v\n%v", errExists, errMissing)
	}
	if _, err := e.m.Instance(t.Context(), alice, e.grant(t, "get", moduleInstances, "default", ""), "default", "no-such-instance"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("covered read of a missing instance = %v, want ErrNotFound", err)
	}
}

func TestKindTheReaderMayNotWatchIsUnavailable(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, denyResources("modulepackages"))
	if _, err := e.m.ListPackages(t.Context(), alice, e.grant(t, "list", modulePackages, "", ""), ""); !errors.Is(err, ErrUnavailable) {
		t.Errorf("ListPackages = %v, want ErrUnavailable", err)
	}
	if _, err := e.m.Package(t.Context(), alice, e.grant(t, "get", modulePackages, "pkg", "podinfo"), "pkg", "podinfo"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Package = %v, want ErrUnavailable", err)
	}
	for _, r := range clusterReads(e.client) {
		if strings.Contains(r, "modulepackages") {
			t.Errorf("read a kind the reader may not watch: %s", r)
		}
	}
}

func TestConfiguredNamespacesBoundTheModel(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll, func(c *Config) { c.Namespaces = []string{"default"} })
	got, err := e.m.ListInstances(t.Context(), alice, e.grant(t, "list", moduleInstances, "default", ""), "default")
	if err != nil || len(got) != 3 {
		t.Fatalf("default = %d items, %v", len(got), err)
	}
	if _, err := e.m.ListInstances(t.Context(), alice, e.grant(t, "list", moduleInstances, "web", ""), "web"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("web, outside the configured namespaces = %v, want ErrUnavailable", err)
	}
	// The cluster-scoped kinds stay cluster-wide.
	if _, err := e.m.Platform(t.Context(), alice, e.grant(t, "get", platforms, "", "cluster")); err != nil {
		t.Errorf("Platform = %v", err)
	}
}

// TestHeldInstancesCarryNoValues: a client-side applied instance arrives
// with values, managed fields and the last-applied annotation; none of them
// is held (0030:D8:R2/R3).
func TestHeldInstancesCarryNoValues(t *testing.T) {
	const secret = "s3cr3t-value"
	podinfo := find(t, loadF1(t), "ModuleInstance", "podinfo").DeepCopy()
	if err := unstructured.SetNestedField(podinfo.Object, map[string]any{"password": secret}, "spec", "values"); err != nil {
		t.Fatal(err)
	}
	podinfo.SetAnnotations(map[string]string{lastAppliedAnnotation: `{"spec":{"values":{"password":"` + secret + `"}}}`})
	podinfo.SetManagedFields([]metav1.ManagedFieldsEntry{{Manager: "kubectl"}})
	e := newEnv(t, []*unstructured.Unstructured{podinfo}, allowAll, allowAll)

	held, err := e.m.getHeld(t.Context(), moduleInstances, "default", "podinfo")
	if err != nil {
		t.Fatal(err)
	}
	d, err := e.m.Instance(t.Context(), alice, e.grant(t, "get", moduleInstances, "default", "podinfo"), "default", "podinfo")
	if err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string]any{"held object": held.Object, "detail view": d} {
		js, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		for _, leak := range []string{secret, "managedFields", lastAppliedAnnotation} {
			if strings.Contains(string(js), leak) {
				t.Errorf("%s holds %q", name, leak)
			}
		}
	}
}

// TestFailedReaderReviewAtStartIsAskedAgain: a reader review that could
// not be made at Start leaves ModuleInstances unavailable, and the first
// read once reviews work starts the informer and is answered.
func TestFailedReaderReviewAtStartIsAskedAgain(t *testing.T) {
	e := newUnstartedEnv(t, loadF1(t), allowAll, allowAll)
	e.readerR.setFail(true)
	if err := e.m.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	g := e.grant(t, "list", moduleInstances, "", "")
	if _, err := e.m.ListInstances(t.Context(), alice, g, ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ListInstances while reviews fail = %v, want ErrUnavailable", err)
	}
	e.readerR.setFail(false)
	items, err := e.m.ListInstances(t.Context(), alice, g, "")
	if err != nil {
		t.Fatalf("ListInstances after reviews recover = %v", err)
	}
	if len(items) == 0 {
		t.Fatal("ListInstances after reviews recover is empty")
	}
}
