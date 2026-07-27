package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAuditCLIHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI test in short mode") // SKIP-OK: #short-mode
	}
	bin := filepath.Join(t.TempDir(), "audit")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = filepath.Join("..", "..", "cmd", "audit")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s: %v", out, err)
	}
	out, err := exec.Command(bin, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("help failed: %s: %v", out, err)
	}
	help := string(out)
	if !containsStr(help, "audit") && !containsStr(help, "root") {
		t.Error("help output should mention audit or root flag")
	}
}

func TestAuditCLIProducesReport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI test in short mode") // SKIP-OK: #short-mode
	}
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main\n// TODO: implement\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(tmpDir, "report.md")
	bin := filepath.Join(t.TempDir(), "audit2")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = filepath.Join("..", "..", "cmd", "audit")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s: %v", out, err)
	}
	cmd := exec.Command(bin, "--root", srcDir, "--output", reportPath)
	cmd.Dir = srcDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run failed: %s: %v", out, err)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	report := string(data)
	if !containsStr(report, "Audit Report") {
		t.Error("report should contain title")
	}
	if !containsStr(report, "implement") {
		t.Error("report should contain TODO message")
	}
}

func TestAuditCLIJSONOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI test in short mode") // SKIP-OK: #short-mode
	}
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(tmpDir, "report.json")
	bin := filepath.Join(t.TempDir(), "audit3")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = filepath.Join("..", "..", "cmd", "audit")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s: %v", out, err)
	}
	cmd := exec.Command(bin, "--root", srcDir, "--output", reportPath, "--format", "json")
	cmd.Dir = srcDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run failed: %s: %v", out, err)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !containsStr(string(data), "timestamp") {
		t.Error("JSON report should contain timestamp field")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || containsStrHelper(s, sub))
}

func containsStrHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
