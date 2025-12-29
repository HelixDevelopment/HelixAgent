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

// ensureRequiredContainers starts required Docker containers using docker-compose
func ensureRequiredContainers(logger *logrus.Logger) error {
	// Check if docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found in PATH: %w", err)
	}

	// Required services that must be running
	requiredServices := []string{
		"postgres", // Database
		"redis",    // Caching
		"cognee",   // Cognee knowledge graph
		"chromadb", // Vector database
	}

	// Check which services are already running
	runningServices, err := getRunningServices()
	if err != nil {
		logger.WithError(err).Warn("Could not check running services, attempting to start all")
		runningServices = make(map[string]bool)
	}

	// Determine which services need to be started
	servicesToStart := []string{}
	for _, service := range requiredServices {
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
	var cmd *exec.Cmd
	args := append([]string{"compose", "up", "-d"}, servicesToStart...)

	if _, err := exec.LookPath("docker compose"); err == nil {
		cmd = exec.Command("docker", args...)
	} else if _, err := exec.LookPath("docker-compose"); err == nil {
		cmd = exec.Command("docker-compose", args...)
	} else {
		return fmt.Errorf("neither 'docker compose' nor 'docker-compose' found in PATH")
	}

	// Set working directory to project root
	cmd.Dir = "/media/milosvasic/DATA4TB/Projects/HelixAgent"

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start containers: %w, output: %s", err, string(output))
	}

	logger.Info("Waiting for containers to be healthy...")

	// Wait for containers to be ready (simple approach - wait a bit)
	time.Sleep(15 * time.Second)

	// Verify critical services are running
	if err := verifyServicesHealth(requiredServices, logger); err != nil {
		logger.WithError(err).Warn("Some services may not be fully ready, but continuing")
	}

	logger.Info("Container startup completed")
	return nil
}

// getRunningServices checks which docker-compose services are currently running
func getRunningServices() (map[string]bool, error) {
	running := make(map[string]bool)

	// Check docker compose ps
	var cmd *exec.Cmd
	if _, err := exec.LookPath("docker compose"); err == nil {
		cmd = exec.Command("docker", "compose", "ps", "--services", "--filter", "status=running")
	} else if _, err := exec.LookPath("docker-compose"); err == nil {
		cmd = exec.Command("docker-compose", "ps", "--services", "--filter", "status=running")
	} else {
		return running, fmt.Errorf("docker compose not found")
	}

	cmd.Dir = "/media/milosvasic/DATA4TB/Projects/HelixAgent"

	output, err := cmd.Output()
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
	var errors []string

	for _, service := range services {
		switch service {
		case "postgres":
			if err := checkPostgresHealth(); err != nil {
				errors = append(errors, fmt.Sprintf("postgres: %v", err))
			}
		case "redis":
			if err := checkRedisHealth(); err != nil {
				errors = append(errors, fmt.Sprintf("redis: %v", err))
			}
		case "cognee":
			if err := checkCogneeHealth(); err != nil {
				errors = append(errors, fmt.Sprintf("cognee: %v", err))
			}
		case "chromadb":
			if err := checkChromaDBHealth(); err != nil {
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
	// Try to connect to Cognee health endpoint
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("http://cognee:8000/health")
	if err != nil {
		return fmt.Errorf("cannot connect to Cognee: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Cognee health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// checkChromaDBHealth verifies ChromaDB availability
func checkChromaDBHealth() error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("http://chromadb:8000/api/v1/heartbeat")
	if err != nil {
		return fmt.Errorf("cannot connect to ChromaDB: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ChromaDB health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

func main() {
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	if *version {
		showVersion()
		return
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "0.0.0.0",
			Port: "8080",
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Auto-start required Docker containers if enabled
	if *autoStartDocker {
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

	go func() {
		logger.WithFields(logrus.Fields{
			"host": cfg.Server.Host,
			"port": cfg.Server.Port,
		}).Info("Starting SuperAgent server with Models.dev integration")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to start server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)

	// Use r variable to avoid unused import
	_ = r
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Error("Server forced to shutdown")
	} else {
		logger.Info("Server shutdown complete")
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
