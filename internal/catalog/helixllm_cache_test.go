package catalog

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/adapters/helixllm"
)

// These tests pin the two properties that make the cache safe to put in front
// of a live listing: it must make repeated catalog requests CHEAP, and it must
// never let "cheap" become "untrue".

func respWith(ids ...string) *helixllm.ModelsResponse {
	r := &helixllm.ModelsResponse{Object: "list"}
	for _, id := range ids {
		r.Data = append(r.Data, helixllm.ModelInfo{ID: id, Availability: "serving"})
	}
	return r
}

// fakeClock is a hand-driven clock so freshness is asserted deterministically
// rather than by sleeping (§11.4.50 — a timing-dependent test is a flaky test).
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// newTestCache builds a cache on a hand-driven clock.
func newTestCache(lister ModelLister, ttl time.Duration, clk *fakeClock) ModelLister {
	c := &listingCache{lister: lister, ttl: ttl, now: clk.Now}
	return c.list
}

func TestCachedLister_NilListerStaysNil(t *testing.T) {
	// A deployment that never wired HelixLLM must keep taking the honest-empty
	// path; wrapping nothing must not manufacture a source.
	if got := CachedLister(nil, time.Minute); got != nil {
		t.Fatal("CachedLister(nil, ...) returned a lister; a deployment with no " +
			"HelixLLM wired must contribute no options at all")
	}
}

func TestCachedLister_ZeroTTLDisablesCaching(t *testing.T) {
	calls := 0
	lister := func(context.Context) (*helixllm.ModelsResponse, error) {
		calls++
		return respWith("a"), nil
	}
	cached := CachedLister(lister, 0)
	for i := 0; i < 3; i++ {
		if _, err := cached(context.Background()); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if calls != 3 {
		t.Errorf("ttl<=0 must disable caching entirely; serving-layer calls = %d, want 3", calls)
	}
}

func TestCachedLister_ReusesListingWithinTTL(t *testing.T) {
	calls := 0
	lister := func(context.Context) (*helixllm.ModelsResponse, error) {
		calls++
		return respWith("a"), nil
	}
	clk := &fakeClock{t: time.Unix(1000, 0)}
	cached := newTestCache(lister, 30*time.Second, clk)

	for i := 0; i < 5; i++ {
		clk.Advance(time.Second)
		if _, err := cached(context.Background()); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if calls != 1 {
		t.Errorf("5 catalog requests inside the freshness window hit the serving "+
			"layer %d times, want 1: the point of the cache is that a handler "+
			"does not pay for the listing per request", calls)
	}
}

func TestCachedLister_RefetchesAfterTTL(t *testing.T) {
	calls := 0
	lister := func(context.Context) (*helixllm.ModelsResponse, error) {
		calls++
		return respWith("a"), nil
	}
	clk := &fakeClock{t: time.Unix(1000, 0)}
	cached := newTestCache(lister, 30*time.Second, clk)

	if _, err := cached(context.Background()); err != nil {
		t.Fatal(err)
	}
	clk.Advance(30 * time.Second) // exactly at the bound: no longer fresh
	if _, err := cached(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("serving-layer calls = %d, want 2: a listing older than the "+
			"freshness bound must be re-obtained, or a model that stopped being "+
			"served could be advertised indefinitely", calls)
	}
}

// TestCachedLister_FailureYieldsNothingNotStale is the anti-bluff core of the
// cache: once the serving layer stops answering, the previously-remembered
// listing stops being a claim anyone may rely on.
func TestCachedLister_FailureYieldsNothingNotStale(t *testing.T) {
	fail := false
	lister := func(context.Context) (*helixllm.ModelsResponse, error) {
		if fail {
			return nil, errors.New("connection refused")
		}
		return respWith("real-model"), nil
	}
	clk := &fakeClock{t: time.Unix(1000, 0)}
	cached := newTestCache(lister, 30*time.Second, clk)

	// Prime the cache with a successful listing.
	got, err := cached(context.Background())
	if err != nil || got == nil || len(got.Data) != 1 {
		t.Fatalf("priming call: resp=%+v err=%v", got, err)
	}

	// The serving layer goes away and the entry expires.
	fail = true
	clk.Advance(time.Minute)

	got, err = cached(context.Background())
	if err == nil {
		t.Fatal("a failed listing was reported as a success")
	}
	if got != nil {
		t.Fatalf("a failed listing returned models %+v: serving a remembered "+
			"list after the serving layer stopped answering asserts those models "+
			"are running on evidence that just failed to confirm it", got.Data)
	}

	// And the failure must have DISCARDED the remembered listing: a later call
	// inside what would have been the old freshness window must still fail.
	got, err = cached(context.Background())
	if err == nil || got != nil {
		t.Fatalf("the stale listing survived the failure: resp=%+v err=%v", got, err)
	}
}

// TestCachedLister_FailureThenRecovery proves the cache is not poisoned by a
// failure: once the serving layer answers again, its answer is served.
func TestCachedLister_FailureThenRecovery(t *testing.T) {
	fail := true
	lister := func(context.Context) (*helixllm.ModelsResponse, error) {
		if fail {
			return nil, errors.New("connection refused")
		}
		return respWith("back-online"), nil
	}
	clk := &fakeClock{t: time.Unix(1000, 0)}
	cached := newTestCache(lister, 30*time.Second, clk)

	if _, err := cached(context.Background()); err == nil {
		t.Fatal("expected the first listing to fail")
	}
	fail = false
	got, err := cached(context.Background())
	if err != nil {
		t.Fatalf("recovery listing failed: %v", err)
	}
	if got == nil || len(got.Data) != 1 || got.Data[0].ID != "back-online" {
		t.Fatalf("recovery listing = %+v, want the serving layer's fresh answer", got)
	}
}

// TestCachedLister_SourceContributesNothingOnFailure joins the cache to the
// catalog source it feeds: a failed listing must reach the catalog as ZERO
// options, never as a remembered set (FR-019, CONST-036).
func TestCachedLister_SourceContributesNothingOnFailure(t *testing.T) {
	fail := false
	lister := func(context.Context) (*helixllm.ModelsResponse, error) {
		if fail {
			return nil, errors.New("connection refused")
		}
		return respWith("served-model"), nil
	}
	clk := &fakeClock{t: time.Unix(1000, 0)}
	src := NewHelixLLMSource(context.Background(), newTestCache(lister, 30*time.Second, clk))

	if opts := src.HelixLLMOptions(); len(opts) != 1 {
		t.Fatalf("priming: options = %+v, want the one served model", opts)
	}

	fail = true
	clk.Advance(time.Minute)

	if opts := src.HelixLLMOptions(); len(opts) != 0 {
		t.Fatalf("after the listing failed the catalog was still offered %+v; a "+
			"listing that could not be obtained is not evidence that anything is "+
			"running", opts)
	}
}

// TestCachedLister_ConcurrentCallsCollapse proves concurrent catalog requests
// arriving on a cold cache do not each open their own connection.
func TestCachedLister_ConcurrentCallsCollapse(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	lister := func(context.Context) (*helixllm.ModelsResponse, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		time.Sleep(10 * time.Millisecond) // a real listing is not instant
		return respWith("a"), nil
	}
	clk := &fakeClock{t: time.Unix(1000, 0)}
	cached := newTestCache(lister, 30*time.Second, clk)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := cached(context.Background()); err != nil {
				t.Errorf("concurrent call: %v", err)
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Errorf("8 concurrent catalog requests produced %d serving-layer calls, "+
			"want 1: a cold cache must not stampede the serving layer", calls)
	}
}
