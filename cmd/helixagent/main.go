package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/router"
)

var (
	configFile           = flag.String("config", "", "Path to configuration file (YAML)")
	version              = flag.Bool("version", false, "Show version information")
	help                 = flag.Bool("help", false, "Show help message")
	autoStartDocker      = flag.Bool("auto-start-docker", true, "Automatically start required Docker containers")
	generateAPIKey       = flag.Bool("generate-api-key", false, "Generate a new HelixAgent API key and output it")
	generateOpenCode     = flag.Bool("generate-opencode-config", false, "Generate OpenCode configuration JSON")
	validateOpenCode     = flag.String("validate-opencode-config", "", "Path to OpenCode config file to validate")
	openCodeOutput       = flag.String("opencode-output", "", "Output path for OpenCode config (default: stdout)")
	apiKeyEnvFile        = flag.String("api-key-env-file", "", "Path to .env file to write the generated API key")
)

// ValidOpenCodeTopLevelKeys contains the valid top-level keys per OpenCode.ai official schema
// Source: https://opencode.ai/config.json (validated by LLMsVerifier)
var ValidOpenCodeTopLevelKeys = map[string]bool{
	"$schema":      true,
	"plugin":       true,
	"enterprise":   true,
	"instructions": true,
	"provider":     true,
	"mcp":          true,
	"tools":        true,
	"agent":        true,
	"command":      true,
	"keybinds":     true,
	"username":     true,
	"share":        true,
	"permission":   true,
	"compaction":   true,
	"sse":          true,
	"mode":         true,
	"autoshare":    true,
}

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
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "helixagent"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "secret"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "helixagent_db"
	}

	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&connect_timeout=5",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to establish a connection
	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}
	defer conn.Close(ctx)

	// Ping to verify connection is working
	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("PostgreSQL ping failed: %w", err)
	}

	return nil
}

