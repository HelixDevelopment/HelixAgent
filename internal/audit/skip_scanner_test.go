package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanSkippedTests(t *testing.T) {
	tmpDir := t.TempDir()
	content := "package main\n\nimport \"testing\"\n\nfunc TestSomething(t *testing.T) {\n\tt.Skip(\"Docker not available\")\n}\n\nfunc TestOther(t *testing.T) {\n\tt.Skipf(\"Skipping: PostgreSQL not accessible: %v\", err)\n}\n\nfunc TestFlaky(t *testing.T) {\n\tt.Skip(\"Skipping test that involves sleep in short mode\")\n}\n" // SKIP-OK: #short-mode
	if err := os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	skips, err := ScanSkippedTests(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skips) != 3 {
		t.Fatalf("expected 3 skips, got %d", len(skips))
	}
	categories := map[SkipCategory]int{}
	for _, s := range skips {
		categories[s.Category]++
	}
	if categories[CategoryInfrastructure] != 2 {
		t.Errorf("expected 2 infrastructure skips, got %d", categories[CategoryInfrastructure])
	}
	if categories[CategoryFlakyGuard] != 1 {
		t.Errorf("expected 1 flaky-guard skip, got %d", categories[CategoryFlakyGuard])
	}
}

func TestScanSkippedTestsNonTestIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	content := "package main\n\nfunc skipMe() {\n\tt.Skip(\"should not be found\")\n}\n" // SKIP-OK: #legacy-untriaged
	if err := os.WriteFile(filepath.Join(tmpDir, "service.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	skips, err := ScanSkippedTests(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skips) != 0 {
		t.Errorf("non-test files should be excluded, got %d skips", len(skips))
	}
}

func TestScanSkippedTestsExcludesVendor(t *testing.T) {
	tmpDir := t.TempDir()
	vendorDir := filepath.Join(tmpDir, "vendor", "pkg")
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "package pkg\n\nimport \"testing\"\n\nfunc TestVendor(t *testing.T) {\n\tt.Skip(\"Docker not available\")\n}\n" // SKIP-OK: #requires-docker
	if err := os.WriteFile(filepath.Join(vendorDir, "pkg_test.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	skips, err := ScanSkippedTests(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skips) != 0 {
		t.Errorf("vendor files should be excluded, got %d skips", len(skips))
	}
}
