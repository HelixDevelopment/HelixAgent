// Package mcp provides MCP server registry and management functionality.
package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
)

// ServerConfig represents the server.json configuration.
type ServerConfig struct {
	Schema      string      `json:"$schema"`
	Name        string      `json:"name"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Version     string      `json:"version"`
	Repository  *Repository `json:"repository,omitempty"`
	Packages    []Package   `json:"packages"`
	Meta        *ServerMeta `json:"_meta,omitempty"`
}

// Repository represents the source repository.
type Repository struct {
	URL       string `json:"url"`
	Source    string `json:"source"`
	Subfolder string `json:"subfolder,omitempty"`
}

// Package represents a package configuration.
type Package struct {
	RegistryType string     `json:"registryType"`
	Identifier   string     `json:"identifier"`
	Version      string     `json:"version"`
	Transport    *Transport `json:"transport"`
}

// Transport represents the transport configuration.
type Transport struct {
	Type string `json:"type"`
}

// ServerMeta contains marketplace metadata.
type ServerMeta struct {
	Marketplace *MarketplaceMeta `json:"io.claudecodeplugins/marketplace,omitempty"`
}

// MarketplaceMeta contains marketplace-specific info.
type MarketplaceMeta struct {
	Category   string `json:"category"`
	Featured   bool   `json:"featured"`
	WebsiteURL string `json:"websiteUrl"`
}

// MCPServer represents a registered MCP server.
type MCPServer struct {
	Config       *ServerConfig `json:"config"`
	Path         string        `json:"path"`
	Enabled      bool          `json:"enabled"`
	Status       ServerStatus  `json:"status"`
	RegisteredAt time.Time     `json:"registered_at"`
}

// ServerStatus represents the status of an MCP server.
type ServerStatus string

const (
	ServerStatusUnknown   ServerStatus = "unknown"
	ServerStatusAvailable ServerStatus = "available"
	ServerStatusStarting  ServerStatus = "starting"
	ServerStatusRunning   ServerStatus = "running"
	ServerStatusStopped   ServerStatus = "stopped"
	ServerStatusError     ServerStatus = "error"
)

// ServerRegistry manages MCP servers.
//
// Concurrent-safe by construction (CONST-029): servers is a safe.Store.
// Field mutations on *MCPServer (Enabled, Status) route through Update;
// readers receive the live pointer (consistent with pre-migration
// behaviour where readers held only an RLock and could observe field
// updates between RUnlock and the subsequent field-access).
type ServerRegistry struct {
	servers    *safe.Store[string, *MCPServer]
	log        *logrus.Logger
	serversDir string
}

// NewServerRegistry creates a new MCP server registry.
func NewServerRegistry(serversDir string) *ServerRegistry {
	return &ServerRegistry{
		servers:    safe.NewStore[string, *MCPServer](),
		log:        logrus.New(),
		serversDir: serversDir,
	}
}

// SetLogger sets the logger.
func (r *ServerRegistry) SetLogger(log *logrus.Logger) {
	r.log = log
}

// LoadServers loads all MCP servers from the servers directory.
func (r *ServerRegistry) LoadServers() (int, error) {
	if r.serversDir == "" {
		return 0, fmt.Errorf("servers directory not configured")
	}

	if _, err := os.Stat(r.serversDir); os.IsNotExist(err) {
		return 0, fmt.Errorf("servers directory not found: %s", r.serversDir)
	}

	count := 0
	entries, err := os.ReadDir(r.serversDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read servers directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		serverPath := filepath.Join(r.serversDir, entry.Name())
		configPath := filepath.Join(serverPath, "server.json")

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			// Try .mcp.json as fallback
			configPath = filepath.Join(serverPath, ".mcp.json")
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				continue
			}
		}

		config, err := r.loadServerConfig(configPath)
		if err != nil {
			r.log.WithError(err).WithField("path", configPath).Warn("Failed to load server config")
			continue
		}

		server := &MCPServer{
			Config:       config,
			Path:         serverPath,
			Enabled:      true,
			Status:       ServerStatusAvailable,
			RegisteredAt: time.Now(),
		}

		r.servers.Put(config.Name, server)

		count++
		r.log.WithFields(logrus.Fields{
			"name":  config.Name,
			"title": config.Title,
		}).Debug("Loaded MCP server")
	}

	r.log.WithField("count", count).Info("Loaded MCP servers")
	return count, nil
}

// loadServerConfig loads a server configuration from a JSON file.
func (r *ServerRegistry) loadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ServerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// Get returns a server by name.
func (r *ServerRegistry) Get(name string) (*MCPServer, bool) {
	return r.servers.Get(name)
}

// GetAll returns all registered servers.
func (r *ServerRegistry) GetAll() []*MCPServer {
	return r.servers.Values()
}

// GetEnabled returns all enabled servers.
func (r *ServerRegistry) GetEnabled() []*MCPServer {
	servers := make([]*MCPServer, 0)
	r.servers.Range(func(_ string, server *MCPServer) bool {
		if server.Enabled {
			servers = append(servers, server)
		}
		return true
	})
	return servers
}

// GetByStatus returns servers with the given status.
func (r *ServerRegistry) GetByStatus(status ServerStatus) []*MCPServer {
	servers := make([]*MCPServer, 0)
	r.servers.Range(func(_ string, server *MCPServer) bool {
		if server.Status == status {
			servers = append(servers, server)
		}
		return true
	})
	return servers
}

// Enable enables a server.
func (r *ServerRegistry) Enable(name string) error {
	return r.mutateServer(name, func(s *MCPServer) { s.Enabled = true })
}

// Disable disables a server.
func (r *ServerRegistry) Disable(name string) error {
	return r.mutateServer(name, func(s *MCPServer) { s.Enabled = false })
}

// UpdateStatus updates a server's status.
func (r *ServerRegistry) UpdateStatus(name string, status ServerStatus) error {
	return r.mutateServer(name, func(s *MCPServer) { s.Status = status })
}

// mutateServer atomically applies fn to the named server. Returns an error
// if the server is not registered.
func (r *ServerRegistry) mutateServer(name string, fn func(*MCPServer)) error {
	var notFound bool
	r.servers.Update(name, func(s *MCPServer, ok bool) (*MCPServer, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		fn(s)
		return s, true
	})
	if notFound {
		return fmt.Errorf("server not found: %s", name)
	}
	return nil
}

// Register adds a new server to the registry.
func (r *ServerRegistry) Register(server *MCPServer) error {
	if server.Config == nil || server.Config.Name == "" {
		return fmt.Errorf("server config or name is empty")
	}
	server.RegisteredAt = time.Now()
	if _, stored := r.servers.PutIfAbsent(server.Config.Name, server); !stored {
		return fmt.Errorf("server already registered: %s", server.Config.Name)
	}
	return nil
}

// Remove removes a server from the registry.
func (r *ServerRegistry) Remove(name string) error {
	if _, ok := r.servers.Delete(name); !ok {
		return fmt.Errorf("server not found: %s", name)
	}
	return nil
}

// Count returns the number of registered servers.
func (r *ServerRegistry) Count() int {
	return r.servers.Len()
}

// Stats returns registry statistics.
type RegistryStats struct {
	TotalServers int                  `json:"total_servers"`
	EnabledCount int                  `json:"enabled_count"`
	StatusCounts map[ServerStatus]int `json:"status_counts"`
	ServerList   []ServerInfo         `json:"server_list"`
}

// ServerInfo provides basic info about a server.
type ServerInfo struct {
	Name        string       `json:"name"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Version     string       `json:"version"`
	Status      ServerStatus `json:"status"`
	Enabled     bool         `json:"enabled"`
	Featured    bool         `json:"featured"`
}

