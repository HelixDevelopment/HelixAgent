package integration

// Go-source npm-package guards.
//
// TestMCPPackageExistence (mcp_server_validation_test.go) derives its package
// list from the shipped JSON configs. That scan is blind to the npm package
// names that live as Go string literals in the generators, the validator and
// the CLI - and those literals are what a user gets from
// `helixagent -generate-*-config`, which is the path most users actually take.
//
// Every one of the packages fixed alongside this file was invisible to the
// config-derived scan for exactly that reason.

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcpconfig "dev.helix.agent/internal/mcp/config"
)

// goSourceRoots are the trees whose Go sources emit MCP configuration.
// Vendored submodules (MCP/submodules/**, external/**) are third-party and
// deliberately out of scope - we do not own their package references.
var goSourceRoots = []string{"cmd", "internal", "challenges"}

// npmPackageFieldNames are struct fields that hold a bare npm package name
// rather than a full argv (see internal/mcp/extended_packages.go).
var npmPackageFieldNames = map[string]bool{
	"NPM":     true,
	"Package": true,
}

// npxPackageSite is one npm package reference found in Go source.
type npxPackageSite struct {
	Package string
	File    string
	Line    int
}

// TestGoSourceMCPPackageExistence verifies that every npm package name written
// into an MCP configuration by Go source actually exists in the npm registry.
//
// The package list is DERIVED FROM THE SOURCE rather than hand-maintained, so a
// newly added (or misspelled) literal is covered automatically - the same
// property that makes the config-derived scan trustworthy.
func TestGoSourceMCPPackageExistence(t *testing.T) {
	if testing.Short() {
		t.Logf("Short mode - skipping Go-source npm package existence test (acceptable)")
		return
	}

	root := getProjectRoot(t)

	sites, err := collectGoSourceNpmPackages(root)
	require.NoError(t, err, "Must be able to scan Go sources for npm package literals")
	require.NotEmpty(t, sites, "Go-source scan must find npm packages - an empty result means the scanner broke, not that the source is clean")

	// Instrument sanity check, mirroring TestMCPPackageExistence: if a package
	// we KNOW exists cannot be resolved, the registry is unreachable and every
	// subsequent 404 would be a false negative rather than a real finding.
	if !checkNpmPackageExists(npmRegistryControlPackage) {
		t.Skipf("npm registry unreachable - control package %s did not resolve; cannot distinguish a missing package from a network failure (SKIP-OK: #npm-registry-unreachable)", npmRegistryControlPackage)
	}

	byPackage := map[string][]npxPackageSite{}
	for _, s := range sites {
		byPackage[s.Package] = append(byPackage[s.Package], s)
	}
	packages := make([]string, 0, len(byPackage))
	for p := range byPackage {
		packages = append(packages, p)
	}
	sort.Strings(packages)

	for _, pkg := range packages {
		t.Run(pkg, func(t *testing.T) {
			h := npmPackageHealthOf(pkg)
			assert.True(t, h.Usable,
				"Package %s is written into an MCP config by Go source (%s) but is NOT installable: %s - a user running the generator gets an MCP server that fails to start",
				pkg, formatSites(byPackage[pkg]), h.Reason)
		})
	}
}

// TestGoSourceAvoidsKnownNonexistentPackages is the network-free regression
// guard: no Go source may reference a package name already proven absent from
// the npm registry.
func TestGoSourceAvoidsKnownNonexistentPackages(t *testing.T) {
	root := getProjectRoot(t)

	sites, err := collectGoSourceNpmPackages(root)
	require.NoError(t, err)
	require.NotEmpty(t, sites, "Go-source scan must find npm packages")

	referenced := map[string][]npxPackageSite{}
	for _, s := range sites {
		referenced[s.Package] = append(referenced[s.Package], s)
	}

	for _, bad := range knownNonexistentMCPPackages {
		assert.Empty(t, referenced[bad],
			"Go source references %s, which does not exist in the npm registry (verified 404) - at %s",
			bad, formatSites(referenced[bad]))
	}
}

