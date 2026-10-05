package auth

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/open-platform-model/opm-portal/internal/auth/oidctest"
)

// browser drives the front door the way a browser would, holding its
// cookies by hand: the portal's cookies are Secure and a test server is
// plain HTTP.
type browser struct {
	t       *testing.T
	f       *oidcFixture
	h       http.Handler
	cookies map[string]string
	reached int
	seen    []string // every cookie value and code the flow handled
}

func newBrowser(t *testing.T, f *oidcFixture) *browser {
	b := &browser{t: t, f: f, cookies: map[string]string{}}
	b.h = f.o.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.reached++
		id, _, err := f.o.Authenticate(r)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, id.Username+" at "+r.URL.RequestURI())
	}))
	return b
}

// answer is a portal response, read whole.
type answer struct {
	status  int
	header  http.Header
	body    string
	cookies []*http.Cookie
}

// do sends a request to the portal with the browser's cookies and keeps
// the cookies the answer sets.
func (b *browser) do(method, target string, header http.Header) answer {
	b.t.Helper()
	r := httptest.NewRequestWithContext(b.t.Context(), method, portalURL+target, http.NoBody)
	for k, v := range header {
		r.Header[k] = v
	}
	for name, value := range b.cookies {
		r.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	w := httptest.NewRecorder()
	b.h.ServeHTTP(w, r)
	res := w.Result()
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		b.t.Fatal(err)
	}
	for _, c := range res.Cookies() {
		if c.MaxAge < 0 {
			delete(b.cookies, c.Name)
			continue
		}
		b.cookies[c.Name] = c.Value
		b.seen = append(b.seen, c.Value)
	}
	return answer{status: res.StatusCode, header: res.Header, body: string(raw), cookies: res.Cookies()}
}

// authorize follows a redirect to the issuer and returns the portal path
// and query the issuer sends the browser back to.
func (b *browser) authorize(location string) string {
	b.t.Helper()
	client := *b.f.iss.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(b.t.Context(), http.MethodGet, location, http.NoBody)
	if err != nil {
		b.t.Fatal(err)
	}
	res, err := client.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	_ = res.Body.Close()
	back, err := url.Parse(res.Header.Get("Location"))
	if err != nil || res.StatusCode != http.StatusFound {
		b.t.Fatalf("issuer answered %d to %q", res.StatusCode, res.Header.Get("Location"))
	}
	if !strings.HasPrefix(back.String(), portalURL+CallbackPath) {
		b.t.Fatalf("issuer redirected to %q", back)
	}
	b.seen = append(b.seen, back.Query().Get("code"))
	return back.RequestURI()
}

// signIn runs the whole flow and returns the callback's answer.
func (b *browser) signIn(ret string) answer {
	b.t.Helper()
	res := b.do(http.MethodGet, LoginPath+"?"+url.Values{"return": {ret}}.Encode(), nil)
	if res.status != http.StatusFound {
		b.t.Fatalf("login answered %d", res.status)
	}
	return b.do(http.MethodGet, b.authorize(res.header.Get("Location")), nil)
}

func TestSignInWithCodeFlowAndPKCE(t *testing.T) {
	f := newOIDCFixture(t, nil, oidctest.Options{})
	f.iss.SetClaims(map[string]any{"sub": "alice", "groups": []string{"dev", "system:masters"}})
	b := newBrowser(t, f)

	res := b.do(http.MethodGet, LoginPath+"?return=%2Finstances%3Fnamespace%3Ddefault", nil)
	if res.status != http.StatusFound {
		t.Fatalf("login: %d", res.status)
	}
	auth := checkAuthorizeURL(t, f, res.header.Get("Location"))
	checkCookie(t, res, loginCookie)

	res = b.do(http.MethodGet, b.authorize(auth.String()), nil)
	if res.status != http.StatusSeeOther || res.header.Get("Location") != "/instances?namespace=default" {
		t.Fatalf("callback: %d to %q: %s", res.status, res.header.Get("Location"), res.body)
	}
	checkCookie(t, res, sessionCookie)
	if _, ok := b.cookies[loginCookie]; ok {
		t.Fatal("the callback left the login cookie set")
	}
	res = b.do(http.MethodGet, "/instances", nil)
	if got := res.body; got != "oidc:alice at /instances" {
		t.Fatalf("signed-in request: %d %q", res.status, got)
	}
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, portalURL+"/", http.NoBody)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: b.cookies[sessionCookie]})
	id, key, err := f.o.Authenticate(r)
	if err != nil || !slices.Equal(id.Groups, []string{"oidc:dev", "system:authenticated"}) || key == "" ||
		key == b.cookies[sessionCookie] {
		t.Fatalf("Authenticate = %+v, %q, %v", id, key, err)
	}
	// Signing in again with a live session goes straight to the return path.
	res = b.do(http.MethodGet, LoginPath+"?return=/platform", nil)
	if res.status != http.StatusSeeOther || res.header.Get("Location") != "/platform" {
		t.Fatalf("login with a session: %d to %q", res.status, res.header.Get("Location"))
	}
}

