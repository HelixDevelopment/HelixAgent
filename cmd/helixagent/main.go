package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	containeradapter "dev.helix.agent/internal/adapters/containers"
	"dev.helix.agent/internal/auth/oauth_credentials"
	"dev.helix.agent/internal/bigdata"
	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/health"
	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/llm/providers/ai21"
	"dev.helix.agent/internal/llm/providers/cerebras"
	"dev.helix.agent/internal/llm/providers/chutes"
	"dev.helix.agent/internal/llm/providers/claude"
	"dev.helix.agent/internal/llm/providers/cloudflare"
	"dev.helix.agent/internal/llm/providers/codestral"
	"dev.helix.agent/internal/llm/providers/cohere"
	"dev.helix.agent/internal/llm/providers/deepseek"
	"dev.helix.agent/internal/llm/providers/fireworks"
	"dev.helix.agent/internal/llm/providers/gemini"
	"dev.helix.agent/internal/llm/providers/generic"
	"dev.helix.agent/internal/llm/providers/githubmodels"
	"dev.helix.agent/internal/llm/providers/groq"
	"dev.helix.agent/internal/llm/providers/huggingface"
	"dev.helix.agent/internal/llm/providers/mistral"
	"dev.helix.agent/internal/llm/providers/nvidia"
	"dev.helix.agent/internal/llm/providers/ollama"
	"dev.helix.agent/internal/llm/providers/openai"
	"dev.helix.agent/internal/llm/providers/openrouter"
	"dev.helix.agent/internal/llm/providers/perplexity"
	"dev.helix.agent/internal/llm/providers/qwen"
	"dev.helix.agent/internal/llm/providers/replicate"
	"dev.helix.agent/internal/llm/providers/together"
	"dev.helix.agent/internal/llm/providers/upstage"
	"dev.helix.agent/internal/llm/providers/venice"
	"dev.helix.agent/internal/llm/providers/xai"
	"dev.helix.agent/internal/llm/providers/zai"
	"dev.helix.agent/internal/llm/providers/zen"
	"dev.helix.agent/internal/mcp"
	mcpconfig "dev.helix.agent/internal/mcp/config"
	"dev.helix.agent/internal/messaging"
	"dev.helix.agent/internal/messaging/inmemory"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/router"
	"dev.helix.agent/internal/services"
	"dev.helix.agent/internal/transport"
	"dev.helix.agent/internal/utils"
	"dev.helix.agent/internal/verifier"
	appversion "dev.helix.agent/internal/version"
	"digital.vasic.llmsverifier/pkg/cliagents"

	"dev.helix.agent/internal/containers"
)

var (
	configFile         = flag.String("config", "", "Path to configuration file (YAML)")
	version            = flag.Bool("version", false, "Show version information")
	help               = flag.Bool("help", false, "Show help message")
	autoStartDocker    = flag.Bool("auto-start-docker", true, "Automatically start required Docker containers")
	strictDependencies = flag.Bool("strict-dependencies", true, "MANDATORY: Fail if any integration dependency (Mem0, DB, Redis) is unavailable")
	generateAPIKey     = flag.Bool("generate-api-key", false, "Generate a new HelixAgent API key and output it")
	generateOpenCode   = flag.Bool("generate-opencode-config", false, "Generate OpenCode configuration JSON")
	validateOpenCode   = flag.String("validate-opencode-config", "", "Path to OpenCode config file to validate")
	openCodeOutput     = flag.String("opencode-output", "", "Output path for OpenCode config (default: stdout)")
	generateCrush      = flag.Bool("generate-crush-config", false, "Generate Crush configuration JSON")
	validateCrush      = flag.String("validate-crush-config", "", "Path to Crush config file to validate")
	crushOutput        = flag.String("crush-output", "", "Output path for Crush config (default: stdout)")
	apiKeyEnvFile      = flag.String("api-key-env-file", "", "Path to .env file to write the generated API key")
	preinstallMCP      = flag.Bool("preinstall-mcp", false, "Pre-install standard MCP server npm packages")
	skipMCPPreinstall  = flag.Bool("skip-mcp-preinstall", false, "Skip automatic MCP package pre-installation at startup")
	workingMCPsOnly    = flag.Bool("working-mcps-only", true, "Only include MCPs that work without API keys (default: true)")
	useLocalMCPServers = flag.Bool("use-local-mcp-servers", false, "Use local Docker-based MCP servers on TCP ports (requires running start-mcp-servers.sh)")
	useContainerMCPs   = flag.Bool("use-container-mcps", false, "Use containerized MCP servers with HTTP SSE endpoints (requires running MCP containers)")
	autoStartMCP       = flag.Bool("auto-start-mcp", true, "Automatically start MCP Docker containers on HelixAgent startup")
	skipVerification   = flag.Bool("skip-verification", false, "Skip startup provider verification (for faster boot in test environments)")
	// Unified CLI agent configuration flags (all 48 agents)
	generateAgentConfig = flag.String("generate-agent-config", "", "Generate config for specified CLI agent (use --list-agents to see all)")
	validateAgentConfig = flag.String("validate-agent-config", "", "Validate config file for agent (format: agent:path)")
	agentConfigOutput   = flag.String("agent-config-output", "", "Output path for generated agent config (default: stdout)")
	listAgents          = flag.Bool("list-agents", false, "List all 48 supported CLI agents")
	generateAllAgents   = flag.Bool("generate-all-agents", false, "Generate configurations for all 48 CLI agents")
	allAgentsOutputDir  = flag.String("all-agents-output-dir", "", "Output directory for all agent configs (required with --generate-all-agents)")
	// Challenge execution flags
	runChallenges          = flag.String("run-challenges", "", "Run challenges: all | category | challenge-id")
	listChallenges         = flag.Bool("list-challenges", false, "List all registered challenges")
	challengeParallel      = flag.Bool("challenge-parallel", false, "Run challenges in parallel")
	challengeVerbose       = flag.Bool("challenge-verbose", false, "Enable verbose challenge output")
	challengeStallDuration = flag.Duration("challenge-stall-threshold", 60*1e9, "Stall threshold for stuck detection (default: 60s)")
)

// ValidOpenCodeTopLevelKeys contains the valid top-level keys per OpenCode.ai official schema
// Supports both v1.0.x (provider, mcp, agent) and v1.1.30+ (providers, mcpServers, agents) schemas
// Source: https://opencode.ai/config.json and OpenCode internal/config/config.go
var ValidOpenCodeTopLevelKeys = map[string]bool{
	// v1.0.x schema keys
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
	// v1.1.30+ schema keys (Viper-based)
	"providers":    true,
	"mcpServers":   true,
	"agents":       true,
	"contextPaths": true,
	"tui":          true,
}

// globalContainerAdapter is the centralized container adapter.
// Initialized during startup to route all container operations
// through the Containers module.
var globalContainerAdapter *containeradapter.Adapter

// globalLazyOrchestrator manages lazy container service startup.
// Services like NVIDIA RAG are started on-demand rather than at boot.
var globalLazyOrchestrator *containers.LazyOrchestrator

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

// ContainerRuntime represents the detected container runtime (Docker or Podman)
type ContainerRuntime string

const (
	RuntimeDocker ContainerRuntime = "docker"
	RuntimePodman ContainerRuntime = "podman"
	RuntimeNone   ContainerRuntime = "none"
)

// DetectContainerRuntime automatically detects available container runtime.
// Routes through the Containers module adapter exclusively.
// Prefers Docker, falls back to Podman if Docker is not available.
func DetectContainerRuntime() (ContainerRuntime, string, error) {
	if globalContainerAdapter == nil {
		return RuntimeNone, "", fmt.Errorf(
			"container adapter not initialized",
		)
	}
	name, err := globalContainerAdapter.DetectRuntime(
		context.Background(),
	)
	if err != nil {
		return RuntimeNone, "", err
	}
	//nolint:errcheck // path not required, empty string is acceptable
	path, _ := exec.LookPath(name)
	return ContainerRuntime(name), path, nil
}

// DetectComposeCommand detects the compose command for the container runtime.
// Routes through the Containers module adapter. The adapter's orchestrator
// handles compose detection internally.
// Returns: compose command, args prefix, error.
func DetectComposeCommand(rt ContainerRuntime) (string, []string, error) {
	if globalContainerAdapter == nil {
		return "", nil, fmt.Errorf("container adapter not initialized")
	}
	// The adapter's orchestrator already detected compose.
	// Return the runtime name as compose command for backward compat.
	name := string(rt)
	return name, []string{"compose"}, nil
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
	defer func() { _ = resp.Body.Close() }()

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
	// Try to detect project directory from executable location
	// or use current working directory
	projectDir, err := os.Getwd()
	if err != nil {
		projectDir = "/run/media/milosvasic/DATA4TB/Projects/HelixAgent"
	}

	return &ContainerConfig{
		ProjectDir:       projectDir,
		RequiredServices: []string{"postgres", "redis", "cognee", "chromadb"},
		CogneeURL:        dependencyURL("HELIXAGENT_PORT_COGNEE", "8000", "/"),
		ChromaDBURL:      dependencyURL("HELIXAGENT_PORT_CHROMADB", "8001", "/api/v2/heartbeat"),
		Executor:         &RealCommandExecutor{},
		HealthChecker:    NewHTTPHealthChecker(10 * time.Second),
	}
}

// dependencyURL builds the health-probe URL for an integration dependency,
// resolving host and port from the environment rather than baking them in
// (§11.4.111 — resolve by config, never by a hardcoded location).
//
// The compiled-in defaults previously pinned ChromaDB to :8001 and Cognee to
// :8000 unconditionally, so on any host whose stack exposed those services on
// different ports the mandatory startup dependency verification could never
// pass and the binary refused to boot — with no configuration knob to correct
// it. The port env-var names are the same ones declared in internal/ports.
func dependencyURL(portEnv, defaultPort, path string) string {
	host := os.Getenv("HELIXAGENT_DEP_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv(portEnv)
	if port == "" {
		port = defaultPort
	}
	return fmt.Sprintf("http://%s:%s%s", host, port, path)
}

// Global container config (can be overridden for testing)
var containerConfig = DefaultContainerConfig()

// ensureRequiredContainers starts required Docker containers using docker-compose
func ensureRequiredContainers(logger *logrus.Logger) error {
	return ensureRequiredContainersWithConfig(logger, containerConfig)
}

// ensureRequiredContainersWithConfig starts required Docker/Podman containers
// using the Containers module adapter. All container operations are centralized
// through the adapter — no direct exec.Command calls.
func ensureRequiredContainersWithConfig(logger *logrus.Logger, cfg *ContainerConfig) error {
	if globalContainerAdapter == nil {
		return fmt.Errorf("container adapter not initialized")
	}

	logger.Info("Starting required containers via Containers module adapter")

	// Use adapter to compose up the project.
	composeFile := filepath.Join(cfg.ProjectDir, "docker-compose.yml")
	ctx := context.Background()

	if globalContainerAdapter.RemoteEnabled() {
		// adapter.RemoteComposeUp internally uses partitioned placement
		// (CONST-034 / BUGFIXES Issue #52): each service runs on
		// EXACTLY one host across the registered remote-host set.
		// Co-location groups (cognee + postgres + redis + chromadb
		// via depends_on) stay together. SVC_<SERVICE>_HOST is set
		// after each successful deploy so the gateway connects to
		// the right host.
		logger.Info("Deploying required containers via partitioned placement")
		if err := globalContainerAdapter.RemoteComposeUp(
			ctx, composeFile, "default",
		); err != nil {
			logger.WithError(err).Warn(
				"Partitioned remote deploy failed, falling back to local",
			)
			if err := globalContainerAdapter.ComposeUp(
				ctx, composeFile, "default",
			); err != nil {
				return fmt.Errorf("compose up failed: %w", err)
			}
		}
	} else if err := globalContainerAdapter.ComposeUp(
		ctx, composeFile, "default",
	); err != nil {
		return fmt.Errorf("compose up failed: %w", err)
	}

	logger.Info("Waiting for containers to be healthy...")

	// Wait for containers to be ready with retry logic.
	var healthErr error
	for attempt := 1; attempt <= 6; attempt++ {
		time.Sleep(10 * time.Second)
		healthErr = verifyServicesHealthWithConfig(
			cfg.RequiredServices, logger, cfg,
		)
		if healthErr == nil {
			break
		}
		logger.WithFields(logrus.Fields{
			"attempt": attempt,
			"max":     6,
		}).Warn("Health check not passed yet, retrying...")
	}

	if healthErr != nil {
		return fmt.Errorf(
			"service health verification failed: %w", healthErr,
		)
	}

	logger.Info("Container startup completed successfully")
	return nil
}

// ensureMCPServers starts all MCP Docker containers from git submodules
// Uses docker-compose.mcp-servers.yml to build and run 32 MCP servers
// All servers use TCP ports (9101-9999) - no npm/npx dependencies.
// Routes through the Containers module adapter.
func ensureMCPServers(logger *logrus.Logger) error {
	if globalContainerAdapter == nil {
		return fmt.Errorf("container adapter not initialized")
	}

	// Get project directory
	projectDir, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("failed to get project directory: %w", err)
	}

	// Check if MCP compose file exists
	mcpComposeFile := filepath.Join(
		projectDir, "docker", "mcp",
		"docker-compose.mcp-servers.yml",
	)
	if _, err := os.Stat(mcpComposeFile); os.IsNotExist(err) {
		logger.WithField("file", mcpComposeFile).Warn(
			"MCP compose file not found, skipping MCP auto-start",
		)
		return nil
	}

	logger.WithField("mcp_servers", 32).Info(
		"Starting MCP servers via Containers module adapter",
	)

	// Start in background — MCP servers are optional.
	go func() {
		// No outer deadline: the Containers module applies its own
		// CommandTimeout (30 min) per SSH call, which is the right
		// backstop for compose operations that legitimately take
		// several minutes to build/pull images. The previous 120s
		// deadline here killed every remote MCP-compose run before
		// it could finish pulling.
		ctx := context.Background()

		if globalContainerAdapter.RemoteEnabled() {
			// PARTITIONED + strict-remote (CONST-031 + Issue #52):
			// adapter.RemoteComposeUp partitions placement
			// internally — each MCP service runs on EXACTLY one
			// host. Co-location pairs (mongodb-backend ↔
			// mongodb-server, etc.) stay together via depends_on.
			// No local fallback per CONST-031.
			if err := globalContainerAdapter.RemoteComposeUp(
				ctx, mcpComposeFile, "",
			); err != nil {
				logger.WithError(err).Error(
					"MCP servers partitioned deploy failed; " +
						"strict-remote mode skips local fallback per " +
						"CONST-031. MCP servers will not be available " +
						"until remote distribution recovers.",
				)
				return
			}
			logger.Info(
				"MCP servers placed across hosts via partitioned distribution",
			)
			return
		}

		// Only reached when CONTAINERS_REMOTE_ENABLED=false.
		if err := globalContainerAdapter.ComposeUp(
			ctx, mcpComposeFile, "",
		); err != nil {
			logger.WithError(err).Warn(
				"Failed to start some MCP servers (local mode)",
			)
			return
		}
		logger.Info(
			"MCP servers started successfully (32 servers on ports 9101-9999)",
		)
	}()

	logger.Info(
		"MCP servers starting in background (32 servers on ports 9101-9999)",
	)
	return nil
}

// getRunningServicesWithRuntimeConfig checks which compose services are
// currently running via the Containers module adapter.
func getRunningServicesWithRuntimeConfig(cfg *ContainerConfig, _ string, _ []string) (map[string]bool, error) {
	running := make(map[string]bool)

	if globalContainerAdapter == nil {
		// Fall back to health probes when adapter unavailable.
		return checkServicesViaHealthProbes(cfg.RequiredServices), nil
	}

	composeFile := filepath.Join(cfg.ProjectDir, "docker-compose.yml")
	ctx := context.Background()
	statuses, err := globalContainerAdapter.ComposeStatus(
		ctx, composeFile,
	)
	if err != nil {
		// Fall back to health probes on error.
		return checkServicesViaHealthProbes(cfg.RequiredServices), nil
	}

	for _, s := range statuses {
		if s.State == "running" {
			running[s.Name] = true
		}
	}
	return running, nil
}

// checkServicesViaHealthProbes checks if services are running via direct TCP/HTTP probes
func checkServicesViaHealthProbes(services []string) map[string]bool {
	running := make(map[string]bool)
	serviceChecks := map[string]string{
		"postgres": "localhost:8101",
		"redis":    "localhost:6379",
		"cognee":   "http://localhost:8000/",
		"chromadb": "http://localhost:8001/",
	}

	client := &http.Client{Timeout: 3 * time.Second}
	for _, svc := range services {
		addr, ok := serviceChecks[svc]
		if !ok {
			continue
		}
		if strings.HasPrefix(addr, "http") {
			resp, err := client.Get(addr)
			if err == nil {
				_ = resp.Body.Close()
				running[svc] = true
			}
		} else {
			conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
			if err == nil {
				_ = conn.Close()
				running[svc] = true
			}
		}
	}
	return running
}

// getRunningServices checks which docker-compose services are currently running
func getRunningServices() (map[string]bool, error) {
	return getRunningServicesWithConfig(containerConfig)
}

