//go:build browser

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-platform-model/opm-portal/internal/api/apitest"
	"github.com/open-platform-model/opm-portal/internal/auth"
	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/ui"
)

// playwrightImage pins the browsers by the image index's digest. The
// Python packages installed in it are hash-locked in playwrightRequirements,
// whose playwright version must match the image tag's.
const (
	playwrightImage        = "mcr.microsoft.com/playwright/python:v1.63.0-noble@sha256:72bd171a9ffc2b4b59532aaa6210e21014d07093120dc25528870c0b840da1f0"
	playwrightRequirements = "../../test/browser/requirements.txt"
)

// TestBrowserLaunch opens the launch page openLaunch writes, as a file://
// page the way serve --open does, in real browsers run from the Playwright
// image (task test:browser). A browser withholds a SameSite=Strict cookie
// from a navigation a cross-site document started, which a Go client never
// does, so only a browser proves the launch lands with the session.
func TestBrowserLaunch(t *testing.T) {
	script := browserScript(t, "launch.py")
	run := func(ctx context.Context, t *testing.T, browser, start, landing string, mounts ...string) error {
		return playwright(ctx, t, script, mounts, browser, start, landing)
	}
	for _, browser := range []string{"chromium", "firefox", "webkit"} {
		t.Run(browser+"/open", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			door := serveFrontDoor(t)
			var page string
			removed := make(chan struct{})
			cleanup, err := openLaunch(ctx, door.launch, func(ctx context.Context, path string) error {
				// As serve does, the page goes the moment its token is
				// spent, while the browser is still landing.
				page = path
				go func() {
					defer close(removed)
					removeWhenLaunched(ctx, door.launched, func() { _ = os.RemoveAll(filepath.Dir(path)) })
				}()
				return run(ctx, t, browser, path, door.url, filepath.Dir(path))
			})
			defer cleanup()
			if err != nil {
				t.Fatalf("the launch page did not land on the landing page with the session: %v", err)
			}
			<-removed
			if _, err := os.Stat(page); !os.IsNotExist(err) {
				t.Errorf("the launch page is still there after the launch: %v", err)
			}
		})
		t.Run(browser+"/link", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			door := serveFrontDoor(t)
			if err := run(ctx, t, browser, door.launch, door.url); err != nil {
				t.Fatalf("the launch link did not land on the landing page with the session: %v", err)
			}
		})
	}
}

// browserScript reads a Playwright script from test/browser.
func browserScript(t *testing.T, name string) []byte {
	t.Helper()
	script, err := os.ReadFile(filepath.Join("..", "..", "test", "browser", name))
	if err != nil {
		t.Fatal(err)
	}
	return script
}

// playwright runs script with args in the Playwright image, on the host
// network, with each of mounts mounted read-only at its own path. When
// OPM_PORTAL_BROWSER_SHOTS names a directory, it is mounted writable and
// the script saves a screenshot there, named after the test, on failure.
func playwright(ctx context.Context, t *testing.T, script []byte, mounts []string, args ...string) error {
	engine := os.Getenv("OPM_PORTAL_CONTAINER_ENGINE")
	if engine == "" {
		engine = "podman"
	}
	cargs := make([]string, 0, 16+2*len(mounts)+len(args))
	requirements, err := filepath.Abs(playwrightRequirements)
	if err != nil {
		return err
	}
	cargs = append(cargs, "run", "--rm", "-i", "--network", "host", "-v", requirements+":/requirements.txt:ro,Z")
	for _, dir := range mounts {
		cargs = append(cargs, "-v", dir+":"+dir+":ro,Z")
	}
	if shots := os.Getenv("OPM_PORTAL_BROWSER_SHOTS"); shots != "" {
		dir, err := filepath.Abs(shots)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
		cargs = append(cargs, "-v", dir+":/shots:Z", "-e", "OPM_PORTAL_BROWSER_SHOTS=/shots", "-e", "OPM_PORTAL_BROWSER_SHOT="+name)
	}
	cargs = append(cargs, playwrightImage, "bash", "-c",
		`pip install -q --root-user-action=ignore --require-hashes -r /requirements.txt >/dev/null && python3 - "$@"`,
		"playwright")
	cargs = append(cargs, args...)
	cmd := exec.CommandContext(ctx, engine, cargs...)
	cmd.Stdin = bytes.NewReader(script)
	out, err := cmd.CombinedOutput()
	t.Logf("%s", out)
	return err
}

