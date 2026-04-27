// Package placement / capability.go — capability matching layer that sits
// on top of the existing Containers/pkg/scheduler primitives.
//
// Why a layer above the scheduler instead of editing scheduler.go?
//
//  1. The scheduler is in a vendored submodule (Containers). Avoiding
//     edits there keeps blast radius local and lets us iterate on the
//     capability model without submodule pointer churn.
//  2. The scheduler's existing strategies (resource_aware, affinity,
//     gpu_affinity) cover narrow slices but don't compose. We need a
//     score that weighs hard constraints + soft preferences + load in
//     one pass. A custom scorer here does that cleanly.
//  3. The scheduler hardcodes LocalHostName injection (BUGFIXES #52
//     follow-up commit `8f58c2e5` documented this). Our scorer never
//     considers local — eligibility starts from the registered remote
//     host set only.
//
// The output of this file (HostCapabilities + ScoreHost) feeds into
// planner.go which iterates groups and picks the highest-scoring host
// per group.

package placement

import (
	"fmt"
	"sort"
	"strings"
)

// HostCapabilities describes what a registered remote host can run,
// distilled from a live probe (prober.go) plus operator-set labels in
// Containers/.env. Boolean fields default to "unknown" (false) when
// the prober couldn't determine them — see eligibility rules below.
type HostCapabilities struct {
	Name string

	// Resources (from /proc/* via Containers/pkg/remote.Prober).
	CPUCores      int
	MemoryTotalMB uint64
	MemoryFreeMB  uint64
	DiskFreeMB    uint64

	// Container runtime.
	Runtime        string // "docker" | "podman" | ""
	RuntimeVersion string // semver-ish, e.g. "5.0.1"

	// Architecture.
	Arch string // "amd64" | "arm64" | ""

	// GPU.
	HasGPU    bool
	GPUVendor string // "nvidia" | "amd" | "intel" | ""
	GPUCount  int

	// Storage class — derived from /sys/block/*/queue/rotational.
	// "fast" if any non-rotational device present, else "slow".
	// Operator override via host label storage=fast|medium|slow wins.
	StorageClass string // "fast" | "medium" | "slow" | ""

	// Memory class — labels memory=high|medium|low (operator).
	// Auto-derived: ≥32 GiB → high, ≥8 GiB → medium, else low.
	MemoryClass string

	// Network class — operator-only via label network=high|medium|low.
	NetworkClass string

	// Operator labels (from Containers/.env CONTAINERS_REMOTE_HOST_N_LABELS).
	Labels map[string]string

	// Already-placed services on this host this boot. The planner
	// updates this incrementally as it places groups. Used by the
	// load-balance penalty.
	PlacementCount int
}

// CapabilityRequirement labels recognized on compose services. The
// parser maps `service.labels.helixagent.placement.X` into the
// ContainerRequirements.Labels map; the scorer here reads them back.
const (
	LabelRequireGPU     = "helixagent.placement.require.gpu"
	LabelRequireRuntime = "helixagent.placement.require.runtime"
	LabelRequireArch    = "helixagent.placement.require.arch"
	LabelPreferStorage  = "helixagent.placement.prefer.storage"
	LabelPreferMemory   = "helixagent.placement.prefer.memory"
	LabelPreferNetwork  = "helixagent.placement.prefer.network"
)

// ScoringWeights exposes the soft-preference weights. Made package-level
// so tests + docs can reference one source of truth. Tweak with care —
// the relative ordering matters more than the absolute values.
var ScoringWeights = struct {
	StorageMatch float64
	MemoryMatch  float64
	NetworkMatch float64
	LoadPenalty  float64
}{
	StorageMatch: 10,
	MemoryMatch:  8,
	NetworkMatch: 5,
	LoadPenalty:  3,
}

