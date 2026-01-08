package services

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockDNSResolver implements DNSResolver for testing
type MockDNSResolver struct {
	srvRecords    map[string][]*net.SRV
	hostRecords   map[string][]string
	txtRecords    map[string][]string
	srvErrors     map[string]error
	hostErrors    map[string]error
	txtErrors     map[string]error
	lookupSRVCall int
	lookupHostCall int
	lookupTXTCall int
}

// NewMockDNSResolver creates a new mock resolver
func NewMockDNSResolver() *MockDNSResolver {
	return &MockDNSResolver{
		srvRecords:  make(map[string][]*net.SRV),
		hostRecords: make(map[string][]string),
		txtRecords:  make(map[string][]string),
		srvErrors:   make(map[string]error),
		hostErrors:  make(map[string]error),
		txtErrors:   make(map[string]error),
	}
}

// AddSRVRecord adds an SRV record for a service
func (m *MockDNSResolver) AddSRVRecord(service, proto, domain string, target string, port uint16, priority, weight uint16) {
	key := m.srvKey(service, proto, domain)
	m.srvRecords[key] = append(m.srvRecords[key], &net.SRV{
		Target:   target,
		Port:     port,
		Priority: priority,
		Weight:   weight,
	})
}

// AddHostRecord adds a host (A/AAAA) record
func (m *MockDNSResolver) AddHostRecord(host string, addresses ...string) {
	m.hostRecords[host] = addresses
}

// AddTXTRecord adds TXT records for a name
func (m *MockDNSResolver) AddTXTRecord(name string, records ...string) {
	m.txtRecords[name] = records
}

// SetSRVError sets an error for SRV lookup
func (m *MockDNSResolver) SetSRVError(service, proto, domain string, err error) {
	key := m.srvKey(service, proto, domain)
	m.srvErrors[key] = err
}

// SetHostError sets an error for host lookup
func (m *MockDNSResolver) SetHostError(host string, err error) {
	m.hostErrors[host] = err
}

// SetTXTError sets an error for TXT lookup
func (m *MockDNSResolver) SetTXTError(name string, err error) {
	m.txtErrors[name] = err
}

func (m *MockDNSResolver) srvKey(service, proto, domain string) string {
	return "_" + service + "._" + proto + "." + domain
}

// LookupSRV implements DNSResolver.LookupSRV
func (m *MockDNSResolver) LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error) {
	m.lookupSRVCall++
	key := m.srvKey(service, proto, name)

	if err, ok := m.srvErrors[key]; ok {
		return "", nil, err
	}

	if records, ok := m.srvRecords[key]; ok {
		return key, records, nil
	}

	// Return no records (empty result, not an error)
	return key, nil, nil
}

// LookupHost implements DNSResolver.LookupHost
func (m *MockDNSResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	m.lookupHostCall++

	if err, ok := m.hostErrors[host]; ok {
		return nil, err
	}

	if addresses, ok := m.hostRecords[host]; ok {
		return addresses, nil
	}

	// Return error for unknown hosts
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

// LookupTXT implements DNSResolver.LookupTXT
func (m *MockDNSResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	m.lookupTXTCall++

	if err, ok := m.txtErrors[name]; ok {
		return nil, err
	}

	if records, ok := m.txtRecords[name]; ok {
		return records, nil
	}

	// TXT records are optional, return empty
	return nil, nil
}

func newDNSTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

func TestDNSDiscovery_Name(t *testing.T) {
	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
	}

	assert.Equal(t, "dns", dns.Name())
}

func TestDNSDiscovery_Discover_NoServices(t *testing.T) {
	mockResolver := NewMockDNSResolver()

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx := context.Background()
	servers, err := dns.Discover(ctx)

	require.NoError(t, err)
	assert.Empty(t, servers)
	// Should have attempted lookups for all service types
	assert.Equal(t, 4, mockResolver.lookupSRVCall)
}

func TestDNSDiscovery_Discover_SingleMCPService(t *testing.T) {
	mockResolver := NewMockDNSResolver()
	mockResolver.AddSRVRecord("mcp", "tcp", "local", "mcp-server.local.", 3000, 10, 100)
	mockResolver.AddHostRecord("mcp-server.local", "192.168.1.100")

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx := context.Background()
	servers, err := dns.Discover(ctx)

	require.NoError(t, err)
	require.Len(t, servers, 1)

	server := servers[0]
	assert.Equal(t, "mcp", server.Protocol)
	assert.Equal(t, "192.168.1.100", server.Address)
	assert.Equal(t, 3000, server.Port)
	assert.Equal(t, "dns-sd", server.Type)
	assert.Contains(t, server.Name, "MCP server")
}

