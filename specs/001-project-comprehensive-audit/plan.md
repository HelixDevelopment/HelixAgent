# HelixAgent Comprehensive Audit, Completion & Optimization — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Systematically audit, complete, test, optimize, and document the entire HelixAgent codebase to meet all 29 constitution rules with zero broken, disabled, or undocumented components.

**Architecture:** Phase-gated approach where Phase 1 builds audit CLI tooling (`cmd/audit/`) that produces structured reports driving all subsequent phases. Each phase is independently testable and produces working artifacts. Phases execute in dependency order: audit → dead code → test fixes → concurrency → security → optimization → stress testing → documentation → challenges.

**Tech Stack:** Go 1.25+, existing Gin/HTTP framework, Docker/Podman Compose, Snyk CLI, SonarQube, Prometheus/Grafana, existing Concurrency/ submodule (digital.vasic.concurrency)

---

## File Structure

### New Files Created

```
cmd/audit/
  main.go                          # CLI entry point for audit tool
  main_test.go                     # CLI integration tests
internal/audit/
  coverage_scanner.go              # Finds Go files without corresponding tests
  coverage_scanner_test.go         # Tests for coverage scanner
  todo_scanner.go                  # Categorizes TODO/FIXME/HACK/XXX markers
  todo_scanner_test.go             # Tests for TODO scanner
  skip_scanner.go                  # Classifies t.Skip calls
  skip_scanner_test.go             # Tests for skip scanner
  deadcode_scanner.go              # Checks reachability from entry points
  deadcode_scanner_test.go         # Tests for dead code scanner
  concurrency_scanner.go           # Analyzes sync primitives and goroutines
  concurrency_scanner_test.go      # Tests for concurrency scanner
  report.go                        # Report aggregation and formatting
  report_test.go                   # Tests for report formatting
  types.go                         # Shared types for audit results
reports/audit/                     # Output directory for generated reports
Makefile                           # Modified: add audit targets
```

### Modified Files

```
Makefile                           # Add audit, audit-full, audit-report targets
```

### Phase-Dependent Files (created in later phases)

Each subsequent phase creates/modifies files identified by the Phase 1 audit report. The exact file list is determined dynamically.

---

## Phase 1: Audit Tooling (US1, FR-001, SC-001 through SC-004)

**Entry Criteria:** None (foundation phase)
**Exit Criteria:** `make audit` produces a complete Markdown report at `reports/audit/full-report.md` listing all untested files, TODO markers, skipped tests, dead code, and concurrency hazards.
**Delivers:** Working `cmd/audit/` binary and structured audit reports.

### Task 1.1: Audit Types and Report Structure

**Files:**
- Create: `internal/audit/types.go`
- Create: `internal/audit/types_test.go`

- [ ] **Step 1: Write the failing test for audit types**

Create `internal/audit/types_test.go`:

```go
package audit

import (
	"encoding/json"
	"testing"
)

func TestCoverageGapJSON(t *testing.T) {
	gap := CoverageGap{
		SourceFile:   "internal/cache/semantic.go",
		Package:      "cache",
		HasAnyTest:   false,
		TestFiles:    nil,
		MissingTypes: []string{"SemanticCache"},
	}
	data, err := json.Marshal(gap)
	if err != nil {
		t.Fatalf("marshal CoverageGap: %v", err)
	}
	if string(data) == "" {
		t.Fatal("expected non-empty JSON")
	}
	var parsed CoverageGap
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.SourceFile != gap.SourceFile {
		t.Fatalf("SourceFile mismatch: got %q want %q", parsed.SourceFile, gap.SourceFile)
	}
}

func TestTodoMarkerSeverity(t *testing.T) {
	cases := []struct {
		marker   string
		severity Severity
	}{
		{"TODO", SeverityIncomplete},
		{"FIXME", SeverityBroken},
		{"HACK", SeverityOptimization},
		{"XXX", SeverityBroken},
	}
	for _, tc := range cases {
		m := TodoMarker{Marker: tc.marker}
		m.Classify()
		if m.Severity != tc.severity {
			t.Errorf("Classify(%q) = %v, want %v", tc.marker, m.Severity, tc.severity)
		}
	}
}

func TestSkipClassification(t *testing.T) {
	skip := SkipEntry{
		File:   "cmd/helixagent/main_test.go",
		Line:   100,
		Reason: "ls command not found, skipping test",
	}
	skip.Classify()
	if skip.Category != CategoryInfrastructure {
		t.Fatalf("expected %v, got %v", CategoryInfrastructure, skip.Category)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/audit/ -run "TestCoverageGapJSON|TestTodoMarkerSeverity|TestSkipClassification" -v`
Expected: FAIL — package does not exist

- [ ] **Step 3: Create the types.go implementation**

Create `internal/audit/types.go`:

```go
package audit

type Severity string

const (
	SeverityBroken      Severity = "broken"
	SeverityIncomplete  Severity = "incomplete"
	SeverityOptimization Severity = "optimization"
	SeverityDocs        Severity = "documentation"
)

type SkipCategory string

const (
	CategoryInfrastructure SkipCategory = "infrastructure"
	CategoryFlakyGuard    SkipCategory = "flaky-guard"
	CategoryUnimplemented SkipCategory = "unimplemented"
)

type CoverageGap struct {
	SourceFile   string   `json:"source_file"`
	Package      string   `json:"package"`
	HasAnyTest   bool     `json:"has_any_test"`
	TestFiles    []string `json:"test_files"`
	MissingTypes []string `json:"missing_types"`
}

type TodoMarker struct {
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Marker   string   `json:"marker"`
	Message  string   `json:"message"`
	Severity Severity `json:"severity"`
	Context  string   `json:"context"`
}

func (m *TodoMarker) Classify() {
	switch m.Marker {
	case "FIXME", "XXX":
		m.Severity = SeverityBroken
	case "TODO":
		m.Severity = SeverityIncomplete
	case "HACK":
		m.Severity = SeverityOptimization
	default:
		m.Severity = SeverityDocs
	}
}

type SkipEntry struct {
	File     string       `json:"file"`
	Line     int          `json:"line"`
	Reason   string       `json:"reason"`
	Category SkipCategory `json:"category"`
}

func (s *SkipEntry) Classify() {
	reasons := map[string]SkipCategory{
		"not found":          CategoryInfrastructure,
		"not available":      CategoryInfrastructure,
		"not accessible":     CategoryInfrastructure,
		"short mode":         CategoryFlakyGuard,
		"involves sleep":     CategoryFlakyGuard,
		"integration test":   CategoryInfrastructure,
		"container runtime":  CategoryInfrastructure,
	}
	for substr, cat := range reasons {
		if containsFold(s.Reason, substr) {
			s.Category = cat
			return
		}
	}
	s.Category = CategoryUnimplemented
}

func containsFold(s, substr string) bool {
	sl := len(s)
	subl := len(substr)
	if subl > sl {
		return false
	}
	for i := 0; i <= sl-subl; i++ {
		match := true
		for j := 0; j < subl; j++ {
			sc := s[i+j]
			tc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

type DeadCodeEntry struct {
	File     string `json:"file"`
	Ident    string `json:"ident"`
	Kind     string `json:"kind"`
	Reachable bool  `json:"reachable"`
}

type ConcurrencyEntry struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	Type      string `json:"type"`
	Safe      bool   `json:"safe"`
	Issue     string `json:"issue,omitempty"`
}

type AuditReport struct {
	Timestamp      string            `json:"timestamp"`
	RootDir        string            `json:"root_dir"`
	CoverageGaps   []CoverageGap     `json:"coverage_gaps"`
	TodoMarkers    []TodoMarker      `json:"todo_markers"`
	SkippedTests   []SkipEntry       `json:"skipped_tests"`
	DeadCode       []DeadCodeEntry   `json:"dead_code"`
	Concurrency    []ConcurrencyEntry `json:"concurrency"`
	Summary        AuditSummary      `json:"summary"`
}

type AuditSummary struct {
	TotalSourceFiles    int `json:"total_source_files"`
	TotalTestFiles      int `json:"total_test_files"`
	FilesWithoutTests   int `json:"files_without_tests"`
	TodoMarkerCount     int `json:"todo_marker_count"`
	SkippedTestCount    int `json:"skipped_test_count"`
	DeadCodeCount       int `json:"dead_code_count"`
	ConcurrencyIssues   int `json:"concurrency_issues"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/audit/ -run "TestCoverageGapJSON|TestTodoMarkerSeverity|TestSkipClassification" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/audit/types.go internal/audit/types_test.go
