package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// D-1 (CONST-036 / BLUFF-002) RED→GREEN polarity test for CompletionHandler.Models
//
// §11.4.115 polarity switch via RED_MODE env var:
//   RED_MODE=1 (default) — reproduce-and-assert-DEFECT-PRESENT on the pre-fix
//                          artifact: the /v1/completion/models list is a STATIC
//                          hardcoded 3-model literal (deepseek-coder /
//                          claude-3-sonnet-20240229 / gemini-pro) that does NOT
//                          change when the provider registry is reconfigured.
//   RED_MODE=0           — standing GREEN regression guard (§11.4.135): the list
//                          is sourced from the provider registry / verifier
//                          capabilities (CONST-036) and DOES change with registry
//                          state; the fabricated 3-model literal is gone.
//
// Run RED baseline (captures defect on broken artifact):
//   RED_MODE=1 go test -run TestCompletionModels_D1_Polarity -v ./internal/handlers
// Run GREEN guard (after the fix lands):
//   RED_MODE=0 go test -run TestCompletionModels_D1_Polarity -v ./internal/handlers
// ---------------------------------------------------------------------------

// modelsCapProvider is a real llm.LLMProvider whose declared SupportedModels are
// fully controllable — used to prove the endpoint reflects (or ignores) registry
// state. NO mock framework: it is a concrete provider with deterministic output.
type modelsCapProvider struct {
	name   string
	models []string
}

func (p *modelsCapProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	return &models.LLMResponse{ID: "x", Content: "ok", ProviderName: p.name, CreatedAt: time.Now()}, nil
}

func (p *modelsCapProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse)
	close(ch)
	return ch, nil
}

func (p *modelsCapProvider) HealthCheck() error { return nil }

func (p *modelsCapProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{SupportedModels: p.models}
}

func (p *modelsCapProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}

// staticModelSource is a minimal ModelSource exposing exactly the registry
// surface CompletionHandler.Models must consume (ListProviders + GetProvider).
// It is reconfigurable so the test can prove the response tracks registry state.
type staticModelSource struct {
	order     []string
	providers map[string]llm.LLMProvider
}

func newStaticModelSource() *staticModelSource {
	return &staticModelSource{providers: map[string]llm.LLMProvider{}}
}

func (s *staticModelSource) add(name string, modelIDs ...string) {
	s.order = append(s.order, name)
	s.providers[name] = &modelsCapProvider{name: name, models: modelIDs}
}

func (s *staticModelSource) ListProviders() []string { return s.order }

