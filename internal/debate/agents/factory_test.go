package agents

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/debate/topology"
)

// =============================================================================
// Agent Factory Tests
// =============================================================================

func TestNewAgentFactory(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()
	assert.NotNil(t, factory)
	assert.NotNil(t, factory.templateRegistry)
}

func TestAgentFactory_GetTemplateRegistry(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()
	registry := factory.GetTemplateRegistry()
	assert.NotNil(t, registry)

	// Should have built-in templates
	templates := registry.GetAll()
	assert.NotEmpty(t, templates)
}

func TestAgentFactory_CreateFromTemplate(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()

	agent, err := factory.CreateFromTemplate("code-specialist", "claude", "claude-3")
	require.NoError(t, err)
	require.NotNil(t, agent)

	assert.Equal(t, "Code Specialist", agent.Name)
	assert.Equal(t, DomainCode, agent.Specialization.PrimaryDomain)
}

func TestAgentFactory_CreateFromTemplate_NotFound(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()

	_, err := factory.CreateFromTemplate("nonexistent", "claude", "claude-3")
	assert.Error(t, err)
}

func TestAgentFactory_CreateForDomain(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()

	testCases := []Domain{
		DomainCode,
		DomainSecurity,
		DomainArchitecture,
	}

	for _, domain := range testCases {
		t.Run(string(domain), func(t *testing.T) {
				t.Parallel()
			agent, err := factory.CreateForDomain(domain, "test", "model")
			require.NoError(t, err)
			require.NotNil(t, agent)

			// Agent should have the correct domain
			assert.Equal(t, domain, agent.Specialization.PrimaryDomain)
		})
	}
}

func TestAgentFactory_CreateForRole(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()

	testCases := []topology.AgentRole{
		topology.RoleProposer,
		topology.RoleCritic,
		topology.RoleReviewer,
		topology.RoleModerator,
		topology.RoleValidator,
	}

	for _, role := range testCases {
		t.Run(string(role), func(t *testing.T) {
				t.Parallel()
			agent, err := factory.CreateForRole(role, "test", "model")
			require.NoError(t, err)
			require.NotNil(t, agent)

			// Role should be in agent's preferred roles or be set as primary
			assert.Equal(t, role, agent.PrimaryRole)
		})
	}
}

func TestAgentFactory_CreateWithDiscovery(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()

	discoverer := &mockCapabilityDiscoverer{
		capabilities: []*Capability{
			{Type: CapVulnerabilityDetection, Proficiency: 0.95},
		},
	}
	factory.SetCapabilityDiscoverer(discoverer)

	agent, err := factory.CreateWithDiscovery(context.Background(), "code-specialist", "claude", "claude-3")
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Should have discovered capability
	cap, ok := agent.Capabilities.Get(CapVulnerabilityDetection)
	assert.True(t, ok)
	assert.True(t, cap.Verified)
}

func TestAgentFactory_CreateDebateTeam(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()

	providers := []ProviderSpec{
		{Provider: "claude", Model: "claude-3", Score: 9.0},
		{Provider: "deepseek", Model: "deepseek-coder", Score: 8.5},
		{Provider: "gemini", Model: "gemini-pro", Score: 8.0},
	}

	agents, err := factory.CreateDebateTeam(providers)
	require.NoError(t, err)
	require.NotEmpty(t, agents)

	// Should have agents for different roles
	roles := make(map[topology.AgentRole]bool)
	for _, agent := range agents {
		roles[agent.PrimaryRole] = true
	}

	// Check key roles are filled
	assert.True(t, roles[topology.RoleProposer])
	assert.True(t, roles[topology.RoleCritic])
	assert.True(t, roles[topology.RoleReviewer])
}

func TestAgentFactory_CreateDebateTeam_Empty(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()

	_, err := factory.CreateDebateTeam([]ProviderSpec{})
	assert.Error(t, err)
}

// =============================================================================
// Agent Pool Tests
// =============================================================================

func TestNewAgentPool(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()
	pool := NewAgentPool(factory)

	assert.NotNil(t, pool)
	assert.Equal(t, 0, pool.Size())
}