// getRunningServicesWithConfig checks which docker-compose services are
// currently running. Uses the adapter when available, otherwise falls back
// to the config's Executor interface.
func getRunningServicesWithConfig(cfg *ContainerConfig) (map[string]bool, error) {
	running := make(map[string]bool)

	if globalContainerAdapter != nil {
		composeFile := filepath.Join(
			cfg.ProjectDir, "docker-compose.yml",
		)
		ctx := context.Background()
		statuses, err := globalContainerAdapter.ComposeStatus(
			ctx, composeFile,
		)
		if err == nil {
			for _, s := range statuses {
				if s.State == "running" {
					running[s.Name] = true
				}
			}
			return running, nil
		}
	}

	// Fallback: use the executor interface (for tests).
	if cfg.Executor == nil {
		return running, fmt.Errorf("no executor configured")
	}
	output, err := cfg.Executor.RunCommandWithDir(
		cfg.ProjectDir, "sh", "-c",
		"docker compose ps --services --filter status=running 2>/dev/null || true",
	)
	if err != nil {
		return running, err
	}

	services := strings.Split(
		strings.TrimSpace(string(output)), "\n",
	)
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
	type healthResult struct {
		service string
		err     error
	}

	results := make(chan healthResult, len(services))
	var wg sync.WaitGroup

	for _, service := range services {
		svc := service
		wg.Add(1)
		go func() {
			defer wg.Done()
			var err error
			switch svc {
			case "postgres":
				err = postgresHealthChecker()
			case "redis":
				err = redisHealthChecker()
			case "cognee":
				err = checkCogneeHealthWithConfig(cfg)
			case "chromadb":
				err = checkChromaDBHealthWithConfig(cfg)
			default:
				err = fmt.Errorf("unknown service")
			}
			results <- healthResult{service: svc, err: err}
		}()
	}

	// Close results channel when all checks complete
	go func() {
		wg.Wait()
		close(results)
	}()

	var errors []string
	for result := range results {
		if result.err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", result.service, result.err))
		} else {
			logger.WithField("service", result.service).Debug("Health check passed")
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
	defer func() { _ = conn.Close(ctx) }()

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
	defer func() { _ = rdb.Close() }()

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

// MandatoryDependency represents a required integration dependency
type MandatoryDependency struct {
	Name        string
	Description string
	CheckFunc   func() error
	Required    bool
}

// GetMandatoryDependencies returns all mandatory integration dependencies
func GetMandatoryDependencies() []MandatoryDependency {
	return []MandatoryDependency{
		{
			Name:        "PostgreSQL",
			Description: "Primary database for storing configuration, sessions, and metadata",
			CheckFunc:   checkPostgresHealth,
			Required:    true,
		},
		{
			Name:        "Redis",
			Description: "Cache layer for sessions, rate limiting, and response caching",
			CheckFunc:   checkRedisHealth,
			Required:    true,
		},
		{
			Name:        "Cognee",
			Description: "Knowledge graph and RAG integration for AI memory and reasoning",
			CheckFunc:   checkCogneeHealth,
			Required:    true,
		},
		{
			Name:        "ChromaDB",
			Description: "Vector database for embeddings and semantic search",
			CheckFunc:   checkChromaDBHealth,
			Required:    true,
		},
	}
}

// depFailureKind distinguishes the two operationally different reasons a
// dependency probe fails. They demand OPPOSITE operator actions, so collapsing
// them into one message actively misleads (see verifyAllMandatoryDependencies).
type depFailureKind int

const (
	// depAbsent — nothing is listening on the endpoint at all: the dependency
	// has not been started, or is bound somewhere else. The operator must start
	// the stack.
	depAbsent depFailureKind = iota
	// depStarting — something accepted the connection but did not serve a
	// healthy response: the dependency process/container exists and is still
	// coming up (or is unhealthy). Starting the stack again is the WRONG advice.
	depStarting
	// depRejected — the dependency is up and answered, but AUTHORITATIVELY
	// rejected us (bad credentials, forbidden). Waiting cannot fix this, so
	// probing retries immediately stop: telling the operator to wait longer
	// would be the same misleading-remedy defect HXC-228 is about.
	depRejected
)

func (k depFailureKind) String() string {
	switch k {
	case depAbsent:
		return "NOT RUNNING"
	case depRejected:
		return "REJECTED OUR CREDENTIALS"
	default:
		return "STILL STARTING"
	}
}

// classifyDepFailure decides whether a failed probe means "nothing is there"
// or "something is there but is not serving yet".
//
// Grounded in captured evidence, not guesswork (§11.4.6). On this host, with
// the Cognee container stopped the probe fails with
//
//	dial tcp 127.0.0.1:8000: connect: connection refused
//
// while during its start-up window — the HXC-228 race — the port forwarder is
// already bound and the probe fails with
//
//	read tcp ...->127.0.0.1:8000: read: connection reset by peer
//
// The refused/unreachable/unresolvable family means depAbsent; anything that
// got far enough to be reset, time out, or answer non-200 means depStarting.
func classifyDepFailure(err error) depFailureKind {
	if err == nil {
		return depStarting
	}
	// Prefer typed matching; fall back to string matching because the pgx and
	// go-redis drivers do not always preserve the syscall error in their chain.
	for _, errno := range []syscall.Errno{syscall.ECONNREFUSED, syscall.EHOSTUNREACH, syscall.ENETUNREACH} {
		if errors.Is(err, errno) {
			return depAbsent
		}
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range []string{
		"connection refused",
		"no route to host",
		"network is unreachable",
		"no such host",
	} {
		if strings.Contains(msg, needle) {
			return depAbsent
		}
	}
	// Authoritative rejections: the dependency IS serving and said no. Observed
	// while validating HXC-228 — a wrong DB_PASSWORD surfaces as
	// "failed SASL auth: FATAL: password authentication failed ... (SQLSTATE 28P01)".
	// Retrying that for the full budget wastes the boot and then advises the
	// operator to wait even longer, which is precisely the misleading remedy
	// this ticket exists to remove.
	for _, needle := range []string{
		"password authentication failed",
		"authentication failed",
		"sqlstate 28p01", // invalid_password
		"sqlstate 28000", // invalid_authorization_specification
		"wrongpass",      // redis: wrong password
		"noauth",         // redis: auth required, none supplied
		"invalid password",
		"status: 401",
		"status: 403",
	} {
		if strings.Contains(msg, needle) {
			return depRejected
		}
	}
	return depStarting
}

// defaultDependencyWaitSeconds bounds how long boot verification will wait for
// slow-starting dependencies. Bounded on purpose: an unbounded wait converts a
// crash into a hang, which is strictly worse — systemd would sit in
// "activating" forever instead of reporting a failure the operator can see.
const defaultDependencyWaitSeconds = 120

// dependencyWaitBudget resolves the total wait budget shared by ALL dependency
// probes. Set HELIXAGENT_DEP_WAIT_SECONDS=0 to restore strict fail-fast
// (single attempt per dependency, no waiting).
func dependencyWaitBudget(logger *logrus.Logger) time.Duration {
	raw := strings.TrimSpace(os.Getenv("HELIXAGENT_DEP_WAIT_SECONDS"))
	if raw == "" {
		return defaultDependencyWaitSeconds * time.Second
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 0 {
		logger.WithFields(logrus.Fields{
			"HELIXAGENT_DEP_WAIT_SECONDS": raw,
			"fallback_seconds":            defaultDependencyWaitSeconds,
		}).Warn("Invalid dependency wait budget, using default")
		return defaultDependencyWaitSeconds * time.Second
	}
	return time.Duration(seconds) * time.Second
}

// depProbeResult is the outcome of probing one dependency until it became
// healthy or the shared deadline expired.
type depProbeResult struct {
	err      error // nil => healthy
	kind     depFailureKind
	attempts int
	waited   time.Duration
}

// probeDependencyUntil polls one dependency until it reports healthy or the
// shared deadline passes, backing off exponentially between attempts.
//
// Condition-based, not a fixed sleep: a dependency that is already up returns
// on the first attempt and costs nothing, so the happy path is unchanged.
func probeDependencyUntil(dep MandatoryDependency, deadline time.Time, logger *logrus.Logger) depProbeResult {
	const maxBackoff = 8 * time.Second

	start := time.Now()
	backoff := time.Second

	for attempts := 1; ; attempts++ {
		err := dep.CheckFunc()
		if err == nil {
			return depProbeResult{attempts: attempts, waited: time.Since(start)}
		}

		kind := classifyDepFailure(err)
		// An authoritative rejection will not change by waiting — fail
		// immediately rather than burning the whole budget on a certainty.
		if kind == depRejected {
			return depProbeResult{err: err, kind: kind, attempts: attempts, waited: time.Since(start)}
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return depProbeResult{err: err, kind: kind, attempts: attempts, waited: time.Since(start)}
		}

		sleep := backoff
		if sleep > remaining {
			sleep = remaining
		}
		logger.WithFields(logrus.Fields{
			"dependency":  dep.Name,
			"state":       kind.String(),
			"attempt":     attempts,
			"retry_in":    sleep.Round(time.Millisecond),
			"deadline_in": remaining.Round(time.Second),
			"error":       err,
		}).Warn("⏳ DEPENDENCY NOT READY — waiting")

		time.Sleep(sleep)
		if backoff < maxBackoff {
			backoff *= 2
		}
	}
}

// depFailure records a dependency that never became healthy within the budget.
type depFailure struct {
	name     string
	err      error
	kind     depFailureKind
	attempts int
}

// verifyAllMandatoryDependencies checks ALL required integration dependencies.
// Returns an error if ANY mandatory dependency is still unavailable after the
// bounded wait budget.
//
// HXC-228: this used to probe each dependency EXACTLY ONCE. On a cold boot all
// four probes and the fatal exit landed inside the same second the unit
// started, so a dependency that was merely still coming up — Cognee, whose
// compose entry waits on postgres+redis being healthy and so is always among
// the last to serve — was treated as permanently absent and blocked the boot.
// Fifteen seconds later the very same check passed. A readiness check that runs
// once, in the same second the process starts, cannot distinguish "not ready
// yet" from "not there at all".
//
// The fix is to WAIT, but only within a bound: a genuinely absent dependency
// must still block the boot (that is the whole point of mandatory verification
// per §11.4 — a service must never claim health it does not have).
func verifyAllMandatoryDependencies(logger *logrus.Logger) error {
	dependencies := GetMandatoryDependencies()
	var failedDeps []depFailure
	var successDeps []string

	budget := dependencyWaitBudget(logger)
	// ONE deadline shared by every dependency, so total added boot latency is
	// the budget — not the budget multiplied by the number of dependencies.
	deadline := time.Now().Add(budget)

	logger.Info("╔══════════════════════════════════════════════════════════════════╗")
	logger.Info("║           MANDATORY DEPENDENCY VERIFICATION                       ║")
	logger.Info("╚══════════════════════════════════════════════════════════════════╝")
	logger.WithField("wait_budget", budget).Info("Waiting up to this long for slow-starting dependencies")

	for _, dep := range dependencies {
		logger.WithField("dependency", dep.Name).Info("Checking dependency...")

		result := probeDependencyUntil(dep, deadline, logger)
		if result.err != nil {
			failedDeps = append(failedDeps, depFailure{
				name:     dep.Name,
				err:      result.err,
				kind:     result.kind,
				attempts: result.attempts,
			})
			logger.WithFields(logrus.Fields{
				"dependency":  dep.Name,
				"description": dep.Description,
				"state":       result.kind.String(),
				"attempts":    result.attempts,
				"waited":      result.waited.Round(time.Second),
				"error":       result.err,
			}).Error("❌ DEPENDENCY FAILED")
			continue
		}

		successDeps = append(successDeps, dep.Name)
		fields := logrus.Fields{
			"dependency":  dep.Name,
			"description": dep.Description,
		}
		if result.attempts > 1 {
			fields["attempts"] = result.attempts
			fields["waited"] = result.waited.Round(time.Second)
		}
		logger.WithFields(fields).Info("✅ DEPENDENCY OK")
	}

	logger.Info("────────────────────────────────────────────────────────────────────")
	logger.WithFields(logrus.Fields{
		"total":  len(dependencies),
		"passed": len(successDeps),
		"failed": len(failedDeps),
	}).Info("Dependency verification summary")

	if len(failedDeps) > 0 {
		return fmt.Errorf("%s", formatBootBlockedMessage(failedDeps, len(dependencies), budget))
	}

	return nil
}

// formatBootBlockedMessage renders the operator-facing boot failure.
//
// HXC-228: the previous message unconditionally advised "docker-compose up -d /
// make docker-start". When the dependency was already starting, that advice
// pointed the operator at a stack that was demonstrably already running and
// sent them debugging the wrong thing. The remedy now follows the actual
// failure kind, and is only offered for dependencies that really are absent.
func formatBootBlockedMessage(failures []depFailure, total int, budget time.Duration) string {
	var anyAbsent, anyStarting, anyRejected bool

	msg := fmt.Sprintf("BOOT BLOCKED: %d of %d mandatory dependencies failed after waiting up to %s:\n",
		len(failures), total, budget)
	for i, f := range failures {
		switch f.kind {
		case depAbsent:
			anyAbsent = true
		case depRejected:
			anyRejected = true
		default:
			anyStarting = true
		}
		msg += fmt.Sprintf("  %d. %s [%s after %d attempt(s)]: %v\n", i+1, f.name, f.kind, f.attempts, f.err)
	}

	msg += "\nHelixAgent REQUIRES all integration components to be running.\n"

	if anyRejected {
		msg += "\nREJECTED OUR CREDENTIALS — these dependencies are up and answering, but\n"
		msg += "refused our credentials. This is a configuration error, not a timing one:\n"
		msg += "waiting longer and restarting the stack will BOTH fail. Check the relevant\n"
		msg += "secrets (DB_PASSWORD, REDIS_PASSWORD, API keys) in your .env.\n"
	}

	if anyAbsent {
		msg += "\nNOT RUNNING — nothing is listening on these endpoints. The dependency\n"
		msg += "stack is not up (or is bound to different ports than configured).\n"
		msg += "Start it with: make docker-start\n"
	}
	if anyStarting {
		msg += "\nSTILL STARTING — these endpoints accepted a connection but never served a\n"
		msg += "healthy response. The dependency IS running and did not finish starting in\n"
		msg += "time; starting the stack again will NOT help. Check that dependency's own\n"
		msg += "logs and health status. If it legitimately needs longer than the current\n"
		msg += fmt.Sprintf("budget, raise HELIXAGENT_DEP_WAIT_SECONDS (currently %d).\n", int(budget.Seconds()))
	}

	return strings.TrimRight(msg, "\n")
}

// runStartupVerification performs unified startup verification using LLMsVerifier
// as the single source of truth for all provider verification and scoring.
// Returns the startup result and verifier instance (both may be nil if verification fails)
func runStartupVerification(logger *logrus.Logger) (*verifier.StartupResult, *verifier.StartupVerifier) {
	if logger == nil {
		logger = logrus.New()
	}

	// Check if verifier is disabled via environment variable
	if os.Getenv("LLM_VERIFIER_DISABLED") == "true" {
		logger.Info("LLMsVerifier startup verification disabled via LLM_VERIFIER_DISABLED environment variable")
		return nil, nil
	}

	logger.Info("╔══════════════════════════════════════════════════════════════════╗")
	logger.Info("║         UNIFIED PROVIDER STARTUP VERIFICATION                     ║")
	logger.Info("║     LLMsVerifier as Single Source of Truth for ALL Providers     ║")
	logger.Info("╚══════════════════════════════════════════════════════════════════╝")

	// Create startup config with defaults
	cfg := verifier.DefaultStartupConfig()
	cfg.ParallelVerification = true
	cfg.EnableFreeProviders = false
	cfg.TrustOAuthOnFailure = true
	cfg.VerificationTimeout = 30 * time.Second
	cfg.MaxConcurrency = 10
	cfg.CacheVerificationResults = true

	// Create startup verifier
	sv := verifier.NewStartupVerifier(cfg, logger)

	// CRITICAL: Wire up the provider function so verification can make actual API calls
	// Without this, the verification service cannot test providers
	sv.SetProviderFunc(func(ctx context.Context, modelID, providerType, prompt string) (string, error) {
		provider := createProviderForVerification(providerType, modelID, logger)
		if provider == nil {
			return "", fmt.Errorf("unable to create provider %s for verification (check API key)", providerType)
		}

		req := &models.LLMRequest{
			ID:        fmt.Sprintf("verify_%s_%d", modelID, time.Now().UnixNano()),
			SessionID: "startup_verification",
			Prompt:    prompt,
			Messages: []models.Message{
				{Role: "user", Content: prompt},
			},
			ModelParams: models.ModelParameters{
				Model:       modelID,
				MaxTokens:   100,
				Temperature: 0.1,
			},
			Status:    "pending",
			CreatedAt: time.Now(),
		}

		resp, err := provider.Complete(ctx, req)
		if err != nil {
			return "", err
		}
		return resp.Content, nil
	})

	// CRITICAL: Wire up instance creator so API key providers get Instance set
	// Without this, only OAuth providers appear in the debate team
	sv.SetInstanceCreator(func(providerType, modelID string) llm.LLMProvider {
		return createProviderForVerification(providerType, modelID, logger)
	})

	// Create context with timeout for verification
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Run full verification pipeline
	// Phase 1: Discover all providers (OAuth, API Key, Free)
	// Phase 2: Verify all providers in parallel
	// Phase 3: Score all verified providers
	// Phase 4: Rank providers by score (OAuth first, then by score)
	// Phase 5: Select AI Debate Team (15 LLMs)
	result, err := sv.VerifyAllProviders(ctx)
	if err != nil {
		logger.WithError(err).Warn("Startup verification encountered errors")
		// Don't fail boot - continue with available providers
	}

	if result == nil {
		logger.Warn("Startup verification returned nil result, continuing with legacy discovery")
		return nil, nil
	}

	// Log verification summary
	logger.Info("────────────────────────────────────────────────────────────────────")
	logger.WithFields(logrus.Fields{
		"total_providers":   result.TotalProviders,
		"verified":          result.VerifiedCount,
		"failed":            result.FailedCount,
		"skipped":           result.SkippedCount,
		"api_key_providers": result.APIKeyProviders,
		"oauth_providers":   result.OAuthProviders,
		"free_providers":    result.FreeProviders,
	}).Info("Provider verification summary")

	// Log any errors
	for _, e := range result.Errors {
		logger.WithFields(logrus.Fields{
			"provider":    e.Provider,
			"phase":       e.Phase,
			"error":       e.Error,
			"recoverable": e.Recoverable,
		}).Warn("Provider verification error")
	}

	// Log ranked providers
	rankedProviders := sv.GetRankedProviders()
	if len(rankedProviders) > 0 {
		logger.Info("Top verified providers by score:")
		for i, p := range rankedProviders {
			if i >= 5 {
				break
			}
			logger.WithFields(logrus.Fields{
				"rank":      i + 1,
				"provider":  p.Name,
				"type":      p.Type,
				"auth_type": p.AuthType,
				"score":     p.Score,
				"verified":  p.Verified,
				"models":    len(p.Models),
			}).Info("Provider ranked")
		}
	}

	// Log debate team selection with full visual representation
	if result.DebateTeam != nil {
		logger.Info("════════════════════════════════════════════════════════════════════")
		logger.Info("AI DEBATE TEAM SELECTION")
		logger.Info(fmt.Sprintf("Total LLMs: %d | Positions: %d | Sorted by Score: %v | LLM Reuse: %d",
			result.DebateTeam.TotalLLMs,
			len(result.DebateTeam.Positions),
			result.DebateTeam.SortedByScore,
			result.DebateTeam.LLMReuseCount))
		logger.Info("════════════════════════════════════════════════════════════════════")

		for i, pos := range result.DebateTeam.Positions {
			logger.Info(fmt.Sprintf("────────────────────────────────────────────────────────────────────"))
			logger.Info(fmt.Sprintf("POSITION %d: %s", pos.Position, pos.Role))
			logger.Info(fmt.Sprintf("────────────────────────────────────────────────────────────────────"))

			// Log primary LLM
			if pos.Primary != nil {
				oauthStr := ""
				if pos.Primary.IsOAuth {
					oauthStr = " [OAuth]"
				}
				logger.Info(fmt.Sprintf("  ★ PRIMARY: %s/%s (Score: %.2f)%s",
					pos.Primary.Provider, pos.Primary.ModelName, pos.Primary.Score, oauthStr))
			} else {
				logger.Warn(fmt.Sprintf("  ⚠ PRIMARY: Not assigned"))
			}

			// Log all fallback LLMs
			if len(pos.Fallbacks) > 0 {
				for j, fb := range pos.Fallbacks {
					oauthStr := ""
					if fb.IsOAuth {
						oauthStr = " [OAuth]"
					}
					logger.Info(fmt.Sprintf("  → FALLBACK %d: %s/%s (Score: %.2f)%s",
						j+1, fb.Provider, fb.ModelName, fb.Score, oauthStr))
				}
			} else {
				logger.Info(fmt.Sprintf("  → No fallbacks assigned"))
			}

			// Log total for this position
			total := 0
			if pos.Primary != nil {
				total = 1
			}
			total += len(pos.Fallbacks)
			logger.Info(fmt.Sprintf("  [Position %d Total: %d LLMs]", i+1, total))
		}

		logger.Info("════════════════════════════════════════════════════════════════════")
	}

	logger.Info("════════════════════════════════════════════════════════════════════")

	return result, sv
}

// createProviderForVerification creates a temporary LLM provider for verification
// This allows the StartupVerifier to make actual API calls to verify providers
func createProviderForVerification(providerType, modelID string, logger *logrus.Logger) llm.LLMProvider {
	// Create provider based on type
	switch strings.ToLower(providerType) {
	case "claude", "anthropic":
		apiKey := os.Getenv("CLAUDE_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if apiKey == "" {
			// Try OAuth credentials
			provider, err := claude.NewClaudeProviderWithOAuth("", modelID)
			if err == nil && provider != nil {
				return provider
			}
			return nil
		}
		return claude.NewClaudeProvider(apiKey, "", modelID)

	case "deepseek":
		apiKey := os.Getenv("DEEPSEEK_API_KEY")
		if apiKey == "" {
			return nil
		}
		return deepseek.NewDeepSeekProvider(apiKey, "", modelID)

	case "gemini", "google":
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("GOOGLE_API_KEY")
		}
		if apiKey == "" {
			return nil
		}
		return gemini.NewGeminiProvider(apiKey, "", modelID)

	case "mistral":
		apiKey := os.Getenv("MISTRAL_API_KEY")
		if apiKey == "" {
			return nil
		}
		return mistral.NewMistralProvider(apiKey, "", modelID)

	case "openrouter":
		apiKey := os.Getenv("OPENROUTER_API_KEY")
		if apiKey == "" {
			return nil
		}
		return openrouter.NewSimpleOpenRouterProvider(apiKey)

	case "qwen", "dashscope":
		apiKey := os.Getenv("QWEN_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("DASHSCOPE_API_KEY")
		}
		if apiKey == "" {
			// Try OAuth credentials
			provider, err := qwen.NewQwenProviderWithOAuth("", modelID)
			if err == nil && provider != nil {
				return provider
			}
			return nil
		}
		return qwen.NewQwenProvider(apiKey, "", modelID)

	case "zai", "zhipu", "glm":
		apiKey := os.Getenv("ZAI_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("ZHIPU_API_KEY")
		}
		if apiKey == "" {
			return nil
		}
		return zai.NewZAIProvider(apiKey, "", modelID)

	case "zen", "opencode":
		// Zen provider works anonymously
		return zen.NewZenProviderAnonymous(modelID)

	case "cerebras":
		apiKey := os.Getenv("CEREBRAS_API_KEY")
		if apiKey == "" {
			return nil
		}
		return cerebras.NewCerebrasProvider(apiKey, "", modelID)

	case "fireworks":
		apiKey := os.Getenv("FIREWORKS_API_KEY")
		if apiKey == "" {
			return nil
		}
		return fireworks.NewProvider(apiKey, "", modelID)

	case "groq":
		apiKey := os.Getenv("GROQ_API_KEY")
		if apiKey == "" {
			return nil
		}
		return groq.NewProvider(apiKey, "", modelID)

	case "huggingface", "hf":
		apiKey := os.Getenv("HUGGINGFACE_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("HF_API_KEY")
		}
		if apiKey == "" {
			return nil
		}
		return huggingface.NewProvider(apiKey, "", modelID)

	case "replicate":
		apiKey := os.Getenv("REPLICATE_API_KEY")
		if apiKey == "" {
			return nil
		}
		return replicate.NewProvider(apiKey, "", modelID)

	case "ollama":
		baseURL := os.Getenv("OLLAMA_BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return ollama.NewOllamaProvider(baseURL, modelID)

	case "openai":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil
		}
		return openai.NewProvider(apiKey, "", modelID)

	case "cohere":
		apiKey := os.Getenv("COHERE_API_KEY")
		if apiKey == "" {
			return nil
		}
		return cohere.NewProvider(apiKey, "", modelID)

	case "ai21":
		apiKey := os.Getenv("AI21_API_KEY")
		if apiKey == "" {
			return nil
		}
		return ai21.NewProvider(apiKey, "", modelID)

	case "together":
		apiKey := os.Getenv("TOGETHER_API_KEY")
		if apiKey == "" {
			return nil
		}
		return together.NewProvider(apiKey, "", modelID)

	case "grok", "xai":
		apiKey := os.Getenv("XAI_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("GROK_API_KEY")
		}
		if apiKey == "" {
			return nil
		}
		return xai.NewProvider(apiKey, "", modelID)

	case "perplexity":
		apiKey := os.Getenv("PERPLEXITY_API_KEY")
		if apiKey == "" {
			return nil
		}
		return perplexity.NewProvider(apiKey, "", modelID)

	case "chutes":
		apiKey := os.Getenv("CHUTES_API_KEY")
		if apiKey == "" {
			return nil
		}
		return chutes.NewProvider(apiKey, "", modelID)

	case "nvidia":
		apiKey := os.Getenv("NVIDIA_API_KEY")
		if apiKey == "" {
			return nil
		}
		return nvidia.NewNvidiaProvider(apiKey, "", modelID)

	case "codestral":
		apiKey := os.Getenv("CODESTRAL_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("MISTRAL_API_KEY")
		}
		if apiKey == "" {
			return nil
		}
		return codestral.NewCodestralProvider(apiKey, "", modelID)

	case "upstage":
		apiKey := os.Getenv("UPSTAGE_API_KEY")
		if apiKey == "" {
			return nil
		}
		return upstage.NewUpstageProvider(apiKey, "", modelID)

	case "cloudflare":
		apiKey := os.Getenv("CLOUDFLARE_API_KEY")
		accountID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
		if apiKey == "" {
			return nil
		}
		return cloudflare.NewCloudflareProvider(apiKey, accountID, "", modelID)

	case "github-models":
		apiKey := os.Getenv("GITHUB_MODELS_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("GITHUB_TOKEN")
		}
		if apiKey == "" {
			return nil
		}
		return githubmodels.NewGitHubModelsProvider(apiKey, "", modelID)

	case "venice":
		apiKey := os.Getenv("VENICE_API_KEY")
		if apiKey == "" {
			return nil
		}
		return venice.NewProvider(apiKey, "", modelID)

	default:
		// Try generic OpenAI-compatible provider using SupportedProviders config
		info, ok := verifier.GetProviderInfo(providerType)
		if !ok || info.BaseURL == "" {
			if logger != nil {
				logger.WithField("provider", providerType).Debug("No provider info for verification")
			}
			return nil
		}
		// Find API key from environment
		apiKey := ""
		for _, envVar := range info.EnvVars {
			if v := os.Getenv(envVar); v != "" {
				apiKey = v
				break
			}
		}
		if apiKey == "" {
			return nil
		}
		model := modelID
		if model == "" && len(info.Models) > 0 {
			model = info.Models[0]
		}
		if model == "" {
			if logger != nil {
				logger.WithField("provider", providerType).Debug("No model available for generic verification")
			}
			return nil
		}
		return generic.NewGenericProvider(providerType, apiKey, info.BaseURL, model)
	}
}

// AppConfig holds application configuration for testing
type AppConfig struct {
	ShowHelp           bool
	ShowVersion        bool
	AutoStartDocker    bool
	StrictDependencies bool // MANDATORY: If true, fail boot when ANY dependency is unavailable
	GenerateAPIKey     bool
	GenerateOpenCode   bool
	ValidateOpenCode   string
	OpenCodeOutput     string
	GenerateCrush      bool
	ValidateCrush      string
	CrushOutput        string
	APIKeyEnvFile      string
	PreinstallMCP      bool // Run MCP package pre-installation and exit
	SkipMCPPreinstall  bool // Skip automatic MCP pre-installation at startup
	AutoStartMCP       bool // Automatically start MCP Docker containers on startup
	// Unified CLI agent configuration (all 48 agents)
	GenerateAgentConfig string // Agent type to generate config for
	ValidateAgentConfig string // Agent:path for validation
	AgentConfigOutput   string // Output path for generated config
	ListAgents          bool   // List all supported agents
	GenerateAllAgents   bool   // Generate configs for all agents
	AllAgentsOutputDir  string // Output directory for all agent configs
	// Challenge execution
	RunChallenges           string        // "all" | category | challenge-id
	ListChallenges          bool          // List all challenges
	ChallengeParallel       bool          // Parallel execution
	ChallengeVerbose        bool          // Verbose output
	ChallengeStallThreshold time.Duration // Stall threshold for stuck detection
	SkipVerification        bool          // Skip startup provider verification (for tests)
	ServerHost              string
	ServerPort              string
	Logger                  *logrus.Logger
	ShutdownSignal          chan os.Signal
	ReadyNotify             chan struct{} // Closed when server is ready to accept shutdown (for tests)
}

// DefaultAppConfig returns the default application configuration
func DefaultAppConfig() *AppConfig {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	return &AppConfig{
		ShowHelp:           false,
		ShowVersion:        false,
		AutoStartDocker:    true,
		AutoStartMCP:       true, // Auto-start all 32 MCP Docker containers
		StrictDependencies: true, // MANDATORY: All dependencies must be available
		ServerHost:         "0.0.0.0",
		ServerPort:         "8100",
		Logger:             logger,
		ShutdownSignal:     nil,
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

	// Handle Crush config validation command
	if appCfg.ValidateCrush != "" {
		return handleValidateCrush(appCfg)
	}

	// Handle Crush config generation command
	if appCfg.GenerateCrush {
		return handleGenerateCrush(appCfg)
	}

	// Handle unified CLI agent commands (all 48 agents)
	if appCfg.ListAgents {
		return handleListAgents(appCfg)
	}

	if appCfg.GenerateAllAgents {
		return handleGenerateAllAgents(appCfg)
	}

	if appCfg.GenerateAgentConfig != "" {
		return handleGenerateAgentConfig(appCfg)
	}

	if appCfg.ValidateAgentConfig != "" {
		return handleValidateAgentConfig(appCfg)
	}

	// Handle challenge commands
	if appCfg.ListChallenges {
		return handleListChallenges(appCfg)
	}

	if appCfg.RunChallenges != "" {
		return handleRunChallenges(appCfg)
	}

	// Handle MCP pre-installation command
	if appCfg.PreinstallMCP {
		return handlePreinstallMCP(appCfg)
	}

	// Load full configuration from environment variables
	cfg := config.Load()

	// Override with command-line specified values if provided
	if appCfg.ServerHost != "" && appCfg.ServerHost != "0.0.0.0" {
		cfg.Server.Host = appCfg.ServerHost
	}
	if appCfg.ServerPort != "" && appCfg.ServerPort != "8100" {
		cfg.Server.Port = appCfg.ServerPort
	}
	// Skip auto-discovery when verification is skipped (test mode)
	if appCfg.SkipVerification {
		cfg.LLM.DisableAutoDiscovery = true
	}

	logger := appCfg.Logger
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.InfoLevel)
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	// Log service configuration for troubleshooting
	logger.WithFields(logrus.Fields{
		"postgresql_remote":  cfg.Services.PostgreSQL.Remote,
		"postgresql_enabled": cfg.Services.PostgreSQL.Enabled,
		"redis_remote":       cfg.Services.Redis.Remote,
		"redis_enabled":      cfg.Services.Redis.Enabled,
	}).Debug("Service configuration loaded")

	// Auto-configure HelixLLM TLS cert trust.
	// HelixLLM uses self-signed TLS certs with SANs. CLI agents (OpenCode etc.)
	// and Go/Node.js HTTP clients need SSL_CERT_FILE / NODE_EXTRA_CA_CERTS
	// to trust the cert. This creates a combined CA bundle automatically.
	configureHelixLLMTLS(logger)

	// Initialize the centralized container adapter (Containers module).
	// Uses NewAdapterFromConfig to auto-load containers/.env for
	// remote distribution, bootstrap SSH key auth, and configure
	// SSH options.
	adapter, adapterErr := containeradapter.NewAdapterFromConfig(cfg)
	if adapterErr != nil {
		logger.WithError(adapterErr).Warn(
			"Container adapter initialization failed",
		)
	} else {
		globalContainerAdapter = adapter
		if name, err := adapter.DetectRuntime(
			context.Background(),
		); err == nil {
			logger.WithField("runtime", name).Info(
				"Container adapter initialized via Containers module",
			)
		}
		if adapter.RemoteEnabled() {
			hosts := adapter.ListHosts()
			logger.WithField("hosts", len(hosts)).Info(
				"Remote container distribution enabled",
			)
		}
	}

	// Initialize lazy container orchestrator for on-demand services
	// This manages services like NVIDIA RAG that start only when first requested
	logger.Info("Initializing lazy container orchestrator...")
	lazyOrch, err := containers.NewLazyOrchestrator("", containers.NewLogrusLogger(logger))
	if err != nil {
		logger.WithError(err).Warn("Failed to initialize lazy orchestrator, continuing without lazy services")
	} else {
		globalLazyOrchestrator = lazyOrch
		// Register default lazy services (NVIDIA RAG, MCP servers)
		if err := containers.InitializeDefaultServices(lazyOrch); err != nil {
			logger.WithError(err).Warn("Failed to register lazy services, continuing")
		} else {
			services := lazyOrch.ListServices()
			logger.WithField("count", len(services)).Info("Lazy container orchestrator initialized with services")
		}
	}

	// Unified service boot manager: starts all enabled local services and health-checks all
	bootMgr := services.NewBootManager(&cfg.Services, logger)

	// Set container adapter for remote health checks
	if globalContainerAdapter != nil {
		bootMgr.SetContainerAdapter(globalContainerAdapter)
	}

	if appCfg.AutoStartDocker {
		logger.Info("Booting all configured services via unified BootManager...")
		if err := bootMgr.BootAll(); err != nil {
			if appCfg.StrictDependencies {
				return fmt.Errorf("FATAL: Service boot failed (strict mode enabled): %w", err)
			}
			logger.WithError(err).Warn("Some services failed to boot, continuing with application startup")
		} else {
			logger.Info("All services booted successfully")
		}
	} else {
		// Even without auto-start, run health checks if strict mode
		if appCfg.StrictDependencies {
			logger.Info("Verifying ALL integration dependencies (strict mode)...")
			if err := verifyAllMandatoryDependencies(logger); err != nil {
				return fmt.Errorf("FATAL: Integration dependency verification failed: %w", err)
			}
			logger.Info("All mandatory dependencies verified successfully")
		}
	}

	// Auto-start MCP servers from git submodules (32 servers, zero npm dependencies)
	if appCfg.AutoStartMCP {
		logger.Info("Starting MCP servers from git submodules...")
		if err := ensureMCPServers(logger); err != nil {
			logger.WithError(err).Warn("Failed to start MCP servers, continuing without them")
		}
	}

	// LSP and RAG infrastructure is now managed by BootManager.BootAll()
	// called during startup verification. No separate orchestration needed.
	logger.Info("Infrastructure startup delegated to BootManager")

	// Drainage report 2026-04-25 Finding #2: bind a tiny liveness probe on
	// the dedicated HelixAgentLiveness port (8111) BEFORE the ~7-min
	// startup verification pipeline. External supervisors / watchdogs /
	// load-balancers can hit this probe immediately to distinguish
	// "starting" from "hung". Stays up for the lifetime of the process so
	// the probe shape is stable.
	livenessProbe := health.NewLiveness(logger)
	if err := livenessProbe.Start(); err != nil {
		logger.WithError(err).Warn("Liveness probe failed to bind — continuing without it")
	}

	// Run unified startup verification (LLMsVerifier as single source of truth)
	// This verifies ALL providers (OAuth, API Key, Free) and selects the AI Debate Team
	var startupResult *verifier.StartupResult
	var startupVerifier *verifier.StartupVerifier
	if appCfg.SkipVerification {
		logger.Info("Startup verification skipped (SkipVerification=true)")
	} else {
		startupResult, startupVerifier = runStartupVerification(logger)
	}
	// Mark the binary ready as soon as verification completes — the main
	// HTTP server still needs to bind below, but the worst delay (the
	// provider-verification wait) is over and the process is functional.
	livenessProbe.SetReady()
	if startupResult != nil {
		logger.WithFields(logrus.Fields{
			"total_providers": startupResult.TotalProviders,
			"verified_count":  startupResult.VerifiedCount,
			"failed_count":    startupResult.FailedCount,
			"oauth_providers": startupResult.OAuthProviders,
			"free_providers":  startupResult.FreeProviders,
			"duration_ms":     startupResult.DurationMs,
		}).Info("Startup verification completed")

		if startupResult.DebateTeam != nil {
			logger.WithFields(logrus.Fields{
				"debate_team_llms": startupResult.DebateTeam.TotalLLMs,
				"debate_positions": len(startupResult.DebateTeam.Positions),
				"sorted_by_score":  startupResult.DebateTeam.SortedByScore,
				"llm_reuse_count":  startupResult.DebateTeam.LLMReuseCount,
			}).Info("AI Debate Team configured (up to 25 LLMs, score-based selection)")
		}
	}

	// Store startup verifier for router access
	_ = startupVerifier // Used by router.SetupRouterWithVerifier if available

	// Initialize messaging system with in-memory fallback
	// This provides RabbitMQ-style task queuing and Kafka-style event streaming
	logger.Info("Initializing messaging system...")
	msgCtx, msgCancel := context.WithTimeout(context.Background(), 30*time.Second)
	msgSystem, err := messaging.InitializeGlobalMessagingSystem(msgCtx, logger, func() messaging.MessageBroker {
		return inmemory.NewBroker(nil)
	})
	msgCancel()
	if err != nil {
		logger.WithError(err).Warn("Failed to initialize messaging system, continuing without messaging")
	} else {
		logger.WithFields(logrus.Fields{
			"initialized":   msgSystem.IsInitialized(),
			"fallback_mode": msgSystem.Config.FallbackToInMemory,
		}).Info("Messaging system initialized")
	}

	var bigDataIntegration *bigdata.BigDataIntegration
	// Initialize BigData integration (optional)
	logger.Info("Initializing BigData integration...")
	bigDataConfig := bigdata.ConfigToIntegrationConfig(&cfg.BigData)

	var messageBroker messaging.MessageBroker
	if msgSystem != nil && msgSystem.Hub != nil {
		// Try to get event stream broker from hub
		messageBroker = msgSystem.Hub.GetMessageBroker()
		if messageBroker == nil {
			// Fallback to in-memory broker
			logger.Warn("No message broker available from hub, using in-memory fallback")
			messageBroker = inmemory.NewBroker(nil)
		} else {
			logger.WithField("broker_type", messageBroker.BrokerType()).Info("Using message broker from messaging hub")
		}
	} else {
		// No messaging system, use in-memory broker
		logger.Warn("Messaging system not available, using in-memory broker for BigData")
		messageBroker = inmemory.NewBroker(nil)
	}
	bigDataIntegration, err = bigdata.NewBigDataIntegration(bigDataConfig, messageBroker, logger)
	if err != nil {
		logger.WithError(err).Warn("Failed to create BigData integration, continuing without it")
	} else {
		if err := bigDataIntegration.Initialize(context.Background()); err != nil {
			logger.WithError(err).Warn("Failed to initialize BigData integration, continuing without it")
		} else {
			logger.WithFields(logrus.Fields{
				"infinite_context":   cfg.BigData.EnableInfiniteContext,
				"distributed_memory": cfg.BigData.EnableDistributedMemory,
				"knowledge_graph":    cfg.BigData.EnableKnowledgeGraph,
				"analytics":          cfg.BigData.EnableAnalytics,
				"cross_learning":     cfg.BigData.EnableCrossLearning,
			}).Info("BigData integration initialized successfully")
		}
	}

	routerCtx := router.SetupRouterWithContext(cfg)
	r := routerCtx.Engine

	// Inject container adapter into CogneeService for centralized container management.
	if globalContainerAdapter != nil && routerCtx.CogneeService != nil {
		routerCtx.CogneeService.ContainerAdapter = globalContainerAdapter
	}

	// CRITICAL: Set StartupVerifier on the router's ProviderRegistry
	// This enables OAuth providers (Claude, Qwen) to be included in the DebateTeamConfig
	if startupVerifier != nil && routerCtx.ProviderRegistry != nil {
		routerCtx.ProviderRegistry.SetStartupVerifier(startupVerifier)
		logger.Info("StartupVerifier configured on router's ProviderRegistry")

		// Re-initialize DebateTeamConfig with StartupVerifier to include OAuth providers
		if err := routerCtx.ReinitializeDebateTeam(context.Background()); err != nil {
			logger.WithError(err).Warn("Failed to re-initialize debate team with StartupVerifier")
		} else {
			logger.Info("DebateTeamConfig re-initialized with StartupVerifier (OAuth providers now included)")
		}
	}

	// Add startup verification status endpoint
	// This endpoint exposes LLMsVerifier re-evaluation results for validation
	r.GET("/v1/startup/verification", func(c *gin.Context) {
		if startupResult == nil {
			c.JSON(503, gin.H{
				"error":   "startup verification not completed",
				"message": "No startup verification result available",
			})
			return
		}

		response := gin.H{
			"reevaluation_completed": true,
			"started_at":             startupResult.StartedAt,
			"completed_at":           startupResult.CompletedAt,
			"duration_ms":            startupResult.DurationMs,
			"total_providers":        startupResult.TotalProviders,
			"verified_count":         startupResult.VerifiedCount,
			"failed_count":           startupResult.FailedCount,
			"skipped_count":          startupResult.SkippedCount,
			"api_key_providers":      startupResult.APIKeyProviders,
			"oauth_providers":        startupResult.OAuthProviders,
			"free_providers":         startupResult.FreeProviders,
			"errors_count":           len(startupResult.Errors),
			// Subscription breakdown
			"free_provider_count":          startupResult.FreeProviderCount,
			"free_credit_provider_count":   startupResult.FreeCreditProviderCount,
			"pay_as_you_go_provider_count": startupResult.PayAsYouGoProviderCount,
			"subscription_detected_count":  startupResult.SubscriptionDetectedCount,
		}

		// Add ranked providers with scores
		if startupVerifier != nil {
			rankedProviders := startupVerifier.GetRankedProviders()
			providerScores := make([]gin.H, 0, len(rankedProviders))
			for i, p := range rankedProviders {
				entry := gin.H{
					"rank":                 i + 1,
					"provider":             p.Name,
					"type":                 p.Type,
					"auth_type":            p.AuthType,
					"score":                p.Score,
					"verified":             p.Verified,
					"verified_at":          p.VerifiedAt,
					"models":               len(p.Models),
					"code_visible":         p.CodeVisible,
					"failure_reason":       p.FailureReason,
					"failure_category":     p.FailureCategory,
					"test_details":         p.TestDetails,
					"verification_message": p.VerificationMsg,
					"error_message":        p.ErrorMessage,
					"subscription":         p.Subscription,
					"access_config":        p.AccessConfig,
				}
				providerScores = append(providerScores, entry)
			}
			response["ranked_providers"] = providerScores
			response["providers_sorted"] = len(rankedProviders) > 0
		}

		// Add debate team info
		if startupResult.DebateTeam != nil {
			response["debate_team"] = gin.H{
				"total_llms":      startupResult.DebateTeam.TotalLLMs,
				"positions":       len(startupResult.DebateTeam.Positions),
				"min_score":       startupResult.DebateTeam.MinScore,
				"sorted_by_score": startupResult.DebateTeam.SortedByScore,
				"llm_reuse_count": startupResult.DebateTeam.LLMReuseCount,
				"selected_at":     startupResult.DebateTeam.SelectedAt,
				"team_configured": true,
			}
		} else {
			response["debate_team"] = gin.H{
				"team_configured": false,
			}
		}

		c.JSON(200, response)
	})

	// BigData endpoints (if integration initialized)
	if bigDataIntegration != nil {
		logger.Info("Registering BigData endpoints")

		// Create debate integration for conversation context access
		var debateIntegration *bigdata.DebateIntegration
		if bigDataIntegration.GetInfiniteContext() != nil && messageBroker != nil {
			debateIntegration = bigdata.NewDebateIntegration(
				bigDataIntegration.GetInfiniteContext(),
				messageBroker,
				logger,
			)
			logger.Info("✓ Debate integration created for BigData endpoints")
		} else {
			logger.Warn("Cannot create debate integration: infinite context or message broker unavailable")
		}

		// Create BigData handler with all integrations
		bigDataHandler := bigdata.NewHandler(
			bigDataIntegration,
			debateIntegration,
			logger,
		)

		// Register all BigData routes (includes /v1/bigdata/health, /v1/context/*, /v1/memory/*, etc.)
		bigDataHandler.RegisterRoutes(r)

		// Keep legacy /v1/bigdata/components endpoint for backward compatibility
		r.GET("/v1/bigdata/components", func(c *gin.Context) {
			components := make(map[string]bool)
			components["infinite_context"] = bigDataIntegration.GetInfiniteContext() != nil
			components["distributed_memory"] = bigDataIntegration.GetDistributedMemory() != nil
			components["knowledge_graph"] = bigDataIntegration.GetKnowledgeGraph() != nil
			components["analytics"] = bigDataIntegration.GetAnalytics() != nil
			components["cross_learning"] = bigDataIntegration.GetCrossLearner() != nil
			c.JSON(200, gin.H{
				"components": components,
				"running":    bigDataIntegration.IsRunning(),
			})
		})
	}

	// Start background MCP package pre-installation (unless skipped)
	if !appCfg.SkipMCPPreinstall {
		startBackgroundMCPPreinstall(logger)
	}

	// Create HTTP/3 server with HTTP/2 fallback
	http3Config := &transport.HTTP3Config{
		Address:        fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		EnableHTTP3:    true,
		EnableHTTP2:    true,
		TLSCertFile:    "", // Auto-generate self-signed cert
		TLSKeyFile:     "",
		MaxConnections: 1000,
		IdleTimeout:    0, // Disabled: SSE streams have long gaps between chunks during debate
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   600 * time.Second, // 10 minutes: debate responses can be very large
	}
	http3Server, err := transport.NewHTTP3Server(r, http3Config)
	if err != nil {
		return fmt.Errorf("failed to create HTTP/3 server: %w", err)
	}

	// Channel for server errors
	serverErr := make(chan error, 1)

	go func() {
		logger.WithFields(logrus.Fields{
			"host": cfg.Server.Host,
			"port": cfg.Server.Port,
		}).Info("Starting HelixAgent server with HTTP/3 QUIC and Models.dev integration")

		if err := http3Server.Start(); err != nil {
			serverErr <- err
		}
	}()

	// Start background OAuth token refresh for Claude and Qwen
	stopRefresh := make(chan struct{})
	oauth_credentials.StartBackgroundRefresh(stopRefresh)
	logger.Info("Started background OAuth token refresh for Claude and Qwen")

	// Auto-regenerate CLI agent configs (OpenCode, Crush, etc.) at boot.
	// This ensures configs always reflect the current system state:
	// - HelixLLM provider included only when models are loaded
	// - Correct HelixAgent endpoint + API key
	// - Up-to-date MCP server list
	go func() {
		openCodeOutput := filepath.Join(os.Getenv("HOME"), ".config", "opencode", "opencode.json")
		os.MkdirAll(filepath.Dir(openCodeOutput), 0755)
		if err := handleGenerateOpenCode(&AppConfig{
			Logger:         logger,
			OpenCodeOutput: openCodeOutput,
		}); err != nil {
			logger.WithError(err).Warn("Auto-regenerate OpenCode config failed (non-fatal)")
		} else {
			logger.WithField("path", openCodeOutput).Info("Auto-regenerated OpenCode config at boot")
		}
	}()

	// Use provided shutdown signal or create one
	quit := appCfg.ShutdownSignal
	if quit == nil {
		quit = make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	}

	// Notify test harness that server is ready for shutdown
	if appCfg.ReadyNotify != nil {
		close(appCfg.ReadyNotify)
	}

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErr:
		return fmt.Errorf("server failed to start: %w", err)
	case <-quit:
		// Continue to shutdown
	}

	logger.Info("Shutting down server...")

	// Stop background OAuth token refresh
	close(stopRefresh)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown BigData integration
	if bigDataIntegration != nil && bigDataIntegration.IsRunning() {
		logger.Info("Shutting down BigData integration...")
		if err := bigDataIntegration.Stop(shutdownCtx); err != nil {
			logger.WithError(err).Warn("Error shutting down BigData integration")
		} else {
			logger.Info("BigData integration shutdown complete")
		}
	}

	// Shutdown messaging system
	if msgSystem != nil && msgSystem.IsInitialized() {
		logger.Info("Shutting down messaging system...")
		if err := msgSystem.Close(shutdownCtx); err != nil {
			logger.WithError(err).Warn("Error shutting down messaging system")
		} else {
			logger.Info("Messaging system shutdown complete")
		}
	}

	// Use r variable to avoid unused import
	_ = r

	// Finding #41: a context-deadline-exceeded on HTTP server shutdown is
	// a cleanup-time symptom (in-flight SSE / streaming connections that
	// didn't drain in time), not an application failure. Returning an
	// error here makes the binary exit `level=fatal` even though the
	// shutdown sequence is succeeding for every OTHER component
	// (container adapter, BootManager, messaging system). Log + continue
	// so the rest of the cleanup runs and main exits with status 0.
	if err := http3Server.Stop(); err != nil {
		logger.WithError(err).Warn(
			"HTTP/QUIC server shutdown reported errors — continuing with " +
				"the rest of the cleanup sequence",
		)
	}

	// Shutdown container adapter (tunnels, volumes, distributed containers).
	if globalContainerAdapter != nil {
		logger.Info("Shutting down container adapter...")
		if err := globalContainerAdapter.Shutdown(shutdownCtx); err != nil {
			logger.WithError(err).Warn(
				"Error shutting down container adapter",
			)
		}
	}

	// Stop all managed container services
	if bootMgr != nil {
		logger.Info("Stopping all managed services via BootManager...")
		if err := bootMgr.ShutdownAll(); err != nil {
			logger.WithError(err).Warn("Error stopping managed services")
		}
	}

	logger.Info("Server shutdown complete")
	return nil
}

func main() {
	// Respect container CPU limits via GOMAXPROCS env var.
	// Go reads GOMAXPROCS automatically from the environment, but we log
	// the effective value so operators can confirm resource limits are applied.
	if v := os.Getenv("GOMAXPROCS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			runtime.GOMAXPROCS(n)
		}
	}
	logrus.WithField("gomaxprocs", runtime.GOMAXPROCS(0)).Info("CPU parallelism configured")

	// Load environment variables from .env files.
	//
	// The project's convention is a two-tier env setup:
	//   1. `.env.bak` contains the REAL secret values under alternate names
	//      (ApiKey_Cerebras=<actual-key>, ApiKey_GitHub_Models=<actual-key>, …).
	//   2. `.env` contains the canonical env-var names referencing those secrets
	//      (CEREBRAS_API_KEY=$ApiKey_Cerebras, …).
	//
	// Two godotenv quirks make a naive load fail:
	//   a) godotenv.Load refuses to overwrite shell env vars even if empty →
	//      use Overload so .env is authoritative.
	//   b) godotenv's bare `$VAR` (no braces) variable-name matcher is NOT
	//      greedy on mixed-case identifiers: `$ApiKey_Cerebras` gets parsed as
	//      `$A` followed by the literal `piKey_Cerebras`, producing a 14-char
	//      garbage string as the expanded value. We fix this by running a
	//      second pass with os.ExpandEnv (which IS greedy, cf. `${NAME}`
	//      semantics) after godotenv.Overload has populated the env.
	//
	// Without both halves, every ${ApiKey_*} in .env ends up as garbage and
	// every provider gets 401 Unauthorized. This is the bug caught in
	// SESSION_2026-04-24 late afternoon (Issues #41 + #42).
	//
	// Each file is optional — if missing, we skip without logging an error.
	for _, f := range []string{".env.bak", ".env"} {
		if _, err := os.Stat(f); err != nil {
			continue
		}
		if lerr := godotenv.Overload(f); lerr != nil {
			logrus.WithError(lerr).WithField("file", f).Warn("Could not load env file")
			continue
		}
		// Second pass: godotenv's bare `$VAR` expander is non-greedy on
		// mixed-case identifiers and returns garbage (e.g. `$ApiKey_Cerebras`
		// → `piKey_Cerebras`). Re-parse the FILE ourselves (not godotenv's
		// pre-expanded output) and re-expand each value with os.ExpandEnv,
		// which IS greedy. Set the env only when the raw value contains a `$`.
		if raw, rerr := os.ReadFile(f); rerr == nil {
			for _, line := range strings.Split(string(raw), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				eq := strings.IndexByte(line, '=')
				if eq <= 0 {
					continue
				}
				key := strings.TrimSpace(line[:eq])
				val := strings.TrimSpace(line[eq+1:])
				// Strip surrounding single or double quotes if present
				if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
					val = val[1 : len(val)-1]
				}
				if !strings.Contains(val, "$") {
					continue
				}
				expanded := os.ExpandEnv(val)
				if expanded != val {
					_ = os.Setenv(key, expanded)
				}
			}
		}
	}

	flag.Parse()

	appCfg := DefaultAppConfig()
	appCfg.ShowHelp = *help
	appCfg.ShowVersion = *version
	appCfg.AutoStartDocker = *autoStartDocker
	appCfg.StrictDependencies = *strictDependencies
	appCfg.GenerateAPIKey = *generateAPIKey
	appCfg.GenerateOpenCode = *generateOpenCode
	appCfg.ValidateOpenCode = *validateOpenCode
	appCfg.OpenCodeOutput = *openCodeOutput
	appCfg.GenerateCrush = *generateCrush
	appCfg.ValidateCrush = *validateCrush
	appCfg.CrushOutput = *crushOutput
	appCfg.APIKeyEnvFile = *apiKeyEnvFile
	appCfg.PreinstallMCP = *preinstallMCP
	appCfg.SkipMCPPreinstall = *skipMCPPreinstall
	appCfg.AutoStartMCP = *autoStartMCP
	appCfg.SkipVerification = *skipVerification
	// Unified CLI agent configuration flags
	appCfg.GenerateAgentConfig = *generateAgentConfig
	appCfg.ValidateAgentConfig = *validateAgentConfig
	appCfg.AgentConfigOutput = *agentConfigOutput
	appCfg.ListAgents = *listAgents
	appCfg.GenerateAllAgents = *generateAllAgents
	appCfg.AllAgentsOutputDir = *allAgentsOutputDir
	// Challenge execution flags
	appCfg.RunChallenges = *runChallenges
	appCfg.ListChallenges = *listChallenges
	appCfg.ChallengeParallel = *challengeParallel
	appCfg.ChallengeVerbose = *challengeVerbose
	appCfg.ChallengeStallThreshold = *challengeStallDuration

	if err := run(appCfg); err != nil {
		appCfg.Logger.WithError(err).Fatal("Application failed")
	}
}

