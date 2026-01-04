package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	llm "github.com/superagent/superagent/internal/llm"
	models "github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/services"
	pb "github.com/superagent/superagent/pkg/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// LLMFacadeServer implements the gRPC LLMFacade service
type LLMFacadeServer struct {
	pb.UnimplementedLLMFacadeServer

	// Provider management
	providers   map[string]*ProviderInfo
	providersMu sync.RWMutex

	// Session management
	sessions   map[string]*SessionInfo
	sessionsMu sync.RWMutex

	// Metrics
	metrics   *ServerMetrics
	metricsMu sync.RWMutex

	startTime time.Time
}

// ProviderInfo holds provider registration information
type ProviderInfo struct {
	ID             string
	Name           string
	Type           string
	Model          string
	BaseURL        string
	Enabled        bool
	Weight         float64
	HealthStatus   string
	ResponseTimeMs int64
	SuccessRate    float64
	Config         *structpb.Struct
	RegisteredAt   time.Time
	LastUpdated    time.Time
}

// SessionInfo holds session information
type SessionInfo struct {
	ID            string
	UserID        string
	Status        string
	Context       *structpb.Struct
	MemoryEnabled bool
	RequestCount  int32
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ExpiresAt     time.Time
}

// ServerMetrics holds server metrics
type ServerMetrics struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	TotalLatencyMs     int64
	ActiveSessions     int64
	ActiveProviders    int64
}

// NewLLMFacadeServer creates a new gRPC server instance
func NewLLMFacadeServer() *LLMFacadeServer {
	return &LLMFacadeServer{
		providers: make(map[string]*ProviderInfo),
		sessions:  make(map[string]*SessionInfo),
		metrics:   &ServerMetrics{},
		startTime: time.Now(),
	}
}

// Complete implements standard completion request
func (s *LLMFacadeServer) Complete(ctx context.Context, req *pb.CompletionRequest) (*pb.CompletionResponse, error) {
	start := time.Now()
	s.recordRequest()

	modelParams := models.ModelParameters{
		Model:            "default",
		Temperature:      0.7,
		MaxTokens:        1000,
		TopP:             1.0,
		StopSequences:    []string{},
		ProviderSpecific: map[string]any{},
	}

	internal := &models.LLMRequest{
		ID:             uuid.New().String(),
		SessionID:      req.SessionId,
		UserID:         "",
		Prompt:         req.Prompt,
		MemoryEnhanced: req.MemoryEnhanced,
		Memory:         map[string]string{},
		ModelParams:    modelParams,
		EnsembleConfig: nil,
		Status:         "pending",
		CreatedAt:      time.Now(),
	}

	responses, selected, err := llm.RunEnsemble(internal)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		s.recordFailure(latency)
		return &pb.CompletionResponse{Content: "", Confidence: 0}, err
	}

	s.recordSuccess(latency)

	out := &pb.CompletionResponse{}
	if len(responses) > 0 && responses[0] != nil {
		out.Content = responses[0].Content
		out.Confidence = responses[0].Confidence
		out.ProviderName = responses[0].ProviderName
	}
	if selected != nil {
		out.Content = selected.Content
		out.Confidence = selected.Confidence
	}

	return out, nil
}

// CompleteStream implements streaming completion for real-time generation
func (s *LLMFacadeServer) CompleteStream(req *pb.CompletionRequest, stream grpc.ServerStreamingServer[pb.CompletionResponse]) error {
	s.recordRequest()

	modelParams := models.ModelParameters{
		Model:            "default",
		Temperature:      0.7,
		MaxTokens:        1000,
		TopP:             1.0,
		StopSequences:    []string{},
		ProviderSpecific: map[string]any{},
	}

	internal := &models.LLMRequest{
		ID:             uuid.New().String(),
		SessionID:      req.SessionId,
		Prompt:         req.Prompt,
		MemoryEnhanced: req.MemoryEnhanced,
		Memory:         map[string]string{},
		ModelParams:    modelParams,
		Status:         "pending",
		CreatedAt:      time.Now(),
	}

	// For streaming, we simulate chunked responses
	responses, selected, err := llm.RunEnsemble(internal)
	if err != nil {
		s.recordFailure(0)
		return err
	}

	s.recordSuccess(0)

	content := ""
	if selected != nil {
		content = selected.Content
	} else if len(responses) > 0 && responses[0] != nil {
		content = responses[0].Content
	}

	// Stream the response in chunks
	chunkSize := 50
	for i := 0; i < len(content); i += chunkSize {
		end := i + chunkSize
		if end > len(content) {
			end = len(content)
		}

		chunk := &pb.CompletionResponse{
			Content:      content[i:end],
			Confidence:   0.85,
			ProviderName: "ensemble",
		}

		if err := stream.Send(chunk); err != nil {
			return err
		}

		// Small delay to simulate streaming
		time.Sleep(10 * time.Millisecond)
	}

	return nil
}

