package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDeadCodeFindsUnreachable(t *testing.T) {
	tmpDir := t.TempDir()
	content := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(UsedFunc())\n}\n\nfunc UsedFunc() string {\n\treturn \"used\"\n}\n\nfunc UnusedFunc() string {\n\treturn \"never called\"\n}\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := ScanDeadCode(tmpDir, []string{filepath.Join(tmpDir, "main.go")})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if e.Ident == "UnusedFunc" {
			found = true
			if e.Reachable {
				t.Error("UnusedFunc should be marked unreachable")
			}
		}
	}
	if !found {
		t.Error("UnusedFunc should be detected as dead code")
	}
}

func TestScanDeadCodeMarksReachable(t *testing.T) {
	tmpDir := t.TempDir()
	content := "package main\n\nfunc main() {\n\tHelper()\n}\n\nfunc Helper() string {\n\treturn \"ok\"\n}\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := ScanDeadCode(tmpDir, []string{filepath.Join(tmpDir, "main.go")})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Ident == "Helper" && !e.Reachable {
			t.Error("Helper should be reachable from main()")
		}
	}
}

func TestScanDeadCodeExcludesTestFiles(t *testing.T) {
	tmpDir := t.TempDir()
	src := "package main\n\nfunc main() {}\n\nfunc UsedInTest() string { return \"\" }\n"
	test := "package main\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) { UsedInTest() }\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte(test), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := ScanDeadCode(tmpDir, []string{filepath.Join(tmpDir, "main.go")})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Ident == "TestX" {
			t.Error("test functions should not be scanned")
		}
	}
}