// generateSecureAPIKey generates a cryptographically secure API key
// configureHelixLLMTLS auto-configures TLS cert trust for HelixLLM's self-signed cert.
// Creates a combined CA bundle (system CAs + HelixLLM cert) and sets SSL_CERT_FILE
// and NODE_EXTRA_CA_CERTS env vars so Go and Node.js HTTP clients trust the cert.
// Also writes ~/.config/environment.d/helixllm-tls.conf for systemd user sessions.
func configureHelixLLMTLS(logger *logrus.Logger) {
	certPath := filepath.Join("HelixLLM", "certs", "cert.pem")
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		logger.Debug("HelixLLM cert not found, skipping TLS auto-configuration")
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.WithError(err).Warn("Failed to get home directory for TLS config")
		return
	}

	helixDir := filepath.Join(homeDir, ".helixagent")
	if err := os.MkdirAll(helixDir, 0755); err != nil {
		logger.WithError(err).Warn("Failed to create ~/.helixagent directory")
		return
	}

	bundlePath := filepath.Join(helixDir, "ca-bundle.pem")

	// Find system CA bundle
	systemCAPaths := []string{
		"/var/lib/ssl/cert.pem",
		"/etc/ssl/certs/ca-certificates.crt",
		"/etc/pki/tls/certs/ca-bundle.crt",
		"/etc/ssl/cert.pem",
	}
	var systemCAPath string
	for _, p := range systemCAPaths {
		if _, err := os.Stat(p); err == nil {
			systemCAPath = p
			break
		}
	}

	if systemCAPath == "" {
		logger.Warn("No system CA bundle found, skipping TLS auto-configuration")
		return
	}

	// Read both certs
	systemCA, err := os.ReadFile(systemCAPath)
	if err != nil {
		logger.WithError(err).Warn("Failed to read system CA bundle")
		return
	}
	helixCert, err := os.ReadFile(certPath)
	if err != nil {
		logger.WithError(err).Warn("Failed to read HelixLLM cert")
		return
	}

	// Write combined bundle
	combined := append(systemCA, '\n')
	combined = append(combined, helixCert...)
	// #nosec G703 -- bundlePath is derived from the user's $HOME/.helixagent/
	// directory under operator control; never from LLM or HTTP input. Writing
	// the bundle there is the documented secure-default workflow in CLAUDE.md.
	if err := os.WriteFile(bundlePath, combined, 0644); err != nil {
		logger.WithError(err).Warn("Failed to write combined CA bundle")
		return
	}

	absCertPath, _ := filepath.Abs(certPath)

	// Set env vars for current process
	os.Setenv("SSL_CERT_FILE", bundlePath)
	os.Setenv("NODE_EXTRA_CA_CERTS", absCertPath)

	// Write systemd environment.d config for user sessions
	envDir := filepath.Join(homeDir, ".config", "environment.d")
	os.MkdirAll(envDir, 0755)
	envConf := fmt.Sprintf("SSL_CERT_FILE=%s\nNODE_EXTRA_CA_CERTS=%s\n", bundlePath, absCertPath)
	os.WriteFile(filepath.Join(envDir, "helixllm-tls.conf"), []byte(envConf), 0644)

	logger.WithFields(logrus.Fields{
		"bundle":              bundlePath,
		"cert":                absCertPath,
		"ssl_cert_file":       bundlePath,
		"node_extra_ca_certs": absCertPath,
	}).Info("HelixLLM TLS cert trust auto-configured")
}

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
	// Validate path for traversal attacks (G304 security fix)
	// Note: This is a CLI-provided path from the admin user
	if !utils.ValidatePath(filePath) {
		return fmt.Errorf("invalid file path: contains path traversal or dangerous characters")
	}

	// Read existing file contents if it exists
	existingContent := make(map[string]string)
	var lineOrder []string

	// #nosec G304 - filePath is validated by utils.ValidatePath and provided via CLI by admin
	if file, err := os.Open(filePath); err == nil {
		defer func() { _ = file.Close() }()
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
	// #nosec G304 - filePath is validated by utils.ValidatePath at function entry and provided via CLI by admin
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create env file: %w", err)
	}
	defer func() { _ = file.Close() }()

	for _, item := range lineOrder {
		if item == "" || strings.HasPrefix(item, "#") {
			// Write empty lines and comments as-is
			_, _ = fmt.Fprintln(file, item)
		} else if value, ok := existingContent[item]; ok {
			// Write key=value
			_, _ = fmt.Fprintf(file, "%s=%s\n", item, value)
		}
	}

	return nil
}

