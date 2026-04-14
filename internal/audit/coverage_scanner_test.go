package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanCoverageGaps(t *testing.T) {
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "internal", "example")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "service.go"), []byte("package example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "helper.go"), []byte("package example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "service_test.go"), []byte("package example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gaps, err := ScanCoverageGaps(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range gaps {
		if filepath.Base(g.SourceFile) == "service.go" && !g.HasAnyTest {
			t.Error("service.go should have HasAnyTest=true because service_test.go exists in same package")
		}
		if filepath.Base(g.SourceFile) == "helper.go" && !g.HasAnyTest {
			t.Error("helper.go should have HasAnyTest=true because service_test.go exists in same package")
		}
	}
}

func TestScanCoverageGapsFindsUntested(t *testing.T) {
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "internal", "solo")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "alone.go"), []byte("package solo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gaps, err := ScanCoverageGaps(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, g := range gaps {
		if filepath.Base(g.SourceFile) == "alone.go" {
			found = true
			if g.Package != "solo" {
				t.Errorf("Package = %q, want %q", g.Package, "solo")
			}
			if g.HasAnyTest {
				t.Error("alone.go should have HasAnyTest=false because no test file exists")
			}
		}
	}
	if !found {
		t.Error("alone.go should be reported as a coverage gap")
	}
}

func TestScanCoverageGapsExcludesVendor(t *testing.T) {
	tmpDir := t.TempDir()
	vendorDir := filepath.Join(tmpDir, "vendor", "pkg")
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "lib.go"), []byte("package pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gaps, err := ScanCoverageGaps(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 0 {
		t.Error("vendor directory files should be excluded from coverage gaps")
	}
}

func TestScanCoverageGapsExcludesGenerated(t *testing.T) {
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "internal", "proto")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "example.pb.go"), []byte("package proto\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "example_grpc.pb.go"), []byte("package proto\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gaps, err := ScanCoverageGaps(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 0 {
		t.Error(".pb.go files should be excluded")
	}
}

func TestScanCoverageGapsExcludesIgnoredDirs(t *testing.T) {
	tmpDir := t.TempDir()
	ignoredDirs := []string{"vendor", "cli_agents", "external", ".git", "node_modules"}
	for _, dir := range ignoredDirs {
		ignoreDir := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(ignoreDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ignoreDir, "file.go"), []byte("package ignore\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gaps, err := ScanCoverageGaps(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 0 {
		t.Error("files in ignored directories should be excluded")
	}
}
