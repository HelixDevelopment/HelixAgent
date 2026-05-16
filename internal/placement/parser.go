// Package placement translates docker-compose service definitions into
// scheduler.ContainerRequirements so HelixAgent can distribute services
// across hosts using the existing scheduler/distributor infrastructure
// in containers/pkg/{scheduler,distribution,serviceregistry}.
//
// All deploy actions remain inside the bin/helixagent boot flow per
// CONST-031 — this package is a pure translation layer. It does NOT
// invoke compose, ssh, or any container runtime.
package placement

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"digital.vasic.containers/pkg/scheduler"
	"gopkg.in/yaml.v3"
)

// CoLocationLabel is the affinity-group label written onto every
// ContainerRequirements. Services that share a value MUST end up on the
// same host (Docker Compose `depends_on` only works intra-host). The
// scheduler's StrategyAffinity respects matching label values.
const CoLocationLabel = "helixagent.placement.group"

// ComposeFileLabel records the source compose file so the per-host
// emitter can later reassemble services that came from the same file.
const ComposeFileLabel = "helixagent.placement.compose"

// composeService is the subset of compose schema we read.
type composeService struct {
	Image     string                 `yaml:"image"`
	Profiles  []string               `yaml:"profiles"`
	DependsOn interface{}            `yaml:"depends_on"`
	Deploy    composeDeploy          `yaml:"deploy"`
	Build     interface{}            `yaml:"build"`
	Ports     []interface{}          `yaml:"ports"`
	// Labels is the standard Compose service labels block. The
	// placement layer reads `helixagent.placement.{require,prefer}.X`
	// keys here as capability hints (capability.go LabelXxx
	// constants). Other label keys are ignored by placement.
	Labels    map[string]string      `yaml:"labels"`
	Extras    map[string]interface{} `yaml:",inline"`
}

type composeDeploy struct {
	Resources composeResources `yaml:"resources"`
}

type composeResources struct {
	Limits       composeLimits       `yaml:"limits"`
	Reservations composeReservations `yaml:"reservations"`
}

type composeLimits struct {
	Memory string `yaml:"memory"`
	CPUs   string `yaml:"cpus"`
}

type composeReservations struct {
	Memory  string                       `yaml:"memory"`
	CPUs    string                       `yaml:"cpus"`
	Devices []composeReservationDevice   `yaml:"devices"`
}

type composeReservationDevice struct {
	Driver       string   `yaml:"driver"`
	Count        int      `yaml:"count"`
	Capabilities []string `yaml:"capabilities"`
}

type composeDoc struct {
	Services map[string]composeService `yaml:"services"`
}

// ParseCompose reads `composeFile` and returns one
// scheduler.ContainerRequirements per service. Co-location groups are
// computed from `depends_on`: any service depending on another is
// placed in the same group.
//
// `profile` filters services: only services whose `profiles:` list
// includes `profile`, OR services with no profiles list, are returned.
// Pass "" to include every service.
func ParseCompose(composeFile, profile string) ([]scheduler.ContainerRequirements, error) {
	data, err := os.ReadFile(composeFile)
	if err != nil {
		return nil, fmt.Errorf("read compose file: %w", err)
	}

	var doc composeDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse compose: %w", err)
	}

	// Step 1: build dependency adjacency for union-find co-location.
	deps := make(map[string][]string, len(doc.Services))
	for name, svc := range doc.Services {
		deps[name] = parseDependsOn(svc.DependsOn)
	}
	groups := computeCoLocationGroups(deps)

	// Step 2: produce ContainerRequirements per service that matches
	// the requested profile.
	var reqs []scheduler.ContainerRequirements
	for name, svc := range doc.Services {
		if !matchesProfile(svc.Profiles, profile) {
			continue
		}
		labels := map[string]string{
			CoLocationLabel:  groups[name],
			ComposeFileLabel: composeFile,
		}
		// Capability hints from compose service labels. We pass
		// through every `helixagent.placement.{require,prefer}.X` key
		// so the scorer in capability.go can read them. Other label
		// keys are ignored.
		for k, v := range svc.Labels {
			if strings.HasPrefix(k, "helixagent.placement.") {
				labels[k] = v
			}
		}
		// Auto-derive: any service with a GPU reservation on
		// devices implicitly requires GPU even if it didn't set the
		// label explicitly. Backward-compatible with services that
		// already declared the device.
		gpu := extractGPU(svc.Deploy.Resources.Reservations.Devices)
		if gpu != nil {
			if _, set := labels[LabelRequireGPU]; !set {
				labels[LabelRequireGPU] = strings.ToLower(gpu.Vendor)
				if labels[LabelRequireGPU] == "" {
					labels[LabelRequireGPU] = "true"
				}
			}
		}
		// Auto-derive memory class preference from limits.memory:
		// XL services (>=8 GiB) prefer high-memory hosts unless
		// the operator already set a value.
		if _, set := labels[LabelPreferMemory]; !set {
			memMB := bytesToMB(svc.Deploy.Resources.Limits.Memory)
			switch {
			case memMB >= 8*1024:
				labels[LabelPreferMemory] = "high"
			case memMB >= 2*1024:
				labels[LabelPreferMemory] = "medium"
			}
		}

		req := scheduler.ContainerRequirements{
			Name:        name,
			Image:       svc.Image,
			ComposeFile: composeFile,
			ServiceName: name,
			MemoryMB:    bytesToMB(svc.Deploy.Resources.Limits.Memory),
			CPUCores:    parseCPUs(svc.Deploy.Resources.Limits.CPUs),
			Labels:      labels,
		}
		if gpu != nil {
			req.GPU = gpu
		}
		reqs = append(reqs, req)
	}
	return reqs, nil
}

