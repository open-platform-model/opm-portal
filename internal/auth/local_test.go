package auth

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

const port = 8123

var me = authz.Identity{Username: "kubernetes-admin", Groups: []string{"kubeadm:cluster-admins", "system:authenticated"}}

// fixture is a Local in front of a handler that records what reached it.
type fixture struct {
	l       *Local
	h       http.Handler
	path    string // the launch URL's path and query, kept past the launch
	now     time.Time
	reached int
	logs    bytes.Buffer
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{now: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)}
	l, err := NewLocal(LocalConfig{
		Identity: me,
		Port:     port,
		Landing:  "/api/v1alpha1/clusters/default/instances",
		Now:      func() time.Time { return f.now },
		Logger:   slog.New(slog.NewTextHandler(&f.logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	if err != nil {
		t.Fatal(err)
	}
	f.l = l
	f.path = launchPathOf(t, l)
	f.h = l.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.reached++
		if _, err := l.Authenticate(r); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "signed in at "+r.URL.RequestURI())
	}))
	return f
}

type request struct {
	method string
	target string
	host   string
	cookie *http.Cookie
	header map[string]string
}

// result is one response, read and closed.
type result struct {
	status  int
	header  http.Header
	cookies []*http.Cookie
	body    string
}

func (f *fixture) do(t *testing.T, rq request) result {
	t.Helper()
	if rq.method == "" {
		rq.method = http.MethodGet
	}
	req := httptest.NewRequestWithContext(t.Context(), rq.method, rq.target, http.NoBody)
	req.Host = "127.0.0.1:8123"
	if rq.host != "" {
		req.Host = rq.host
	}
	if rq.cookie != nil {
		req.AddCookie(rq.cookie)
	}
	for k, v := range rq.header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return result{status: res.StatusCode, header: res.Header, cookies: res.Cookies(), body: string(b)}
}

// launchPath returns the path and query of the launch URL as it was
// before any launch.
func (f *fixture) launchPath(*testing.T) string { return f.path }

func launchPathOf(t *testing.T, l *Local) string {
	t.Helper()
	u, err := url.Parse(l.LaunchURL())
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "http" || u.Host != "127.0.0.1:8123" || u.Path != LaunchPath || u.Query().Get("token") == "" {
		t.Fatalf("launch URL = %s", u.Redacted())
	}
	return u.RequestURI()
}

// checkHandOff checks that res is the landing page itself, served by the
// next handler under the session for the landing path, with no redirect
// and no token.
func checkHandOff(t *testing.T, res result) {
	t.Helper()
	if res.status != http.StatusOK {
		t.Fatalf("launch status = %d; want 200", res.status)
	}
	if res.header.Get("Location") != "" || res.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("launch headers = %v; want no Location and Cache-Control no-store", res.header)
	}
	if want := "signed in at /api/v1alpha1/clusters/default/instances"; res.body != want {
		t.Fatalf("launch answered %q; want the landing page under the session (%q)", res.body, want)
	}
}

// launch exchanges the token and returns the session cookie.
func (f *fixture) launch(t *testing.T) *http.Cookie {
	t.Helper()
	res := f.do(t, request{target: f.launchPath(t)})
	checkHandOff(t, res)
	for _, c := range res.cookies {
		if c.Name == f.l.CookieName() {
			return c
		}
	}
	t.Fatal("launch set no session cookie")
	return nil
}

func TestNewLocalRefusesNoIdentity(t *testing.T) {
	for _, id := range []authz.Identity{{}, {Username: "  "}, {Username: "system:anonymous", Groups: []string{"system:unauthenticated"}}} {
		if _, err := NewLocal(LocalConfig{Identity: id, Port: port}); err == nil {
			t.Errorf("NewLocal(%q) succeeded; want an error", id.Username)
		}
	}
	if _, err := NewLocal(LocalConfig{Identity: me}); err == nil {
		t.Error("NewLocal without a port succeeded; want an error")
	}
}

func TestLaunchExchangesTheTokenOnce(t *testing.T) {
	f := newFixture(t)
	path := f.launchPath(t)
	cookie := f.launch(t)

	// The session reads as the kubeconfig's identity.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody)
	req.AddCookie(cookie)
	s, err := f.l.Authenticate(req)
	if err != nil || s.Identity.Username != me.Username || s.Key == "" {
		t.Fatalf("Authenticate = %+v, %v", s.Identity, err)
	}
	if !s.Expires.Equal(f.now.Add(defaultSessionTTL)) {
		t.Errorf("session expires %v; want the launch plus the TTL", s.Expires)
	}
	if s.Key == cookie.Value {
		t.Fatal("the session key is the cookie value; it must not be")
	}
	if got := f.do(t, request{target: "/api/v1alpha1/clusters/default/instances", cookie: cookie}).status; got != http.StatusOK {
		t.Fatalf("read with the session = %d; want 200", got)
	}

	// The token is spent: a second client without the session is refused
	// and gets no cookie.
	res := f.do(t, request{target: path})
	if res.status != http.StatusForbidden || len(res.cookies) != 0 {
		t.Fatalf("second launch = %d with %d cookies; want 403 and none", res.status, len(res.cookies))
	}

	// The browser that holds the session is sent on, whatever the token.
	res = f.do(t, request{target: path, cookie: cookie})
	checkHandOff(t, res)
	if len(res.cookies) != 0 {
		t.Fatalf("relaunch with the session set %d cookies; want none", len(res.cookies))
	}
}

