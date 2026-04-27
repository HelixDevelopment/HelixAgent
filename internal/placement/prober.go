package placement

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"digital.vasic.containers/pkg/remote"
)

// CapabilityProber probes a single registered remote host for the
// capabilities the scorer needs. It composes on top of
// remote.RemoteExecutor (already used by the Containers module) so we
// reuse its SSH plumbing rather than reinventing a pool here.
//
// One probe = one SSH round-trip with a compound shell command. The
// output is parsed into a HostCapabilities. Failures degrade
// gracefully: missing fields are recorded as "" / 0; the host is
// still considered for placement using whatever was determinable.
type CapabilityProber struct {
	exec remote.RemoteExecutor
}

// NewCapabilityProber wires an executor (typically the same one the
// adapter uses).
func NewCapabilityProber(exec remote.RemoteExecutor) *CapabilityProber {
	return &CapabilityProber{exec: exec}
}

// Probe collects capability + resource snapshot for one host.
// Mirrors the shape of remote.Prober.Probe but adds the runtime,
// arch, GPU, and storage-class fields the scorer needs.
//
// The probe shell script is a single ssh round-trip that emits five
// `---SECTION-X---` markers; the parser splits and reads each.
func (p *CapabilityProber) Probe(
	ctx context.Context, host remote.RemoteHost,
) (*HostCapabilities, error) {
	if p.exec == nil {
		return nil, fmt.Errorf("capability prober has no executor")
	}

	cmd := strings.Join([]string{
		// 1. Architecture
		"uname -m",
		"echo '---SECTION-2---'",
		// 2. Container runtime + version. Try docker first, then
		// podman (matches the runtime selection logic in the adapter).
		"if command -v docker >/dev/null 2>&1; then echo docker; docker version --format '{{.Server.Version}}' 2>/dev/null || docker --version 2>/dev/null; " +
			"elif command -v podman >/dev/null 2>&1; then echo podman; podman version --format '{{.Server.Version}}' 2>/dev/null || podman --version 2>/dev/null; " +
			"else echo none; fi",
		"echo '---SECTION-3---'",
		// 3. GPU detection. nvidia-smi if present, else lspci grep.
		"if command -v nvidia-smi >/dev/null 2>&1; then nvidia-smi --query-gpu=count --format=csv,noheader 2>/dev/null | head -1; echo nvidia; " +
			"elif command -v rocm-smi >/dev/null 2>&1; then rocm-smi --showid 2>/dev/null | grep -c GPU || echo 0; echo amd; " +
			"elif command -v lspci >/dev/null 2>&1; then lspci 2>/dev/null | grep -iE 'vga|3d|display' | grep -ciE 'nvidia|amd|intel' || echo 0; lspci 2>/dev/null | grep -iE 'vga|3d|display' | head -1 | grep -oiE 'nvidia|amd|intel' | head -1 | tr 'A-Z' 'a-z' || echo none; " +
			"else echo 0; echo none; fi",
		"echo '---SECTION-4---'",
		// 4. Memory (total + free in kB) + disk free (MB on /) +
		// CPU core count + max CPU MHz. CPU MHz comes from lscpu
		// when available (`Max MHz`), falling back to
		// /proc/cpuinfo's first "cpu MHz" entry. lscpu's value is
		// the boost-locked maximum which is the right number for
		// scheduling decisions.
		"awk '/MemTotal:/ {print $2} /MemAvailable:/ {print $2}' /proc/meminfo",
		"df -BM --output=avail / 2>/dev/null | tail -1 | tr -d 'M' || echo 0",
		"df -BM --output=size / 2>/dev/null | tail -1 | tr -d 'M' || echo 0",
		"nproc 2>/dev/null || echo 1",
		"(lscpu 2>/dev/null | awk -F: '/CPU max MHz/ {gsub(/^[ \\t]+/,\"\",$2); print int($2); exit} /Max MHz/ {gsub(/^[ \\t]+/,\"\",$2); print int($2); exit}') || " +
			"awk -F: '/cpu MHz/ {gsub(/^[ \\t]+/,\"\",$2); print int($2); exit}' /proc/cpuinfo || echo 0",
		"echo '---SECTION-5---'",
		// 5. Storage type detection. We classify by walking
		// /sys/block: if any nvme*n* device exists ⇒ nvme; else if
		// any sd*/vd*/xvd* with rotational=0 exists ⇒ ssd; else
		// hdd. Network-attached storage isn't auto-detectable here
		// — operators set storage_type=network via host labels.
		"if ls /sys/block/nvme*n* 2>/dev/null | head -1 | grep -q .; then echo nvme; " +
			"elif for d in /sys/block/sd? /sys/block/vd? /sys/block/xvd?; do [ -r \"$d/queue/rotational\" ] && [ \"$(cat \"$d/queue/rotational\" 2>/dev/null)\" = \"0\" ] && echo found && break; done | grep -q found; then echo ssd; " +
			"else echo hdd; fi",
		"echo '---SECTION-6---'",
		// 6. Network speed. Walk /sys/class/net/* skipping
		// loopback, virtual interfaces (lo, docker*, veth*, br-*,
		// virbr*). Take the max valid speed (kernel reports -1 for
		// down/virtual interfaces; we filter those out).
		"max=0; for ifc in /sys/class/net/*; do " +
			"  name=$(basename \"$ifc\"); " +
			"  case \"$name\" in lo|docker*|veth*|br-*|virbr*|cni*|flannel*|tun*|tap*) continue;; esac; " +
			"  [ -r \"$ifc/speed\" ] || continue; " +
			"  s=$(cat \"$ifc/speed\" 2>/dev/null); " +
			"  if [ \"$s\" -gt 0 ] 2>/dev/null && [ \"$s\" -gt \"$max\" ]; then max=$s; fi; " +
			"done; echo $max",
	}, " && ")

	result, err := p.exec.Execute(ctx, host, cmd)
	if err != nil {
		return nil, fmt.Errorf("probe %s: %w", host.Name, err)
	}

	caps := &HostCapabilities{
		Name:   host.Name,
		Labels: copyLabels(host.Labels),
	}
	parseCapabilityProbeOutput(caps, result.Stdout)
	deriveClassesFromCaps(caps)
	overrideFromHostLabels(caps, host.Labels)
	return caps, nil
}