// ScoreResult is the output of ScoreHost: a numeric score plus a
// human-readable reason chain so operators can audit why a host was
// (or wasn't) chosen. Eligible=false means a hard constraint failed
// and this host MUST NOT be selected, regardless of score.
type ScoreResult struct {
	HostName string
	Eligible bool
	Score    float64
	Reasons  []string
}

// String renders a one-line summary for log lines.
func (r ScoreResult) String() string {
	if !r.Eligible {
		return fmt.Sprintf("%s: INELIGIBLE — %s",
			r.HostName, strings.Join(r.Reasons, "; "))
	}
	return fmt.Sprintf("%s: score=%.1f (%s)",
		r.HostName, r.Score, strings.Join(r.Reasons, "; "))
}

// ScoreHost evaluates one (service-group, host) pair. The scorer is
// pure — no I/O, no hidden state. Tests cover every constraint and
// preference combination.
//
// `groupReq` is the AGGREGATED requirement of a co-location group
// (memory/cpu summed across members; GPU/runtime/arch propagated as
// the strictest member's). Labels carry the placement.require.X /
// placement.prefer.X annotations.
func ScoreHost(groupReq Requirement, host *HostCapabilities) ScoreResult {
	res := ScoreResult{HostName: host.Name, Eligible: true}

	// HARD CONSTRAINT — GPU
	if want := groupReq.Labels[LabelRequireGPU]; want != "" && want != "false" {
		if !host.HasGPU {
			res.Eligible = false
			res.Reasons = append(res.Reasons,
				fmt.Sprintf("requires gpu=%s but host has no GPU", want))
			return res
		}
		// Vendor match if specified.
		if want != "true" && !strings.EqualFold(want, host.GPUVendor) {
			res.Eligible = false
			res.Reasons = append(res.Reasons,
				fmt.Sprintf("requires gpu vendor %q, host has %q",
					want, host.GPUVendor))
			return res
		}
		res.Reasons = append(res.Reasons,
			fmt.Sprintf("gpu match (%s)", host.GPUVendor))
	}

	// HARD CONSTRAINT — runtime
	if want := groupReq.Labels[LabelRequireRuntime]; want != "" && want != "any" {
		if !strings.EqualFold(host.Runtime, want) {
			res.Eligible = false
			res.Reasons = append(res.Reasons,
				fmt.Sprintf("requires runtime=%s, host has %s",
					want, host.Runtime))
			return res
		}
		res.Reasons = append(res.Reasons,
			fmt.Sprintf("runtime match (%s)", host.Runtime))
	}

	// HARD CONSTRAINT — arch
	if want := groupReq.Labels[LabelRequireArch]; want != "" && want != "any" {
		if !strings.EqualFold(host.Arch, want) {
			res.Eligible = false
			res.Reasons = append(res.Reasons,
				fmt.Sprintf("requires arch=%s, host has %s",
					want, host.Arch))
			return res
		}
	}

	// HARD CONSTRAINT — memory fit. The group's aggregated memory
	// requirement must fit in the host's free memory minus a 10%
	// safety margin (kernel + sidecars need headroom).
	if groupReq.MemoryMB > 0 && host.MemoryFreeMB > 0 {
		safe := uint64(float64(host.MemoryFreeMB) * 0.9)
		if groupReq.MemoryMB > safe {
			res.Eligible = false
			res.Reasons = append(res.Reasons,
				fmt.Sprintf("memory: requires %d MB, host has %d MB free (safe %d MB)",
					groupReq.MemoryMB, host.MemoryFreeMB, safe))
			return res
		}
	}

	// SOFT PREFERENCES — storage class
	if want := groupReq.Labels[LabelPreferStorage]; want != "" {
		if classMatches(want, host.StorageClass) {
			res.Score += ScoringWeights.StorageMatch
			res.Reasons = append(res.Reasons,
				fmt.Sprintf("storage match (+%.0f)", ScoringWeights.StorageMatch))
		} else {
			res.Reasons = append(res.Reasons,
				fmt.Sprintf("storage prefers %s, host is %s (no bonus)",
					want, host.StorageClass))
		}
	}

	// SOFT PREFERENCES — memory class
	if want := groupReq.Labels[LabelPreferMemory]; want != "" {
		if classMatches(want, host.MemoryClass) {
			res.Score += ScoringWeights.MemoryMatch
			res.Reasons = append(res.Reasons,
				fmt.Sprintf("memory match (+%.0f)", ScoringWeights.MemoryMatch))
		} else {
			res.Reasons = append(res.Reasons,
				fmt.Sprintf("memory prefers %s, host is %s (no bonus)",
					want, host.MemoryClass))
		}
	}

	// SOFT PREFERENCES — network class
	if want := groupReq.Labels[LabelPreferNetwork]; want != "" {
		if classMatches(want, host.NetworkClass) {
			res.Score += ScoringWeights.NetworkMatch
			res.Reasons = append(res.Reasons,
				fmt.Sprintf("network match (+%.0f)", ScoringWeights.NetworkMatch))
		}
	}

	// LOAD PENALTY: more existing placements on this host = less
	// attractive. Subtractive so a heavily-loaded perfect-prefs host
	// can still lose to an empty good-prefs host.
	loadPenalty := float64(host.PlacementCount) * ScoringWeights.LoadPenalty
	if loadPenalty > 0 {
		res.Score -= loadPenalty
		res.Reasons = append(res.Reasons,
			fmt.Sprintf("load penalty (%d placed = -%.0f)",
				host.PlacementCount, loadPenalty))
	}

	if len(res.Reasons) == 0 {
		res.Reasons = append(res.Reasons, "no preferences declared")
	}
	return res
}

