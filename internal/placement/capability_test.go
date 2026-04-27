package placement

import (
	"strings"
	"testing"
)

// TestScoreHost_GPUHardConstraint asserts a service requiring a GPU
// is INELIGIBLE on a CPU-only host and ELIGIBLE on a GPU host.
func TestScoreHost_GPUHardConstraint(t *testing.T) {
	gpuHost := &HostCapabilities{Name: "gpu-host", HasGPU: true, GPUVendor: "nvidia",
		MemoryFreeMB: 32 * 1024}
	cpuHost := &HostCapabilities{Name: "cpu-host", HasGPU: false,
		MemoryFreeMB: 32 * 1024}

	req := Requirement{Name: "sglang", Labels: map[string]string{
		LabelRequireGPU: "true",
	}}

	if r := ScoreHost(req, gpuHost); !r.Eligible {
		t.Errorf("gpu-host should be eligible: %s", r.String())
	}
	if r := ScoreHost(req, cpuHost); r.Eligible {
		t.Errorf("cpu-host must NOT be eligible for GPU service: %s", r.String())
	}
}

// TestScoreHost_GPUVendorMismatch asserts a service requiring an
// nvidia GPU is INELIGIBLE on an AMD-only host.
func TestScoreHost_GPUVendorMismatch(t *testing.T) {
	amdHost := &HostCapabilities{Name: "amd", HasGPU: true, GPUVendor: "amd",
		MemoryFreeMB: 32 * 1024}
	req := Requirement{Name: "cuda-thing", Labels: map[string]string{
		LabelRequireGPU: "nvidia",
	}}
	r := ScoreHost(req, amdHost)
	if r.Eligible {
		t.Errorf("AMD host must NOT be eligible for nvidia-specific service: %s", r.String())
	}
}

// TestScoreHost_RuntimeConstraint asserts a service requiring docker
// is INELIGIBLE on a podman host.
func TestScoreHost_RuntimeConstraint(t *testing.T) {
	dockerHost := &HostCapabilities{Name: "d", Runtime: "docker", MemoryFreeMB: 32 * 1024}
	podmanHost := &HostCapabilities{Name: "p", Runtime: "podman", MemoryFreeMB: 32 * 1024}
	req := Requirement{Name: "service", Labels: map[string]string{
		LabelRequireRuntime: "docker",
	}}
	if r := ScoreHost(req, dockerHost); !r.Eligible {
		t.Errorf("docker host eligible for docker service: %s", r.String())
	}
	if r := ScoreHost(req, podmanHost); r.Eligible {
		t.Errorf("podman host must NOT be eligible: %s", r.String())
	}
}

// TestScoreHost_MemoryFitConstraint asserts a service requiring more
// memory than the host has free is INELIGIBLE.
func TestScoreHost_MemoryFitConstraint(t *testing.T) {
	tinyHost := &HostCapabilities{Name: "tiny", MemoryFreeMB: 1024}
	req := Requirement{Name: "big", MemoryMB: 8 * 1024}
	r := ScoreHost(req, tinyHost)
	if r.Eligible {
		t.Errorf("tiny host must NOT fit 8GB service: %s", r.String())
	}
}

// TestScoreHost_StoragePreferenceBonus asserts a service preferring
// fast storage scores higher on a fast-storage host than on a
// slow-storage host (both still eligible).
func TestScoreHost_StoragePreferenceBonus(t *testing.T) {
	fastHost := &HostCapabilities{Name: "fast", StorageClass: "fast", MemoryFreeMB: 32 * 1024}
	slowHost := &HostCapabilities{Name: "slow", StorageClass: "slow", MemoryFreeMB: 32 * 1024}
	req := Requirement{Name: "db", Labels: map[string]string{
		LabelPreferStorage: "fast",
	}}
	rFast := ScoreHost(req, fastHost)
	rSlow := ScoreHost(req, slowHost)
	if rFast.Score <= rSlow.Score {
		t.Errorf("fast-storage host (score %v) must outrank slow-storage host (score %v) "+
			"when service prefers fast", rFast.Score, rSlow.Score)
	}
	if !rFast.Eligible || !rSlow.Eligible {
		t.Errorf("both hosts should be eligible (preference is soft, not hard)")
	}
}

// TestScoreHost_MemoryClassUpgradeTolerance asserts a service
// preferring "medium" memory IS satisfied by a "high"-memory host
// (the tolerant upgrade rule).
func TestScoreHost_MemoryClassUpgradeTolerance(t *testing.T) {
	highHost := &HostCapabilities{Name: "h", MemoryClass: "high", MemoryFreeMB: 32 * 1024}
	req := Requirement{Name: "svc", Labels: map[string]string{
		LabelPreferMemory: "medium",
	}}
	r := ScoreHost(req, highHost)
	if r.Score < ScoringWeights.MemoryMatch {
		t.Errorf("medium-pref must be satisfied by high host (score=%v)", r.Score)
	}
}