func TestAgentPool_Add(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()
	pool := NewAgentPool(factory)

	agent := NewSpecializedAgent("Test", "claude", "claude-3", DomainCode)
	pool.Add(agent)

	assert.Equal(t, 1, pool.Size())

	// Should be retrievable by ID
	got, ok := pool.Get(agent.ID)
	assert.True(t, ok)
	assert.Equal(t, agent.ID, got.ID)
}

func TestAgentPool_GetByRole(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()
	pool := NewAgentPool(factory)

	// Add agents with different roles
	agent1 := NewSpecializedAgent("Agent1", "claude", "claude-3", DomainCode)
	agent1.PrimaryRole = topology.RoleProposer

	agent2 := NewSpecializedAgent("Agent2", "deepseek", "deepseek-coder", DomainSecurity)
	agent2.PrimaryRole = topology.RoleCritic

	agent3 := NewSpecializedAgent("Agent3", "gemini", "gemini-pro", DomainCode)
	agent3.PrimaryRole = topology.RoleProposer

	pool.Add(agent1)
	pool.Add(agent2)
	pool.Add(agent3)

	proposers := pool.GetByRole(topology.RoleProposer)
	assert.Len(t, proposers, 2)

	critics := pool.GetByRole(topology.RoleCritic)
	assert.Len(t, critics, 1)
}

func TestAgentPool_GetByDomain(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()
	pool := NewAgentPool(factory)

	agent1 := NewSpecializedAgent("Agent1", "claude", "claude-3", DomainCode)
	agent2 := NewSpecializedAgent("Agent2", "deepseek", "deepseek-coder", DomainSecurity)
	agent3 := NewSpecializedAgent("Agent3", "gemini", "gemini-pro", DomainCode)

	pool.Add(agent1)
	pool.Add(agent2)
	pool.Add(agent3)

	codeAgents := pool.GetByDomain(DomainCode)
	assert.Len(t, codeAgents, 2)

	securityAgents := pool.GetByDomain(DomainSecurity)
	assert.Len(t, securityAgents, 1)
}

func TestAgentPool_Remove(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()
	pool := NewAgentPool(factory)

	agent := NewSpecializedAgent("Test", "claude", "claude-3", DomainCode)
	pool.Add(agent)

	assert.Equal(t, 1, pool.Size())

	pool.Remove(agent.ID)

	assert.Equal(t, 0, pool.Size())

	_, ok := pool.Get(agent.ID)
	assert.False(t, ok)
}

func TestAgentPool_SelectBestForRole(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()
	pool := NewAgentPool(factory)

	// Add agents with different scores
	agent1 := NewSpecializedAgent("Agent1", "claude", "claude-3", DomainCode)
	agent1.Score = 9.0

	agent2 := NewSpecializedAgent("Agent2", "deepseek", "deepseek-coder", DomainCode)
	agent2.Score = 8.0

	pool.Add(agent1)
	pool.Add(agent2)

	best := pool.SelectBestForRole(topology.RoleProposer, DomainCode)
	assert.NotNil(t, best)
	assert.Equal(t, agent1.ID, best.ID) // Higher score
}

func TestAgentPool_SelectTopNForRole(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()
	pool := NewAgentPool(factory)

	for i := 0; i < 5; i++ {
		agent := NewSpecializedAgent("Agent", "test", "model", DomainCode)
		agent.Score = float64(9 - i)
		pool.Add(agent)
	}

	topAgents := pool.SelectTopNForRole(topology.RoleProposer, DomainCode, 3)
	assert.Len(t, topAgents, 3)

	// Should be in descending order by composite score
	for i := 0; i < len(topAgents)-1; i++ {
		score1 := topAgents[i].ScoreAgent(topology.RoleProposer, DomainCode)
		score2 := topAgents[i+1].ScoreAgent(topology.RoleProposer, DomainCode)
		assert.GreaterOrEqual(t, score1.CompositeScore, score2.CompositeScore)
	}
}

func TestAgentPool_ToTopologyAgents(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()
	pool := NewAgentPool(factory)

	agent := NewSpecializedAgent("Test", "claude", "claude-3", DomainCode)
	pool.Add(agent)

	topoAgents := pool.ToTopologyAgents()
	assert.Len(t, topoAgents, 1)
	assert.Equal(t, agent.ID, topoAgents[0].ID)
}

// =============================================================================
// Team Builder Tests
// =============================================================================

