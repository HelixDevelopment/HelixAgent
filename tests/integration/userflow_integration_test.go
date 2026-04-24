package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.challenges/pkg/challenge"

	challenges "dev.helix.agent/internal/challenges"
	"dev.helix.agent/internal/challenges/userflow"
)

// expectedChallengeIDs is the canonical set of 22 userflow
// challenge IDs. Keep this in sync with
// internal/challenges/userflow/orchestrator.go.
var expectedChallengeIDs = []string{
	"helix-health-check",
	"helix-feature-flags",
	"helix-provider-discovery",
	"helix-monitoring",
	"helix-formatters",
	"helix-chat-completion",
	"helix-streaming-completion",
	"helix-embeddings",
	"helix-debate",
	"helix-mcp-protocol",
	"helix-rag",
	"helix-authentication",
	"helix-error-handling",
	"helix-concurrent-users",
	"helix-multi-turn",
	"helix-tool-calling",
	"helix-provider-failover",
	"helix-websocket-streaming",
	"helix-grpc-service",
	"helix-rate-limiting",
	"helix-pagination",
	"helix-full-system",
}

// dependencyGraph maps each challenge ID to its expected
// dependency IDs. Challenges with no dependencies map to nil.
var dependencyGraph = map[string][]string{
	"helix-health-check":         nil,
	"helix-feature-flags":        nil,
	"helix-full-system":          nil,
	"helix-provider-discovery":   {"helix-health-check"},
	"helix-monitoring":           {"helix-health-check"},
	"helix-formatters":           {"helix-health-check"},
	"helix-mcp-protocol":         {"helix-health-check"},
	"helix-authentication":       {"helix-health-check"},
	"helix-error-handling":       {"helix-health-check"},
	"helix-concurrent-users":     {"helix-health-check"},
	"helix-websocket-streaming":  {"helix-health-check"},
	"helix-grpc-service":         {"helix-health-check"},
	"helix-rate-limiting":        {"helix-health-check"},
	"helix-pagination":           {"helix-health-check"},
	"helix-chat-completion":      {"helix-provider-discovery"},
	"helix-embeddings":           {"helix-provider-discovery"},
	"helix-provider-failover":    {"helix-provider-discovery"},
	"helix-streaming-completion": {"helix-chat-completion"},
	"helix-debate":               {"helix-chat-completion"},
	"helix-multi-turn":           {"helix-chat-completion"},
	"helix-tool-calling":         {"helix-chat-completion"},
	"helix-rag":                  {"helix-embeddings"},
}

// --- Full Orchestrator Lifecycle ---

func TestUserflowOrchestrator_FullLifecycle(
	t *testing.T,
) {
	o, err := userflow.NewOrchestrator(
		"http://localhost:8100",
	)
	require.NoError(t, err)
	require.NotNil(t, o,
		"orchestrator must not be nil")

	// Count.
	count := o.ChallengeCount()
	assert.Equal(t, 22, count,
		"orchestrator must register exactly 22 challenges")

	// List.
	ids := o.ListChallenges()
	assert.Len(t, ids, 22,
		"ListChallenges must return 22 IDs")

	// Summary.
	summary := o.Summary()
	assert.Contains(t, summary, "22 challenges",
		"summary must mention the challenge count")
	assert.Contains(t, summary, "localhost:8100",
		"summary must mention the base URL")

	// Challenges() returns the same count.
	all := o.Challenges()
	assert.Len(t, all, 22,
		"Challenges() must return 22 entries")
}

// --- Challenge Registry Integrity ---

func TestUserflowOrchestrator_RegistryIntegrity(
	t *testing.T,
) {
	o, err := userflow.NewOrchestrator(
		"http://localhost:8100",
	)
	require.NoError(t, err)

	ids := o.ListChallenges()
	require.Len(t, ids, len(expectedChallengeIDs),
		"count must match expected")

	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		require.False(t, idSet[id],
			"duplicate challenge ID: %s", id)
		idSet[id] = true
	}

	for _, expected := range expectedChallengeIDs {
		assert.True(t, idSet[expected],
			"missing challenge ID: %s", expected)
	}
}

// --- RunByID with Invalid ID ---

func TestUserflowOrchestrator_RunByID_InvalidID(
	t *testing.T,
) {
	o, err := userflow.NewOrchestrator(
		"http://localhost:8100",
	)
	require.NoError(t, err)
	ctx := context.Background()

	_, err = o.RunByID(ctx, "does-not-exist-xyz")
	require.Error(t, err,
		"RunByID with invalid ID must return error")
	assert.Contains(t, err.Error(), "failed to get challenge",
		"error must indicate lookup failure")
}

// --- RunAll Without Server (Graceful Failure) ---

func TestUserflowOrchestrator_RunAll_NoServer(
	t *testing.T,
) {
	o, err := userflow.NewOrchestrator(
		"http://localhost:8100",
	)
	require.NoError(t, err)

	// Use an already-cancelled context so we don't wait for
	// real HTTP connections.
	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	// Must not panic. We don't assert specific results
	// because the behaviour depends on the runner's handling
	// of a cancelled context, but it MUST NOT crash.
	assert.NotPanics(t, func() {
		_, _ = o.RunAll(ctx)
	}, "RunAll with cancelled context must not panic")
}

// --- Dependency Graph Validation ---

