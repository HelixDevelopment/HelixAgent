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

func TestGenerateReportEmptySections(t *testing.T) {
	report := &AuditReport{
		Timestamp: "2026-04-14",
		RootDir:   "/empty",
		Summary:   AuditSummary{},
	}
	md := GenerateReportMarkdown(report)
	if !strings.Contains(md, "# HelixAgent Audit Report") {
		t.Error("report should contain title even with empty data")
	}
	if strings.Contains(md, "## Files Without Tests") {
		t.Error("should not include section header when no coverage gaps")
	}
}

func TestNewAuditReport(t *testing.T) {
	report := NewAuditReport("/test")
	if report.RootDir != "/test" {
		t.Errorf("RootDir = %q, want %q", report.RootDir, "/test")
	}
	if report.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}
}
