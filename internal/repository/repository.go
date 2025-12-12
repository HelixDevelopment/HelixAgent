package repository

import (
	"context"
	"time"

	"github.com/superagent/superagent/internal/models"
)

// UserRepository defines operations for user data
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	FindByID(ctx context.Context, id string) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByAPIKey(ctx context.Context, apiKey string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, limit, offset int) ([]*models.User, error)
	Count(ctx context.Context) (int, error)
}

// SessionRepository defines operations for user session data
type SessionRepository interface {
	Create(ctx context.Context, session *models.UserSession) error
	FindByID(ctx context.Context, id string) (*models.UserSession, error)
	FindByToken(ctx context.Context, token string) (*models.UserSession, error)
	FindByUserID(ctx context.Context, userID string) ([]*models.UserSession, error)
	Update(ctx context.Context, session *models.UserSession) error
	Delete(ctx context.Context, id string) error
	DeleteExpired(ctx context.Context) (int64, error)
	UpdateLastActivity(ctx context.Context, id string, lastActivity time.Time) error
}

// LLMRequestRepository defines operations for LLM request data
type LLMRequestRepository interface {
	Create(ctx context.Context, request *models.LLMRequest) error
	FindByID(ctx context.Context, id string) (*models.LLMRequest, error)
	FindBySessionID(ctx context.Context, sessionID string, limit, offset int) ([]*models.LLMRequest, error)
	FindByUserID(ctx context.Context, userID string, limit, offset int) ([]*models.LLMRequest, error)
	UpdateStatus(ctx context.Context, id string, status string, startedAt, completedAt *time.Time) error
	UpdateResponse(ctx context.Context, id string, providerID, responseContent string, tokensUsed, responseTime int) error
	UpdateError(ctx context.Context, id string, errorMessage string) error
	Delete(ctx context.Context, id string) error
	CountBySessionID(ctx context.Context, sessionID string) (int, error)
	CountByUserID(ctx context.Context, userID string) (int, error)
}

// LLMResponseRepository defines operations for LLM response data
type LLMResponseRepository interface {
	Create(ctx context.Context, response *models.LLMResponse) error
	FindByID(ctx context.Context, id string) (*models.LLMResponse, error)
	FindByRequestID(ctx context.Context, requestID string) ([]*models.LLMResponse, error)
	FindSelectedByRequestID(ctx context.Context, requestID string) (*models.LLMResponse, error)
	UpdateSelected(ctx context.Context, id string, selected bool, selectionScore float64) error
	Delete(ctx context.Context, id string) error
	DeleteByRequestID(ctx context.Context, requestID string) error
}

// MemoryRepository defines operations for memory data
type MemoryRepository interface {
	Create(ctx context.Context, memory *models.CogneeMemory) error
	FindByID(ctx context.Context, id string) (*models.CogneeMemory, error)
	FindBySessionID(ctx context.Context, sessionID string, limit, offset int) ([]*models.CogneeMemory, error)
	FindByDatasetName(ctx context.Context, datasetName string, limit, offset int) ([]*models.CogneeMemory, error)
	FindBySearchKey(ctx context.Context, searchKey string, limit, offset int) ([]*models.CogneeMemory, error)
	Update(ctx context.Context, memory *models.CogneeMemory) error
	Delete(ctx context.Context, id string) error
	DeleteExpired(ctx context.Context) (int64, error)
	CountBySessionID(ctx context.Context, sessionID string) (int, error)
}

// ProviderRepository defines operations for LLM provider data
type ProviderRepository interface {
	Create(ctx context.Context, provider *models.LLMProvider) error
	FindByID(ctx context.Context, id string) (*models.LLMProvider, error)
	FindByName(ctx context.Context, name string) (*models.LLMProvider, error)
	FindAll(ctx context.Context, enabledOnly bool) ([]*models.LLMProvider, error)
	Update(ctx context.Context, provider *models.LLMProvider) error
	UpdateHealthStatus(ctx context.Context, id string, healthStatus string, responseTime int64) error
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context, enabledOnly bool) (int, error)
}

// Repository combines all repository interfaces
type Repository interface {
	Users() UserRepository
	Sessions() SessionRepository
	LLMRequests() LLMRequestRepository
	LLMResponses() LLMResponseRepository
	Memories() MemoryRepository
	Providers() ProviderRepository
	BeginTx(ctx context.Context) (Repository, error)
	Commit() error
	Rollback() error
	Close() error
}