// checkAuthorizeURL asserts location is the issuer's authorization
// endpoint asking for a code with PKCE, state and nonce.
func checkAuthorizeURL(t *testing.T, f *oidcFixture, location string) *url.URL {
	t.Helper()
	auth, err := url.Parse(location)
	if err != nil || !strings.HasPrefix(auth.String(), f.iss.URL+oidctest.AuthorizePath) {
		t.Fatalf("login redirected to %q", location)
	}
	q := auth.Query()
	for k, want := range map[string]string{
		"client_id":             f.iss.ClientID(),
		"redirect_uri":          portalURL + CallbackPath,
		"response_type":         "code",
		"code_challenge_method": "S256",
	} {
		if q.Get(k) != want {
			t.Errorf("authorize %s = %q, want %q", k, q.Get(k), want)
		}
	}
	for _, k := range []string{"state", "nonce", "code_challenge"} {
		if q.Get(k) == "" {
			t.Errorf("authorize carries no %s", k)
		}
	}
	if !slices.Contains(strings.Fields(q.Get("scope")), "openid") {
		t.Errorf("scope %q lacks openid", q.Get("scope"))
	}
	return auth
}

// checkCookie asserts the cookie name was set with the __Host- rules,
// Secure, HttpOnly and SameSite=Lax.
func checkCookie(t *testing.T, res answer, name string) {
	t.Helper()
	for _, c := range res.cookies {
		if c.Name != name {
			continue
		}
		if !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Domain != "" || c.MaxAge <= 0 {
			t.Fatalf("cookie %s = %+v", name, c)
		}
		return
	}
	t.Fatalf("no %s cookie set", name)
}

func TestBearerAndBrowserMapToTheSameIdentity(t *testing.T) {
	f := newOIDCFixture(t, nil, oidctest.Options{})
	claims := map[string]any{"sub": "alice", "groups": []string{"dev", "ops"}}
	f.iss.SetClaims(claims)
	b := newBrowser(t, f)
	b.signIn("/")
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, portalURL+"/", http.NoBody)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: b.cookies[sessionCookie]})
	browserID, _, err := f.o.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	token := f.iss.Claims("alice", f.iss.ClientID())
	token["groups"] = claims["groups"]
	bearerID, _, err := f.o.Authenticate(bearerRequest(t, f.iss.Sign(token)))
	if err != nil {
		t.Fatal(err)
	}
	if browserID.Username != bearerID.Username || !slices.Equal(browserID.Groups, bearerID.Groups) {
		t.Fatalf("browser %+v, bearer %+v", browserID, bearerID)
	}
	// A bearer header never falls back to the cookie.
	r.Header.Set("Authorization", "Bearer not-a-token")
	if _, _, err := f.o.Authenticate(r); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("bad bearer with a live cookie: err = %v", err)
	}
}

