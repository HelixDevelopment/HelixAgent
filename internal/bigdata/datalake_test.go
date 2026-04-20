package bigdata

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockS3Server implements a minimal S3-compatible server for testing.
//
// Concurrency model (CONST-029): objects and timestamps are safe.Store
// instances; the Store's internal lock serialises all handler paths.
type mockS3Server struct {
	objects    *safe.Store[string, []byte]    // bucket/key -> content
	timestamps *safe.Store[string, time.Time] // bucket/key -> last modified time
	server     *httptest.Server
}

func newMockS3Server() *mockS3Server {
	s := &mockS3Server{
		objects:    safe.NewStore[string, []byte](),
		timestamps: safe.NewStore[string, time.Time](),
	}

	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Path format: /bucket/key or /bucket
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
		bucket := parts[0]
		key := ""
		if len(parts) > 1 {
			key = parts[1]
		}

		switch r.Method {
		case "HEAD":
			if key == "" {
				// BucketExists - always return 200
				w.WriteHeader(http.StatusOK)
			} else {
				// StatObject
				fullKey := bucket + "/" + key
				data, exists := s.objects.Get(fullKey)
				ts, hasTS := s.timestamps.Get(fullKey)
				if exists {
					w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
					w.Header().Set("Content-Type", "application/json")
					if hasTS {
						w.Header().Set("Last-Modified", ts.UTC().Format(http.TimeFormat))
					} else {
						w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
					}
					w.WriteHeader(http.StatusOK)
				} else {
					w.Header().Set("Content-Type", "application/xml")
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message></Error>`))
				}
			}

		case "PUT":
			if key == "" {
				// MakeBucket
				w.WriteHeader(http.StatusOK)
			} else {
				// PutObject
				body, err := io.ReadAll(r.Body)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				s.objects.Put(bucket+"/"+key, body)
				w.Header().Set("ETag", `"mock-etag"`)
				w.WriteHeader(http.StatusOK)
			}

		case "GET":
			if key == "" {
				// ListObjectsV2
				prefix := r.URL.Query().Get("prefix")
				type Content struct {
					Key          string `xml:"Key"`
					LastModified string `xml:"LastModified"`
					Size         int    `xml:"Size"`
				}
				type ListResult struct {
					XMLName  xml.Name  `xml:"ListBucketResult"`
					Contents []Content `xml:"Contents"`
				}
				result := ListResult{}
				for objKey, data := range s.objects.Snapshot() {
					if strings.HasPrefix(objKey, bucket+"/"+prefix) {
						actualKey := strings.TrimPrefix(objKey, bucket+"/")
						modTime := time.Now().UTC()
						if tsList, hasTsList := s.timestamps.Get(objKey); hasTsList {
							modTime = tsList.UTC()
						}
						result.Contents = append(result.Contents, Content{
							Key:          actualKey,
							LastModified: modTime.Format(time.RFC3339),
							Size:         len(data),
						})
					}
				}
				w.Header().Set("Content-Type", "application/xml")
				xmlData, _ := xml.Marshal(result)
				_, _ = w.Write(xmlData)
			} else {
				// GetObject
				fullKey := bucket + "/" + key
				data, exists := s.objects.Get(fullKey)
				tsGet, hasTSGet := s.timestamps.Get(fullKey)
				if exists {
					w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
					w.Header().Set("Content-Type", "application/json")
					if hasTSGet {
						w.Header().Set("Last-Modified", tsGet.UTC().Format(http.TimeFormat))
					} else {
						w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
					}
					_, _ = w.Write(data)
				} else {
					w.Header().Set("Content-Type", "application/xml")
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message></Error>`))
				}
			}

		case "DELETE":
			s.objects.Delete(bucket + "/" + key)
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	return s
}

func (s *mockS3Server) Close() {
	s.server.Close()
}

func (s *mockS3Server) Endpoint() string {
	return strings.TrimPrefix(s.server.URL, "http://")
}

// newTestDataLakeClient creates a DataLakeClient backed by a mock S3 server
func newTestDataLakeClient(t *testing.T) (*DataLakeClient, *mockS3Server) {
	t.Helper()
	s := newMockS3Server()

	client, err := minio.New(s.Endpoint(), &minio.Options{
		Creds:  credentials.NewStaticV4("testkey", "testsecret", ""),
		Secure: false,
	})
	require.NoError(t, err)

	dlc := &DataLakeClient{
		client:     client,
		bucketName: "test-bucket",
		logger:     logrus.New(),
	}

	return dlc, s
}

