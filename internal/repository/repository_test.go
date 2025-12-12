package repository

import (
	"context"
	"testing"
	"time"

	"github.com/superagent/superagent/internal/models"
)

// TestUserRepositoryInterface tests that UserRepository interface is properly defined
func TestUserRepositoryInterface(t *testing.T) {
	var repo UserRepository
	_ = repo // Just to satisfy compiler
	t.Log("UserRepository interface is properly defined")
}

// TestSessionRepositoryInterface tests that SessionRepository interface is properly defined
func TestSessionRepositoryInterface(t *testing.T) {
	var repo SessionRepository
	_ = repo // Just to satisfy compiler
	t.Log("SessionRepository interface is properly defined")
}

// TestLLMRequestRepositoryInterface tests that LLMRequestRepository interface is properly defined
func TestLLMRequestRepositoryInterface(t *testing.T) {
	var repo LLMRequestRepository
	_ = repo // Just to satisfy compiler
	t.Log("LLMRequestRepository interface is properly defined")
}

// TestLLMResponseRepositoryInterface tests that LLMResponseRepository interface is properly defined
func TestLLMResponseRepositoryInterface(t *testing.T) {
	var repo LLMResponseRepository
	_ = repo // Just to satisfy compiler
	t.Log("LLMResponseRepository interface is properly defined")
}

// TestMemoryRepositoryInterface tests that MemoryRepository interface is properly defined
func TestMemoryRepositoryInterface(t *testing.T) {
	var repo MemoryRepository
	_ = repo // Just to satisfy compiler
	t.Log("MemoryRepository interface is properly defined")
}

// TestProviderRepositoryInterface tests that ProviderRepository interface is properly defined
func TestProviderRepositoryInterface(t *testing.T) {
	var repo ProviderRepository
	_ = repo // Just to satisfy compiler
	t.Log("ProviderRepository interface is properly defined")
}

// TestRepositoryInterface tests that Repository interface is properly defined
func TestRepositoryInterface(t *testing.T) {
	var repo Repository
	_ = repo // Just to satisfy compiler
	t.Log("Repository interface is properly defined")
}

// MockUserRepository is a mock implementation of UserRepository for testing
type MockUserRepository struct {
	users map[string]*models.User
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users: make(map[string]*models.User),
	}
}

func (m *MockUserRepository) Create(ctx context.Context, user *models.User) error {
	if user.ID == "" {
		return &RepositoryError{Message: "user ID is required"}
	}
	if _, exists := m.users[user.ID]; exists {
		return &RepositoryError{Message: "user already exists"}
	}
	m.users[user.ID] = user
	return nil
}

func (m *MockUserRepository) FindByID(ctx context.Context, id string) (*models.User, error) {
	user, exists := m.users[id]
	if !exists {
		return nil, &RepositoryError{Message: "user not found"}
	}
	return user, nil
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	for _, user := range m.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, &RepositoryError{Message: "user not found"}
}

func (m *MockUserRepository) FindByAPIKey(ctx context.Context, apiKey string) (*models.User, error) {
	for _, user := range m.users {
		if user.APIKey == apiKey {
			return user, nil
		}
	}
	return nil, &RepositoryError{Message: "user not found"}
}

func (m *MockUserRepository) Update(ctx context.Context, user *models.User) error {
	if _, exists := m.users[user.ID]; !exists {
		return &RepositoryError{Message: "user not found"}
	}
	m.users[user.ID] = user
	return nil
}

func (m *MockUserRepository) Delete(ctx context.Context, id string) error {
	if _, exists := m.users[id]; !exists {
		return &RepositoryError{Message: "user not found"}
	}
	delete(m.users, id)
	return nil
}

func (m *MockUserRepository) List(ctx context.Context, limit, offset int) ([]*models.User, error) {
	users := make([]*models.User, 0, len(m.users))
	for _, user := range m.users {
		users = append(users, user)
	}
	// Simple pagination
	if offset >= len(users) {
		return []*models.User{}, nil
	}
	end := offset + limit
	if end > len(users) {
		end = len(users)
	}
	return users[offset:end], nil
}

func (m *MockUserRepository) Count(ctx context.Context) (int, error) {
	return len(m.users), nil
}

