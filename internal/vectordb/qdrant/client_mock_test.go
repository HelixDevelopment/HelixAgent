package qdrant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockServer creates a mock Qdrant server for testing
type mockServer struct {
	server *httptest.Server
	config *Config
}

// parseServerURL splits an httptest server URL into its host and port.
//
// It is a pure function so the parsing can be regression-tested against
// literal URLs with no listener and no network (see hostport_ipv6_test.go).
//
// This MUST NOT be done with strings.Split(authority, ":"). httptest's
// newLocalListener falls back to the IPv6 loopback when it cannot bind IPv4
// loopback (i.e. under port pressure), producing "http://[::1]:PORT". Naive
// splitting of that bracketed authority yields host "[" and an empty second
// field, and — in the code this replaced — a discarded fmt.Sscanf error left
// the port silently defaulting to 80, so the client was built for
// "http://[:80" and every request went nowhere near the mock server.
//
// net.SplitHostPort understands bracketed IPv6 authorities and returns the
// host UNBRACKETED ("::1"); Config.GetHTTPURL re-brackets it via hostPort.
// Every error is returned, never discarded.
func parseServerURL(rawURL string) (string, int, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", 0, fmt.Errorf("parsing server URL %q: %w", rawURL, err)
	}

	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", 0, fmt.Errorf("splitting authority %q: %w", u.Host, err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("parsing port %q from %q: %w", portStr, rawURL, err)
	}

	return host, port, nil
}

// newMockServer creates a new mock server and returns it along with a configured client
func newMockServer(handler http.HandlerFunc) *mockServer {
	server := httptest.NewServer(handler)

	host, port, err := parseServerURL(server.URL)
	if err != nil {
		// Unrecoverable test-harness setup failure. newMockServer has no
		// *testing.T to report through, and silently defaulting the port is
		// exactly the defect this replaced (see parseServerURL).
		server.Close()
		panic(fmt.Sprintf("newMockServer: %v", err))
	}

	config := &Config{
		Host:           host,
		HTTPPort:       port,
		GRPCPort:       6334,
		Timeout:        5 * time.Second,
		MaxRetries:     3,
		RetryDelay:     100 * time.Millisecond,
		DefaultLimit:   10,
		ScoreThreshold: 0.0,
		WithPayload:    true,
		WithVectors:    false,
	}

	return &mockServer{
		server: server,
		config: config,
	}
}

func (m *mockServer) close() {
	m.server.Close()
}

func (m *mockServer) newClient() (*Client, error) {
	return NewClient(m.config, logrus.New())
}

func (m *mockServer) newConnectedClient(t *testing.T) *Client {
	client, err := m.newClient()
	require.NoError(t, err)
	err = client.Connect(context.Background())
	require.NoError(t, err)
	return client
}

// =============================================================================
// Connection and Health Check Tests
// =============================================================================

func TestConnect_Success(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	defer ms.close()

	client, err := ms.newClient()
	require.NoError(t, err)

	err = client.Connect(context.Background())
	require.NoError(t, err)
	assert.True(t, client.IsConnected())
}

func TestConnect_ServerError(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer ms.close()

	client, err := ms.newClient()
	require.NoError(t, err)

	err = client.Connect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unhealthy status")
	assert.False(t, client.IsConnected())
}

func TestConnect_WithAPIKey(t *testing.T) {
	var receivedAPIKey string
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		receivedAPIKey = r.Header.Get("api-key")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	defer ms.close()

	ms.config.APIKey = "test-api-key"
	client, err := ms.newClient()
	require.NoError(t, err)

	err = client.Connect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "test-api-key", receivedAPIKey)
}

func TestHealthCheck_Success(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer ms.close()

	client, err := ms.newClient()
	require.NoError(t, err)

	err = client.HealthCheck(context.Background())
	require.NoError(t, err)
}

func TestHealthCheck_Failure(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	defer ms.close()

	client, err := ms.newClient()
	require.NoError(t, err)

	err = client.HealthCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unhealthy status")
}

