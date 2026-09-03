package catalog

import (
	"context"
	"sync"
	"time"

	"dev.helix.agent/internal/adapters/helixllm"
)

// This file bounds what a live listing costs a request handler, without ever
// letting that bound turn into a lie.
//
// The catalog's HelixLLM options come from a real HTTP call to the serving
// layer. Two things follow, and the cache below exists for the first while
// refusing to trade away the second:
//
//   - A JSON handler must not inherit the serving layer's latency. An
//     unreachable or slow HelixLLM would otherwise hang every /v1/catalog
//     request for as long as the transport allows.
//   - A cheap answer must still be a TRUE answer. Serving a remembered list
//     after the serving layer stopped answering would state that models are
//     running when nothing confirmed it — the exact false-availability defect
//     the option pipeline exists to prevent (FR-019, CONST-036).
//
// The resolution is a cache with a HARD freshness bound and no stale fallback:
// within the bound the remembered listing is served; past it a fresh listing is
// required, and a failure to obtain one yields NOTHING rather than the last
// thing that worked.

// CachedLister wraps a ModelLister so repeated calls within ttl reuse one
// listing instead of re-querying the serving layer per request.
//
// The cache is deliberately unforgiving in three ways:
//
//   - It caches ONLY successes. A failed listing is not evidence about what is
//     running, so it is never remembered and never returned.
//   - A failure DISCARDS whatever was remembered. Once the serving layer stops
//     answering, the previous listing stops being a claim anyone may rely on;
//     the caller receives the error and, per NewHelixLLMSource, contributes no
//     options at all.
//   - Entries EXPIRE. ttl is the maximum age any answer may have, so a model
//     that stopped being served cannot be advertised indefinitely.
//
// ttl <= 0 disables caching entirely: every call goes to the serving layer.
// A nil lister is returned unchanged so a deployment that never wired HelixLLM
// keeps taking the honest-empty path.
//
// Concurrency: one in-flight refresh at a time. Concurrent callers arriving on
// an expired entry queue behind that single refresh rather than each opening
// its own connection; the caller-supplied context still bounds how long any of
// them can be held up.
func CachedLister(lister ModelLister, ttl time.Duration) ModelLister {
	if lister == nil {
		return nil
	}
	if ttl <= 0 {
		return lister
	}
	c := &listingCache{lister: lister, ttl: ttl, now: time.Now}
	return c.list
}

type listingCache struct {
	mu     sync.Mutex
	lister ModelLister
	ttl    time.Duration
	// now is injectable so the freshness bound is testable without sleeping
	// (§11.4.50 — a timing-dependent test is a flaky test).
	now func() time.Time

	cached   *helixllm.ModelsResponse
	cachedAt time.Time
}

func (c *listingCache) list(ctx context.Context) (*helixllm.ModelsResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cached != nil && c.now().Sub(c.cachedAt) < c.ttl {
		return c.cached, nil
	}

	resp, err := c.lister(ctx)
	if err != nil {
		// Drop the remembered listing. Continuing to serve it would assert
		// that those models are running on the strength of a call that just
		// failed to confirm anything.
		c.cached = nil
		c.cachedAt = time.Time{}
		return nil, err
	}
	c.cached = resp
	c.cachedAt = c.now()
	return resp, nil
}
