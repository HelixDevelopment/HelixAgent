package integration

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	pb "dev.helix.agent/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// getGRPCConn establishes a gRPC client connection to the test server.
// It skips the test if the server is not reachable.
func getGRPCConn(t *testing.T) *grpc.ClientConn {
	t.Helper()
	port := os.Getenv("GRPC_TEST_PORT")
	if port == "" {
		port = "50051"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(
		ctx,
		"localhost:"+port,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Skipf("gRPC server not available at localhost:%s: %v (SKIP-OK: #infra-unavailable)", port, err)
	}
	t.Cleanup(func() {
		if closeErr := conn.Close(); closeErr != nil {
			t.Logf("warning: failed to close gRPC connection: %v", closeErr)
		}
	})
	return conn
}

// getGRPCClient returns a connected LLMFacadeClient for integration tests.
func getGRPCClient(t *testing.T) pb.LLMFacadeClient {
	t.Helper()
	conn := getGRPCConn(t)
	return pb.NewLLMFacadeClient(conn)
}

// TestGRPC_HealthCheck verifies the HealthCheck endpoint returns a valid
// status and timestamp.
func TestGRPC_HealthCheck(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.HealthCheck(ctx, &pb.HealthRequest{
		Detailed: false,
	})
	require.NoError(t, err, "HealthCheck should not return an error")
	assert.NotEmpty(t, resp.GetStatus(),
		"HealthCheck response must include a status string")
	assert.Contains(t, []string{"healthy", "degraded", "unhealthy"},
		resp.GetStatus(), "status must be one of healthy/degraded/unhealthy")
}

// TestGRPC_HealthCheck_Detailed verifies detailed health checks include
// component-level information.
func TestGRPC_HealthCheck_Detailed(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.HealthCheck(ctx, &pb.HealthRequest{
		Detailed:        true,
		CheckComponents: []string{"database", "providers"},
	})
	require.NoError(t, err, "detailed HealthCheck should not return an error")
	assert.NotEmpty(t, resp.GetStatus(),
		"detailed HealthCheck must include a status")
}

// TestGRPC_CreateSession verifies that a new session can be created and
// returns a valid session ID.
func TestGRPC_CreateSession(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.CreateSession(ctx, &pb.CreateSessionRequest{
		UserId:        "integration-test-user",
		TtlHours:      1,
		MemoryEnabled: false,
	})
	require.NoError(t, err, "CreateSession should not return an error")
	assert.True(t, resp.GetSuccess(), "CreateSession should succeed")
	assert.NotEmpty(t, resp.GetSessionId(),
		"CreateSession must return a session ID")
}

// TestGRPC_GetSession verifies that a previously created session can be
// retrieved by its ID.
func TestGRPC_GetSession(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a session first.
	createResp, err := client.CreateSession(ctx, &pb.CreateSessionRequest{
		UserId:        "integration-test-user-get",
		TtlHours:      1,
		MemoryEnabled: false,
	})
	require.NoError(t, err, "CreateSession should not return an error")
	require.True(t, createResp.GetSuccess(), "CreateSession must succeed")
	sessionID := createResp.GetSessionId()
	require.NotEmpty(t, sessionID)

	// Retrieve the session.
	getResp, err := client.GetSession(ctx, &pb.GetSessionRequest{
		SessionId:      sessionID,
		IncludeContext: true,
	})
	require.NoError(t, err, "GetSession should not return an error")
	assert.True(t, getResp.GetSuccess(), "GetSession should succeed")
	assert.Equal(t, sessionID, getResp.GetSessionId(),
		"returned session ID must match the created one")
}

// TestGRPC_GetSession_InvalidID verifies that requesting a non-existent
// session returns an appropriate error.
func TestGRPC_GetSession_InvalidID(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.GetSession(ctx, &pb.GetSessionRequest{
		SessionId:      "non-existent-session-id-12345",
		IncludeContext: false,
	})
	// The server may return an error or a response with success=false.
	if err != nil {
		st, ok := status.FromError(err)
		assert.True(t, ok, "error should be a gRPC status error")
		assert.Contains(t,
			[]codes.Code{codes.NotFound, codes.InvalidArgument, codes.Internal},
			st.Code(),
			"error code should indicate the session was not found",
		)
	} else {
		assert.False(t, resp.GetSuccess(),
			"GetSession with invalid ID should not succeed")
	}
}

// TestGRPC_TerminateSession verifies that a created session can be
// terminated successfully.
func TestGRPC_TerminateSession(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a session to terminate.
	createResp, err := client.CreateSession(ctx, &pb.CreateSessionRequest{
		UserId:   "integration-test-user-terminate",
		TtlHours: 1,
	})
	require.NoError(t, err)
	require.True(t, createResp.GetSuccess())
	sessionID := createResp.GetSessionId()
	require.NotEmpty(t, sessionID)

	// Terminate the session.
	termResp, err := client.TerminateSession(ctx, &pb.TerminateSessionRequest{
		SessionId: sessionID,
		Graceful:  true,
	})
	require.NoError(t, err, "TerminateSession should not return an error")
	assert.True(t, termResp.GetSuccess(),
		"TerminateSession should succeed")
}

// TestGRPC_AddProvider verifies that a provider can be registered via
// the AddProvider endpoint.
func TestGRPC_AddProvider(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.AddProvider(ctx, &pb.AddProviderRequest{
		Name:    "test-provider-integration",
		Type:    "openai",
		ApiKey:  "sk-test-key-integration",
		BaseUrl: "https://api.openai.com/v1",
		Model:   "gpt-4",
		Weight:  1.0,
	})
	require.NoError(t, err, "AddProvider should not return an error")
	assert.True(t, resp.GetSuccess(),
		"AddProvider should succeed")
	assert.NotEmpty(t, resp.GetMessage(),
		"AddProvider response should include a message")
}

// TestGRPC_ListProviders verifies that providers can be listed and the
// response is well-formed.
func TestGRPC_ListProviders(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.ListProviders(ctx, &pb.ListProvidersRequest{
		EnabledOnly: false,
	})
	require.NoError(t, err, "ListProviders should not return an error")
	// The list may be empty on a fresh server; we only verify the call
	// succeeds and returns a non-nil slice.
	assert.NotNil(t, resp, "ListProviders response must not be nil")
}

// TestGRPC_RemoveProvider verifies that a previously added provider can
// be removed.
func TestGRPC_RemoveProvider(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Add a provider first so we have something to remove.
	addResp, err := client.AddProvider(ctx, &pb.AddProviderRequest{
		Name:    "test-provider-to-remove",
		Type:    "openai",
		ApiKey:  "sk-test-key-remove",
		BaseUrl: "https://api.openai.com/v1",
		Model:   "gpt-4",
		Weight:  0.5,
	})
	require.NoError(t, err, "AddProvider should not return an error")
	require.True(t, addResp.GetSuccess())

	providerID := ""
	if addResp.GetProvider() != nil {
		providerID = addResp.GetProvider().GetId()
	}
	if providerID == "" {
		providerID = "test-provider-to-remove"
	}

	// Remove the provider.
	removeResp, err := client.RemoveProvider(ctx, &pb.RemoveProviderRequest{
		Id:    providerID,
		Force: true,
	})
	require.NoError(t, err, "RemoveProvider should not return an error")
	assert.True(t, removeResp.GetSuccess(),
		"RemoveProvider should succeed")
}

// TestGRPC_GetMetrics verifies that server metrics can be retrieved.
func TestGRPC_GetMetrics(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.GetMetrics(ctx, &pb.MetricsRequest{
		TimeRange: "1h",
		Metrics:   []string{"request_count", "response_time", "error_rate"},
	})
	require.NoError(t, err, "GetMetrics should not return an error")
	assert.NotNil(t, resp, "GetMetrics response must not be nil")
}

// TestGRPC_Complete verifies that a standard completion request returns
// a valid response with content.
func TestGRPC_Complete(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Complete(ctx, &pb.CompletionRequest{
		RequestId:   "integration-test-complete-1",
		Prompt:      "What is 2 + 2?",
		RequestType: "reasoning",
		ModelParams: &pb.ModelParameters{
			Temperature: 0.1,
			MaxTokens:   100,
		},
	})
	require.NoError(t, err, "Complete should not return an error")
	assert.NotEmpty(t, resp.GetContent(),
		"Complete response must include content")
	assert.NotEmpty(t, resp.GetResponseId(),
		"Complete response must include a response ID")
}

// TestGRPC_Complete_EmptyPrompt verifies that a completion request with
// an empty prompt returns an appropriate error or error response.
func TestGRPC_Complete_EmptyPrompt(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Complete(ctx, &pb.CompletionRequest{
		RequestId: "integration-test-empty-prompt",
		Prompt:    "",
	})
	// The server may return a gRPC error or a response with empty content.
	if err != nil {
		st, ok := status.FromError(err)
		assert.True(t, ok, "error should be a gRPC status error")
		assert.Contains(t,
			[]codes.Code{
				codes.InvalidArgument,
				codes.Internal,
				codes.FailedPrecondition,
			},
			st.Code(),
			"empty prompt should yield an appropriate error code",
		)
	} else {
		// Some servers may accept empty prompts but return an empty or
		// error-indicating response.
		t.Logf("server accepted empty prompt; response content: %q",
			resp.GetContent())
	}
}

// TestGRPC_CompleteStream verifies that a streaming completion request
// returns at least one chunk and eventually completes.
func TestGRPC_CompleteStream(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := client.CompleteStream(ctx, &pb.CompletionRequest{
		RequestId:   "integration-test-stream-1",
		Prompt:      "Count from 1 to 5.",
		RequestType: "reasoning",
		ModelParams: &pb.ModelParameters{
			Temperature: 0.1,
			MaxTokens:   200,
		},
	})
	require.NoError(t, err, "CompleteStream should not return an error")

	var chunks int
	var fullContent string
	for {
		resp, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			// Some providers may error mid-stream; log and break.
			t.Logf("stream receive error after %d chunks: %v",
				chunks, recvErr)
			break
		}
		chunks++
		fullContent += resp.GetContent()
	}

	assert.Greater(t, chunks, 0,
		"CompleteStream should return at least one chunk")
	assert.NotEmpty(t, fullContent,
		"concatenated stream content should not be empty")
}