func TestClose_Success(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer ms.close()

	client := ms.newConnectedClient(t)
	assert.True(t, client.IsConnected())

	err := client.Close()
	require.NoError(t, err)
	assert.False(t, client.IsConnected())
}

// =============================================================================
// Collection Management Tests
// =============================================================================

func TestCreateCollection_Success(t *testing.T) {
	var requestBody map[string]interface{}
	var requestPath string
	var requestMethod string

	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		requestPath = r.URL.Path
		requestMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &requestBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"result": true})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	config := DefaultCollectionConfig("test-collection", 1536)
	err := client.CreateCollection(context.Background(), config)
	require.NoError(t, err)

	assert.Equal(t, "/collections/test-collection", requestPath)
	assert.Equal(t, http.MethodPut, requestMethod)

	vectors, ok := requestBody["vectors"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(1536), vectors["size"])
	assert.Equal(t, "Cosine", vectors["distance"])
}

func TestCreateCollection_WithAllOptions(t *testing.T) {
	var requestBody map[string]interface{}

	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &requestBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"result": true})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	config := DefaultCollectionConfig("test-collection", 768).
		WithDistance(DistanceEuclid).
		WithOnDiskPayload().
		WithIndexingThreshold(50000).
		WithShards(3).
		WithReplication(2)

	err := client.CreateCollection(context.Background(), config)
	require.NoError(t, err)

	assert.True(t, requestBody["on_disk_payload"].(bool))
	assert.Equal(t, float64(3), requestBody["shard_number"])
	assert.Equal(t, float64(2), requestBody["replication_factor"])

	optimizers := requestBody["optimizers_config"].(map[string]interface{})
	assert.Equal(t, float64(50000), optimizers["indexing_threshold"])
}

func TestCreateCollection_InvalidConfig(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	// Invalid config - empty name
	config := &CollectionConfig{
		Name:       "",
		VectorSize: 1536,
		Distance:   DistanceCosine,
	}

	err := client.CreateCollection(context.Background(), config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid collection config")
}

func TestCreateCollection_ServerError(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "collection already exists"})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	config := DefaultCollectionConfig("existing-collection", 1536)
	err := client.CreateCollection(context.Background(), config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create collection")
}

func TestDeleteCollection_Success(t *testing.T) {
	var requestPath string
	var requestMethod string

	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		requestPath = r.URL.Path
		requestMethod = r.Method
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]bool{"result": true})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	err := client.DeleteCollection(context.Background(), "test-collection")
	require.NoError(t, err)

	assert.Equal(t, "/collections/test-collection", requestPath)
	assert.Equal(t, http.MethodDelete, requestMethod)
}

func TestDeleteCollection_ServerError(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "collection not found"})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	err := client.DeleteCollection(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete collection")
}

