package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"maps"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// Cookie names. The __Host- prefix makes a browser accept them only with
// Secure, Path=/ and no Domain, so no other host can set or read them.
const (
	sessionCookie = "__Host-opm-portal-session"
	loginCookie   = "__Host-opm-portal-login"
	// loginTTL is how long a sign-in may take at the issuer.
	loginTTL = 10 * time.Minute
)

// session authenticates a request by its browser session.
func (o *OIDC) session(r *http.Request) (authz.Identity, string, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return authz.Identity{}, "", ErrNoSession
	}
	s, ok := o.sessions.lookup(c.Value)
	if !ok {
		return authz.Identity{}, "", ErrNoSession
	}
	id := s.identity
	id.Groups = slices.Clone(id.Groups)
	return id, s.key, nil
}

// sessionCookieFor is the cookie carrying a new session's value.
func sessionCookieFor(value string, ttl time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   int(ttl / time.Second),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

// expiredCookie clears the cookie name.
func expiredCookie(name string) *http.Cookie {
	return &http.Cookie{Name: name, Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}
}

// oidcSession is one signed-in browser.
type oidcSession struct {
	identity authz.Identity
	// key identifies the session to the broker. It is random and is not the
	// cookie value, so a log line that names it reveals nothing.
	key     string
	expires time.Time
}

// maxSessionsPerUser bounds the sessions one mapped username holds. At the
// bound a new sign-in ends that user's own session closest to expiry, so
// one account signing in over and over cannot push other users out.
const maxSessionsPerUser = 10

type sessionDigest = [sha256.Size]byte

// sessionStore holds sessions in memory, keyed by the SHA-256 of their
// cookie value, so the value itself is never kept. Logout deletes the
// entry, which revokes the cookie at once.
type sessionStore struct {
	max     int
	perUser int
	now     func() time.Time

	mu       sync.Mutex
	sessions map[sessionDigest]*oidcSession
	byUser   map[string]map[sessionDigest]struct{}
}

func newSessionStore(maxSessions int, now func() time.Time) *sessionStore {
	return &sessionStore{
		max:      maxSessions,
		perUser:  maxSessionsPerUser,
		now:      now,
		sessions: map[sessionDigest]*oidcSession{},
		byUser:   map[string]map[sessionDigest]struct{}{},
	}
}

// create starts a session for id and returns its cookie value. At the
// per-user bound, that user's expired sessions are dropped first, then
// their one closest to expiry. At the global cap, expired sessions are
// dropped first, then the one closest to expiry, whoever holds it.
func (s *sessionStore) create(id authz.Identity, ttl time.Duration) string {
	value := rand.Text()
	digest := sha256.Sum256([]byte(value))
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if own := s.byUser[id.Username]; len(own) >= s.perUser {
		s.evict(maps.Keys(own), now)
	}
	if len(s.sessions) >= s.max {
		s.evict(maps.Keys(s.sessions), now)
	}
	s.sessions[digest] = &oidcSession{identity: id, key: rand.Text(), expires: now.Add(ttl)}
	own := s.byUser[id.Username]
	if own == nil {
		own = map[sessionDigest]struct{}{}
		s.byUser[id.Username] = own
	}
	own[digest] = struct{}{}
	return value
}

// evict drops the expired sessions among digests, and when none had
// expired, the one closest to expiry. The caller holds s.mu.
func (s *sessionStore) evict(digests iter.Seq[sessionDigest], now time.Time) {
	var oldest sessionDigest
	var oldestAt time.Time
	found, dropped := false, false
	for d := range digests {
		v := s.sessions[d]
		if !now.Before(v.expires) {
			s.remove(d)
			dropped = true
			continue
		}
		if !found || v.expires.Before(oldestAt) {
			oldest, oldestAt, found = d, v.expires, true
		}
	}
	if !dropped && found {
		s.remove(oldest)
	}
}

// remove drops one session and its entry in the per-user index. The caller
// holds s.mu.
func (s *sessionStore) remove(digest sessionDigest) {
	sess, ok := s.sessions[digest]
	if !ok {
		return
	}
	delete(s.sessions, digest)
	own := s.byUser[sess.identity.Username]
	delete(own, digest)
	if len(own) == 0 {
		delete(s.byUser, sess.identity.Username)
	}
}

// lookup returns the live session for a cookie value.
func (s *sessionStore) lookup(value string) (oidcSession, bool) {
	digest := sha256.Sum256([]byte(value))
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[digest]
	if !ok {
		return oidcSession{}, false
	}
	if !s.now().Before(sess.expires) {
		s.remove(digest)
		return oidcSession{}, false
	}
	return *sess, true
}

// delete ends the session for a cookie value, if there is one.
func (s *sessionStore) delete(value string) {
	digest := sha256.Sum256([]byte(value))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.remove(digest)
}

// loginState is what a sign-in carries between /auth/login and
// /auth/callback, sealed in the login cookie.
type loginState struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Return   string `json:"r"`
	Expires  int64  `json:"e"`
}

// sealer encrypts and authenticates the login cookie under a key that
// lives only in this process. Holding the sign-in in the browser rather
// than in memory leaves an unauthenticated caller nothing to fill, and
// binds the callback to the browser that started the sign-in.
type sealer struct {
	aead cipher.AEAD
}

func newSealer() (*sealer, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("creating the login cookie key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating the login cookie cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating the login cookie cipher: %w", err)
	}
	return &sealer{aead: aead}, nil
}

func (s *sealer) seal(v loginState) (string, error) {
	plain, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encoding the login state: %w", err)
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("sealing the login state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(s.aead.Seal(nonce, nonce, plain, []byte(loginCookie))), nil
}

var errBadLoginCookie = errors.New("the login cookie does not open")

func (s *sealer) open(raw string) (loginState, error) {
	sealed, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(sealed) < s.aead.NonceSize() {
		return loginState{}, errBadLoginCookie
	}
	n := s.aead.NonceSize()
	plain, err := s.aead.Open(nil, sealed[:n], sealed[n:], []byte(loginCookie))
	if err != nil {
		return loginState{}, errBadLoginCookie
	}
	var v loginState
	if err := json.Unmarshal(plain, &v); err != nil {
		return loginState{}, errBadLoginCookie
	}
	return v, nil
}
