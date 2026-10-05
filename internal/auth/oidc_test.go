package auth

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"

	"github.com/open-platform-model/opm-portal/internal/auth/oidctest"
)

const portalURL = "https://portal.example.com"

// clock is a settable time shared by the portal and the fake issuer.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// oidcFixture is an OIDC authenticator against an in-process issuer.
type oidcFixture struct {
	iss  *oidctest.Issuer
	o    *OIDC
	clk  *clock
	logs *bytes.Buffer
}

func newOIDCFixture(t *testing.T, edit func(*OIDCConfig), opts oidctest.Options) *oidcFixture {
	t.Helper()
	clk := &clock{now: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)}
	opts.Now = clk.Now
	iss := oidctest.New(t, opts)
	logs := &bytes.Buffer{}
	cfg := testOIDCConfig(iss)
	cfg.Now = clk.Now
	cfg.Logger = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	if edit != nil {
		edit(&cfg)
	}
	o, err := NewOIDC(t.Context(), cfg)
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	return &oidcFixture{iss: iss, o: o, clk: clk, logs: logs}
}

func testOIDCConfig(iss *oidctest.Issuer) OIDCConfig {
	return OIDCConfig{
		IssuerURL:      iss.URL,
		ClientID:       iss.ClientID(),
		ClientSecret:   iss.ClientSecret(),
		RedirectURL:    portalURL + CallbackPath,
		UsernamePrefix: "oidc:",
		GroupsClaim:    "groups",
		GroupsPrefix:   "oidc:",
		HTTPClient:     iss.Client(),
	}
}

// bearerRequest is a GET carrying token as a bearer token.
func bearerRequest(t *testing.T, token string) *http.Request {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, portalURL+"/api/v1alpha1/clusters/default/instances", http.NoBody)
	r.Header.Set("Authorization", "Bearer "+token)
	return r
}

// countingTransport fails every request and counts them.
type countingTransport struct {
	mu    sync.Mutex
	count int
}

func (c *countingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
	return nil, errors.New("no network in this test")
}

func TestNewOIDCRefusesUnsafeConfiguration(t *testing.T) {
	valid := OIDCConfig{
		IssuerURL:      "https://issuer.example.com",
		ClientID:       "opm-portal",
		RedirectURL:    portalURL + CallbackPath,
		UsernamePrefix: "oidc:",
		GroupsClaim:    "groups",
		GroupsPrefix:   "oidc:",
	}
	tests := []struct {
		name    string
		edit    func(*OIDCConfig)
		wantErr string
	}{
		{"plain http issuer", func(c *OIDCConfig) { c.IssuerURL = "http://issuer.example.com" }, "https"},
		{"relative issuer", func(c *OIDCConfig) { c.IssuerURL = "issuer.example.com" }, "https"},
		{"no client ID", func(c *OIDCConfig) { c.ClientID = "" }, "client ID"},
		{"relative redirect", func(c *OIDCConfig) { c.RedirectURL = CallbackPath }, "absolute"},
		{"redirect elsewhere", func(c *OIDCConfig) { c.RedirectURL = portalURL + "/callback" }, CallbackPath},
		{"plain http redirect", func(c *OIDCConfig) { c.RedirectURL = "http://portal.example.com" + CallbackPath }, "loopback"},
		{"empty username prefix", func(c *OIDCConfig) { c.UsernamePrefix = "" }, "username prefix is empty"},
		{"empty groups prefix", func(c *OIDCConfig) { c.GroupsPrefix = "" }, "groups prefix is empty"},
		{"system groups prefix", func(c *OIDCConfig) { c.GroupsPrefix = "system:" }, "system:"},
		{"partial system prefix", func(c *OIDCConfig) { c.UsernamePrefix = "sys" }, "system:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.edit(&cfg)
			rt := &countingTransport{}
			cfg.HTTPClient = &http.Client{Transport: rt}
			o, err := NewOIDC(t.Context(), cfg)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want one naming %q", err, tt.wantErr)
			}
			if o != nil {
				t.Fatal("a refused configuration returned an authenticator")
			}
			if rt.count != 0 {
				t.Fatalf("%d requests reached the issuer before the configuration was checked", rt.count)
			}
		})
	}
}

func TestNewOIDCAcceptsLoopbackHTTPRedirectAndTrustedEmptyPrefixes(t *testing.T) {
	newOIDCFixture(t, func(c *OIDCConfig) {
		c.RedirectURL = "http://127.0.0.1:8080" + CallbackPath
		c.UsernamePrefix, c.GroupsPrefix, c.APIServerTrustsIssuer = "", "", true
	}, oidctest.Options{})
}

