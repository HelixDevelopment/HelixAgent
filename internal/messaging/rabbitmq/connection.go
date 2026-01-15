package rabbitmq

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// ConnectionState represents the state of the connection
type ConnectionState int32

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateClosed
)

// Connection manages a RabbitMQ connection with automatic reconnection
type Connection struct {
	config     *Config
	logger     *zap.Logger
	conn       *amqp.Connection
	mu         sync.RWMutex
	state      atomic.Int32
	closeCh    chan struct{}
	notifyCh   chan *amqp.Error
	reconnects int64

	// Callbacks
	onConnect    []func()
	onDisconnect []func(error)
	onReconnect  []func()
}

// NewConnection creates a new RabbitMQ connection manager
func NewConnection(config *Config, logger *zap.Logger) *Connection {
	if config == nil {
		config = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	c := &Connection{
		config:   config,
		logger:   logger,
		closeCh:  make(chan struct{}),
	}
	c.state.Store(int32(StateDisconnected))
	return c
}

// Connect establishes a connection to RabbitMQ
func (c *Connection) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ConnectionState(c.state.Load()) == StateConnected {
		return nil
	}

	c.state.Store(int32(StateConnecting))

	conn, err := c.dial(ctx)
	if err != nil {
		c.state.Store(int32(StateDisconnected))
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	c.conn = conn
	c.state.Store(int32(StateConnected))

	// Set up connection close notification
	c.notifyCh = make(chan *amqp.Error, 1)
	c.conn.NotifyClose(c.notifyCh)

	// Start reconnection monitor
	go c.handleReconnect()

	// Call connect callbacks
	for _, cb := range c.onConnect {
		go cb()
	}

	c.logger.Info("Connected to RabbitMQ",
		zap.String("host", c.config.Host),
		zap.Int("port", c.config.Port),
		zap.String("vhost", c.config.VHost))

	return nil
}

// dial creates the actual connection
func (c *Connection) dial(ctx context.Context) (*amqp.Connection, error) {
	url := c.buildURL()

	amqpConfig := amqp.Config{
		Heartbeat: c.config.HeartbeatInterval,
		Locale:    "en_US",
	}

	if c.config.TLSEnabled {
		tlsConfig := c.config.TLSConfig
		if tlsConfig == nil {
			tlsConfig = &tls.Config{
				InsecureSkipVerify: c.config.TLSSkipVerify,
			}
		}
		amqpConfig.TLSClientConfig = tlsConfig
	}

	// Create connection with timeout
	connCh := make(chan *amqp.Connection, 1)
	errCh := make(chan error, 1)

	go func() {
		conn, err := amqp.DialConfig(url, amqpConfig)
		if err != nil {
			errCh <- err
			return
		}
		connCh <- conn
	}()

	select {
	case conn := <-connCh:
		return conn, nil
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(c.config.ConnectionTimeout):
		return nil, fmt.Errorf("connection timeout after %v", c.config.ConnectionTimeout)
	}
}

// buildURL constructs the AMQP URL
func (c *Connection) buildURL() string {
	scheme := "amqp"
	if c.config.TLSEnabled {
		scheme = "amqps"
	}
	return fmt.Sprintf("%s://%s:%s@%s:%d/%s",
		scheme,
		c.config.Username,
		c.config.Password,
		c.config.Host,
		c.config.Port,
		c.config.VHost)
}

// handleReconnect monitors for disconnection and attempts reconnection
func (c *Connection) handleReconnect() {
	for {
		select {
		case <-c.closeCh:
			return
		case err := <-c.notifyCh:
			if err == nil {
				// Normal close
				return
			}

			c.logger.Warn("RabbitMQ connection lost",
				zap.Error(err))

			// Call disconnect callbacks
			for _, cb := range c.onDisconnect {
				go cb(err)
			}

			c.state.Store(int32(StateReconnecting))

			// Attempt reconnection
			if err := c.reconnect(); err != nil {
				c.logger.Error("Failed to reconnect to RabbitMQ",
					zap.Error(err))
				c.state.Store(int32(StateDisconnected))
				return
			}
		}
	}
}

// reconnect attempts to reconnect with exponential backoff
func (c *Connection) reconnect() error {
	delay := c.config.ReconnectDelay
	attempts := 0

	for {
		select {
		case <-c.closeCh:
			return fmt.Errorf("connection closed during reconnection")
		default:
		}

		if c.config.MaxReconnectCount > 0 && attempts >= c.config.MaxReconnectCount {
			return fmt.Errorf("max reconnection attempts (%d) exceeded", c.config.MaxReconnectCount)
		}

		attempts++
		atomic.AddInt64(&c.reconnects, 1)

		c.logger.Info("Attempting to reconnect to RabbitMQ",
			zap.Int("attempt", attempts),
			zap.Duration("delay", delay))

		time.Sleep(delay)

		ctx, cancel := context.WithTimeout(context.Background(), c.config.ConnectionTimeout)
		conn, err := c.dial(ctx)
		cancel()

		if err != nil {
			c.logger.Warn("Reconnection attempt failed",
				zap.Int("attempt", attempts),
				zap.Error(err))

			// Exponential backoff
			delay = time.Duration(float64(delay) * c.config.ReconnectBackoff)
			if delay > c.config.MaxReconnectDelay {
				delay = c.config.MaxReconnectDelay
			}
			continue
		}

		c.mu.Lock()
		c.conn = conn
		c.notifyCh = make(chan *amqp.Error, 1)
		c.conn.NotifyClose(c.notifyCh)
		c.state.Store(int32(StateConnected))
		c.mu.Unlock()

		c.logger.Info("Successfully reconnected to RabbitMQ",
			zap.Int("attempts", attempts))

		// Call reconnect callbacks
		for _, cb := range c.onReconnect {
			go cb()
		}

		return nil
	}
}

// Close closes the connection
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ConnectionState(c.state.Load()) == StateClosed {
		return nil
	}

	c.state.Store(int32(StateClosed))
	close(c.closeCh)

	if c.conn != nil && !c.conn.IsClosed() {
		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
	}

	c.logger.Info("RabbitMQ connection closed")
	return nil
}

// GetConnection returns the underlying AMQP connection
func (c *Connection) GetConnection() *amqp.Connection {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

// Channel creates a new channel
func (c *Connection) Channel() (*amqp.Channel, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil || c.conn.IsClosed() {
		return nil, fmt.Errorf("connection is not available")
	}

	ch, err := c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

	return ch, nil
}

// IsConnected returns true if connected
func (c *Connection) IsConnected() bool {
	return ConnectionState(c.state.Load()) == StateConnected
}

// State returns the current connection state
func (c *Connection) State() ConnectionState {
	return ConnectionState(c.state.Load())
}

// ReconnectCount returns the number of reconnection attempts
func (c *Connection) ReconnectCount() int64 {
	return atomic.LoadInt64(&c.reconnects)
}

// OnConnect registers a callback for connection events
func (c *Connection) OnConnect(cb func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onConnect = append(c.onConnect, cb)
}

// OnDisconnect registers a callback for disconnection events
func (c *Connection) OnDisconnect(cb func(error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onDisconnect = append(c.onDisconnect, cb)
}

// OnReconnect registers a callback for reconnection events
func (c *Connection) OnReconnect(cb func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onReconnect = append(c.onReconnect, cb)
}

// String returns the connection state as a string
func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	case StateClosed:
		return "closed"
	default:
		return "unknown"
	}
}
