package api

import (
	"bytes"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

const (
	podinfoAll     = base + "/instances/default/podinfo/events?scope=all"
	certManagerAll = base + "/instances/cert-manager/cert-manager/events?scope=all"
	podinfoPackAll = base + "/packages/pkg/podinfo/events?scope=all"
)

// The two Pods of F1's podinfo, and the ReplicaSet that made them.
const (
	podinfoPodA = "podinfo-podinfo-d9585d794-4lg6h"
	podinfoPodB = "podinfo-podinfo-d9585d794-kp2jx"
)

func listsOf(e *env) int {
	n := 0
	for _, line := range readmodeltest.Actions(e.client) {
		if strings.HasPrefix(line, "list events default/") {
			n++
		}
	}
	return n
}

func kindsOf(l v1.EventList) map[string]int {
	out := map[string]int{}
	for i := range l.Items {
		out[l.Items[i].Regarding.Kind]++
	}
	return out
}

// TestMergedEventsPodinfo: podinfo's merged feed on F1 holds the instance's
// own events, the Deployment's ScalingReplicaSet and the ReplicaSet's and
// Pods' events, newest first, with no partial mark, read with one list per
// regarded kind.
func TestMergedEventsPodinfo(t *testing.T) { //nolint:gocyclo // one test, one assertion for each branch of the case table
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
	l := decode[v1.EventList](t, e.get(t, podinfoAll))
	if l.Scope != v1.EventScopeAll || l.Partial || l.Regarding.Name != "podinfo" || l.Regarding.Kind != "ModuleInstance" {
		t.Errorf("scope %q, partial %v, regarding %+v", l.Scope, l.Partial, l.Regarding)
	}
	kinds := kindsOf(l)
	for _, kind := range []string{"ModuleInstance", "Deployment", "ReplicaSet", "Pod"} {
		if kinds[kind] == 0 {
			t.Errorf("no event about a %s in %v", kind, kinds)
		}
	}
	var scaling bool
	for i := range l.Items {
		ev := &l.Items[i]
		scaling = scaling || (ev.Regarding.Kind == "Deployment" && ev.Reason == "ScalingReplicaSet")
		if i > 0 && l.Items[i-1].LastSeen != nil && ev.LastSeen != nil && ev.LastSeen.After(*l.Items[i-1].LastSeen) {
			t.Errorf("item %d (%s %s) is newer than the item before it", i, ev.Regarding.Kind, ev.Reason)
		}
	}
	if !scaling {
		t.Error("the Deployment's ScalingReplicaSet event is missing")
	}
	// One list per regarded kind: instance, Deployment, ReplicaSet, Pod, Service.
	if n := listsOf(e); n != 5 {
		t.Errorf("%d event lists in default, want 5 (one per regarded kind)", n)
	}
	own := decode[v1.EventList](t, e.get(t, base+"/instances/default/podinfo/events"))
	if own.Scope != "" || own.Partial || kindsOf(own)["Pod"] != 0 {
		t.Errorf("the owner's own feed changed: scope %q, partial %v, kinds %v", own.Scope, own.Partial, kindsOf(own))
	}
	pkg := decode[v1.EventList](t, e.get(t, podinfoPackAll))
	if pkg.Scope != v1.EventScopeAll || pkg.Partial || pkg.Regarding.Kind != "ModulePackage" || len(pkg.Items) == 0 {
		t.Errorf("package merged feed: scope %q, partial %v, regarding %+v, %d items", pkg.Scope, pkg.Partial, pkg.Regarding, len(pkg.Items))
	}
}

// TestMergedEventsHideWhatTheCallerCannotGet (OP-A1, portal:D7:R4). The
// runtime children of an owner are read all or none for each owner: the
// caller's view holds them only when the caller may list ReplicaSets, Pods
// and Jobs together. An event is kept only for an object the caller's own
// view of the owner lists. Both rows state their grant.
func TestMergedEventsHideWhatTheCallerCannotGet(t *testing.T) { //nolint:gocyclo // one test, one assertion for each branch of the case table
	denyEverythingOnPods := func(who string, ra authorizationv1.ResourceAttributes) bool {
		return who != alice.Username || ra.Resource != "pods"
	}
	denyGetOnPods := func(who string, ra authorizationv1.ResourceAttributes) bool {
		return who != alice.Username || ra.Verb != "get" || ra.Resource != "pods"
	}

	t.Run("row 1: a caller denied every verb on Pods loses every runtime child", func(t *testing.T) {
		// Grant: alice may get the instance and its Deployment, list events in
		// default, and list ReplicaSets and Jobs; every verb on Pods is denied.
		// bob may do everything.
		e := newEnv(t, loadF1(t), denyEverythingOnPods)
		a := e.get(t, podinfoAll)
		e.principal = Principal{Identity: bob, Session: "session-bob"}
		b := e.get(t, podinfoAll)
		listA, listB := decode[v1.EventList](t, a), decode[v1.EventList](t, b)

		kindsB := kindsOf(listB)
		if kindsB["ReplicaSet"] == 0 || kindsB["Pod"] == 0 || listB.Partial {
			t.Fatalf("bob's list: kinds %v, partial %v; want the ReplicaSet's and the Pods' events and no partial", kindsB, listB.Partial)
		}
		for _, kind := range []string{"ReplicaSet", "Pod", "Job"} {
			if n := kindsOf(listA)[kind]; n != 0 {
				t.Errorf("alice's list holds %d events about a %s", n, kind)
			}
		}
		if kindsOf(listA)["Deployment"] == 0 {
			t.Error("alice's list lost the Deployment's own events")
		}
		for _, leak := range []string{podinfoPodA, podinfoPodB, "Created pod", "4lg6h", "kp2jx"} {
			if bytes.Contains(a.body, []byte(leak)) {
				t.Errorf("alice's body holds %q", leak)
			}
		}
		if !listA.Partial {
			t.Error("alice's list is not marked partial")
		}
		// Counts are taken after the filter: alice's items are bob's without
		// the items about a ReplicaSet, a Pod or a Job.
		var want []v1.Event
		for _, ev := range listB.Items {
			if !slices.Contains([]string{"ReplicaSet", "Pod", "Job"}, ev.Regarding.Kind) {
				want = append(want, ev)
			}
		}
		if len(want) != len(listA.Items) {
			t.Fatalf("alice has %d items, bob's without the children %d", len(listA.Items), len(want))
		}
		for i := range want {
			if want[i].Reason != listA.Items[i].Reason || want[i].Count != listA.Items[i].Count || want[i].Regarding != listA.Items[i].Regarding {
				t.Errorf("item %d: alice %+v, bob's %+v", i, listA.Items[i], want[i])
			}
		}
		// One Pod's events through the object parameters are refused as today.
		e.principal = Principal{Identity: alice, Session: "session-alice"}
		one := e.get(t, base+"/instances/default/podinfo/events?kind=Pod&namespace=default&name="+podinfoPodA)
		expectProblem(t, one, http.StatusForbidden, v1.CodeForbidden)
	})

	t.Run("row 2: list-not-get, every Pod in the events is a Pod of the caller's own view", func(t *testing.T) {
		// Grant: alice may get the instance and its Deployment, list events, and
		// list ReplicaSets, Pods and Jobs in default; only get on Pods is denied.
		e := newEnv(t, loadF1(t), denyGetOnPods)
		list := decode[v1.EventList](t, e.get(t, podinfoAll))
		doc := decode[v1.Instance](t, e.get(t, base+"/instances/default/podinfo"))
		inView := map[string]bool{}
		for _, c := range doc.Components {
			for _, o := range c.Objects {
				for _, ch := range o.Children {
					if ch.Ref.Kind == "Pod" {
						inView[ch.Ref.Name] = true
					}
				}
			}
		}
		if len(inView) == 0 {
			t.Fatal("alice's instance document lists no Pod, so the row proves nothing")
		}
		pods := 0
		for _, ev := range list.Items {
			if ev.Regarding.Kind != "Pod" {
				continue
			}
			pods++
			if !inView[ev.Regarding.Name] {
				t.Errorf("an event about Pod %s, which alice's own view does not list", ev.Regarding.Name)
			}
		}
		if pods == 0 {
			t.Error("no Pod event at all: a caller who may list the Pods sees them on the Resources tab, so their events follow")
		}
	})
}

// TestMergedEventsLeaveOutAnObjectTheCallerCannotRead: an inventory object
// the caller may not get is left out of the view, and of the feed with its
// events.
func TestMergedEventsLeaveOutAnObjectTheCallerCannotRead(t *testing.T) {
	noService := func(_ string, ra authorizationv1.ResourceAttributes) bool { return ra.Resource != "services" }
	e := newEnv(t, loadF1(t), noService)
	l := decode[v1.EventList](t, e.get(t, podinfoAll))
	if kindsOf(l)["Service"] != 0 || kindsOf(l)["Deployment"] == 0 {
		t.Errorf("kinds = %v; want the Deployment's events and none about the Service", kindsOf(l))
	}
	if n := listsOf(e); n != 4 {
		t.Errorf("%d event lists in default, want 4: no list for the Service the caller cannot read", n)
	}
	// The feed is not everything the owner reaches, and the caller is told so
	// (portal:D9:R6): a reader of `partial` absent takes the feed for complete.
	if !l.Partial {
		t.Error("partial = false for a feed that left out an inventory object the caller may not read")
	}
	// A caller who reads every object gets no such mark.
	if all := decode[v1.EventList](t, newEnv(t, loadF1(t), readmodeltest.AllowAll).get(t, podinfoAll)); all.Partial {
		t.Error("partial for a caller who reads every object")
	}
}

// TestMergedEventsSecretOmissionIsNotPartial: a core Secret in the inventory
// is left out for every caller, whatever the caller's grants (portal:D8), so
// the omission says nothing about the caller and does not mark the feed
// partial; and no Secret is read or reviewed.
func TestMergedEventsSecretOmissionIsNotPartial(t *testing.T) {
	objs := loadF1(t)
	podinfo := readmodeltest.Find(t, objs, "ModuleInstance", "podinfo")
	entries, _, err := unstructured.NestedSlice(podinfo.Object, "status", "inventory", "entries")
	if err != nil {
		t.Fatal(err)
	}
	entries = append(entries, map[string]any{"component": "app", "kind": "Secret", "v": "v1", "namespace": "default", "name": "db-password"})
	if err := unstructured.SetNestedSlice(podinfo.Object, entries, "status", "inventory", "entries"); err != nil {
		t.Fatal(err)
	}
	for name, rule := range map[string]readmodeltest.Rule{
		"a caller who reads everything": readmodeltest.AllowAll,
		"a caller denied secrets":       readmodeltest.DenyResources("secrets"),
	} {
		e := newEnv(t, objs, rule)
		l := decode[v1.EventList](t, e.get(t, podinfoAll))
		if l.Partial || len(l.Items) == 0 || kindsOf(l)["Secret"] != 0 {
			t.Errorf("%s: partial %v, %d items, kinds %v; want the usual feed, not partial, nothing about a Secret", name, l.Partial, len(l.Items), kindsOf(l))
		}
	}
}

// TestMergedEventsProblems: every row of the problem table of design.md.
func TestMergedEventsProblems(t *testing.T) { //nolint:gocyclo // one test, one assertion for each branch of the case table
	forbidden := newEnv(t, loadF1(t), denyAll).get(t, base+"/instances/default/podinfo").body

	t.Run("not signed in", func(t *testing.T) {
		e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
		e.principal = Principal{}
		expectProblem(t, e.get(t, podinfoAll), http.StatusUnauthorized, v1.CodeUnauthenticated)
	})
	t.Run("get on the owner denied, whether or not it exists", func(t *testing.T) {
		e := newEnv(t, loadF1(t), readmodeltest.DenyResources("moduleinstances"))
		existing, missing := e.get(t, podinfoAll), e.get(t, base+"/instances/default/nothing/events?scope=all")
		expectProblem(t, existing, http.StatusForbidden, v1.CodeForbidden)
		if existing.status != missing.status || !bytes.Equal(existing.body, missing.body) || !bytes.Equal(existing.body, forbidden) {
			t.Errorf("existing and missing owners differ:\n%s\n%s", existing.body, missing.body)
		}
	})
	t.Run("list of events denied in the owner's namespace, before any lookup", func(t *testing.T) {
		e := newEnv(t, loadF1(t), readmodeltest.DenyResources("events"))
		res := e.get(t, podinfoAll)
		expectProblem(t, res, http.StatusForbidden, v1.CodeForbidden)
		if !bytes.Equal(res.body, forbidden) {
			t.Errorf("not the forbidden document: %s", res.body)
		}
		missing := e.get(t, base+"/instances/default/nothing/events?scope=all")
		if !bytes.Equal(res.body, missing.body) {
			t.Errorf("a missing owner is told apart from an existing one: %s", missing.body)
		}
		want := []string{"get moduleinstances default/podinfo", "list events default/"}
		got := make([]string, 0, 2)
		for _, a := range e.az.callerAsked()[:2] {
			got = append(got, a.Verb+" "+a.Resource.Resource+" "+a.Namespace+"/"+a.Name)
		}
		if !slices.Equal(got, want) {
			t.Errorf("checks = %v, want %v first", got, want)
		}
		if n := listsOf(e); n != 0 {
			t.Errorf("%d event lists ran for a caller who may not list events", n)
		}
	})
	t.Run("the owner missing", func(t *testing.T) {
		e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
		expectProblem(t, e.get(t, base+"/instances/default/nothing/events?scope=all"), http.StatusNotFound, v1.CodeNotFound)
		expectProblem(t, e.get(t, base+"/packages/pkg/nothing/events?scope=all"), http.StatusNotFound, v1.CodeNotFound)
	})
	t.Run("the portal's reader may not list events in the owner's namespace", func(t *testing.T) {
		e := newEnvWithReader(t, loadF1(t), readmodeltest.AllowAll, readmodeltest.DenyResources("events"))
		expectProblem(t, e.get(t, podinfoAll), http.StatusServiceUnavailable, v1.CodeNotReadableByPortal)
	})
	t.Run("a review that cannot be made", func(t *testing.T) {
		e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
		e.az.setFail(func(who authz.Identity, _ authz.Attributes) bool { return who.Username == alice.Username })
		expectProblem(t, e.get(t, podinfoAll), http.StatusServiceUnavailable, v1.CodeUpstreamUnavailable)
	})
	t.Run("list denied in another namespace", func(t *testing.T) {
		noDefault := func(_ string, ra authorizationv1.ResourceAttributes) bool {
			return ra.Resource != "events" || ra.Namespace != "default"
		}
		e := newEnv(t, loadF1(t), noDefault)
		l := decode[v1.EventList](t, e.get(t, certManagerAll))
		if !l.Partial || len(l.Items) == 0 {
			t.Errorf("partial %v with %d items; want the cert-manager events and partial", l.Partial, len(l.Items))
		}
		for _, ev := range l.Items {
			if ev.Regarding.Namespace == "" {
				t.Errorf("an event about a cluster-scoped object, whose events live in default: %+v", ev)
			}
		}
		if n := listsOf(e); n != 0 {
			t.Errorf("%d lists ran in default without a review that allowed them", n)
		}
	})
	t.Run("a list fails in some namespace", func(t *testing.T) {
		e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
		e.client.PrependReactor("list", "events", func(a k8stesting.Action) (bool, runtime.Object, error) {
			if a.GetNamespace() == "default" {
				return true, nil, errors.New("connection refused")
			}
			return false, nil, nil
		})
		l := decode[v1.EventList](t, e.get(t, certManagerAll))
		if !l.Partial || len(l.Items) == 0 {
			t.Errorf("partial %v with %d items; want the cert-manager events and partial", l.Partial, len(l.Items))
		}
	})
	t.Run("a workload whose children the caller cannot read marks the feed partial", func(t *testing.T) {
		noPods := func(_ string, ra authorizationv1.ResourceAttributes) bool { return ra.Resource != "pods" }
		e := newEnv(t, loadF1(t), noPods)
		if l := decode[v1.EventList](t, e.get(t, podinfoAll)); !l.Partial {
			t.Error("not partial")
		}
		// A caller who reads every child gets no such mark.
		e = newEnv(t, loadF1(t), readmodeltest.AllowAll)
		if l := decode[v1.EventList](t, e.get(t, podinfoAll)); l.Partial {
			t.Error("partial for a caller who reads everything")
		}
	})
	t.Run("an unknown scope or an object beside scope=all", func(t *testing.T) {
		e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
		for _, query := range []string{
			"scope=everything", "scope=", "scope=ALL", "scope=all&scope=owner",
			"scope=all&kind=Deployment&name=podinfo-podinfo",
			"scope=all&kind=Deployment&namespace=default&name=podinfo-podinfo&group=apps",
			"scope=all&namespace=default", "scope=all&group=apps", "scope=all&name=podinfo-podinfo",
		} {
			for _, owner := range []string{"/instances/default/podinfo", "/packages/pkg/podinfo"} {
				expectProblem(t, e.get(t, base+owner+"/events?"+query), http.StatusBadRequest, v1.CodeBadRequest)
			}
		}
		if len(e.az.callerAsked()) != 0 {
			t.Errorf("a bad request was authorized: %v", e.az.callerAsked())
		}
	})
	t.Run("scope=owner is the owner's own feed", func(t *testing.T) {
		e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
		plain, owner := e.get(t, base+"/instances/default/podinfo/events"), e.get(t, base+"/instances/default/podinfo/events?scope=owner")
		if plain.status != http.StatusOK || !bytes.Equal(plain.body, owner.body) {
			t.Errorf("scope=owner differs from no scope:\n%s\n%s", plain.body, owner.body)
		}
		// The object parameters stay valid beside scope=owner.
		one := e.get(t, base+"/instances/default/podinfo/events?scope=owner&kind=Pod&namespace=default&name="+podinfoPodA)
		if one.status != http.StatusOK {
			t.Errorf("one object's feed beside scope=owner = %d %s", one.status, one.body)
		}
	})
}

// operatorEvent builds an event about default/podinfo, an OPM object.
func operatorEvent(name, reason, note, controller string) *unstructured.Unstructured {
	ev := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "events.k8s.io/v1",
		"kind":       "Event",
		"metadata": map[string]any{
			"name": name, "namespace": "default", "uid": name + "-uid", "creationTimestamp": "2026-10-04T18:06:00Z",
		},
		"eventTime": "2026-10-04T18:06:00.000000Z",
		"reason":    reason,
		"note":      note,
		"type":      "Normal",
		"regarding": map[string]any{
			"apiVersion": "opmodel.dev/v1alpha1", "kind": "ModuleInstance", "namespace": "default", "name": "podinfo", "uid": "podinfo-uid",
		},
	}}
	if controller != "" {
		ev.Object["reportingController"] = controller
	}
	return ev
}

