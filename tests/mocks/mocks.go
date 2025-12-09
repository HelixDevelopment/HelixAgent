package mocks

import (
	"context"
	"time"

	"github.com/superagent/superagent/internal/models"
)

// MockLLMProvider implements a mock LLM provider for testing
type MockLLMProvider struct {
	Name         string
	Model        string
	ShouldFail   bool
	ResponseTime time.Duration
	Response     string
	Confidence   float64
	TokensUsed   int
}

func (m *MockLLMProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if m.ShouldFail {
		return nil, &MockError{Message: "Mock provider error"}
	}

	// Simulate processing time
	time.Sleep(m.ResponseTime)

	return &models.LLMResponse{
		ID:             "mock-response-" + req.ID,
		RequestID:      req.ID,
		ProviderID:     m.Name,
		ProviderName:   m.Name,
		Content:        m.Response,
		Confidence:     m.Confidence,
		TokensUsed:     m.TokensUsed,
		ResponseTime:   m.ResponseTime.Milliseconds(),
		FinishReason:   "stop",
		Metadata:       map[string]any{},
		Selected:       false,
		SelectionScore: 0.0,
		CreatedAt:      time.Now(),
	}, nil
}

func (m *MockLLMProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	if m.ShouldFail {
		return nil, &MockError{Message: "Mock provider streaming error"}
	}

	ch := make(chan *models.LLMResponse)

	go func() {
		defer close(ch)

		// Simulate streaming response
		words := []string{"This", "is", "a", "mock", "streaming", "response"}
		for i, word := range words {
			select {
			case <-ctx.Done():
				return
			default:
				response := &models.LLMResponse{
					ID:             "mock-stream-" + req.ID,
					RequestID:      req.ID,
					ProviderID:     m.Name,
					ProviderName:   m.Name,
					Content:        word + " ",
					Confidence:     m.Confidence,
					TokensUsed:     i + 1,
					ResponseTime:   int64((i + 1) * 100),
					FinishReason:   "",
					Selected:       false,
					SelectionScore: 0.0,
					CreatedAt:      time.Now(),
				}
				ch <- response
				time.Sleep(50 * time.Millisecond)
			}
		}

		// Final response
		final := &models.LLMResponse{
			ID:             "mock-stream-final-" + req.ID,
			RequestID:      req.ID,
			ProviderID:     m.Name,
			ProviderName:   m.Name,
			Content:        "",
			Confidence:     m.Confidence,
			TokensUsed:     len(words),
			ResponseTime:   int64(len(words) * 100),
			FinishReason:   "stop",
			Selected:       false,
			SelectionScore: 0.0,
			CreatedAt:      time.Now(),
		}
		ch <- final
	}()

	return ch, nil
}

// MockPlugin implements a mock plugin for testing
type MockPlugin struct {
	Name_        string
	Version_     string
	Initialized  bool
	IsShutdown   bool
	HealthPassed bool
	Config       map[string]any
}

func (m *MockPlugin) Name() string {
	return m.Name_
}

func (m *MockPlugin) Version() string {
	return m.Version_
}

func (m *MockPlugin) Capabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels:         []string{"mock-model"},
		SupportedFeatures:       []string{"streaming"},
		SupportedRequestTypes:   []string{"code_generation", "reasoning"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: false,
		SupportsVision:          false,
		Limits: models.ModelLimits{
			MaxTokens:             4096,
			MaxInputLength:        2048,
			MaxOutputLength:       2048,
			MaxConcurrentRequests: 10,
		},
		Metadata: map[string]string{
			"mock": "true",
		},
	}
}

func (m *MockPlugin) Init(config map[string]any) error {
	m.Config = config
	m.Initialized = true
	return nil
}

func (m *MockPlugin) Shutdown(ctx context.Context) error {
	m.IsShutdown = true
	return nil
}

func (m *MockPlugin) HealthCheck(ctx context.Context) error {
	if !m.HealthPassed {
		return &MockError{Message: "Mock health check failed"}
	}
	time.Sleep(10 * time.Millisecond) // Simulate health check latency
	return nil
}

