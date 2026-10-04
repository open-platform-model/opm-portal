package graph

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

var update = flag.Bool("update", false, "rewrite the golden files under testdata/golden")

const goldenDir = "testdata/golden"

// TestGoldens builds the F1 graphs and compares their JSON and SVG with
// the committed goldens. `go test ./internal/graph -run TestGoldens
// -update` rewrites them; review the SVGs before committing.
func TestGoldens(t *testing.T) {
	c := newCluster(t, f1(t), readmodeltest.AllowAll)
	broken := newCluster(t, f1Broken(t), readmodeltest.AllowAll)
	cmGroup := "grp:configuration/mi/cert-manager/cert-manager"
	cases := []struct {
		name  string
		build func(t *testing.T) Graph
	}{
		{"instance-cert-manager", func(t *testing.T) Graph {
			return Instance(c.instance(t, "cert-manager", "cert-manager"), Options{})
		}},
		{"instance-cert-manager-expanded", func(t *testing.T) Graph {
			return Instance(c.instance(t, "cert-manager", "cert-manager"), Options{Expand: []string{cmGroup}})
		}},
		{"instance-podinfo-broken", func(t *testing.T) Graph {
			return Instance(broken.instance(t, "default", "podinfo"), Options{})
		}},
		{"instance-web-cli-owned", func(t *testing.T) Graph {
			return Instance(c.instance(t, "web", "web"), Options{})
		}},
		{"instance-backup-provider", func(t *testing.T) Graph {
			return Instance(c.instance(t, "default", "backup-provider"), Options{})
		}},
		{"instance-backup-consumer", func(t *testing.T) Graph {
			return Instance(c.instance(t, "default", "backup-consumer"), Options{})
		}},
		{"package-podinfo", func(t *testing.T) Graph {
			return Package(c.pkg(t, "pkg", "podinfo"), Options{})
		}},
		{"platform-f1", func(t *testing.T) Graph {
			return Platform(c.platform(t), Options{})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := tc.build(t)
			js, err := json.MarshalIndent(g, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			compareGolden(t, tc.name+".json", append(js, '\n'))
			compareGolden(t, tc.name+".svg", writeSVG(g))
		})
	}
}

func compareGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join(goldenDir, name)
	if *update {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s (run with -update to create it): %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from the golden; run with -update and review the diff", name)
	}
}

// TestMeasureCertManager records what building cert-manager's graph costs
// on top of the read model, and how big it is. Run with -v; design.md
// records the numbers.
func TestMeasureCertManager(t *testing.T) {
	c := newCluster(t, f1(t), readmodeltest.AllowAll)
	d := c.instance(t, "cert-manager", "cert-manager")
	cmGroup := "grp:configuration/mi/cert-manager/cert-manager"
	const runs = 200
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{"collapsed", Options{}},
		{"expanded", Options{Expand: []string{cmGroup}}},
	} {
		var g Graph
		start := time.Now()
		for range runs {
			g = Instance(d, tc.opts)
		}
		per := time.Since(start) / runs
		t.Logf("cert-manager %s: %d nodes, %d edges, %d columns, %dx%d, build %v per graph (mean of %d)",
			tc.name, len(g.Nodes), len(g.Edges), len(g.Layout.Columns), g.Layout.Width, g.Layout.Height, per, runs)
	}
	start := time.Now()
	for range runs {
		Instance(c.instance(t, "cert-manager", "cert-manager"), Options{})
	}
	t.Logf("cert-manager warm read model view plus collapsed graph: %v per request (mean of %d)", time.Since(start)/runs, runs)
}
