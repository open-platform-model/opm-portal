package api

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

// inCluster serves as in-cluster mode would: with in-cluster credentials.
func inCluster(cfg *Config) {
	cfg.Mode = ModeInCluster
	cfg.Connection = Connection{Source: v1.SourceInCluster}
}

// The backup-provider instance reaches a TransformerRegistration: an OPM
// object whose raw status carries operator text.
const backupRegistrationObject = base + "/instances/default/backup-provider/object?group=opmodel.dev&kind=TransformerRegistration&name=default.backup-provider"

// inClusterGoldens are the documents that carry operator text in local
// mode, served in-cluster. Their local goldens are the same names without
// the directory.
var inClusterGoldens = []goldenCase{
	{"in-cluster/cluster", base},
	{"in-cluster/instance-list", base + "/instances"},
	{"in-cluster/instance-podinfo", base + "/instances/default/podinfo"},
	{"in-cluster/instance-backup-provider", base + "/instances/default/backup-provider"},
	{"in-cluster/instance-backup-provider-object-registration", backupRegistrationObject},
	{"in-cluster/instance-podinfo-graph", base + "/instances/default/podinfo/graph"},
	{"in-cluster/instance-podinfo-events", base + "/instances/default/podinfo/events"},
	{"in-cluster/instance-podinfo-events-pod", base + "/instances/default/podinfo/events?kind=Pod&namespace=default&name=podinfo-podinfo-d9585d794-4lg6h"},
	{"in-cluster/package-list", base + "/packages"},
	{"in-cluster/package-podinfo", base + "/packages/pkg/podinfo"},
	{"in-cluster/package-podinfo-graph", base + "/packages/pkg/podinfo/graph"},
	{"in-cluster/platform", base + "/platform"},
	{"in-cluster/platform-graph", base + "/platform/graph"},
	{"in-cluster/platform-events", base + "/platform/events"},
	{"in-cluster/registration-refused-events", base + "/platform/registrations/default.refused-claim-fixture/events"},
}

// TestGoldenInCluster serves the F1 documents that carry operator text from
// an in-cluster server: the same documents as local mode, without it.
func TestGoldenInCluster(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll, inCluster)
	checkGoldens(t, e, inClusterGoldens)
	for _, c := range inClusterGoldens {
		assertNoOperatorText(t, c.path, e.get(t, c.path).body)
	}
}

func TestGoldenBrokenRolloutInCluster(t *testing.T) {
	e := newEnv(t, f1Broken(t), readmodeltest.AllowAll, inCluster)
	checkGoldens(t, e, []goldenCase{{"in-cluster/instance-podinfo-broken", base + "/instances/default/podinfo"}})
	assertNoOperatorText(t, "broken podinfo", e.get(t, base+"/instances/default/podinfo").body)
}

// TestInClusterEventsDifferingOnlyInNoteFoldIntoOneRow: the operator's
// Applied note counts the resources each apply touched, so two applies
// leave two Applied events. In-cluster their notes are dropped, and they
// are served as one row with both counted: two rows that look the same
// would tell how many distinct notes were hidden.
func TestInClusterEventsDifferingOnlyInNoteFoldIntoOneRow(t *testing.T) {
	const path = base + "/instances/default/podinfo/events"
	objs := withASecondApplied(t, loadF1(t))

	e := newEnv(t, objs, readmodeltest.AllowAll, inCluster)
	checkGoldens(t, e, []goldenCase{{"in-cluster/instance-podinfo-events-two-applied", path}})
	assertNoOperatorText(t, path, e.get(t, path).body)
	if rows, count := appliedRows(t, e.get(t, path).body); rows != 1 || count != 2 {
		t.Errorf("in-cluster: %d Applied rows counting %d, want one row counting 2", rows, count)
	}

	local := newEnv(t, objs, readmodeltest.AllowAll)
	if rows, _ := appliedRows(t, local.get(t, path).body); rows != 2 {
		t.Errorf("local: %d Applied rows, want 2 (one per note)", rows)
	}
}

// withASecondApplied adds to objs a later Applied event about
// default/podinfo whose note differs from the first one's.
func withASecondApplied(t *testing.T, objs []*unstructured.Unstructured) []*unstructured.Unstructured {
	t.Helper()
	for _, o := range objs {
		if o.GetKind() != "Event" || o.Object["reason"] != "Applied" {
			continue
		}
		r, _ := o.Object["regarding"].(map[string]any)
		if r["kind"] != "ModuleInstance" || r["namespace"] != "default" || r["name"] != "podinfo" {
			continue
		}
		ev := o.DeepCopy()
		ev.SetName(o.GetName() + "-again")
		ev.SetUID(types.UID(string(o.GetUID()) + "-again"))
		ev.Object["note"] = "Applied 3 resources (0 created, 3 updated, 0 unchanged)"
		ev.Object["eventTime"] = "2026-10-04T18:05:00.000000Z"
		return append(objs, ev)
	}
	t.Fatal("F1 has no Applied event about default/podinfo")
	return nil
}

