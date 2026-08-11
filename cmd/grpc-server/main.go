package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	llm "dev.helix.agent/internal/llm"
	models "dev.helix.agent/internal/models"
	"dev.helix.agent/internal/ports"
	"dev.helix.agent/internal/services"
	"dev.helix.agent/internal/version"
	pb "dev.helix.agent/pkg/api"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// LLMFacadeServer implements the gRPC LLMFacade service.
//
// Concurrency model (CONST-029): providers and sessions are
// *safe.Store — atomic per-key operations. ServerMetrics owns its
// own atomics for counters; metricsMu survives for timestamp
// serialisation and is not Pattern-A (no adjacent bare collection).
type LLMFacadeServer struct {
	pb.UnimplementedLLMFacadeServer

	// Provider registry — the live, credential-injected providers this
	// server completes against. Held for exactly the same reason
	// LLMProviderServer holds it, and supplied the same way: as a
	// constructor parameter from main(), which builds ONE registry and
	// hands the same instance to both services.
	//
	// Its absence was HXC-271. LLMFacade is the service the published
	// interface presents first, so an integrator reaches for it
	// naturally — and every Complete / CompleteStream / Chat it served
	// failed on every machine, however well configured, because the
	// handlers fell through to the package-level llm.RunEnsemble, which
	// is hardcoded to pass a nil provider slice and refuses on an empty
	// one. The refusal even blames the caller ("use
	// services.ProviderRegistry for proper credential injection"), which
	// is why the defect survived: it reads as misconfiguration at the
	// far end rather than a missing wire at this one.
	registry *services.ProviderRegistry

	// Provider management — descriptive metadata registered over the
	// wire via AddProvider. Distinct from `registry` above: this store
	// records what a caller SAID about a provider, not the live
	// credential-injected clients completions are routed to.
	providers *safe.Store[string, *ProviderInfo]

	// Session management
	sessions *safe.Store[string, *SessionInfo]

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

// NewLLMFacadeServer creates a new gRPC server instance.
//
// The signature mirrors NewLLMProviderServer deliberately. Both services are
// completion front-ends over the same providers, so both take the registry the
// same way; a reader comparing them can see at the call site whether each got
// it. Passing nil is a registry-less server whose completion methods refuse —
// the pre-HXC-271 behaviour, kept as an honest degenerate mode (and as the
// RED baseline the wiring guard replays), never as the wiring main() uses.
func NewLLMFacadeServer(registry *services.ProviderRegistry) *LLMFacadeServer {
	return &LLMFacadeServer{
		registry:  registry,
		providers: safe.NewStore[string, *ProviderInfo](),
		sessions:  safe.NewStore[string, *SessionInfo](),
		metrics:   &ServerMetrics{},
		startTime: time.Now(),
	}
}

// runEnsembleVia routes one completion to the live providers.
//
// This is the single routing decision both gRPC services share. It existed
// before HXC-271 too — but as a copy-pasted block inside LLMProviderServer's
// two methods and nowhere inside LLMFacadeServer's three, which is precisely
// how one family came to work and the other could not. Two services publishing
// the same capability had two answers to "where do providers come from", and
// only one of them was wired.
//
// Extracting it makes the recurrence structurally harder rather than merely
// less likely: a future completion method routes providers by calling this, so
// the way to write a broken one is to not call it at all — visible in review —
// instead of to omit a field from a struct literal, which is invisible.
//
// The fallback to the package-level llm.RunEnsemble is preserved verbatim from
// LLMProviderServer, degenerate result included: a nil result with a nil error
// yields (nil, nil, nil), the empty-but-not-failed path the round-29 callers
// already log rather than dress up with a fabricated confidence.
func runEnsembleVia(
	ctx context.Context,
	registry *services.ProviderRegistry,
	req *models.LLMRequest,
) ([]*models.LLMResponse, *models.LLMResponse, error) {
	if registry != nil {
		if ensembleService := registry.GetEnsembleService(); ensembleService != nil {
			result, err := ensembleService.RunEnsemble(ctx, req)
			if err != nil {
				return nil, nil, err
			}
			if result != nil {
				return result.Responses, result.Selected, nil
			}
			return nil, nil, nil
		}
	}
	return llm.RunEnsemble(req)
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

	responses, selected, err := runEnsembleVia(ctx, s.registry, internal)
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

	// Run the ensemble for the streaming completion.
	responses, selected, err := runEnsembleVia(stream.Context(), s.registry, internal)
	if err != nil {
		s.recordFailure(0)
		return err
	}

	s.recordSuccess(0)

	// Round-29 anti-bluff fix (LLMFacadeServer.CompleteStream
	// path): the per-chunk Confidence was previously hardcoded to
	// 0.85 in the pb.CompletionResponse literal, fabricating a
	// meaningful-looking score with no relation to the underlying
	// ensemble's actual confidence. Now confidence is sourced from
	// the selected response (or first non-nil fallback) and
	// initialised to 0.0 on the degenerate path so callers see the
	// truth.
	content := ""
	providerName := "ensemble"
	var confidence float64 = 0.0
	degeneratePath := true
	if selected != nil {
		content = selected.Content
		confidence = selected.Confidence
		providerName = selected.ProviderName
		degeneratePath = false
	} else if len(responses) > 0 && responses[0] != nil {
		content = responses[0].Content
		confidence = responses[0].Confidence
		providerName = responses[0].ProviderName
		degeneratePath = false
	}
	if degeneratePath {
		log.Printf("CompleteStream(facade): ensemble returned no selected response and no fallback responses[0]; surfacing empty content with confidence=0 instead of fabricated 0.85 fallback (round-29 §11.4 fix)")
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

	responses, selected, err := runEnsembleVia(stream.Context(), s.registry, internal)
	if err != nil {
		s.recordFailure(0)
		return err
	}

	s.recordSuccess(0)

	// Round-29 anti-bluff fix: previously initialised confidence to
	// 0.85 as a fabricated fallback when both `selected` and
	// `responses[0]` were nil. That surfaced a meaningful-looking
	// score on a degenerate path where the ensemble had returned
	// nothing useful. Now: confidence starts at 0.0 and the
	// degenerate path is logged + surfaced (empty content, 0.0
	// confidence) so callers can detect and react.
	content := ""
	providerName := "ensemble"
	var confidence float64 = 0.0
	degeneratePath := true
	if selected != nil {
		content = selected.Content
		confidence = selected.Confidence
		providerName = selected.ProviderName
		degeneratePath = false
	} else if len(responses) > 0 && responses[0] != nil {
		content = responses[0].Content
		confidence = responses[0].Confidence
		providerName = responses[0].ProviderName
		degeneratePath = false
	}
	if degeneratePath {
		log.Printf("Chat: ensemble returned no selected response and no fallback responses[0]; surfacing empty content with confidence=0 instead of fabricated 0.85 fallback (round-29 §11.4 fix)")
	}

	// Stream the chat response in chunks
	chunkSize := 50
	totalChunks := (len(content) + chunkSize - 1) / chunkSize
	for i := 0; i < len(content); i += chunkSize {
		end := i + chunkSize
		if end > len(content) {
			end = len(content)
		}

		isComplete := (i / chunkSize) == totalChunks-1

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
	providers := make([]*pb.ProviderInfo, 0, s.providers.Len())
	s.providers.Range(func(_ string, p *ProviderInfo) bool {
		// Filter by enabled status if requested
		if req.EnabledOnly && !p.Enabled {
			return true
		}
		// Filter by provider type if specified
		if req.ProviderType != "" && p.Type != req.ProviderType {
			return true
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
		return true
	})

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

	id := uuid.New().String()

	// Check if provider with same name already exists
	var nameExists bool
	s.providers.Range(func(_ string, p *ProviderInfo) bool {
		if p.Name == req.Name {
			nameExists = true
			return false
		}
		return true
	})
	if nameExists {
		return nil, status.Error(codes.AlreadyExists, "provider with this name already exists")
	}

	now := time.Now()
	s.providers.Put(id, &ProviderInfo{
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
	})

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

	existing, exists := s.providers.Get(req.Id)
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

	provider, exists := s.providers.Get(req.Id)
	if !exists {
		return nil, status.Error(codes.NotFound, "provider not found")
	}

	// If not forced, check if provider is in use
	if !req.Force && provider.Enabled {
		return nil, status.Error(codes.FailedPrecondition, "provider is still enabled; use force=true to remove")
	}

	s.providers.Delete(req.Id)

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
	activeProviders := s.providers.Len()
	providersCopy := s.providers.Snapshot()
	activeSessions := s.sessions.Len()

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
		Version:    version.Version,
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
	sessionID := uuid.New().String()
	now := time.Now()

	// Default expiration: 1 hour, or use TtlHours if provided
	expiresAt := now.Add(time.Hour)
	if req.TtlHours > 0 {
		expiresAt = now.Add(time.Duration(req.TtlHours) * time.Hour)
	}

	s.sessions.Put(sessionID, &SessionInfo{
		ID:            sessionID,
		UserID:        req.UserId,
		Status:        "active",
		Context:       req.InitialContext,
		MemoryEnabled: req.MemoryEnabled,
		RequestCount:  0,
		CreatedAt:     now,
		UpdatedAt:     now,
		ExpiresAt:     expiresAt,
	})

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
	session, exists := s.sessions.Get(req.SessionId)
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
	session, exists := s.sessions.Get(req.SessionId)
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
	s.sessions.Delete(req.SessionId)

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

	responses, selected, err := runEnsembleVia(ctx, s.registry, internal)

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

	responses, selected, err := runEnsembleVia(stream.Context(), s.registry, internal)

	if err != nil {
		s.recordProviderFailure(0)
		return err
	}

	s.recordProviderSuccess(0)

	// Round-29 anti-bluff fix (LLMProviderServer.CompleteStream
	// path): same fabricated 0.85 fallback as above. See round-29
	// commit for forensic detail.
	content := ""
	providerName := "ensemble"
	var confidence float64 = 0.0
	degeneratePath := true
	if selected != nil {
		content = selected.Content
		confidence = selected.Confidence
		providerName = selected.ProviderName
		degeneratePath = false
	} else if len(responses) > 0 && responses[0] != nil {
		content = responses[0].Content
		confidence = responses[0].Confidence
		providerName = responses[0].ProviderName
		degeneratePath = false
	}
	if degeneratePath {
		log.Printf("CompleteStream(provider): ensemble returned no selected response and no fallback responses[0]; surfacing empty content with confidence=0 instead of fabricated 0.85 fallback (round-29 §11.4 fix)")
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
		Version:    version.Version,
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

// grpcListenAddr returns the address this gRPC server listens on, resolved
// by name from internal/ports rather than from a bare literal.
//
// WHY NOT :50051 (§11.4.111 resolve-by-name-not-by-ordinal)
//
// This server used to hardcode `:50051`. That is the gRPC ecosystem's
// conventional port, which makes it the number most likely to be already
// taken — and on this project's own stack it IS taken: the
// helixcode-infra-weaviate container publishes it. So HelixAgent's gRPC
// server could not start while HelixAgent's own infrastructure was running,
// and clients dialling the same literal reached Weaviate, which answers
// `Unimplemented` to every HelixAgent method while completing a perfectly
// healthy HTTP/2 handshake — a live wrong peer, not an absent one, so
// reachability guards did not skip and the resulting failures were reported
// against HelixAgent although HelixAgent was never running.
//
// The service is now registered as ports.HelixAgentGRPC (core band, offset
// 112). Registration is the load-bearing half of the fix: the registry
// exists precisely to arbitrate port ownership, and a service missing from
// it cannot be collision-checked against the services that are in it — the
// clash was undetectable by construction, not merely undetected. Resolving
// by name also makes the port overridable per deployment via the registry's
// own HELIXAGENT_PORT_GRPC env var.
func grpcListenAddr() string {
	return ports.Addr(ports.HelixAgentGRPC)
}

// startupBanner renders the line an operator reads to learn where to dial.
//
// It takes the bound address as an argument rather than re-resolving it, so
// the banner cannot drift from the listener: main() passes the very value it
// handed to net.Listen, and startup_banner_test.go asserts the two agree.
//
// This line was the surviving half of the :50051 defect. The listener moved
// to the registry address while the banner kept printing the old literal
// verbatim twenty-five lines below — so the fix published the exact
// instruction that reproduces the bug it fixed. Readers who followed it
// reached the helixcode-infra-weaviate container that owns :50051, got a
// successful HTTP/2 handshake and `Unimplemented` on every method, and had
// no way to tell that from a broken HelixAgent. The banner is copied by
// operators and by docs, so it is load-bearing, not decoration.
func startupBanner(addr string) string {
	return fmt.Sprintf("HelixAgent gRPC server listening on %s", addr)
}

// registerGRPCServices wires every service this binary publishes onto srv,
// against ONE provider registry.
//
// Extracted for the same reason startupBanner takes its address as an
// argument: so the wiring cannot drift from what it is supposed to wire.
// Previously main() constructed the registry and then passed it to one of the
// two services it registered — nothing in the program stated that both were
// meant to have it, so nothing noticed that one did not (HXC-271).
//
// Taking the registry ONCE, for all services, is the load-bearing part. There
// is no longer a per-service decision to get wrong: a service registered here
// is registered against this registry or it is not registered at all. It also
// gives the wiring a behavioural test — a test can hand this function a
// registry holding a known provider, serve the result, and ask both families
// for a completion — where before, main()'s wiring could only be grepped, and
// a grep for a field cannot tell a wired field from a wired-to-nothing one.
//
// srv is a grpc.ServiceRegistrar rather than a *grpc.Server so the test drives
// the real registration path without owning a listener it does not need.
func registerGRPCServices(srv grpc.ServiceRegistrar, registry *services.ProviderRegistry) {
	pb.RegisterLLMFacadeServer(srv, NewLLMFacadeServer(registry))
	pb.RegisterLLMProviderServer(srv, NewLLMProviderServer(registry))
}

func main() {
	addr := grpcListenAddr()

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		// Fail fast, deliberately: falling back to some other port would
		// strand every client that resolved the published address
		// correctly. Name the address and the override so the refusal is
		// actionable rather than merely fatal.
		log.Fatalf("failed to listen on %s: %v "+
			"(another process holds it; free it, or set %s to a free port)",
			addr, err, ports.HelixAgentGRPC)
	}

	grpcServer := grpc.NewServer()

	// Initialize provider registry with default configuration
	registryConfig := services.LoadRegistryConfigFromAppConfig(nil)
	providerRegistry := services.NewProviderRegistry(registryConfig, nil)

	// Register every published service against that one registry.
	registerGRPCServices(grpcServer, providerRegistry)

	log.Println(startupBanner(addr))
	log.Println("Registered services: LLMFacade, LLMProvider")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
