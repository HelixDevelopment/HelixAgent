// Package claude_code provides session management for Claude Code.
package claude_code

import (
	"context"
	"time"

	"digital.vasic.concurrency/pkg/safe"
)

// SessionManager manages Claude Code sessions
type SessionManager struct {
	sessions    *safe.Store[string, *Session]
	config      *Config
	cleanupTick *time.Ticker
	stopCleanup chan struct{}
}

// NewSessionManager creates a new session manager
func NewSessionManager(config *Config) *SessionManager {
	sm := &SessionManager{
		sessions:    safe.NewStore[string, *Session](),
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
	session := NewSession(workDir, sm.config)
	sm.sessions.Put(session.ID, session)
	return session
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(id string) (*Session, bool) {
	return sm.sessions.Get(id)
}

// GetOrCreateSession gets an existing session or creates a new one
func (sm *SessionManager) GetOrCreateSession(id, workDir string) *Session {
	var existing *Session
	sm.sessions.Update(id, func(s *Session, ok bool) (*Session, bool) {
		if ok && s.Active {
			s.LastActivity = time.Now()
			existing = s
			return s, true
		}
		return s, ok
	})
	if existing != nil {
		return existing
	}

	session := NewSession(workDir, sm.config)
	sm.sessions.Put(session.ID, session)
	return session
}

// EndSession ends a session
func (sm *SessionManager) EndSession(id string) bool {
	found := false
	sm.sessions.Update(id, func(s *Session, ok bool) (*Session, bool) {
		if !ok {
			return nil, false
		}
		s.Active = false
		found = true
		return s, true
	})
	return found
}

// DeleteSession completely removes a session
func (sm *SessionManager) DeleteSession(id string) bool {
	_, existed := sm.sessions.Delete(id)
	return existed
}

// ListSessions returns all sessions
func (sm *SessionManager) ListSessions() []*Session {
	return sm.sessions.Values()
}

// ListActiveSessions returns only active sessions
func (sm *SessionManager) ListActiveSessions() []*Session {
	var sessions []*Session
	sm.sessions.Range(func(_ string, s *Session) bool {
		if s.Active {
			sessions = append(sessions, s)
		}
		return true
	})
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
	for _, id := range sm.sessions.Keys() {
		sm.sessions.Update(id, func(s *Session, ok bool) (*Session, bool) {
			if !ok {
				return nil, false
			}
			if s.IsExpired(sm.config.TimeoutMinutes) {
				s.Active = false
				return nil, false
			}
			return s, true
		})
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
	return sm.sessions.Len()
}

// GetActiveSessionCount returns the number of active sessions
func (sm *SessionManager) GetActiveSessionCount() int {
	count := 0
	sm.sessions.Range(func(_ string, s *Session) bool {
		if s.Active {
			count++
		}
		return true
	})
	return count
}