func (s *staticModelSource) GetProvider(name string) (llm.LLMProvider, error) {
	p, ok := s.providers[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return p, nil
}

func decodeModelsBody(t *testing.T, body []byte) []string {
	t.Helper()
	var parsed struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	ids := make([]string, 0, len(parsed.Data))
	for _, d := range parsed.Data {
		ids = append(ids, d.ID)
	}
	return ids
}

// callModels issues GET against the handler's Models endpoint and returns the
// model id list from the response body.
func callModels(t *testing.T, h *CompletionHandler) []string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/v1/completion/models", h.Models)
	req := httptest.NewRequest(http.MethodGet, "/v1/completion/models", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	return decodeModelsBody(t, w.Body.Bytes())
}

// modelSourceSetter is implemented by the post-fix CompletionHandler via its
// SetModelSource method. Using an interface assertion keeps this test compiling
// against BOTH the pre-fix artifact (where the method is absent → assertion is
// false) and the post-fix artifact (assertion true), so the RED baseline runs
// honestly on the broken code.
type modelSourceSetter interface {
	SetModelSource(ModelSource)
}

func trySetModelSource(h *CompletionHandler, src ModelSource) bool {
	if s, ok := any(h).(modelSourceSetter); ok {
		s.SetModelSource(src)
		return true
	}
	return false
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func TestCompletionModels_D1_Polarity(t *testing.T) {
	// §11.4.115 polarity: RED_MODE=1 explicitly reproduces the defect on a
	// pre-fix artifact (used to capture the RED baseline). The STANDING regression
	// guard (§11.4.135) — what `go test ./...` runs by default — is GREEN-mode
	// (RED_MODE unset / "0"), asserting the defect is ABSENT on the fixed artifact.
	redMode := os.Getenv("RED_MODE")
	if redMode == "" {
		redMode = "0"
	}

	// The fabricated hardcoded literal from the BLUFF-002 defect.
	hardcoded := []string{"deepseek-coder", "claude-3-sonnet-20240229", "gemini-pro"}

	// Build a handler exactly as production wires it (RequestService from a
	// registry), then attach a reconfigurable ModelSource.
	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	rs := services.NewRequestService("weighted", ensemble, nil)
	h := NewCompletionHandler(rs)

	src := newStaticModelSource()
	src.add("alpha", "alpha-model-1", "alpha-model-2")
	// SetModelSource is the GREEN wiring; it does not exist on the pre-fix
	// artifact, so under RED_MODE=1 we must not depend on it. We detect its
	// presence reflectively via a tiny capability check.
	wired := trySetModelSource(h, src)

	got := callModels(t, h)

	if redMode == "1" {
		// RED baseline: the pre-fix artifact returns EXACTLY the hardcoded literal,
		// independent of any registry/source. Assert the defect is present.
		require.ElementsMatch(t, hardcoded, got,
			"RED baseline expects the hardcoded BLUFF-002 3-model literal on the pre-fix artifact; "+
				"got %v (if this fails, the defect may already be fixed — flip RED_MODE=0)", got)
		// And prove it is STATIC: reconfiguring the source must NOT change it.
		src.add("beta", "beta-model-9")
		got2 := callModels(t, h)
		require.ElementsMatch(t, hardcoded, got2,
			"RED baseline expects the list to stay the static hardcoded literal even after the registry changes")
		require.False(t, contains(got2, "beta-model-9"),
			"RED baseline: a newly-registered provider's model must NOT appear (proves static hardcoded list)")
		return
	}

	// GREEN guard (RED_MODE=0):
	require.True(t, wired, "GREEN guard requires SetModelSource wiring to exist on the handler")

	// 1) The fabricated hardcoded ids must be GONE (unless a real provider
	//    genuinely advertises them — our source advertises none of them).
	require.False(t, contains(got, "claude-3-sonnet-20240229"),
		"GREEN: hardcoded literal 'claude-3-sonnet-20240229' must not be fabricated; got %v", got)
	require.False(t, contains(got, "gemini-pro"),
		"GREEN: hardcoded literal 'gemini-pro' must not be fabricated; got %v", got)

	// 2) The list MUST reflect the registry/verifier source.
	require.True(t, contains(got, "alpha-model-1"),
		"GREEN: model from a registered provider must be listed; got %v", got)

	// 3) The list MUST CHANGE when the registry changes (CONST-036 live-source).
	src.add("beta", "beta-model-9")
	got2 := callModels(t, h)
	require.True(t, contains(got2, "beta-model-9"),
		"GREEN: newly-registered provider's model must appear (list is live, not static); got %v", got2)
}

// TestCompletionModels_D1_HonestEmptyWhenNoSource proves the cold/source-down
// behaviour: with no registry/verifier source attached, the endpoint returns an
// HONEST EMPTY list, NEVER a fabricated "working" list (§11.4 PASS-bluff guard).
// This is a GREEN-only assertion (skipped under RED_MODE=1 since the pre-fix
// artifact cannot satisfy it).
func TestCompletionModels_D1_HonestEmptyWhenNoSource(t *testing.T) {
	if os.Getenv("RED_MODE") == "1" {
		t.Skip("SKIP-OK: ATM-SP2-D1 — GREEN-only assertion; pre-fix artifact returns the hardcoded literal here, not honest-empty")
	}
	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	rs := services.NewRequestService("weighted", ensemble, nil)
	h := NewCompletionHandler(rs) // no ModelSource attached

	got := callModels(t, h)
	require.Empty(t, got,
		"GREEN: with no registry/verifier source, the model list MUST be honestly empty, never a fabricated list; got %v", got)
}
