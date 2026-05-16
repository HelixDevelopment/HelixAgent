# Debate Specification Full Compliance — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement every feature described in the 4 debate research/spec documents to achieve full spec compliance, with 100% test coverage and complete documentation.

**Architecture:** Infrastructure-Then-Features approach. Phase 1 builds shared foundations (DB schema, episodic memory, gates, roles). Phase 2 implements features grouped by dependency. Phase 3 fills all test gaps. Phase 4 updates documentation. Each phase leaves the system fully functional.

**Tech Stack:** Go 1.24+, PostgreSQL/pgx, testify, Gin, Docker/Podman sandboxing, SSE/WebSocket notifications, Mermaid diagrams.

**Design Doc:** `docs/plans/2026-02-24-debate-spec-full-compliance-design.md`

---

## Phase 1: Shared Infrastructure

### Task 1: Database Schema — debate_sessions table

**Files:**
- Create: `sql/schema/debate_sessions.sql`
- Modify: `sql/schema/complete_schema.sql` (add include reference)

**Step 1: Write the migration SQL**

Create `sql/schema/debate_sessions.sql`:

```sql
-- Debate Sessions: lifecycle tracking with full metadata
-- Extends debate_logs with session-level state for replay/recovery

CREATE TABLE IF NOT EXISTS debate_sessions (
    id                    UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    debate_id             VARCHAR(255) NOT NULL,
    topic                 TEXT         NOT NULL,
    status                VARCHAR(50)  NOT NULL DEFAULT 'pending',
    topology_type         VARCHAR(50),
    coordination_protocol VARCHAR(50),
    config                JSONB        DEFAULT '{}',
    initiated_by          VARCHAR(255),
    created_at            TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at            TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at          TIMESTAMP WITH TIME ZONE,
    total_rounds          INTEGER      DEFAULT 0,
    final_consensus_score DECIMAL(5,4),
    outcome               JSONB        DEFAULT '{}',
    metadata              JSONB        DEFAULT '{}'
);

-- Status constraint
ALTER TABLE debate_sessions
    ADD CONSTRAINT chk_debate_sessions_status
    CHECK (status IN ('pending', 'running', 'paused', 'completed', 'failed', 'cancelled'));

-- Indexes
CREATE INDEX IF NOT EXISTS idx_debate_sessions_debate_id ON debate_sessions(debate_id);
CREATE INDEX IF NOT EXISTS idx_debate_sessions_status ON debate_sessions(status);
CREATE INDEX IF NOT EXISTS idx_debate_sessions_created_at ON debate_sessions(created_at);
CREATE INDEX IF NOT EXISTS idx_debate_sessions_topology ON debate_sessions(topology_type);
CREATE INDEX IF NOT EXISTS idx_debate_sessions_active
    ON debate_sessions(status) WHERE status IN ('pending', 'running', 'paused');
CREATE INDEX IF NOT EXISTS idx_debate_sessions_metadata ON debate_sessions USING GIN (metadata);
```

**Step 2: Verify SQL syntax**

Run: `psql -h localhost -p 15432 -U helixagent -d helixagent_db -f sql/schema/debate_sessions.sql`
Expected: CREATE TABLE, ALTER TABLE, CREATE INDEX (no errors)

**Step 3: Commit**

```bash
git add sql/schema/debate_sessions.sql
git commit -m "feat(debate): add debate_sessions table schema"
```

---

### Task 2: Database Schema — debate_turns table

**Files:**
- Create: `sql/schema/debate_turns.sql`

**Step 1: Write the migration SQL**

Create `sql/schema/debate_turns.sql`:

```sql
-- Debate Turns: granular turn-level state for replay/recovery
-- Each turn captures one agent's action in one phase of one round

CREATE TABLE IF NOT EXISTS debate_turns (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id      UUID         NOT NULL REFERENCES debate_sessions(id) ON DELETE CASCADE,
    round           INTEGER      NOT NULL,
    phase           VARCHAR(50)  NOT NULL,
    agent_id        VARCHAR(255) NOT NULL,
    agent_role      VARCHAR(100),
    provider        VARCHAR(100),
    model           VARCHAR(255),
    content         TEXT,
    confidence      DECIMAL(5,4),
    tool_calls      JSONB        DEFAULT '[]',
    test_results    JSONB        DEFAULT '{}',
    reflections     JSONB        DEFAULT '[]',
    metadata        JSONB        DEFAULT '{}',
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    response_time_ms INTEGER
);

-- Phase constraint
ALTER TABLE debate_turns
    ADD CONSTRAINT chk_debate_turns_phase
    CHECK (phase IN (
        'dehallucination', 'self_evolvement', 'proposal', 'critique',
        'review', 'optimization', 'adversarial', 'convergence'
    ));

-- Indexes
CREATE INDEX IF NOT EXISTS idx_debate_turns_session_id ON debate_turns(session_id);
CREATE INDEX IF NOT EXISTS idx_debate_turns_round ON debate_turns(session_id, round);
CREATE INDEX IF NOT EXISTS idx_debate_turns_phase ON debate_turns(phase);
CREATE INDEX IF NOT EXISTS idx_debate_turns_agent ON debate_turns(agent_id);
CREATE INDEX IF NOT EXISTS idx_debate_turns_session_round_phase
    ON debate_turns(session_id, round, phase);
CREATE INDEX IF NOT EXISTS idx_debate_turns_created_at ON debate_turns(created_at);
CREATE INDEX IF NOT EXISTS idx_debate_turns_reflections ON debate_turns USING GIN (reflections)
    WHERE reflections != '[]'::jsonb;
```

**Step 2: Verify SQL syntax**

Run: `psql -h localhost -p 15432 -U helixagent -d helixagent_db -f sql/schema/debate_turns.sql`
Expected: CREATE TABLE, ALTER TABLE, CREATE INDEX (no errors)

**Step 3: Commit**

```bash
git add sql/schema/debate_turns.sql
git commit -m "feat(debate): add debate_turns table schema"
```

---

### Task 3: Database Schema — code_versions table

**Files:**
- Create: `sql/schema/code_versions.sql`

**Step 1: Write the migration SQL**

Create `sql/schema/code_versions.sql`:

```sql
-- Code Versions: snapshots of code at debate milestones
-- Tracks evolution of solutions through debate rounds

CREATE TABLE IF NOT EXISTS code_versions (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id        UUID         NOT NULL REFERENCES debate_sessions(id) ON DELETE CASCADE,
    turn_id           UUID         REFERENCES debate_turns(id) ON DELETE SET NULL,
    language          VARCHAR(50),
    code              TEXT         NOT NULL,
    version_number    INTEGER      NOT NULL,
    quality_score     DECIMAL(5,4),
    test_pass_rate    DECIMAL(5,4),
    metrics           JSONB        DEFAULT '{}',
    diff_from_previous TEXT,
    created_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_code_versions_session_id ON code_versions(session_id);
CREATE INDEX IF NOT EXISTS idx_code_versions_turn_id ON code_versions(turn_id);
CREATE INDEX IF NOT EXISTS idx_code_versions_session_version
    ON code_versions(session_id, version_number);
CREATE INDEX IF NOT EXISTS idx_code_versions_language ON code_versions(language);
CREATE INDEX IF NOT EXISTS idx_code_versions_quality ON code_versions(quality_score)
    WHERE quality_score IS NOT NULL;

-- Unique constraint: one version number per session
ALTER TABLE code_versions
    ADD CONSTRAINT uq_code_versions_session_version
    UNIQUE (session_id, version_number);
```

**Step 2: Verify SQL syntax**

