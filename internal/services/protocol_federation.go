package services

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ProtocolDiscovery provides automatic discovery of protocol servers
type ProtocolDiscovery struct {
	discoveredServers map[string]*DiscoveredServer
	discoveryMethods  []DiscoveryMethod
	mu                sync.RWMutex
	logger            *logrus.Logger
	stopChan          chan struct{}
}

// DiscoveredServer represents a discovered protocol server
type DiscoveredServer struct {
	ID           string
	Protocol     string
	Address      string
	Port         int
	Name         string
	Type         string
	Capabilities map[string]interface{}
	LastSeen     time.Time
	Status       ServerStatus
}

// ServerStatus represents the status of a discovered server
type ServerStatus int

const (
	StatusUnknown ServerStatus = iota
	StatusOnline
	StatusOffline
	StatusError
)

// DiscoveryMethod defines an interface for server discovery
type DiscoveryMethod interface {
	Name() string
	Discover(ctx context.Context) ([]*DiscoveredServer, error)
	Start(ctx context.Context) error
	Stop() error
}

// NetworkDiscovery implements network-based discovery (UDP broadcasts, mDNS)
type NetworkDiscovery struct {
	port     int
	protocol string
	services map[string]*DiscoveredServer
	logger   *logrus.Logger
}

// DNSDiscovery implements DNS-based service discovery
type DNSDiscovery struct {
	domain   string
	services map[string]*DiscoveredServer
	logger   *logrus.Logger
}

// ConfigurationDiscovery implements configuration-based discovery
type ConfigurationDiscovery struct {
	config   map[string]interface{}
	services map[string]*DiscoveredServer
	logger   *logrus.Logger
}

// NewProtocolDiscovery creates a new protocol discovery service
func NewProtocolDiscovery(logger *logrus.Logger) *ProtocolDiscovery {
	discovery := &ProtocolDiscovery{
		discoveredServers: make(map[string]*DiscoveredServer),
		discoveryMethods:  []DiscoveryMethod{},
		stopChan:          make(chan struct{}),
		logger:            logger,
	}

	// Add default discovery methods
	discovery.AddDiscoveryMethod(&NetworkDiscovery{
		port:     9999,
		protocol: "udp",
		services: make(map[string]*DiscoveredServer),
		logger:   logger,
	})

	discovery.AddDiscoveryMethod(&DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   logger,
	})

	discovery.AddDiscoveryMethod(&ConfigurationDiscovery{
		config:   make(map[string]interface{}),
		services: make(map[string]*DiscoveredServer),
		logger:   logger,
	})

	return discovery
}

// AddDiscoveryMethod adds a discovery method
func (d *ProtocolDiscovery) AddDiscoveryMethod(method DiscoveryMethod) {
	d.discoveryMethods = append(d.discoveryMethods, method)
}

// Start begins the discovery process
func (d *ProtocolDiscovery) Start(ctx context.Context) error {
	d.logger.Info("Starting protocol discovery")

	for _, method := range d.discoveryMethods {
		if err := method.Start(ctx); err != nil {
			d.logger.WithError(err).WithField("method", method.Name()).Warn("Failed to start discovery method")
		}
	}

	// Start periodic discovery
	go d.periodicDiscovery()

	return nil
}

// Stop stops the discovery process
func (d *ProtocolDiscovery) Stop() {
	d.logger.Info("Stopping protocol discovery")

	close(d.stopChan)

	for _, method := range d.discoveryMethods {
		if err := method.Stop(); err != nil {
			d.logger.WithError(err).WithField("method", method.Name()).Warn("Failed to stop discovery method")
		}
	}
}

// DiscoverServers performs a discovery scan
func (d *ProtocolDiscovery) DiscoverServers(ctx context.Context) error {
	d.logger.Info("Performing protocol server discovery")

	for _, method := range d.discoveryMethods {
		servers, err := method.Discover(ctx)
		if err != nil {
			d.logger.WithError(err).WithField("method", method.Name()).Warn("Discovery method failed")
			continue
		}

		for _, server := range servers {
			d.addOrUpdateServer(server)
		}
	}

	d.logger.WithField("totalServers", len(d.discoveredServers)).Info("Discovery scan completed")
	return nil
}

