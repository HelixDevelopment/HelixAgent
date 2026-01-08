package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"dev.helix.agent/internal/config"
)

func newACPManagerTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

// ========== ACPClient Tests ==========

func TestNewACPClient(t *testing.T) {
	log := newACPManagerTestLogger()
	client := NewACPClient(30*time.Second, 3, log)

	require.NotNil(t, client)
	assert.NotNil(t, client.httpClient)
	assert.NotNil(t, client.wsDialer)
	assert.NotNil(t, client.wsConns)
	assert.Equal(t, 30*time.Second, client.timeout)
	assert.Equal(t, 3, client.maxRetries)
}

func TestACPClient_ExecuteHTTP_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req ACPProtocolRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := ACPProtocolResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]string{"status": "success"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	log := newACPManagerTestLogger()
	client := NewACPClient(5*time.Second, 1, log)

	req := ACPProtocolRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test_action",
		Params:  map[string]string{"key": "value"},
	}

	ctx := context.Background()
	resp, err := client.ExecuteHTTP(ctx, server.URL, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
}

func TestACPClient_ExecuteHTTP_ServerError(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	log := newACPManagerTestLogger()
	client := NewACPClient(1*time.Second, 0, log) // No retries

	req := ACPProtocolRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test_action",
	}

	ctx := context.Background()
	resp, err := client.ExecuteHTTP(ctx, server.URL, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "500")
}

func TestACPClient_ExecuteHTTP_InvalidURL(t *testing.T) {
	log := newACPManagerTestLogger()
	client := NewACPClient(1*time.Second, 0, log)

	req := ACPProtocolRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test_action",
	}

	ctx := context.Background()
	resp, err := client.ExecuteHTTP(ctx, "http://invalid-url-that-does-not-exist.local:12345", req)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestACPClient_ExecuteHTTP_InvalidJSON(t *testing.T) {
	// Create test server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	log := newACPManagerTestLogger()
	client := NewACPClient(1*time.Second, 0, log)

	req := ACPProtocolRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test_action",
	}

	ctx := context.Background()
	resp, err := client.ExecuteHTTP(ctx, server.URL, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestACPClient_ExecuteHTTP_ContextCancellation(t *testing.T) {
	log := newACPManagerTestLogger()
	client := NewACPClient(5*time.Second, 3, log)

	req := ACPProtocolRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test_action",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	resp, err := client.ExecuteHTTP(ctx, "http://localhost:7061", req)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestACPClient_ExecuteHTTP_WithRetries(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := ACPProtocolResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  "success",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	log := newACPManagerTestLogger()
	client := NewACPClient(1*time.Second, 3, log)

	req := ACPProtocolRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test_action",
	}

	ctx := context.Background()
	resp, err := client.ExecuteHTTP(ctx, server.URL, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 3, attempts)
}

func TestACPClient_ExecuteWS_Success(t *testing.T) {
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		var req ACPProtocolRequest
		conn.ReadJSON(&req)

		resp := ACPProtocolResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]string{"status": "ws_success"},
		}
		conn.WriteJSON(resp)
	}))
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	log := newACPManagerTestLogger()
	client := NewACPClient(5*time.Second, 1, log)
	defer client.CloseAll()

	req := ACPProtocolRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test_action",
	}

	ctx := context.Background()
	resp, err := client.ExecuteWS(ctx, wsURL, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "2.0", resp.JSONRPC)
}

func TestACPClient_ExecuteWS_ConnectionError(t *testing.T) {
	log := newACPManagerTestLogger()
	client := NewACPClient(1*time.Second, 1, log)

	req := ACPProtocolRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test_action",
	}

	ctx := context.Background()
	resp, err := client.ExecuteWS(ctx, "ws://localhost:59999", req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "connect")
}

