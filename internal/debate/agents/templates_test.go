package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/debate/topology"
)

// =============================================================================
// Template Tests
// =============================================================================

func TestNewTemplateRegistry(t *testing.T) {
	t.Parallel()
	registry := NewTemplateRegistry()
	assert.NotNil(t, registry)

	// Should have built-in templates
	templates := registry.GetAll()
	assert.NotEmpty(t, templates)
}

func TestTemplateRegistry_BuiltInTemplates(t *testing.T) {
	t.Parallel()
	registry := NewTemplateRegistry()

	// Check domain specialist templates
	expectedTemplates := []string{
		"code-specialist",
		"security-specialist",
		"architecture-specialist",
		"debug-specialist",
		"optimization-specialist",
		"reasoning-specialist",
	}

	for _, id := range expectedTemplates {
		template, ok := registry.Get(id)
		assert.True(t, ok, "Expected template %s", id)
		assert.NotEmpty(t, template.Name)
		assert.NotEmpty(t, template.Domain)
	}

	// Check role templates
	roleTemplates := []string{
		"role-proposer",
		"role-critic",
		"role-reviewer",
		"role-moderator",
		"role-validator",
		"role-red-team",
	}

	for _, id := range roleTemplates {
		template, ok := registry.Get(id)
		assert.True(t, ok, "Expected role template %s", id)
		assert.NotEmpty(t, template.PreferredRoles)
	}
}

func TestTemplateRegistry_Register(t *testing.T) {
	t.Parallel()
	registry := NewTemplateRegistry()

	customTemplate := &AgentTemplate{
		TemplateID:     "custom-template",
		Name:           "Custom Agent",
		Domain:         DomainGeneral,
		ExpertiseLevel: 0.8,
	}

	err := registry.Register(customTemplate)
	assert.NoError(t, err)

	template, ok := registry.Get("custom-template")
	assert.True(t, ok)
	assert.Equal(t, "Custom Agent", template.Name)
}

func TestTemplateRegistry_Register_EmptyID(t *testing.T) {
	t.Parallel()
	registry := NewTemplateRegistry()

	customTemplate := &AgentTemplate{
		TemplateID: "",
		Name:       "Invalid",
	}

	err := registry.Register(customTemplate)
	assert.Error(t, err)
}

func TestTemplateRegistry_GetByDomain(t *testing.T) {
	t.Parallel()
	registry := NewTemplateRegistry()

	codeTemplates := registry.GetByDomain(DomainCode)
	assert.NotEmpty(t, codeTemplates)

	for _, template := range codeTemplates {
		assert.Equal(t, DomainCode, template.Domain)
	}
}

func TestTemplateRegistry_GetByRole(t *testing.T) {
	t.Parallel()
	registry := NewTemplateRegistry()

	criticTemplates := registry.GetByRole(topology.RoleCritic)
	assert.NotEmpty(t, criticTemplates)

	for _, template := range criticTemplates {
		found := false
		for _, role := range template.PreferredRoles {
			if role == topology.RoleCritic {
				found = true
				break
			}
		}
		assert.True(t, found, "Template should have RoleCritic as preferred")
	}
}

func TestTemplateRegistry_CreateAgent(t *testing.T) {
	t.Parallel()
	registry := NewTemplateRegistry()

	agent, err := registry.CreateAgent("code-specialist", "claude", "claude-3")
	require.NoError(t, err)
	require.NotNil(t, agent)

	assert.Equal(t, "Code Specialist", agent.Name)
	assert.Equal(t, "claude", agent.Provider)
	assert.Equal(t, "claude-3", agent.Model)
	assert.Equal(t, DomainCode, agent.Specialization.PrimaryDomain)
}

func TestTemplateRegistry_CreateAgent_NotFound(t *testing.T) {
	t.Parallel()
	registry := NewTemplateRegistry()

	_, err := registry.CreateAgent("nonexistent", "claude", "claude-3")
	assert.Error(t, err)
}

