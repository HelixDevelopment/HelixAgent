//go:build performance

package performance

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseline_FilesExist(t *testing.T) {
	baselines := []string{
		"../../benchmarks/baselines/handlers.txt",
		"../../benchmarks/baselines/pool.txt",
		"../../benchmarks/baselines/http.txt",
		"../../benchmarks/baselines/ensemble.txt",
	}
	for _, path := range baselines {
		_, err := os.Stat(path)
		assert.NoError(t, err, "baseline %s must exist — run: make benchmark-baseline", path)
	}
}
