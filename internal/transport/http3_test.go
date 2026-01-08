package transport

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTP3Config_DefaultValues(t *testing.T) {
	config := &HTTP3Config{}

	assert.Equal(t, "", config.Address)
	assert.Equal(t, false, config.EnableHTTP3)
	assert.Equal(t, false, config.EnableHTTP2)
	assert.Equal(t, "", config.TLSCertFile)
	assert.Equal(t, "", config.TLSKeyFile)
	assert.Equal(t, 0, config.MaxConnections)
	assert.Equal(t, time.Duration(0), config.IdleTimeout)
	assert.Equal(t, time.Duration(0), config.ReadTimeout)
	assert.Equal(t, time.Duration(0), config.WriteTimeout)
}

func TestNewHTTP3Server_WithNilConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	server, err := NewHTTP3Server(router, nil)
	require.NoError(t, err)
	require.NotNil(t, server)
	assert.Equal(t, ":7061", server.addr)
	assert.True(t, server.enableHTTP3)
	assert.True(t, server.enableHTTP2)
	assert.NotNil(t, server.httpServer)
	assert.NotNil(t, server.http3Server)
	assert.NotNil(t, server.tlsConfig)
}

func TestNewHTTP3Server_WithCustomConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	config := &HTTP3Config{
		Address:        ":8443",
		EnableHTTP3:    true,
		EnableHTTP2:    true,
		TLSCertFile:    "test.crt",
		TLSKeyFile:     "test.key",
		MaxConnections: 500,
		IdleTimeout:    60 * time.Second,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
	}

	server, err := NewHTTP3Server(router, config)
	require.NoError(t, err)
	require.NotNil(t, server)
	assert.Equal(t, ":8443", server.addr)
	assert.True(t, server.enableHTTP3)
	assert.True(t, server.enableHTTP2)
	assert.NotNil(t, server.httpServer)
	assert.NotNil(t, server.http3Server)
	assert.NotNil(t, server.tlsConfig)

	assert.Equal(t, config.Address, server.httpServer.Addr)
	assert.Equal(t, config.ReadTimeout, server.httpServer.ReadTimeout)
	assert.Equal(t, config.WriteTimeout, server.httpServer.WriteTimeout)
	assert.Equal(t, config.IdleTimeout, server.httpServer.IdleTimeout)
}

func TestNewHTTP3Server_HTTP3Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	config := &HTTP3Config{
		Address:     ":7061",
		EnableHTTP3: false,
		EnableHTTP2: true,
	}

	server, err := NewHTTP3Server(router, config)
	require.NoError(t, err)
	require.NotNil(t, server)
	assert.False(t, server.enableHTTP3)
	assert.True(t, server.enableHTTP2)
	assert.Nil(t, server.http3Server)
	assert.NotNil(t, server.httpServer)
}

func TestNewHTTP3Server_HTTP2Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	config := &HTTP3Config{
		Address:     ":7061",
		EnableHTTP3: true,
		EnableHTTP2: false,
	}

	server, err := NewHTTP3Server(router, config)
	require.NoError(t, err)
	require.NotNil(t, server)
	assert.True(t, server.enableHTTP3)
	assert.False(t, server.enableHTTP2)
	assert.NotNil(t, server.http3Server)
	assert.NotNil(t, server.httpServer)
}

func TestGetServerInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	tests := []struct {
		name          string
		config        *HTTP3Config
		expectedHTTP3 bool
		expectedHTTP2 bool
		expectedAddr  string
	}{
		{
			name: "HTTP3 and HTTP2 enabled",
			config: &HTTP3Config{
				Address:     ":8443",
				EnableHTTP3: true,
				EnableHTTP2: true,
			},
			expectedHTTP3: true,
			expectedHTTP2: true,
			expectedAddr:  ":8443",
		},
		{
			name: "HTTP3 only",
			config: &HTTP3Config{
				Address:     ":7061",
				EnableHTTP3: true,
				EnableHTTP2: false,
			},
			expectedHTTP3: true,
			expectedHTTP2: false,
			expectedAddr:  ":7061",
		},
		{
			name: "HTTP2 only",
			config: &HTTP3Config{
				Address:     ":8081",
				EnableHTTP3: false,
				EnableHTTP2: true,
			},
			expectedHTTP3: false,
			expectedHTTP2: true,
			expectedAddr:  ":8081",
		},
		{
			name: "HTTP1.1 only",
			config: &HTTP3Config{
				Address:     ":8082",
				EnableHTTP3: false,
				EnableHTTP2: false,
			},
			expectedHTTP3: false,
			expectedHTTP2: false,
			expectedAddr:  ":8082",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewHTTP3Server(router, tt.config)
			require.NoError(t, err)
			info := server.GetServerInfo()

			assert.Equal(t, tt.expectedHTTP3, info["http3_enabled"])
			assert.Equal(t, tt.expectedHTTP2, info["http2_enabled"])
			assert.Equal(t, tt.expectedAddr, info["address"])

			protocols, ok := info["protocols"].([]string)
			assert.True(t, ok, "protocols should be a []string")

			features, ok := info["features"].([]string)
			assert.True(t, ok, "features should be a []string")

			if tt.expectedHTTP3 {
				assert.Contains(t, protocols, "HTTP/3")
				assert.Contains(t, features, "HTTP/3 with QUIC")
			}

			if tt.expectedHTTP2 {
				assert.Contains(t, protocols, "HTTP/2")
				assert.Contains(t, features, "HTTP/2 with server push")
			}

			assert.Contains(t, protocols, "HTTP/1.1")
			assert.Contains(t, features, "HTTP/1.1 fallback")
			assert.Contains(t, features, "TLS encryption")
		})
	}
}