// =============================================================================
// Template Creation Tests
// =============================================================================

func TestAgentTemplate_CreateAgent(t *testing.T) {
	t.Parallel()
	template := NewCodeSpecialistTemplate()

	agent, err := template.CreateAgent("deepseek", "deepseek-coder")
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Check basic properties
	assert.Equal(t, template.Name, agent.Name)
	assert.Equal(t, template.Description, agent.Description)
	assert.Equal(t, "deepseek", agent.Provider)
	assert.Equal(t, "deepseek-coder", agent.Model)

	// Check specialization
	assert.Equal(t, template.Domain, agent.Specialization.PrimaryDomain)
	assert.Equal(t, template.ExpertiseLevel, agent.Specialization.ExpertiseLevel)

	// Check capabilities were set
	for _, capType := range template.RequiredCapabilities {
		assert.True(t, agent.Capabilities.HasCapability(capType, 0.5),
			"Should have capability %s", capType)
	}

	// Check system prompt was generated
	assert.NotEmpty(t, agent.SystemPrompt)
	assert.Contains(t, agent.SystemPrompt, "deepseek")

	// Check template metadata
	assert.Equal(t, template.TemplateID, agent.Metadata["template_id"])
}

func TestAgentTemplate_GenerateSystemPrompt(t *testing.T) {
	t.Parallel()
	template := NewCodeSpecialistTemplate()

	prompt := template.GenerateSystemPrompt("claude", "claude-3-opus")

	assert.Contains(t, prompt, "Code Specialist")
	assert.Contains(t, prompt, "claude")
	assert.Contains(t, prompt, "claude-3-opus")
	assert.Contains(t, prompt, "code")
}

// =============================================================================
// Domain Specialist Template Tests
// =============================================================================

func TestNewCodeSpecialistTemplate(t *testing.T) {
	t.Parallel()
	template := NewCodeSpecialistTemplate()

	assert.Equal(t, "code-specialist", template.TemplateID)
	assert.Equal(t, DomainCode, template.Domain)
	assert.Contains(t, template.RequiredCapabilities, CapCodeAnalysis)
	assert.Contains(t, template.PreferredRoles, topology.RoleProposer)
	assert.NotEmpty(t, template.SystemPromptTemplate)
}

func TestNewSecuritySpecialistTemplate(t *testing.T) {
	t.Parallel()
	template := NewSecuritySpecialistTemplate()

	assert.Equal(t, "security-specialist", template.TemplateID)
	assert.Equal(t, DomainSecurity, template.Domain)
	assert.Contains(t, template.RequiredCapabilities, CapVulnerabilityDetection)
	assert.Contains(t, template.PreferredRoles, topology.RoleCritic)
}

func TestNewArchitectureSpecialistTemplate(t *testing.T) {
	t.Parallel()
	template := NewArchitectureSpecialistTemplate()

	assert.Equal(t, "architecture-specialist", template.TemplateID)
	assert.Equal(t, DomainArchitecture, template.Domain)
	assert.Contains(t, template.RequiredCapabilities, CapSystemDesign)
	assert.Contains(t, template.PreferredRoles, topology.RoleArchitect)
}

func TestNewDebugSpecialistTemplate(t *testing.T) {
	t.Parallel()
	template := NewDebugSpecialistTemplate()

	assert.Equal(t, "debug-specialist", template.TemplateID)
	assert.Equal(t, DomainDebug, template.Domain)
	assert.Contains(t, template.RequiredCapabilities, CapErrorDiagnosis)
}

func TestNewOptimizationSpecialistTemplate(t *testing.T) {
	t.Parallel()
	template := NewOptimizationSpecialistTemplate()

	assert.Equal(t, "optimization-specialist", template.TemplateID)
	assert.Equal(t, DomainOptimization, template.Domain)
	assert.Contains(t, template.RequiredCapabilities, CapPerformanceAnalysis)
	assert.Contains(t, template.PreferredRoles, topology.RoleOptimizer)
}