// Stats returns statistics about registered servers.
func (r *ServerRegistry) Stats() *RegistryStats {
	stats := &RegistryStats{
		StatusCounts: make(map[ServerStatus]int),
		ServerList:   make([]ServerInfo, 0),
	}

	r.servers.Range(func(_ string, server *MCPServer) bool {
		stats.TotalServers++
		if server.Enabled {
			stats.EnabledCount++
		}
		stats.StatusCounts[server.Status]++

		featured := false
		if server.Config.Meta != nil && server.Config.Meta.Marketplace != nil {
			featured = server.Config.Meta.Marketplace.Featured
		}

		stats.ServerList = append(stats.ServerList, ServerInfo{
			Name:        server.Config.Name,
			Title:       server.Config.Title,
			Description: server.Config.Description,
			Version:     server.Config.Version,
			Status:      server.Status,
			Enabled:     server.Enabled,
			Featured:    featured,
		})
		return true
	})

	return stats
}

// ToMCPConfig generates MCP configuration for client use.
func (r *ServerRegistry) ToMCPConfig() map[string]interface{} {
	mcpServers := make(map[string]interface{})
	r.servers.Range(func(_ string, server *MCPServer) bool {
		if !server.Enabled || len(server.Config.Packages) == 0 {
			return true
		}

		pkg := server.Config.Packages[0]
		serverConfig := map[string]interface{}{
			"command": "npx",
			"args":    []string{"-y", pkg.Identifier},
		}

		if pkg.Transport != nil {
			serverConfig["transport"] = pkg.Transport.Type
		}

		// Use short name as key
		shortName := filepath.Base(server.Path)
		mcpServers[shortName] = serverConfig
		return true
	})

	return map[string]interface{}{
		"mcpServers": mcpServers,
	}
}