// TestBrowserLogs refreshes a logs region under a tailed, focused, open
// pane in real browsers (task test:browser). A pane moved through a
// detached tree loses its scroll offset, so only a browser shows that a
// refresh, the 30 s republish among them, keeps the pane where it was and
// still tailing.
func TestBrowserLogs(t *testing.T) {
	script := browserScript(t, "logs.py")
	for _, browser := range []string{"chromium", "firefox", "webkit"} {
		t.Run(browser, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			if err := playwright(ctx, t, script, nil, browser, serveLogsPage(t)); err != nil {
				t.Fatalf("a refresh did not keep the tailed log pane: %v", err)
			}
		})
	}
}

// TestBrowserTheme checks, in real browsers (task test:browser), what only
// a browser shows: a stored Dark theme paints dark first on a light system,
// a stored Installed filter opens filtered on a full load and on a boosted
// navigation, a stale stored value is dropped (portal:D14), the Instance chip
// reads in both themes, and nothing animates in on page load (portal:D19).
func TestBrowserTheme(t *testing.T) {
	script := browserScript(t, "theme.py")
	for _, browser := range []string{"chromium", "firefox", "webkit"} {
		t.Run(browser, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			if err := playwright(ctx, t, script, nil, browser, serveF1Site(t), apitest.Context); err != nil {
				t.Fatalf("the theme or the remembered filters did not hold: %v", err)
			}
		})
	}
}

// TestBrowserGraph drives the instance graph in real browsers (task
// test:browser): Clear selection, the Resources hand-off, a group's fit and
// Overview, the state and full screen a live refresh keeps, and the details
// panel a node selection fills.
func TestBrowserGraph(t *testing.T) {
	script := browserScript(t, "graph.py")
	for _, browser := range []string{"chromium", "firefox", "webkit"} {
		t.Run(browser, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			if err := playwright(ctx, t, script, nil, browser, serveF1Site(t)); err != nil {
				t.Fatalf("a graph interaction did not hold: %v", err)
			}
		})
	}
}

// TestBrowserOwner drives an owner page's details panel in real browsers (task
// test:browser): the panel at rest shows the ModuleInstance, a node click fills it
// and Clear selection brings the ModuleInstance back, the YAML tab opens on the
// selected object, and a click on a Resources row opens the Graph focused with
// the panel filled. A last step selects podinfo's Service over F1 plus the
// recorded EndpointSlice and reads the Endpoints fact, once for a caller who
// may list EndpointSlices (`2 ready`) and once for a caller who may not
// (`Could not be read`, never `0 ready`).
func TestBrowserOwner(t *testing.T) {
	script := browserScript(t, "owner.py")
	for _, browser := range []string{"chromium", "firefox", "webkit"} {
		t.Run(browser, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			counted, unread := serveEndpointSites(t)
			if err := playwright(ctx, t, script, nil, browser, serveF1Site(t), counted, unread); err != nil {
				t.Fatalf("an owner page interaction did not hold: %v", err)
			}
		})
	}
}

// serveEndpointSites serves F1 plus the recorded EndpointSlice of podinfo's
// Service twice: for a caller who may list EndpointSlices, and for one who
// may not.
func serveEndpointSites(t *testing.T) (counted, unread string) {
	t.Helper()
	counted = serveSite(t, apitest.F1WithEndpointSlices(t))
	srv := apitest.New(t, apitest.F1WithEndpointSlices(t), apitest.DenyResources("endpointslices"))
	return counted, serveHandlerWith(t, srv, nil, nil, nil)
}

