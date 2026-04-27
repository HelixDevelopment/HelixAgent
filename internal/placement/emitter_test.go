package placement

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestEmitPerHostCompose_KeepsOnlyRequested(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "compose.yml")
	if err := os.WriteFile(src, []byte(`
version: '3.8'
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
      - postgres
      - redis
      - chromadb
  ollama:
    image: ollama/ollama
networks:
  helixagent-network:
    driver: bridge
volumes:
  postgres_data:
    driver: local
  redis_data:
    driver: local
`), 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "thinker.yml")
	if _, err := EmitPerHostCompose(
		src,
		[]string{"postgres", "redis", "chromadb", "cognee"},
		out,
	); err != nil {
		t.Fatalf("EmitPerHostCompose: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read out: %v", err)
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse out: %v", err)
	}

	services, ok := doc["services"].(map[string]interface{})
	if !ok {
		t.Fatalf("services not a map: %T", doc["services"])
	}
	if len(services) != 4 {
		t.Errorf("want 4 services, got %d (%v)", len(services), keysOf(services))
	}
	for _, want := range []string{"postgres", "redis", "chromadb", "cognee"} {
		if _, has := services[want]; !has {
			t.Errorf("missing service %q in output", want)
		}
	}
	if _, has := services["ollama"]; has {
		t.Errorf("ollama should have been dropped")
	}

	// Networks and volumes preserved (they're top-level).
	if _, ok := doc["networks"]; !ok {
		t.Errorf("networks section missing")
	}
	if _, ok := doc["volumes"]; !ok {
		t.Errorf("volumes section missing")
	}
}

func TestEmitPerHostCompose_DependsOnPreservedWithinSlice(t *testing.T) {
	// If a service depends on another that's also in the kept slice,
	// the depends_on stays. We DO NOT strip cross-host dangling
	// references here — by construction, the scheduler's co-location
	// keeps depends_on graphs intact.
	dir := t.TempDir()
	src := filepath.Join(dir, "compose.yml")
	if err := os.WriteFile(src, []byte(`
services:
  a:
    image: a:1
  b:
    image: b:1
    depends_on: [a]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.yml")
	if _, err := EmitPerHostCompose(src, []string{"a", "b"}, out); err != nil {
		t.Fatalf("EmitPerHostCompose: %v", err)
	}
	data, _ := os.ReadFile(out)
	var doc map[string]interface{}
	_ = yaml.Unmarshal(data, &doc)
	services := doc["services"].(map[string]interface{})
	b := services["b"].(map[string]interface{})
	if _, ok := b["depends_on"]; !ok {
		t.Errorf("b.depends_on stripped — expected preserved")
	}
}

func keysOf(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