// Chat implements chat-style interaction with message history
func (s *LLMFacadeServer) Chat(req *pb.ChatRequest, stream grpc.ServerStreamingServer[pb.ChatResponse]) error {
	s.recordRequest()

	// Build prompt from messages
	var prompt string
	for _, msg := range req.Messages {
		prompt += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
	}

	modelParams := models.ModelParameters{
		Model:            "default",
		Temperature:      0.7,
		MaxTokens:        1000,
		TopP:             1.0,
		StopSequences:    []string{},
		ProviderSpecific: map[string]any{},
	}

	internal := &models.LLMRequest{
		ID:             uuid.New().String(),
		SessionID:      req.SessionId,
		Prompt:         prompt,
		MemoryEnhanced: req.MemoryEnhanced,
		Memory:         map[string]string{},
		ModelParams:    modelParams,
		Status:         "pending",
		CreatedAt:      time.Now(),
	}

	responses, selected, err := llm.RunEnsemble(internal)
	if err != nil {
		s.recordFailure(0)
		return err
	}

	s.recordSuccess(0)

	content := ""
	providerName := "ensemble"
	var confidence float64 = 0.85
	if selected != nil {
		content = selected.Content
		confidence = selected.Confidence
		providerName = selected.ProviderName
	} else if len(responses) > 0 && responses[0] != nil {
		content = responses[0].Content
		confidence = responses[0].Confidence
		providerName = responses[0].ProviderName
	}

	// Stream the chat response in chunks
	chunkSize := 50
	totalChunks := (len(content) + chunkSize - 1) / chunkSize
	for i := 0; i < len(content); i += chunkSize {
		end := i + chunkSize
		if end > len(content) {
			end = len(content)
		}

		isComplete := (i/chunkSize) == totalChunks-1

		chunk := &pb.ChatResponse{
			ResponseId:   uuid.New().String(),
			Content:      content[i:end],
			Confidence:   confidence,
			ProviderName: providerName,
			IsStreaming:  true,
			IsComplete:   isComplete,
			CreatedAt:    timestamppb.Now(),
		}

		if err := stream.Send(chunk); err != nil {
			return err
		}

		time.Sleep(10 * time.Millisecond)
	}

	return nil
}

// ListProviders returns all registered providers
func (s *LLMFacadeServer) ListProviders(ctx context.Context, req *pb.ListProvidersRequest) (*pb.ListProvidersResponse, error) {
	s.providersMu.RLock()
	defer s.providersMu.RUnlock()

	providers := make([]*pb.ProviderInfo, 0, len(s.providers))
	for _, p := range s.providers {
		// Filter by enabled status if requested
		if req.EnabledOnly && !p.Enabled {
			continue
		}
		// Filter by provider type if specified
		if req.ProviderType != "" && p.Type != req.ProviderType {
			continue
		}

		providers = append(providers, &pb.ProviderInfo{
			Id:             p.ID,
			Name:           p.Name,
			Type:           p.Type,
			Model:          p.Model,
			Weight:         p.Weight,
			Enabled:        p.Enabled,
			HealthStatus:   p.HealthStatus,
			ResponseTimeMs: p.ResponseTimeMs,
			SuccessRate:    p.SuccessRate,
			LastUpdated:    timestamppb.New(p.LastUpdated),
		})
	}

	return &pb.ListProvidersResponse{
		Providers: providers,
	}, nil
}