// TestScoreHost_LoadPenalty asserts an empty host scores higher than
// the same host with placements already on it.
func TestScoreHost_LoadPenalty(t *testing.T) {
	emptyHost := &HostCapabilities{Name: "e", MemoryFreeMB: 32 * 1024,
		StorageClass: "fast", MemoryClass: "high",
		PlacementCount: 0}
	loadedHost := &HostCapabilities{Name: "l", MemoryFreeMB: 32 * 1024,
		StorageClass: "fast", MemoryClass: "high",
		PlacementCount: 5}
	req := Requirement{Name: "svc", Labels: map[string]string{
		LabelPreferStorage: "fast",
	}}
	rEmpty := ScoreHost(req, emptyHost)
	rLoaded := ScoreHost(req, loadedHost)
	if rEmpty.Score <= rLoaded.Score {
		t.Errorf("empty host (score %v) must outrank loaded host (score %v)",
			rEmpty.Score, rLoaded.Score)
	}
}

// TestPickBestHost_GPUServiceLandsOnGPU asserts that with one GPU
// host and one CPU host, a GPU-required service goes to the GPU host
// regardless of load advantage on the CPU host.
func TestPickBestHost_GPUServiceLandsOnGPU(t *testing.T) {
	gpu := &HostCapabilities{Name: "gpu", HasGPU: true, GPUVendor: "nvidia",
		MemoryFreeMB: 32 * 1024, PlacementCount: 10}
	cpu := &HostCapabilities{Name: "cpu", HasGPU: false,
		MemoryFreeMB: 32 * 1024, PlacementCount: 0}
	req := Requirement{Name: "sglang", Labels: map[string]string{
		LabelRequireGPU: "true",
	}}
	picked, _ := PickBestHost(req, []*HostCapabilities{gpu, cpu})
	if picked != "gpu" {
		t.Errorf("expected gpu host, got %q", picked)
	}
}

// TestPickBestHost_NoEligibleReturnsEmpty asserts the function
// returns "" (with reasons) when no host is eligible.
func TestPickBestHost_NoEligibleReturnsEmpty(t *testing.T) {
	cpu := &HostCapabilities{Name: "cpu", MemoryFreeMB: 32 * 1024}
	req := Requirement{Name: "sglang", Labels: map[string]string{
		LabelRequireGPU: "true",
	}}
	picked, results := PickBestHost(req, []*HostCapabilities{cpu})
	if picked != "" {
		t.Errorf("expected empty pick, got %q", picked)
	}
	if len(results) == 0 {
		t.Errorf("expected at least one result with reasons")
	}
	if results[0].Eligible {
		t.Errorf("expected ineligible result")
	}
	if !strings.Contains(strings.Join(results[0].Reasons, " "), "GPU") &&
		!strings.Contains(strings.Join(results[0].Reasons, " "), "gpu") {
		t.Errorf("expected reason to mention GPU, got %v", results[0].Reasons)
	}
}

// TestPickBestHost_DeterministicTieBreak asserts that with two
// equally-scoring hosts, the alphabetically-first wins (stable plans
// across reboots).
func TestPickBestHost_DeterministicTieBreak(t *testing.T) {
	a := &HostCapabilities{Name: "amber", MemoryFreeMB: 32 * 1024,
		StorageClass: "fast", MemoryClass: "high"}
	t2 := &HostCapabilities{Name: "thinker", MemoryFreeMB: 32 * 1024,
		StorageClass: "fast", MemoryClass: "high"}
	req := Requirement{Name: "svc", Labels: map[string]string{
		LabelPreferStorage: "fast",
	}}
	for i := 0; i < 5; i++ {
		picked, _ := PickBestHost(req, []*HostCapabilities{t2, a}) // try both orders
		if picked != "amber" {
			t.Errorf("iteration %d: expected amber (alphabetically first), got %q", i, picked)
		}
	}
}

// TestClassMatches covers the upgrade-tolerance rules.
func TestClassMatches(t *testing.T) {
	cases := []struct {
		want, have string
		match      bool
	}{
		{"medium", "high", true},  // upgrade tolerated
		{"medium", "medium", true}, // exact match
		{"high", "medium", false},   // downgrade rejected
		{"fast", "fast", true},
		{"fast", "slow", false},
		{"slow", "fast", true},      // upgrade tolerated
		{"", "high", false},          // empty want fails
		{"high", "", false},          // empty have fails
	}
	for _, c := range cases {
		got := classMatches(c.want, c.have)
		if got != c.match {
			t.Errorf("classMatches(%q,%q) = %v, want %v", c.want, c.have, got, c.match)
		}
	}
}
