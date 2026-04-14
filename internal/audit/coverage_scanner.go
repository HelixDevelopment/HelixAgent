package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ScanCoverageGaps(rootDir string) ([]CoverageGap, error) {
	var gaps []CoverageGap
	pkgHasTests := map[string]bool{}

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			if shouldExcludeDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(rel, "_test.go") {
			pkgHasTests[filepath.Dir(rel)] = true
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan test files: %w", err)
	}

	err = filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			if shouldExcludeDir(d.Name()) {
				return filepath.SkipDir
			}
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
	if err != nil {
		return nil, fmt.Errorf("scan source files: %w", err)
	}

	return gaps, nil
}

func shouldExcludeDir(name string) bool {
	switch name {
	case "vendor", "cli_agents", "external", ".git", "node_modules":
		return true
	}
	return false
}
