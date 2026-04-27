package placement

import "sync"

// globalCapabilityProber is the package-level prober installed by
// internal/adapters/containers.RemoteComposeUp at boot. The planner
// reads it under proberMu when building HostCapabilities. We use a
// package singleton (rather than threading the prober through every
// PlanCompose call) because PlanCompose has many call sites and the
// adapter is the natural lifecycle owner of the executor anyway.
var (
	globalCapabilityProber *CapabilityProber
	proberMu               sync.RWMutex
)

// SetCapabilityProber installs (or replaces) the package-level prober.
// Calling with nil disables capability probing — buildHostCapabilities
// falls back to label-only HostCapabilities for every host.
func SetCapabilityProber(p *CapabilityProber) {
	proberMu.Lock()
	globalCapabilityProber = p
	proberMu.Unlock()
}
