// Package oidctest runs an in-process OpenID Connect issuer for tests: a
// TLS httptest server with discovery, a key set, an authorization endpoint
// that approves every request with the claims the test set, and a token
// endpoint that checks the client and the PKCE verifier. It is not an
// identity provider and must never run outside a test.
package oidctest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// Endpoint paths under the issuer URL.
const (
	AuthorizePath  = "/authorize"
	TokenPath      = "/token"
	KeysPath       = "/keys"
	EndSessionPath = "/logout"
)

// Options tune an Issuer.
type Options struct {
	// ClientID and ClientSecret are the only client the issuer serves.
	// Default "opm-portal" and "portal-secret".
	ClientID     string
	ClientSecret string
	// EndSession advertises an end_session_endpoint in discovery.
	EndSession bool
	// Now is the clock tokens are stamped with. Default time.Now.
	Now func() time.Time
}

// Issuer is the in-process issuer. Its methods fail the test rather than
// return errors.
type Issuer struct {
	// URL is the issuer identifier and base URL.
	URL string

	t    testing.TB
	opts Options
	srv  *httptest.Server

	mu         sync.Mutex
	keys       []signingKey // published; the last one signs
	claims     map[string]any
	deny       bool
	codes      map[string]grant
	keyFetches int
	tokenCalls int
}

type signingKey struct {
	id   string
	priv *rsa.PrivateKey
}

type grant struct {
	redirect  string
	challenge string
	nonce     string
	claims    map[string]any
}

// New starts an issuer and stops it when the test ends. Its first sign-in
// carries the claims {"sub": "alice"}.
func New(t testing.TB, opts Options) *Issuer {
	t.Helper()
	if opts.ClientID == "" {
		opts.ClientID = "opm-portal"
	}
	if opts.ClientSecret == "" {
		opts.ClientSecret = "portal-secret"
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	i := &Issuer{t: t, opts: opts, claims: map[string]any{"sub": "alice"}, codes: map[string]grant{}}
	i.keys = []signingKey{i.newKey()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", i.serveDiscovery)
	mux.HandleFunc("GET "+KeysPath, i.serveKeys)
	mux.HandleFunc("GET "+AuthorizePath, i.serveAuthorize)
	mux.HandleFunc("POST "+TokenPath, i.serveToken)
	i.srv = httptest.NewTLSServer(mux)
	t.Cleanup(i.srv.Close)
	i.URL = i.srv.URL
	return i
}

// Client returns an HTTP client that trusts the issuer's certificate.
func (i *Issuer) Client() *http.Client { return i.srv.Client() }

// ClientID returns the client the issuer serves.
func (i *Issuer) ClientID() string { return i.opts.ClientID }

// ClientSecret returns that client's secret.
func (i *Issuer) ClientSecret() string { return i.opts.ClientSecret }

// SetClaims sets the claims later sign-ins carry in their ID token, beside
// iss, aud, exp, iat and the request's nonce (a "nonce" here wins).
func (i *Issuer) SetClaims(claims map[string]any) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.claims = maps.Clone(claims)
}

// Deny makes later authorization requests answer error=access_denied.
func (i *Issuer) Deny(deny bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.deny = deny
}

// Rotate publishes a new key and signs with it from now on. The old keys
// stay published.
func (i *Issuer) Rotate() {
	k := i.newKey()
	i.mu.Lock()
	defer i.mu.Unlock()
	i.keys = append(i.keys, k)
}

// KeyFetches returns how many times the key set was fetched.
func (i *Issuer) KeyFetches() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.keyFetches
}

// TokenRequests returns how many token requests were made.
func (i *Issuer) TokenRequests() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.tokenCalls
}

// Claims returns standard claims for a token from this issuer to aud,
// valid for an hour, with sub as the subject.
func (i *Issuer) Claims(sub, aud string) map[string]any {
	now := i.opts.Now()
	return map[string]any{
		"iss": i.URL,
		"aud": aud,
		"sub": sub,
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	}
}

// Sign signs claims, as given, with the issuer's current key.
func (i *Issuer) Sign(claims map[string]any) string {
	i.mu.Lock()
	k := i.keys[len(i.keys)-1]
	i.mu.Unlock()
	return SignWith(i.t, k.priv, k.id, jose.RS256, claims)
}