// newTestDataLakeClientFromServer creates a DataLakeClient from an existing mock S3 server
func newTestDataLakeClientFromServer(t *testing.T, s *mockS3Server) (*DataLakeClient, *mockS3Server) {
	t.Helper()

	client, err := minio.New(s.Endpoint(), &minio.Options{
		Creds:  credentials.NewStaticV4("testkey", "testsecret", ""),
		Secure: false,
	})
	require.NoError(t, err)

	dlc := &DataLakeClient{
		client:     client,
		bucketName: "test-bucket",
		logger:     logrus.New(),
	}

	return dlc, s
}

// --- DataLakeClient method tests with mock S3 ---

func TestDataLakeClient_ArchiveConversation_Success(t *testing.T) {
	dlc, s := newTestDataLakeClient(t)
	defer s.Close()

	archive := ConversationArchive{
		ConversationID: "conv-test-1",
		UserID:         "user-1",
		StartedAt:      time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
		CompletedAt:    time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC),
		MessageCount:   5,
		Messages: []ArchivedMessage{
			{MessageID: "msg-1", Role: "user", Content: "Hello", Tokens: 1},
		},
	}

	err := dlc.ArchiveConversation(context.Background(), archive)
	assert.NoError(t, err)

	// Verify the object was stored
	key := "test-bucket/conversations/year=2025/month=06/day=15/conversation_conv-test-1.json"
	data, exists := s.objects.Get(key)
	assert.True(t, exists)
	assert.NotEmpty(t, data)
}

func TestDataLakeClient_GetConversation_Success(t *testing.T) {
	dlc, s := newTestDataLakeClient(t)
	defer s.Close()

	// Pre-populate the mock S3 with a conversation
	archive := ConversationArchive{
		ConversationID: "conv-get-1",
		UserID:         "user-2",
		MessageCount:   3,
	}
	data, _ := json.Marshal(archive)
	s.objects.Put("test-bucket/conversations/year=2025/month=01/day=01/conversation_conv-get-1.json", data)

	result, err := dlc.GetConversation(
		context.Background(),
		"conv-get-1",
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	assert.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "conv-get-1", result.ConversationID)
	assert.Equal(t, "user-2", result.UserID)
	assert.Equal(t, 3, result.MessageCount)
}

