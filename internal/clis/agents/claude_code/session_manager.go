// Package claude_code provides session management for Claude Code.
package claude_code

import (
	"context"
	"sync"
	"time"
)

// SessionManager manages Claude Code sessions
type SessionManager struct {
	sessions    map[string]*Session
	mu          sync.RWMutex
	config      *Config
	cleanupTick *time.Ticker
	stopCleanup chan struct{}
}

// NewSessionManager creates a new session manager
func NewSessionManager(config *Config) *SessionManager {
	sm := &SessionManager{
		sessions:    make(map[string]*Session),
		config:      config,
		stopCleanup: make(chan struct{}),
	}

	// Start cleanup goroutine
	if config.TimeoutMinutes > 0 {
		sm.cleanupTick = time.NewTicker(1 * time.Minute)
		go sm.cleanupLoop()
	}

	return sm
}

// CreateSession creates a new session
func (sm *SessionManager) CreateSession(workDir string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session := NewSession(workDir, sm.config)
	sm.sessions[session.ID] = session
	return session
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(id string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[id]
	return session, ok
}

// GetOrCreateSession gets an existing session or creates a new one
func (sm *SessionManager) GetOrCreateSession(id, workDir string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, ok := sm.sessions[id]; ok && session.Active {
		session.LastActivity = time.Now()
		return session
	}

	session := NewSession(workDir, sm.config)
	sm.sessions[session.ID] = session
	return session
}

// EndSession ends a session
func (sm *SessionManager) EndSession(id string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, ok := sm.sessions[id]; ok {
		session.Active = false
		return true
	}
	return false
}

// DeleteSession completely removes a session
func (sm *SessionManager) DeleteSession(id string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.sessions[id]; ok {
		delete(sm.sessions, id)
		return true
	}
	return false
}

// ListSessions returns all sessions
func (sm *SessionManager) ListSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*Session, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// ListActiveSessions returns only active sessions
func (sm *SessionManager) ListActiveSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var sessions []*Session
	for _, session := range sm.sessions {
		if session.Active {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// cleanupLoop periodically cleans up expired sessions
func (sm *SessionManager) cleanupLoop() {
	for {
		select {
		case <-sm.cleanupTick.C:
			sm.cleanupExpired()
		case <-sm.stopCleanup:
			return
		}
	}
}

// cleanupExpired removes expired sessions
func (sm *SessionManager) cleanupExpired() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for id, session := range sm.sessions {
		if session.IsExpired(sm.config.TimeoutMinutes) {
			session.Active = false
			delete(sm.sessions, id)
		}
	}
}

// Stop stops the session manager
func (sm *SessionManager) Stop(ctx context.Context) error {
	if sm.cleanupTick != nil {
		sm.cleanupTick.Stop()
		close(sm.stopCleanup)
	}
	return nil
}

// GetSessionCount returns the total number of sessions
func (sm *SessionManager) GetSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// GetActiveSessionCount returns the number of active sessions
func (sm *SessionManager) GetActiveSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	count := 0
	for _, session := range sm.sessions {
		if session.Active {
			count++
		}
	}
	return count
}
