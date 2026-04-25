// Package health provides early-bind operability endpoints that are
// available before the main HelixAgent HTTP server is ready.
//
// Drainage report 2026-04-25 Finding #2: the main HTTP server (on
// HelixAgentHTTP / 8100) does not bind until the ~7-min startup
// verification pipeline completes. This violates CLAUDE.md rule #6
// ("Every service MUST expose health endpoints") during the boot
// window — supervisors, watchdogs, and load-balancers cannot
// distinguish "starting up" from "hung" for ~7 minutes.
//
// The Liveness server here listens on a dedicated port
// (HelixAgentLiveness / 8111) the moment the binary starts and reports
// status=starting until SetReady() is called by main, at which point
// it reports status=ready. The server stays up for the lifetime of the
// process so external probes always have a deterministic endpoint.
package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/ports"
)

// Liveness is a minimal HTTP server that exposes /health and /readyz on
// the early-bind liveness port. It is safe to call SetReady from any
// goroutine; only the most recent state is observable.
type Liveness struct {
	server     *http.Server
	startedAt  time.Time
	readyAtRaw atomic.Int64 // unix nano of the SetReady call, 0 if not yet ready
	logger     *logrus.Logger
}

// NewLiveness constructs a Liveness server bound to ports.Addr(HelixAgentLiveness).
// It does not start listening until Start() is called.
func NewLiveness(logger *logrus.Logger) *Liveness {
	if logger == nil {
		logger = logrus.New()
	}
	l := &Liveness{
		startedAt: time.Now(),
		logger:    logger,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", l.handle)
	mux.HandleFunc("/readyz", l.handle)
	mux.HandleFunc("/livez", l.handle)
	l.server = &http.Server{
		Addr:              ports.Addr(ports.HelixAgentLiveness),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return l
}

// Start binds and serves in a background goroutine. It returns once the
// listener has successfully bound, or returns the bind error
// synchronously so main can fall back / log clearly.
func (l *Liveness) Start() error {
	listener, err := net.Listen("tcp", l.server.Addr)
	if err != nil {
		return fmt.Errorf("liveness: bind %s: %w", l.server.Addr, err)
	}
	l.logger.WithField("addr", l.server.Addr).Info("Liveness probe listening (status=starting)")
	go func() {
		if err := l.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			l.logger.WithError(err).Warn("Liveness probe server exited")
		}
	}()
	return nil
}

// SetReady marks the binary as fully booted. After this call, /health
// returns status=ready instead of status=starting. Idempotent.
func (l *Liveness) SetReady() {
	if l.readyAtRaw.Load() == 0 {
		l.readyAtRaw.Store(time.Now().UnixNano())
		l.logger.WithField("startup_elapsed_ms", time.Since(l.startedAt).Milliseconds()).
			Info("Liveness probe transitioning to status=ready")
	}
}

// Shutdown stops the liveness server.
func (l *Liveness) Shutdown(ctx context.Context) error {
	return l.server.Shutdown(ctx)
}

// status payload shape kept stable so external probes can rely on it.
type statusResp struct {
	Status     string `json:"status"`
	StartedAt  string `json:"started_at"`
	ElapsedMs  int64  `json:"elapsed_ms"`
	ReadyAt    string `json:"ready_at,omitempty"`
	ReadySince int64  `json:"ready_since_ms,omitempty"`
}

func (l *Liveness) handle(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	resp := statusResp{
		StartedAt: l.startedAt.UTC().Format(time.RFC3339Nano),
		ElapsedMs: now.Sub(l.startedAt).Milliseconds(),
	}
	if readyNs := l.readyAtRaw.Load(); readyNs > 0 {
		readyAt := time.Unix(0, readyNs)
		resp.Status = "ready"
		resp.ReadyAt = readyAt.UTC().Format(time.RFC3339Nano)
		resp.ReadySince = now.Sub(readyAt).Milliseconds()
	} else {
		resp.Status = "starting"
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