func TestCreateTLSConfig_WithCertFiles(t *testing.T) {
	config := &HTTP3Config{
		TLSCertFile: "testdata/cert.pem",
		TLSKeyFile:  "testdata/key.pem",
	}

	tlsConfig, err := createTLSConfig(config)
	require.NoError(t, err)
	require.NotNil(t, tlsConfig)
	assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
	assert.Len(t, tlsConfig.Certificates, 1)
	assert.Contains(t, tlsConfig.NextProtos, "h3")
	assert.Contains(t, tlsConfig.NextProtos, "h2")
	assert.Contains(t, tlsConfig.NextProtos, "http/1.1")
}

func TestCreateTLSConfig_WithoutCertFiles(t *testing.T) {
	config := &HTTP3Config{}

	tlsConfig, err := createTLSConfig(config)
	require.NoError(t, err)
	require.NotNil(t, tlsConfig)
	assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
	assert.Len(t, tlsConfig.Certificates, 1)
	assert.Contains(t, tlsConfig.NextProtos, "h3")
	assert.Contains(t, tlsConfig.NextProtos, "h2")
	assert.Contains(t, tlsConfig.NextProtos, "http/1.1")
}

func TestHTTP3Server_Start_NoError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	config := &HTTP3Config{
		Address:     "localhost:0", // Use port 0 for automatic port assignment
		EnableHTTP3: false,         // Disable HTTP3 to avoid network binding issues in tests
		EnableHTTP2: false,         // Disable HTTP2 to avoid TLS issues in tests
	}

	server, err := NewHTTP3Server(router, config)
	require.NoError(t, err)
	require.NotNil(t, server)

	// Start should not return an error (it starts goroutines)
	err = server.Start()
	assert.NoError(t, err)

	// Give goroutines a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop the server
	err = server.Stop()
	assert.NoError(t, err)
}

func TestHTTP3Server_Stop_WithNoServers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	config := &HTTP3Config{
		Address:     ":7061",
		EnableHTTP3: false,
		EnableHTTP2: false,
	}

	server, err := NewHTTP3Server(router, config)
	require.NoError(t, err)
	require.NotNil(t, server)

	// Stop should work even if server wasn't started
	err = server.Stop()
	assert.NoError(t, err)
}

func TestGenerateSelfSignedCert_Success(t *testing.T) {
	// This test ensures generateSelfSignedCert returns a valid certificate
	cert, err := generateSelfSignedCert()
	require.NoError(t, err)
	assert.NotEmpty(t, cert.Certificate)
	assert.NotNil(t, cert.PrivateKey)
}

func TestHTTP3Server_AddressValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	tests := []struct {
		name    string
		address string
		valid   bool
	}{
		{"Valid address with port", ":7061", true},
		{"Valid address with host and port", "localhost:7061", true},
		{"Valid address with IP and port", "127.0.0.1:7061", true},
		{"Empty address", "", true},                 // Defaults to :7061
		{"Address without port", "localhost", true}, // Might fail at runtime
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &HTTP3Config{
				Address:     tt.address,
				EnableHTTP3: false,
				EnableHTTP2: false,
			}

			server, err := NewHTTP3Server(router, config)
			require.NoError(t, err)
			assert.NotNil(t, server)
			assert.Equal(t, tt.address, server.addr)
		})
	}
}

func TestTLSConfig_CipherSuites(t *testing.T) {
	config := &HTTP3Config{}
	tlsConfig, err := createTLSConfig(config)
	require.NoError(t, err)
	require.NotNil(t, tlsConfig)
	assert.NotEmpty(t, tlsConfig.CipherSuites)

	// Check that we have modern cipher suites
	expectedCiphers := []uint16{
		tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_AES_256_GCM_SHA384,
		tls.TLS_CHACHA20_POLY1305_SHA256,
	}

	for _, cipher := range expectedCiphers {
		assert.Contains(t, tlsConfig.CipherSuites, cipher)
	}
}

func TestTLSConfig_CurvePreferences(t *testing.T) {
	config := &HTTP3Config{}
	tlsConfig, err := createTLSConfig(config)
	require.NoError(t, err)
	require.NotNil(t, tlsConfig)
	assert.NotEmpty(t, tlsConfig.CurvePreferences)

	// Check for modern elliptic curves
	expectedCurves := []tls.CurveID{
		tls.X25519,
		tls.CurveP256,
	}

	for _, curve := range expectedCurves {
		assert.Contains(t, tlsConfig.CurvePreferences, curve)
	}
}