// TestGoSourceAvoidsKnownUnusablePackages is the Go-source half of the
// resolves-but-unusable guard.
//
// The 404 list above cannot catch these: a security holding package and an
// empty registry shell both answer HTTP 200, so they are "existing" names that
// install nothing. Five of them shipped in this repo's generators until
// 2026-09-03 precisely because every check only asked whether the name
// resolved.
func TestGoSourceAvoidsKnownUnusablePackages(t *testing.T) {
	root := getProjectRoot(t)

	sites, err := collectGoSourceNpmPackages(root)
	require.NoError(t, err)
	require.NotEmpty(t, sites, "Go-source scan must find npm packages")

	referenced := map[string][]npxPackageSite{}
	for _, s := range sites {
		referenced[s.Package] = append(referenced[s.Package], s)
	}

	for bad, why := range knownUnusableMCPPackages {
		assert.Empty(t, referenced[bad],
			"Go source references %s, which resolves in the npm registry but installs nothing: %s - at %s",
			bad, why, formatSites(referenced[bad]))
	}
}

// TestGeneratorAndShippedConfigAgree pins the two independent surfaces that
// produce an OpenCode MCP config to the same answer.
//
// `configs/cli-agents/opencode.json` is written by
// scripts/cli-agents/generate-all-configs.sh; `GenerateOpenCodeMCPs` is what
// `helixagent -generate-opencode-config` runs. Nothing previously compared
// them, and they DID diverge: the shipped JSON used
// `mcp-server-sqlite-npx <path>` while the Go generator still emitted
// `@modelcontextprotocol/server-sqlite --db-path <path>` - a package that does
// not exist, invoked with a flag it would not accept.
func TestGeneratorAndShippedConfigAgree(t *testing.T) {
	root := getProjectRoot(t)

	configPath := filepath.Join(root, "configs", "cli-agents", "opencode.json")
	data, err := os.ReadFile(configPath) // #nosec G304 - repo-local config
	require.NoError(t, err, "Shipped OpenCode config must be readable")

	var doc struct {
		MCP map[string]struct {
			Command []string `json:"command"`
		} `json:"mcp"`
	}
	require.NoError(t, json.Unmarshal(data, &doc), "Shipped OpenCode config must be valid JSON")
	require.NotEmpty(t, doc.MCP, "Shipped OpenCode config must declare MCP servers")

	generated := mcpconfig.NewMCPConfigGenerator("http://localhost:8080").GenerateOpenCodeMCPs()
	require.NotEmpty(t, generated, "Generator must produce MCP servers")

	compared := 0
	for name, shipped := range doc.MCP {
		gen, ok := generated[name]
		if !ok {
			// Env-gated servers legitimately appear in only one surface.
			continue
		}
		shippedPkg := npxPackageFromArgv(shipped.Command)
		genPkg := npxPackageFromArgv(gen.Command)
		if shippedPkg == "" && genPkg == "" {
			continue // not an npx invocation on either side
		}
		compared++
		assert.Equal(t, shippedPkg, genPkg,
			"Server %q: shipped config uses package %q but the Go generator emits %q - the two surfaces disagree, so which one a user gets depends on how they generated the config",
			name, shippedPkg, genPkg)
		assert.Equal(t, shipped.Command, gen.Command,
			"Server %q: shipped config argv %v differs from generator argv %v - a package-name match is not enough, the invocation must agree too (the sqlite case had the right name with the wrong flag)",
			name, shipped.Command, gen.Command)
	}
	require.Positive(t, compared,
		"No server was present in BOTH the shipped config and the generator - the comparison proved nothing, which means this guard broke rather than passed")
}

// collectGoSourceNpmPackages parses every owned Go source file and returns the
// npm package name of each MCP invocation it declares.
//
// Four literal shapes are recognised, all of which occur in this repo:
//
//	[]string{"npx", "-y", "<pkg>", ...}                 // Command as one argv
//	{Command: "npx", Args: []string{"-y", "<pkg>"}}     // split command/args
//	{NPM: "<pkg>"} / {Package: "<pkg>"}                 // bare package field
//
// _test.go files are excluded: they deliberately reference fabricated package
// names as fixtures, and asserting those exist would be nonsense.
func collectGoSourceNpmPackages(root string) ([]npxPackageSite, error) {
	skip, err := vendoredSubmodulePaths(root)
	if err != nil {
		return nil, err
	}

	var sites []npxPackageSite
	fset := token.NewFileSet()

	for _, sub := range goSourceRoots {
		dir := filepath.Join(root, sub)
		if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
			continue
		}
		walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			if info.IsDir() {
				base := info.Name()
				if base == "vendor" || base == "node_modules" || base == ".git" || skip[rel] {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, parseErr := parser.ParseFile(fset, path, nil, 0)
			if parseErr != nil {
				// A file that does not parse cannot be scanned; surface it
				// rather than silently treating the source as clean.
				return parseErr
			}
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				for _, pkg := range npxPackagesFromCompositeLit(lit) {
					pos := fset.Position(lit.Pos())
					sites = append(sites, npxPackageSite{Package: pkg, File: rel, Line: pos.Line})
				}
				return true
			})
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}
	return sites, nil
}

