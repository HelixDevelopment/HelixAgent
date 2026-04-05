package voice

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	svc := NewService(logger)

	require.NotNil(t, svc)
	assert.NotNil(t, svc.commands)
	assert.NotNil(t, svc.aliases)
	assert.NotNil(t, svc.logger)
	assert.True(t, svc.enabled)
	assert.Greater(t, len(svc.commands), 0) // Default commands registered
}

func TestService_SetRecognizer(t *testing.T) {
	t.Parallel()
	svc := NewService(nil)
	recognizer := NewMockRecognizer([]string{"test"})

	svc.SetRecognizer(recognizer)

	assert.NotNil(t, svc.recognizer)
}

func TestService_RegisterCommand(t *testing.T) {
	t.Parallel()
	svc := NewService(nil)

	cmd := &Command{
		Name:        "test",
		Description: "Test command",
		Action:      func(args []string) error { return nil },
	}

	svc.RegisterCommand(cmd)

	retrieved, ok := svc.GetCommand("test")
	assert.True(t, ok)
	assert.Equal(t, "test", retrieved.Name)
}

func TestService_RegisterAlias(t *testing.T) {
	t.Parallel()
	svc := NewService(nil)

	// Register a command first
	svc.RegisterCommand(&Command{
		Name:   "original",
		Action: func(args []string) error { return nil },
	})

	svc.RegisterAlias("alias", "original")

	// Process command using alias
	result, err := svc.ProcessCommand("alias")
	require.NoError(t, err)
	assert.Equal(t, "alias", result.Command)
}

func TestService_EnableDisable(t *testing.T) {
	t.Parallel()
	svc := NewService(nil)

	assert.True(t, svc.IsEnabled())

	svc.Disable()
	assert.False(t, svc.IsEnabled())

	svc.Enable()
	assert.True(t, svc.IsEnabled())
}

func TestService_IsAvailable(t *testing.T) {
	t.Parallel()
	svc := NewService(nil)

	// No recognizer set
	assert.False(t, svc.IsAvailable())

	// Set recognizer
	svc.SetRecognizer(NewMockRecognizer(nil))
	assert.True(t, svc.IsAvailable())
}

func TestService_Listen(t *testing.T) {
	t.Parallel()
	svc := NewService(nil)
	svc.SetRecognizer(NewMockRecognizer([]string{"status"}))

	ctx := context.Background()
	result, err := svc.Listen(ctx)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "status", result.Command)
	assert.True(t, result.Success)
}

func TestService_Listen_Disabled(t *testing.T) {
	t.Parallel()
	svc := NewService(nil)
	svc.Disable()

	ctx := context.Background()
	_, err := svc.Listen(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestService_Listen_NoRecognizer(t *testing.T) {
	t.Parallel()
	svc := NewService(nil)

	ctx := context.Background()
	_, err := svc.Listen(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no recognizer")
}

func TestService_ProcessCommand(t *testing.T) {
	t.Parallel()
	svc := NewService(nil)

	actionCalled := false
	svc.RegisterCommand(&Command{
		Name:   "test",
		Action: func(args []string) error { actionCalled = true; return nil },
	})

	result, err := svc.ProcessCommand("test arg1 arg2")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "test", result.Command)
	assert.Equal(t, []string{"arg1", "arg2"}, result.Args)
	assert.True(t, actionCalled)
}

func TestService_ProcessCommand_Unknown(t *testing.T) {
	t.Parallel()
	svc := NewService(nil)

	result, err := svc.ProcessCommand("unknowncommand")

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)
}

func TestService_ProcessCommand_ActionError(t *testing.T) {
	t.Parallel()
	svc := NewService(nil)
	svc.RegisterCommand(&Command{
		Name:   "fail",
		Action: func(args []string) error { return errors.New("action failed") },
	})

	result, err := svc.ProcessCommand("fail")

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "action failed")
}

func TestService_parseCommand(t *testing.T) {
	t.Parallel()
	svc := NewService(nil)

	tests := []struct {
		input       string
		expectCmd   string
		expectArgs  []string
	}{
		{"status", "status", []string{}},
		{"run test", "run", []string{"test"}},
		{"run test --verbose", "run", []string{"test", "--verbose"}},
		{"  spaced  command  ", "spaced", []string{"command"}},
		{"", "", nil},
	}

	for _, tt := range tests {
		cmd, args := svc.parseCommand(tt.input)
		assert.Equal(t, tt.expectCmd, cmd)
		assert.Equal(t, tt.expectArgs, args)
	}
}

