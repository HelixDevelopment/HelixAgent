package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
)

// MCPPackage represents an MCP npm package to be pre-installed
type MCPPackage struct {
	Name        string             // Short name for identification
	NPM         string             // NPM package name (e.g., "@modelcontextprotocol/server-filesystem")
	Binary      string             // Binary name after installation (optional, defaults to "index.js")
	Description string             // Package description
	Category    MCPPackageCategory // Package category (core, vectordb, design, image, dev, search, cloud)
	RequiresEnv []string           // Environment variables required for this package
	Optional    bool               // Whether this package is optional (won't fail if install fails)
}

// StandardMCPPackages defines the standard MCP packages to pre-install
var StandardMCPPackages = []MCPPackage{
	{
		Name:        "filesystem",
		NPM:         "@modelcontextprotocol/server-filesystem",
		Description: "MCP server for filesystem operations",
	},
	{
		Name:        "github",
		NPM:         "@modelcontextprotocol/server-github",
		Description: "MCP server for GitHub API operations",
	},
	{
		Name:        "memory",
		NPM:         "@modelcontextprotocol/server-memory",
		Description: "MCP server for memory/state management",
	},
	{
		Name:        "fetch",
		NPM:         "mcp-fetch-server",
		Description: "MCP server for HTTP fetch operations",
	},
	{
		Name:        "puppeteer",
		NPM:         "@modelcontextprotocol/server-puppeteer",
		Description: "MCP server for browser automation",
	},
	{
		Name:        "sqlite",
		NPM:         "mcp-server-sqlite",
		Description: "MCP server for SQLite database operations",
	},
}

// InstallStatus represents the installation status of a package
type InstallStatus string

const (
	StatusPending     InstallStatus = "pending"
	StatusInstalling  InstallStatus = "installing"
	StatusInstalled   InstallStatus = "installed"
	StatusFailed      InstallStatus = "failed"
	StatusUnavailable InstallStatus = "unavailable"
)

// PackageStatus tracks the installation status of a package
type PackageStatus struct {
	Package     MCPPackage
	Status      InstallStatus
	InstallPath string
	InstalledAt time.Time
	Error       error
	Duration    time.Duration
}

// MCPPreinstaller handles pre-installation of MCP npm packages.
//
// Concurrency model (CONST-029):
//   - statuses is a safe.Store; each individual entry read/write is
//     atomic. Mutations use Store.Update with a copy-on-write closure
//     so readers (via Store.Get) never see partially written fields.
//   - No Pattern-Zeta survivor mutex: every compound operation can be
//     expressed as a single Update on the entry's pointer, or as an
//     independent Get+onProgress callback that snapshots via Range.
type MCPPreinstaller struct {
	packages    []MCPPackage
	installDir  string
	logger      *logrus.Logger
	statuses    *safe.Store[string, *PackageStatus]
	npxPath     string
	nodePath    string
	npmPath     string
	concurrency int
	timeout     time.Duration
	onProgress  func(pkg string, status InstallStatus, progress float64)
}

// PreinstallerConfig holds configuration for the preinstaller
type PreinstallerConfig struct {
	InstallDir  string
	Packages    []MCPPackage
	Logger      *logrus.Logger
	Concurrency int
	Timeout     time.Duration
	OnProgress  func(pkg string, status InstallStatus, progress float64)
}

// NewPreinstaller creates a new MCP preinstaller
func NewPreinstaller(config PreinstallerConfig) (*MCPPreinstaller, error) {
	if config.InstallDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		config.InstallDir = filepath.Join(homeDir, ".helixagent", "mcp-servers")
	}

	if config.Packages == nil {
		config.Packages = StandardMCPPackages
	}

	if config.Logger == nil {
		config.Logger = logrus.New()
	}

	if config.Concurrency <= 0 {
		config.Concurrency = 4
	}

	if config.Timeout <= 0 {
		config.Timeout = 5 * time.Minute
	}

	p := &MCPPreinstaller{
		packages:    config.Packages,
		installDir:  config.InstallDir,
		logger:      config.Logger,
		statuses:    safe.NewStore[string, *PackageStatus](),
		concurrency: config.Concurrency,
		timeout:     config.Timeout,
		onProgress:  config.OnProgress,
	}

	// Initialize statuses
	for _, pkg := range config.Packages {
		p.statuses.Put(pkg.Name, &PackageStatus{
			Package: pkg,
			Status:  StatusPending,
		})
	}

	// Find node/npm/npx paths
	if err := p.findNodePaths(); err != nil {
		p.logger.WithError(err).Warn("Node.js tools not found, pre-installation will be skipped")
	}

	return p, nil
}

