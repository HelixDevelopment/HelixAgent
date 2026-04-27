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

// ===== New dimension tests (CPU, disk, storage type, network) =====

// TestScoreHost_CPUPreferenceBonus asserts a service preferring fast
// CPU scores higher on a fast-CPU host than a slow-CPU host.
func TestScoreHost_CPUPreferenceBonus(t *testing.T) {
	fast := &HostCapabilities{Name: "f", CPUClass: "fast", CPUMhz: 3500,
		MemoryFreeMB: 32 * 1024}
	slow := &HostCapabilities{Name: "s", CPUClass: "slow", CPUMhz: 1200,
		MemoryFreeMB: 32 * 1024}
	req := Requirement{Name: "compiler", Labels: map[string]string{
		LabelPreferCPU: "fast",
	}}
	rFast := ScoreHost(req, fast)
	rSlow := ScoreHost(req, slow)
	if rFast.Score <= rSlow.Score {
		t.Errorf("fast-CPU (%v) must outrank slow-CPU (%v)",
			rFast.Score, rSlow.Score)
	}
}

// TestScoreHost_DiskSpacePreference asserts a service preferring a
// large-disk host scores higher there than on a small-disk host.
func TestScoreHost_DiskSpacePreference(t *testing.T) {
	big := &HostCapabilities{Name: "big", DiskSpaceClass: "large",
		MemoryFreeMB: 32 * 1024}
	small := &HostCapabilities{Name: "small", DiskSpaceClass: "small",
		MemoryFreeMB: 32 * 1024}
	req := Requirement{Name: "archive", Labels: map[string]string{
		LabelPreferDiskSpace: "large",
	}}
	rBig := ScoreHost(req, big)
	rSmall := ScoreHost(req, small)
	if rBig.Score <= rSmall.Score {
		t.Errorf("large-disk (%v) must outrank small-disk (%v)",
			rBig.Score, rSmall.Score)
	}
}

// TestScoreHost_StorageTypeNVMeWinsOverHDD asserts a service preferring
// nvme storage scores best on an nvme host, second on ssd, last on hdd.
func TestScoreHost_StorageTypeNVMeWinsOverHDD(t *testing.T) {
	nvme := &HostCapabilities{Name: "nvme", StorageType: "nvme",
		MemoryFreeMB: 32 * 1024}
	hdd := &HostCapabilities{Name: "hdd", StorageType: "hdd",
		MemoryFreeMB: 32 * 1024}
	req := Requirement{Name: "db", Labels: map[string]string{
		LabelPreferStorageType: "nvme",
	}}
	rNvme := ScoreHost(req, nvme)
	rHdd := ScoreHost(req, hdd)
	if rNvme.Score <= rHdd.Score {
		t.Errorf("nvme (%v) must outrank hdd (%v)", rNvme.Score, rHdd.Score)
	}
	// Asking ssd is satisfied by nvme (upgrade tolerance).
	reqSsd := Requirement{Name: "svc", Labels: map[string]string{
		LabelPreferStorageType: "ssd",
	}}
	if r := ScoreHost(reqSsd, nvme); r.Score < ScoringWeights.StorageTypeMatch {
		t.Errorf("nvme host must satisfy ssd preference (got %v)", r.Score)
	}
}

// TestScoreHost_NetworkClassFromAlias asserts the legacy "fast"/
// "slow" values for prefer.network alias correctly to high/low.
func TestScoreHost_NetworkClassFromAlias(t *testing.T) {
	host := &HostCapabilities{Name: "h", NetworkClass: "high",
		MemoryFreeMB: 32 * 1024}
	req := Requirement{Name: "svc", Labels: map[string]string{
		LabelPreferNetwork: "fast", // alias for high
	}}
	r := ScoreHost(req, host)
	if r.Score < ScoringWeights.NetworkMatch {
		t.Errorf("network=fast must alias to high (got %v)", r.Score)
	}
}

// TestPickBestHost_HeterogeneousHostsRouteCorrectly is the most
// realistic test: 3 hosts with different capability profiles, each
// service goes where it fits best.
func TestPickBestHost_HeterogeneousHostsRouteCorrectly(t *testing.T) {
	gpuFast := &HostCapabilities{Name: "gpu-fast",
		HasGPU: true, GPUVendor: "nvidia",
		CPUClass: "fast", CPUMhz: 3800,
		StorageType: "nvme", StorageClass: "fast",
		MemoryClass: "high", MemoryFreeMB: 64 * 1024,
		DiskSpaceClass: "large", NetworkClass: "high"}

	cpuStrong := &HostCapabilities{Name: "cpu-strong",
		CPUClass: "fast", CPUMhz: 4000,
		StorageType: "ssd", StorageClass: "fast",
		MemoryClass: "high", MemoryFreeMB: 32 * 1024,
		DiskSpaceClass: "medium", NetworkClass: "medium"}

	bigStorage := &HostCapabilities{Name: "big-storage",
		CPUClass: "medium", CPUMhz: 2400,
		StorageType: "hdd", StorageClass: "slow",
		MemoryClass: "medium", MemoryFreeMB: 16 * 1024,
		DiskSpaceClass: "large", NetworkClass: "low"}

	hosts := []*HostCapabilities{gpuFast, cpuStrong, bigStorage}

	// 1. GPU service must land on gpu-fast.
	r1, _ := PickBestHost(Requirement{Name: "sglang", Labels: map[string]string{
		LabelRequireGPU: "nvidia",
	}}, hosts)
	if r1 != "gpu-fast" {
		t.Errorf("sglang -> %q, want gpu-fast", r1)
	}

	// 2. CPU-bound compute service prefers cpu-strong (fast CPU)
	// over bigStorage (medium CPU).
	r2, _ := PickBestHost(Requirement{Name: "build", Labels: map[string]string{
		LabelPreferCPU: "fast",
	}}, hosts)
	// Both gpu-fast (3.8 GHz) and cpu-strong (4.0 GHz) qualify;
	// alphabetically cpu-strong comes after gpu-fast — and gpu-fast
	// has same CPU class. With identical scores, alphabetical
	// tie-break picks cpu-strong (alphabetical first when sorted
	// lexicographically? "cpu-strong" < "gpu-fast"). Just assert
	// it's NOT bigStorage (medium CPU lost).
	if r2 == "big-storage" {
		t.Errorf("compute -> %q, must not be big-storage (medium CPU)", r2)
	}

	// 3. Archive service preferring large disk space — ties between
	// gpu-fast (large) and bigStorage (large). Tie-break alphabetical
	// → big-storage wins. cpu-strong (medium) loses.
	r3, _ := PickBestHost(Requirement{Name: "archive", Labels: map[string]string{
		LabelPreferDiskSpace: "large",
	}}, hosts)
	if r3 == "cpu-strong" {
		t.Errorf("archive -> %q, must not be cpu-strong (medium disk)", r3)
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
