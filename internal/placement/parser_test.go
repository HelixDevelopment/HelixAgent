package placement

import (
	"os"
	"path/filepath"
	"testing"

	"digital.vasic.containers/pkg/scheduler"
)

// composeFixture writes a compose file to a temp dir and returns its path.
func composeFixture(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

func TestParseCompose_BasicService(t *testing.T) {
	p := composeFixture(t, `
services:
  redis:
    image: redis:7-alpine
    deploy:
      resources:
        limits:
          memory: 1G
          cpus: "0.50"
`)
	reqs, err := ParseCompose(p, "")
	if err != nil {
		t.Fatalf("ParseCompose: %v", err)
	}
	if len(reqs) != 1 {
		t.Fatalf("want 1 req, got %d", len(reqs))
	}
	r := reqs[0]
	if r.Name != "redis" {
		t.Errorf("Name=%q want redis", r.Name)
	}
	if r.Image != "redis:7-alpine" {
		t.Errorf("Image=%q", r.Image)
	}
	if r.MemoryMB != 1024 {
		t.Errorf("MemoryMB=%d want 1024", r.MemoryMB)
	}
	if r.CPUCores != 0.5 {
		t.Errorf("CPUCores=%v want 0.5", r.CPUCores)
	}
	if r.Labels[CoLocationLabel] != "redis" {
		t.Errorf("CoLocationLabel=%q want redis (no deps -> own group)",
			r.Labels[CoLocationLabel])
	}
}

func TestParseCompose_DependsOnFormsCoLocationGroup(t *testing.T) {
	// cognee depends on postgres + redis + chromadb. All four MUST be
	// in the same co-location group.
	p := composeFixture(t, `
services:
  postgres:
    image: postgres:15
  redis:
    image: redis:7
  chromadb:
    image: chromadb/chroma
  cognee:
    image: cognee:latest
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      chromadb:
        condition: service_started
  unrelated:
    image: foo:latest
`)
	reqs, err := ParseCompose(p, "")
	if err != nil {
		t.Fatalf("ParseCompose: %v", err)
	}
	groups := make(map[string]string)
	for _, r := range reqs {
		groups[r.Name] = r.Labels[CoLocationLabel]
	}
	core := groups["cognee"]
	if groups["postgres"] != core {
		t.Errorf("postgres group=%q != cognee group=%q", groups["postgres"], core)
	}
	if groups["redis"] != core {
		t.Errorf("redis group=%q != cognee group=%q", groups["redis"], core)
	}
	if groups["chromadb"] != core {
		t.Errorf("chromadb group=%q != cognee group=%q", groups["chromadb"], core)
	}
	if groups["unrelated"] == core {
		t.Errorf("unrelated should NOT share group with cognee stack")
	}
}

func TestParseCompose_ListFormDependsOn(t *testing.T) {
	// `depends_on: [a, b]` — list form, not map form.
	p := composeFixture(t, `
services:
  a:
    image: a:1
  b:
    image: b:1
  c:
    image: c:1
    depends_on:
      - a
      - b
`)
	reqs, err := ParseCompose(p, "")
	if err != nil {
		t.Fatalf("ParseCompose: %v", err)
	}
	groups := make(map[string]string)
	for _, r := range reqs {
		groups[r.Name] = r.Labels[CoLocationLabel]
	}
	if groups["a"] != groups["c"] || groups["b"] != groups["c"] {
		t.Errorf("expected a, b, c all in same group; got %+v", groups)
	}
}

func TestParseCompose_EnvVarInterpolation(t *testing.T) {
	p := composeFixture(t, `
services:
  postgres:
    image: postgres:15
    deploy:
      resources:
        limits:
          memory: ${POSTGRES_MEM_LIMIT:-4G}
          cpus: "${POSTGRES_CPU_LIMIT:-2.00}"
`)
	reqs, err := ParseCompose(p, "")
	if err != nil {
		t.Fatalf("ParseCompose: %v", err)
	}
	r := reqs[0]
	if r.MemoryMB != 4096 {
		t.Errorf("MemoryMB=%d want 4096 (from ${X:-4G})", r.MemoryMB)
	}
	if r.CPUCores != 2.0 {
		t.Errorf("CPUCores=%v want 2.0", r.CPUCores)
	}
}

func TestParseCompose_GPUDetection(t *testing.T) {
	p := composeFixture(t, `
services:
  sglang:
    image: lmsysorg/sglang
    deploy:
      resources:
        limits:
          memory: 4G
          cpus: "2.00"
        reservations:
          memory: 1G
          cpus: "0.50"
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
`)
	reqs, err := ParseCompose(p, "")
	if err != nil {
		t.Fatalf("ParseCompose: %v", err)
	}
	r := reqs[0]
	if r.GPU == nil {
		t.Fatalf("GPU=nil — expected non-nil GPU requirement")
	}
	if r.GPU.Vendor != "nvidia" {
		t.Errorf("GPU.Vendor=%q want nvidia", r.GPU.Vendor)
	}
	if r.GPU.Count != 1 {
		t.Errorf("GPU.Count=%d want 1", r.GPU.Count)
	}
}

func TestParseCompose_ProfileFiltering(t *testing.T) {
	p := composeFixture(t, `
services:
  always:
    image: a:1
  optional:
    image: o:1
    profiles:
      - extra
  default-only:
    image: d:1
    profiles:
      - default
`)
	// Default profile (no profile argument) returns only services
	// with no profiles list.
	reqs, err := ParseCompose(p, "")
	if err != nil {
		t.Fatalf("ParseCompose: %v", err)
	}
	if len(reqs) != 1 || reqs[0].Name != "always" {
		t.Errorf("default profile: got %d reqs, names=%v", len(reqs), names(reqs))
	}
	// "extra" profile activates `optional` AND keeps `always`.
	reqs2, _ := ParseCompose(p, "extra")
	got := names(reqs2)
	if !contains(got, "always") || !contains(got, "optional") || contains(got, "default-only") {
		t.Errorf("extra profile names=%v", got)
	}
}

func TestParseCompose_NoServices(t *testing.T) {
	p := composeFixture(t, `version: '3.8'`)
	reqs, err := ParseCompose(p, "")
	if err != nil {
		t.Fatalf("ParseCompose: %v", err)
	}
	if len(reqs) != 0 {
		t.Errorf("expected 0 reqs, got %d", len(reqs))
	}
}

// Real-file integration: ensure the project's own docker-compose.yml
// parses cleanly and cognee's group includes its declared dependencies.
func TestParseCompose_RealMainCompose(t *testing.T) {
	p := findRealCompose(t, "docker-compose.yml")
	reqs, err := ParseCompose(p, "default")
	if err != nil {
		t.Fatalf("ParseCompose: %v", err)
	}
	if len(reqs) == 0 {
		t.Fatal("no services parsed")
	}
	groups := make(map[string]string)
	for _, r := range reqs {
		groups[r.Name] = r.Labels[CoLocationLabel]
	}
	// cognee depends on postgres + redis + chromadb.
	if groups["cognee"] == "" {
		// SKIP-OK: #compose-profile-default — cognee is in an optional
		// profile of docker-compose.yml; when this test runs against a
		// profile that doesn't include it, the co-location invariant
		// has nothing to verify. Per CLAUDE.md DoD rule 4 the marker
		// makes absence-of-coverage loud rather than silent.
		t.Skip("cognee not in default profile of this compose file (SKIP-OK: #compose-profile-default)")
	}
	for _, dep := range []string{"postgres", "redis", "chromadb"} {
		if groups[dep] != groups["cognee"] {
			t.Errorf("expected %s group=%q == cognee group=%q",
				dep, groups[dep], groups["cognee"])
		}
	}
}

func names(reqs []scheduler.ContainerRequirements) []string {
	out := make([]string, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, r.Name)
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func findRealCompose(t *testing.T, name string) string {
	t.Helper()
	wd, _ := os.Getwd()
	d := wd
	for i := 0; i < 8; i++ {
		c := filepath.Join(d, name)
		if _, err := os.Stat(c); err == nil {
			return c
		}
		d = filepath.Dir(d)
	}
	t.Fatalf("%s not found above %s", name, wd)
	return ""
}