func TestDataLakeClient_GetConversation_NotFound(t *testing.T) {
	dlc, s := newTestDataLakeClient(t)
	defer s.Close()

	result, err := dlc.GetConversation(
		context.Background(),
		"nonexistent",
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestDataLakeClient_PathExists_True(t *testing.T) {
	dlc, s := newTestDataLakeClient(t)
	defer s.Close()

	s.objects.Put("test-bucket/data/test-path", []byte("content"))

	exists, err := dlc.PathExists(context.Background(), "data/test-path")
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestDataLakeClient_PathExists_False(t *testing.T) {
	dlc, s := newTestDataLakeClient(t)
	defer s.Close()

	exists, err := dlc.PathExists(context.Background(), "nonexistent/path")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestDataLakeClient_DeleteConversation_Success(t *testing.T) {
	dlc, s := newTestDataLakeClient(t)
	defer s.Close()

	key := "test-bucket/conversations/year=2025/month=06/day=15/conversation_conv-del-1.json"
	s.objects.Put(key, []byte(`{"conversation_id":"conv-del-1"}`))

	err := dlc.DeleteConversation(
		context.Background(),
		"conv-del-1",
		time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
	)
	assert.NoError(t, err)
}

func TestDataLakeClient_ArchiveDebateResults_Success(t *testing.T) {
	dlc, s := newTestDataLakeClient(t)
	defer s.Close()

	results := map[string]interface{}{
		"debate_id":    "debate-1",
		"winner":       "claude",
		"total_rounds": 3,
		"confidence":   0.92,
	}

	err := dlc.ArchiveDebateResults(
		context.Background(),
		"debate-1",
		time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC),
		results,
	)
	assert.NoError(t, err)
}

func TestDataLakeClient_ArchiveEntities_Success(t *testing.T) {
	dlc, s := newTestDataLakeClient(t)
	defer s.Close()

	entities := []ArchivedEntity{
		{EntityID: "ent-1", Name: "Alice", Type: "person", Confidence: 0.95},
		{EntityID: "ent-2", Name: "Anthropic", Type: "organization", Confidence: 0.99},
	}

	err := dlc.ArchiveEntities(
		context.Background(),
		time.Date(2025, 7, 20, 0, 0, 0, 0, time.UTC),
		entities,
	)
	assert.NoError(t, err)
}

func TestDataLakeClient_DeletePath_NonRecursive(t *testing.T) {
	dlc, s := newTestDataLakeClient(t)
	defer s.Close()

	s.objects.Put("test-bucket/some/path/file.json", []byte("data"))

	err := dlc.DeletePath(context.Background(), "some/path/file.json", false)
	assert.NoError(t, err)
}

func TestDataLakeClient_DeletePath_Recursive(t *testing.T) {
	dlc, s := newTestDataLakeClient(t)
	defer s.Close()

	s.objects.Put("test-bucket/dir/file1.json", []byte("data1"))
	s.objects.Put("test-bucket/dir/file2.json", []byte("data2"))
	s.objects.Put("test-bucket/dir/subdir/file3.json", []byte("data3"))

	err := dlc.DeletePath(context.Background(), "dir", true)
	assert.NoError(t, err)
}

func TestDataLakeClient_ListConversations_Success(t *testing.T) {
	dlc, s := newTestDataLakeClient(t)
	defer s.Close()

	// Pre-populate conversations for a date range
	s.objects.Put("test-bucket/conversations/year=2025/month=01/day=01/conversation_conv-a.json", []byte("{}"))
	s.objects.Put("test-bucket/conversations/year=2025/month=01/day=01/conversation_conv-b.json", []byte("{}"))
	s.objects.Put("test-bucket/conversations/year=2025/month=01/day=02/conversation_conv-c.json", []byte("{}"))

	ids, err := dlc.ListConversations(
		context.Background(),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
	)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(ids), 2) // At least from both days
}

func TestDataLakeClient_GetStorageStats_Success(t *testing.T) {
	dlc, s := newTestDataLakeClient(t)
	defer s.Close()

	// Pre-populate various objects
	s.objects.Put("test-bucket/conversations/year=2025/month=01/day=01/conv1.json", make([]byte, 1024))
	s.objects.Put("test-bucket/conversations/year=2025/month=01/day=01/conv2.json", make([]byte, 2048))
	s.objects.Put("test-bucket/debates/year=2025/month=01/day=01/debate1.json", make([]byte, 512))

	stats, err := dlc.GetStorageStats(context.Background())
	assert.NoError(t, err)
	require.NotNil(t, stats)
}

func TestDataLakeClient_ListDirectories_Success(t *testing.T) {
	dlc, s := newTestDataLakeClient(t)
	defer s.Close()

	s.objects.Put("test-bucket/output/dir1/file.json", []byte("data1"))
	s.objects.Put("test-bucket/output/dir2/file.json", []byte("data2"))

	dirs, err := dlc.ListDirectories(context.Background(), "output/")
	assert.NoError(t, err)
	assert.NotNil(t, dirs)
}

func TestDataLakeClient_GetMetadata_AsDirectory(t *testing.T) {
	dlc, s := newTestDataLakeClient(t)
	defer s.Close()

	// When path doesn't exist, GetMetadata falls back to directory mode
	metadata, err := dlc.GetMetadata(context.Background(), "nonexistent/path")
	assert.NoError(t, err)
	require.NotNil(t, metadata)
	assert.True(t, metadata.IsDir)
	assert.Equal(t, int64(0), metadata.Size)
}

func TestDataLakeClient_GetMetadata_ExistingObject(t *testing.T) {
	dlc, s := newTestDataLakeClient(t)
	defer s.Close()

	s.objects.Put("test-bucket/data/file.json", []byte(`{"key": "value"}`))

	metadata, err := dlc.GetMetadata(context.Background(), "data/file.json")
	assert.NoError(t, err)
	require.NotNil(t, metadata)
	assert.False(t, metadata.IsDir)
}

func TestDataLakeClient_NewDataLakeClient_WithMockServer(t *testing.T) {
	s := newMockS3Server()
	defer s.Close()

	config := DataLakeConfig{
		Endpoint:        s.Endpoint(),
		AccessKeyID:     "testkey",
		SecretAccessKey: "testsecret",
		BucketName:      "test-bucket",
		Region:          "us-east-1",
		UseSSL:          false,
	}

	client, err := NewDataLakeClient(config, logrus.New())
	assert.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, "test-bucket", client.bucketName)
}

// --- NewDataLakeClient tests ---

func TestNewDataLakeClient_InvalidEndpoint(t *testing.T) {
	// An empty endpoint should fail when the MinIO client tries to connect.
	config := DataLakeConfig{
		Endpoint:        "",
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
		BucketName:      "test-bucket",
		Region:          "us-east-1",
		UseSSL:          false,
	}
	logger := logrus.New()

	client, err := NewDataLakeClient(config, logger)
	// minio.New with empty endpoint returns an error
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestNewDataLakeClient_NilLogger(t *testing.T) {
	// Verify that nil logger does not cause a panic during client creation.
	// We use an empty endpoint which fails immediately (no network call).
	config := DataLakeConfig{
		Endpoint:        "",
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
		BucketName:      "test-bucket",
		Region:          "us-east-1",
		UseSSL:          false,
	}

	// Should not panic even with nil logger
	client, err := NewDataLakeClient(config, nil)
	// minio.New with empty endpoint returns an error
	assert.Error(t, err)
	assert.Nil(t, client)
}

// --- DataLakeConfig tests ---

func TestDataLakeConfig_FieldAssignment(t *testing.T) {
	config := DataLakeConfig{
		Endpoint:        "localhost:9000",
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		BucketName:      "helixagent-datalake",
		Region:          "us-east-1",
		UseSSL:          true,
	}

	assert.Equal(t, "localhost:9000", config.Endpoint)
	assert.Equal(t, "minioadmin", config.AccessKeyID)
	assert.Equal(t, "minioadmin", config.SecretAccessKey)
	assert.Equal(t, "helixagent-datalake", config.BucketName)
	assert.Equal(t, "us-east-1", config.Region)
	assert.True(t, config.UseSSL)
}

func TestDataLakeConfig_DefaultValues(t *testing.T) {
	config := DataLakeConfig{}

	assert.Empty(t, config.Endpoint)
	assert.Empty(t, config.AccessKeyID)
	assert.Empty(t, config.SecretAccessKey)
	assert.Empty(t, config.BucketName)
	assert.Empty(t, config.Region)
	assert.False(t, config.UseSSL)
}

// --- ConversationArchive tests ---

func TestConversationArchive_JSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	archive := ConversationArchive{
		ConversationID: "conv-123",
		UserID:         "user-456",
		SessionID:      "session-789",
		StartedAt:      now,
		CompletedAt:    now.Add(5 * time.Minute),
		MessageCount:   10,
		EntityCount:    3,
		TotalTokens:    1500,
		Messages: []ArchivedMessage{
			{
				MessageID: "msg-1",
				Role:      "user",
				Content:   "Hello world",
				Model:     "",
				Tokens:    5,
				Timestamp: now,
			},
		},
		Entities: []ArchivedEntity{
			{
				EntityID:   "ent-1",
				Type:       "person",
				Name:       "Alice",
				Value:      "Alice Smith",
				Confidence: 0.95,
				FirstSeen:  now,
			},
		},
		DebateRounds: []ArchivedDebateRound{
			{
				Round:        1,
				Position:     "affirmative",
				Provider:     "claude",
				Model:        "claude-3-opus",
				Response:     "I agree because...",
				Confidence:   0.88,
				ResponseTime: 1500,
				Timestamp:    now,
			},
		},
		Metadata: map[string]interface{}{
			"topic": "AI ethics",
		},
	}

	data, err := json.Marshal(archive)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	var decoded ConversationArchive
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, archive.ConversationID, decoded.ConversationID)
	assert.Equal(t, archive.UserID, decoded.UserID)
	assert.Equal(t, archive.SessionID, decoded.SessionID)
	assert.Equal(t, archive.MessageCount, decoded.MessageCount)
	assert.Equal(t, archive.EntityCount, decoded.EntityCount)
	assert.Equal(t, archive.TotalTokens, decoded.TotalTokens)
	assert.Len(t, decoded.Messages, 1)
	assert.Len(t, decoded.Entities, 1)
	assert.Len(t, decoded.DebateRounds, 1)
}