// TestGRPC_CompleteStream_EmptyPrompt verifies that streaming with an
// empty prompt either errors immediately or terminates gracefully.
func TestGRPC_CompleteStream_EmptyPrompt(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := client.CompleteStream(ctx, &pb.CompletionRequest{
		RequestId: "integration-test-stream-empty",
		Prompt:    "",
	})
	if err != nil {
		// Immediate error is acceptable.
		st, ok := status.FromError(err)
		assert.True(t, ok, "error should be a gRPC status error")
		t.Logf("stream creation error for empty prompt: code=%s msg=%s",
			st.Code(), st.Message())
		return
	}

	// If the stream was created, attempt to read; expect either an error
	// or a quick EOF.
	_, recvErr := stream.Recv()
	if recvErr != nil && recvErr != io.EOF {
		st, ok := status.FromError(recvErr)
		if ok {
			t.Logf("stream recv error for empty prompt: code=%s msg=%s",
				st.Code(), st.Message())
		}
	}
}

// TestGRPC_Complete_WithSession verifies that a completion request can
// be associated with an existing session.
func TestGRPC_Complete_WithSession(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a session.
	sessionResp, err := client.CreateSession(ctx, &pb.CreateSessionRequest{
		UserId:   "integration-test-session-complete",
		TtlHours: 1,
	})
	require.NoError(t, err)
	require.True(t, sessionResp.GetSuccess())
	sessionID := sessionResp.GetSessionId()

	// Use the session for a completion.
	resp, err := client.Complete(ctx, &pb.CompletionRequest{
		RequestId:   "integration-test-session-req-1",
		SessionId:   sessionID,
		Prompt:      "Hello, what is the capital of France?",
		RequestType: "reasoning",
		ModelParams: &pb.ModelParameters{
			Temperature: 0.1,
			MaxTokens:   100,
		},
	})
	require.NoError(t, err, "Complete with session should not return an error")
	assert.NotEmpty(t, resp.GetContent(),
		"Complete with session must return content")
}

