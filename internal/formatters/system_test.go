package formatters

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSystem(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := DefaultConfig()
	// Disable cache to avoid cleanup goroutine interference
	config.CacheEnabled = false
	config.Metrics = false
	config.Tracing = false

	sys, err := NewSystem(config, logger)
	require.NoError(t, err)
	assert.NotNil(t, sys)
	assert.NotNil(t, sys.Config)
	assert.NotNil(t, sys.Registry)
	assert.NotNil(t, sys.Executor)
	assert.NotNil(t, sys.Health)
	assert.NotNil(t, sys.Logger)

	// Shutdown
	err = sys.Shutdown()
	assert.NoError(t, err)
}

func TestNewSystem_WithCache(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := DefaultConfig()
	config.CacheEnabled = true
	config.CacheTTL = 1 * time.Hour
	config.Metrics = false
	config.Tracing = false

	sys, err := NewSystem(config, logger)
	require.NoError(t, err)
	assert.NotNil(t, sys)

	err = sys.Shutdown()
	assert.NoError(t, err)
}

func TestNewSystem_WithAllFeatures(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := DefaultConfig()

	sys, err := NewSystem(config, logger)
	require.NoError(t, err)
	assert.NotNil(t, sys)

	err = sys.Shutdown()
	assert.NoError(t, err)
}

func TestSystem_Shutdown(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	config := DefaultConfig()
	config.CacheEnabled = false
	config.Metrics = false
	config.Tracing = false

	sys, err := NewSystem(config, logger)
	require.NoError(t, err)

	// Register a formatter before shutdown
	mock := newMockFormatter("test", "1.0", []string{"go"})
	err = sys.Registry.Register(mock, &FormatterMetadata{
		Name: "test", Version: "1.0", Languages: []string{"go"},
		Type: FormatterTypeNative,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, sys.Registry.Count())

	err = sys.Shutdown()
	assert.NoError(t, err)
	assert.Equal(t, 0, sys.Registry.Count())
}
