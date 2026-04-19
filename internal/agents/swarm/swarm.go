// Package swarm provides multi-agent swarm enhancements
// Inspired by Claude Code's XML-based communication and team features
package swarm

import (
	"encoding/xml"
	"fmt"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
)

// AgentColor represents agent color assignment for identification
type AgentColor string

const (
	ColorRed     AgentColor = "red"
	ColorGreen   AgentColor = "green"
	ColorYellow  AgentColor = "yellow"
	ColorBlue    AgentColor = "blue"
	ColorMagenta AgentColor = "magenta"
	ColorCyan    AgentColor = "cyan"
	ColorWhite   AgentColor = "white"
)

// AgentRole represents agent specialization role
type AgentRole string

const (
	RoleLeader     AgentRole = "leader"
	RoleWorker     AgentRole = "worker"
	RoleSpecialist AgentRole = "specialist"
	RoleReviewer   AgentRole = "reviewer"
	RoleExplorer   AgentRole = "explorer"
)

// SwarmAgent represents an agent in the swarm
type SwarmAgent struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Color        AgentColor             `json:"color"`
	Role         AgentRole              `json:"role"`
	Status       AgentStatus            `json:"status"`
	Capabilities []string               `json:"capabilities"`
	Metadata     map[string]interface{} `json:"metadata"`
	JoinedAt     time.Time              `json:"joined_at"`
}

// AgentStatus represents agent status
type AgentStatus string

const (
	AgentIdle    AgentStatus = "idle"
	AgentWorking AgentStatus = "working"
	AgentDone    AgentStatus = "done"
	AgentError   AgentStatus = "error"
)

// Swarm manages a team of agents with shared resources.
//
// Concurrent-safe by construction (CONST-029): agents is a safe.Store;
// colorIdx and agentSeq are atomic.Int64 counters so AddAgent does not
// need a mutex for ID/colour generation. The colors slice is set once
// at construction and never mutated.
type Swarm struct {
	id         string
	agents     *safe.Store[string, *SwarmAgent]
	scratchpad *Scratchpad
	logger     *logrus.Logger
	colors     []AgentColor
	colorIdx   atomic.Int64
	agentSeq   atomic.Int64
}

// NewSwarm creates a new agent swarm
func NewSwarm(id string, logger *logrus.Logger) *Swarm {
	if logger == nil {
		logger = logrus.New()
	}

	return &Swarm{
		id:         id,
		agents:     safe.NewStore[string, *SwarmAgent](),
		scratchpad: NewScratchpad(),
		logger:     logger,
		colors: []AgentColor{
			ColorRed, ColorGreen, ColorYellow,
			ColorBlue, ColorMagenta, ColorCyan,
		},
	}
}

// ID returns the swarm ID
func (s *Swarm) ID() string {
	return s.id
}

// AddAgent adds an agent to the swarm
func (s *Swarm) AddAgent(name string, role AgentRole) (*SwarmAgent, error) {
	colorIdx := s.colorIdx.Add(1) - 1
	color := s.colors[int(colorIdx)%len(s.colors)]
	seq := s.agentSeq.Add(1)

	agent := &SwarmAgent{
		ID:       fmt.Sprintf("%s-%d", s.id, seq),
		Name:     name,
		Color:    color,
		Role:     role,
		Status:   AgentIdle,
		JoinedAt: time.Now(),
	}

	s.agents.Put(agent.ID, agent)

	s.logger.WithFields(logrus.Fields{
		"agent_id": agent.ID,
		"name":     name,
		"color":    color,
		"role":     role,
	}).Info("Agent joined swarm")

	return agent, nil
}

// RemoveAgent removes an agent from the swarm
func (s *Swarm) RemoveAgent(agentID string) error {
	if _, ok := s.agents.Delete(agentID); !ok {
		return fmt.Errorf("agent not found: %s", agentID)
	}
	s.logger.WithField("agent_id", agentID).Info("Agent left swarm")
	return nil
}

// GetAgent retrieves an agent by ID
func (s *Swarm) GetAgent(agentID string) (*SwarmAgent, bool) {
	return s.agents.Get(agentID)
}

// GetAgentsByRole returns agents with a specific role
func (s *Swarm) GetAgentsByRole(role AgentRole) []*SwarmAgent {
	var agents []*SwarmAgent
	s.agents.Range(func(_ string, agent *SwarmAgent) bool {
		if agent.Role == role {
			agents = append(agents, agent)
		}
		return true
	})
	return agents
}

// ListAgents returns all agents in the swarm
func (s *Swarm) ListAgents() []*SwarmAgent {
	return s.agents.Values()
}

