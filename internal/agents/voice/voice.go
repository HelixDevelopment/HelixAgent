// Package voice provides voice command functionality
// Inspired by Aider's voice commands
package voice

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
)

// Recognizer interface for speech recognition
type Recognizer interface {
	Name() string
	Recognize(ctx context.Context) (string, error)
	IsAvailable() bool
}

// Command represents a voice command
type Command struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Patterns    []string `json:"patterns"`
	Action      func(args []string) error
}

// Service manages voice recognition and commands.
// mu is Pattern Zeta: it guards the scalar recognizer interface and
// the enabled flag. Command and alias stores have their own locking
// via safe.Store.
type Service struct {
	recognizer Recognizer
	commands   *safe.Store[string, *Command]
	aliases    *safe.Store[string, string]
	logger     *logrus.Logger
	mu         sync.RWMutex
	enabled    bool
}

// NewService creates a new voice service
func NewService(logger *logrus.Logger) *Service {
	if logger == nil {
		logger = logrus.New()
	}

	s := &Service{
		commands: safe.NewStore[string, *Command](),
		aliases:  safe.NewStore[string, string](),
		logger:   logger,
		enabled:  true,
	}

	s.registerDefaultCommands()
	return s
}

// SetRecognizer sets the speech recognizer
func (s *Service) SetRecognizer(r Recognizer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recognizer = r
}

// RegisterCommand registers a voice command
func (s *Service) RegisterCommand(cmd *Command) {
	s.commands.Put(cmd.Name, cmd)
}

// RegisterAlias registers a command alias
func (s *Service) RegisterAlias(alias, command string) {
	s.aliases.Put(alias, command)
}

// Enable enables voice commands
func (s *Service) Enable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = true
}

// Disable disables voice commands
func (s *Service) Disable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = false
}

// IsEnabled returns if voice commands are enabled
func (s *Service) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// IsAvailable returns if voice recognition is available
func (s *Service) IsAvailable() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recognizer != nil && s.recognizer.IsAvailable()
}

// Listen listens for voice commands
func (s *Service) Listen(ctx context.Context) (*CommandResult, error) {
	s.mu.RLock()
	if !s.enabled {
		s.mu.RUnlock()
		return nil, fmt.Errorf("voice service is disabled")
	}
	if s.recognizer == nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("no recognizer configured")
	}
	s.mu.RUnlock()

	s.logger.Info("Listening for voice command...")

	text, err := s.recognizer.Recognize(ctx)
	if err != nil {
		return nil, fmt.Errorf("recognition failed: %w", err)
	}

	s.logger.WithField("text", text).Debug("Voice recognized")

	return s.ProcessCommand(text)
}

// CommandResult represents the result of processing a command
type CommandResult struct {
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Text      string   `json:"text"`
	Success   bool     `json:"success"`
	Error     string   `json:"error,omitempty"`
	Timestamp time.Time
}

// ProcessCommand processes a text command
func (s *Service) ProcessCommand(text string) (*CommandResult, error) {
	result := &CommandResult{
		Text:      text,
		Timestamp: time.Now(),
	}

	// Parse command
	cmd, args := s.parseCommand(text)
	result.Command = cmd
	result.Args = args

	// Find and execute command
	command, ok := s.commands.Get(cmd)
	if !ok {
		// Try aliases
		if aliased, found := s.aliases.Get(cmd); found {
			command, ok = s.commands.Get(aliased)
		}
	}

	if !ok {
		result.Error = fmt.Sprintf("unknown command: %s", cmd)
		return result, nil
	}

	// Execute command
	if err := command.Action(args); err != nil {
		result.Error = err.Error()
		return result, nil
	}

	result.Success = true
	return result, nil
}

// parseCommand parses command text
func (s *Service) parseCommand(text string) (string, []string) {
	// Normalize text
	text = strings.ToLower(strings.TrimSpace(text))

	// Split into words
	words := strings.Fields(text)
	if len(words) == 0 {
		return "", nil
	}

	return words[0], words[1:]
}

// GetCommands returns all registered commands
func (s *Service) GetCommands() []*Command {
	return s.commands.Values()
}

// GetCommand returns a specific command
func (s *Service) GetCommand(name string) (*Command, bool) {
	return s.commands.Get(name)
}

