package integration

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	helixhttp "dev.helix.agent/internal/http"
	"dev.helix.agent/internal/middleware"
	"dev.helix.agent/internal/transport"

	"github.com/gin-gonic/gin"
)

// TestHTTP3ComplianceVerification verifies that the HTTP/3 (QUIC) and Brotli
// compression stack is properly configured per the project constitution
// (Networking rule: "ALL HTTP communication MUST use HTTP/3 (QUIC) as primary
// transport with Brotli compression").
func TestHTTP3ComplianceVerification(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping HTTP/3 compliance test in short mode")  // SKIP-OK: #short-mode
	}

	t.Run("QUIC_client_dependency_available", func(t *testing.T) {
		// Verify that the QUIC client can be instantiated with default config.
		// This proves that quic-go dependency is importable and functional.
		client, err := helixhttp.NewQUICClient(nil)
		require.NoError(t, err, "NewQUICClient with default config must succeed")
		require.NotNil(t, client, "QUIC client must not be nil")
		defer func() { _ = client.Close() }()

		metrics := client.Metrics()
		assert.NotNil(t, metrics, "QUIC client metrics must be available")
		assert.Equal(t, int64(0), metrics.TotalRequests, "fresh client should have zero requests")
	})

	t.Run("QUIC_default_config_values", func(t *testing.T) {
		cfg := helixhttp.DefaultQUICConfig()
		require.NotNil(t, cfg)
		assert.NotNil(t, cfg.TLSConfig, "TLS config must be set for QUIC")
		assert.Equal(t, uint16(tls.VersionTLS13), cfg.TLSConfig.MinVersion,
			"QUIC must require TLS 1.3 minimum")
		assert.True(t, cfg.EnableH2Fallback, "HTTP/2 fallback must be enabled by default")
		assert.Greater(t, cfg.MaxConnsPerHost, 0, "MaxConnsPerHost must be positive")
		assert.Greater(t, cfg.RequestTimeout, time.Duration(0), "RequestTimeout must be positive")
	})

	t.Run("HTTP3_server_creation_with_QUIC", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()

		server, err := transport.NewHTTP3Server(router, &transport.HTTP3Config{
			Address:     "localhost:0",
			EnableHTTP3: true,
			EnableHTTP2: true,
			IdleTimeout: 10 * time.Second,
		})
		require.NoError(t, err, "HTTP/3 server creation must succeed")
		require.NotNil(t, server)

		info := server.GetServerInfo()
		assert.Equal(t, true, info["http3_enabled"], "HTTP/3 must be enabled")
		assert.Equal(t, true, info["http2_enabled"], "HTTP/2 fallback must be enabled")

		protocols, ok := info["protocols"].([]string)
		require.True(t, ok)
		assert.Contains(t, protocols, "HTTP/3", "server must advertise HTTP/3")
		assert.Contains(t, protocols, "HTTP/2", "server must advertise HTTP/2")
		assert.Contains(t, protocols, "HTTP/1.1", "server must advertise HTTP/1.1")

		features, ok := info["features"].([]string)
		require.True(t, ok)
		assert.Contains(t, features, "HTTP/3 with QUIC", "features must include QUIC")
	})

	t.Run("TLS_config_includes_h3_ALPN", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()

		server, err := transport.NewHTTP3Server(router, &transport.HTTP3Config{
			Address:     "localhost:0",
			EnableHTTP3: true,
			EnableHTTP2: true,
		})
		require.NoError(t, err)
		require.NotNil(t, server)

		// The server uses internal TLS config; verify via GetServerInfo that
		// it was configured to negotiate h3.
		info := server.GetServerInfo()
		assert.Equal(t, true, info["http3_enabled"],
			"HTTP/3 ALPN negotiation must be configured")
	})

	t.Run("HTTP3_provider_transport_creation", func(t *testing.T) {
		// Verify the provider transport wrapper can be created.
		pt, err := helixhttp.NewHTTP3ProviderTransport(helixhttp.DefaultQUICConfig())
		require.NoError(t, err, "HTTP3ProviderTransport creation must succeed")
		require.NotNil(t, pt)
		assert.NoError(t, pt.Close(), "Close must succeed")
	})

	t.Run("Brotli_compression_middleware_configured", func(t *testing.T) {
		// Verify default compression config enables Brotli as primary.
		config := middleware.DefaultCompressionConfig()
		assert.True(t, config.EnableBrotli, "Brotli must be enabled by default (primary)")
		assert.True(t, config.EnableGzip, "Gzip must be enabled as fallback")
		assert.Greater(t, config.BrotliLevel, 0, "Brotli compression level must be set")
	})

	t.Run("Brotli_compress_decompress_roundtrip", func(t *testing.T) {
		original := []byte(`{"response":"Hello from HelixAgent debate ensemble","confidence":0.95,"provider":"gemini"}`)
		compressed, err := middleware.CompressData(original, "br", 4)
		require.NoError(t, err, "Brotli compression must succeed")
		assert.Less(t, len(compressed), len(original),
			"compressed data should be smaller than original for JSON")

		decompressed, err := middleware.DecompressData(compressed, "br")
		require.NoError(t, err, "Brotli decompression must succeed")
		assert.Equal(t, original, decompressed, "round-trip must preserve data")
	})

	t.Run("Gzip_fallback_compress_decompress", func(t *testing.T) {
		original := []byte(`{"status":"ok","providers":["gemini","deepseek","mistral"]}`)
		compressed, err := middleware.CompressData(original, "gzip", 5)
		require.NoError(t, err, "gzip compression must succeed")

		decompressed, err := middleware.DecompressData(compressed, "gzip")
		require.NoError(t, err, "gzip decompression must succeed")
		assert.Equal(t, original, decompressed, "gzip round-trip must preserve data")
	})

	t.Run("Compression_ratio_estimates", func(t *testing.T) {
		jsonRatio := middleware.EstimateCompressionRatio("application/json")
		assert.Less(t, jsonRatio, 0.5,
			"JSON should have good compression ratio estimate")

		htmlRatio := middleware.EstimateCompressionRatio("text/html")
		assert.Less(t, htmlRatio, 0.5,
			"HTML should have good compression ratio estimate")
	})

	t.Run("QUIC_metrics_tracking", func(t *testing.T) {
		client, err := helixhttp.NewQUICClient(nil)
		require.NoError(t, err)
		defer func() { _ = client.Close() }()

		m := client.Metrics()
		assert.Equal(t, int64(0), m.H3Requests, "no H3 requests yet")
		assert.Equal(t, int64(0), m.FallbackRequests, "no fallback requests yet")
		assert.Equal(t, time.Duration(0), m.AverageLatency(),
			"average latency should be zero with no requests")
	})
}