Run: `psql -h localhost -p 15432 -U helixagent -d helixagent_db -f sql/schema/code_versions.sql`
Expected: CREATE TABLE, CREATE INDEX, ALTER TABLE (no errors)

**Step 3: Commit**

```bash
git add sql/schema/code_versions.sql
git commit -m "feat(debate): add code_versions table schema"
```

---

### Task 4: Database Repository — debate_session_repository.go

**Files:**
- Create: `internal/database/debate_session_repository.go`
- Create: `internal/database/debate_session_repository_test.go`

**Step 1: Write the failing test**

Create `internal/database/debate_session_repository_test.go` — test CreateSession, GetSession, UpdateStatus, ListByStatus. Follow pattern from `debate_log_repository.go:59-75` (pgxpool.Pool + logrus.Logger + retention policy). Use testify assertions.

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestDebateSessionRepository -short ./internal/database/ -count=1`
Expected: FAIL — file not found or methods not defined

**Step 3: Write the repository**

Create `internal/database/debate_session_repository.go` following the existing `DebateLogRepository` pattern:
- `DebateSessionRepository` struct with `pool *pgxpool.Pool`, `log *logrus.Logger`
- `DebateSessionEntry` data model matching the SQL table (UUID id, string debate_id, topic, status, topology_type, coordination_protocol, config as string/JSONB, initiated_by, timestamps, total_rounds, final_consensus_score, outcome as string/JSONB, metadata as string/JSONB)
- Methods: `CreateSession(ctx, entry) (string, error)`, `GetSession(ctx, id) (*DebateSessionEntry, error)`, `UpdateStatus(ctx, id, status) error`, `UpdateOutcome(ctx, id, outcome) error`, `ListByStatus(ctx, status) ([]*DebateSessionEntry, error)`, `ListByDebateID(ctx, debateID) ([]*DebateSessionEntry, error)`

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestDebateSessionRepository -short ./internal/database/ -count=1`
Expected: PASS (unit tests with mocked pool or skipped if no infra)

**Step 5: Commit**

```bash
git add internal/database/debate_session_repository.go internal/database/debate_session_repository_test.go
git commit -m "feat(debate): add debate session repository with tests"
```

---

### Task 5: Database Repository — debate_turn_repository.go

**Files:**
- Create: `internal/database/debate_turn_repository.go`
- Create: `internal/database/debate_turn_repository_test.go`

**Step 1-5:** Same TDD pattern as Task 4. Repository for debate_turns table.

- `DebateTurnRepository` struct
- `DebateTurnEntry` data model: UUID id, session_id, round int, phase string, agent_id, agent_role, provider, model, content, confidence float64, tool_calls/test_results/reflections/metadata as string (JSONB), created_at, response_time_ms int
- Methods: `CreateTurn(ctx, entry) (string, error)`, `GetTurn(ctx, id) (*DebateTurnEntry, error)`, `ListBySession(ctx, sessionID) ([]*DebateTurnEntry, error)`, `ListBySessionAndRound(ctx, sessionID, round) ([]*DebateTurnEntry, error)`, `ListBySessionAndPhase(ctx, sessionID, phase) ([]*DebateTurnEntry, error)`, `GetReflections(ctx, sessionID, agentID) ([]string, error)`

**Commit:** `feat(debate): add debate turn repository with tests`

---

### Task 6: Database Repository — code_version_repository.go

**Files:**
- Create: `internal/database/code_version_repository.go`
- Create: `internal/database/code_version_repository_test.go`

**Step 1-5:** Same TDD pattern.

- `CodeVersionRepository` struct
- `CodeVersionEntry` data model matching SQL
- Methods: `CreateVersion(ctx, entry) (string, error)`, `GetVersion(ctx, id) (*CodeVersionEntry, error)`, `GetLatestBySession(ctx, sessionID) (*CodeVersionEntry, error)`, `ListBySession(ctx, sessionID) ([]*CodeVersionEntry, error)`, `GetDiff(ctx, sessionID, fromVersion, toVersion) (string, error)`

**Commit:** `feat(debate): add code version repository with tests`

---

### Task 7: New Role Constants — topology.go updates

**Files:**
- Modify: `internal/debate/topology/topology.go:23-55` (AgentRole enum)

**Step 1: Write the failing test**

Add test in existing or new `topology_roles_test.go` asserting all 21 roles exist and have unique values.

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestAgentRole_AllRolesExist ./internal/debate/topology/ -count=1`
Expected: FAIL — missing role constants

**Step 3: Add missing role constants**

Modify `internal/debate/topology/topology.go` at line ~55 (after `RoleTeacher`), add:

```go
    // Additional specialized roles (from debate spec documents)
    RoleCompiler    AgentRole = "compiler"
    RoleExecutor    AgentRole = "executor"
    RoleJudge       AgentRole = "judge"
    RoleImplementer AgentRole = "implementer"
    RoleDesigner    AgentRole = "designer"
```

Also add `TopologyTree TopologyType = "tree"` after line 20 (TopologyChain).

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestAgentRole_AllRolesExist ./internal/debate/topology/ -count=1`
Expected: PASS

**Step 5: Verify compilation**

Run: `go build ./internal/debate/...`
Expected: No errors

**Step 6: Commit**

```bash
git add internal/debate/topology/topology.go internal/debate/topology/topology_roles_test.go
git commit -m "feat(debate): add Compiler/Executor/Judge/Implementer/Designer roles and Tree topology type"
```

---

### Task 8: New Phase Constants — topology.go phase updates

**Files:**
- Modify: `internal/debate/topology/topology.go` (DebatePhase enum)

**Step 1: Find current DebatePhase enum and add new phases**

Add after existing phases (PhaseProposal, PhaseCritique, PhaseReview, PhaseOptimization, PhaseConvergence):

```go
    PhaseDehallucination DebatePhase = "dehallucination"
    PhaseSelfEvolvement  DebatePhase = "self_evolvement"
    PhaseAdversarial     DebatePhase = "adversarial"
```

**Step 2: Write test asserting all 8 phases exist**

**Step 3: Run tests and verify**

Run: `go test -v ./internal/debate/topology/ -count=1`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/debate/topology/topology.go
git commit -m "feat(debate): add Dehallucination, SelfEvolvement, Adversarial phase constants"
```

---

### Task 9: Missing Role Templates (10 templates)

**Files:**
- Modify: `internal/debate/agents/templates.go:69-82` (registerBuiltInTemplates)

**Step 1: Write failing test**

Create test asserting all 22 templates exist in registry (12 existing + 10 new): Generator, Refactorer, PerformanceAnalyzer, Security (role template, not domain), Teacher, Compiler, Executor, Judge, Implementer, Designer.

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestTemplateRegistry_AllRoleTemplates ./internal/debate/agents/ -count=1`
Expected: FAIL — 10 templates missing

**Step 3: Implement all 10 template factory functions**

Add to `templates.go` following the pattern at lines 233-272 (`NewCodeSpecialistTemplate`). Each template needs:
- `TemplateID` (e.g., `"generator-role"`)
- `Name`, `Description`, `Version "1.0.0"`, `Domain`
- `ExpertiseLevel` (0.80-0.90)
- `RequiredCapabilities`, `OptionalCapabilities`
- `PreferredRoles` (the role this template fills)
- `PreferredProviders` (diverse set)
- `SystemPromptTemplate` (specialized prompt)
- `RequiredTools`, `Tags`

Register all 10 in `registerBuiltInTemplates()` after line 81.

**Generator** — Focus: produce complete, production-ready implementations. Domain: Code. Tools: code executor, formatters. Preferred role: RoleGenerator.