// TestBrowserTabCount drives a live tab count in real browsers (task
// test:browser): a stream event makes the page fetch its own render, and the
// real portal.js replaces the count span with the render's, whether the count
// changed, the source became a problem and left the span empty, or the count
// came back (portal:D19:R3). The F1 capture is static, so the script answers
// that fetch with the real render edited as a later render would be.
func TestBrowserTabCount(t *testing.T) {
	script := browserScript(t, "tabcount.py")
	for _, browser := range []string{"chromium", "firefox", "webkit"} {
		t.Run(browser, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			if err := playwright(ctx, t, script, nil, browser, serveF1Site(t)); err != nil {
				t.Fatalf("a tab count did not follow the page: %v", err)
			}
		})
	}
}

// phonePages lists the pages TestBrowserPhone opens: the non-fragment
// paths of f1Pages in internal/ui/ui_test.go, which package main cannot
// import, the Graph tab of each owner page, the Provider tab of the package
// that holds a refused claim, and the not-found page. A UI change that adds
// a page adds its path
// here, as it adds it to f1Pages.
var phonePages = []string{
	"/",
	"/?tab=providers",
	"/?tab=catalogs",
	"/?pstatus=refused&eresource=registration:default.refused-claim-fixture",
	"/installed",
	"/installed?q=backup",
	"/installed?q=zzz",
	"/installed?namespace=default",
	"/installed?kind=package",
	"/installed?uses=opmodel.dev/catalogs/opm/traits/backup@v1alpha1",
	"/installed?health=Bogus&q=pod",
	"/instances/default/podinfo",
	"/instances/default/podinfo?tab=graph&focus=obj:apps/Deployment/default/podinfo-podinfo",
	"/instances/default/podinfo?tab=resources",
	"/instances/default/podinfo?tab=events&type=Normal",
	"/instances/default/podinfo?tab=logs",
	"/instances/default/podinfo?tab=yaml&object=apps/Deployment/default/podinfo-podinfo",
	"/instances/default/backup-provider",
	"/instances/default/backup-provider?tab=provider",
	"/catalog?path=testing.opmodel.dev/catalogs/controller/backup@v0",
	"/catalog?path=testing.opmodel.dev/catalogs/controller/refused-claim-fixture-absent@v0",
	"/catalog?path=testing.opmodel.dev/catalogs/controller/backup@v0&tab=events",
	"/catalog?path=testing.opmodel.dev/catalogs/controller/refused-claim-fixture-absent@v0&tab=events&reason=CatalogUnresolved",
	"/catalog?path=opmodel.dev/catalogs/opm@v4",
	"/catalog?path=opmodel.dev/catalogs/opm@v4&tab=events",
	"/instances/cert-manager/cert-manager?tab=resources",
	"/instances/cert-manager/cert-manager",
	"/instances/cert-manager/cert-manager?expand=grp:configuration/mi/cert-manager/cert-manager/rbac",
	"/instances/default/podinfo?tab=graph",
	"/instances/cert-manager/cert-manager?tab=graph",
	"/instances/cert-manager/cert-manager?tab=graph&expand=grp:configuration/mi/cert-manager/cert-manager/rbac",
	"/instances/cert-manager/cert-manager?tab=graph&group=off",
	"/instances/default/backup-provider?tab=graph",
	"/packages/pkg/podinfo?tab=graph",
	"/instances/web/web",
	"/packages/pkg/podinfo",
	"/packages/pkg/podinfo?tab=resources",
	"/packages/pkg/provider?tab=provider",
	"/nope",
}