git commit -m "feat(audit): add audit types with coverage gap, TODO, skip, deadcode, concurrency models"
```

---

### Task 1.2: Coverage Gap Scanner

**Files:**
- Create: `internal/audit/coverage_scanner.go`
- Create: `internal/audit/coverage_scanner_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/audit/coverage_scanner_test.go`:

```go
package audit

import (
	"os"
	"path/filepath"
	"sort"
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
	filePaths := make(map[string]bool)
	for _, g := range gaps {
		filePaths[g.SourceFile] = true
	}
	if filePaths[filepath.Join("internal", "example", "helper.go")] {
		t.Error("helper.go should NOT be a gap — it has a sibling test file in the same package")
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
	for _, g := range gaps {
		if g.SourceFile == filepath.Join("vendor", "pkg", "lib.go") {
			t.Error("vendor files should be excluded")
		}
	}
}

func TestScanCoverageGapsExcludesTestFiles(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "gen_test.go"), []byte("package root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gaps, err := ScanCoverageGaps(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range gaps {
		if filepath.Base(g.SourceFile) == "gen_test.go" {
			t.Error("_test.go files should not be scanned as source files")
		}
	}
}

func TestScanCoverageGapsPackageLevel(t *testing.T) {
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "internal", "mixed")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "a.go"), []byte("package mixed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "b.go"), []byte("package mixed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "a_test.go"), []byte("package mixed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gaps, err := ScanCoverageGaps(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(gaps)
	for _, g := range gaps {
		if filepath.Base(g.SourceFile) == "b.go" && g.HasAnyTest {
			return
		}
	}
	for _, g := range gaps {
		if filepath.Base(g.SourceFile) == "b.go" {
			if !g.HasAnyTest {
				t.Error("b.go should have HasAnyTest=true because a_test.go exists in same package")
			}
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/audit/ -run TestScanCoverage -v`
Expected: FAIL — `ScanCoverageGaps` undefined

- [ ] **Step 3: Write the coverage scanner implementation**

Create `internal/audit/coverage_scanner.go`:

```go
package audit

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func ScanCoverageGaps(rootDir string) ([]CoverageGap, error) {
	var gaps []CoverageGap
	pkgHasTests := map[string]bool{}

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			return nil
		}
		if shouldExclude(rel) {
			return nil
		}
		if strings.HasSuffix(rel, "_test.go") {
			pkg := filepath.Dir(rel)
			pkgHasTests[pkg] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			return nil
		}
		if shouldExclude(rel) {
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		if strings.HasSuffix(rel, ".pb.go") || strings.HasSuffix(rel, "_grpc.pb.go") {
			return nil
		}
		pkg := filepath.Dir(rel)
		gap := CoverageGap{
			SourceFile: rel,
			Package:    filepath.Base(pkg),
			HasAnyTest: pkgHasTests[pkg],
		}
		gaps = append(gaps, gap)
		return nil
	})
	return gaps, err
}

func shouldExclude(rel string) bool {
	excludePrefixes := []string{
		"vendor" + string(os.PathSeparator),
		"cli_agents" + string(os.PathSeparator),
		"external" + string(os.PathSeparator),
		".git" + string(os.PathSeparator),
		"node_modules" + string(os.PathSeparator),
	}
	for _, prefix := range excludePrefixes {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/audit/ -run TestScanCoverage -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/audit/coverage_scanner.go internal/audit/coverage_scanner_test.go
git commit -m "feat(audit): add coverage gap scanner that finds Go files without package-level tests"
```

---

### Task 1.3: TODO/FIXME Marker Scanner

**Files:**
- Create: `internal/audit/todo_scanner.go`
- Create: `internal/audit/todo_scanner_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/audit/todo_scanner_test.go`:

```go
package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanTodoMarkers(t *testing.T) {
	tmpDir := t.TempDir()
	content := `package main

// TODO: implement error handling
// FIXME: this crashes on nil input
// HACK: temporary workaround for race condition
// XXX: security vulnerability here
func process() {}
`
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
	expected := map[string]bool{
		"TODO": false, "FIXME": false, "HACK": false, "XXX": false,
	}
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
	content := `package main

import "fmt"

func main() {
	fmt.Println("TODO: this is a string, not a real marker")
}
`
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
	for _, m := range markers {
		t.Errorf("test file markers should be excluded but found %s at %s:%d", m.Marker, m.File, m.Line)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/audit/ -run TestScanTodo -v`
Expected: FAIL — `ScanTodoMarkers` undefined

- [ ] **Step 3: Write the TODO scanner implementation**

Create `internal/audit/todo_scanner.go`:

```go
package audit

import (
	"bufio"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
)

var markerPattern = regexp.MustCompile(`^//\s*(TODO|FIXME|HACK|XXX)[:\s](.*)`)

func ScanTodoMarkers(rootDir string) ([]TodoMarker, error) {
	var markers []TodoMarker
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() || shouldExclude(rel) {
			return nil
		}
		if !strings.HasSuffix(rel, ".go") {
			return nil
		}
		if strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		fileMarkers, parseErr := parseTodoFile(path, rel)
		if parseErr != nil {
			return parseErr
		}
		markers = append(markers, fileMarkers...)
		return nil
	})
	return markers, err
}

func parseTodoFile(path, rel string) ([]TodoMarker, error) {
	f, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	file, err := filepath.Glob(f)
	if err != nil || len(file) == 0 {
		fd, openErr := openFile(f)
		if openErr != nil {
			return nil, openErr
		}
		defer fd.Close()
		return scanLines(fd, rel)
	}
	fd, err := openFile(file[0])
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	return scanLines(fd, rel)
}

func openFile(path string) (interface{ Read(p []byte) (int, error) }, error) {
	return nil, nil
}

func scanLines(scanner interface{ Read(p []byte) (int, error) }, rel string) ([]TodoMarker, error) {
	return nil, nil
}

func init() {
	openFile = func(path string) (*bufio.Reader, error) {
		fd, err := filepath.Abs(path)
		_ = fd
		return nil, err
	}
}
```

Wait — the above has issues. Let me rewrite this properly.

Rewrite `internal/audit/todo_scanner.go`:

```go
package audit

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func ScanTodoMarkers(rootDir string) ([]TodoMarker, error) {
	var markers []TodoMarker
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() || shouldExclude(rel) {
			return nil
		}
		if !strings.HasSuffix(rel, ".go") {
			return nil
		}
		if strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		fileMarkers, parseErr := parseTodoFile(path, rel)
		if parseErr != nil {
			return parseErr
		}
		markers = append(markers, fileMarkers...)
		return nil
	})
	return markers, err
}

func parseTodoFile(absPath, relPath string) ([]TodoMarker, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var markers []TodoMarker
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "//") {
			continue
		}
		comment := strings.TrimPrefix(trimmed, "//")
		comment = strings.TrimSpace(comment)
		marker, msg := extractMarker(comment)
		if marker == "" {
			continue
		}
		m := TodoMarker{
			File:    relPath,
			Line:    lineNum,
			Marker:  marker,
			Message: msg,
			Context: trimmed,
		}
		m.Classify()
		markers = append(markers, m)
	}
	return markers, scanner.Err()
}

