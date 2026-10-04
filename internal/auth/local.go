package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// LaunchPath is where the launch token is exchanged for a session.
const LaunchPath = "/launch"

const defaultSessionTTL = 12 * time.Hour

// ErrNoSession is returned by Authenticate for a request that carries no
// live session. Its text names no cookie value.
var ErrNoSession = errors.New("no local session")

// LocalConfig wires a Local.
type LocalConfig struct {
	// Identity is who every admitted request reads as: the kubeconfig's
	// user as its SelfSubjectReview reported it. It must name a principal.
	Identity authz.Identity
	// Host is the loopback IP the portal listens on, for the launch URL.
	// Default 127.0.0.1.
	Host string
	// Port is the port the portal listens on. The Host allowlist, the
	// launch URL and the cookie name use it.
	Port int
	// Landing is where a successful launch redirects. Default "/".
	Landing string
	// SessionTTL is how long the session lasts from the launch. Default 12
	// hours.
	SessionTTL time.Duration
	// Now is the clock. Default time.Now.
	Now func() time.Time
	// Logger receives operational logs, never a token, cookie or session.
	// Default: discarded.
	Logger *slog.Logger
}

// Local is local mode's front door (0030:D5:R3/R4). It holds a one-time
// launch token, exchanges it for the one session, and admits only requests
// that carry that session and name a loopback host.
//
// Only SHA-256 digests of the token and the cookie value are kept, so
// neither can leak from memory into a log or an error.
type Local struct {
	cfg     LocalConfig
	log     *slog.Logger
	hosts   map[string]bool
	cookie  string
	landing string

	mu sync.Mutex
	// token is the digest of the launch token until it is spent.
	token *[sha256.Size]byte
	// session is the digest of the session cookie value once launched.
	session *[sha256.Size]byte
	// sessionKey identifies the session to the broker. It is random and is
	// not the cookie value, so a log line that names it reveals nothing.
	sessionKey string
	expires    time.Time

	launchToken string
}

// NewLocal returns a Local with a fresh launch token.
func NewLocal(cfg LocalConfig) (*Local, error) {
	switch {
	case !cfg.Identity.Authenticated():
		return nil, errors.New("local auth: the kubeconfig identity names no user")
	case cfg.Port <= 0 || cfg.Port > 65535:
		return nil, errors.New("local auth: no listening port")
	}
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Landing == "" {
		cfg.Landing = "/"
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = defaultSessionTTL
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	port := strconv.Itoa(cfg.Port)
	token := rand.Text()
	digest := sha256.Sum256([]byte(token))
	return &Local{
		cfg: cfg,
		log: cfg.Logger,
		hosts: map[string]bool{
			"127.0.0.1:" + port: true,
			"localhost:" + port: true,
			"[::1]:" + port:     true,
			strings.ToLower(net.JoinHostPort(cfg.Host, port)): true,
		},
		cookie:      "opm-portal-" + port,
		landing:     cfg.Landing,
		token:       &digest,
		launchToken: token,
	}, nil
}

// LaunchURL returns the one-time launch URL. It is the only place the
// token is ever given out: print it to the user, never log it.
func (l *Local) LaunchURL() string {
	u := url.URL{
		Scheme:   "http",
		Host:     net.JoinHostPort(l.cfg.Host, strconv.Itoa(l.cfg.Port)),
		Path:     LaunchPath,
		RawQuery: url.Values{"token": {l.launchToken}}.Encode(),
	}
	return u.String()
}

// CookieName returns the session cookie's name.
func (l *Local) CookieName() string { return l.cookie }

// Authenticate returns the identity and the session key for a request that
// carries the live session, and ErrNoSession for any other.
func (l *Local) Authenticate(r *http.Request) (authz.Identity, string, error) {
	key, ok := l.sessionOf(r)
	if !ok {
		return authz.Identity{}, "", ErrNoSession
	}
	return l.cfg.Identity, key, nil
}

// sessionOf reports the session key of the live session r carries.
func (l *Local) sessionOf(r *http.Request) (string, bool) {
	c, err := r.Cookie(l.cookie)
	if err != nil || c.Value == "" {
		return "", false
	}
	digest := sha256.Sum256([]byte(c.Value))
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.session == nil || subtle.ConstantTimeCompare(digest[:], l.session[:]) != 1 {
		return "", false
	}
	if !l.cfg.Now().Before(l.expires) {
		l.session, l.sessionKey = nil, ""
		return "", false
	}
	return l.sessionKey, true
}

// launch spends token and returns the new session's cookie, or false when
// the token is missing, wrong or already spent.
func (l *Local) launch(token string) (*http.Cookie, bool) {
	if token == "" {
		return nil, false
	}
	digest := sha256.Sum256([]byte(token))
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.token == nil || subtle.ConstantTimeCompare(digest[:], l.token[:]) != 1 {
		return nil, false
	}
	l.token, l.launchToken = nil, ""
	value := rand.Text()
	session := sha256.Sum256([]byte(value))
	l.session = &session
	l.sessionKey = rand.Text()
	l.expires = l.cfg.Now().Add(l.cfg.SessionTTL)
	// Local mode serves plain HTTP, so the cookie cannot be Secure, and a
	// __Host- name requires Secure. It keeps that prefix's other rules:
	// host-only (no Domain) and Path=/.
	return &http.Cookie{
		Name:     l.cookie,
		Value:    value,
		Path:     "/",
		MaxAge:   int(l.cfg.SessionTTL / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}, true
}