// TestBrowserPhone opens every page of phonePages at a 360 px wide viewport
// in real browsers (task test:browser) and fails when a page's scroll width
// is above 360. The body clips sideways overflow, so only a browser that
// measures the layout shows content a phone would lose. The script first
// proves the measure can fail with an element it adds that is 500 px wide.
// A last step selects a graph node from the keyboard and measures again with
// the details panel filled. The /endpoints leg opens the panel of podinfo's
// Service with the Endpoints fact reading `2 ready` and `Could not be read`.
func TestBrowserPhone(t *testing.T) {
	script := browserScript(t, "phone.py")
	banner := browserScript(t, "phone_banner.py")
	endpoints := browserScript(t, "phone_endpoints.py")
	for _, browser := range []string{"chromium", "firefox", "webkit"} {
		t.Run(browser, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			// F1 with the package that holds a refused claim, as the UI tests build it.
			args := append([]string{browser, serveSite(t, apitest.F1PackageHolder(t))}, phonePages...)
			if err := playwright(ctx, t, script, nil, args...); err != nil {
				t.Fatalf("a page is wider than 360 px: %v", err)
			}
		})
		t.Run(browser+"/banner", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			// A 90 character context and a different 90 character cluster
			// entry, with no space in either, and the banner shown.
			longContext, longEntry := strings.Repeat("c", 90), strings.Repeat("e", 90)
			srv := apitest.NewWithCluster(t, apitest.F1(t), apitest.AllowAll, longContext, longEntry)
			url := serveHandlerWith(t, srv, nil, nil, nil)
			if err := playwright(ctx, t, banner, nil, browser, url, "/", "/instances/default/podinfo"); err != nil {
				t.Fatalf("the header with a banner and two long names does not hold 360 px: %v", err)
			}
		})
		t.Run(browser+"/endpoints", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			counted, unread := serveEndpointSites(t)
			if err := playwright(ctx, t, endpoints, nil, browser, counted, unread); err != nil {
				t.Fatalf("the Endpoints fact does not hold 360 px: %v", err)
			}
		})
	}
}

// tipFragment is the golden fragment-tip.html, the tip partial's output, which
// package main cannot import from internal/ui.
const tipFragment = "../../internal/ui/testdata/golden/fragment-tip.html"

// TestBrowserTip drives the info tip in real browsers (task test:browser):
// focus and hover show its box, Escape hides it with focus or the pointer
// kept where they were, Enter and Space toggle it, the pointer reaches the
// box, and a tap opens, closes and is closed by a tap outside (WCAG 2.2
// 1.4.13). No page shows a tip yet, so the script inserts the markup of the
// golden fragment-tip.html into the Platform page, where the real portal.css
// and portal.js act on it; the change that puts a tip on a page adds a case
// on that page.
func TestBrowserTip(t *testing.T) {
	script := browserScript(t, "tip.py")
	markup, err := os.ReadFile(tipFragment)
	if err != nil {
		t.Fatal(err)
	}
	for _, browser := range []string{"chromium", "firefox", "webkit"} {
		t.Run(browser, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			if err := playwright(ctx, t, script, nil, browser, serveF1Site(t), strings.TrimSpace(string(markup))); err != nil {
				t.Fatalf("the tip did not hold: %v", err)
			}
		})
	}
}

// TestBrowserFilters drives the filter forms in real browsers (task
// test:browser): typing in Installed's search applies after a pause and keeps
// focus, caret and the characters typed meanwhile, and replaces the history
// entry so Back leaves the filtered view; Enter submits once; a select
// applies at once; the Apply button hides until it takes keyboard focus; a
// text field applies when the user leaves it; and the new count is announced
// once through the header's status region (PIC-L1).
func TestBrowserFilters(t *testing.T) {
	script := browserScript(t, "filters.py")
	for _, browser := range []string{"chromium", "firefox", "webkit"} {
		t.Run(browser, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			if err := playwright(ctx, t, script, nil, browser, serveF1Site(t), apitest.Context); err != nil {
				t.Fatalf("a filter form did not hold: %v", err)
			}
		})
	}
}

// relativeTimeTable is the table the page script's tick and the UI's
// relative both follow, which package main cannot import from internal/ui.
const relativeTimeTable = "../../internal/ui/testdata/relative-time.json"

