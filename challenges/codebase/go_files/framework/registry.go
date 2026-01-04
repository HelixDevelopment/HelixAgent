// Package framework provides the challenge registry implementation.
package framework

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
)

// DefaultRegistry is the default challenge registry instance.
var DefaultRegistry = NewRegistry()

// Registry implements ChallengeRegistry.
type Registry struct {
	mu         sync.RWMutex
	challenges map[ChallengeID]Challenge
	definitions map[ChallengeID]*ChallengeDefinition
}

// NewRegistry creates a new challenge registry.
func NewRegistry() *Registry {
	return &Registry{
		challenges:  make(map[ChallengeID]Challenge),
		definitions: make(map[ChallengeID]*ChallengeDefinition),
	}
}

// Register adds a challenge to the registry.
func (r *Registry) Register(challenge Challenge) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := challenge.ID()
	if _, exists := r.challenges[id]; exists {
		return fmt.Errorf("challenge already registered: %s", id)
	}

	r.challenges[id] = challenge
	return nil
}

// RegisterDefinition adds a challenge definition to the registry.
func (r *Registry) RegisterDefinition(def *ChallengeDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.definitions[def.ID]; exists {
		return fmt.Errorf("challenge definition already registered: %s", def.ID)
	}

	r.definitions[def.ID] = def
	return nil
}

// Get retrieves a challenge by ID.
func (r *Registry) Get(id ChallengeID) (Challenge, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	challenge, exists := r.challenges[id]
	if !exists {
		return nil, fmt.Errorf("challenge not found: %s", id)
	}

	return challenge, nil
}

// GetDefinition retrieves a challenge definition by ID.
func (r *Registry) GetDefinition(id ChallengeID) (*ChallengeDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	def, exists := r.definitions[id]
	if !exists {
		return nil, fmt.Errorf("challenge definition not found: %s", id)
	}

	return def, nil
}

// List returns all registered challenges.
func (r *Registry) List() []Challenge {
	r.mu.RLock()
	defer r.mu.RUnlock()

	challenges := make([]Challenge, 0, len(r.challenges))
	for _, c := range r.challenges {
		challenges = append(challenges, c)
	}

	// Sort by ID for consistent ordering
	sort.Slice(challenges, func(i, j int) bool {
		return challenges[i].ID() < challenges[j].ID()
	})

	return challenges
}

// ListDefinitions returns all registered challenge definitions.
func (r *Registry) ListDefinitions() []*ChallengeDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]*ChallengeDefinition, 0, len(r.definitions))
	for _, d := range r.definitions {
		defs = append(defs, d)
	}

	// Sort by ID for consistent ordering
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].ID < defs[j].ID
	})

	return defs
}

// ListByCategory returns challenges filtered by category.
func (r *Registry) ListByCategory(category string) []Challenge {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var challenges []Challenge
	for id, c := range r.challenges {
		if def, exists := r.definitions[id]; exists && def.Category == category {
			challenges = append(challenges, c)
		}
	}

	sort.Slice(challenges, func(i, j int) bool {
		return challenges[i].ID() < challenges[j].ID()
	})

	return challenges
}

// GetDependencyOrder returns challenges sorted by dependency order.
func (r *Registry) GetDependencyOrder() ([]Challenge, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Build dependency graph
	inDegree := make(map[ChallengeID]int)
	dependents := make(map[ChallengeID][]ChallengeID)

	for id, c := range r.challenges {
		if _, exists := inDegree[id]; !exists {
			inDegree[id] = 0
		}
		for _, dep := range c.Dependencies() {
			inDegree[id]++
			dependents[dep] = append(dependents[dep], id)
		}
	}

	// Topological sort (Kahn's algorithm)
	var queue []ChallengeID
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	var ordered []Challenge
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		if c, exists := r.challenges[id]; exists {
			ordered = append(ordered, c)
		}

		for _, dependent := range dependents[id] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Check for cycles
	if len(ordered) != len(r.challenges) {
		return nil, fmt.Errorf("circular dependency detected in challenges")
	}

	return ordered, nil
}

// LoadDefinitionsFromFile loads challenge definitions from a JSON file.
func (r *Registry) LoadDefinitionsFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read definitions file: %w", err)
	}

	var bank struct {
		Version    string               `json:"version"`
		Challenges []ChallengeDefinition `json:"challenges"`
	}

	if err := json.Unmarshal(data, &bank); err != nil {
		return fmt.Errorf("failed to parse definitions file: %w", err)
	}

	for i := range bank.Challenges {
		if err := r.RegisterDefinition(&bank.Challenges[i]); err != nil {
			return err
		}
	}

	return nil
}

// ValidateDependencies checks that all challenge dependencies are registered.
func (r *Registry) ValidateDependencies() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for id, c := range r.challenges {
		for _, dep := range c.Dependencies() {
			if _, exists := r.challenges[dep]; !exists {
				return fmt.Errorf("challenge %s has unregistered dependency: %s", id, dep)
			}
		}
	}

	return nil
}

// Clear removes all challenges from the registry.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.challenges = make(map[ChallengeID]Challenge)
	r.definitions = make(map[ChallengeID]*ChallengeDefinition)
}

// Count returns the number of registered challenges.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.challenges)
}

// Register adds a challenge to the default registry.
func Register(challenge Challenge) error {
	return DefaultRegistry.Register(challenge)
}

// Get retrieves a challenge from the default registry.
func Get(id ChallengeID) (Challenge, error) {
	return DefaultRegistry.Get(id)
}

// List returns all challenges from the default registry.
func List() []Challenge {
	return DefaultRegistry.List()
}
