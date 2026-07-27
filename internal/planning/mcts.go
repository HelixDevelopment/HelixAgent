// Package planning provides Monte Carlo Tree Search (MCTS) implementation
// based on the MASTER framework for code generation and planning.
package planning

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
)

// MCTSNodeState represents the state of an MCTS node
type MCTSNodeState string

const (
	MCTSNodeStateUnexpanded MCTSNodeState = "unexpanded"
	MCTSNodeStateExpanded   MCTSNodeState = "expanded"
	MCTSNodeStateTerminal   MCTSNodeState = "terminal"
)

// MCTSNode represents a node in the MCTS tree.
//
// CONST-029: Visits and TotalReward are stored as atomics for lock-free
// updates from parallel backpropagation; the historical JSON wire format
// (plain int / float64 under `visits` / `total_reward`) is preserved via
// custom MarshalJSON / UnmarshalJSON. Children is a *safe.Slice; Metadata
// is a *safe.Store. Accessor methods Visits() / TotalReward() expose the
// values via the historical int / float64 contract.
type MCTSNode struct {
	ID        string        `json:"id"`
	ParentID  string        `json:"parent_id,omitempty"`
	State     interface{}   `json:"state"`
	Action    string        `json:"action,omitempty"`
	NodeState MCTSNodeState `json:"node_state"`
	Depth     int           `json:"depth"`
	CreatedAt time.Time     `json:"created_at"`

	// Hot-path counters: lock-free under parallel backpropagation.
	visits      atomic.Int64
	totalReward atomic.Uint64 // math.Float64bits storage

	// Children and Metadata are concurrent-safe containers; internal code
	// uses Append / Range / Snapshot instead of direct slice/map ops.
	Children *safe.Slice[*MCTSNode]           `json:"-"`
	Metadata *safe.Store[string, interface{}] `json:"-"`
}

// Visits returns the visit count.
func (n *MCTSNode) Visits() int {
	return int(n.visits.Load())
}

// TotalReward returns the accumulated reward.
func (n *MCTSNode) TotalReward() float64 {
	return math.Float64frombits(n.totalReward.Load())
}

// SetVisits overrides the visit count (test-only helper).
func (n *MCTSNode) SetVisits(v int) {
	n.visits.Store(int64(v))
}

// SetTotalReward overrides the accumulated reward (test-only helper).
func (n *MCTSNode) SetTotalReward(r float64) {
	n.totalReward.Store(math.Float64bits(r))
}

// AverageReward returns the average reward for this node.
func (n *MCTSNode) AverageReward() float64 {
	v := n.visits.Load()
	if v == 0 {
		return 0
	}
	return math.Float64frombits(n.totalReward.Load()) / float64(v)
}

// AddReward adds a reward to the node. Lock-free under parallel backprop.
func (n *MCTSNode) AddReward(reward float64) {
	n.visits.Add(1)
	for {
		prev := n.totalReward.Load()
		next := math.Float64bits(math.Float64frombits(prev) + reward)
		if n.totalReward.CompareAndSwap(prev, next) {
			return
		}
	}
}

// ChildrenSnapshot returns the children as a plain slice (copy); safe for
// iteration without holding any lock.
func (n *MCTSNode) ChildrenSnapshot() []*MCTSNode {
	if n.Children == nil {
		return nil
	}
	return n.Children.Snapshot()
}