**Refactorer** — Focus: improve structure without changing behavior. Domain: Code. Tools: static analyzers, formatters. Preferred role: RoleRefactorer.

**PerformanceAnalyzer** — Focus: profile, optimize, memory/CPU efficiency. Domain: Optimization. Tools: profilers, benchmarks. Preferred role: RolePerformanceAnalyzer.

**SecurityRole** — Focus: find vulnerabilities, OWASP top 10. Domain: Security. Tools: security scanners. Preferred role: RoleSecurity. (Note: different from SecuritySpecialistTemplate which is domain-level)

**Teacher** — Focus: explain decisions, document rationale. Domain: Reasoning. Tools: documentation. Preferred role: RoleTeacher.

**Compiler** — Focus: validate syntax, type safety, build correctness. Domain: Code. Tools: compilers, go vet. Preferred role: RoleCompiler.

**Executor** — Focus: run code in sandbox, collect metrics. Domain: Code. Tools: Docker executor, test runner. Preferred role: RoleExecutor.

**Judge** — Focus: score solutions objectively against rubric (0-1). Domain: Reasoning. Tools: scoring rubrics. Preferred role: RoleJudge.

**Implementer** — Focus: turn specs into concrete code. Domain: Code. Tools: code executor, LSP. Preferred role: RoleImplementer.

**Designer** — Focus: high-level design, decomposition, interface specs. Domain: Architecture. Tools: design analysis, RAG. Preferred role: RoleDesigner.

**Step 4: Run tests**

Run: `go test -v ./internal/debate/agents/ -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/debate/agents/templates.go internal/debate/agents/templates_test.go
git commit -m "feat(debate): add 10 missing role templates (Generator through Designer)"
```

---

### Task 10: Approval Gate Framework

**Files:**
- Create: `internal/debate/gates/approval_gate.go`
- Create: `internal/debate/gates/approval_gate_test.go`

**Step 1: Write failing test**

Test: create gate with config (enabled=false), call `CheckGate()` → auto-approve. Create gate with config (enabled=true, gate points=[convergence]), call `CheckGate(convergence)` → returns GateRequest. Call `Approve(requestID)` → debate resumes. Call `Reject(requestID)` → debate stopped. Test timeout auto-rejection.

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestApprovalGate ./internal/debate/gates/ -count=1`
Expected: FAIL — package doesn't exist

**Step 3: Implement approval gate**

```go
package gates

type GateConfig struct {
    Enabled              bool
    GatePoints           []topology.DebatePhase
    Timeout              time.Duration // default 30min
    NotificationChannels []string      // "sse", "websocket"
}

type GateRequest struct {
    ID        string
    DebateID  string
    SessionID string
    Phase     topology.DebatePhase
    Summary   string
    Artifacts map[string]interface{}
    RequestedAt time.Time
    Status    GateRequestStatus // pending, approved, rejected, timed_out
}

type GateDecision struct {
    RequestID string
    Decision  GateRequestStatus
    Reviewer  string
    Reason    string
    DecidedAt time.Time
}

type ApprovalGate struct {
    config   GateConfig
    requests map[string]*GateRequest
    decisions chan GateDecision
    mu       sync.RWMutex
}

func NewApprovalGate(config GateConfig) *ApprovalGate
func DefaultGateConfig() GateConfig // Enabled: false
func (g *ApprovalGate) CheckGate(ctx context.Context, debateID, sessionID string, phase topology.DebatePhase, summary string, artifacts map[string]interface{}) (*GateDecision, error)
func (g *ApprovalGate) Approve(requestID, reviewer, reason string) error
func (g *ApprovalGate) Reject(requestID, reviewer, reason string) error
func (g *ApprovalGate) GetPendingRequests(debateID string) []*GateRequest
func (g *ApprovalGate) isGatePoint(phase topology.DebatePhase) bool
```

When `config.Enabled == false` or phase not in GatePoints: return auto-approved decision immediately.
When enabled: create GateRequest, store it, wait on decisions channel with timeout.

**Step 4: Run tests**

Run: `go test -v ./internal/debate/gates/ -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/debate/gates/
git commit -m "feat(debate): add configurable approval gate framework"
```

---

## Phase 2: Feature Implementation

### Group A: Independent Features

### Task 11: Tree Topology

**Files:**
- Create: `internal/debate/topology/tree.go`
- Create: `internal/debate/topology/tree_test.go`
- Modify: `internal/debate/topology/factory.go:9-23` (add Tree case)

**Step 1: Write failing test**

Test: create TreeTopology with 7 agents (1 Architect root, 2 leads, 4 specialists). Assert: root has 2 children, each lead has 2 children. Test `RouteMessage` up (leaf→root) and down (root→leaf). Test `GetParallelGroups` returns independent subtrees. Test `SelectLeader` returns appropriate node per phase. Test rebalancing when agent fails.

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestTreeTopology ./internal/debate/topology/ -count=1`
Expected: FAIL

**Step 3: Implement TreeTopology**

Create `tree.go`:

```go
package topology

type TreeNode struct {
    AgentID    string
    Role       AgentRole
    Parent     *TreeNode
    Children   []*TreeNode
    Level      int
    Subtree    string // responsibility area
}

type TreeTopology struct {
    BaseTopology
    root     *TreeNode
    nodes    map[string]*TreeNode
    maxDepth int
}

func NewTreeTopology(config TopologyConfig) *TreeTopology
func (t *TreeTopology) GetType() TopologyType { return TopologyTree }
func (t *TreeTopology) Initialize(ctx context.Context) error // BuildTree from agents
func (t *TreeTopology) BuildTree(agents map[string]*Agent) *TreeNode
func (t *TreeTopology) RouteMessage(msg *Message) error // up/down routing
func (t *TreeTopology) BroadcastMessage(msg *Message) error // flood all nodes
func (t *TreeTopology) SelectLeader(phase DebatePhase) (string, error)
func (t *TreeTopology) GetParallelGroups(phase DebatePhase) [][]string
func (t *TreeTopology) GetCommunicationTargets(agentID string) []string // parent + children
func (t *TreeTopology) Rebalance(failedAgentID string) error
```

Tree construction: Architect at root (level 0). security/Performance leads at level 1. Remaining specialists at level 2, distributed evenly across leads. Message routing: messages go to parent (escalation) or children (delegation). Siblings communicate through shared parent.

**Step 4: Update factory**

Modify `factory.go` line ~14, add case:
```go
case TopologyTree:
    return NewTreeTopology(config), nil
```

Update `SelectTopologyType` to consider tree for hierarchical requirements.

**Step 5: Run tests**

Run: `go test -v ./internal/debate/topology/ -count=1`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/debate/topology/tree.go internal/debate/topology/tree_test.go internal/debate/topology/factory.go
git commit -m "feat(debate): implement Tree topology with hierarchical routing"
```

---

### Task 12: Planning Styles (CPDE/DPDE)

**Files:**
- Create: `internal/debate/topology/planning.go`
- Create: `internal/debate/topology/planning_test.go`

**Step 1: Write failing test**

Test CPDE: root creates plan, distributes tasks, leaf agents execute independently. Test DPDE: each agent plans locally, negotiates with peers. Test auto-selection: well-defined task → CPDE, exploratory task → DPDE. Test integration with CognitivePlanner.

**Step 2-4: Implement and test**

```go
package topology

type PlanningStyle string

const (
    PlanningCPDE PlanningStyle = "cpde" // Centralized Planning, Decentralized Execution
    PlanningDPDE PlanningStyle = "dpde" // Decentralized Planning, Decentralized Execution
)