// checkRedisHealth verifies Redis connectivity
func checkRedisHealth() error {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")

	rdb := redis.NewClient(&redis.Options{
		Addr:        redisHost + ":" + redisPort,
		Password:    redisPassword,
		DB:          0,
		DialTimeout: 5 * time.Second,
	})
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

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
	ShowHelp             bool
	ShowVersion          bool
	AutoStartDocker      bool
	GenerateAPIKey       bool
	GenerateOpenCode     bool
	ValidateOpenCode     string
	OpenCodeOutput       string
	APIKeyEnvFile        string
	ServerHost           string
	ServerPort           string
	Logger               *logrus.Logger
	ShutdownSignal       chan os.Signal
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
		ServerPort:      "7061",
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

	// Handle API key generation command
	if appCfg.GenerateAPIKey {
		return handleGenerateAPIKey(appCfg)
	}

	// Handle OpenCode config validation command
	if appCfg.ValidateOpenCode != "" {
		return handleValidateOpenCode(appCfg)
	}

	// Handle OpenCode config generation command
	if appCfg.GenerateOpenCode {
		return handleGenerateOpenCode(appCfg)
	}

	// Load full configuration from environment variables
	cfg := config.Load()

	// Override with command-line specified values if provided
	if appCfg.ServerHost != "" && appCfg.ServerHost != "0.0.0.0" {
		cfg.Server.Host = appCfg.ServerHost
	}
	if appCfg.ServerPort != "" && appCfg.ServerPort != "7061" {
		cfg.Server.Port = appCfg.ServerPort
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
		WriteTimeout: 300 * time.Second, // 5 minutes for SSE streaming support
		IdleTimeout:  120 * time.Second,
	}

	// Channel for server errors
	serverErr := make(chan error, 1)

	go func() {
		logger.WithFields(logrus.Fields{
			"host": cfg.Server.Host,
			"port": cfg.Server.Port,
		}).Info("Starting HelixAgent server with Models.dev integration")

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
	appCfg.GenerateAPIKey = *generateAPIKey
	appCfg.GenerateOpenCode = *generateOpenCode
	appCfg.ValidateOpenCode = *validateOpenCode
	appCfg.OpenCodeOutput = *openCodeOutput
	appCfg.APIKeyEnvFile = *apiKeyEnvFile

	if err := run(appCfg); err != nil {
		appCfg.Logger.WithError(err).Fatal("Application failed")
	}
}

// generateSecureAPIKey generates a cryptographically secure API key
func generateSecureAPIKey() (string, error) {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return "sk-" + hex.EncodeToString(bytes), nil
}

// handleGenerateAPIKey handles the --generate-api-key command
func handleGenerateAPIKey(appCfg *AppConfig) error {
	logger := appCfg.Logger
	if logger == nil {
		logger = logrus.New()
	}

	// Generate the API key
	apiKey, err := generateSecureAPIKey()
	if err != nil {
		return fmt.Errorf("failed to generate API key: %w", err)
	}

	// If env file is specified, write to it
	if appCfg.APIKeyEnvFile != "" {
		if err := writeAPIKeyToEnvFile(appCfg.APIKeyEnvFile, apiKey); err != nil {
			return fmt.Errorf("failed to write API key to env file: %w", err)
		}
		logger.WithField("file", appCfg.APIKeyEnvFile).Info("API key written to env file")
	}

	// Output the API key to stdout
	fmt.Println(apiKey)
	return nil
}

// writeAPIKeyToEnvFile writes or updates the HELIXAGENT_API_KEY in the specified .env file
func writeAPIKeyToEnvFile(filePath, apiKey string) error {
	// Read existing file contents if it exists
	existingContent := make(map[string]string)
	var lineOrder []string

	if file, err := os.Open(filePath); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			// Skip empty lines and comments
			if line == "" || strings.HasPrefix(line, "#") {
				lineOrder = append(lineOrder, line)
				continue
			}
			// Parse key=value
			if idx := strings.Index(line, "="); idx > 0 {
				key := strings.TrimSpace(line[:idx])
				value := strings.TrimSpace(line[idx+1:])
				existingContent[key] = value
				lineOrder = append(lineOrder, key)
			} else {
				lineOrder = append(lineOrder, line)
			}
		}
	}

	// Update the API key
	existingContent["HELIXAGENT_API_KEY"] = apiKey

	// Check if key already exists in order
	keyExists := false
	for _, item := range lineOrder {
		if item == "HELIXAGENT_API_KEY" {
			keyExists = true
			break
		}
	}
	if !keyExists {
		lineOrder = append(lineOrder, "HELIXAGENT_API_KEY")
	}

	// Write back to file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create env file: %w", err)
	}
	defer file.Close()

	for _, item := range lineOrder {
		if item == "" || strings.HasPrefix(item, "#") {
			// Write empty lines and comments as-is
			fmt.Fprintln(file, item)
		} else if value, ok := existingContent[item]; ok {
			// Write key=value
			fmt.Fprintf(file, "%s=%s\n", item, value)
		}
	}

	return nil
}

// OpenCodeConfig represents the OpenCode configuration structure
type OpenCodeConfig struct {
	Schema   string                  `json:"$schema"`
	Provider map[string]ProviderDef  `json:"provider"`
	Agent    *AgentDef               `json:"agent,omitempty"`
}

// ProviderDef represents a provider definition in OpenCode config
type ProviderDef struct {
	NPM     string                    `json:"npm,omitempty"`
	Name    string                    `json:"name"`
	Options map[string]interface{}    `json:"options"`
	Models  map[string]ModelDef       `json:"models,omitempty"`
}

// ModelDef represents a model definition with its capabilities
type ModelDef struct {
	Name        string `json:"name"`
	Attachments bool   `json:"attachments,omitempty"`
	Reasoning   bool   `json:"reasoning,omitempty"`
}

// AgentDef represents agent configuration
type AgentDef struct {
	Model *ModelRef `json:"model"`
}

