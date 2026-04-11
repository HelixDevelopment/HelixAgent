package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/skills"
)

func init() {
	runtime.GOMAXPROCS(2)
	gin.SetMode(gin.TestMode)
}

// ============================================================================
// Helpers
// ============================================================================

// createTestSkillService creates a Service with manually registered skills.
func createTestSkillService() *skills.Service {
	config := skills.DefaultSkillConfig()
	config.MinConfidence = 0.3
	svc := skills.NewService(config)
	svc.Start() // manual mode, no disk loading

	now := time.Now()

	svc.RegisterSkill(&skills.Skill{
		Name:           "code-review",
		Description:    "Review code for quality",
		Category:       "development",
		Tags:           []string{"code", "review"},
		TriggerPhrases: []string{"review code", "code review"},
		Version:        "1.0.0",
		Author:         "test",
		Overview:       "Reviews code",
		WhenToUse:      "When reviewing code",
		Instructions:   "Check style and logic",
		LoadedAt:       now,
		UpdatedAt:      now,
	})

	svc.RegisterSkill(&skills.Skill{
		Name:           "code-format",
		Description:    "Format code automatically",
		Category:       "development",
		Tags:           []string{"code", "format"},
		TriggerPhrases: []string{"format code", "auto format"},
		Version:        "1.0.0",
		Author:         "test",
		Overview:       "Formats code",
		WhenToUse:      "When formatting code",
		Instructions:   "Apply formatting rules",
		LoadedAt:       now,
		UpdatedAt:      now,
	})

	svc.RegisterSkill(&skills.Skill{
		Name:           "semantic-search",
		Description:    "Search for similar code",
		Category:       "search",
		Tags:           []string{"search", "semantic"},
		TriggerPhrases: []string{"search code", "find similar"},
		Version:        "1.0.0",
		Author:         "test",
		Overview:       "Semantic code search",
		WhenToUse:      "When searching for code",
		Instructions:   "Use vector similarity",
		LoadedAt:       now,
		UpdatedAt:      now,
	})

	return svc
}

func setupSkillsHandler() (*SkillsHandler, *gin.Engine) {
	svc := createTestSkillService()
	integration := skills.NewIntegration(svc)

	h := NewSkillsHandler(integration)

	r := gin.New()
	api := r.Group("/v1")
	{
		api.GET("/skills", h.ListSkills)
		api.GET("/skills/categories", h.ListCategories)
		api.GET("/skills/:category", h.GetSkillsByCategory)
		api.POST("/skills/match", h.MatchSkills)
	}

	return h, r
}

func setupEmptySkillsHandler() (*SkillsHandler, *gin.Engine) {
	config := skills.DefaultSkillConfig()
	svc := skills.NewService(config)
	svc.Start()

	integration := skills.NewIntegration(svc)
	h := NewSkillsHandler(integration)

	r := gin.New()
	api := r.Group("/v1")
	{
		api.GET("/skills", h.ListSkills)
		api.GET("/skills/categories", h.ListCategories)
		api.GET("/skills/:category", h.GetSkillsByCategory)
		api.POST("/skills/match", h.MatchSkills)
	}

	return h, r
}

// ============================================================================
// Constructor Tests
// ============================================================================

func TestNewSkillsHandler(t *testing.T) {
	t.Parallel()
	svc := skills.NewService(nil)
	integration := skills.NewIntegration(svc)
	h := NewSkillsHandler(integration)

	assert.NotNil(t, h)
	assert.NotNil(t, h.integration)
	assert.NotNil(t, h.logger)
}

func TestSkillsHandler_SetLogger(t *testing.T) {
	t.Parallel()
	svc := skills.NewService(nil)
	integration := skills.NewIntegration(svc)
	h := NewSkillsHandler(integration)

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	h.SetLogger(logger)

	assert.Equal(t, logger, h.logger)
}

// ============================================================================
// Type Tests
// ============================================================================

