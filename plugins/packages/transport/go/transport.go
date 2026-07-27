// Package transport provides a unified transport layer for HelixAgent CLI agent plugins.
// It supports HTTP/3 (QUIC), HTTP/2, and HTTP/1.1 with automatic fallback,
// TOON protocol for efficient token serialization, and Brotli compression.
package transport

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/quic-go/quic-go/http3"
)

// Protocol represents the negotiated HTTP protocol version
type Protocol string

const (
	ProtocolHTTP3 Protocol = "h3"
	ProtocolHTTP2 Protocol = "h2"
	ProtocolHTTP1 Protocol = "http/1.1"
)

// ContentType represents the content encoding format
type ContentType string

const (
	ContentTypeTOON ContentType = "application/toon+json"
	ContentTypeJSON ContentType = "application/json"
)

// Compression represents the compression method
type Compression string

const (
	CompressionBrotli Compression = "br"
	CompressionGzip   Compression = "gzip"
	CompressionNone   Compression = "identity"
)

// ConnectOptions configures the transport connection
type ConnectOptions struct {
	// PreferHTTP3 attempts HTTP/3 connection first
	PreferHTTP3 bool
	// EnableTOON enables TOON protocol encoding
	EnableTOON bool
	// EnableBrotli enables Brotli compression
	EnableBrotli bool
	// Timeout for connection establishment
	Timeout time.Duration
	// TLSConfig for secure connections
	TLSConfig *tls.Config
	// Headers to include in all requests
	Headers map[string]string
}

// DefaultConnectOptions returns sensible defaults
func DefaultConnectOptions() *ConnectOptions {
	return &ConnectOptions{
		PreferHTTP3:  true,
		EnableTOON:   true,
		EnableBrotli: true,
		Timeout:      30 * time.Second,
		Headers:      make(map[string]string),
	}
}

// Request represents an outgoing request
type Request struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    interface{}
}

// Response represents an incoming response
type Response struct {
	StatusCode  int
	Headers     map[string]string
	Body        []byte
	Protocol    Protocol
	ContentType ContentType
	Compression Compression
}

// Event represents a streaming event
type Event struct {
	Type string
	Data json.RawMessage
	ID   string
}

// HelixTransport provides a unified transport interface for CLI agent plugins
type HelixTransport interface {
	// Connect establishes connection to the endpoint
	Connect(endpoint string, opts *ConnectOptions) error
	// NegotiateProtocol returns the negotiated HTTP protocol
	NegotiateProtocol() (Protocol, error)
	// NegotiateContent returns the negotiated content type
	NegotiateContent() (ContentType, error)
	// NegotiateCompression returns the negotiated compression
	NegotiateCompression() (Compression, error)
	// Do performs a request and returns the response
	Do(ctx context.Context, req *Request) (*Response, error)
	// Stream performs a streaming request
	Stream(ctx context.Context, req *Request) (<-chan *Event, error)
	// Close closes the transport
	Close() error
}

// Transport implements HelixTransport with fallback chain support
type Transport struct {
	endpoint    string
	opts        *ConnectOptions
	protocol    Protocol
	contentType ContentType
	compression Compression

	http3Client  *http.Client
	http2Client  *http.Client
	http1Client  *http.Client
	activeClient *http.Client

	mu     sync.RWMutex
	closed bool
}

// NewTransport creates a new transport instance
func NewTransport() *Transport {
	return &Transport{}
}

// Connect establishes connection to the endpoint with automatic fallback
func (t *Transport) Connect(endpoint string, opts *ConnectOptions) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if opts == nil {
		opts = DefaultConnectOptions()
	}

	t.endpoint = strings.TrimSuffix(endpoint, "/")
	t.opts = opts

	// Initialize HTTP clients
	t.initClients()

	// Attempt protocol negotiation with fallback
	if err := t.negotiateProtocol(); err != nil {
		return fmt.Errorf("protocol negotiation failed: %w", err)
	}

	// Negotiate content type
	if opts.EnableTOON {
		t.contentType = ContentTypeTOON
	} else {
		t.contentType = ContentTypeJSON
	}

	// Negotiate compression
	if opts.EnableBrotli {
		t.compression = CompressionBrotli
	} else {
		t.compression = CompressionGzip
	}

	return nil
}

func (t *Transport) initClients() {
	tlsConfig := t.opts.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	// HTTP/3 client
	t.http3Client = &http.Client{
		Transport: &http3.RoundTripper{
			TLSClientConfig: tlsConfig,
		},
		Timeout: t.opts.Timeout,
	}

	// HTTP/2 client
	t.http2Client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:   tlsConfig,
			ForceAttemptHTTP2: true,
		},
		Timeout: t.opts.Timeout,
	}

	// HTTP/1.1 client
	t.http1Client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:   tlsConfig,
			ForceAttemptHTTP2: false,
		},
		Timeout: t.opts.Timeout,
	}
}

func (t *Transport) negotiateProtocol() error {
	// Try HTTP/3 first if preferred
	if t.opts.PreferHTTP3 {
		if err := t.testConnection(t.http3Client); err == nil {
			t.protocol = ProtocolHTTP3
			t.activeClient = t.http3Client
			return nil
		}
	}

	// Fall back to HTTP/2
	if err := t.testConnection(t.http2Client); err == nil {
		t.protocol = ProtocolHTTP2
		t.activeClient = t.http2Client
		return nil
	}

	// Fall back to HTTP/1.1
	if err := t.testConnection(t.http1Client); err == nil {
		t.protocol = ProtocolHTTP1
		t.activeClient = t.http1Client
		return nil
	}

	return fmt.Errorf("no supported protocol available")
}