// MockError implements a mock error for testing
type MockError struct {
	Message string
}

func (e *MockError) Error() string {
	return e.Message
}

// MockEnsembleService implements a mock ensemble service for testing
type MockEnsembleService struct {
	Responses  []*models.LLMResponse
	ShouldFail bool
}

func (m *MockEnsembleService) RunEnsemble(req *models.LLMRequest) ([]*models.LLMResponse, *models.LLMResponse, error) {
	if m.ShouldFail {
		return nil, nil, &MockError{Message: "Mock ensemble error"}
	}

	if len(m.Responses) == 0 {
		// Generate default mock responses
		m.Responses = []*models.LLMResponse{
			{
				ID:             "mock-ensemble-1",
				RequestID:      req.ID,
				ProviderID:     "mock-provider-1",
				ProviderName:   "Mock Provider 1",
				Content:        "Mock response 1",
				Confidence:     0.9,
				TokensUsed:     100,
				ResponseTime:   500,
				FinishReason:   "stop",
				Selected:       true,
				SelectionScore: 0.9,
				CreatedAt:      time.Now(),
			},
			{
				ID:             "mock-ensemble-2",
				RequestID:      req.ID,
				ProviderID:     "mock-provider-2",
				ProviderName:   "Mock Provider 2",
				Content:        "Mock response 2",
				Confidence:     0.8,
				TokensUsed:     120,
				ResponseTime:   600,
				FinishReason:   "stop",
				Selected:       false,
				SelectionScore: 0.8,
				CreatedAt:      time.Now(),
			},
		}
	}

	// Find the selected response (highest confidence)
	var selected *models.LLMResponse
	maxConfidence := 0.0
	for _, resp := range m.Responses {
		if resp.Confidence > maxConfidence {
			maxConfidence = resp.Confidence
			selected = resp
		}
	}

	return m.Responses, selected, nil
}

// MockRequestService implements a mock request service for testing
type MockRequestService struct {
	Requests   map[string]*models.LLMRequest
	ShouldFail bool
}

func NewMockRequestService() *MockRequestService {
	return &MockRequestService{
		Requests: make(map[string]*models.LLMRequest),
	}
}

func (m *MockRequestService) CreateRequest(req *models.LLMRequest) error {
	if m.ShouldFail {
		return &MockError{Message: "Mock create request error"}
	}
	m.Requests[req.ID] = req
	return nil
}

func (m *MockRequestService) GetRequest(id string) (*models.LLMRequest, error) {
	if m.ShouldFail {
		return nil, &MockError{Message: "Mock get request error"}
	}
	req, exists := m.Requests[id]
	if !exists {
		return nil, &MockError{Message: "Request not found"}
	}
	return req, nil
}

func (m *MockRequestService) UpdateRequestStatus(id, status string) error {
	if m.ShouldFail {
		return &MockError{Message: "Mock update request error"}
	}
	req, exists := m.Requests[id]
	if !exists {
		return &MockError{Message: "Request not found"}
	}
	req.Status = status
	return nil
}

// Helper functions to create mock instances

func NewMockLLMProvider(name string) *MockLLMProvider {
	return &MockLLMProvider{
		Name:         name,
		Model:        "mock-model",
		ShouldFail:   false,
		ResponseTime: 100 * time.Millisecond,
		Response:     "This is a mock response from " + name,
		Confidence:   0.85,
		TokensUsed:   50,
	}
}

func NewMockFailingLLMProvider(name string) *MockLLMProvider {
	provider := NewMockLLMProvider(name)
	provider.ShouldFail = true
	return provider
}

func NewMockPlugin(name, version string) *MockPlugin {
	return &MockPlugin{
		Name_:        name,
		Version_:     version,
		Initialized:  false,
		IsShutdown:   false,
		HealthPassed: true,
		Config:       make(map[string]any),
	}
}

func NewMockFailingPlugin(name, version string) *MockPlugin {
	plugin := NewMockPlugin(name, version)
	plugin.HealthPassed = false
	return plugin
}
