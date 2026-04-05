package specifier_test

import (
	"testing"

	adapter "dev.helix.agent/internal/adapters/specifier"
	"github.com/stretchr/testify/assert"
)

func TestFactoryReturnsConsistentState(t *testing.T) {
	t.Parallel()
	enabled := adapter.IsHelixSpecifierEnabled()
	sa := adapter.NewOptimalSpecAdapter()

	if enabled {
		assert.NotNil(t, sa)
		assert.Equal(t, "digital.vasic.helixspecifier",
			adapter.SpecifierBackendName())
	} else {
		assert.Nil(t, sa)
		assert.Equal(t, "internal.speckit",
			adapter.SpecifierBackendName())
	}
}
