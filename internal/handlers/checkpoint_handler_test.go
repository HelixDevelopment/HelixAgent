// Package handlers provides HTTP handler tests
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"dev.helix.agent/internal/checkpoints"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckpointHandler_CreateCheckpoint(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// Create a temporary directory for checkpoints
	tempDir := t.TempDir()

	// Initialize git repo
	os.MkdirAll(filepath.Join(tempDir, ".git"), 0755)

	manager, err := checkpoints.NewManager(tempDir)
	require.NoError(t, err)

	handler := NewCheckpointHandler(manager)
	router := gin.New()
	handler.RegisterRoutes(router)

	createReq := CreateCheckpointRequest{
		Name:        "test-checkpoint",
		Description: "Test checkpoint for unit testing",
		Tags:        []string{"test", "unit"},
	}

	body, _ := json.Marshal(createReq)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/checkpoints", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp CreateCheckpointResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "test-checkpoint", resp.Name)
	assert.NotEmpty(t, resp.ID)
}

func TestCheckpointHandler_ListCheckpoints(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tempDir := t.TempDir()
	os.MkdirAll(filepath.Join(tempDir, ".git"), 0755)

	manager, err := checkpoints.NewManager(tempDir)
	require.NoError(t, err)

	// Create a checkpoint first
	_, err = manager.Create("test-checkpoint", "Test", []string{})
	require.NoError(t, err)

	handler := NewCheckpointHandler(manager)
	router := gin.New()
	handler.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/checkpoints", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListCheckpointsResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(resp.Checkpoints), 1)
}

func TestCheckpointHandler_DeleteCheckpoint(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tempDir := t.TempDir()
	os.MkdirAll(filepath.Join(tempDir, ".git"), 0755)

	manager, err := checkpoints.NewManager(tempDir)
	require.NoError(t, err)

	// Create a checkpoint first
	cp, err := manager.Create("test-checkpoint", "Test", []string{})
	require.NoError(t, err)

	handler := NewCheckpointHandler(manager)
	router := gin.New()
	handler.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/v1/checkpoints/"+cp.ID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify it's deleted
	checkpoints, err := manager.List()
	require.NoError(t, err)
	for _, c := range checkpoints {
		assert.NotEqual(t, cp.ID, c.ID)
	}
}