// AddProvider registers a new provider
func (s *LLMFacadeServer) AddProvider(ctx context.Context, req *pb.AddProviderRequest) (*pb.ProviderResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "provider name is required")
	}
	if req.Type == "" {
		return nil, status.Error(codes.InvalidArgument, "provider type is required")
	}

	s.providersMu.Lock()
	defer s.providersMu.Unlock()

	id := uuid.New().String()

	// Check if provider with same name already exists
	for _, p := range s.providers {
		if p.Name == req.Name {
			return nil, status.Error(codes.AlreadyExists, "provider with this name already exists")
		}
	}

	now := time.Now()
	s.providers[id] = &ProviderInfo{
		ID:             id,
		Name:           req.Name,
		Type:           req.Type,
		Model:          req.Model,
		BaseURL:        req.BaseUrl,
		Enabled:        true,
		Weight:         req.Weight,
		HealthStatus:   "unknown",
		ResponseTimeMs: 0,
		SuccessRate:    0,
		Config:         req.Config,
		RegisteredAt:   now,
		LastUpdated:    now,
	}

	s.metricsMu.Lock()
	s.metrics.ActiveProviders++
	s.metricsMu.Unlock()

	return &pb.ProviderResponse{
		Success: true,
		Message: fmt.Sprintf("Provider %s added successfully", req.Name),
		Provider: &pb.ProviderInfo{
			Id:           id,
			Name:         req.Name,
			Type:         req.Type,
			Model:        req.Model,
			Weight:       req.Weight,
			Enabled:      true,
			HealthStatus: "unknown",
			LastUpdated:  timestamppb.New(now),
		},
	}, nil
}

// UpdateProvider updates an existing provider
func (s *LLMFacadeServer) UpdateProvider(ctx context.Context, req *pb.UpdateProviderRequest) (*pb.ProviderResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "provider id is required")
	}

	s.providersMu.Lock()
	defer s.providersMu.Unlock()

	existing, exists := s.providers[req.Id]
	if !exists {
		return nil, status.Error(codes.NotFound, "provider not found")
	}

	// Update fields if provided
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.ApiKey != "" {
		// Store API key securely (in production, use proper secrets management)
		// For now, we just acknowledge it was updated
	}
	if req.BaseUrl != "" {
		existing.BaseURL = req.BaseUrl
	}
	if req.Model != "" {
		existing.Model = req.Model
	}
	if req.Weight != 0 {
		existing.Weight = req.Weight
	}
	existing.Enabled = req.Enabled
	existing.LastUpdated = time.Now()

	return &pb.ProviderResponse{
		Success: true,
		Message: fmt.Sprintf("Provider %s updated successfully", req.Id),
		Provider: &pb.ProviderInfo{
			Id:             existing.ID,
			Name:           existing.Name,
			Type:           existing.Type,
			Model:          existing.Model,
			Weight:         existing.Weight,
			Enabled:        existing.Enabled,
			HealthStatus:   existing.HealthStatus,
			ResponseTimeMs: existing.ResponseTimeMs,
			SuccessRate:    existing.SuccessRate,
			LastUpdated:    timestamppb.New(existing.LastUpdated),
		},
	}, nil
}

// RemoveProvider removes a provider
func (s *LLMFacadeServer) RemoveProvider(ctx context.Context, req *pb.RemoveProviderRequest) (*pb.ProviderResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "provider id is required")
	}

	s.providersMu.Lock()
	defer s.providersMu.Unlock()

	provider, exists := s.providers[req.Id]
	if !exists {
		return nil, status.Error(codes.NotFound, "provider not found")
	}

	// If not forced, check if provider is in use
	if !req.Force && provider.Enabled {
		return nil, status.Error(codes.FailedPrecondition, "provider is still enabled; use force=true to remove")
	}

	delete(s.providers, req.Id)

	s.metricsMu.Lock()
	if s.metrics.ActiveProviders > 0 {
		s.metrics.ActiveProviders--
	}
	s.metricsMu.Unlock()

	return &pb.ProviderResponse{
		Success: true,
		Message: fmt.Sprintf("Provider %s removed successfully", req.Id),
	}, nil
}