// mctsNodeJSON is the wire-format shape (historical fields preserved).
type mctsNodeJSON struct {
	ID          string                 `json:"id"`
	ParentID    string                 `json:"parent_id,omitempty"`
	State       interface{}            `json:"state"`
	Action      string                 `json:"action,omitempty"`
	Visits      int                    `json:"visits"`
	TotalReward float64                `json:"total_reward"`
	Children    []*MCTSNode            `json:"children,omitempty"`
	NodeState   MCTSNodeState          `json:"node_state"`
	Depth       int                    `json:"depth"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

// MarshalJSON preserves the public /v1/planning/mcts wire format.
func (n *MCTSNode) MarshalJSON() ([]byte, error) {
	var children []*MCTSNode
	if n.Children != nil {
		children = n.Children.Snapshot()
	}
	var metadata map[string]interface{}
	if n.Metadata != nil {
		metadata = n.Metadata.Snapshot()
	}
	return json.Marshal(&mctsNodeJSON{
		ID:          n.ID,
		ParentID:    n.ParentID,
		State:       n.State,
		Action:      n.Action,
		Visits:      int(n.visits.Load()),
		TotalReward: math.Float64frombits(n.totalReward.Load()),
		Children:    children,
		NodeState:   n.NodeState,
		Depth:       n.Depth,
		Metadata:    metadata,
		CreatedAt:   n.CreatedAt,
	})
}

// UnmarshalJSON restores a node from its wire format.
func (n *MCTSNode) UnmarshalJSON(data []byte) error {
	var tmp mctsNodeJSON
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	n.ID = tmp.ID
	n.ParentID = tmp.ParentID
	n.State = tmp.State
	n.Action = tmp.Action
	n.NodeState = tmp.NodeState
	n.Depth = tmp.Depth
	n.CreatedAt = tmp.CreatedAt
	n.visits.Store(int64(tmp.Visits))
	n.totalReward.Store(math.Float64bits(tmp.TotalReward))
	if len(tmp.Children) > 0 {
		n.Children = safe.NewSlice(tmp.Children...)
	}
	if len(tmp.Metadata) > 0 {
		n.Metadata = safe.NewStoreFromMap(tmp.Metadata)
	}
	return nil
}

// MCTSConfig holds configuration for MCTS
type MCTSConfig struct {
	// ExplorationConstant (C) for UCB1 formula
	ExplorationConstant float64 `json:"exploration_constant"`
	// DepthPreferenceAlpha for UCT-DP formula
	DepthPreferenceAlpha float64 `json:"depth_preference_alpha"`
	// MaxDepth is the maximum search depth
	MaxDepth int `json:"max_depth"`
	// MaxIterations limits total MCTS iterations
	MaxIterations int `json:"max_iterations"`
	// RolloutDepth is the depth for simulation rollouts
	RolloutDepth int `json:"rollout_depth"`
	// SimulationCount is the number of simulations per expansion
	SimulationCount int `json:"simulation_count"`
	// DiscountFactor for future rewards
	DiscountFactor float64 `json:"discount_factor"`
	// EnableParallel enables parallel simulations
	EnableParallel bool `json:"enable_parallel"`
	// ParallelWorkers is the number of parallel workers
	ParallelWorkers int `json:"parallel_workers"`
	// Timeout for the entire search
	Timeout time.Duration `json:"timeout"`
	// UseUCTDP uses depth-preferred UCT formula
	UseUCTDP bool `json:"use_uct_dp"`
}

// DefaultMCTSConfig returns default MCTS configuration
func DefaultMCTSConfig() MCTSConfig {
	return MCTSConfig{
		ExplorationConstant:  1.414, // sqrt(2)
		DepthPreferenceAlpha: 0.5,
		MaxDepth:             50,
		MaxIterations:        1000,
		RolloutDepth:         10,
		SimulationCount:      1,
		DiscountFactor:       0.99,
		EnableParallel:       true,
		ParallelWorkers:      4,
		Timeout:              5 * time.Minute,
		UseUCTDP:             true,
	}
}

// MCTSActionGenerator generates possible actions from a state
type MCTSActionGenerator interface {
	// GetActions returns possible actions from a state
	GetActions(ctx context.Context, state interface{}) ([]string, error)
	// ApplyAction applies an action to get a new state
	ApplyAction(ctx context.Context, state interface{}, action string) (interface{}, error)
}

// MCTSRewardFunction evaluates states and returns rewards
type MCTSRewardFunction interface {
	// Evaluate returns a reward for a state (0-1)
	Evaluate(ctx context.Context, state interface{}) (float64, error)
	// IsTerminal checks if a state is terminal
	IsTerminal(ctx context.Context, state interface{}) (bool, error)
}

// MCTSRolloutPolicy performs simulation rollouts
type MCTSRolloutPolicy interface {
	// Rollout performs a rollout from a state and returns estimated value
	Rollout(ctx context.Context, state interface{}, depth int) (float64, error)
}

// MCTS implements Monte Carlo Tree Search
// Note: Uses math/rand for algorithmic randomness in tree exploration - this doesn't require cryptographic security
type MCTS struct {
	config        MCTSConfig
	actionGen     MCTSActionGenerator
	rewardFunc    MCTSRewardFunction
	rolloutPolicy MCTSRolloutPolicy
	root          *MCTSNode
	iterations    int
	mu            sync.RWMutex
	logger        *logrus.Logger
	rng           *rand.Rand // #nosec G404 - MCTS exploration doesn't require cryptographic randomness
}

// NewMCTS creates a new MCTS instance
func NewMCTS(config MCTSConfig, actionGen MCTSActionGenerator, rewardFunc MCTSRewardFunction, rolloutPolicy MCTSRolloutPolicy, logger *logrus.Logger) *MCTS {
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.WarnLevel)
	}

	return &MCTS{
		config:        config,
		actionGen:     actionGen,
		rewardFunc:    rewardFunc,
		rolloutPolicy: rolloutPolicy,
		logger:        logger,
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())), // #nosec G404 - MCTS uses non-cryptographic randomness for exploration
	}
}

// Search performs MCTS search from initial state
func (m *MCTS) Search(ctx context.Context, initialState interface{}) (*MCTSResult, error) {
	m.mu.Lock()
	m.iterations = 0
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	startTime := time.Now()

	// Create root node
	m.root = &MCTSNode{
		ID:        "root",
		State:     initialState,
		NodeState: MCTSNodeStateUnexpanded,
		Depth:     0,
		CreatedAt: time.Now(),
		Children:  safe.NewSlice[*MCTSNode](),
		Metadata:  safe.NewStore[string, interface{}](),
	}

	// Main MCTS loop
	for m.iterations < m.config.MaxIterations {
		if ctx.Err() != nil {
			break
		}

		m.iterations++

		// Selection
		node := m.selectNode(ctx, m.root)
		if node == nil {
			continue
		}

		// Expansion
		expandedNode, err := m.expand(ctx, node)
		if err != nil {
			m.logger.Warnf("Expansion failed: %v", err)
			continue
		}

		// Simulation
		reward, err := m.simulate(ctx, expandedNode)
		if err != nil {
			m.logger.Warnf("Simulation failed: %v", err)
			continue
		}

		// Backpropagation
		m.backpropagate(expandedNode, reward)
	}

	// Get best action sequence
	bestPath := m.getBestPath()
	bestActions := make([]string, 0)
	for _, node := range bestPath {
		if node.Action != "" {
			bestActions = append(bestActions, node.Action)
		}
	}

	result := &MCTSResult{
		BestActions:     bestActions,
		BestPath:        bestPath,
		TotalIterations: m.iterations,
		Duration:        time.Since(startTime),
		RootVisits:      m.root.Visits(),
		TreeSize:        m.countNodes(m.root),
	}

	if len(bestPath) > 0 {
		result.FinalState = bestPath[len(bestPath)-1].State
		result.FinalReward = bestPath[len(bestPath)-1].AverageReward()
	}

	return result, nil
}

// selectNode selects a node to expand using UCB1/UCT-DP
func (m *MCTS) selectNode(ctx context.Context, node *MCTSNode) *MCTSNode {
	current := node

	for {
		children := current.ChildrenSnapshot()
		if current.NodeState != MCTSNodeStateExpanded || len(children) == 0 {
			break
		}
		// Check if any child is unexpanded
		for _, child := range children {
			if child.NodeState == MCTSNodeStateUnexpanded {
				return child
			}
		}

		// All children expanded, select best using UCB
		current = m.selectBestChild(current)
		if current == nil {
			return nil
		}
	}

	return current
}

// UCTValue calculates the UCT (Upper Confidence bound for Trees) value for a node
// This implements the UCT-DP formula with optional depth preference
func (m *MCTS) UCTValue(node *MCTSNode, parentVisits int) float64 {
	if node == nil {
		return 0
	}

	visits := int(node.visits.Load())
	reward := math.Float64frombits(node.totalReward.Load())
	depth := node.Depth

	if visits == 0 {
		return math.Inf(1) // Prioritize unvisited nodes
	}

	if parentVisits == 0 {
		parentVisits = 1
	}

	// Calculate UCB value
	exploitation := reward / float64(visits)
	exploration := m.config.ExplorationConstant * math.Sqrt(math.Log(float64(parentVisits))/float64(visits))

	ucbValue := exploitation + exploration

	// Add depth preference for UCT-DP
	if m.config.UseUCTDP && m.config.MaxDepth > 0 {
		depthBonus := m.config.DepthPreferenceAlpha * (float64(depth) / float64(m.config.MaxDepth))
		ucbValue += depthBonus
	}

	return ucbValue
}

// selectBestChild selects the best child using UCB1 or UCT-DP
func (m *MCTS) selectBestChild(node *MCTSNode) *MCTSNode {
	children := node.ChildrenSnapshot()
	if len(children) == 0 {
		return nil
	}

	var bestChild *MCTSNode
	bestValue := math.Inf(-1)

	parentVisits := int(node.visits.Load())
	if parentVisits == 0 {
		parentVisits = 1
	}

	for _, child := range children {
		visits := int(child.visits.Load())
		reward := math.Float64frombits(child.totalReward.Load())
		depth := child.Depth

		if visits == 0 {
			// Prioritize unvisited nodes
			return child
		}

		// Calculate UCB value
		exploitation := reward / float64(visits)
		exploration := m.config.ExplorationConstant * math.Sqrt(math.Log(float64(parentVisits))/float64(visits))

		ucbValue := exploitation + exploration

		// Add depth preference for UCT-DP
		if m.config.UseUCTDP && m.config.MaxDepth > 0 {
			depthBonus := m.config.DepthPreferenceAlpha * (float64(depth) / float64(m.config.MaxDepth))
			ucbValue += depthBonus
		}

		if ucbValue > bestValue {
			bestValue = ucbValue
			bestChild = child
		}
	}

	return bestChild
}

// expand expands a node by generating children
func (m *MCTS) expand(ctx context.Context, node *MCTSNode) (*MCTSNode, error) {
	if node.NodeState == MCTSNodeStateTerminal {
		return node, nil
	}

	if node.Depth >= m.config.MaxDepth {
		node.NodeState = MCTSNodeStateTerminal
		return node, nil
	}

	// Check if terminal
	isTerminal, err := m.rewardFunc.IsTerminal(ctx, node.State)
	if err != nil {
		return nil, err
	}
	if isTerminal {
		node.NodeState = MCTSNodeStateTerminal
		return node, nil
	}

	// Get possible actions
	actions, err := m.actionGen.GetActions(ctx, node.State)
	if err != nil {
		return nil, err
	}

	if len(actions) == 0 {
		node.NodeState = MCTSNodeStateTerminal
		return node, nil
	}

	// Create child nodes
	if node.Children == nil {
		node.Children = safe.NewSlice[*MCTSNode]()
	}
	for i, action := range actions {
		newState, err := m.actionGen.ApplyAction(ctx, node.State, action)
		if err != nil {
			continue
		}

		child := &MCTSNode{
			ID:        fmt.Sprintf("%s-%d", node.ID, i),
			ParentID:  node.ID,
			State:     newState,
			Action:    action,
			NodeState: MCTSNodeStateUnexpanded,
			Depth:     node.Depth + 1,
			CreatedAt: time.Now(),
			Children:  safe.NewSlice[*MCTSNode](),
			Metadata:  safe.NewStore[string, interface{}](),
		}
		node.Children.Append(child)
	}
	node.NodeState = MCTSNodeStateExpanded

	// Return a random unexpanded child
	children := node.ChildrenSnapshot()
	if len(children) > 0 {
		return children[m.rng.Intn(len(children))], nil
	}

	return node, nil
}

// simulate performs a rollout from a node
func (m *MCTS) simulate(ctx context.Context, node *MCTSNode) (float64, error) {
	if m.rolloutPolicy != nil {
		return m.rolloutPolicy.Rollout(ctx, node.State, m.config.RolloutDepth)
	}

	// Default: use reward function directly
	return m.rewardFunc.Evaluate(ctx, node.State)
}

// backpropagate propagates reward up the tree
func (m *MCTS) backpropagate(node *MCTSNode, reward float64) {
	current := node
	discount := 1.0

	for current != nil {
		current.AddReward(reward * discount)
		discount *= m.config.DiscountFactor

		// Find parent
		if current.ParentID == "" {
			break
		}
		current = m.findParent(m.root, current.ParentID)
	}
}

// findParent finds a node's parent by ID
func (m *MCTS) findParent(root *MCTSNode, parentID string) *MCTSNode {
	if root.ID == parentID {
		return root
	}

	for _, child := range root.ChildrenSnapshot() {
		if found := m.findParent(child, parentID); found != nil {
			return found
		}
	}

	return nil
}

// getBestPath returns the best path from root
func (m *MCTS) getBestPath() []*MCTSNode {
	path := []*MCTSNode{m.root}
	current := m.root

	for {
		children := current.ChildrenSnapshot()
		if len(children) == 0 {
			break
		}
		// Select child with highest average reward
		var bestChild *MCTSNode
		bestReward := math.Inf(-1)

		for _, child := range children {
			avgReward := child.AverageReward()
			if avgReward > bestReward {
				bestReward = avgReward
				bestChild = child
			}
		}

		if bestChild == nil {
			break
		}

		path = append(path, bestChild)
		current = bestChild
	}

	return path
}

// countNodes counts total nodes in tree
func (m *MCTS) countNodes(node *MCTSNode) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range node.ChildrenSnapshot() {
		count += m.countNodes(child)
	}
	return count
}

// MCTSResult holds the result of an MCTS search
type MCTSResult struct {
	BestActions     []string      `json:"best_actions"`
	BestPath        []*MCTSNode   `json:"best_path"`
	FinalState      interface{}   `json:"final_state"`
	FinalReward     float64       `json:"final_reward"`
	TotalIterations int           `json:"total_iterations"`
	Duration        time.Duration `json:"duration"`
	RootVisits      int           `json:"root_visits"`
	TreeSize        int           `json:"tree_size"`
}

// MarshalJSON implements custom JSON marshaling
func (r *MCTSResult) MarshalJSON() ([]byte, error) {
	type Alias MCTSResult
	return json.Marshal(&struct {
		*Alias
		DurationMs int64 `json:"duration_ms"`
	}{
		Alias:      (*Alias)(r),
		DurationMs: r.Duration.Milliseconds(),
	})
}

// CodeActionGenerator implements MCTSActionGenerator for code generation
type CodeActionGenerator struct {
	generateFunc func(ctx context.Context, prompt string) (string, error)
	logger       *logrus.Logger
}

// NewCodeActionGenerator creates a new code action generator
func NewCodeActionGenerator(generateFunc func(ctx context.Context, prompt string) (string, error), logger *logrus.Logger) *CodeActionGenerator {
	return &CodeActionGenerator{
		generateFunc: generateFunc,
		logger:       logger,
	}
}

// GetActions returns possible code actions from current state
func (g *CodeActionGenerator) GetActions(ctx context.Context, state interface{}) ([]string, error) {
	stateStr, ok := state.(string)
	if !ok {
		stateStr = fmt.Sprintf("%v", state)
	}

	prompt := fmt.Sprintf(`Given the current code state:
%s

Generate 3-5 possible next coding actions or modifications.
Each action should be distinct and meaningful.
Format: one action per line.`, stateStr)

	response, err := g.generateFunc(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return splitLines(response), nil
}

// ApplyAction applies an action to the code state
func (g *CodeActionGenerator) ApplyAction(ctx context.Context, state interface{}, action string) (interface{}, error) {
	stateStr, ok := state.(string)
	if !ok {
		stateStr = fmt.Sprintf("%v", state)
	}

	prompt := fmt.Sprintf(`Current code state:
%s

Apply this action: %s

Return the updated code state after applying the action.`, stateStr, action)

	return g.generateFunc(ctx, prompt)
}

// CodeRewardFunction implements MCTSRewardFunction for code evaluation
type CodeRewardFunction struct {
	evaluateFunc func(ctx context.Context, code string) (float64, error)
	testFunc     func(ctx context.Context, code string) (bool, error)
	logger       *logrus.Logger
}

// NewCodeRewardFunction creates a new code reward function
func NewCodeRewardFunction(
	evaluateFunc func(ctx context.Context, code string) (float64, error),
	testFunc func(ctx context.Context, code string) (bool, error),
	logger *logrus.Logger,
) *CodeRewardFunction {
	return &CodeRewardFunction{
		evaluateFunc: evaluateFunc,
		testFunc:     testFunc,
		logger:       logger,
	}
}

// Evaluate evaluates code quality
func (f *CodeRewardFunction) Evaluate(ctx context.Context, state interface{}) (float64, error) {
	code, ok := state.(string)
	if !ok {
		code = fmt.Sprintf("%v", state)
	}

	if f.evaluateFunc == nil {
		return 0.5, nil
	}

	return f.evaluateFunc(ctx, code)
}

// IsTerminal checks if code passes all tests
func (f *CodeRewardFunction) IsTerminal(ctx context.Context, state interface{}) (bool, error) {
	code, ok := state.(string)
	if !ok {
		code = fmt.Sprintf("%v", state)
	}

	if f.testFunc == nil {
		return false, nil
	}

	return f.testFunc(ctx, code)
}

// DefaultRolloutPolicy implements a simple rollout policy
// Note: Uses math/rand for simulation randomness - this doesn't require cryptographic security
type DefaultRolloutPolicy struct {
	actionGen  MCTSActionGenerator
	rewardFunc MCTSRewardFunction
	rng        *rand.Rand // #nosec G404 - MCTS rollout doesn't require cryptographic randomness
}

// NewDefaultRolloutPolicy creates a new default rollout policy
func NewDefaultRolloutPolicy(actionGen MCTSActionGenerator, rewardFunc MCTSRewardFunction) *DefaultRolloutPolicy {
	return &DefaultRolloutPolicy{
		actionGen:  actionGen,
		rewardFunc: rewardFunc,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())), // #nosec G404 - MCTS rollout uses non-cryptographic randomness
	}
}

// Rollout performs a random rollout from a state
func (p *DefaultRolloutPolicy) Rollout(ctx context.Context, state interface{}, depth int) (float64, error) {
	currentState := state
	totalReward := 0.0
	discount := 1.0

	for i := 0; i < depth; i++ {
		// Check terminal
		isTerminal, err := p.rewardFunc.IsTerminal(ctx, currentState)
		if err != nil || isTerminal {
			break
		}

		// Get actions
		actions, err := p.actionGen.GetActions(ctx, currentState)
		if err != nil || len(actions) == 0 {
			break
		}

		// Random action selection
		action := actions[p.rng.Intn(len(actions))]

		// Apply action
		newState, err := p.actionGen.ApplyAction(ctx, currentState, action)
		if err != nil {
			break
		}

		// Evaluate
		reward, err := p.rewardFunc.Evaluate(ctx, newState)
		if err != nil {
			break
		}

		totalReward += reward * discount
		discount *= 0.99
		currentState = newState
	}

	return totalReward, nil
}
