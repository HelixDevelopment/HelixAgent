package grpcclient

import (
	"testing"
	"time"

	"github.com/elder-plinius/go-plinius-common/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	cfg := config.New("test")
	client := New(cfg)
	assert.NotNil(t, client)
	assert.False(t, client.IsConnected())
}

func TestIsConnectedBeforeConnect(t *testing.T) {
	cfg := config.New("test")
	client := New(cfg)
	assert.False(t, client.IsConnected())
}

func TestCloseWithoutConnect(t *testing.T) {
	cfg := config.New("test")
	client := New(cfg)
	err := client.Close()
	assert.NoError(t, err)
}

func TestDoubleClose(t *testing.T) {
	cfg := config.New("test")
	client := New(cfg)
	assert.NoError(t, client.Close())
	assert.NoError(t, client.Close()) // Should be no-op
}

func TestConnectionNilWhenNotConnected(t *testing.T) {
	cfg := config.New("test")
	client := New(cfg)
	assert.Nil(t, client.Connection())
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name    string
		base    time.Duration
		max     time.Duration
		attempt int
		minWant time.Duration
		maxWant time.Duration
	}{
		{"first attempt", time.Second, 30 * time.Second, 1, 750 * time.Millisecond, 1250 * time.Millisecond},
		{"second attempt", time.Second, 30 * time.Second, 2, 1500 * time.Millisecond, 2500 * time.Millisecond},
		{"capped", time.Second, 2 * time.Second, 10, 1500 * time.Millisecond, 2500 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateBackoff(tt.base, tt.max, tt.attempt)
			assert.GreaterOrEqual(t, result, tt.minWant)
			assert.LessOrEqual(t, result, tt.maxWant)
		})
	}
}

func TestContextWithMetadata(t *testing.T) {
	cfg := config.New("test",
		WithAuthToken("test-token"),
	)
	client := New(cfg)
	// Cannot fully test without connection, but we can verify it doesn't panic
	ctx := client.ContextWithMetadata(nil)
	assert.NotNil(t, ctx)
}

func TestClientConfiguration(t *testing.T) {
	cfg := config.New("autotemp",
		config.WithAddress("autotemp:50051"),
		config.WithTimeout(45*time.Second),
		config.WithMaxRetries(5),
		config.WithAuthToken("token123"),
	)
	client := New(cfg)
	assert.NotNil(t, client)
	assert.Equal(t, "autotemp:50051", cfg.Address)
	assert.Equal(t, 45*time.Second, cfg.Timeout)
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, "token123", cfg.AuthToken)
}