func extractMarker(comment string) (marker, message string) {
	for _, m := range []string{"TODO", "FIXME", "HACK", "XXX"} {
		if strings.HasPrefix(comment, m) {
			rest := strings.TrimSpace(strings.TrimPrefix(comment, m))
			rest = strings.TrimPrefix(rest, ":")
			rest = strings.TrimPrefix(rest, " ")
			return m, rest
		}
	}
	return "", ""
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/audit/ -run TestScanTodo -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/audit/todo_scanner.go internal/audit/todo_scanner_test.go
git commit -m "feat(audit): add TODO/FIXME/HACK/XXX marker scanner with severity classification"
```

---

### Task 1.4: Skipped Test Scanner

**Files:**
- Create: `internal/audit/skip_scanner.go`
- Create: `internal/audit/skip_scanner_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/audit/skip_scanner_test.go`:

```go
package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanSkippedTests(t *testing.T) {
	tmpDir := t.TempDir()
	content := `package main

import "testing"

func TestSomething(t *testing.T) {
	t.Skip("Docker not available")
}

func TestOther(t *testing.T) {
	t.Skipf("Skipping: PostgreSQL not accessible: %v", err)
}

func TestFlaky(t *testing.T) {
	t.Skip("Skipping test that involves sleep in short mode")
}
`
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
	content := `package main

func skipMe() {
	t.Skip("should not be found")
}
`
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/audit/ -run TestScanSkipped -v`
Expected: FAIL — `ScanSkippedTests` undefined

- [ ] **Step 3: Write the skip scanner implementation**

Create `internal/audit/skip_scanner.go`:

```go
package audit

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func ScanSkippedTests(rootDir string) ([]SkipEntry, error) {
	var skips []SkipEntry
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() || shouldExclude(rel) {
			return nil
		}
		if !strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		fileSkips, parseErr := parseSkipFile(path, rel)
		if parseErr != nil {
			return parseErr
		}
		skips = append(skips, fileSkips...)
		return nil
	})
	return skips, err
}

func parseSkipFile(absPath, relPath string) ([]SkipEntry, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var skips []SkipEntry
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		reason := extractSkipReason(line)
		if reason == "" {
			continue
		}
		s := SkipEntry{
			File:   relPath,
			Line:   lineNum,
			Reason: reason,
		}
		s.Classify()
		skips = append(skips, s)
	}
	return skips, scanner.Err()
}

func extractSkipReason(line string) string {
	trimmed := strings.TrimSpace(line)
	for _, prefix := range []string{"t.Skip(\"", "t.Skipf(\""} {
		if strings.Contains(trimmed, prefix) {
			start := strings.Index(trimmed, prefix) + len(prefix)
			end := strings.Index(trimmed[start:], "\"")
			if end < 0 {
				end = strings.Index(trimmed[start:], ",")
			}
			if end < 0 {
				return trimmed[start:]
			}
			return trimmed[start : start+end]
		}
	}
	return ""
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/audit/ -run TestScanSkipped -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/audit/skip_scanner.go internal/audit/skip_scanner_test.go
git commit -m "feat(audit): add skipped test scanner with infrastructure/flaky/unimplemented classification"
```

---

### Task 1.5: Concurrency Hazard Scanner

**Files:**
- Create: `internal/audit/concurrency_scanner.go`
- Create: `internal/audit/concurrency_scanner_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/audit/concurrency_scanner_test.go`:

```go
package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	content := `package main

import (
	"context"
	"sync"
)

type Service struct {
	mu sync.Mutex
	data map[string]string
}

func (s *Service) Process(ctx context.Context) {
	go func() {
		s.mu.Lock()
		s.data["key"] = "value"
		s.mu.Unlock()
	}()
}
`
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/audit/ -run TestScanConcurrency -v`
Expected: FAIL

- [ ] **Step 3: Write the concurrency scanner implementation**

Create `internal/audit/concurrency_scanner.go`:

```go
package audit

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func ScanConcurrency(rootDir string) ([]ConcurrencyEntry, error) {
	var entries []ConcurrencyEntry
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() || shouldExclude(rel) {
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		fileEntries, parseErr := parseConcurrencyFile(path, rel)
		if parseErr != nil {
			return parseErr
		}
		entries = append(entries, fileEntries...)
		return nil
	})
	return entries, err
}

func parseConcurrencyFile(absPath, relPath string) ([]ConcurrencyEntry, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var entries []ConcurrencyEntry
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if entry := detectConcurrencyPattern(line, relPath, lineNum); entry != nil {
			entries = append(entries, *entry)
		}
	}
	return entries, scanner.Err()
}