func TestConversationArchive_EmptyFieldsSerialization(t *testing.T) {
	archive := ConversationArchive{
		ConversationID: "conv-empty",
	}

	data, err := json.Marshal(archive)
	require.NoError(t, err)

	var decoded ConversationArchive
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "conv-empty", decoded.ConversationID)
	assert.Empty(t, decoded.Messages)
	assert.Empty(t, decoded.Entities)
	assert.Nil(t, decoded.DebateRounds) // omitempty
	assert.Nil(t, decoded.Metadata)
}

// --- ArchivedMessage tests ---

func TestArchivedMessage_JSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	msg := ArchivedMessage{
		MessageID: "msg-100",
		Role:      "assistant",
		Content:   "Here is my response.",
		Model:     "gpt-4",
		Tokens:    25,
		Timestamp: now,
		Metadata: map[string]interface{}{
			"provider": "openai",
		},
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded ArchivedMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, msg.MessageID, decoded.MessageID)
	assert.Equal(t, msg.Role, decoded.Role)
	assert.Equal(t, msg.Content, decoded.Content)
	assert.Equal(t, msg.Model, decoded.Model)
	assert.Equal(t, msg.Tokens, decoded.Tokens)
}

func TestArchivedMessage_OmitsEmptyOptionalFields(t *testing.T) {
	msg := ArchivedMessage{
		MessageID: "msg-200",
		Role:      "user",
		Content:   "test",
		Tokens:    3,
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	// Model and Metadata have omitempty tags
	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	_, hasModel := raw["model"]
	_, hasMetadata := raw["metadata"]
	// model with omitempty should be omitted when empty string
	assert.False(t, hasModel, "empty model should be omitted")
	assert.False(t, hasMetadata, "nil metadata should be omitted")
}

// --- ArchivedEntity tests ---

func TestArchivedEntity_JSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	entity := ArchivedEntity{
		EntityID:   "ent-100",
		Type:       "organization",
		Name:       "Anthropic",
		Value:      "Anthropic PBC",
		Confidence: 0.99,
		FirstSeen:  now,
		Properties: map[string]interface{}{
			"industry": "AI",
		},
	}

	data, err := json.Marshal(entity)
	require.NoError(t, err)

	var decoded ArchivedEntity
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, entity.EntityID, decoded.EntityID)
	assert.Equal(t, entity.Type, decoded.Type)
	assert.Equal(t, entity.Name, decoded.Name)
	assert.Equal(t, entity.Value, decoded.Value)
	assert.InDelta(t, entity.Confidence, decoded.Confidence, 0.001)
}

