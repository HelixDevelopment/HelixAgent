package agents

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/debate/topology"
)

// =============================================================================
// Capability Tests
// =============================================================================

func TestNewCapabilitySet(t *testing.T) {
	t.Parallel()
	cs := NewCapabilitySet()
	assert.NotNil(t, cs)
	assert.NotNil(t, cs.Capabilities)
	assert.Empty(t, cs.Capabilities)
}

func TestCapabilitySet_Add(t *testing.T) {
	t.Parallel()
	cs := NewCapabilitySet()

	cap := &Capability{
		Type:        CapCodeAnalysis,
		Proficiency: 0.85,
		Verified:    true,
		Source:      "test",
	}

	cs.Add(cap)

	got, ok := cs.Get(CapCodeAnalysis)
	assert.True(t, ok)
	assert.Equal(t, 0.85, got.Proficiency)
}

func TestCapabilitySet_HasCapability(t *testing.T) {
	t.Parallel()
	cs := NewCapabilitySet()
	cs.Add(&Capability{
		Type:        CapCodeAnalysis,
		Proficiency: 0.8,
	})

	assert.True(t, cs.HasCapability(CapCodeAnalysis, 0.7))
	assert.True(t, cs.HasCapability(CapCodeAnalysis, 0.8))
	assert.False(t, cs.HasCapability(CapCodeAnalysis, 0.9))
	assert.False(t, cs.HasCapability(CapSecurityAudit, 0.5))
}

func TestCapabilitySet_GetByDomain(t *testing.T) {
	t.Parallel()
	cs := NewCapabilitySet()

	// Add code capabilities
	cs.Add(&Capability{Type: CapCodeAnalysis, Proficiency: 0.8})
	cs.Add(&Capability{Type: CapCodeGeneration, Proficiency: 0.7})

	// Add security capability
	cs.Add(&Capability{Type: CapVulnerabilityDetection, Proficiency: 0.9})

	codeCaps := cs.GetByDomain(DomainCode)
	assert.Len(t, codeCaps, 2)

	secCaps := cs.GetByDomain(DomainSecurity)
	assert.Len(t, secCaps, 1)
}

func TestCapabilitySet_CalculateDomainScore(t *testing.T) {
	t.Parallel()
	cs := NewCapabilitySet()

	cs.Add(&Capability{Type: CapCodeAnalysis, Proficiency: 0.8})
	cs.Add(&Capability{Type: CapCodeGeneration, Proficiency: 0.6})

	score := cs.CalculateDomainScore(DomainCode)
	assert.InDelta(t, 0.7, score, 0.01) // (0.8 + 0.6) / 2

	// Empty domain should return 0
	emptyScore := cs.CalculateDomainScore(DomainDebug)
	assert.Equal(t, 0.0, emptyScore)
}

// =============================================================================
// Specialization Tests
// =============================================================================

func TestNewSpecializedAgent(t *testing.T) {
	t.Parallel()
	agent := NewSpecializedAgent("Test Agent", "claude", "claude-3", DomainCode)

	assert.NotEmpty(t, agent.ID)
	assert.Equal(t, "Test Agent", agent.Name)
	assert.Equal(t, "claude", agent.Provider)
	assert.Equal(t, "claude-3", agent.Model)
	assert.Equal(t, DomainCode, agent.Specialization.PrimaryDomain)
	assert.NotNil(t, agent.Capabilities)
	assert.NotEmpty(t, agent.RoleAffinities)
}

func TestSpecializedAgent_InitDefaultCapabilities(t *testing.T) {
	t.Parallel()
	agent := NewSpecializedAgent("Code Agent", "test", "test-model", DomainCode)

	// Should have code capabilities
	assert.True(t, agent.Capabilities.HasCapability(CapCodeAnalysis, 0.5))
	assert.True(t, agent.Capabilities.HasCapability(CapCodeGeneration, 0.5))

	// Should also have general capabilities at lower proficiency
	assert.True(t, agent.Capabilities.HasCapability(CapTextGeneration, 0.3))
}

