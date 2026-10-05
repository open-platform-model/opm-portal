package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

// bob is a second caller.
var bob = authz.Identity{Username: "bob", Groups: []string{"system:authenticated"}}

func decode[T any](t *testing.T, res response) T {
	t.Helper()
	if res.status != http.StatusOK {
		t.Fatalf("status %d: %s", res.status, res.body)
	}
	var doc T
	if err := json.Unmarshal(res.body, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func registrationIn(t *testing.T, p v1.Platform, name string) v1.Registration {
	t.Helper()
	for i := range p.Registrations {
		if p.Registrations[i].Name == name {
			return p.Registrations[i]
		}
	}
	t.Fatalf("no registration %s in %+v", name, p.Registrations)
	return v1.Registration{}
}

// TestClusterDocument (portal:D18): the connection's names, the caller's own
// username and the version, read with no review and no cluster read.
func TestClusterDocument(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
	before, reads := e.az.callerChecks(), len(e.client.Actions())
	c := decode[v1.Cluster](t, e.get(t, base))
	want := v1.Cluster{
		TypeMeta: meta(v1.KindCluster), Name: "default", Mode: "local", Source: "kubeconfig",
		Context: "kind-opm-portal-e2e", ClusterEntry: "kind-opm-portal-e2e",
		ReadingAs: v1.ReadingAs{Username: "alice"}, KubernetesVersion: readmodeltest.ServerVersion,
	}
	if c != want {
		t.Errorf("cluster = %+v; want %+v", c, want)
	}
	if e.az.callerChecks() != before || len(e.client.Actions()) != reads {
		t.Error("serving the cluster document sent a review or read the cluster")
	}

	// The username is the requesting caller's, whoever that is.
	e.principal = Principal{Identity: bob, Session: "session-bob"}
	if c := decode[v1.Cluster](t, e.get(t, base)); c.ReadingAs.Username != bob.Username {
		t.Errorf("reading as %q; want the caller %q", c.ReadingAs.Username, bob.Username)
	}

	e.principal = Principal{}
	expectProblem(t, e.get(t, base), http.StatusUnauthorized, v1.CodeUnauthenticated)
	e.principal = Principal{Identity: alice, Session: "session-alice"}
	expectProblem(t, e.get(t, Prefix+"/clusters/prod"), http.StatusNotFound, v1.CodeNotFound)
	expectProblem(t, e.do(t, http.MethodPost, base), http.StatusMethodNotAllowed, v1.CodeMethodNotAllowed)
}

func TestClusterDocumentWithoutAVersion(t *testing.T) {
	disc := readmodeltest.Discovery(readmodeltest.Kinds...)
	disc.PrependReactor("get", "version", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("connection refused")
	})
	e := newEnvWithDiscovery(t, loadF1(t), readmodeltest.AllowAll, readmodeltest.AllowAll, disc)
	res := e.get(t, base)
	if c := decode[v1.Cluster](t, res); c.KubernetesVersion != "" || strings.Contains(string(res.body), "kubernetesVersion") {
		t.Errorf("cluster = %s; want no kubernetesVersion", res.body)
	}
}

func TestNewRefusesAMissingConnection(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
	for _, source := range []string{"", "kubeconfig-ish"} {
		_, err := New(Config{Mode: ModeLocal, Model: e.model, Authorizer: e.az, Authenticate: e.srv.cfg.Authenticate,
			Connection: Connection{Source: source}})
		if err == nil || !strings.Contains(err.Error(), "connection") {
			t.Errorf("New with source %q: %v; want a refusal naming the connection", source, err)
		}
	}
}

// TestProviderOfRegistrationsForbidden: a caller who may not list
// registrations sees the claim's name and its access, nothing else.
func TestProviderOfRegistrationsForbidden(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.DenyResources("transformerregistrations"))
	res := e.get(t, base+"/instances/default/backup-provider")
	d := decode[v1.Instance](t, res)
	want := []v1.ProviderClaim{{Registration: "default.backup-provider", Access: v1.AccessForbidden}}
	if !slices.Equal(d.ProviderOf, want) {
		t.Errorf("providerOf = %+v; want %+v", d.ProviderOf, want)
	}
	if strings.Contains(string(res.body), "providerRefMatches") {
		t.Errorf("a forbidden claim carries providerRefMatches: %s", res.body)
	}
}

