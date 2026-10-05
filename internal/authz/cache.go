package authz

import (
	"sync"
	"time"
)

// DefaultTTL is how long a decision is reused when Options.TTL is zero.
// A package that bounds its work by one decision lifetime derives its
// default from it, so the two cannot drift apart.
const DefaultTTL = 30 * time.Second

const defaultMaxEntries = 4096

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
		ttl = DefaultTTL
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

// clock returns the cache's time source. A nil cache uses the wall clock.
func (c *decisionCache) clock() func() time.Time {
	if c == nil {
		return time.Now
	}
	return c.now
}

// get returns the live decision for key. A nil cache holds nothing.
func (c *decisionCache) get(key string) (cachedDecision, bool) {
	if c == nil {
		return cachedDecision{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	d, ok := c.entries[key]
	if !ok {
		return cachedDecision{}, false
	}
	if !c.now().Before(d.expires) {
		delete(c.entries, key)
		return cachedDecision{}, false
	}
	return d, true
}

// put stores a decision and returns it with its expiry, which holds whether
// or not it was stored. When the cache is full it first drops expired
// entries; if it is still full the decision is not stored, which costs a
// review later and never changes an answer. A nil cache stores nothing and
// gives the decision the default TTL.
func (c *decisionCache) put(key string, allowed bool) cachedDecision {
	if c == nil {
		return cachedDecision{allowed: allowed, expires: time.Now().Add(DefaultTTL)}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	d := cachedDecision{allowed: allowed, expires: now.Add(c.ttl)}
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.max {
		for k, e := range c.entries {
			if !now.Before(e.expires) {
				delete(c.entries, k)
			}
		}
		if len(c.entries) >= c.max {
			return d
		}
	}
	c.entries[key] = d
	return d
}

func (c *decisionCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}