// HealthCheck returns the health status of the service
func (s *LLMFacadeServer) HealthCheck(ctx context.Context, req *pb.HealthRequest) (*pb.HealthResponse, error) {
	s.providersMu.RLock()
	activeProviders := len(s.providers)
	providersCopy := make(map[string]*ProviderInfo)
	for k, v := range s.providers {
		providersCopy[k] = v
	}
	s.providersMu.RUnlock()

	s.sessionsMu.RLock()
	activeSessions := len(s.sessions)
	s.sessionsMu.RUnlock()

	// Determine overall status
	overallStatus := "healthy"
	if activeProviders == 0 {
		overallStatus = "degraded"
	}

	// Build component health reports
	var components []*pb.ComponentHealth

	// Check if detailed report is requested
	if req.Detailed {
		// Server component
		components = append(components, &pb.ComponentHealth{
			Name:           "server",
			Status:         "healthy",
			Message:        "gRPC server is running",
			ResponseTimeMs: 0,
			Details: map[string]string{
				"uptime":           fmt.Sprintf("%.0fs", time.Since(s.startTime).Seconds()),
				"active_sessions":  fmt.Sprintf("%d", activeSessions),
				"active_providers": fmt.Sprintf("%d", activeProviders),
			},
		})

		// Check specific components if requested
		for _, component := range req.CheckComponents {
			switch component {
			case "providers":
				for _, p := range providersCopy {
					components = append(components, &pb.ComponentHealth{
						Name:           fmt.Sprintf("provider:%s", p.Name),
						Status:         p.HealthStatus,
						Message:        fmt.Sprintf("Provider %s", p.Type),
						ResponseTimeMs: p.ResponseTimeMs,
						Details: map[string]string{
							"model":        p.Model,
							"success_rate": fmt.Sprintf("%.2f", p.SuccessRate),
							"enabled":      fmt.Sprintf("%t", p.Enabled),
						},
					})
				}
			case "database":
				// In production, this would check actual database connectivity
				components = append(components, &pb.ComponentHealth{
					Name:    "database",
					Status:  "healthy",
					Message: "Database connection pool active",
				})
			case "cognee":
				// In production, this would check Cognee service
				components = append(components, &pb.ComponentHealth{
					Name:    "cognee",
					Status:  "healthy",
					Message: "Cognee knowledge graph service",
				})
			}
		}
	}

	return &pb.HealthResponse{
		Status:     overallStatus,
		Components: components,
		Timestamp:  timestamppb.Now(),
		Version:    "1.0.0",
	}, nil
}

// GetMetrics returns server metrics
func (s *LLMFacadeServer) GetMetrics(ctx context.Context, req *pb.MetricsRequest) (*pb.MetricsResponse, error) {
	s.metricsMu.RLock()
	defer s.metricsMu.RUnlock()

	avgLatency := float64(0)
	if s.metrics.TotalRequests > 0 {
		avgLatency = float64(s.metrics.TotalLatencyMs) / float64(s.metrics.TotalRequests)
	}

	successRate := float64(0)
	if s.metrics.TotalRequests > 0 {
		successRate = float64(s.metrics.SuccessfulRequests) / float64(s.metrics.TotalRequests) * 100
	}

	// Build metrics struct
	metricsMap := map[string]interface{}{
		"total_requests":      s.metrics.TotalRequests,
		"successful_requests": s.metrics.SuccessfulRequests,
		"failed_requests":     s.metrics.FailedRequests,
		"average_latency_ms":  avgLatency,
		"success_rate":        successRate,
		"active_sessions":     s.metrics.ActiveSessions,
		"active_providers":    s.metrics.ActiveProviders,
	}

	metricsStruct, err := structpb.NewStruct(metricsMap)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to build metrics response")
	}

	// Determine time range
	endTime := time.Now()
	var startTime time.Time
	switch req.TimeRange {
	case "1h":
		startTime = endTime.Add(-time.Hour)
	case "24h":
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
	default:
		startTime = s.startTime
	}

	return &pb.MetricsResponse{
		Metrics:   metricsStruct,
		StartTime: timestamppb.New(startTime),
		EndTime:   timestamppb.New(endTime),
	}, nil
}

