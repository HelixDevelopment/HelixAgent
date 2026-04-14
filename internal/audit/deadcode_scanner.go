package audit

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var funcDeclPattern = regexp.MustCompile(`^func\s+(?:\(\w+\s+\*?\w+\)\s+)?(\w+)\s*\(`)
var funcCallPattern = regexp.MustCompile(`(\w+)\s*\(`)

func ScanDeadCode(rootDir string, entryPoints []string) ([]DeadCodeEntry, error) {
	allFuncs := map[string]*DeadCodeEntry{}
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
		return extractFuncDecls(path, rel, allFuncs)
	})
	if err != nil {
		return nil, err
	}

	referenced := map[string]bool{"main": true}
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
	for scanner.Scan() {
		matches := funcDeclPattern.FindStringSubmatch(scanner.Text())
		if len(matches) > 1 {
			funcs[matches[1]] = &DeadCodeEntry{
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
		if funcDeclPattern.MatchString(line) {
			continue
		}
		for _, m := range funcCallPattern.FindAllStringSubmatch(line, -1) {
			if len(m) > 1 {
				refs[m[1]] = true
			}
		}
	}
	return refs, scanner.Err()
}