func TestSignInRefusals(t *testing.T) {
	tests := []struct {
		name   string
		run    func(t *testing.T, f *oidcFixture, b *browser) answer
		status int
		body   string
	}{
		{
			name: "callback without a login cookie",
			run: func(t *testing.T, f *oidcFixture, b *browser) answer {
				res := b.do(http.MethodGet, LoginPath, nil)
				back := b.authorize(res.header.Get("Location"))
				delete(b.cookies, loginCookie)
				return b.do(http.MethodGet, back, nil)
			},
			status: http.StatusBadRequest, body: bodySignInExpired,
		},
		{
			name: "state from another sign-in",
			run: func(t *testing.T, f *oidcFixture, b *browser) answer {
				first := b.do(http.MethodGet, LoginPath, nil)
				back := b.authorize(first.header.Get("Location"))
				b.do(http.MethodGet, LoginPath, nil) // a second sign-in replaces the cookie
				return b.do(http.MethodGet, back, nil)
			},
			status: http.StatusBadRequest, body: bodySignInExpired,
		},
		{
			name: "forged login cookie",
			run: func(t *testing.T, f *oidcFixture, b *browser) answer {
				res := b.do(http.MethodGet, LoginPath, nil)
				back := b.authorize(res.header.Get("Location"))
				b.cookies[loginCookie] = "AAAA" + b.cookies[loginCookie][4:]
				return b.do(http.MethodGet, back, nil)
			},
			status: http.StatusBadRequest, body: bodySignInExpired,
		},
		{
			name: "sign-in took too long",
			run: func(t *testing.T, f *oidcFixture, b *browser) answer {
				res := b.do(http.MethodGet, LoginPath, nil)
				back := b.authorize(res.header.Get("Location"))
				f.clk.Advance(loginTTL)
				return b.do(http.MethodGet, back, nil)
			},
			status: http.StatusBadRequest, body: bodySignInExpired,
		},
		{
			name: "issuer refuses",
			run: func(t *testing.T, f *oidcFixture, b *browser) answer {
				f.iss.Deny(true)
				return b.signIn("/")
			},
			status: http.StatusForbidden, body: bodySignInRefused,
		},
		{
			name: "ID token for another nonce",
			run: func(t *testing.T, f *oidcFixture, b *browser) answer {
				f.iss.SetClaims(map[string]any{"sub": "alice", "nonce": "replayed"})
				return b.signIn("/")
			},
			status: http.StatusForbidden, body: bodySignInFailed,
		},
		{
			name: "ID token for another audience",
			run: func(t *testing.T, f *oidcFixture, b *browser) answer {
				f.iss.SetClaims(map[string]any{"sub": "alice", "aud": "another-client"})
				return b.signIn("/")
			},
			status: http.StatusForbidden, body: bodySignInFailed,
		},
		{
			name: "replayed code",
			run: func(t *testing.T, f *oidcFixture, b *browser) answer {
				res := b.do(http.MethodGet, LoginPath, nil)
				login := b.cookies[loginCookie]
				back := b.authorize(res.header.Get("Location"))
				b.do(http.MethodGet, back, nil)
				delete(b.cookies, sessionCookie)
				b.cookies[loginCookie] = login
				return b.do(http.MethodGet, back, nil)
			},
			status: http.StatusForbidden, body: bodySignInFailed,
		},
		{
			name: "empty username claim",
			run: func(t *testing.T, f *oidcFixture, b *browser) answer {
				f.iss.SetClaims(map[string]any{"sub": "", "groups": []string{"dev"}})
				return b.signIn("/")
			},
			status: http.StatusForbidden, body: bodyUnmapped,
		},
		{
			name: "system username",
			run: func(t *testing.T, f *oidcFixture, b *browser) answer {
				f.iss.SetClaims(map[string]any{"sub": "system:admin"})
				return b.signIn("/")
			},
			status: http.StatusForbidden, body: bodyUnmapped,
		},
		{
			name: "callback with POST",
			run: func(t *testing.T, f *oidcFixture, b *browser) answer {
				return b.do(http.MethodPost, CallbackPath, nil)
			},
			status: http.StatusMethodNotAllowed, body: "method not allowed\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newOIDCFixture(t, nil, oidctest.Options{})
			b := newBrowser(t, f)
			res := tt.run(t, f, b)
			if got := res.body; res.status != tt.status || got != tt.body {
				t.Fatalf("answer %d %q, want %d %q", res.status, got, tt.status, tt.body)
			}
			if _, ok := b.cookies[sessionCookie]; ok && tt.name != "replayed code" {
				t.Fatal("a refused callback set a session cookie")
			}
			for _, c := range res.cookies {
				if c.Name == sessionCookie && c.MaxAge > 0 {
					t.Fatal("a refused callback set a session cookie")
				}
			}
			assertSecurityHeaders(t, res)
			assertNoSecrets(t, f, b)
		})
	}
}

