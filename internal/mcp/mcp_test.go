package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Test Helpers and Mocks
// ============================================================================

// MockMCPTransport implements MCPTransportInterface for testing
type MockMCPTransport struct {
	connected      bool
	sendError      error
	receiveError   error
	receiveData    interface{}
	sentMessages   []interface{}
	mu             sync.Mutex
	sendCallCount  int
	closeCallCount int
}

func NewMockMCPTransport() *MockMCPTransport {
	return &MockMCPTransport{
		connected:    true,
		sentMessages: make([]interface{}, 0),
	}
}

func (m *MockMCPTransport) Send(ctx context.Context, message interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sendCallCount++
	if m.sendError != nil {
		return m.sendError
	}
	m.sentMessages = append(m.sentMessages, message)
	return nil
}

func (m *MockMCPTransport) Receive(ctx context.Context) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.receiveError != nil {
		return nil, m.receiveError
	}
	if m.receiveData != nil {
		return m.receiveData, nil
	}
	// Return a valid MCP initialize response by default
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"serverInfo": map[string]string{
				"name":    "mock-server",
				"version": "1.0.0",
			},
		},
	}, nil
}

func (m *MockMCPTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeCallCount++
	m.connected = false
	return nil
}

func (m *MockMCPTransport) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

func (m *MockMCPTransport) GetSentMessages() []interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sentMessages
}

// createTestLogger creates a logger for testing (discards output)
func createTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return logger
}

// createTempDir creates a temporary directory for testing
func createTempDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "mcp_test_*")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	return dir
}

// ============================================================================
// MCPPackage Tests
// ============================================================================

func TestMCPPackage(t *testing.T) {
	t.Run("StandardMCPPackages contains expected packages", func(t *testing.T) {
		assert.NotEmpty(t, StandardMCPPackages)
		assert.GreaterOrEqual(t, len(StandardMCPPackages), 6)

		// Check for specific standard packages
		packageNames := make(map[string]bool)
		for _, pkg := range StandardMCPPackages {
			packageNames[pkg.Name] = true
		}

		assert.True(t, packageNames["filesystem"], "filesystem package should exist")
		assert.True(t, packageNames["github"], "github package should exist")
		assert.True(t, packageNames["memory"], "memory package should exist")
		assert.True(t, packageNames["fetch"], "fetch package should exist")
		assert.True(t, packageNames["puppeteer"], "puppeteer package should exist")
		assert.True(t, packageNames["sqlite"], "sqlite package should exist")
	})

	t.Run("Package structure is valid", func(t *testing.T) {
		for _, pkg := range StandardMCPPackages {
			assert.NotEmpty(t, pkg.Name, "Package name should not be empty")
			assert.NotEmpty(t, pkg.NPM, "NPM field should not be empty")
			assert.NotEmpty(t, pkg.Description, "Description should not be empty")
		}
	})
}

// ============================================================================
// InstallStatus Tests
// ============================================================================

func TestInstallStatus(t *testing.T) {
	t.Run("Status constants are defined", func(t *testing.T) {
		assert.Equal(t, InstallStatus("pending"), StatusPending)
		assert.Equal(t, InstallStatus("installing"), StatusInstalling)
		assert.Equal(t, InstallStatus("installed"), StatusInstalled)
		assert.Equal(t, InstallStatus("failed"), StatusFailed)
		assert.Equal(t, InstallStatus("unavailable"), StatusUnavailable)
	})
}

// ============================================================================
// PackageStatus Tests
// ============================================================================

func TestPackageStatus(t *testing.T) {
	t.Run("PackageStatus fields are accessible", func(t *testing.T) {
		pkg := MCPPackage{
			Name:        "test-pkg",
			NPM:         "test-npm-pkg",
			Description: "Test package",
		}
		status := PackageStatus{
			Package:     pkg,
			Status:      StatusInstalled,
			InstallPath: "/path/to/install",
			InstalledAt: time.Now(),
			Error:       nil,
			Duration:    5 * time.Second,
		}

		assert.Equal(t, "test-pkg", status.Package.Name)
		assert.Equal(t, StatusInstalled, status.Status)
		assert.Equal(t, "/path/to/install", status.InstallPath)
		assert.Nil(t, status.Error)
		assert.Equal(t, 5*time.Second, status.Duration)
	})
}

// ============================================================================
// MCPPreinstaller Tests
// ============================================================================

func TestNewPreinstaller(t *testing.T) {
	t.Run("Creates preinstaller with default config", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)

		require.NoError(t, err)
		assert.NotNil(t, preinstaller)
		assert.NotEmpty(t, preinstaller.installDir)
		assert.Equal(t, StandardMCPPackages, preinstaller.packages)
		assert.NotNil(t, preinstaller.logger)
		assert.Equal(t, 4, preinstaller.concurrency)
		assert.Equal(t, 5*time.Minute, preinstaller.timeout)
	})

	t.Run("Creates preinstaller with custom install dir", func(t *testing.T) {
		tempDir := createTempDir(t)
		config := PreinstallerConfig{
			InstallDir: tempDir,
		}
		preinstaller, err := NewPreinstaller(config)

		require.NoError(t, err)
		assert.Equal(t, tempDir, preinstaller.installDir)
	})

	t.Run("Creates preinstaller with custom packages", func(t *testing.T) {
		customPkgs := []MCPPackage{
			{Name: "custom", NPM: "custom-pkg", Description: "Custom package"},
		}
		config := PreinstallerConfig{
			Packages: customPkgs,
		}
		preinstaller, err := NewPreinstaller(config)

		require.NoError(t, err)
		assert.Equal(t, customPkgs, preinstaller.packages)
	})

	t.Run("Creates preinstaller with custom logger", func(t *testing.T) {
		logger := createTestLogger()
		config := PreinstallerConfig{
			Logger: logger,
		}
		preinstaller, err := NewPreinstaller(config)

		require.NoError(t, err)
		assert.Equal(t, logger, preinstaller.logger)
	})

	t.Run("Creates preinstaller with custom concurrency", func(t *testing.T) {
		config := PreinstallerConfig{
			Concurrency: 8,
		}
		preinstaller, err := NewPreinstaller(config)

		require.NoError(t, err)
		assert.Equal(t, 8, preinstaller.concurrency)
	})

	t.Run("Creates preinstaller with custom timeout", func(t *testing.T) {
		config := PreinstallerConfig{
			Timeout: 10 * time.Minute,
		}
		preinstaller, err := NewPreinstaller(config)

		require.NoError(t, err)
		assert.Equal(t, 10*time.Minute, preinstaller.timeout)
	})

	t.Run("Creates preinstaller with progress callback", func(t *testing.T) {
		var callCount int
		onProgress := func(pkg string, status InstallStatus, progress float64) {
			callCount++
		}
		config := PreinstallerConfig{
			OnProgress: onProgress,
		}
		preinstaller, err := NewPreinstaller(config)

		require.NoError(t, err)
		assert.NotNil(t, preinstaller.onProgress)
	})

	t.Run("Initializes statuses for all packages", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)

		require.NoError(t, err)
		assert.Equal(t, len(StandardMCPPackages), preinstaller.statuses.Len())

		for _, pkg := range StandardMCPPackages {
			status, ok := preinstaller.statuses.Get(pkg.Name)
			assert.True(t, ok, "Status should exist for package %s", pkg.Name)
			assert.Equal(t, StatusPending, status.Status)
		}
	})

	t.Run("Handles negative concurrency", func(t *testing.T) {
		config := PreinstallerConfig{
			Concurrency: -1,
		}
		preinstaller, err := NewPreinstaller(config)

		require.NoError(t, err)
		assert.Equal(t, 4, preinstaller.concurrency) // Default value
	})

	t.Run("Handles zero timeout", func(t *testing.T) {
		config := PreinstallerConfig{
			Timeout: 0,
		}
		preinstaller, err := NewPreinstaller(config)

		require.NoError(t, err)
		assert.Equal(t, 5*time.Minute, preinstaller.timeout) // Default value
	})
}