func TestNewOIDCFailsWithoutTheIssuer(t *testing.T) {
	iss := oidctest.New(t, oidctest.Options{})
	tests := []struct {
		name string
		edit func(*OIDCConfig)
	}{
		{"unreachable", func(c *OIDCConfig) { c.IssuerURL = "https://127.0.0.1:1" }},
		{"discovered issuer differs", func(c *OIDCConfig) { c.IssuerURL = iss.URL + "/" }},
		{"untrusted certificate", func(c *OIDCConfig) { c.HTTPClient = &http.Client{} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testOIDCConfig(iss)
			tt.edit(&cfg)
			if o, err := NewOIDC(t.Context(), cfg); err == nil || o != nil {
				t.Fatalf("NewOIDC = %v, %v; want an error", o, err)
			}
		})
	}
}

func TestBearerTokenAuthenticates(t *testing.T) {
	f := newOIDCFixture(t, nil, oidctest.Options{})
	claims := f.iss.Claims("alice", f.iss.ClientID())
	claims["groups"] = []string{"dev", "system:masters"}
	token := f.iss.Sign(claims)
	id, key, err := f.o.Authenticate(bearerRequest(t, token))
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if id.Username != "oidc:alice" || !slices.Equal(id.Groups, []string{"oidc:dev", "system:authenticated"}) {
		t.Fatalf("identity = %+v", id)
	}
	if !strings.HasPrefix(key, "bearer:") || strings.Contains(key, token) {
		t.Fatalf("session key %q", key)
	}
	_, again, _ := f.o.Authenticate(bearerRequest(t, token))
	if again != key {
		t.Fatal("one token gave two session keys")
	}
	claims["jti"] = "other"
	_, other, err := f.o.Authenticate(bearerRequest(t, f.iss.Sign(claims)))
	if err != nil || other == key {
		t.Fatalf("another token: key %q, err %v", other, err)
	}
	// The scheme is case-insensitive.
	r := bearerRequest(t, token)
	r.Header.Set("Authorization", "bearer "+token)
	if _, _, err := f.o.Authenticate(r); err != nil {
		t.Fatalf("lowercase scheme: %v", err)
	}
}

func TestBearerTokenNamesTheConfiguredAudience(t *testing.T) {
	f := newOIDCFixture(t, func(c *OIDCConfig) { c.Audience = "opm-portal-api" }, oidctest.Options{ClientID: "portal-ui"})
	if _, _, err := f.o.Authenticate(bearerRequest(t, f.iss.Sign(f.iss.Claims("alice", "opm-portal-api")))); err != nil {
		t.Fatalf("configured audience: %v", err)
	}
	if _, _, err := f.o.Authenticate(bearerRequest(t, f.iss.Sign(f.iss.Claims("alice", f.iss.ClientID())))); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("client ID audience: err = %v, want ErrInvalidToken", err)
	}
}

