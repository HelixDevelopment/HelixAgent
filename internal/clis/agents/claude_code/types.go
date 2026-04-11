// Package claude_code provides shared types for Claude Code.
package claude_code

import (
	"time"
)

// Message represents a chat message
type Message struct {
	Role      string                 `json:"role"` // "user", "assistant", "system"
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Session represents a Claude Code session
type Session struct {
	ID           string                 `json:"id"`
	WorkDir      string                 `json:"work_dir"`
	Messages     []Message              `json:"messages"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	Config       *Config                `json:"config"`
	Context      map[string]interface{} `json:"context"`
	Active       bool                   `json:"active"`
	LastActivity time.Time              `json:"last_activity"`
}

// NewSession creates a new session
func NewSession(workDir string, config *Config) *Session {
	now := time.Now()
	return &Session{
		ID:           generateSessionID(),
		WorkDir:      workDir,
		Messages:     []Message{},
		CreatedAt:    now,
		UpdatedAt:    now,
		LastActivity: now,
		Config:       config,
		Context:      make(map[string]interface{}),
		Active:       true,
	}
}

// AddMessage adds a message to the session
func (s *Session) AddMessage(role, content string) {
	s.Messages = append(s.Messages, Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
	s.UpdatedAt = time.Now()
	s.LastActivity = time.Now()
}

// GetMessages returns all messages
func (s *Session) GetMessages() []Message {
	return s.Messages
}

// GetLastMessages returns the last n messages
func (s *Session) GetLastMessages(n int) []Message {
	if n >= len(s.Messages) {
		return s.Messages
	}
	return s.Messages[len(s.Messages)-n:]
}

// Clear clears all messages
func (s *Session) Clear() {
	s.Messages = []Message{}
	s.UpdatedAt = time.Now()
}

// IsExpired checks if the session has expired
func (s *Session) IsExpired(timeoutMinutes int) bool {
	if timeoutMinutes < 0 {
		return false // Negative means no expiration
	}
	if timeoutMinutes == 0 {
		// Zero means immediate expiration if last activity is in the past
		return time.Since(s.LastActivity) > 0
	}
	return time.Since(s.LastActivity) > time.Duration(timeoutMinutes)*time.Minute
}

// EditRequest represents a code edit request
type EditRequest struct {
	FilePath    string `json:"file_path"`
	OldString   string `json:"old_string"`
	NewString   string `json:"new_string"`
	Description string `json:"description,omitempty"`
}

// EditResult represents the result of an edit operation
type EditResult struct {
	Success  bool   `json:"success"`
	FilePath string `json:"file_path"`
	Diff     string `json:"diff,omitempty"`
	Error    string `json:"error,omitempty"`
}

// BashRequest represents a bash command request
type BashRequest struct {
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
	Timeout     int    `json:"timeout,omitempty"` // seconds
}

// BashResult represents the result of a bash command
type BashResult struct {
	Success    bool   `json:"success"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// GitRequest represents a git operation request
type GitRequest struct {
	Operation string   `json:"operation"` // "status", "diff", "commit", "push", "pull", etc.
	Args      []string `json:"args,omitempty"`
	Message   string   `json:"message,omitempty"` // For commit
}

// GitResult represents the result of a git operation
type GitResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}

// SearchRequest represents a search request
type SearchRequest struct {
	Pattern  string `json:"pattern"`
	Path     string `json:"path,omitempty"`
	FileType string `json:"file_type,omitempty"`
}

// SearchResult represents a search result
type SearchResult struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// SearchResults represents a collection of search results
type SearchResults struct {
	Success bool           `json:"success"`
	Matches []SearchResult `json:"matches"`
	Total   int            `json:"total"`
	Error   string         `json:"error,omitempty"`
}

// ReviewRequest represents a code review request
type ReviewRequest struct {
	Target     string   `json:"target"`                // file or directory
	ReviewType string   `json:"review_type,omitempty"` // "general", "security", "performance"
	FocusAreas []string `json:"focus_areas,omitempty"`
}

// ReviewResult represents a code review result
type ReviewResult struct {
	Success bool          `json:"success"`
	Summary string        `json:"summary"`
	Issues  []ReviewIssue `json:"issues"`
	Score   int           `json:"score"` // 0-100
	Error   string        `json:"error,omitempty"`
}

// ReviewIssue represents an issue found during review
type ReviewIssue struct {
	Type       string `json:"type"` // "error", "warning", "suggestion"
	File       string `json:"file"`
	Line       int    `json:"line"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

// TestRequest represents a test execution request
type TestRequest struct {
	Command  string   `json:"command,omitempty"`
	Files    []string `json:"files,omitempty"`
	Coverage bool     `json:"coverage,omitempty"`
	Verbose  bool     `json:"verbose,omitempty"`
}

// TestResult represents test execution results
type TestResult struct {
	Success     bool    `json:"success"`
	Output      string  `json:"output"`
	Passed      int     `json:"passed"`
	Failed      int     `json:"failed"`
	Skipped     int     `json:"skipped"`
	DurationMs  int64   `json:"duration_ms"`
	CoveragePct float64 `json:"coverage_pct,omitempty"`
	Error       string  `json:"error,omitempty"`
}
