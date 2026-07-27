package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"digital.vasic.containers/pkg/orchestrator"

	"github.com/sirupsen/logrus"
)

var (
	autoStartLSP          = true
	autoStartRAG          = true
	globalSvcOrchestrator *orchestrator.DefaultOrchestrator
)

func initServiceOrchestrator(logger *logrus.Logger) error {
	if globalContainerAdapter == nil {
		return fmt.Errorf("container adapter not initialized")
	}

	globalSvcOrchestrator = globalContainerAdapter.NewServiceOrchestrator()
	if globalSvcOrchestrator == nil {
		return fmt.Errorf("failed to create service orchestrator")
	}

	projectDir, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("failed to get project directory: %w", err)
	}
	dockerDir := filepath.Join(projectDir, "docker")

	if err := globalSvcOrchestrator.DiscoverServices(dockerDir); err != nil {
		logger.WithError(err).Warn("Failed to discover some docker services")
	}

	globalSvcOrchestrator.AddService(orchestrator.Service{
		Name:        "mcp",
		ComposeFile: "docker/mcp/docker-compose.mcp-servers.yml",
		Description: "MCP servers (32+ containerized servers on ports 9101-9999)",
	})

	globalSvcOrchestrator.AddService(orchestrator.Service{
		Name:        "lsp",
		ComposeFile: "docker/lsp/docker-compose.lsp.yml",
		Profile:     "lsp",
		Description: "LSP servers (11 language servers on ports 5001-5024)",
	})

	globalSvcOrchestrator.AddService(orchestrator.Service{
		Name:        "rag",
		ComposeFile: "docker/rag/docker-compose.rag.yml",
		Profile:     "rag",
		Description: "RAG services (Qdrant, RAG Manager on ports 6333-8030)",
	})

	globalSvcOrchestrator.AddService(orchestrator.Service{
		Name:        "formatters",
		ComposeFile: "docker/formatters/docker-compose.formatters.yml",
		Description: "Formatters (32+ code formatters on ports 9210-9300)",
	})

	globalSvcOrchestrator.AddService(orchestrator.Service{
		Name:        "embeddings",
		ComposeFile: "docker/embeddings/docker-compose.embeddings.yml",
		Description: "Embeddings service for vector generation",
	})

	globalSvcOrchestrator.AddService(orchestrator.Service{
		Name:        "acp",
		ComposeFile: "docker/acp/docker-compose.acp.yml",
		Description: "ACP servers for agent communication",
	})

	globalSvcOrchestrator.AddService(orchestrator.Service{
		Name:        "vision",
		ComposeFile: "docker/vision/docker-compose.vision.yml",
		Description: "Vision analysis service",
	})

	globalSvcOrchestrator.AddService(orchestrator.Service{
		Name:        "monitoring",
		ComposeFile: "docker/monitoring/docker-compose.yml",
		Description: "Prometheus, Grafana, Alertmanager stack",
	})

	logger.Infof("Service orchestrator initialized with %d services", globalSvcOrchestrator.ServiceCount())
	if globalSvcOrchestrator.IsRemoteEnabled() {
		logger.Info("Remote distribution ENABLED - all services will deploy to remote host(s)")
	}

	return nil
}

func ensureLSPServers(logger *logrus.Logger) error {
	if globalSvcOrchestrator == nil {
		if err := initServiceOrchestrator(logger); err != nil {
			return err
		}
	}

	logger.Info("Starting LSP servers via unified orchestrator")
	return globalSvcOrchestrator.StartService(context.Background(), "lsp")
}

func ensureRAGServices(logger *logrus.Logger) error {
	if globalSvcOrchestrator == nil {
		if err := initServiceOrchestrator(logger); err != nil {
			return err
		}
	}

	logger.Info("Starting RAG services via unified orchestrator")
	return globalSvcOrchestrator.StartService(context.Background(), "rag")
}