// TestPackageHolderOnTheWire: a package's claim, refused by the controller,
// shows as held by the package, with the registration's own refusal and a
// providerRef that names a ModuleInstance (constructed fixture,
// readmodeltest.WithPackageHolder).
func TestPackageHolderOnTheWire(t *testing.T) {
	e := newEnv(t, readmodeltest.WithPackageHolder(t, loadF1(t)), readmodeltest.AllowAll)
	r := registrationIn(t, decode[v1.Platform](t, e.get(t, base+"/platform")), readmodeltest.PackageHolderClaim)
	wantHolder := v1.ObjectRef{Group: opmGroup, Version: opmVersion, Kind: kindModulePackage, Namespace: "pkg", Name: "provider"}
	if !slices.Equal(r.HeldBy, []v1.ObjectRef{wantHolder}) || r.HeldByPartial {
		t.Errorf("heldBy = %+v (partial %v)", r.HeldBy, r.HeldByPartial)
	}
	if r.Provider == nil || r.Provider.Kind != kindModuleInstance || r.Provider.Namespace != "pkg" || r.Provider.Name != "provider" {
		t.Errorf("provider = %+v; want ModuleInstance pkg/provider", r.Provider)
	}
	if r.Verdict != "Refused" || r.Reason != "ProviderMismatch" {
		t.Errorf("verdict %s, reason %s; want Refused, ProviderMismatch", r.Verdict, r.Reason)
	}
}

// TestPackageProviderOfOnTheWire: the package names the claim it holds,
// with the controller's refusal and a providerRef that cannot name it.
func TestPackageProviderOfOnTheWire(t *testing.T) {
	e := newEnv(t, readmodeltest.WithPackageHolder(t, loadF1(t)), readmodeltest.AllowAll)
	p := decode[v1.Package](t, e.get(t, base+"/packages/pkg/provider"))
	if len(p.ProviderOf) != 1 {
		t.Fatalf("providerOf = %+v", p.ProviderOf)
	}
	c := p.ProviderOf[0]
	if c.Registration != readmodeltest.PackageHolderClaim || c.Access != v1.AccessOK || c.Verdict != "Refused" ||
		c.Reason != "ProviderMismatch" || c.Accepted || c.ProviderRefMatches == nil || *c.ProviderRefMatches {
		t.Errorf("providerOf = %+v; want refused, ProviderMismatch, providerRefMatches false", c)
	}
	list := decode[v1.PackageList](t, e.get(t, base+"/packages"))
	for _, it := range list.Items {
		if (it.Ref.Name == "provider") != (len(it.ProviderOf) == 1) {
			t.Errorf("package list item %s providerOf = %+v", it.Ref.Name, it.ProviderOf)
		}
	}
}

func TestHeldByPartialWhenInstancesAreNotListableEverywhere(t *testing.T) {
	onlyDefault := func(_ string, ra authorizationv1.ResourceAttributes) bool {
		return ra.Resource != "moduleinstances" || ra.Verb != verbList || ra.Namespace == "default"
	}
	e := newEnv(t, loadF1(t), onlyDefault)
	for _, r := range decode[v1.Platform](t, e.get(t, base+"/platform")).Registrations {
		if !r.HeldByPartial {
			t.Errorf("%s: heldByPartial false; the caller could not list instances everywhere", r.Name)
		}
	}
}

