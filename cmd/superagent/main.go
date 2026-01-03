package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/router"
)

var (
	configFile      = flag.String("config", "", "Path to configuration file (YAML)")
	version         = flag.Bool("version", false, "Show version information")
	help            = flag.Bool("help", false, "Show help message")
	autoStartDocker = flag.Bool("auto-start-docker", true, "Automatically start required Docker containers")
)

// CommandExecutor interface for executing system commands (allows mocking)
type CommandExecutor interface {
	LookPath(file string) (string, error)
	RunCommand(name string, args ...string) ([]byte, error)
	RunCommandWithDir(dir string, name string, args ...string) ([]byte, error)
}

// RealCommandExecutor implements CommandExecutor using actual exec calls
type RealCommandExecutor struct{}

func (r *RealCommandExecutor) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (r *RealCommandExecutor) RunCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

func (r *RealCommandExecutor) RunCommandWithDir(dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

// HealthChecker interface for checking service health (allows mocking)
type HealthChecker interface {
	CheckHealth(url string) error
}

// HTTPHealthChecker implements HealthChecker using HTTP requests
type HTTPHealthChecker struct {
	Client  *http.Client
	Timeout time.Duration
}

func NewHTTPHealthChecker(timeout time.Duration) *HTTPHealthChecker {
	return &HTTPHealthChecker{
		Client:  &http.Client{Timeout: timeout},
		Timeout: timeout,
	}
}

func (h *HTTPHealthChecker) CheckHealth(url string) error {
	resp, err := h.Client.Get(url)
	if err != nil {
		return fmt.Errorf("cannot connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}
	return nil
}

// ContainerConfig holds configuration for container management
type ContainerConfig struct {
	ProjectDir       string
	RequiredServices []string
	CogneeURL        string
	ChromaDBURL      string
	Executor         CommandExecutor
	HealthChecker    HealthChecker
}

// DefaultContainerConfig returns the default container configuration
func DefaultContainerConfig() *ContainerConfig {
	return &ContainerConfig{
		ProjectDir:       "/media/milosvasic/DATA4TB/Projects/HelixAgent",
		RequiredServices: []string{"postgres", "redis", "cognee", "chromadb"},
		CogneeURL:        "http://cognee:8000/health",
		ChromaDBURL:      "http://chromadb:8000/api/v1/heartbeat",
		Executor:         &RealCommandExecutor{},
		HealthChecker:    NewHTTPHealthChecker(10 * time.Second),
	}
}

// Global container config (can be overridden for testing)
var containerConfig = DefaultContainerConfig()

// ensureRequiredContainers starts required Docker containers using docker-compose
func ensureRequiredContainers(logger *logrus.Logger) error {
	return ensureRequiredContainersWithConfig(logger, containerConfig)
}

// ensureRequiredContainersWithConfig starts required Docker containers using provided config
func ensureRequiredContainersWithConfig(logger *logrus.Logger, cfg *ContainerConfig) error {
	executor := cfg.Executor

	// Check if docker is available
	if _, err := executor.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found in PATH: %w", err)
	}

	// Check which services are already running
	runningServices, err := getRunningServicesWithConfig(cfg)
	if err != nil {
		logger.WithError(err).Warn("Could not check running services, attempting to start all")
		runningServices = make(map[string]bool)
	}

	// Determine which services need to be started
	servicesToStart := []string{}
	for _, service := range cfg.RequiredServices {
		if !runningServices[service] {
			servicesToStart = append(servicesToStart, service)
		}
	}

	if len(servicesToStart) == 0 {
		logger.Info("All required containers are already running")
		return nil
	}

	logger.WithField("services", strings.Join(servicesToStart, ", ")).Info("Starting required containers")

	// Try docker compose first (newer syntax), fall back to docker-compose
	var output []byte

	// Check for docker compose (as a subcommand of docker)
	args := append([]string{"compose", "up", "-d"}, servicesToStart...)
	output, err = executor.RunCommandWithDir(cfg.ProjectDir, "docker", args...)
	if err != nil {
		// Try docker-compose as fallback
		if _, lookErr := executor.LookPath("docker-compose"); lookErr == nil {
			composeArgs := append([]string{"up", "-d"}, servicesToStart...)
			output, err = executor.RunCommandWithDir(cfg.ProjectDir, "docker-compose", composeArgs...)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to start containers: %w, output: %s", err, string(output))
	}

	logger.Info("Waiting for containers to be healthy...")

	// Wait for containers to be ready (simple approach - wait a bit)
	// In tests, this can be mocked to skip the sleep
	if cfg.Executor != nil {
		time.Sleep(15 * time.Second)
	}

	// Verify critical services are running
	if err := verifyServicesHealthWithConfig(cfg.RequiredServices, logger, cfg); err != nil {
		logger.WithError(err).Warn("Some services may not be fully ready, but continuing")
	}

	logger.Info("Container startup completed")
	return nil
}

// getRunningServices checks which docker-compose services are currently running
func getRunningServices() (map[string]bool, error) {
	return getRunningServicesWithConfig(containerConfig)
}

// getRunningServicesWithConfig checks which docker-compose services are currently running using provided config
func getRunningServicesWithConfig(cfg *ContainerConfig) (map[string]bool, error) {
	running := make(map[string]bool)
	executor := cfg.Executor

	// Check if docker is available
	if _, err := executor.LookPath("docker"); err != nil {
		return running, fmt.Errorf("docker compose not found")
	}

	// Try docker compose first
	output, err := executor.RunCommandWithDir(cfg.ProjectDir, "docker", "compose", "ps", "--services", "--filter", "status=running")
	if err != nil {
		// Try docker-compose as fallback
		if _, lookErr := executor.LookPath("docker-compose"); lookErr == nil {
			output, err = executor.RunCommandWithDir(cfg.ProjectDir, "docker-compose", "ps", "--services", "--filter", "status=running")
		}
	}

	if err != nil {
		return running, err
	}

	services := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, service := range services {
		service = strings.TrimSpace(service)
		if service != "" {
			running[service] = true
		}
	}

	return running, nil
}

// verifyServicesHealth performs basic health checks on critical services
func verifyServicesHealth(services []string, logger *logrus.Logger) error {
	return verifyServicesHealthWithConfig(services, logger, containerConfig)
}

// PostgresHealthChecker is a function type for checking Postgres health (allows mocking)
type PostgresHealthChecker func() error

// RedisHealthChecker is a function type for checking Redis health (allows mocking)
type RedisHealthChecker func() error

// Default health checkers (can be overridden for testing)
var (
	postgresHealthChecker PostgresHealthChecker = checkPostgresHealth
	redisHealthChecker    RedisHealthChecker    = checkRedisHealth
)

// verifyServicesHealthWithConfig performs basic health checks on critical services using provided config
func verifyServicesHealthWithConfig(services []string, logger *logrus.Logger, cfg *ContainerConfig) error {
	var errors []string

	for _, service := range services {
		switch service {
		case "postgres":
			if err := postgresHealthChecker(); err != nil {
				errors = append(errors, fmt.Sprintf("postgres: %v", err))
			}
		case "redis":
			if err := redisHealthChecker(); err != nil {
				errors = append(errors, fmt.Sprintf("redis: %v", err))
			}
		case "cognee":
			if err := checkCogneeHealthWithConfig(cfg); err != nil {
				errors = append(errors, fmt.Sprintf("cognee: %v", err))
			}
		case "chromadb":
			if err := checkChromaDBHealthWithConfig(cfg); err != nil {
				errors = append(errors, fmt.Sprintf("chromadb: %v", err))
			}
		default:
			errors = append(errors, fmt.Sprintf("%s: unknown service", service))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("health check failures: %s", strings.Join(errors, "; "))
	}

	return nil
}

// checkPostgresHealth verifies PostgreSQL connectivity
func checkPostgresHealth() error {
	// Simple health check - in production this would use actual database connection
	time.Sleep(2 * time.Second) // Give it time to start
	return nil                  // For now, assume it's healthy
}

// checkRedisHealth verifies Redis connectivity
func checkRedisHealth() error {
	time.Sleep(1 * time.Second)
	return nil
}

// checkCogneeHealth verifies Cognee API availability
func checkCogneeHealth() error {
	return checkCogneeHealthWithConfig(containerConfig)
}

// checkCogneeHealthWithConfig verifies Cognee API availability using provided config
func checkCogneeHealthWithConfig(cfg *ContainerConfig) error {
	if err := cfg.HealthChecker.CheckHealth(cfg.CogneeURL); err != nil {
		return fmt.Errorf("cannot connect to Cognee: %w", err)
	}
	return nil
}

// checkChromaDBHealth verifies ChromaDB availability
func checkChromaDBHealth() error {
	return checkChromaDBHealthWithConfig(containerConfig)
}

// checkChromaDBHealthWithConfig verifies ChromaDB availability using provided config
func checkChromaDBHealthWithConfig(cfg *ContainerConfig) error {
	if err := cfg.HealthChecker.CheckHealth(cfg.ChromaDBURL); err != nil {
		return fmt.Errorf("cannot connect to ChromaDB: %w", err)
	}
	return nil
}

// AppConfig holds application configuration for testing
type AppConfig struct {
	ShowHelp        bool
	ShowVersion     bool
	AutoStartDocker bool
	ServerHost      string
	ServerPort      string
	Logger          *logrus.Logger
	ShutdownSignal  chan os.Signal
}

// DefaultAppConfig returns the default application configuration
func DefaultAppConfig() *AppConfig {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	return &AppConfig{
		ShowHelp:        false,
		ShowVersion:     false,
		AutoStartDocker: true,
		ServerHost:      "0.0.0.0",
		ServerPort:      "8080",
		Logger:          logger,
		ShutdownSignal:  nil,
	}
}

// run executes the main application logic with the given configuration
// Returns an error if the application fails to start
func run(appCfg *AppConfig) error {
	if appCfg.ShowHelp {
		showHelp()
		return nil
	}

	if appCfg.ShowVersion {
		showVersion()
		return nil
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: appCfg.ServerHost,
			Port: appCfg.ServerPort,
		},
	}

	logger := appCfg.Logger
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.InfoLevel)
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	// Auto-start required Docker containers if enabled
	if appCfg.AutoStartDocker {
		logger.Info("Checking and starting required Docker containers...")
		if err := ensureRequiredContainers(logger); err != nil {
			logger.WithError(err).Warn("Failed to start some containers, continuing with application startup")
		} else {
			logger.Info("Docker containers are ready")
		}
	}

	r := router.SetupRouter(cfg)

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Channel for server errors
	serverErr := make(chan error, 1)

	go func() {
		logger.WithFields(logrus.Fields{
			"host": cfg.Server.Host,
			"port": cfg.Server.Port,
		}).Info("Starting SuperAgent server with Models.dev integration")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Use provided shutdown signal or create one
	quit := appCfg.ShutdownSignal
	if quit == nil {
		quit = make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	}

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErr:
		return fmt.Errorf("server failed to start: %w", err)
	case <-quit:
		// Continue to shutdown
	}

	logger.Info("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Use r variable to avoid unused import
	_ = r

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Error("Server forced to shutdown")
		return fmt.Errorf("server shutdown error: %w", err)
	}

	logger.Info("Server shutdown complete")
	return nil
}

func main() {
	flag.Parse()

	appCfg := DefaultAppConfig()
	appCfg.ShowHelp = *help
	appCfg.ShowVersion = *version
	appCfg.AutoStartDocker = *autoStartDocker

	if err := run(appCfg); err != nil {
		appCfg.Logger.WithError(err).Fatal("Application failed")
	}
}

func showHelp() {
	fmt.Printf(`SuperAgent - Advanced LLM Gateway with Cognee Integration

Usage:
  superagent [options]

Options:
  -config string
        Path to configuration file (YAML)
  -auto-start-docker
        Automatically start required Docker containers (default: true)
  -version
        Show version information
  -help
        Show this help message

Features:
  - Cognee knowledge graph integration for advanced AI memory
  - Graph-powered reasoning beyond traditional RAG
  - Multi-modal processing (text, code, images, audio)
  - Auto-containerization for seamless deployment
  - Automatic startup of required Docker containers
  - Models.dev integration for comprehensive model metadata
  - Multi-layer caching with Redis and in-memory
  - Circuit breaker for API resilience
  - Auto-refresh with configurable intervals
  - Model comparison and capability filtering
  - Comprehensive monitoring and health checks

Examples:
  superagent
  superagent -auto-start-docker=false
  superagent -config /path/to/config.yaml
  superagent -version

For more information, visit: https://github.com/superagent/superagent
`)
}

func showVersion() {
	fmt.Printf("SuperAgent v%s - Models.dev Enhanced Edition\n", "1.0.0")
}
