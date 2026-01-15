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

// DNSResolver defines the interface for DNS lookups (allows mocking in tests)
type DNSResolver interface {
	LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error)
	LookupHost(ctx context.Context, host string) ([]string, error)
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

// DefaultDNSResolver uses the standard library net package for DNS lookups
type DefaultDNSResolver struct{}

// LookupSRV performs a DNS SRV lookup using the standard library
func (d *DefaultDNSResolver) LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error) {
	return net.DefaultResolver.LookupSRV(ctx, service, proto, name)
}

// LookupHost performs a DNS host lookup using the standard library
func (d *DefaultDNSResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, host)
}

// LookupTXT performs a DNS TXT record lookup using the standard library
func (d *DefaultDNSResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return net.DefaultResolver.LookupTXT(ctx, name)
}

// DNSDiscovery implements DNS-based service discovery using DNS-SD (RFC 6763)
type DNSDiscovery struct {
	domain   string
	services map[string]*DiscoveredServer
	logger   *logrus.Logger
	resolver DNSResolver
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
		resolver: &DefaultDNSResolver{},
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

	address := net.JoinHostPort(server.Address, fmt.Sprintf("%d", server.Port))

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

// SetResolver sets a custom DNS resolver (primarily for testing)
func (dns *DNSDiscovery) SetResolver(resolver DNSResolver) {
	dns.resolver = resolver
}

// Discover performs DNS-SD service discovery using SRV records (RFC 6763)
func (dns *DNSDiscovery) Discover(ctx context.Context) ([]*DiscoveredServer, error) {
	servers := make([]*DiscoveredServer, 0)

	// Ensure we have a resolver
	if dns.resolver == nil {
		dns.resolver = &DefaultDNSResolver{}
	}

	// Protocol service definitions: service name -> protocol identifier
	serviceDefinitions := []struct {
		service  string // DNS-SD service name (without leading underscore)
		proto    string // Protocol (tcp/udp)
		protocol string // Our internal protocol name
	}{
		{service: "mcp", proto: "tcp", protocol: "mcp"},
		{service: "lsp", proto: "tcp", protocol: "lsp"},
		{service: "acp", proto: "tcp", protocol: "acp"},
		{service: "protocols", proto: "tcp", protocol: "protocols"},
	}

	for _, svcDef := range serviceDefinitions {
		discovered, err := dns.discoverService(ctx, svcDef.service, svcDef.proto, svcDef.protocol)
		if err != nil {
			// Log the error but continue with other services
			if dns.logger != nil {
				dns.logger.WithError(err).WithFields(logrus.Fields{
					"service":  svcDef.service,
					"protocol": svcDef.protocol,
					"domain":   dns.domain,
				}).Debug("DNS-SD lookup failed for service")
			}
			continue
		}
		servers = append(servers, discovered...)
	}

	return servers, nil
}

// discoverService performs DNS-SD discovery for a specific service type
func (dns *DNSDiscovery) discoverService(ctx context.Context, service, proto, protocol string) ([]*DiscoveredServer, error) {
	var servers []*DiscoveredServer

	// Perform SRV lookup: _service._proto.domain
	// For example: _mcp._tcp.local
	_, srvRecords, err := dns.resolver.LookupSRV(ctx, service, proto, dns.domain)
	if err != nil {
		return nil, fmt.Errorf("SRV lookup failed: %w", err)
	}

	for _, srv := range srvRecords {
		// Resolve the target hostname to IP addresses
		addresses, err := dns.resolveHost(ctx, srv.Target)
		if err != nil {
			if dns.logger != nil {
				dns.logger.WithError(err).WithFields(logrus.Fields{
					"target": srv.Target,
					"port":   srv.Port,
				}).Debug("Failed to resolve SRV target host")
			}
			// Use the hostname as-is if resolution fails
			addresses = []string{strings.TrimSuffix(srv.Target, ".")}
		}

		// Try to get additional metadata from TXT records
		metadata := dns.getServiceMetadata(ctx, service, proto, srv.Target)

		for _, addr := range addresses {
			serverID := fmt.Sprintf("dns-%s-%s-%d", protocol, addr, srv.Port)
			serverName := metadata["name"]
			if serverName == "" {
				serverName = fmt.Sprintf("%s server at %s", strings.ToUpper(protocol), srv.Target)
			}

			server := &DiscoveredServer{
				ID:           serverID,
				Protocol:     protocol,
				Address:      addr,
				Port:         int(srv.Port),
				Name:         serverName,
				Type:         "dns-sd",
				Status:       StatusUnknown, // Status will be updated by health check
				LastSeen:     time.Now(),
				Capabilities: dns.parseCapabilities(metadata),
			}
			servers = append(servers, server)
		}
	}

	return servers, nil
}

// resolveHost resolves a hostname to IP addresses
func (dns *DNSDiscovery) resolveHost(ctx context.Context, host string) ([]string, error) {
	// Remove trailing dot from FQDN if present
	host = strings.TrimSuffix(host, ".")

	// Check if it's already an IP address
	if ip := net.ParseIP(host); ip != nil {
		return []string{host}, nil
	}

	return dns.resolver.LookupHost(ctx, host)
}

// getServiceMetadata retrieves additional service metadata from TXT records
func (dns *DNSDiscovery) getServiceMetadata(ctx context.Context, service, proto, target string) map[string]string {
	metadata := make(map[string]string)

	// TXT record name for DNS-SD: target or _service._proto.domain
	txtName := strings.TrimSuffix(target, ".")

	txtRecords, err := dns.resolver.LookupTXT(ctx, txtName)
	if err != nil {
		// TXT records are optional, ignore errors
		return metadata
	}

	// Parse TXT records in key=value format (per DNS-SD RFC 6763)
	for _, txt := range txtRecords {
		if idx := strings.Index(txt, "="); idx > 0 {
			key := txt[:idx]
			value := txt[idx+1:]
			metadata[key] = value
		}
	}

	return metadata
}

// parseCapabilities converts TXT metadata to capabilities map
func (dns *DNSDiscovery) parseCapabilities(metadata map[string]string) map[string]interface{} {
	capabilities := make(map[string]interface{})

	// Known capability keys that should be parsed
	capabilityKeys := []string{"version", "capabilities", "features", "api"}

	for _, key := range capabilityKeys {
		if value, ok := metadata[key]; ok {
			capabilities[key] = value
		}
	}

	// Copy any remaining metadata as capabilities
	for key, value := range metadata {
		if _, exists := capabilities[key]; !exists && key != "name" {
			capabilities[key] = value
		}
	}

	return capabilities
}

func (dns *DNSDiscovery) Start(ctx context.Context) error {
	if dns.logger != nil {
		dns.logger.Info("Starting DNS-SD discovery")
	}
	return nil
}

func (dns *DNSDiscovery) Stop() error {
	if dns.logger != nil {
		dns.logger.Info("Stopping DNS-SD discovery")
	}
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