type PlanningStyleSelector struct {
    defaultStyle PlanningStyle
}

type TaskPlan struct {
    PlannerAgentID string
    Style          PlanningStyle
    Tasks          []*PlannedTask
    Ordering       []string // execution order
    CreatedAt      time.Time
}

type PlannedTask struct {
    ID           string
    Description  string
    AssignedTo   string // agent ID
    Dependencies []string
    Priority     int
}

func NewPlanningStyleSelector(defaultStyle PlanningStyle) *PlanningStyleSelector
func (s *PlanningStyleSelector) SelectStyle(taskComplexity float64, ambiguity float64, agentCount int) PlanningStyle
func CreateCentralizedPlan(ctx context.Context, planner *Agent, agents []*Agent, task string) (*TaskPlan, error)
func CreateDecentralizedPlan(ctx context.Context, agents []*Agent, task string) (*TaskPlan, error)
```

Selection logic: complexity > 0.7 AND ambiguity < 0.3 → CPDE. ambiguity > 0.6 → DPDE. Otherwise use default.

**Step 5: Commit**

```bash
git add internal/debate/topology/planning.go internal/debate/topology/planning_test.go
git commit -m "feat(debate): implement CPDE/DPDE planning styles with auto-selection"
```

---

### Task 13: Borda Count Voting (complete stub)

**Files:**
- Modify: `internal/debate/voting/weighted_voting.go:574-629`

**Step 1: Write failing test**

Test Borda with 5 agents ranking 3 candidates. Verify point allocation: rank 1 = N-1 points, rank 2 = N-2, etc. Test tie scenarios. Test integration with existing tie-breaking strategies.

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestWeightedVotingSystem_BordaCount ./internal/debate/voting/ -count=1`
Expected: FAIL or incomplete results

**Step 3: Complete Borda implementation**

Replace stub at lines 574-629 with full implementation:
- Validate all rankings reference known candidates
- Calculate Borda scores: for each voter's ranking, give position i score (N - i - 1) where N = number of candidates
- Sum scores per candidate
- Find winner (highest total score)
- Build VotingResult with VotingMethodBorda, candidate scores, consensus level
- Apply tie-breaking if needed

**Step 4: Run tests**

Run: `go test -v -run TestWeightedVotingSystem_Borda ./internal/debate/voting/ -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/debate/voting/weighted_voting.go internal/debate/voting/weighted_voting_test.go
git commit -m "feat(debate): complete Borda count voting implementation"
```

---

### Task 14: Condorcet Voting (new)

**Files:**
- Modify: `internal/debate/voting/weighted_voting.go` (add after Borda)

**Step 1: Write failing test**

Test Condorcet with clear winner (beats all others head-to-head). Test Condorcet cycle (no winner) → falls back to Borda with FallbackUsed flag. Test pairwise matrix construction. Test with single candidate. Test with 2 candidates.

**Step 2: Implement Condorcet**

Add to `weighted_voting.go`:

```go
type CondorcetMatrix struct {
    Candidates []string
    Wins       map[string]map[string]int // wins[A][B] = number of voters preferring A over B
}

func (wvs *WeightedVotingSystem) CalculateCondorcet(ctx context.Context, rankings map[string][]string) (*VotingResult, error)
func (wvs *WeightedVotingSystem) buildCondorcetMatrix(rankings map[string][]string) *CondorcetMatrix
func (wvs *WeightedVotingSystem) findCondorcetWinner(matrix *CondorcetMatrix) (string, bool)
```

Algorithm: Build pairwise matrix. For each pair (A, B), count voters preferring A over B. Condorcet winner = candidate where wins[winner][X] > wins[X][winner] for ALL X. If no winner found (cycle), call CalculateBordaCount as fallback.

**Step 3: Run tests**

Run: `go test -v -run TestWeightedVotingSystem_Condorcet ./internal/debate/voting/ -count=1`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/debate/voting/weighted_voting.go internal/debate/voting/weighted_voting_test.go
git commit -m "feat(debate): implement Condorcet voting with cycle detection and Borda fallback"
```

---

### Task 15: Plurality and Unanimous Voting (ensure complete)

**Files:**
- Modify: `internal/debate/voting/weighted_voting.go`

**Step 1: Write tests for both**

Test Plurality: most first-place votes wins (no ranking needed). Test Unanimous: all must agree, returns ConsensusReached=false with disagreements if not.

**Step 2: Verify/complete implementations**

`CalculatePlurality`: count first-choice votes per candidate, winner = most votes.
`CalculateUnanimous`: check if all votes agree, return result with ConsensusReached flag.

**Step 3: Add voting method auto-selection**

```go
func (wvs *WeightedVotingSystem) AutoSelectMethod(agentCount int) VotingMethod {
    if agentCount < 3 {
        return VotingMethodUnanimous
    }
    if agentCount <= 5 {
        return VotingMethodWeighted
    }
    return VotingMethodBorda
}
```

**Step 4: Run tests and commit**

```bash
git commit -m "feat(debate): complete Plurality/Unanimous voting and add auto-selection"
```

---

### Group B: Depends on Episodic Memory

### Task 16: Episodic Memory Buffer

**Files:**
- Create: `internal/debate/reflexion/episodic_memory.go`
- Create: `internal/debate/reflexion/episodic_memory_test.go`

**Step 1: Write failing test**

Test: store episode, retrieve by agent, retrieve by session, FIFO eviction at max size, persistence to JSONB, similarity-based retrieval.

**Step 2: Implement**

```go
package reflexion

type Episode struct {
    ID              string
    SessionID       string
    TurnID          string
    AgentID         string
    TaskDescription string
    AttemptNumber   int
    Code            string
    TestResults     map[string]interface{}
    FailureAnalysis string
    Reflection      *Reflection
    Improvement     string
    Confidence      float64
    Timestamp       time.Time
}

type EpisodicMemoryBuffer struct {
    episodes    []*Episode
    byAgent     map[string][]*Episode
    bySession   map[string][]*Episode
    maxSize     int
    mu          sync.RWMutex
}

func NewEpisodicMemoryBuffer(maxSize int) *EpisodicMemoryBuffer
func (b *EpisodicMemoryBuffer) Store(episode *Episode) error
func (b *EpisodicMemoryBuffer) GetByAgent(agentID string) []*Episode
func (b *EpisodicMemoryBuffer) GetBySession(sessionID string) []*Episode
func (b *EpisodicMemoryBuffer) GetRecent(n int) []*Episode
func (b *EpisodicMemoryBuffer) GetRelevant(taskDescription string, limit int) []*Episode
func (b *EpisodicMemoryBuffer) Size() int
func (b *EpisodicMemoryBuffer) Clear()
func (b *EpisodicMemoryBuffer) MarshalJSON() ([]byte, error) // for DB persistence
func (b *EpisodicMemoryBuffer) UnmarshalJSON(data []byte) error
```

Max size default: 1000 episodes. FIFO: when full, remove oldest. Relevance: keyword overlap between task descriptions (simple matching; embedding-based is optional enhancement).

**Step 3: Run tests and commit**

```bash
git commit -m "feat(debate): implement episodic memory buffer with FIFO eviction"
```

---

### Task 17: Reflection Generator

**Files:**
- Create: `internal/debate/reflexion/reflection_generator.go`
- Create: `internal/debate/reflexion/reflection_generator_test.go`

**Step 1: Write failing test**

Test: given failed code + test results + error messages, generate structured Reflection. Test with no prior reflections. Test with accumulated reflections in context. Test fallback when LLM unavailable.

**Step 2: Implement**

```go
type Reflection struct {
    RootCause        string
    WhatWentWrong    string
    WhatToChangeNext string
    ConfidenceInFix  float64
    GeneratedAt      time.Time
}