func TestACPClient_ExecuteWS_ReuseConnection(t *testing.T) {
	connectionCount := 0
	var mu sync.Mutex
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		connectionCount++
		mu.Unlock()

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			var req ACPProtocolRequest
			if err := conn.ReadJSON(&req); err != nil {
				return
			}

			resp := ACPProtocolResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  "success",
			}
			if err := conn.WriteJSON(resp); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	log := newACPManagerTestLogger()
	client := NewACPClient(5*time.Second, 1, log)
	defer client.CloseAll()

	ctx := context.Background()

	// Make multiple requests
	for i := 0; i < 3; i++ {
		req := ACPProtocolRequest{
			JSONRPC: "2.0",
			ID:      i,
			Method:  "test_action",
		}
		_, err := client.ExecuteWS(ctx, wsURL, req)
		require.NoError(t, err)
	}

	// Should only have one connection
	assert.Equal(t, 1, connectionCount)
}

func TestACPClient_GetServerInfo_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.True(t, strings.HasSuffix(r.URL.Path, "/info"))

		info := ACPServerInfo{
			Name:    "Test Server",
			Version: "1.0.0",
			Capabilities: []ACPCapability{
				{Name: "test", Description: "Test capability"},
			},
		}
		json.NewEncoder(w).Encode(info)
	}))
	defer server.Close()

	log := newACPManagerTestLogger()
	client := NewACPClient(5*time.Second, 1, log)

	ctx := context.Background()
	info, err := client.GetServerInfo(ctx, server.URL)

	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "Test Server", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Len(t, info.Capabilities, 1)
}

func TestACPClient_GetServerInfo_WithInfoSuffix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := ACPServerInfo{
			Name:    "Test Server",
			Version: "1.0.0",
		}
		json.NewEncoder(w).Encode(info)
	}))
	defer server.Close()

	log := newACPManagerTestLogger()
	client := NewACPClient(5*time.Second, 1, log)

	ctx := context.Background()
	// URL already has /info suffix
	info, err := client.GetServerInfo(ctx, server.URL+"/info")

	require.NoError(t, err)
	require.NotNil(t, info)
}

func TestACPClient_GetServerInfo_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	log := newACPManagerTestLogger()
	client := NewACPClient(1*time.Second, 0, log)

	ctx := context.Background()
	info, err := client.GetServerInfo(ctx, server.URL)

	assert.Error(t, err)
	assert.Nil(t, info)
}

func TestACPClient_CloseAll(t *testing.T) {
	log := newACPManagerTestLogger()
	client := NewACPClient(5*time.Second, 1, log)

	// Add some mock connections
	client.wsConnsMu.Lock()
	client.wsConns["ws://test1"] = nil
	client.wsConns["ws://test2"] = nil
	client.wsConnsMu.Unlock()

	client.CloseAll()

	client.wsConnsMu.RLock()
	assert.Empty(t, client.wsConns)
	client.wsConnsMu.RUnlock()
}

// ========== ACPManager Tests ==========

func TestNewACPManager(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)

	require.NotNil(t, manager)
	assert.Nil(t, manager.repo)
	assert.Nil(t, manager.cache)
	assert.NotNil(t, manager.log)
	assert.NotNil(t, manager.client)
	assert.NotNil(t, manager.servers)
}

func TestNewACPManagerWithConfig(t *testing.T) {
	log := newACPManagerTestLogger()
	cfg := &config.ACPConfig{
		Enabled:        true,
		DefaultTimeout: 60 * time.Second,
		MaxRetries:     5,
		Servers: []config.ACPServerConfig{
			{
				ID:      "server-1",
				Name:    "Test Server",
				URL:     "http://localhost:7061",
				Enabled: true,
			},
		},
	}

	manager := NewACPManagerWithConfig(nil, nil, log, cfg)

	require.NotNil(t, manager)
	assert.Equal(t, cfg, manager.config)
	assert.Len(t, manager.servers, 1)
	assert.Equal(t, "server-1", manager.servers["server-1"].ID)
}

func TestNewACPManagerWithConfig_NilConfig(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManagerWithConfig(nil, nil, log, nil)

	require.NotNil(t, manager)
	assert.Empty(t, manager.servers)
}

