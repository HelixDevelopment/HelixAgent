package framework

import (
	"context"
	"testing"
)

// mockChallenge implements Challenge for testing.
type mockChallenge struct {
	id           ChallengeID
	name         string
	description  string
	dependencies []ChallengeID
	executeFunc  func(ctx context.Context) (*ChallengeResult, error)
}

func newMockChallenge(id ChallengeID, name string, deps []ChallengeID) *mockChallenge {
	return &mockChallenge{
		id:           id,
		name:         name,
		dependencies: deps,
	}
}

func (m *mockChallenge) ID() ChallengeID          { return m.id }
func (m *mockChallenge) Name() string             { return m.name }
func (m *mockChallenge) Description() string      { return m.description }
func (m *mockChallenge) Dependencies() []ChallengeID { return m.dependencies }
func (m *mockChallenge) Configure(config *ChallengeConfig) error { return nil }
func (m *mockChallenge) Validate(ctx context.Context) error     { return nil }
func (m *mockChallenge) Execute(ctx context.Context) (*ChallengeResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx)
	}
	return &ChallengeResult{
		ChallengeID: m.id,
		Status:      StatusPassed,
	}, nil
}
func (m *mockChallenge) Cleanup(ctx context.Context) error { return nil }

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	c := newMockChallenge("test_challenge", "Test Challenge", nil)

	// Register should succeed
	if err := r.Register(c); err != nil {
		t.Errorf("Register() failed: %v", err)
	}

	// Duplicate registration should fail
	if err := r.Register(c); err == nil {
		t.Error("Register() should fail for duplicate")
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()

	c := newMockChallenge("test_challenge", "Test Challenge", nil)
	_ = r.Register(c)

	// Get existing challenge
	got, err := r.Get("test_challenge")
	if err != nil {
		t.Errorf("Get() failed: %v", err)
	}
	if got.ID() != c.ID() {
		t.Errorf("Get() returned wrong challenge: got %s, want %s", got.ID(), c.ID())
	}

	// Get non-existing challenge
	_, err = r.Get("non_existing")
	if err == nil {
		t.Error("Get() should fail for non-existing challenge")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()

	c1 := newMockChallenge("challenge_a", "Challenge A", nil)
	c2 := newMockChallenge("challenge_b", "Challenge B", nil)

	_ = r.Register(c1)
	_ = r.Register(c2)

	list := r.List()
	if len(list) != 2 {
		t.Errorf("List() returned %d challenges, want 2", len(list))
	}

	// Should be sorted by ID
	if list[0].ID() != "challenge_a" {
		t.Errorf("List() not sorted: first is %s, want challenge_a", list[0].ID())
	}
}

func TestRegistry_GetDependencyOrder(t *testing.T) {
	r := NewRegistry()

	// Create challenges with dependencies
	// c3 depends on c2, c2 depends on c1
	c1 := newMockChallenge("challenge_1", "Challenge 1", nil)
	c2 := newMockChallenge("challenge_2", "Challenge 2", []ChallengeID{"challenge_1"})
	c3 := newMockChallenge("challenge_3", "Challenge 3", []ChallengeID{"challenge_2"})

	_ = r.Register(c1)
	_ = r.Register(c2)
	_ = r.Register(c3)

	ordered, err := r.GetDependencyOrder()
	if err != nil {
		t.Errorf("GetDependencyOrder() failed: %v", err)
	}

	if len(ordered) != 3 {
		t.Errorf("GetDependencyOrder() returned %d challenges, want 3", len(ordered))
	}

	// Verify order: c1 before c2 before c3
	idx := make(map[ChallengeID]int)
	for i, c := range ordered {
		idx[c.ID()] = i
	}

	if idx["challenge_1"] >= idx["challenge_2"] {
		t.Error("challenge_1 should come before challenge_2")
	}
	if idx["challenge_2"] >= idx["challenge_3"] {
		t.Error("challenge_2 should come before challenge_3")
	}
}

func TestRegistry_CircularDependency(t *testing.T) {
	r := NewRegistry()

	// Create circular dependency: c1 -> c2 -> c1
	c1 := newMockChallenge("challenge_1", "Challenge 1", []ChallengeID{"challenge_2"})
	c2 := newMockChallenge("challenge_2", "Challenge 2", []ChallengeID{"challenge_1"})

	_ = r.Register(c1)
	_ = r.Register(c2)

	_, err := r.GetDependencyOrder()
	if err == nil {
		t.Error("GetDependencyOrder() should fail for circular dependency")
	}
}

func TestRegistry_Count(t *testing.T) {
	r := NewRegistry()

	if r.Count() != 0 {
		t.Error("Count() should be 0 for empty registry")
	}

	_ = r.Register(newMockChallenge("c1", "C1", nil))
	_ = r.Register(newMockChallenge("c2", "C2", nil))

	if r.Count() != 2 {
		t.Errorf("Count() = %d, want 2", r.Count())
	}
}

func TestRegistry_Clear(t *testing.T) {
	r := NewRegistry()

	_ = r.Register(newMockChallenge("c1", "C1", nil))
	r.Clear()

	if r.Count() != 0 {
		t.Error("Count() should be 0 after Clear()")
	}
}