// TestMergedEventsInCluster (OP-A2): in-cluster, both writers of an operator
// note lose it, the one that names its controller and the one that names none
// and regards an OPM object; the kubelet's notes are served as written. In
// local mode every note is served.
func TestMergedEventsInCluster(t *testing.T) {
	objs := append(loadF1(t),
		operatorEvent("by-the-operator", "OperatorSaid", "operator text one", "opm-controller"),
		operatorEvent("by-nobody-we-know", "SomebodySaid", "operator text two", ""),
	)
	notes := func(l v1.EventList) map[string]string {
		out := map[string]string{}
		for _, ev := range l.Items {
			if ev.Regarding.Kind == "ModuleInstance" || ev.ReportingController == "kubelet" {
				out[ev.Regarding.Kind+"/"+ev.Reason] = ev.Note
			}
		}
		return out
	}

	local := notes(decode[v1.EventList](t, newEnv(t, objs, readmodeltest.AllowAll).get(t, podinfoAll)))
	in := notes(decode[v1.EventList](t, newEnv(t, objs, readmodeltest.AllowAll, inCluster).get(t, podinfoAll)))

	for _, reason := range []string{"ModuleInstance/OperatorSaid", "ModuleInstance/SomebodySaid"} {
		if local[reason] == "" {
			t.Errorf("local mode serves no note for %s: %v", reason, local)
		}
		if note, ok := in[reason]; !ok || note != "" {
			t.Errorf("in-cluster note for %s = %q (present %v); want none", reason, note, ok)
		}
	}
	var kubelet int
	for key, note := range local {
		if !strings.HasPrefix(key, "Pod/") {
			continue
		}
		kubelet++
		if in[key] != note || note == "" {
			t.Errorf("kubelet event %s: local %q, in-cluster %q; want the same, as written", key, note, in[key])
		}
	}
	if kubelet == 0 {
		t.Fatal("no kubelet Pod event in the merged feed, so the in-cluster case proves nothing")
	}
}