func TestDNSDiscovery_Discover_MultipleServices(t *testing.T) {
	mockResolver := NewMockDNSResolver()

	// Add MCP service
	mockResolver.AddSRVRecord("mcp", "tcp", "local", "mcp-server.local.", 3000, 10, 100)
	mockResolver.AddHostRecord("mcp-server.local", "192.168.1.100")

	// Add LSP service
	mockResolver.AddSRVRecord("lsp", "tcp", "local", "lsp-server.local.", 6006, 10, 100)
	mockResolver.AddHostRecord("lsp-server.local", "192.168.1.101")

	// Add ACP service
	mockResolver.AddSRVRecord("acp", "tcp", "local", "acp-server.local.", 7061, 10, 100)
	mockResolver.AddHostRecord("acp-server.local", "192.168.1.102")

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx := context.Background()
	servers, err := dns.Discover(ctx)

	require.NoError(t, err)
	require.Len(t, servers, 3)

	// Check we found all protocols
	protocols := make(map[string]bool)
	for _, server := range servers {
		protocols[server.Protocol] = true
	}

	assert.True(t, protocols["mcp"])
	assert.True(t, protocols["lsp"])
	assert.True(t, protocols["acp"])
}

func TestDNSDiscovery_Discover_MultipleTargets(t *testing.T) {
	mockResolver := NewMockDNSResolver()

	// Add multiple SRV records for the same service (load balancing)
	mockResolver.AddSRVRecord("mcp", "tcp", "local", "mcp1.local.", 3000, 10, 100)
	mockResolver.AddSRVRecord("mcp", "tcp", "local", "mcp2.local.", 3001, 20, 100)
	mockResolver.AddHostRecord("mcp1.local", "192.168.1.100")
	mockResolver.AddHostRecord("mcp2.local", "192.168.1.101")

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx := context.Background()
	servers, err := dns.Discover(ctx)

	require.NoError(t, err)
	require.Len(t, servers, 2)

	// Check both servers were discovered
	addresses := make(map[string]int)
	for _, server := range servers {
		addresses[server.Address] = server.Port
	}

	assert.Equal(t, 3000, addresses["192.168.1.100"])
	assert.Equal(t, 3001, addresses["192.168.1.101"])
}

func TestDNSDiscovery_Discover_WithTXTMetadata(t *testing.T) {
	mockResolver := NewMockDNSResolver()

	mockResolver.AddSRVRecord("mcp", "tcp", "local", "mcp-server.local.", 3000, 10, 100)
	mockResolver.AddHostRecord("mcp-server.local", "192.168.1.100")
	mockResolver.AddTXTRecord("mcp-server.local", "name=My MCP Server", "version=1.2.3", "features=streaming,tools")

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx := context.Background()
	servers, err := dns.Discover(ctx)

	require.NoError(t, err)
	require.Len(t, servers, 1)

	server := servers[0]
	assert.Equal(t, "My MCP Server", server.Name)
	assert.Equal(t, "1.2.3", server.Capabilities["version"])
	assert.Equal(t, "streaming,tools", server.Capabilities["features"])
}

func TestDNSDiscovery_Discover_SRVLookupError(t *testing.T) {
	mockResolver := NewMockDNSResolver()

	// Set error for MCP lookup
	mockResolver.SetSRVError("mcp", "tcp", "local", errors.New("DNS server unreachable"))

	// But LSP should work
	mockResolver.AddSRVRecord("lsp", "tcp", "local", "lsp-server.local.", 6006, 10, 100)
	mockResolver.AddHostRecord("lsp-server.local", "192.168.1.101")

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx := context.Background()
	servers, err := dns.Discover(ctx)

	// Should not return error, just skip the failed service
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Equal(t, "lsp", servers[0].Protocol)
}

func TestDNSDiscovery_Discover_HostResolutionFailure(t *testing.T) {
	mockResolver := NewMockDNSResolver()

	mockResolver.AddSRVRecord("mcp", "tcp", "local", "mcp-server.local.", 3000, 10, 100)
	// Don't add host record - should fall back to hostname
	mockResolver.SetHostError("mcp-server.local", &net.DNSError{Err: "no such host", Name: "mcp-server.local"})

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx := context.Background()
	servers, err := dns.Discover(ctx)

	require.NoError(t, err)
	require.Len(t, servers, 1)

	// Should use hostname when resolution fails
	assert.Equal(t, "mcp-server.local", servers[0].Address)
}

func TestDNSDiscovery_Discover_MultipleAddressesPerHost(t *testing.T) {
	mockResolver := NewMockDNSResolver()

	mockResolver.AddSRVRecord("mcp", "tcp", "local", "mcp-server.local.", 3000, 10, 100)
	// Host has multiple IP addresses
	mockResolver.AddHostRecord("mcp-server.local", "192.168.1.100", "192.168.1.101", "10.0.0.100")

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx := context.Background()
	servers, err := dns.Discover(ctx)

	require.NoError(t, err)
	require.Len(t, servers, 3)

	// All servers should have same port but different addresses
	addresses := make(map[string]bool)
	for _, server := range servers {
		assert.Equal(t, 3000, server.Port)
		addresses[server.Address] = true
	}

	assert.True(t, addresses["192.168.1.100"])
	assert.True(t, addresses["192.168.1.101"])
	assert.True(t, addresses["10.0.0.100"])
}

