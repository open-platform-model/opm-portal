package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

func denyAll(string, authorizationv1.ResourceAttributes) bool { return false }

func problemOf(t *testing.T, res response) v1.Problem {
	t.Helper()
	if ct := res.header.Get("Content-Type"); ct != v1.ProblemContentType {
		t.Fatalf("Content-Type = %q, want %q (body %s)", ct, v1.ProblemContentType, res.body)
	}
	var p v1.Problem
	if err := json.Unmarshal(res.body, &p); err != nil {
		t.Fatalf("problem body: %v", err)
	}
	if p.Status != res.status || p.Type != v1.ProblemTypeBlank || p.Title != http.StatusText(res.status) {
		t.Errorf("problem %+v does not match status %d", p, res.status)
	}
	return p
}

func expectProblem(t *testing.T, res response, status int, code string) v1.Problem {
	t.Helper()
	if res.status != status {
		t.Fatalf("status = %d, want %d (body %s)", res.status, status, res.body)
	}
	p := problemOf(t, res)
	if p.Code != code {
		t.Errorf("code = %q, want %q", p.Code, code)
	}
	return p
}

// TestForbiddenReadsTheSameWhetherOrNotTheObjectExists (portal:D7:R1).
func TestForbiddenReadsTheSameWhetherOrNotTheObjectExists(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.DenyResources("moduleinstances", "modulepackages", "platforms"))
	for _, pair := range [][2]string{
		{base + "/instances/default/podinfo", base + "/instances/default/nothing"},
		{base + "/instances/default/podinfo/graph", base + "/instances/default/nothing/graph"},
		{base + "/instances/default/podinfo/events", base + "/instances/default/nothing/events"},
		{base + "/packages/pkg/podinfo", base + "/packages/pkg/nothing"},
	} {
		existing, missing := e.get(t, pair[0]), e.get(t, pair[1])
		expectProblem(t, existing, http.StatusForbidden, v1.CodeForbidden)
		if existing.status != missing.status || !bytes.Equal(existing.body, missing.body) {
			t.Errorf("%s and %s differ:\n%s\n%s", pair[0], pair[1], existing.body, missing.body)
		}
		if strings.Contains(string(existing.body), "podinfo") {
			t.Errorf("the refusal names the object: %s", existing.body)
		}
	}
	expectProblem(t, e.get(t, base+"/platform"), http.StatusForbidden, v1.CodeForbidden)
}

func TestAllowedAndMissingIsNotFound(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
	for _, path := range []string{
		base + "/instances/default/nothing",
		base + "/instances/default/nothing/graph",
		base + "/instances/default/nothing/events",
		base + "/packages/pkg/nothing",
		base + "/platform/registrations/nothing/events",
	} {
		expectProblem(t, e.get(t, path), http.StatusNotFound, v1.CodeNotFound)
	}
}

// TestListsHoldOnlyWhatTheCallerMayList (portal:D7:R2, portal:D5:R5).
func TestListsHoldOnlyWhatTheCallerMayList(t *testing.T) {
	onlyDefault := func(_ string, ra authorizationv1.ResourceAttributes) bool {
		return ra.Resource != "moduleinstances" || ra.Namespace == "default"
	}
	e := newEnv(t, loadF1(t), onlyDefault)

	var all v1.InstanceList
	res := e.get(t, base+"/instances")
	if res.status != http.StatusOK {
		t.Fatalf("cluster-wide list = %d %s", res.status, res.body)
	}
	if err := json.Unmarshal(res.body, &all); err != nil {
		t.Fatal(err)
	}
	if all.Access != v1.AccessForbidden || len(all.Items) != 0 || all.Items == nil {
		t.Errorf("cluster-wide list = %+v, want no items and access forbidden", all)
	}
	if string(res.body) != `{"apiVersion":"portal.opmodel.dev/v1alpha1","kind":"InstanceList","access":"forbidden","items":[]}` {
		t.Errorf("the refused list says more than that it is refused: %s", res.body)
	}

	var ns v1.InstanceList
	if err := json.Unmarshal(e.get(t, base+"/instances?namespace=default").body, &ns); err != nil {
		t.Fatal(err)
	}
	if ns.Access != v1.AccessOK || len(ns.Items) != 3 {
		t.Errorf("namespace list = %+v, want the three instances in default", ns)
	}
	for _, it := range ns.Items {
		if it.Ref.Namespace != "default" {
			t.Errorf("an instance outside the namespace: %+v", it.Ref)
		}
	}
}

