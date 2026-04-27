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
// Linux box running docker with one nvidia GPU and an NVMe SSD.
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
		"33554432", // MemTotal kB ≈ 32 GiB
		"16777216", // MemAvailable kB ≈ 16 GiB
		"100000",   // disk MB
		"8",        // nproc
		"---SECTION-5---",
		"0", // non-rotational present
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
}

// TestProbe_PodmanHostNoGPU exercises a podman host without a GPU
// — confirms HasGPU=false propagates and runtime is detected.
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
		"8388608", // MemTotal kB ≈ 8 GiB
		"4194304",
		"50000",
		"4",
		"---SECTION-5---",
		"1", // rotational only
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
	if caps.StorageClass != "slow" {
		t.Errorf("StorageClass=%q want slow", caps.StorageClass)
	}
	if caps.MemoryClass != "medium" {
		t.Errorf("MemoryClass=%q want medium (8 GiB)", caps.MemoryClass)
	}
}

// TestProbe_HostLabelOverride asserts operator-set labels in
// CONTAINERS_REMOTE_HOST_N_LABELS override probed values.
func TestProbe_HostLabelOverride(t *testing.T) {
	// Host probe says rotational/slow, but label says storage=fast.
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
		"100000",
		"8",
		"---SECTION-5---",
		"1", // rotational only
	}, "\n")

	prober := NewCapabilityProber(&fakeExec{stdout: canned})
	host := remote.RemoteHost{Name: "h", Address: "h.local", User: "u",
		Labels: map[string]string{"storage": "fast", "memory": "high", "network": "high"}}
	caps, err := prober.Probe(context.Background(), host)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if caps.StorageClass != "fast" {
		t.Errorf("operator override storage=fast lost; got %q", caps.StorageClass)
	}
	if caps.NetworkClass != "high" {
		t.Errorf("operator override network=high lost; got %q", caps.NetworkClass)
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