func TestDNSDiscovery_Discover_IPAddressAsTarget(t *testing.T) {
	mockResolver := NewMockDNSResolver()

	// SRV record with IP address as target (unusual but valid)
	mockResolver.AddSRVRecord("mcp", "tcp", "local", "192.168.1.100", 3000, 10, 100)

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx := context.Background()
	servers, err := dns.Discover(ctx)

	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Equal(t, "192.168.1.100", servers[0].Address)
}

func TestDNSDiscovery_Discover_NilResolver(t *testing.T) {
	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: nil, // No resolver set
	}

	ctx := context.Background()
	// Should not panic, should use DefaultDNSResolver
	servers, err := dns.Discover(ctx)

	// Will likely return empty (no real DNS-SD services on localhost)
	// but should not error
	require.NoError(t, err)
	assert.NotNil(t, servers)
}

func TestDNSDiscovery_Discover_ContextCancellation(t *testing.T) {
	mockResolver := NewMockDNSResolver()
	mockResolver.AddSRVRecord("mcp", "tcp", "local", "mcp-server.local.", 3000, 10, 100)
	mockResolver.AddHostRecord("mcp-server.local", "192.168.1.100")

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// With cancelled context, lookups should still work in our mock
	// but in real implementation would fail
	servers, err := dns.Discover(ctx)
	require.NoError(t, err)
	assert.NotNil(t, servers)
}

func TestDNSDiscovery_SetResolver(t *testing.T) {
	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: &DefaultDNSResolver{},
	}

	mockResolver := NewMockDNSResolver()
	dns.SetResolver(mockResolver)

	ctx := context.Background()
	_, _ = dns.Discover(ctx)

	// Verify mock was used
	assert.True(t, mockResolver.lookupSRVCall > 0)
}

func TestDNSDiscovery_Start(t *testing.T) {
	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
	}

	ctx := context.Background()
	err := dns.Start(ctx)
	require.NoError(t, err)
}

func TestDNSDiscovery_Stop(t *testing.T) {
	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
	}

	err := dns.Stop()
	require.NoError(t, err)
}

func TestDNSDiscovery_Start_NilLogger(t *testing.T) {
	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   nil,
	}

	ctx := context.Background()
	err := dns.Start(ctx)
	require.NoError(t, err)
}

func TestDNSDiscovery_Stop_NilLogger(t *testing.T) {
	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   nil,
	}

	err := dns.Stop()
	require.NoError(t, err)
}

func TestDNSDiscovery_discoverService_Error(t *testing.T) {
	mockResolver := NewMockDNSResolver()
	mockResolver.SetSRVError("mcp", "tcp", "local", errors.New("lookup failed"))

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx := context.Background()
	servers, err := dns.discoverService(ctx, "mcp", "tcp", "mcp")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SRV lookup failed")
	assert.Nil(t, servers)
}

func TestDNSDiscovery_resolveHost_IPAddress(t *testing.T) {
	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: NewMockDNSResolver(),
	}

	ctx := context.Background()

	// Test IPv4
	addrs, err := dns.resolveHost(ctx, "192.168.1.100")
	require.NoError(t, err)
	assert.Equal(t, []string{"192.168.1.100"}, addrs)

	// Test IPv6
	addrs, err = dns.resolveHost(ctx, "::1")
	require.NoError(t, err)
	assert.Equal(t, []string{"::1"}, addrs)
}

func TestDNSDiscovery_resolveHost_TrailingDot(t *testing.T) {
	mockResolver := NewMockDNSResolver()
	mockResolver.AddHostRecord("myhost.local", "192.168.1.100")

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx := context.Background()
	addrs, err := dns.resolveHost(ctx, "myhost.local.")

	require.NoError(t, err)
	assert.Equal(t, []string{"192.168.1.100"}, addrs)
}

func TestDNSDiscovery_getServiceMetadata_NoRecords(t *testing.T) {
	mockResolver := NewMockDNSResolver()

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx := context.Background()
	metadata := dns.getServiceMetadata(ctx, "mcp", "tcp", "mcp-server.local.")

	assert.Empty(t, metadata)
}

func TestDNSDiscovery_getServiceMetadata_InvalidFormat(t *testing.T) {
	mockResolver := NewMockDNSResolver()
	// TXT record without = sign
	mockResolver.AddTXTRecord("mcp-server.local", "invalid-record", "another-invalid")

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx := context.Background()
	metadata := dns.getServiceMetadata(ctx, "mcp", "tcp", "mcp-server.local.")

	// Invalid records should be ignored
	assert.Empty(t, metadata)
}

