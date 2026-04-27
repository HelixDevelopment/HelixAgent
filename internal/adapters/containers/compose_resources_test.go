package containers

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestComposeResourceInvariants is the in-process counterpart to
// challenges/scripts/compose_resource_limits_challenge.sh. It enforces, on
// every CI run and `go test ./...`, that docker-compose.yml is at the
// canonical Compose v2/v3 resource form. Specifically:
//
//  1. No service mixes legacy `mem_limit` / `memswap_limit` / `pids_limit`
//     with `deploy.resources.limits.*` (the conflict that caused the
//     amber.local boot to fail with "can't set distinct values" on
//     2026-04-27).
//  2. Every required runtime service declares `deploy.resources.limits.memory`,
//     `deploy.resources.limits.cpus`, and matching `reservations` so that
//     remote distribution gets predictable performance.
//  3. Reservations are not greater than limits.
//  4. The file uses the env-var-driven form (`${SERVICE_FIELD:-default}`) so
//     resources scale to production without YAML edits.
//
// Worked-example failure this catches: a future PR re-introducing
// `mem_limit: 2g` next to `deploy.resources.limits.memory: 4G` (cognee
// regression, BUGFIXES.md issue #compose-mem-conflict).
func TestComposeResourceInvariants(t *testing.T) {
	path := findComposeFile(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var doc struct {
		Services map[string]resourceTestSvc `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	required := map[string]bool{
		"postgres": true, "redis": true, "chromadb": true, "cognee": true,
		"neo4j": true, "memgraph": true, "prometheus": true, "grafana": true,
		"mock-llm": true, "langchain-server": true, "llamaindex-server": true,
		"guidance-server": true, "lmql-server": true, "sglang": true,
		"ollama": true, "helixagent": true,
	}

	var violations []string
	seen := map[string]bool{}

	for name, svc := range doc.Services {
		seen[name] = true

		// Check 1: no legacy keys mixed with deploy.resources.
		if svc.MemLimit != "" {
			violations = append(violations, fmt.Sprintf(
				"%s: legacy mem_limit=%q must not be set; use deploy.resources.limits.memory only",
				name, svc.MemLimit))
		}
		if svc.MemswapLimit != "" {
			violations = append(violations, fmt.Sprintf(
				"%s: legacy memswap_limit=%q must not be set", name, svc.MemswapLimit))
		}
		if svc.PidsLimit != 0 {
			violations = append(violations, fmt.Sprintf(
				"%s: legacy pids_limit=%d must not be set; use deploy.resources.limits.pids only",
				name, svc.PidsLimit))
		}

		if !required[name] {
			continue
		}

		lim := svc.Deploy.Resources.Limits
		rsv := svc.Deploy.Resources.Reservations

		// Check 2 & 4: required fields with env-var form.
		mustEnvVar(t, &violations, name, "limits.memory", lim.Memory)
		mustEnvVar(t, &violations, name, "limits.cpus", lim.CPUs)
		if lim.Pids == 0 {
			violations = append(violations, fmt.Sprintf(
				"%s: missing deploy.resources.limits.pids", name))
		}
		mustEnvVar(t, &violations, name, "reservations.memory", rsv.Memory)
		mustEnvVar(t, &violations, name, "reservations.cpus", rsv.CPUs)

		// Check 3: reservations <= limits.
		if lb, rb := parseBytes(lim.Memory), parseBytes(rsv.Memory); lb > 0 && rb > lb {
			violations = append(violations, fmt.Sprintf(
				"%s: reservations.memory (%s) > limits.memory (%s)",
				name, rsv.Memory, lim.Memory))
		}
		if lc, rc := parseCPUs(lim.CPUs), parseCPUs(rsv.CPUs); lc > 0 && rc > lc {
			violations = append(violations, fmt.Sprintf(
				"%s: reservations.cpus (%s) > limits.cpus (%s)",
				name, rsv.CPUs, lim.CPUs))
		}
	}

	for name := range required {
		if !seen[name] {
			violations = append(violations, fmt.Sprintf("required service %q not present", name))
		}
	}

	if len(violations) > 0 {
		t.Errorf("docker-compose.yml has %d resource-config violation(s):\n  - %s",
			len(violations), strings.Join(violations, "\n  - "))
	}
}

// resourceTestSvc captures only the fields needed for this test. Other keys
// in docker-compose.yml are ignored.
type resourceTestSvc struct {
	MemLimit     string `yaml:"mem_limit"`
	MemswapLimit string `yaml:"memswap_limit"`
	PidsLimit    int    `yaml:"pids_limit"`
	Deploy       struct {
		Resources struct {
			Limits struct {
				Memory string `yaml:"memory"`
				CPUs   string `yaml:"cpus"`
				Pids   int    `yaml:"pids"`
			} `yaml:"limits"`
			Reservations struct {
				Memory string `yaml:"memory"`
				CPUs   string `yaml:"cpus"`
			} `yaml:"reservations"`
		} `yaml:"resources"`
	} `yaml:"deploy"`
}

var envVarRe = regexp.MustCompile(`^\$\{[A-Z][A-Z0-9_]*:-([^}]+)\}$`)

func mustEnvVar(t *testing.T, violations *[]string, service, field, value string) {
	t.Helper()
	if value == "" {
		*violations = append(*violations, fmt.Sprintf(
			"%s: missing deploy.resources.%s", service, field))
		return
	}
	if !envVarRe.MatchString(value) {
		*violations = append(*violations, fmt.Sprintf(
			"%s: deploy.resources.%s = %q is not in canonical ${X:-default} form",
			service, field, value))
	}
}

// findComposeFile locates docker-compose.yml relative to this test source
// file's directory, walking up until found. This keeps the test runnable
// from any working directory.
func findComposeFile(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "docker-compose.yml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("docker-compose.yml not found above %s", thisFile)
	return ""
}

// parseBytes turns "4G" / "${X:-4G}" / "256M" / etc. into bytes. 0 on parse
// failure or unknown.
func parseBytes(v string) int64 {
	if m := envVarRe.FindStringSubmatch(v); m != nil {
		v = m[1]
	}
	v = strings.TrimSpace(strings.ToLower(v))
	if strings.HasSuffix(v, "b") {
		v = strings.TrimSuffix(v, "b")
	}
	if v == "" {
		return 0
	}
	last := v[len(v)-1]
	mult := int64(1)
	switch last {
	case 'k':
		mult = 1024
		v = v[:len(v)-1]
	case 'm':
		mult = 1024 * 1024
		v = v[:len(v)-1]
	case 'g':
		mult = 1024 * 1024 * 1024
		v = v[:len(v)-1]
	case 't':
		mult = 1024 * 1024 * 1024 * 1024
		v = v[:len(v)-1]
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return int64(f * float64(mult))
}

func parseCPUs(v string) float64 {
	if m := envVarRe.FindStringSubmatch(v); m != nil {
		v = m[1]
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return 0
	}
	return f
}
