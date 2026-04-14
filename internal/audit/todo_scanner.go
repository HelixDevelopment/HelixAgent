package audit

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func ScanTodoMarkers(rootDir string) ([]TodoMarker, error) {
	var markers []TodoMarker
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
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
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
		comment := strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))
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

func extractMarker(comment string) (string, string) {
	for _, m := range []string{"TODO", "FIXME", "HACK", "XXX"} {
		if strings.HasPrefix(comment, m) {
			rest := strings.TrimSpace(strings.TrimPrefix(comment, m))
			rest = strings.TrimPrefix(rest, ":")
			rest = strings.TrimSpace(rest)
			return m, rest
		}
	}
	return "", ""
}