func TestPreinstaller_IsInstalled(t *testing.T) {
	t.Run("Returns false for pending package", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		result := preinstaller.IsInstalled("filesystem")
		assert.False(t, result)
	})

	t.Run("Returns true for installed package", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		// Manually set status to installed
		mutatePackageStatusForTest(preinstaller, "filesystem", func(s *PackageStatus) {
			s.Status = StatusInstalled
		})

		result := preinstaller.IsInstalled("filesystem")
		assert.True(t, result)
	})

	t.Run("Returns false for unknown package", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		result := preinstaller.IsInstalled("unknown-package")
		assert.False(t, result)
	})
}

func TestPreinstaller_GetStatus(t *testing.T) {
	t.Run("Returns status for known package", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		status, err := preinstaller.GetStatus("filesystem")
		require.NoError(t, err)
		assert.Equal(t, StatusPending, status.Status)
		assert.Equal(t, "filesystem", status.Package.Name)
	})

	t.Run("Returns error for unknown package", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		status, err := preinstaller.GetStatus("unknown-package")
		assert.Error(t, err)
		assert.Nil(t, status)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Returns copy of status", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		status1, err := preinstaller.GetStatus("filesystem")
		require.NoError(t, err)

		status1.Status = StatusInstalled

		status2, err := preinstaller.GetStatus("filesystem")
		require.NoError(t, err)

		// Original should still be pending
		assert.Equal(t, StatusPending, status2.Status)
	})
}

func TestPreinstaller_GetAllStatuses(t *testing.T) {
	t.Run("Returns all statuses", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		statuses := preinstaller.GetAllStatuses()
		assert.Equal(t, len(StandardMCPPackages), len(statuses))

		for _, pkg := range StandardMCPPackages {
			status, ok := statuses[pkg.Name]
			assert.True(t, ok)
			assert.Equal(t, StatusPending, status.Status)
		}
	})

	t.Run("Returns copies of statuses", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		statuses := preinstaller.GetAllStatuses()
		statuses["filesystem"].Status = StatusInstalled

		// Original should still be pending
		originalStatus, _ := preinstaller.GetStatus("filesystem")
		assert.Equal(t, StatusPending, originalStatus.Status)
	})
}

func TestPreinstaller_GetInstalledPath(t *testing.T) {
	t.Run("Returns error for not installed package", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		path, err := preinstaller.GetInstalledPath("filesystem")
		assert.Error(t, err)
		assert.Empty(t, path)
		assert.Contains(t, err.Error(), "not installed")
	})

	t.Run("Returns error for unknown package", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		path, err := preinstaller.GetInstalledPath("unknown-package")
		assert.Error(t, err)
		assert.Empty(t, path)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Returns path for installed package", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		// Manually set status to installed with path
		mutatePackageStatusForTest(preinstaller, "filesystem", func(s *PackageStatus) {
			s.Status = StatusInstalled
			s.InstallPath = "/path/to/install"
		})

		path, err := preinstaller.GetInstalledPath("filesystem")
		require.NoError(t, err)
		assert.Equal(t, "/path/to/install", path)
	})
}

func TestPreinstaller_IsNodeAvailable(t *testing.T) {
	t.Run("Returns true when all tools available", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		// Manually set paths
		preinstaller.nodePath = "/usr/bin/node"
		preinstaller.npmPath = "/usr/bin/npm"
		preinstaller.npxPath = "/usr/bin/npx"

		assert.True(t, preinstaller.IsNodeAvailable())
	})

	t.Run("Returns false when node missing", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		preinstaller.nodePath = ""
		preinstaller.npmPath = "/usr/bin/npm"
		preinstaller.npxPath = "/usr/bin/npx"

		assert.False(t, preinstaller.IsNodeAvailable())
	})

	t.Run("Returns false when npm missing", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		preinstaller.nodePath = "/usr/bin/node"
		preinstaller.npmPath = ""
		preinstaller.npxPath = "/usr/bin/npx"

		assert.False(t, preinstaller.IsNodeAvailable())
	})

	t.Run("Returns false when npx missing", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		preinstaller.nodePath = "/usr/bin/node"
		preinstaller.npmPath = "/usr/bin/npm"
		preinstaller.npxPath = ""

		assert.False(t, preinstaller.IsNodeAvailable())
	})
}

func TestPreinstaller_GetInstallDir(t *testing.T) {
	t.Run("Returns configured install dir", func(t *testing.T) {
		tempDir := createTempDir(t)
		config := PreinstallerConfig{
			InstallDir: tempDir,
		}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		assert.Equal(t, tempDir, preinstaller.GetInstallDir())
	})
}