func parseCapabilityProbeOutput(caps *HostCapabilities, output string) {
	sections := strings.Split(output, "---SECTION-")
	if len(sections) < 1 {
		return
	}
	// sections[0] = arch
	// sections[1] starts with "2---\n" then runtime info etc.
	if arch := strings.TrimSpace(sections[0]); arch != "" {
		caps.Arch = normalizeArch(arch)
	}
	// Trim leading "<n>---\n" tags from subsequent sections.
	for i := 1; i < len(sections); i++ {
		sections[i] = trimSectionTag(sections[i])
	}

	// 2: runtime
	if len(sections) >= 2 {
		lines := splitNonEmpty(sections[1])
		if len(lines) >= 1 {
			caps.Runtime = strings.TrimSpace(lines[0])
		}
		if len(lines) >= 2 {
			caps.RuntimeVersion = extractVersion(lines[1])
		}
	}

	// 3: GPU (count, vendor)
	if len(sections) >= 3 {
		lines := splitNonEmpty(sections[2])
		if len(lines) >= 1 {
			if n, err := strconv.Atoi(strings.TrimSpace(lines[0])); err == nil && n > 0 {
				caps.HasGPU = true
				caps.GPUCount = n
			}
		}
		if len(lines) >= 2 {
			v := strings.ToLower(strings.TrimSpace(lines[1]))
			if v != "" && v != "none" {
				caps.GPUVendor = v
			}
		}
		if !caps.HasGPU {
			// A "0\nnvidia" output from lspci means the vendor card was
			// detected but not counted — unusual; default to no GPU.
			caps.GPUVendor = ""
		}
	}

	// 4: memory(total kB) + memory(free kB) + disk(free MB) +
	// disk(total MB) + cpu cores + max cpu MHz
	if len(sections) >= 4 {
		lines := splitNonEmpty(sections[3])
		if len(lines) >= 1 {
			if kb, err := strconv.ParseUint(strings.TrimSpace(lines[0]), 10, 64); err == nil {
				caps.MemoryTotalMB = kb / 1024
			}
		}
		if len(lines) >= 2 {
			if kb, err := strconv.ParseUint(strings.TrimSpace(lines[1]), 10, 64); err == nil {
				caps.MemoryFreeMB = kb / 1024
			}
		}
		if len(lines) >= 3 {
			if mb, err := strconv.ParseUint(strings.TrimSpace(lines[2]), 10, 64); err == nil {
				caps.DiskFreeMB = mb
			}
		}
		if len(lines) >= 4 {
			if mb, err := strconv.ParseUint(strings.TrimSpace(lines[3]), 10, 64); err == nil {
				caps.DiskTotalMB = mb
			}
		}
		if len(lines) >= 5 {
			if n, err := strconv.Atoi(strings.TrimSpace(lines[4])); err == nil {
				caps.CPUCores = n
			}
		}
		if len(lines) >= 6 {
			if mhz, err := strconv.Atoi(strings.TrimSpace(lines[5])); err == nil {
				caps.CPUMhz = mhz
			}
		}
	}

	// 5: storage type — direct value from probe (nvme/ssd/hdd).
	if len(sections) >= 5 {
		val := strings.TrimSpace(sections[4])
		// Take the first non-empty token; older shells echo empty
		// lines on else branches.
		for _, ln := range strings.Split(val, "\n") {
			t := strings.TrimSpace(ln)
			if t == "nvme" || t == "ssd" || t == "hdd" {
				caps.StorageType = t
				break
			}
		}
	}

	// 6: network speed (Mbps; 0 if undetectable).
	if len(sections) >= 6 {
		val := strings.TrimSpace(sections[5])
		// Find the first integer token (the script ends with the
		// max value but earlier sections may have leaked content
		// when the shell builtin behaved unexpectedly).
		for _, ln := range strings.Split(val, "\n") {
			t := strings.TrimSpace(ln)
			if n, err := strconv.Atoi(t); err == nil {
				caps.NetworkSpeedMbps = n
				break
			}
		}
	}
}