// TestMergedEventsInClusterOtherObjects (OP-A2, portal:D8:R5): the in-cluster
// omission of an operator note applies to every item of the merged feed, not
// to the events about the owner only. backup-provider reaches the
// TransformerRegistration default.backup-provider, an OPM object other than
// the owner; an event of each writer about it loses its note in-cluster and
// keeps it in local mode.
func TestMergedEventsInClusterOtherObjects(t *testing.T) {
	const feed = base + "/instances/default/backup-provider/events?scope=all"
	registrationEvent := func(name, reason, note, controller string) *unstructured.Unstructured {
		ev := operatorEvent(name, reason, note, controller)
		ev.Object["regarding"] = map[string]any{
			"apiVersion": "opmodel.dev/v1alpha1", "kind": "TransformerRegistration", "name": "default.backup-provider",
		}
		return ev
	}
	objs := append(loadF1(t),
		registrationEvent("registration-by-the-operator", "RegistrationOperatorSaid", "registration text one", "opm-controller"),
		registrationEvent("registration-by-nobody-we-know", "RegistrationSomebodySaid", "registration text two", ""),
	)
	notes := func(l v1.EventList) map[string]string {
		out := map[string]string{}
		for _, ev := range l.Items {
			if ev.Regarding.Kind == "TransformerRegistration" && strings.HasPrefix(ev.Reason, "Registration") {
				out[ev.Reason] = ev.Note
			}
		}
		return out
	}
	local := notes(decode[v1.EventList](t, newEnv(t, objs, readmodeltest.AllowAll).get(t, feed)))
	in := notes(decode[v1.EventList](t, newEnv(t, objs, readmodeltest.AllowAll, inCluster).get(t, feed)))
	for _, reason := range []string{"RegistrationOperatorSaid", "RegistrationSomebodySaid"} {
		if local[reason] == "" {
			t.Errorf("local mode serves no note for %s: %v", reason, local)
		}
		if note, ok := in[reason]; !ok || note != "" {
			t.Errorf("in-cluster note for %s = %q (present %v); want the event with no note", reason, note, ok)
		}
	}
}

