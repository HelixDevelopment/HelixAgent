package router

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/config"
)

// GinRouter provides a wrapper around the Gin router with lifecycle management
type GinRouter struct {
	engine     *gin.Engine
	server     *http.Server
	config     *config.Config
	log        *logrus.Logger
	mu         sync.RWMutex
	running    bool
	startedAt  time.Time
	requestCnt int64
}

// GinRouterOption configures the GinRouter
type GinRouterOption func(*GinRouter)

// WithLogger sets a custom logger for the router
func WithLogger(log *logrus.Logger) GinRouterOption {
	return func(r *GinRouter) {
		r.log = log
	}
}

// WithGinMode sets the Gin mode (debug, release, test)
func WithGinMode(mode string) GinRouterOption {
	return func(r *GinRouter) {
		gin.SetMode(mode)
	}
}

// NewGinRouter creates a new GinRouter instance
func NewGinRouter(cfg *config.Config, opts ...GinRouterOption) *GinRouter {
	router := &GinRouter{
		config:  cfg,
		log:     logrus.New(),
		running: false,
	}

	// Apply options
	for _, opt := range opts {
		opt(router)
	}

	// Set default log level
	router.log.SetLevel(logrus.InfoLevel)

	// Create the Gin engine using the existing SetupRouter
	router.engine = SetupRouter(cfg)

	// Add request counting middleware
	router.engine.Use(router.requestCounterMiddleware())

	return router
}

// requestCounterMiddleware counts all incoming requests
func (r *GinRouter) requestCounterMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		r.mu.Lock()
		r.requestCnt++
		r.mu.Unlock()
		c.Next()
	}
}

// Engine returns the underlying Gin engine
func (r *GinRouter) Engine() *gin.Engine {
	return r.engine
}

// Start starts the HTTP server
func (r *GinRouter) Start(addr string) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("router is already running")
	}

	r.server = &http.Server{
		Addr:         addr,
		Handler:      r.engine,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	r.running = true
	r.startedAt = time.Now()
	r.mu.Unlock()

	r.log.WithField("addr", addr).Info("Starting HTTP server")

	if err := r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// StartTLS starts the HTTPS server with TLS
func (r *GinRouter) StartTLS(addr, certFile, keyFile string) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("router is already running")
	}

	r.server = &http.Server{
		Addr:         addr,
		Handler:      r.engine,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	r.running = true
	r.startedAt = time.Now()
	r.mu.Unlock()

	r.log.WithFields(logrus.Fields{
		"addr":     addr,
		"certFile": certFile,
	}).Info("Starting HTTPS server")

	if err := r.server.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the server
func (r *GinRouter) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	r.log.Info("Shutting down HTTP server...")

	if err := r.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	r.mu.Lock()
	r.running = false
	r.mu.Unlock()

	r.log.Info("HTTP server stopped")
	return nil
}

// IsRunning returns whether the server is currently running
func (r *GinRouter) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// GetStats returns router statistics
func (r *GinRouter) GetStats() RouterStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var uptime time.Duration
	if r.running {
		uptime = time.Since(r.startedAt)
	}

	return RouterStats{
		Running:      r.running,
		StartedAt:    r.startedAt,
		Uptime:       uptime,
		RequestCount: r.requestCnt,
	}
}

// RouterStats contains router statistics
type RouterStats struct {
	Running      bool          `json:"running"`
	StartedAt    time.Time     `json:"started_at"`
	Uptime       time.Duration `json:"uptime"`
	RequestCount int64         `json:"request_count"`
}

// RegisterRoutes allows registering additional routes on the engine
func (r *GinRouter) RegisterRoutes(fn func(*gin.Engine)) {
	fn(r.engine)
}

// AddMiddleware adds middleware to the router
func (r *GinRouter) AddMiddleware(middleware ...gin.HandlerFunc) {
	r.engine.Use(middleware...)
}

// ServeHTTP implements http.Handler interface
func (r *GinRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.engine.ServeHTTP(w, req)
}
