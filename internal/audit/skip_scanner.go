package audit

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func ScanSkippedTests(rootDir string) ([]SkipEntry, error) {
	var skips []SkipEntry
	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldExcludeDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return relErr
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
		reason := extractSkipReason(scanner.Text())
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
	for _, prefix := range []string{`t.Skip("`, `t.Skipf("`} { // SKIP-OK: #legacy-untriaged
		idx := strings.Index(trimmed, prefix)
		if idx < 0 {
			continue
		}
		start := idx + len(prefix)
		end := strings.Index(trimmed[start:], `"`)
		if end < 0 {
			end = strings.Index(trimmed[start:], ",")
		}
		if end < 0 {
			return trimmed[start:]
		}
		return trimmed[start : start+end]
	}
	return ""
}