func TestNewTeamBuilder(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()
	pool := NewAgentPool(factory)
	builder := NewTeamBuilder(pool)

	assert.NotNil(t, builder)
}

func TestDefaultTeamConfig(t *testing.T) {
	t.Parallel()
	config := DefaultTeamConfig()

	assert.NotEmpty(t, config.RequiredRoles)
	assert.NotEmpty(t, config.PreferredDomains)
	assert.Equal(t, 1, config.MinAgentsPerRole)
	assert.False(t, config.AllowRoleSharing)
}

func TestTeamBuilder_BuildTeam(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()
	pool := NewAgentPool(factory)

	// Add agents for different domains
	domains := []Domain{DomainCode, DomainSecurity, DomainArchitecture, DomainOptimization, DomainReasoning}
	for i, domain := range domains {
		agent := NewSpecializedAgent("Agent", "test", "model", domain)
		agent.Score = float64(9 - i)
		pool.Add(agent)
	}

	builder := NewTeamBuilder(pool)
	config := DefaultTeamConfig()

	assignments, err := builder.BuildTeam(config)
	require.NoError(t, err)
	require.NotEmpty(t, assignments)

	// Check all required roles are filled
	filledRoles := make(map[topology.AgentRole]bool)
	for _, assignment := range assignments {
		filledRoles[assignment.Role] = true
		assert.NotNil(t, assignment.Agent)
		assert.NotNil(t, assignment.Score)
	}

	for _, role := range config.RequiredRoles {
		assert.True(t, filledRoles[role], "Role %s should be filled", role)
	}
}

func TestTeamBuilder_BuildTeam_NoSuitable(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()
	pool := NewAgentPool(factory) // Empty pool

	builder := NewTeamBuilder(pool)

	_, err := builder.BuildTeam(nil)
	assert.Error(t, err)
}

func TestTeamBuilder_BuildTeamTopologyAgents(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()
	pool := NewAgentPool(factory)

	// Add agents for all required domains (including those needed for DefaultTeamConfig)
	// DefaultTeamConfig requires: Code, Security, General, Optimization, Reasoning
	domains := []Domain{DomainCode, DomainSecurity, DomainGeneral, DomainOptimization, DomainReasoning}
	for i := 0; i < 10; i++ {
		domain := domains[i%len(domains)]
		agent := NewSpecializedAgent("Agent", "test", "model", domain)
		agent.Score = float64(9 - i%len(domains))
		pool.Add(agent)
	}

	builder := NewTeamBuilder(pool)
	config := DefaultTeamConfig()

	topoAgents, err := builder.BuildTeamTopologyAgents(config)
	require.NoError(t, err)
	require.NotEmpty(t, topoAgents)

	// All should be valid topology agents
	for _, agent := range topoAgents {
		assert.NotEmpty(t, agent.ID)
		assert.NotEmpty(t, agent.Role)
	}
}