// findNodePaths locates node, npm, and npx executables
func (p *MCPPreinstaller) findNodePaths() error {
	var err error

	p.nodePath, err = exec.LookPath("node")
	if err != nil {
		return fmt.Errorf("node not found: %w", err)
	}

	p.npmPath, err = exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("npm not found: %w", err)
	}

	p.npxPath, err = exec.LookPath("npx")
	if err != nil {
		return fmt.Errorf("npx not found: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"node": p.nodePath,
		"npm":  p.npmPath,
		"npx":  p.npxPath,
	}).Debug("Node.js tools found")

	return nil
}

// PreInstallAll installs all MCP packages concurrently
func (p *MCPPreinstaller) PreInstallAll(ctx context.Context) error {
	if p.npmPath == "" {
		p.logger.Warn("npm not available, skipping MCP package pre-installation")
		return nil
	}

	// Ensure install directory exists
	// #nosec G301 -- MCP package install directories use standard 0750 permissions
	if err := os.MkdirAll(p.installDir, 0750); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"installDir":  p.installDir,
		"packages":    len(p.packages),
		"concurrency": p.concurrency,
	}).Info("Starting MCP package pre-installation")

	// Create semaphore for concurrency control
	sem := make(chan struct{}, p.concurrency)
	var wg sync.WaitGroup
	errChan := make(chan error, len(p.packages))

	for _, pkg := range p.packages {
		wg.Add(1)
		go func(pkg MCPPackage) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}

			// Install package
			if err := p.installPackage(ctx, pkg); err != nil {
				p.logger.WithError(err).WithField("package", pkg.Name).Error("Failed to install MCP package")
				errChan <- fmt.Errorf("failed to install %s: %w", pkg.Name, err)
			}
		}(pkg)
	}

	// Wait for all installations to complete
	wg.Wait()
	close(errChan)

	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		p.logger.WithField("failedCount", len(errors)).Warn("Some MCP packages failed to install")
		return fmt.Errorf("%d packages failed to install", len(errors))
	}

	p.logger.Info("All MCP packages pre-installed successfully")
	return nil
}

// installPackage installs a single MCP package
func (p *MCPPreinstaller) installPackage(ctx context.Context, pkg MCPPackage) error {
	p.updateStatus(pkg.Name, StatusInstalling, "", nil)
	startTime := time.Now()

	// Create package-specific install directory
	pkgDir := filepath.Join(p.installDir, pkg.Name)
	// #nosec G301 -- MCP package-specific directories use standard 0750 permissions
	if err := os.MkdirAll(pkgDir, 0750); err != nil {
		p.updateStatus(pkg.Name, StatusFailed, "", fmt.Errorf("failed to create directory: %w", err))
		return err
	}

	// Check if already installed
	nodeModulesDir := filepath.Join(pkgDir, "node_modules")
	if p.isPackageInstalled(pkgDir, pkg.NPM) {
		p.logger.WithField("package", pkg.Name).Debug("Package already installed, skipping")
		p.updateStatus(pkg.Name, StatusInstalled, pkgDir, nil)
		return nil
	}

	// Create package.json if not exists
	// Path is constructed from installDir (config-provided, defaults to ~/.helixagent/mcp-servers) + pkg.Name (hardcoded in StandardMCPPackages)
	packageJSON := filepath.Join(pkgDir, "package.json")
	// #nosec G304 - packageJSON path is internally constructed from trusted installDir + hardcoded package names, not user input
	if _, err := os.Stat(packageJSON); os.IsNotExist(err) {
		content := fmt.Sprintf(`{
  "name": "helixagent-mcp-%s",
  "version": "1.0.0",
  "private": true,
  "dependencies": {}
}`, pkg.Name)
		// #nosec G306 - package.json is a non-sensitive npm config file that must be world-readable for npm to function
		// #nosec G304 - packageJSON path is internally constructed from trusted installDir + hardcoded package names, not user input
		if err := os.WriteFile(packageJSON, []byte(content), 0644); err != nil {
			p.updateStatus(pkg.Name, StatusFailed, "", fmt.Errorf("failed to create package.json: %w", err))
			return err
		}
	}

	// Create context with timeout
	installCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// Run npm install
	cmd := exec.CommandContext(installCtx, p.npmPath, "install", "--save", "--prefer-offline", pkg.NPM)
	cmd.Dir = pkgDir
	cmd.Env = append(os.Environ(),
		"NODE_ENV=production",
		"npm_config_progress=false",
		"npm_config_audit=false",
		"npm_config_fund=false",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Errorf("npm install failed: %w, output: %s", err, string(output))
		p.updateStatus(pkg.Name, StatusFailed, "", errMsg)
		return errMsg
	}

	// Verify installation
	if !p.isPackageInstalled(pkgDir, pkg.NPM) {
		err := fmt.Errorf("package not found after installation")
		p.updateStatus(pkg.Name, StatusFailed, "", err)
		return err
	}

	duration := time.Since(startTime)
	now := time.Now()
	p.statuses.Update(pkg.Name, func(cur *PackageStatus, present bool) (*PackageStatus, bool) {
		if !present {
			return nil, false
		}
		cp := *cur
		cp.Duration = duration
		cp.InstallPath = nodeModulesDir
		cp.InstalledAt = now
		return &cp, true
	})

	p.updateStatus(pkg.Name, StatusInstalled, nodeModulesDir, nil)

	p.logger.WithFields(logrus.Fields{
		"package":  pkg.Name,
		"duration": duration,
		"path":     nodeModulesDir,
	}).Info("MCP package installed successfully")

	return nil
}