func TestPreinstaller_Cleanup(t *testing.T) {
	t.Run("Removes install directory", func(t *testing.T) {
		tempDir := createTempDir(t)
		config := PreinstallerConfig{
			InstallDir: tempDir,
		}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		// Create a test file in the directory
		testFile := filepath.Join(tempDir, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		// Cleanup
		err = preinstaller.Cleanup()
		require.NoError(t, err)

		// Verify directory is gone
		_, err = os.Stat(tempDir)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestPreinstaller_WaitForPackage(t *testing.T) {
	t.Run("Returns immediately for installed package", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		// Manually set status to installed
		mutatePackageStatusForTest(preinstaller, "filesystem", func(s *PackageStatus) {
			s.Status = StatusInstalled
		})

		ctx := context.Background()
		err = preinstaller.WaitForPackage(ctx, "filesystem")
		assert.NoError(t, err)
	})

	t.Run("Returns error for failed package", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		// Manually set status to failed
		mutatePackageStatusForTest(preinstaller, "filesystem", func(s *PackageStatus) {
			s.Status = StatusFailed
			s.Error = fmt.Errorf("install failed")
		})

		ctx := context.Background()
		err = preinstaller.WaitForPackage(ctx, "filesystem")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to install")
	})

	t.Run("Returns error for unavailable package", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		// Manually set status to unavailable
		mutatePackageStatusForTest(preinstaller, "filesystem", func(s *PackageStatus) {
			s.Status = StatusUnavailable
		})

		ctx := context.Background()
		err = preinstaller.WaitForPackage(ctx, "filesystem")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unavailable")
	})

	t.Run("Returns error for unknown package", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		ctx := context.Background()
		err = preinstaller.WaitForPackage(ctx, "unknown-package")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Returns error on context cancellation", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err = preinstaller.WaitForPackage(ctx, "filesystem")
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("Waits and returns when package becomes installed", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Update status in background after a short delay
		go func() {
			time.Sleep(200 * time.Millisecond)
			mutatePackageStatusForTest(preinstaller, "filesystem", func(s *PackageStatus) {
				s.Status = StatusInstalled
			})
		}()

		err = preinstaller.WaitForPackage(ctx, "filesystem")
		assert.NoError(t, err)
	})
}

func TestPreinstaller_PreInstallAll(t *testing.T) {
	t.Run("Skips when npm not available", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		// Clear npm path
		preinstaller.npmPath = ""

		ctx := context.Background()
		err = preinstaller.PreInstallAll(ctx)
		assert.NoError(t, err) // Should not error, just skip
	})

	t.Run("Respects context cancellation", func(t *testing.T) {
		tempDir := createTempDir(t)
		config := PreinstallerConfig{
			InstallDir: tempDir,
		}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		// Make sure npm is available for this test
		if preinstaller.npmPath == "" {
			t.Skip("npm not available")
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err = preinstaller.PreInstallAll(ctx)
		// May or may not error depending on timing
	})
}

func TestPreinstaller_calculateProgress(t *testing.T) {
	t.Run("Returns 0 for all pending", func(t *testing.T) {
		config := PreinstallerConfig{
			Packages: []MCPPackage{
				{Name: "pkg1"},
				{Name: "pkg2"},
			},
		}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		progress := preinstaller.calculateProgress()
		assert.Equal(t, 0.0, progress)
	})

	t.Run("Returns 0.5 for half completed", func(t *testing.T) {
		config := PreinstallerConfig{
			Packages: []MCPPackage{
				{Name: "pkg1"},
				{Name: "pkg2"},
			},
		}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		mutatePackageStatusForTest(preinstaller, "pkg1", func(s *PackageStatus) {
			s.Status = StatusInstalled
		})

		progress := preinstaller.calculateProgress()
		assert.Equal(t, 0.5, progress)
	})

	t.Run("Returns 1.0 for all completed", func(t *testing.T) {
		config := PreinstallerConfig{
			Packages: []MCPPackage{
				{Name: "pkg1"},
				{Name: "pkg2"},
			},
		}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		mutatePackageStatusForTest(preinstaller, "pkg1", func(s *PackageStatus) {
			s.Status = StatusInstalled
		})
		mutatePackageStatusForTest(preinstaller, "pkg2", func(s *PackageStatus) {
			s.Status = StatusFailed
		})

		progress := preinstaller.calculateProgress()
		assert.Equal(t, 1.0, progress)
	})

	t.Run("Counts unavailable as completed", func(t *testing.T) {
		config := PreinstallerConfig{
			Packages: []MCPPackage{
				{Name: "pkg1"},
			},
		}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		mutatePackageStatusForTest(preinstaller, "pkg1", func(s *PackageStatus) {
			s.Status = StatusUnavailable
		})

		progress := preinstaller.calculateProgress()
		assert.Equal(t, 1.0, progress)
	})
}

func TestPreinstaller_updateStatus(t *testing.T) {
	t.Run("Updates status correctly", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		preinstaller.updateStatus("filesystem", StatusInstalled, "/path/to/install", nil)

		status, _ := preinstaller.GetStatus("filesystem")
		assert.Equal(t, StatusInstalled, status.Status)
		assert.Equal(t, "/path/to/install", status.InstallPath)
	})

	t.Run("Calls progress callback", func(t *testing.T) {
		var callCount int
		var lastStatus InstallStatus
		var lastProgress float64

		config := PreinstallerConfig{
			OnProgress: func(pkg string, status InstallStatus, progress float64) {
				callCount++
				lastStatus = status
				lastProgress = progress
			},
		}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		preinstaller.updateStatus("filesystem", StatusInstalled, "", nil)

		assert.Equal(t, 1, callCount)
		assert.Equal(t, StatusInstalled, lastStatus)
		assert.Greater(t, lastProgress, 0.0)
	})

	t.Run("Sets error correctly", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		testErr := fmt.Errorf("test error")
		preinstaller.updateStatus("filesystem", StatusFailed, "", testErr)

		status, _ := preinstaller.GetStatus("filesystem")
		assert.Equal(t, StatusFailed, status.Status)
		assert.Equal(t, testErr, status.Error)
	})
}

