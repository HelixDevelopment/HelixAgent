package audit

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func ScanConcurrency(rootDir string) ([]ConcurrencyEntry, error) {
	var entries []ConcurrencyEntry
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
		if entry := detectConcurrencyPattern(scanner.Text(), relPath, lineNum); entry != nil {
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
