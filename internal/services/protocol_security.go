package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"digital.vasic.concurrency/pkg/safe"
)

// ProtocolSecurity provides authentication and authorization for protocols.
// Concurrent-safe by construction (CONST-029 Tier 1): apiKeys and
// permissions are independent safe.Stores (no method atomically touches
// both; CreateAPIKeyWithValue writes to both sequentially, which is safe
// because readers of one vs. the other never need joint consistency).
type ProtocolSecurity struct {
	apiKeys     *safe.Store[string, *APIKey]
	permissions *safe.Store[string, []string] // key -> permissions
	logger      *logrus.Logger
	// entropy is the source of cryptographic randomness for auto-generated
	// API keys. It is always crypto/rand.Reader in production (set by
	// NewProtocolSecurity); tests override it to exercise the
	// entropy-failure path. A nil entropy falls back to crypto/rand.Reader
	// so a zero-value ProtocolSecurity can never silently weaken key
	// generation.
	entropy io.Reader
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
		apiKeys:     safe.NewStore[string, *APIKey](),
		permissions: safe.NewStore[string, []string](),
		logger:      logger,
		entropy:     rand.Reader,
	}
}

// entropySource returns the configured entropy source, defaulting to
// crypto/rand.Reader.
func (s *ProtocolSecurity) entropySource() io.Reader {
	if s.entropy == nil {
		return rand.Reader
	}
	return s.entropy
}

// CreateAPIKey creates a new API key with permissions (auto-generates the key)
func (s *ProtocolSecurity) CreateAPIKey(name, owner string, permissions []string) (*APIKey, error) {
	key, err := generateSecureToken(s.entropySource())
	if err != nil {
		// Fail closed: no key is minted and none is stored, so nothing
		// downstream can mistake a weak value for a valid credential.
		return nil, fmt.Errorf("failed to generate API key %q: %w", name, err)
	}
	return s.CreateAPIKeyWithValue(name, owner, key, permissions)
}

// CreateAPIKeyWithValue creates an API key with a specific key value
// This is useful when you want to set the API key from an environment variable
func (s *ProtocolSecurity) CreateAPIKeyWithValue(name, owner, key string, permissions []string) (*APIKey, error) {
	apiKey := &APIKey{
		Key:         key,
		Name:        name,
		Owner:       owner,
		Permissions: permissions,
		CreatedAt:   time.Now(),
		Active:      true,
	}
	s.apiKeys.Put(key, apiKey)
	s.permissions.Put(key, permissions)

	s.logger.WithFields(logrus.Fields{
		"name":  name,
		"owner": owner,
	}).Info("API key created")

	return apiKey, nil
}

// ValidateAccess validates if an API key has access to a protocol operation.
// Active/LastUsed reads/writes happen inside Update callbacks to serialize
// with concurrent access (eliminated the pre-migration lock-upgrade IIFE).
func (s *ProtocolSecurity) ValidateAccess(ctx context.Context, req ProtocolAccessRequest) error {
	var activeOK bool
	s.apiKeys.Update(req.APIKey, func(k *APIKey, ok bool) (*APIKey, bool) {
		if !ok || !k.Active {
			return k, ok
		}
		activeOK = true
		return k, true
	})
	if !activeOK {
		return fmt.Errorf("invalid API key")
	}

	permissions, exists := s.permissions.Get(req.APIKey)
	if !exists {
		return fmt.Errorf("no permissions found for API key")
	}

	requiredPermission := fmt.Sprintf("%s:%s", req.Protocol, req.Action)

	for _, permission := range permissions {
		if permission == requiredPermission ||
			permission == fmt.Sprintf("%s:*", req.Protocol) ||
			permission == "*" {
			// Update last used time atomically.
			s.apiKeys.Update(req.APIKey, func(k *APIKey, ok bool) (*APIKey, bool) {
				if !ok {
					return nil, false
				}
				k.LastUsed = time.Now()
				return k, true
			})
			return nil
		}
	}

	return fmt.Errorf("insufficient permissions for %s", requiredPermission)
}