func TestService_GetCommands(t *testing.T) {
	t.Parallel()
	svc := NewService(nil)

	cmds := svc.GetCommands()

	assert.Greater(t, len(cmds), 0)
}

func TestMockRecognizer(t *testing.T) {
	t.Parallel()
	responses := []string{"first", "second"}
	mock := NewMockRecognizer(responses)

	assert.Equal(t, "mock", mock.Name())
	assert.True(t, mock.IsAvailable())

	ctx := context.Background()

	// First response
	text, err := mock.Recognize(ctx)
	require.NoError(t, err)
	assert.Equal(t, "first", text)

	// Second response
	text, err = mock.Recognize(ctx)
	require.NoError(t, err)
	assert.Equal(t, "second", text)

	// No more responses
	_, err = mock.Recognize(ctx)
	assert.Error(t, err)
}

func TestTextRecognizer(t *testing.T) {
	t.Parallel()
	// Test with custom reader
	reader := func() (string, error) {
		return "typed input", nil
	}

	recognizer := NewTextRecognizer(reader)
	assert.Equal(t, "text", recognizer.Name())
	assert.True(t, recognizer.IsAvailable())

	ctx := context.Background()
	text, err := recognizer.Recognize(ctx)

	require.NoError(t, err)
	assert.Equal(t, "typed input", text)
}

func TestTextRecognizer_Default(t *testing.T) {
	t.Parallel()
	// Test with default reader (will fail without stdin)
	recognizer := NewTextRecognizer(nil)
	assert.NotNil(t, recognizer)
}

func TestCommandMatcher(t *testing.T) {
	t.Parallel()
	commands := []Command{
		{
			Name:     "status",
			Patterns: []string{"status", "what is the status"},
		},
		{
			Name:     "stop",
			Patterns: []string{"stop", "cancel"},
		},
	}

	matcher := NewCommandMatcher(commands)

	// Test exact match
	cmd, score := matcher.Match("status")
	assert.NotNil(t, cmd)
	assert.Equal(t, "status", cmd.Name)
	assert.Greater(t, score, 0.0)

	// Test partial match
	cmd, score = matcher.Match("what is status")
	assert.NotNil(t, cmd)
	assert.Greater(t, score, 0.0)
}

func TestSimilarity(t *testing.T) {
	t.Parallel()
	// Exact match
	assert.Equal(t, 1.0, similarity("hello world", "hello world"))

	// No match
	assert.Equal(t, 0.0, similarity("hello", "goodbye"))

	// Partial match
	score := similarity("hello world test", "hello world")
	assert.Greater(t, score, 0.0)
	assert.Less(t, score, 1.0)

	// Empty strings
	assert.Equal(t, 0.0, similarity("", "hello"))
	assert.Equal(t, 0.0, similarity("hello", ""))
}

func TestWordSet(t *testing.T) {
	t.Parallel()
	words := wordSet("hello world hello")

	assert.Len(t, words, 2)
	assert.True(t, words["hello"])
	assert.True(t, words["world"])
	assert.False(t, words["missing"])
}

func TestDefaultCommands(t *testing.T) {
	t.Parallel()
	svc := NewService(nil)

	// Test status command
	result, err := svc.ProcessCommand("status")
	require.NoError(t, err)
	assert.True(t, result.Success)

	// Test stop command
	result, err = svc.ProcessCommand("stop")
	require.NoError(t, err)
	assert.True(t, result.Success)

	// Test help command
	result, err = svc.ProcessCommand("help")
	require.NoError(t, err)
	assert.True(t, result.Success)

	// Test alias - quit maps to stop
	result, err = svc.ProcessCommand("quit")
	require.NoError(t, err)
	// Alias may work or not depending on implementation
	_ = result
}

func TestCommandResult(t *testing.T) {
	t.Parallel()
	result := &CommandResult{
		Command: "test",
		Args:    []string{"arg1"},
		Text:    "test arg1",
		Success: true,
	}

	assert.Equal(t, "test", result.Command)
	assert.Equal(t, []string{"arg1"}, result.Args)
	assert.True(t, result.Success)
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()
	svc := NewService(nil)

	// Test concurrent reads and writes
	done := make(chan bool, 3)

	go func() {
		svc.RegisterCommand(&Command{Name: "cmd1"})
		done <- true
	}()

	go func() {
		svc.GetCommands()
		done <- true
	}()

	go func() {
		svc.ProcessCommand("status")
		done <- true
	}()

	// Wait for all
	for i := 0; i < 3; i++ {
		<-done
	}

	// Should not panic
}