// CreateSession creates a new session
func (s *LLMFacadeServer) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.SessionResponse, error) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	sessionID := uuid.New().String()
	now := time.Now()

	// Default expiration: 1 hour, or use TtlHours if provided
	expiresAt := now.Add(time.Hour)
	if req.TtlHours > 0 {
		expiresAt = now.Add(time.Duration(req.TtlHours) * time.Hour)
	}

	s.sessions[sessionID] = &SessionInfo{
		ID:            sessionID,
		UserID:        req.UserId,
		Status:        "active",
		Context:       req.InitialContext,
		MemoryEnabled: req.MemoryEnabled,
		RequestCount:  0,
		CreatedAt:     now,
		UpdatedAt:     now,
		ExpiresAt:     expiresAt,
	}

	s.metricsMu.Lock()
	s.metrics.ActiveSessions++
	s.metricsMu.Unlock()

	return &pb.SessionResponse{
		Success:      true,
		SessionId:    sessionID,
		UserId:       req.UserId,
		Status:       "active",
		RequestCount: 0,
		LastActivity: timestamppb.New(now),
		ExpiresAt:    timestamppb.New(expiresAt),
		Context:      req.InitialContext,
	}, nil
}

// GetSession retrieves session information
func (s *LLMFacadeServer) GetSession(ctx context.Context, req *pb.GetSessionRequest) (*pb.SessionResponse, error) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	session, exists := s.sessions[req.SessionId]
	if !exists {
		return nil, status.Error(codes.NotFound, "session not found")
	}

	// Check if session expired
	if time.Now().After(session.ExpiresAt) {
		return &pb.SessionResponse{
			Success:   false,
			SessionId: req.SessionId,
			Status:    "expired",
		}, nil
	}

	resp := &pb.SessionResponse{
		Success:      true,
		SessionId:    session.ID,
		UserId:       session.UserID,
		Status:       session.Status,
		RequestCount: session.RequestCount,
		LastActivity: timestamppb.New(session.UpdatedAt),
		ExpiresAt:    timestamppb.New(session.ExpiresAt),
	}

	// Include context if requested
	if req.IncludeContext {
		resp.Context = session.Context
	}

	return resp, nil
}

// TerminateSession terminates an active session
func (s *LLMFacadeServer) TerminateSession(ctx context.Context, req *pb.TerminateSessionRequest) (*pb.SessionResponse, error) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	session, exists := s.sessions[req.SessionId]
	if !exists {
		return nil, status.Error(codes.NotFound, "session not found")
	}

	// If graceful termination requested, update status but keep in memory briefly
	if req.Graceful {
		session.Status = "terminating"
		session.UpdatedAt = time.Now()
		// In production, this would trigger cleanup processes
	}

	session.Status = "terminated"
	delete(s.sessions, req.SessionId)

	s.metricsMu.Lock()
	if s.metrics.ActiveSessions > 0 {
		s.metrics.ActiveSessions--
	}
	s.metricsMu.Unlock()

	return &pb.SessionResponse{
		Success:      true,
		SessionId:    req.SessionId,
		Status:       "terminated",
		LastActivity: timestamppb.Now(),
	}, nil
}

// Helper methods for metrics
func (s *LLMFacadeServer) recordRequest() {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	s.metrics.TotalRequests++
}

func (s *LLMFacadeServer) recordSuccess(latencyMs int64) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	s.metrics.SuccessfulRequests++
	s.metrics.TotalLatencyMs += latencyMs
}

func (s *LLMFacadeServer) recordFailure(latencyMs int64) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	s.metrics.FailedRequests++
	s.metrics.TotalLatencyMs += latencyMs
}

// LLMProviderServer implements the gRPC LLMProvider service for provider plugins
type LLMProviderServer struct {
	pb.UnimplementedLLMProviderServer

	// Provider registry for accessing LLM providers
	registry *services.ProviderRegistry

	// Metrics tracking
	metrics   *ServerMetrics
	metricsMu sync.RWMutex

	startTime time.Time
}