func TestArchivedEntity_OmitsEmptyProperties(t *testing.T) {
	entity := ArchivedEntity{
		EntityID:   "ent-200",
		Type:       "person",
		Name:       "Bob",
		Value:      "Bob Jones",
		Confidence: 0.8,
	}

	data, err := json.Marshal(entity)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	_, hasProps := raw["properties"]
	assert.False(t, hasProps, "nil properties should be omitted")
}

// --- ArchivedDebateRound tests ---

func TestArchivedDebateRound_JSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	round := ArchivedDebateRound{
		Round:        2,
		Position:     "negative",
		Provider:     "deepseek",
		Model:        "deepseek-chat",
		Response:     "I disagree because...",
		Confidence:   0.75,
		ResponseTime: 2500,
		Timestamp:    now,
		Metadata: map[string]interface{}{
			"tokens_used": 150,
		},
	}

	data, err := json.Marshal(round)
	require.NoError(t, err)

	var decoded ArchivedDebateRound
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, round.Round, decoded.Round)
	assert.Equal(t, round.Position, decoded.Position)
	assert.Equal(t, round.Provider, decoded.Provider)
	assert.Equal(t, round.Model, decoded.Model)
	assert.Equal(t, round.Response, decoded.Response)
	assert.InDelta(t, round.Confidence, decoded.Confidence, 0.001)
	assert.Equal(t, round.ResponseTime, decoded.ResponseTime)
}

func TestArchivedDebateRound_OmitsEmptyMetadata(t *testing.T) {
	round := ArchivedDebateRound{
		Round:    1,
		Position: "affirmative",
		Provider: "gemini",
		Model:    "gemini-pro",
		Response: "Yes, because...",
	}

	data, err := json.Marshal(round)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	_, hasMetadata := raw["metadata"]
	assert.False(t, hasMetadata, "nil metadata should be omitted")
}