// TestMockUserRepository tests the mock user repository implementation
func TestMockUserRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewMockUserRepository()

	// Test Create
	user := &models.User{
		ID:        "test-user-1",
		Username:  "testuser",
		Email:     "test@example.com",
		APIKey:    "test-api-key",
		Role:      "user",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Test FindByID
	foundUser, err := repo.FindByID(ctx, "test-user-1")
	if err != nil {
		t.Fatalf("Failed to find user by ID: %v", err)
	}
	if foundUser.Username != "testuser" {
		t.Fatalf("Expected username 'testuser', got %s", foundUser.Username)
	}

	// Test FindByEmail
	foundByEmail, err := repo.FindByEmail(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("Failed to find user by email: %v", err)
	}
	if foundByEmail.ID != "test-user-1" {
		t.Fatalf("Expected user ID 'test-user-1', got %s", foundByEmail.ID)
	}

	// Test FindByAPIKey
	foundByAPIKey, err := repo.FindByAPIKey(ctx, "test-api-key")
	if err != nil {
		t.Fatalf("Failed to find user by API key: %v", err)
	}
	if foundByAPIKey.ID != "test-user-1" {
		t.Fatalf("Expected user ID 'test-user-1', got %s", foundByAPIKey.ID)
	}

	// Test Update
	user.Username = "updateduser"
	err = repo.Update(ctx, user)
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	updatedUser, err := repo.FindByID(ctx, "test-user-1")
	if err != nil {
		t.Fatalf("Failed to find updated user: %v", err)
	}
	if updatedUser.Username != "updateduser" {
		t.Fatalf("Expected updated username 'updateduser', got %s", updatedUser.Username)
	}

	// Test List
	users, err := repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("Failed to list users: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("Expected 1 user, got %d", len(users))
	}

	// Test Count
	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected count 1, got %d", count)
	}

	// Test Delete
	err = repo.Delete(ctx, "test-user-1")
	if err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}

	// Verify deletion
	_, err = repo.FindByID(ctx, "test-user-1")
	if err == nil {
		t.Fatal("Expected error when finding deleted user")
	}

	t.Log("MockUserRepository tests passed")
}

// MockSessionRepository is a mock implementation of SessionRepository for testing
type MockSessionRepository struct {
	sessions map[string]*models.UserSession
}

func NewMockSessionRepository() *MockSessionRepository {
	return &MockSessionRepository{
		sessions: make(map[string]*models.UserSession),
	}
}

func (m *MockSessionRepository) Create(ctx context.Context, session *models.UserSession) error {
	if session.ID == "" {
		return &RepositoryError{Message: "session ID is required"}
	}
	if _, exists := m.sessions[session.ID]; exists {
		return &RepositoryError{Message: "session already exists"}
	}
	m.sessions[session.ID] = session
	return nil
}

func (m *MockSessionRepository) FindByID(ctx context.Context, id string) (*models.UserSession, error) {
	session, exists := m.sessions[id]
	if !exists {
		return nil, &RepositoryError{Message: "session not found"}
	}
	return session, nil
}

func (m *MockSessionRepository) FindByToken(ctx context.Context, token string) (*models.UserSession, error) {
	for _, session := range m.sessions {
		if session.SessionToken == token {
			return session, nil
		}
	}
	return nil, &RepositoryError{Message: "session not found"}
}