// parseDependsOn handles both list form (`["postgres", "redis"]`) and
// map form (`postgres: {condition: service_healthy}`).
func parseDependsOn(raw interface{}) []string {
	if raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case map[string]interface{}:
		out := make([]string, 0, len(v))
		for k := range v {
			out = append(out, k)
		}
		return out
	}
	return nil
}

// computeCoLocationGroups uses union-find to merge any services
// connected by `depends_on` edges into the same group. Returns
// service-name -> group-id (deterministic: lexicographically smallest
// name in the group).
func computeCoLocationGroups(deps map[string][]string) map[string]string {
	parent := make(map[string]string, len(deps))
	for name := range deps {
		parent[name] = name
	}
	var find func(string) string
	find = func(x string) string {
		if parent[x] == x {
			return x
		}
		parent[x] = find(parent[x])
		return parent[x]
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		// Always elect the lexicographically smallest name as root —
		// makes group IDs deterministic across reboots.
		if ra < rb {
			parent[rb] = ra
		} else {
			parent[ra] = rb
		}
	}
	for name, ds := range deps {
		for _, d := range ds {
			if _, ok := parent[d]; !ok {
				// External dependency (not in this compose) — record
				// it as a fresh root so the group still anchors.
				parent[d] = d
			}
			union(name, d)
		}
	}
	out := make(map[string]string, len(parent))
	for name := range parent {
		out[name] = find(name)
	}
	return out
}

// matchesProfile honors the docker-compose v2/v3 profile semantics: a
// service runs IF (no profiles set) OR (one of its profiles == active).
// Pass "" as `active` to match services with NO profiles list (default
// profile only).
func matchesProfile(svcProfiles []string, active string) bool {
	if len(svcProfiles) == 0 {
		return true
	}
	if active == "" {
		return false
	}
	for _, p := range svcProfiles {
		if p == active {
			return true
		}
	}
	return false
}

// bytesToMB converts a Compose memory string (e.g. "4G", "${X:-512M}")
// to megabytes. Unwraps `${VAR:-default}` interpolation. Returns 0 on
// parse failure (treated as unconstrained by the scheduler).
func bytesToMB(v string) uint64 {
	v = unwrapEnvVar(strings.TrimSpace(v))
	if v == "" {
		return 0
	}
	v = strings.ToLower(v)
	if strings.HasSuffix(v, "b") {
		v = v[:len(v)-1]
	}
	if v == "" {
		return 0
	}
	last := v[len(v)-1]
	mult := uint64(1)
	switch last {
	case 'k':
		mult = 1
		v = v[:len(v)-1]
		f, _ := strconv.ParseFloat(v, 64)
		return uint64(f) / 1024 // KB → MB
	case 'm':
		mult = 1
		v = v[:len(v)-1]
	case 'g':
		mult = 1024
		v = v[:len(v)-1]
	case 't':
		mult = 1024 * 1024
		v = v[:len(v)-1]
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return uint64(f * float64(mult))
}

// parseCPUs converts "${X:-2.00}" or "2.0" to a float. 0 on failure.
func parseCPUs(v string) float64 {
	v = unwrapEnvVar(strings.TrimSpace(v))
	if v == "" {
		return 0
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return f
}

// unwrapEnvVar handles `${NAME:-default}` interpolation. Returns the
// default value (we evaluate placement at boot using the dev defaults;
// production overrides via env are honored by Compose at deploy time).
func unwrapEnvVar(v string) string {
	if !strings.HasPrefix(v, "${") || !strings.HasSuffix(v, "}") {
		return v
	}
	inner := v[2 : len(v)-1]
	if i := strings.Index(inner, ":-"); i >= 0 {
		return inner[i+2:]
	}
	if i := strings.Index(inner, ":"); i >= 0 {
		return inner[i+1:]
	}
	return inner
}

// extractGPU surfaces a GPURequirement when any device reservation
// asks for a GPU capability. Returns nil if no GPU is requested.
func extractGPU(devices []composeReservationDevice) *scheduler.GPURequirement {
	for _, d := range devices {
		for _, cap := range d.Capabilities {
			if strings.EqualFold(cap, "gpu") {
				count := d.Count
				if count <= 0 {
					count = 1
				}
				return &scheduler.GPURequirement{
					Vendor: strings.ToLower(d.Driver),
					Count:  count,
				}
			}
		}
	}
	return nil
}