// TestPartialAccessIsMarkedPerItem (portal:D7:R3).
func TestPartialAccessIsMarkedPerItem(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.DenyResources("clusterroles"))
	var doc v1.Instance
	res := e.get(t, base+"/instances/cert-manager/cert-manager")
	if err := json.Unmarshal(res.body, &doc); err != nil || res.status != http.StatusOK {
		t.Fatalf("GET = %d %s", res.status, res.body)
	}
	var roles int
	for _, c := range doc.Components {
		for _, o := range c.Objects {
			if o.Ref.Kind != "ClusterRole" {
				continue
			}
			roles++
			if o.Access != v1.AccessForbidden || o.Health != nil {
				t.Errorf("ClusterRole %s: access %q, health %+v", o.Ref.Name, o.Access, o.Health)
			}
		}
	}
	if roles == 0 || !doc.Health.Partial || doc.Health.Counts.Forbidden != roles {
		t.Errorf("roles %d, health %+v", roles, doc.Health)
	}
}

func TestPlatformRegistrationsForbidden(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.DenyResources("transformerregistrations"))
	var doc v1.Platform
	if err := json.Unmarshal(e.get(t, base+"/platform").body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.RegistrationsAccess != v1.AccessForbidden || len(doc.Registrations) != 0 {
		t.Errorf("registrations %+v, access %q", doc.Registrations, doc.RegistrationsAccess)
	}
}

// TestPlatformGraphForbiddenProvider: providers are authorized for the
// caller, one by one, and a refused one is shown as forbidden.
func TestPlatformGraphForbiddenProvider(t *testing.T) {
	deny := func(_ string, ra authorizationv1.ResourceAttributes) bool {
		return ra.Resource != "moduleinstances" || ra.Namespace != "default"
	}
	e := newEnv(t, loadF1(t), deny)
	var g v1.Graph
	res := e.get(t, base+"/platform/graph")
	if err := json.Unmarshal(res.body, &g); err != nil || res.status != http.StatusOK {
		t.Fatalf("GET = %d %s", res.status, res.body)
	}
	provider := nodeByID(g, "mi:default/backup-provider")
	if provider == nil || provider.Access != v1.AccessForbidden || provider.Health != nil {
		t.Fatalf("provider node = %+v", provider)
	}
	for _, edge := range g.Edges {
		if edge.Kind != "providedBy" || edge.To != provider.ID {
			continue
		}
		if edge.Verified == nil || *edge.Verified || edge.Reason != "ProviderUnreadable" {
			t.Errorf("providedBy edge = %+v, want unverified ProviderUnreadable", edge)
		}
	}
	want := authz.Attributes{Verb: "get", Resource: instancesGVR, Namespace: "default", Name: "backup-provider"}
	if !slices.Contains(e.az.callerAsked(), want) {
		t.Errorf("the provider was not authorized for the caller: %v", e.az.callerAsked())
	}
}

func TestUnauthenticatedIsRefusedBeforeAnyReview(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
	for _, p := range []Principal{
		{},
		{Identity: authz.Identity{Username: "", Groups: []string{"system:masters"}}, Session: "s"},
		{Identity: authz.Identity{Username: "system:anonymous"}, Session: "s"},
		{Identity: alice, Session: " "},
	} {
		e.principal = p
		before := e.az.callerChecks()
		expectProblem(t, e.get(t, base+"/instances/default/podinfo"), http.StatusUnauthorized, v1.CodeUnauthenticated)
		expectProblem(t, e.get(t, base+"/stream?topics=platform"), http.StatusUnauthorized, v1.CodeUnauthenticated)
		if e.az.callerChecks() != before {
			t.Errorf("principal %+v: a review was sent", p)
		}
	}
}

