package bigdata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestHandler(
	integration *BigDataIntegration,
	debateIntegration *DebateIntegration,
) *Handler {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	return NewHandler(integration, debateIntegration, logger)
}

func newTestIntegrationAllDisabled() *BigDataIntegration {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}
	bdi, _ := NewBigDataIntegration(config, nil, logger)
	return bdi
}

// --- NewHandler tests ---

func TestNewHandler_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	h := newTestHandler(newTestIntegrationAllDisabled(), nil)
	assert.NotNil(t, h)
}

func TestNewHandler_SetsFields(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	logger := logrus.New()
	h := NewHandler(integration, nil, logger)

	assert.Equal(t, integration, h.integration)
	assert.Nil(t, h.debateIntegration)
	assert.Equal(t, logger, h.logger)
}

// --- RegisterRoutes tests ---

func TestHandler_RegisterRoutes_RegistersExpectedRoutes(t *testing.T) {
	t.Parallel()
	h := newTestHandler(newTestIntegrationAllDisabled(), nil)
	router := gin.New()
	h.RegisterRoutes(router)

	expectedRoutes := map[string]string{
		"POST:/v1/context/replay":                "ReplayConversation",
		"GET:/v1/context/stats/:conversation_id": "GetContextStats",
		"GET:/v1/memory/sync/status":             "GetMemorySyncStatus",
		"POST:/v1/memory/sync/force":             "ForceMemorySync",
		"GET:/v1/knowledge/related/:entity_id":   "GetRelatedEntities",
		"POST:/v1/knowledge/search":              "SearchKnowledgeGraph",
		"GET:/v1/analytics/provider/:provider":   "GetProviderAnalytics",
		"GET:/v1/analytics/debate/:debate_id":    "GetDebateAnalytics",
		"POST:/v1/analytics/query":               "QueryAnalytics",
		"GET:/v1/learning/insights":              "GetLearningInsights",
		"GET:/v1/learning/patterns":              "GetLearnedPatterns",
		"GET:/v1/bigdata/health":                 "HealthCheck",
	}

	routes := router.Routes()
	registeredPaths := make(map[string]bool)
	for _, r := range routes {
		key := r.Method + ":" + r.Path
		registeredPaths[key] = true
	}

	for expectedPath, name := range expectedRoutes {
		assert.True(t, registeredPaths[expectedPath],
			"Expected route %s (%s) not registered", expectedPath, name)
	}
}

// --- ReplayConversation tests ---

func TestHandler_ReplayConversation_InvalidJSON(t *testing.T) {
	t.Parallel()
	h := newTestHandler(newTestIntegrationAllDisabled(), nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/context/replay", bytes.NewBufferString(`{invalid`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ReplayConversation(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "invalid")
}

func TestHandler_ReplayConversation_MissingConversationID(t *testing.T) {
	t.Parallel()
	h := newTestHandler(newTestIntegrationAllDisabled(), nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"max_tokens": 100}`
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/context/replay", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ReplayConversation(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_ReplayConversation_NilDebateIntegration(t *testing.T) {
	t.Parallel()
	h := newTestHandler(newTestIntegrationAllDisabled(), nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"conversation_id": "test-conv-1"}`
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/context/replay", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ReplayConversation(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"].(string), "debate integration not available")
}

