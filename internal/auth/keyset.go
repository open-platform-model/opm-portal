package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

const (
	// maxKeySetBytes caps the issuer's key set response.
	maxKeySetBytes = 1 << 20
	// keySetMaxAge is how old the cached keys may grow before a
	// verification refreshes them first, so a key the issuer withdrew stops
	// verifying.
	keySetMaxAge = time.Hour
)

// errKeyRefreshTooSoon is returned when a token names a key the cache does
// not hold and the last fetch was too recent to fetch again.
var errKeyRefreshTooSoon = errors.New("the token's signing key is unknown and the key set was fetched recently")

// keySet is the issuer's signing keys, cached. go-oidc's RemoteKeySet
// fetches the key set again for every token naming an unknown key, with no
// lower bound, so any caller could make the portal hammer the issuer; this
// one fetches at most once per interval.
type keySet struct {
	url      string
	client   *http.Client
	algs     []jose.SignatureAlgorithm
	interval time.Duration
	now      func() time.Time
	log      *slog.Logger

	mu      sync.RWMutex
	keys    []jose.JSONWebKey
	gen     int // successful fetches so far
	fetched time.Time

	// fetchMu serializes fetches; attempted is when the last one started.
	fetchMu   sync.Mutex
	attempted time.Time
}

// VerifySignature implements oidc.KeySet: it returns the payload of raw
// when one of the issuer's keys verifies its single signature.
func (k *keySet) VerifySignature(ctx context.Context, raw string) ([]byte, error) {
	jws, err := jose.ParseSigned(raw, k.algs)
	if err != nil {
		return nil, fmt.Errorf("parsing the token: %w", err)
	}
	if len(jws.Signatures) != 1 {
		return nil, errors.New("the token does not carry exactly one signature")
	}
	keys, gen, fetched := k.snapshot()
	if !k.now().Before(fetched.Add(keySetMaxAge)) {
		if fresh, err := k.refresh(ctx, gen); err == nil {
			keys, gen = fresh, gen+1
		} else if !errors.Is(err, errKeyRefreshTooSoon) {
			k.log.Warn("refreshing the issuer's signing keys failed; using the cached keys", "error", err)
		}
	}
	if payload, ok := verifyWith(jws, keys); ok {
		return payload, nil
	}
	fresh, err := k.refresh(ctx, gen)
	if err != nil {
		return nil, err
	}
	if payload, ok := verifyWith(jws, fresh); ok {
		return payload, nil
	}
	return nil, errors.New("no signing key of the issuer verifies the token")
}

func verifyWith(jws *jose.JSONWebSignature, keys []jose.JSONWebKey) ([]byte, bool) {
	header := jws.Signatures[0].Header
	for i := range keys {
		key := &keys[i]
		if header.KeyID != "" && key.KeyID != header.KeyID {
			continue
		}
		if key.Algorithm != "" && key.Algorithm != header.Algorithm {
			continue
		}
		if payload, err := jws.Verify(key); err == nil {
			return payload, true
		}
	}
	return nil, false
}

func (k *keySet) snapshot() ([]jose.JSONWebKey, int, time.Time) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.keys, k.gen, k.fetched
}

// refresh fetches the key set, unless another caller already replaced the
// generation seen, or the last attempt is within the interval.
func (k *keySet) refresh(ctx context.Context, seen int) ([]jose.JSONWebKey, error) {
	k.fetchMu.Lock()
	defer k.fetchMu.Unlock()
	if keys, gen, _ := k.snapshot(); gen != seen {
		return keys, nil
	}
	now := k.now()
	if !k.attempted.IsZero() && now.Before(k.attempted.Add(k.interval)) {
		return nil, errKeyRefreshTooSoon
	}
	k.attempted = now
	keys, err := k.fetch(ctx)
	if err != nil {
		return nil, err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.keys, k.fetched = keys, now
	k.gen++
	return keys, nil
}

// fetch reads the key set, keeping only public keys meant for signatures.
func (k *keySet) fetch(ctx context.Context) ([]jose.JSONWebKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, k.url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("fetching the issuer's signing keys: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := k.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching the issuer's signing keys: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching the issuer's signing keys: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxKeySetBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading the issuer's signing keys: %w", err)
	}
	if len(body) > maxKeySetBytes {
		return nil, errors.New("the issuer's key set is larger than 1 MiB")
	}
	var set struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, fmt.Errorf("decoding the issuer's signing keys: %w", err)
	}
	var keys []jose.JSONWebKey
	for _, raw := range set.Keys {
		var key jose.JSONWebKey
		if err := key.UnmarshalJSON(raw); err != nil {
			continue // a key type this portal cannot use
		}
		if !key.Valid() || !key.IsPublic() || (key.Use != "" && key.Use != "sig") {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, errors.New("the issuer's key set holds no public signing key")
	}
	return keys, nil
}
