// Package agents provides tests for the agent registry
package agents

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// mockAgent is a mock implementation of AgentIntegration for testing
type mockAgent struct {
	info      AgentInfo
	available bool
	started   bool
	healthy   bool
}

func (m *mockAgent) Info() AgentInfo {
	return m.info
}

func (m *mockAgent) Initialize(ctx context.Context, config interface{}) error {
	return nil
}

func (m *mockAgent) Start(ctx context.Context) error {
	m.started = true
	return nil
}

func (m *mockAgent) Stop(ctx context.Context) error {
	m.started = false
	return nil
}

func (m *mockAgent) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !m.started {
		return nil, errors.New("agent not started")
	}
	return map[string]string{"result": "success"}, nil
}

func (m *mockAgent) Health(ctx context.Context) error {
	if !m.healthy {
		return errors.New("unhealthy")
	}
	return nil
}

func (m *mockAgent) IsAvailable() bool {
	return m.available
}

func newMockAgent(agentType AgentType, name string, available bool) *mockAgent {
	return &mockAgent{
		info: AgentInfo{
			Type:         agentType,
			Name:         name,
			Description:  "Mock agent for testing",
			Vendor:       "Test",
			Version:      "1.0.0",
			Capabilities: []string{"test"},
			IsEnabled:    true,
			Priority:     1,
		},
		available: available,
		healthy:   true,
	}
}

func TestRegistry(t *testing.T) {
	t.Parallel()
	t.Run("NewRegistry", func(t *testing.T) {
		r := NewRegistry()
		if r == nil {
			t.Fatal("NewRegistry() = nil")
		}

		agents := r.List()
		if len(agents) != 0 {
			t.Errorf("New registry has %d agents, want 0", len(agents))
		}
	})

	t.Run("Register", func(t *testing.T) {
		t.Parallel()
		r := NewRegistry()
		agent := newMockAgent("test1", "Test Agent 1", true)

		err := r.Register(agent)
		if err != nil {
			t.Errorf("Register() error = %v", err)
		}

		// Verify agent was registered
		agents := r.List()
		if len(agents) != 1 {
			t.Errorf("Registry has %d agents, want 1", len(agents))
		}
	})

	t.Run("RegisterDuplicate", func(t *testing.T) {
		t.Parallel()
		r := NewRegistry()
		agent := newMockAgent("test1", "Test Agent 1", true)

		_ = r.Register(agent)
		err := r.Register(agent)

		if err == nil {
			t.Error("Register() duplicate = nil, want error")
		}
	})

	t.Run("Get", func(t *testing.T) {
		t.Parallel()
		r := NewRegistry()
		agent := newMockAgent("test1", "Test Agent 1", true)

		_ = r.Register(agent)

		got, ok := r.Get("test1")
		if !ok {
			t.Error("Get() = (_, false), want (_, true)")
		}
		if got.Info().Name != "Test Agent 1" {
			t.Errorf("Get().Name = %q, want %q", got.Info().Name, "Test Agent 1")
		}
	})

	t.Run("GetNotFound", func(t *testing.T) {
		t.Parallel()
		r := NewRegistry()

		_, ok := r.Get("nonexistent")
		if ok {
			t.Error("Get() = (_, true), want (_, false)")
		}
	})

	t.Run("List", func(t *testing.T) {
		t.Parallel()
		r := NewRegistry()

		// Register multiple agents
		agents := []AgentIntegration{
			newMockAgent("agent1", "Agent 1", true),
			newMockAgent("agent2", "Agent 2", false),
			newMockAgent("agent3", "Agent 3", true),
		}

		for _, agent := range agents {
			if err := r.Register(agent); err != nil {
				t.Fatalf("Register() error = %v", err)
			}
		}

		list := r.List()
		if len(list) != 3 {
			t.Errorf("List() = %d agents, want 3", len(list))
		}
	})

	t.Run("ListAvailable", func(t *testing.T) {
		t.Parallel()
		r := NewRegistry()

		// Register agents with different availability
		_ = r.Register(newMockAgent("available1", "Available 1", true))
		_ = r.Register(newMockAgent("unavailable", "Unavailable", false))
		_ = r.Register(newMockAgent("available2", "Available 2", true))

		available := r.ListAvailable()
		if len(available) != 2 {
			t.Errorf("ListAvailable() = %d agents, want 2", len(available))
		}
	})
}