// TestBrowserLoading drives the loading bar and the ticking times in real
// browsers (task test:browser). loading.py holds requests the way a slow
// cluster would and checks the bar, the details panel's busy state and the
// line it carries, against Playwright's clock: nothing before a second, the
// bar and "Loading..." after it, the bar until the last of two requests ends,
// an end through an error or an abort, and nothing for a background refresh.
// time.py sets a browser clock ten minutes off the page's, then checks the
// script's text against the rows of relative-time.json, that a tick writes
// only the text of a time that changed, and that a counting time keeps its
// width to a glyph.
func TestBrowserLoading(t *testing.T) {
	loading := browserScript(t, "loading.py")
	timing := browserScript(t, "time.py")
	table, err := os.ReadFile(relativeTimeTable)
	if err != nil {
		t.Fatal(err)
	}
	for _, browser := range []string{"chromium", "firefox", "webkit"} {
		t.Run(browser+"/loading", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			if err := playwright(ctx, t, loading, nil, browser, serveF1Site(t)); err != nil {
				t.Fatalf("a slow request did not show as designed: %v", err)
			}
		})
		t.Run(browser+"/time", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			url := serveSiteAt(t, apitest.F1(t), func() time.Time { return apitest.Now })
			if err := playwright(ctx, t, timing, nil, browser, url, apitest.Now.UTC().Format(time.RFC3339), strings.TrimSpace(string(table))); err != nil {
				t.Fatalf("a relative time did not count as designed: %v", err)
			}
		})
	}
}

// TestBrowserStale drives the banner of a lost stream in real browsers (task
// test:browser). stale.py holds a page open on a stream the test controls
// (serveStaleSite): it sends heartbeats and changes, drops the connection,
// refuses reconnects, and holds, fails and answers 401 to the page's refresh
// fetch with routes. It checks the banner's words and time at each step, that
// the live mark is the only status text that changes, that the banner goes
// only after one fetch has replaced the followed regions, that a failed
// fetch is tried again on the next heartbeat, and that a reader scrolled far
// down still sees the banner. Run it after any change to the page script's
// stream, banner or resync code.
func TestBrowserStale(t *testing.T) {
	script := browserScript(t, "stale.py")
	for _, browser := range []string{"chromium", "firefox", "webkit"} {
		t.Run(browser, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Minute)
			defer cancel()
			url := serveStaleSite(t)
			if err := playwright(ctx, t, script, nil, browser, url, apitest.Now.UTC().Format(time.RFC3339)); err != nil {
				t.Fatalf("the banner did not hold as designed: %v", err)
			}
		})
	}
}

// serveStaleSite serves the pages over the F1 capture with a stream and a
// page fetch the script drives. GET /api/v1alpha1/clusters/default/stream
// answers 503 while refusing, else opens a stream with a fresh id (retry
// 300 ms) and writes the frames the script posts until it asks to drop.
// The page's refresh fetch (a GET asking for exactly text/html) is counted
// and answered by mode: ok, hold (until released), abort (the connection
// is reset), 500 or 401.
//
//	POST /ctl/send?frame=...   write one frame      POST /ctl/drop     end the connection
//	POST /ctl/refuse?on=1|0    refuse connections   POST /ctl/fetch?mode=...
//	POST /ctl/release          release one held fetch
//	POST /ctl/render?mode=ok|broken   render the pages over F1 or the image-break capture
//	GET  /ctl/state            "<streams> <fetches>"
func serveStaleSite(t *testing.T) string {
	t.Helper()
	const apiBase = "/api/v1alpha1/clusters/default"
	var (
		mu      sync.Mutex
		frames  = make(chan string, 16)
		refuse  atomic.Bool
		streams atomic.Int32
		fetches atomic.Int32
		mode    atomic.Value
		release = make(chan struct{}, 16)
	)
	mode.Store("ok")
	var broken atomic.Bool
	wrap := func(f1 http.Handler) http.Handler {
		// The same pages over the image-break capture: what a cluster that changed
		// would render. The stream stays the script's; only the page renders move.
		altPages := imageBreakPages(t)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pages := f1
			if broken.Load() {
				pages = altPages
			}
			if r.Method != http.MethodGet || r.Header.Get("Accept") != "text/html" {
				pages.ServeHTTP(w, r)
				return
			}
			fetches.Add(1)
			switch mode.Load().(string) {
			case "hold":
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
			case "abort":
				panic(http.ErrAbortHandler)
			case "500":
				http.Error(w, "boom", http.StatusInternalServerError)
				return
			case "401":
				http.Error(w, "signed out", http.StatusUnauthorized)
				return
			}
			pages.ServeHTTP(w, r)
		})
	}
	return serveSiteWith(t, apitest.F1(t), func() time.Time { return apitest.Now }, wrap, func(mux *http.ServeMux) {
		mux.HandleFunc("GET "+apiBase+"/stream", func(w http.ResponseWriter, r *http.Request) {
			n := streams.Add(1)
			if refuse.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			mu.Lock()
			mine := make(chan string, 16)
			frames = mine
			mu.Unlock()
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-store")
			rc := http.NewResponseController(w)
			_, _ = io.WriteString(w, "retry: 300\nevent: open\ndata: {\"stream\":\"s"+strconv.Itoa(int(n))+"\"}\n\n")
			_ = rc.Flush()
			for {
				select {
				case f := <-mine:
					if f == "" {
						return
					}
					_, _ = io.WriteString(w, f)
					_ = rc.Flush()
				case <-r.Context().Done():
					return
				}
			}
		})
		mux.HandleFunc("POST "+apiBase+"/stream/{stream}/topics", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
		mux.HandleFunc("POST /ctl/send", func(_ http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			frames <- r.URL.Query().Get("frame") + "\n\n"
		})
		mux.HandleFunc("POST /ctl/drop", func(_ http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			frames <- ""
		})
		mux.HandleFunc("POST /ctl/refuse", func(_ http.ResponseWriter, r *http.Request) {
			refuse.Store(r.URL.Query().Get("on") == "1")
		})
		mux.HandleFunc("POST /ctl/fetch", func(_ http.ResponseWriter, r *http.Request) {
			mode.Store(r.URL.Query().Get("mode"))
		})
		mux.HandleFunc("POST /ctl/render", func(_ http.ResponseWriter, r *http.Request) {
			broken.Store(r.URL.Query().Get("mode") == "broken")
		})
		mux.HandleFunc("POST /ctl/release", func(_ http.ResponseWriter, _ *http.Request) {
			release <- struct{}{}
		})
		mux.HandleFunc("GET /ctl/state", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, strconv.Itoa(int(streams.Load()))+" "+strconv.Itoa(int(fetches.Load())))
		})
	})
}