func TestPreinstaller_isPackageInstalled(t *testing.T) {
	t.Run("Returns false for non-existent directory", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		result := preinstaller.isPackageInstalled("/non/existent/path", "some-package")
		assert.False(t, result)
	})

	t.Run("Returns false when package.json missing", func(t *testing.T) {
		tempDir := createTempDir(t)
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		// Create node_modules but no package.json
		nodeModules := filepath.Join(tempDir, "node_modules", "test-package")
		_ = os.MkdirAll(nodeModules, 0755)

		result := preinstaller.isPackageInstalled(tempDir, "test-package")
		assert.False(t, result)
	})

	t.Run("Returns true when package.json exists", func(t *testing.T) {
		tempDir := createTempDir(t)
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		// Create node_modules with package.json
		pkgDir := filepath.Join(tempDir, "node_modules", "test-package")
		_ = os.MkdirAll(pkgDir, 0755)
		_ = os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{}`), 0644)

		result := preinstaller.isPackageInstalled(tempDir, "test-package")
		assert.True(t, result)
	})

	t.Run("Handles scoped packages", func(t *testing.T) {
		tempDir := createTempDir(t)
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		// Create scoped package
		pkgDir := filepath.Join(tempDir, "node_modules", "@modelcontextprotocol", "server-filesystem")
		_ = os.MkdirAll(pkgDir, 0755)
		_ = os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{}`), 0644)

		result := preinstaller.isPackageInstalled(tempDir, "@modelcontextprotocol/server-filesystem")
		assert.True(t, result)
	})
}

func TestPreinstaller_GetPackageCommand(t *testing.T) {
	t.Run("Returns error for unknown package", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		cmd, err := preinstaller.GetPackageCommand("unknown-package")
		assert.Error(t, err)
		assert.Nil(t, cmd)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Returns error for not installed package", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		cmd, err := preinstaller.GetPackageCommand("filesystem")
		assert.Error(t, err)
		assert.Nil(t, cmd)
		assert.Contains(t, err.Error(), "not installed")
	})
}

// ============================================================================
// ConnectionStatus Tests
// ============================================================================

func TestConnectionStatus(t *testing.T) {
	t.Run("Status constants are defined", func(t *testing.T) {
		assert.Equal(t, ConnectionStatus("pending"), StatusConnectionPending)
		assert.Equal(t, ConnectionStatus("connecting"), StatusConnectionConnecting)
		assert.Equal(t, ConnectionStatus("connected"), StatusConnectionConnected)
		assert.Equal(t, ConnectionStatus("failed"), StatusConnectionFailed)
		assert.Equal(t, ConnectionStatus("closed"), StatusConnectionClosed)
	})
}

// ============================================================================
// MCPServerType Tests
// ============================================================================

func TestMCPServerType(t *testing.T) {
	t.Run("Server type constants are defined", func(t *testing.T) {
		assert.Equal(t, MCPServerType("local"), MCPServerTypeLocal)
		assert.Equal(t, MCPServerType("remote"), MCPServerTypeRemote)
	})
}

// ============================================================================
// MCPPoolConfig Tests
// ============================================================================

func TestDefaultPoolConfig(t *testing.T) {
	t.Run("Returns valid default config", func(t *testing.T) {
		config := DefaultPoolConfig()

		assert.Equal(t, 12, config.MaxConnections)
		assert.Equal(t, 30*time.Second, config.ConnectionTimeout)
		assert.Equal(t, 5*time.Minute, config.IdleTimeout)
		assert.Equal(t, 30*time.Second, config.HealthCheckPeriod)
		assert.Equal(t, 3, config.RetryAttempts)
		assert.Equal(t, 1*time.Second, config.RetryDelay)
		assert.False(t, config.WarmUpOnStart)
		assert.Nil(t, config.WarmUpServers)
	})
}

// ============================================================================
// MCPConnectionPool Tests
// ============================================================================

func TestNewConnectionPool(t *testing.T) {
	t.Run("Creates pool with default config", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, nil)

		assert.NotNil(t, pool)
		assert.NotNil(t, pool.connections)
		assert.NotNil(t, pool.config)
		assert.NotNil(t, pool.logger)
		assert.NotNil(t, pool.metrics)
		assert.False(t, pool.closed.Load())
	})

	t.Run("Creates pool with custom config", func(t *testing.T) {
		config := &MCPPoolConfig{
			MaxConnections: 20,
			RetryAttempts:  5,
		}
		pool := NewConnectionPool(nil, config, nil)

		assert.Equal(t, 20, pool.config.MaxConnections)
		assert.Equal(t, 5, pool.config.RetryAttempts)
	})

	t.Run("Creates pool with custom logger", func(t *testing.T) {
		logger := createTestLogger()
		pool := NewConnectionPool(nil, nil, logger)

		assert.Equal(t, logger, pool.logger)
	})

	t.Run("Creates pool with preinstaller", func(t *testing.T) {
		preConfig := PreinstallerConfig{}
		preinstaller, _ := NewPreinstaller(preConfig)

		pool := NewConnectionPool(preinstaller, nil, nil)
		assert.Equal(t, preinstaller, pool.preinstaller)
	})
}

func TestConnectionPool_RegisterServer(t *testing.T) {
	t.Run("Registers server successfully", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		config := MCPServerConfig{
			Name:    "test-server",
			Type:    MCPServerTypeRemote,
			URL:     "http://localhost:8080",
			Enabled: true,
		}

		err := pool.RegisterServer(config)
		require.NoError(t, err)

		servers := pool.ListServers()
		assert.Contains(t, servers, "test-server")
	})

	t.Run("Returns error for duplicate server", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		config := MCPServerConfig{
			Name: "test-server",
			Type: MCPServerTypeRemote,
		}

		err := pool.RegisterServer(config)
		require.NoError(t, err)

		err = pool.RegisterServer(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("Returns error when pool is closed", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())
		_ = pool.Close()

		config := MCPServerConfig{
			Name: "test-server",
			Type: MCPServerTypeRemote,
		}

		err := pool.RegisterServer(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "pool is closed")
	})

	t.Run("Sets default timeout from pool config", func(t *testing.T) {
		poolConfig := &MCPPoolConfig{
			ConnectionTimeout: 60 * time.Second,
		}
		pool := NewConnectionPool(nil, poolConfig, createTestLogger())

		config := MCPServerConfig{
			Name: "test-server",
			Type: MCPServerTypeRemote,
			// Timeout not set
		}

		err := pool.RegisterServer(config)
		require.NoError(t, err)

		conn, _ := pool.connections.Get("test-server")
		assert.Equal(t, 60*time.Second, conn.Config.Timeout)
	})

	t.Run("Sets connection status to pending", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		config := MCPServerConfig{
			Name: "test-server",
			Type: MCPServerTypeRemote,
		}

		err := pool.RegisterServer(config)
		require.NoError(t, err)

		conn, _ := pool.connections.Get("test-server")
		assert.Equal(t, StatusConnectionPending, conn.Status)
	})
}

