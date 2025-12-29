package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ProtocolSecurity provides authentication and authorization for protocols
type ProtocolSecurity struct {
	mu          sync.RWMutex
	apiKeys     map[string]*APIKey
	permissions map[string][]string // key -> permissions
	logger      *logrus.Logger
}

// APIKey represents an API key with permissions
type APIKey struct {
	Key         string
	Name        string
	Owner       string
	Permissions []string
	CreatedAt   time.Time
	LastUsed    time.Time
	Active      bool
}

// ProtocolAccessRequest represents a request for protocol access
type ProtocolAccessRequest struct {
	APIKey   string
	Protocol string
	Action   string
	Resource string
}

// NewProtocolSecurity creates a new protocol security manager
func NewProtocolSecurity(logger *logrus.Logger) *ProtocolSecurity {
	return &ProtocolSecurity{
		apiKeys:     make(map[string]*APIKey),
		permissions: make(map[string][]string),
		logger:      logger,
	}
}

// CreateAPIKey creates a new API key with permissions
func (s *ProtocolSecurity) CreateAPIKey(name, owner string, permissions []string) (*APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := generateSecureToken()

	apiKey := &APIKey{
		Key:         key,
		Name:        name,
		Owner:       owner,
		Permissions: permissions,
		CreatedAt:   time.Now(),
		Active:      true,
	}

	s.apiKeys[key] = apiKey
	s.permissions[key] = permissions

	s.logger.WithFields(logrus.Fields{
		"name":  name,
		"owner": owner,
	}).Info("API key created")

	return apiKey, nil
}

// ValidateAccess validates if an API key has access to a protocol operation
func (s *ProtocolSecurity) ValidateAccess(ctx context.Context, req ProtocolAccessRequest) error {
	s.mu.RLock()
	apiKey, exists := s.apiKeys[req.APIKey]
	s.mu.RUnlock()

	if !exists || !apiKey.Active {
		return fmt.Errorf("invalid API key")
	}

	// Check permissions
	permissions, exists := s.permissions[req.APIKey]
	if !exists {
		return fmt.Errorf("no permissions found for API key")
	}

	requiredPermission := fmt.Sprintf("%s:%s", req.Protocol, req.Action)

	// Check for exact match or wildcard
	for _, permission := range permissions {
		if permission == requiredPermission ||
			permission == fmt.Sprintf("%s:*", req.Protocol) ||
			permission == "*" {
			// Update last used time
			s.mu.Lock()
			apiKey.LastUsed = time.Now()
			s.mu.Unlock()

			return nil
		}
	}

	return fmt.Errorf("insufficient permissions for %s", requiredPermission)
}

// RevokeAPIKey revokes an API key
func (s *ProtocolSecurity) RevokeAPIKey(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if apiKey, exists := s.apiKeys[key]; exists {
		apiKey.Active = false
		delete(s.permissions, key)

		s.logger.WithField("key", key[:8]+"...").Info("API key revoked")
		return nil
	}

	return fmt.Errorf("API key not found")
}

// ListAPIKeys returns all API keys (for admin purposes)
func (s *ProtocolSecurity) ListAPIKeys() []*APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]*APIKey, 0, len(s.apiKeys))
	for _, key := range s.apiKeys {
		keys = append(keys, key)
	}

	return keys
}

// InitializeDefaultSecurity sets up default security configuration
func (s *ProtocolSecurity) InitializeDefaultSecurity() error {
	// Create admin key with full access
	adminKey, err := s.CreateAPIKey("admin-key", "system", []string{
		"*", // Full access
	})
	if err != nil {
		return fmt.Errorf("failed to create admin key: %w", err)
	}

	s.logger.WithField("key", adminKey.Key[:8]+"...").Info("Admin API key created")

	// Create user key with limited access
	userKey, err := s.CreateAPIKey("user-key", "demo", []string{
		"mcp:read",
		"mcp:execute",
		"lsp:read",
		"lsp:execute",
		"acp:read",
		"acp:execute",
		"embedding:read",
		"embedding:execute",
	})
	if err != nil {
		return fmt.Errorf("failed to create user key: %w", err)
	}

	s.logger.WithField("key", userKey.Key[:8]+"...").Info("User API key created")

	return nil
}

// Rate limiting (basic implementation)

type RateLimiter struct {
	mu        sync.Mutex
	requests  map[string][]time.Time
	maxPerMin int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxPerMin int) *RateLimiter {
	return &RateLimiter{
		requests:  make(map[string][]time.Time),
		maxPerMin: maxPerMin,
	}
}

// Allow checks if a request should be allowed
func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-time.Minute)

	// Clean old requests
	if requests, exists := r.requests[key]; exists {
		valid := make([]time.Time, 0)
		for _, t := range requests {
			if t.After(windowStart) {
				valid = append(valid, t)
			}
		}
		r.requests[key] = valid

		// Check if under limit
		if len(valid) < r.maxPerMin {
			r.requests[key] = append(r.requests[key], now)
			return true
		}
		return false
	}

	// First request
	r.requests[key] = []time.Time{now}
	return true
}

// Global rate limiter
var GlobalRateLimiter = NewRateLimiter(100) // 100 requests per minute per API key

// Utility functions

func generateSecureToken() string {
	return fmt.Sprintf("sk-%s", generateID())
}

func generateID() string {
	// Simple ID generation - in production, use crypto/rand
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Middleware helpers

// ExtractAPIKeyFromHeader extracts API key from request headers
func ExtractAPIKeyFromHeader(headerValue string) string {
	if strings.HasPrefix(headerValue, "Bearer ") {
		return strings.TrimPrefix(headerValue, "Bearer ")
	}
	return headerValue
}

// ValidateProtocolAccess is a convenience function for protocol access validation
func (s *ProtocolSecurity) ValidateProtocolAccess(ctx context.Context, apiKey, protocol, action, resource string) error {
	return s.ValidateAccess(ctx, ProtocolAccessRequest{
		APIKey:   apiKey,
		Protocol: protocol,
		Action:   action,
		Resource: resource,
	})
}