func TestBearerTokenRefusals(t *testing.T) {
	f := newOIDCFixture(t, nil, oidctest.Options{})
	foreign, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	good := func() map[string]any { return f.iss.Claims("alice", f.iss.ClientID()) }
	with := func(k string, v any) map[string]any {
		c := good()
		c[k] = v
		return c
	}
	knownKID := jwtHeader(t, f.iss.Sign(good()))["kid"].(string)
	const notVerified = "does not verify"
	tokens := map[string]struct{ token, reason string }{
		"wrong audience":          {f.iss.Sign(with("aud", "someone-else")), notVerified},
		"wrong issuer":            {f.iss.Sign(with("iss", "https://evil.example.com")), notVerified},
		"expired":                 {f.iss.Sign(with("exp", f.clk.Now().Add(-time.Minute).Unix())), "expired"},
		"not yet valid":           {f.iss.Sign(with("nbf", f.clk.Now().Add(time.Hour).Unix())), notVerified},
		"foreign key":             {oidctest.SignWith(t, foreign, "foreign", jose.RS256, good()), notVerified},
		"foreign key, known kid":  {oidctest.SignWith(t, foreign, knownKID, jose.RS256, good()), notVerified},
		"shared secret":           {oidctest.SignWith(t, []byte(strings.Repeat("k", 32)), knownKID, jose.HS256, good()), notVerified},
		"alg none":                {unsignedToken(t, good()), notVerified},
		"garbage":                 {"not-a-jwt", notVerified},
		"tampered payload":        {tamper(t, f.iss.Sign(good())), notVerified},
		"too large":               {f.iss.Sign(with("pad", strings.Repeat("x", maxBearerBytes))), "too large"},
		"empty username (bearer)": {f.iss.Sign(with("sub", "")), "map to no user"},
	}
	for name, tt := range tokens {
		t.Run(name, func(t *testing.T) {
			f.logs.Reset()
			id, key, err := f.o.Authenticate(bearerRequest(t, tt.token))
			if !errors.Is(err, ErrInvalidToken) || id.Username != "" || key != "" {
				t.Fatalf("Authenticate = %+v, %q, %v; want ErrInvalidToken", id, key, err)
			}
			if !strings.Contains(f.logs.String(), tt.reason) {
				t.Fatalf("log %q does not give the reason %q", f.logs.String(), tt.reason)
			}
			if strings.Contains(f.logs.String(), tt.token) || strings.Contains(err.Error(), tt.token) {
				t.Fatal("the token reached a log line or an error")
			}
		})
	}
	headers := map[string][]string{
		"basic scheme":    {"Basic YWxpY2U6cGFzcw=="},
		"empty bearer":    {"Bearer "},
		"bare token":      {f.iss.Sign(good())},
		"two headers":     {"Bearer " + f.iss.Sign(good()), "Bearer " + f.iss.Sign(good())},
		"empty header":    {""},
		"scheme only":     {"Bearer"},
		"token in scheme": {"Bearer" + f.iss.Sign(good())},
	}
	for name, values := range headers {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, portalURL+"/", http.NoBody)
			r.Header["Authorization"] = values
			if _, _, err := f.o.Authenticate(r); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("err = %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestUnknownKeyIDsFetchTheKeySetAtMostOncePerInterval(t *testing.T) {
	f := newOIDCFixture(t, nil, oidctest.Options{})
	if got := f.iss.KeyFetches(); got != 1 {
		t.Fatalf("key fetches after construction = %d, want 1", got)
	}
	foreign, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	burst := func() {
		for i := range 20 {
			token := oidctest.SignWith(t, foreign, "unknown-"+string(rune('a'+i)), jose.RS256, f.iss.Claims("alice", f.iss.ClientID()))
			if _, _, err := f.o.Authenticate(bearerRequest(t, token)); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("err = %v, want ErrInvalidToken", err)
			}
		}
	}
	burst()
	if got := f.iss.KeyFetches(); got != 1 {
		t.Fatalf("key fetches after a burst within the interval = %d, want 1", got)
	}
	f.clk.Advance(defaultKeyRefreshInterval)
	burst()
	if got := f.iss.KeyFetches(); got != 2 {
		t.Fatalf("key fetches after a burst past the interval = %d, want 2", got)
	}
}

func TestRotatedKeyIsAcceptedAfterTheInterval(t *testing.T) {
	f := newOIDCFixture(t, nil, oidctest.Options{})
	f.iss.Rotate()
	token := f.iss.Sign(f.iss.Claims("alice", f.iss.ClientID()))
	if _, _, err := f.o.Authenticate(bearerRequest(t, token)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("within the interval: err = %v, want ErrInvalidToken", err)
	}
	f.clk.Advance(defaultKeyRefreshInterval)
	token = f.iss.Sign(f.iss.Claims("alice", f.iss.ClientID()))
	if _, _, err := f.o.Authenticate(bearerRequest(t, token)); err != nil {
		t.Fatalf("after the interval: %v", err)
	}
	if got := f.iss.KeyFetches(); got != 2 {
		t.Fatalf("key fetches = %d, want 2", got)
	}
}

func TestStaleKeysAreRefreshedBeforeUse(t *testing.T) {
	f := newOIDCFixture(t, nil, oidctest.Options{})
	f.clk.Advance(keySetMaxAge)
	if _, _, err := f.o.Authenticate(bearerRequest(t, f.iss.Sign(f.iss.Claims("alice", f.iss.ClientID())))); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if got := f.iss.KeyFetches(); got != 2 {
		t.Fatalf("key fetches = %d, want 2", got)
	}
}

func jwtHeader(t *testing.T, token string) map[string]any {
	t.Helper()
	jws, err := jose.ParseSigned(token, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{"kid": jws.Signatures[0].Header.KeyID}
}

func unsignedToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	enc := base64.RawURLEncoding
	return enc.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`)) + "." + enc.EncodeToString(mustJSON(t, claims)) + "."
}

// tamper replaces a signed token's payload, keeping its signature.
func tamper(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	parts[1] = base64.RawURLEncoding.EncodeToString(mustJSON(t, map[string]any{"sub": "mallory"}))
	return strings.Join(parts, ".")
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