func TestConnectionPool_GetConnection(t *testing.T) {
	t.Run("Returns error for unregistered server", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		ctx := context.Background()
		conn, err := pool.GetConnection(ctx, "unknown-server")
		assert.Error(t, err)
		assert.Nil(t, conn)
		assert.Contains(t, err.Error(), "not registered")
	})
}

func TestConnectionPool_ListServers(t *testing.T) {
	t.Run("Returns empty list for new pool", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		servers := pool.ListServers()
		assert.Empty(t, servers)
	})

	t.Run("Returns registered servers", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		_ = pool.RegisterServer(MCPServerConfig{Name: "server1", Type: MCPServerTypeRemote})
		_ = pool.RegisterServer(MCPServerConfig{Name: "server2", Type: MCPServerTypeLocal})

		servers := pool.ListServers()
		assert.Len(t, servers, 2)
		assert.Contains(t, servers, "server1")
		assert.Contains(t, servers, "server2")
	})
}

func TestConnectionPool_GetServerStatus(t *testing.T) {
	t.Run("Returns error for unknown server", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		status, err := pool.GetServerStatus("unknown-server")
		assert.Error(t, err)
		assert.Empty(t, status)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Returns status for registered server", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		_ = pool.RegisterServer(MCPServerConfig{Name: "test-server", Type: MCPServerTypeRemote})

		status, err := pool.GetServerStatus("test-server")
		require.NoError(t, err)
		assert.Equal(t, StatusConnectionPending, status)
	})
}

func TestConnectionPool_CloseConnection(t *testing.T) {
	t.Run("Returns error for unknown server", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		err := pool.CloseConnection("unknown-server")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Closes registered connection", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		_ = pool.RegisterServer(MCPServerConfig{Name: "test-server", Type: MCPServerTypeRemote})

		// Manually set up a mock transport
		conn, _ := pool.connections.Get("test-server")
		conn.Transport = NewMockMCPTransport()
		conn.Status = StatusConnectionConnected

		err := pool.CloseConnection("test-server")
		require.NoError(t, err)

		status, _ := pool.GetServerStatus("test-server")
		assert.Equal(t, StatusConnectionClosed, status)
	})
}

func TestConnectionPool_Close(t *testing.T) {
	t.Run("Closes all connections", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		_ = pool.RegisterServer(MCPServerConfig{Name: "server1", Type: MCPServerTypeRemote})
		_ = pool.RegisterServer(MCPServerConfig{Name: "server2", Type: MCPServerTypeRemote})

		err := pool.Close()
		require.NoError(t, err)

		assert.True(t, pool.closed.Load())
	})

	t.Run("Can be called multiple times", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		err := pool.Close()
		require.NoError(t, err)

		err = pool.Close()
		require.NoError(t, err)
	})
}

func TestConnectionPool_HealthCheck(t *testing.T) {
	t.Run("Returns empty map for pool with no connections", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		ctx := context.Background()
		results := pool.HealthCheck(ctx)
		assert.Empty(t, results)
	})

	t.Run("Returns false for pending connections", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		_ = pool.RegisterServer(MCPServerConfig{Name: "test-server", Type: MCPServerTypeRemote})

		ctx := context.Background()
		results := pool.HealthCheck(ctx)

		assert.False(t, results["test-server"])
	})

	t.Run("Returns true for connected connections with transport", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		_ = pool.RegisterServer(MCPServerConfig{Name: "test-server", Type: MCPServerTypeRemote})

		// Manually set up connection
		conn, _ := pool.connections.Get("test-server")
		conn.Status = StatusConnectionConnected
		conn.Transport = NewMockMCPTransport()

		ctx := context.Background()
		results := pool.HealthCheck(ctx)

		assert.True(t, results["test-server"])
	})
}

func TestConnectionPool_GetMetrics(t *testing.T) {
	t.Run("Returns initial metrics", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		metrics := pool.GetMetrics()

		assert.NotNil(t, metrics)
		assert.Equal(t, int64(0), metrics.TotalConnections)
		assert.Equal(t, int64(0), metrics.ActiveConnections)
		assert.Equal(t, int64(0), metrics.FailedConnections)
		assert.Equal(t, int64(0), metrics.TotalRequests)
	})

	t.Run("Returns updated metrics after changes", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		// Manually update metrics
		atomic.AddInt64(&pool.metrics.TotalConnections, 5)
		atomic.AddInt64(&pool.metrics.ActiveConnections, 3)
		atomic.AddInt64(&pool.metrics.FailedConnections, 1)

		metrics := pool.GetMetrics()

		assert.Equal(t, int64(5), metrics.TotalConnections)
		assert.Equal(t, int64(3), metrics.ActiveConnections)
		assert.Equal(t, int64(1), metrics.FailedConnections)
	})
}

func TestConnectionPool_WarmUp(t *testing.T) {
	t.Run("Warms up specified servers", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		_ = pool.RegisterServer(MCPServerConfig{
			Name: "server1",
			Type: MCPServerTypeRemote,
			URL:  "http://localhost:8080",
		})

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Will fail because there's no actual server, but tests the flow
		_ = pool.WarmUp(ctx, []string{"server1"})
	})

	t.Run("Uses config warmup servers when no servers specified", func(t *testing.T) {
		config := &MCPPoolConfig{
			WarmUpServers: []string{"server1"},
		}
		pool := NewConnectionPool(nil, config, createTestLogger())

		_ = pool.RegisterServer(MCPServerConfig{
			Name: "server1",
			Type: MCPServerTypeRemote,
			URL:  "http://localhost:8080",
		})

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Will fail but tests the warmup server selection logic
		_ = pool.WarmUp(ctx, nil)
	})
}

