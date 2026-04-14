package audit

type Severity string

const (
	SeverityBroken       Severity = "broken"
	SeverityIncomplete   Severity = "incomplete"
	SeverityOptimization Severity = "optimization"
	SeverityDocs         Severity = "documentation"
)

type SkipCategory string

const (
	CategoryInfrastructure SkipCategory = "infrastructure"
	CategoryFlakyGuard     SkipCategory = "flaky-guard"
	CategoryUnimplemented  SkipCategory = "unimplemented"
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

func (tm *TodoMarker) Classify() {
	switch tm.Marker {
	case "TODO":
		tm.Severity = SeverityIncomplete
	case "FIXME", "XXX":
		tm.Severity = SeverityBroken
	case "HACK":
		tm.Severity = SeverityOptimization
	default:
		tm.Severity = SeverityDocs
	}
}

type SkipEntry struct {
	File     string       `json:"file"`
	Line     int          `json:"line"`
	Reason   string       `json:"reason"`
	Category SkipCategory `json:"category"`
}

type DeadCodeEntry struct {
	File      string `json:"file"`
	Ident     string `json:"ident"`
	Kind      string `json:"kind"`
	Reachable bool   `json:"reachable"`
}

type ConcurrencyEntry struct {
	File  string `json:"file"`
	Line  int    `json:"line"`
	Type  string `json:"type"`
	Safe  bool   `json:"safe"`
	Issue string `json:"issue"`
}

type AuditReport struct {
	Timestamp    string             `json:"timestamp"`
	RootDir      string             `json:"root_dir"`
	CoverageGaps []CoverageGap      `json:"coverage_gaps"`
	TodoMarkers  []TodoMarker       `json:"todo_markers"`
	SkippedTests []SkipEntry        `json:"skipped_tests"`
	DeadCode     []DeadCodeEntry    `json:"dead_code"`
	Concurrency  []ConcurrencyEntry `json:"concurrency"`
	Summary      AuditSummary       `json:"summary"`
}

type AuditSummary struct {
	TotalSourceFiles  int `json:"total_source_files"`
	TotalTestFiles    int `json:"total_test_files"`
	FilesWithoutTests int `json:"files_without_tests"`
	TodoMarkerCount   int `json:"todo_marker_count"`
	SkippedTestCount  int `json:"skipped_test_count"`
	DeadCodeCount     int `json:"dead_code_count"`
	ConcurrencyIssues int `json:"concurrency_issues"`
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

var skipPatterns = []struct {
	substr   string
	category SkipCategory
}{
	{"not found", CategoryInfrastructure},
	{"not available", CategoryInfrastructure},
	{"not accessible", CategoryInfrastructure},
	{"integration test", CategoryInfrastructure},
	{"container runtime", CategoryInfrastructure},
	{"short mode", CategoryFlakyGuard},
	{"involves sleep", CategoryFlakyGuard},
}

func (se *SkipEntry) Classify() {
	for _, p := range skipPatterns {
		if containsFold(se.Reason, p.substr) {
			se.Category = p.category
			return
		}
	}
	se.Category = CategoryUnimplemented
}