// OpenCodeConfig represents the CORRECT OpenCode configuration structure
// Based on actual working config and OpenCode documentation
// Uses @ai-sdk/openai-compatible for custom providers
type OpenCodeConfig struct {
	Schema       string                             `json:"$schema,omitempty"`
	Provider     map[string]OpenCodeProviderDefNew  `json:"provider,omitempty"`     // Note: singular "provider"
	Agent        map[string]OpenCodeAgentDefNew     `json:"agent,omitempty"`        // Note: singular "agent"
	MCP          map[string]OpenCodeMCPServerDefNew `json:"mcp,omitempty"`          // OpenCode expects "mcp" key
	Plugin       []string                           `json:"plugin,omitempty"`       // OpenCode plugins (e.g., "opencode-agent-skills@0.6.5")
	Instructions []string                           `json:"instructions,omitempty"` // Rule files
	TUI          *OpenCodeTUIDef                    `json:"tui,omitempty"`
}

// OpenCodeProviderDefNew represents a provider in OpenCode config (correct schema)
type OpenCodeProviderDefNew struct {
	NPM     string                         `json:"npm,omitempty"`     // e.g., "@ai-sdk/openai-compatible"
	Name    string                         `json:"name,omitempty"`    // Display name
	Options *OpenCodeProviderOptionsNew    `json:"options,omitempty"` // Provider options
	Models  map[string]OpenCodeModelDefNew `json:"models,omitempty"`  // Model definitions (map, not array)
}

// OpenCodeProviderOptionsNew represents provider options
type OpenCodeProviderOptionsNew struct {
	BaseURL string            `json:"baseURL,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	APIKey  string            `json:"apiKey,omitempty"` // Can use {env:VAR_NAME} syntax
}

// OpenCodeModelDefNew represents a model definition
type OpenCodeModelDefNew struct {
	Name    string              `json:"name,omitempty"`    // Display name
	Limit   *OpenCodeModelLimit `json:"limit,omitempty"`   // Token limits
	Options map[string]any      `json:"options,omitempty"` // Model-specific options
}

// OpenCodeModelLimit represents token limits for a model
type OpenCodeModelLimit struct {
	Context int64 `json:"context,omitempty"` // Context window size
	Output  int64 `json:"output,omitempty"`  // Max output tokens
}

// OpenCodeAgentDefNew represents an agent in OpenCode config (correct schema)
type OpenCodeAgentDefNew struct {
	Model           string `json:"model,omitempty"`           // Format: provider-id/model-id
	MaxTokens       int64  `json:"maxTokens,omitempty"`       // Max output tokens
	ReasoningEffort string `json:"reasoningEffort,omitempty"` // low, medium, high
}

// OpenCodeMCPServerDefNew represents an MCP server in OpenCode config (correct schema)
// Based on actual working OpenCode config format:
// Local servers (stdio): command="npx", args=["-y", "package"]
// Remote SSE servers: type="sse", url="http://..."
type OpenCodeMCPServerDefNew struct {
	// Type: "local" for stdio/command MCP servers, "remote" for HTTP/SSE servers
	Type string `json:"type,omitempty"`

	// For local MCP servers (type: "local")
	Command []string `json:"command,omitempty"` // Command array e.g., ["npx", "-y", "mcp-server"]

	// For remote MCP servers (type: "remote")
	URL     string            `json:"url,omitempty"`     // Remote URL endpoint
	Headers map[string]string `json:"headers,omitempty"` // HTTP headers for remote

	// Environment variables (for local servers)
	Environment map[string]string `json:"environment,omitempty"` // Environment variables

	// Enable/disable toggle
	Enabled *bool `json:"enabled,omitempty"` // Set to false to disable
}

// OpenCodeConfigOld represents the OLD OpenCode configuration structure
// For opencode.json files (without leading dot) - uses legacy key names
// This format is validated by OpenCode's strict validator
type OpenCodeConfigOld struct {
	Schema     string                             `json:"$schema,omitempty"`
	Provider   map[string]OpenCodeProviderDefOld  `json:"provider,omitempty"`
	MCP        map[string]OpenCodeMCPServerDefOld `json:"mcp,omitempty"`
	Agent      map[string]OpenCodeAgentDefOld     `json:"agent,omitempty"`
	Tools      *OpenCodeToolsDefOld               `json:"tools,omitempty"`
	Permission *OpenCodePermissionDefOld          `json:"permission,omitempty"`
}

// OpenCodeProviderDefOld represents a provider in OLD OpenCode config
type OpenCodeProviderDefOld struct {
	Options *OpenCodeProviderOptionsOld `json:"options,omitempty"`
}

// OpenCodeProviderOptionsOld represents provider options in OLD OpenCode config
type OpenCodeProviderOptionsOld struct {
	BaseURL      string                `json:"baseURL,omitempty"`
	APIKeyEnvVar string                `json:"apiKeyEnvVar,omitempty"`
	Models       []OpenCodeModelDefOld `json:"models,omitempty"`
}

// OpenCodeModelDefOld represents a model in OLD OpenCode config
type OpenCodeModelDefOld struct {
	ID           string                        `json:"id"`
	Name         string                        `json:"name"`
	MaxTokens    int64                         `json:"maxTokens,omitempty"`
	Capabilities *OpenCodeModelCapabilitiesOld `json:"capabilities,omitempty"`
}

// OpenCodeModelCapabilitiesOld represents model capabilities
type OpenCodeModelCapabilitiesOld struct {
	Vision        bool `json:"vision,omitempty"`
	ImageInput    bool `json:"imageInput,omitempty"`
	ImageOutput   bool `json:"imageOutput,omitempty"`
	OCR           bool `json:"ocr,omitempty"`
	PDF           bool `json:"pdf,omitempty"`
	Streaming     bool `json:"streaming,omitempty"`
	FunctionCalls bool `json:"functionCalls,omitempty"`
	ToolUse       bool `json:"toolUse,omitempty"`
	Embeddings    bool `json:"embeddings,omitempty"`
	FileUpload    bool `json:"fileUpload,omitempty"`
	NoFileLimit   bool `json:"noFileLimit,omitempty"`
	MCP           bool `json:"mcp,omitempty"`
	ACP           bool `json:"acp,omitempty"`
	LSP           bool `json:"lsp,omitempty"`
}

// OpenCodeMCPServerDefOld represents an MCP server in OLD OpenCode config
type OpenCodeMCPServerDefOld struct {
	Type    string   `json:"type"`              // "local" or "remote"
	Command []string `json:"command,omitempty"` // For local type - array format
	URL     string   `json:"url,omitempty"`     // For remote type
}

// OpenCodeAgentDefOld represents an agent in OLD OpenCode config
type OpenCodeAgentDefOld struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt,omitempty"`
}

// OpenCodeToolsDefOld represents tools configuration
type OpenCodeToolsDefOld struct {
	Browser    bool `json:"browser,omitempty"`
	Embeddings bool `json:"embeddings,omitempty"`
	File       bool `json:"file,omitempty"`
	LSP        bool `json:"lsp,omitempty"`
	MCP        bool `json:"mcp,omitempty"`
	Search     bool `json:"search,omitempty"`
	Terminal   bool `json:"terminal,omitempty"`
	Vision     bool `json:"vision,omitempty"`
}

// OpenCodePermissionDefOld represents permissions
type OpenCodePermissionDefOld struct {
	AllowRead  bool `json:"allowRead,omitempty"`
	AllowWrite bool `json:"allowWrite,omitempty"`
	AllowExec  bool `json:"allowExec,omitempty"`
	AllowNet   bool `json:"allowNet,omitempty"`
}

// OpenCodeTUIDef represents TUI configuration
type OpenCodeTUIDef struct {
	Theme string `json:"theme,omitempty"` // opencode, catppuccin, dracula, etc.
}

