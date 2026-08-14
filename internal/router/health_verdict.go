package router

// healthVerdict derives the service-level health verdict of the /v1/health
// endpoint from the per-provider health map returned by
// ProviderRegistry.HealthCheck() (a nil map value means that provider is
// healthy; a non-nil error means it is not).
//
// This function exists so the verdict is testable in isolation. The verdict
// previously lived inline in the /v1/health closure in router.go, where it
// could only be exercised by booting the entire router — with the result that
// every existing /health test registers its OWN inline stub handler and
// asserts a literal it wrote two lines earlier, so no test ever executed the
// shipping handler at all.
//
// The verdict is drawn from a three-state vocabulary:
//
//	"healthy"    every registered provider answered its health check
//	"degraded"   at least one provider is serving, at least one is not —
//	             the agent can still serve completions
//	"unhealthy"  NO provider is serving (including the case where none is
//	             registered at all) — the agent cannot serve a single
//	             completion, which is its entire reason to exist
//
// HXC-244: this function previously computed the counts and then DISCARDED
// them, returning the constant "healthy". An agent with every provider down
// reported itself healthy — a check that could not fail for the most common
// way the service breaks. The regression guard is TestHealthVerdict, whose
// RED_MODE=1 polarity reproduces that historical defect.
func healthVerdict(health map[string]error) (status string, healthy, unhealthy int) {
	for _, err := range health {
		if err == nil {
			healthy++
		}
	}
	unhealthy = len(health) - healthy

	switch {
	case healthy == 0:
		// Covers both "every provider is down" and "no providers are
		// registered": in either case nothing can be served.
		return "unhealthy", healthy, unhealthy
	case unhealthy > 0:
		return "degraded", healthy, unhealthy
	default:
		return "healthy", healthy, unhealthy
	}
}