// SignWith signs claims with key, naming keyID in the header.
func SignWith(t testing.TB, key any, keyID string, alg jose.SignatureAlgorithm, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("oidctest: encoding claims: %v", err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: alg, Key: jose.JSONWebKey{Key: key, KeyID: keyID}},
		(&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		t.Fatalf("oidctest: signer: %v", err)
	}
	jws, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("oidctest: signing: %v", err)
	}
	raw, err := jws.CompactSerialize()
	if err != nil {
		t.Fatalf("oidctest: serializing: %v", err)
	}
	return raw
}

func (i *Issuer) newKey() signingKey {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		i.t.Fatalf("oidctest: generating a key: %v", err)
	}
	return signingKey{id: rand.Text()[:8], priv: priv}
}

func (i *Issuer) serveDiscovery(w http.ResponseWriter, _ *http.Request) {
	doc := map[string]any{
		"issuer":                                i.URL,
		"authorization_endpoint":                i.URL + AuthorizePath,
		"token_endpoint":                        i.URL + TokenPath,
		"jwks_uri":                              i.URL + KeysPath,
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"code_challenge_methods_supported":      []string{"S256"},
	}
	if i.opts.EndSession {
		doc["end_session_endpoint"] = i.URL + EndSessionPath
	}
	i.writeJSON(w, http.StatusOK, doc)
}

func (i *Issuer) serveKeys(w http.ResponseWriter, _ *http.Request) {
	i.mu.Lock()
	i.keyFetches++
	set := jose.JSONWebKeySet{}
	for _, k := range i.keys {
		set.Keys = append(set.Keys, jose.JSONWebKey{Key: k.priv.Public(), KeyID: k.id, Algorithm: string(jose.RS256), Use: "sig"})
	}
	i.mu.Unlock()
	i.writeJSON(w, http.StatusOK, set)
}

// serveAuthorize approves the request at once: it checks the client, the
// redirect URI's presence and the S256 challenge, then redirects back with
// a single-use code.
func (i *Issuer) serveAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	redirect, err := url.Parse(q.Get("redirect_uri"))
	if err != nil || redirect.Scheme == "" {
		http.Error(w, "bad redirect_uri", http.StatusBadRequest)
		return
	}
	back := redirect.Query()
	back.Set("state", q.Get("state"))
	i.mu.Lock()
	deny := i.deny
	claims := maps.Clone(i.claims)
	i.mu.Unlock()
	switch {
	case deny:
		back.Set("error", "access_denied")
	case q.Get("client_id") != i.opts.ClientID || q.Get("response_type") != "code" ||
		q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "":
		back.Set("error", "invalid_request")
	default:
		code := rand.Text()
		i.mu.Lock()
		i.codes[code] = grant{redirect: redirect.String(), challenge: q.Get("code_challenge"), nonce: q.Get("nonce"), claims: claims}
		i.mu.Unlock()
		back.Set("code", code)
	}
	redirect.RawQuery = back.Encode()
	http.Redirect(w, r, redirect.String(), http.StatusFound)
}

// serveToken exchanges a code once, for the client that holds the secret
// and the verifier whose S256 digest is the code's challenge.
func (i *Issuer) serveToken(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		i.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	i.mu.Lock()
	i.tokenCalls++
	g, found := i.codes[r.PostForm.Get("code")]
	delete(i.codes, r.PostForm.Get("code"))
	i.mu.Unlock()
	if id != i.opts.ClientID || subtle.ConstantTimeCompare([]byte(secret), []byte(i.opts.ClientSecret)) != 1 {
		i.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_client"})
		return
	}
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	switch {
	case r.PostForm.Get("grant_type") != "authorization_code", !found,
		r.PostForm.Get("redirect_uri") != g.redirect,
		base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge:
		i.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
		return
	}
	claims := i.Claims("", i.opts.ClientID)
	delete(claims, "sub")
	claims["nonce"] = g.nonce
	maps.Copy(claims, g.claims)
	i.writeJSON(w, http.StatusOK, map[string]any{
		"access_token": rand.Text(),
		"token_type":   "Bearer",
		"expires_in":   3600,
		"id_token":     i.Sign(claims),
	})
}

func (i *Issuer) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		i.t.Logf("oidctest: writing a response: %v", err)
	}
}