func deriveClassesFromCaps(caps *HostCapabilities) {
	if caps.MemoryClass == "" {
		switch {
		case caps.MemoryTotalMB >= 32*1024:
			caps.MemoryClass = "high"
		case caps.MemoryTotalMB >= 8*1024:
			caps.MemoryClass = "medium"
		default:
			caps.MemoryClass = "low"
		}
	}
	if caps.CPUClass == "" {
		switch {
		case caps.CPUMhz >= 3000:
			caps.CPUClass = "fast"
		case caps.CPUMhz >= 2000:
			caps.CPUClass = "medium"
		case caps.CPUMhz > 0:
			caps.CPUClass = "slow"
		}
	}
	if caps.DiskSpaceClass == "" {
		switch {
		case caps.DiskFreeMB >= 500_000:
			caps.DiskSpaceClass = "large"
		case caps.DiskFreeMB >= 100_000:
			caps.DiskSpaceClass = "medium"
		case caps.DiskFreeMB > 0:
			caps.DiskSpaceClass = "small"
		}
	}
	// Coarse StorageClass kept in sync with the more specific
	// StorageType so existing prefer.storage labels keep working.
	if caps.StorageClass == "" {
		switch caps.StorageType {
		case "nvme", "ssd":
			caps.StorageClass = "fast"
		case "hdd":
			caps.StorageClass = "slow"
		}
	}
	if caps.NetworkClass == "" {
		switch {
		case caps.NetworkSpeedMbps >= 10000:
			caps.NetworkClass = "high"
		case caps.NetworkSpeedMbps >= 1000:
			caps.NetworkClass = "medium"
		case caps.NetworkSpeedMbps > 0:
			caps.NetworkClass = "low"
		}
	}
}

// overrideFromHostLabels lets operators force a class via
// CONTAINERS_REMOTE_HOST_N_LABELS. Already-parsed labels at the
// adapter level appear here as host.Labels. Every dimension is
// override-able so an operator can correct probe misclassification
// (e.g. SAN-mounted disk that reports rotational=1 but is fast).
func overrideFromHostLabels(caps *HostCapabilities, labels map[string]string) {
	if v, ok := labels["storage"]; ok && v != "" {
		caps.StorageClass = strings.ToLower(v)
	}
	if v, ok := labels["storage_type"]; ok && v != "" {
		caps.StorageType = strings.ToLower(v)
	}
	if v, ok := labels["memory"]; ok && v != "" {
		caps.MemoryClass = strings.ToLower(v)
	}
	if v, ok := labels["network"]; ok && v != "" {
		caps.NetworkClass = strings.ToLower(v)
	}
	if v, ok := labels["cpu"]; ok && v != "" {
		caps.CPUClass = strings.ToLower(v)
	}
	if v, ok := labels["disk_space"]; ok && v != "" {
		caps.DiskSpaceClass = strings.ToLower(v)
	}
}

// trimSectionTag drops a leading "<n>---\n" marker.
func trimSectionTag(s string) string {
	if i := strings.Index(s, "---"); i >= 0 && i < 3 {
		s = s[i+3:]
	}
	return strings.TrimLeft(s, "\n")
}

func splitNonEmpty(s string) []string {
	out := make([]string, 0, 4)
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func normalizeArch(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "x86_64", "amd64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func extractVersion(line string) string {
	// docker version --format -> "27.0.3" (clean)
	// docker --version       -> "Docker version 27.0.3, build abc"
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if !strings.Contains(line, " ") {
		return line
	}
	for _, tok := range strings.Fields(line) {
		// Pick the first dotted-numeric.
		dots := 0
		ok := true
		for _, r := range tok {
			if r == '.' {
				dots++
			} else if r < '0' || r > '9' {
				if r != ',' {
					ok = false
					break
				}
			}
		}
		if ok && dots >= 1 {
			return strings.TrimSuffix(tok, ",")
		}
	}
	return line
}

func copyLabels(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