// ============================================================================
// StdioMCPTransport Tests
// ============================================================================

func TestStdioMCPTransport_Send(t *testing.T) {
	t.Run("Returns error when not connected", func(t *testing.T) {
		transport := &StdioMCPTransport{
			connected: false,
		}

		ctx := context.Background()
		err := transport.Send(ctx, map[string]string{"test": "data"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
	})

	t.Run("Sends JSON message with newline", func(t *testing.T) {
		// Create a pipe to capture output
		reader, writer := io.Pipe()
		defer func() { _ = reader.Close() }()

		transport := &StdioMCPTransport{
			stdin:     writer,
			connected: true,
		}

		message := map[string]string{"test": "data"}

		// Read in background
		go func() {
			buf := make([]byte, 1024)
			n, _ := reader.Read(buf)
			data := buf[:n]

			// Verify it ends with newline
			assert.Equal(t, byte('\n'), data[len(data)-1])

			// Verify it's valid JSON
			var parsed map[string]string
			err := json.Unmarshal(data[:len(data)-1], &parsed)
			assert.NoError(t, err)
			assert.Equal(t, "data", parsed["test"])
		}()

		ctx := context.Background()
		err := transport.Send(ctx, message)
		require.NoError(t, err)
	})
}

func TestStdioMCPTransport_Receive(t *testing.T) {
	t.Run("Returns error when not connected", func(t *testing.T) {
		transport := &StdioMCPTransport{
			connected: false,
		}

		ctx := context.Background()
		msg, err := transport.Receive(ctx)
		assert.Error(t, err)
		assert.Nil(t, msg)
		assert.Contains(t, err.Error(), "not connected")
	})

	t.Run("Receives and parses JSON message", func(t *testing.T) {
		reader := strings.NewReader(`{"jsonrpc":"2.0","id":1,"result":{"status":"ok"}}` + "\n")

		transport := &StdioMCPTransport{
			stdout:    io.NopCloser(reader),
			scanner:   bufio.NewScanner(reader),
			connected: true,
		}

		ctx := context.Background()
		msg, err := transport.Receive(ctx)
		require.NoError(t, err)

		result, ok := msg.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "2.0", result["jsonrpc"])
	})

	t.Run("Returns EOF when no more data", func(t *testing.T) {
		reader := strings.NewReader("")

		transport := &StdioMCPTransport{
			stdout:    io.NopCloser(reader),
			scanner:   bufio.NewScanner(reader),
			connected: true,
		}

		ctx := context.Background()
		msg, err := transport.Receive(ctx)
		assert.Equal(t, io.EOF, err)
		assert.Nil(t, msg)
	})
}

func TestStdioMCPTransport_Close(t *testing.T) {
	t.Run("Sets connected to false", func(t *testing.T) {
		transport := &StdioMCPTransport{
			connected: true,
		}

		err := transport.Close()
		require.NoError(t, err)
		assert.False(t, transport.connected)
	})

	t.Run("Closes stdin if set", func(t *testing.T) {
		_, writer := io.Pipe()

		transport := &StdioMCPTransport{
			stdin:     writer,
			connected: true,
		}

		err := transport.Close()
		require.NoError(t, err)
		assert.False(t, transport.connected)
	})
}

func TestStdioMCPTransport_IsConnected(t *testing.T) {
	t.Run("Returns connected state", func(t *testing.T) {
		transport := &StdioMCPTransport{
			connected: true,
		}
		assert.True(t, transport.IsConnected())

		transport.connected = false
		assert.False(t, transport.IsConnected())
	})
}

// ============================================================================
// HTTPMCPTransport Tests
// ============================================================================

func TestHTTPMCPTransport_Send(t *testing.T) {
	t.Run("Returns error when not connected", func(t *testing.T) {
		transport := &HTTPMCPTransport{}

		ctx := context.Background()
		err := transport.Send(ctx, map[string]string{"test": "data"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
	})

	t.Run("Sends HTTP POST request", func(t *testing.T) {
		var receivedBody []byte
		var receivedHeaders http.Header

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedBody, _ = io.ReadAll(r.Body)
			receivedHeaders = r.Header

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
		}))
		defer server.Close()

		transport := &HTTPMCPTransport{
			baseURL: server.URL,
			headers: safe.NewStoreFromMap(map[string]string{"X-Custom": "header"}),
			client:  &http.Client{},
		}
		transport.connected.Store(true)

		ctx := context.Background()
		message := map[string]string{"test": "data"}
		err := transport.Send(ctx, message)
		require.NoError(t, err)

		// Verify request
		assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"))
		assert.Equal(t, "header", receivedHeaders.Get("X-Custom"))

		var parsed map[string]string
		_ = json.Unmarshal(receivedBody, &parsed)
		assert.Equal(t, "data", parsed["test"])
	})

	t.Run("Returns error for non-2xx status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal error"}`))
		}))
		defer server.Close()

		transport := &HTTPMCPTransport{
			baseURL: server.URL,
			client:  &http.Client{},
		}
		transport.connected.Store(true)

		ctx := context.Background()
		err := transport.Send(ctx, map[string]string{"test": "data"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("Stores response data for Receive", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"value":"test"}}`))
		}))
		defer server.Close()

		transport := &HTTPMCPTransport{
			baseURL: server.URL,
			client:  &http.Client{},
		}
		transport.connected.Store(true)

		ctx := context.Background()
		err := transport.Send(ctx, map[string]string{})
		require.NoError(t, err)

		payload := transport.responseData.Load()
		require.NotNil(t, payload)
		assert.NotEmpty(t, *payload)
	})
}