func TestNewReasoningSpecialistTemplate(t *testing.T) {
	t.Parallel()
	template := NewReasoningSpecialistTemplate()

	assert.Equal(t, "reasoning-specialist", template.TemplateID)
	assert.Equal(t, DomainReasoning, template.Domain)
	assert.Contains(t, template.RequiredCapabilities, CapLogicalReasoning)
	assert.Contains(t, template.PreferredRoles, topology.RoleModerator)
}

// =============================================================================
// Role Template Tests
// =============================================================================

func TestNewProposerTemplate(t *testing.T) {
	t.Parallel()
	template := NewProposerTemplate()

	assert.Equal(t, "role-proposer", template.TemplateID)
	assert.Contains(t, template.PreferredRoles, topology.RoleProposer)
}

func TestNewCriticTemplate(t *testing.T) {
	t.Parallel()
	template := NewCriticTemplate()

	assert.Equal(t, "role-critic", template.TemplateID)
	assert.Contains(t, template.PreferredRoles, topology.RoleCritic)
}

func TestNewReviewerTemplate(t *testing.T) {
	t.Parallel()
	template := NewReviewerTemplate()

	assert.Equal(t, "role-reviewer", template.TemplateID)
	assert.Contains(t, template.PreferredRoles, topology.RoleReviewer)
}

func TestNewModeratorTemplate(t *testing.T) {
	t.Parallel()
	template := NewModeratorTemplate()

	assert.Equal(t, "role-moderator", template.TemplateID)
	assert.Contains(t, template.PreferredRoles, topology.RoleModerator)
}

func TestNewValidatorTemplate(t *testing.T) {
	t.Parallel()
	template := NewValidatorTemplate()

	assert.Equal(t, "role-validator", template.TemplateID)
	assert.Contains(t, template.PreferredRoles, topology.RoleValidator)
}

func TestNewRedTeamTemplate(t *testing.T) {
	t.Parallel()
	template := NewRedTeamTemplate()

	assert.Equal(t, "role-red-team", template.TemplateID)
	assert.Equal(t, DomainSecurity, template.Domain)
	assert.Contains(t, template.PreferredRoles, topology.RoleRedTeam)
}

// =============================================================================
// Template Integration Tests
// =============================================================================

func TestAllTemplates_CreateValidAgents(t *testing.T) {
	t.Parallel()
	registry := NewTemplateRegistry()
	templates := registry.GetAll()

	for _, template := range templates {
		t.Run(template.TemplateID, func(t *testing.T) {
				t.Parallel()
			agent, err := template.CreateAgent("test-provider", "test-model")
			require.NoError(t, err)
			require.NotNil(t, agent)

			// Basic validation
			assert.NotEmpty(t, agent.ID)
			assert.NotEmpty(t, agent.Name)
			assert.NotEmpty(t, agent.Provider)
			assert.NotEmpty(t, agent.Model)
			assert.NotNil(t, agent.Capabilities)
			assert.NotNil(t, agent.Specialization)
			assert.NotEmpty(t, agent.RoleAffinities)
		})
	}
}

func TestAllTemplates_HaveValidRoleAffinities(t *testing.T) {
	t.Parallel()
	registry := NewTemplateRegistry()
	templates := registry.GetAll()

	for _, template := range templates {
		t.Run(template.TemplateID, func(t *testing.T) {
				t.Parallel()
			agent, err := template.CreateAgent("test", "model")
			require.NoError(t, err)

			// All agents should have role affinities
			assert.NotEmpty(t, agent.RoleAffinities)

			// All affinities should be valid
			for _, affinity := range agent.RoleAffinities {
				assert.GreaterOrEqual(t, affinity.Affinity, 0.0)
				assert.LessOrEqual(t, affinity.Affinity, 1.0)
			}
		})
	}
}
