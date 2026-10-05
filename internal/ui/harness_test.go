package ui

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// site is the read API and the UI mounted the way serve mounts them.
type site struct {
	handler http.Handler
}

// newSite serves objs over a local-mode read API; the caller's reviews
// follow rule.
func newSite(t testing.TB, objs []*unstructured.Unstructured, rule apitest.Rule) *site {
	t.Helper()
	return mount(t, apitest.New(t, objs, rule))
}

// newInClusterSite serves objs over an in-cluster read API.
func newInClusterSite(t testing.TB, objs []*unstructured.Unstructured, rule apitest.Rule) *site {
	t.Helper()
	return mount(t, apitest.NewInCluster(t, objs, rule))
}

func mount(t testing.TB, srv http.Handler) *site {
	t.Helper()
	h, err := New(Config{API: srv, Now: func() time.Time { return apitest.Now }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", srv)
	mux.Handle("/", h)
	return &site{handler: mux}
}

type result struct {
	status int
	header http.Header
	body   string
}

// get requests path with the session, or without it when anon is set,
// and with HX-Request when hx is set.
func (s *site) get(t testing.TB, path string, opts ...func(*http.Request)) result {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	req.AddCookie(&http.Cookie{Name: apitest.SessionCookie, Value: apitest.SessionValue})
	for _, o := range opts {
		o(req)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return result{status: res.StatusCode, header: res.Header, body: string(b)}
}

func anonymous(r *http.Request) { r.Header.Del("Cookie") }

func htmxRequest(r *http.Request) { r.Header.Set("HX-Request", "true") }

// TestDevServe serves the UI over the F1 capture on the address in
// OPM_PORTAL_UI_DEV until OPM_PORTAL_UI_DEV_FOR passes (default 10m), for
// looking at the pages in a browser. Every request reads as the test
// caller, so no launch is needed. Skipped unless the variable is set.
func TestDevServe(t *testing.T) {
	addr := os.Getenv("OPM_PORTAL_UI_DEV")
	if addr == "" {
		t.Skip("OPM_PORTAL_UI_DEV is not set")
	}
	objs := apitest.F1(t)
	if os.Getenv("OPM_PORTAL_UI_DEV_BROKEN") != "" {
		objs = apitest.F1Broken(t)
	}
	rule := apitest.AllowAll
	if deny := os.Getenv("OPM_PORTAL_UI_DEV_DENY"); deny != "" {
		rule = apitest.DenyResources(deny)
	}
	s := newSite(t, objs, rule)
	d, err := time.ParseDuration(os.Getenv("OPM_PORTAL_UI_DEV_FOR"))
	if err != nil {
		d = 10 * time.Minute
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.AddCookie(&http.Cookie{Name: apitest.SessionCookie, Value: apitest.SessionValue})
			s.handler.ServeHTTP(w, r)
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = srv.Serve(ln) }()
	t.Logf("serving the UI over F1 on http://%s", ln.Addr())
	select {
	case <-time.After(d):
	case <-t.Context().Done():
	}
	_ = srv.Close()
}
