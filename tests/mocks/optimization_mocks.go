package mocks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
)

// MockLlamaIndexServer creates a mock LlamaIndex HTTP server for testing.
type MockLlamaIndexServer struct {
	Server        *httptest.Server
	QueryCount    int
	HealthCount   int
	ShouldFail    bool
	ResponseDelay time.Duration
	mu            sync.Mutex
}

// NewMockLlamaIndexServer creates and starts a new mock LlamaIndex server.
func NewMockLlamaIndexServer() *MockLlamaIndexServer {
	mock := &MockLlamaIndexServer{}
	mock.Server = httptest.NewServer(http.HandlerFunc(mock.handler))
	return mock
}

func (m *MockLlamaIndexServer) handler(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ResponseDelay > 0 {
		time.Sleep(m.ResponseDelay)
	}

	switch r.URL.Path {
	case "/health":
		m.HealthCount++
		if m.ShouldFail {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})

	case "/query":
		m.QueryCount++
		if m.ShouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "query failed"})
			return
		}
		resp := map[string]interface{}{
			"response": "This is a test response from LlamaIndex",
			"sources": []map[string]interface{}{
				{
					"content":  "Relevant context from document 1",
					"metadata": map[string]string{"source": "doc1.txt"},
					"score":    0.95,
				},
				{
					"content":  "Additional context from document 2",
					"metadata": map[string]string{"source": "doc2.txt"},
					"score":    0.87,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// URL returns the server URL.
func (m *MockLlamaIndexServer) URL() string {
	return m.Server.URL
}

// Close shuts down the mock server.
func (m *MockLlamaIndexServer) Close() {
	m.Server.Close()
}

// Reset clears all counters.
func (m *MockLlamaIndexServer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.QueryCount = 0
	m.HealthCount = 0
}

// MockLangChainServer creates a mock LangChain HTTP server for testing.
type MockLangChainServer struct {
	Server         *httptest.Server
	DecomposeCount int
	ExecuteCount   int
	HealthCount    int
	ShouldFail     bool
	ResponseDelay  time.Duration
	mu             sync.Mutex
}

// NewMockLangChainServer creates and starts a new mock LangChain server.
func NewMockLangChainServer() *MockLangChainServer {
	mock := &MockLangChainServer{}
	mock.Server = httptest.NewServer(http.HandlerFunc(mock.handler))
	return mock
}

func (m *MockLangChainServer) handler(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ResponseDelay > 0 {
		time.Sleep(m.ResponseDelay)
	}

	switch r.URL.Path {
	case "/health":
		m.HealthCount++
		if m.ShouldFail {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})

	case "/decompose":
		m.DecomposeCount++
		if m.ShouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "decomposition failed"})
			return
		}
		resp := map[string]interface{}{
			"subtasks": []map[string]interface{}{
				{"description": "Step 1: Analyze the problem", "order": 1},
				{"description": "Step 2: Design a solution", "order": 2},
				{"description": "Step 3: Implement the solution", "order": 3},
				{"description": "Step 4: Test the implementation", "order": 4},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	case "/execute":
		m.ExecuteCount++
		if m.ShouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "execution failed"})
			return
		}
		resp := map[string]interface{}{
			"result":      "Task executed successfully",
			"steps_taken": 4,
			"output":      "The operation completed with all steps executed.",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// URL returns the server URL.
func (m *MockLangChainServer) URL() string {
	return m.Server.URL
}

// Close shuts down the mock server.
func (m *MockLangChainServer) Close() {
	m.Server.Close()
}

// Reset clears all counters.
func (m *MockLangChainServer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DecomposeCount = 0
	m.ExecuteCount = 0
	m.HealthCount = 0
}

// MockSGLangServer creates a mock SGLang HTTP server for testing.
type MockSGLangServer struct {
	Server          *httptest.Server
	GenerateCount   int
	WarmPrefixCount int
	HealthCount     int
	ShouldFail      bool
	ResponseDelay   time.Duration
	mu              sync.Mutex
}

// NewMockSGLangServer creates and starts a new mock SGLang server.
func NewMockSGLangServer() *MockSGLangServer {
	mock := &MockSGLangServer{}
	mock.Server = httptest.NewServer(http.HandlerFunc(mock.handler))
	return mock
}

func (m *MockSGLangServer) handler(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ResponseDelay > 0 {
		time.Sleep(m.ResponseDelay)
	}

	switch r.URL.Path {
	case "/health":
		m.HealthCount++
		if m.ShouldFail {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})

	case "/generate":
		m.GenerateCount++
		if m.ShouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "generation failed"})
			return
		}
		resp := map[string]interface{}{
			"text":         "Generated response from SGLang",
			"meta_info":    map[string]interface{}{"tokens_used": 150},
			"finish_reason": "stop",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	case "/warm_prefix":
		m.WarmPrefixCount++
		if m.ShouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "warm prefix failed"})
			return
		}
		resp := map[string]interface{}{
			"success":   true,
			"prefix_id": "prefix_12345",
			"cached":    true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// URL returns the server URL.
func (m *MockSGLangServer) URL() string {
	return m.Server.URL
}

// Close shuts down the mock server.
func (m *MockSGLangServer) Close() {
	m.Server.Close()
}

// Reset clears all counters.
func (m *MockSGLangServer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GenerateCount = 0
	m.WarmPrefixCount = 0
	m.HealthCount = 0
}

// MockGuidanceServer creates a mock Guidance HTTP server for testing.
type MockGuidanceServer struct {
	Server       *httptest.Server
	ExecuteCount int
	HealthCount  int
	ShouldFail   bool
	ResponseDelay time.Duration
	mu           sync.Mutex
}

// NewMockGuidanceServer creates and starts a new mock Guidance server.
func NewMockGuidanceServer() *MockGuidanceServer {
	mock := &MockGuidanceServer{}
	mock.Server = httptest.NewServer(http.HandlerFunc(mock.handler))
	return mock
}

func (m *MockGuidanceServer) handler(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ResponseDelay > 0 {
		time.Sleep(m.ResponseDelay)
	}

	switch r.URL.Path {
	case "/health":
		m.HealthCount++
		if m.ShouldFail {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})

	case "/execute":
		m.ExecuteCount++
		if m.ShouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "execution failed"})
			return
		}
		resp := map[string]interface{}{
			"result": map[string]interface{}{
				"output": "Structured output from Guidance",
				"name":   "John Doe",
				"age":    30,
			},
			"success": true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// URL returns the server URL.
func (m *MockGuidanceServer) URL() string {
	return m.Server.URL
}

// Close shuts down the mock server.
func (m *MockGuidanceServer) Close() {
	m.Server.Close()
}

// Reset clears all counters.
func (m *MockGuidanceServer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ExecuteCount = 0
	m.HealthCount = 0
}

// MockLMQLServer creates a mock LMQL HTTP server for testing.
type MockLMQLServer struct {
	Server       *httptest.Server
	QueryCount   int
	HealthCount  int
	ShouldFail   bool
	ResponseDelay time.Duration
	mu           sync.Mutex
}

// NewMockLMQLServer creates and starts a new mock LMQL server.
func NewMockLMQLServer() *MockLMQLServer {
	mock := &MockLMQLServer{}
	mock.Server = httptest.NewServer(http.HandlerFunc(mock.handler))
	return mock
}

func (m *MockLMQLServer) handler(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ResponseDelay > 0 {
		time.Sleep(m.ResponseDelay)
	}

	switch r.URL.Path {
	case "/health":
		m.HealthCount++
		if m.ShouldFail {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})

	case "/query":
		m.QueryCount++
		if m.ShouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "query failed"})
			return
		}
		resp := map[string]interface{}{
			"result": "Query result from LMQL",
			"variables": map[string]interface{}{
				"answer":  "The capital of France is Paris.",
				"country": "France",
			},
			"prompt_tokens":     50,
			"completion_tokens": 20,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// URL returns the server URL.
func (m *MockLMQLServer) URL() string {
	return m.Server.URL
}

// Close shuts down the mock server.
func (m *MockLMQLServer) Close() {
	m.Server.Close()
}

// Reset clears all counters.
func (m *MockLMQLServer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.QueryCount = 0
	m.HealthCount = 0
}

// MockSemanticCacheServer creates a mock semantic cache HTTP server for testing.
type MockSemanticCacheServer struct {
	Server      *httptest.Server
	GetCount    int
	SetCount    int
	HealthCount int
	CacheData   map[string]interface{}
	ShouldFail  bool
	ResponseDelay time.Duration
	mu          sync.Mutex
}

// NewMockSemanticCacheServer creates and starts a new mock semantic cache server.
func NewMockSemanticCacheServer() *MockSemanticCacheServer {
	mock := &MockSemanticCacheServer{
		CacheData: make(map[string]interface{}),
	}
	mock.Server = httptest.NewServer(http.HandlerFunc(mock.handler))
	return mock
}

func (m *MockSemanticCacheServer) handler(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ResponseDelay > 0 {
		time.Sleep(m.ResponseDelay)
	}

	switch r.URL.Path {
	case "/health":
		m.HealthCount++
		if m.ShouldFail {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})

	case "/get":
		m.GetCount++
		if m.ShouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "get failed"})
			return
		}
		// Simulate cache hit for testing
		if len(m.CacheData) > 0 {
			resp := map[string]interface{}{
				"hit":        true,
				"response":   "Cached response",
				"similarity": 0.95,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		// Cache miss
		resp := map[string]interface{}{
			"hit": false,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	case "/set":
		m.SetCount++
		if m.ShouldFail {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "set failed"})
			return
		}
		// Store in mock cache
		var data map[string]interface{}
		json.NewDecoder(r.Body).Decode(&data)
		key := fmt.Sprintf("cache_%d", m.SetCount)
		m.CacheData[key] = data
		resp := map[string]interface{}{
			"success": true,
			"key":     key,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// URL returns the server URL.
func (m *MockSemanticCacheServer) URL() string {
	return m.Server.URL
}

// Close shuts down the mock server.
func (m *MockSemanticCacheServer) Close() {
	m.Server.Close()
}

// Reset clears all counters and cache data.
func (m *MockSemanticCacheServer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetCount = 0
	m.SetCount = 0
	m.HealthCount = 0
	m.CacheData = make(map[string]interface{})
}

// OptimizationMockServers holds all mock servers for optimization testing.
type OptimizationMockServers struct {
	LlamaIndex    *MockLlamaIndexServer
	LangChain     *MockLangChainServer
	SGLang        *MockSGLangServer
	Guidance      *MockGuidanceServer
	LMQL          *MockLMQLServer
	SemanticCache *MockSemanticCacheServer
}

// NewOptimizationMockServers creates and starts all mock servers.
func NewOptimizationMockServers() *OptimizationMockServers {
	return &OptimizationMockServers{
		LlamaIndex:    NewMockLlamaIndexServer(),
		LangChain:     NewMockLangChainServer(),
		SGLang:        NewMockSGLangServer(),
		Guidance:      NewMockGuidanceServer(),
		LMQL:          NewMockLMQLServer(),
		SemanticCache: NewMockSemanticCacheServer(),
	}
}

// CloseAll shuts down all mock servers.
func (s *OptimizationMockServers) CloseAll() {
	if s.LlamaIndex != nil {
		s.LlamaIndex.Close()
	}
	if s.LangChain != nil {
		s.LangChain.Close()
	}
	if s.SGLang != nil {
		s.SGLang.Close()
	}
	if s.Guidance != nil {
		s.Guidance.Close()
	}
	if s.LMQL != nil {
		s.LMQL.Close()
	}
	if s.SemanticCache != nil {
		s.SemanticCache.Close()
	}
}

// ResetAll clears all counters and state in all mock servers.
func (s *OptimizationMockServers) ResetAll() {
	if s.LlamaIndex != nil {
		s.LlamaIndex.Reset()
	}
	if s.LangChain != nil {
		s.LangChain.Reset()
	}
	if s.SGLang != nil {
		s.SGLang.Reset()
	}
	if s.Guidance != nil {
		s.Guidance.Reset()
	}
	if s.LMQL != nil {
		s.LMQL.Reset()
	}
	if s.SemanticCache != nil {
		s.SemanticCache.Reset()
	}
}

// SetAllFailing sets all servers to fail mode.
func (s *OptimizationMockServers) SetAllFailing(fail bool) {
	if s.LlamaIndex != nil {
		s.LlamaIndex.ShouldFail = fail
	}
	if s.LangChain != nil {
		s.LangChain.ShouldFail = fail
	}
	if s.SGLang != nil {
		s.SGLang.ShouldFail = fail
	}
	if s.Guidance != nil {
		s.Guidance.ShouldFail = fail
	}
	if s.LMQL != nil {
		s.LMQL.ShouldFail = fail
	}
	if s.SemanticCache != nil {
		s.SemanticCache.ShouldFail = fail
	}
}

// SetAllDelay sets response delay for all servers.
func (s *OptimizationMockServers) SetAllDelay(delay time.Duration) {
	if s.LlamaIndex != nil {
		s.LlamaIndex.ResponseDelay = delay
	}
	if s.LangChain != nil {
		s.LangChain.ResponseDelay = delay
	}
	if s.SGLang != nil {
		s.SGLang.ResponseDelay = delay
	}
	if s.Guidance != nil {
		s.Guidance.ResponseDelay = delay
	}
	if s.LMQL != nil {
		s.LMQL.ResponseDelay = delay
	}
	if s.SemanticCache != nil {
		s.SemanticCache.ResponseDelay = delay
	}
}
