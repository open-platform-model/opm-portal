//go:build browser

package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
// navigation, and a stale stored value is dropped (portal:D14).
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
// Whole graph, and the state and full screen a live refresh keeps.
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

// phonePages lists the pages TestBrowserPhone opens: the 25 non-fragment
// paths of f1Pages in internal/ui/ui_test.go, which package main cannot
// import, and the not-found page. A UI change that adds a page adds its path
// here, as it adds it to f1Pages.
var phonePages = []string{
	"/",
	"/?tab=catalogs",
	"/?pstatus=refused&eresource=registration:default.refused-claim-fixture",
	"/installed",
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
	"/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0",
	"/catalog?path=testing.opmodel.dev/catalogs/operator/refused-claim-fixture-absent@v0",
	"/catalog?path=opmodel.dev/catalogs/opm@v4&tab=events",
	"/instances/cert-manager/cert-manager?tab=resources",
	"/instances/cert-manager/cert-manager",
	"/instances/cert-manager/cert-manager?expand=grp:configuration/mi/cert-manager/cert-manager",
	"/instances/web/web",
	"/packages/pkg/podinfo",
	"/packages/pkg/podinfo?tab=resources",
	"/nope",
}

// TestBrowserPhone opens every page of phonePages at a 360 px wide viewport
// in real browsers (task test:browser) and fails when a page's scroll width
// is above 360. The body clips sideways overflow, so only a browser that
// measures the layout shows content a phone would lose. The script first
// proves the measure can fail with an element it adds that is 500 px wide.
func TestBrowserPhone(t *testing.T) {
	script := browserScript(t, "phone.py")
	for _, browser := range []string{"chromium", "firefox", "webkit"} {
		t.Run(browser, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
			defer cancel()
			args := append([]string{browser, serveF1Site(t)}, phonePages...)
			if err := playwright(ctx, t, script, nil, args...); err != nil {
				t.Fatalf("a page is wider than 360 px: %v", err)
			}
		})
	}
}

// serveF1Site serves, on a free loopback port, the read API and the pages
// over the F1 capture, every request reading as the test caller.
func serveF1Site(t *testing.T) string {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := apitest.New(t, apitest.F1(t), apitest.AllowAll)
	pages, err := ui.New(ui.Config{API: srv})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", srv)
	mux.Handle("/", pages)
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
// closes for good instead of reconnecting into a refusal, and that the page
// says the session expired.
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
			if n := streams.Load(); n != 1 {
				t.Errorf("the page opened %d streams; want 1, and no reconnect after expired", n)
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
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return "http://" + ln.Addr().String() + "/", &streams
}

// serveLogsPage serves, on a free loopback port, a page holding a logs
// region shaped as the owner page renders it. Its first render lists Pod
// a's pane, its second adds Pod b's, and every later one lists no Pod.
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
	pane := func(pod string) string {
		return `<details class="log" id="log_` + pod + `_c" data-log-topic="log:ns/` + pod + `/c">` +
			`<summary><span class="mono">` + pod + `</span> <span class="log-container">c</span></summary>` +
			`<div class="log-tools"><label><input type="checkbox" data-log-previous="log:ns/` + pod + `/c/previous"> Previous instance</label>` +
			`<span class="log-state" aria-live="polite"></span></div>` +
			`<pre class="log-pane" tabindex="0" aria-label="Log of c in ` + pod + `"></pre></details>`
	}
	var renders atomic.Int32
	mux := http.NewServeMux()
	mux.Handle("/static/", pages)
	mux.HandleFunc("/logs", func(w http.ResponseWriter, _ *http.Request) {
		var panes, note string
		switch renders.Add(1) {
		case 1:
			panes, note = pane("a"), `<p class="note">Logs stream only while a pane is open.</p>`
		case 2:
			panes, note = pane("a")+pane("b"), `<p class="note">Logs stream only while a pane is open.</p>`
		default:
			note = `<p class="muted">No Pod below this inventory that you may read.</p>`
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", ui.PagePolicy)
		_, _ = io.WriteString(w, `<!doctype html><meta charset="utf-8"><title>logs</title>`+
			`<link rel="stylesheet" href="/static/portal.css">`+
			`<body><div id="stream-events" hidden></div><main id="main" data-topics="owner:t">`+
			`<section class="panel" id="logs" data-follow="owner:t"><h2>Logs</h2>`+
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