// isPackageInstalled checks if a package is already installed
func (p *MCPPreinstaller) isPackageInstalled(pkgDir string, npmPkg string) bool {
	// Get the package name from the NPM package identifier
	// Handle scoped packages like @modelcontextprotocol/server-filesystem
	// Note: pkgDir comes from installDir (config-provided) + pkg.Name (hardcoded)
	// Note: npmPkg comes from StandardMCPPackages (hardcoded NPM package names)
	checkPath := filepath.Join(pkgDir, "node_modules", npmPkg)

	// Check if package.json exists in the package directory
	packageJSON := filepath.Join(checkPath, "package.json")
	// #nosec G304 - packageJSON path is internally constructed from trusted installDir + hardcoded package names, not user input
	if _, err := os.Stat(packageJSON); err == nil {
		return true
	}

	return false
}

// updateStatus updates the installation status of a package. The
// entry is replaced under safe.Store.Update so readers never observe
// a half-written PackageStatus.
func (p *MCPPreinstaller) updateStatus(name string, status InstallStatus, path string, err error) {
	p.statuses.Update(name, func(cur *PackageStatus, present bool) (*PackageStatus, bool) {
		if !present {
			return nil, false
		}
		cp := *cur
		cp.Status = status
		if path != "" {
			cp.InstallPath = path
		}
		cp.Error = err
		return &cp, true
	})

	if p.onProgress != nil {
		p.onProgress(name, status, p.calculateProgress())
	}
}

// calculateProgress calculates installation progress (0.0 to 1.0).
func (p *MCPPreinstaller) calculateProgress() float64 {
	completed := 0
	total := 0
	p.statuses.Range(func(_ string, status *PackageStatus) bool {
		total++
		if status.Status == StatusInstalled || status.Status == StatusFailed || status.Status == StatusUnavailable {
			completed++
		}
		return true
	})
	if total == 0 {
		return 0
	}
	return float64(completed) / float64(total)
}

// IsInstalled checks if a package is installed
func (p *MCPPreinstaller) IsInstalled(name string) bool {
	if status, ok := p.statuses.Get(name); ok {
		return status.Status == StatusInstalled
	}
	return false
}

// WaitForPackage waits for a specific package to be installed
func (p *MCPPreinstaller) WaitForPackage(ctx context.Context, name string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status, ok := p.statuses.Get(name)
			if !ok {
				return fmt.Errorf("package %s not found", name)
			}
			// status is the committed pointer from the Store; its fields
			// are frozen because updateStatus replaces via copy-on-write.
			currentStatus := status.Status
			statusError := status.Error

			switch currentStatus {
			case StatusInstalled:
				return nil
			case StatusFailed:
				return fmt.Errorf("package %s failed to install: %v", name, statusError)
			case StatusUnavailable:
				return fmt.Errorf("package %s is unavailable", name)
			}
		}
	}
}