// imageBreakPages are the pages over the image-break capture, on the clock of
// the F1 pages.
func imageBreakPages(t *testing.T) http.Handler {
	t.Helper()
	pages, err := ui.New(ui.Config{API: apitest.New(t, apitest.F1Broken(t), apitest.AllowAll), Now: func() time.Time { return apitest.Now }})
	if err != nil {
		t.Fatal(err)
	}
	return pages
}

// serveF1Site serves, on a free loopback port, the read API and the pages
// over the F1 capture, every request reading as the test caller.
func serveF1Site(t *testing.T) string {
	t.Helper()
	return serveSite(t, apitest.F1(t))
}

// serveSite is serveF1Site over objs.
func serveSite(t *testing.T, objs []*unstructured.Unstructured) string {
	t.Helper()
	return serveSiteAt(t, objs, nil)
}

// serveSiteAt is serveSite with the clock the pages read relative times
// against; a nil clock is the wall clock.
func serveSiteAt(t *testing.T, objs []*unstructured.Unstructured, now func() time.Time) string {
	t.Helper()
	return serveSiteWith(t, objs, now, nil, nil)
}

// serveSiteWith is serveSiteAt with wrap, which may stand in front of the
// pages, and extra, which may add handlers to the mux beside the read API
// and the pages (a more specific pattern wins).
func serveSiteWith(t *testing.T, objs []*unstructured.Unstructured, now func() time.Time, wrap func(http.Handler) http.Handler, extra func(*http.ServeMux)) string {
	t.Helper()
	return serveHandlerWith(t, apitest.New(t, objs, apitest.AllowAll), now, wrap, extra)
}