func TestHTTPMCPTransport_Receive(t *testing.T) {
	t.Run("Returns error when not connected", func(t *testing.T) {
		transport := &HTTPMCPTransport{}

		ctx := context.Background()
		msg, err := transport.Receive(ctx)
		assert.Error(t, err)
		assert.Nil(t, msg)
		assert.Contains(t, err.Error(), "not connected")
	})

	t.Run("Returns error when no response data", func(t *testing.T) {
		transport := &HTTPMCPTransport{}
		transport.connected.Store(true)

		ctx := context.Background()
		msg, err := transport.Receive(ctx)
		assert.Error(t, err)
		assert.Nil(t, msg)
		assert.Contains(t, err.Error(), "no response data")
	})

	t.Run("Parses and returns response data", func(t *testing.T) {
		transport := &HTTPMCPTransport{}
		transport.connected.Store(true)
		payload := []byte(`{"jsonrpc":"2.0","id":1,"result":{"value":"test"}}`)
		transport.responseData.Store(&payload)

		ctx := context.Background()
		msg, err := transport.Receive(ctx)
		require.NoError(t, err)

		result, ok := msg.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "2.0", result["jsonrpc"])
	})

	t.Run("Clears response data after receive", func(t *testing.T) {
		transport := &HTTPMCPTransport{}
		transport.connected.Store(true)
		payload := []byte(`{"test":"data"}`)
		transport.responseData.Store(&payload)

		ctx := context.Background()
		_, err := transport.Receive(ctx)
		require.NoError(t, err)

		assert.Nil(t, transport.responseData.Load())
	})
}

func TestHTTPMCPTransport_Close(t *testing.T) {
	t.Run("Sets connected to false", func(t *testing.T) {
		transport := &HTTPMCPTransport{}
		transport.connected.Store(true)

		err := transport.Close()
		require.NoError(t, err)
		assert.False(t, transport.connected.Load())
	})
}

func TestHTTPMCPTransport_IsConnected(t *testing.T) {
	t.Run("Returns connected state", func(t *testing.T) {
		transport := &HTTPMCPTransport{}
		transport.connected.Store(true)
		assert.True(t, transport.IsConnected())

		transport.connected.Store(false)
		assert.False(t, transport.IsConnected())
	})
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestConnectionPool_WithMockTransport(t *testing.T) {
	t.Run("Full connection lifecycle with mock transport", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		// Register a remote server
		err := pool.RegisterServer(MCPServerConfig{
			Name:    "test-server",
			Type:    MCPServerTypeRemote,
			URL:     "http://localhost:8080",
			Enabled: true,
		})
		require.NoError(t, err)

		// Verify initial state
		status, err := pool.GetServerStatus("test-server")
		require.NoError(t, err)
		assert.Equal(t, StatusConnectionPending, status)

		// Check health (should be false - not connected)
		ctx := context.Background()
		health := pool.HealthCheck(ctx)
		assert.False(t, health["test-server"])

		// Close the connection
		err = pool.CloseConnection("test-server")
		require.NoError(t, err)

		status, _ = pool.GetServerStatus("test-server")
		assert.Equal(t, StatusConnectionClosed, status)
	})
}

func TestPreinstallerAndConnectionPool_Integration(t *testing.T) {
	t.Run("Pool uses preinstaller for package status", func(t *testing.T) {
		tempDir := createTempDir(t)

		preConfig := PreinstallerConfig{
			InstallDir: tempDir,
			Packages: []MCPPackage{
				{Name: "test-mcp", NPM: "test-mcp-pkg", Description: "Test MCP"},
			},
		}
		preinstaller, err := NewPreinstaller(preConfig)
		require.NoError(t, err)

		pool := NewConnectionPool(preinstaller, nil, createTestLogger())

		// Verify preinstaller is accessible through pool
		assert.NotNil(t, pool.preinstaller)
		assert.False(t, pool.preinstaller.IsInstalled("test-mcp"))
	})
}

// ============================================================================
// Concurrency Tests
// ============================================================================

func TestConnectionPool_ConcurrentAccess(t *testing.T) {
	t.Run("Handles concurrent server registration", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		var wg sync.WaitGroup
		errors := make(chan error, 100)

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				err := pool.RegisterServer(MCPServerConfig{
					Name: fmt.Sprintf("server-%d", idx),
					Type: MCPServerTypeRemote,
				})
				if err != nil && !strings.Contains(err.Error(), "already registered") {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("Unexpected error: %v", err)
		}

		servers := pool.ListServers()
		assert.Len(t, servers, 100)
	})

	t.Run("Handles concurrent health checks", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		for i := 0; i < 10; i++ {
			_ = pool.RegisterServer(MCPServerConfig{
				Name: fmt.Sprintf("server-%d", i),
				Type: MCPServerTypeRemote,
			})
		}

		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx := context.Background()
				_ = pool.HealthCheck(ctx)
			}()
		}

		wg.Wait()
	})

	t.Run("Handles concurrent status queries", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		_ = pool.RegisterServer(MCPServerConfig{
			Name: "test-server",
			Type: MCPServerTypeRemote,
		})

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = pool.GetServerStatus("test-server")
			}()
		}

		wg.Wait()
	})
}

func TestPreinstaller_ConcurrentAccess(t *testing.T) {
	t.Run("Handles concurrent status reads", func(t *testing.T) {
		config := PreinstallerConfig{}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = preinstaller.GetAllStatuses()
				_, _ = preinstaller.GetStatus("filesystem")
				_ = preinstaller.IsInstalled("filesystem")
			}()
		}

		wg.Wait()
	})
}

// ============================================================================
// Edge Case Tests
// ============================================================================

func TestEdgeCases(t *testing.T) {
	t.Run("Empty package list creates valid preinstaller", func(t *testing.T) {
		config := PreinstallerConfig{
			Packages: []MCPPackage{},
		}
		preinstaller, err := NewPreinstaller(config)

		require.NoError(t, err)
		assert.Equal(t, 0, preinstaller.statuses.Len())
	})

	t.Run("Pool metrics are thread-safe", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(2)
			go func() {
				defer wg.Done()
				atomic.AddInt64(&pool.metrics.TotalRequests, 1)
			}()
			go func() {
				defer wg.Done()
				_ = pool.GetMetrics()
			}()
		}

		wg.Wait()

		metrics := pool.GetMetrics()
		assert.Equal(t, int64(100), metrics.TotalRequests)
	})

	t.Run("Transport handles invalid JSON gracefully", func(t *testing.T) {
		reader := strings.NewReader("not valid json\n")

		transport := &StdioMCPTransport{
			stdout:    io.NopCloser(reader),
			scanner:   bufio.NewScanner(reader),
			connected: true,
		}

		ctx := context.Background()
		_, err := transport.Receive(ctx)
		assert.Error(t, err)
	})

	t.Run("HTTP transport handles connection errors", func(t *testing.T) {
		transport := &HTTPMCPTransport{
			baseURL: "http://localhost:99999", // Invalid port
			client:  &http.Client{Timeout: 100 * time.Millisecond},
		}
		transport.connected.Store(true)

		ctx := context.Background()
		err := transport.Send(ctx, map[string]string{})
		assert.Error(t, err)
	})

	t.Run("Preinstaller handles special characters in package names", func(t *testing.T) {
		config := PreinstallerConfig{
			Packages: []MCPPackage{
				{Name: "pkg-with-dash", NPM: "@scope/pkg-name", Description: "Test"},
			},
		}
		preinstaller, err := NewPreinstaller(config)
		require.NoError(t, err)

		status, err := preinstaller.GetStatus("pkg-with-dash")
		require.NoError(t, err)
		assert.Equal(t, "@scope/pkg-name", status.Package.NPM)
	})
}