func TestUserflowOrchestrator_DependencyGraph(
	t *testing.T,
) {
	o, err := userflow.NewOrchestrator(
		"http://localhost:8100",
	)
	require.NoError(t, err)

	challengeMap := make(
		map[string]challenge.Challenge,
	)
	for _, c := range o.Challenges() {
		challengeMap[string(c.ID())] = c
	}

	for id, expectedDeps := range dependencyGraph {
		c, ok := challengeMap[id]
		require.True(t, ok,
			"challenge %s must be registered", id)

		deps := c.Dependencies()
		if expectedDeps == nil {
			assert.Empty(t, deps,
				"challenge %s should have no deps", id)
			continue
		}

		actualDeps := make([]string, len(deps))
		for i, d := range deps {
			actualDeps[i] = string(d)
		}
		assert.ElementsMatch(t,
			expectedDeps, actualDeps,
			"challenge %s dependency mismatch", id,
		)
	}
}

// --- Challenge Metadata Consistency ---

func TestUserflowOrchestrator_MetadataConsistency(
	t *testing.T,
) {
	o, err := userflow.NewOrchestrator(
		"http://localhost:8100",
	)
	require.NoError(t, err)

	tests := make([]struct {
		id   string
		chal challenge.Challenge
	}, 0, o.ChallengeCount())

	for _, c := range o.Challenges() {
		tests = append(tests, struct {
			id   string
			chal challenge.Challenge
		}{string(c.ID()), c})
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			assert.NotEmpty(t, tt.chal.ID(),
				"challenge must have non-empty ID")
			assert.NotEmpty(t, tt.chal.Name(),
				"challenge must have non-empty Name")
			assert.NotEmpty(t, tt.chal.Description(),
				"challenge must have non-empty Description")
			assert.True(t,
				strings.HasPrefix(tt.id, "helix-"),
				"ID must have helix- prefix",
			)
		})
	}
}

// --- Orchestrator Summary Format ---

func TestUserflowOrchestrator_SummaryFormat(
	t *testing.T,
) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{
			"default_url",
			"http://localhost:8100",
		},
		{
			"custom_url",
			"http://10.0.0.5:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, oErr := userflow.NewOrchestrator(tt.baseURL)
			require.NoError(t, oErr)
			summary := o.Summary()

			assert.Contains(t, summary,
				"UserFlow Orchestrator",
				"summary must contain orchestrator label")
			assert.Contains(t, summary,
				"22 challenges",
				"summary must contain challenge count")
			assert.Contains(t, summary,
				tt.baseURL,
				"summary must contain base URL")
		})
	}
}

// --- Category Assignment via Main Orchestrator ---

func TestMainOrchestrator_UserflowCategory(
	t *testing.T,
) {
	// Create a temp directory with a minimal challenge script
	// so RegisterAll does not fail on missing scripts dir.
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(
		tmpDir, "challenges", "scripts",
	)
	require.NoError(t, os.MkdirAll(scriptsDir, 0o755))

	// Create a dummy challenge script so the directory is
	// not empty and the shell registration path succeeds.
	dummyScript := filepath.Join(
		scriptsDir, "dummy_challenge.sh",
	)
	require.NoError(t, os.WriteFile(
		dummyScript,
		[]byte("#!/bin/bash\nexit 0\n"),
		0o755,
	))

	cfg := challenges.OrchestratorConfig{
		ProjectRoot: tmpDir,
		ScriptsDir:  scriptsDir,
		BaseURL:     "http://localhost:8100",
	}
	orch := challenges.NewOrchestrator(cfg)
	require.NotNil(t, orch)

	err := orch.RegisterAll()
	require.NoError(t, err,
		"RegisterAll must succeed")

	infos := orch.List()
	require.NotEmpty(t, infos,
		"orchestrator must have registered challenges")

	// All challenges with "helix-" prefix must have category
	// "userflow" (set by the main orchestrator's RegisterAll).
	var userflowCount int
	for _, info := range infos {
		if strings.HasPrefix(info.ID, "helix-") {
			assert.Equal(t, "userflow", info.Category,
				"challenge %s must have category userflow",
				info.ID,
			)
			userflowCount++
		}
	}
	assert.Equal(t, 22, userflowCount,
		"all 22 helix-* challenges must be present")
}

// --- Duplicate Registration Guard ---

func TestUserflowOrchestrator_NoDuplicateIDs(
	t *testing.T,
) {
	o, err := userflow.NewOrchestrator(
		"http://localhost:8100",
	)
	require.NoError(t, err)

	ids := o.ListChallenges()
	seen := make(map[string]int, len(ids))
	for _, id := range ids {
		seen[id]++
	}

	for id, count := range seen {
		assert.Equal(t, 1, count,
			"challenge %s registered %d times", id, count)
	}
}

// --- Dependencies Reference Existing Challenges ---

func TestUserflowOrchestrator_DepsReferenceRegistered(
	t *testing.T,
) {
	o, err := userflow.NewOrchestrator(
		"http://localhost:8100",
	)
	require.NoError(t, err)

	idSet := make(map[string]bool)
	for _, c := range o.Challenges() {
		idSet[string(c.ID())] = true
	}

	for _, c := range o.Challenges() {
		for _, dep := range c.Dependencies() {
			assert.True(t, idSet[string(dep)],
				"challenge %s depends on %s "+
					"which is not registered",
				c.ID(), dep,
			)
		}
	}
}

// --- RunByID with Cancelled Context ---

func TestUserflowOrchestrator_RunByID_CancelledCtx(
	t *testing.T,
) {
	o, err := userflow.NewOrchestrator(
		"http://localhost:8100",
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	assert.NotPanics(t, func() {
		_, _ = o.RunByID(ctx, "helix-health-check")
	}, "RunByID with cancelled context must not panic")
}
