package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	jose "github.com/go-jose/go-jose/v4"
	"golang.org/x/oauth2"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// Paths the OIDC front door serves itself.
const (
	LoginPath    = "/auth/login"
	CallbackPath = "/auth/callback"
	LogoutPath   = "/auth/logout"
)

const (
	defaultOIDCSessionTTL     = 8 * time.Hour
	defaultMaxSessions        = 10000
	defaultKeyRefreshInterval = 30 * time.Second
	defaultIssuerTimeout      = 10 * time.Second
	// maxBearerBytes caps a bearer token; real ones are a few KiB.
	maxBearerBytes = 16 << 10
)

// ErrInvalidToken is returned by Authenticate for an Authorization header
// that is not a valid bearer token for this portal. Its text names no
// token.
var ErrInvalidToken = errors.New("the bearer token is not valid for this portal")

// signingAlgs are the algorithms a token may be signed with: asymmetric
// only, so a token signed with a shared secret, or none, never verifies.
var signingAlgs = []string{
	oidc.RS256, oidc.RS384, oidc.RS512,
	oidc.ES256, oidc.ES384, oidc.ES512,
	oidc.PS256, oidc.PS384, oidc.PS512,
	oidc.EdDSA,
}

// OIDCConfig wires an OIDC.
type OIDCConfig struct {
	// IssuerURL is the issuer identifier; https only. Discovery reads
	// IssuerURL/.well-known/openid-configuration.
	IssuerURL string
	// ClientID is the portal's client at the issuer, and the audience a
	// browser's ID token must name.
	ClientID string
	// ClientSecret authenticates the client at the token endpoint; empty
	// for a public client. It is never logged.
	ClientSecret string
	// RedirectURL is the absolute callback URL registered at the issuer.
	// Its path must be CallbackPath, and it must be https unless its host
	// is a loopback address.
	RedirectURL string
	// Scopes requested at sign-in. Default openid, email, profile; openid
	// is always requested.
	Scopes []string
	// Audience is the audience a bearer token must name. Default ClientID.
	Audience string

	// UsernameClaim names the claim the username comes from. Default "sub".
	UsernameClaim string
	// UsernamePrefix is prepended to every username.
	UsernamePrefix string
	// GroupsClaim names the claim groups come from; empty reads no groups.
	GroupsClaim string
	// GroupsPrefix is prepended to every group.
	GroupsPrefix string
	// APIServerTrustsIssuer declares that the API server trusts this
	// issuer with the same prefixes. Only then may a prefix be empty.
	APIServerTrustsIssuer bool

	// PostLogoutRedirectURL is where the issuer's end-session endpoint
	// sends the browser after sign-out, when the issuer has one.
	PostLogoutRedirectURL string
	// SessionTTL is a browser session's absolute lifetime. Default 8 hours.
	SessionTTL time.Duration
	// MaxSessions bounds the sessions held at once. Default 10000.
	MaxSessions int
	// KeyRefreshInterval is the least time between two fetches of the
	// issuer's signing keys. Default 30 seconds.
	KeyRefreshInterval time.Duration
	// HTTPClient reaches the issuer. Default: a client with a 10 second
	// timeout.
	HTTPClient *http.Client
	// Now is the clock. Default time.Now.
	Now func() time.Time
	// Logger receives operational logs, never a token, cookie, code or
	// secret. Default: discarded.
	Logger *slog.Logger
}

// OIDC is the in-cluster front door: it signs browsers in through the
// issuer and accepts the issuer's bearer tokens, mapping both to an
// identity that fails closed (portal:D6:R2/R3/R8).
type OIDC struct {
	cfg        OIDCConfig
	log        *slog.Logger
	mapper     claimMapper
	oauth      oauth2.Config
	idTokens   *oidc.IDTokenVerifier
	bearers    *oidc.IDTokenVerifier
	endSession string
	// bearerKey keys the HMAC that turns a bearer token into its session
	// key, so the key identifies the token only inside this process.
	bearerKey []byte
	logins    *sealer
	sessions  *sessionStore
}

// NewOIDC checks cfg, runs discovery against the issuer and fetches its
// signing keys, so a portal that cannot verify tokens never starts.
func NewOIDC(ctx context.Context, cfg OIDCConfig) (*OIDC, error) {
	cfg, mapper, err := checkOIDCConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("oidc auth: %w", err)
	}
	provider, err := oidc.NewProvider(oidc.ClientContext(ctx, cfg.HTTPClient), cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc auth: discovering the issuer: %w", err)
	}
	var meta struct {
		JWKSURL    string   `json:"jwks_uri"`
		Algs       []string `json:"id_token_signing_alg_values_supported"`
		EndSession string   `json:"end_session_endpoint"`
	}
	if err := provider.Claims(&meta); err != nil {
		return nil, fmt.Errorf("oidc auth: reading the issuer's discovery document: %w", err)
	}
	if meta.JWKSURL == "" {
		return nil, errors.New("oidc auth: the issuer's discovery document names no jwks_uri")
	}
	algs := slices.DeleteFunc(slices.Clone(meta.Algs), func(a string) bool { return !slices.Contains(signingAlgs, a) })
	if len(algs) == 0 {
		algs = []string{oidc.RS256}
	}
	keys := &keySet{
		url:      meta.JWKSURL,
		client:   cfg.HTTPClient,
		interval: cfg.KeyRefreshInterval,
		timeout:  defaultIssuerTimeout,
		now:      cfg.Now,
		log:      cfg.Logger,
	}
	for _, a := range algs {
		keys.algs = append(keys.algs, jose.SignatureAlgorithm(a))
	}
	if _, err := keys.refresh(ctx, 0); err != nil {
		return nil, fmt.Errorf("oidc auth: %w", err)
	}
	verifier := func(audience string) *oidc.IDTokenVerifier {
		return oidc.NewVerifier(cfg.IssuerURL, keys, &oidc.Config{ClientID: audience, SupportedSigningAlgs: algs, Now: cfg.Now})
	}
	bearerKey := make([]byte, 32)
	if _, err := rand.Read(bearerKey); err != nil {
		return nil, fmt.Errorf("oidc auth: %w", err)
	}
	logins, err := newSealer()
	if err != nil {
		return nil, fmt.Errorf("oidc auth: %w", err)
	}
	return &OIDC{
		cfg:    cfg,
		log:    cfg.Logger,
		mapper: mapper,
		oauth: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
		},
		idTokens:   verifier(cfg.ClientID),
		bearers:    verifier(cfg.Audience),
		endSession: meta.EndSession,
		bearerKey:  bearerKey,
		logins:     logins,
		sessions:   newSessionStore(cfg.MaxSessions, cfg.Now),
	}, nil
}