func TestLaunchRefusals(t *testing.T) {
	f := newFixture(t)
	targets := []string{LaunchPath, LaunchPath + "?token=", LaunchPath + "?token=AAAAAAAAAAAAAAAAAAAAAAAAAA"}
	bodies := make([]string, 0, len(targets))
	for _, target := range targets {
		res := f.do(t, request{target: target})
		if res.status != http.StatusForbidden || len(res.cookies) != 0 {
			t.Errorf("%s = %d; want 403 and no cookie", target, res.status)
		}
		bodies = append(bodies, res.body)
	}
	if bodies[0] != bodies[1] || bodies[1] != bodies[2] {
		t.Errorf("refusal bodies differ: %q", bodies)
	}
	// A refused attempt reaches nothing and does not spend the token; the
	// launch then serves the landing page once.
	if f.reached != 0 {
		t.Fatalf("refused launches reached the next handler %d times", f.reached)
	}
	if spent(f.l) {
		t.Fatal("Launched closed on a refused launch")
	}
	f.launch(t)
	if !spent(f.l) {
		t.Fatal("Launched is still open after the launch")
	}
	if f.reached != 1 {
		t.Fatalf("the launch reached the next handler %d times; want once, for the landing page", f.reached)
	}
	if got := f.do(t, request{method: http.MethodPost, target: f.launchPath(t), header: map[string]string{"Sec-Fetch-Site": "same-origin"}}).status; got != http.StatusMethodNotAllowed {
		t.Errorf("POST launch = %d; want 405", got)
	}
}

// spent reports whether l's Launched channel is closed.
func spent(l *Local) bool {
	select {
	case <-l.Launched():
		return true
	default:
		return false
	}
}

func TestCookieFlags(t *testing.T) {
	f := newFixture(t)
	res := f.do(t, request{target: f.launchPath(t)})
	raw := res.header.Get("Set-Cookie")
	for _, want := range []string{"opm-portal-8123=", "Path=/", "Max-Age=43200", "HttpOnly", "SameSite=Strict"} {
		if !strings.Contains(raw, want) {
			t.Errorf("Set-Cookie %q lacks %q", redactCookie(raw), want)
		}
	}
	for _, unwanted := range []string{"Domain=", "Secure", "__Host-"} {
		if strings.Contains(raw, unwanted) {
			t.Errorf("Set-Cookie %q carries %q", redactCookie(raw), unwanted)
		}
	}
	if res.header.Get("Cache-Control") != "no-store" {
		t.Errorf("launch Cache-Control = %q", res.header.Get("Cache-Control"))
	}
}

func redactCookie(raw string) string {
	name, rest, _ := strings.Cut(raw, "=")
	_, attrs, _ := strings.Cut(rest, ";")
	return name + "=<redacted>;" + attrs
}

func TestRequestsWithoutTheSession(t *testing.T) {
	f := newFixture(t)
	cookie := f.launch(t)
	forged := &http.Cookie{Name: cookie.Name, Value: "forged"}
	otherPort := &http.Cookie{Name: "opm-portal-9999", Value: cookie.Value}
	for name, c := range map[string]*http.Cookie{"none": nil, "forged": forged, "other name": otherPort} {
		if got := f.do(t, request{target: "/api/v1alpha1/clusters/default/instances", cookie: c}).status; got != http.StatusUnauthorized {
			t.Errorf("%s: status = %d; want 401", name, got)
		}
	}
}

func TestSessionExpires(t *testing.T) {
	f := newFixture(t)
	cookie := f.launch(t)
	f.now = f.now.Add(defaultSessionTTL)
	if got := f.do(t, request{target: "/x", cookie: cookie}).status; got != http.StatusUnauthorized {
		t.Fatalf("after the TTL: status = %d; want 401", got)
	}
}

func TestHostAllowlist(t *testing.T) {
	f := newFixture(t)
	cookie := f.launch(t)
	for _, host := range []string{"127.0.0.1:8123", "localhost:8123", "LOCALHOST:8123", "[::1]:8123"} {
		if got := f.do(t, request{target: "/x", host: host, cookie: cookie}).status; got != http.StatusOK {
			t.Errorf("Host %s = %d; want 200", host, got)
		}
	}
	reached := f.reached
	for _, host := range []string{"attacker.example:8123", "attacker.example", "127.0.0.1", "127.0.0.1:9999", "localhost", "127.0.0.2:8123", "0.0.0.0:8123", "127.0.0.1:8123.attacker.example"} {
		if got := f.do(t, request{target: "/x", host: host, cookie: cookie}).status; got != http.StatusForbidden {
			t.Errorf("Host %q = %d; want 403", host, got)
		}
	}
	// A rebinding page cannot launch either.
	if got := f.do(t, request{target: f.launchPath(t), host: "attacker.example:8123"}).status; got != http.StatusForbidden {
		t.Errorf("launch through a foreign host = %d; want 403", got)
	}
	if f.reached != reached {
		t.Fatalf("refused hosts reached the next handler %d times", f.reached-reached)
	}
}

