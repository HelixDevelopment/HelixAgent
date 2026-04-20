// Package planning provides advanced planning capabilities for AI agents
// including Tree of Thoughts, Monte Carlo Tree Search, and Hierarchical Planning.
package planning

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/sirupsen/logrus"
)

// ThoughtState represents the state of a thought in the tree
type ThoughtState string

const (
	ThoughtStatePending   ThoughtState = "pending"
	ThoughtStateActive    ThoughtState = "active"
	ThoughtStateEvaluated ThoughtState = "evaluated"
	ThoughtStatePruned    ThoughtState = "pruned"
	ThoughtStateSelected  ThoughtState = "selected"
)

// Thought represents a single reasoning step in the Tree of Thoughts
type Thought struct {
	ID          string                 `json:"id"`
	ParentID    string                 `json:"parent_id,omitempty"`
	Content     string                 `json:"content"`
	Reasoning   string                 `json:"reasoning"`
	State       ThoughtState           `json:"state"`
	Score       float64                `json:"score"`
	Confidence  float64                `json:"confidence"`
	Depth       int                    `json:"depth"`
	Children    []*Thought             `json:"children,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	EvaluatedAt *time.Time             `json:"evaluated_at,omitempty"`
}

// ThoughtNode is a node in the thought tree for navigation
type ThoughtNode struct {
	Thought  *Thought
	Parent   *ThoughtNode
	Children []*ThoughtNode
}

// TreeOfThoughtsConfig holds configuration for ToT
type TreeOfThoughtsConfig struct {
	// MaxDepth is the maximum depth of the thought tree
	MaxDepth int `json:"max_depth"`
	// MaxBranches is the maximum number of branches at each node
	MaxBranches int `json:"max_branches"`
	// MinScore is the minimum score for a thought to be considered
	MinScore float64 `json:"min_score"`
	// PruneThreshold is the score below which thoughts are pruned
	PruneThreshold float64 `json:"prune_threshold"`
	// SearchStrategy is the search strategy (bfs, dfs, beam)
	SearchStrategy string `json:"search_strategy"`
	// BeamWidth is the beam width for beam search
	BeamWidth int `json:"beam_width"`
	// Temperature for thought generation diversity
	Temperature float64 `json:"temperature"`
	// EnableBacktracking allows backtracking on dead ends
	EnableBacktracking bool `json:"enable_backtracking"`
	// MaxIterations limits total iterations
	MaxIterations int `json:"max_iterations"`
	// Timeout for the entire search
	Timeout time.Duration `json:"timeout"`
}

// DefaultTreeOfThoughtsConfig returns default configuration
func DefaultTreeOfThoughtsConfig() TreeOfThoughtsConfig {
	return TreeOfThoughtsConfig{
		MaxDepth:           10,
		MaxBranches:        5,
		MinScore:           0.3,
		PruneThreshold:     0.2,
		SearchStrategy:     "beam",
		BeamWidth:          3,
		Temperature:        0.7,
		EnableBacktracking: true,
		MaxIterations:      100,
		Timeout:            5 * time.Minute,
	}
}

// ThoughtGenerator generates new thoughts from a parent thought
type ThoughtGenerator interface {
	// GenerateThoughts generates child thoughts from a parent
	GenerateThoughts(ctx context.Context, parent *Thought, count int) ([]*Thought, error)
	// GenerateInitialThoughts generates initial thoughts from a problem
	GenerateInitialThoughts(ctx context.Context, problem string, count int) ([]*Thought, error)
}

// ThoughtEvaluator evaluates the quality of thoughts
type ThoughtEvaluator interface {
	// EvaluateThought scores a thought
	EvaluateThought(ctx context.Context, thought *Thought) (float64, error)
	// EvaluatePath scores an entire path from root to leaf
	EvaluatePath(ctx context.Context, path []*Thought) (float64, error)
	// IsTerminal checks if a thought represents a terminal/solution state
	IsTerminal(ctx context.Context, thought *Thought) (bool, error)
}

// TreeOfThoughts implements the Tree of Thoughts reasoning framework.
//
// Concurrency model (CONST-029): Solve runs the search single-threaded
// (generator/evaluator callbacks are awaited synchronously), so mu
// was effectively redundant — it only guarded the initial reset in
// Solve. Dropped. Callers must not invoke Solve concurrently on the
// same instance; the type is not safe for concurrent Solve calls
// and never was (most reads of iterations/bestScore/bestPath in the
// loop were lock-free under the old design too).
type TreeOfThoughts struct {
	config     TreeOfThoughtsConfig
	generator  ThoughtGenerator
	evaluator  ThoughtEvaluator
	root       *ThoughtNode
	bestPath   []*Thought
	bestScore  float64
	iterations int
	logger     *logrus.Logger
}

// NewTreeOfThoughts creates a new Tree of Thoughts instance
func NewTreeOfThoughts(config TreeOfThoughtsConfig, generator ThoughtGenerator, evaluator ThoughtEvaluator, logger *logrus.Logger) *TreeOfThoughts {
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.WarnLevel)
	}

	return &TreeOfThoughts{
		config:    config,
		generator: generator,
		evaluator: evaluator,
		bestScore: -1,
		logger:    logger,
	}
}

// Solve attempts to solve a problem using Tree of Thoughts
func (t *TreeOfThoughts) Solve(ctx context.Context, problem string) (*ToTResult, error) {
	t.iterations = 0
	t.bestPath = nil
	t.bestScore = -1

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, t.config.Timeout)
	defer cancel()

	startTime := time.Now()

	// Generate initial thoughts
	initialThoughts, err := t.generator.GenerateInitialThoughts(ctx, problem, t.config.MaxBranches)
	if err != nil {
		return nil, fmt.Errorf("failed to generate initial thoughts: %w", err)
	}

	// Create root node
	rootThought := &Thought{
		ID:        "root",
		Content:   problem,
		State:     ThoughtStateActive,
		Depth:     0,
		CreatedAt: time.Now(),
		Metadata:  map[string]interface{}{"type": "problem"},
	}

	t.root = &ThoughtNode{
		Thought:  rootThought,
		Children: make([]*ThoughtNode, 0),
	}

	// Add initial thoughts as children
	for _, thought := range initialThoughts {
		thought.ParentID = rootThought.ID
		thought.Depth = 1
		childNode := &ThoughtNode{
			Thought: thought,
			Parent:  t.root,
		}
		t.root.Children = append(t.root.Children, childNode)
	}

	// Execute search based on strategy
	var solution []*Thought
	switch t.config.SearchStrategy {
	case "bfs":
		solution, err = t.breadthFirstSearch(ctx)
	case "dfs":
		solution, err = t.depthFirstSearch(ctx)
	case "beam":
		solution, err = t.beamSearch(ctx)
	default:
		solution, err = t.beamSearch(ctx)
	}

	if err != nil {
		return nil, err
	}

	// Build result
	result := &ToTResult{
		Problem:       problem,
		Solution:      solution,
		BestScore:     t.bestScore,
		Iterations:    t.iterations,
		Duration:      time.Since(startTime),
		Strategy:      t.config.SearchStrategy,
		TreeDepth:     t.getMaxDepth(t.root),
		NodesExplored: t.countNodes(t.root),
	}

	return result, nil
}

// breadthFirstSearch performs BFS on the thought tree
func (t *TreeOfThoughts) breadthFirstSearch(ctx context.Context) ([]*Thought, error) {
	queue := []*ThoughtNode{t.root}

	for len(queue) > 0 && t.iterations < t.config.MaxIterations {
		select {
		case <-ctx.Done():
			return t.bestPath, ctx.Err()
		default:
		}

		// Dequeue
		current := queue[0]
		queue = queue[1:]
		t.iterations++

		// Skip if pruned
		if current.Thought.State == ThoughtStatePruned {
			continue
		}

		// Evaluate current thought
		score, err := t.evaluator.EvaluateThought(ctx, current.Thought)
		if err != nil {
			t.logger.Warnf("Failed to evaluate thought: %v", err)
			continue
		}
		current.Thought.Score = score
		current.Thought.State = ThoughtStateEvaluated
		now := time.Now()
		current.Thought.EvaluatedAt = &now

		// Prune if below threshold
		if score < t.config.PruneThreshold {
			current.Thought.State = ThoughtStatePruned
			continue
		}

		// Check if terminal
		isTerminal, err := t.evaluator.IsTerminal(ctx, current.Thought)
		if err != nil {
			t.logger.Warnf("Failed to check terminal: %v", err)
			continue
		}

		if isTerminal {
			path := t.getPath(current)
			pathScore, _ := t.evaluator.EvaluatePath(ctx, path) //nolint:errcheck
			if pathScore > t.bestScore {
				t.bestScore = pathScore
				t.bestPath = path
			}
			continue
		}

		// Generate children if not at max depth
		if current.Thought.Depth < t.config.MaxDepth {
			children, err := t.generator.GenerateThoughts(ctx, current.Thought, t.config.MaxBranches)
			if err != nil {
				t.logger.Warnf("Failed to generate thoughts: %v", err)
				continue
			}

			for _, child := range children {
				child.ParentID = current.Thought.ID
				child.Depth = current.Thought.Depth + 1
				childNode := &ThoughtNode{
					Thought: child,
					Parent:  current,
				}
				current.Children = append(current.Children, childNode)
				queue = append(queue, childNode)
			}
		}
	}

	return t.bestPath, nil
}

// depthFirstSearch performs DFS on the thought tree
func (t *TreeOfThoughts) depthFirstSearch(ctx context.Context) ([]*Thought, error) {
	stack := []*ThoughtNode{}

	// Add root children to stack in reverse order
	for i := len(t.root.Children) - 1; i >= 0; i-- {
		stack = append(stack, t.root.Children[i])
	}

	for len(stack) > 0 && t.iterations < t.config.MaxIterations {
		select {
		case <-ctx.Done():
			return t.bestPath, ctx.Err()
		default:
		}

		// Pop
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		t.iterations++

		if current.Thought.State == ThoughtStatePruned {
			continue
		}

		// Evaluate
		score, err := t.evaluator.EvaluateThought(ctx, current.Thought)
		if err != nil {
			continue
		}
		current.Thought.Score = score
		current.Thought.State = ThoughtStateEvaluated
		now := time.Now()
		current.Thought.EvaluatedAt = &now

		if score < t.config.PruneThreshold {
			current.Thought.State = ThoughtStatePruned
			continue
		}

		// Check terminal
		isTerminal, _ := t.evaluator.IsTerminal(ctx, current.Thought) //nolint:errcheck
		if isTerminal {
			path := t.getPath(current)
			pathScore, _ := t.evaluator.EvaluatePath(ctx, path) //nolint:errcheck
			if pathScore > t.bestScore {
				t.bestScore = pathScore
				t.bestPath = path
			}
			continue
		}

		// Generate children
		if current.Thought.Depth < t.config.MaxDepth {
			children, err := t.generator.GenerateThoughts(ctx, current.Thought, t.config.MaxBranches)
			if err != nil {
				continue
			}

			for i := len(children) - 1; i >= 0; i-- {
				child := children[i]
				child.ParentID = current.Thought.ID
				child.Depth = current.Thought.Depth + 1
				childNode := &ThoughtNode{
					Thought: child,
					Parent:  current,
				}
				current.Children = append(current.Children, childNode)
				stack = append(stack, childNode)
			}
		}
	}

	return t.bestPath, nil
}

// beamSearch performs beam search on the thought tree
func (t *TreeOfThoughts) beamSearch(ctx context.Context) ([]*Thought, error) {
	beam := t.root.Children

	for depth := 1; depth <= t.config.MaxDepth && len(beam) > 0 && t.iterations < t.config.MaxIterations; depth++ {
		select {
		case <-ctx.Done():
			return t.bestPath, ctx.Err()
		default:
		}

		// Evaluate all nodes in current beam
		for _, node := range beam {
			t.iterations++
			score, err := t.evaluator.EvaluateThought(ctx, node.Thought)
			if err != nil {
				node.Thought.Score = 0
				continue
			}
			node.Thought.Score = score
			node.Thought.State = ThoughtStateEvaluated
			now := time.Now()
			node.Thought.EvaluatedAt = &now
		}

		// Sort by score
		sort.Slice(beam, func(i, j int) bool {
			return beam[i].Thought.Score > beam[j].Thought.Score
		})

		// Keep top k
		if len(beam) > t.config.BeamWidth {
			for i := t.config.BeamWidth; i < len(beam); i++ {
				beam[i].Thought.State = ThoughtStatePruned
			}
			beam = beam[:t.config.BeamWidth]
		}

		// Check for terminal states
		for _, node := range beam {
			isTerminal, _ := t.evaluator.IsTerminal(ctx, node.Thought) //nolint:errcheck
			if isTerminal {
				path := t.getPath(node)
				pathScore, _ := t.evaluator.EvaluatePath(ctx, path) //nolint:errcheck
				if pathScore > t.bestScore {
					t.bestScore = pathScore
					t.bestPath = path
				}
			}
		}

		// Generate next level
		nextBeam := []*ThoughtNode{}
		for _, node := range beam {
			if node.Thought.Score < t.config.PruneThreshold {
				continue
			}

			children, err := t.generator.GenerateThoughts(ctx, node.Thought, t.config.MaxBranches)
			if err != nil {
				continue
			}

			for _, child := range children {
				child.ParentID = node.Thought.ID
				child.Depth = depth + 1
				childNode := &ThoughtNode{
					Thought: child,
					Parent:  node,
				}
				node.Children = append(node.Children, childNode)
				nextBeam = append(nextBeam, childNode)
			}
		}

		beam = nextBeam
	}

	return t.bestPath, nil
}

// getPath returns the path from root to a node
func (t *TreeOfThoughts) getPath(node *ThoughtNode) []*Thought {
	path := []*Thought{}
	current := node
	for current != nil {
		path = append([]*Thought{current.Thought}, path...)
		current = current.Parent
	}
	return path
}

// getMaxDepth returns the maximum depth of the tree
func (t *TreeOfThoughts) getMaxDepth(node *ThoughtNode) int {
	if node == nil {
		return 0
	}
	maxChildDepth := 0
	for _, child := range node.Children {
		childDepth := t.getMaxDepth(child)
		if childDepth > maxChildDepth {
			maxChildDepth = childDepth
		}
	}
	return maxChildDepth + 1
}

// countNodes counts total nodes in the tree
func (t *TreeOfThoughts) countNodes(node *ThoughtNode) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range node.Children {
		count += t.countNodes(child)
	}
	return count
}

// ToTResult holds the result of a Tree of Thoughts search
type ToTResult struct {
	Problem       string        `json:"problem"`
	Solution      []*Thought    `json:"solution"`
	BestScore     float64       `json:"best_score"`
	Iterations    int           `json:"iterations"`
	Duration      time.Duration `json:"duration"`
	Strategy      string        `json:"strategy"`
	TreeDepth     int           `json:"tree_depth"`
	NodesExplored int           `json:"nodes_explored"`
}

// GetSolutionContent returns the content of the solution path
func (r *ToTResult) GetSolutionContent() []string {
	contents := make([]string, len(r.Solution))
	for i, thought := range r.Solution {
		contents[i] = thought.Content
	}
	return contents
}

// MarshalJSON implements custom JSON marshaling
func (r *ToTResult) MarshalJSON() ([]byte, error) {
	type Alias ToTResult
	return json.Marshal(&struct {
		*Alias
		DurationMs int64 `json:"duration_ms"`
	}{
		Alias:      (*Alias)(r),
		DurationMs: r.Duration.Milliseconds(),
	})
}

// LLMThoughtGenerator implements ThoughtGenerator using an LLM
type LLMThoughtGenerator struct {
	generateFunc func(ctx context.Context, prompt string) (string, error)
	temperature  float64
	logger       *logrus.Logger
}

// NewLLMThoughtGenerator creates a new LLM-based thought generator
func NewLLMThoughtGenerator(generateFunc func(ctx context.Context, prompt string) (string, error), temperature float64, logger *logrus.Logger) *LLMThoughtGenerator {
	return &LLMThoughtGenerator{
		generateFunc: generateFunc,
		temperature:  temperature,
		logger:       logger,
	}
}

// GenerateThoughts generates child thoughts using LLM
func (g *LLMThoughtGenerator) GenerateThoughts(ctx context.Context, parent *Thought, count int) ([]*Thought, error) {
	prompt := fmt.Sprintf(`Given the current reasoning step:
"%s"

Generate %d distinct next steps or approaches to continue solving this problem.
Each step should be different and explore a unique angle.
Format each step on a new line starting with a number.`, parent.Content, count)

	response, err := g.generateFunc(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Parse response into thoughts
	thoughts := make([]*Thought, 0, count)
	lines := splitLines(response)

	for i, line := range lines {
		if len(line) == 0 || i >= count {
			continue
		}

		thought := &Thought{
			ID:        fmt.Sprintf("%s-%d", parent.ID, i),
			Content:   line,
			State:     ThoughtStatePending,
			CreatedAt: time.Now(),
			Metadata:  map[string]interface{}{"generated_from": parent.ID},
		}
		thoughts = append(thoughts, thought)
	}

	return thoughts, nil
}

// GenerateInitialThoughts generates initial thoughts from a problem
func (g *LLMThoughtGenerator) GenerateInitialThoughts(ctx context.Context, problem string, count int) ([]*Thought, error) {
	prompt := fmt.Sprintf(`Given the problem:
"%s"

Generate %d distinct initial approaches or strategies to solve this problem.
Each approach should be different and explore a unique angle.
Format each approach on a new line starting with a number.`, problem, count)

	response, err := g.generateFunc(ctx, prompt)
	if err != nil {
		return nil, err
	}

	thoughts := make([]*Thought, 0, count)
	lines := splitLines(response)

	for i, line := range lines {
		if len(line) == 0 || i >= count {
			continue
		}

		thought := &Thought{
			ID:        fmt.Sprintf("init-%d", i),
			Content:   line,
			State:     ThoughtStatePending,
			Depth:     1,
			CreatedAt: time.Now(),
			Metadata:  map[string]interface{}{"type": "initial"},
		}
		thoughts = append(thoughts, thought)
	}

	return thoughts, nil
}

// LLMThoughtEvaluator implements ThoughtEvaluator using an LLM
type LLMThoughtEvaluator struct {
	evaluateFunc     func(ctx context.Context, prompt string) (string, error)
	terminalKeywords []string
	logger           *logrus.Logger
}

// NewLLMThoughtEvaluator creates a new LLM-based thought evaluator
func NewLLMThoughtEvaluator(evaluateFunc func(ctx context.Context, prompt string) (string, error), logger *logrus.Logger) *LLMThoughtEvaluator {
	return &LLMThoughtEvaluator{
		evaluateFunc:     evaluateFunc,
		terminalKeywords: []string{"solution", "answer", "result", "conclusion", "final"},
		logger:           logger,
	}
}

// EvaluateThought scores a thought
func (e *LLMThoughtEvaluator) EvaluateThought(ctx context.Context, thought *Thought) (float64, error) {
	prompt := fmt.Sprintf(`Evaluate the following reasoning step on a scale of 0.0 to 1.0:
"%s"

Consider:
- Logical validity
- Progress toward solution
- Feasibility
- Clarity

Respond with only a number between 0.0 and 1.0.`, thought.Content)

	response, err := e.evaluateFunc(ctx, prompt)
	if err != nil {
		return 0, err
	}

	var score float64
	if _, err := fmt.Sscanf(response, "%f", &score); err != nil {
		return 0, fmt.Errorf("failed to parse score: %w", err)
	}

	// Clamp to valid range
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score, nil
}

// EvaluatePath scores an entire path
func (e *LLMThoughtEvaluator) EvaluatePath(ctx context.Context, path []*Thought) (float64, error) {
	if len(path) == 0 {
		return 0, nil
	}

	// Calculate weighted average of scores
	totalScore := 0.0
	totalWeight := 0.0

	for i, thought := range path {
		// Later thoughts have higher weight
		weight := math.Pow(1.5, float64(i))
		totalScore += thought.Score * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 0, nil
	}

	return totalScore / totalWeight, nil
}

// IsTerminal checks if a thought is a terminal state
func (e *LLMThoughtEvaluator) IsTerminal(ctx context.Context, thought *Thought) (bool, error) {
	// Check for terminal keywords
	content := thought.Content
	for _, keyword := range e.terminalKeywords {
		if containsIgnoreCase(content, keyword) {
			return true, nil
		}
	}

	// Check if score is very high (indicates solution)
	if thought.Score > 0.9 {
		return true, nil
	}

	return false, nil
}

// Helper functions
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 {
				// Remove leading numbers and dots
				for j := 0; j < len(line); j++ {
					if line[j] >= '0' && line[j] <= '9' || line[j] == '.' || line[j] == ' ' || line[j] == ')' {
						continue
					}
					line = line[j:]
					break
				}
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		line := s[start:]
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}
	return lines
}

func containsIgnoreCase(s, substr string) bool {
	sLower := make([]byte, len(s))
	substrLower := make([]byte, len(substr))

	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			sLower[i] = s[i] + 32
		} else {
			sLower[i] = s[i]
		}
	}

	for i := 0; i < len(substr); i++ {
		if substr[i] >= 'A' && substr[i] <= 'Z' {
			substrLower[i] = substr[i] + 32
		} else {
			substrLower[i] = substr[i]
		}
	}

	return contains(string(sLower), string(substrLower))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
