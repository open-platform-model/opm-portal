package authz

import (
	"sync"
	"time"
)

const (
	defaultTTL        = 30 * time.Second
	defaultMaxEntries = 4096
)

// decisionCache remembers allow and deny decisions for a short time. It
// stores only what a backend decided; failures never reach it.
type decisionCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	max     int
	now     func() time.Time
	entries map[string]cachedDecision
}

type cachedDecision struct {
	allowed bool
	expires time.Time
}

func newDecisionCache(ttl time.Duration, maxEntries int) *decisionCache {
	if ttl <= 0 {
		ttl = defaultTTL
	}
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}
	return &decisionCache{ttl: ttl, max: maxEntries, now: time.Now, entries: map[string]cachedDecision{}}
}

// cacheKey joins two canonical, quoted encodings, so distinct identity and
// request pairs never share a key.
func cacheKey(who Identity, req Attributes) string {
	return who.key() + "|" + req.key()
}

func (c *decisionCache) get(key string) (allowed, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	d, ok := c.entries[key]
	if !ok {
		return false, false
	}
	if !c.now().Before(d.expires) {
		delete(c.entries, key)
		return false, false
	}
	return d.allowed, true
}

// put stores a decision. When the cache is full it first drops expired
// entries; if it is still full the decision is not stored, which costs a
// review later and never changes an answer.
func (c *decisionCache) put(key string, allowed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.max {
		for k, d := range c.entries {
			if !now.Before(d.expires) {
				delete(c.entries, k)
			}
		}
		if len(c.entries) >= c.max {
			return
		}
	}
	c.entries[key] = cachedDecision{allowed: allowed, expires: now.Add(c.ttl)}
}

func (c *decisionCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}
