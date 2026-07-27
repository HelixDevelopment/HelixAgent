package compliance

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

// projectRootEnv lets the consuming project tell this submodule where its root
// lives. HelixAgent MUST stay project-not-aware (CONST-051(B)), so it never
// hardcodes a consumer's path or name — the location is injected, or derived
// generically.
const projectRootEnv = "HELIX_PROJECT_ROOT"

// getRoot returns the root of the project that INCORPORATES this submodule —
// not this submodule's own root.
//
// Why (CONST-051(C)): extracted modules are dependencies of HelixAgent, and
// dependencies live at the CONSUMING project's root (`<root>/<name>/` or
// `<root>/submodules/<name>/`). Nested own-org submodule chains are forbidden,
// so they are deliberately NOT inside this repository. The previous
// implementation returned this submodule's own root, where they can never
// appear — which is why the count assertion reported 0/20 while all twenty
// modules were present at the consuming project's root all along.
//
// Resolution order: the injected env var first, then a generic walk-up for the
// nearest ancestor containing a `submodules/` directory. Returns "" when the
// submodule is checked out standalone, in which case the callers skip honestly
// rather than assert against a layout that genuinely is not there.
func getRoot() string {
	// An explicit override is a deliberate operator choice: honour it or fail
	// loudly. Silently falling through to the walk-up would let a typo'd path
	// produce a confident verdict about a root the operator never selected.
	if v := os.Getenv(projectRootEnv); v != "" {
		info, err := os.Stat(v)
		switch {
		case err != nil:
			return "" // signalled to the caller as a hard error, not a skip
		case !info.IsDir():
			return ""
		default:
			return v
		}
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}

	dir := filepath.Dir(filename)
	for i := 0; i < 8; i++ {
		dir = filepath.Dir(dir)
		if dir == "/" || dir == "." {
			break
		}
		if info, err := os.Stat(filepath.Join(dir, "submodules")); err == nil && info.IsDir() {
			return dir
		}
	}
	return ""
}

// resolveModulePath returns the on-disk path of an extracted module, honouring
// both layouts CONST-051(C) permits: the grouped `<root>/submodules/<name>/`
// and the flat `<root>/<name>/`. The grouped path is returned as the canonical
// form when neither exists, so failure messages name the expected location.
func resolveModulePath(root, module string) string {
	grouped := filepath.Join(root, "submodules", module)
	if _, err := os.Stat(grouped); err == nil {
		return grouped
	}
	flat := filepath.Join(root, module)
	if _, err := os.Stat(flat); err == nil {
		return flat
	}
	return grouped
}

