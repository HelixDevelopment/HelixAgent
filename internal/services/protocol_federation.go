package services

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
)

// ProtocolDiscovery provides automatic discovery of protocol servers.
//
// CONST-029: both the server table and the method list are now
// concurrent-safe containers. `discoveryMethods` is append-only in
// practice, but we route it through *safe.Slice so concurrent
// AddDiscoveryMethod + Start races become impossible rather than
// review-caught.
type ProtocolDiscovery struct {
	discoveredServers *safe.Store[string, *DiscoveredServer]
	discoveryMethods  *safe.Slice[DiscoveryMethod]
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
		discoveredServers: safe.NewStore[string, *DiscoveredServer](),
		discoveryMethods:  safe.NewSlice[DiscoveryMethod](),
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
	d.discoveryMethods.Append(method)
}

// Start begins the discovery process
func (d *ProtocolDiscovery) Start(ctx context.Context) error {
	d.logger.Info("Starting protocol discovery")

	for _, method := range d.discoveryMethods.Snapshot() {
		if err := method.Start(ctx); err != nil {
			d.logger.WithError(err).WithField("method", method.Name()).Warn("Failed to start discovery method")
		}
	}

	// Start periodic discovery
	//nolint:gosec // G118: long-lived maintenance loop, intentionally decoupled from the caller's context
	go d.periodicDiscovery()

	return nil
}

// Stop stops the discovery process
func (d *ProtocolDiscovery) Stop() {
	d.logger.Info("Stopping protocol discovery")

	close(d.stopChan)

	for _, method := range d.discoveryMethods.Snapshot() {
		if err := method.Stop(); err != nil {
			d.logger.WithError(err).WithField("method", method.Name()).Warn("Failed to stop discovery method")
		}
	}
}

// DiscoverServers performs a discovery scan
func (d *ProtocolDiscovery) DiscoverServers(ctx context.Context) error {
	d.logger.Info("Performing protocol server discovery")

	for _, method := range d.discoveryMethods.Snapshot() {
		servers, err := method.Discover(ctx)
		if err != nil {
			d.logger.WithError(err).WithField("method", method.Name()).Warn("Discovery method failed")
			continue
		}

		for _, server := range servers {
			d.addOrUpdateServer(server)
		}
	}

	d.logger.WithField("totalServers", d.discoveredServers.Len()).Info("Discovery scan completed")
	return nil
}

// GetDiscoveredServers returns all discovered servers
func (d *ProtocolDiscovery) GetDiscoveredServers() []*DiscoveredServer {
	return d.discoveredServers.Values()
}

// GetServersByProtocol returns servers for a specific protocol
func (d *ProtocolDiscovery) GetServersByProtocol(protocol string) []*DiscoveredServer {
	var servers []*DiscoveredServer
	d.discoveredServers.Range(func(_ string, server *DiscoveredServer) bool {
		if server.Protocol == protocol {
			servers = append(servers, server)
		}
		return true
	})
	return servers
}

// GetServerByID returns a server by ID
func (d *ProtocolDiscovery) GetServerByID(serverID string) (*DiscoveredServer, error) {
	server, exists := d.discoveredServers.Get(serverID)
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
	if _, existed := d.discoveredServers.Delete(serverID); !existed {
		return fmt.Errorf("server %s not found", serverID)
	}
	d.logger.WithField("serverId", serverID).Info("Server unregistered")
	return nil
}

// HealthCheck performs health checks on discovered servers
func (d *ProtocolDiscovery) HealthCheck(ctx context.Context) error {
	for serverID, server := range d.discoveredServers.Snapshot() {
		status := d.checkServerHealth(ctx, server)
		d.updateServerStatus(serverID, status)
	}
	return nil
}

// Private methods

func (d *ProtocolDiscovery) addOrUpdateServer(server *DiscoveredServer) {
	isNew := false
	d.discoveredServers.Update(server.ID, func(existing *DiscoveredServer, present bool) (*DiscoveredServer, bool) {
		if present {
			// Mutate in place — callers may hold references from Values()/
			// Snapshot() and expect the same *DiscoveredServer to reflect
			// updates. This preserves pre-migration semantics.
			existing.LastSeen = time.Now()
			existing.Status = server.Status
			existing.Capabilities = server.Capabilities
			return existing, true
		}
		isNew = true
		return server, true
	})

	if isNew {
		d.logger.WithFields(logrus.Fields{
			"serverId": server.ID,
			"protocol": server.Protocol,
			"address":  server.Address,
		}).Info("New server discovered")
	}
}

func (d *ProtocolDiscovery) updateServerStatus(serverID string, status ServerStatus) {
	d.discoveredServers.Update(serverID, func(server *DiscoveredServer, present bool) (*DiscoveredServer, bool) {
		if !present {
			return nil, false
		}
		server.Status = status
		server.LastSeen = time.Now()
		return server, true
	})
}

func (d *ProtocolDiscovery) checkServerHealth(ctx context.Context, server *DiscoveredServer) ServerStatus {
	timeout := 5 * time.Second
	_, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	address := net.JoinHostPort(server.Address, fmt.Sprintf("%d", server.Port))

	switch server.Protocol {
	case "mcp":
		// For MCP, try to connect via stdio or network
		conn, err := net.DialTimeout("tcp", address, timeout)
		if err != nil {
			return StatusOffline
		}
		_ = conn.Close()
		return StatusOnline

	case "lsp":
		// LSP servers typically listen on TCP
		conn, err := net.DialTimeout("tcp", address, timeout)
		if err != nil {
			return StatusOffline
		}
		_ = conn.Close()
		return StatusOnline

	case "acp":
		// ACP agents might use WebSocket or HTTP
		// Try TCP connection first
		conn, err := net.DialTimeout("tcp", address, timeout)
		if err != nil {
			return StatusOffline
		}
		_ = conn.Close()
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
			_ = d.DiscoverServers(ctx) //nolint:errcheck
			_ = d.HealthCheck(ctx)     //nolint:errcheck
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
	defer func() { _ = conn.Close() }()

	// Send discovery broadcast
	broadcastAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("255.255.255.255:%d", n.port)) //nolint:errcheck
	message := []byte("DISCOVER_PROTOCOL_SERVERS")
	_, _ = conn.WriteToUDP(message, broadcastAddr) //nolint:errcheck

	// Listen for responses with timeout
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck

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
				port, _ := strconv.Atoi(parts[3]) //nolint:errcheck
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
						//nolint:errcheck // schema validation ensures correct type
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