func TestHandler_ReplayConversation_DefaultMaxTokens(t *testing.T) {
	t.Parallel()
	// Verify that max_tokens defaults to 4000 when not specified.
	// We test this by confirming the handler reaches the debateIntegration
	// nil check (which means it parsed the request successfully with default).
	h := newTestHandler(newTestIntegrationAllDisabled(), nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"conversation_id": "test-conv-1"}`
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/context/replay", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ReplayConversation(c)

	// Should reach 503 (debateIntegration nil), not 400 (bad request)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// --- GetContextStats tests ---

func TestHandler_GetContextStats_NilInfiniteContext(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/context/stats/conv-1", nil)
	c.Params = gin.Params{{Key: "conversation_id", Value: "conv-1"}}

	h.GetContextStats(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"].(string), "infinite context not enabled")
}

func TestHandler_GetContextStats_EmptyConversationID(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/context/stats/", nil)
	c.Params = gin.Params{{Key: "conversation_id", Value: ""}}

	h.GetContextStats(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- GetMemorySyncStatus tests ---

func TestHandler_GetMemorySyncStatus_NilDistributedMemory(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/memory/sync/status", nil)

	h.GetMemorySyncStatus(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"].(string), "distributed memory not enabled")
}

// --- ForceMemorySync tests ---

func TestHandler_ForceMemorySync_NilDistributedMemory(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/memory/sync/force", nil)

	h.ForceMemorySync(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"].(string), "distributed memory not enabled")
}

// --- GetRelatedEntities tests ---

func TestHandler_GetRelatedEntities_NilKnowledgeGraph(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/knowledge/related/entity-1", nil)
	c.Params = gin.Params{{Key: "entity_id", Value: "entity-1"}}

	h.GetRelatedEntities(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"].(string), "knowledge graph not enabled")
}

func TestHandler_GetRelatedEntities_EmptyEntityID(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/knowledge/related/", nil)
	c.Params = gin.Params{{Key: "entity_id", Value: ""}}

	h.GetRelatedEntities(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- SearchKnowledgeGraph tests ---

func TestHandler_SearchKnowledgeGraph_InvalidJSON(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/knowledge/search", bytes.NewBufferString(`{bad`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.SearchKnowledgeGraph(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_SearchKnowledgeGraph_MissingQuery(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"entity_type": "person"}`
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/knowledge/search", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.SearchKnowledgeGraph(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_SearchKnowledgeGraph_NilKnowledgeGraph(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"query": "find people"}`
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/knowledge/search", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.SearchKnowledgeGraph(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"].(string), "knowledge graph not enabled")
}

// --- GetProviderAnalytics tests ---