// A portal bound to another loopback IP also answers that IP, and no other.
func TestHostAllowlistAddsTheBoundIP(t *testing.T) {
	l, err := NewLocal(LocalConfig{Identity: me, Host: "127.0.0.2", Port: port})
	if err != nil {
		t.Fatal(err)
	}
	h := l.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	for host, want := range map[string]int{
		"127.0.0.2:8123": http.StatusOK,
		"127.0.0.1:8123": http.StatusOK,
		"127.0.0.3:8123": http.StatusForbidden,
		"127.0.0.2:9999": http.StatusForbidden,
	} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Host %q = %d; want %d", host, rec.Code, want)
		}
	}
}

func TestCrossOriginWritesAreRefused(t *testing.T) {
	f := newFixture(t)
	cookie := f.launch(t)
	tests := []struct {
		name   string
		method string
		header map[string]string
		want   int
	}{
		{"cross-site POST", http.MethodPost, map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"same-site POST", http.MethodPost, map[string]string{"Sec-Fetch-Site": "same-site"}, http.StatusForbidden},
		{"foreign Origin DELETE", http.MethodDelete, map[string]string{"Origin": "http://attacker.example"}, http.StatusForbidden},
		{"same-origin POST", http.MethodPost, map[string]string{"Sec-Fetch-Site": "same-origin"}, http.StatusMethodNotAllowed},
		{"cross-site GET", http.MethodGet, map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := f.do(t, request{method: tt.method, target: "/x", cookie: cookie, header: tt.header}).status
			if got != tt.want {
				t.Fatalf("status = %d; want %d", got, tt.want)
			}
		})
	}
}

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	f := newFixture(t)
	cookie := f.launch(t)
	responses := map[string]result{
		"admitted":        f.do(t, request{target: "/x", cookie: cookie}),
		"unauthenticated": f.do(t, request{target: "/x"}),
		"foreign host":    f.do(t, request{target: "/x", host: "attacker.example:8123"}),
		"cross-origin":    f.do(t, request{method: http.MethodPost, target: "/x", header: map[string]string{"Sec-Fetch-Site": "cross-site"}}),
		"bad launch":      f.do(t, request{target: LaunchPath + "?token=x"}),
	}
	for name, res := range responses {
		for _, kv := range securityHeaders {
			if got := res.header.Get(kv[0]); got != kv[1] {
				t.Errorf("%s: %s = %q; want %q", name, kv[0], got, kv[1])
			}
		}
	}
	if csp := securityHeaders[0][1]; !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP = %q", csp)
	}
}

func TestNoTokenOrCookieInLogs(t *testing.T) {
	f := newFixture(t)
	u, err := url.Parse(f.l.LaunchURL())
	if err != nil {
		t.Fatal(err)
	}
	token := u.Query().Get("token")
	f.do(t, request{target: LaunchPath + "?token=wrong-token-value"})
	cookie := f.launch(t)
	f.do(t, request{target: "/x", host: "attacker.example:8123", cookie: cookie})
	f.do(t, request{target: f.launchPath(t)})
	logs := f.logs.String()
	if logs == "" {
		t.Fatal("no logs recorded")
	}
	for _, secret := range []string{token, cookie.Value, "wrong-token-value"} {
		if strings.Contains(logs, secret) {
			t.Fatalf("logs carry a secret: %s", logs)
		}
	}
}

func TestBoundHostIsAllowedAndLaunched(t *testing.T) {
	for host, want := range map[string]string{"127.0.0.5": "127.0.0.5:8123", "::1": "[::1]:8123"} {
		l, err := NewLocal(LocalConfig{Identity: me, Host: host, Port: port})
		if err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(l.LaunchURL())
		if err != nil || u.Host != want {
			t.Fatalf("launch URL host for %s = %q, %v; want %q", host, u.Host, err, want)
		}
		h := l.Handler(http.NotFoundHandler())
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody)
		req.Host = want
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusForbidden {
			t.Errorf("Host %s refused; want it allowed", want)
		}
	}
}

// TestAPageReplacesOnlyThePolicy: a page the next handler serves may set
// its own Content-Security-Policy; every other security header stays.
func TestAPageReplacesOnlyThePolicy(t *testing.T) {
	l, err := NewLocal(LocalConfig{Identity: me, Port: port})
	if err != nil {
		t.Fatal(err)
	}
	const pagePolicy = "default-src 'none'; script-src 'self'"
	h := l.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Security-Policy", pagePolicy)
	}))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	req.Host = "127.0.0.1:8123"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	for _, kv := range securityHeaders {
		want := kv[1]
		if kv[0] == "Content-Security-Policy" {
			want = pagePolicy
		}
		if got := rec.Header().Get(kv[0]); got != want {
			t.Errorf("%s = %q; want %q", kv[0], got, want)
		}
	}
}