func TestReviewFailureIsUnavailable(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
	e.az.setFail(func(who authz.Identity, _ authz.Attributes) bool { return who.Username == alice.Username })
	p := expectProblem(t, e.get(t, base+"/instances/default/podinfo"), http.StatusServiceUnavailable, v1.CodeUpstreamUnavailable)
	if strings.Contains(p.Detail, "alice") {
		t.Errorf("the problem names the identity: %+v", p)
	}
	expectProblem(t, e.get(t, base+"/instances"), http.StatusServiceUnavailable, v1.CodeUpstreamUnavailable)
}

func TestRefusedBeforeAnyReview(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
	for _, c := range []struct {
		method, path string
		status       int
		code         string
	}{
		{http.MethodGet, Prefix + "/clusters/prod/instances", http.StatusNotFound, v1.CodeNotFound},
		{http.MethodGet, base + "/instances/Default/podinfo", http.StatusBadRequest, v1.CodeBadRequest},
		{http.MethodGet, base + "/instances/default/Podinfo", http.StatusBadRequest, v1.CodeBadRequest},
		{http.MethodGet, base + "/instances?namespace=a_b", http.StatusBadRequest, v1.CodeBadRequest},
		{http.MethodGet, base + "/instances/default/podinfo/graph?showScaledDown=maybe", http.StatusBadRequest, v1.CodeBadRequest},
		{http.MethodGet, base + "/instances/default/podinfo/events?kind=Pod", http.StatusBadRequest, v1.CodeBadRequest},
		{http.MethodGet, base + "/nothing", http.StatusNotFound, v1.CodeNotFound},
		{http.MethodPost, base + "/instances", http.StatusMethodNotAllowed, v1.CodeMethodNotAllowed},
		{http.MethodDelete, base + "/instances/default/podinfo", http.StatusMethodNotAllowed, v1.CodeMethodNotAllowed},
	} {
		before := e.az.callerChecks()
		res := e.do(t, c.method, c.path)
		expectProblem(t, res, c.status, c.code)
		if e.az.callerChecks() != before {
			t.Errorf("%s %s: a review was sent", c.method, c.path)
		}
		if c.status == http.StatusMethodNotAllowed && res.header.Get("Allow") != http.MethodGet {
			t.Errorf("%s %s: Allow = %q", c.method, c.path, res.header.Get("Allow"))
		}
	}
}

// TestEventsOnlyAboutReachedObjects (portal:D7:R4): an object no inventory
// reaches is refused with the forbidden problem itself, and only after the
// caller's checks; its events are never listed.
func TestEventsOnlyAboutReachedObjects(t *testing.T) {
	forbiddenBody := newEnv(t, loadF1(t), denyAll).get(t, base+"/instances/default/podinfo").body

	e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
	for _, path := range []string{
		base + "/instances/cert-manager/cert-manager/events?group=coordination.k8s.io&kind=Lease&namespace=cert-manager&name=cert-manager-controller",
		base + "/instances/default/podinfo/events?group=apps&kind=Deployment&namespace=cert-manager&name=cert-manager-webhook",
		base + "/instances/default/podinfo/events?group=example.com&kind=Widget&namespace=default&name=w",
		base + "/instances/default/podinfo/events?kind=Secret&namespace=default&name=podinfo",
	} {
		res := e.get(t, path)
		if res.status != http.StatusForbidden || !bytes.Equal(res.body, forbiddenBody) {
			t.Errorf("GET %s = %d %s, want the forbidden problem", path, res.status, res.body)
		}
	}
	for _, line := range readmodeltest.Actions(e.client) {
		if strings.HasPrefix(line, "list events cert-manager") {
			t.Errorf("events of an unreached object were listed: %s", line)
		}
	}
}

