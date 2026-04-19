package handlers

import (
	"net/http"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// SessionHandler handles session management endpoints.
//
// Writes to per-session fields (Status, Context, LastActivity,
// RequestCount) must happen inside a Store.Update callback so the
// mutation runs under the Store's write lock. Reads that copy
// fields also go through Update so they are serialised with
// concurrent writers (Pattern Beta). Previously the map and
// session fields were all unsynchronised — `-race` caught
// concurrent-write crashes in UpdateSessionContext (BUGFIX #29).
type SessionHandler struct {
	sessions *safe.Store[string, *models.UserSession]
	log      *logrus.Logger
}

// NewSessionHandler creates a new session handler
func NewSessionHandler(log *logrus.Logger) *SessionHandler {
	return &SessionHandler{
		sessions: safe.NewStore[string, *models.UserSession](),
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

	h.sessions.Put(sessionID, session)

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

	var (
		found    bool
		response SessionResponse
	)

	// Update-as-read so that the expired-flip mutation and the
	// field-snapshot both happen under the Store's write lock.
	h.sessions.Update(sessionID, func(s *models.UserSession, ok bool) (*models.UserSession, bool) {
		if !ok {
			return nil, false
		}
		found = true

		if time.Now().After(s.ExpiresAt) {
			s.Status = "expired"
		}

		response = SessionResponse{
			Success:      true,
			Message:      "Session retrieved successfully",
			SessionID:    s.ID,
			UserID:       s.UserID,
			Status:       s.Status,
			RequestCount: s.RequestCount,
			LastActivity: s.LastActivity,
			ExpiresAt:    s.ExpiresAt,
			CreatedAt:    s.CreatedAt,
		}
		if includeContext {
			response.Context = s.Context
		}
		return s, true
	})

	if !found {
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "session not found",
			"session_id": sessionID,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// TerminateSession handles DELETE /v1/sessions/:id
func (h *SessionHandler) TerminateSession(c *gin.Context) {
	sessionID := c.Param("id")
	graceful := c.Query("graceful") != "false" // Default to graceful

	var (
		found    bool
		response SessionResponse
	)

	h.sessions.Update(sessionID, func(s *models.UserSession, ok bool) (*models.UserSession, bool) {
		if !ok {
			return nil, false
		}
		found = true

		if graceful {
			s.Status = "terminated"
		}

		response = SessionResponse{
			Success:      true,
			Message:      "Session terminated successfully",
			SessionID:    sessionID,
			UserID:       s.UserID,
			Status:       "terminated",
			RequestCount: s.RequestCount,
			LastActivity: s.LastActivity,
			ExpiresAt:    s.ExpiresAt,
			CreatedAt:    s.CreatedAt,
		}

		if graceful {
			return s, true
		}
		return nil, false // non-graceful → delete
	})

	if !found {
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "session not found",
			"session_id": sessionID,
		})
		return
	}

	if graceful {
		h.log.WithField("session_id", sessionID).Info("Session terminated gracefully")
	} else {
		h.log.WithField("session_id", sessionID).Info("Session terminated immediately")
	}

	c.JSON(http.StatusOK, response)
}

// ListSessions handles GET /v1/sessions (admin endpoint)
func (h *SessionHandler) ListSessions(c *gin.Context) {
	userID := c.Query("user_id")
	status := c.Query("status")

	// Iterate keys and inspect each session under Update so the
	// expired-flip mutation and field reads happen atomically.
	var sessions []SessionResponse
	for _, id := range h.sessions.Keys() {
		h.sessions.Update(id, func(s *models.UserSession, ok bool) (*models.UserSession, bool) {
			if !ok {
				return nil, false
			}

			if time.Now().After(s.ExpiresAt) && s.Status == "active" {
				s.Status = "expired"
			}

			// Filter by user_id if provided
			if userID != "" && s.UserID != userID {
				return s, true
			}
			// Filter by status if provided
			if status != "" && s.Status != status {
				return s, true
			}

			sessions = append(sessions, SessionResponse{
				SessionID:    s.ID,
				UserID:       s.UserID,
				Status:       s.Status,
				RequestCount: s.RequestCount,
				LastActivity: s.LastActivity,
				ExpiresAt:    s.ExpiresAt,
				CreatedAt:    s.CreatedAt,
			})
			return s, true
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// UpdateSessionContext updates the session context (internal use)
func (h *SessionHandler) UpdateSessionContext(sessionID string, context map[string]interface{}) error {
	h.sessions.Update(sessionID, func(s *models.UserSession, ok bool) (*models.UserSession, bool) {
		if !ok {
			return nil, false
		}

		if s.Context == nil {
			s.Context = make(map[string]interface{})
		}
		for k, v := range context {
			s.Context[k] = v
		}

		s.LastActivity = time.Now()
		s.RequestCount++
		return s, true
	})

	return nil
}

// GetSessionByID returns a session by ID (internal use).
// Returns a pointer; callers should treat the returned session as
// read-only and not mutate fields without routing through
// UpdateSessionContext (or another Update-callback path).
func (h *SessionHandler) GetSessionByID(sessionID string) *models.UserSession {
	s, _ := h.sessions.Get(sessionID)
	return s
}
