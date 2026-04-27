package containers

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestComposeBuildContextsAreShippable is the in-process counterpart to
// challenges/scripts/mcp_servers_distribution_challenge.sh — extends to
// every compose file the orchestrator deploys (main + MCP). For each
// service that declares `build:`, asserts:
//
//  1. The resolved build context is NOT the project root itself (the
//     orchestrator deliberately skips project-root contexts to avoid
//     scp'ing 27 GB to remote workers — see BUGFIXES.md Issue #51).
//     EXCEPT the helixagent service in the main compose, where the
//     orchestrator IS the local host and the skip is intentional.
//  2. The resolved context exists on disk.
//  3. The resolved dockerfile exists on disk.
//  4. The context is not a forbidden top-level heavy directory
//     (vendor, releases, cli_agents, node_modules).
//
// This catches a future PR that adds a `context: ../..` to any deployed
// compose, or renames a Dockerfile without updating the compose
// reference.
func TestComposeBuildContextsAreShippable(t *testing.T) {
	projectRoot := findProjectRoot(t)

	composeFiles := []string{
		filepath.Join(projectRoot, "docker-compose.yml"),
		filepath.Join(projectRoot, "docker", "mcp", "docker-compose.mcp-servers.yml"),
	}

	// Services whose context IS the project root by intent — they are
	// built locally on the orchestrator and never shipped to a remote
	// worker. Everything else is fair game.
	allowedProjectRootServices := map[string]bool{
		"helixagent": true,
	}

	forbiddenTopDirs := map[string]bool{
		"vendor": true, "releases": true, "cli_agents": true,
		"node_modules": true,
	}

	var violations []string

	for _, composePath := range composeFiles {
		data, err := os.ReadFile(composePath)
		if err != nil {
			t.Fatalf("read %s: %v", composePath, err)
		}
		composeDir := filepath.Dir(composePath)

		var doc struct {
			Services map[string]struct {
				Build interface{} `yaml:"build"`
			} `yaml:"services"`
		}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			t.Fatalf("parse %s: %v", composePath, err)
		}

		for name, svc := range doc.Services {
			if svc.Build == nil {
				continue
			}
			ctxRel := "."
			dfRel := "Dockerfile"

			switch b := svc.Build.(type) {
			case string:
				ctxRel = b
			case map[string]interface{}:
				if c, ok := b["context"].(string); ok {
					ctxRel = c
				}
				if d, ok := b["dockerfile"].(string); ok {
					dfRel = d
				}
			default:
				violations = append(violations, fmt.Sprintf(
					"%s/%s: unsupported build form: %T",
					filepath.Base(composePath), name, b))
				continue
			}

			ctxAbs := filepath.Clean(filepath.Join(composeDir, ctxRel))

			// Check 1: project-root contexts are forbidden except for
			// the explicitly-allowlisted services.
			if ctxAbs == projectRoot && !allowedProjectRootServices[name] {
				violations = append(violations, fmt.Sprintf(
					"%s/%s: build.context=%q resolves to project root; the "+
						"orchestrator skips project-root contexts (Issue #51). "+
						"Use a focused sub-context.",
					filepath.Base(composePath), name, ctxRel))
				continue
			}

			// Allowlisted services that ARE the project root: skip
			// further file checks (they're built locally; the project
			// root is always present).
			if ctxAbs == projectRoot {
				continue
			}

			// Check 4: forbidden top-level heavy dirs.
			rel, err := filepath.Rel(projectRoot, ctxAbs)
			if err == nil && !strings.HasPrefix(rel, "..") {
				top := strings.SplitN(rel, string(filepath.Separator), 2)[0]
				if forbiddenTopDirs[top] {
					violations = append(violations, fmt.Sprintf(
						"%s/%s: build.context=%q points at forbidden heavy "+
							"top-level directory %q",
						filepath.Base(composePath), name, ctxRel, top))
					continue
				}
			}

			// Check 2: context exists.
			if info, err := os.Stat(ctxAbs); err != nil || !info.IsDir() {
				violations = append(violations, fmt.Sprintf(
					"%s/%s: build.context=%q resolves to %q which is not a "+
						"directory on disk",
					filepath.Base(composePath), name, ctxRel, ctxAbs))
				continue
			}

			// Check 3: dockerfile exists relative to context.
			dfAbs := filepath.Clean(filepath.Join(ctxAbs, dfRel))
			if info, err := os.Stat(dfAbs); err != nil || info.IsDir() {
				violations = append(violations, fmt.Sprintf(
					"%s/%s: dockerfile=%q resolves to %q which is not a file "+
						"on disk",
					filepath.Base(composePath), name, dfRel, dfAbs))
			}
		}
	}

	if len(violations) > 0 {
		t.Errorf("%d build-context violation(s):\n  - %s",
			len(violations), strings.Join(violations, "\n  - "))
	}
}

func findProjectRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "challenges", "scripts")); err == nil {
				return dir
			}
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("project root not found above %s", thisFile)
	return ""
}