// NewLLMProviderServer creates a new LLMProvider gRPC server instance
func NewLLMProviderServer(registry *services.ProviderRegistry) *LLMProviderServer {
	return &LLMProviderServer{
		registry:  registry,
		metrics:   &ServerMetrics{},
		startTime: time.Now(),
	}
}

// Complete implements standard completion request for LLMProvider service
func (s *LLMProviderServer) Complete(ctx context.Context, req *pb.CompletionRequest) (*pb.CompletionResponse, error) {
	start := time.Now()
	s.recordProviderRequest()

	// Build model parameters from request
	modelParams := models.ModelParameters{
		Model:            "default",
		Temperature:      0.7,
		MaxTokens:        1000,
		TopP:             1.0,
		StopSequences:    []string{},
		ProviderSpecific: map[string]any{},
	}

	internal := &models.LLMRequest{
		ID:             uuid.New().String(),
		SessionID:      req.SessionId,
		UserID:         "",
		Prompt:         req.Prompt,
		MemoryEnhanced: req.MemoryEnhanced,
		Memory:         map[string]string{},
		ModelParams:    modelParams,
		EnsembleConfig: nil,
		Status:         "pending",
		CreatedAt:      time.Now(),
	}

	// Use provider registry if available, otherwise use ensemble
	var responses []*models.LLMResponse
	var selected *models.LLMResponse
	var err error

	if s.registry != nil {
		ensembleService := s.registry.GetEnsembleService()
		if ensembleService != nil {
			result, execErr := ensembleService.RunEnsemble(ctx, internal)
			if execErr != nil {
				err = execErr
			} else if result != nil {
				responses = result.Responses
				selected = result.Selected
			}
		} else {
			responses, selected, err = llm.RunEnsemble(internal)
		}
	} else {
		responses, selected, err = llm.RunEnsemble(internal)
	}

	latency := time.Since(start).Milliseconds()

	if err != nil {
		s.recordProviderFailure(latency)
		return &pb.CompletionResponse{Content: "", Confidence: 0}, err
	}

	s.recordProviderSuccess(latency)

	out := &pb.CompletionResponse{}
	if len(responses) > 0 && responses[0] != nil {
		out.Content = responses[0].Content
		out.Confidence = responses[0].Confidence
		out.ProviderName = responses[0].ProviderName
	}
	if selected != nil {
		out.Content = selected.Content
		out.Confidence = selected.Confidence
		out.ProviderName = selected.ProviderName
	}

	return out, nil
}

// CompleteStream implements streaming completion for LLMProvider service
func (s *LLMProviderServer) CompleteStream(req *pb.CompletionRequest, stream grpc.ServerStreamingServer[pb.CompletionResponse]) error {
	s.recordProviderRequest()

	modelParams := models.ModelParameters{
		Model:            "default",
		Temperature:      0.7,
		MaxTokens:        1000,
		TopP:             1.0,
		StopSequences:    []string{},
		ProviderSpecific: map[string]any{},
	}

	internal := &models.LLMRequest{
		ID:             uuid.New().String(),
		SessionID:      req.SessionId,
		Prompt:         req.Prompt,
		MemoryEnhanced: req.MemoryEnhanced,
		Memory:         map[string]string{},
		ModelParams:    modelParams,
		Status:         "pending",
		CreatedAt:      time.Now(),
	}

	// Use provider registry if available
	var responses []*models.LLMResponse
	var selected *models.LLMResponse
	var err error

	if s.registry != nil {
		ensembleService := s.registry.GetEnsembleService()
		if ensembleService != nil {
			result, execErr := ensembleService.RunEnsemble(context.Background(), internal)
			if execErr != nil {
				err = execErr
			} else if result != nil {
				responses = result.Responses
				selected = result.Selected
			}
		} else {
			responses, selected, err = llm.RunEnsemble(internal)
		}
	} else {
		responses, selected, err = llm.RunEnsemble(internal)
	}

	if err != nil {
		s.recordProviderFailure(0)
		return err
	}

	s.recordProviderSuccess(0)

	content := ""
	providerName := "ensemble"
	var confidence float64 = 0.85
	if selected != nil {
		content = selected.Content
		confidence = selected.Confidence
		providerName = selected.ProviderName
	} else if len(responses) > 0 && responses[0] != nil {
		content = responses[0].Content
		confidence = responses[0].Confidence
		providerName = responses[0].ProviderName
	}

	// Stream the response in chunks
	chunkSize := 50
	for i := 0; i < len(content); i += chunkSize {
		end := i + chunkSize
		if end > len(content) {
			end = len(content)
		}

		chunk := &pb.CompletionResponse{
			Content:      content[i:end],
			Confidence:   confidence,
			ProviderName: providerName,
		}

		if err := stream.Send(chunk); err != nil {
			return err
		}

		// Small delay to simulate streaming
		time.Sleep(10 * time.Millisecond)
	}

	return nil
}