// TestJoinedFieldsFollowChangesOnBothSides: a registration's change reaches
// its holder's topics, and a holder dropping its claim reaches the
// Platform. The registration loses its instance label first, so it reaches
// backup-provider only through the join.
func TestJoinedFieldsFollowChangesOnBothSides(t *testing.T) {
	objs := loadF1(t)
	reg := readmodeltest.Find(t, objs, "TransformerRegistration", "default.backup-provider")
	reg.SetLabels(nil)
	if err := unstructured.SetNestedField(reg.Object, false, "status", "active"); err != nil {
		t.Fatal(err)
	}
	e := newEnv(t, objs, readmodeltest.AllowAll, fastStream)
	ts := newHTTPServer(t, e)
	c, res := openStream(t, ts, "instance:default/backup-provider,instances,platform")
	if c == nil {
		t.Fatalf("stream refused: %d %s", res.status, res.body)
	}
	for range 3 {
		c.next(t, func(ev sse, _ message) bool { return ev.event == stream.EventSnapshot })
	}

	// The claim turns active.
	updateObject(t, e, registrationsGVR, "", "default.backup-provider", func(u *unstructured.Unstructured) {
		_ = unstructured.SetNestedField(u.Object, true, "status", "active")
	})
	// The instance and the list topic are published in either order.
	seen := map[string]bool{}
	for len(seen) < 2 {
		c.next(t, func(ev sse, m message) bool {
			if ev.event == stream.EventUpsert && showsTheClaimActive(m) {
				seen[m.Topic] = true
				return true
			}
			return false
		})
	}

	// backup-provider's inventory stops holding the claim.
	updateObject(t, e, instancesGVR, "default", "backup-provider", func(u *unstructured.Unstructured) {
		_ = unstructured.SetNestedSlice(u.Object, []any{}, "status", "inventory", "entries")
	})
	c.next(t, func(ev sse, m message) bool {
		if ev.event != stream.EventUpsert || m.Topic != "platform" {
			return false
		}
		var p v1.Platform
		if json.Unmarshal(m.Item, &p) != nil {
			return false
		}
		i := slices.IndexFunc(p.Registrations, func(r v1.Registration) bool { return r.Name == "default.backup-provider" })
		return i >= 0 && p.Registrations[i].HeldBy == nil
	})
}

// showsTheClaimActive reports whether m is backup-provider's instance, or
// an instance list holding it, whose one provider claim is active.
func showsTheClaimActive(m message) bool {
	active := func(claims []v1.ProviderClaim) bool { return len(claims) == 1 && claims[0].Active }
	switch m.Topic {
	case "instance:default/backup-provider":
		var d v1.Instance
		return json.Unmarshal(m.Item, &d) == nil && active(d.ProviderOf)
	case "instances":
		var l v1.InstanceList
		if json.Unmarshal(m.Item, &l) != nil {
			return false
		}
		i := slices.IndexFunc(l.Items, func(s v1.InstanceSummary) bool { return s.Ref.Name == "backup-provider" })
		return i >= 0 && active(l.Items[i].ProviderOf)
	}
	return false
}

// TestJoinedChangesLeaveEventsTopicsAlone: a joined change marks the
// document topics only.
func TestJoinedChangesLeaveEventsTopicsAlone(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll, func(cfg *Config) {
		cfg.Coalesce, cfg.Refresh = time.Hour, time.Hour
	})
	p := e.srv.producer
	names := []string{"instance:default/backup-provider", "events:instance:default/backup-provider",
		"instances", "instances:default", "platform", "events:platform", "package:pkg/podinfo", "events:package:pkg/podinfo"}
	releases := make([]func(), 0, len(names))
	for _, name := range names {
		topic, err := stream.ParseTopic(name)
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, p.Activate(topic))
	}
	defer func() {
		for _, r := range releases {
			r()
		}
	}()
	dirty := func() []string {
		p.mu.Lock()
		defer p.mu.Unlock()
		out := make([]string, 0, len(p.dirty))
		for t := range p.dirty {
			out = append(out, t.String())
		}
		slices.Sort(out)
		p.dirty = map[stream.Topic]bool{}
		return out
	}
	dirty()
	for _, tt := range []struct {
		change readmodel.Change
		want   []string
	}{
		{readmodel.Change{Kind: readmodel.ChangeInstance, Namespace: "default", Name: "backup-provider", Joined: true},
			[]string{"instance:default/backup-provider", "instances", "instances:default"}},
		{readmodel.Change{Kind: readmodel.ChangePackage, Namespace: "pkg", Name: "podinfo", Joined: true}, []string{"package:pkg/podinfo"}},
		{readmodel.Change{Kind: readmodel.ChangePlatform, Name: "cluster", Joined: true}, []string{"platform"}},
	} {
		p.changed(tt.change)
		if got := dirty(); !slices.Equal(got, tt.want) {
			t.Errorf("joined %+v marked %v; want %v", tt.change, got, tt.want)
		}
	}
}

// updateObject reads an object from the fake cluster, changes it and writes it
// back.
func updateObject(t *testing.T, e *env, gvr schema.GroupVersionResource, namespace, name string, change func(*unstructured.Unstructured)) {
	t.Helper()
	u, err := e.client.Resource(gvr).Namespace(namespace).Get(t.Context(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	change(u)
	if _, err := e.client.Resource(gvr).Namespace(namespace).Update(t.Context(), u, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
}