func TestRegistryStartStop(t *testing.T) {
	t.Parallel()
	t.Run("StartAll", func(t *testing.T) {
		r := NewRegistry()
		agent1 := newMockAgent("agent1", "Agent 1", true)
		agent2 := newMockAgent("agent2", "Agent 2", true)

		_ = r.Register(agent1)
		_ = r.Register(agent2)

		ctx := context.Background()
		errs := r.StartAll(ctx)

		if len(errs) != 0 {
			t.Errorf("StartAll() errors = %v, want none", errs)
		}

		if !agent1.started {
			t.Error("Agent 1 not started")
		}
		if !agent2.started {
			t.Error("Agent 2 not started")
		}
	})

	t.Run("StopAll", func(t *testing.T) {
		t.Parallel()
		r := NewRegistry()
		agent := newMockAgent("agent1", "Agent 1", true)

		_ = r.Register(agent)
		_ = agent.Start(context.Background())

		ctx := context.Background()
		errs := r.StopAll(ctx)

		if len(errs) != 0 {
			t.Errorf("StopAll() errors = %v, want none", errs)
		}

		if agent.started {
			t.Error("Agent still started after StopAll")
		}
	})

	t.Run("Execute", func(t *testing.T) {
		t.Parallel()
		r := NewRegistry()
		agent := newMockAgent("agent1", "Agent 1", true)

		_ = r.Register(agent)
		_ = agent.Start(context.Background())

		ctx := context.Background()
		result, err := r.Execute(ctx, "agent1", "test", map[string]interface{}{})

		if err != nil {
			t.Errorf("Execute() error = %v", err)
		}

		if result == nil {
			t.Error("Execute() result = nil")
		}
	})

	t.Run("ExecuteNotFound", func(t *testing.T) {
		t.Parallel()
		r := NewRegistry()

		ctx := context.Background()
		_, err := r.Execute(ctx, "nonexistent", "test", map[string]interface{}{})

		if err == nil {
			t.Error("Execute() = nil, want error")
		}
	})
}

func TestRegistryHealth(t *testing.T) {
	t.Parallel()
	t.Run("HealthCheck", func(t *testing.T) {
		r := NewRegistry()

		healthy := newMockAgent("healthy", "Healthy", true)
		unhealthy := newMockAgent("unhealthy", "Unhealthy", true)
		unhealthy.healthy = false

		_ = r.Register(healthy)
		_ = r.Register(unhealthy)

		ctx := context.Background()
		results := r.HealthCheck(ctx)

		if len(results) != 2 {
			t.Errorf("HealthCheck() = %d results, want 2", len(results))
		}

		if results["healthy"] != nil {
			t.Error("Healthy agent reported error")
		}

		if results["unhealthy"] == nil {
			t.Error("Unhealthy agent reported no error")
		}
	})
}

func TestRegistryStats(t *testing.T) {
	t.Parallel()
	t.Run("GetStats", func(t *testing.T) {
		r := NewRegistry()

		_ = r.Register(newMockAgent("a1", "Agent 1", true))
		_ = r.Register(newMockAgent("a2", "Agent 2", false))
		_ = r.Register(newMockAgent("a3", "Agent 3", true))

		stats := r.GetStats()

		if stats["total"] != 3 {
			t.Errorf("Stats[total] = %v, want 3", stats["total"])
		}

		if stats["available"] != 2 {
			t.Errorf("Stats[available] = %v, want 2", stats["available"])
		}
	})
}

func TestGlobalRegistry(t *testing.T) {
	t.Parallel()
	t.Run("GetGlobalRegistry", func(t *testing.T) {
		r1 := GetGlobalRegistry()
		r2 := GetGlobalRegistry()

		if r1 != r2 {
			t.Error("GetGlobalRegistry() returned different instances")
		}
	})
}

func TestAllAgentTypes(t *testing.T) {
	t.Parallel()
	types := AllAgentTypes()

	if len(types) == 0 {
		t.Error("AllAgentTypes() returned empty slice")
	}

	// Check for duplicates
	seen := make(map[AgentType]bool)
	for _, at := range types {
		if seen[at] {
			t.Errorf("Duplicate agent type: %s", at)
		}
		seen[at] = true
	}
}

func TestRegistryConcurrency(t *testing.T) {
	t.Parallel()
	r := NewRegistry()

	// Concurrent registrations
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			agentType := AgentType(string(rune('a' + i)))
			agent := newMockAgent(agentType, "Agent", true)
			_ = r.Register(agent)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent registrations")
		}
	}

	// Verify all agents were registered
	agents := r.List()
	if len(agents) != 10 {
		t.Errorf("Registry has %d agents, want 10", len(agents))
	}
}

func BenchmarkRegistryRegister(b *testing.B) {
	r := NewRegistry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agentType := AgentType(fmt.Sprintf("agent%d", i))
		agent := newMockAgent(agentType, "Agent", true)
		_ = r.Register(agent)
	}
}

func BenchmarkRegistryGet(b *testing.B) {
	r := NewRegistry()
	agent := newMockAgent("test", "Test", true)
	_ = r.Register(agent)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.Get("test")
	}
}

func BenchmarkRegistryList(b *testing.B) {
	r := NewRegistry()

	// Register 100 agents
	for i := 0; i < 100; i++ {
		agent := newMockAgent(AgentType(string(rune(i))), "Agent", true)
		_ = r.Register(agent)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.List()
	}
}