func (m *MockSessionRepository) FindByUserID(ctx context.Context, userID string) ([]*models.UserSession, error) {
	sessions := make([]*models.UserSession, 0)
	for _, session := range m.sessions {
		if session.UserID == userID {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

func (m *MockSessionRepository) Update(ctx context.Context, session *models.UserSession) error {
	if _, exists := m.sessions[session.ID]; !exists {
		return &RepositoryError{Message: "session not found"}
	}
	m.sessions[session.ID] = session
	return nil
}

func (m *MockSessionRepository) Delete(ctx context.Context, id string) error {
	if _, exists := m.sessions[id]; !exists {
		return &RepositoryError{Message: "session not found"}
	}
	delete(m.sessions, id)
	return nil
}

func (m *MockSessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	now := time.Now()
	deleted := int64(0)
	for id, session := range m.sessions {
		if session.ExpiresAt.Before(now) {
			delete(m.sessions, id)
			deleted++
		}
	}
	return deleted, nil
}

func (m *MockSessionRepository) UpdateLastActivity(ctx context.Context, id string, lastActivity time.Time) error {
	session, exists := m.sessions[id]
	if !exists {
		return &RepositoryError{Message: "session not found"}
	}
	session.LastActivity = lastActivity
	return nil
}

// TestMockSessionRepository tests the mock session repository implementation
func TestMockSessionRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewMockSessionRepository()

	// Test Create
	session := &models.UserSession{
		ID:           "test-session-1",
		UserID:       "test-user-1",
		SessionToken: "test-token-123",
		Context:      map[string]interface{}{"key": "value"},
		Status:       "active",
		RequestCount: 0,
		LastActivity: time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		CreatedAt:    time.Now(),
	}

	err := repo.Create(ctx, session)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test FindByID
	foundSession, err := repo.FindByID(ctx, "test-session-1")
	if err != nil {
		t.Fatalf("Failed to find session by ID: %v", err)
	}
	if foundSession.UserID != "test-user-1" {
		t.Fatalf("Expected user ID 'test-user-1', got %s", foundSession.UserID)
	}

	// Test FindByToken
	foundByToken, err := repo.FindByToken(ctx, "test-token-123")
	if err != nil {
		t.Fatalf("Failed to find session by token: %v", err)
	}
	if foundByToken.ID != "test-session-1" {
		t.Fatalf("Expected session ID 'test-session-1', got %s", foundByToken.ID)
	}

	// Test FindByUserID
	sessions, err := repo.FindByUserID(ctx, "test-user-1")
	if err != nil {
		t.Fatalf("Failed to find sessions by user ID: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(sessions))
	}

	// Test UpdateLastActivity
	newActivity := time.Now().Add(1 * time.Hour)
	err = repo.UpdateLastActivity(ctx, "test-session-1", newActivity)
	if err != nil {
		t.Fatalf("Failed to update last activity: %v", err)
	}

	updatedSession, err := repo.FindByID(ctx, "test-session-1")
	if err != nil {
		t.Fatalf("Failed to find updated session: %v", err)
	}
	if !updatedSession.LastActivity.Equal(newActivity) {
		t.Fatalf("Last activity not updated correctly")
	}

	// Test DeleteExpired
	expiredSession := &models.UserSession{
		ID:        "expired-session",
		UserID:    "test-user-1",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		CreatedAt: time.Now(),
	}
	repo.Create(ctx, expiredSession)

	deleted, err := repo.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("Failed to delete expired sessions: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("Expected 1 expired session deleted, got %d", deleted)
	}

	// Test Delete
	err = repo.Delete(ctx, "test-session-1")
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify deletion
	_, err = repo.FindByID(ctx, "test-session-1")
	if err == nil {
		t.Fatal("Expected error when finding deleted session")
	}

	t.Log("MockSessionRepository tests passed")
}

// RepositoryError represents a repository error
type RepositoryError struct {
	Message string
}

func (e *RepositoryError) Error() string {
	return e.Message
}

// TestRepositoryError tests the repository error type
func TestRepositoryError(t *testing.T) {
	err := &RepositoryError{Message: "test error"}
	if err.Error() != "test error" {
		t.Fatalf("Expected error message 'test error', got %s", err.Error())
	}
	t.Log("RepositoryError tests passed")
}

// MockLLMRequestRepository is a mock implementation of LLMRequestRepository for testing
type MockLLMRequestRepository struct {
	requests map[string]*models.LLMRequest
}

func NewMockLLMRequestRepository() *MockLLMRequestRepository {
	return &MockLLMRequestRepository{
		requests: make(map[string]*models.LLMRequest),
	}
}

func (m *MockLLMRequestRepository) Create(ctx context.Context, request *models.LLMRequest) error {
	if request.ID == "" {
		return &RepositoryError{Message: "request ID is required"}
	}
	if _, exists := m.requests[request.ID]; exists {
		return &RepositoryError{Message: "request already exists"}
	}
	m.requests[request.ID] = request
	return nil
}

func (m *MockLLMRequestRepository) FindByID(ctx context.Context, id string) (*models.LLMRequest, error) {
	request, exists := m.requests[id]
	if !exists {
		return nil, &RepositoryError{Message: "request not found"}
	}
	return request, nil
}

func (m *MockLLMRequestRepository) FindBySessionID(ctx context.Context, sessionID string, limit, offset int) ([]*models.LLMRequest, error) {
	requests := make([]*models.LLMRequest, 0)
	for _, request := range m.requests {
		if request.SessionID == sessionID {
			requests = append(requests, request)
		}
	}
	// Simple pagination
	if offset >= len(requests) {
		return []*models.LLMRequest{}, nil
	}
	end := offset + limit
	if end > len(requests) {
		end = len(requests)
	}
	return requests[offset:end], nil
}

func (m *MockLLMRequestRepository) FindByUserID(ctx context.Context, userID string, limit, offset int) ([]*models.LLMRequest, error) {
	requests := make([]*models.LLMRequest, 0)
	for _, request := range m.requests {
		if request.UserID == userID {
			requests = append(requests, request)
		}
	}
	// Simple pagination
	if offset >= len(requests) {
		return []*models.LLMRequest{}, nil
	}
	end := offset + limit
	if end > len(requests) {
		end = len(requests)
	}
	return requests[offset:end], nil
}

func (m *MockLLMRequestRepository) UpdateStatus(ctx context.Context, id string, status string, startedAt, completedAt *time.Time) error {
	request, exists := m.requests[id]
	if !exists {
		return &RepositoryError{Message: "request not found"}
	}
	request.Status = status
	if startedAt != nil {
		request.StartedAt = startedAt
	}
	if completedAt != nil {
		request.CompletedAt = completedAt
	}
	return nil
}

func (m *MockLLMRequestRepository) UpdateResponse(ctx context.Context, id string, providerID, responseContent string, tokensUsed, responseTime int) error {
	request, exists := m.requests[id]
	if !exists {
		return &RepositoryError{Message: "request not found"}
	}
	// In a real implementation, this would update related response records
	request.Status = "completed"
	now := time.Now()
	request.CompletedAt = &now
	return nil
}

func (m *MockLLMRequestRepository) UpdateError(ctx context.Context, id string, errorMessage string) error {
	request, exists := m.requests[id]
	if !exists {
		return &RepositoryError{Message: "request not found"}
	}
	request.Status = "failed"
	now := time.Now()
	request.CompletedAt = &now
	return nil
}

func (m *MockLLMRequestRepository) Delete(ctx context.Context, id string) error {
	if _, exists := m.requests[id]; !exists {
		return &RepositoryError{Message: "request not found"}
	}
	delete(m.requests, id)
	return nil
}

func (m *MockLLMRequestRepository) CountBySessionID(ctx context.Context, sessionID string) (int, error) {
	count := 0
	for _, request := range m.requests {
		if request.SessionID == sessionID {
			count++
		}
	}
	return count, nil
}

func (m *MockLLMRequestRepository) CountByUserID(ctx context.Context, userID string) (int, error) {
	count := 0
	for _, request := range m.requests {
		if request.UserID == userID {
			count++
		}
	}
	return count, nil
}

// TestMockLLMRequestRepository tests the mock LLM request repository implementation
func TestMockLLMRequestRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewMockLLMRequestRepository()

	// Test Create
	request := &models.LLMRequest{
		ID:        "test-request-1",
		SessionID: "test-session-1",
		UserID:    "test-user-1",
		Prompt:    "Test prompt",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		ModelParams: models.ModelParameters{
			Model:       "gpt-4",
			Temperature: 0.7,
			MaxTokens:   1000,
		},
		Status:      "pending",
		CreatedAt:   time.Now(),
		RequestType: "completion",
	}

	err := repo.Create(ctx, request)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Test FindByID
	foundRequest, err := repo.FindByID(ctx, "test-request-1")
	if err != nil {
		t.Fatalf("Failed to find request by ID: %v", err)
	}
	if foundRequest.Prompt != "Test prompt" {
		t.Fatalf("Expected prompt 'Test prompt', got %s", foundRequest.Prompt)
	}

	// Test FindBySessionID
	requests, err := repo.FindBySessionID(ctx, "test-session-1", 10, 0)
	if err != nil {
		t.Fatalf("Failed to find requests by session ID: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("Expected 1 request, got %d", len(requests))
	}

	// Test FindByUserID
	userRequests, err := repo.FindByUserID(ctx, "test-user-1", 10, 0)
	if err != nil {
		t.Fatalf("Failed to find requests by user ID: %v", err)
	}
	if len(userRequests) != 1 {
		t.Fatalf("Expected 1 request, got %d", len(userRequests))
	}

	// Test UpdateStatus
	startedAt := time.Now()
	err = repo.UpdateStatus(ctx, "test-request-1", "processing", &startedAt, nil)
	if err != nil {
		t.Fatalf("Failed to update request status: %v", err)
	}

	updatedRequest, err := repo.FindByID(ctx, "test-request-1")
	if err != nil {
		t.Fatalf("Failed to find updated request: %v", err)
	}
	if updatedRequest.Status != "processing" {
		t.Fatalf("Expected status 'processing', got %s", updatedRequest.Status)
	}

	// Test CountBySessionID
	count, err := repo.CountBySessionID(ctx, "test-session-1")
	if err != nil {
		t.Fatalf("Failed to count requests by session ID: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected count 1, got %d", count)
	}

	// Test CountByUserID
	userCount, err := repo.CountByUserID(ctx, "test-user-1")
	if err != nil {
		t.Fatalf("Failed to count requests by user ID: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("Expected count 1, got %d", userCount)
	}

	// Test Delete
	err = repo.Delete(ctx, "test-request-1")
	if err != nil {
		t.Fatalf("Failed to delete request: %v", err)
	}

	// Verify deletion
	_, err = repo.FindByID(ctx, "test-request-1")
	if err == nil {
		t.Fatal("Expected error when finding deleted request")
	}

	t.Log("MockLLMRequestRepository tests passed")
}

// MockLLMResponseRepository is a mock implementation of LLMResponseRepository for testing
type MockLLMResponseRepository struct {
	responses map[string]*models.LLMResponse
}

func NewMockLLMResponseRepository() *MockLLMResponseRepository {
	return &MockLLMResponseRepository{
		responses: make(map[string]*models.LLMResponse),
	}
}

func (m *MockLLMResponseRepository) Create(ctx context.Context, response *models.LLMResponse) error {
	if response.ID == "" {
		return &RepositoryError{Message: "response ID is required"}
	}
	if _, exists := m.responses[response.ID]; exists {
		return &RepositoryError{Message: "response already exists"}
	}
	m.responses[response.ID] = response
	return nil
}

func (m *MockLLMResponseRepository) FindByID(ctx context.Context, id string) (*models.LLMResponse, error) {
	response, exists := m.responses[id]
	if !exists {
		return nil, &RepositoryError{Message: "response not found"}
	}
	return response, nil
}

func (m *MockLLMResponseRepository) FindByRequestID(ctx context.Context, requestID string) ([]*models.LLMResponse, error) {
	responses := make([]*models.LLMResponse, 0)
	for _, response := range m.responses {
		if response.RequestID == requestID {
			responses = append(responses, response)
		}
	}
	return responses, nil
}

func (m *MockLLMResponseRepository) FindSelectedByRequestID(ctx context.Context, requestID string) (*models.LLMResponse, error) {
	for _, response := range m.responses {
		if response.RequestID == requestID && response.Selected {
			return response, nil
		}
	}
	return nil, &RepositoryError{Message: "selected response not found"}
}

func (m *MockLLMResponseRepository) UpdateSelected(ctx context.Context, id string, selected bool, selectionScore float64) error {
	response, exists := m.responses[id]
	if !exists {
		return &RepositoryError{Message: "response not found"}
	}
	response.Selected = selected
	response.SelectionScore = selectionScore
	return nil
}

func (m *MockLLMResponseRepository) Delete(ctx context.Context, id string) error {
	if _, exists := m.responses[id]; !exists {
		return &RepositoryError{Message: "response not found"}
	}
	delete(m.responses, id)
	return nil
}

func (m *MockLLMResponseRepository) DeleteByRequestID(ctx context.Context, requestID string) error {
	idsToDelete := make([]string, 0)
	for id, response := range m.responses {
		if response.RequestID == requestID {
			idsToDelete = append(idsToDelete, id)
		}
	}
	for _, id := range idsToDelete {
		delete(m.responses, id)
	}
	return nil
}

// TestMockLLMResponseRepository tests the mock LLM response repository implementation
func TestMockLLMResponseRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewMockLLMResponseRepository()

	// Test Create
	response := &models.LLMResponse{
		ID:           "test-response-1",
		RequestID:    "test-request-1",
		ProviderID:   "test-provider-1",
		ProviderName: "OpenAI",
		Content:      "Test response content",
		Confidence:   0.95,
		TokensUsed:   100,
		ResponseTime: 1500,
		FinishReason: "stop",
		Selected:     false,
		CreatedAt:    time.Now(),
	}

	err := repo.Create(ctx, response)
	if err != nil {
		t.Fatalf("Failed to create response: %v", err)
	}

	// Test FindByID
	foundResponse, err := repo.FindByID(ctx, "test-response-1")
	if err != nil {
		t.Fatalf("Failed to find response by ID: %v", err)
	}
	if foundResponse.Content != "Test response content" {
		t.Fatalf("Expected content 'Test response content', got %s", foundResponse.Content)
	}

	// Test FindByRequestID
	responses, err := repo.FindByRequestID(ctx, "test-request-1")
	if err != nil {
		t.Fatalf("Failed to find responses by request ID: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(responses))
	}

	// Test UpdateSelected
	err = repo.UpdateSelected(ctx, "test-response-1", true, 0.98)
	if err != nil {
		t.Fatalf("Failed to update response selection: %v", err)
	}

	updatedResponse, err := repo.FindByID(ctx, "test-response-1")
	if err != nil {
		t.Fatalf("Failed to find updated response: %v", err)
	}
	if !updatedResponse.Selected {
		t.Fatal("Expected response to be selected")
	}
	if updatedResponse.SelectionScore != 0.98 {
		t.Fatalf("Expected selection score 0.98, got %f", updatedResponse.SelectionScore)
	}

	// Test FindSelectedByRequestID
	selectedResponse, err := repo.FindSelectedByRequestID(ctx, "test-request-1")
	if err != nil {
		t.Fatalf("Failed to find selected response: %v", err)
	}
	if selectedResponse.ID != "test-response-1" {
		t.Fatalf("Expected response ID 'test-response-1', got %s", selectedResponse.ID)
	}

	// Test DeleteByRequestID
	// First create another response for the same request
	response2 := &models.LLMResponse{
		ID:        "test-response-2",
		RequestID: "test-request-1",
		CreatedAt: time.Now(),
	}
	repo.Create(ctx, response2)

	err = repo.DeleteByRequestID(ctx, "test-request-1")
	if err != nil {
		t.Fatalf("Failed to delete responses by request ID: %v", err)
	}

	// Verify deletion
	responsesAfterDelete, _ := repo.FindByRequestID(ctx, "test-request-1")
	if len(responsesAfterDelete) != 0 {
		t.Fatalf("Expected 0 responses after delete, got %d", len(responsesAfterDelete))
	}

	t.Log("MockLLMResponseRepository tests passed")
}

// MockProviderRepository is a mock implementation of ProviderRepository for testing
type MockProviderRepository struct {
	providers map[string]*models.LLMProvider
}

func NewMockProviderRepository() *MockProviderRepository {
	return &MockProviderRepository{
		providers: make(map[string]*models.LLMProvider),
	}
}

func (m *MockProviderRepository) Create(ctx context.Context, provider *models.LLMProvider) error {
	if provider.ID == "" {
		return &RepositoryError{Message: "provider ID is required"}
	}
	if _, exists := m.providers[provider.ID]; exists {
		return &RepositoryError{Message: "provider already exists"}
	}
	m.providers[provider.ID] = provider
	return nil
}

func (m *MockProviderRepository) FindByID(ctx context.Context, id string) (*models.LLMProvider, error) {
	provider, exists := m.providers[id]
	if !exists {
		return nil, &RepositoryError{Message: "provider not found"}
	}
	return provider, nil
}

func (m *MockProviderRepository) FindByName(ctx context.Context, name string) (*models.LLMProvider, error) {
	for _, provider := range m.providers {
		if provider.Name == name {
			return provider, nil
		}
	}
	return nil, &RepositoryError{Message: "provider not found"}
}

func (m *MockProviderRepository) FindAll(ctx context.Context, enabledOnly bool) ([]*models.LLMProvider, error) {
	providers := make([]*models.LLMProvider, 0)
	for _, provider := range m.providers {
		if !enabledOnly || provider.Enabled {
			providers = append(providers, provider)
		}
	}
	return providers, nil
}

func (m *MockProviderRepository) Update(ctx context.Context, provider *models.LLMProvider) error {
	if _, exists := m.providers[provider.ID]; !exists {
		return &RepositoryError{Message: "provider not found"}
	}
	m.providers[provider.ID] = provider
	return nil
}

func (m *MockProviderRepository) UpdateHealthStatus(ctx context.Context, id string, healthStatus string, responseTime int64) error {
	provider, exists := m.providers[id]
	if !exists {
		return &RepositoryError{Message: "provider not found"}
	}
	provider.HealthStatus = healthStatus
	provider.ResponseTime = responseTime
	return nil
}

func (m *MockProviderRepository) Delete(ctx context.Context, id string) error {
	if _, exists := m.providers[id]; !exists {
		return &RepositoryError{Message: "provider not found"}
	}
	delete(m.providers, id)
	return nil
}

func (m *MockProviderRepository) Count(ctx context.Context, enabledOnly bool) (int, error) {
	count := 0
	for _, provider := range m.providers {
		if !enabledOnly || provider.Enabled {
			count++
		}
	}
	return count, nil
}

// TestMockProviderRepository tests the mock provider repository implementation
func TestMockProviderRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewMockProviderRepository()

	// Test Create
	provider := &models.LLMProvider{
		ID:           "test-provider-1",
		Name:         "OpenAI",
		Type:         "openai",
		APIKey:       "test-api-key",
		BaseURL:      "https://api.openai.com",
		Model:        "gpt-4",
		Weight:       1.0,
		Enabled:      true,
		Config:       map[string]interface{}{"temperature": 0.7},
		HealthStatus: "healthy",
		ResponseTime: 1000,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := repo.Create(ctx, provider)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Test FindByID
	foundProvider, err := repo.FindByID(ctx, "test-provider-1")
	if err != nil {
		t.Fatalf("Failed to find provider by ID: %v", err)
	}
	if foundProvider.Name != "OpenAI" {
		t.Fatalf("Expected provider name 'OpenAI', got %s", foundProvider.Name)
	}

	// Test FindByName
	foundByName, err := repo.FindByName(ctx, "OpenAI")
	if err != nil {
		t.Fatalf("Failed to find provider by name: %v", err)
	}
	if foundByName.ID != "test-provider-1" {
		t.Fatalf("Expected provider ID 'test-provider-1', got %s", foundByName.ID)
	}

	// Test FindAll (enabled only)
	providers, err := repo.FindAll(ctx, true)
	if err != nil {
		t.Fatalf("Failed to find all providers: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("Expected 1 provider, got %d", len(providers))
	}

	// Test UpdateHealthStatus
	err = repo.UpdateHealthStatus(ctx, "test-provider-1", "degraded", 2000)
	if err != nil {
		t.Fatalf("Failed to update health status: %v", err)
	}

	updatedProvider, err := repo.FindByID(ctx, "test-provider-1")
	if err != nil {
		t.Fatalf("Failed to find updated provider: %v", err)
	}
	if updatedProvider.HealthStatus != "degraded" {
		t.Fatalf("Expected health status 'degraded', got %s", updatedProvider.HealthStatus)
	}
	if updatedProvider.ResponseTime != 2000 {
		t.Fatalf("Expected response time 2000, got %d", updatedProvider.ResponseTime)
	}

	// Test Count
	count, err := repo.Count(ctx, true)
	if err != nil {
		t.Fatalf("Failed to count providers: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected count 1, got %d", count)
	}

	// Create a disabled provider
	disabledProvider := &models.LLMProvider{
		ID:      "test-provider-2",
		Name:    "Disabled Provider",
		Enabled: false,
	}
	repo.Create(ctx, disabledProvider)

	// Test Count with enabledOnly=false
	totalCount, err := repo.Count(ctx, false)
	if err != nil {
		t.Fatalf("Failed to count all providers: %v", err)
	}
	if totalCount != 2 {
		t.Fatalf("Expected total count 2, got %d", totalCount)
	}

	// Test Delete
	err = repo.Delete(ctx, "test-provider-1")
	if err != nil {
		t.Fatalf("Failed to delete provider: %v", err)
	}

	// Verify deletion
	_, err = repo.FindByID(ctx, "test-provider-1")
	if err == nil {
		t.Fatal("Expected error when finding deleted provider")
	}

	t.Log("MockProviderRepository tests passed")
}

// MockRepository is a comprehensive mock implementation of the Repository interface
type MockRepository struct {
	users     *MockUserRepository
	sessions  *MockSessionRepository
	requests  *MockLLMRequestRepository
	responses *MockLLMResponseRepository
	memories  *MockMemoryRepository
	providers *MockProviderRepository
	inTx      bool
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		users:     NewMockUserRepository(),
		sessions:  NewMockSessionRepository(),
		requests:  NewMockLLMRequestRepository(),
		responses: NewMockLLMResponseRepository(),
		memories:  NewMockMemoryRepository(),
		providers: NewMockProviderRepository(),
		inTx:      false,
	}
}

func (m *MockRepository) Users() UserRepository {
	return m.users
}

func (m *MockRepository) Sessions() SessionRepository {
	return m.sessions
}

func (m *MockRepository) LLMRequests() LLMRequestRepository {
	return m.requests
}

func (m *MockRepository) LLMResponses() LLMResponseRepository {
	return m.responses
}

func (m *MockRepository) Memories() MemoryRepository {
	return m.memories
}

func (m *MockRepository) Providers() ProviderRepository {
	return m.providers
}

func (m *MockRepository) BeginTx(ctx context.Context) (Repository, error) {
	if m.inTx {
		return nil, &RepositoryError{Message: "already in transaction"}
	}
	// Create a new mock repository for the transaction
	txRepo := NewMockRepository()
	txRepo.inTx = true
	// Copy data from parent repository
	// Note: In a real implementation, this would be more sophisticated
	return txRepo, nil
}

func (m *MockRepository) Commit() error {
	if !m.inTx {
		return &RepositoryError{Message: "not in transaction"}
	}
	// In a real implementation, this would apply changes to parent repository
	m.inTx = false
	return nil
}

func (m *MockRepository) Rollback() error {
	if !m.inTx {
		return &RepositoryError{Message: "not in transaction"}
	}
	m.inTx = false
	return nil
}

func (m *MockRepository) Close() error {
	// Clean up resources
	m.users = nil
	m.sessions = nil
	m.requests = nil
	m.responses = nil
	m.memories = nil
	m.providers = nil
	return nil
}

// MockMemoryRepository is a mock implementation of MemoryRepository for testing
type MockMemoryRepository struct {
	memories map[string]*models.CogneeMemory
}

func NewMockMemoryRepository() *MockMemoryRepository {
	return &MockMemoryRepository{
		memories: make(map[string]*models.CogneeMemory),
	}
}

func (m *MockMemoryRepository) Create(ctx context.Context, memory *models.CogneeMemory) error {
	if memory.ID == "" {
		return &RepositoryError{Message: "memory ID is required"}
	}
	if _, exists := m.memories[memory.ID]; exists {
		return &RepositoryError{Message: "memory already exists"}
	}
	m.memories[memory.ID] = memory
	return nil
}

func (m *MockMemoryRepository) FindByID(ctx context.Context, id string) (*models.CogneeMemory, error) {
	memory, exists := m.memories[id]
	if !exists {
		return nil, &RepositoryError{Message: "memory not found"}
	}
	return memory, nil
}

func (m *MockMemoryRepository) FindBySessionID(ctx context.Context, sessionID string, limit, offset int) ([]*models.CogneeMemory, error) {
	memories := make([]*models.CogneeMemory, 0)
	for _, memory := range m.memories {
		if memory.SessionID != nil && *memory.SessionID == sessionID {
			memories = append(memories, memory)
		}
	}
	// Simple pagination
	if offset >= len(memories) {
		return []*models.CogneeMemory{}, nil
	}
	end := offset + limit
	if end > len(memories) {
		end = len(memories)
	}
	return memories[offset:end], nil
}

func (m *MockMemoryRepository) FindByDatasetName(ctx context.Context, datasetName string, limit, offset int) ([]*models.CogneeMemory, error) {
	memories := make([]*models.CogneeMemory, 0)
	for _, memory := range m.memories {
		if memory.DatasetName == datasetName {
			memories = append(memories, memory)
		}
	}
	// Simple pagination
	if offset >= len(memories) {
		return []*models.CogneeMemory{}, nil
	}
	end := offset + limit
	if end > len(memories) {
		end = len(memories)
	}
	return memories[offset:end], nil
}

func (m *MockMemoryRepository) FindBySearchKey(ctx context.Context, searchKey string, limit, offset int) ([]*models.CogneeMemory, error) {
	memories := make([]*models.CogneeMemory, 0)
	for _, memory := range m.memories {
		if memory.SearchKey == searchKey {
			memories = append(memories, memory)
		}
	}
	// Simple pagination
	if offset >= len(memories) {
		return []*models.CogneeMemory{}, nil
	}
	end := offset + limit
	if end > len(memories) {
		end = len(memories)
	}
	return memories[offset:end], nil
}

func (m *MockMemoryRepository) Update(ctx context.Context, memory *models.CogneeMemory) error {
	if _, exists := m.memories[memory.ID]; !exists {
		return &RepositoryError{Message: "memory not found"}
	}
	m.memories[memory.ID] = memory
	return nil
}

func (m *MockMemoryRepository) Delete(ctx context.Context, id string) error {
	if _, exists := m.memories[id]; !exists {
		return &RepositoryError{Message: "memory not found"}
	}
	delete(m.memories, id)
	return nil
}

func (m *MockMemoryRepository) DeleteExpired(ctx context.Context) (int64, error) {
	// Simple implementation - no expiration logic in mock
	return 0, nil
}

func (m *MockMemoryRepository) CountBySessionID(ctx context.Context, sessionID string) (int, error) {
	count := 0
	for _, memory := range m.memories {
		if memory.SessionID != nil && *memory.SessionID == sessionID {
			count++
		}
	}
	return count, nil
}

// TestMockMemoryRepository tests the mock memory repository implementation
func TestMockMemoryRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewMockMemoryRepository()

	// Test Create
	sessionID := "test-session-1"
	memory := &models.CogneeMemory{
		ID:          "test-memory-1",
		SessionID:   &sessionID,
		DatasetName: "test-dataset",
		ContentType: "text",
		Content:     "Test memory content",
		VectorID:    "test-vector-1",
		GraphNodes:  map[string]interface{}{"node1": "value1"},
		SearchKey:   "test-key",
		CreatedAt:   time.Now(),
	}

	err := repo.Create(ctx, memory)
	if err != nil {
		t.Fatalf("Failed to create memory: %v", err)
	}

	// Test FindByID
	foundMemory, err := repo.FindByID(ctx, "test-memory-1")
	if err != nil {
		t.Fatalf("Failed to find memory by ID: %v", err)
	}
	if foundMemory.Content != "Test memory content" {
		t.Fatalf("Expected content 'Test memory content', got %s", foundMemory.Content)
	}

	// Test FindBySessionID
	memories, err := repo.FindBySessionID(ctx, "test-session-1", 10, 0)
	if err != nil {
		t.Fatalf("Failed to find memories by session ID: %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("Expected 1 memory, got %d", len(memories))
	}

	// Test FindByDatasetName
	datasetMemories, err := repo.FindByDatasetName(ctx, "test-dataset", 10, 0)
	if err != nil {
		t.Fatalf("Failed to find memories by dataset name: %v", err)
	}
	if len(datasetMemories) != 1 {
		t.Fatalf("Expected 1 memory, got %d", len(datasetMemories))
	}

	// Test FindBySearchKey
	searchMemories, err := repo.FindBySearchKey(ctx, "test-key", 10, 0)
	if err != nil {
		t.Fatalf("Failed to find memories by search key: %v", err)
	}
	if len(searchMemories) != 1 {
		t.Fatalf("Expected 1 memory, got %d", len(searchMemories))
	}

	// Test CountBySessionID
	count, err := repo.CountBySessionID(ctx, "test-session-1")
	if err != nil {
		t.Fatalf("Failed to count memories by session ID: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected count 1, got %d", count)
	}

	// Test Update
	memory.Content = "Updated content"
	err = repo.Update(ctx, memory)
	if err != nil {
		t.Fatalf("Failed to update memory: %v", err)
	}

	updatedMemory, err := repo.FindByID(ctx, "test-memory-1")
	if err != nil {
		t.Fatalf("Failed to find updated memory: %v", err)
	}
	if updatedMemory.Content != "Updated content" {
		t.Fatalf("Expected updated content 'Updated content', got %s", updatedMemory.Content)
	}

	// Test Delete
	err = repo.Delete(ctx, "test-memory-1")
	if err != nil {
		t.Fatalf("Failed to delete memory: %v", err)
	}

	// Verify deletion
	_, err = repo.FindByID(ctx, "test-memory-1")
	if err == nil {
		t.Fatal("Expected error when finding deleted memory")
	}

	t.Log("MockMemoryRepository tests passed")
}

// TestMockRepository tests the comprehensive mock repository implementation
func TestMockRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()

	// Test Users repository
	user := &models.User{
		ID:        "test-user-1",
		Username:  "testuser",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := repo.Users().Create(ctx, user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Test Sessions repository
	session := &models.UserSession{
		ID:           "test-session-1",
		UserID:       "test-user-1",
		SessionToken: "test-token",
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	err = repo.Sessions().Create(ctx, session)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test LLMRequests repository
	request := &models.LLMRequest{
		ID:        "test-request-1",
		SessionID: "test-session-1",
		UserID:    "test-user-1",
		Prompt:    "Test prompt",
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	err = repo.LLMRequests().Create(ctx, request)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Test LLMResponses repository
	response := &models.LLMResponse{
		ID:        "test-response-1",
		RequestID: "test-request-1",
		Content:   "Test response",
		CreatedAt: time.Now(),
	}
	err = repo.LLMResponses().Create(ctx, response)
	if err != nil {
		t.Fatalf("Failed to create response: %v", err)
	}

	// Test Providers repository
	provider := &models.LLMProvider{
		ID:        "test-provider-1",
		Name:      "Test Provider",
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = repo.Providers().Create(ctx, provider)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Test Memories repository
	sessionID := "test-session-1"
	memory := &models.CogneeMemory{
		ID:        "test-memory-1",
		SessionID: &sessionID,
		Content:   "Test memory",
		CreatedAt: time.Now(),
	}
	err = repo.Memories().Create(ctx, memory)
	if err != nil {
		t.Fatalf("Failed to create memory: %v", err)
	}

	// Test transaction support
	txRepo, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Create a user in transaction
	txUser := &models.User{
		ID:        "tx-user-1",
		Username:  "txuser",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = txRepo.Users().Create(ctx, txUser)
	if err != nil {
		t.Fatalf("Failed to create user in transaction: %v", err)
	}

	// Test commit
	err = txRepo.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Test rollback
	txRepo2, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("Failed to begin second transaction: %v", err)
	}

	txUser2 := &models.User{
		ID:        "tx-user-2",
		Username:  "txuser2",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = txRepo2.Users().Create(ctx, txUser2)
	if err != nil {
		t.Fatalf("Failed to create user in second transaction: %v", err)
	}

	err = txRepo2.Rollback()
	if err != nil {
		t.Fatalf("Failed to rollback transaction: %v", err)
	}

	// Test Close
	err = repo.Close()
	if err != nil {
		t.Fatalf("Failed to close repository: %v", err)
	}

	t.Log("MockRepository tests passed")
}
