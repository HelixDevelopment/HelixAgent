package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewACPHandler tests handler creation
func TestNewACPHandler(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.agents)
	assert.Equal(t, logger, handler.logger)
	assert.Len(t, handler.agents, 6, "Should have 6 built-in agents")
}

// TestACPHandler_HandleJSONRPC_Initialize tests the initialize method
func TestACPHandler_HandleJSONRPC_Initialize(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{}`),
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.Equal(t, float64(1), response.ID)
	assert.Nil(t, response.Error)

	result := response.Result.(map[string]interface{})
	assert.Equal(t, "2.0", result["protocolVersion"])
	assert.NotNil(t, result["serverInfo"])
	assert.NotNil(t, result["capabilities"])

	serverInfo := result["serverInfo"].(map[string]interface{})
	assert.Equal(t, "helixagent-acp", serverInfo["name"])
	assert.Equal(t, "1.0.0", serverInfo["version"])

	capabilities := result["capabilities"].(map[string]interface{})
	assert.NotNil(t, capabilities["agents"])
	assert.NotNil(t, capabilities["sessions"])
}

// TestACPHandler_HandleJSONRPC_AgentList tests the agent/list method
func TestACPHandler_HandleJSONRPC_AgentList(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "agent/list",
		Params:  json.RawMessage(`{}`),
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.Nil(t, response.Error)

	result := response.Result.(map[string]interface{})
	agents := result["agents"].([]interface{})
	assert.Equal(t, 6, len(agents), "Should return 6 agents")
	assert.Equal(t, float64(6), result["count"])

	// Test with filter
	msgWithFilter := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "agent/list",
		Params:  json.RawMessage(`{"filter": "review"}`),
	}

	reqBytes2, _ := json.Marshal(msgWithFilter)
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes2))
	c2.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c2)

	assert.Equal(t, http.StatusOK, w2.Code)

	var response2 JSONRPCMessage
	err = json.Unmarshal(w2.Body.Bytes(), &response2)
	require.NoError(t, err)

	result2 := response2.Result.(map[string]interface{})
	agents2 := result2["agents"].([]interface{})
	assert.GreaterOrEqual(t, len(agents2), 1, "Should find at least one agent with 'review' in name or description")
}

// TestACPHandler_HandleJSONRPC_AgentGet tests the agent/get method
func TestACPHandler_HandleJSONRPC_AgentGet(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	// Test existing agent
	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "agent/get",
		Params:  json.RawMessage(`{"agent_id": "code-reviewer"}`),
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.Nil(t, response.Error)

	agent := response.Result.(map[string]interface{})
	assert.Equal(t, "code-reviewer", agent["id"])
	assert.Equal(t, "Code Reviewer", agent["name"])
	assert.Equal(t, "active", agent["status"])

	// Test non-existent agent
	msg2 := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "agent/get",
		Params:  json.RawMessage(`{"agent_id": "non-existent"}`),
	}

	reqBytes2, _ := json.Marshal(msg2)
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes2))
	c2.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c2)

	assert.Equal(t, http.StatusOK, w2.Code)

	var response2 JSONRPCMessage
	err = json.Unmarshal(w2.Body.Bytes(), &response2)
	require.NoError(t, err)

	assert.NotNil(t, response2.Error)
	assert.Equal(t, -32602, response2.Error.Code)
	assert.Contains(t, response2.Error.Message, "Agent not found")
}

// TestACPHandler_HandleJSONRPC_AgentExecute tests the agent/execute method
func TestACPHandler_HandleJSONRPC_AgentExecute(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "agent/execute",
		Params:  json.RawMessage(`{"agent_id": "code-reviewer", "task": "Review this code"}`),
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.Nil(t, response.Error)

	result := response.Result.(map[string]interface{})
	assert.Equal(t, "completed", result["status"])
	assert.Equal(t, "code-reviewer", result["agent_id"])
	assert.NotNil(t, result["result"])
	assert.NotNil(t, result["metadata"])
	assert.NotNil(t, result["duration"])
	assert.NotNil(t, result["timestamp"])

	// Test with context
	msg2 := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "agent/execute",
		Params:  json.RawMessage(`{"agent_id": "bug-finder", "task": "Find bugs", "context": {"code": "func test() {}", "language": "go"}}`),
	}

	reqBytes2, _ := json.Marshal(msg2)
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes2))
	c2.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c2)

	assert.Equal(t, http.StatusOK, w2.Code)

	var response2 JSONRPCMessage
	err = json.Unmarshal(w2.Body.Bytes(), &response2)
	require.NoError(t, err)

	assert.Nil(t, response2.Error)
	result2 := response2.Result.(map[string]interface{})
	assert.Equal(t, "bug-finder", result2["agent_id"])

	// Test non-existent agent
	msg3 := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "agent/execute",
		Params:  json.RawMessage(`{"agent_id": "non-existent", "task": "test"}`),
	}

	reqBytes3, _ := json.Marshal(msg3)
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes3))
	c3.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c3)

	assert.Equal(t, http.StatusOK, w3.Code)

	var response3 JSONRPCMessage
	err = json.Unmarshal(w3.Body.Bytes(), &response3)
	require.NoError(t, err)

	assert.NotNil(t, response3.Error)
	assert.Equal(t, -32602, response3.Error.Code)
}

// TestACPHandler_HandleJSONRPC_SessionCreate tests the session/create method
func TestACPHandler_HandleJSONRPC_SessionCreate(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      9,
		Method:  "session/create",
		Params:  json.RawMessage(`{"agent_id": "code-reviewer"}`),
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.Nil(t, response.Error)

	result := response.Result.(map[string]interface{})
	assert.NotEmpty(t, result["session_id"])
	assert.Equal(t, "code-reviewer", result["agent_id"])
	assert.NotNil(t, result["created_at"])

	// Test with non-existent agent
	msg2 := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      10,
		Method:  "session/create",
		Params:  json.RawMessage(`{"agent_id": "non-existent"}`),
	}

	reqBytes2, _ := json.Marshal(msg2)
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes2))
	c2.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c2)

	assert.Equal(t, http.StatusOK, w2.Code)

	var response2 JSONRPCMessage
	err = json.Unmarshal(w2.Body.Bytes(), &response2)
	require.NoError(t, err)

	assert.NotNil(t, response2.Error)
	assert.Equal(t, -32602, response2.Error.Code)
}

// TestACPHandler_HandleJSONRPC_SessionUpdate tests the session/update method
func TestACPHandler_HandleJSONRPC_SessionUpdate(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	// First create a session
	createMsg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      11,
		Method:  "session/create",
		Params:  json.RawMessage(`{"agent_id": "code-reviewer"}`),
	}

	reqBytes, _ := json.Marshal(createMsg)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	var createResponse JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &createResponse)
	require.NoError(t, err)
	sessionID := createResponse.Result.(map[string]interface{})["session_id"].(string)

	// Update session with context
	updateMsg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      12,
		Method:  "session/update",
		Params:  json.RawMessage(`{"session_id": "` + sessionID + `", "context": {"user_id": "test-user"}}`),
	}

	reqBytes2, _ := json.Marshal(updateMsg)
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes2))
	c2.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c2)

	assert.Equal(t, http.StatusOK, w2.Code)

	var updateResponse JSONRPCMessage
	err = json.Unmarshal(w2.Body.Bytes(), &updateResponse)
	require.NoError(t, err)

	assert.Nil(t, updateResponse.Error)
	result := updateResponse.Result.(map[string]interface{})
	assert.Equal(t, sessionID, result["session_id"])
	assert.NotNil(t, result["updated_at"])
	assert.Equal(t, float64(0), result["history_len"]) // No message added

	// Update session with message
	updateMsg2 := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      13,
		Method:  "session/update",
		Params:  json.RawMessage(`{"session_id": "` + sessionID + `", "message": {"role": "user", "content": "Hello, agent!"}}`),
	}

	reqBytes3, _ := json.Marshal(updateMsg2)
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes3))
	c3.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c3)

	assert.Equal(t, http.StatusOK, w3.Code)

	var updateResponse2 JSONRPCMessage
	err = json.Unmarshal(w3.Body.Bytes(), &updateResponse2)
	require.NoError(t, err)

	assert.Nil(t, updateResponse2.Error)
	result2 := updateResponse2.Result.(map[string]interface{})
	assert.Equal(t, float64(1), result2["history_len"]) // One message added

	// Test with non-existent session
	updateMsg3 := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      14,
		Method:  "session/update",
		Params:  json.RawMessage(`{"session_id": "non-existent-session"}`),
	}

	reqBytes4, _ := json.Marshal(updateMsg3)
	w4 := httptest.NewRecorder()
	c4, _ := gin.CreateTestContext(w4)
	c4.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes4))
	c4.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c4)

	assert.Equal(t, http.StatusOK, w4.Code)

	var updateResponse3 JSONRPCMessage
	err = json.Unmarshal(w4.Body.Bytes(), &updateResponse3)
	require.NoError(t, err)

	assert.NotNil(t, updateResponse3.Error)
	assert.Equal(t, -32602, updateResponse3.Error.Code)
}

// TestACPHandler_HandleJSONRPC_SessionClose tests the session/close method
func TestACPHandler_HandleJSONRPC_SessionClose(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	// First create a session
	createMsg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      15,
		Method:  "session/create",
		Params:  json.RawMessage(`{"agent_id": "code-reviewer"}`),
	}

	reqBytes, _ := json.Marshal(createMsg)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	var createResponse JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &createResponse)
	require.NoError(t, err)
	sessionID := createResponse.Result.(map[string]interface{})["session_id"].(string)

	// Close the session
	closeMsg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      16,
		Method:  "session/close",
		Params:  json.RawMessage(`{"session_id": "` + sessionID + `"}`),
	}

	reqBytes2, _ := json.Marshal(closeMsg)
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes2))
	c2.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c2)

	assert.Equal(t, http.StatusOK, w2.Code)

	var closeResponse JSONRPCMessage
	err = json.Unmarshal(w2.Body.Bytes(), &closeResponse)
	require.NoError(t, err)

	assert.Nil(t, closeResponse.Error)
	result := closeResponse.Result.(map[string]interface{})
	assert.Equal(t, sessionID, result["session_id"])
	assert.Equal(t, true, result["closed"])

	// Try to close non-existent session
	closeMsg2 := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      17,
		Method:  "session/close",
		Params:  json.RawMessage(`{"session_id": "non-existent"}`),
	}

	reqBytes3, _ := json.Marshal(closeMsg2)
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes3))
	c3.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c3)

	assert.Equal(t, http.StatusOK, w3.Code)

	var closeResponse2 JSONRPCMessage
	err = json.Unmarshal(w3.Body.Bytes(), &closeResponse2)
	require.NoError(t, err)

	assert.NotNil(t, closeResponse2.Error)
	assert.Equal(t, -32602, closeResponse2.Error.Code)
}

// TestACPHandler_HandleJSONRPC_Health tests the health method
func TestACPHandler_HandleJSONRPC_Health(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      18,
		Method:  "health",
		Params:  json.RawMessage(`{}`),
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.Nil(t, response.Error)

	result := response.Result.(map[string]interface{})
	assert.Equal(t, "healthy", result["status"])
	assert.Equal(t, "acp", result["service"])
	assert.Equal(t, "1.0.0", result["version"])
	assert.Equal(t, float64(6), result["agent_count"])
	assert.Equal(t, float64(0), result["session_count"])
	assert.NotNil(t, result["timestamp"])
}

// TestACPHandler_HandleJSONRPC_MethodNotFound tests unknown method
func TestACPHandler_HandleJSONRPC_MethodNotFound(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      19,
		Method:  "unknown.method",
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.NotNil(t, response.Error)
	assert.Equal(t, -32601, response.Error.Code)
	assert.Contains(t, response.Error.Message, "Method not found")
}

// TestACPHandler_HandleJSONRPC_InvalidJSON tests invalid JSON parsing
func TestACPHandler_HandleJSONRPC_InvalidJSON(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer([]byte("invalid json")))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.NotNil(t, response.Error)
	assert.Equal(t, -32700, response.Error.Code)
	assert.Contains(t, response.Error.Message, "Parse error")
}

// TestACPHandler_HandleJSONRPC_InvalidParams tests invalid parameters
func TestACPHandler_HandleJSONRPC_InvalidParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      20,
		Method:  "agent/get",
		Params:  json.RawMessage(`123`),
	}

	reqBytes, _ := json.Marshal(msg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.NotNil(t, response.Error)
	assert.Equal(t, -32602, response.Error.Code)
	assert.Contains(t, response.Error.Message, "Invalid params")
}

// TestACPHandler_ConcurrentAccess tests concurrent access to sessions
func TestACPHandler_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	var wg sync.WaitGroup
	sessionIDs := make([]string, 10)
	errors := make([]error, 10)

	// Create 10 sessions concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			msg := JSONRPCMessage{
				JSONRPC: "2.0",
				ID:      idx,
				Method:  "session/create",
				Params:  json.RawMessage(`{"agent_id": "code-reviewer"}`),
			}

			reqBytes, _ := json.Marshal(msg)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.HandleJSONRPC(c)

			var response JSONRPCMessage
			err := json.Unmarshal(w.Body.Bytes(), &response)
			if err != nil {
				errors[idx] = err
				return
			}

			if response.Error != nil {
				errors[idx] = fmt.Errorf("JSON-RPC error: %v", response.Error)
				return
			}

			result := response.Result.(map[string]interface{})
			sessionIDs[idx] = result["session_id"].(string)
		}(i)
	}

	wg.Wait()

	// Verify all sessions were created successfully
	for i := 0; i < 10; i++ {
		assert.NoError(t, errors[i])
		assert.NotEmpty(t, sessionIDs[i])
	}

	// Verify unique session IDs
	uniqueIDs := make(map[string]bool)
	for _, id := range sessionIDs {
		uniqueIDs[id] = true
	}
	assert.Equal(t, 10, len(uniqueIDs), "All session IDs should be unique")
}

// TestACPHandler_REST_Endpoints tests backward compatibility with REST endpoints
func TestACPHandler_REST_Endpoints(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	// Test health endpoint
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/acp/health", nil)

	handler.Health(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var healthResponse map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &healthResponse)
	require.NoError(t, err)
	assert.Equal(t, "healthy", healthResponse["status"])
	assert.Equal(t, float64(6), healthResponse["agent_count"])

	// Test list agents endpoint
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("GET", "/v1/acp/agents", nil)

	handler.ListAgents(c2)

	assert.Equal(t, http.StatusOK, w2.Code)
	var listResponse map[string]interface{}
	err = json.Unmarshal(w2.Body.Bytes(), &listResponse)
	require.NoError(t, err)
	agents := listResponse["agents"].([]interface{})
	assert.Equal(t, 6, len(agents))
	assert.Equal(t, float64(6), listResponse["count"])

	// Test get agent endpoint (existing agent)
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest("GET", "/v1/acp/agents/code-reviewer", nil)
	c3.Params = gin.Params{gin.Param{Key: "agent_id", Value: "code-reviewer"}}

	handler.GetAgent(c3)

	assert.Equal(t, http.StatusOK, w3.Code)
	var agentResponse map[string]interface{}
	err = json.Unmarshal(w3.Body.Bytes(), &agentResponse)
	require.NoError(t, err)
	assert.Equal(t, "code-reviewer", agentResponse["id"])

	// Test get agent endpoint (non-existent agent)
	w4 := httptest.NewRecorder()
	c4, _ := gin.CreateTestContext(w4)
	c4.Request = httptest.NewRequest("GET", "/v1/acp/agents/non-existent", nil)
	c4.Params = gin.Params{gin.Param{Key: "agent_id", Value: "non-existent"}}

	handler.GetAgent(c4)

	assert.Equal(t, http.StatusNotFound, w4.Code)
	var errorResponse map[string]interface{}
	err = json.Unmarshal(w4.Body.Bytes(), &errorResponse)
	require.NoError(t, err)
	assert.Equal(t, "agent not found", errorResponse["error"])

	// Test execute endpoint
	executeReq := map[string]interface{}{
		"agent_id": "code-reviewer",
		"task":     "Review code",
	}
	reqBytes, _ := json.Marshal(executeReq)
	w5 := httptest.NewRecorder()
	c5, _ := gin.CreateTestContext(w5)
	c5.Request = httptest.NewRequest("POST", "/v1/acp/execute", bytes.NewBuffer(reqBytes))
	c5.Request.Header.Set("Content-Type", "application/json")

	handler.Execute(c5)

	assert.Equal(t, http.StatusOK, w5.Code)
	var executeResponse map[string]interface{}
	err = json.Unmarshal(w5.Body.Bytes(), &executeResponse)
	require.NoError(t, err)
	assert.Equal(t, "completed", executeResponse["status"])
	assert.Equal(t, "code-reviewer", executeResponse["agent_id"])
}

// TestACPHandler_RegisterRoutes tests route registration
func TestACPHandler_RegisterRoutes(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	router := gin.New()
	group := router.Group("/v1")
	handler.RegisterRoutes(group)

	// Verify routes are registered
	routes := router.Routes()

	expectedRoutes := map[string][]string{
		"/v1/acp/health":           {"GET"},
		"/v1/acp/agents":           {"GET"},
		"/v1/acp/agents/:agent_id": {"GET"},
		"/v1/acp/execute":          {"POST"},
		"/v1/acp/rpc":              {"POST"},
	}

	for path, methods := range expectedRoutes {
		for _, method := range methods {
			found := false
			for _, route := range routes {
				if route.Path == path && route.Method == method {
					found = true
					break
				}
			}
			assert.True(t, found, "Route %s %s should be registered", method, path)
		}
	}
}

// TestACPHandler_AllAgentExecutions tests execution of all agent types
func TestACPHandler_AllAgentExecutions(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	agents := []string{
		"code-reviewer",
		"bug-finder",
		"refactor-assistant",
		"documentation-generator",
		"test-generator",
		"security-scanner",
	}

	for _, agentID := range agents {
		t.Run(agentID, func(t *testing.T) {
				t.Parallel()
			msg := JSONRPCMessage{
				JSONRPC: "2.0",
				ID:      agentID,
				Method:  "agent/execute",
				Params:  json.RawMessage(`{"agent_id": "` + agentID + `", "task": "Test task", "context": {"code": "func test() {}", "language": "go"}}`),
			}

			reqBytes, _ := json.Marshal(msg)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.HandleJSONRPC(c)

			assert.Equal(t, http.StatusOK, w.Code)

			var response JSONRPCMessage
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, "2.0", response.JSONRPC)
			assert.Nil(t, response.Error, "Agent %s execution failed: %v", agentID, response.Error)

			result := response.Result.(map[string]interface{})
			assert.Equal(t, "completed", result["status"])
			assert.Equal(t, agentID, result["agent_id"])
		})
	}
}

// TestACPHandler_DetectTaskType tests the detectTaskType function
func TestACPHandler_DetectTaskType(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	tests := []struct {
		task     string
		expected string
	}{
		{"review this code", "review"},
		{"find bugs", "bug_detection"},
		{"refactor code", "refactoring"},
		{"document this", "documentation"},
		{"generate documentation", "documentation"},
		{"create tests", "test_generation"},
		{"security scan", "security_scan"},
		{"scan for vulnerabilities", "security_scan"},
		{"analyze code", "analysis"},
		{"", "analysis"},
	}

	for _, tc := range tests {
		result := handler.detectTaskType(tc.task)
		assert.Equal(t, tc.expected, result, "Task: %s", tc.task)
	}
}

// TestACPHandler_InitializeWithClientInfo tests initialize with clientInfo
func TestACPHandler_InitializeWithClientInfo(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      100,
		Method:  "initialize",
		Params:  json.RawMessage(`{"clientInfo": {"name": "test-client", "version": "1.0"}}`),
	}

	reqBytes, _ := json.Marshal(msg)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.Nil(t, response.Error)
}

// TestACPHandler_AgentListFilter tests agent/list with various filters
func TestACPHandler_AgentListFilter(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	// Test filter that matches no agents
	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      101,
		Method:  "agent/list",
		Params:  json.RawMessage(`{"filter": "xyz123nonexistent"}`),
	}

	reqBytes, _ := json.Marshal(msg)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Nil(t, response.Error)
	result := response.Result.(map[string]interface{})
	agents := result["agents"].([]interface{})
	assert.Equal(t, 0, len(agents))
	assert.Equal(t, float64(0), result["count"])
}

// TestACPHandler_ExecuteWithTimeout tests agent execution with timeout (should be ignored but not error)
func TestACPHandler_ExecuteWithTimeout(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	// Test REST execute with timeout field (should be ignored)
	executeReq := map[string]interface{}{
		"agent_id": "code-reviewer",
		"task":     "Review code",
		"timeout":  5000,
	}
	reqBytes, _ := json.Marshal(executeReq)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/execute", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Execute(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var executeResponse map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &executeResponse)
	require.NoError(t, err)
	assert.Equal(t, "completed", executeResponse["status"])
}

// TestACPHandler_GetContextKeys tests getContextKeys function
func TestACPHandler_GetContextKeys(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	// Test nil context
	keys := handler.getContextKeys(nil)
	assert.Empty(t, keys)

	// Test empty context
	keys = handler.getContextKeys(map[string]interface{}{})
	assert.Empty(t, keys)

	// Test with keys
	context := map[string]interface{}{
		"code":     "func test() {}",
		"language": "go",
		"user_id":  123,
	}
	keys = handler.getContextKeys(context)
	assert.Len(t, keys, 3)
	assert.Contains(t, keys, "code")
	assert.Contains(t, keys, "language")
	assert.Contains(t, keys, "user_id")
}

// TestACPHandler_HandleJSONRPC_Initialize_InvalidParams tests initialize with invalid params
func TestACPHandler_HandleJSONRPC_Initialize_InvalidParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      200,
		Method:  "initialize",
		Params:  json.RawMessage(`123`), // Invalid params (should be object)
	}

	reqBytes, _ := json.Marshal(msg)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.NotNil(t, response.Error)
	assert.Equal(t, -32602, response.Error.Code)
	assert.Contains(t, response.Error.Message, "Invalid params")
}

// TestACPHandler_HandleJSONRPC_AgentList_InvalidParams tests agent/list with invalid params
func TestACPHandler_HandleJSONRPC_AgentList_InvalidParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      201,
		Method:  "agent/list",
		Params:  json.RawMessage(`123`), // Invalid params (should be object)
	}

	reqBytes, _ := json.Marshal(msg)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.NotNil(t, response.Error)
	assert.Equal(t, -32602, response.Error.Code)
	assert.Contains(t, response.Error.Message, "Invalid params")
}

// TestACPHandler_HandleJSONRPC_AgentExecute_InvalidParams tests agent/execute with invalid params
func TestACPHandler_HandleJSONRPC_AgentExecute_InvalidParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      202,
		Method:  "agent/execute",
		Params:  json.RawMessage(`123`), // Invalid params (should be object)
	}

	reqBytes, _ := json.Marshal(msg)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.NotNil(t, response.Error)
	assert.Equal(t, -32602, response.Error.Code)
	assert.Contains(t, response.Error.Message, "Invalid params")
}

// TestACPHandler_HandleJSONRPC_SessionCreate_InvalidParams tests session/create with invalid params
func TestACPHandler_HandleJSONRPC_SessionCreate_InvalidParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      203,
		Method:  "session/create",
		Params:  json.RawMessage(`123`), // Invalid params (should be object)
	}

	reqBytes, _ := json.Marshal(msg)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.NotNil(t, response.Error)
	assert.Equal(t, -32602, response.Error.Code)
	assert.Contains(t, response.Error.Message, "Invalid params")
}

// TestACPHandler_HandleJSONRPC_SessionUpdate_InvalidParams tests session/update with invalid params
func TestACPHandler_HandleJSONRPC_SessionUpdate_InvalidParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	handler := NewACPHandler(nil, logger)

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      204,
		Method:  "session/update",
		Params:  json.RawMessage(`123`), // Invalid params (should be object)
	}

	reqBytes, _ := json.Marshal(msg)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/acp/rpc", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleJSONRPC(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response JSONRPCMessage
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.NotNil(t, response.Error)
	assert.Equal(t, -32602, response.Error.Code)
	assert.Contains(t, response.Error.Message, "Invalid params")
}

// TestACPHandler_ExecuteGenericAgent tests the generic agent execution fallback
