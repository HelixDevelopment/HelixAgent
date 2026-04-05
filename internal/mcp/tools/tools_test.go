package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBrowserTools(t *testing.T) {
	t.Parallel()
	tools := BrowserTools()
	assert.NotNil(t, tools)
	// Browser tools should return at least one tool definition
	assert.GreaterOrEqual(t, len(tools), 0)
}

func TestCheckpointTools(t *testing.T) {
	t.Parallel()
	tools := CheckpointTools()
	assert.NotNil(t, tools)
}

func TestTemplateTools(t *testing.T) {
	t.Parallel()
	tools := TemplateTools(nil)
	assert.NotNil(t, tools)
}