// GetDiscoveredServers returns all discovered servers
func (d *ProtocolDiscovery) GetDiscoveredServers() []*DiscoveredServer {
	d.mu.RLock()
	defer d.mu.RUnlock()

	servers := make([]*DiscoveredServer, 0, len(d.discoveredServers))
	for _, server := range d.discoveredServers {
		servers = append(servers, server)
	}

	return servers
}

// GetServersByProtocol returns servers for a specific protocol
func (d *ProtocolDiscovery) GetServersByProtocol(protocol string) []*DiscoveredServer {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var servers []*DiscoveredServer
	for _, server := range d.discoveredServers {
		if server.Protocol == protocol {
			servers = append(servers, server)
		}
	}

	return servers
}

// GetServerByID returns a server by ID
func (d *ProtocolDiscovery) GetServerByID(serverID string) (*DiscoveredServer, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	server, exists := d.discoveredServers[serverID]
	if !exists {
		return nil, fmt.Errorf("server %s not found", serverID)
	}

	return server, nil
}

// RegisterServer manually registers a server
func (d *ProtocolDiscovery) RegisterServer(protocol, address string, port int, name string) error {
	server := &DiscoveredServer{
		ID:           fmt.Sprintf("%s-%s-%d", protocol, address, port),
		Protocol:     protocol,
		Address:      address,
		Port:         port,
		Name:         name,
		Type:         "manual",
		Status:       StatusOnline,
		LastSeen:     time.Now(),
		Capabilities: make(map[string]interface{}),
	}

	d.addOrUpdateServer(server)
	d.logger.WithFields(logrus.Fields{
		"serverId": server.ID,
		"protocol": protocol,
		"address":  address,
		"port":     port,
	}).Info("Server manually registered")

	return nil
}

// UnregisterServer removes a server
func (d *ProtocolDiscovery) UnregisterServer(serverID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.discoveredServers[serverID]; !exists {
		return fmt.Errorf("server %s not found", serverID)
	}

	delete(d.discoveredServers, serverID)
	d.logger.WithField("serverId", serverID).Info("Server unregistered")

	return nil
}

// HealthCheck performs health checks on discovered servers
func (d *ProtocolDiscovery) HealthCheck(ctx context.Context) error {
	d.mu.RLock()
	servers := make(map[string]*DiscoveredServer)
	for k, v := range d.discoveredServers {
		servers[k] = v
	}
	d.mu.RUnlock()

	for serverID, server := range servers {
		status := d.checkServerHealth(ctx, server)
		d.updateServerStatus(serverID, status)
	}

	return nil
}

// Private methods

func (d *ProtocolDiscovery) addOrUpdateServer(server *DiscoveredServer) {
	d.mu.Lock()
	defer d.mu.Unlock()

	existing, exists := d.discoveredServers[server.ID]
	if exists {
		// Update existing server
		existing.LastSeen = time.Now()
		existing.Status = server.Status
		existing.Capabilities = server.Capabilities
	} else {
		// Add new server
		d.discoveredServers[server.ID] = server
		d.logger.WithFields(logrus.Fields{
			"serverId": server.ID,
			"protocol": server.Protocol,
			"address":  server.Address,
		}).Info("New server discovered")
	}
}

func (d *ProtocolDiscovery) updateServerStatus(serverID string, status ServerStatus) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if server, exists := d.discoveredServers[serverID]; exists {
		server.Status = status
		server.LastSeen = time.Now()
	}
}

func (d *ProtocolDiscovery) checkServerHealth(ctx context.Context, server *DiscoveredServer) ServerStatus {
	timeout := 5 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	address := fmt.Sprintf("%s:%d", server.Address, server.Port)

	switch server.Protocol {
	case "mcp":
		// For MCP, try to connect via stdio or network
		conn, err := net.DialTimeout("tcp", address, timeout)
		if err != nil {
			return StatusOffline
		}
		conn.Close()
		return StatusOnline

	case "lsp":
		// LSP servers typically listen on TCP
		conn, err := net.DialTimeout("tcp", address, timeout)
		if err != nil {
			return StatusOffline
		}
		conn.Close()
		return StatusOnline

	case "acp":
		// ACP agents might use WebSocket or HTTP
		// Try TCP connection first
		conn, err := net.DialTimeout("tcp", address, timeout)
		if err != nil {
			return StatusOffline
		}
		conn.Close()
		return StatusOnline

	default:
		return StatusUnknown
	}
}

func (d *ProtocolDiscovery) periodicDiscovery() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopChan:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			d.DiscoverServers(ctx)
			d.HealthCheck(ctx)
			cancel()
		}
	}
}