// registerDefaultCommands registers default voice commands
func (s *Service) registerDefaultCommands() {
	// Status command
	s.RegisterCommand(&Command{
		Name:        "status",
		Description: "Check system status",
		Patterns:    []string{"status", "what is the status", "system status"},
		Action: func(args []string) error {
			s.logger.Info("System status: OK")
			return nil
		},
	})

	// Stop command
	s.RegisterCommand(&Command{
		Name:        "stop",
		Description: "Stop current operation",
		Patterns:    []string{"stop", "cancel", "abort", "halt"},
		Action: func(args []string) error {
			s.logger.Info("Stopping current operation")
			return nil
		},
	})

	// Help command
	s.RegisterCommand(&Command{
		Name:        "help",
		Description: "Show available commands",
		Patterns:    []string{"help", "what can you do", "commands"},
		Action: func(args []string) error {
			commands := s.GetCommands()
			s.logger.Infof("Available commands: %d", len(commands))
			for _, cmd := range commands {
				s.logger.Infof("  - %s: %s", cmd.Name, cmd.Description)
			}
			return nil
		},
	})

	// Pause command
	s.RegisterCommand(&Command{
		Name:        "pause",
		Description: "Pause execution",
		Patterns:    []string{"pause", "wait", "hold"},
		Action: func(args []string) error {
			s.logger.Info("Pausing...")
			return nil
		},
	})

	// Resume command
	s.RegisterCommand(&Command{
		Name:        "resume",
		Description: "Resume execution",
		Patterns:    []string{"resume", "continue", "proceed"},
		Action: func(args []string) error {
			s.logger.Info("Resuming...")
			return nil
		},
	})

	// Register aliases
	s.RegisterAlias("quit", "stop")
	s.RegisterAlias("exit", "stop")
	s.RegisterAlias("go", "resume")
}

// MockRecognizer is a mock recognizer for testing
type MockRecognizer struct {
	responses []string
	index     int
}

// NewMockRecognizer creates a mock recognizer
func NewMockRecognizer(responses []string) *MockRecognizer {
	return &MockRecognizer{
		responses: responses,
	}
}

// Name returns recognizer name
func (m *MockRecognizer) Name() string {
	return "mock"
}

// Recognize returns the next mock response
func (m *MockRecognizer) Recognize(ctx context.Context) (string, error) {
	if m.index >= len(m.responses) {
		return "", fmt.Errorf("no more responses")
	}
	response := m.responses[m.index]
	m.index++
	return response, nil
}

// IsAvailable always returns true for mock
func (m *MockRecognizer) IsAvailable() bool {
	return true
}

// TextRecognizer uses text input (keyboard) instead of voice
type TextRecognizer struct {
	reader func() (string, error)
}

// NewTextRecognizer creates a text-based recognizer
func NewTextRecognizer(reader func() (string, error)) *TextRecognizer {
	if reader == nil {
		reader = func() (string, error) {
			var input string
			_, err := fmt.Scanln(&input)
			return input, err
		}
	}
	return &TextRecognizer{reader: reader}
}

// Name returns recognizer name
func (t *TextRecognizer) Name() string {
	return "text"
}

// Recognize reads text input
func (t *TextRecognizer) Recognize(ctx context.Context) (string, error) {
	return t.reader()
}

// IsAvailable always returns true
func (t *TextRecognizer) IsAvailable() bool {
	return true
}

// CommandMatcher matches voice input to commands
type CommandMatcher struct {
	commands []Command
}

// NewCommandMatcher creates a new command matcher
func NewCommandMatcher(commands []Command) *CommandMatcher {
	return &CommandMatcher{commands: commands}
}

// Match finds the best matching command
func (m *CommandMatcher) Match(input string) (*Command, float64) {
	input = strings.ToLower(strings.TrimSpace(input))

	var bestMatch *Command
	bestScore := 0.0

	for i := range m.commands {
		cmd := &m.commands[i]
		for _, pattern := range cmd.Patterns {
			score := similarity(input, pattern)
			if score > bestScore {
				bestScore = score
				bestMatch = cmd
			}
		}
	}

	return bestMatch, bestScore
}

// similarity calculates simple word overlap similarity
func similarity(a, b string) float64 {
	wordsA := wordSet(a)
	wordsB := wordSet(b)

	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0.0
	}

	intersection := 0
	for word := range wordsA {
		if wordsB[word] {
			intersection++
		}
	}

	return float64(intersection) / float64(len(wordsA)+len(wordsB)-intersection)
}

// wordSet creates a set of words
func wordSet(s string) map[string]bool {
	words := make(map[string]bool)
	for _, w := range strings.Fields(strings.ToLower(s)) {
		words[w] = true
	}
	return words
}
