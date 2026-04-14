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
var interfaceMethodPattern = regexp.MustCompile(`^func\s+\(\w+\s+\*?\w+\)\s+(\w+)\s*\(`)

var runtimeWiredPatterns = []string{
	"HandleFunc(",
	"Handle(",
	"ServeMux",
	"Serve(",
	"ListenAndServe(",
	"Register(",
	"RegisterServer(",
	"RegisterService(",
	"grpc.NewServer",
	"proto.Register",
	"wire.Build(",
	"wire.NewSet(",
	"fx.Provide(",
	"fx.Invoke(",
	"fx.New(",
	"OnStart(",
	"OnStop(",
	"Provide(",
	"Invoke(",
	"router.",
	"GET(",
	"POST(",
	"PUT(",
	"DELETE(",
	"PATCH(",
	"Group(",
	"Use(",
	"Middleware(",
	"plugin.Open(",
	"reflect.Value",
	"MethodByName(",
}

func ScanDeadCode(rootDir string, entryPoints []string) ([]DeadCodeEntry, error) {
	allFuncs := map[string]*DeadCodeEntry{}
	fileContents := map[string][]string{}
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
		lines, readErr := readFileLines(path)
		if readErr != nil {
			return readErr
		}
		fileContents[rel] = lines
		extractFuncDeclsFromLines(lines, rel, allFuncs)
		return nil
	})
	if err != nil {
		return nil, err
	}

	referenced := map[string]bool{"main": true, "init": true}
	for _, ep := range entryPoints {
		refs := extractRefsFromFile(ep)
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
			lines, ok := fileContents[entry.File]
			if !ok {
				continue
			}
			for _, refName := range extractRefsFromLines(lines) {
				if !referenced[refName] {
					referenced[refName] = true
					changed = true
				}
			}
		}
	}

	var results []DeadCodeEntry
	for name, entry := range allFuncs {
		isReachable := referenced[name]
		entry.Reachable = isReachable
		entry.Ident = name
		if !isReachable {
			entry.Confidence, entry.Reason = classifyDeadCode(name, entry.File, fileContents)
		}
		results = append(results, *entry)
	}
	return results, nil
}

func classifyDeadCode(funcName, file string, fileContents map[string][]string) (DeadCodeConfidence, string) {
	lines, ok := fileContents[file]
	if !ok {
		return ConfidenceNeedsReview, "file not available for analysis"
	}

	for _, line := range lines {
		if isFuncDecl(line, funcName) {
			continue
		}
		for _, pattern := range runtimeWiredPatterns {
			if strings.Contains(line, pattern) {
				return ConfidenceWiredByRuntime, "used in runtime wiring pattern: " + extractPattern(line, pattern)
			}
		}
	}

	if strings.HasPrefix(funcName, "Serve") || strings.HasPrefix(funcName, "Handle") {
		return ConfidenceWiredByRuntime, "function name suggests HTTP/gRPC handler"
	}
	if strings.HasSuffix(funcName, "Handler") || strings.HasSuffix(funcName, "Endpoint") {
		return ConfidenceWiredByRuntime, "function name suggests handler/endpoint registration"
	}
	if strings.HasSuffix(funcName, "Middleware") {
		return ConfidenceWiredByRuntime, "function name suggests middleware registration"
	}
	if strings.HasSuffix(funcName, "Provider") || strings.HasSuffix(funcName, "Constructor") {
		return ConfidenceNeedsReview, "function name suggests dependency injection provider"
	}
	if strings.HasPrefix(funcName, "New") && len(funcName) > 3 {
		return ConfidenceNeedsReview, "constructor function may be used via reflection or DI"
	}

	return ConfidenceSafeToRemove, "no references found in codebase"
}

func extractPattern(line, pattern string) string {
	idx := strings.Index(line, pattern)
	start := idx - 20
	if start < 0 {
		start = 0
	}
	end := idx + len(pattern) + 20
	if end > len(line) {
		end = len(line)
	}
	return line[start:end]
}

func isFuncDecl(line, funcName string) bool {
	matches := funcDeclPattern.FindStringSubmatch(line)
	if len(matches) > 1 && matches[1] == funcName {
		return true
	}
	return false
}

func readFileLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func extractFuncDeclsFromLines(lines []string, relPath string, funcs map[string]*DeadCodeEntry) {
	for _, line := range lines {
		matches := funcDeclPattern.FindStringSubmatch(line)
		if len(matches) > 1 {
			funcs[matches[1]] = &DeadCodeEntry{
				File: relPath,
				Kind: "function",
			}
		}
	}
}

func extractRefsFromFile(path string) map[string]bool {
	f, err := os.Open(path)
	if err != nil {
		return nil
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
	return refs
}

func extractRefsFromLines(lines []string) []string {
	var refs []string
	for _, line := range lines {
		if funcDeclPattern.MatchString(line) {
			continue
		}
		for _, m := range funcCallPattern.FindAllStringSubmatch(line, -1) {
			if len(m) > 1 {
				refs = append(refs, m[1])
			}
		}
	}
	return refs
}