func TestEventsAuthorizeEveryReadFirst(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.DenyResources("events"))
	path := base + "/instances/default/podinfo/events?group=apps&kind=Deployment&namespace=default&name=podinfo-podinfo"
	expectProblem(t, e.get(t, path), http.StatusForbidden, v1.CodeForbidden)
	asked := e.az.callerAsked()
	want := []string{"get moduleinstances default/podinfo", "get deployments default/podinfo-podinfo", "list events default/"}
	got := make([]string, 0, len(asked))
	for _, a := range asked {
		got = append(got, a.Verb+" "+a.Resource.Resource+" "+a.Namespace+"/"+a.Name)
	}
	if !slices.Equal(got, want) {
		t.Errorf("checks = %v, want %v", got, want)
	}
	for _, line := range readmodeltest.Actions(e.client) {
		if strings.HasPrefix(line, "list events") {
			t.Errorf("events listed for a caller who may not list them: %s", line)
		}
	}
}

// TestEventsNamedKindNeverRefreshesDiscovery: a caller who may not read
// the owner is refused before the kind they name is looked up, and a kind
// the cluster does not serve never refreshes the discovery every reader
// shares, whoever asks.
func TestEventsNamedKindNeverRefreshesDiscovery(t *testing.T) {
	for _, rule := range []readmodeltest.Rule{denyAll, readmodeltest.AllowAll} {
		e := newEnv(t, loadF1(t), rule)
		e.get(t, base+"/instances/default/podinfo")
		before := len(e.disc.Actions())
		for _, kind := range []string{"Widgeta", "Widgetb", "Widgetc"} {
			res := e.get(t, base+"/instances/default/podinfo/events?group=example.com&kind="+kind+"&namespace=default&name=w")
			expectProblem(t, res, http.StatusForbidden, v1.CodeForbidden)
		}
		if n := len(e.disc.Actions()) - before; n != 0 {
			t.Errorf("unknown kinds sent %d discovery requests", n)
		}
	}
}

func nodeByID(g v1.Graph, id string) *v1.GraphNode {
	for i := range g.Nodes {
		if g.Nodes[i].ID == id {
			return &g.Nodes[i]
		}
	}
	return nil
}

// TestPortalCannotReadIsItsOwnCode: when the reading identity may not list
// a kind the caller may read, the answer says the portal cannot read it.
func TestPortalCannotReadIsItsOwnCode(t *testing.T) {
	e := newEnvWithReader(t, loadF1(t), readmodeltest.AllowAll, readmodeltest.DenyResources("modulepackages"))
	expectProblem(t, e.get(t, base+"/packages/pkg/podinfo"), http.StatusServiceUnavailable, v1.CodeNotReadableByPortal)
	expectProblem(t, e.get(t, base+"/packages"), http.StatusServiceUnavailable, v1.CodeNotReadableByPortal)
}

// TestGraphOptions: expand shows a group's members; showScaledDown is read.
func TestGraphOptions(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
	graphOf := func(path string) v1.Graph {
		t.Helper()
		var g v1.Graph
		res := e.get(t, path)
		if err := json.Unmarshal(res.body, &g); err != nil || res.status != http.StatusOK {
			t.Fatalf("GET %s = %d %s", path, res.status, res.body)
		}
		return g
	}
	collapsed := graphOf(base + "/instances/cert-manager/cert-manager/graph")
	var group string
	for _, n := range collapsed.Nodes {
		if n.Group != nil && n.Group.Kind == "configuration" {
			group = n.ID
		}
	}
	if group == "" {
		t.Fatal("no configuration group in the collapsed graph")
	}
	expanded := graphOf(base + "/instances/cert-manager/cert-manager/graph?expand=" + url.QueryEscape(group) + "&showScaledDown=true")
	members := 0
	for _, n := range expanded.Nodes {
		if n.MemberOf == group {
			members++
		}
	}
	if members == 0 || len(expanded.Nodes) <= len(collapsed.Nodes) {
		t.Errorf("expanding %s: %d members, %d nodes against %d", group, members, len(expanded.Nodes), len(collapsed.Nodes))
	}
}
