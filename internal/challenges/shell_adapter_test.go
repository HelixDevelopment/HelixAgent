package challenges

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.challenges/pkg/registry"
)

func TestRegisterShellChallenges_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := registry.NewRegistry()

	err := RegisterShellChallenges(reg, dir, "")
	require.NoError(t, err)
	assert.Equal(t, 0, reg.Count())
}

func TestRegisterShellChallenges_WithScripts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create test scripts matching the _challenge.sh pattern.
	scripts := []string{
		"provider_verification_challenge.sh",
		"api_quality_challenge.sh",
		"not_a_challenge.txt",
	}
	for _, s := range scripts {
		err := os.WriteFile(
			filepath.Join(dir, s),
			[]byte("#!/bin/bash\necho ok"),
			0755,
		)
		require.NoError(t, err)
	}

	reg := registry.NewRegistry()
	err := RegisterShellChallenges(reg, dir, "")
	require.NoError(t, err)

	// Only _challenge.sh files should be registered.
	assert.Equal(t, 2, reg.Count())
}

func TestRegisterShellChallenges_TestScripts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// _test.sh files should also be registered.
	err := os.WriteFile(
		filepath.Join(dir, "smoke_test.sh"),
		[]byte("#!/bin/bash\necho ok"),
		0755,
	)
	require.NoError(t, err)

	reg := registry.NewRegistry()
	err = RegisterShellChallenges(reg, dir, "")
	require.NoError(t, err)

	assert.Equal(t, 1, reg.Count())
}

func TestRegisterShellChallenges_NonexistentDir(t *testing.T) {
	t.Parallel()
	reg := registry.NewRegistry()
	err := RegisterShellChallenges(reg, "/nonexistent/path", "")
	require.Error(t, err)
}

func TestRegisterShellChallenges_SkipsDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a subdirectory named like a challenge.
	err := os.Mkdir(
		filepath.Join(dir, "some_challenge.sh"),
		0755,
	)
	require.NoError(t, err)

	reg := registry.NewRegistry()
	err = RegisterShellChallenges(reg, dir, "")
	require.NoError(t, err)
	assert.Equal(t, 0, reg.Count())
}

func TestFormatName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{
			"provider-verification-challenge",
			"Provider Verification Challenge",
		},
		{"api-quality", "Api Quality"},
		{"simple", "Simple"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, formatName(tt.input))
		})
	}
}