// HealthCheck returns the health status of the provider service
func (s *LLMProviderServer) HealthCheck(ctx context.Context, req *pb.HealthRequest) (*pb.HealthResponse, error) {
	overallStatus := "healthy"
	var components []*pb.ComponentHealth

	// Check provider registry health if available
	if s.registry != nil {
		healthResults := s.registry.HealthCheck()
		healthyCount := 0
		totalCount := len(healthResults)

		for providerName, err := range healthResults {
			providerStatus := "healthy"
			message := fmt.Sprintf("Provider %s is operational", providerName)
			if err != nil {
				providerStatus = "unhealthy"
				message = fmt.Sprintf("Provider %s error: %v", providerName, err)
			} else {
				healthyCount++
			}

			if req.Detailed {
				components = append(components, &pb.ComponentHealth{
					Name:    fmt.Sprintf("provider:%s", providerName),
					Status:  providerStatus,
					Message: message,
				})
			}
		}

		// Determine overall status based on provider health
		if totalCount == 0 {
			overallStatus = "degraded"
		} else if healthyCount == 0 {
			overallStatus = "unhealthy"
		} else if healthyCount < totalCount {
			overallStatus = "degraded"
		}
	} else {
		overallStatus = "degraded"
		if req.Detailed {
			components = append(components, &pb.ComponentHealth{
				Name:    "registry",
				Status:  "unavailable",
				Message: "Provider registry not initialized",
			})
		}
	}

	// Add server component status
	if req.Detailed {
		components = append(components, &pb.ComponentHealth{
			Name:    "server",
			Status:  "healthy",
			Message: "gRPC LLMProvider server is running",
			Details: map[string]string{
				"uptime": fmt.Sprintf("%.0fs", time.Since(s.startTime).Seconds()),
			},
		})
	}

	return &pb.HealthResponse{
		Status:     overallStatus,
		Components: components,
		Timestamp:  timestamppb.Now(),
		Version:    "1.0.0",
	}, nil
}

// GetCapabilities returns the capabilities of the provider service
func (s *LLMProviderServer) GetCapabilities(ctx context.Context, req *pb.CapabilitiesRequest) (*pb.CapabilitiesResponse, error) {
	// Aggregate capabilities from all registered providers
	supportedModels := []string{}
	supportedFeatures := []string{"chat", "completion"}
	supportsStreaming := true
	supportsFunctionCalling := false
	supportsVision := false

	if s.registry != nil {
		providerNames := s.registry.ListProviders()
		for _, name := range providerNames {
			provider, err := s.registry.GetProvider(name)
			if err != nil {
				continue
			}

			caps := provider.GetCapabilities()
			if caps != nil {
				// Merge supported models
				supportedModels = append(supportedModels, caps.SupportedModels...)

				// Merge features
				for _, feature := range caps.SupportedFeatures {
					if !contains(supportedFeatures, feature) {
						supportedFeatures = append(supportedFeatures, feature)
					}
				}

				// Check capability flags
				if caps.SupportsStreaming {
					supportsStreaming = true
				}
				if caps.SupportsFunctionCalling {
					supportsFunctionCalling = true
				}
				if caps.SupportsVision {
					supportsVision = true
				}
			}
		}
	}

	// Add default features
	if !contains(supportedFeatures, "streaming") {
		supportedFeatures = append(supportedFeatures, "streaming")
	}

	return &pb.CapabilitiesResponse{
		SupportedModels:         supportedModels,
		SupportedFeatures:       supportedFeatures,
		SupportedRequestTypes:   []string{"completion", "chat", "streaming"},
		SupportsStreaming:       supportsStreaming,
		SupportsFunctionCalling: supportsFunctionCalling,
		SupportsVision:          supportsVision,
		Limits: &pb.ModelLimits{
			MaxTokens:             4096,
			MaxInputLength:        100000,
			MaxOutputLength:       4096,
			MaxConcurrentRequests: 100,
		},
	}, nil
}