func detectConcurrencyPattern(line, file string, lineNum int) *ConcurrencyEntry {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "//") {
		return nil
	}
	patterns := []struct {
		keyword string
		kind    string
	}{
		{"sync.Mutex", "sync.Mutex"},
		{"sync.RWMutex", "sync.RWMutex"},
		{"sync.WaitGroup", "sync.WaitGroup"},
		{"sync.Once", "sync.Once"},
		{"sync.Pool", "sync.Pool"},
		{"sync.Map", "sync.Map"},
		{"sync.Cond", "sync.Cond"},
		{"go func", "goroutine"},
		{" go ", "goroutine-call"},
	}
	for _, p := range patterns {
		if strings.Contains(line, p.keyword) {
			return &ConcurrencyEntry{
				File: file,
				Line: lineNum,
				Type: p.kind,
				Safe: true,
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/audit/ -run TestScanConcurrency -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/audit/concurrency_scanner.go internal/audit/concurrency_scanner_test.go
git commit -m "feat(audit): add concurrency primitive scanner detecting mutex, goroutine, sync patterns"
```

---

### Task 1.6: Dead Code Scanner

**Files:**
- Create: `internal/audit/deadcode_scanner.go`
- Create: `internal/audit/deadcode_scanner_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/audit/deadcode_scanner_test.go`:

```go
package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDeadCodeFindsUnreachable(t *testing.T) {
	tmpDir := t.TempDir()
	entryPoint := `package main

import "fmt"

func main() {
	fmt.Println(UsedFunc())
}

func UsedFunc() string {
	return "used"
}

func UnusedFunc() string {
	return "never called"
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(entryPoint), 0o644); err != nil {
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
	entryPoint := `package main

func main() {
	Helper()
}

func Helper() string {
	return "ok"
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(entryPoint), 0o644); err != nil {
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/audit/ -run TestScanDeadCode -v`
Expected: FAIL

- [ ] **Step 3: Write the dead code scanner implementation**

Create `internal/audit/deadcode_scanner.go`:

```go
package audit

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var funcDeclPattern = regexp.MustCompile(`^func\s+(?:\(\w+\s+\*?\w+\)\s+)?(\w+)\s*\(`)
var funcCallPattern = regexp.MustCompile(`(\w+)\s*\(`)

func ScanDeadCode(rootDir string, entryPoints []string) ([]DeadCodeEntry, error) {
	allFuncs := map[string]*DeadCodeEntry{}
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() || shouldExclude(rel) {
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		return extractFuncDecls(path, rel, allFuncs)
	})
	if err != nil {
		return nil, err
	}

	referenced := map[string]bool{}
	for _, ep := range entryPoints {
		refs, refErr := extractReferences(ep)
		if refErr != nil {
			continue
		}
		for name := range refs {
			referenced[name] = true
		}
	}

	changed := true
	for changed {
		changed = false
		for name := range referenced {
			entry := allFuncs[name]
			if entry == nil {
				continue
			}
			absPath := filepath.Join(rootDir, entry.File)
			refs, refErr := extractReferences(absPath)
			if refErr != nil {
				continue
			}
			for refName := range refs {
				if !referenced[refName] {
					referenced[refName] = true
					changed = true
				}
			}
		}
	}

	var results []DeadCodeEntry
	for name, entry := range allFuncs {
		entry.Reachable = referenced[name]
		entry.Ident = name
		results = append(results, *entry)
	}
	return results, nil
}

func extractFuncDecls(absPath, relPath string, funcs map[string]*DeadCodeEntry) error {
	f, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		matches := funcDeclPattern.FindStringSubmatch(line)
		if len(matches) > 1 {
			name := matches[1]
			funcs[name] = &DeadCodeEntry{
				File: relPath,
				Kind: "function",
			}
		}
	}
	return scanner.Err()
}

func extractReferences(absPath string) (map[string]bool, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	refs := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		matches := funcCallPattern.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			if len(m) > 1 {
				refs[m[1]] = true
			}
		}
	}
	return refs, scanner.Err()
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/audit/ -run TestScanDeadCode -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/audit/deadcode_scanner.go internal/audit/deadcode_scanner_test.go
git commit -m "feat(audit): add dead code scanner with transitive reachability from entry points"
```

---

### Task 1.7: Report Aggregator

**Files:**
- Create: `internal/audit/report.go`
- Create: `internal/audit/report_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/audit/report_test.go`:

```go
package audit

import (
	"strings"
	"testing"
)

func TestGenerateReportMarkdown(t *testing.T) {
	report := &AuditReport{
		Timestamp: "2026-04-14",
		RootDir:   "/project",
		CoverageGaps: []CoverageGap{
			{SourceFile: "internal/cache/semantic.go", Package: "cache", HasAnyTest: false},
		},
		TodoMarkers: []TodoMarker{
			{File: "main.go", Line: 10, Marker: "TODO", Message: "implement", Severity: SeverityIncomplete},
		},
		SkippedTests: []SkipEntry{
			{File: "main_test.go", Line: 5, Reason: "Docker not available", Category: CategoryInfrastructure},
		},
		Summary: AuditSummary{
			TotalSourceFiles:  100,
			FilesWithoutTests: 50,
			TodoMarkerCount:   1,
			SkippedTestCount:  1,
		},
	}
	md := GenerateReportMarkdown(report)
	if !strings.Contains(md, "# HelixAgent Audit Report") {
		t.Error("report should contain title")
	}
	if !strings.Contains(md, "Files Without Tests") {
		t.Error("report should contain coverage gap section")
	}
	if !strings.Contains(md, "TODO/FIXME Markers") {
		t.Error("report should contain TODO section")
	}
	if !strings.Contains(md, "Skipped Tests") {
		t.Error("report should contain skip section")
	}
	if !strings.Contains(md, "semantic.go") {
		t.Error("report should list specific file")
	}
}

func TestGenerateReportSummary(t *testing.T) {
	report := &AuditReport{
		Summary: AuditSummary{
			TotalSourceFiles:  4689,
			FilesWithoutTests: 2328,
			TodoMarkerCount:   6166,
			SkippedTestCount:  2891,
		},
	}
	md := GenerateReportMarkdown(report)
	if !strings.Contains(md, "4689") {
		t.Error("summary should contain total source file count")
	}
	if !strings.Contains(md, "2328") {
		t.Error("summary should contain untested file count")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/audit/ -run TestGenerateReport -v`
Expected: FAIL

- [ ] **Step 3: Write the report generator implementation**

Create `internal/audit/report.go`:

```go
package audit

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func GenerateReportMarkdown(report *AuditReport) string {
	var b strings.Builder
	b.WriteString("# HelixAgent Audit Report\n\n")
	b.WriteString(fmt.Sprintf("**Generated:** %s\n", report.Timestamp))
	b.WriteString(fmt.Sprintf("**Root:** `%s`\n\n", report.RootDir))
	b.WriteString("## Summary\n\n")
	b.WriteString("| Metric | Count |\n|--------|-------|\n")
	b.WriteString(fmt.Sprintf("| Total Source Files | %d |\n", report.Summary.TotalSourceFiles))
	b.WriteString(fmt.Sprintf("| Files Without Tests | %d |\n", report.Summary.FilesWithoutTests))
	b.WriteString(fmt.Sprintf("| TODO/FIXME Markers | %d |\n", report.Summary.TodoMarkerCount))
	b.WriteString(fmt.Sprintf("| Skipped Tests | %d |\n", report.Summary.SkippedTestCount))
	b.WriteString(fmt.Sprintf("| Dead Code Entries | %d |\n", report.Summary.DeadCodeCount))
	b.WriteString(fmt.Sprintf("| Concurrency Issues | %d |\n", report.Summary.ConcurrencyIssues))
	b.WriteString("\n")

	if len(report.CoverageGaps) > 0 {
		b.WriteString("## Files Without Tests\n\n")
		b.WriteString("| Package | File | Has Sibling Tests |\n|---------|------|-------------------|\n")
		sort.Slice(report.CoverageGaps, func(i, j int) bool {
			return report.CoverageGaps[i].SourceFile < report.CoverageGaps[j].SourceFile
		})
		for _, g := range report.CoverageGaps {
			if !g.HasAnyTest {
				b.WriteString(fmt.Sprintf("| %s | `%s` | %v |\n", g.Package, filepath.Base(g.SourceFile), g.HasAnyTest))
			}
		}
		b.WriteString("\n")
	}

	if len(report.TodoMarkers) > 0 {
		b.WriteString("## TODO/FIXME Markers\n\n")
		b.WriteString("| Severity | Marker | File:Line | Message |\n|----------|--------|-----------|--------|\n")
		sort.Slice(report.TodoMarkers, func(i, j int) bool {
			return report.TodoMarkers[i].File < report.TodoMarkers[j].File
		})
		for _, m := range report.TodoMarkers {
			b.WriteString(fmt.Sprintf("| %s | %s | `%s:%d` | %s |\n", m.Severity, m.Marker, m.File, m.Line, m.Message))
		}
		b.WriteString("\n")
	}

	if len(report.SkippedTests) > 0 {
		b.WriteString("## Skipped Tests\n\n")
		b.WriteString("| Category | File:Line | Reason |\n|----------|-----------|--------|\n")
		sort.Slice(report.SkippedTests, func(i, j int) bool {
			return report.SkippedTests[i].File < report.SkippedTests[j].File
		})
		for _, s := range report.SkippedTests {
			b.WriteString(fmt.Sprintf("| %s | `%s:%d` | %s |\n", s.Category, s.File, s.Line, s.Reason))
		}
		b.WriteString("\n")
	}

	if len(report.DeadCode) > 0 {
		b.WriteString("## Dead Code\n\n")
		b.WriteString("| File | Identifier | Reachable |\n|------|------------|----------|\n")
		for _, d := range report.DeadCode {
			if !d.Reachable {
				b.WriteString(fmt.Sprintf("| `%s` | %s | %v |\n", d.File, d.Ident, d.Reachable))
			}
		}
		b.WriteString("\n")
	}

	if len(report.Concurrency) > 0 {
		b.WriteString("## Concurrency Analysis\n\n")
		b.WriteString("| Type | Count |\n|------|-------|\n")
		typeCounts := map[string]int{}
		for _, c := range report.Concurrency {
			typeCounts[c.Type]++
		}
		for typ, count := range typeCounts {
			b.WriteString(fmt.Sprintf("| %s | %d |\n", typ, count))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func NewAuditReport(rootDir string) *AuditReport {
	return &AuditReport{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		RootDir:   rootDir,
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/audit/ -run TestGenerateReport -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/audit/report.go internal/audit/report_test.go
git commit -m "feat(audit): add Markdown report generator with summary, coverage, TODO, skip sections"
```

---

### Task 1.8: Audit CLI Entry Point

**Files:**
- Create: `cmd/audit/main.go`
- Create: `cmd/audit/main_test.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/audit/main_test.go`:

```go
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAuditCLIHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI test in short mode")
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
	if !containsStr(help, "audit") {
		t.Error("help output should mention audit")
	}
}

func TestAuditCLIProducesReport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI test in short mode")
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/audit/ -v -timeout 60s`
Expected: FAIL — directory does not exist

- [ ] **Step 3: Write the CLI implementation**

Create `cmd/audit/main.go`:

```go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dev.helix.agent/internal/audit"
)

func main() {
	var (
		rootDir   string
		output    string
		format    string
		scanners  string
	)
	flag.StringVar(&rootDir, "root", ".", "Root directory to audit")
	flag.StringVar(&output, "output", "", "Output file path (default: stdout)")
	flag.StringVar(&format, "format", "markdown", "Output format: markdown, json")
	flag.StringVar(&scanners, "scanners", "all", "Scanners to run: all,coverage,todo,skip,deadcode,concurrency (comma-separated)")
	flag.Parse()

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve root: %v\n", err)
		os.Exit(1)
	}

	report := audit.NewAuditReport(absRoot)
	active := parseScanners(scanners)

	if active["coverage"] || active["all"] {
		gaps, err := audit.ScanCoverageGaps(absRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "coverage scan: %v\n", err)
			os.Exit(1)
		}
		report.CoverageGaps = gaps
	}

	if active["todo"] || active["all"] {
		markers, err := audit.ScanTodoMarkers(absRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "TODO scan: %v\n", err)
			os.Exit(1)
		}
		report.TodoMarkers = markers
	}

	if active["skip"] || active["all"] {
		skips, err := audit.ScanSkippedTests(absRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip scan: %v\n", err)
			os.Exit(1)
		}
		report.SkippedTests = skips
	}

	if active["deadcode"] || active["all"] {
		entryPoints := findEntryPoints(absRoot)
		dead, err := audit.ScanDeadCode(absRoot, entryPoints)
		if err != nil {
			fmt.Fprintf(os.Stderr, "deadcode scan: %v\n", err)
			os.Exit(1)
		}
		report.DeadCode = dead
	}

	if active["concurrency"] || active["all"] {
		conc, err := audit.ScanConcurrency(absRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "concurrency scan: %v\n", err)
			os.Exit(1)
		}
		report.Concurrency = conc
	}

	report.Summary = audit.AuditSummary{
		TotalSourceFiles:    countSourceFiles(report),
		TotalTestFiles:      countTestFiles(report),
		FilesWithoutTests:   countGaps(report),
		TodoMarkerCount:     len(report.TodoMarkers),
		SkippedTestCount:    len(report.SkippedTests),
		DeadCodeCount:       countDead(report),
		ConcurrencyIssues:   len(report.Concurrency),
	}

	var result string
	switch format {
	case "json":
		data, _ := json.MarshalIndent(report, "", "  ")
		result = string(data)
	default:
		result = audit.GenerateReportMarkdown(report)
	}

	if output != "" {
		if err := os.WriteFile(output, []byte(result), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write output: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Report written to %s\n", output)
	} else {
		fmt.Println(result)
	}
}

func parseScanners(s string) map[string]bool {
	result := map[string]bool{}
	for _, sc := range strings.Split(s, ",") {
		result[strings.TrimSpace(sc)] = true
	}
	return result
}

func findEntryPoints(root string) []string {
	var entries []string
	cmdDir := filepath.Join(root, "cmd")
	filepath.WalkDir(cmdDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Base(path) == "main.go" {
			entries = append(entries, path)
		}
		return nil
	})
	return entries
}

func countSourceFiles(r *audit.AuditReport) int {
	seen := map[string]bool{}
	for _, g := range r.CoverageGaps {
		seen[g.SourceFile] = true
	}
	return len(seen)
}

func countTestFiles(r *audit.AuditReport) int {
	return 0
}

func countGaps(r *audit.AuditReport) int {
	count := 0
	for _, g := range r.CoverageGaps {
		if !g.HasAnyTest {
			count++
		}
	}
	return count
}

func countDead(r *audit.AuditReport) int {
	count := 0
	for _, d := range r.DeadCode {
		if !d.Reachable {
			count++
		}
	}
	return count
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./cmd/audit/ -v -timeout 60s`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/audit/main.go cmd/audit/main_test.go
git commit -m "feat(audit): add CLI entry point for audit tool with multi-scanner support"
```

---

### Task 1.9: Makefile Targets and Integration

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add audit targets to Makefile**

Append to Makefile after the existing test targets:

```makefile
# =============================================================================
# AUDIT TARGETS
# =============================================================================

audit:
	@echo "🔍 Running full codebase audit..."
	@mkdir -p reports/audit
	go run ./cmd/audit --root . --output reports/audit/full-report.md

audit-json:
	@echo "🔍 Running full codebase audit (JSON)..."
	@mkdir -p reports/audit
	go run ./cmd/audit --root . --output reports/audit/full-report.json --format json

audit-coverage:
	@echo "🔍 Scanning for coverage gaps..."
	go run ./cmd/audit --root . --scanners coverage

audit-todo:
	@echo "🔍 Scanning for TODO/FIXME markers..."
	go run ./cmd/audit --root . --scanners todo

audit-skip:
	@echo "🔍 Scanning for skipped tests..."
	go run ./cmd/audit --root . --scanners skip

audit-deadcode:
	@echo "🔍 Scanning for dead code..."
	go run ./cmd/audit --root . --scanners deadcode

audit-concurrency:
	@echo "🔍 Scanning for concurrency hazards..."
	go run ./cmd/audit --root . --scanners concurrency

.PHONY: audit audit-json audit-coverage audit-todo audit-skip audit-deadcode audit-concurrency
```

- [ ] **Step 2: Verify Makefile parses correctly**

Run: `make -n audit`
Expected: No errors; prints the commands without executing

- [ ] **Step 3: Run audit against the actual codebase**

Run: `make audit 2>&1 | tail -20`
Expected: Report generated at `reports/audit/full-report.md`

- [ ] **Step 4: Commit**

```bash
git add Makefile reports/audit/full-report.md
git commit -m "feat(audit): add Makefile targets for audit tooling and initial full audit report"
```

---

## Phase 2: Dead Code Elimination (US1, FR-021, SC-004)

**Entry Criteria:** Phase 1 audit report (`reports/audit/full-report.md`) generated with dead code section populated.
**Exit Criteria:** Zero dead code entries remain in a fresh audit run. All removed code verified not to break any tests.
**Delivers:** Cleaned codebase with every function reachable from at least one entry point.

### Approach

1. Run `make audit-deadcode` to get the dead code list
2. For each dead code entry, classify as:
   - **Remove**: Truly dead — no tests reference it, no config references it, no documentation references it
   - **Connect**: Has tests but wasn't wired up — connect to an entry point via handler, service, or CLI command
   - **Document**: Intentionally kept for future use — add documentation and a constitution exception
3. For each removal:
   - Run `make test` to verify nothing breaks
   - Commit with message referencing the dead code entry
4. Re-run `make audit-deadcode` to verify zero dead code

### Task Pattern (repeated per dead code cluster)

- [ ] **Step 1:** Identify dead code cluster from audit report
- [ ] **Step 2:** Check if any test file references the code
- [ ] **Step 3:** Check if any config, handler, or service references the code
- [ ] **Step 4:** Remove the dead code (or connect it if still needed)
- [ ] **Step 5:** Run `make test` to verify no breakage
- [ ] **Step 6:** Commit

---

## Phase 3: Skipped Test Resolution (US2, FR-003, SC-003)

**Entry Criteria:** Phase 1 audit report with skip classification populated.
**Exit Criteria:** `make audit-skip` reports zero skips. All tests run reliably.
**Delivers:** Zero skipped tests across the entire codebase.

### Approach

1. Run `make audit-skip` to get the classified skip list
2. For each skip category, apply the resolution pattern:

**CategoryInfrastructure (valid skips):**
- Replace with infrastructure availability check that uses `testcontainers` or `make test-infra-start` 
- Create a test helper at `tests/testutils/infra.go` that ensures infrastructure is available
- Each test calls the helper; if infra is missing, the test fails with a clear error (not a skip)

**CategoryFlakyGuard (needs fix):**
- Remove the sleep/timeout dependency
- Use deterministic test patterns (channels, sync primitives) instead
- Add retry logic within the test if needed

**CategoryUnimplemented (must implement):**
- Write the actual test implementation
- Use TDD: write test first, verify it fails, implement, verify it passes

### Key Files

- Create: `tests/testutils/infra.go` — Infrastructure availability helpers
- Modify: All `*_test.go` files containing `t.Skip` calls (exact list from audit report)

### Task Pattern (repeated per skip cluster)

- [ ] **Step 1:** Classify the skip using audit report
- [ ] **Step 2:** Write the replacement test that doesn't skip
- [ ] **Step 3:** Run the test to verify it works
- [ ] **Step 4:** Run `make test` to verify no regression
- [ ] **Step 5:** Commit

---

## Phase 4: Missing Test Coverage (US2, FR-002, FR-004, SC-001, SC-009)

**Entry Criteria:** Phase 1 audit report with coverage gaps populated. Phase 3 (skip resolution) complete.
**Exit Criteria:** `make audit-coverage` reports zero gaps. `make test-all` passes with 100% coverage.
**Delivers:** Complete test suites for all source files.

### Approach

1. Run `make audit-coverage` to get the coverage gap list
2. Prioritize by module:
   - `internal/llm/providers/helixllm/` and `internal/llm/providers/kimicode/` (FR-004)
   - `internal/cache/` (untested semantic.go)
   - `internal/database/` (doc.go and repository files)
   - `internal/config/` (loader files)
   - Remaining packages by alphabetical order
3. For each package, create test files following TDD:
   - Unit tests for every exported function
   - Integration tests where database/Redis dependencies exist
   - The existing test pattern in the codebase uses `testify/assert` and standard `testing`

### Key Files

- Create: `internal/llm/providers/helixllm/helixllm_test.go`
- Create: `internal/llm/providers/kimicode/kimicode_test.go`
- Create: Tests for all packages identified in audit report
- Verify: All 17 QA banks (`qa-banks/*.yaml`) pass
- Verify: All 3 test banks (`test_banks/*.yaml`) pass

### Task Pattern (repeated per package)

- [ ] **Step 1:** List exported functions in the package
- [ ] **Step 2:** Write failing tests for each exported function
- [ ] **Step 3:** Run tests to verify they fail
- [ ] **Step 4:** Write minimal implementation to pass
- [ ] **Step 5:** Run tests to verify they pass
- [ ] **Step 6:** Run `make test` to verify no regression
- [ ] **Step 7:** Commit

---

## Phase 5: Concurrency Safety (US3, FR-011-013, SC-005-006)

**Entry Criteria:** Phase 1 audit report with concurrency analysis. Phase 4 test coverage complete.
**Exit Criteria:** `go test -race ./...` passes with zero races. 30-minute stress test shows stable memory.
**Delivers:** Race-free, deadlock-free, leak-free codebase.

### Approach

1. Run `go test -race ./...` to find actual race conditions
2. Fix each race using these patterns:
   - Add proper mutex protection where data is shared
   - Use channels instead of shared memory where appropriate
   - Ensure context cancellation propagates to all goroutines
   - Add `defer Close()` for all resources
3. Run `go vet ./...` for additional static analysis
4. Run extended stress test suite (30+ minutes) with memory profiling

### Key Tools

- `go test -race ./...` — Built-in race detector
- `go tool pprof` — Memory profiling during stress tests
- Existing: `Concurrency/pkg/lazyloader/`, `Concurrency/pkg/semaphore/`, `Concurrency/pkg/pool/`

### Task Pattern

- [ ] **Step 1:** Run `go test -race ./...` and capture output
- [ ] **Step 2:** For each race detected, identify the shared variable
- [ ] **Step 3:** Add proper synchronization (mutex, channel, or atomic)
- [ ] **Step 4:** Write a regression test that would fail without the fix
- [ ] **Step 5:** Run `go test -race ./...` to verify fix
- [ ] **Step 6:** Commit

---

## Phase 6: Security Scanning (US4, FR-005-007, SC-007-008)

**Entry Criteria:** Docker/Podman available. Snyk token (`SNYK_TOKEN`) and SonarQube token (`SONAR_TOKEN`) configured.
**Exit Criteria:** All scans pass with zero critical/high/medium findings.
**Delivers:** Verified security posture.

### Approach

1. Start SonarQube via Docker Compose:
   ```bash
   docker compose -f docker/security/sonarqube/docker-compose.yml up -d
   ```
2. Wait for SonarQube health check to pass
3. Run SonarQube scanner:
   ```bash
   docker compose -f docker/security/sonarqube/docker-compose.yml --profile scanner up
   ```
4. Start Snyk scans:
   ```bash
   docker compose -f docker/security/snyk/docker-compose.yml --profile all up
   ```
5. Analyze findings and resolve each one
6. Run `make security-scan` (existing target) for gosec, semgrep, trivy

### Key Files

- Verify: `docker/security/sonarqube/docker-compose.yml` (already exists)
- Verify: `docker/security/snyk/docker-compose.yml` (already exists)
- Verify: `sonar-project.properties` (already exists)
- Verify: `.snyk` (already exists)
- Verify: `.gosec-baseline.json` (already exists)
- Verify: `.gosec.yml` (already exists)

### Task Pattern

- [ ] **Step 1:** Start scanning infrastructure
- [ ] **Step 2:** Run scan
- [ ] **Step 3:** Analyze findings
- [ ] **Step 4:** Fix or document each finding
- [ ] **Step 5:** Re-run scan to verify zero findings
- [ ] **Step 6:** Commit

---

## Phase 7: Lazy Loading & Non-Blocking Optimization (US5, FR-008-010, SC-010)

**Entry Criteria:** Phase 5 (concurrency safety) complete. Phase 6 (security) complete.
**Exit Criteria:** API p99 response time < 500ms under 1,000 concurrent requests. No head-of-line blocking.
**Delivers:** Optimized initialization and request handling.

### Approach

1. Apply existing `Concurrency/pkg/lazyloader/` pattern to:
   - LLM provider client initialization in `internal/llm/providers/`
   - Database connection pool initialization in `internal/database/`
   - Cache service initialization in `internal/cache/`
   - MCP server connections in `internal/mcp/`
2. Apply existing `Concurrency/pkg/semaphore/` pattern to:
   - LLM API call concurrency (already partially in `internal/concurrency/`)
   - Database query concurrency
   - Debate round execution
3. Implement non-blocking patterns for API handlers:
   - Use buffered channels for request dispatch
   - Use semaphore-based backpressure for resource-intensive operations
   - Use async cache population

### Key Existing Patterns

```go
// From Concurrency/pkg/lazyloader/lazyloader.go
func New(totalSize, chunkSize int, loadFn func(index int) (string, error)) *LazyLoader

// From Concurrency/pkg/semaphore/semaphore.go  
func NewSemaphore(max int) *Semaphore
func (s *Semaphore) Acquire(ctx context.Context) error

// From internal/concurrency/worker_pool.go
func NewWorkerPool(maxWorkers int) *WorkerPool
```

### Task Pattern

- [ ] **Step 1:** Identify eager initialization in target file
- [ ] **Step 2:** Write failing test that verifies lazy behavior (startup time, first-request latency)
- [ ] **Step 3:** Replace eager init with lazy init using `sync.Once` or lazyloader
- [ ] **Step 4:** Run tests
- [ ] **Step 5:** Run benchmark to verify improvement
- [ ] **Step 6:** Commit

---

## Phase 8: Comprehensive Stress & Integration Testing (US6, FR-020, SC-010, SC-015)

**Entry Criteria:** Phases 4-7 complete. All infrastructure containers running.
**Exit Criteria:** 10,000 concurrent request test passes. 100 concurrent debate test passes. Zero panics.
**Delivers:** Validated system resilience.

### Approach

1. Extend existing stress test suite at `tests/stress/` (62 files already exist)
2. Add monitoring metrics collection during stress tests
3. Ensure `tests/integration/` (100+ files already exist) covers all cross-component paths
4. Create performance regression detection via `make benchmark-baseline` (existing target)

### Key Existing Infrastructure

- `tests/stress/` — 62 stress test files already exist
- `tests/integration/` — 100+ integration test files already exist
- `tests/performance/` — Performance benchmarks
- `monitoring/` — Prometheus, Grafana, Loki configurations
- `Makefile` targets: `benchmark-baseline`, `test-performance`, `test-all-must-pass`

### Task Pattern

- [ ] **Step 1:** Identify uncovered stress scenario
- [ ] **Step 2:** Write stress test with specific load parameters
- [ ] **Step 3:** Run stress test with metrics collection
- [ ] **Step 4:** Analyze results and identify bottlenecks
- [ ] **Step 5:** Fix bottlenecks (may feed back to Phase 7)
- [ ] **Step 6:** Re-run to verify fix
- [ ] **Step 7:** Commit

---

## Phase 9: Complete Documentation (US7, FR-014-018, SC-011)

**Entry Criteria:** Phases 1-8 complete. All code changes finalized.
**Exit Criteria:** Every module has README.md. User guides cover all features. Video course materials updated. Website content current. SQL schema documented.
**Delivers:** Complete documentation suite.

### Approach

1. Generate documentation completeness map from Phase 1 audit
2. For each module missing docs, create:
   - `README.md` with purpose, usage, API reference, testing instructions
   - Update `CLAUDE.md` and `AGENTS.md` if not synchronized per CONST-020
3. Update user guides in `docs/user-guides/` covering all features
4. Update video course materials in `docs/courses/` (outlines, slides, labs, assessments)
5. Update `Website/` content
6. Update SQL schema documentation (`docs/SQL_SCHEMA.md`, `COMPLETE_SQL_SCHEMA.sql`)

### Key Documentation Directories

- `docs/` — 100+ subdirectories
- `docs/user-guides/` — 6 existing files
- `docs/courses/` — Course materials directory
- `Website/` — Website content
- Per-module: `README.md`, `CLAUDE.md`, `AGENTS.md`

### Task Pattern

- [ ] **Step 1:** Identify module missing documentation
- [ ] **Step 2:** Write README.md with standard template sections
- [ ] **Step 3:** Verify CLAUDE.md and AGENTS.md are synchronized per CONST-020
- [ ] **Step 4:** Commit

---

## Phase 10: Challenge Test Bank Completion (US8, FR-019, SC-013)

**Entry Criteria:** Phases 1-9 complete. All tests passing.
**Exit Criteria:** All challenges validate real-world behavior. No false positives. No placeholder data.
**Delivers:** Complete challenge test bank.

### Approach

1. Audit existing challenges in `challenges/` and `Challenges/`
2. For each challenge, verify:
   - Tests actual system behavior (API responses, database state, file contents)
   - Uses real test data (no placeholders)
   - Results include actual output verification
3. Add missing challenges for any module without one (per CONST-003)
4. Run all QA banks and verify all pass

### Key Directories

- `challenges/` — Challenge test files
- `Challenges/` — Challenge framework
- `qa-banks/` — 17 QA bank YAML files
- `test_banks/` — 3 test bank YAML files
- `challenge-results/` — Results directory

### Task Pattern

- [ ] **Step 1:** Identify module without challenge or with placeholder challenge
- [ ] **Step 2:** Write challenge that exercises real system behavior
- [ ] **Step 3:** Run challenge and verify output
- [ ] **Step 4:** Commit

---

## Cross-Phase Requirements

### Non-Functional Requirements Applied to All Phases

| Requirement | How Verified |
|-------------|-------------|
| FR-022: No breaking changes | `make test` after every commit |
| FR-023: Real services (no mocks) | Integration tests use Docker Compose infra |
| FR-024: All 29 constitution rules | Constitution compliance check in audit report |
| TR-002: No sudo/root | All commands run as current user |
| TR-003: Makefile targets | `make test-infra-start` for infrastructure |
| TR-007: Race detection | `go test -race ./...` in Phase 5 |

### Makefile Targets Summary

| Target | Phase | Purpose |
|--------|-------|---------|
| `make audit` | 1 | Full codebase audit |
| `make audit-coverage` | 1,4 | Coverage gap scan |
| `make audit-todo` | 1 | TODO marker scan |
| `make audit-skip` | 1,3 | Skipped test scan |
| `make audit-deadcode` | 1,2 | Dead code scan |
| `make audit-concurrency` | 1,5 | Concurrency hazard scan |
| `make test-all` | All | Full test suite |
| `make test-all-must-pass` | All | Zero-skip test suite |
| `make test-coverage-100` | 4 | 100% coverage verification |
| `make security-scan` | 6 | All security scanners |
| `make benchmark-baseline` | 7,8 | Performance baselines |
| `make test-challenges` | 10 | Challenge test suite |

---

## Spec Coverage Self-Review

| Spec Requirement | Phase/Task | Status |
|-----------------|------------|--------|
| FR-001: Audit report | Phase 1, Tasks 1.1-1.9 | Covered |
| FR-002: 100% test coverage | Phase 4 | Covered |
| FR-003: Resolve skipped tests | Phase 3 | Covered |
| FR-004: Test untested providers | Phase 4 | Covered |
| FR-005: Snyk zero vulns | Phase 6 | Covered |
| FR-006: SonarQube zero issues | Phase 6 | Covered |
| FR-007: gosec baseline resolved | Phase 6 | Covered |
| FR-008: Lazy loading | Phase 7 | Covered |
| FR-009: Semaphore concurrency | Phase 7 | Covered |
| FR-010: Non-blocking patterns | Phase 7 | Covered |
| FR-011: Zero races | Phase 5 | Covered |
| FR-012: No memory leaks | Phase 5 | Covered |
| FR-013: No deadlocks | Phase 5 | Covered |
| FR-014: Module docs | Phase 9 | Covered |
| FR-015: User manuals | Phase 9 | Covered |
| FR-016: Video courses | Phase 9 | Covered |
| FR-017: Website content | Phase 9 | Covered |
| FR-018: SQL schema docs | Phase 9 | Covered |
| FR-019: Challenge tests | Phase 10 | Covered |
| FR-020: Monitoring/metrics | Phase 8 | Covered |
| FR-021: Eliminate dead code | Phase 2 | Covered |
| FR-022: No breakage | All phases | Covered |
| FR-023: Real services | All phases | Covered |
| FR-024: Constitution rules | All phases | Covered |
| FR-025: Snyk/SonarQube Docker | Phase 6 | Covered |
| SC-001 through SC-015 | Phases 1-10 | All covered |
