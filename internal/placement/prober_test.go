package placement

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"digital.vasic.containers/pkg/remote"
)

// fakeExec implements remote.RemoteExecutor with a canned probe
// payload so prober_test.go can exercise parsing without SSH.
type fakeExec struct {
	stdout string
	err    error
}

func (f *fakeExec) Execute(_ context.Context, _ remote.RemoteHost, _ string) (*remote.CommandResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &remote.CommandResult{Stdout: f.stdout, ExitCode: 0}, nil
}
func (*fakeExec) ExecuteStream(context.Context, remote.RemoteHost, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (*fakeExec) CopyFile(context.Context, remote.RemoteHost, string, string) error { return nil }
func (*fakeExec) CopyDir(context.Context, remote.RemoteHost, string, string) error  { return nil }
func (*fakeExec) IsReachable(context.Context, remote.RemoteHost) bool                { return true }
func (*fakeExec) Close() error                                                       { return nil }

// TestProbe_DockerHostWithNvidia exercises the happy path: an x86_64
// Linux box running docker with one nvidia GPU, NVMe SSD, fast CPU,
// 10 GbE.
func TestProbe_DockerHostWithNvidia(t *testing.T) {
	canned := strings.Join([]string{
		"x86_64",
		"---SECTION-2---",
		"docker",
		"27.0.3",
		"---SECTION-3---",
		"1",
		"nvidia",
		"---SECTION-4---",
		"33554432",  // MemTotal kB ≈ 32 GiB
		"16777216",  // MemAvailable kB ≈ 16 GiB
		"600000",    // disk free MB (≥500 GB → large)
		"1000000",   // disk total MB
		"8",         // nproc
		"3500",      // CPU max MHz (3.5 GHz → fast)
		"---SECTION-5---",
		"nvme",      // storage type
		"---SECTION-6---",
		"10000",     // network speed Mbps (10 GbE → high)
	}, "\n")

	prober := NewCapabilityProber(&fakeExec{stdout: canned})
	caps, err := prober.Probe(context.Background(),
		remote.RemoteHost{Name: "amber", Address: "amber.local", User: "u"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if caps.Arch != "amd64" {
		t.Errorf("Arch=%q want amd64", caps.Arch)
	}
	if caps.Runtime != "docker" {
		t.Errorf("Runtime=%q want docker", caps.Runtime)
	}
	if caps.RuntimeVersion != "27.0.3" {
		t.Errorf("RuntimeVersion=%q want 27.0.3", caps.RuntimeVersion)
	}
	if !caps.HasGPU {
		t.Errorf("HasGPU=false, want true")
	}
	if caps.GPUVendor != "nvidia" {
		t.Errorf("GPUVendor=%q want nvidia", caps.GPUVendor)
	}
	if caps.GPUCount != 1 {
		t.Errorf("GPUCount=%d want 1", caps.GPUCount)
	}
	if caps.MemoryTotalMB < 32000 || caps.MemoryTotalMB > 32800 {
		t.Errorf("MemoryTotalMB=%d, want ~32768", caps.MemoryTotalMB)
	}
	if caps.CPUCores != 8 {
		t.Errorf("CPUCores=%d want 8", caps.CPUCores)
	}
	if caps.StorageClass != "fast" {
		t.Errorf("StorageClass=%q want fast", caps.StorageClass)
	}
	if caps.MemoryClass != "high" {
		t.Errorf("MemoryClass=%q want high (32 GiB)", caps.MemoryClass)
	}
	if caps.StorageType != "nvme" {
		t.Errorf("StorageType=%q want nvme", caps.StorageType)
	}
	if caps.CPUMhz != 3500 {
		t.Errorf("CPUMhz=%d want 3500", caps.CPUMhz)
	}
	if caps.CPUClass != "fast" {
		t.Errorf("CPUClass=%q want fast (3500 MHz)", caps.CPUClass)
	}
	if caps.DiskTotalMB != 1_000_000 {
		t.Errorf("DiskTotalMB=%d want 1000000", caps.DiskTotalMB)
	}
	if caps.DiskSpaceClass != "large" {
		t.Errorf("DiskSpaceClass=%q want large (600 GB free)", caps.DiskSpaceClass)
	}
	if caps.NetworkSpeedMbps != 10000 {
		t.Errorf("NetworkSpeedMbps=%d want 10000", caps.NetworkSpeedMbps)
	}
	if caps.NetworkClass != "high" {
		t.Errorf("NetworkClass=%q want high (10 GbE)", caps.NetworkClass)
	}
}

// TestProbe_PodmanHostNoGPU exercises a slower podman host: HDD, no
// GPU, modest CPU, 1 GbE — confirms degraded classes propagate.
func TestProbe_PodmanHostNoGPU(t *testing.T) {
	canned := strings.Join([]string{
		"x86_64",
		"---SECTION-2---",
		"podman",
		"podman version 5.4.0",
		"---SECTION-3---",
		"0",
		"none",
		"---SECTION-4---",
		"8388608",  // MemTotal kB ≈ 8 GiB
		"4194304",
		"80000",    // disk free MB (<100 GB → small)
		"500000",   // disk total MB
		"4",        // nproc
		"2400",     // CPU MHz (2.4 GHz → medium)
		"---SECTION-5---",
		"hdd",      // rotational only
		"---SECTION-6---",
		"1000",     // 1 GbE → medium network class
	}, "\n")

	prober := NewCapabilityProber(&fakeExec{stdout: canned})
	caps, err := prober.Probe(context.Background(),
		remote.RemoteHost{Name: "thinker", Address: "thinker.local", User: "u"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if caps.HasGPU {
		t.Errorf("HasGPU=true on no-GPU host")
	}
	if caps.Runtime != "podman" {
		t.Errorf("Runtime=%q want podman", caps.Runtime)
	}
	if caps.StorageType != "hdd" {
		t.Errorf("StorageType=%q want hdd", caps.StorageType)
	}
	if caps.StorageClass != "slow" {
		t.Errorf("StorageClass=%q want slow (derived from hdd)", caps.StorageClass)
	}
	if caps.MemoryClass != "medium" {
		t.Errorf("MemoryClass=%q want medium (8 GiB)", caps.MemoryClass)
	}
	if caps.CPUClass != "medium" {
		t.Errorf("CPUClass=%q want medium (2400 MHz)", caps.CPUClass)
	}
	if caps.DiskSpaceClass != "small" {
		t.Errorf("DiskSpaceClass=%q want small (80 GB free)", caps.DiskSpaceClass)
	}
	if caps.NetworkClass != "medium" {
		t.Errorf("NetworkClass=%q want medium (1 GbE)", caps.NetworkClass)
	}
}

// TestProbe_HostLabelOverride asserts operator-set labels in
// CONTAINERS_REMOTE_HOST_N_LABELS override probed values across
// every dimension.
func TestProbe_HostLabelOverride(t *testing.T) {
	// Host probe says hdd / slow CPU / small disk, but operator
	// labels paint it as a high-spec node (e.g. SAN-mounted fast
	// disk, custom hardware).
	canned := strings.Join([]string{
		"x86_64",
		"---SECTION-2---",
		"docker",
		"27.0.3",
		"---SECTION-3---",
		"0",
		"none",
		"---SECTION-4---",
		"33554432",
		"16777216",
		"50000",   // small disk per probe
		"500000",
		"8",
		"1500",    // slow CPU per probe
		"---SECTION-5---",
		"hdd",     // probe says rotational
		"---SECTION-6---",
		"100",     // probe says 100 Mbps
	}, "\n")

	prober := NewCapabilityProber(&fakeExec{stdout: canned})
	host := remote.RemoteHost{Name: "h", Address: "h.local", User: "u",
		Labels: map[string]string{
			"storage":      "fast",
			"storage_type": "nvme",
			"memory":       "high",
			"network":      "high",
			"cpu":          "fast",
			"disk_space":   "large",
		}}
	caps, err := prober.Probe(context.Background(), host)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if caps.StorageClass != "fast" {
		t.Errorf("storage override lost; got %q", caps.StorageClass)
	}
	if caps.StorageType != "nvme" {
		t.Errorf("storage_type override lost; got %q", caps.StorageType)
	}
	if caps.NetworkClass != "high" {
		t.Errorf("network override lost; got %q", caps.NetworkClass)
	}
	if caps.CPUClass != "fast" {
		t.Errorf("cpu override lost; got %q (probe MHz=%d)", caps.CPUClass, caps.CPUMhz)
	}
	if caps.DiskSpaceClass != "large" {
		t.Errorf("disk_space override lost; got %q", caps.DiskSpaceClass)
	}
}

// TestProbe_ExecutorErrorPropagates asserts the prober surfaces SSH
// errors so the caller can decide whether to skip the host or retry.
func TestProbe_ExecutorErrorPropagates(t *testing.T) {
	prober := NewCapabilityProber(&fakeExec{err: errors.New("ssh: connection refused")})
	_, err := prober.Probe(context.Background(),
		remote.RemoteHost{Name: "down", Address: "x.local", User: "u"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("expected error to wrap ssh failure, got %v", err)
	}
}