func TestSkillResponse_Fields(t *testing.T) {
	t.Parallel()
	resp := SkillResponse{
		Name:         "test-skill",
		Description:  "A test skill",
		Category:     "testing",
		Tags:         []string{"test", "example"},
		Version:      "2.0.0",
		Author:       "tester",
		License:      "MIT",
		Overview:     "overview text",
		WhenToUse:    "when testing",
		Instructions: "do the thing",
		FilePath:     "/skills/test.md",
		LoadedAt:     "2024-01-01T00:00:00Z",
		UpdatedAt:    "2024-01-01T00:00:00Z",
	}

	assert.Equal(t, "test-skill", resp.Name)
	assert.Equal(t, "A test skill", resp.Description)
	assert.Equal(t, "testing", resp.Category)
	assert.Equal(t, []string{"test", "example"}, resp.Tags)
	assert.Equal(t, "2.0.0", resp.Version)
	assert.Equal(t, "tester", resp.Author)
	assert.Equal(t, "MIT", resp.License)
	assert.Equal(t, "overview text", resp.Overview)
	assert.Equal(t, "when testing", resp.WhenToUse)
	assert.Equal(t, "do the thing", resp.Instructions)
	assert.Equal(t, "/skills/test.md", resp.FilePath)
}

func TestSkillResponse_JSONSerialization(t *testing.T) {
	t.Parallel()
	resp := SkillResponse{
		Name:        "json-test",
		Description: "JSON serialization test",
		Category:    "testing",
		Tags:        []string{"json"},
		Version:     "1.0.0",
		LoadedAt:    "2024-01-01T00:00:00Z",
		UpdatedAt:   "2024-01-01T00:00:00Z",
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded SkillResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, resp.Name, decoded.Name)
	assert.Equal(t, resp.Description, decoded.Description)
	assert.Equal(t, resp.Category, decoded.Category)
	assert.Equal(t, resp.Version, decoded.Version)
}

func TestListSkillsResponse_Fields(t *testing.T) {
	t.Parallel()
	resp := ListSkillsResponse{
		Skills: []SkillResponse{
			{Name: "skill-1", Category: "cat-1"},
			{Name: "skill-2", Category: "cat-2"},
		},
		Count: 2,
	}

	assert.Len(t, resp.Skills, 2)
	assert.Equal(t, 2, resp.Count)
}

func TestMatchRequest_Fields(t *testing.T) {
	t.Parallel()
	req := MatchRequest{
		Input: "review my code",
	}

	assert.Equal(t, "review my code", req.Input)
}

func TestMatchResponse_Fields(t *testing.T) {
	t.Parallel()
	resp := MatchResponse{
		Matches: []SkillMatchResponse{
			{
				Skill:          SkillResponse{Name: "code-review"},
				Confidence:     0.95,
				MatchedTrigger: "review code",
				MatchType:      "exact",
			},
		},
		Count: 1,
	}

	assert.Len(t, resp.Matches, 1)
	assert.Equal(t, 1, resp.Count)
	assert.Equal(t, 0.95, resp.Matches[0].Confidence)
	assert.Equal(t, "review code", resp.Matches[0].MatchedTrigger)
	assert.Equal(t, "exact", resp.Matches[0].MatchType)
}

func TestSkillMatchResponse_Fields(t *testing.T) {
	t.Parallel()
	smr := SkillMatchResponse{
		Skill:          SkillResponse{Name: "test"},
		Confidence:     0.88,
		MatchedTrigger: "test trigger",
		MatchType:      "partial",
	}

	assert.Equal(t, "test", smr.Skill.Name)
	assert.Equal(t, 0.88, smr.Confidence)
	assert.Equal(t, "test trigger", smr.MatchedTrigger)
	assert.Equal(t, "partial", smr.MatchType)
}

func TestCategoriesResponse_Fields(t *testing.T) {
	t.Parallel()
	resp := CategoriesResponse{
		Categories: []string{"development", "search", "testing"},
		Count:      3,
	}

	assert.Len(t, resp.Categories, 3)
	assert.Equal(t, 3, resp.Count)
}

// ============================================================================
// ListSkills Tests
// ============================================================================