// cappedNote is n copies of one code point of three bytes.
func cappedNote(n int) string { return strings.Repeat("€", n) }

// TestEventNoteIsCapped (OP-A2): eventListDoc caps every note it serves at
// 512 Unicode code points, cuts between code points, serves markup as text,
// and leaves a note at the limit alone. Two notes that share their first 511
// code points are two lines from the model and stay two lines.
func TestEventNoteIsCapped(t *testing.T) {
	about := readmodel.ObjectRef{Group: opmGroup, Version: opmVersion, Kind: kindModuleInstance, Namespace: "default", Name: "podinfo"}
	cases := []struct {
		name string
		note string
		want string
	}{
		{"2,000 code points", strings.Repeat("a", 2000), strings.Repeat("a", 511) + "…"},
		{"at the limit", strings.Repeat("a", 512), strings.Repeat("a", 512)},
		{"one over", strings.Repeat("a", 513), strings.Repeat("a", 511) + "…"},
		{"multi-byte, 600 code points", cappedNote(600), cappedNote(511) + "…"},
		{"markup, never executed", "<script>alert(1)</script>" + strings.Repeat("a", 600), "<script>alert(1)</script>" + strings.Repeat("a", 486) + "…"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := eventListDoc(about, []readmodel.Event{{Type: "Normal", Reason: "R", Note: tc.note, Count: 1, Regarding: about}})
			got := doc.Items[0].Note
			if got != tc.want || !utf8.ValidString(got) || utf8.RuneCountInString(got) > 512 {
				t.Errorf("note = %.30q... (%d code points), want %.30q...", got, utf8.RuneCountInString(got), tc.want)
			}
		})
	}
	t.Run("two notes sharing their first 511 code points", func(t *testing.T) {
		shared := strings.Repeat("a", 511)
		doc := eventListDoc(about, []readmodel.Event{
			{Type: "Normal", Reason: "R", Note: shared + "xx", Count: 3, Regarding: about},
			{Type: "Normal", Reason: "R", Note: shared + "yy", Count: 4, Regarding: about},
		})
		if len(doc.Items) != 2 || doc.Items[0].Note != doc.Items[1].Note || doc.Items[0].Count != 3 || doc.Items[1].Count != 4 {
			t.Errorf("items = %+v; want two lines with equal notes and their own counts", doc.Items)
		}
	})
}