// UpdateAgentStatus updates an agent's status
func (s *Swarm) UpdateAgentStatus(agentID string, status AgentStatus) error {
	var notFound bool
	s.agents.Update(agentID, func(agent *SwarmAgent, ok bool) (*SwarmAgent, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		agent.Status = status
		return agent, true
	})
	if notFound {
		return fmt.Errorf("agent not found: %s", agentID)
	}
	return nil
}

// GetScratchpad returns the shared scratchpad
func (s *Swarm) GetScratchpad() *Scratchpad {
	return s.scratchpad
}

// Broadcast sends a message to all agents
func (s *Swarm) Broadcast(from string, content string) error {
	msg := XMLMessage{
		Type:      "broadcast",
		From:      from,
		Content:   content,
		Timestamp: time.Now(),
	}

	// Add to scratchpad
	s.scratchpad.AddEntry(ScratchpadEntry{
		Type:      "message",
		AgentID:   from,
		Content:   content,
		Timestamp: time.Now(),
	})

	s.logger.WithFields(logrus.Fields{
		"from":    from,
		"content": content,
	}).Debug("Broadcast message")

	_ = msg
	return nil
}

// SendTo sends a message to a specific agent
func (s *Swarm) SendTo(from, to, content string) error {
	if _, ok := s.agents.Get(to); !ok {
		return fmt.Errorf("recipient not found: %s", to)
	}

	msg := XMLMessage{
		Type:      "direct",
		From:      from,
		To:        to,
		Content:   content,
		Timestamp: time.Now(),
	}

	_ = msg
	return nil
}

// XMLMessage represents XML-based communication between agents
type XMLMessage struct {
	XMLName   xml.Name  `xml:"message"`
	Type      string    `xml:"type,attr"`
	From      string    `xml:"from,attr"`
	To        string    `xml:"to,attr,omitempty"`
	Timestamp time.Time `xml:"timestamp,attr"`
	Content   string    `xml:"content"`
}

// ToXML serializes message to XML
func (m *XMLMessage) ToXML() ([]byte, error) {
	return xml.MarshalIndent(m, "", "  ")
}