func (t *Transport) testConnection(client *http.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "HEAD", t.endpoint+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("health check failed: %d", resp.StatusCode)
	}

	return nil
}

// NegotiateProtocol returns the negotiated HTTP protocol
func (t *Transport) NegotiateProtocol() (Protocol, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.protocol, nil
}

// NegotiateContent returns the negotiated content type
func (t *Transport) NegotiateContent() (ContentType, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.contentType, nil
}

// NegotiateCompression returns the negotiated compression
func (t *Transport) NegotiateCompression() (Compression, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.compression, nil
}

// Do performs a request and returns the response
func (t *Transport) Do(ctx context.Context, req *Request) (*Response, error) {
	t.mu.RLock()
	client := t.activeClient
	endpoint := t.endpoint
	contentType := t.contentType
	compression := t.compression
	t.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("transport not connected")
	}

	// Serialize body
	var bodyReader io.Reader
	if req.Body != nil {
		var bodyBytes []byte
		var err error

		if contentType == ContentTypeTOON {
			bodyBytes, err = EncodeTOON(req.Body)
		} else {
			bodyBytes, err = json.Marshal(req.Body)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to serialize body: %w", err)
		}

		// Apply compression
		if compression == CompressionBrotli {
			bodyBytes, err = compressBrotli(bodyBytes)
			if err != nil {
				return nil, fmt.Errorf("brotli compression failed: %w", err)
			}
		} else if compression == CompressionGzip {
			bodyBytes, err = compressGzip(bodyBytes)
			if err != nil {
				return nil, fmt.Errorf("gzip compression failed: %w", err)
			}
		}

		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Build HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, endpoint+req.Path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", string(contentType))
	httpReq.Header.Set("Accept", string(contentType))
	if compression != CompressionNone {
		httpReq.Header.Set("Content-Encoding", string(compression))
		httpReq.Header.Set("Accept-Encoding", string(compression)+", gzip")
	}

	for k, v := range t.opts.Headers {
		httpReq.Header.Set(k, v)
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Perform request
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Decompress if needed
	respCompression := Compression(httpResp.Header.Get("Content-Encoding"))
	if respCompression == CompressionBrotli {
		respBody, err = decompressBrotli(respBody)
		if err != nil {
			return nil, fmt.Errorf("brotli decompression failed: %w", err)
		}
	} else if respCompression == CompressionGzip {
		respBody, err = decompressGzip(respBody)
		if err != nil {
			return nil, fmt.Errorf("gzip decompression failed: %w", err)
		}
	}

	// Build response
	resp := &Response{
		StatusCode:  httpResp.StatusCode,
		Headers:     make(map[string]string),
		Body:        respBody,
		Protocol:    t.protocol,
		ContentType: ContentType(httpResp.Header.Get("Content-Type")),
		Compression: respCompression,
	}

	for k, v := range httpResp.Header {
		if len(v) > 0 {
			resp.Headers[k] = v[0]
		}
	}

	return resp, nil
}

// Stream performs a streaming request (SSE)
func (t *Transport) Stream(ctx context.Context, req *Request) (<-chan *Event, error) {
	t.mu.RLock()
	client := t.activeClient
	endpoint := t.endpoint
	t.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("transport not connected")
	}

	// Build HTTP request
	var bodyReader io.Reader
	if req.Body != nil {
		bodyBytes, err := json.Marshal(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, endpoint+req.Path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")

	for k, v := range t.opts.Headers {
		httpReq.Header.Set(k, v)
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Perform request
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("stream request failed: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		httpResp.Body.Close()
		return nil, fmt.Errorf("stream request returned status %d", httpResp.StatusCode)
	}

	// Create event channel
	events := make(chan *Event, 100)

	go func() {
		defer close(events)
		defer httpResp.Body.Close()

		reader := httpResp.Body
		buf := make([]byte, 4096)
		var eventData strings.Builder
		var eventType string
		var eventID string

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			n, err := reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					// Log error but don't block
				}
				return
			}

			lines := strings.Split(string(buf[:n]), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)

				if line == "" {
					// Empty line means end of event
					if eventData.Len() > 0 {
						data := eventData.String()
						if data == "[DONE]" {
							return
						}

						event := &Event{
							Type: eventType,
							ID:   eventID,
						}
						if json.Valid([]byte(data)) {
							event.Data = json.RawMessage(data)
						} else {
							event.Data = json.RawMessage(`"` + data + `"`)
						}

						select {
						case events <- event:
						case <-ctx.Done():
							return
						}

						eventData.Reset()
						eventType = ""
						eventID = ""
					}
					continue
				}

				if strings.HasPrefix(line, "event:") {
					eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				} else if strings.HasPrefix(line, "data:") {
					data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
					eventData.WriteString(data)
				} else if strings.HasPrefix(line, "id:") {
					eventID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
				}
			}
		}
	}()

	return events, nil
}

// Close closes the transport
func (t *Transport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.closed = true
	t.activeClient = nil

	return nil
}

// Compression helpers
func compressBrotli(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := brotli.NewWriterLevel(&buf, brotli.BestCompression)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decompressBrotli(data []byte) ([]byte, error) {
	reader := brotli.NewReader(bytes.NewReader(data))
	return io.ReadAll(reader)
}

func compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	return io.ReadAll(reader)
}