func TestDNSDiscovery_getServiceMetadata_EmptyKey(t *testing.T) {
	mockResolver := NewMockDNSResolver()
	// TXT record with empty key
	mockResolver.AddTXTRecord("mcp-server.local", "=value", "valid=record")

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx := context.Background()
	metadata := dns.getServiceMetadata(ctx, "mcp", "tcp", "mcp-server.local.")

	// Empty key records should be ignored
	assert.Len(t, metadata, 1)
	assert.Equal(t, "record", metadata["valid"])
}

func TestDNSDiscovery_parseCapabilities(t *testing.T) {
	dns := &DNSDiscovery{}

	metadata := map[string]string{
		"name":         "Test Server",
		"version":      "1.0.0",
		"capabilities": "streaming,tools",
		"features":     "advanced",
		"api":          "v2",
		"custom":       "value",
	}

	caps := dns.parseCapabilities(metadata)

	// Name should be excluded
	_, hasName := caps["name"]
	assert.False(t, hasName)

	// Known capability keys
	assert.Equal(t, "1.0.0", caps["version"])
	assert.Equal(t, "streaming,tools", caps["capabilities"])
	assert.Equal(t, "advanced", caps["features"])
	assert.Equal(t, "v2", caps["api"])

	// Custom keys should also be included
	assert.Equal(t, "value", caps["custom"])
}

func TestDNSDiscovery_ServerIDUniqueness(t *testing.T) {
	mockResolver := NewMockDNSResolver()

	// Two services on different ports at the same address
	mockResolver.AddSRVRecord("mcp", "tcp", "local", "server.local.", 3000, 10, 100)
	mockResolver.AddSRVRecord("lsp", "tcp", "local", "server.local.", 6006, 10, 100)
	mockResolver.AddHostRecord("server.local", "192.168.1.100")

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx := context.Background()
	servers, err := dns.Discover(ctx)

	require.NoError(t, err)
	require.Len(t, servers, 2)

	// Server IDs should be unique
	assert.NotEqual(t, servers[0].ID, servers[1].ID)
}

func TestDNSDiscovery_Discover_NilLogger(t *testing.T) {
	mockResolver := NewMockDNSResolver()
	mockResolver.SetSRVError("mcp", "tcp", "local", errors.New("test error"))

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   nil, // No logger
		resolver: mockResolver,
	}

	ctx := context.Background()
	// Should not panic even without logger
	servers, err := dns.Discover(ctx)

	require.NoError(t, err)
	assert.NotNil(t, servers)
}

func TestDefaultDNSResolver_LookupSRV(t *testing.T) {
	resolver := &DefaultDNSResolver{}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Try to look up a non-existent service
	// This should return an error but not panic
	_, _, err := resolver.LookupSRV(ctx, "nonexistent-service-test", "tcp", "local")

	// The error is expected (no such service)
	// We're just verifying it doesn't panic and uses the correct API
	assert.NotNil(t, err)
}

func TestDefaultDNSResolver_LookupHost(t *testing.T) {
	resolver := &DefaultDNSResolver{}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Try to look up localhost
	addrs, err := resolver.LookupHost(ctx, "localhost")

	// localhost should resolve
	require.NoError(t, err)
	assert.NotEmpty(t, addrs)
}

func TestDefaultDNSResolver_LookupTXT(t *testing.T) {
	resolver := &DefaultDNSResolver{}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Try to look up TXT records for a non-existent domain
	_, err := resolver.LookupTXT(ctx, "nonexistent-domain-test.invalid")

	// Error is expected
	assert.NotNil(t, err)
}

// Benchmark tests

func BenchmarkDNSDiscovery_Discover(b *testing.B) {
	mockResolver := NewMockDNSResolver()

	// Set up some services
	mockResolver.AddSRVRecord("mcp", "tcp", "local", "mcp-server.local.", 3000, 10, 100)
	mockResolver.AddHostRecord("mcp-server.local", "192.168.1.100")
	mockResolver.AddTXTRecord("mcp-server.local", "name=Test", "version=1.0")

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dns.Discover(ctx)
	}
}

func BenchmarkDNSDiscovery_discoverService(b *testing.B) {
	mockResolver := NewMockDNSResolver()
	mockResolver.AddSRVRecord("mcp", "tcp", "local", "mcp-server.local.", 3000, 10, 100)
	mockResolver.AddHostRecord("mcp-server.local", "192.168.1.100")

	dns := &DNSDiscovery{
		domain:   "local",
		services: make(map[string]*DiscoveredServer),
		logger:   newDNSTestLogger(),
		resolver: mockResolver,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dns.discoverService(ctx, "mcp", "tcp", "mcp")
	}
}