func TestSpecializedAgent_RoleAffinities(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		domain       Domain
		expectedTop  topology.AgentRole
		expectedHigh []topology.AgentRole
	}{
		{
			domain:       DomainCode,
			expectedTop:  topology.RoleProposer,
			expectedHigh: []topology.AgentRole{topology.RoleProposer, topology.RoleReviewer},
		},
		{
			domain:       DomainSecurity,
			expectedTop:  topology.RoleCritic,
			expectedHigh: []topology.AgentRole{topology.RoleCritic, topology.RoleRedTeam},
		},
		{
			domain:       DomainArchitecture,
			expectedTop:  topology.RoleArchitect,
			expectedHigh: []topology.AgentRole{topology.RoleArchitect, topology.RoleModerator},
		},
	}

	for _, tc := range testCases {
		t.Run(string(tc.domain), func(t *testing.T) {
				t.Parallel()
			agent := NewSpecializedAgent("Test", "test", "model", tc.domain)

			// Check top role
			assert.Equal(t, tc.expectedTop, agent.GetBestRole())

			// Check high affinity roles
			highRoles := agent.GetRolesAboveThreshold(0.5)
			for _, expected := range tc.expectedHigh {
				found := false
				for _, r := range highRoles {
					if r == expected {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected role %s in high affinity roles", expected)
			}
		})
	}
}

func TestSpecializedAgent_GetAffinityForRole(t *testing.T) {
	t.Parallel()
	agent := NewSpecializedAgent("Security Agent", "test", "model", DomainSecurity)

	// Security agent should have high affinity for Critic
	// Note: affinity = base_affinity * expertise_level, so 0.95 * 0.7 = 0.665
	criticAffinity := agent.GetAffinityForRole(topology.RoleCritic)
	assert.Greater(t, criticAffinity, 0.6)

	// Lower affinity for Moderator
	moderatorAffinity := agent.GetAffinityForRole(topology.RoleModerator)
	assert.Less(t, moderatorAffinity, criticAffinity)
}

func TestSpecializedAgent_ToTopologyAgent(t *testing.T) {
	t.Parallel()
	agent := NewSpecializedAgent("Test Agent", "claude", "claude-3", DomainCode)
	agent.Score = 8.5

	topoAgent := agent.ToTopologyAgent()

	assert.Equal(t, agent.ID, topoAgent.ID)
	assert.Equal(t, agent.PrimaryRole, topoAgent.Role)
	assert.Equal(t, "claude", topoAgent.Provider)
	assert.Equal(t, "claude-3", topoAgent.Model)
	assert.Equal(t, 8.5, topoAgent.Score)
	assert.Equal(t, "code", topoAgent.Specialization)
	assert.NotEmpty(t, topoAgent.Capabilities)
}

func TestSpecializedAgent_SetScore(t *testing.T) {
	t.Parallel()
	agent := NewSpecializedAgent("Test", "test", "model", DomainCode)

	agent.SetScore(9.0)
	assert.Equal(t, 9.0, agent.Score)
}

func TestSpecializedAgent_SetSystemPrompt(t *testing.T) {
	t.Parallel()
	agent := NewSpecializedAgent("Test", "test", "model", DomainCode)

	agent.SetSystemPrompt("You are a helpful assistant.")
	assert.Equal(t, "You are a helpful assistant.", agent.SystemPrompt)
}

func TestSpecializedAgent_UpdateActivity(t *testing.T) {
	t.Parallel()
	agent := NewSpecializedAgent("Test", "test", "model", DomainCode)

	originalTime := agent.LastActive
	agent.UpdateActivity()

	assert.True(t, agent.LastActive.After(originalTime) || agent.LastActive.Equal(originalTime))
}

// =============================================================================
// Scoring Tests
// =============================================================================

func TestCalculateCompositeScore(t *testing.T) {
	t.Parallel()
	// Weights: 40% verifier, 35% domain, 25% affinity
	score := CalculateCompositeScore(0.8, 0.7, 0.9)
	expected := 0.8*0.4 + 0.7*0.35 + 0.9*0.25
	assert.InDelta(t, expected, score, 0.001)
}

func TestSpecializedAgent_ScoreAgent(t *testing.T) {
	t.Parallel()
	agent := NewSpecializedAgent("Code Agent", "claude", "claude-3", DomainCode)
	agent.Score = 8.5

	score := agent.ScoreAgent(topology.RoleProposer, DomainCode)

	assert.Equal(t, agent.ID, score.AgentID)
	assert.Equal(t, 8.5, score.VerifierScore)
	assert.Greater(t, score.DomainScore, 0.0)
	assert.Greater(t, score.RoleAffinity, 0.0)
	assert.Greater(t, score.CompositeScore, 0.0)
}

// =============================================================================
// Capability Discovery Tests
// =============================================================================

type mockCapabilityDiscoverer struct {
	capabilities []*Capability
}

func (m *mockCapabilityDiscoverer) DiscoverCapabilities(ctx context.Context, provider, model string) ([]*Capability, error) {
	return m.capabilities, nil
}

func TestSpecializedAgent_DiscoverCapabilities(t *testing.T) {
	t.Parallel()
	agent := NewSpecializedAgent("Test", "test", "model", DomainCode)

	discoverer := &mockCapabilityDiscoverer{
		capabilities: []*Capability{
			{Type: CapVulnerabilityDetection, Proficiency: 0.9},
			{Type: CapLogAnalysis, Proficiency: 0.8},
		},
	}

	err := agent.DiscoverCapabilities(context.Background(), discoverer)
	require.NoError(t, err)

	// Check new capabilities were added
	cap, ok := agent.Capabilities.Get(CapVulnerabilityDetection)
	assert.True(t, ok)
	assert.True(t, cap.Verified)
	assert.Equal(t, "runtime", cap.Source)
}

func TestSpecializedAgent_DiscoverCapabilities_NilDiscoverer(t *testing.T) {
	t.Parallel()
	agent := NewSpecializedAgent("Test", "test", "model", DomainCode)

	err := agent.DiscoverCapabilities(context.Background(), nil)
	assert.NoError(t, err)
}

// =============================================================================
// Domain Capability Mapping Tests
// =============================================================================

func TestGetDomainCapabilities(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		domain   Domain
		expected []CapabilityType
	}{
		{
			domain:   DomainCode,
			expected: []CapabilityType{CapCodeAnalysis, CapCodeGeneration},
		},
		{
			domain:   DomainSecurity,
			expected: []CapabilityType{CapVulnerabilityDetection, CapThreatModeling},
		},
		{
			domain:   DomainArchitecture,
			expected: []CapabilityType{CapSystemDesign, CapScalabilityDesign},
		},
		{
			domain:   DomainDebug,
			expected: []CapabilityType{CapErrorDiagnosis, CapStackTraceAnalysis},
		},
		{
			domain:   DomainOptimization,
			expected: []CapabilityType{CapPerformanceAnalysis, CapBenchmarking},
		},
		{
			domain:   DomainReasoning,
			expected: []CapabilityType{CapLogicalReasoning, CapProblemDecomposition},
		},
	}

	for _, tc := range testCases {
		t.Run(string(tc.domain), func(t *testing.T) {
				t.Parallel()
			caps := getDomainCapabilities(tc.domain)
			assert.NotEmpty(t, caps)

			for _, expected := range tc.expected {
				found := false
				for _, cap := range caps {
					if cap == expected {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected capability %s for domain %s", expected, tc.domain)
			}
		})
	}
}

func TestGetDomainRoleAffinities(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		domain  Domain
		topRole topology.AgentRole
	}{
		{DomainCode, topology.RoleProposer},
		{DomainSecurity, topology.RoleCritic},
		{DomainArchitecture, topology.RoleArchitect},
		{DomainDebug, topology.RoleCritic},
		{DomainOptimization, topology.RoleOptimizer},
		{DomainReasoning, topology.RoleModerator},
	}

	for _, tc := range testCases {
		t.Run(string(tc.domain), func(t *testing.T) {
				t.Parallel()
			affinities := getDomainRoleAffinities(tc.domain)
			assert.NotEmpty(t, affinities)

			// Find highest affinity role
			var topRole topology.AgentRole
			var topAffinity float64
			for role, affinity := range affinities {
				if affinity > topAffinity {
					topAffinity = affinity
					topRole = role
				}
			}

			assert.Equal(t, tc.topRole, topRole)
		})
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestSpecializedAgent_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	agent := NewSpecializedAgent("Test", "test", "model", DomainCode)

	done := make(chan bool, 4)

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			_ = agent.GetBestRole()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = agent.GetAffinityForRole(topology.RoleCritic)
		}
		done <- true
	}()

	// Concurrent writes
	go func() {
		for i := 0; i < 100; i++ {
			agent.SetScore(float64(i))
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			agent.UpdateActivity()
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		<-done
	}
}

func TestCapabilitySet_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	cs := NewCapabilitySet()

	done := make(chan bool, 3)

	// Concurrent adds
	go func() {
		for i := 0; i < 100; i++ {
			cs.Add(&Capability{
				Type:        CapCodeAnalysis,
				Proficiency: float64(i) / 100,
			})
		}
		done <- true
	}()

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			_ = cs.HasCapability(CapCodeAnalysis, 0.5)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = cs.GetByDomain(DomainCode)
		}
		done <- true
	}()

	for i := 0; i < 3; i++ {
		<-done
	}
}