// handleGenerateOpenCode handles the --generate-opencode-config command
// Generates OpenCode v1.1.30+ compatible configuration
// Config file should be saved as opencode.json (WITHOUT leading dot) in ~/.config/opencode/
// User must set LOCAL_ENDPOINT env var to HelixAgent URL (e.g., http://localhost:8100)
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

	// Get host and port for MCP SSE URLs
	host := os.Getenv("HELIXAGENT_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8100"
	}

	baseURL := fmt.Sprintf("http://%s:%s", host, port)

	// Get HelixLLM configuration from environment or .env file
	envVarsAll := loadEnvVars()
	helixLLMEndpoint := os.Getenv("HELIX_LLM_ENDPOINT")
	if helixLLMEndpoint == "" {
		if val, ok := envVarsAll["HELIX_LLM_ENDPOINT"]; ok && val != "" {
			helixLLMEndpoint = val
		} else {
			helixLLMEndpoint = "https://localhost:8443"
		}
	}
	helixLLMAPIKey := os.Getenv("HELIX_LLM_API_KEY")
	if helixLLMAPIKey == "" {
		if val, ok := envVarsAll["HELIX_LLM_API_KEY"]; ok && val != "" {
			helixLLMAPIKey = val
		}
	}

	var jsonData []byte
	var err error

	// Build OpenCode configuration.
	// Single "Helix Agent" provider with TWO models under it:
	// - helix-llm: Fast local inference via HelixLLM (provider chain with fallback)
	// - helix-debate: AI Debate Ensemble for best quality
	// Agents: task/title -> helix-llm (fast), coder/summarizer -> helix-debate (quality)
	config := OpenCodeConfig{
		Schema: "https://opencode.ai/config.json",
		Provider: map[string]OpenCodeProviderDefNew{
			"helixagent": {
				NPM:  "@ai-sdk/openai-compatible",
				Name: "Helix Agent",
				Options: &OpenCodeProviderOptionsNew{
					BaseURL: baseURL + "/v1",
					APIKey:  apiKey,
				},
				Models: map[string]OpenCodeModelDefNew{
					"helix-debate": {
						Name: "Helix AI Debate Ensemble",
						Limit: &OpenCodeModelLimit{
							Context: 128000,
							Output:  8192,
						},
					},
					"helix-llm": {
						Name: "Helix LLM",
						Limit: &OpenCodeModelLimit{
							Context: 128000,
							Output:  8192,
						},
					},
				},
			},
		},
		// Agent configuration - uses provider-id/model-id format
		// task/title → helix-llm (provider chain with fallback for fast local inference)
		// coder/summarizer → helix-debate (full debate ensemble for quality)
		Agent: map[string]OpenCodeAgentDefNew{
			"coder": {
				Model:     "helixagent/helix-debate",
				MaxTokens: 8192,
			},
			"summarizer": {
				Model:     "helixagent/helix-debate",
				MaxTokens: 4096,
			},
			"task": {
				Model:     "helixagent/helix-llm",
				MaxTokens: 4096,
			},
			"title": {
				Model:     "helixagent/helix-llm",
				MaxTokens: 80,
			},
		},
		MCP:    getMCPServersWithHelixLLM(baseURL, helixLLMEndpoint, *workingMCPsOnly),
		Plugin: []string{"opencode-agent-skills@0.6.5"},
		// NOTE: Instructions removed — CLAUDE.md (~15K tokens) exceeds local model's
		// 16K context when combined with conversation history and tools.
		// OpenCode reads CLAUDE.md automatically from project root when present.
		TUI: &OpenCodeTUIDef{Theme: "opencode"},
	}

	// Expand environment variables in entire configuration
	envVars := loadEnvVars()
	expandedConfig, err := expandEnvInOpenCodeConfig(config, envVars)
	if err != nil {
		return fmt.Errorf("failed to expand environment variables: %w", err)
	}
	config = expandedConfig
	jsonData, err = json.MarshalIndent(config, "", "  ")

	if err != nil {
		return fmt.Errorf("failed to marshal OpenCode config: %w", err)
	}

	// Output to file or stdout
	if appCfg.OpenCodeOutput != "" {
		// Validate path for traversal attacks (G304 security fix)
		// Note: This is a CLI-provided path from the admin user
		if !utils.ValidatePath(appCfg.OpenCodeOutput) {
			return fmt.Errorf("invalid output path: contains path traversal or dangerous characters")
		}
		// #nosec G304 - OpenCodeOutput is validated by utils.ValidatePath and provided via CLI by admin
		if err := os.WriteFile(appCfg.OpenCodeOutput, jsonData, 0644); err != nil {
			return fmt.Errorf("failed to write OpenCode config to file: %w", err)
		}
		logger.WithField("file", appCfg.OpenCodeOutput).Info("OpenCode configuration written to file")
		logger.Info("Generated OpenCode config for opencode.json")
		logger.Infof("IMPORTANT: Set LOCAL_ENDPOINT=%s before running opencode", baseURL)
	} else {
		fmt.Println(string(jsonData))
	}

	return nil
}

// buildOpenCodeMCPServersNew creates MCP server configurations using the correct OpenCode schema
// Based on: https://opencode.ai/docs/mcp-servers
// Local servers: type="local", command=["npx", "-y", "package"] - started on demand by OpenCode
// Remote servers: type="remote", url="https://..." - must be pre-running
// MCP servers are available from:
// - npm packages (local - started on demand via npx)
// - HelixAgent protocol endpoints (/v1/mcp, /v1/acp, /v1/lsp, etc. - remote)
// COMPLIANCE: 62+ MCPs required for system compliance

// getMCPServers returns MCP configurations based on flags
// If useContainerMCPs is true, uses containerized MCP servers with HTTP SSE endpoints (ZERO npx)
// If useLocalMCPServers is true, uses local Docker-based MCP servers on TCP ports
// If workingOnly is true, only MCPs with all dependencies met are returned
func getMCPServers(baseURL string, workingOnly bool) map[string]OpenCodeMCPServerDefNew {
	return getMCPServersWithHelixLLM(baseURL, "", workingOnly)
}

// getMCPServersWithHelixLLM returns MCP configurations including HelixLLM endpoints
func getMCPServersWithHelixLLM(baseURL string, helixLLMBaseURL string, workingOnly bool) map[string]OpenCodeMCPServerDefNew {
	if *useContainerMCPs {
		return buildContainerizedMCPs(baseURL)
	}
	if *useLocalMCPServers {
		return buildLocalDockerMCPServers(baseURL)
	}

	var mcps map[string]OpenCodeMCPServerDefNew
	if workingOnly {
		mcps = buildWorkingMCPsOnly(baseURL)
	} else {
		mcps = buildOpenCodeMCPServersNew(baseURL)
	}

	// NOTE: HelixLLM REST API endpoints (/v1/chat/completions, /v1/models, etc.)
	// are NOT MCP protocol servers — they don't speak JSON-RPC over SSE.
	// Adding them as remote MCPs causes OpenCode to hang on startup waiting
	// for an MCP handshake that never comes. HelixLLM is accessed via the
	// "helixllm" provider entry instead, not via MCP.

	return mcps
}

// buildContainerizedMCPs builds MCP configurations using HTTP SSE container endpoints
// ZERO npx commands - all MCPs use containerized remote endpoints
func buildContainerizedMCPs(baseURL string) map[string]OpenCodeMCPServerDefNew {
	generator := mcpconfig.NewContainerMCPConfigGenerator(baseURL)
	containerMCPs := generator.GenerateContainerMCPs()

	result := make(map[string]OpenCodeMCPServerDefNew)

	// Convert ContainerMCPServerConfig to OpenCodeMCPServerDefNew
	for name, cfg := range containerMCPs {
		// Only include enabled MCPs
		if !cfg.Enabled {
			continue
		}
		result[name] = OpenCodeMCPServerDefNew{
			Type: cfg.Type,
			URL:  cfg.URL,
		}
	}

	return result
}

// buildLocalDockerMCPServers builds MCP configurations for local Docker-based servers
// DEPRECATED: TCP protocol is NOT supported by OpenCode. Use npx-based MCPs instead.
// This function is kept for backward compatibility but should not be used with OpenCode.
// TCP servers run on ports 9101-9999 via socat - requires start-mcp-servers.sh
func buildLocalDockerMCPServers(baseURL string) map[string]OpenCodeMCPServerDefNew {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/home"
	}

	helixHome := os.Getenv("HELIXAGENT_HOME")
	if helixHome == "" {
		helixHome = homeDir + "/.helixagent"
	}

	logrus.Warn("TCP-based MCP servers are NOT supported by OpenCode. Use default npx-based MCPs instead.")

	return map[string]OpenCodeMCPServerDefNew{
		// HelixAgent local plugin and remote endpoints
		"helixagent": {
			Type:    "local",
			Command: []string{"node", helixHome + "/plugins/mcp-server/dist/index.js", "--endpoint", baseURL},
		},
		"helixagent-mcp": {
			Type: "remote",
			URL:  baseURL + "/v1/mcp",
		},
		"helixagent-acp": {
			Type: "remote",
			URL:  baseURL + "/v1/acp",
		},
		"helixagent-lsp": {
			Type: "remote",
			URL:  baseURL + "/v1/lsp",
		},
		"helixagent-embeddings": {
			Type: "remote",
			URL:  baseURL + "/v1/embeddings",
		},
		"helixagent-vision": {
			Type: "remote",
			URL:  baseURL + "/v1/vision",
		},
		"helixagent-cognee": {
			Type: "remote",
			URL:  baseURL + "/v1/cognee",
		},

		// NOTE: All TCP-based MCPs below will NOT work with OpenCode
		// They are kept for compatibility but will cause errors
		"fetch":               {Type: "remote", URL: "tcp://localhost:9101"},
		"git":                 {Type: "remote", URL: "tcp://localhost:9102"},
		"time":                {Type: "remote", URL: "tcp://localhost:9103"},
		"filesystem":          {Type: "remote", URL: "tcp://localhost:9104"},
		"memory":              {Type: "remote", URL: "tcp://localhost:9105"},
		"everything":          {Type: "remote", URL: "tcp://localhost:9106"},
		"sequential-thinking": {Type: "remote", URL: "tcp://localhost:9107"},
		"redis":               {Type: "remote", URL: "tcp://localhost:9201"},
		"mongodb":             {Type: "remote", URL: "tcp://localhost:9202"},
		"supabase":            {Type: "remote", URL: "tcp://localhost:9203"},
		"qdrant":              {Type: "remote", URL: "tcp://localhost:9301"},
		"kubernetes":          {Type: "remote", URL: "tcp://localhost:9401"},
		"github":              {Type: "remote", URL: "tcp://localhost:9402"},
		"cloudflare":          {Type: "remote", URL: "tcp://localhost:9403"},
		"heroku":              {Type: "remote", URL: "tcp://localhost:9404"},
		"sentry":              {Type: "remote", URL: "tcp://localhost:9405"},
		"playwright":          {Type: "remote", URL: "tcp://localhost:9501"},
		"browserbase":         {Type: "remote", URL: "tcp://localhost:9502"},
		"firecrawl":           {Type: "remote", URL: "tcp://localhost:9503"},
		"slack":               {Type: "remote", URL: "tcp://localhost:9601"},
		"telegram":            {Type: "remote", URL: "tcp://localhost:9602"},
		"notion":              {Type: "remote", URL: "tcp://localhost:9701"},
		"trello":              {Type: "remote", URL: "tcp://localhost:9702"},
		"airtable":            {Type: "remote", URL: "tcp://localhost:9703"},
		"obsidian":            {Type: "remote", URL: "tcp://localhost:9704"},
		"atlassian":           {Type: "remote", URL: "tcp://localhost:9705"},
		"brave-search":        {Type: "sse", URL: "tcp://localhost:9801"},
		"perplexity":          {Type: "sse", URL: "tcp://localhost:9802"},
		"omnisearch":          {Type: "sse", URL: "tcp://localhost:9803"},
		"context7":            {Type: "sse", URL: "tcp://localhost:9804"},
		"llamaindex":          {Type: "sse", URL: "tcp://localhost:9805"},
		"workers":             {Type: "sse", URL: "tcp://localhost:9901"},
	}
}

func buildOpenCodeMCPServersNew(baseURL string) map[string]OpenCodeMCPServerDefNew {
	return buildOpenCodeMCPServersFiltered(baseURL, false)
}

// buildWorkingMCPsOnly builds MCP configurations for only those MCPs that have
// all their dependencies met (API keys set, services running, etc.)
func buildWorkingMCPsOnly(baseURL string) map[string]OpenCodeMCPServerDefNew {
	return buildOpenCodeMCPServersFiltered(baseURL, true)
}

// buildOpenCodeMCPServersFiltered builds MCP configurations with optional filtering
// When filterWorking is true, only MCPs with all dependencies met are included
func buildOpenCodeMCPServersFiltered(baseURL string, filterWorking bool) map[string]OpenCodeMCPServerDefNew {
	// Get user home directory for paths
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/home"
	}

	// Get HELIXAGENT_HOME for local MCP plugin path
	helixHome := os.Getenv("HELIXAGENT_HOME")
	if helixHome == "" {
		helixHome = homeDir + "/.helixagent"
	}

	allMCPs := map[string]OpenCodeMCPServerDefNew{
		// =============================================================================
		// HelixAgent Protocol Endpoints (7 MCPs) - LOCAL + REMOTE
		// =============================================================================
		// Local MCP plugin - runs the HelixAgent MCP server as a subprocess
		"helixagent": {
			Type:    "local",
			Command: []string{"node", helixHome + "/plugins/mcp-server/dist/index.js", "--endpoint", baseURL},
		},
		// Remote protocol endpoints (running at port 8100)
		"helixagent-mcp": {
			Type: "remote",
			URL:  baseURL + "/v1/mcp",
		},
		"helixagent-acp": {
			Type: "remote",
			URL:  baseURL + "/v1/acp",
		},
		"helixagent-lsp": {
			Type: "remote",
			URL:  baseURL + "/v1/lsp",
		},
		"helixagent-embeddings": {
			Type: "remote",
			URL:  baseURL + "/v1/embeddings",
		},
		"helixagent-vision": {
			Type: "remote",
			URL:  baseURL + "/v1/vision",
		},
		"helixagent-cognee": {
			Type: "remote",
			URL:  baseURL + "/v1/cognee",
		},
		// HelixAgent Extended Services (3 MCPs) - REMOTE
		"helixagent-rag": {
			Type: "remote",
			URL:  baseURL + "/v1/rag",
		},
		"helixagent-formatters": {
			Type: "remote",
			URL:  baseURL + "/v1/formatters",
		},
		"helixagent-monitoring": {
			Type: "remote",
			URL:  baseURL + "/v1/monitoring",
		},

		// =============================================================================
		// Free Remote MCP Servers (3) - No authentication required
		// =============================================================================
		"context7": {
			Type: "remote",
			URL:  "https://mcp.context7.com/mcp",
		},
		"deepwiki": {
			Type: "remote",
			URL:  "https://mcp.deepwiki.com/mcp",
		},
		"cloudflare-docs": {
			Type: "remote",
			URL:  "https://docs.mcp.cloudflare.com/sse",
		},

		// =============================================================================
		// Anthropic Official MCPs - LOCAL (started on demand via npx)
		// From: https://github.com/modelcontextprotocol/servers
		// =============================================================================
		"filesystem": {
			Type:    "local",
			Command: []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", homeDir},
		},
		"fetch": {
			Type:    "local",
			Command: []string{"npx", "-y", "mcp-fetch-server"},
		},
		"memory": {
			Type:    "local",
			Command: []string{"npx", "-y", "@modelcontextprotocol/server-memory"},
		},
		"time": {
			Type:    "local",
			Command: []string{"npx", "-y", "@theo.foobar/mcp-time"},
		},
		"git": {
			Type:    "local",
			Command: []string{"npx", "-y", "mcp-git"},
		},
		"sqlite": {
			Type:    "local",
			Command: []string{"npx", "-y", "mcp-server-sqlite-npx", "/tmp/helixagent.db"},
		},
		"postgres": {
			Type:    "local",
			Command: []string{"npx", "-y", "@modelcontextprotocol/server-postgres", "postgresql://helixagent:helixagent123@localhost:15432/helixagent_db"},
		},
		"puppeteer": {
			Type:    "local",
			Command: []string{"npx", "-y", "@modelcontextprotocol/server-puppeteer"},
		},
		"sequential-thinking": {
			Type:    "local",
			Command: []string{"npx", "-y", "@modelcontextprotocol/server-sequential-thinking"},
		},
		"everything": {
			Type:    "local",
			Command: []string{"npx", "-y", "@modelcontextprotocol/server-everything"},
		},
		"brave-search": {
			Type:        "local",
			Command:     []string{"npx", "-y", "@modelcontextprotocol/server-brave-search"},
			Environment: map[string]string{"BRAVE_API_KEY": "{env:BRAVE_API_KEY}"},
		},
		"google-maps": {
			Type:        "local",
			Command:     []string{"npx", "-y", "@modelcontextprotocol/server-google-maps"},
			Environment: map[string]string{"GOOGLE_MAPS_API_KEY": "{env:GOOGLE_MAPS_API_KEY}"},
		},
		"slack": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "@modelcontextprotocol/server-slack"},
			Environment: map[string]string{"SLACK_BOT_TOKEN": "{env:SLACK_BOT_TOKEN}", "SLACK_TEAM_ID": "{env:SLACK_TEAM_ID}"},
		},
		"github": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "@modelcontextprotocol/server-github"},
			Environment: map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": "{env:GITHUB_TOKEN}"},
		},
		// NOTE: @modelcontextprotocol/server-gitlab has broken tool schemas upstream
		// (all inputSchema are empty: {"$schema":"http://json-schema.org/draft-07/schema#"})
		// in ALL versions (0.5.1, 0.6.2, 2025.4.25). OpenCode rejects these with
		// "Failed to get tools". Disabled until upstream fixes the schemas.
		// GitLab API access is available via GitLab token in environment.
		"sentry": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "@modelcontextprotocol/server-sentry"},
			Environment: map[string]string{"SENTRY_AUTH_TOKEN": "{env:SENTRY_AUTH_TOKEN}", "SENTRY_ORG": "{env:SENTRY_ORG}"},
		},
		"everart": {
			Type:        "local",
			Command:     []string{"npx", "-y", "@modelcontextprotocol/server-everart"},
			Environment: map[string]string{"EVERART_API_KEY": "{env:EVERART_API_KEY}"},
		},
		"aws-kb-retrieval": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "@modelcontextprotocol/server-aws-kb-retrieval"},
			Environment: map[string]string{"AWS_ACCESS_KEY_ID": "{env:AWS_ACCESS_KEY_ID}", "AWS_SECRET_ACCESS_KEY": "{env:AWS_SECRET_ACCESS_KEY}"},
		},

		// =============================================================================
		// Additional Anthropic & Community MCPs - LOCAL
		// =============================================================================
		"exa": {
			Type:        "local",
			Command:     []string{"npx", "-y", "exa-mcp-server"},
			Environment: map[string]string{"EXA_API_KEY": "{env:EXA_API_KEY}"},
		},
		"linear": {
			Type:        "local",
			Command:     []string{"npx", "-y", "@modelcontextprotocol/server-linear"},
			Environment: map[string]string{"LINEAR_API_KEY": "{env:LINEAR_API_KEY}"},
		},
		"notion": {
			Type:        "local",
			Command:     []string{"npx", "-y", "@notionhq/notion-mcp-server"},
			Environment: map[string]string{"NOTION_API_KEY": "{env:NOTION_API_KEY}"},
		},
		"figma": {
			Type:        "local",
			Command:     []string{"npx", "-y", "figma-developer-mcp"},
			Environment: map[string]string{"FIGMA_API_KEY": "{env:FIGMA_API_KEY}"},
		},
		"todoist": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "@modelcontextprotocol/server-todoist"},
			Environment: map[string]string{"TODOIST_API_TOKEN": "{env:TODOIST_API_TOKEN}"},
		},
		"obsidian": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-obsidian"},
			Environment: map[string]string{"OBSIDIAN_VAULT_PATH": "{env:OBSIDIAN_VAULT_PATH}"},
		},
		"raycast": {
			Type:    "local",
			Command: []string{"npx", "-y", "@raycast/mcp-server-raycast"},
		},
		"tinybird": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-tinybird"},
			Environment: map[string]string{"TINYBIRD_TOKEN": "{env:TINYBIRD_TOKEN}"},
		},
		"cloudflare": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "@cloudflare/mcp-server-cloudflare"},
			Environment: map[string]string{"CLOUDFLARE_API_TOKEN": "{env:CLOUDFLARE_API_TOKEN}"},
		},
		"neon": {
			Type:        "local",
			Command:     []string{"npx", "-y", "@neondatabase/mcp-server-neon"},
			Environment: map[string]string{"NEON_API_KEY": "{env:NEON_API_KEY}"},
		},
		"gdrive": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "@anthropic/mcp-server-gdrive"},
			Environment: map[string]string{"GOOGLE_CREDENTIALS_PATH": "{env:GOOGLE_CREDENTIALS_PATH}"},
		},

		// =============================================================================
		// Container/Infrastructure MCPs - LOCAL
		// =============================================================================
		"docker": {
			Type:    "local",
			Command: []string{"npx", "-y", "mcp-server-docker"},
		},
		"kubernetes": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-kubernetes"},
			Environment: map[string]string{"KUBECONFIG": "{env:KUBECONFIG}"},
		},
		"redis": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-redis"},
			Environment: map[string]string{"REDIS_URL": "redis://localhost:6379"},
		},
		"mongodb": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-mongodb"},
			Environment: map[string]string{"MONGODB_URI": "mongodb://localhost:27017"},
		},
		"elasticsearch": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-elasticsearch"},
			Environment: map[string]string{"ELASTICSEARCH_URL": "http://localhost:9200"},
		},
		"qdrant": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-qdrant"},
			Environment: map[string]string{"QDRANT_URL": "http://localhost:6333"},
		},
		"chroma": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-chroma"},
			Environment: map[string]string{"CHROMA_URL": "http://localhost:8001"},
		},
		"pinecone": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-pinecone"},
			Environment: map[string]string{"PINECONE_API_KEY": "{env:PINECONE_API_KEY}"},
		},
		"milvus": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-milvus"},
			Environment: map[string]string{"MILVUS_HOST": "localhost", "MILVUS_PORT": "19530"},
		},
		"weaviate": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-weaviate"},
			Environment: map[string]string{"WEAVIATE_URL": "http://localhost:8080"},
		},
		"supabase": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-supabase"},
			Environment: map[string]string{"SUPABASE_URL": "{env:SUPABASE_URL}", "SUPABASE_KEY": "{env:SUPABASE_KEY}"},
		},

		// =============================================================================
		// Productivity/Collaboration MCPs - LOCAL
		// =============================================================================
		"jira": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-atlassian"},
			Environment: map[string]string{"JIRA_URL": "{env:JIRA_URL}", "JIRA_EMAIL": "{env:JIRA_EMAIL}", "JIRA_API_TOKEN": "{env:JIRA_API_TOKEN}"},
		},
		"asana": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-asana"},
			Environment: map[string]string{"ASANA_ACCESS_TOKEN": "{env:ASANA_ACCESS_TOKEN}"},
		},
		"trello": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-trello"},
			Environment: map[string]string{"TRELLO_API_KEY": "{env:TRELLO_API_KEY}", "TRELLO_TOKEN": "{env:TRELLO_TOKEN}"},
		},
		"monday": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-monday"},
			Environment: map[string]string{"MONDAY_API_KEY": "{env:MONDAY_API_KEY}"},
		},
		"clickup": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-clickup"},
			Environment: map[string]string{"CLICKUP_API_KEY": "{env:CLICKUP_API_KEY}"},
		},
		"discord": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-discord"},
			Environment: map[string]string{"DISCORD_BOT_TOKEN": "{env:DISCORD_BOT_TOKEN}"},
		},
		"microsoft-teams": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-teams"},
			Environment: map[string]string{"TEAMS_CLIENT_ID": "{env:TEAMS_CLIENT_ID}", "TEAMS_CLIENT_SECRET": "{env:TEAMS_CLIENT_SECRET}"},
		},
		"gmail": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-gmail"},
			Environment: map[string]string{"GOOGLE_CREDENTIALS_PATH": "{env:GOOGLE_CREDENTIALS_PATH}"},
		},
		"calendar": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-google-calendar"},
			Environment: map[string]string{"GOOGLE_CREDENTIALS_PATH": "{env:GOOGLE_CREDENTIALS_PATH}"},
		},
		"zoom": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-zoom"},
			Environment: map[string]string{"ZOOM_CLIENT_ID": "{env:ZOOM_CLIENT_ID}", "ZOOM_CLIENT_SECRET": "{env:ZOOM_CLIENT_SECRET}"},
		},

		// =============================================================================
		// Cloud/DevOps MCPs - LOCAL
		// =============================================================================
		"aws-s3": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-s3"},
			Environment: map[string]string{"AWS_ACCESS_KEY_ID": "{env:AWS_ACCESS_KEY_ID}", "AWS_SECRET_ACCESS_KEY": "{env:AWS_SECRET_ACCESS_KEY}"},
		},
		"aws-lambda": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-aws-lambda"},
			Environment: map[string]string{"AWS_ACCESS_KEY_ID": "{env:AWS_ACCESS_KEY_ID}", "AWS_SECRET_ACCESS_KEY": "{env:AWS_SECRET_ACCESS_KEY}"},
		},
		"azure": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-azure"},
			Environment: map[string]string{"AZURE_SUBSCRIPTION_ID": "{env:AZURE_SUBSCRIPTION_ID}", "AZURE_TENANT_ID": "{env:AZURE_TENANT_ID}"},
		},
		"gcp": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-gcp"},
			Environment: map[string]string{"GOOGLE_APPLICATION_CREDENTIALS": "{env:GOOGLE_APPLICATION_CREDENTIALS}"},
		},
		"terraform": {
			Type:    "local",
			Command: []string{"npx", "-y", "mcp-server-terraform"},
		},
		"ansible": {
			Type:    "local",
			Command: []string{"npx", "-y", "mcp-server-ansible"},
		},
		"datadog": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-datadog"},
			Environment: map[string]string{"DD_API_KEY": "{env:DD_API_KEY}", "DD_APP_KEY": "{env:DD_APP_KEY}"},
		},
		"grafana": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-grafana"},
			Environment: map[string]string{"GRAFANA_URL": "{env:GRAFANA_URL}", "GRAFANA_API_KEY": "{env:GRAFANA_API_KEY}"},
		},
		"prometheus": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-prometheus"},
			Environment: map[string]string{"PROMETHEUS_URL": "{env:PROMETHEUS_URL}"},
		},
		"circleci": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-circleci"},
			Environment: map[string]string{"CIRCLECI_TOKEN": "{env:CIRCLECI_TOKEN}"},
		},

		// =============================================================================
		// AI/ML Integration MCPs - LOCAL
		// =============================================================================
		"langchain": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-langchain"},
			Environment: map[string]string{"OPENAI_API_KEY": "{env:OPENAI_API_KEY}"},
		},
		"llamaindex": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-llamaindex"},
			Environment: map[string]string{"OPENAI_API_KEY": "{env:OPENAI_API_KEY}"},
		},
		// NOTE: No official huggingface MCP server npm package exists.
		// "mcp-server-huggingface" is NOT on npm. HuggingFace access is via
		// the huggingface provider or @huggingface/mcp-client (client, not server).
		"replicate": { // #nosec G101 -- not a credential (map key / config label / env-var reference)
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-replicate"},
			Environment: map[string]string{"REPLICATE_API_TOKEN": "{env:REPLICATE_API_TOKEN}"},
		},
		"stable-diffusion": {
			Type:        "local",
			Command:     []string{"npx", "-y", "mcp-server-stable-diffusion"},
			Environment: map[string]string{"STABILITY_API_KEY": "{env:STABILITY_API_KEY}"},
		},

		// =============================================================================
		// Utility MCPs - LOCAL (no API keys required, all verified working)
		// =============================================================================
		"shell": {
			Type:    "local",
			Command: []string{"npx", "-y", "mcp-shell-server"},
		},
		"markdown": {
			Type:    "local",
			Command: []string{"npx", "-y", "mcp-server-markdown"},
		},
		"tavily": {
			Type:        "local",
			Command:     []string{"npx", "-y", "tavily-mcp"},
			Environment: map[string]string{"TAVILY_API_KEY": "{env:TAVILY_API_KEY}"},
		},
		"commands": {
			Type:    "local",
			Command: []string{"npx", "-y", "mcp-server-commands"},
		},
	}

	// If not filtering, return all MCPs
	if !filterWorking {
		return allMCPs
	}

	// Filter to only working MCPs (those with all dependencies met)
	return filterWorkingMCPs(allMCPs)
}

