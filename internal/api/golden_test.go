package api

import (
	"bytes"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

var update = flag.Bool("update", false, "rewrite the golden documents")

// goldenCase is one request whose response body is compared with
// testdata/golden/<name>.json.
type goldenCase struct {
	name string
	path string
}

var f1Goldens = []goldenCase{
	{"cluster", base},
	{"instance-list", base + "/instances"},
	{"instance-list-default", base + "/instances?namespace=default"},
	{"instance-podinfo", base + "/instances/default/podinfo"},
	{"instance-cert-manager", base + "/instances/cert-manager/cert-manager"},
	{"instance-web-cli-owned", base + "/instances/web/web"},
	{"instance-backup-provider", base + "/instances/default/backup-provider"},
	{"instance-podinfo-graph", base + "/instances/default/podinfo/graph"},
	{"instance-cert-manager-graph", base + "/instances/cert-manager/cert-manager/graph"},
	{"instance-podinfo-events", base + "/instances/default/podinfo/events"},
	{"instance-podinfo-events-deployment", base + "/instances/default/podinfo/events?group=apps&kind=Deployment&namespace=default&name=podinfo-podinfo"},
	{"instance-podinfo-events-pod", base + "/instances/default/podinfo/events?kind=Pod&namespace=default&name=podinfo-podinfo-d9585d794-4lg6h"},
	{"instance-podinfo-object-deployment", base + "/instances/default/podinfo/object?group=apps&kind=Deployment&namespace=default&name=podinfo-podinfo"},
	{"instance-podinfo-object-pod", base + "/instances/default/podinfo/object?kind=Pod&namespace=default&name=podinfo-podinfo-d9585d794-4lg6h"},
	{"instance-backup-provider-object-registration", backupRegistrationObject},
	{"instance-cert-manager-object-crd", base + "/instances/cert-manager/cert-manager/object?group=apiextensions.k8s.io&kind=CustomResourceDefinition&name=certificates.cert-manager.io"},
	{"package-list", base + "/packages"},
	{"package-podinfo", base + "/packages/pkg/podinfo"},
	{"package-podinfo-graph", base + "/packages/pkg/podinfo/graph"},
	{"package-podinfo-events", base + "/packages/pkg/podinfo/events"},
	{"platform", base + "/platform"},
	{"platform-graph", base + "/platform/graph"},
	{"platform-events", base + "/platform/events"},
	{"registration-refused-events", base + "/platform/registrations/default.refused-claim-fixture/events"},
}

var brokenGoldens = []goldenCase{
	{"instance-podinfo-broken", base + "/instances/default/podinfo"},
}

// TestGoldenF1 serves every resource for the F1 capture through the real
// handlers, with SSAR-backed checkers on the fake cluster allowing every
// read, and compares each body with its golden document.
func TestGoldenF1(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll)
	checkGoldens(t, e, f1Goldens)
}

// TestGoldenBrokenRollout: the image-break sample is Applied and Degraded
// at once (portal:D3:R1/R2).
func TestGoldenBrokenRollout(t *testing.T) {
	e := newEnv(t, f1Broken(t), readmodeltest.AllowAll)
	checkGoldens(t, e, brokenGoldens)
	var doc struct {
		Reconcile struct{ State string } `json:"reconcile"`
		Health    struct{ State string } `json:"health"`
	}
	if err := json.Unmarshal(e.get(t, base+"/instances/default/podinfo").body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Reconcile.State != "Applied" || doc.Health.State != "Degraded" {
		t.Errorf("broken podinfo: reconcile %q, health %q; want Applied and Degraded", doc.Reconcile.State, doc.Health.State)
	}
}

func checkGoldens(t *testing.T, e *env, cases []goldenCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := e.get(t, c.path)
			if res.status != http.StatusOK {
				t.Fatalf("GET %s = %d %s", c.path, res.status, res.body)
			}
			if ct := res.header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q", ct)
			}
			var pretty bytes.Buffer
			if err := json.Indent(&pretty, res.body, "", "  "); err != nil {
				t.Fatalf("body is not JSON: %v", err)
			}
			pretty.WriteByte('\n')
			assertNothingWithheldIsServed(t, pretty.Bytes())
			golden := filepath.Join("testdata", "golden", c.name+".json")
			if *update {
				if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, pretty.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("reading %s (run with -update to write it): %v", golden, err)
			}
			if !bytes.Equal(pretty.Bytes(), want) {
				t.Errorf("GET %s differs from %s (run with -update after checking the diff):\n%s", c.path, golden, pretty.String())
			}
		})
	}
}

// assertNothingWithheldIsServed fails on a values field or the last-applied
// annotation anywhere in a document (portal:D8:R2/R3).
func assertNothingWithheldIsServed(t *testing.T, body []byte) {
	t.Helper()
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				if k == "values" {
					t.Errorf("%s.values is served", path)
				}
				walk(path+"."+k, child)
			}
		case []any:
			for _, child := range x {
				walk(path+"[]", child)
			}
		case string:
			if strings.Contains(x, "last-applied-configuration") {
				t.Errorf("%s names the last-applied annotation", path)
			}
		}
	}
	walk("$", doc)
}
