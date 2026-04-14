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
		rootDir  string
		output   string
		format   string
		scanners string
	)
	flag.StringVar(&rootDir, "root", ".", "Root directory to audit")
	flag.StringVar(&output, "output", "", "Output file path (default: stdout)")
	flag.StringVar(&format, "format", "markdown", "Output format: markdown, json")
	flag.StringVar(&scanners, "scanners", "all", "Comma-separated scanners: all,coverage,todo,skip,deadcode,concurrency")
	flag.Parse()

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve root: %v\n", err)
		os.Exit(1)
	}

	report := audit.NewAuditReport(absRoot)
	active := parseScanners(scanners)

	if active["coverage"] || active["all"] {
		gaps, scanErr := audit.ScanCoverageGaps(absRoot)
		if scanErr != nil {
			fmt.Fprintf(os.Stderr, "coverage scan: %v\n", scanErr)
			os.Exit(1)
		}
		report.CoverageGaps = gaps
	}

	if active["todo"] || active["all"] {
		markers, scanErr := audit.ScanTodoMarkers(absRoot)
		if scanErr != nil {
			fmt.Fprintf(os.Stderr, "TODO scan: %v\n", scanErr)
			os.Exit(1)
		}
		report.TodoMarkers = markers
	}

	if active["skip"] || active["all"] {
		skips, scanErr := audit.ScanSkippedTests(absRoot)
		if scanErr != nil {
			fmt.Fprintf(os.Stderr, "skip scan: %v\n", scanErr)
			os.Exit(1)
		}
		report.SkippedTests = skips
	}

	if active["deadcode"] || active["all"] {
		entryPoints := findEntryPoints(absRoot)
		dead, scanErr := audit.ScanDeadCode(absRoot, entryPoints)
		if scanErr != nil {
			fmt.Fprintf(os.Stderr, "deadcode scan: %v\n", scanErr)
			os.Exit(1)
		}
		report.DeadCode = dead
	}

	if active["concurrency"] || active["all"] {
		conc, scanErr := audit.ScanConcurrency(absRoot)
		if scanErr != nil {
			fmt.Fprintf(os.Stderr, "concurrency scan: %v\n", scanErr)
			os.Exit(1)
		}
		report.Concurrency = conc
	}

	report.Summary = audit.AuditSummary{
		TotalSourceFiles:  countSourceFiles(report),
		TotalTestFiles:    countWithTests(report),
		FilesWithoutTests: countGaps(report),
		TodoMarkerCount:   len(report.TodoMarkers),
		SkippedTestCount:  len(report.SkippedTests),
		DeadCodeCount:     countDead(report),
		ConcurrencyIssues: len(report.Concurrency),
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
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
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
	return len(r.CoverageGaps)
}

func countWithTests(r *audit.AuditReport) int {
	count := 0
	for _, g := range r.CoverageGaps {
		if g.HasAnyTest {
			count++
		}
	}
	return count
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