type ReflectionGenerator struct {
    llmClient LLMClient // reuse from testing package
}

func NewReflectionGenerator(llmClient LLMClient) *ReflectionGenerator
func (g *ReflectionGenerator) Generate(ctx context.Context, req *ReflectionRequest) (*Reflection, error)
func (g *ReflectionGenerator) buildReflectionPrompt(req *ReflectionRequest) string
func (g *ReflectionGenerator) parseReflectionResponse(response string) (*Reflection, error)
func (g *ReflectionGenerator) generateFallbackReflection(req *ReflectionRequest) *Reflection

type ReflectionRequest struct {
    Code               string
    TestResults        map[string]interface{}
    ErrorMessages      []string
    PriorReflections   []*Reflection
    TaskDescription    string
    AttemptNumber      int
}
```

Prompt template: "You are analyzing a failed code attempt. Code: {code}. Test results: {results}. Errors: {errors}. Previous reflections: {prior}. Analyze: 1) Root cause of failure. 2) What went wrong. 3) What should change in the next attempt. 4) Your confidence (0-1) this fix will work."

**Step 3: Run tests and commit**

```bash
git commit -m "feat(debate): implement verbal reflection generator for Reflexion framework"
```

---

### Task 18: Reflexion Loop

**Files:**
- Create: `internal/debate/reflexion/reflexion_loop.go`
- Create: `internal/debate/reflexion/reflexion_loop_test.go`

**Step 1: Write failing test**

Test: execute loop where first attempt fails, reflection generated, second attempt passes. Test max attempts reached. Test confidence threshold termination. Test accumulated reflections grow with each attempt.

**Step 2: Implement**

```go
type ReflexionConfig struct {
    MaxAttempts         int           // default 3
    ConfidenceThreshold float64       // default 0.95
    Timeout             time.Duration // default 5min
}

type ReflexionLoop struct {
    config    ReflexionConfig
    generator *ReflectionGenerator
    executor  *testing.SandboxedTestExecutor
    memory    *EpisodicMemoryBuffer
}

type ReflexionResult struct {
    FinalCode       string
    Attempts        int
    AllPassed       bool
    Reflections     []*Reflection
    Episodes        []*Episode
    FinalConfidence float64
    Duration        time.Duration
}

func NewReflexionLoop(config ReflexionConfig, generator *ReflectionGenerator, executor *testing.SandboxedTestExecutor, memory *EpisodicMemoryBuffer) *ReflexionLoop
func (l *ReflexionLoop) Execute(ctx context.Context, task *ReflexionTask) (*ReflexionResult, error)

type ReflexionTask struct {
    Description    string
    InitialCode    string
    TestCases      []*testing.TestCase
    Language       string
    AgentID        string
    SessionID      string
    CodeGenerator  func(ctx context.Context, task string, priorReflections []*Reflection) (string, error)
}
```

Loop: Generate code (using CodeGenerator callback with accumulated reflections) → Execute tests → If all pass: return success. If fail: Generate reflection → Store episode in memory → Increment attempt → Loop. Stop at MaxAttempts or ConfidenceThreshold.

**Step 3: Run tests and commit**

```bash
git commit -m "feat(debate): implement Reflexion loop with retry-and-learn cycle"
```

---

### Task 19: Accumulated Wisdom (Cross-Session Learning)

**Files:**
- Create: `internal/debate/reflexion/accumulated_wisdom.go`
- Create: `internal/debate/reflexion/accumulated_wisdom_test.go`

**Step 1: Write failing test**

Test: extract wisdom from episodes, store as reusable patterns, retrieve relevant wisdom for new task.

**Step 2: Implement**

```go
type Wisdom struct {
    ID          string
    Pattern     string  // generalized insight
    Source      string  // extracted from which episodes
    Frequency   int     // how often this pattern appeared
    Impact      float64 // how much it improved outcomes
    Domain      string
    CreatedAt   time.Time
    LastUsedAt  time.Time
}

type AccumulatedWisdom struct {
    insights   []*Wisdom
    repository knowledge.Repository // link to existing knowledge system
    mu         sync.RWMutex
}

func NewAccumulatedWisdom(repo knowledge.Repository) *AccumulatedWisdom
func (w *AccumulatedWisdom) ExtractFromEpisodes(episodes []*Episode) ([]*Wisdom, error)
func (w *AccumulatedWisdom) GetRelevant(taskDescription string, limit int) []*Wisdom
func (w *AccumulatedWisdom) Store(wisdom *Wisdom) error
func (w *AccumulatedWisdom) RecordUsage(wisdomID string, success bool) error
```

Extraction: group episodes by similar RootCause, generalize into patterns, weight by frequency and impact.

**Step 3: Run tests and commit**

```bash
git commit -m "feat(debate): implement accumulated wisdom for cross-session learning"
```

---

### Group C: Depends on Roles

### Task 20: Adversarial Dynamics (Red/Blue Team)

**Files:**
- Create: `internal/debate/agents/adversarial.go`
- Create: `internal/debate/agents/adversarial_test.go`

**Step 1: Write failing test**

Test: Red Team generates attack report against solution. Blue Team produces defense. Red re-attacks after defense. Protocol terminates when Red finds no new issues. Test max rounds.

**Step 2: Implement**

```go
type AttackReport struct {
    Vulnerabilities []Vulnerability
    EdgeCases       []EdgeCase
    StressScenarios []StressScenario
    OverallRisk     float64 // 0-1
}

type Vulnerability struct {
    ID          string
    Category    string // injection, overflow, race_condition, etc.
    Severity    string // critical, high, medium, low
    Description string
    Evidence    string
    Exploit     string // how to exploit
}

type DefenseReport struct {
    PatchedVulnerabilities []string // IDs from AttackReport
    Patches                map[string]string // vulnerability ID -> fix
    RemainingRisks         []string
    ConfidenceInDefense    float64
}

type AdversarialProtocol struct {
    maxRounds    int // default 3
    redTeam      *SpecializedAgent
    blueTeam     *SpecializedAgent
    llmClient    LLMClient
}

func NewAdversarialProtocol(redTeam, blueTeam *SpecializedAgent, llmClient LLMClient, maxRounds int) *AdversarialProtocol
func (p *AdversarialProtocol) Execute(ctx context.Context, solution string, language string) (*AdversarialResult, error)
func (p *AdversarialProtocol) attack(ctx context.Context, solution string, priorDefense *DefenseReport) (*AttackReport, error)
func (p *AdversarialProtocol) defend(ctx context.Context, solution string, attackReport *AttackReport) (*DefenseReport, string, error) // returns patched code

type AdversarialResult struct {
    Rounds          int
    FinalCode       string
    AttackReports   []*AttackReport
    DefenseReports  []*DefenseReport
    AllResolved     bool
    RemainingRisks  []string
}
```

**Step 3: Run tests and commit**

```bash
git commit -m "feat(debate): implement Red/Blue Team adversarial dynamics protocol"
```

---

### Task 21: Communicative Dehallucination Phase

**Files:**
- Create: `internal/debate/protocol/dehallucination.go`
- Create: `internal/debate/protocol/dehallucination_test.go`

**Step 1: Write failing test**

Test: instructor sends task → assistant asks clarification → instructor answers → assistant confirms understanding (confidence > 0.9). Test max rounds (3). Test with already-clear task (skip).

**Step 2: Implement**

```go
type ClarificationRequest struct {
    Question string
    Category string // requirements, constraints, edge_cases, performance, integration
    Priority int    // 1-5
}