// npxPackagesFromCompositeLit extracts npm package names declared by one
// composite literal.
func npxPackagesFromCompositeLit(lit *ast.CompositeLit) []string {
	var found []string

	// Shape 1: a plain slice of strings that starts an npx invocation.
	if argv, ok := stringSliceLiteral(lit); ok {
		if pkg := npxPackageFromArgv(argv); pkg != "" {
			found = append(found, pkg)
		}
		return found
	}

	// Shapes 2 and 3: a struct literal with named fields.
	var commandIsNpx bool
	var args []string
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch {
		case key.Name == "Command":
			if s, ok := stringLiteral(kv.Value); ok && filepath.Base(s) == "npx" {
				commandIsNpx = true
			}
		case key.Name == "Args":
			if inner, ok := kv.Value.(*ast.CompositeLit); ok {
				if argv, ok := stringSliceLiteral(inner); ok {
					args = argv
				}
			}
		case npmPackageFieldNames[key.Name]:
			if s, ok := stringLiteral(kv.Value); ok && looksLikeNpmPackage(s) {
				found = append(found, s)
			}
		}
	}
	if commandIsNpx && len(args) > 0 {
		if pkg := npxPackageFromArgv(append([]string{"npx"}, args...)); pkg != "" {
			found = append(found, pkg)
		}
	}
	return found
}

// npxPackageFromArgv returns the package an `npx` argv installs, or "".
func npxPackageFromArgv(argv []string) string {
	if len(argv) == 0 || filepath.Base(argv[0]) != "npx" {
		return ""
	}
	for _, tok := range argv[1:] {
		if strings.HasPrefix(tok, "-") {
			continue // npx flag (-y, --yes, ...)
		}
		if !looksLikeNpmPackage(tok) {
			return "" // a path or variable reference, not a package
		}
		return tok
	}
	return ""
}

// looksLikeNpmPackage reports whether s has the shape of an npm package name.
func looksLikeNpmPackage(s string) bool {
	if s == "" || strings.ContainsAny(s, " \t/\\$~") && !strings.HasPrefix(s, "@") {
		return false
	}
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, ".") {
		return false
	}
	if strings.HasPrefix(s, "@") {
		parts := strings.Split(s, "/")
		return len(parts) == 2 && parts[0] != "@" && parts[1] != ""
	}
	return !strings.Contains(s, "/")
}

func stringLiteral(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// stringSliceLiteral returns the contents of a composite literal when every
// element is a plain string constant.
func stringSliceLiteral(lit *ast.CompositeLit) ([]string, bool) {
	if len(lit.Elts) == 0 {
		return nil, false
	}
	out := make([]string, 0, len(lit.Elts))
	for _, elt := range lit.Elts {
		s, ok := stringLiteral(elt)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// vendoredSubmodulePaths returns the repo-relative paths of every git submodule
// so third-party trees are excluded from the scan.
func vendoredSubmodulePaths(root string) (map[string]bool, error) {
	skip := map[string]bool{}
	data, err := os.ReadFile(filepath.Join(root, ".gitmodules")) // #nosec G304 - repo-local
	if os.IsNotExist(err) {
		return skip, nil
	}
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "path") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			skip[filepath.Clean(strings.TrimSpace(parts[1]))] = true
		}
	}
	return skip, nil
}

func formatSites(sites []npxPackageSite) string {
	if len(sites) == 0 {
		return "(no sites)"
	}
	parts := make([]string, 0, len(sites))
	for _, s := range sites {
		parts = append(parts, s.File+":"+strconv.Itoa(s.Line))
	}
	sort.Strings(parts)
	if len(parts) > 6 {
		parts = append(parts[:6], "...")
	}
	return strings.Join(parts, ", ")
}