// TestCappedNotesOnEveryRoute: the cap holds for the owner's feed, one
// object's feed and the merged feed, and the stream snapshot serves the same
// document.
func TestCappedNotesOnEveryRoute(t *testing.T) {
	long := strings.Repeat("€", 600)
	objs := append(loadF1(t), longEvent("long-podinfo-note", "Deployment", "apps/v1", "podinfo-podinfo", "deployment-controller", long, "x"))
	e := newEnv(t, objs, readmodeltest.AllowAll)
	for _, path := range []string{
		podinfoAll,
		base + "/instances/default/podinfo/events?scope=owner&group=apps&kind=Deployment&namespace=default&name=podinfo-podinfo",
		base + "/instances/default/podinfo/events?group=apps&kind=Deployment&namespace=default&name=podinfo-podinfo",
	} {
		l := decode[v1.EventList](t, e.get(t, path))
		found := false
		for _, ev := range l.Items {
			if ev.Reason == "LongNote" {
				found = true
				if utf8.RuneCountInString(ev.Note) != 512 || !strings.HasSuffix(ev.Note, "…") {
					t.Errorf("%s: note has %d code points: %.20q", path, utf8.RuneCountInString(ev.Note), ev.Note)
				}
			}
		}
		if !found {
			t.Errorf("%s: the long-note event is missing", path)
		}
	}
}

