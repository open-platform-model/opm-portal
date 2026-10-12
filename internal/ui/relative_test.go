package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// TestRelativeReadsTheSharedTable: relative gives, for every age in
// testdata/relative-time.json, the text of the file. The page script's tick
// repeats this table, and test/browser/time.py reads the same file, so the
// two copies cannot drift apart unseen.
func TestRelativeReadsTheSharedTable(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "relative-time.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		AgeSeconds int64  `json:"age_seconds"`
		Text       string `json:"text"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) < 10 {
		t.Fatalf("relative-time.json has %d rows; want the 10 boundary ages", len(rows))
	}
	for _, r := range rows {
		if got := relative(time.Duration(r.AgeSeconds) * time.Second); got != r.Text {
			t.Errorf("relative(%d s) = %q, want %q", r.AgeSeconds, got, r.Text)
		}
	}
}

// TestTimePartialTicks: the time partial marks its element for the page
// script's tick (data-relative) and keeps the tick out of the polite
// regions (aria-live="off"), so a counting time is announced by nobody.
func TestTimePartialTicks(t *testing.T) {
	h := &Handler{cfg: Config{Now: func() time.Time { return apitest.Now }}}
	pages, err := parsePages(h.templateFuncs())
	if err != nil {
		t.Fatal(err)
	}
	when := apitest.Now.Add(-4 * time.Minute)
	var b strings.Builder
	if err := pages["message"].ExecuteTemplate(&b, "time", &when); err != nil {
		t.Fatal(err)
	}
	want := `<time datetime="2026-10-04T18:06:00Z" title="2026-10-04 18:06:00 UTC" data-relative aria-live="off">4m ago</time>`
	if b.String() != want {
		t.Errorf("time partial:\n got %s\nwant %s", b.String(), want)
	}
	b.Reset()
	if err := pages["message"].ExecuteTemplate(&b, "time", (*time.Time)(nil)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "<time") {
		t.Errorf("a missing time rendered a time element: %s", b.String())
	}
}

// TestEveryPageCarriesTheServerClock: every full page carries the meta the
// tick measures the browser clock against, with the handler's clock.
func TestEveryPageCarriesTheServerClock(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	want := `<meta name="opm-portal-now" content="` + apitest.Now.UTC().Format(time.RFC3339) + `">`
	pages := append([]goldenPage{{"not-found", "/nope", false}}, f1Pages...)
	for _, p := range pages {
		if p.hx {
			continue
		}
		if body := s.get(t, p.path).body; !strings.Contains(body, want) {
			t.Errorf("%s: no %s in the page", p.path, want)
		}
	}
	if body := s.get(t, "/", anonymous).body; !strings.Contains(body, want) {
		t.Error("the sign-in page carries no server clock")
	}
}
