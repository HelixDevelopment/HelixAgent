package challenges

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.challenges/pkg/challenge"
)

func TestReporter_WriteResults_Summary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := NewReporter(dir)

	results := []*challenge.Result{
		{
			ChallengeID:   "test-1",
			ChallengeName: "Test One",
			Status:        challenge.StatusPassed,
			Duration:      100 * time.Millisecond,
		},
		{
			ChallengeID:   "test-2",
			ChallengeName: "Test Two",
			Status:        challenge.StatusFailed,
			Duration:      200 * time.Millisecond,
			Error:         "assertion failed",
		},
		{
			ChallengeID:   "test-3",
			ChallengeName: "Test Three",
			Status:        challenge.StatusStuck,
			Duration:      60 * time.Second,
			Error:         "stuck: no output",
		},
	}

	err := r.WriteResults(results)
	require.NoError(t, err)

	// Verify latest symlink exists.
	latestPath := filepath.Join(dir, "latest")
	info, err := os.Lstat(latestPath)
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0)

	// Read summary.
	target, err := os.Readlink(latestPath)
	require.NoError(t, err)
	summaryPath := filepath.Join(target, "summary.json")
	data, err := os.ReadFile(summaryPath)
	require.NoError(t, err)

	var summary ResultSummary
	require.NoError(t, json.Unmarshal(data, &summary))

	assert.Equal(t, 3, summary.Total)
	assert.Equal(t, 1, summary.Passed)
	assert.Equal(t, 1, summary.Failed)
	assert.Equal(t, 1, summary.Stuck)
	assert.Equal(t, 0, summary.TimedOut)
	assert.Len(t, summary.Results, 3)
}

func TestReporter_WriteResults_IndividualFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := NewReporter(dir)

	results := []*challenge.Result{
		{
			ChallengeID:   "single-test",
			ChallengeName: "Single",
			Status:        challenge.StatusPassed,
			Duration:      50 * time.Millisecond,
		},
	}

	err := r.WriteResults(results)
	require.NoError(t, err)

	// Check individual result file exists.
	latestPath := filepath.Join(dir, "latest")
	target, err := os.Readlink(latestPath)
	require.NoError(t, err)

	resultPath := filepath.Join(target, "single-test.json")
	_, err = os.Stat(resultPath)
	assert.NoError(t, err)
}

func TestReporter_WriteResults_Empty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := NewReporter(dir)

	err := r.WriteResults(nil)
	require.NoError(t, err)

	latestPath := filepath.Join(dir, "latest")
	target, err := os.Readlink(latestPath)
	require.NoError(t, err)

	summaryPath := filepath.Join(target, "summary.json")
	data, err := os.ReadFile(summaryPath)
	require.NoError(t, err)

	var summary ResultSummary
	require.NoError(t, json.Unmarshal(data, &summary))
	assert.Equal(t, 0, summary.Total)
}

func TestReporter_WriteResults_AllStatuses(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := NewReporter(dir)

	results := []*challenge.Result{
		{ChallengeID: "p", Status: challenge.StatusPassed},
		{ChallengeID: "f", Status: challenge.StatusFailed},
		{ChallengeID: "s", Status: challenge.StatusSkipped},
		{ChallengeID: "t", Status: challenge.StatusTimedOut},
		{ChallengeID: "k", Status: challenge.StatusStuck},
		{ChallengeID: "e", Status: challenge.StatusError},
	}

	err := r.WriteResults(results)
	require.NoError(t, err)

	latestPath := filepath.Join(dir, "latest")
	target, err := os.Readlink(latestPath)
	require.NoError(t, err)

	data, err := os.ReadFile(
		filepath.Join(target, "summary.json"),
	)
	require.NoError(t, err)

	var summary ResultSummary
	require.NoError(t, json.Unmarshal(data, &summary))

	assert.Equal(t, 6, summary.Total)
	assert.Equal(t, 1, summary.Passed)
	assert.Equal(t, 1, summary.Failed)
	assert.Equal(t, 1, summary.Skipped)
	assert.Equal(t, 1, summary.TimedOut)
	assert.Equal(t, 1, summary.Stuck)
	assert.Equal(t, 1, summary.Errors)
}

func TestNewReporter(t *testing.T) {
	t.Parallel()
	r := NewReporter("/tmp/test")
	assert.Equal(t, "/tmp/test", r.baseDir)
}
