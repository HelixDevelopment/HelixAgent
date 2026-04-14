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