// --- formatPartition tests ---

func TestDataLakeClient_FormatPartition(t *testing.T) {
	// We can test formatPartition by creating a DataLakeClient with just the
	// minimum fields needed (it is a method on the struct, not on the minio client).
	dlc := &DataLakeClient{
		logger: logrus.New(),
	}

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "standard date",
			time:     time.Date(2025, 3, 15, 10, 30, 0, 0, time.UTC),
			expected: "year=2025/month=03/day=15",
		},
		{
			name:     "single digit month and day",
			time:     time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
			expected: "year=2024/month=01/day=05",
		},
		{
			name:     "end of year",
			time:     time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			expected: "year=2024/month=12/day=31",
		},
		{
			name:     "leap year date",
			time:     time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC),
			expected: "year=2024/month=02/day=29",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dlc.formatPartition(tt.time)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Close tests ---

func TestDataLakeClient_Close(t *testing.T) {
	// Close should always return nil (MinIO client doesn't need explicit close)
	dlc := &DataLakeClient{
		logger: logrus.New(),
	}

	err := dlc.Close()
	assert.NoError(t, err)
}

// --- NewDataLakeClient with valid endpoint ---

func TestNewDataLakeClient_ValidEndpointButUnreachable(t *testing.T) {
	// A valid-looking endpoint that no server is listening on.
	// MinIO client creation succeeds, but BucketExists will fail.
	config := DataLakeConfig{
		Endpoint:        "127.0.0.1:19999",
		AccessKeyID:     "testkey",
		SecretAccessKey: "testsecret",
		BucketName:      "test-bucket",
		Region:          "us-east-1",
		UseSSL:          false,
	}
	logger := logrus.New()

	client, err := NewDataLakeClient(config, logger)
	// MinIO client is created, but BucketExists fails to connect
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "failed to check bucket existence")
}

func TestNewDataLakeClient_WithSSL(t *testing.T) {
	config := DataLakeConfig{
		Endpoint:        "127.0.0.1:19998",
		AccessKeyID:     "testkey",
		SecretAccessKey: "testsecret",
		BucketName:      "ssl-bucket",
		Region:          "eu-west-1",
		UseSSL:          true,
	}
	logger := logrus.New()

	client, err := NewDataLakeClient(config, logger)
	// Should fail to connect (no server), but MinIO client creation succeeds
	assert.Error(t, err)
	assert.Nil(t, client)
}

// --- ConversationArchive large data ---

func TestConversationArchive_WithMultipleMessages(t *testing.T) {
	now := time.Now()
	messages := make([]ArchivedMessage, 100)
	for i := 0; i < 100; i++ {
		messages[i] = ArchivedMessage{
			MessageID: "msg-" + string(rune('0'+i%10)),
			Role:      "user",
			Content:   "Message content",
			Tokens:    10,
			Timestamp: now.Add(time.Duration(i) * time.Second),
		}
	}

	archive := ConversationArchive{
		ConversationID: "conv-large",
		MessageCount:   100,
		Messages:       messages,
	}

	data, err := json.Marshal(archive)
	require.NoError(t, err)

	var decoded ConversationArchive
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Messages, 100)
	assert.Equal(t, 100, decoded.MessageCount)
}

// --- Integration path generation tests ---

func TestDataLakeClient_ObjectKeyGeneration_Conversations(t *testing.T) {
	dlc := &DataLakeClient{
		logger: logrus.New(),
	}

	timestamp := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	partition := dlc.formatPartition(timestamp)
	conversationID := "abc-def-123"

	expectedKey := "conversations/year=2025/month=06/day=15/conversation_abc-def-123.json"
	actualKey := "conversations/" + partition + "/conversation_" + conversationID + ".json"

	assert.Equal(t, expectedKey, actualKey)
}

func TestDataLakeClient_ObjectKeyGeneration_Debates(t *testing.T) {
	dlc := &DataLakeClient{
		logger: logrus.New(),
	}

	timestamp := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	partition := dlc.formatPartition(timestamp)
	debateID := "debate-xyz"

	expectedKey := "debates/year=2025/month=01/day=01/debate_debate-xyz.json"
	actualKey := "debates/" + partition + "/debate_" + debateID + ".json"

	assert.Equal(t, expectedKey, actualKey)
}