func TestACPManager_RegisterServer(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)

	server := &ACPServer{
		ID:      "new-server",
		Name:    "New Server",
		URL:     "http://localhost:9090",
		Enabled: true,
	}

	err := manager.RegisterServer(server)
	require.NoError(t, err)

	// Verify server was added
	ctx := context.Background()
	retrieved, err := manager.GetACPServer(ctx, "new-server")
	require.NoError(t, err)
	assert.Equal(t, "New Server", retrieved.Name)
}

func TestACPManager_RegisterServer_MissingID(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)

	server := &ACPServer{
		Name: "No ID Server",
		URL:  "http://localhost:9090",
	}

	err := manager.RegisterServer(server)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server ID is required")
}

func TestACPManager_RegisterServer_MissingURL(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)

	server := &ACPServer{
		ID:   "server-1",
		Name: "No URL Server",
	}

	err := manager.RegisterServer(server)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server URL is required")
}

func TestACPManager_UnregisterServer(t *testing.T) {
	log := newACPManagerTestLogger()
	cfg := &config.ACPConfig{
		Servers: []config.ACPServerConfig{
			{ID: "server-1", Name: "Test", URL: "http://localhost:7061", Enabled: true},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)

	err := manager.UnregisterServer("server-1")
	require.NoError(t, err)

	// Verify server was removed
	ctx := context.Background()
	_, err = manager.GetACPServer(ctx, "server-1")
	assert.Error(t, err)
}

func TestACPManager_UnregisterServer_NotFound(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)

	err := manager.UnregisterServer("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestACPManager_ListACPServers_Empty(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)
	ctx := context.Background()

	servers, err := manager.ListACPServers(ctx)
	require.NoError(t, err)
	assert.Empty(t, servers)
}

func TestACPManager_ListACPServers_WithServers(t *testing.T) {
	log := newACPManagerTestLogger()
	cfg := &config.ACPConfig{
		Servers: []config.ACPServerConfig{
			{ID: "server-1", Name: "Server 1", URL: "http://localhost:7061", Enabled: true},
			{ID: "server-2", Name: "Server 2", URL: "http://localhost:8081", Enabled: false},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)
	ctx := context.Background()

	servers, err := manager.ListACPServers(ctx)
	require.NoError(t, err)
	assert.Len(t, servers, 2)
}

func TestACPManager_GetACPServer_Found(t *testing.T) {
	log := newACPManagerTestLogger()
	cfg := &config.ACPConfig{
		Servers: []config.ACPServerConfig{
			{ID: "server-1", Name: "Test Server", URL: "http://localhost:7061", Enabled: true},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)
	ctx := context.Background()

	server, err := manager.GetACPServer(ctx, "server-1")
	require.NoError(t, err)
	require.NotNil(t, server)
	assert.Equal(t, "Test Server", server.Name)
}

func TestACPManager_GetACPServer_NotFound(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)
	ctx := context.Background()

	server, err := manager.GetACPServer(ctx, "non-existent")
	assert.Error(t, err)
	assert.Nil(t, server)
	assert.Contains(t, err.Error(), "not found")
}

func TestACPManager_ExecuteACPAction_HTTPSuccess(t *testing.T) {
	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ACPProtocolRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := ACPProtocolResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]string{"status": "completed"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	log := newACPManagerTestLogger()
	cfg := &config.ACPConfig{
		Servers: []config.ACPServerConfig{
			{ID: "http-server", Name: "HTTP Server", URL: server.URL, Enabled: true},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)
	ctx := context.Background()

	req := ACPRequest{
		ServerID:   "http-server",
		Action:     "test_action",
		Parameters: map[string]interface{}{"key": "value"},
	}

	resp, err := manager.ExecuteACPAction(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Data)
}

func TestACPManager_ExecuteACPAction_WebSocketSuccess(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		var req ACPProtocolRequest
		conn.ReadJSON(&req)

		resp := ACPProtocolResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]string{"status": "ws_completed"},
		}
		conn.WriteJSON(resp)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	log := newACPManagerTestLogger()
	cfg := &config.ACPConfig{
		Servers: []config.ACPServerConfig{
			{ID: "ws-server", Name: "WS Server", URL: wsURL, Enabled: true},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)
	defer manager.Close()
	ctx := context.Background()

	req := ACPRequest{
		ServerID: "ws-server",
		Action:   "test_action",
	}

	resp, err := manager.ExecuteACPAction(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
}

func TestACPManager_ExecuteACPAction_ServerNotFound(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)
	ctx := context.Background()

	req := ACPRequest{
		ServerID: "non-existent",
		Action:   "test_action",
	}

	resp, err := manager.ExecuteACPAction(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "invalid server ID")
}

func TestACPManager_ExecuteACPAction_ServerDisabled(t *testing.T) {
	log := newACPManagerTestLogger()
	cfg := &config.ACPConfig{
		Servers: []config.ACPServerConfig{
			{ID: "disabled-server", Name: "Disabled", URL: "http://localhost:7061", Enabled: false},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)
	ctx := context.Background()

	req := ACPRequest{
		ServerID: "disabled-server",
		Action:   "test_action",
	}

	resp, err := manager.ExecuteACPAction(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not enabled")
}

func TestACPManager_ExecuteACPAction_ConnectionError(t *testing.T) {
	log := newACPManagerTestLogger()
	cfg := &config.ACPConfig{
		DefaultTimeout: 1 * time.Second,
		MaxRetries:     0,
		Servers: []config.ACPServerConfig{
			{ID: "bad-server", Name: "Bad Server", URL: "http://localhost:59999", Enabled: true},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)
	ctx := context.Background()

	req := ACPRequest{
		ServerID: "bad-server",
		Action:   "test_action",
	}

	resp, err := manager.ExecuteACPAction(ctx, req)
	require.NoError(t, err) // No error returned, but response indicates failure
	require.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.NotEmpty(t, resp.Error)
}

func TestACPManager_ExecuteACPAction_RPCError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ACPProtocolResponse{
			JSONRPC: "2.0",
			ID:      1,
			Error: &ACPRPCError{
				Code:    -32600,
				Message: "Invalid Request",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	log := newACPManagerTestLogger()
	cfg := &config.ACPConfig{
		Servers: []config.ACPServerConfig{
			{ID: "error-server", Name: "Error Server", URL: server.URL, Enabled: true},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)
	ctx := context.Background()

	req := ACPRequest{
		ServerID: "error-server",
		Action:   "test_action",
	}

	resp, err := manager.ExecuteACPAction(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Invalid Request", resp.Error)
}

func TestACPManager_ValidateACPRequest_Valid(t *testing.T) {
	log := newACPManagerTestLogger()
	cfg := &config.ACPConfig{
		Servers: []config.ACPServerConfig{
			{ID: "server-1", Name: "Test", URL: "http://localhost:7061", Enabled: true},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)
	ctx := context.Background()

	req := ACPRequest{
		ServerID: "server-1",
		Action:   "test_action",
	}

	err := manager.ValidateACPRequest(ctx, req)
	assert.NoError(t, err)
}

func TestACPManager_ValidateACPRequest_MissingServerID(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)
	ctx := context.Background()

	req := ACPRequest{
		ServerID: "",
		Action:   "test_action",
	}

	err := manager.ValidateACPRequest(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server ID is required")
}

func TestACPManager_ValidateACPRequest_MissingAction(t *testing.T) {
	log := newACPManagerTestLogger()
	cfg := &config.ACPConfig{
		Servers: []config.ACPServerConfig{
			{ID: "server-1", Name: "Test", URL: "http://localhost:7061", Enabled: true},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)
	ctx := context.Background()

	req := ACPRequest{
		ServerID: "server-1",
		Action:   "",
	}

	err := manager.ValidateACPRequest(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "action is required")
}

func TestACPManager_ValidateACPRequest_InvalidServer(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)
	ctx := context.Background()

	req := ACPRequest{
		ServerID: "non-existent",
		Action:   "test_action",
	}

	err := manager.ValidateACPRequest(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid server ID")
}

func TestACPManager_ValidateACPRequest_DisabledServer(t *testing.T) {
	log := newACPManagerTestLogger()
	cfg := &config.ACPConfig{
		Servers: []config.ACPServerConfig{
			{ID: "disabled-server", Name: "Disabled", URL: "http://localhost:7061", Enabled: false},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)
	ctx := context.Background()

	req := ACPRequest{
		ServerID: "disabled-server",
		Action:   "test_action",
	}

	err := manager.ValidateACPRequest(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enabled")
}

func TestACPManager_SyncACPServer_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := ACPServerInfo{
			Name:    "Synced Server",
			Version: "2.0.0",
			Capabilities: []ACPCapability{
				{Name: "capability1", Description: "First capability"},
				{Name: "capability2", Description: "Second capability"},
			},
		}
		json.NewEncoder(w).Encode(info)
	}))
	defer server.Close()

	log := newACPManagerTestLogger()
	cfg := &config.ACPConfig{
		Servers: []config.ACPServerConfig{
			{ID: "sync-server", Name: "Original Name", URL: server.URL, Enabled: true},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)
	ctx := context.Background()

	err := manager.SyncACPServer(ctx, "sync-server")
	require.NoError(t, err)

	// Verify server was updated
	s, err := manager.GetACPServer(ctx, "sync-server")
	require.NoError(t, err)
	assert.Equal(t, "Synced Server", s.Name)
	assert.Equal(t, "2.0.0", s.Version)
	assert.Len(t, s.Capabilities, 2)
	assert.NotNil(t, s.LastSync)
}

func TestACPManager_SyncACPServer_WebSocketURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := ACPServerInfo{
			Name:    "WS Synced Server",
			Version: "1.5.0",
		}
		json.NewEncoder(w).Encode(info)
	}))
	defer server.Close()

	// Use HTTP URL for the mock server but configure as WS
	// The manager should convert ws:// to http:// for the info endpoint
	httpURL := server.URL

	log := newACPManagerTestLogger()
	cfg := &config.ACPConfig{
		Servers: []config.ACPServerConfig{
			{ID: "ws-sync-server", Name: "WS Server", URL: httpURL, Enabled: true},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)
	ctx := context.Background()

	err := manager.SyncACPServer(ctx, "ws-sync-server")
	require.NoError(t, err)
}

func TestACPManager_SyncACPServer_NotFound(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)
	ctx := context.Background()

	err := manager.SyncACPServer(ctx, "non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server not found")
}

func TestACPManager_SyncACPServer_ConnectionError(t *testing.T) {
	log := newACPManagerTestLogger()
	cfg := &config.ACPConfig{
		DefaultTimeout: 1 * time.Second,
		Servers: []config.ACPServerConfig{
			{ID: "bad-server", Name: "Bad", URL: "http://localhost:59999", Enabled: true},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)
	ctx := context.Background()

	err := manager.SyncACPServer(ctx, "bad-server")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch server info")
}

func TestACPManager_GetACPStats_Empty(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)
	ctx := context.Background()

	stats, err := manager.GetACPStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 0, stats["totalServers"])
	assert.Equal(t, 0, stats["enabledServers"])
	assert.Equal(t, 0, stats["totalCapabilities"])
}

func TestACPManager_GetACPStats_WithServers(t *testing.T) {
	log := newACPManagerTestLogger()
	cfg := &config.ACPConfig{
		Servers: []config.ACPServerConfig{
			{ID: "server-1", Name: "Server 1", URL: "http://localhost:7061", Enabled: true},
			{ID: "server-2", Name: "Server 2", URL: "http://localhost:8081", Enabled: false},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)

	// Add capabilities to enabled server
	manager.serversMu.Lock()
	manager.servers["server-1"].Capabilities = []ACPCapability{
		{Name: "cap1"},
		{Name: "cap2"},
	}
	manager.serversMu.Unlock()

	ctx := context.Background()
	stats, err := manager.GetACPStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 2, stats["totalServers"])
	assert.Equal(t, 1, stats["enabledServers"])
	assert.Equal(t, 2, stats["totalCapabilities"])
	assert.Contains(t, stats, "lastSync")
}

func TestACPManager_Close(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)

	err := manager.Close()
	assert.NoError(t, err)
}

func TestACPManager_isWebSocketURL(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)

	tests := []struct {
		url      string
		expected bool
	}{
		{"ws://localhost:7061", true},
		{"wss://localhost:7061", true},
		{"http://localhost:7061", false},
		{"https://localhost:7061", false},
		{"invalid-url", false},
		{"://bad-scheme", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := manager.isWebSocketURL(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestACPManager_getHTTPURL(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)

	tests := []struct {
		input    string
		expected string
	}{
		{"ws://localhost:7061/path", "http://localhost:7061/path"},
		{"wss://localhost:7061/path", "https://localhost:7061/path"},
		{"http://localhost:7061/path", "http://localhost:7061/path"},
		{"https://localhost:7061/path", "https://localhost:7061/path"},
		{"://invalid", "://invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := manager.getHTTPURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== Type Structure Tests ==========

func TestACPServer_Structure(t *testing.T) {
	now := time.Now()
	server := ACPServer{
		ID:      "test-server",
		Name:    "Test Server",
		URL:     "ws://localhost:7061/agent",
		Enabled: true,
		Version: "1.0.0",
		Capabilities: []ACPCapability{
			{Name: "test", Description: "Test capability"},
		},
		LastSync: &now,
	}

	assert.Equal(t, "test-server", server.ID)
	assert.Equal(t, "Test Server", server.Name)
	assert.True(t, server.Enabled)
	assert.Len(t, server.Capabilities, 1)
	assert.NotNil(t, server.LastSync)
}

func TestACPCapability_Structure(t *testing.T) {
	cap := ACPCapability{
		Name:        "code_execution",
		Description: "Execute code and return results",
		Parameters: map[string]interface{}{
			"language": "string",
			"code":     "string",
		},
	}

	assert.Equal(t, "code_execution", cap.Name)
	assert.Equal(t, "Execute code and return results", cap.Description)
	assert.NotNil(t, cap.Parameters)
}

func TestACPRequest_Structure(t *testing.T) {
	req := ACPRequest{
		ServerID: "server-1",
		Action:   "code_execution",
		Parameters: map[string]interface{}{
			"language": "go",
			"code":     "fmt.Println()",
		},
	}

	assert.Equal(t, "server-1", req.ServerID)
	assert.Equal(t, "code_execution", req.Action)
	assert.NotNil(t, req.Parameters)
}

func TestACPResponse_Structure(t *testing.T) {
	resp := ACPResponse{
		Success:   true,
		Data:      map[string]string{"result": "success"},
		Error:     "",
		Timestamp: time.Now(),
	}

	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Data)
	assert.Empty(t, resp.Error)
	assert.False(t, resp.Timestamp.IsZero())
}

func TestACPProtocolRequest_Structure(t *testing.T) {
	req := ACPProtocolRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test_method",
		Params:  map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, "2.0", req.JSONRPC)
	assert.Equal(t, 1, req.ID)
	assert.Equal(t, "test_method", req.Method)
	assert.NotNil(t, req.Params)
}

func TestACPProtocolResponse_Structure(t *testing.T) {
	resp := ACPProtocolResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  map[string]string{"status": "ok"},
		Error:   nil,
	}

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 1, resp.ID)
	assert.NotNil(t, resp.Result)
	assert.Nil(t, resp.Error)
}

func TestACPRPCError_Structure(t *testing.T) {
	err := ACPRPCError{
		Code:    -32600,
		Message: "Invalid Request",
		Data:    "Additional info",
	}

	assert.Equal(t, -32600, err.Code)
	assert.Equal(t, "Invalid Request", err.Message)
	assert.Equal(t, "Additional info", err.Data)
}

func TestACPServerInfo_Structure(t *testing.T) {
	info := ACPServerInfo{
		Name:    "Test Server",
		Version: "1.0.0",
		Capabilities: []ACPCapability{
			{Name: "test"},
		},
	}

	assert.Equal(t, "Test Server", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Len(t, info.Capabilities, 1)
}

// ========== Concurrency Tests ==========

func TestACPManager_ConcurrentAccess(t *testing.T) {
	log := newACPManagerTestLogger()
	manager := NewACPManager(nil, nil, log)
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Register servers concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			server := &ACPServer{
				ID:      fmt.Sprintf("server-%d", id),
				Name:    fmt.Sprintf("Server %d", id),
				URL:     fmt.Sprintf("http://localhost:%d", 8000+id),
				Enabled: true,
			}
			manager.RegisterServer(server)
		}(i)
	}

	// List servers concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.ListACPServers(ctx)
		}()
	}

	// Get stats concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.GetACPStats(ctx)
		}()
	}

	wg.Wait()

	// Verify all servers were registered
	servers, err := manager.ListACPServers(ctx)
	require.NoError(t, err)
	assert.Len(t, servers, numGoroutines)
}