// ModelRef represents a model reference in OpenCode config
type ModelRef struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// handleGenerateOpenCode handles the --generate-opencode-config command
func handleGenerateOpenCode(appCfg *AppConfig) error {
	logger := appCfg.Logger
	if logger == nil {
		logger = logrus.New()
	}

	// Get configuration values
	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if apiKey == "" {
		// If no API key in env, check if we should generate one
		var err error
		apiKey, err = generateSecureAPIKey()
		if err != nil {
			return fmt.Errorf("failed to generate API key: %w", err)
		}
		logger.Warn("No HELIXAGENT_API_KEY found in environment, generated a new one")

		// If env file is specified, write the generated key
		if appCfg.APIKeyEnvFile != "" {
			if err := writeAPIKeyToEnvFile(appCfg.APIKeyEnvFile, apiKey); err != nil {
				logger.WithError(err).Warn("Failed to write generated API key to env file")
			}
		}
	}

	// Get host and port
	host := os.Getenv("HELIXAGENT_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "7061"
	}

	baseURL := fmt.Sprintf("http://%s:%s/v1", host, port)

	// Build the OpenCode configuration
	// Use custom "helixagent" provider with explicit model definition
	// This prevents OpenCode from showing models from other providers
	config := OpenCodeConfig{
		Schema: "https://opencode.ai/config.json",
		Provider: map[string]ProviderDef{
			"helixagent": {
				NPM:  "@ai-sdk/openai-compatible",
				Name: "HelixAgent AI Debate Ensemble",
				Options: map[string]interface{}{
					"apiKey":  apiKey,
					"baseURL": baseURL,
				},
				Models: map[string]ModelDef{
					"helixagent-debate": {
						Name:        "HelixAgent Debate Ensemble",
						Attachments: true,
						Reasoning:   true,
					},
				},
			},
		},
		Agent: &AgentDef{
			Model: &ModelRef{
				Provider: "helixagent",
				Model:    "helixagent-debate",
			},
		},
	}

	// Marshal to JSON with indentation
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal OpenCode config: %w", err)
	}

	// Output to file or stdout
	if appCfg.OpenCodeOutput != "" {
		if err := os.WriteFile(appCfg.OpenCodeOutput, jsonData, 0644); err != nil {
			return fmt.Errorf("failed to write OpenCode config to file: %w", err)
		}
		logger.WithField("file", appCfg.OpenCodeOutput).Info("OpenCode configuration written to file")
	} else {
		fmt.Println(string(jsonData))
	}

	return nil
}

// OpenCodeValidationError represents a validation error in OpenCode config
type OpenCodeValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// OpenCodeValidationResult holds the complete validation results
type OpenCodeValidationResult struct {
	Valid    bool                      `json:"valid"`
	Errors   []OpenCodeValidationError `json:"errors"`
	Warnings []string                  `json:"warnings"`
	Stats    *OpenCodeValidationStats  `json:"stats,omitempty"`
}

// OpenCodeValidationStats contains statistics about the validated config
type OpenCodeValidationStats struct {
	Providers int `json:"providers"`
	MCPServers int `json:"mcp_servers"`
	Agents    int `json:"agents"`
	Commands  int `json:"commands"`
}

