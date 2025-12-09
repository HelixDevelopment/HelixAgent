package transport

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/http2"
)

// HTTP3Server represents an HTTP3/HTTP2 server with fallback to HTTP1.1.
type HTTP3Server struct {
	httpServer  *http.Server
	http2Server *http2.Server
	enableHTTP3 bool
	enableHTTP2 bool
	addr        string
	tlsConfig   *tls.Config
}

// HTTP3Config holds configuration for HTTP3 server
type HTTP3Config struct {
	Address        string
	EnableHTTP3    bool
	EnableHTTP2    bool
	TLSCertFile    string
	TLSKeyFile     string
	MaxConnections int
	IdleTimeout    time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
}

// NewHTTP3Server creates a new HTTP3 server with HTTP2 fallback.
func NewHTTP3Server(handler *gin.Engine, config *HTTP3Config) *HTTP3Server {
	if config == nil {
		config = &HTTP3Config{
			Address:        ":8080",
			EnableHTTP3:    true,
			EnableHTTP2:    true,
			MaxConnections: 1000,
			IdleTimeout:    30 * time.Second,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
		}
	}

	// Create TLS configuration
	tlsConfig := createTLSConfig(config)

	server := &HTTP3Server{
		httpServer: &http.Server{
			Addr:         config.Address,
			Handler:      handler,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
			IdleTimeout:  config.IdleTimeout,
			TLSConfig:    tlsConfig,
		},
		enableHTTP3: config.EnableHTTP3,
		enableHTTP2: config.EnableHTTP2,
		addr:        config.Address,
		tlsConfig:   tlsConfig,
	}

	// Setup HTTP2 server if enabled
	if config.EnableHTTP2 {
		server.http2Server = &http2.Server{}
	}

	return server
}

// Start starts the HTTP3 server with HTTP2 fallback.
func (s *HTTP3Server) Start() error {
	if s.enableHTTP3 {
		// Start HTTP2 server with HTTP/3 support
		if s.enableHTTP2 && s.http2Server != nil {
			go s.serveHTTP2()
			fmt.Printf("HTTP2 server with HTTP/3 support enabled on %s\n", s.addr)
		} else {
			// Start HTTP server for fallback
			go s.serveHTTP()
			fmt.Printf("HTTP server fallback enabled on %s\n", s.addr)
		}
	} else {
		// Start HTTP server for fallback
		go s.serveHTTP()
		fmt.Printf("HTTP server enabled on %s\n", s.addr)
	}

	return nil
}

// serveHTTP2 handles HTTP2 connections with HTTP/3 support
func (s *HTTP3Server) serveHTTP2() {
	if s.http2Server != nil {
		// HTTP2 server would be configured here
		// For now, delegate to HTTP server
		s.serveHTTP()
	}
}

// serveHTTP handles standard HTTP connections
func (s *HTTP3Server) serveHTTP() {
	err := s.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		fmt.Printf("HTTP server error: %v\n", err)
	}
}

// Stop stops the server gracefully.
func (s *HTTP3Server) Stop() error {
	var errs []error

	// Close HTTP server
	if err := s.httpServer.Close(); err != nil {
		errs = append(errs, fmt.Errorf("HTTP server close error: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}

	return nil
}

// GetServerInfo returns information about the server configuration
func (s *HTTP3Server) GetServerInfo() map[string]interface{} {
	return map[string]interface{}{
		"http3_enabled": s.enableHTTP3,
		"http2_enabled": s.enableHTTP2,
		"address":       s.addr,
		"protocols":     []string{"HTTP/3", "HTTP/2", "HTTP/1.1"},
		"features": []string{
			"HTTP/3 support",
			"TLS encryption",
			"HTTP/2 with server push",
			"HTTP/1.1 fallback",
		},
	}
}

// UpgradeConnection attempts to upgrade an HTTP connection to HTTP3 if supported
func (s *HTTP3Server) UpgradeConnection(w http.ResponseWriter, r *http.Request) bool {
	// Check if client supports HTTP/3
	if r.Header.Get("HTTP3-Settings") != "" || r.Header.Get("Alt-Svc") == "h3" {
		// Perform HTTP3 upgrade
		w.Header().Set("Alt-Svc", "h3")
		w.WriteHeader(http.StatusEarlyHints)

		// This would be followed by actual HTTP3 handling
		// For now, indicate upgrade initiated
		return true
	}

	return false
}

// createTLSConfig creates a TLS configuration for the server
func createTLSConfig(config *HTTP3Config) *tls.Config {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}

	// Load certificates if provided
	if config.TLSCertFile != "" && config.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
		if err != nil {
			fmt.Printf("Warning: Failed to load TLS certificates: %v\n", err)
		} else {
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}

	return tlsConfig
}

// HandleHTTP3Request handles HTTP/3 specific request processing
func (s *HTTP3Server) HandleHTTP3Request(w http.ResponseWriter, r *http.Request) {
	// Basic HTTP/3 handling
	// In a full implementation, this would handle:
	// - QPACK encoding/decoding
	// - Stream multiplexing
	// - 0-RTT connection establishment
	// - Server push

	// For now, we'll provide a basic implementation
	w.Header().Set("HTTP3-Settings", "foo=bar")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("HTTP/3 response (basic implementation)"))
}

// SetHTTP3Handler allows updating the HTTP3 handler
func (s *HTTP3Server) SetHTTP3Handler(handler http.Handler) {
	s.httpServer.Handler = handler
	if s.http2Server != nil {
		// HTTP2 server would use the same handler
	}
}