// ParseXML parses XML message
func ParseXML(data []byte) (*XMLMessage, error) {
	var msg XMLMessage
	if err := xml.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// Scratchpad is shared memory for all agents.
//
// Concurrent-safe by construction (CONST-029): entries is a safe.Slice.
type Scratchpad struct {
	entries *safe.Slice[ScratchpadEntry]
}

// ScratchpadEntry represents a single entry in the scratchpad
type ScratchpadEntry struct {
	Type      string                 `json:"type"`
	AgentID   string                 `json:"agent_id"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewScratchpad creates a new scratchpad
func NewScratchpad() *Scratchpad {
	return &Scratchpad{
		entries: safe.NewSlice[ScratchpadEntry](),
	}
}

// AddEntry adds an entry to the scratchpad
func (s *Scratchpad) AddEntry(entry ScratchpadEntry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	s.entries.Append(entry)
}

// GetEntries returns all entries
func (s *Scratchpad) GetEntries() []ScratchpadEntry {
	return s.entries.Snapshot()
}

// GetEntriesByType returns entries of a specific type
func (s *Scratchpad) GetEntriesByType(entryType string) []ScratchpadEntry {
	var entries []ScratchpadEntry
	s.entries.Range(func(_ int, entry ScratchpadEntry) bool {
		if entry.Type == entryType {
			entries = append(entries, entry)
		}
		return true
	})
	return entries
}

// GetEntriesByAgent returns entries from a specific agent
func (s *Scratchpad) GetEntriesByAgent(agentID string) []ScratchpadEntry {
	var entries []ScratchpadEntry
	s.entries.Range(func(_ int, entry ScratchpadEntry) bool {
		if entry.AgentID == agentID {
			entries = append(entries, entry)
		}
		return true
	})
	return entries
}

// Clear clears all entries
func (s *Scratchpad) Clear() {
	s.entries.Clear()
}

// LastN returns the last n entries
func (s *Scratchpad) LastN(n int) []ScratchpadEntry {
	snapshot := s.entries.Snapshot()
	if n >= len(snapshot) {
		return snapshot
	}
	return snapshot[len(snapshot)-n:]
}

// ToXML exports scratchpad to XML
func (s *Scratchpad) ToXML() ([]byte, error) {
	type scratchpadXML struct {
		XMLName xml.Name          `xml:"scratchpad"`
		Entries []ScratchpadEntry `xml:"entry"`
	}
	return xml.MarshalIndent(scratchpadXML{Entries: s.entries.Snapshot()}, "", "  ")
}

// Coordinator manages coordination between agents.
//
// Concurrent-safe by construction (CONST-029): tasks is a safe.Store;
// taskSeq is an atomic.Int64 counter so CreateTask cannot race on
// `len(tasks)`-derived IDs. Field mutations on *CoordinatedTask
// (Assignments, Results) route through Update (Pattern Beta).
type Coordinator struct {
	swarm   *Swarm
	logger  *logrus.Logger
	tasks   *safe.Store[string, *CoordinatedTask]
	taskSeq atomic.Int64
}

// CoordinatedTask represents a task being coordinated
type CoordinatedTask struct {
	ID          string                 `json:"id"`
	Description string                 `json:"description"`
	Assignments map[string]string      `json:"assignments"` // agentID -> subtask
	Status      string                 `json:"status"`
	Results     map[string]interface{} `json:"results"`
	CreatedAt   time.Time              `json:"created_at"`
}

// NewCoordinator creates a new coordinator
func NewCoordinator(swarm *Swarm, logger *logrus.Logger) *Coordinator {
	if logger == nil {
		logger = logrus.New()
	}

	return &Coordinator{
		swarm:  swarm,
		logger: logger,
		tasks:  safe.NewStore[string, *CoordinatedTask](),
	}
}

// CreateTask creates a coordinated task.
//
// taskSeq atomic counter takes ID generation off `len(c.tasks)+1`, so
// concurrent callers cannot collide on IDs (the original mutex-based
// fix from race-debt BUGFIX #22 — preserved by structural means here).
func (c *Coordinator) CreateTask(description string) *CoordinatedTask {
	seq := c.taskSeq.Add(1)
	task := &CoordinatedTask{
		ID:          fmt.Sprintf("task-%d", seq),
		Description: description,
		Assignments: make(map[string]string),
		Status:      "pending",
		Results:     make(map[string]interface{}),
		CreatedAt:   time.Now(),
	}
	c.tasks.Put(task.ID, task)

	// Add to scratchpad (outside the lock — scratchpad has its own
	// synchronisation and we must not hold c.mu during a call that
	// could block on another subsystem).
	c.swarm.GetScratchpad().AddEntry(ScratchpadEntry{
		Type:    "task_created",
		Content: description,
		Metadata: map[string]interface{}{
			"task_id": task.ID,
		},
	})

	return task
}

// Assign assigns a subtask to an agent
func (c *Coordinator) Assign(taskID, agentID, subtask string) error {
	if !c.tasks.Has(taskID) {
		return fmt.Errorf("task not found: %s", taskID)
	}
	if _, ok := c.swarm.GetAgent(agentID); !ok {
		return fmt.Errorf("agent not found: %s", agentID)
	}
	c.tasks.Update(taskID, func(task *CoordinatedTask, ok bool) (*CoordinatedTask, bool) {
		if !ok {
			return nil, false
		}
		task.Assignments[agentID] = subtask
		return task, true
	})

	c.logger.WithFields(logrus.Fields{
		"task":    taskID,
		"agent":   agentID,
		"subtask": subtask,
	}).Info("Task assigned")

	return nil
}

// ReportResult reports task result from an agent
func (c *Coordinator) ReportResult(taskID, agentID string, result interface{}) error {
	var notFound bool
	c.tasks.Update(taskID, func(task *CoordinatedTask, ok bool) (*CoordinatedTask, bool) {
		if !ok {
			notFound = true
			return nil, false
		}
		task.Results[agentID] = result
		return task, true
	})
	if notFound {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// Add to scratchpad
	c.swarm.GetScratchpad().AddEntry(ScratchpadEntry{
		Type:    "task_result",
		AgentID: agentID,
		Content: fmt.Sprintf("%v", result),
		Metadata: map[string]interface{}{
			"task_id": taskID,
		},
	})

	return nil
}

// GetTask retrieves a task
func (c *Coordinator) GetTask(taskID string) (*CoordinatedTask, bool) {
	return c.tasks.Get(taskID)
}

// Colorize adds color formatting for display
func Colorize(color AgentColor, text string) string {
	// ANSI color codes
	codes := map[AgentColor]string{
		ColorRed:     "\033[31m",
		ColorGreen:   "\033[32m",
		ColorYellow:  "\033[33m",
		ColorBlue:    "\033[34m",
		ColorMagenta: "\033[35m",
		ColorCyan:    "\033[36m",
		ColorWhite:   "\033[37m",
	}

	reset := "\033[0m"

	if code, ok := codes[color]; ok {
		return code + text + reset
	}
	return text
}