// handleValidateOpenCode handles the --validate-opencode-config command
func handleValidateOpenCode(appCfg *AppConfig) error {
	logger := appCfg.Logger
	if logger == nil {
		logger = logrus.New()
	}

	filePath := appCfg.ValidateOpenCode

	// Read the config file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Perform validation
	result := validateOpenCodeConfig(data)

	// Output header
	fmt.Println("======================================================================")
	fmt.Println("HELIXAGENT OPENCODE CONFIGURATION VALIDATION")
	fmt.Println("Using LLMsVerifier schema compliance rules")
	fmt.Println("======================================================================")
	fmt.Println()
	fmt.Printf("File: %s\n", filePath)
	fmt.Println()

	if result.Valid {
		fmt.Println("✅ CONFIGURATION IS VALID")
		fmt.Println()
		if result.Stats != nil {
			fmt.Printf("Configuration contains:\n")
			fmt.Printf("  - Providers: %d\n", result.Stats.Providers)
			fmt.Printf("  - MCP servers: %d\n", result.Stats.MCPServers)
			fmt.Printf("  - Agents: %d\n", result.Stats.Agents)
			fmt.Printf("  - Commands: %d\n", result.Stats.Commands)
		}
	} else {
		fmt.Println("❌ CONFIGURATION HAS ERRORS:")
		fmt.Println()
		for _, e := range result.Errors {
			if e.Field != "" {
				fmt.Printf("  - [%s] %s\n", e.Field, e.Message)
			} else {
				fmt.Printf("  - %s\n", e.Message)
			}
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Println()
		fmt.Println("⚠️  WARNINGS:")
		for _, w := range result.Warnings {
			fmt.Printf("  - %s\n", w)
		}
	}

	fmt.Println()
	fmt.Println("======================================================================")

	if !result.Valid {
		return fmt.Errorf("validation failed with %d errors", len(result.Errors))
	}

	return nil
}

// validateOpenCodeConfig performs comprehensive validation of an OpenCode config
func validateOpenCodeConfig(data []byte) *OpenCodeValidationResult {
	result := &OpenCodeValidationResult{
		Valid:    true,
		Errors:   []OpenCodeValidationError{},
		Warnings: []string{},
		Stats:    &OpenCodeValidationStats{},
	}

	// Parse as generic map to check top-level keys
	var rawConfig map[string]interface{}
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, OpenCodeValidationError{
			Field:   "",
			Message: fmt.Sprintf("invalid JSON: %v", err),
		})
		return result
	}

	// Check for invalid top-level keys (per LLMsVerifier schema)
	var invalidKeys []string
	for key := range rawConfig {
		if !ValidOpenCodeTopLevelKeys[key] {
			invalidKeys = append(invalidKeys, key)
		}
	}
	if len(invalidKeys) > 0 {
		result.Valid = false
		result.Errors = append(result.Errors, OpenCodeValidationError{
			Field:   "",
			Message: fmt.Sprintf("invalid top-level keys: %v (valid keys: $schema, plugin, enterprise, instructions, provider, mcp, tools, agent, command, keybinds, username, share, permission, compaction, sse, mode, autoshare)", invalidKeys),
		})
	}

	// Parse and validate providers
	if providers, ok := rawConfig["provider"].(map[string]interface{}); ok {
		result.Stats.Providers = len(providers)
		for name, providerData := range providers {
			if provider, ok := providerData.(map[string]interface{}); ok {
				// Provider must have options
				if _, hasOptions := provider["options"]; !hasOptions {
					result.Valid = false
					result.Errors = append(result.Errors, OpenCodeValidationError{
						Field:   fmt.Sprintf("provider.%s.options", name),
						Message: "provider must have options configured",
					})
				}
			}
		}
	} else if rawConfig["provider"] == nil {
		result.Valid = false
		result.Errors = append(result.Errors, OpenCodeValidationError{
			Field:   "provider",
			Message: "at least one provider must be configured",
		})
	}

	// Parse and validate MCP servers
	if mcpServers, ok := rawConfig["mcp"].(map[string]interface{}); ok {
		result.Stats.MCPServers = len(mcpServers)
		for name, serverData := range mcpServers {
			if server, ok := serverData.(map[string]interface{}); ok {
				serverType, hasType := server["type"].(string)
				if !hasType {
					result.Valid = false
					result.Errors = append(result.Errors, OpenCodeValidationError{
						Field:   fmt.Sprintf("mcp.%s.type", name),
						Message: "type is required for MCP servers",
					})
					continue
				}
				if serverType != "local" && serverType != "remote" {
					result.Valid = false
					result.Errors = append(result.Errors, OpenCodeValidationError{
						Field:   fmt.Sprintf("mcp.%s.type", name),
						Message: "type must be 'local' or 'remote'",
					})
				}
				if serverType == "local" {
					if _, hasCommand := server["command"]; !hasCommand {
						result.Valid = false
						result.Errors = append(result.Errors, OpenCodeValidationError{
							Field:   fmt.Sprintf("mcp.%s.command", name),
							Message: "command is required for local MCP servers",
						})
					}
				}
				if serverType == "remote" {
					if _, hasURL := server["url"]; !hasURL {
						result.Valid = false
						result.Errors = append(result.Errors, OpenCodeValidationError{
							Field:   fmt.Sprintf("mcp.%s.url", name),
							Message: "url is required for remote MCP servers",
						})
					}
				}
			}
		}
	}

	// Parse and validate agents
	if agents, ok := rawConfig["agent"].(map[string]interface{}); ok {
		// Check if this is a single agent object with "model" directly
		if _, hasModel := agents["model"]; hasModel {
			result.Stats.Agents = 1
			// Single agent config - validate it has model or prompt
			// This is valid - it has model
		} else {
			result.Stats.Agents = len(agents)
			for name, agentData := range agents {
				if agent, ok := agentData.(map[string]interface{}); ok {
					_, hasModel := agent["model"]
					_, hasPrompt := agent["prompt"]
					if !hasModel && !hasPrompt {
						result.Valid = false
						result.Errors = append(result.Errors, OpenCodeValidationError{
							Field:   fmt.Sprintf("agent.%s", name),
							Message: "agent must have either model or prompt configured",
						})
					}
				}
			}
		}
	}

	// Parse commands
	if commands, ok := rawConfig["command"].(map[string]interface{}); ok {
		result.Stats.Commands = len(commands)
	}

	// Add warnings for missing recommended fields
	if _, hasSchema := rawConfig["$schema"]; !hasSchema {
		result.Warnings = append(result.Warnings, "$schema field is recommended for validation")
	}

	return result
}