// GetStatus returns the status of a package
func (p *MCPPreinstaller) GetStatus(name string) (*PackageStatus, error) {
	status, ok := p.statuses.Get(name)
	if !ok {
		return nil, fmt.Errorf("package %s not found", name)
	}

	// Return a copy
	statusCopy := *status
	return &statusCopy, nil
}

// GetAllStatuses returns the status of all packages
func (p *MCPPreinstaller) GetAllStatuses() map[string]*PackageStatus {
	result := make(map[string]*PackageStatus)
	p.statuses.Range(func(name string, status *PackageStatus) bool {
		statusCopy := *status
		result[name] = &statusCopy
		return true
	})
	return result
}

// GetInstalledPath returns the install path for a package
func (p *MCPPreinstaller) GetInstalledPath(name string) (string, error) {
	status, ok := p.statuses.Get(name)
	if !ok {
		return "", fmt.Errorf("package %s not found", name)
	}

	if status.Status != StatusInstalled {
		return "", fmt.Errorf("package %s is not installed (status: %s)", name, status.Status)
	}

	return status.InstallPath, nil
}

// GetPackageCommand returns the command to run an MCP server
func (p *MCPPreinstaller) GetPackageCommand(name string) ([]string, error) {
	status, ok := p.statuses.Get(name)
	if !ok {
		return nil, fmt.Errorf("package %s not found", name)
	}

	if status.Status != StatusInstalled {
		return nil, fmt.Errorf("package %s is not installed", name)
	}

	// Find the package's main script
	var mainScript string
	pkg := status.Package

	// Try to find the main entry point
	// Note: status.InstallPath is set internally from installDir + hardcoded pkg.Name
	// Note: pkg.NPM comes from hardcoded StandardMCPPackages
	pkgJSON := filepath.Join(status.InstallPath, pkg.NPM, "package.json")
	// #nosec G304 - pkgJSON path is internally constructed from trusted installDir + hardcoded package names, not user input
	if data, err := os.ReadFile(pkgJSON); err == nil {
		// Parse to find "bin" or "main"
		var pkgInfo struct {
			Main string      `json:"main"`
			Bin  interface{} `json:"bin"`
		}
		if err := jsonUnmarshal(data, &pkgInfo); err == nil {
			if pkgInfo.Main != "" {
				mainScript = filepath.Join(status.InstallPath, pkg.NPM, pkgInfo.Main)
			} else if pkgInfo.Bin != nil {
				// Handle bin field (can be string or object)
				switch b := pkgInfo.Bin.(type) {
				case string:
					mainScript = filepath.Join(status.InstallPath, pkg.NPM, b)
				case map[string]interface{}:
					for _, v := range b {
						if binPath, ok := v.(string); ok {
							mainScript = filepath.Join(status.InstallPath, pkg.NPM, binPath)
							break
						}
					}
				}
			}
		}
	}

	// Fallback to index.js
	if mainScript == "" {
		mainScript = filepath.Join(status.InstallPath, pkg.NPM, "index.js")
	}

	// Check if the script exists
	// Note: mainScript is constructed from status.InstallPath (internal) + package.json "main"/"bin" fields or hardcoded fallbacks
	// #nosec G304 - mainScript path is internally constructed from trusted installDir + package metadata, not user input
	if _, err := os.Stat(mainScript); err != nil {
		// Try dist/index.js
		distScript := filepath.Join(status.InstallPath, pkg.NPM, "dist", "index.js")
		// #nosec G304 - distScript path is internally constructed from trusted installDir + hardcoded package names, not user input
		if _, err := os.Stat(distScript); err == nil {
			mainScript = distScript
		} else {
			return nil, fmt.Errorf("main script not found for package %s", name)
		}
	}

	return []string{p.nodePath, mainScript}, nil
}

// IsNodeAvailable checks if Node.js tools are available
func (p *MCPPreinstaller) IsNodeAvailable() bool {
	return p.nodePath != "" && p.npmPath != "" && p.npxPath != ""
}

// GetInstallDir returns the installation directory
func (p *MCPPreinstaller) GetInstallDir() string {
	return p.installDir
}

// Cleanup removes all installed packages
func (p *MCPPreinstaller) Cleanup() error {
	p.logger.Info("Cleaning up MCP packages")
	return os.RemoveAll(p.installDir)
}

// jsonUnmarshal is a helper for JSON unmarshaling
func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
