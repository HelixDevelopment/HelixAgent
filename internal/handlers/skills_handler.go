package handlers

import (
	"net/http"

	"dev.helix.agent/internal/skills"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// SkillsHandler handles skill-related HTTP requests
type SkillsHandler struct {
	integration *skills.Integration
	logger      *logrus.Logger
}

// NewSkillsHandler creates a new skills handler
func NewSkillsHandler(integration *skills.Integration) *SkillsHandler {
	return &SkillsHandler{
		integration: integration,
		logger:      logrus.New(),
	}
}

// SetLogger sets the logger for the handler
func (h *SkillsHandler) SetLogger(logger *logrus.Logger) {
	h.logger = logger
}

// SkillResponse represents a single skill in API responses
type SkillResponse struct {
	Name           string                `json:"name"`
	Description    string                `json:"description"`
	Category       string                `json:"category"`
	Tags           []string              `json:"tags,omitempty"`
	TriggerPhrases []string              `json:"trigger_phrases,omitempty"`
	Version        string                `json:"version,omitempty"`
	Author         string                `json:"author,omitempty"`
	License        string                `json:"license,omitempty"`
	Overview       string                `json:"overview,omitempty"`
	WhenToUse      string                `json:"when_to_use,omitempty"`
	Instructions   string                `json:"instructions,omitempty"`
	Examples       []skills.SkillExample `json:"examples,omitempty"`
	Prerequisites  []string              `json:"prerequisites,omitempty"`
	Outputs        []string              `json:"outputs,omitempty"`
	ErrorHandling  []skills.SkillError   `json:"error_handling,omitempty"`
	Resources      []string              `json:"resources,omitempty"`
	RelatedSkills  []string              `json:"related_skills,omitempty"`
	FilePath       string                `json:"file_path,omitempty"`
	LoadedAt       string                `json:"loaded_at,omitempty"`
	UpdatedAt      string                `json:"updated_at,omitempty"`
}

// convertSkillToResponse converts a skill to API response format
func convertSkillToResponse(skill *skills.Skill) SkillResponse {
	return SkillResponse{
		Name:           skill.Name,
		Description:    skill.Description,
		Category:       skill.Category,
		Tags:           skill.Tags,
		TriggerPhrases: skill.TriggerPhrases,
		Version:        skill.Version,
		Author:         skill.Author,
		License:        skill.License,
		Overview:       skill.Overview,
		WhenToUse:      skill.WhenToUse,
		Instructions:   skill.Instructions,
		Examples:       skill.Examples,
		Prerequisites:  skill.Prerequisites,
		Outputs:        skill.Outputs,
		ErrorHandling:  skill.ErrorHandling,
		Resources:      skill.Resources,
		RelatedSkills:  skill.RelatedSkills,
		FilePath:       skill.FilePath,
		LoadedAt:       skill.LoadedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      skill.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// ListSkillsResponse represents the list of all skills
type ListSkillsResponse struct {
	Skills []SkillResponse `json:"skills"`
	Count  int             `json:"count"`
}

// ListSkills returns all registered skills
// GET /v1/skills
func (h *SkillsHandler) ListSkills(c *gin.Context) {
	service := h.integration.GetService()
	allSkills := service.GetAllSkills()

	response := ListSkillsResponse{
		Skills: make([]SkillResponse, 0, len(allSkills)),
		Count:  len(allSkills),
	}

	for _, skill := range allSkills {
		response.Skills = append(response.Skills, convertSkillToResponse(skill))
	}

	c.JSON(http.StatusOK, response)
}

// GetSkillsByCategory returns skills filtered by category
// GET /v1/skills/:category
func (h *SkillsHandler) GetSkillsByCategory(c *gin.Context) {
	category := c.Param("category")
	service := h.integration.GetService()

	// CONST-035 §c: validate the category exists before returning skills.
	// Previously returned 200 + empty list for ANY category string —
	// caller couldn't distinguish "valid category, no skills" from
	// "invalid category". Now 404 if not in the known categories list.
	known := service.GetCategories()
	exists := false
	for _, k := range known {
		if k == category {
			exists = true
			break
		}
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error":       "category not found: " + category,
			"category":    category,
			"known_count": len(known),
		})
		return
	}

	categorySkills := service.GetSkillsByCategory(category)

	response := ListSkillsResponse{
		Skills: make([]SkillResponse, 0, len(categorySkills)),
		Count:  len(categorySkills),
	}

	for _, skill := range categorySkills {
		response.Skills = append(response.Skills, convertSkillToResponse(skill))
	}

	c.JSON(http.StatusOK, response)
}

// CategoriesResponse represents the list of skill categories
type CategoriesResponse struct {
	Categories []string `json:"categories"`
	Count      int      `json:"count"`
}

// ListCategories returns all skill categories
// GET /v1/skills/categories
func (h *SkillsHandler) ListCategories(c *gin.Context) {
	service := h.integration.GetService()
	categories := service.GetCategories()

	response := CategoriesResponse{
		Categories: categories,
		Count:      len(categories),
	}

	c.JSON(http.StatusOK, response)
}

// MatchRequest represents a request to match skills against user input
type MatchRequest struct {
	Input string `json:"input" binding:"required"`
}

// MatchResponse represents the result of skill matching
type MatchResponse struct {
	Matches []SkillMatchResponse `json:"matches"`
	Count   int                  `json:"count"`
}

// SkillMatchResponse represents a matched skill with confidence
type SkillMatchResponse struct {
	Skill          SkillResponse `json:"skill"`
	Confidence     float64       `json:"confidence"`
	MatchedTrigger string        `json:"matched_trigger"`
	MatchType      string        `json:"match_type"`
}

// MatchSkills matches skills against user input
// POST /v1/skills/match
func (h *SkillsHandler) MatchSkills(c *gin.Context) {
	var req MatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	matches, err := h.integration.GetService().FindSkills(ctx, req.Input)
	if err != nil {
		h.logger.WithError(err).Error("Failed to find skills")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "skill_matching_failed",
			"message": "Failed to match skills: " + err.Error(),
		})
		return
	}

	response := MatchResponse{
		Matches: make([]SkillMatchResponse, 0, len(matches)),
		Count:   len(matches),
	}

	for _, match := range matches {
		response.Matches = append(response.Matches, SkillMatchResponse{
			Skill:          convertSkillToResponse(match.Skill),
			Confidence:     match.Confidence,
			MatchedTrigger: match.MatchedTrigger,
			MatchType:      string(match.MatchType),
		})
	}

	c.JSON(http.StatusOK, response)
}
