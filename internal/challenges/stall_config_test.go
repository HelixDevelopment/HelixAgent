package challenges

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStallThresholdForCategory_Known(t *testing.T) {
	t.Parallel()
	tests := []struct {
		category string
		expected time.Duration
	}{
		{"provider", 120 * time.Second},
		{"performance", 180 * time.Second},
		{"debate", 120 * time.Second},
		{"security", 120 * time.Second},
		{"mcp", 90 * time.Second},
		{"cli", 90 * time.Second},
		{"bigdata", 120 * time.Second},
		{"memory", 90 * time.Second},
		{"default", 60 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.category, func(t *testing.T) {
				t.Parallel()
			assert.Equal(t, tc.expected,
				StallThresholdForCategory(tc.category))
		})
	}
}

func TestStallThresholdForCategory_Unknown(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 60*time.Second,
		StallThresholdForCategory("unknown"))
	assert.Equal(t, 60*time.Second,
		StallThresholdForCategory(""))
}

func TestCategoryStallThresholds_HasDefault(t *testing.T) {
	t.Parallel()
	_, ok := CategoryStallThresholds["default"]
	assert.True(t, ok, "must have a default category")
}

func TestCategoryStallThresholds_AllPositive(t *testing.T) {
	t.Parallel()
	for cat, dur := range CategoryStallThresholds {
		assert.True(t, dur > 0,
			"category %s has non-positive threshold", cat)
	}
}