// classMatches treats class hierarchies tolerantly: a service
// preferring "high" memory is satisfied by a host classed "high",
// and vice versa exact match is required for storage and network.
// Memory tolerates upgrade (asking medium, getting high is fine).
// Storage tolerates upgrade (asking medium, getting fast is fine).
// Network is exact match.
func classMatches(want, have string) bool {
	want = strings.ToLower(want)
	have = strings.ToLower(have)
	if want == "" || have == "" {
		return false
	}
	if want == have {
		return true
	}
	// Tolerant upgrades for storage and memory: higher class
	// satisfies a lower-class preference.
	rank := map[string]int{"low": 1, "medium": 2, "high": 3, "slow": 1, "fast": 3}
	return rank[have] >= rank[want] && rank[want] > 0
}

// PickBestHost runs ScoreHost across `hosts` and returns the highest-
// scoring eligible host. Hosts are evaluated in deterministic name
// order so ties break the same way across reboots. Returns
// ("", reasons) when no host is eligible.
func PickBestHost(req Requirement, hosts []*HostCapabilities) (string, []ScoreResult) {
	results := make([]ScoreResult, 0, len(hosts))
	// Sort by name for deterministic tie-breaking.
	sortedHosts := make([]*HostCapabilities, len(hosts))
	copy(sortedHosts, hosts)
	sort.Slice(sortedHosts, func(i, j int) bool {
		return sortedHosts[i].Name < sortedHosts[j].Name
	})

	for _, h := range sortedHosts {
		results = append(results, ScoreHost(req, h))
	}

	// Highest score wins among eligible; ties break by name (stable).
	var best *ScoreResult
	for i := range results {
		r := &results[i]
		if !r.Eligible {
			continue
		}
		if best == nil || r.Score > best.Score {
			best = r
		}
	}
	if best == nil {
		return "", results
	}
	return best.HostName, results
}

// Requirement is the lightweight subset of scheduler.ContainerRequirements
// that the capability scorer cares about. Defined here (instead of
// reusing scheduler.ContainerRequirements directly) so this package's
// scorer is self-contained and testable without the full scheduler
// dependency graph.
type Requirement struct {
	Name     string
	MemoryMB uint64
	CPUCores float64
	Labels   map[string]string
}