func TestCollectionExists_True(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"status": "green",
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	exists, err := client.CollectionExists(context.Background(), "test-collection")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestCollectionExists_False(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	exists, err := client.CollectionExists(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestListCollections_Success(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"collections": []map[string]string{
					{"name": "collection1"},
					{"name": "collection2"},
					{"name": "collection3"},
				},
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	collections, err := client.ListCollections(context.Background())
	require.NoError(t, err)
	assert.Len(t, collections, 3)
	assert.Equal(t, "collection1", collections[0])
	assert.Equal(t, "collection2", collections[1])
	assert.Equal(t, "collection3", collections[2])
}

func TestListCollections_Empty(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"collections": []map[string]string{},
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	collections, err := client.ListCollections(context.Background())
	require.NoError(t, err)
	assert.Empty(t, collections)
}

func TestListCollections_ServerError(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	collections, err := client.ListCollections(context.Background())
	require.Error(t, err)
	assert.Nil(t, collections)
}

func TestListCollections_InvalidJSON(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	collections, err := client.ListCollections(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
	assert.Nil(t, collections)
}

func TestGetCollectionInfo_Success(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"status":         "green",
				"vectors_count":  10000,
				"points_count":   10000,
				"segments_count": 5,
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	info, err := client.GetCollectionInfo(context.Background(), "test-collection")
	require.NoError(t, err)
	assert.Equal(t, "test-collection", info.Name)
	assert.Equal(t, "green", info.Status)
	assert.Equal(t, int64(10000), info.VectorCount)
	assert.Equal(t, int64(10000), info.PointsCount)
	assert.Equal(t, 5, info.SegmentsCount)
}

func TestGetCollectionInfo_ServerError(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	info, err := client.GetCollectionInfo(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.Nil(t, info)
}

func TestGetCollectionInfo_InvalidJSON(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	info, err := client.GetCollectionInfo(context.Background(), "test-collection")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
	assert.Nil(t, info)
}

// =============================================================================
// Point Operations Tests
// =============================================================================

func TestUpsertPoints_Success(t *testing.T) {
	var requestBody map[string]interface{}
	var requestPath string

	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		requestPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &requestBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": map[string]string{"status": "completed"}})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	points := []Point{
		{ID: "point-1", Vector: []float32{0.1, 0.2, 0.3}, Payload: map[string]interface{}{"key": "value1"}},
		{ID: "point-2", Vector: []float32{0.4, 0.5, 0.6}, Payload: map[string]interface{}{"key": "value2"}},
	}

	err := client.UpsertPoints(context.Background(), "test-collection", points)
	require.NoError(t, err)

	assert.Equal(t, "/collections/test-collection/points", requestPath)
	assert.NotNil(t, requestBody["points"])
}

func TestUpsertPoints_EmptySlice(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Should not reach here for empty points
		t.Error("Should not make request for empty points")
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	err := client.UpsertPoints(context.Background(), "test-collection", []Point{})
	require.NoError(t, err)
}

func TestUpsertPoints_AutoGenerateID(t *testing.T) {
	var requestBody map[string]interface{}

	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &requestBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": map[string]string{"status": "completed"}})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	points := []Point{
		{ID: "", Vector: []float32{0.1, 0.2, 0.3}}, // Empty ID should be auto-generated
	}

	err := client.UpsertPoints(context.Background(), "test-collection", points)
	require.NoError(t, err)

	// Verify ID was generated
	pointsData := requestBody["points"].([]interface{})
	point := pointsData[0].(map[string]interface{})
	assert.NotEmpty(t, point["id"])
}

func TestUpsertPoints_ServerError(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid vector dimensions"})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	points := []Point{
		{ID: "point-1", Vector: []float32{0.1}},
	}

	err := client.UpsertPoints(context.Background(), "test-collection", points)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to upsert points")
}

func TestDeletePoints_Success(t *testing.T) {
	var requestBody map[string]interface{}
	var requestPath string

	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		requestPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &requestBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": map[string]string{"status": "completed"}})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	err := client.DeletePoints(context.Background(), "test-collection", []string{"point-1", "point-2"})
	require.NoError(t, err)

	assert.Equal(t, "/collections/test-collection/points/delete", requestPath)
	points := requestBody["points"].([]interface{})
	assert.Len(t, points, 2)
}

func TestDeletePoints_EmptySlice(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		t.Error("Should not make request for empty IDs")
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	err := client.DeletePoints(context.Background(), "test-collection", []string{})
	require.NoError(t, err)
}

func TestDeletePoints_ServerError(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	err := client.DeletePoints(context.Background(), "test-collection", []string{"nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete points")
}

func TestGetPoint_Success(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"id":      "point-1",
				"vector":  []float32{0.1, 0.2, 0.3},
				"payload": map[string]interface{}{"key": "value"},
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	point, err := client.GetPoint(context.Background(), "test-collection", "point-1")
	require.NoError(t, err)
	assert.Equal(t, "point-1", point.ID)
	assert.Equal(t, "value", point.Payload["key"])
}

func TestGetPoint_NotFound(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	point, err := client.GetPoint(context.Background(), "test-collection", "nonexistent")
	require.Error(t, err)
	assert.Nil(t, point)
}

func TestGetPoint_InvalidJSON(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	point, err := client.GetPoint(context.Background(), "test-collection", "point-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
	assert.Nil(t, point)
}

func TestGetPoints_Success(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": []map[string]interface{}{
				{"id": "point-1", "vector": []float32{0.1, 0.2}, "payload": map[string]interface{}{"key": "value1"}},
				{"id": "point-2", "vector": []float32{0.3, 0.4}, "payload": map[string]interface{}{"key": "value2"}},
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	points, err := client.GetPoints(context.Background(), "test-collection", []string{"point-1", "point-2"})
	require.NoError(t, err)
	assert.Len(t, points, 2)
	assert.Equal(t, "point-1", points[0].ID)
	assert.Equal(t, "point-2", points[1].ID)
}

func TestGetPoints_ServerError(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	points, err := client.GetPoints(context.Background(), "test-collection", []string{"point-1"})
	require.Error(t, err)
	assert.Nil(t, points)
}

func TestGetPoints_InvalidJSON(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	points, err := client.GetPoints(context.Background(), "test-collection", []string{"point-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
	assert.Nil(t, points)
}

// =============================================================================
// Search Tests
// =============================================================================

func TestSearch_Success(t *testing.T) {
	var requestBody map[string]interface{}

	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &requestBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": []map[string]interface{}{
				{"id": "point-1", "score": 0.95, "payload": map[string]interface{}{"key": "value1"}},
				{"id": "point-2", "score": 0.85, "payload": map[string]interface{}{"key": "value2"}},
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	vector := []float32{0.1, 0.2, 0.3}
	results, err := client.Search(context.Background(), "test-collection", vector, nil)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "point-1", results[0].ID)
	assert.Equal(t, float32(0.95), results[0].Score)
}

func TestSearch_WithOptions(t *testing.T) {
	var requestBody map[string]interface{}

	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &requestBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": []map[string]interface{}{},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	vector := []float32{0.1, 0.2, 0.3}
	opts := DefaultSearchOptions().
		WithLimit(20).
		WithOffset(5).
		WithScoreThreshold(0.7).
		WithVectorsEnabled().
		WithFilter(map[string]interface{}{"must": []interface{}{}})

	_, err := client.Search(context.Background(), "test-collection", vector, opts)
	require.NoError(t, err)

	assert.Equal(t, float64(20), requestBody["limit"])
	assert.Equal(t, float64(5), requestBody["offset"])
	assert.Equal(t, 0.7, requestBody["score_threshold"])
	assert.Equal(t, true, requestBody["with_vector"])
	assert.NotNil(t, requestBody["filter"])
}

func TestSearch_ServerError(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	vector := []float32{0.1, 0.2, 0.3}
	results, err := client.Search(context.Background(), "test-collection", vector, nil)
	require.Error(t, err)
	assert.Nil(t, results)
}

func TestSearch_InvalidJSON(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	vector := []float32{0.1, 0.2, 0.3}
	results, err := client.Search(context.Background(), "test-collection", vector, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
	assert.Nil(t, results)
}

func TestSearchBatch_Success_NewFormat(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		// New format (Qdrant 1.16+): result is array of arrays
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": [][]map[string]interface{}{
				{
					{"id": "point-1", "score": 0.95},
					{"id": "point-2", "score": 0.85},
				},
				{
					{"id": "point-3", "score": 0.75},
				},
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	vectors := [][]float32{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
	}
	results, err := client.SearchBatch(context.Background(), "test-collection", vectors, nil)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Len(t, results[0], 2)
	assert.Len(t, results[1], 1)
}

func TestSearchBatch_Success_OldFormat(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		// Old format: result is array of objects with result field
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": []map[string]interface{}{
				{
					"result": []map[string]interface{}{
						{"id": "point-1", "score": 0.95},
					},
				},
				{
					"result": []map[string]interface{}{
						{"id": "point-2", "score": 0.85},
					},
				},
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	vectors := [][]float32{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
	}
	results, err := client.SearchBatch(context.Background(), "test-collection", vectors, nil)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestSearchBatch_WithOptions(t *testing.T) {
	var requestBody map[string]interface{}

	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &requestBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": [][]map[string]interface{}{{}},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	vectors := [][]float32{{0.1, 0.2}}
	opts := DefaultSearchOptions().
		WithLimit(15).
		WithScoreThreshold(0.5).
		WithFilter(map[string]interface{}{"must": []interface{}{}})

	_, err := client.SearchBatch(context.Background(), "test-collection", vectors, opts)
	require.NoError(t, err)

	searches := requestBody["searches"].([]interface{})
	search := searches[0].(map[string]interface{})
	assert.Equal(t, float64(15), search["limit"])
	assert.Equal(t, 0.5, search["score_threshold"])
	assert.NotNil(t, search["filter"])
}

func TestSearchBatch_ServerError(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	vectors := [][]float32{{0.1, 0.2}}
	results, err := client.SearchBatch(context.Background(), "test-collection", vectors, nil)
	require.Error(t, err)
	assert.Nil(t, results)
}

// =============================================================================
// Scroll Tests
// =============================================================================

func TestScroll_Success(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		nextOffset := "next-offset-id"
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"next_page_offset": nextOffset,
				"points": []map[string]interface{}{
					{"id": "point-1", "vector": []float32{0.1, 0.2}, "payload": map[string]interface{}{"key": "value1"}},
					{"id": "point-2", "vector": []float32{0.3, 0.4}, "payload": map[string]interface{}{"key": "value2"}},
				},
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	points, nextOffset, err := client.Scroll(context.Background(), "test-collection", 10, nil, nil)
	require.NoError(t, err)
	assert.Len(t, points, 2)
	assert.NotNil(t, nextOffset)
	assert.Equal(t, "next-offset-id", *nextOffset)
}

func TestScroll_WithOffset(t *testing.T) {
	var requestBody map[string]interface{}

	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &requestBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"points": []map[string]interface{}{},
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	offset := "some-offset"
	_, _, err := client.Scroll(context.Background(), "test-collection", 10, &offset, nil)
	require.NoError(t, err)

	assert.Equal(t, "some-offset", requestBody["offset"])
}

func TestScroll_WithFilter(t *testing.T) {
	var requestBody map[string]interface{}

	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &requestBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"points": []map[string]interface{}{},
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	filter := map[string]interface{}{"must": []interface{}{}}
	_, _, err := client.Scroll(context.Background(), "test-collection", 10, nil, filter)
	require.NoError(t, err)

	assert.NotNil(t, requestBody["filter"])
}

func TestScroll_ServerError(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	points, nextOffset, err := client.Scroll(context.Background(), "test-collection", 10, nil, nil)
	require.Error(t, err)
	assert.Nil(t, points)
	assert.Nil(t, nextOffset)
}

func TestScroll_InvalidJSON(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	points, nextOffset, err := client.Scroll(context.Background(), "test-collection", 10, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
	assert.Nil(t, points)
	assert.Nil(t, nextOffset)
}

// =============================================================================
// Count Tests
// =============================================================================

func TestCountPoints_Success(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"count": 1000,
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	count, err := client.CountPoints(context.Background(), "test-collection", nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), count)
}

func TestCountPoints_WithFilter(t *testing.T) {
	var requestBody map[string]interface{}

	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &requestBody)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"count": 500,
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	filter := map[string]interface{}{"must": []interface{}{}}
	count, err := client.CountPoints(context.Background(), "test-collection", filter)
	require.NoError(t, err)
	assert.Equal(t, int64(500), count)
	assert.NotNil(t, requestBody["filter"])
}

func TestCountPoints_ServerError(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	count, err := client.CountPoints(context.Background(), "test-collection", nil)
	require.Error(t, err)
	assert.Equal(t, int64(0), count)
}

func TestCountPoints_InvalidJSON(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	count, err := client.CountPoints(context.Background(), "test-collection", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
	assert.Equal(t, int64(0), count)
}

// =============================================================================
// Snapshot Tests
// =============================================================================

func TestCreateSnapshot_Success(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"name": "snapshot-2024-01-15",
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	name, err := client.CreateSnapshot(context.Background(), "test-collection")
	require.NoError(t, err)
	assert.Equal(t, "snapshot-2024-01-15", name)
}

func TestCreateSnapshot_ServerError(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	name, err := client.CreateSnapshot(context.Background(), "test-collection")
	require.Error(t, err)
	assert.Empty(t, name)
}

func TestCreateSnapshot_InvalidJSON(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	name, err := client.CreateSnapshot(context.Background(), "test-collection")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
	assert.Empty(t, name)
}

// =============================================================================
// Metrics Tests
// =============================================================================

func TestGetMetrics_Success(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"app": map[string]interface{}{
					"version": "1.7.0",
				},
				"requests": map[string]interface{}{
					"rest": map[string]interface{}{
						"responses": map[string]interface{}{
							"success": 1000,
						},
					},
				},
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	metrics, err := client.GetMetrics(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, metrics)
	assert.NotNil(t, metrics["app"])
}

func TestGetMetrics_ServerError(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	metrics, err := client.GetMetrics(context.Background())
	require.Error(t, err)
	assert.Nil(t, metrics)
}

func TestGetMetrics_InvalidJSON(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	metrics, err := client.GetMetrics(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
	assert.Nil(t, metrics)
}

// =============================================================================
// WaitForCollection Tests
// =============================================================================

func TestWaitForCollection_Success(t *testing.T) {
	callCount := 0
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		callCount++
		status := "yellow"
		if callCount >= 3 {
			status = "green"
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"status": status,
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	err := client.WaitForCollection(context.Background(), "test-collection", 5*time.Second)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, callCount, 3)
}

func TestWaitForCollection_Timeout(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"status": "yellow", // Never becomes green
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	err := client.WaitForCollection(context.Background(), "test-collection", 1*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestWaitForCollection_ContextCancelled(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"status": "yellow",
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	err := client.WaitForCollection(ctx, "test-collection", 10*time.Second)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestWaitForCollection_CollectionNotFound(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	err := client.WaitForCollection(context.Background(), "nonexistent", 2*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

// =============================================================================
// Edge Cases and Error Handling
// =============================================================================

func TestDoRequest_ContentTypeHeader(t *testing.T) {
	var contentType string

	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		contentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"collections": []interface{}{},
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	_, _ = client.ListCollections(context.Background())
	assert.Equal(t, "application/json", contentType)
}

func TestConcurrentAccess(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"collections": []interface{}{},
			},
		})
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	// Run concurrent operations
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = client.ListCollections(context.Background())
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we get here without panic, the test passes
	assert.True(t, client.IsConnected())
}

func TestScoredPoint_Fields(t *testing.T) {
	point := ScoredPoint{
		ID:      "test-id",
		Version: 1,
		Score:   0.95,
		Payload: map[string]interface{}{"key": "value"},
		Vector:  []float32{0.1, 0.2, 0.3},
	}

	assert.Equal(t, "test-id", point.ID)
	assert.Equal(t, 1, point.Version)
	assert.Equal(t, float32(0.95), point.Score)
	assert.Equal(t, "value", point.Payload["key"])
	assert.Len(t, point.Vector, 3)
}

func TestSearchBatch_InvalidJSON(t *testing.T) {
	ms := newMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	})
	defer ms.close()

	client := ms.newConnectedClient(t)

	vectors := [][]float32{{0.1, 0.2}}
	results, err := client.SearchBatch(context.Background(), "test-collection", vectors, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
	assert.Nil(t, results)
}