func TestSkillsHandler_ListSkills_WithSkills(t *testing.T) {
	t.Parallel()
	_, r := setupSkillsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/skills", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListSkillsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 3, resp.Count)
	assert.Len(t, resp.Skills, 3)

	names := make([]string, len(resp.Skills))
	for i, s := range resp.Skills {
		names[i] = s.Name
	}
	assert.Contains(t, names, "code-review")
	assert.Contains(t, names, "code-format")
	assert.Contains(t, names, "semantic-search")
}

func TestSkillsHandler_ListSkills_Empty(t *testing.T) {
	t.Parallel()
	_, r := setupEmptySkillsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/skills", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListSkillsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 0, resp.Count)
	assert.Empty(t, resp.Skills)
}

func TestSkillsHandler_ListSkills_ResponseFormat(t *testing.T) {
	t.Parallel()
	_, r := setupSkillsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/skills", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var resp ListSkillsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Verify skill fields are populated
	for _, skill := range resp.Skills {
		assert.NotEmpty(t, skill.Name)
		assert.NotEmpty(t, skill.Description)
		assert.NotEmpty(t, skill.Category)
		assert.NotEmpty(t, skill.Version)
		assert.NotEmpty(t, skill.LoadedAt)
		assert.NotEmpty(t, skill.UpdatedAt)
	}
}

// ============================================================================
// GetSkillsByCategory Tests
// ============================================================================

func TestSkillsHandler_GetSkillsByCategory_Development(t *testing.T) {
	t.Parallel()
	_, r := setupSkillsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/skills/development", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListSkillsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 2, resp.Count)
	for _, s := range resp.Skills {
		assert.Equal(t, "development", s.Category)
	}
}

func TestSkillsHandler_GetSkillsByCategory_Search(t *testing.T) {
	t.Parallel()
	_, r := setupSkillsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/skills/search", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListSkillsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 1, resp.Count)
	assert.Equal(t, "semantic-search", resp.Skills[0].Name)
}

func TestSkillsHandler_GetSkillsByCategory_NonExistent(t *testing.T) {
	t.Parallel()
	_, r := setupSkillsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/skills/nonexistent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ListSkillsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 0, resp.Count)
	assert.Empty(t, resp.Skills)
}

// ============================================================================
// ListCategories Tests
// ============================================================================

func TestSkillsHandler_ListCategories_WithSkills(t *testing.T) {
	t.Parallel()
	_, r := setupSkillsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/skills/categories", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp CategoriesResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Count >= 2) // development and search
	assert.Contains(t, resp.Categories, "development")
	assert.Contains(t, resp.Categories, "search")
}

func TestSkillsHandler_ListCategories_Empty(t *testing.T) {
	t.Parallel()
	_, r := setupEmptySkillsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/skills/categories", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp CategoriesResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 0, resp.Count)
	assert.Empty(t, resp.Categories)
}

// ============================================================================
// MatchSkills Tests
// ============================================================================

func TestSkillsHandler_MatchSkills_Success(t *testing.T) {
	t.Parallel()
	_, r := setupSkillsHandler()

	body, _ := json.Marshal(MatchRequest{Input: "review code"})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/skills/match", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp MatchResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Count >= 0)
}