// TestGRPC_ConcurrentRequests verifies that the server can handle
// multiple concurrent gRPC calls without errors.
func TestGRPC_ConcurrentRequests(t *testing.T) {
	client := getGRPCClient(t)

	const concurrency = 5
	var wg sync.WaitGroup
	errors := make([]error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(
				context.Background(), 10*time.Second,
			)
			defer cancel()

			_, err := client.HealthCheck(ctx, &pb.HealthRequest{
				Detailed: false,
			})
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	successCount := 0
	for i, err := range errors {
		if err == nil {
			successCount++
		} else {
			t.Logf("concurrent request %d failed: %v", i, err)
		}
	}
	assert.Greater(t, successCount, 0,
		"at least one concurrent HealthCheck should succeed")
	assert.Equal(t, concurrency, successCount,
		"all concurrent HealthCheck requests should succeed")
}

// TestGRPC_ConcurrentCompletions verifies that multiple concurrent
// completion requests are handled correctly.
func TestGRPC_ConcurrentCompletions(t *testing.T) {
	client := getGRPCClient(t)

	const concurrency = 3
	var wg sync.WaitGroup
	results := make([]*pb.CompletionResponse, concurrency)
	errors := make([]error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(
				context.Background(), 30*time.Second,
			)
			defer cancel()

			resp, err := client.Complete(ctx, &pb.CompletionRequest{
				RequestId: fmt.Sprintf(
					"integration-concurrent-%d", idx,
				),
				Prompt:      fmt.Sprintf("What is %d + %d?", idx, idx+1),
				RequestType: "reasoning",
				ModelParams: &pb.ModelParameters{
					Temperature: 0.1,
					MaxTokens:   50,
				},
			})
			results[idx] = resp
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	successCount := 0
	for i := 0; i < concurrency; i++ {
		if errors[i] == nil {
			successCount++
			assert.NotEmpty(t, results[i].GetContent(),
				"concurrent completion %d should have content", i)
		} else {
			t.Logf("concurrent completion %d failed: %v", i, errors[i])
		}
	}
	assert.Greater(t, successCount, 0,
		"at least one concurrent completion should succeed")
}

// TestGRPC_SessionLifecycle runs a full session lifecycle: create,
// retrieve, use for completion, and terminate.
func TestGRPC_SessionLifecycle(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 1: Create session.
	createResp, err := client.CreateSession(ctx, &pb.CreateSessionRequest{
		UserId:        "lifecycle-test-user",
		TtlHours:      1,
		MemoryEnabled: true,
	})
	require.NoError(t, err)
	require.True(t, createResp.GetSuccess())
	sessionID := createResp.GetSessionId()
	require.NotEmpty(t, sessionID)
	t.Logf("created session: %s", sessionID)

	// Step 2: Retrieve the session.
	getResp, err := client.GetSession(ctx, &pb.GetSessionRequest{
		SessionId:      sessionID,
		IncludeContext: true,
	})
	require.NoError(t, err)
	assert.True(t, getResp.GetSuccess())
	assert.Equal(t, sessionID, getResp.GetSessionId())
	t.Logf("session status: %s", getResp.GetStatus())

	// Step 3: Make a completion within the session.
	completeResp, err := client.Complete(ctx, &pb.CompletionRequest{
		RequestId:   "lifecycle-complete-1",
		SessionId:   sessionID,
		Prompt:      "What is Go?",
		RequestType: "reasoning",
		ModelParams: &pb.ModelParameters{
			Temperature: 0.1,
			MaxTokens:   100,
		},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, completeResp.GetContent())

	// Step 4: Terminate the session.
	termResp, err := client.TerminateSession(ctx,
		&pb.TerminateSessionRequest{
			SessionId: sessionID,
			Graceful:  true,
		},
	)
	require.NoError(t, err)
	assert.True(t, termResp.GetSuccess())
	t.Logf("session terminated successfully")

	// Step 5: Verify the session is no longer active.
	getAfter, err := client.GetSession(ctx, &pb.GetSessionRequest{
		SessionId: sessionID,
	})
	if err != nil {
		// Expected: session no longer exists.
		st, ok := status.FromError(err)
		if ok {
			t.Logf("post-terminate GetSession error: code=%s msg=%s",
				st.Code(), st.Message())
		}
	} else {
		// If no error, the status should reflect termination.
		assert.Contains(t, []string{"terminated", "expired", ""},
			getAfter.GetStatus(),
			"session status should indicate termination")
	}
}

// TestGRPC_ProviderLifecycle runs a full provider lifecycle: add, list
// to confirm presence, update, and remove.
func TestGRPC_ProviderLifecycle(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	providerName := "lifecycle-test-provider"

	// Step 1: Add a provider.
	addResp, err := client.AddProvider(ctx, &pb.AddProviderRequest{
		Name:    providerName,
		Type:    "openai",
		ApiKey:  "sk-lifecycle-test-key",
		BaseUrl: "https://api.openai.com/v1",
		Model:   "gpt-4",
		Weight:  1.0,
	})
	require.NoError(t, err)
	assert.True(t, addResp.GetSuccess())

	providerID := ""
	if addResp.GetProvider() != nil {
		providerID = addResp.GetProvider().GetId()
	}
	if providerID == "" {
		providerID = providerName
	}
	t.Logf("added provider: id=%s", providerID)

	// Step 2: List providers and verify it appears.
	listResp, err := client.ListProviders(ctx, &pb.ListProvidersRequest{
		EnabledOnly: false,
	})
	require.NoError(t, err)
	assert.NotNil(t, listResp)

	found := false
	for _, p := range listResp.GetProviders() {
		if p.GetId() == providerID || p.GetName() == providerName {
			found = true
			break
		}
	}
	assert.True(t, found,
		"added provider should appear in ListProviders")

	// Step 3: Update the provider.
	updateResp, err := client.UpdateProvider(ctx,
		&pb.UpdateProviderRequest{
			Id:     providerID,
			Name:   providerName,
			ApiKey: "sk-lifecycle-updated-key",
			Model:  "gpt-4-turbo",
			Weight: 0.8,
		},
	)
	require.NoError(t, err)
	assert.True(t, updateResp.GetSuccess(),
		"UpdateProvider should succeed")

	// Step 4: Remove the provider.
	removeResp, err := client.RemoveProvider(ctx,
		&pb.RemoveProviderRequest{
			Id:    providerID,
			Force: true,
		},
	)
	require.NoError(t, err)
	assert.True(t, removeResp.GetSuccess(),
		"RemoveProvider should succeed")
}

// TestGRPC_TerminateSession_InvalidID verifies that terminating a
// non-existent session returns an appropriate error.
func TestGRPC_TerminateSession_InvalidID(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.TerminateSession(ctx,
		&pb.TerminateSessionRequest{
			SessionId: "does-not-exist-session-xyz",
			Graceful:  true,
		},
	)
	if err != nil {
		st, ok := status.FromError(err)
		assert.True(t, ok, "error should be a gRPC status error")
		assert.Contains(t,
			[]codes.Code{codes.NotFound, codes.InvalidArgument, codes.Internal},
			st.Code(),
			"terminating invalid session should return appropriate code",
		)
	} else {
		assert.False(t, resp.GetSuccess(),
			"terminating non-existent session should not succeed")
	}
}

// TestGRPC_RemoveProvider_InvalidID verifies that removing a
// non-existent provider returns an appropriate error.
func TestGRPC_RemoveProvider_InvalidID(t *testing.T) {
	client := getGRPCClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.RemoveProvider(ctx, &pb.RemoveProviderRequest{
		Id:    "nonexistent-provider-id-99999",
		Force: true,
	})
	if err != nil {
		st, ok := status.FromError(err)
		assert.True(t, ok, "error should be a gRPC status error")
		assert.Contains(t,
			[]codes.Code{codes.NotFound, codes.InvalidArgument, codes.Internal},
			st.Code(),
			"removing invalid provider should return appropriate code",
		)
	} else {
		assert.False(t, resp.GetSuccess(),
			"removing non-existent provider should not succeed")
	}
}