// checkOIDCConfig validates cfg and fills its defaults, before any network
// call (portal:D6:R6).
func checkOIDCConfig(cfg OIDCConfig) (OIDCConfig, claimMapper, error) {
	issuer, err := url.Parse(cfg.IssuerURL)
	if err != nil || issuer.Scheme != "https" || issuer.Host == "" {
		return cfg, claimMapper{}, errors.New("the issuer URL must be an absolute https URL")
	}
	if cfg.ClientID == "" {
		return cfg, claimMapper{}, errors.New("no client ID")
	}
	if err := checkRedirectURL(cfg.RedirectURL); err != nil {
		return cfg, claimMapper{}, err
	}
	if cfg.UsernameClaim == "" {
		cfg.UsernameClaim = "sub"
	}
	mapper := claimMapper{
		usernameClaim:  cfg.UsernameClaim,
		usernamePrefix: cfg.UsernamePrefix,
		groupsClaim:    cfg.GroupsClaim,
		groupsPrefix:   cfg.GroupsPrefix,
	}
	if err := checkPrefixes(mapper, cfg.APIServerTrustsIssuer); err != nil {
		return cfg, claimMapper{}, err
	}
	return withOIDCDefaults(cfg), mapper, nil
}

// withOIDCDefaults fills cfg's unset fields.
func withOIDCDefaults(cfg OIDCConfig) OIDCConfig {
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{oidc.ScopeOpenID, "email", "profile"}
	} else if !slices.Contains(cfg.Scopes, oidc.ScopeOpenID) {
		cfg.Scopes = append([]string{oidc.ScopeOpenID}, cfg.Scopes...)
	}
	if cfg.Audience == "" {
		cfg.Audience = cfg.ClientID
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = defaultOIDCSessionTTL
	}
	if cfg.MaxSessions <= 0 {
		cfg.MaxSessions = defaultMaxSessions
	}
	if cfg.KeyRefreshInterval <= 0 {
		cfg.KeyRefreshInterval = defaultKeyRefreshInterval
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: defaultIssuerTimeout}
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	return cfg
}

func checkRedirectURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return errors.New("the redirect URL must be absolute")
	}
	if u.Path != CallbackPath {
		return fmt.Errorf("the redirect URL's path must be %s", CallbackPath)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if ip := net.ParseIP(u.Hostname()); u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback()) {
			return nil
		}
	}
	return errors.New("the redirect URL must be https unless its host is a loopback address")
}

// Authenticate returns the identity and the session key of a request. A
// request with an Authorization header is authenticated from that header
// alone, and never falls back to its cookie; any other request needs a live
// browser session.
func (o *OIDC) Authenticate(r *http.Request) (authz.Identity, string, error) {
	if values := r.Header.Values("Authorization"); len(values) > 0 {
		return o.bearer(r.Context(), values)
	}
	return o.session(r)
}

// bearer verifies a bearer token's issuer, audience, expiry and signature,
// then maps its claims.
func (o *OIDC) bearer(ctx context.Context, values []string) (authz.Identity, string, error) {
	if len(values) != 1 {
		return o.refuseBearer("several Authorization headers")
	}
	scheme, token, ok := strings.Cut(values[0], " ")
	token = strings.TrimSpace(token)
	switch {
	case !ok || !strings.EqualFold(scheme, "Bearer") || token == "":
		return o.refuseBearer("not a bearer token")
	case len(token) > maxBearerBytes:
		return o.refuseBearer("the token is too large")
	}
	idToken, err := o.bearers.Verify(ctx, token)
	if err != nil {
		var expired *oidc.TokenExpiredError
		if errors.As(err, &expired) {
			return o.refuseBearer("the token has expired")
		}
		return o.refuseBearer("the token does not verify for this issuer and audience")
	}
	id, err := o.identityOf(idToken)
	if err != nil {
		return o.refuseBearer(err.Error())
	}
	mac := hmac.New(sha256.New, o.bearerKey)
	mac.Write([]byte(token))
	return id, "bearer:" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// refuseBearer logs why a bearer token was refused, never the token.
func (o *OIDC) refuseBearer(reason string) (authz.Identity, string, error) {
	o.log.Warn("refused a bearer token", "reason", reason)
	return authz.Identity{}, "", ErrInvalidToken
}

// identityOf maps a verified token's claims.
func (o *OIDC) identityOf(t *oidc.IDToken) (authz.Identity, error) {
	var claims map[string]any
	if err := t.Claims(&claims); err != nil {
		return authz.Identity{}, fmt.Errorf("%w: the claims do not decode", ErrUnmappedIdentity)
	}
	return o.mapper.identity(claims)
}