func TestReturnPathStaysOnThePortal(t *testing.T) {
	tests := map[string]string{
		"/instances?namespace=default": "/instances?namespace=default",
		"//evil.example":               "/",
		"/\\evil.example":              "/",
		"/a/../\\evil.example":         "/",
		"/a/..//evil.example":          "/evil.example",
		"/ok\\":                        "/",
		"https://evil.example/":        "/",
		"evil.example":                 "/",
		"":                             "/",
		"/ok\r\nLocation: x":           "/",
	}
	for ret, want := range tests {
		t.Run(ret, func(t *testing.T) {
			f := newOIDCFixture(t, nil, oidctest.Options{})
			b := newBrowser(t, f)
			res := b.signIn(ret)
			if res.status != http.StatusSeeOther || res.header.Get("Location") != want {
				t.Fatalf("landed %d at %q, want %q", res.status, res.header.Get("Location"), want)
			}
			// A browser already signed in is redirected at once, with no
			// round trip to the issuer.
			res = b.do(http.MethodGet, LoginPath+"?"+url.Values{"return": {ret}}.Encode(), nil)
			if res.status != http.StatusSeeOther || res.header.Get("Location") != want {
				t.Fatalf("signed in: landed %d at %q, want %q", res.status, res.header.Get("Location"), want)
			}
		})
	}
}

func TestLogoutRevokesTheSession(t *testing.T) {
	sameOrigin := http.Header{"Sec-Fetch-Site": {"same-origin"}}
	t.Run("to the end-session endpoint", func(t *testing.T) {
		f := newOIDCFixture(t, func(c *OIDCConfig) { c.PostLogoutRedirectURL = portalURL + "/signed-out" },
			oidctest.Options{EndSession: true})
		b := newBrowser(t, f)
		b.signIn("/")
		old := b.cookies[sessionCookie]
		res := b.do(http.MethodPost, LogoutPath, sameOrigin)
		end, err := url.Parse(res.header.Get("Location"))
		if res.status != http.StatusSeeOther || err != nil || !strings.HasPrefix(end.String(), f.iss.URL+oidctest.EndSessionPath) {
			t.Fatalf("logout: %d to %q", res.status, res.header.Get("Location"))
		}
		if end.Query().Get("client_id") != f.iss.ClientID() || end.Query().Get("post_logout_redirect_uri") != portalURL+"/signed-out" {
			t.Fatalf("end-session query %q", end.RawQuery)
		}
		if _, ok := b.cookies[sessionCookie]; ok {
			t.Fatal("logout left the session cookie set")
		}
		b.cookies[sessionCookie] = old
		if res := b.do(http.MethodGet, "/x", nil); res.status != http.StatusUnauthorized {
			t.Fatalf("the old cookie after logout: %d", res.status)
		}
	})
	t.Run("without an end-session endpoint", func(t *testing.T) {
		f := newOIDCFixture(t, nil, oidctest.Options{})
		b := newBrowser(t, f)
		b.signIn("/")
		res := b.do(http.MethodPost, LogoutPath, sameOrigin)
		if got := res.body; res.status != http.StatusOK || got != bodySignedOut {
			t.Fatalf("logout: %d %q", res.status, got)
		}
	})
	t.Run("cross-origin", func(t *testing.T) {
		f := newOIDCFixture(t, nil, oidctest.Options{})
		b := newBrowser(t, f)
		b.signIn("/")
		res := b.do(http.MethodPost, LogoutPath, http.Header{"Sec-Fetch-Site": {"cross-site"}})
		if res.status != http.StatusForbidden {
			t.Fatalf("cross-site logout: %d", res.status)
		}
		assertSecurityHeaders(t, res)
		if res := b.do(http.MethodGet, "/x", nil); res.status != http.StatusOK {
			t.Fatalf("the session after a cross-site logout: %d", res.status)
		}
	})
	t.Run("GET", func(t *testing.T) {
		f := newOIDCFixture(t, nil, oidctest.Options{})
		res := newBrowser(t, f).do(http.MethodGet, LogoutPath, nil)
		if res.status != http.StatusMethodNotAllowed || res.header.Get("Allow") != http.MethodPost {
			t.Fatalf("GET logout: %d", res.status)
		}
	})
}

