package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanTodoMarkers(t *testing.T) {
	tmpDir := t.TempDir()
	content := "package main\n\n// TODO: implement error handling\n// FIXME: this crashes on nil input\n// HACK: temporary workaround\n// XXX: security vulnerability\nfunc process() {}\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	markers, err := ScanTodoMarkers(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 4 {
		t.Fatalf("expected 4 markers, got %d", len(markers))
	}
	expected := map[string]bool{"TODO": false, "FIXME": false, "HACK": false, "XXX": false}
	for _, m := range markers {
		expected[m.Marker] = true
		if m.Line == 0 {
			t.Errorf("marker %s should have non-zero line number", m.Marker)
		}
		if m.Severity == "" {
			t.Errorf("marker %s should have classified severity", m.Marker)
		}
	}
	for marker, found := range expected {
		if !found {
			t.Errorf("marker %s not found", marker)
		}
	}
}

func TestScanTodoMarkersExcludesStrings(t *testing.T) {
	tmpDir := t.TempDir()
	content := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"TODO: this is a string, not a real marker\")\n}\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	markers, err := ScanTodoMarkers(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range markers {
		if m.Message == "this is a string, not a real marker" {
			t.Error("markers inside string literals should be excluded")
		}
	}
}

func TestScanTodoMarkersExcludesTestFiles(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte("// TODO: fix test\npackage main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	markers, err := ScanTodoMarkers(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 0 {
		t.Errorf("test file markers should be excluded, got %d", len(markers))
	}
}