// serveHandlerWith is serveSiteWith over a read API the caller built.
func serveHandlerWith(t *testing.T, srv http.Handler, now func() time.Time, wrap func(http.Handler) http.Handler, extra func(*http.ServeMux)) string {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	pages, err := ui.New(ui.Config{API: srv, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", srv)
	var front http.Handler = pages
	if wrap != nil {
		front = wrap(pages)
	}
	mux.Handle("/", front)
	if extra != nil {
		extra(mux)
	}
	httpSrv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.AddCookie(&http.Cookie{Name: apitest.SessionCookie, Value: apitest.SessionValue})
			mux.ServeHTTP(w, r)
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = httpSrv.Serve(ln) }()
	t.Cleanup(func() { _ = httpSrv.Close() })
	return "http://" + ln.Addr().String()
}

// TestBrowserExpired ends a page's stream with the expired event in real
// browsers (task test:browser). Only a browser shows that the EventSource
// closes for good instead of reconnecting into a refusal, that the page
// says the session expired in the live mark and in the banner with the time
// of the data and a Reload link, and that the link loads a new page with a
// new stream.
func TestBrowserExpired(t *testing.T) {
	script := browserScript(t, "expired.py")
	for _, browser := range []string{"chromium", "firefox", "webkit"} {
		t.Run(browser, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			url, streams := serveExpiringPage(t)
			if err := playwright(ctx, t, script, nil, browser, url); err != nil {
				t.Fatalf("the page did not show the expired session: %v", err)
			}
			// One stream for the page, none after expired (the script counts
			// that before it acts), and one for the page its Reload link loads.
			if n := streams.Load(); n != 2 {
				t.Errorf("the page opened %d streams; want 2: the page's, no reconnect after expired, and the reload's", n)
			}
		})
	}
}

