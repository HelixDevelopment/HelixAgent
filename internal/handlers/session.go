package handlers

import (
	"net/http"
	"sync"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// SessionHandler handles session management endpoints.
//
// All access to `sessions` AND to the per-session fields
// (`session.Context`, `session.LastActivity`, `session.RequestCount`)
// must go through `mu`. Previously the map and session fields were
// all unsynchronised — `-race` caught concurrent-write crashes in
// UpdateSessionContext (BUGFIX #29).
type SessionHandler struct {
	mu       sync.RWMutex
	sessions map[string]*models.UserSession
	log      *logrus.Logger
}

// NewSessionHandler creates a new session handler
func NewSessionHandler(log *logrus.Logger) *SessionHandler {
	return &SessionHandler{
		sessions: make(map[string]*models.UserSession),
		log:      log,
	}
}

// SessionCreateRequest represents a request to create a new session
type SessionCreateRequest struct {
	UserID         string                 `json:"user_id" binding:"required"`
	InitialContext map[string]interface{} `json:"initial_context"`
	TTLHours       int                    `json:"ttl_hours"`
	MemoryEnabled  bool                   `json:"memory_enabled"`
}

// SessionResponse represents a session response
type SessionResponse struct {
	Success      bool                   `json:"success"`
	Message      string                 `json:"message"`
	SessionID    string                 `json:"session_id"`
	UserID       string                 `json:"user_id"`
	Status       string                 `json:"status"`
	RequestCount int                    `json:"request_count"`
	LastActivity time.Time              `json:"last_activity"`
	ExpiresAt    time.Time              `json:"expires_at"`
	Context      map[string]interface{} `json:"context,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

// CreateSession handles POST /v1/sessions
func (h *SessionHandler) CreateSession(c *gin.Context) {
	var req SessionCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.WithError(err).Error("Failed to bind create session request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set default TTL if not provided
	ttlHours := req.TTLHours
	if ttlHours <= 0 {
		ttlHours = 24 // Default 24 hours
	}
	if ttlHours > 168 { // Max 7 days
		ttlHours = 168
	}

	now := time.Now()
	sessionID := uuid.New().String()
	sessionToken := uuid.New().String()

	// Create memory ID if memory is enabled
	var memoryID *string
	if req.MemoryEnabled {
		mid := uuid.New().String()
		memoryID = &mid
	}

	session := &models.UserSession{
		ID:           sessionID,
		UserID:       req.UserID,
		SessionToken: sessionToken,
		Context:      req.InitialContext,
		MemoryID:     memoryID,
		Status:       "active",
		RequestCount: 0,
		LastActivity: now,
		ExpiresAt:    now.Add(time.Duration(ttlHours) * time.Hour),
		CreatedAt:    now,
	}

	h.mu.Lock()
	h.sessions[sessionID] = session
	h.mu.Unlock()

	h.log.WithFields(logrus.Fields{
		"session_id": sessionID,
		"user_id":    req.UserID,
		"ttl_hours":  ttlHours,
	}).Info("Session created successfully")

	c.JSON(http.StatusCreated, SessionResponse{
		Success:      true,
		Message:      "Session created successfully",
		SessionID:    sessionID,
		UserID:       req.UserID,
		Status:       "active",
		RequestCount: 0,
		LastActivity: now,
		ExpiresAt:    session.ExpiresAt,
		Context:      req.InitialContext,
		CreatedAt:    now,
	})
}

// GetSession handles GET /v1/sessions/:id
func (h *SessionHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("id")
	includeContext := c.Query("includeContext") == "true"

	h.mu.Lock() // write lock because we may mutate session.Status below
	session, exists := h.sessions[sessionID]
	if !exists {
		h.mu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "session not found",
			"session_id": sessionID,
		})
		return
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		session.Status = "expired"
	}

	response := SessionResponse{
		Success:      true,
		Message:      "Session retrieved successfully",
		SessionID:    session.ID,
		UserID:       session.UserID,
		Status:       session.Status,
		RequestCount: session.RequestCount,
		LastActivity: session.LastActivity,
		ExpiresAt:    session.ExpiresAt,
		CreatedAt:    session.CreatedAt,
	}

	if includeContext {
		response.Context = session.Context
	}
	h.mu.Unlock()

	c.JSON(http.StatusOK, response)
}

// TerminateSession handles DELETE /v1/sessions/:id
func (h *SessionHandler) TerminateSession(c *gin.Context) {
	sessionID := c.Param("id")
	graceful := c.Query("graceful") != "false" // Default to graceful

	h.mu.Lock()
	session, exists := h.sessions[sessionID]
	if !exists {
		h.mu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "session not found",
			"session_id": sessionID,
		})
		return
	}

	if graceful {
		// Graceful termination - mark as terminated but keep for reference
		session.Status = "terminated"
		h.log.WithField("session_id", sessionID).Info("Session terminated gracefully")
	} else {
		// Immediate termination - remove from memory
		delete(h.sessions, sessionID)
		h.log.WithField("session_id", sessionID).Info("Session terminated immediately")
	}

	// Snapshot response fields while holding the lock.
	response := SessionResponse{
		Success:      true,
		Message:      "Session terminated successfully",
		SessionID:    sessionID,
		UserID:       session.UserID,
		Status:       "terminated",
		RequestCount: session.RequestCount,
		LastActivity: session.LastActivity,
		ExpiresAt:    session.ExpiresAt,
		CreatedAt:    session.CreatedAt,
	}
	h.mu.Unlock()

	c.JSON(http.StatusOK, response)
}

// ListSessions handles GET /v1/sessions (admin endpoint)
func (h *SessionHandler) ListSessions(c *gin.Context) {
	userID := c.Query("user_id")
	status := c.Query("status")

	var sessions []SessionResponse
	h.mu.Lock() // write lock — session.Status may be mutated below
	defer h.mu.Unlock()
	for _, session := range h.sessions {
		// Check if session is expired
		if time.Now().After(session.ExpiresAt) && session.Status == "active" {
			session.Status = "expired"
		}

		// Filter by user_id if provided
		if userID != "" && session.UserID != userID {
			continue
		}

		// Filter by status if provided
		if status != "" && session.Status != status {
			continue
		}

		sessions = append(sessions, SessionResponse{
			SessionID:    session.ID,
			UserID:       session.UserID,
			Status:       session.Status,
			RequestCount: session.RequestCount,
			LastActivity: session.LastActivity,
			ExpiresAt:    session.ExpiresAt,
			CreatedAt:    session.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// UpdateSessionContext updates the session context (internal use)
func (h *SessionHandler) UpdateSessionContext(sessionID string, context map[string]interface{}) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	session, exists := h.sessions[sessionID]
	if !exists {
		return nil
	}

	if session.Context == nil {
		session.Context = make(map[string]interface{})
	}

	for k, v := range context {
		session.Context[k] = v
	}

	session.LastActivity = time.Now()
	session.RequestCount++

	return nil
}

// GetSessionByID returns a session by ID (internal use).
// Returns a pointer; callers should treat the returned session as
// read-only and not mutate fields without taking h.mu themselves.
func (h *SessionHandler) GetSessionByID(sessionID string) *models.UserSession {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sessions[sessionID]
}
