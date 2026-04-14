package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	content := "package main\n\nimport (\n\t\"context\"\n\t\"sync\"\n)\n\ntype Service struct {\n\tmu   sync.Mutex\n\tdata map[string]string\n}\n\nfunc (s *Service) Process(ctx context.Context) {\n\tgo func() {\n\t\ts.mu.Lock()\n\t\ts.data[\"key\"] = \"value\"\n\t\ts.mu.Unlock()\n\t}()\n}\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "service.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := ScanConcurrency(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]int{}
	for _, e := range entries {
		types[e.Type]++
	}
	if types["sync.Mutex"] == 0 {
		t.Error("expected to find sync.Mutex usage")
	}
	if types["goroutine"] == 0 {
		t.Error("expected to find goroutine launch")
	}
}

func TestScanConcurrencyExcludesComments(t *testing.T) {
	tmpDir := t.TempDir()
	content := "package main\n\n// go func() { sync.Mutex Lock }()\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := ScanConcurrency(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("comment lines should be excluded, got %d entries", len(entries))
	}
}

func TestScanConcurrencyExcludesTestFiles(t *testing.T) {
	tmpDir := t.TempDir()
	content := "package main\n\nimport \"sync\"\n\nfunc TestMutex(t *testing.T) {\n\tvar mu sync.Mutex\n\tmu.Lock()\n}\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := ScanConcurrency(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("test files should be excluded, got %d entries", len(entries))
	}
}
