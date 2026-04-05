package formatters

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitializeFormattersSystem(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := DefaultConfig()
	config.CacheEnabled = false
	config.Metrics = false
	config.Tracing = false

	registry, executor, health, err := InitializeFormattersSystem(config, logger)
	require.NoError(t, err)
	assert.NotNil(t, registry)
	assert.NotNil(t, executor)
	assert.NotNil(t, health)
}

func TestInitializeFormattersSystem_WithCache(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := DefaultConfig()
	config.CacheEnabled = true
	config.CacheTTL = 1 * time.Hour
	config.Metrics = false
	config.Tracing = false

	registry, executor, health, err := InitializeFormattersSystem(config, logger)
	require.NoError(t, err)
	assert.NotNil(t, registry)
	assert.NotNil(t, executor)
	assert.NotNil(t, health)
}

func TestInitializeFormattersSystem_AllFeatures(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := DefaultConfig()

	registry, executor, health, err := InitializeFormattersSystem(config, logger)
	require.NoError(t, err)
	assert.NotNil(t, registry)
	assert.NotNil(t, executor)
	assert.NotNil(t, health)
}

func TestRegisterFormatter(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := DefaultConfig()
	config.CacheEnabled = false
	config.Metrics = false
	config.Tracing = false

	registry, _, _, err := InitializeFormattersSystem(config, logger)
	require.NoError(t, err)

	mock := newMockFormatter("black", "26.1a1", []string{"python"})
	metadata := &FormatterMetadata{
		Name:      "black",
		Version:   "26.1a1",
		Languages: []string{"python"},
		Type:      FormatterTypeNative,
	}

	err = RegisterFormatter(registry, mock, metadata)
	assert.NoError(t, err)
	assert.Equal(t, 1, registry.Count())

	// Verify retrievable
	f, err := registry.Get("black")
	assert.NoError(t, err)
	assert.Equal(t, "black", f.Name())
}

func TestRegisterFormatter_Duplicate(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := DefaultConfig()
	config.CacheEnabled = false
	config.Metrics = false
	config.Tracing = false

	registry, _, _, err := InitializeFormattersSystem(config, logger)
	require.NoError(t, err)

	mock := newMockFormatter("black", "26.1a1", []string{"python"})
	metadata := &FormatterMetadata{
		Name:      "black",
		Version:   "26.1a1",
		Languages: []string{"python"},
		Type:      FormatterTypeNative,
	}

	err = RegisterFormatter(registry, mock, metadata)
	assert.NoError(t, err)

	err = RegisterFormatter(registry, mock, metadata)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}