// NetworkDiscovery implementation

func (n *NetworkDiscovery) Name() string {
	return "network"
}

func (n *NetworkDiscovery) Discover(ctx context.Context) ([]*DiscoveredServer, error) {
	// UDP broadcast discovery
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", n.port))
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Send discovery broadcast
	broadcastAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("255.255.255.255:%d", n.port))
	message := []byte("DISCOVER_PROTOCOL_SERVERS")
	conn.WriteToUDP(message, broadcastAddr)

	// Listen for responses with timeout
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	var servers []*DiscoveredServer
	buffer := make([]byte, 1024)

	for {
		_, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break // Timeout, stop listening
			}
			continue
		}

		response := string(buffer)
		if strings.HasPrefix(response, "PROTOCOL_SERVER:") {
			parts := strings.Split(response, ":")
			if len(parts) >= 4 {
				port, _ := strconv.Atoi(parts[3])
				server := &DiscoveredServer{
					ID:       fmt.Sprintf("net-%s-%s", remoteAddr.IP.String(), parts[1]),
					Protocol: parts[1],
					Address:  remoteAddr.IP.String(),
					Port:     port,
					Name:     parts[2],
					Type:     "network",
					Status:   StatusOnline,
					LastSeen: time.Now(),
				}
				servers = append(servers, server)
			}
		}
	}

	return servers, nil
}

func (n *NetworkDiscovery) Start(ctx context.Context) error {
	n.logger.Info("Starting network discovery")
	return nil
}

func (n *NetworkDiscovery) Stop() error {
	n.logger.Info("Stopping network discovery")
	return nil
}

// DNSDiscovery implementation

func (dns *DNSDiscovery) Name() string {
	return "dns"
}

func (dns *DNSDiscovery) Discover(ctx context.Context) ([]*DiscoveredServer, error) {
	var servers []*DiscoveredServer

	// Look for common protocol service names
	serviceNames := []string{
		"_mcp._tcp",
		"_lsp._tcp",
		"_acp._tcp",
		"_protocols._tcp",
	}

	for _, serviceName := range serviceNames {
		protocol := strings.TrimSuffix(strings.TrimPrefix(serviceName, "_"), "._tcp")

		// In a real implementation, you would use DNS-SD/mDNS discovery
		// For this demo, we'll simulate finding some services
		if protocol == "mcp" {
			server := &DiscoveredServer{
				ID:       fmt.Sprintf("dns-mcp-%s", dns.domain),
				Protocol: "mcp",
				Address:  "localhost",
				Port:     3000,
				Name:     "MCP Server",
				Type:     "dns",
				Status:   StatusOnline,
				LastSeen: time.Now(),
			}
			servers = append(servers, server)
		}
	}

	return servers, nil
}

func (dns *DNSDiscovery) Start(ctx context.Context) error {
	dns.logger.Info("Starting DNS discovery")
	return nil
}

func (dns *DNSDiscovery) Stop() error {
	dns.logger.Info("Stopping DNS discovery")
	return nil
}

// ConfigurationDiscovery implementation

func (c *ConfigurationDiscovery) Name() string {
	return "config"
}

func (c *ConfigurationDiscovery) Discover(ctx context.Context) ([]*DiscoveredServer, error) {
	var servers []*DiscoveredServer

	// Read from configuration
	if mcpConfig, ok := c.config["mcp"].(map[string]interface{}); ok {
		if serversConfig, ok := mcpConfig["servers"].([]interface{}); ok {
			for _, serverConfig := range serversConfig {
				if serverMap, ok := serverConfig.(map[string]interface{}); ok {
					server := &DiscoveredServer{
						ID:       fmt.Sprintf("config-mcp-%s", serverMap["name"]),
						Protocol: "mcp",
						Address:  "localhost", // Default
						Port:     3000,        // Default
						Name:     serverMap["name"].(string),
						Type:     "config",
						Status:   StatusOnline,
						LastSeen: time.Now(),
					}
					servers = append(servers, server)
				}
			}
		}
	}

	return servers, nil
}

func (c *ConfigurationDiscovery) Start(ctx context.Context) error {
	c.logger.Info("Starting configuration discovery")
	return nil
}

func (c *ConfigurationDiscovery) Stop() error {
	c.logger.Info("Stopping configuration discovery")
	return nil
}
