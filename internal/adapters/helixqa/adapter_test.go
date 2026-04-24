package helixqa_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	helixqaadapter "dev.helix.agent/internal/adapters/helixqa"
)

func TestAdapter_New_NilLogger(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	require.NotNil(t, adapter, "New with nil logger should return non-nil adapter")
}

func TestAdapter_New_WithLogger(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	adapter := helixqaadapter.New(logger)
	require.NotNil(t, adapter)
}

func TestAdapter_Initialize_DefaultPath(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-memory.db")

	err := adapter.Initialize(dbPath)
	require.NoError(t, err)

	// Verify DB file was created.
	_, statErr := os.Stat(dbPath)
	assert.NoError(t, statErr, "memory.db should exist after init")

	require.NoError(t, adapter.Close())
}

func TestAdapter_Close_BeforeInit(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	err := adapter.Close()
	assert.NoError(t, err, "Close before init should be a no-op")
}

func TestAdapter_Close_Multiple(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	dbPath := filepath.Join(t.TempDir(), "test.db")

	require.NoError(t, adapter.Initialize(dbPath))
	require.NoError(t, adapter.Close())

	// Second close should be safe.
	err := adapter.Close()
	assert.NoError(t, err, "second Close should be a no-op")
}

func TestAdapter_GetFindings_BeforeInit(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	_, err := adapter.GetFindings("open")
	assert.Error(t, err, "GetFindings before init should fail")
	assert.Contains(t, err.Error(), "not initialized")
}

func TestAdapter_GetFinding_BeforeInit(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	_, err := adapter.GetFinding("HELIX-001")
	assert.Error(t, err, "GetFinding before init should fail")
	assert.Contains(t, err.Error(), "not initialized")
}

func TestAdapter_UpdateFindingStatus_BeforeInit(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	err := adapter.UpdateFindingStatus("HELIX-001", "fixed")
	assert.Error(t, err, "UpdateFindingStatus before init should fail")
}

func TestAdapter_GetFindings_EmptyStore(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	dbPath := filepath.Join(t.TempDir(), "test.db")
	require.NoError(t, adapter.Initialize(dbPath))
	defer adapter.Close()

	findings, err := adapter.GetFindings("open")
	require.NoError(t, err)
	assert.Empty(t, findings, "fresh store should have no findings")
}

func TestAdapter_GetFindings_DefaultStatus(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	dbPath := filepath.Join(t.TempDir(), "test.db")
	require.NoError(t, adapter.Initialize(dbPath))
	defer adapter.Close()

	// Empty status should default to "open".
	findings, err := adapter.GetFindings("")
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestAdapter_SupportedPlatforms(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	platforms := adapter.SupportedPlatforms()

	require.NotEmpty(t, platforms)
	assert.Contains(t, platforms, "android")
	assert.Contains(t, platforms, "web")
	assert.Contains(t, platforms, "desktop")
	assert.Contains(t, platforms, "api")
}

func TestAdapter_DiscoverCredentials_EmptyRoot(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	_, err := adapter.DiscoverCredentials("")
	assert.Error(t, err, "empty root should fail")
	assert.Contains(t, err.Error(), "project root is required")
}

func TestAdapter_DiscoverCredentials_InvalidRoot(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	_, err := adapter.DiscoverCredentials("/nonexistent/path/xyz")
	assert.Error(t, err, "nonexistent root should fail")
}

func TestAdapter_DiscoverCredentials_ValidRoot(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	tmpDir := t.TempDir()

	// Create a .env file with test credentials.
	envContent := "ADMIN_USERNAME=testuser\nADMIN_PASSWORD=testpass\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".env"), []byte(envContent), 0o644))

	creds, err := adapter.DiscoverCredentials(tmpDir)
	require.NoError(t, err)
	assert.NotEmpty(t, creds)
}

func TestAdapter_DiscoverKnowledge_EmptyRoot(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	_, err := adapter.DiscoverKnowledge("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project root is required")
}

func TestAdapter_DiscoverKnowledge_ValidRoot(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	tmpDir := t.TempDir()

	// Create docs dir with a markdown file.
	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(docsDir, "guide.md"),
		[]byte("# Guide\n\nSome documentation."),
		0o644,
	))

	kb, err := adapter.DiscoverKnowledge(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, kb)
}

func TestAdapter_RunAutonomousSession_NilConfig(t *testing.T) {
	t.Parallel()
	adapter := helixqaadapter.New(nil)
	ctx := context.Background()
	_, err := adapter.RunAutonomousSession(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session config is required")
}

func TestAdapter_RunAutonomousSession_EmptyProject(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping live pipeline test in short mode")  // SKIP-OK: #short-mode
	}
	adapter := helixqaadapter.New(nil)

	cfg := &helixqaadapter.SessionConfig{
		ProjectRoot: t.TempDir(),
		Platforms:   []string{"api"},
		OutputDir:   t.TempDir(),
		IssuesDir:   t.TempDir(),
	}

	ctx := context.Background()
	result, err := adapter.RunAutonomousSession(ctx, cfg)
	if err != nil {
		// No providers available — expected in isolated env.
		assert.Contains(t, err.Error(), "provider")
		return
	}

	// Pipeline completed — verify result shape.
	require.NotNil(t, result)
	assert.Equal(t, helixqaadapter.StatusCompleted, result.Status)
}

func TestSessionStatus_Constants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, helixqaadapter.SessionStatus("pending"),
		helixqaadapter.StatusPending)
	assert.Equal(t, helixqaadapter.SessionStatus("running"),
		helixqaadapter.StatusRunning)
	assert.Equal(t, helixqaadapter.SessionStatus("completed"),
		helixqaadapter.StatusCompleted)
	assert.Equal(t, helixqaadapter.SessionStatus("failed"),
		helixqaadapter.StatusFailed)
}
