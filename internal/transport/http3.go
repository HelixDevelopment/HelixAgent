package transport

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// HTTP3Server represents an HTTP3/HTTP2 server with fallback to HTTP1.1.
type HTTP3Server struct {
	http3Server *http3.Server
	httpServer  *http.Server
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
func NewHTTP3Server(handler *gin.Engine, config *HTTP3Config) (*HTTP3Server, error) {
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
	tlsConfig, err := createTLSConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS config: %w", err)
	}

	server := &HTTP3Server{
		enableHTTP3: config.EnableHTTP3,
		enableHTTP2: config.EnableHTTP2,
		addr:        config.Address,
		tlsConfig:   tlsConfig,
	}

	// Setup HTTP3 server if enabled
	if config.EnableHTTP3 {
		server.http3Server = &http3.Server{
			Handler:   handler,
			Addr:      config.Address,
			TLSConfig: tlsConfig,
			QUICConfig: &quic.Config{
				MaxIdleTimeout:  config.IdleTimeout,
				KeepAlivePeriod: 10 * time.Second,
			},
		}
	}

	// Setup HTTP server for HTTP2/HTTP1.1 fallback
	server.httpServer = &http.Server{
		Addr:         config.Address,
		Handler:      handler,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
		TLSConfig:    tlsConfig,
	}

	return server, nil
}

// Start starts the HTTP3 server with HTTP2 fallback.
func (s *HTTP3Server) Start() error {
	if s.enableHTTP3 && s.http3Server != nil {
		// Start HTTP3 server
		go func() {
			fmt.Printf("Starting HTTP/3 server on %s\n", s.addr)
			if err := s.http3Server.ListenAndServe(); err != nil {
				fmt.Printf("HTTP/3 server error: %v\n", err)
			}
		}()
	}

	if s.enableHTTP2 {
		// Start HTTP2/HTTP1.1 server
		go func() {
			fmt.Printf("Starting HTTP/2 + HTTP/1.1 server on %s\n", s.addr)
			if err := s.httpServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				fmt.Printf("HTTP server error: %v\n", err)
			}
		}()
	} else {
		// Start HTTP1.1 only
		go func() {
			fmt.Printf("Starting HTTP/1.1 server on %s\n", s.addr)
			if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Printf("HTTP server error: %v\n", err)
			}
		}()
	}

	return nil
}

// Stop stops the server gracefully.
func (s *HTTP3Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var errs []error

	// Close HTTP3 server
	if s.http3Server != nil {
		if err := s.http3Server.Close(); err != nil {
			errs = append(errs, fmt.Errorf("HTTP/3 server close error: %w", err))
		}
	}

	// Close HTTP server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("HTTP server shutdown error: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}

	return nil
}

// GetServerInfo returns information about the server configuration
func (s *HTTP3Server) GetServerInfo() map[string]interface{} {
	protocols := []string{}
	if s.enableHTTP3 {
		protocols = append(protocols, "HTTP/3")
	}
	if s.enableHTTP2 {
		protocols = append(protocols, "HTTP/2")
	}
	protocols = append(protocols, "HTTP/1.1")

	features := []string{"TLS encryption"}
	if s.enableHTTP3 {
		features = append(features, "HTTP/3 with QUIC")
	}
	if s.enableHTTP2 {
		features = append(features, "HTTP/2 with server push")
	}
	features = append(features, "HTTP/1.1 fallback")

	return map[string]interface{}{
		"http3_enabled": s.enableHTTP3,
		"http2_enabled": s.enableHTTP2,
		"address":       s.addr,
		"protocols":     protocols,
		"features":      features,
	}
}

// createTLSConfig creates a TLS configuration for the server
func createTLSConfig(config *HTTP3Config) (*tls.Config, error) {
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
		NextProtos: []string{"h3", "h2", "http/1.1"},
	}

	// Load certificates if provided
	if config.TLSCertFile != "" && config.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
		if err != nil {
			fmt.Printf("Warning: Failed to load TLS certificates: %v, generating self-signed cert\n", err)
			// Generate self-signed certificate for development
			cert, err = generateSelfSignedCert()
			if err != nil {
				return nil, fmt.Errorf("failed to generate self-signed certificate: %w", err)
			}
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	} else {
		// Generate self-signed certificate for development
		cert, err := generateSelfSignedCert()
		if err != nil {
			return nil, fmt.Errorf("failed to generate self-signed certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// generateSelfSignedCert generates a self-signed certificate for development
func generateSelfSignedCert() (tls.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"SuperAgent Dev"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:              []string{"localhost"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create X509 key pair: %w", err)
	}

	return cert, nil
}