// filterWorkingMCPs filters MCP configurations to include MCPs that have
// all their dependencies met (API keys set, services available).
// NOTE: HelixLLM REST endpoints (/v1/chat/completions, /v1/models, etc.)
// are NOT MCP protocol servers and must NEVER be added as remote MCPs —
// they don't speak JSON-RPC over SSE and cause OpenCode to hang.
func filterWorkingMCPs(allMCPs map[string]OpenCodeMCPServerDefNew) map[string]OpenCodeMCPServerDefNew {
	workingMCPs := make(map[string]OpenCodeMCPServerDefNew)

	// MCPs that always work — no external dependencies beyond npx/Node.js
	// NOTE: helixagent-* remote MCPs are EXCLUDED — tested and confirmed
	// 8/9 endpoints timeout (only /v1/formatters responds). They are not
	// functional MCP protocol servers and cause OpenCode to hang on startup.
	alwaysWorking := map[string]bool{
		// Free remote MCP servers — always available, no auth required
		"context7":        true,
		"deepwiki":        true,
		"cloudflare-docs": true,
		// Core local MCPs — started on demand by OpenCode via npx, never hang
		"filesystem":          true,
		"fetch":               true,
		"memory":              true,
		"time":                true,
		"git":                 true,
		"sqlite":              true,
		"puppeteer":           true,
		"sequential-thinking": true,
		"everything":          true,
		"postgres":            true,
		// Additional verified MCPs — all tested with MCP protocol handshake + valid schemas
		"docker":   true, // mcp-server-docker: 1 tool, valid schemas
		"notion":   true, // @notionhq/notion-mcp-server: 22 tools, valid schemas
		"shell":    true, // mcp-shell-server: 1 tool, valid schemas
		"markdown": true, // mcp-server-markdown: 6 tools, valid schemas
		"tavily":   true, // tavily-mcp: 5 tools, valid schemas (search/extract/crawl/map/research)
		"commands": true, // mcp-server-commands: 1 tool, valid schemas (run_process)
	}

	// MCPs that require specific env vars — only include if the key is set
	envDependent := map[string]string{
		"github": "GITHUB_TOKEN",
		// "gitlab" removed — upstream @modelcontextprotocol/server-gitlab has broken tool schemas
		"brave-search": "BRAVE_API_KEY",
		"slack":        "SLACK_BOT_TOKEN",
		"sentry":       "SENTRY_AUTH_TOKEN",
		"linear":       "LINEAR_API_KEY",
		"notion":       "NOTION_API_KEY",
		// NOTE: huggingface MCP removed — npm package "mcp-server-huggingface" does not exist
		"replicate": "REPLICATE_API_TOKEN",
		"exa":       "EXA_API_KEY",
	}

	for name, mcpConfig := range allMCPs {
		if alwaysWorking[name] {
			workingMCPs[name] = mcpConfig
			continue
		}
		// Check env-dependent MCPs
		if envVar, ok := envDependent[name]; ok {
			if os.Getenv(envVar) != "" {
				workingMCPs[name] = mcpConfig
			}
		}
	}

	return workingMCPs
}