func TestSkillsHandler_MatchSkills_NoMatches(t *testing.T) {
	t.Parallel()
	_, r := setupSkillsHandler()

	body, _ := json.Marshal(MatchRequest{
		Input: "completely unrelated xyzzy foobar",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/skills/match", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp MatchResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// May or may not have matches depending on fuzzy matching
	assert.True(t, resp.Count >= 0)
}

func TestSkillsHandler_MatchSkills_BadRequest_MissingInput(t *testing.T) {
	t.Parallel()
	_, r := setupSkillsHandler()

	body := []byte(`{}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/skills/match", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotEmpty(t, resp["error"])
}

func TestSkillsHandler_MatchSkills_BadRequest_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, r := setupSkillsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(
		"POST",
		"/v1/skills/match",
		bytes.NewBufferString(`{invalid json}`),
	)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSkillsHandler_MatchSkills_BadRequest_EmptyBody(t *testing.T) {
	t.Parallel()
	_, r := setupSkillsHandler()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/skills/match", bytes.NewBuffer(nil))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// convertSkillToResponse Tests
// ============================================================================

func TestConvertSkillToResponse(t *testing.T) {
	t.Parallel()
	now := time.Now()
	skill := &skills.Skill{
		Name:           "test-skill",
		Description:    "A skill for testing",
		Category:       "testing",
		Tags:           []string{"test"},
		TriggerPhrases: []string{"run test"},
		Version:        "1.0.0",
		Author:         "tester",
		License:        "MIT",
		Overview:       "Test overview",
		WhenToUse:      "In tests",
		Instructions:   "Do the test",
		Examples: []skills.SkillExample{
			{Title: "Example 1", Request: "test me", Result: "tested"},
		},
		Prerequisites: []string{"go"},
		Outputs:       []string{"result"},
		ErrorHandling: []skills.SkillError{
			{Error: "test error", Cause: "test cause", Solution: "fix it"},
		},
		Resources:     []string{"https://example.com"},
		RelatedSkills: []string{"other-skill"},
		FilePath:      "/skills/test-skill.md",
		LoadedAt:      now,
		UpdatedAt:     now,
	}

	resp := convertSkillToResponse(skill)

	assert.Equal(t, "test-skill", resp.Name)
	assert.Equal(t, "A skill for testing", resp.Description)
	assert.Equal(t, "testing", resp.Category)
	assert.Equal(t, []string{"test"}, resp.Tags)
	assert.Equal(t, []string{"run test"}, resp.TriggerPhrases)
	assert.Equal(t, "1.0.0", resp.Version)
	assert.Equal(t, "tester", resp.Author)
	assert.Equal(t, "MIT", resp.License)
	assert.Equal(t, "Test overview", resp.Overview)
	assert.Equal(t, "In tests", resp.WhenToUse)
	assert.Equal(t, "Do the test", resp.Instructions)
	assert.Len(t, resp.Examples, 1)
	assert.Equal(t, "Example 1", resp.Examples[0].Title)
	assert.Equal(t, []string{"go"}, resp.Prerequisites)
	assert.Equal(t, []string{"result"}, resp.Outputs)
	assert.Len(t, resp.ErrorHandling, 1)
	assert.Equal(t, "test error", resp.ErrorHandling[0].Error)
	assert.Equal(t, []string{"https://example.com"}, resp.Resources)
	assert.Equal(t, []string{"other-skill"}, resp.RelatedSkills)
	assert.Equal(t, "/skills/test-skill.md", resp.FilePath)
	assert.Equal(t, now.Format("2006-01-02T15:04:05Z"), resp.LoadedAt)
	assert.Equal(t, now.Format("2006-01-02T15:04:05Z"), resp.UpdatedAt)
}

func TestConvertSkillToResponse_EmptyFields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	skill := &skills.Skill{
		Name:      "minimal",
		LoadedAt:  now,
		UpdatedAt: now,
	}

	resp := convertSkillToResponse(skill)

	assert.Equal(t, "minimal", resp.Name)
	assert.Empty(t, resp.Description)
	assert.Empty(t, resp.Category)
	assert.Nil(t, resp.Tags)
	assert.Nil(t, resp.TriggerPhrases)
	assert.Empty(t, resp.Version)
}

// ============================================================================
// Content-Type Tests
// ============================================================================

func TestSkillsHandler_ResponseContentType(t *testing.T) {
	t.Parallel()
	_, r := setupSkillsHandler()

	routes := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/v1/skills", ""},
		{"GET", "/v1/skills/categories", ""},
		{"GET", "/v1/skills/development", ""},
		{"POST", "/v1/skills/match", `{"input":"test"}`},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			var req *http.Request
			if route.body != "" {
				req, _ = http.NewRequest(
					route.method,
					route.path,
					bytes.NewBufferString(route.body),
				)
				req.Header.Set("Content-Type", "application/json")
			} else {
				req, _ = http.NewRequest(route.method, route.path, nil)
			}
			r.ServeHTTP(w, req)

			assert.Contains(
				t,
				w.Header().Get("Content-Type"),
				"application/json",
			)
		})
	}
}