func TestSessionLifetimeAndCap(t *testing.T) {
	t.Run("expiry", func(t *testing.T) {
		f := newOIDCFixture(t, func(c *OIDCConfig) { c.SessionTTL = time.Hour }, oidctest.Options{})
		b := newBrowser(t, f)
		b.signIn("/")
		f.clk.Advance(time.Hour - time.Second)
		if res := b.do(http.MethodGet, "/x", nil); res.status != http.StatusOK {
			t.Fatalf("before expiry: %d", res.status)
		}
		f.clk.Advance(time.Second)
		if res := b.do(http.MethodGet, "/x", nil); res.status != http.StatusUnauthorized {
			t.Fatalf("at expiry: %d", res.status)
		}
	})
	t.Run("cap evicts the session closest to expiry", func(t *testing.T) {
		f := newOIDCFixture(t, func(c *OIDCConfig) { c.MaxSessions = 2 }, oidctest.Options{})
		browsers := make([]*browser, 3)
		for i := range browsers {
			browsers[i] = newBrowser(t, f)
			browsers[i].signIn("/")
			f.clk.Advance(time.Second)
		}
		want := []int{http.StatusUnauthorized, http.StatusOK, http.StatusOK}
		for i, b := range browsers {
			if res := b.do(http.MethodGet, "/x", nil); res.status != want[i] {
				t.Fatalf("browser %d: %d, want %d", i, res.status, want[i])
			}
		}
	})
}

func TestNavigationWithoutASessionSignsIn(t *testing.T) {
	f := newOIDCFixture(t, nil, oidctest.Options{})
	tests := []struct {
		name     string
		header   http.Header
		redirect bool
	}{
		{"navigation", http.Header{"Sec-Fetch-Mode": {"navigate"}}, true},
		{"old browser asking for HTML", http.Header{"Accept": {"text/html,application/xhtml+xml"}}, true},
		{"fetch", http.Header{"Sec-Fetch-Mode": {"cors"}, "Accept": {"text/html"}}, false},
		{"htmx", http.Header{"Sec-Fetch-Mode": {"navigate"}, "Hx-Request": {"true"}}, false},
		{"API client", http.Header{"Accept": {"application/json"}}, false},
		{"bearer client", http.Header{"Sec-Fetch-Mode": {"navigate"}, "Authorization": {"Bearer x"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newBrowser(t, f)
			res := b.do(http.MethodGet, "/instances?namespace=default", tt.header)
			assertSecurityHeaders(t, res)
			if tt.redirect {
				if res.status != http.StatusSeeOther || res.header.Get("Location") != LoginPath+"?return=%2Finstances%3Fnamespace%3Ddefault" {
					t.Fatalf("answer %d to %q", res.status, res.header.Get("Location"))
				}
				if b.reached != 0 {
					t.Fatal("a redirected request reached next")
				}
				return
			}
			if res.status != http.StatusUnauthorized || b.reached != 1 {
				t.Fatalf("answer %d, reached %d; want next's 401", res.status, b.reached)
			}
		})
	}
	t.Run("signed in", func(t *testing.T) {
		b := newBrowser(t, f)
		b.signIn("/")
		res := b.do(http.MethodGet, "/instances", http.Header{"Sec-Fetch-Mode": {"navigate"}})
		if res.status != http.StatusOK {
			t.Fatalf("answer %d", res.status)
		}
		assertSecurityHeaders(t, res)
	})
}

func assertSecurityHeaders(t *testing.T, res answer) {
	t.Helper()
	for _, kv := range securityHeaders {
		if got := res.header.Get(kv[0]); got != kv[1] {
			t.Errorf("%s = %q, want %q", kv[0], got, kv[1])
		}
	}
	if got := res.header.Get("Strict-Transport-Security"); got != hsts {
		t.Errorf("Strict-Transport-Security = %q", got)
	}
}

// assertNoSecrets fails when a cookie value, a code or the client secret
// reached the logs.
func assertNoSecrets(t *testing.T, f *oidcFixture, b *browser) {
	t.Helper()
	logs := f.logs.String()
	for _, s := range append(slices.Clone(b.seen), f.iss.ClientSecret()) {
		if s != "" && strings.Contains(logs, s) {
			t.Fatalf("a secret reached the logs: %s", logs)
		}
	}
}

func TestSignInLogsNoSecrets(t *testing.T) {
	f := newOIDCFixture(t, nil, oidctest.Options{EndSession: true})
	b := newBrowser(t, f)
	b.signIn("/")
	b.do(http.MethodPost, LogoutPath, http.Header{"Sec-Fetch-Site": {"same-origin"}})
	if !strings.Contains(f.logs.String(), "browser signed in") {
		t.Fatalf("no sign-in log line: %s", f.logs.String())
	}
	assertNoSecrets(t, f, b)
}