// buildOpenCodeMCPServersOld builds MCP servers in OLD format for opencode.json
// OLD format uses "type": "local"/"remote" with "command" as array
func buildOpenCodeMCPServersOld(baseURL string) map[string]OpenCodeMCPServerDefOld {
	return map[string]OpenCodeMCPServerDefOld{
		// Anthropic Official MCPs
		"filesystem":          {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", "/home"}},
		"fetch":               {Type: "local", Command: []string{"npx", "-y", "mcp-fetch-server"}},
		"memory":              {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-memory"}},
		"time":                {Type: "local", Command: []string{"npx", "-y", "@theo.foobar/mcp-time"}},
		"git":                 {Type: "local", Command: []string{"npx", "-y", "mcp-git"}},
		"sqlite":              {Type: "local", Command: []string{"npx", "-y", "mcp-server-sqlite-npx", "/tmp/helixagent.db"}},
		"postgres":            {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-postgres", "postgresql://localhost:8101/helixagent"}},
		"puppeteer":           {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-puppeteer"}},
		"brave-search":        {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-brave-search"}},
		"google-maps":         {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-google-maps"}},
		"slack":               {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-slack"}},
		"github":              {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-github"}},
		"gitlab":              {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-gitlab"}},
		"sequential-thinking": {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-sequential-thinking"}},
		"everart":             {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-everart"}},
		"exa":                 {Type: "local", Command: []string{"npx", "-y", "exa-mcp-server"}},
		"linear":              {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-linear"}},
		"sentry":              {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-sentry"}},
		"notion":              {Type: "local", Command: []string{"npx", "-y", "@notionhq/notion-mcp-server"}},
		"figma":               {Type: "local", Command: []string{"npx", "-y", "figma-developer-mcp"}},
		"aws-kb-retrieval":    {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-aws-kb-retrieval"}},
		// HelixAgent Remote MCP - endpoint is /v1/mcp
		"helixagent": {Type: "remote", URL: baseURL + "/v1/mcp"}, // Note: OLD format doesn't support headers
		// Community/Infrastructure MCPs
		"docker":        {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-docker"}},
		"kubernetes":    {Type: "local", Command: []string{"npx", "-y", "mcp-server-kubernetes"}},
		"redis":         {Type: "local", Command: []string{"npx", "-y", "mcp-server-redis"}},
		"mongodb":       {Type: "local", Command: []string{"npx", "-y", "mcp-server-mongodb"}},
		"elasticsearch": {Type: "local", Command: []string{"npx", "-y", "mcp-server-elasticsearch"}},
		"qdrant":        {Type: "local", Command: []string{"npx", "-y", "mcp-server-qdrant"}},
		"chroma":        {Type: "local", Command: []string{"npx", "-y", "mcp-server-chroma"}},
		// Productivity MCPs
		"jira":         {Type: "local", Command: []string{"npx", "-y", "mcp-server-atlassian"}},
		"asana":        {Type: "local", Command: []string{"npx", "-y", "mcp-server-asana"}},
		"google-drive": {Type: "local", Command: []string{"npx", "-y", "@anthropic/mcp-server-gdrive"}},
		"aws-s3":       {Type: "local", Command: []string{"npx", "-y", "mcp-server-s3"}},
		"datadog":      {Type: "local", Command: []string{"npx", "-y", "mcp-server-datadog"}},
	}
}

// handlePreinstallMCP handles the --preinstall-mcp command
// Pre-installs all standard MCP server npm packages for faster startup
func handlePreinstallMCP(appCfg *AppConfig) error {
	logger := appCfg.Logger
	if logger == nil {
		logger = logrus.New()
	}

	logger.Info("Starting MCP package pre-installation...")

	// Get home directory for install location
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		return fmt.Errorf("HOME environment variable not set")
	}

	// Create preinstaller
	preinstaller, err := mcp.NewPreinstaller(mcp.PreinstallerConfig{
		InstallDir:  fmt.Sprintf("%s/.helixagent/mcp-servers", homeDir),
		Logger:      logger,
		Concurrency: 4,
		Timeout:     5 * time.Minute,
		OnProgress: func(pkg string, status mcp.InstallStatus, progress float64) {
			logger.WithFields(logrus.Fields{
				"package":  pkg,
				"status":   status,
				"progress": fmt.Sprintf("%.0f%%", progress*100),
			}).Info("MCP package installation progress")
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create MCP preinstaller: %w", err)
	}

	// Check if Node.js is available
	if !preinstaller.IsNodeAvailable() {
		return fmt.Errorf("Node.js is not available - MCP packages cannot be installed")
	}

	// Run pre-installation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := preinstaller.PreInstallAll(ctx); err != nil {
		return fmt.Errorf("MCP pre-installation failed: %w", err)
	}

	// Print summary
	statuses := preinstaller.GetAllStatuses()
	installed := 0
	failed := 0
	for _, status := range statuses {
		if status.Status == mcp.StatusInstalled {
			installed++
			logger.WithFields(logrus.Fields{
				"package":  status.Package.Name,
				"path":     status.InstallPath,
				"duration": status.Duration,
			}).Info("Package installed")
		} else if status.Status == mcp.StatusFailed {
			failed++
			logger.WithError(status.Error).WithField("package", status.Package.Name).Error("Package failed")
		}
	}

	logger.WithFields(logrus.Fields{
		"installed": installed,
		"failed":    failed,
		"total":     len(statuses),
	}).Info("MCP pre-installation complete")

	if failed > 0 {
		return fmt.Errorf("%d packages failed to install", failed)
	}

	return nil
}

// startBackgroundMCPPreinstall starts MCP package pre-installation in background
// This is called at server startup unless --skip-mcp-preinstall is specified
func startBackgroundMCPPreinstall(logger *logrus.Logger) {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		logger.Warn("HOME not set, skipping background MCP pre-installation")
		return
	}

	preinstaller, err := mcp.NewPreinstaller(mcp.PreinstallerConfig{
		InstallDir:  fmt.Sprintf("%s/.helixagent/mcp-servers", homeDir),
		Logger:      logger,
		Concurrency: 2, // Lower concurrency for background
		Timeout:     10 * time.Minute,
	})
	if err != nil {
		logger.WithError(err).Warn("Failed to create background MCP preinstaller")
		return
	}

	if !preinstaller.IsNodeAvailable() {
		logger.Debug("Node.js not available, skipping background MCP pre-installation")
		return
	}

	go func() {
		logger.Info("Starting background MCP package pre-installation...")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		if err := preinstaller.PreInstallAll(ctx); err != nil {
			logger.WithError(err).Warn("Background MCP pre-installation had errors")
		} else {
			logger.Info("Background MCP pre-installation completed successfully")
		}
	}()
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
	Providers  int `json:"providers"`
	MCPServers int `json:"mcp_servers"`
	Agents     int `json:"agents"`
	Commands   int `json:"commands"`
}

// handleValidateOpenCode handles the --validate-opencode-config command
func handleValidateOpenCode(appCfg *AppConfig) error {
	logger := appCfg.Logger
	if logger == nil {
		logger = logrus.New()
	}

	filePath := appCfg.ValidateOpenCode

	// Validate path for traversal attacks (G304 security fix)
	// Note: This is a CLI-provided path from the admin user
	if !utils.ValidatePath(filePath) {
		return fmt.Errorf("invalid config file path: contains path traversal or dangerous characters")
	}

	// Read the config file
	// #nosec G304 - filePath is validated by utils.ValidatePath and provided via CLI by admin
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
			Message: fmt.Sprintf("invalid top-level keys: %v (valid keys: $schema, plugin, enterprise, instructions, provider/providers, mcp/mcpServers, tools, agent/agents, command, keybinds, username, share, permission, compaction, sse, mode, autoshare, contextPaths, tui)", invalidKeys),
		})
	}

	// Detect schema version: v1.1.30+ uses "providers" (plural), v1.0.x uses "provider" (singular)
	isV1130Plus := rawConfig["providers"] != nil || rawConfig["mcpServers"] != nil || rawConfig["agents"] != nil

	// Parse and validate providers (both v1.0.x and v1.1.30+ schemas)
	if isV1130Plus {
		// v1.1.30+ schema: providers (plural)
		if providers, ok := rawConfig["providers"].(map[string]interface{}); ok {
			result.Stats.Providers = len(providers)
			// v1.1.30+ schema: each provider can have apiKey and disabled
			for name, providerData := range providers {
				if provider, ok := providerData.(map[string]interface{}); ok {
					// Provider is valid if it has apiKey (can be empty for local provider)
					_, _ = provider["apiKey"], name // Allow any apiKey value
				}
			}
		} else if rawConfig["providers"] == nil {
			result.Valid = false
			result.Errors = append(result.Errors, OpenCodeValidationError{
				Field:   "providers",
				Message: "at least one provider must be configured",
			})
		}
	} else {
		// v1.0.x schema: provider (singular)
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
	}

	// Parse and validate MCP servers (both v1.0.x and v1.1.30+ schemas)
	if isV1130Plus {
		// v1.1.30+ schema: mcpServers (plural)
		if mcpServers, ok := rawConfig["mcpServers"].(map[string]interface{}); ok {
			result.Stats.MCPServers = len(mcpServers)
			for name, serverData := range mcpServers {
				if server, ok := serverData.(map[string]interface{}); ok {
					// In v1.1.30+ schema, type is "sse" for remote, or command/args for stdio
					serverType, hasType := server["type"].(string)
					_, hasCommand := server["command"]
					_, hasURL := server["url"]

					// If type is "sse", url is required
					if hasType && serverType == "sse" {
						if !hasURL {
							result.Valid = false
							result.Errors = append(result.Errors, OpenCodeValidationError{
								Field:   fmt.Sprintf("mcpServers.%s.url", name),
								Message: "url is required for SSE MCP servers",
							})
						}
					} else if !hasCommand && !hasURL {
						// For stdio servers (no type or type != sse), command is required
						result.Valid = false
						result.Errors = append(result.Errors, OpenCodeValidationError{
							Field:   fmt.Sprintf("mcpServers.%s.command", name),
							Message: "command is required for stdio MCP servers",
						})
					}
				}
			}
		}
	} else {
		// v1.0.x schema: mcp (singular)
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
	}

	// Parse and validate agents (both v1.0.x and v1.1.30+ schemas)
	if isV1130Plus {
		// v1.1.30+ schema: agents (plural)
		if agents, ok := rawConfig["agents"].(map[string]interface{}); ok {
			result.Stats.Agents = len(agents)
			for name, agentData := range agents {
				if agent, ok := agentData.(map[string]interface{}); ok {
					// In v1.1.30+ schema, agents need model
					if _, hasModel := agent["model"]; !hasModel {
						result.Valid = false
						result.Errors = append(result.Errors, OpenCodeValidationError{
							Field:   fmt.Sprintf("agents.%s", name),
							Message: "agent must have model configured",
						})
					}
				}
			}
		}
	} else if agents, ok := rawConfig["agent"].(map[string]interface{}); ok {
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

// CrushConfig represents the Crush CLI configuration structure
type CrushConfig struct {
	Schema     string                          `json:"$schema,omitempty"`
	Providers  map[string]CrushProvider        `json:"providers,omitempty"`
	Lsp        map[string]CrushLspConfig       `json:"lsp,omitempty"`
	Mcp        map[string]CrushMcpConfig       `json:"mcp,omitempty"`
	Plugins    []string                        `json:"plugins,omitempty"`
	Extensions *cliagents.HelixAgentExtensions `json:"extensions,omitempty"`
	Formatters cliagents.FormattersConfig      `json:"formatters,omitempty"`
	Options    *CrushOptions                   `json:"options,omitempty"`
}

// CrushMcpConfig represents MCP server configuration for Crush
type CrushMcpConfig struct {
	Type    string            `json:"type"`
	URL     string            `json:"url,omitempty"`
	Command []string          `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Enabled bool              `json:"enabled"`
}

// CrushProvider represents a provider configuration for Crush
type CrushProvider struct {
	Name    string       `json:"name"`
	Type    string       `json:"type"`
	BaseURL string       `json:"base_url"`
	APIKey  string       `json:"api_key,omitempty"`
	Models  []CrushModel `json:"models"`
}

// CrushModel represents a model configuration for Crush
type CrushModel struct {
	ID                  string                 `json:"id"`
	Name                string                 `json:"name"`
	CostPer1MIn         float64                `json:"cost_per_1m_in"`
	CostPer1MOut        float64                `json:"cost_per_1m_out"`
	CostPer1MInCached   float64                `json:"cost_per_1m_in_cached,omitempty"`
	CostPer1MOutCached  float64                `json:"cost_per_1m_out_cached,omitempty"`
	ContextWindow       int                    `json:"context_window"`
	DefaultMaxTokens    int                    `json:"default_max_tokens"`
	CanReason           bool                   `json:"can_reason"`
	SupportsAttachments bool                   `json:"supports_attachments"`
	Streaming           bool                   `json:"streaming"`
	SupportsBrotli      bool                   `json:"supports_brotli,omitempty"`
	Options             map[string]interface{} `json:"options,omitempty"`
}

// CrushLspConfig represents Language Server Protocol configuration for Crush
type CrushLspConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Enabled bool     `json:"enabled"`
}

// CrushOptions represents global configuration options for Crush
type CrushOptions struct {
	DisableProviderAutoUpdate bool `json:"disable_provider_auto_update,omitempty"`
}

// handleGenerateCrush handles the --generate-crush-config command
func handleGenerateCrush(appCfg *AppConfig) error {
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
		port = "8100"
	}

	baseURL := fmt.Sprintf("http://%s:%s/v1", host, port)
	crushPortInt, err := strconv.Atoi(port)
	if err != nil {
		crushPortInt = 8100 // default port (HELIXAGENT_PORT_HTTP)
	}

	// Build the Crush configuration
	// Crush uses a different structure than OpenCode - providers with models array
	// COMPLIANCE: 62+ MCPs required for all CLI agents
	config := CrushConfig{
		Schema: "https://charm.land/crush.json",
		Providers: map[string]CrushProvider{
			"helixagent": {
				Name:    "HelixAgent AI Debate Ensemble",
				Type:    "openai",
				BaseURL: baseURL,
				APIKey:  apiKey,
				Models: []CrushModel{
					{
						ID:                  "helixagent-debate",
						Name:                "HelixAgent Debate Ensemble",
						CostPer1MIn:         0.0, // Local deployment, no cost
						CostPer1MOut:        0.0,
						CostPer1MInCached:   0.0,
						CostPer1MOutCached:  0.0,
						ContextWindow:       128000,
						DefaultMaxTokens:    8192,
						CanReason:           true,
						SupportsAttachments: true,
						Streaming:           true,
						SupportsBrotli:      true,
						Options: map[string]interface{}{
							"vision":         true,
							"image_input":    true,
							"image_output":   true,
							"ocr":            true,
							"pdf":            true,
							"function_calls": true,
							"tool_use":       true,
							"embeddings":     true,
						},
					},
				},
			},
		},
		Lsp: map[string]CrushLspConfig{
			"helixagent-lsp": {
				Command: fmt.Sprintf("curl -X POST %s/lsp", baseURL),
				Args:    []string{"-H", "Authorization: Bearer " + apiKey},
				Enabled: true,
			},
		},
		Mcp:        getCrushMCPServers(fmt.Sprintf("http://%s:%s", host, port), *workingMCPsOnly),
		Plugins:    cliagents.DefaultPlugins(),
		Extensions: cliagents.DefaultHelixAgentExtensions(host, crushPortInt),
		Formatters: cliagents.DefaultFormattersConfig(host, crushPortInt),
		Options: &CrushOptions{
			DisableProviderAutoUpdate: false,
		},
	}

	// Marshal to JSON with indentation
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Crush config: %w", err)
	}

	// Output to file or stdout
	if appCfg.CrushOutput != "" {
		// Validate path for traversal attacks (G304 security fix)
		if !utils.ValidatePath(appCfg.CrushOutput) {
			return fmt.Errorf("invalid output path: contains path traversal or dangerous characters")
		}
		// #nosec G304 - CrushOutput is validated by utils.ValidatePath and provided via CLI by admin
		if err := os.WriteFile(appCfg.CrushOutput, jsonData, 0644); err != nil {
			return fmt.Errorf("failed to write Crush config to file: %w", err)
		}
		logger.WithField("file", appCfg.CrushOutput).Info("Crush configuration written to file")
	} else {
		fmt.Println(string(jsonData))
	}

	return nil
}

// getCrushMCPServers returns Crush MCP configurations based on the workingOnly flag
func getCrushMCPServers(baseURL string, workingOnly bool) map[string]CrushMcpConfig {
	// Use containerized MCPs if flag is set (ZERO npx dependencies)
	if *useContainerMCPs {
		return buildContainerizedCrushMCPs(baseURL)
	}

	allMCPs := buildCrushMCPServers(baseURL)
	if !workingOnly {
		return allMCPs
	}
	return filterWorkingCrushMCPs(allMCPs)
}

// buildContainerizedCrushMCPs builds Crush MCP configurations using HTTP SSE container endpoints
// ZERO npx commands - all MCPs use containerized remote endpoints
func buildContainerizedCrushMCPs(baseURL string) map[string]CrushMcpConfig {
	generator := mcpconfig.NewContainerMCPConfigGenerator(baseURL)
	containerMCPs := generator.GenerateContainerMCPs()

	result := make(map[string]CrushMcpConfig)

	// Convert ContainerMCPServerConfig to CrushMcpConfig
	for name, cfg := range containerMCPs {
		// Only include enabled MCPs
		if !cfg.Enabled {
			continue
		}
		result[name] = CrushMcpConfig{
			Type:    cfg.Type,
			URL:     cfg.URL,
			Env:     cfg.Env,
			Enabled: cfg.Enabled,
		}
	}

	return result
}

// filterWorkingCrushMCPs filters Crush MCP configurations to only include those with all dependencies met
func filterWorkingCrushMCPs(allMCPs map[string]CrushMcpConfig) map[string]CrushMcpConfig {
	workingMCPs := make(map[string]CrushMcpConfig)

	// MCPs that always work (no external dependencies or API keys)
	// NOTE: Only includes MCPs with VERIFIED npm packages on registry.npmjs.org
	alwaysWorking := map[string]bool{
		// HelixAgent remote endpoints
		"helixagent":            true,
		"helixagent-mcp":        true,
		"helixagent-acp":        true,
		"helixagent-lsp":        true,
		"helixagent-embeddings": true,
		"helixagent-vision":     true,
		"helixagent-cognee":     true,
		// Core Anthropic official MCPs - no API keys required
		"filesystem":          true, // @modelcontextprotocol/server-filesystem - VERIFIED
		"fetch":               true, // uvx mcp-server-fetch (Python) - VERIFIED
		"memory":              true, // @modelcontextprotocol/server-memory - VERIFIED
		"time":                true, // uvx mcp-server-time (Python) - VERIFIED
		"git":                 true, // uvx mcp-server-git (Python) - VERIFIED
		"sqlite":              true, // mcp-server-sqlite-npx (npm) - VERIFIED
		"puppeteer":           true, // @modelcontextprotocol/server-puppeteer - VERIFIED
		"sequential-thinking": true, // @modelcontextprotocol/server-sequential-thinking - VERIFIED
		"everything":          true, // @modelcontextprotocol/server-everything - VERIFIED
	}

	// Environment variable requirements (same as OpenCode)
	envRequirements := map[string][]string{
		"github": {"GITHUB_TOKEN"}, // @modelcontextprotocol/server-github - VERIFIED
	}

	for name, mcpConfig := range allMCPs {
		if alwaysWorking[name] {
			workingMCPs[name] = mcpConfig
			continue
		}

		if reqs, hasReqs := envRequirements[name]; hasReqs {
			allMet := true
			for _, envVar := range reqs {
				if os.Getenv(envVar) == "" {
					allMet = false
					break
				}
			}
			if allMet {
				workingMCPs[name] = mcpConfig
			}
		}
	}

	return workingMCPs
}

// buildCrushMCPServers creates MCP server configurations for Crush CLI
// Local servers are started on demand via npx - no remote servers needed
// COMPLIANCE: 62+ MCPs required for all CLI agents
func buildCrushMCPServers(baseURL string) map[string]CrushMcpConfig {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/home"
	}

	// Get HELIXAGENT_HOME for local MCP plugin path
	helixHome := os.Getenv("HELIXAGENT_HOME")
	if helixHome == "" {
		helixHome = homeDir + "/.helixagent"
	}

	return map[string]CrushMcpConfig{
		// HelixAgent Protocol Endpoints (7 MCPs) - LOCAL + REMOTE
		// Local MCP plugin - runs the HelixAgent MCP server as a subprocess
		"helixagent": {Type: "local", Command: []string{"node", helixHome + "/plugins/mcp-server/dist/index.js", "--endpoint", baseURL}, Enabled: true},
		// Remote protocol endpoints (running at port 8100)
		"helixagent-mcp":        {Type: "remote", URL: baseURL + "/v1/mcp", Enabled: true},
		"helixagent-acp":        {Type: "remote", URL: baseURL + "/v1/acp", Enabled: true},
		"helixagent-lsp":        {Type: "remote", URL: baseURL + "/v1/lsp", Enabled: true},
		"helixagent-embeddings": {Type: "remote", URL: baseURL + "/v1/embeddings", Enabled: true},
		"helixagent-vision":     {Type: "remote", URL: baseURL + "/v1/vision", Enabled: true},
		"helixagent-cognee":     {Type: "remote", URL: baseURL + "/v1/cognee", Enabled: true},

		// Anthropic Official MCPs - LOCAL (started on demand via npx)
		"filesystem":          {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", homeDir}, Enabled: true},
		"fetch":               {Type: "local", Command: []string{"npx", "-y", "mcp-fetch-server"}, Enabled: true},
		"memory":              {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-memory"}, Enabled: true},
		"time":                {Type: "local", Command: []string{"npx", "-y", "@theo.foobar/mcp-time"}, Enabled: true}, // #nosec G101 -- not a credential (map key / config label / env-var reference)
		"git":                 {Type: "local", Command: []string{"npx", "-y", "mcp-git"}, Enabled: true},
		"sqlite":              {Type: "local", Command: []string{"npx", "-y", "mcp-server-sqlite-npx", "/tmp/helixagent.db"}, Enabled: true},
		"postgres":            {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-postgres"}, Env: map[string]string{"POSTGRES_URL": "postgresql://helixagent:helixagent123@localhost:8101/helixagent_db"}, Enabled: true},
		"puppeteer":           {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-puppeteer"}, Enabled: true},
		"sequential-thinking": {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-sequential-thinking"}, Enabled: true},
		"everything":          {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-everything"}, Enabled: true},                                                                              // #nosec G101 -- not a credential (map key / config label / env-var reference)
		"brave-search":        {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-brave-search"}, Env: map[string]string{"BRAVE_API_KEY": "{env:BRAVE_API_KEY}"}, Enabled: true},            // #nosec G101 -- not a credential (map key / config label / env-var reference)
		"google-maps":         {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-google-maps"}, Env: map[string]string{"GOOGLE_MAPS_API_KEY": "{env:GOOGLE_MAPS_API_KEY}"}, Enabled: true}, // #nosec G101 -- not a credential (map key / config label / env-var reference)
		"slack":               {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-slack"}, Env: map[string]string{"SLACK_BOT_TOKEN": "{env:SLACK_BOT_TOKEN}"}, Enabled: true},               // #nosec G101 -- not a credential (map key / config label / env-var reference)
		"github":              {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-github"}, Env: map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": "{env:GITHUB_TOKEN}"}, Enabled: true},
		"gitlab":              {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-gitlab"}, Env: map[string]string{"GITLAB_PERSONAL_ACCESS_TOKEN": "{env:GITLAB_TOKEN}"}, Enabled: true},
		"sentry":              {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-sentry"}, Env: map[string]string{"SENTRY_AUTH_TOKEN": "{env:SENTRY_AUTH_TOKEN}"}, Enabled: true}, // #nosec G101 -- not a credential (map key / config label / env-var reference)
		"everart":             {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-everart"}, Env: map[string]string{"EVERART_API_KEY": "{env:EVERART_API_KEY}"}, Enabled: true},
		"aws-kb-retrieval":    {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-aws-kb-retrieval"}, Env: map[string]string{"AWS_ACCESS_KEY_ID": "{env:AWS_ACCESS_KEY_ID}"}, Enabled: true},
		"gdrive":              {Type: "local", Command: []string{"npx", "-y", "@anthropic/mcp-server-gdrive"}, Env: map[string]string{"GOOGLE_CREDENTIALS_PATH": "{env:GOOGLE_CREDENTIALS_PATH}"}, Enabled: true},

		// Additional Anthropic & Community MCPs - LOCAL
		"exa":        {Type: "local", Command: []string{"npx", "-y", "exa-mcp-server"}, Env: map[string]string{"EXA_API_KEY": "{env:EXA_API_KEY}"}, Enabled: true},
		"linear":     {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-linear"}, Env: map[string]string{"LINEAR_API_KEY": "{env:LINEAR_API_KEY}"}, Enabled: true}, // #nosec G101 -- not a credential (map key / config label / env-var reference)
		"notion":     {Type: "local", Command: []string{"npx", "-y", "@notionhq/notion-mcp-server"}, Env: map[string]string{"NOTION_API_KEY": "{env:NOTION_API_KEY}"}, Enabled: true},
		"figma":      {Type: "local", Command: []string{"npx", "-y", "figma-developer-mcp"}, Env: map[string]string{"FIGMA_API_KEY": "{env:FIGMA_API_KEY}"}, Enabled: true},
		"todoist":    {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-todoist"}, Env: map[string]string{"TODOIST_API_TOKEN": "{env:TODOIST_API_TOKEN}"}, Enabled: true}, // #nosec G101 -- not a credential (map key / config label / env-var reference)
		"obsidian":   {Type: "local", Command: []string{"npx", "-y", "mcp-obsidian"}, Env: map[string]string{"OBSIDIAN_VAULT_PATH": "{env:OBSIDIAN_VAULT_PATH}"}, Enabled: true},                     // #nosec G101 -- not a credential (map key / config label / env-var reference)
		"raycast":    {Type: "local", Command: []string{"npx", "-y", "@raycast/mcp-server-raycast"}, Enabled: true},
		"tinybird":   {Type: "local", Command: []string{"npx", "-y", "mcp-tinybird"}, Env: map[string]string{"TINYBIRD_TOKEN": "{env:TINYBIRD_TOKEN}"}, Enabled: true},
		"cloudflare": {Type: "local", Command: []string{"npx", "-y", "@cloudflare/mcp-server-cloudflare"}, Env: map[string]string{"CLOUDFLARE_API_TOKEN": "{env:CLOUDFLARE_API_TOKEN}"}, Enabled: true},
		"neon":       {Type: "local", Command: []string{"npx", "-y", "@neondatabase/mcp-server-neon"}, Env: map[string]string{"NEON_API_KEY": "{env:NEON_API_KEY}"}, Enabled: true},

		// Container/Infrastructure MCPs - LOCAL
		"docker":        {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-docker"}, Enabled: true},
		"kubernetes":    {Type: "local", Command: []string{"npx", "-y", "mcp-server-kubernetes"}, Env: map[string]string{"KUBECONFIG": "{env:KUBECONFIG}"}, Enabled: true},
		"redis":         {Type: "local", Command: []string{"npx", "-y", "mcp-server-redis"}, Env: map[string]string{"REDIS_URL": "redis://localhost:6379"}, Enabled: true},
		"mongodb":       {Type: "local", Command: []string{"npx", "-y", "mcp-server-mongodb"}, Env: map[string]string{"MONGODB_URI": "mongodb://localhost:27017"}, Enabled: true},
		"elasticsearch": {Type: "local", Command: []string{"npx", "-y", "mcp-server-elasticsearch"}, Env: map[string]string{"ELASTICSEARCH_URL": "http://localhost:9200"}, Enabled: true},
		"qdrant":        {Type: "local", Command: []string{"npx", "-y", "mcp-server-qdrant"}, Env: map[string]string{"QDRANT_URL": "http://localhost:6333"}, Enabled: true},
		"chroma":        {Type: "local", Command: []string{"npx", "-y", "mcp-server-chroma"}, Env: map[string]string{"CHROMA_URL": "http://localhost:8001"}, Enabled: true},
		"pinecone":      {Type: "local", Command: []string{"npx", "-y", "mcp-server-pinecone"}, Env: map[string]string{"PINECONE_API_KEY": "{env:PINECONE_API_KEY}"}, Enabled: true},
		"milvus":        {Type: "local", Command: []string{"npx", "-y", "mcp-server-milvus"}, Env: map[string]string{"MILVUS_HOST": "localhost"}, Enabled: true},
		"weaviate":      {Type: "local", Command: []string{"npx", "-y", "mcp-server-weaviate"}, Env: map[string]string{"WEAVIATE_URL": "http://localhost:8080"}, Enabled: true},
		"supabase":      {Type: "local", Command: []string{"npx", "-y", "mcp-server-supabase"}, Env: map[string]string{"SUPABASE_URL": "{env:SUPABASE_URL}"}, Enabled: true},
		// #nosec G101 -- not a credential (map key / config label / env-var reference)
		// Productivity/Collaboration MCPs - LOCAL
		"jira":            {Type: "local", Command: []string{"npx", "-y", "mcp-server-atlassian"}, Env: map[string]string{"JIRA_URL": "{env:JIRA_URL}"}, Enabled: true},
		"asana":           {Type: "local", Command: []string{"npx", "-y", "mcp-server-asana"}, Env: map[string]string{"ASANA_ACCESS_TOKEN": "{env:ASANA_ACCESS_TOKEN}"}, Enabled: true},
		"trello":          {Type: "local", Command: []string{"npx", "-y", "mcp-server-trello"}, Env: map[string]string{"TRELLO_API_KEY": "{env:TRELLO_API_KEY}"}, Enabled: true}, // #nosec G101 -- not a credential (map key / config label / env-var reference)
		"monday":          {Type: "local", Command: []string{"npx", "-y", "mcp-server-monday"}, Env: map[string]string{"MONDAY_API_KEY": "{env:MONDAY_API_KEY}"}, Enabled: true},
		"clickup":         {Type: "local", Command: []string{"npx", "-y", "mcp-server-clickup"}, Env: map[string]string{"CLICKUP_API_KEY": "{env:CLICKUP_API_KEY}"}, Enabled: true},     // #nosec G101 -- not a credential (map key / config label / env-var reference)
		"discord":         {Type: "local", Command: []string{"npx", "-y", "mcp-server-discord"}, Env: map[string]string{"DISCORD_BOT_TOKEN": "{env:DISCORD_BOT_TOKEN}"}, Enabled: true}, // #nosec G101 -- not a credential (map key / config label / env-var reference)
		"microsoft-teams": {Type: "local", Command: []string{"npx", "-y", "mcp-server-teams"}, Env: map[string]string{"TEAMS_CLIENT_ID": "{env:TEAMS_CLIENT_ID}"}, Enabled: true},
		"gmail":           {Type: "local", Command: []string{"npx", "-y", "mcp-server-gmail"}, Env: map[string]string{"GOOGLE_CREDENTIALS_PATH": "{env:GOOGLE_CREDENTIALS_PATH}"}, Enabled: true},
		"calendar":        {Type: "local", Command: []string{"npx", "-y", "mcp-server-google-calendar"}, Env: map[string]string{"GOOGLE_CREDENTIALS_PATH": "{env:GOOGLE_CREDENTIALS_PATH}"}, Enabled: true},
		"zoom":            {Type: "local", Command: []string{"npx", "-y", "mcp-server-zoom"}, Env: map[string]string{"ZOOM_CLIENT_ID": "{env:ZOOM_CLIENT_ID}"}, Enabled: true},

		// Cloud/DevOps MCPs - LOCAL
		"aws-s3":     {Type: "local", Command: []string{"npx", "-y", "mcp-server-s3"}, Env: map[string]string{"AWS_ACCESS_KEY_ID": "{env:AWS_ACCESS_KEY_ID}"}, Enabled: true}, // #nosec G101 -- not a credential (map key / config label / env-var reference)
		"aws-lambda": {Type: "local", Command: []string{"npx", "-y", "mcp-server-aws-lambda"}, Env: map[string]string{"AWS_ACCESS_KEY_ID": "{env:AWS_ACCESS_KEY_ID}"}, Enabled: true},
		"azure":      {Type: "local", Command: []string{"npx", "-y", "mcp-server-azure"}, Env: map[string]string{"AZURE_SUBSCRIPTION_ID": "{env:AZURE_SUBSCRIPTION_ID}"}, Enabled: true},
		"gcp":        {Type: "local", Command: []string{"npx", "-y", "mcp-server-gcp"}, Env: map[string]string{"GOOGLE_APPLICATION_CREDENTIALS": "{env:GOOGLE_APPLICATION_CREDENTIALS}"}, Enabled: true},
		"terraform":  {Type: "local", Command: []string{"npx", "-y", "mcp-server-terraform"}, Enabled: true},
		"ansible":    {Type: "local", Command: []string{"npx", "-y", "mcp-server-ansible"}, Enabled: true},
		"datadog":    {Type: "local", Command: []string{"npx", "-y", "mcp-server-datadog"}, Env: map[string]string{"DD_API_KEY": "{env:DD_API_KEY}"}, Enabled: true}, // #nosec G101 -- not a credential (map key / config label / env-var reference)
		"grafana":    {Type: "local", Command: []string{"npx", "-y", "mcp-server-grafana"}, Env: map[string]string{"GRAFANA_URL": "{env:GRAFANA_URL}"}, Enabled: true},
		"prometheus": {Type: "local", Command: []string{"npx", "-y", "mcp-server-prometheus"}, Env: map[string]string{"PROMETHEUS_URL": "{env:PROMETHEUS_URL}"}, Enabled: true},
		"circleci":   {Type: "local", Command: []string{"npx", "-y", "mcp-server-circleci"}, Env: map[string]string{"CIRCLECI_TOKEN": "{env:CIRCLECI_TOKEN}"}, Enabled: true},

		// AI/ML Integration MCPs - LOCAL
		"langchain":  {Type: "local", Command: []string{"npx", "-y", "mcp-server-langchain"}, Env: map[string]string{"OPENAI_API_KEY": "{env:OPENAI_API_KEY}"}, Enabled: true}, // #nosec G101 -- not a credential (map key / config label / env-var reference)
		"llamaindex": {Type: "local", Command: []string{"npx", "-y", "mcp-server-llamaindex"}, Env: map[string]string{"OPENAI_API_KEY": "{env:OPENAI_API_KEY}"}, Enabled: true},
		// NOTE: huggingface MCP removed — npm package does not exist
		"replicate":        {Type: "local", Command: []string{"npx", "-y", "mcp-server-replicate"}, Env: map[string]string{"REPLICATE_API_TOKEN": "{env:REPLICATE_API_TOKEN}"}, Enabled: true},
		"stable-diffusion": {Type: "local", Command: []string{"npx", "-y", "mcp-server-stable-diffusion"}, Env: map[string]string{"STABILITY_API_KEY": "{env:STABILITY_API_KEY}"}, Enabled: true},
	}
}

// CrushValidationResult holds the validation results for Crush config
type CrushValidationResult struct {
	Valid    bool                      `json:"valid"`
	Errors   []OpenCodeValidationError `json:"errors"`
	Warnings []string                  `json:"warnings"`
	Stats    *CrushValidationStats     `json:"stats,omitempty"`
}

// CrushValidationStats contains statistics about the validated Crush config
type CrushValidationStats struct {
	Providers  int `json:"providers"`
	Models     int `json:"models"`
	LspConfigs int `json:"lsp_configs"`
}

// handleValidateCrush handles the --validate-crush-config command
func handleValidateCrush(appCfg *AppConfig) error {
	logger := appCfg.Logger
	if logger == nil {
		logger = logrus.New()
	}

	filePath := appCfg.ValidateCrush

	// Validate path for traversal attacks (G304 security fix)
	if !utils.ValidatePath(filePath) {
		return fmt.Errorf("invalid config file path: contains path traversal or dangerous characters")
	}

	// Read the config file
	// #nosec G304 - filePath is validated by utils.ValidatePath and provided via CLI by admin
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Perform validation
	result := validateCrushConfig(data)

	// Output header
	fmt.Println("======================================================================")
	fmt.Println("HELIXAGENT CRUSH CONFIGURATION VALIDATION")
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
			fmt.Printf("  - Models: %d\n", result.Stats.Models)
			fmt.Printf("  - LSP configs: %d\n", result.Stats.LspConfigs)
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

// ValidCrushTopLevelKeys contains the valid top-level keys per Crush schema
var ValidCrushTopLevelKeys = map[string]bool{
	"$schema":   true,
	"providers": true,
	"lsp":       true,
	"mcp":       true,
	"options":   true,
}

// validateCrushConfig performs comprehensive validation of a Crush config
func validateCrushConfig(data []byte) *CrushValidationResult {
	result := &CrushValidationResult{
		Valid:    true,
		Errors:   []OpenCodeValidationError{},
		Warnings: []string{},
		Stats:    &CrushValidationStats{},
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

	// Check for invalid top-level keys
	var invalidKeys []string
	for key := range rawConfig {
		if !ValidCrushTopLevelKeys[key] {
			invalidKeys = append(invalidKeys, key)
		}
	}
	if len(invalidKeys) > 0 {
		result.Valid = false
		result.Errors = append(result.Errors, OpenCodeValidationError{
			Field:   "",
			Message: fmt.Sprintf("invalid top-level keys: %v (valid keys: $schema, providers, lsp, mcp, options)", invalidKeys),
		})
	}

	// Parse and validate providers
	totalModels := 0
	if providers, ok := rawConfig["providers"].(map[string]interface{}); ok {
		result.Stats.Providers = len(providers)
		for name, providerData := range providers {
			if provider, ok := providerData.(map[string]interface{}); ok {
				// Provider must have name
				if _, hasName := provider["name"]; !hasName {
					result.Valid = false
					result.Errors = append(result.Errors, OpenCodeValidationError{
						Field:   fmt.Sprintf("providers.%s.name", name),
						Message: "provider must have a name",
					})
				}

				// Provider must have type
				if _, hasType := provider["type"]; !hasType {
					result.Valid = false
					result.Errors = append(result.Errors, OpenCodeValidationError{
						Field:   fmt.Sprintf("providers.%s.type", name),
						Message: "provider must have a type",
					})
				}

				// Provider must have base_url
				if _, hasBaseURL := provider["base_url"]; !hasBaseURL {
					result.Valid = false
					result.Errors = append(result.Errors, OpenCodeValidationError{
						Field:   fmt.Sprintf("providers.%s.base_url", name),
						Message: "provider must have a base_url",
					})
				}

				// Provider must have models
				if models, hasModels := provider["models"].([]interface{}); hasModels {
					totalModels += len(models)
					for i, modelData := range models {
						if model, ok := modelData.(map[string]interface{}); ok {
							// Model must have id
							if _, hasID := model["id"]; !hasID {
								result.Valid = false
								result.Errors = append(result.Errors, OpenCodeValidationError{
									Field:   fmt.Sprintf("providers.%s.models[%d].id", name, i),
									Message: "model must have an id",
								})
							}
							// Model must have name
							if _, hasName := model["name"]; !hasName {
								result.Valid = false
								result.Errors = append(result.Errors, OpenCodeValidationError{
									Field:   fmt.Sprintf("providers.%s.models[%d].name", name, i),
									Message: "model must have a name",
								})
							}
						}
					}
				} else {
					result.Valid = false
					result.Errors = append(result.Errors, OpenCodeValidationError{
						Field:   fmt.Sprintf("providers.%s.models", name),
						Message: "provider must have at least one model",
					})
				}
			}
		}
	} else if rawConfig["providers"] == nil {
		result.Valid = false
		result.Errors = append(result.Errors, OpenCodeValidationError{
			Field:   "providers",
			Message: "at least one provider must be configured",
		})
	}
	result.Stats.Models = totalModels

	// Parse and validate LSP configs
	if lspConfigs, ok := rawConfig["lsp"].(map[string]interface{}); ok {
		result.Stats.LspConfigs = len(lspConfigs)
		for name, lspData := range lspConfigs {
			if lsp, ok := lspData.(map[string]interface{}); ok {
				// LSP must have command
				if _, hasCommand := lsp["command"]; !hasCommand {
					result.Valid = false
					result.Errors = append(result.Errors, OpenCodeValidationError{
						Field:   fmt.Sprintf("lsp.%s.command", name),
						Message: "LSP config must have a command",
					})
				}
			}
		}
	}

	// Add warnings for missing recommended fields
	if _, hasSchema := rawConfig["$schema"]; !hasSchema {
		result.Warnings = append(result.Warnings, "$schema field is recommended for validation")
	}

	return result
}

// ============================================================================
// Unified CLI Agent Handlers (All 48 Agents)
// ============================================================================

// handleListAgents lists all 48 supported CLI agents
func handleListAgents(appCfg *AppConfig) error {
	fmt.Println("HelixAgent - Supported CLI Agents (48 total)")
	fmt.Println("=============================================")
	fmt.Println()

	generator := cliagents.NewUnifiedGenerator(nil)
	schemas := generator.GetAllSchemas()

	// Group by category
	original18 := []cliagents.AgentType{
		cliagents.AgentOpenCode, cliagents.AgentCrush, cliagents.AgentHelixCode,
		cliagents.AgentKiro, cliagents.AgentAider, cliagents.AgentClaudeCode,
		cliagents.AgentCline, cliagents.AgentCodenameGoose, cliagents.AgentDeepSeekCLI,
		cliagents.AgentForge, cliagents.AgentGeminiCLI, cliagents.AgentGPTEngineer,
		cliagents.AgentKiloCode, cliagents.AgentMistralCode, cliagents.AgentOllamaCode,
		cliagents.AgentPlandex, cliagents.AgentQwenCode, cliagents.AgentAmazonQ,
	}

	new30 := []cliagents.AgentType{
		cliagents.AgentAgentDeck, cliagents.AgentBridle, cliagents.AgentCheshireCat,
		cliagents.AgentClaudePlugins, cliagents.AgentClaudeSquad, cliagents.AgentCodai,
		cliagents.AgentCodex, cliagents.AgentCodexSkills, cliagents.AgentConduit,
		cliagents.AgentContinue, cliagents.AgentEmdash, cliagents.AgentFauxPilot,
		cliagents.AgentGetShitDone, cliagents.AgentGitHubCopilotCLI, cliagents.AgentGitHubSpecKit,
		cliagents.AgentGitMCP, cliagents.AgentGPTME, cliagents.AgentMobileAgent,
		cliagents.AgentMultiagentCoding, cliagents.AgentNanocoder, cliagents.AgentNoi,
		cliagents.AgentOctogen, cliagents.AgentOpenHands, cliagents.AgentPostgresMCP,
		cliagents.AgentShai, cliagents.AgentSnowCLI, cliagents.AgentTaskWeaver,
		cliagents.AgentUIUXProMax, cliagents.AgentVTCode, cliagents.AgentWarp,
	}

	fmt.Println("Original 18 Agents:")
	fmt.Println("-------------------")
	for _, agent := range original18 {
		if schema, ok := schemas[agent]; ok {
			fmt.Printf("  %-20s  %s\n", agent, schema.Description)
		}
	}

	fmt.Println()
	fmt.Println("New 30 Agents:")
	fmt.Println("--------------")
	for _, agent := range new30 {
		if schema, ok := schemas[agent]; ok {
			fmt.Printf("  %-20s  %s\n", agent, schema.Description)
		}
	}

	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  helixagent --generate-agent-config=<agent-name>")
	fmt.Println("  helixagent --generate-agent-config=<agent-name> --agent-config-output=<path>")
	fmt.Println("  helixagent --validate-agent-config=<agent-name>:<config-path>")
	fmt.Println("  helixagent --generate-all-agents --all-agents-output-dir=<directory>")

	return nil
}

// handleGenerateAgentConfig generates configuration for a specific CLI agent
func handleGenerateAgentConfig(appCfg *AppConfig) error {
	logger := appCfg.Logger
	if logger == nil {
		logger = logrus.New()
	}

	// Special case for OpenCode - use the dedicated handler with v1.1.30+ schema
	if strings.EqualFold(appCfg.GenerateAgentConfig, "opencode") {
		// Transfer output path setting if specified
		if appCfg.AgentConfigOutput != "" {
			appCfg.OpenCodeOutput = appCfg.AgentConfigOutput
		}
		return handleGenerateOpenCode(appCfg)
	}

	agentType := cliagents.AgentType(appCfg.GenerateAgentConfig)

	// Load environment variables for HelixLLM configuration
	agentEnvVars := loadEnvVars()

	// Get HelixLLM endpoint from env — parse host and port from URL
	helixLLMHost := "localhost"
	helixLLMPort := 8443
	helixLLMAPIKey := ""
	if val, ok := agentEnvVars["HELIX_LLM_ENDPOINT"]; ok && val != "" {
		if u, err := url.Parse(val); err == nil {
			helixLLMHost = u.Hostname()
			if p := u.Port(); p != "" {
				if pp, err := strconv.Atoi(p); err == nil {
					helixLLMPort = pp
				}
			}
		}
	}
	if val, ok := agentEnvVars["HELIX_LLM_API_KEY"]; ok && val != "" {
		helixLLMAPIKey = val
	}

	// Build MCP server list: default + HelixLLM endpoints
	mcpServers := cliagents.DefaultMCPServers()
	mcpServers = append(mcpServers, cliagents.HelixLLMMCPServers(helixLLMHost, helixLLMPort)...)

	// Create generator with HelixAgent + HelixLLM settings
	config := &cliagents.GeneratorConfig{
		HelixAgentHost: "localhost",
		HelixAgentPort: 8100,
		HelixLLMHost:   helixLLMHost,
		HelixLLMPort:   helixLLMPort,
		HelixLLMAPIKey: helixLLMAPIKey,
		MCPServers:     mcpServers,
		IncludeScores:  true,
	}
	generator := cliagents.NewUnifiedGenerator(config)

	ctx := context.Background()
	result, err := generator.Generate(ctx, agentType)
	if err != nil {
		return fmt.Errorf("failed to generate config for %s: %w", agentType, err)
	}

	if !result.Success {
		return fmt.Errorf("config generation failed for %s: %v", agentType, result.Errors)
	}

	// Expand {env:VAR_NAME} placeholders with actual environment variable values
	envVars := loadEnvVars()
	expandedConfig := expandEnvInConfig(result.Config, envVars)

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(expandedConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Output to file or stdout
	if appCfg.AgentConfigOutput != "" {
		if !utils.ValidatePath(appCfg.AgentConfigOutput) {
			return fmt.Errorf("invalid output path: %s", appCfg.AgentConfigOutput)
		}
		if err := os.WriteFile(appCfg.AgentConfigOutput, jsonData, 0644); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}
		logger.Infof("Generated %s config written to: %s", agentType, appCfg.AgentConfigOutput)
	} else {
		fmt.Println(string(jsonData))
	}

	return nil
}

// handleValidateAgentConfig validates a configuration file for a specific CLI agent
func handleValidateAgentConfig(appCfg *AppConfig) error {
	logger := appCfg.Logger
	if logger == nil {
		logger = logrus.New()
	}

	// Parse agent:path format
	parts := strings.SplitN(appCfg.ValidateAgentConfig, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid format for --validate-agent-config, expected: agent-name:config-path")
	}

	agentType := cliagents.AgentType(parts[0])
	configPath := parts[1]

	// Validate path
	if !utils.ValidatePath(configPath) {
		return fmt.Errorf("invalid config path: %s", configPath)
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Special case for OpenCode - use v1.1.30+ schema validation
	if strings.EqualFold(string(agentType), "opencode") {
		result := validateOpenCodeConfig(data)
		if result.Valid {
			fmt.Printf("✓ Config file is valid for %s\n", agentType)
			if len(result.Warnings) > 0 {
				fmt.Println("\nWarnings:")
				for _, w := range result.Warnings {
					fmt.Printf("  - %s\n", w)
				}
			}
		} else {
			fmt.Printf("✗ Config file is invalid for %s\n", agentType)
			fmt.Println("\nErrors:")
			for _, e := range result.Errors {
				fmt.Printf("  - %s\n", e.Message)
			}
			return fmt.Errorf("validation failed with %d errors", len(result.Errors))
		}
		return nil
	}

	// Parse as JSON
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Validate using LLMsVerifier
	generator := cliagents.NewUnifiedGenerator(nil)
	result, err := generator.Validate(agentType, config)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Output results
	if result.Valid {
		fmt.Printf("✓ Config file is valid for %s\n", agentType)
		if len(result.Warnings) > 0 {
			fmt.Println("\nWarnings:")
			for _, warning := range result.Warnings {
				fmt.Printf("  - %s\n", warning)
			}
		}
	} else {
		fmt.Printf("✗ Config file is invalid for %s\n", agentType)
		fmt.Println("\nErrors:")
		for _, e := range result.Errors {
			fmt.Printf("  - %s\n", e)
		}
		return fmt.Errorf("validation failed with %d errors", len(result.Errors))
	}

	return nil
}

// handleGenerateAllAgents generates configurations for all 48 CLI agents
func handleGenerateAllAgents(appCfg *AppConfig) error {
	logger := appCfg.Logger
	if logger == nil {
		logger = logrus.New()
	}

	if appCfg.AllAgentsOutputDir == "" {
		return fmt.Errorf("--all-agents-output-dir is required when using --generate-all-agents")
	}

	if !utils.ValidatePath(appCfg.AllAgentsOutputDir) {
		return fmt.Errorf("invalid output directory: %s", appCfg.AllAgentsOutputDir)
	}
	outputDir := appCfg.AllAgentsOutputDir

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create generator with HelixAgent settings
	config := &cliagents.GeneratorConfig{
		HelixAgentHost: "localhost",
		HelixAgentPort: 8100,
		OutputDir:      outputDir,
		MCPServers:     cliagents.DefaultMCPServers(),
		IncludeScores:  true,
	}
	generator := cliagents.NewUnifiedGenerator(config)

	ctx := context.Background()
	results, err := generator.GenerateAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to generate all configs: %w", err)
	}

	// Save each config and report results
	successCount := 0
	failCount := 0

	fmt.Printf("Generating configurations for 48 CLI agents in: %s\n\n", outputDir)

	for _, result := range results {
		// Special case for OpenCode - use v1.1.30+ schema
		if string(result.AgentType) == "opencode" {
			outputPath := fmt.Sprintf("%s/opencode.json", outputDir)
			openCodeAppCfg := &AppConfig{
				Logger:         logger,
				OpenCodeOutput: outputPath,
			}
			if err := handleGenerateOpenCode(openCodeAppCfg); err != nil {
				fmt.Printf("✗ %-20s  Failed to generate: %v\n", result.AgentType, err)
				failCount++
			} else {
				fmt.Printf("✓ %-20s  %s\n", result.AgentType, "opencode.json")
				successCount++
			}
			continue
		}

		if result.Success {
			// Get schema for filename
			schema, _ := generator.GetSchema(result.AgentType)
			outputPath := fmt.Sprintf("%s/%s", outputDir, schema.ConfigFileName)

			jsonData, err := json.MarshalIndent(result.Config, "", "  ")
			if err != nil {
				fmt.Printf("✗ %-20s  Failed to marshal: %v\n", result.AgentType, err)
				failCount++
				continue
			}

			if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
				fmt.Printf("✗ %-20s  Failed to write: %v\n", result.AgentType, err)
				failCount++
				continue
			}

			fmt.Printf("✓ %-20s  %s\n", result.AgentType, schema.ConfigFileName)
			successCount++
		} else {
			fmt.Printf("✗ %-20s  %v\n", result.AgentType, result.Errors)
			failCount++
		}
	}

	fmt.Printf("\n")
	fmt.Printf("Summary: %d succeeded, %d failed\n", successCount, failCount)

	if failCount > 0 {
		return fmt.Errorf("%d configurations failed to generate", failCount)
	}

	logger.Infof("All 48 agent configurations generated in: %s", outputDir)
	return nil
}

func showHelp() {
	fmt.Printf(`HelixAgent - Advanced LLM Gateway with Mem0 Memory Integration

Usage:
  helixagent [options]

Options:
  -config string
        Path to configuration file (YAML)
  -auto-start-docker
        Automatically start required Docker containers (default: true)
  -strict-dependencies
        MANDATORY: Fail if any integration dependency is unavailable (default: true)
        When enabled, HelixAgent will NOT start unless ALL dependencies are healthy:
        - PostgreSQL (database)
        - Redis (cache)
        - Mem0 (memory system with entity graphs)
        - ChromaDB (vector database)
  -generate-api-key
        Generate a new HelixAgent API key and output it to stdout
  -generate-opencode-config
        Generate OpenCode configuration JSON (uses HELIXAGENT_API_KEY env or generates new)
  -validate-opencode-config string
        Validate an existing OpenCode configuration file (uses LLMsVerifier schema rules)
  -opencode-output string
        Output path for OpenCode config (default: stdout)
  -generate-crush-config
        Generate Crush CLI configuration JSON (uses HELIXAGENT_API_KEY env or generates new)
  -validate-crush-config string
        Validate an existing Crush configuration file (uses LLMsVerifier schema rules)
  -crush-output string
        Output path for Crush config (default: stdout)
  -api-key-env-file string
        Path to .env file to write the generated API key
  -preinstall-mcp
        Pre-install standard MCP server npm packages for faster startup
  -skip-mcp-preinstall
        Skip automatic MCP package pre-installation at startup
  -working-mcps-only
        Only include MCPs with all dependencies met (API keys set, services running)
        Use with -generate-opencode-config or -generate-crush-config
  -use-local-mcp-servers
        Use local Docker-based MCP servers instead of npx commands
        Requires: ./scripts/mcp/start-mcp-servers.sh --start
        MCP servers run on TCP ports 9101-9601 via socat

Unified CLI Agent Configuration (48 agents):
  -list-agents
        List all 48 supported CLI agents with descriptions
  -generate-agent-config string
        Generate config for specified CLI agent (e.g., codex, openhands, claude-squad)
  -agent-config-output string
        Output path for generated agent config (default: stdout)
  -validate-agent-config string
        Validate config file for agent (format: agent-name:config-path)
  -generate-all-agents
        Generate configurations for all 48 CLI agents
  -all-agents-output-dir string
        Output directory for all agent configs (required with --generate-all-agents)

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

  # Generate Crush CLI configuration
  helixagent -generate-crush-config

  # Generate Crush config and save to file
  helixagent -generate-crush-config -crush-output crush.json

  # Validate an existing Crush configuration file
  helixagent -validate-crush-config ~/.config/crush/crush.json

Examples:
  helixagent
  helixagent -auto-start-docker=false
  helixagent -config /path/to/config.yaml
  helixagent -generate-crush-config -crush-output /tmp/crush.json
  helixagent -version

For more information, visit: https://dev.helix.agent
`)
}

// expandEnvInMCPServers expands {env:VAR_NAME} placeholders in MCP server configurations
func expandEnvInMCPServers(mcpServers map[string]OpenCodeMCPServerDefNew, envVars map[string]string) map[string]OpenCodeMCPServerDefNew {
	result := make(map[string]OpenCodeMCPServerDefNew)
	for name, server := range mcpServers {
		expanded := server
		if expanded.Environment != nil {
			expandedEnv := make(map[string]string)
			for key, value := range expanded.Environment {
				expandedEnv[key] = expandEnvValue(value, envVars)
			}
			expanded.Environment = expandedEnv
		}
		if expanded.Headers != nil {
			expandedHeaders := make(map[string]string)
			for key, value := range expanded.Headers {
				expandedHeaders[key] = expandEnvValue(value, envVars)
			}
			expanded.Headers = expandedHeaders
		}
		result[name] = expanded
	}
	return result
}

// expandEnvInConfig recursively expands {env:VAR_NAME} placeholders in a configuration map
func expandEnvInConfig(config interface{}, envVars map[string]string) interface{} {
	switch v := config.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			result[key] = expandEnvInConfig(val, envVars)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = expandEnvInConfig(val, envVars)
		}
		return result
	case string:
		return expandEnvValue(v, envVars)
	default:
		return v
	}
}

// expandEnvInOpenCodeConfig expands {env:VAR_NAME} placeholders in OpenCode configuration
func expandEnvInOpenCodeConfig(config OpenCodeConfig, envVars map[string]string) (OpenCodeConfig, error) {
	// Convert config to map[string]interface{}
	data, err := json.Marshal(config)
	if err != nil {
		return config, fmt.Errorf("failed to marshal config: %w", err)
	}
	var configMap map[string]interface{}
	if err := json.Unmarshal(data, &configMap); err != nil {
		return config, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	// Expand environment variables
	expandedMap := expandEnvInConfig(configMap, envVars)
	// Convert back to OpenCodeConfig
	expandedData, err := json.Marshal(expandedMap)
	if err != nil {
		return config, fmt.Errorf("failed to marshal expanded config: %w", err)
	}
	var expandedConfig OpenCodeConfig
	if err := json.Unmarshal(expandedData, &expandedConfig); err != nil {
		return config, fmt.Errorf("failed to unmarshal expanded config: %w", err)
	}
	return expandedConfig, nil
}

// expandEnvValue expands {env:VAR_NAME} placeholders with actual environment variable values
func expandEnvValue(value string, envVars map[string]string) string {
	if strings.HasPrefix(value, "{env:") && strings.HasSuffix(value, "}") {
		envVar := strings.TrimSuffix(strings.TrimPrefix(value, "{env:"), "}")
		if val, ok := envVars[envVar]; ok && val != "" {
			return val
		}
	}
	return value
}

// loadEnvVars loads environment variables from .env and environment
func loadEnvVars() map[string]string {
	envVars := make(map[string]string)
	// Load from .env file
	if data, err := os.ReadFile(".env"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				envVars[key] = value
			}
		}
	}
	// Also load from environment (overrides .env)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envVars[parts[0]] = parts[1]
		}
	}
	return envVars
}

func showVersion() {
	info := appversion.Get()
	fmt.Println(info.String())
}
