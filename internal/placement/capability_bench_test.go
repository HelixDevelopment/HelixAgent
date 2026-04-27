package placement

import (
	"fmt"
	"testing"
)

// BenchmarkPickBestHost_ScaleAcrossHosts measures placement-decision
// time as the registered-host count grows. Placement runs at boot —
// it must be sub-millisecond per group even with 16 hosts so it
// doesn't bottleneck the orchestrator's startup.
func BenchmarkPickBestHost_ScaleAcrossHosts(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16} {
		b.Run(fmt.Sprintf("hosts=%d", n), func(b *testing.B) {
			hosts := make([]*HostCapabilities, n)
			for i := 0; i < n; i++ {
				hosts[i] = &HostCapabilities{
					Name:           fmt.Sprintf("h%02d", i),
					MemoryFreeMB:   32 * 1024,
					StorageClass:   "fast",
					MemoryClass:    "high",
					Runtime:        "docker",
					Arch:           "amd64",
					HasGPU:         i == 0, // only first has GPU
					GPUVendor:      "nvidia",
					PlacementCount: i, // simulate spread of existing load
				}
			}
			req := Requirement{
				Name:     "svc",
				MemoryMB: 4 * 1024,
				Labels: map[string]string{
					LabelPreferStorage: "fast",
					LabelPreferMemory:  "medium",
				},
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = PickBestHost(req, hosts)
			}
		})
	}
}

// BenchmarkPickBestHost_GPURequired measures the eligibility-filter
// cost when most hosts get rejected by the hard GPU constraint.
func BenchmarkPickBestHost_GPURequired(b *testing.B) {
	hosts := make([]*HostCapabilities, 16)
	for i := 0; i < len(hosts); i++ {
		hosts[i] = &HostCapabilities{
			Name:         fmt.Sprintf("h%02d", i),
			MemoryFreeMB: 32 * 1024,
			HasGPU:       i == 0, // only h00 has GPU
			GPUVendor:    "nvidia",
		}
	}
	req := Requirement{Name: "sglang", Labels: map[string]string{
		LabelRequireGPU: "true",
	}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = PickBestHost(req, hosts)
	}
}