type ClarificationResponse struct {
    Answer              string
    Confidence          float64
    RemainingAmbiguities []string
}

type DehallucationConfig struct {
    Enabled             bool
    MaxClarificationRounds int     // default 3
    ConfidenceThreshold    float64 // default 0.9
}

type DehallucationPhase struct {
    config    DehallucationConfig
    llmClient LLMClient
}

func NewDehallucationPhase(config DehallucationConfig, llmClient LLMClient) *DehallucationPhase
func (d *DehallucationPhase) Execute(ctx context.Context, task string, context map[string]interface{}) (*DehallucationResult, error)
func (d *DehallucationPhase) generateClarifications(ctx context.Context, task string, priorAnswers []ClarificationResponse) ([]ClarificationRequest, float64, error)
func (d *DehallucationPhase) answerClarifications(ctx context.Context, task string, questions []ClarificationRequest) (*ClarificationResponse, error)

type DehallucationResult struct {
    ClarifiedTask     string
    ClarificationRounds int
    FinalConfidence   float64
    Clarifications    []ClarificationRequest
    Responses         []ClarificationResponse
    Skipped           bool // true if confidence was already high
}
```

**Step 3: Run tests and commit**

```bash
git commit -m "feat(debate): implement Communicative Dehallucination pre-debate phase"
```

---

### Task 22: Self-Evolvement Phase

**Files:**
- Create: `internal/debate/protocol/self_evolvement.go`
- Create: `internal/debate/protocol/self_evolvement_test.go`

**Step 1: Write failing test**

Test: agent generates solution → generates tests against own solution → runs in sandbox → finds failures → refines → refined solution returned. Test max iterations. Test no-failure path (solution passes immediately).

**Step 2: Implement**

```go
type SelfEvolvementConfig struct {
    Enabled          bool
    MaxIterations    int     // default 2
    Timeout          time.Duration
}

type SelfEvolvementPhase struct {
    config        SelfEvolvementConfig
    testGenerator *testing.LLMTestCaseGenerator
    testExecutor  *testing.SandboxedTestExecutor
    llmClient     LLMClient
}

func NewSelfEvolvementPhase(config SelfEvolvementConfig, generator *testing.LLMTestCaseGenerator, executor *testing.SandboxedTestExecutor, llmClient LLMClient) *SelfEvolvementPhase
func (s *SelfEvolvementPhase) Execute(ctx context.Context, agentID string, initialSolution string, task string, language string) (*SelfEvolvementResult, error)

type SelfEvolvementResult struct {
    FinalSolution    string
    Iterations       int
    TestResults      []*testing.TestExecutionResult
    Improvements     []string
    FinalPassRate    float64
}
```

Loop: Generate self-tests → Execute → Analyze failures → If all pass or max iterations: return. Else: call LLM with failures to produce refined code → loop.

**Step 3: Run tests and commit**

```bash
git commit -m "feat(debate): implement Self-Evolvement pre-debate validation phase"
```

---

### Task 23: Protocol Integration — wire new phases into protocol.go

**Files:**
- Modify: `internal/debate/protocol/protocol.go:223-340` (initializePhaseConfigs)
- Modify: `internal/debate/protocol/protocol.go:351-406` (Execute)

**Step 1: Write failing test**

Test full 8-phase execution: Dehallucination → SelfEvolvement → Proposal → Critique → Review → Optimization → Adversarial → Convergence. Verify phases execute in order. Test skipping disabled phases.

**Step 2: Update initializePhaseConfigs**

Add configs for 3 new phases before the existing 5:
- `PhaseDehallucination`: timeout, min_responses=1, required_roles=[RoleModerator]
- `PhaseSelfEvolvement`: timeout, min_responses per agent, required_roles=[all participants]
- `PhaseAdversarial` (after Optimization): required_roles=[RoleRedTeam, RoleBlueTeam]

**Step 3: Update Execute loop**

Change phase iteration order from 5 phases to 8 phases:
```go
phases := []topology.DebatePhase{
    topology.PhaseDehallucination,
    topology.PhaseSelfEvolvement,
    topology.PhaseProposal,
    topology.PhaseCritique,
    topology.PhaseReview,
    topology.PhaseOptimization,
    topology.PhaseAdversarial,
    topology.PhaseConvergence,
}
```

Each phase checks if enabled (via PhaseConfig existence). Skip if not configured.

**Step 4: Run tests**

Run: `go test -v ./internal/debate/protocol/ -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/debate/protocol/protocol.go internal/debate/protocol/protocol_test.go
git commit -m "feat(debate): wire 8-phase protocol (dehallucination through convergence)"
```

---

### Group D: External Integrations

### Task 24: Benchmark Bridge

**Files:**
- Create: `internal/debate/evaluation/benchmark_bridge.go`
- Create: `internal/debate/evaluation/benchmark_bridge_test.go`

**Step 1-3: TDD implementation**

```go
package evaluation

type BenchmarkBridge struct {
    // Links to Benchmark module
}

type EvaluationScore struct {
    BenchmarkType string
    Score         float64
    Details       map[string]float64 // correctness, maintainability, performance, security, test_coverage
    Timestamp     time.Time
}

type DebateBenchmarkSuite struct {
    bridge      *BenchmarkBridge
    problems    []*BenchmarkProblem
}

type BenchmarkProblem struct {
    ID          string
    Type        string // swe_bench, human_eval, mmlu, custom
    Description string
    TestCases   []string
    Expected    string
}

func NewBenchmarkBridge() *BenchmarkBridge
func (b *BenchmarkBridge) EvaluateDebateResult(result *protocol.DebateResult, benchmarkType string) (*EvaluationScore, error)
func (b *BenchmarkBridge) CalculateCustomMetrics(code string, language string) (map[string]float64, error)
func NewDebateBenchmarkSuite(bridge *BenchmarkBridge) *DebateBenchmarkSuite
func (s *DebateBenchmarkSuite) Run(ctx context.Context, problems []*BenchmarkProblem) ([]*EvaluationScore, error)
```

**Step 4: Commit**

```bash
git commit -m "feat(debate): add benchmark bridge for SWE-bench/HumanEval evaluation"
```

---

### Task 25: Git Tool

**Files:**
- Create: `internal/debate/tools/git_tool.go`
- Create: `internal/debate/tools/git_tool_test.go`

**Step 1-3: TDD implementation**

```go
type GitTool struct {
    workDir    string
    worktreeDir string
}

func NewGitTool(repoDir string) *GitTool
func (g *GitTool) CreateWorktree(sessionID string) (string, error) // creates debate/<session-id> branch
func (g *GitTool) CommitSnapshot(worktreeDir string, code string, message string) error
func (g *GitTool) CreateDiff(worktreeDir string, fromRef, toRef string) (string, error)
func (g *GitTool) Cleanup(worktreeDir string) error
```

Uses `exec.Command("git", ...)` for worktree operations. Each debate session gets isolated worktree.

**Step 4: Commit**

```bash
git commit -m "feat(debate): add Git worktree tool for debate version control"
```

---

### Task 26: CI/CD Hooks

**Files:**
- Create: `internal/debate/tools/cicd_hook.go`
- Create: `internal/debate/tools/cicd_hook_test.go`

**Step 1-3: TDD implementation**

```go
type HookPoint string

const (
    HookPostProposal    HookPoint = "post_proposal"
    HookPostOptimization HookPoint = "post_optimization"
    HookPostAdversarial HookPoint = "post_adversarial"
    HookPostConvergence HookPoint = "post_convergence"
)