// RevokeAPIKey revokes an API key
func (s *ProtocolSecurity) RevokeAPIKey(key string) error {
	var found bool
	s.apiKeys.Update(key, func(k *APIKey, ok bool) (*APIKey, bool) {
		if !ok {
			return nil, false
		}
		k.Active = false
		found = true
		return k, true
	})
	if !found {
		return fmt.Errorf("API key not found")
	}
	s.permissions.Delete(key)
	s.logger.WithField("key", key[:8]+"...").Info("API key revoked")
	return nil
}

// ListAPIKeys returns all API keys (for admin purposes)
func (s *ProtocolSecurity) ListAPIKeys() []*APIKey {
	return s.apiKeys.Values()
}

// InitializeDefaultSecurity sets up default security configuration
func (s *ProtocolSecurity) InitializeDefaultSecurity() error {
	// Check if HELIXAGENT_API_KEY is set in environment
	// This allows external configuration of the admin API key
	envAPIKey := os.Getenv("HELIXAGENT_API_KEY")

	var adminKey *APIKey
	var err error

	if envAPIKey != "" {
		// Use the API key from environment variable
		adminKey, err = s.CreateAPIKeyWithValue("admin-key", "system", envAPIKey, []string{
			"*", // Full access
		})
		if err != nil {
			return fmt.Errorf("failed to create admin key from env: %w", err)
		}
		s.logger.WithField("key", adminKey.Key[:8]+"...").Info("Admin API key created from HELIXAGENT_API_KEY env var")
	} else {
		// Generate a new admin key
		adminKey, err = s.CreateAPIKey("admin-key", "system", []string{
			"*", // Full access
		})
		if err != nil {
			return fmt.Errorf("failed to create admin key: %w", err)
		}
		s.logger.WithField("key", adminKey.Key[:8]+"...").Info("Admin API key created (auto-generated)")
	}

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

// RateLimiter is a simple sliding-window rate limiter.
// Concurrent-safe by construction (CONST-029): requests is a safe.Store;
// the check-then-append compound operation happens atomically inside
// Store.Update.
type RateLimiter struct {
	requests  *safe.Store[string, []time.Time]
	maxPerMin int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxPerMin int) *RateLimiter {
	return &RateLimiter{
		requests:  safe.NewStore[string, []time.Time](),
		maxPerMin: maxPerMin,
	}
}

// Allow checks if a request should be allowed
func (r *RateLimiter) Allow(key string) bool {
	now := time.Now()
	windowStart := now.Add(-time.Minute)
	var allowed bool

	r.requests.Update(key, func(cur []time.Time, _ bool) ([]time.Time, bool) {
		valid := make([]time.Time, 0, len(cur))
		for _, t := range cur {
			if t.After(windowStart) {
				valid = append(valid, t)
			}
		}
		if len(valid) < r.maxPerMin {
			valid = append(valid, now)
			allowed = true
		}
		return valid, true
	})
	return allowed
}

// Global rate limiter (lazy-loaded)
var (
	globalRateLimiter     *RateLimiter
	globalRateLimiterOnce sync.Once
)

// GetGlobalRateLimiter returns the global rate limiter, initializing on first use.
func GetGlobalRateLimiter() *RateLimiter {
	globalRateLimiterOnce.Do(func() {
		globalRateLimiter = NewRateLimiter(100) // 100 requests per minute per API key
	})
	return globalRateLimiter
}

// Utility functions

// generateSecureToken returns a new API key, or an error if the entropy source
// cannot supply the required randomness.
//
// It FAILS CLOSED (HXC-221). There is deliberately no fallback: an API key is a
// credential matched against the `Authorization: Bearer` header by
// ExtractAPIKeyFromHeader + ValidateAccess, so substituting a predictable value
// (previously the wall clock) for randomness would issue a guessable
// credential that the caller could not distinguish from a sound one. Refusing
// is the only safe outcome; the caller must propagate the error.
func generateSecureToken(entropy io.Reader) (string, error) {
	id, err := generateID(entropy)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sk-%s", id), nil
}

// generateID returns 16 bytes of cryptographic randomness, hex-encoded, or an
// error if the entropy source cannot supply all 16 bytes. A short read is an
// error, never a partially-random result.
func generateID(entropy io.Reader) (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(entropy, b); err != nil {
		return "", fmt.Errorf("read 16 bytes of entropy: %w", err)
	}
	return hex.EncodeToString(b), nil
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