// requireProjectRoot skips honestly when this submodule is checked out
// standalone, where the consuming project's module layout genuinely does not
// exist. Asserting against an absent consumer would be a false failure; the
// suite runs for real whenever the submodule sits inside a consuming project.
func requireProjectRoot(t *testing.T) string {
	t.Helper()

	// A set-but-invalid override is an operator error, never a skip: skipping
	// would hide the mistake behind a green run.
	if v := os.Getenv(projectRootEnv); v != "" {
		info, err := os.Stat(v)
		if err != nil {
			t.Fatalf("%s=%q is not usable: %v — fix the path or unset it", projectRootEnv, v, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s=%q is not a directory — it must point at the consuming project root", projectRootEnv, v)
		}
	}

	root := getRoot()
	if root == "" {
		t.Skip("SKIP-OK: #standalone-checkout — no consuming project root found; " +
			"set " + projectRootEnv + " to run the extracted-module compliance suite")
	}
	t.Logf("resolved consuming project root: %s", root) // §11.4.5 — say WHERE the verdict came from
	return root
}

// extractedModules lists all 20 extracted modules that must exist as
// independent Go modules at the consuming project's root.
//
// Names are lowercase snake_case per CONST-052, which mandates that convention
// for every directory and submodule. The earlier CamelCase spellings
// ("EventBus", "VectorDB", "MCP_Module", …) never matched anything on disk —
// reference drift of exactly the kind CONST-052 calls a violation of equal
// severity to the rename itself.
var extractedModules = []string{
	"event_bus",
	"concurrency",
	"observability",
	"auth",
	"storage",
	"streaming",
	"security",
	"vector_db",
	"embeddings",
	"database",
	"cache",
	"messaging",
	"formatters",
	"mcp_module",
	"rag",
	"memory",
	"optimization",
	"plugins",
	"containers",
	"challenges",
}

// TestExtractedModuleCount verifies that all 20 extracted modules exist
// as directories in the project root.
func TestExtractedModuleCount(t *testing.T) {
	root := requireProjectRoot(t)

	existingModules := []string{}
	missingModules := []string{}

	for _, module := range extractedModules {
		modulePath := resolveModulePath(root, module)
		if _, err := os.Stat(modulePath); err == nil {
			existingModules = append(existingModules, module)
		} else {
			missingModules = append(missingModules, module)
		}
	}

	t.Logf("Found %d/%d extracted modules", len(existingModules), len(extractedModules))
	if len(missingModules) > 0 {
		t.Logf("Missing modules: %v", missingModules)
	}

	assert.GreaterOrEqual(t, len(existingModules), 18,
		"COMPLIANCE FAILED: At least 18 of 20 extracted modules must exist")
}

// TestModuleGoModExists verifies that each extracted module has a go.mod
// file (confirming it's an independent Go module).
func TestModuleGoModExists(t *testing.T) {
	root := requireProjectRoot(t)
	missingGoMod := []string{}

	for _, module := range extractedModules {
		goModPath := filepath.Join(resolveModulePath(root, module), "go.mod")
		if _, err := os.Stat(goModPath); err != nil {
			modulePath := resolveModulePath(root, module)
			if _, dirErr := os.Stat(modulePath); dirErr == nil {
				// Module dir exists but no go.mod
				missingGoMod = append(missingGoMod, module)
			}
		}
	}

	if len(missingGoMod) > 0 {
		t.Errorf("COMPLIANCE FAILED: %d modules missing go.mod: %v",
			len(missingGoMod), missingGoMod)
	} else {
		t.Logf("COMPLIANCE: All present extracted modules have go.mod files")
	}
}

// TestModuleReadmeExists verifies that each extracted module has a README.md.
func TestModuleReadmeExists(t *testing.T) {
	root := requireProjectRoot(t)
	missingReadme := []string{}

	for _, module := range extractedModules {
		modulePath := resolveModulePath(root, module)
		if _, err := os.Stat(modulePath); err != nil {
			continue // Module doesn't exist, skip
		}
		readmePath := filepath.Join(modulePath, "README.md")
		if _, err := os.Stat(readmePath); err != nil {
			missingReadme = append(missingReadme, module)
		}
	}

	// Errorf, not Logf: a "WARNING" that still passes is a check that can never
	// fail, and this suite's whole purpose is to make the claim meaningful.
	if len(missingReadme) > 0 {
		t.Errorf("COMPLIANCE FAILED: %d modules missing README.md: %v",
			len(missingReadme), missingReadme)
	} else {
		t.Logf("COMPLIANCE: All present extracted modules have README.md files")
	}
}

// TestModuleClaudeMdExists verifies that each extracted module has a CLAUDE.md
// file with project-specific guidance for AI assistants.
func TestModuleClaudeMdExists(t *testing.T) {
	root := requireProjectRoot(t)
	missingClaudeMd := []string{}
	presentCount := 0

	for _, module := range extractedModules {
		modulePath := resolveModulePath(root, module)
		if _, err := os.Stat(modulePath); err != nil {
			continue // Module doesn't exist, skip
		}
		claudeMdPath := filepath.Join(modulePath, "CLAUDE.md")
		if _, err := os.Stat(claudeMdPath); err != nil {
			missingClaudeMd = append(missingClaudeMd, module)
		} else {
			presentCount++
		}
	}

	t.Logf("Modules with CLAUDE.md: %d, missing: %d (%v)",
		presentCount, len(missingClaudeMd), missingClaudeMd)
	// Errorf, not Logf — see the README.md check above.
	if len(missingClaudeMd) > 0 {
		t.Errorf("COMPLIANCE FAILED: %d modules missing CLAUDE.md: %v",
			len(missingClaudeMd), missingClaudeMd)
	} else {
		t.Logf("COMPLIANCE: All present extracted modules have CLAUDE.md files")
	}
}

// TestModuleTestsExist verifies that each extracted module has at least
// one test file (confirming test coverage requirements are met).
func TestModuleTestsExist(t *testing.T) {
	root := requireProjectRoot(t)
	missingTests := []string{}

	for _, module := range extractedModules {
		modulePath := resolveModulePath(root, module)
		if _, err := os.Stat(modulePath); err != nil {
			continue // Module doesn't exist, skip
		}

		// Look for any *_test.go file recursively
		hasTests := false
		_ = filepath.Walk(modulePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				name := info.Name()
				if len(name) > 8 && name[len(name)-8:] == "_test.go" {
					hasTests = true
					return filepath.SkipAll
				}
			}
			return nil
		})

		if !hasTests {
			missingTests = append(missingTests, module)
		}
	}

	if len(missingTests) > 0 {
		t.Errorf("COMPLIANCE FAILED: %d modules have no test files: %v",
			len(missingTests), missingTests)
	} else {
		t.Logf("COMPLIANCE: All present extracted modules have test files")
	}
}