func showHelp() {
	fmt.Printf(`HelixAgent - Advanced LLM Gateway with Cognee Integration

Usage:
  helixagent [options]

Options:
  -config string
        Path to configuration file (YAML)
  -auto-start-docker
        Automatically start required Docker containers (default: true)
  -generate-api-key
        Generate a new HelixAgent API key and output it to stdout
  -generate-opencode-config
        Generate OpenCode configuration JSON (uses HELIXAGENT_API_KEY env or generates new)
  -validate-opencode-config string
        Validate an existing OpenCode configuration file (uses LLMsVerifier schema rules)
  -opencode-output string
        Output path for OpenCode config (default: stdout)
  -api-key-env-file string
        Path to .env file to write the generated API key
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

API Key & Configuration Commands:
  # Generate a new API key and display it
  helixagent -generate-api-key

  # Generate API key and save to .env file
  helixagent -generate-api-key -api-key-env-file .env

  # Generate OpenCode configuration (uses HELIXAGENT_API_KEY from env)
  helixagent -generate-opencode-config

  # Generate OpenCode config and save to file, with API key to .env
  helixagent -generate-opencode-config -opencode-output opencode.json -api-key-env-file .env

  # Validate an existing OpenCode configuration file
  helixagent -validate-opencode-config ~/.config/opencode/opencode.json

  # Validate a generated config
  helixagent -validate-opencode-config ~/Downloads/opencode-helix-agent.json

Examples:
  helixagent
  helixagent -auto-start-docker=false
  helixagent -config /path/to/config.yaml
  helixagent -version

For more information, visit: https://dev.helix.agent
`)
}

func showVersion() {
	fmt.Printf("HelixAgent v%s - Models.dev Enhanced Edition\n", "1.0.0")
}
