package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/graphql/resolvers"
	"dev.helix.agent/internal/toon"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestDefaultGraphQLHandlerConfig(t *testing.T) {
	config := DefaultGraphQLHandlerConfig()

	assert.True(t, config.EnableTOON)
	assert.Equal(t, toon.CompressionStandard, config.TOONCompression)
}

func TestNewGraphQLHandler(t *testing.T) {
	logger := logrus.New()
	config := DefaultGraphQLHandlerConfig()

	handler, err := NewGraphQLHandler(logger, config)

	require.NoError(t, err)
	require.NotNil(t, handler)
	assert.True(t, handler.IsInitialized())
}

func TestNewGraphQLHandler_NilConfig(t *testing.T) {
	handler, err := NewGraphQLHandler(nil, nil)

	require.NoError(t, err)
	require.NotNil(t, handler)
	assert.True(t, handler.IsInitialized())
}

func TestNewGraphQLHandler_DisableTOON(t *testing.T) {
	config := &GraphQLHandlerConfig{
		EnableTOON: false,
	}

	handler, err := NewGraphQLHandler(nil, config)

	require.NoError(t, err)
	require.NotNil(t, handler)
	stats := handler.GetStats()
	assert.False(t, stats.TOONEnabled)
}

func TestGraphQLHandler_Handle_ValidQuery(t *testing.T) {
	handler, err := NewGraphQLHandler(nil, nil)
	require.NoError(t, err)

	// Set up resolver context
	ctx := resolvers.NewResolverContext(nil)
	handler.SetResolverContext(ctx)
	defer resolvers.SetGlobalContext(nil)

	// Create router
	router := gin.New()
	router.POST("/graphql", handler.Handle)

	// Create request
	reqBody := GraphQLRequest{
		Query: `{ providers { id name } }`,
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GraphQLResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Empty(t, resp.Errors)
}

func TestGraphQLHandler_Handle_InvalidJSON(t *testing.T) {
	handler, err := NewGraphQLHandler(nil, nil)
	require.NoError(t, err)

	router := gin.New()
	router.POST("/graphql", handler.Handle)

	req, _ := http.NewRequest("POST", "/graphql", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGraphQLHandler_Handle_EmptyQuery(t *testing.T) {
	handler, err := NewGraphQLHandler(nil, nil)
	require.NoError(t, err)

	router := gin.New()
	router.POST("/graphql", handler.Handle)

	reqBody := GraphQLRequest{
		Query: "",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp GraphQLResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Errors)
	assert.Contains(t, resp.Errors[0].Message, "Query is required")
}

func TestGraphQLHandler_Handle_InvalidQuery(t *testing.T) {
	handler, err := NewGraphQLHandler(nil, nil)
	require.NoError(t, err)

	// Set up resolver context
	ctx := resolvers.NewResolverContext(nil)
	handler.SetResolverContext(ctx)
	defer resolvers.SetGlobalContext(nil)

	router := gin.New()
	router.POST("/graphql", handler.Handle)

	reqBody := GraphQLRequest{
		Query: `{ nonExistentField }`,
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code) // GraphQL returns 200 with errors

	var resp GraphQLResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Errors)
}

func TestGraphQLHandler_Handle_WithVariables(t *testing.T) {
	handler, err := NewGraphQLHandler(nil, nil)
	require.NoError(t, err)

	ctx := resolvers.NewResolverContext(nil)
	handler.SetResolverContext(ctx)
	defer resolvers.SetGlobalContext(nil)

	router := gin.New()
	router.POST("/graphql", handler.Handle)

	reqBody := GraphQLRequest{
		Query: `query GetProvider($id: ID!) { provider(id: $id) { id name } }`,
		Variables: map[string]interface{}{
			"id": "test-provider",
		},
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGraphQLHandler_Handle_TOON_Request(t *testing.T) {
	config := DefaultGraphQLHandlerConfig()
	handler, err := NewGraphQLHandler(nil, config)
	require.NoError(t, err)

	ctx := resolvers.NewResolverContext(nil)
	handler.SetResolverContext(ctx)
	defer resolvers.SetGlobalContext(nil)

	router := gin.New()
	router.POST("/graphql", handler.Handle)

	// Create TOON-encoded request
	reqBody := GraphQLRequest{
		Query: `{ providers { id name } }`,
	}
	encoder := toon.NewEncoder(nil)
	body, _ := encoder.Encode(reqBody)

	req, _ := http.NewRequest("POST", "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/toon+json")
	req.Header.Set("Accept", "application/toon+json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/toon+json", w.Header().Get("Content-Type"))
}

func TestGraphQLHandler_HandleIntrospection(t *testing.T) {
	handler, err := NewGraphQLHandler(nil, nil)
	require.NoError(t, err)

	router := gin.New()
	router.GET("/graphql/introspection", handler.HandleIntrospection)

	req, _ := http.NewRequest("GET", "/graphql/introspection", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp GraphQLResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotNil(t, resp.Data)
}

func TestGraphQLHandler_HandlePlayground(t *testing.T) {
	handler, err := NewGraphQLHandler(nil, nil)
	require.NoError(t, err)

	router := gin.New()
	router.GET("/graphql", handler.HandlePlayground)

	req, _ := http.NewRequest("GET", "/graphql", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "GraphQL Playground")
}

func TestGraphQLHandler_RegisterRoutes(t *testing.T) {
	handler, err := NewGraphQLHandler(nil, nil)
	require.NoError(t, err)

	router := gin.New()
	handler.RegisterRoutes(router)

	// Test POST /v1/graphql
	reqBody := GraphQLRequest{Query: `{ providers { id } }`}
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/v1/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test GET /v1/graphql (playground)
	req, _ = http.NewRequest("GET", "/v1/graphql", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test GET /v1/graphql/introspection
	req, _ = http.NewRequest("GET", "/v1/graphql/introspection", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGraphQLHandler_RegisterRoutesOnGroup(t *testing.T) {
	handler, err := NewGraphQLHandler(nil, nil)
	require.NoError(t, err)

	router := gin.New()
	group := router.Group("/api")
	handler.RegisterRoutesOnGroup(group)

	// Test POST /api/graphql
	reqBody := GraphQLRequest{Query: `{ providers { id } }`}
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGraphQLHandler_HandleBatch(t *testing.T) {
	handler, err := NewGraphQLHandler(nil, nil)
	require.NoError(t, err)

	ctx := resolvers.NewResolverContext(nil)
	handler.SetResolverContext(ctx)
	defer resolvers.SetGlobalContext(nil)

	router := gin.New()
	router.POST("/graphql/batch", handler.HandleBatch)

	batch := GraphQLBatchRequest{
		{Query: `{ providers { id } }`},
		{Query: `{ verificationResults { total_providers } }`},
	}
	body, _ := json.Marshal(batch)

	req, _ := http.NewRequest("POST", "/graphql/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var responses []GraphQLResponse
	err = json.Unmarshal(w.Body.Bytes(), &responses)
	require.NoError(t, err)
	assert.Len(t, responses, 2)
}

func TestGraphQLHandler_HandleBatch_EmptyBatch(t *testing.T) {
	handler, err := NewGraphQLHandler(nil, nil)
	require.NoError(t, err)

	router := gin.New()
	router.POST("/graphql/batch", handler.HandleBatch)

	body, _ := json.Marshal(GraphQLBatchRequest{})

	req, _ := http.NewRequest("POST", "/graphql/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGraphQLHandler_HandleBatch_InvalidJSON(t *testing.T) {
	handler, err := NewGraphQLHandler(nil, nil)
	require.NoError(t, err)

	router := gin.New()
	router.POST("/graphql/batch", handler.HandleBatch)

	req, _ := http.NewRequest("POST", "/graphql/batch", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGraphQLHandler_GetStats(t *testing.T) {
	config := &GraphQLHandlerConfig{
		EnableTOON: true,
	}
	handler, err := NewGraphQLHandler(nil, config)
	require.NoError(t, err)

	stats := handler.GetStats()

	assert.True(t, stats.Initialized)
	assert.True(t, stats.TOONEnabled)
}

func TestGraphQLHandler_SetResolverContext(t *testing.T) {
	handler, err := NewGraphQLHandler(nil, nil)
	require.NoError(t, err)

	ctx := resolvers.NewResolverContext(nil)
	handler.SetResolverContext(ctx)

	// Verify context is set globally
	assert.Equal(t, ctx, resolvers.GetGlobalContext())

	// Clean up
	resolvers.SetGlobalContext(nil)
}

func TestGraphQLRequest_Fields(t *testing.T) {
	req := GraphQLRequest{
		Query:         `{ providers { id } }`,
		OperationName: "GetProviders",
		Variables: map[string]interface{}{
			"id": "test",
		},
	}

	assert.Equal(t, `{ providers { id } }`, req.Query)
	assert.Equal(t, "GetProviders", req.OperationName)
	assert.Equal(t, "test", req.Variables["id"])
}

func TestGraphQLResponse_Fields(t *testing.T) {
	resp := GraphQLResponse{
		Data: map[string]interface{}{
			"providers": []interface{}{},
		},
		Errors: []GraphQLError{
			{Message: "test error"},
		},
	}

	assert.NotNil(t, resp.Data)
	assert.Len(t, resp.Errors, 1)
	assert.Equal(t, "test error", resp.Errors[0].Message)
}

func TestGraphQLError_Fields(t *testing.T) {
	err := GraphQLError{
		Message: "test error",
		Locations: []Location{
			{Line: 1, Column: 5},
		},
		Path: []interface{}{"providers", 0, "id"},
	}

	assert.Equal(t, "test error", err.Message)
	assert.Len(t, err.Locations, 1)
	assert.Equal(t, 1, err.Locations[0].Line)
	assert.Equal(t, 5, err.Locations[0].Column)
	assert.Len(t, err.Path, 3)
}

func TestLocation_Fields(t *testing.T) {
	loc := Location{
		Line:   10,
		Column: 20,
	}

	assert.Equal(t, 10, loc.Line)
	assert.Equal(t, 20, loc.Column)
}

func TestGraphQLHandlerConfig_Fields(t *testing.T) {
	config := GraphQLHandlerConfig{
		EnableTOON:      true,
		TOONCompression: toon.CompressionAggressive,
	}

	assert.True(t, config.EnableTOON)
	assert.Equal(t, toon.CompressionAggressive, config.TOONCompression)
}
