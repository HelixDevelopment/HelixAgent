// Package swarm provides multi-agent swarm enhancements
// Inspired by Claude Code's XML-based communication and team features
package swarm

import (
	"encoding/xml"
	"fmt"
	"sync"
	"time"

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

// Swarm manages a team of agents with shared resources
type Swarm struct {
	id         string
	agents     map[string]*SwarmAgent
	scratchpad *Scratchpad
	logger     *logrus.Logger
	mu         sync.RWMutex
	colors     []AgentColor
	colorIdx   int
}

// NewSwarm creates a new agent swarm
func NewSwarm(id string, logger *logrus.Logger) *Swarm {
	if logger == nil {
		logger = logrus.New()
	}

	return &Swarm{
		id:         id,
		agents:     make(map[string]*SwarmAgent),
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
	s.mu.Lock()
	defer s.mu.Unlock()

	// Assign color
	color := s.colors[s.colorIdx%len(s.colors)]
	s.colorIdx++

	agent := &SwarmAgent{
		ID:       fmt.Sprintf("%s-%d", s.id, len(s.agents)+1),
		Name:     name,
		Color:    color,
		Role:     role,
		Status:   AgentIdle,
		JoinedAt: time.Now(),
	}

	s.agents[agent.ID] = agent

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
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.agents[agentID]; !ok {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	delete(s.agents, agentID)

	s.logger.WithField("agent_id", agentID).Info("Agent left swarm")
	return nil
}

// GetAgent retrieves an agent by ID
func (s *Swarm) GetAgent(agentID string) (*SwarmAgent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agent, ok := s.agents[agentID]
	return agent, ok
}

// GetAgentsByRole returns agents with a specific role
func (s *Swarm) GetAgentsByRole(role AgentRole) []*SwarmAgent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var agents []*SwarmAgent
	for _, agent := range s.agents {
		if agent.Role == role {
			agents = append(agents, agent)
		}
	}
	return agents
}

// ListAgents returns all agents in the swarm
func (s *Swarm) ListAgents() []*SwarmAgent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]*SwarmAgent, 0, len(s.agents))
	for _, agent := range s.agents {
		agents = append(agents, agent)
	}
	return agents
}

// UpdateAgentStatus updates an agent's status
func (s *Swarm) UpdateAgentStatus(agentID string, status AgentStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[agentID]
	if !ok {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	agent.Status = status
	return nil
}

// GetScratchpad returns the shared scratchpad
func (s *Swarm) GetScratchpad() *Scratchpad {
	return s.scratchpad
}

// Broadcast sends a message to all agents
func (s *Swarm) Broadcast(from string, content string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

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
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.agents[to]; !ok {
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

// Scratchpad is shared memory for all agents
type Scratchpad struct {
	entries []ScratchpadEntry
	mu      sync.RWMutex
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
		entries: make([]ScratchpadEntry, 0),
	}
}

// AddEntry adds an entry to the scratchpad
func (s *Scratchpad) AddEntry(entry ScratchpadEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	s.entries = append(s.entries, entry)
}

// GetEntries returns all entries
func (s *Scratchpad) GetEntries() []ScratchpadEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy
	entries := make([]ScratchpadEntry, len(s.entries))
	copy(entries, s.entries)
	return entries
}

// GetEntriesByType returns entries of a specific type
func (s *Scratchpad) GetEntriesByType(entryType string) []ScratchpadEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entries []ScratchpadEntry
	for _, entry := range s.entries {
		if entry.Type == entryType {
			entries = append(entries, entry)
		}
	}
	return entries
}

// GetEntriesByAgent returns entries from a specific agent
func (s *Scratchpad) GetEntriesByAgent(agentID string) []ScratchpadEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entries []ScratchpadEntry
	for _, entry := range s.entries {
		if entry.AgentID == agentID {
			entries = append(entries, entry)
		}
	}
	return entries
}

// Clear clears all entries
func (s *Scratchpad) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = s.entries[:0]
}

// LastN returns the last n entries
func (s *Scratchpad) LastN(n int) []ScratchpadEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n >= len(s.entries) {
		entries := make([]ScratchpadEntry, len(s.entries))
		copy(entries, s.entries)
		return entries
	}

	return s.entries[len(s.entries)-n:]
}

// ToXML exports scratchpad to XML
func (s *Scratchpad) ToXML() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type scratchpadXML struct {
		XMLName xml.Name          `xml:"scratchpad"`
		Entries []ScratchpadEntry `xml:"entry"`
	}

	return xml.MarshalIndent(scratchpadXML{Entries: s.entries}, "", "  ")
}

// Coordinator manages coordination between agents
type Coordinator struct {
	swarm  *Swarm
	logger *logrus.Logger
	tasks  map[string]*CoordinatedTask
	mu     sync.RWMutex
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
		tasks:  make(map[string]*CoordinatedTask),
	}
}

// CreateTask creates a coordinated task
func (c *Coordinator) CreateTask(description string) *CoordinatedTask {
	task := &CoordinatedTask{
		ID:          fmt.Sprintf("task-%d", len(c.tasks)+1),
		Description: description,
		Assignments: make(map[string]string),
		Status:      "pending",
		Results:     make(map[string]interface{}),
		CreatedAt:   time.Now(),
	}

	c.mu.Lock()
	c.tasks[task.ID] = task
	c.mu.Unlock()

	// Add to scratchpad
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
	c.mu.Lock()
	defer c.mu.Unlock()

	task, ok := c.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// Check agent exists
	if _, ok := c.swarm.GetAgent(agentID); !ok {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	task.Assignments[agentID] = subtask

	c.logger.WithFields(logrus.Fields{
		"task":    taskID,
		"agent":   agentID,
		"subtask": subtask,
	}).Info("Task assigned")

	return nil
}

// ReportResult reports task result from an agent
func (c *Coordinator) ReportResult(taskID, agentID string, result interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	task, ok := c.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.Results[agentID] = result

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
	c.mu.RLock()
	defer c.mu.RUnlock()
	task, ok := c.tasks[taskID]
	return task, ok
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