func TestTeamBuilder_BuildTeam_AllowRoleSharing(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()
	pool := NewAgentPool(factory)

	// Add only one agent
	agent := NewSpecializedAgent("Solo", "claude", "claude-3", DomainGeneral)
	agent.Score = 9.0
	pool.Add(agent)

	builder := NewTeamBuilder(pool)
	config := &TeamConfig{
		RequiredRoles: []topology.AgentRole{
			topology.RoleProposer, topology.RoleCritic,
		},
		PreferredDomains: map[topology.AgentRole]Domain{},
		MinAgentsPerRole: 1,
		AllowRoleSharing: true, // Allow same agent for multiple roles
	}

	assignments, err := builder.BuildTeam(config)
	require.NoError(t, err)
	assert.Len(t, assignments, 2)

	// Both assignments should use the same agent
	assert.Equal(t, assignments[0].Agent.ID, assignments[1].Agent.ID)
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestFullWorkflow_CreateAndBuildTeam(t *testing.T) {
	t.Parallel()
	// 1. Create factory
	factory := NewAgentFactory()

	// 2. Create agents from different templates
	codeAgent, err := factory.CreateFromTemplate("code-specialist", "claude", "claude-3")
	require.NoError(t, err)
	codeAgent.Score = 9.0

	securityAgent, err := factory.CreateFromTemplate("security-specialist", "gemini", "gemini-pro")
	require.NoError(t, err)
	securityAgent.Score = 8.5

	archAgent, err := factory.CreateFromTemplate("architecture-specialist", "qwen", "qwen-max")
	require.NoError(t, err)
	archAgent.Score = 8.0

	reasoningAgent, err := factory.CreateFromTemplate("reasoning-specialist", "deepseek", "deepseek-chat")
	require.NoError(t, err)
	reasoningAgent.Score = 8.2

	optimizationAgent, err := factory.CreateFromTemplate("optimization-specialist", "mistral", "mistral-large")
	require.NoError(t, err)
	optimizationAgent.Score = 7.8

	// 3. Add to pool
	pool := NewAgentPool(factory)
	pool.Add(codeAgent)
	pool.Add(securityAgent)
	pool.Add(archAgent)
	pool.Add(reasoningAgent)
	pool.Add(optimizationAgent)

	// 4. Build team
	builder := NewTeamBuilder(pool)
	assignments, err := builder.BuildTeam(DefaultTeamConfig())
	require.NoError(t, err)
	require.NotEmpty(t, assignments)

	// 5. Verify assignments are optimal
	for _, assignment := range assignments {
		assert.NotNil(t, assignment.Agent)
		assert.NotNil(t, assignment.Score)

		// Primary assignment should have highest score for that role
		if assignment.IsPrimary {
			assert.Greater(t, assignment.Score.CompositeScore, 0.0)
		}
	}

	// 6. Convert to topology agents
	topoAgents, err := builder.BuildTeamTopologyAgents(DefaultTeamConfig())
	require.NoError(t, err)

	// 7. Verify we have agents for all required roles
	filledRoles := make(map[topology.AgentRole]bool)
	for _, agent := range topoAgents {
		filledRoles[agent.Role] = true
	}

	for _, role := range DefaultTeamConfig().RequiredRoles {
		assert.True(t, filledRoles[role], "Required role %s should be filled", role)
	}
}

func TestConcurrentPoolAccess(t *testing.T) {
	t.Parallel()
	factory := NewAgentFactory()
	pool := NewAgentPool(factory)

	done := make(chan bool, 4)

	// Concurrent adds
	go func() {
		for i := 0; i < 50; i++ {
			agent := NewSpecializedAgent("Agent", "test", "model", DomainCode)
			pool.Add(agent)
		}
		done <- true
	}()

	// Concurrent reads
	go func() {
		for i := 0; i < 50; i++ {
			_ = pool.GetByRole(topology.RoleProposer)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 50; i++ {
			_ = pool.GetByDomain(DomainCode)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 50; i++ {
			_ = pool.ToTopologyAgents()
		}
		done <- true
	}()

	for i := 0; i < 4; i++ {
		<-done
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkNewAgentFactory(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewAgentFactory()
	}
}

func BenchmarkAgentFactory_CreateFromTemplate(b *testing.B) {
	factory := NewAgentFactory()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		factory.CreateFromTemplate("code-specialist", "claude", "claude-3") //nolint:errcheck
	}
}

func BenchmarkAgentPool_Add(b *testing.B) {
	factory := NewAgentFactory()
	pool := NewAgentPool(factory)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent := NewSpecializedAgent("Agent", "test", "model", DomainCode)
		pool.Add(agent)
	}
}

func BenchmarkAgentPool_SelectBestForRole(b *testing.B) {
	factory := NewAgentFactory()
	pool := NewAgentPool(factory)

	// Pre-populate pool with agents
	for i := 0; i < 20; i++ {
		agent := NewSpecializedAgent("Agent", "test", "model", DomainCode)
		agent.Score = float64(i)
		pool.Add(agent)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pool.SelectBestForRole(topology.RoleProposer, DomainCode)
	}
}

func BenchmarkAgentPool_GetByDomain(b *testing.B) {
	factory := NewAgentFactory()
	pool := NewAgentPool(factory)

	domains := []Domain{DomainCode, DomainSecurity, DomainArchitecture}
	for i := 0; i < 30; i++ {
		agent := NewSpecializedAgent("Agent", "test", "model", domains[i%3])
		pool.Add(agent)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pool.GetByDomain(DomainCode)
	}
}
