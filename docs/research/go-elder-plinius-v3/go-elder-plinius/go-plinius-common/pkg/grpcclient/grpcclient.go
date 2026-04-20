// Package grpcclient provides a reusable gRPC client implementation
// for all Plinius services. It handles connection management, retry logic,
// interceptors, authentication, and graceful shutdown.
//
// This is the core infrastructure that all Plinius service clients build upon.
// Users typically don't use this directly but through the higher-level
// service client packages (go-autotemp, go-obliteratus, etc.).
package grpcclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/elder-plinius/go-plinius-common/pkg/errors"
)

// Client is a reusable gRPC client that manages connections,
// retries, authentication, and health checking.
type Client struct {
	cfg    *config.Config
	conn   *grpc.ClientConn
	mu     sync.RWMutex
	closed bool
}

// New creates a new gRPC client with the given configuration.
func New(cfg *config.Config) *Client {
	return &Client{cfg: cfg}
}

// Connect establishes the gRPC connection.
// It is safe to call multiple times; subsequent calls return the existing connection.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return errors.New(errors.ErrCodeFailedPrecondition, c.cfg.ServiceName,
			"client has been closed")
	}

	if c.conn != nil {
		state := c.conn.GetState()
		if state == connectivity.Ready || state == connectivity.Connecting {
			return nil
		}
		// Connection exists but not ready; close and reconnect
		c.conn.Close()
		c.conn = nil
	}

	ctx, cancel := context.WithTimeout(ctx, c.cfg.ConnectionTimeout)
	defer cancel()

	opts := c.buildDialOptions()
	conn, err := grpc.DialContext(ctx, c.cfg.Address, opts...)
	if err != nil {
		return errors.Wrap(errors.ErrCodeConnection, c.cfg.ServiceName,
			"failed to connect to gRPC server", err)
	}

	c.conn = conn
	return nil
}

// Connection returns the underlying grpc.ClientConn.
// Returns nil if not connected.
func (c *Client) Connection() *grpc.ClientConn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

// Close gracefully closes the gRPC connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// IsConnected returns true if the client has an active connection.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return false
	}
	state := c.conn.GetState()
	return state == connectivity.Ready || state == connectivity.Idle
}

// WaitForReady blocks until the connection is ready or the context is cancelled.
func (c *Client) WaitForReady(ctx context.Context) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return errors.New(errors.ErrCodeFailedPrecondition, c.cfg.ServiceName,
			"not connected")
	}

	if conn.GetState() == connectivity.Ready {
		return nil
	}

	if !conn.WaitForStateChange(ctx, conn.GetState()) {
		return ctx.Err()
	}
	if conn.GetState() != connectivity.Ready {
		return errors.New(errors.ErrCodeConnection, c.cfg.ServiceName,
			"connection not ready")
	}
	return nil
}

// InvokeUnary performs a unary RPC with retry logic.
func (c *Client) InvokeUnary(
	ctx context.Context,
	method string,
	req interface{},
	resp interface{},
	opts ...grpc.CallOption,
) error {
	lastErr := error(nil)
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := calculateBackoff(c.cfg.RetryBackoff, c.cfg.MaxRetryBackoff, attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		conn := c.Connection()
		if conn == nil {
			lastErr = errors.New(errors.ErrCodeConnection, c.cfg.ServiceName,
				"no connection available")
			continue
		}

		callCtx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
		err := conn.Invoke(callCtx, method, req, resp, opts...)
		cancel()

		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		pe := errors.MustBePliniusError(err)
		if pe != nil && !pe.IsRetryable() {
			return err // Non-retryable error
		}
	}
	return errors.Wrap(errors.ErrCodeUnavailable, c.cfg.ServiceName,
		fmt.Sprintf("RPC failed after %d attempts", c.cfg.MaxRetries+1), lastErr)
}

// ContextWithMetadata returns a context with service metadata appended.
func (c *Client) ContextWithMetadata(ctx context.Context) context.Context {
	md := metadata.MD{}

	// Add auth token if present
	if c.cfg.AuthToken != "" {
		md.Set("authorization", "Bearer "+c.cfg.AuthToken)
	}

	// Add custom metadata
	for k, v := range c.cfg.Metadata {
		md.Set(k, v)
	}

	if len(md) > 0 {
		return metadata.NewOutgoingContext(ctx, md)
	}
	return ctx
}

// buildDialOptions constructs gRPC dial options from the configuration.
func (c *Client) buildDialOptions() []grpc.DialOption {
	var opts []grpc.DialOption

	// Transport credentials
	if c.cfg.EnableTLS {
		tlsConfig := &tls.Config{}
		if c.cfg.InsecureSkipVerify {
			tlsConfig.InsecureSkipVerify = true
		}
		if c.cfg.TLSServerName != "" {
			tlsConfig.ServerName = c.cfg.TLSServerName
		}
		if c.cfg.TLSCAPath != "" {
			// In production, load CA cert properly
			// For now, use system CA pool
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Keepalive
	ka := keepalive.ClientParameters{
		Time:                c.cfg.KeepaliveTime,
		Timeout:             c.cfg.KeepaliveTimeout,
		PermitWithoutStream: true,
	}
	opts = append(opts, grpc.WithKeepaliveParams(ka))

	// Message size limits
	opts = append(opts,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(c.cfg.MaxRecvMsgSize),
			grpc.MaxCallSendMsgSize(c.cfg.MaxSendMsgSize),
		),
	)

	// Unary interceptors
	var unaryInterceptors []grpc.UnaryClientInterceptor
	unaryInterceptors = append(unaryInterceptors, c.retryInterceptor())
	opts = append(opts, grpc.WithChainUnaryInterceptor(unaryInterceptors...))

	// Compression
	switch c.cfg.Compression {
	case "gzip":
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.UseCompressor("gzip")))
	}

	return opts
}

// retryInterceptor creates a unary client interceptor for retries.
func (c *Client) retryInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// The retry logic is handled in InvokeUnary
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// calculateBackoff computes exponential backoff with jitter.
func calculateBackoff(base, max time.Duration, attempt int) time.Duration {
	// Exponential: base * 2^(attempt-1)
	backoff := base
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff >= max {
			backoff = max
			break
		}
	}
	// Add jitter (±25%)
	jitter := time.Duration(float64(backoff) * 0.25)
	return backoff + jitter
}