// serveExpiringPage serves, on a free loopback port, the Platform page as
// the UI renders it over an API that serves nothing, and a stream that
// sends its open event, then its expired event, and ends, asking the
// client to retry after 300 ms. It returns the page's URL and the count of
// stream requests.
func serveExpiringPage(t *testing.T) (string, *atomic.Int32) {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	const apiBase = "/api/v1alpha1/clusters/default"
	pages, err := ui.New(ui.Config{API: http.NotFoundHandler(), APIBase: apiBase})
	if err != nil {
		t.Fatal(err)
	}
	var streams atomic.Int32
	mux := http.NewServeMux()
	mux.Handle("/", pages)
	mux.HandleFunc("GET "+apiBase+"/stream", func(w http.ResponseWriter, _ *http.Request) {
		streams.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		rc := http.NewResponseController(w)
		_, _ = io.WriteString(w, "retry: 300\nevent: open\ndata: {\"stream\":\"s1\"}\n\n")
		_ = rc.Flush()
		time.Sleep(500 * time.Millisecond)
		_, _ = io.WriteString(w, "event: expired\ndata: {\"code\":\"unauthenticated\"}\n\n")
		_ = rc.Flush()
	})
	mux.HandleFunc("POST "+apiBase+"/stream/{stream}/topics", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /streams", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, strconv.Itoa(int(streams.Load())))
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return "http://" + ln.Addr().String() + "/", &streams
}

// serveLogsPage serves, on a free loopback port, a page holding a logs
// region shaped as the owner page renders it: a Pod / container select form
// and one open pane. The page loads portal.js but not htmx, so a select
// change is a full page load and the page's htmx:afterSettle listeners do
// not run here; TestBrowserOwner drives the swapped select on the real page
// (owner.py, step 8). Its first render lists Pod a's pane, its second adds
// Pod b's, and every later one lists no Pod; a request that names a pane
// (log=) is rendered at once, with that one pane open. The stream's topics
// endpoint records what each stream is subscribed to, and /subs?stream=ID
// lists it; POST /subs/reset forgets all of it, so a step starts from nothing
// and a topic listed afterwards was subscribed in that step.
func serveLogsPage(t *testing.T) string {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	pages, err := ui.New(ui.Config{API: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	pane := func(pod string, open bool) string {
		attr := ""
		if open {
			attr = " open"
		}
		return `<details class="log" id="log_` + pod + `_c" data-log-topic="log:ns/` + pod + `/c"` + attr + `>` +
			`<summary><span class="mono">` + pod + `</span> <span class="log-container">c</span></summary>` +
			`<div class="log-tools"><label><input type="checkbox" data-log-previous="log:ns/` + pod + `/c/previous"> Previous instance</label>` +
			`<span class="log-state" aria-live="polite"></span></div>` +
			`<pre class="log-pane" tabindex="0" aria-label="Log of c in ` + pod + `"></pre></details>`
	}
	form := func(picked string) string {
		opt := func(pod string) string {
			sel := ""
			if pod == picked {
				sel = " selected"
			}
			return `<option value="log_` + pod + `_c"` + sel + `>c</option>`
		}
		return `<form class="filters" method="get" action="/logs" data-filters="owner-logs"><input type="hidden" name="tab" value="logs">` +
			`<select id="f-log" name="log"><optgroup label="a">` + opt("a") + `</optgroup><optgroup label="b">` + opt("b") + `</optgroup></select>` +
			`<button type="submit">Show</button></form>`
	}
	var (
		renders atomic.Int32
		mu      sync.Mutex
		subs    = map[string][]string{}
	)
	mux := http.NewServeMux()
	mux.Handle("/static/", pages)
	mux.HandleFunc("POST /stream/{id}/topics", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Add []string `json:"add"`
		}
		if json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body) == nil {
			mu.Lock()
			subs[r.PathValue("id")] = append(subs[r.PathValue("id")], body.Add...)
			mu.Unlock()
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /subs", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		got := subs[r.URL.Query().Get("stream")]
		if got == nil {
			got = []string{}
		}
		_ = json.NewEncoder(w).Encode(got)
	})
	mux.HandleFunc("POST /subs/reset", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		subs = map[string][]string{}
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		var panes, note, picker string
		note = `<p class="note">Logs stream only while the pane is open.</p>`
		if pick := r.URL.Query().Get("log"); pick != "" {
			pod := strings.TrimSuffix(strings.TrimPrefix(pick, "log_"), "_c")
			picker, panes = form(pick), pane(pod, true)
		} else {
			switch renders.Add(1) {
			case 1:
				picker, panes = form("log_a_c"), pane("a", true)
			case 2:
				picker, panes = form("log_a_c"), pane("a", true)+pane("b", false)
			default:
				note = `<p class="muted">No Pod below this inventory that you may read.</p>`
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", ui.PagePolicy)
		_, _ = io.WriteString(w, `<!doctype html><meta charset="utf-8"><title>logs</title>`+
			`<link rel="stylesheet" href="/static/portal.css">`+
			`<body data-stream-base="/stream"><div id="stream-events" hidden></div><main id="main" data-topics="owner:t">`+
			`<section class="panel" id="logs" data-follow="owner:t"><h2>Logs</h2>`+picker+
			`<div class="logs">`+panes+`</div>`+note+`</section></main>`+
			`<script src="/static/portal.js"></script></body>`)
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return "http://" + ln.Addr().String() + "/logs"
}

type frontDoor struct {
	launch   string          // the one-time launch URL
	url      string          // the landing page's URL
	launched <-chan struct{} // closed when the launch token is spent
}

// serveFrontDoor serves local mode's front door on a free loopback port in
// front of a page that says whether the request carried the session.
func serveFrontDoor(t *testing.T) frontDoor {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	gate, err := auth.NewLocal(auth.LocalConfig{
		Identity: authz.Identity{Username: "browser-test"},
		Port:     port,
		Landing:  landing,
	})
	if err != nil {
		t.Fatal(err)
	}
	pages, err := ui.New(ui.Config{API: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{
		Handler: gate.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/static/") {
				pages.ServeHTTP(w, r)
				return
			}
			// A stand-in landing page under the page policy, loading the
			// page script that replaces the launch address.
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Security-Policy", ui.PagePolicy)
			if _, err := gate.Authenticate(r); err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = io.WriteString(w, "no session\n")
				return
			}
			_, _ = io.WriteString(w, `<!doctype html><meta charset="utf-8"><title>landing</title>`+
				`<body><main id="main" data-canonical="`+landing+`">signed in</main>`+
				`<script src="/static/portal.js"></script></body>`)
		})),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return frontDoor{launch: gate.LaunchURL(), url: "http://" + ln.Addr().String() + landing, launched: gate.Launched()}
}