func appliedRows(t *testing.T, body []byte) (rows int, count int64) {
	t.Helper()
	var doc v1.EventList
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	for i := range doc.Items {
		if ev := &doc.Items[i]; ev.Reason == "Applied" {
			rows++
			count += ev.Count
		}
	}
	return rows, count
}

// TestEveryDocumentInClusterCarriesNoOperatorText reads every F1 resource
// from an in-cluster server and fails on any field that carries operator
// text, while local mode serves at least one of each kind of field, so the
// walk is not vacuous.
func TestEveryDocumentInClusterCarriesNoOperatorText(t *testing.T) {
	cases := f1Goldens
	in := newEnv(t, loadF1(t), readmodeltest.AllowAll, inCluster)
	local := newEnv(t, loadF1(t), readmodeltest.AllowAll)
	seen := map[string]bool{}
	for _, c := range cases {
		res := in.get(t, c.path)
		if res.status != http.StatusOK {
			t.Fatalf("in-cluster GET %s = %d %s", c.path, res.status, res.body)
		}
		assertNoOperatorText(t, c.path, res.body)
		for _, f := range operatorTextFields(t, local.get(t, c.path).body) {
			seen[f] = true
		}
	}
	for _, f := range []string{"conditions[].message", "reconcile.message", "history[].message", "note (operator)", "registration.message", "health.message (OPM object)", "status.conditions[].message (OPM object)"} {
		if !seen[f] {
			t.Errorf("no local document carries %s: the in-cluster walk proves nothing for it", f)
		}
	}
}

// TestTheStreamCarriesTheInClusterDocuments: a topic's snapshot is the
// in-cluster GET document.
func TestTheStreamCarriesTheInClusterDocuments(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll, fastStream, inCluster)
	ts := newHTTPServer(t, e)
	topics := map[string]string{
		"platform":                        base + "/platform",
		"instance:default/podinfo":        base + "/instances/default/podinfo",
		"events:instance:default/podinfo": base + "/instances/default/podinfo/events",
	}
	names := make([]string, 0, len(topics))
	for name := range topics {
		names = append(names, name)
	}
	c, res := openStream(t, ts, strings.Join(names, ","))
	if c == nil {
		t.Fatalf("stream refused: %d %s", res.status, res.body)
	}
	for range topics {
		m := c.next(t, func(ev sse, _ message) bool { return ev.event == stream.EventSnapshot })
		path := topics[m.Topic]
		if len(m.Items) != 1 {
			t.Fatalf("snapshot %s with %d items", m.Topic, len(m.Items))
		}
		assertNoOperatorText(t, m.Topic, m.Items[0])
		if got, want := compact(t, m.Items[0]), compact(t, e.get(t, path).body); got != want {
			t.Errorf("snapshot of %s differs from GET %s:\n%s\n%s", m.Topic, path, got, want)
		}
	}
}

// TestEveryRouteDocumentIsKnownToTheOmission: a document type the omission
// does not know would fail every in-cluster request for it, so each route's
// type must be one it handles.
func TestEveryRouteDocumentIsKnownToTheOmission(t *testing.T) {
	for _, rt := range routes {
		if rt.doc == nil {
			continue
		}
		if _, err := omitOperatorText(rt.doc); err != nil {
			t.Errorf("%s: %v", rt.pattern, err)
		}
	}
	if _, err := omitOperatorText(v1.Removed{}); err != nil {
		t.Errorf("Removed: %v", err)
	}
	if _, err := omitOperatorText(&v1.Instance{}); err == nil {
		t.Error("a pointer to a document is not a document the omission knows; want an error")
	}
}