type HookAction string

const (
    ActionRunTests      HookAction = "run_tests"
    ActionRunLinter     HookAction = "run_linter"
    ActionStaticAnalysis HookAction = "static_analysis"
    ActionSecurityScan  HookAction = "security_scan"
    ActionRunBenchmarks HookAction = "run_benchmarks"
)

type CICDHook struct {
    hookPoints map[HookPoint][]HookAction
    executor   *testing.SandboxedTestExecutor
    enabled    bool
}

type HookResult struct {
    HookPoint HookPoint
    Actions   map[HookAction]*ActionResult
    AllPassed bool
}

type ActionResult struct {
    Action  HookAction
    Passed  bool
    Output  string
    Details map[string]interface{}
}

func NewCICDHook(executor *testing.SandboxedTestExecutor) *CICDHook
func (h *CICDHook) Configure(hookPoint HookPoint, actions []HookAction)
func (h *CICDHook) Execute(ctx context.Context, hookPoint HookPoint, code string, language string) (*HookResult, error)
func (h *CICDHook) Enable() / Disable()
```

**Step 4: Commit**

```bash
git commit -m "feat(debate): add CI/CD hook system for debate validation pipelines"
```

---

### Task 27: Provenance & Audit Trail

**Files:**
- Create: `internal/debate/audit/provenance.go`
- Create: `internal/debate/audit/provenance_test.go`

**Step 1-3: TDD implementation**

```go
package audit

type AuditEntry struct {
    Timestamp   time.Time
    EventType   string // prompt_sent, response_received, tool_called, vote_cast, gate_decision, reflection_generated
    AgentID     string
    Phase       string
    Round       int
    Data        map[string]interface{} // full details
}

type AuditTrail struct {
    SessionID string
    Entries   []*AuditEntry
    Summary   *AuditSummary
}

type AuditSummary struct {
    TotalPrompts    int
    TotalResponses  int
    TotalToolCalls  int
    TotalVotes      int
    TotalReflections int
    ModelsUsed      []string
    ProvidersUsed   []string
    Duration        time.Duration
}

type ProvenanceTracker struct {
    entries map[string][]*AuditEntry // keyed by session ID
    mu      sync.RWMutex
}