func startAllInfrastructure(logger *logrus.Logger) {
	if globalSvcOrchestrator == nil {
		if err := initServiceOrchestrator(logger); err != nil {
			logger.WithError(err).Warn("Failed to initialize service orchestrator")
			return
		}
	}

	go func() {
		logger.Info("Starting all infrastructure services via unified orchestrator...")
		if err := globalSvcOrchestrator.StartAll(context.Background()); err != nil {
			logger.WithError(err).Warn("Some infrastructure services failed to start")
		} else {
			logger.Info("All infrastructure services started successfully")
		}
	}()
}

func waitForInfrastructure(logger *logrus.Logger, timeout time.Duration) error {
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(timeout)

	services := map[string]string{
		"LSP Manager": "http://localhost:5100/health",
		"RAG Manager": "http://localhost:8030/health",
		"Qdrant":      "http://localhost:6333/readyz",
	}

	for name, url := range services {
		for time.Now().Before(deadline) {
			resp, err := client.Get(url)
			if err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					logger.WithField("service", name).Debug("Service is healthy")
					break
				}
			}
			time.Sleep(2 * time.Second)
		}
	}

	return nil
}

type InfrastructureStatus struct {
	Core         map[string]bool `json:"core"`
	MCP          map[string]bool `json:"mcp"`
	LSP          map[string]bool `json:"lsp"`
	RAG          map[string]bool `json:"rag"`
	TotalUp      int             `json:"total_up"`
	TotalDown    int             `json:"total_down"`
	AllHealthy   bool            `json:"all_healthy"`
	RemoteMode   bool            `json:"remote_mode"`
	ServiceCount int             `json:"service_count"`
}

func GetInfrastructureStatus(logger *logrus.Logger) *InfrastructureStatus {
	client := &http.Client{Timeout: 2 * time.Second}
	status := &InfrastructureStatus{
		Core: make(map[string]bool),
		MCP:  make(map[string]bool),
		LSP:  make(map[string]bool),
		RAG:  make(map[string]bool),
	}

	if globalSvcOrchestrator != nil {
		status.RemoteMode = globalSvcOrchestrator.IsRemoteEnabled()
		status.ServiceCount = globalSvcOrchestrator.ServiceCount()
	}

	// Endpoints resolve from the environment (§11.4.111) so this status probe
	// agrees with the boot-time dependency verification in main.go rather than
	// reporting against a different, hardcoded pair of ports.
	coreServices := map[string]string{
		"postgres": "",
		"redis":    "",
		"chromadb": dependencyURL("HELIXAGENT_PORT_CHROMADB", "8001", "/api/v2/heartbeat"),
		"cognee":   dependencyURL("HELIXAGENT_PORT_COGNEE", "8000", "/"),
	}

	for name, url := range coreServices {
		if url == "" {
			status.Core[name] = true
			status.TotalUp++
		} else {
			resp, err := client.Get(url)
			if err == nil && resp.StatusCode == http.StatusOK {
				status.Core[name] = true
				status.TotalUp++
			} else {
				status.Core[name] = false
				status.TotalDown++
			}
			if resp != nil {
				_ = resp.Body.Close()
			}
		}
	}

	mcpPorts := map[string]int{
		"filesystem": 9101,
		"memory":     9102,
		"postgres":   9103,
	}
	for name, port := range mcpPorts {
		if checkTCPPort("localhost", port) {
			status.MCP[name] = true
			status.TotalUp++
		} else {
			status.MCP[name] = false
			status.TotalDown++
		}
	}

	lspServices := map[string]string{
		"lsp-manager": "http://localhost:5100/health",
	}
	for name, url := range lspServices {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			status.LSP[name] = true
			status.TotalUp++
		} else {
			status.LSP[name] = false
			status.TotalDown++
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
	}

	ragServices := map[string]string{
		"qdrant":      "http://localhost:6333/readyz",
		"rag-manager": "http://localhost:8030/health",
	}
	for name, url := range ragServices {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			status.RAG[name] = true
			status.TotalUp++
		} else {
			status.RAG[name] = false
			status.TotalDown++
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
	}

	status.AllHealthy = status.TotalDown == 0
	return status
}

func checkTCPPort(host string, port int) bool {
	if globalContainerAdapter != nil {
		return globalContainerAdapter.HealthCheckTCP(host, port)
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