// ========== Benchmarks ==========

func BenchmarkACPManager_ListACPServers(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	cfg := &config.ACPConfig{
		Servers: []config.ACPServerConfig{
			{ID: "server-1", Name: "Server 1", URL: "http://localhost:7061", Enabled: true},
			{ID: "server-2", Name: "Server 2", URL: "http://localhost:8081", Enabled: true},
			{ID: "server-3", Name: "Server 3", URL: "http://localhost:8082", Enabled: false},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.ListACPServers(ctx)
	}
}

func BenchmarkACPManager_GetACPServer(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	cfg := &config.ACPConfig{
		Servers: []config.ACPServerConfig{
			{ID: "server-1", Name: "Server 1", URL: "http://localhost:7061", Enabled: true},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetACPServer(ctx, "server-1")
	}
}

func BenchmarkACPManager_GetACPStats(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)

	cfg := &config.ACPConfig{
		Servers: []config.ACPServerConfig{
			{ID: "server-1", Name: "Server 1", URL: "http://localhost:7061", Enabled: true},
			{ID: "server-2", Name: "Server 2", URL: "http://localhost:8081", Enabled: true},
		},
	}
	manager := NewACPManagerWithConfig(nil, nil, log, cfg)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetACPStats(ctx)
	}
}

func TestACPClient_ExecuteWS_WriteError(t *testing.T) {
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		// Close connection immediately to cause write error
		conn.Close()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	log := newACPManagerTestLogger()
	client := NewACPClient(5*time.Second, 0, log)
	defer client.CloseAll()

	ctx := context.Background()

	req := ACPProtocolRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test_action",
	}

	// Wait a bit for server to close connection
	time.Sleep(100 * time.Millisecond)

	resp, err := client.ExecuteWS(ctx, wsURL, req)
	// Should either fail to connect or fail to write
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestACPClient_ExecuteWS_ReadError(t *testing.T) {
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Read request but close connection before sending response
		var req ACPProtocolRequest
		conn.ReadJSON(&req)
		// Close without responding
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	log := newACPManagerTestLogger()
	client := NewACPClient(1*time.Second, 0, log)
	defer client.CloseAll()

	ctx := context.Background()

	req := ACPProtocolRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test_action",
	}

	resp, err := client.ExecuteWS(ctx, wsURL, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestACPClient_GetServerInfo_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	log := newACPManagerTestLogger()
	client := NewACPClient(1*time.Second, 0, log)

	ctx := context.Background()
	info, err := client.GetServerInfo(ctx, server.URL)

	assert.Error(t, err)
	assert.Nil(t, info)
}

func TestACPClient_GetServerInfo_RequestCreateError(t *testing.T) {
	log := newACPManagerTestLogger()
	client := NewACPClient(1*time.Second, 0, log)

	ctx := context.Background()
	// Invalid URL that will fail to create request
	info, err := client.GetServerInfo(ctx, "://invalid-url")

	assert.Error(t, err)
	assert.Nil(t, info)
}

func BenchmarkACPClient_ExecuteHTTP(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ACPProtocolResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  "success",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	client := NewACPClient(5*time.Second, 1, log)
	ctx := context.Background()

	req := ACPProtocolRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.ExecuteHTTP(ctx, server.URL, req)
	}
}