func NewProvenanceTracker() *ProvenanceTracker
func (t *ProvenanceTracker) Record(sessionID string, entry *AuditEntry)
func (t *ProvenanceTracker) GetAuditTrail(sessionID string) *AuditTrail
func (t *ProvenanceTracker) MarshalJSON(sessionID string) ([]byte, error) // for DB storage
```

**Step 4: Commit**

```bash
git commit -m "feat(debate): add provenance tracker and audit trail for debate reproducibility"
```

---

### Task 28: Handler Updates — new endpoints

**Files:**
- Modify: `internal/handlers/debate_handler.go`

**Step 1: Add approval gate endpoints**

Add to DebateHandler:
- `ApproveDebate(c *gin.Context)` — `POST /v1/debates/:id/approve`
- `RejectDebate(c *gin.Context)` — `POST /v1/debates/:id/reject`
- `GetDebateGates(c *gin.Context)` — `GET /v1/debates/:id/gates`
- `GetDebateAudit(c *gin.Context)` — `GET /v1/debates/:id/audit`

**Step 2: Update CreateDebateRequest**

Add fields: `TopologyType`, `CoordinationProtocol`, `EnableAdversarial`, `EnableDehallucination`, `EnableSelfEvolvement`, `EnableApprovalGates`, `VotingMethod`, `ReflexionConfig`.

**Step 3: Register routes in router**

**Step 4: Run tests and commit**

```bash
git commit -m "feat(debate): add approval gate and audit trail API endpoints"
```

---

### Task 29: Service Integration — wire everything into DebateService

**Files:**
- Modify: `internal/services/debate_service.go`

**Step 1: Add new dependencies to DebateService struct**

Add fields for: `reflexionLoop`, `adversarialProtocol`, `dehallucationPhase`, `selfEvolvementPhase`, `approvalGate`, `provenanceTracker`, `benchmarkBridge`, `sessionRepo`, `turnRepo`, `codeVersionRepo`.

**Step 2: Update NewDebateServiceWithDeps**

Initialize new components with appropriate configs.

**Step 3: Update ConductDebate to use new phases**

Wire the 8-phase flow: check dehallucination → self-evolvement → existing debate → adversarial → gate check → persist to DB.

**Step 4: Run tests and commit**

```bash
git commit -m "feat(debate): integrate Reflexion, adversarial, gates, and persistence into DebateService"
```

---

## Phase 3: Test Completeness

### Task 30: Unit Tests — agents package

**Files:**
- Create: `internal/debate/agents/factory_test.go`
- Create: `internal/debate/agents/specialization_test.go`
- Create: `internal/debate/agents/templates_test.go` (expand existing or create)

Test: AgentFactory creation, AgentPool add/get/select, team building, all 22 templates exist, template creation from each factory function, specialization scoring.

**Commit:** `test(debate): add unit tests for agents package (factory, specialization, templates)`

---

### Task 31: Unit Tests — cognitive package

**Files:**
- Create: `internal/debate/cognitive/cognitive_planning_test.go`

Test: SetExpectation, Compare deltas, Refine adjustments, learning history accumulation, meta-cognition metrics, baseline updates.

**Commit:** `test(debate): add unit tests for cognitive planning`

---

### Task 32: Unit Tests — knowledge package

**Files:**
- Create: `internal/debate/knowledge/repository_test.go`
- Create: `internal/debate/knowledge/integration_test.go`

Test: ExtractLessons, SearchLessons, GetPatterns, RecordPattern (dedup), GetSuccessfulStrategies, CrossDebateLearner.

**Commit:** `test(debate): add unit tests for knowledge repository and integration`

---

### Task 33: Unit Tests — testing package

**Files:**
- Create: `internal/debate/testing/contrastive_analyzer_test.go`
- Create: `internal/debate/testing/protocol_integration_test.go`
- Create: `internal/debate/testing/test_case_generator_test.go`
- Create: `internal/debate/testing/test_executor_test.go`

Test: DifferentialContrastiveAnalyzer with different/same results, root cause detection patterns, LLMTestCaseGenerator with mock LLM, SandboxedTestExecutor config, DebateTestIntegration 5-step round.

**Commit:** `test(debate): add unit tests for testing package (analyzer, generator, executor, integration)`

---

### Task 34: Unit Tests — tools package

**Files:**
- Create: `internal/debate/tools/service_bridge_test.go`
- Create: `internal/debate/tools/tool_integration_test.go`

Test: ServiceBridge adapter creation, health checks, context enrichment, ToolIntegration enable/disable, tool listing aggregation.

**Commit:** `test(debate): add unit tests for tools package (service bridge, tool integration)`

---

### Task 35: Unit Tests — topology package (individual topologies)

**Files:**
- Create: `internal/debate/topology/chain_test.go`
- Create: `internal/debate/topology/graph_mesh_test.go`
- Create: `internal/debate/topology/star_test.go`
- Create: `internal/debate/topology/factory_test.go`

Test each topology: initialization, message routing, leader selection, parallel groups, phase transitions, communication targets.

**Commit:** `test(debate): add unit tests for all topology implementations and factory`

---

### Task 36: Integration Tests

**Files:**
- Create: `tests/integration/debate_reflexion_integration_test.go`
- Create: `tests/integration/debate_adversarial_integration_test.go`
- Create: `tests/integration/debate_tree_topology_integration_test.go`
- Create: `tests/integration/debate_cross_learning_integration_test.go`
- Create: `tests/integration/debate_approval_gate_integration_test.go`
- Create: `tests/integration/debate_full_protocol_integration_test.go`
- Create: `tests/integration/debate_persistence_integration_test.go`
- Create: `tests/integration/debate_tool_integration_test.go`

Follow pattern from `tests/integration/debate_integration_test.go`: `skipIfNoInfra`, testify assertions, real service creation, context with timeout.

Each test file covers its specific scenario end-to-end:
- Reflexion: failure → reflection → retry → success
- Adversarial: Red attack → Blue defend → re-attack
- Tree: hierarchical message routing
- Cross-learning: debate A lessons improve debate B
- Gate: pause → approve/reject → resume
- Full protocol: all 8 phases
- Persistence: DB write → read → verify
- Tool: MCP/LSP/RAG calls from debate

**Commit:** `test(debate): add 8 integration test files for new debate features`

---

### Task 37: E2E Tests

**Files:**
- Create: `tests/e2e/debate_full_workflow_e2e_test.go`
- Create: `tests/e2e/debate_concurrent_e2e_test.go`
- Create: `tests/e2e/debate_recovery_e2e_test.go`

Full HTTP API flow: POST /v1/debates → monitor → GET result → verify DB. Concurrent: 5+ simultaneous debates. Recovery: start debate → simulate restart → verify session recoverable.

**Commit:** `test(debate): add E2E tests for full workflow, concurrency, and recovery`

---

### Task 38: Security Tests

**Files:**
- Create: `tests/security/debate_security_test.go`

Tests: prompt injection in debate params, code injection in responses, resource exhaustion in sandbox, information leakage between debates, malformed provider responses, debate poisoning.

**Commit:** `test(debate): add security tests for debate system`

---

### Task 39: Stress Tests

**Files:**
- Create: `tests/stress/debate_stress_test.go`

Tests (with resource limits per CLAUDE.md rule 15): 50+ concurrent debates (limited by GOMAXPROCS=2), agent failure recovery, memory usage monitoring, deadlock detection, provider cascade failure.

**Commit:** `test(debate): add stress tests for debate system`

---

### Task 40: Benchmark Tests

**Files:**
- Create: `tests/performance/debate_benchmark_test.go`

Benchmarks: BenchmarkDebateRoundLatency, BenchmarkWeightedVoting, BenchmarkBordaVoting, BenchmarkCondorcetVoting, BenchmarkConsensusCalculation, BenchmarkTopologyInitialization, BenchmarkMessageRouting, BenchmarkEpisodicMemoryStore.

**Commit:** `test(debate): add benchmark tests for debate performance metrics`

---

### Task 41: Challenge Scripts (12 new)

**Files:**
- Create 12 challenge scripts in `challenges/scripts/`

Each script follows the pattern from `debate_voting_mechanism_challenge.sh`: source challenge_framework.sh, init_challenge, load_env, HTTP endpoint tests with curl, record_assertion, finalize.

Scripts:
1. `debate_reflexion_challenge.sh` — POST debate with reflexion enabled, verify reflections in response
2. `debate_adversarial_dynamics_challenge.sh` — POST debate with adversarial enabled, verify attack/defense rounds
3. `debate_tree_topology_challenge.sh` — POST debate with topology=tree, verify hierarchical execution
4. `debate_dehallucination_challenge.sh` — POST debate with dehallucination enabled, verify clarification rounds
5. `debate_self_evolvement_challenge.sh` — POST debate with self_evolvement enabled, verify pre-debate testing
6. `debate_condorcet_voting_challenge.sh` — POST debate, verify Condorcet/Borda voting in response
7. `debate_approval_gate_challenge.sh` — POST debate with gates enabled, verify gate endpoints work
8. `debate_persistence_challenge.sh` — POST debate, then GET sessions/turns from DB endpoints
9. `debate_benchmark_integration_challenge.sh` — Verify benchmark bridge evaluates debate results
10. `debate_provenance_audit_challenge.sh` — POST debate, GET audit trail, verify completeness
11. `debate_deadlock_detection_challenge.sh` — Verify debates don't hang, timeout handling works
12. `debate_git_integration_challenge.sh` — Verify worktree creation and cleanup

**Commit:** `test(debate): add 12 challenge scripts for new debate features`

---

## Phase 4: Documentation

### Task 42: Feature Documentation Updates

**Files:**
- Modify: `docs/features/AI_DEBATE_ORCHESTRATOR.md`
- Modify: `docs/features/ai-debate-configuration.md`
- Create: `docs/features/debate-reflexion-framework.md`
- Create: `docs/features/debate-adversarial-dynamics.md`
- Create: `docs/features/debate-approval-gates.md`
- Create: `docs/guides/debate-database-schema.md`

**Commit:** `docs(debate): update and add feature documentation for full spec compliance`

---

### Task 43: Diagrams

**Files:**
- Create: `docs/diagrams/debate-full-protocol.mmd`
- Create: `docs/diagrams/debate-reflexion-loop.mmd`
- Create: `docs/diagrams/debate-tree-topology.mmd`
- Create: `docs/diagrams/debate-adversarial-round.mmd`
- Create: `docs/diagrams/debate-voting-methods.mmd`
- Create: `docs/diagrams/debate-database-er.mmd`
- Create: `docs/diagrams/debate-approval-gate-flow.mmd`

All Mermaid format.

**Commit:** `docs(debate): add 7 Mermaid diagrams for debate system architecture`

---

### Task 44: CLAUDE.md and AGENTS.md Updates

**Files:**
- Modify: `CLAUDE.md`
- Modify: `AGENTS.md`
- Modify: `docs/MODULES.md`

Update Architecture section: add new packages (reflexion, gates, evaluation, audit), new DB tables, new challenge scripts in challenge list, update phase list from 5→8, update role count, add new endpoints to Protocol Endpoints section.

Synchronize AGENTS.md per Constitution rule.

**Commit:** `docs(debate): synchronize CLAUDE.md, AGENTS.md, MODULES.md with new debate features`

---

### Task 45: Update debate/doc.go

**Files:**
- Modify: `internal/debate/doc.go`

Update package documentation with new subpackages (reflexion, gates, evaluation, audit), new examples, updated phase list.

**Commit:** `docs(debate): update package documentation with new subpackages`

---

### Task 46: Final Verification

**Step 1: Run full test suite**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -v -p 1 ./internal/debate/... -count=1
```

**Step 2: Run all debate challenges**

```bash
./challenges/scripts/debate_reflexion_challenge.sh
./challenges/scripts/debate_adversarial_dynamics_challenge.sh
# ... all 12 new + existing 31
```

**Step 3: Verify compilation**

```bash
go build ./...
```

**Step 4: Run lint and vet**

```bash
make fmt vet lint
```

**Step 5: Commit any fixes**

```bash
git commit -m "fix(debate): address lint and test issues from final verification"
```

---

## Summary

| Phase | Tasks | New Files | Modified Files |
|-------|-------|-----------|----------------|
| Phase 1: Infrastructure | 1-10 | ~16 | ~4 |
| Phase 2A: Independent | 11-15 | ~10 | ~3 |
| Phase 2B: Episodic Memory | 16-19 | ~8 | ~0 |
| Phase 2C: Role-Dependent | 20-23 | ~8 | ~2 |
| Phase 2D: Integrations | 24-29 | ~10 | ~3 |
| Phase 3: Tests | 30-41 | ~35 | ~0 |
| Phase 4: Docs | 42-45 | ~15 | ~5 |
| Verification | 46 | 0 | ~2 |
| **Total** | **46** | **~102** | **~19** |