// TestInClusterKeepsOtherWritersText: the omission is the operator's text
// only (supervisor ruling on the scope of portal:D8:R5). The kubelet's
// event notes and a rendered workload's health message carry the
// remediation a user needs, are not kernel diagnostics, and are served
// in-cluster as in local mode.
func TestInClusterKeepsOtherWritersText(t *testing.T) {
	for _, c := range []struct {
		objs []*unstructured.Unstructured
		path string
	}{
		// The broken rollout's Pod waits on an image it cannot pull.
		{f1Broken(t), base + "/instances/default/podinfo"},
		{f1Broken(t), base + "/instances/default/podinfo/graph"},
		// The scheduler's and the kubelet's events about a Pod.
		{loadF1(t), base + "/instances/default/podinfo/events?kind=Pod&namespace=default&name=podinfo-podinfo-d9585d794-4lg6h"},
	} {
		in := newEnv(t, c.objs, readmodeltest.AllowAll, inCluster)
		local := newEnv(t, c.objs, readmodeltest.AllowAll)
		want := otherWritersText(t, local.get(t, c.path).body)
		if len(want) == 0 {
			t.Fatalf("%s: local mode serves no other writer's text, so this test proves nothing", c.path)
		}
		got := otherWritersText(t, in.get(t, c.path).body)
		if !slices.Equal(got, want) {
			t.Errorf("%s in-cluster serves\n%q\nwant local mode's\n%q", c.path, got, want)
		}
	}
}

// otherWritersText lists, sorted, the event notes no operator reported and
// the health messages of objects outside the OPM group in a document.
func otherWritersText(t *testing.T, body []byte) []string {
	t.Helper()
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	var out []string
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if note, ok := x["note"].(string); ok && note != "" {
				if c, _ := x["reportingController"].(string); c != "" && c != operatorController {
					out = append(out, c+": "+note)
				}
			}
			if h, ok := x["health"].(map[string]any); ok {
				ref, _ := x["ref"].(map[string]any)
				if msg, _ := h["message"].(string); msg != "" && ref["group"] != opmGroup {
					out = append(out, "health: "+msg)
				}
			}
			for _, child := range x {
				walk(child)
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(doc)
	slices.Sort(out)
	return out
}

func TestNewRefusesAMissingMode(t *testing.T) {
	for _, mode := range []Mode{"", "cluster", "Local"} {
		_, err := New(Config{Mode: mode})
		if err == nil || !strings.Contains(err.Error(), "mode") {
			t.Errorf("New with mode %q: %v, want a refusal naming the mode", mode, err)
		}
	}
}

// operatorTextFields lists the kinds of operator-text field a document
// carries with a value.
func operatorTextFields(t *testing.T, body []byte) []string {
	t.Helper()
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	var out []string
	var walk func(parentKey string, parent map[string]any, v any)
	walk = func(parentKey string, parent map[string]any, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				if k == "object" {
					// An Object's raw content is checked by objectStatusText.
					continue
				}
				if s, ok := child.(string); ok && s != "" {
					if f := classifyText(parentKey, parent, k); f != "" {
						out = append(out, f)
					}
				}
				walk(k, x, child)
			}
		case []any:
			for _, child := range x {
				walk(parentKey, parent, child)
			}
		}
	}
	walk("", nil, doc)
	return append(out, objectStatusText(body)...)
}

// objectStatusText lists the messages in the raw status of an OPM Object.
func objectStatusText(body []byte) []string {
	var obj v1.Object
	if json.Unmarshal(body, &obj) != nil || obj.Kind != v1.KindObject || obj.Ref.Group != opmGroup {
		return nil
	}
	var out []string
	status, _ := obj.Object["status"].(map[string]any)
	for _, field := range []string{"conditions", "history"} {
		list, _ := status[field].([]any)
		for _, e := range list {
			if m, ok := e.(map[string]any); ok && m["message"] != nil {
				out = append(out, "status."+field+"[].message (OPM object)")
			}
		}
	}
	return out
}

// classifyText names the operator-text field key is inside a map reached
// through parentKey, or "" when key carries no operator text there.
func classifyText(parentKey string, parent map[string]any, key string) string {
	switch key {
	case "note":
		ref, _ := parent["regarding"].(map[string]any)
		switch parent["reportingController"] {
		case operatorController:
			return "note (operator)"
		case nil, "":
			if ref["group"] == opmGroup {
				return "note (operator)"
			}
		}
		return ""
	case "activeMessage":
		return "registration.message"
	case "message":
	default:
		return ""
	}
	switch parentKey {
	case "conditions", "notes":
		return "conditions[].message"
	case "reconcile":
		return "reconcile.message"
	case "history":
		return "history[].message"
	case "registrations", "registration":
		return "registration.message"
	case "health":
		if ref, ok := parent["ref"].(map[string]any); ok && ref["group"] == opmGroup {
			return "health.message (OPM object)"
		}
		return ""
	}
	return ""
}

// assertNoOperatorText fails on any operator-text field in an in-cluster
// document.
func assertNoOperatorText(t *testing.T, what string, body []byte) {
	t.Helper()
	for _, f := range operatorTextFields(t, body) {
		t.Errorf("%s: in-cluster document carries %s", what, f)
	}
}