// ============================================================================
// JSON Unmarshal Helper Test
// ============================================================================

func TestJsonUnmarshal(t *testing.T) {
	t.Run("Unmarshals valid JSON", func(t *testing.T) {
		data := []byte(`{"key": "value"}`)
		var result map[string]string

		err := jsonUnmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, "value", result["key"])
	})

	t.Run("Returns error for invalid JSON", func(t *testing.T) {
		data := []byte(`not json`)
		var result map[string]string

		err := jsonUnmarshal(data, &result)
		assert.Error(t, err)
	})
}

// ============================================================================
// MCPConnection Tests
// ============================================================================

func TestMCPConnection(t *testing.T) {
	t.Run("Connection fields are accessible", func(t *testing.T) {
		conn := &MCPConnection{
			Config: MCPServerConfig{
				Name: "test-server",
				Type: MCPServerTypeRemote,
				URL:  "http://localhost:8080",
			},
			Status:       StatusConnectionConnected,
			LastUsed:     time.Now(),
			ConnectedAt:  time.Now(),
			RequestCount: 10,
		}

		assert.Equal(t, "test-server", conn.Config.Name)
		assert.Equal(t, StatusConnectionConnected, conn.Status)
		assert.Equal(t, int64(10), conn.RequestCount)
	})
}

// ============================================================================
// MCPServerConfig Tests
// ============================================================================

func TestMCPServerConfig(t *testing.T) {
	t.Run("Local server config", func(t *testing.T) {
		config := MCPServerConfig{
			Name:    "local-server",
			Type:    MCPServerTypeLocal,
			Command: []string{"node", "server.js"},
			Environment: map[string]string{
				"NODE_ENV": "production",
			},
			Timeout: 30 * time.Second,
			Enabled: true,
		}

		assert.Equal(t, MCPServerTypeLocal, config.Type)
		assert.Len(t, config.Command, 2)
		assert.Equal(t, "production", config.Environment["NODE_ENV"])
	})

	t.Run("Remote server config", func(t *testing.T) {
		config := MCPServerConfig{
			Name: "remote-server",
			Type: MCPServerTypeRemote,
			URL:  "http://localhost:8080/mcp",
			Headers: map[string]string{
				"Authorization": "Bearer token",
			},
			Timeout: 60 * time.Second,
			Enabled: true,
		}

		assert.Equal(t, MCPServerTypeRemote, config.Type)
		assert.Equal(t, "http://localhost:8080/mcp", config.URL)
		assert.Equal(t, "Bearer token", config.Headers["Authorization"])
	})
}

// ============================================================================
// Pool Metrics Tests
// ============================================================================

func TestPoolMetrics(t *testing.T) {
	t.Run("Metrics struct is properly initialized", func(t *testing.T) {
		metrics := &PoolMetrics{
			TotalConnections:   10,
			ActiveConnections:  5,
			FailedConnections:  2,
			TotalRequests:      100,
			SuccessfulRequests: 95,
			FailedRequests:     5,
			AverageLatency:     1500,
		}

		assert.Equal(t, int64(10), metrics.TotalConnections)
		assert.Equal(t, int64(5), metrics.ActiveConnections)
		assert.Equal(t, int64(2), metrics.FailedConnections)
		assert.Equal(t, int64(100), metrics.TotalRequests)
		assert.Equal(t, int64(95), metrics.SuccessfulRequests)
		assert.Equal(t, int64(5), metrics.FailedRequests)
		assert.Equal(t, int64(1500), metrics.AverageLatency)
	})
}

// ============================================================================
// Transport Interface Compliance Tests
// ============================================================================

func TestTransportInterfaceCompliance(t *testing.T) {
	t.Run("StdioMCPTransport implements MCPTransportInterface", func(t *testing.T) {
		var _ MCPTransportInterface = (*StdioMCPTransport)(nil)
	})

	t.Run("HTTPMCPTransport implements MCPTransportInterface", func(t *testing.T) {
		var _ MCPTransportInterface = (*HTTPMCPTransport)(nil)
	})

	t.Run("MockMCPTransport implements MCPTransportInterface", func(t *testing.T) {
		var _ MCPTransportInterface = (*MockMCPTransport)(nil)
	})
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkConnectionPool_RegisterServer(b *testing.B) {
	pool := NewConnectionPool(nil, nil, createTestLogger())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pool.RegisterServer(MCPServerConfig{
			Name: fmt.Sprintf("server-%d", i),
			Type: MCPServerTypeRemote,
		})
	}
}

func BenchmarkConnectionPool_GetServerStatus(b *testing.B) {
	pool := NewConnectionPool(nil, nil, createTestLogger())
	_ = pool.RegisterServer(MCPServerConfig{
		Name: "test-server",
		Type: MCPServerTypeRemote,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pool.GetServerStatus("test-server")
	}
}

func BenchmarkPreinstaller_GetStatus(b *testing.B) {
	config := PreinstallerConfig{}
	preinstaller, _ := NewPreinstaller(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = preinstaller.GetStatus("filesystem")
	}
}

func BenchmarkPreinstaller_IsInstalled(b *testing.B) {
	config := PreinstallerConfig{}
	preinstaller, _ := NewPreinstaller(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		preinstaller.IsInstalled("filesystem")
	}
}

func BenchmarkPoolMetrics_AtomicOperations(b *testing.B) {
	pool := NewConnectionPool(nil, nil, createTestLogger())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			atomic.AddInt64(&pool.metrics.TotalRequests, 1)
			pool.GetMetrics()
		}
	})
}