// longEvent builds an event about an object of default.
func longEvent(name, kind, apiVersion, object, controller, note, uid string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "events.k8s.io/v1",
		"kind":       "Event",
		"metadata": map[string]any{
			"name": name, "namespace": "default", "uid": name + "-" + uid, "creationTimestamp": "2026-10-04T18:07:00Z",
		},
		"eventTime":           "2026-10-04T18:07:00.000000Z",
		"reason":              "LongNote",
		"note":                note,
		"type":                "Warning",
		"reportingController": controller,
		"regarding": map[string]any{
			"apiVersion": apiVersion, "kind": kind, "namespace": "default", "name": object, "uid": "regarding-" + object,
		},
	}}
}

// TestCappedNotesFoldInClusterOnly (OP-A2): two events whose notes differ
// only after the cut are two lines in local mode, with equal notes, and one
// line in-cluster, whose second fold adds their counts.
func TestCappedNotesFoldInClusterOnly(t *testing.T) {
	shared := strings.Repeat("a", 600)
	objs := append(loadF1(t),
		longEvent("fold-one", "Deployment", "apps/v1", "podinfo-podinfo", "deployment-controller", shared+"1", "1"),
		longEvent("fold-two", "Deployment", "apps/v1", "podinfo-podinfo", "deployment-controller", shared+"2", "2"),
	)
	lines := func(e *env) []v1.Event {
		var out []v1.Event
		for _, ev := range decode[v1.EventList](t, e.get(t, podinfoAll)).Items {
			if ev.Reason == "LongNote" {
				out = append(out, ev)
			}
		}
		return out
	}
	local := lines(newEnv(t, objs, readmodeltest.AllowAll))
	if len(local) != 2 || local[0].Note != local[1].Note || local[0].Count != 1 || local[1].Count != 1 {
		t.Errorf("local: %d lines %+v; want two with equal notes and one count each", len(local), local)
	}
	in := lines(newEnv(t, objs, readmodeltest.AllowAll, inCluster))
	if len(in) != 1 || in[0].Count != 2 || utf8.RuneCountInString(in[0].Note) != 512 {
		t.Errorf("in-cluster: %d lines %+v; want one line counting 2 with the capped note", len(in), in)
	}
}