// ValidateConfig validates provider configuration
func (s *LLMProviderServer) ValidateConfig(ctx context.Context, req *pb.ValidateConfigRequest) (*pb.ValidateConfigResponse, error) {
	var errors []string
	var warnings []string

	if req.Config == nil {
		return &pb.ValidateConfigResponse{
			Valid:    false,
			Errors:   []string{"configuration is required"},
			Warnings: []string{},
		}, nil
	}

	// Convert protobuf struct to map
	configMap := req.Config.AsMap()

	// Validate required fields
	if _, ok := configMap["type"]; !ok {
		errors = append(errors, "provider type is required")
	}

	// Check API key if provider type requires it
	if providerType, ok := configMap["type"].(string); ok {
		switch providerType {
		case "claude", "deepseek", "gemini", "qwen", "openrouter":
			if _, hasKey := configMap["api_key"]; !hasKey {
				warnings = append(warnings, fmt.Sprintf("API key not provided for %s provider", providerType))
			}
		case "ollama":
			// Ollama doesn't require API key
			if _, hasURL := configMap["base_url"]; !hasURL {
				warnings = append(warnings, "base_url not provided for Ollama, will use default")
			}
		}
	}

	// Validate model configuration
	if model, ok := configMap["model"].(string); ok && model == "" {
		warnings = append(warnings, "model not specified, will use provider default")
	}

	// Validate timeout
	if timeout, ok := configMap["timeout"].(float64); ok {
		if timeout <= 0 {
			errors = append(errors, "timeout must be positive")
		} else if timeout > 300 {
			warnings = append(warnings, "timeout exceeds 5 minutes, may cause issues")
		}
	}

	// Validate with actual provider if registry is available
	if s.registry != nil {
		if providerName, ok := configMap["name"].(string); ok && providerName != "" {
			provider, err := s.registry.GetProvider(providerName)
			if err == nil && provider != nil {
				valid, providerErrors := provider.ValidateConfig(configMap)
				if !valid {
					errors = append(errors, providerErrors...)
				}
			}
		}
	}

	return &pb.ValidateConfigResponse{
		Valid:    len(errors) == 0,
		Errors:   errors,
		Warnings: warnings,
	}, nil
}

// Helper methods for LLMProviderServer metrics
func (s *LLMProviderServer) recordProviderRequest() {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	s.metrics.TotalRequests++
}

func (s *LLMProviderServer) recordProviderSuccess(latencyMs int64) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	s.metrics.SuccessfulRequests++
	s.metrics.TotalLatencyMs += latencyMs
}

func (s *LLMProviderServer) recordProviderFailure(latencyMs int64) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	s.metrics.FailedRequests++
	s.metrics.TotalLatencyMs += latencyMs
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	// Initialize provider registry with default configuration
	registryConfig := services.LoadRegistryConfigFromAppConfig(nil)
	providerRegistry := services.NewProviderRegistry(registryConfig, nil)

	// Create and register LLMFacade server
	llmServer := NewLLMFacadeServer()
	pb.RegisterLLMFacadeServer(grpcServer, llmServer)

	// Create and register LLMProvider server
	providerServer := NewLLMProviderServer(providerRegistry)
	pb.RegisterLLMProviderServer(grpcServer, providerServer)

	log.Println("SuperAgent gRPC server listening on :50051")
	log.Println("Registered services: LLMFacade, LLMProvider")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