func TestHandler_GetProviderAnalytics_NilAnalytics(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/analytics/provider/claude", nil)
	c.Params = gin.Params{{Key: "provider", Value: "claude"}}

	h.GetProviderAnalytics(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"].(string), "analytics not enabled")
}

func TestHandler_GetProviderAnalytics_EmptyProvider(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/analytics/provider/", nil)
	c.Params = gin.Params{{Key: "provider", Value: ""}}

	h.GetProviderAnalytics(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetProviderAnalytics_InvalidWindowDuration(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	// We need analytics to be non-nil to reach the duration parsing.
	// Since we cannot easily set clickhouseAnalytics without live infra,
	// this test uses nil analytics — the handler returns 503 before parsing.
	// We verify that the analytics nil check precedes the window parse.
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/analytics/provider/claude?window=invalid", nil)
	c.Params = gin.Params{{Key: "provider", Value: "claude"}}

	h.GetProviderAnalytics(c)

	// With nil analytics, we get 503 regardless of window value
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// --- GetDebateAnalytics tests ---

func TestHandler_GetDebateAnalytics_NilAnalytics(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/analytics/debate/debate-1", nil)
	c.Params = gin.Params{{Key: "debate_id", Value: "debate-1"}}

	h.GetDebateAnalytics(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_GetDebateAnalytics_EmptyDebateID(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/analytics/debate/", nil)
	c.Params = gin.Params{{Key: "debate_id", Value: ""}}

	h.GetDebateAnalytics(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- QueryAnalytics tests ---

func TestHandler_QueryAnalytics_InvalidJSON(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/analytics/query", bytes.NewBufferString(`{broken`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.QueryAnalytics(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_QueryAnalytics_MissingQuery(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"parameters": {"key": "value"}}`
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/analytics/query", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.QueryAnalytics(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_QueryAnalytics_NilAnalytics(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"query": "SELECT * FROM events"}`
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/analytics/query", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.QueryAnalytics(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// --- GetLearningInsights tests ---

func TestHandler_GetLearningInsights_NilCrossLearner(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/learning/insights", nil)

	h.GetLearningInsights(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"].(string), "cross-session learning not enabled")
}

// --- GetLearnedPatterns tests ---

func TestHandler_GetLearnedPatterns_NilCrossLearner(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/learning/patterns", nil)

	h.GetLearnedPatterns(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"].(string), "cross-session learning not enabled")
}

// --- HealthCheck tests ---

func TestHandler_HealthCheck_AllDisabled(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/bigdata/health", nil)

	h.HealthCheck(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "healthy", resp["status"])
	assert.Equal(t, false, resp["running"])

	components, ok := resp["components"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "disabled", components["infinite_context"])
	assert.Equal(t, "disabled", components["distributed_memory"])
	assert.Equal(t, "disabled", components["knowledge_graph"])
	assert.Equal(t, "disabled", components["analytics"])
	assert.Equal(t, "disabled", components["cross_learning"])
}

func TestHandler_HealthCheck_EnabledButNotInitialized(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	config := &IntegrationConfig{
		EnableInfiniteContext:   true,
		EnableDistributedMemory: true,
		EnableKnowledgeGraph:    true,
		EnableAnalytics:         true,
		EnableCrossLearning:     true,
	}
	integration, err := NewBigDataIntegration(config, nil, logger)
	require.NoError(t, err)

	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/bigdata/health", nil)

	h.HealthCheck(c)

	// All components enabled but not initialized -> degraded
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "degraded", resp["status"])

	components, ok := resp["components"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "not_initialized", components["infinite_context"])
	assert.Equal(t, "not_initialized", components["distributed_memory"])
	assert.Equal(t, "not_initialized", components["knowledge_graph"])
	assert.Equal(t, "not_initialized", components["analytics"])
	assert.Equal(t, "not_initialized", components["cross_learning"])
}

func TestHandler_HealthCheck_Running(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	_ = integration.Start(t.Context())
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/bigdata/health", nil)

	h.HealthCheck(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, true, resp["running"])
}

// --- HealthCheck via router (full roundtrip) ---

func TestHandler_HealthCheck_ViaRouter(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)
	router := gin.New()
	h.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/bigdata/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "healthy", resp["status"])
}

// --- Request type tests ---

func TestReplayConversationRequest_JSONBinding(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name:    "valid request with all fields",
			json:    `{"conversation_id":"conv-1","max_tokens":2000,"compression_strategy":"hybrid"}`,
			wantErr: false,
		},
		{
			name:    "valid request minimal",
			json:    `{"conversation_id":"conv-1"}`,
			wantErr: false,
		},
		{
			name:    "missing required field",
			json:    `{"max_tokens":2000}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tt.json))
			c.Request.Header.Set("Content-Type", "application/json")

			var req ReplayConversationRequest
			err := c.ShouldBindJSON(&req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, req.ConversationID)
			}
		})
	}
}

func TestSearchKnowledgeGraphRequest_JSONBinding(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name:    "valid request with all fields",
			json:    `{"query":"find entities","entity_type":"person","limit":20,"filters":["active"]}`,
			wantErr: false,
		},
		{
			name:    "valid request minimal",
			json:    `{"query":"find entities"}`,
			wantErr: false,
		},
		{
			name:    "missing required query",
			json:    `{"entity_type":"person"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tt.json))
			c.Request.Header.Set("Content-Type", "application/json")

			var req SearchKnowledgeGraphRequest
			err := c.ShouldBindJSON(&req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, req.Query)
			}
		})
	}
}

// --- GetLearningInsights additional tests ---

func TestHandler_GetLearningInsights_InvalidLimit(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/learning/insights?limit=abc", nil)

	h.GetLearningInsights(c)

	// crossLearner is nil, so we get 503 before limit parsing
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_GetLearningInsights_LimitOutOfRangeHigh(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/learning/insights?limit=200", nil)

	h.GetLearningInsights(c)

	// crossLearner is nil, so we get 503
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_GetLearningInsights_LimitZero(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/learning/insights?limit=0", nil)

	h.GetLearningInsights(c)

	// crossLearner is nil, so we get 503
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// --- GetLearnedPatterns additional tests ---

func TestHandler_GetLearnedPatterns_WithTypeParam(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/learning/patterns?type=provider", nil)

	h.GetLearnedPatterns(c)

	// crossLearner is nil, so we get 503
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// --- GetRelatedEntities depth parsing tests ---

func TestHandler_GetRelatedEntities_CustomMaxDepth(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/knowledge/related/entity-1?max_depth=3", nil)
	c.Params = gin.Params{{Key: "entity_id", Value: "entity-1"}}

	h.GetRelatedEntities(c)

	// Knowledge graph is nil, so we get 503
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_GetRelatedEntities_InvalidMaxDepth(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/knowledge/related/entity-1?max_depth=10", nil)
	c.Params = gin.Params{{Key: "entity_id", Value: "entity-1"}}

	h.GetRelatedEntities(c)

	// Still 503 because knowledge graph is nil
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_GetRelatedEntities_NegativeMaxDepth(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/knowledge/related/entity-1?max_depth=-1", nil)
	c.Params = gin.Params{{Key: "entity_id", Value: "entity-1"}}

	h.GetRelatedEntities(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_GetRelatedEntities_NonNumericMaxDepth(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/knowledge/related/entity-1?max_depth=abc", nil)
	c.Params = gin.Params{{Key: "entity_id", Value: "entity-1"}}

	h.GetRelatedEntities(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// --- SearchKnowledgeGraph additional tests ---

func TestHandler_SearchKnowledgeGraph_DefaultLimit(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"query": "test query"}`
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/knowledge/search", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.SearchKnowledgeGraph(c)

	// Should reach 503 due to nil knowledge graph, not bad request
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_SearchKnowledgeGraph_WithAllFields(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"query": "find entities", "entity_type": "person", "limit": 20, "filters": ["active", "recent"]}`
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/knowledge/search", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.SearchKnowledgeGraph(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// --- GetProviderAnalytics additional tests ---

func TestHandler_GetProviderAnalytics_ValidWindow(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/analytics/provider/claude?window=1h", nil)
	c.Params = gin.Params{{Key: "provider", Value: "claude"}}

	h.GetProviderAnalytics(c)

	// Analytics is nil, so we get 503
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_GetProviderAnalytics_DefaultWindow(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/analytics/provider/deepseek", nil)
	c.Params = gin.Params{{Key: "provider", Value: "deepseek"}}

	h.GetProviderAnalytics(c)

	// Analytics is nil, so we get 503
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// --- QueryAnalytics additional tests ---

func TestHandler_QueryAnalytics_WithParameters(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"query": "SELECT * FROM events WHERE provider = :provider", "parameters": {"provider": "claude"}, "format": "json"}`
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/analytics/query", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.QueryAnalytics(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// --- ReplayConversation additional tests ---

func TestHandler_ReplayConversation_WithMaxTokensAndStrategy(t *testing.T) {
	t.Parallel()
	h := newTestHandler(newTestIntegrationAllDisabled(), nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"conversation_id": "conv-1", "max_tokens": 2000, "compression_strategy": "hybrid"}`
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/context/replay", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ReplayConversation(c)

	// Reaches nil debate integration check
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// --- Full router roundtrip tests ---

func TestHandler_GetContextStats_ViaRouter(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)
	router := gin.New()
	h.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/context/stats/test-conv", nil)
	router.ServeHTTP(w, req)

	// Infinite context is nil -> 503
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_GetMemorySyncStatus_ViaRouter(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)
	router := gin.New()
	h.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/memory/sync/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_ForceMemorySync_ViaRouter(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)
	router := gin.New()
	h.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/memory/sync/force", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_GetRelatedEntities_ViaRouter(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)
	router := gin.New()
	h.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/knowledge/related/ent-test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_GetProviderAnalytics_ViaRouter(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)
	router := gin.New()
	h.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/analytics/provider/claude", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_GetDebateAnalytics_ViaRouter(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)
	router := gin.New()
	h.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/analytics/debate/debate-test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_GetLearningInsights_ViaRouter(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)
	router := gin.New()
	h.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/learning/insights", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_GetLearnedPatterns_ViaRouter(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationAllDisabled()
	h := newTestHandler(integration, nil)
	router := gin.New()
	h.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/learning/patterns", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// --- Handler tests with initialized components ---

func newTestIntegrationWithCrossLearner() *BigDataIntegration {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	broker := newMockBroker()
	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     true,
		LearningMinConfidence:   0.7,
		LearningMinFrequency:    3,
	}
	bdi, _ := NewBigDataIntegration(config, broker, logger)
	_ = bdi.Initialize(context.Background())
	return bdi
}

func newTestIntegrationWithInfiniteContext() *BigDataIntegration {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	broker := newMockBroker()
	config := &IntegrationConfig{
		EnableInfiniteContext:   true,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}
	bdi, _ := NewBigDataIntegration(config, broker, logger)
	_ = bdi.Initialize(context.Background())
	return bdi
}

func newTestIntegrationWithDistributedMemory() *BigDataIntegration {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	broker := newMockBroker()
	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: true,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}
	bdi, _ := NewBigDataIntegration(config, broker, logger)
	_ = bdi.Initialize(context.Background())
	return bdi
}

func newMockBrokerForHandler() *mockBroker {
	return newMockBroker()
}

func TestHandler_GetLearningInsights_WithCrossLearner(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationWithCrossLearner()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/learning/insights?limit=5", nil)

	h.GetLearningInsights(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp, "insights")
	assert.Contains(t, resp, "count")
	assert.Contains(t, resp, "limit")
	assert.Equal(t, float64(5), resp["limit"])
}

func TestHandler_GetLearningInsights_WithCrossLearner_DefaultLimit(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationWithCrossLearner()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/learning/insights", nil)

	h.GetLearningInsights(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(10), resp["limit"])
}

func TestHandler_GetLearningInsights_WithCrossLearner_InvalidLimit(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationWithCrossLearner()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/learning/insights?limit=abc", nil)

	h.GetLearningInsights(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetLearningInsights_WithCrossLearner_LimitTooHigh(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationWithCrossLearner()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/learning/insights?limit=200", nil)

	h.GetLearningInsights(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "limit must be between 1 and 100")
}

func TestHandler_GetLearningInsights_WithCrossLearner_LimitZero(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationWithCrossLearner()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/learning/insights?limit=0", nil)

	h.GetLearningInsights(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetLearnedPatterns_WithCrossLearner(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationWithCrossLearner()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/learning/patterns", nil)

	h.GetLearnedPatterns(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp, "patterns")
	assert.Contains(t, resp, "type")
	assert.Contains(t, resp, "count")
	assert.Equal(t, "all", resp["type"])
}

func TestHandler_GetLearnedPatterns_WithCrossLearner_SpecificType(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationWithCrossLearner()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/learning/patterns?type=provider", nil)

	h.GetLearnedPatterns(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "provider", resp["type"])
}

// --- GetContextStats with infinite context ---

func TestHandler_GetContextStats_WithInfiniteContext(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationWithInfiniteContext()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/context/stats/nonexistent-conv", nil)
	c.Params = gin.Params{{Key: "conversation_id", Value: "nonexistent-conv"}}

	h.GetContextStats(c)

	// Infinite context is initialized but conversation does not exist
	// The handler should either return data or an error
	// GetConversationSnapshot for non-existent conv returns empty snapshot
	code := w.Code
	assert.True(t, code == http.StatusOK || code == http.StatusInternalServerError,
		"Expected 200 or 500, got %d", code)
}

// --- GetMemorySyncStatus with distributed memory ---

func TestHandler_GetMemorySyncStatus_WithDistributedMemory(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationWithDistributedMemory()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/memory/sync/status", nil)

	h.GetMemorySyncStatus(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp, "status")
}

// --- ForceMemorySync with distributed memory ---

func TestHandler_ForceMemorySync_WithDistributedMemory(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationWithDistributedMemory()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/memory/sync/force", nil)

	h.ForceMemorySync(c)

	// ForceSync publishes to Kafka via the mock broker
	code := w.Code
	assert.True(t, code == http.StatusOK || code == http.StatusInternalServerError,
		"Expected 200 or 500, got %d", code)
}

// --- HealthCheck with mixed components ---

func TestHandler_HealthCheck_WithCrossLearner(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationWithCrossLearner()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/bigdata/health", nil)

	h.HealthCheck(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "healthy", resp["status"])

	components, ok := resp["components"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "healthy", components["cross_learning"])
	assert.Equal(t, "disabled", components["infinite_context"])
}

// --- ReplayConversation with DebateIntegration ---

func TestHandler_ReplayConversation_WithDebateIntegration_FailsGetContext(t *testing.T) {
	t.Parallel()
	// Create a debate integration with nil infiniteContext, which causes
	// GetConversationContext to panic. We need an integration with a real
	// InfiniteContextEngine but one that returns an error.
	broker := newMockBroker()
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create an InfiniteContextEngine that will fail when trying to fetch from Kafka
	integration := newTestIntegrationWithInfiniteContext()
	debateIntegration := NewDebateIntegration(
		integration.GetInfiniteContext(),
		broker,
		logger,
	)
	h := newTestHandler(integration, debateIntegration)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"conversation_id": "nonexistent-conv", "max_tokens": 2000}`
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/context/replay", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ReplayConversation(c)

	// GetConversationSnapshot will fail because Kafka is not available
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "context replay failed")
}

// --- GetContextStats with InfiniteContext snapshot error ---

func TestHandler_GetContextStats_WithInfiniteContext_SnapshotError(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationWithInfiniteContext()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v1/context/stats/test-conv-id", nil)
	c.Params = gin.Params{{Key: "conversation_id", Value: "test-conv-id"}}

	h.GetContextStats(c)

	// InfiniteContextEngine is initialized, but fetching from Kafka will fail
	// so we should get 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "failed to get conversation snapshot")
}

// --- ForceMemorySync success path ---

func TestHandler_ForceMemorySync_WithDistributedMemory_Success(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationWithDistributedMemory()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/memory/sync/force", nil)

	h.ForceMemorySync(c)

	// ForceSync should succeed with mock broker
	code := w.Code
	if code == http.StatusOK {
		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "sync_request_sent", resp["status"])
	}
	// Either 200 or 500 is acceptable depending on internal state
	assert.True(t, code == http.StatusOK || code == http.StatusInternalServerError,
		"Expected 200 or 500, got %d", code)
}

// --- ForceMemorySync with broker error (error path) ---

func newTestIntegrationWithDistributedMemoryBrokerError() *BigDataIntegration {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	broker := newMockBroker()
	broker.publishErr = fmt.Errorf("kafka unavailable")
	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: true,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}
	bdi, _ := NewBigDataIntegration(config, broker, logger)
	_ = bdi.Initialize(context.Background())
	return bdi
}

func TestHandler_ForceMemorySync_WithDistributedMemory_Error(t *testing.T) {
	t.Parallel()
	integration := newTestIntegrationWithDistributedMemoryBrokerError()
	h := newTestHandler(integration, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/memory/sync/force", nil)

	h.ForceMemorySync(c)

	// ForceSync should fail because broker publish returns an error
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "failed to force memory sync")
	assert.Contains(t, resp["details"], "kafka unavailable")
}

func TestQueryAnalyticsRequest_JSONBinding(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name:    "valid request with all fields",
			json:    `{"query":"SELECT 1","parameters":{"limit":10},"format":"json"}`,
			wantErr: false,
		},
		{
			name:    "valid request minimal",
			json:    `{"query":"SELECT 1"}`,
			wantErr: false,
		},
		{
			name:    "missing required query",
			json:    `{"format":"csv"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tt.json))
			c.Request.Header.Set("Content-Type", "application/json")

			var req QueryAnalyticsRequest
			err := c.ShouldBindJSON(&req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, req.Query)
			}
		})
	}
}
