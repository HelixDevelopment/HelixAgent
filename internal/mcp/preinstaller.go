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

	"github.com/sirupsen/logrus"
)

// MCPPackage represents an MCP npm package to be pre-installed
type MCPPackage struct {
	Name        string // Short name for identification
	NPM         string // NPM package name (e.g., "@modelcontextprotocol/server-filesystem")
	Binary      string // Binary name after installation (optional, defaults to "index.js")
	Description string // Package description
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
		NPM:         "mcp-fetch",
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
	Package      MCPPackage
	Status       InstallStatus
	InstallPath  string
	InstalledAt  time.Time
	Error        error
	Duration     time.Duration
}

// MCPPreinstaller handles pre-installation of MCP npm packages
type MCPPreinstaller struct {
	packages     []MCPPackage
	installDir   string
	logger       *logrus.Logger
	statuses     map[string]*PackageStatus
	mu           sync.RWMutex
	npxPath      string
	nodePath     string
	npmPath      string
	concurrency  int
	timeout      time.Duration
	onProgress   func(pkg string, status InstallStatus, progress float64)
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
		statuses:    make(map[string]*PackageStatus),
		concurrency: config.Concurrency,
		timeout:     config.Timeout,
		onProgress:  config.OnProgress,
	}

	// Initialize statuses
	for _, pkg := range config.Packages {
		p.statuses[pkg.Name] = &PackageStatus{
			Package: pkg,
			Status:  StatusPending,
		}
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
	if err := os.MkdirAll(p.installDir, 0755); err != nil {
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
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
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
	packageJSON := filepath.Join(pkgDir, "package.json")
	if _, err := os.Stat(packageJSON); os.IsNotExist(err) {
		content := fmt.Sprintf(`{
  "name": "helixagent-mcp-%s",
  "version": "1.0.0",
  "private": true,
  "dependencies": {}
}`, pkg.Name)
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
	p.mu.Lock()
	if status, ok := p.statuses[pkg.Name]; ok {
		status.Duration = duration
		status.InstallPath = nodeModulesDir
		status.InstalledAt = time.Now()
	}
	p.mu.Unlock()

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
	var checkPath string
	if npmPkg[0] == '@' {
		checkPath = filepath.Join(pkgDir, "node_modules", npmPkg)
	} else {
		checkPath = filepath.Join(pkgDir, "node_modules", npmPkg)
	}

	// Check if package.json exists in the package directory
	packageJSON := filepath.Join(checkPath, "package.json")
	if _, err := os.Stat(packageJSON); err == nil {
		return true
	}

	return false
}

// updateStatus updates the installation status of a package
func (p *MCPPreinstaller) updateStatus(name string, status InstallStatus, path string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if s, ok := p.statuses[name]; ok {
		s.Status = status
		if path != "" {
			s.InstallPath = path
		}
		s.Error = err
	}

	if p.onProgress != nil {
		p.onProgress(name, status, p.calculateProgress())
	}
}

// calculateProgress calculates installation progress (0.0 to 1.0)
func (p *MCPPreinstaller) calculateProgress() float64 {
	completed := 0
	for _, status := range p.statuses {
		if status.Status == StatusInstalled || status.Status == StatusFailed || status.Status == StatusUnavailable {
			completed++
		}
	}
	return float64(completed) / float64(len(p.statuses))
}

// IsInstalled checks if a package is installed
func (p *MCPPreinstaller) IsInstalled(name string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if status, ok := p.statuses[name]; ok {
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
			p.mu.RLock()
			status, ok := p.statuses[name]
			if !ok {
				p.mu.RUnlock()
				return fmt.Errorf("package %s not found", name)
			}
			// Copy values while holding the lock to avoid race conditions
			currentStatus := status.Status
			statusError := status.Error
			p.mu.RUnlock()

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
	p.mu.RLock()
	defer p.mu.RUnlock()

	status, ok := p.statuses[name]
	if !ok {
		return nil, fmt.Errorf("package %s not found", name)
	}

	// Return a copy
	statusCopy := *status
	return &statusCopy, nil
}

// GetAllStatuses returns the status of all packages
func (p *MCPPreinstaller) GetAllStatuses() map[string]*PackageStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*PackageStatus)
	for name, status := range p.statuses {
		statusCopy := *status
		result[name] = &statusCopy
	}
	return result
}

// GetInstalledPath returns the install path for a package
func (p *MCPPreinstaller) GetInstalledPath(name string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	status, ok := p.statuses[name]
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
	p.mu.RLock()
	defer p.mu.RUnlock()

	status, ok := p.statuses[name]
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
	pkgJSON := filepath.Join(status.InstallPath, pkg.NPM, "package.json")
	if data, err := os.ReadFile(pkgJSON); err == nil {
		// Parse to find "bin" or "main"
		var pkgInfo struct {
			Main string            `json:"main"`
			Bin  interface{}       `json:"bin"`
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
	if _, err := os.Stat(mainScript); err != nil {
		// Try dist/index.js
		distScript := filepath.Join(status.InstallPath, pkg.NPM, "dist", "index.js")
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
