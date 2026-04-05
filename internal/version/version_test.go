package version

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet_DefaultValues(t *testing.T) {
	t.Parallel()
	info := Get()

	assert.Equal(t, Version, info.Version)
	assert.Equal(t, VersionCode, info.VersionCode)
	assert.Equal(t, GitCommit, info.GitCommit)
	assert.Equal(t, GitBranch, info.GitBranch)
	assert.Equal(t, BuildDate, info.BuildDate)
	assert.Equal(t, SourceHash, info.SourceHash)
	assert.Equal(t, Builder, info.Builder)
	assert.Equal(t, runtime.Version(), info.GoVersion)
	assert.Equal(t, runtime.GOOS+"/"+runtime.GOARCH, info.Platform)
}

func TestGet_RuntimeFields(t *testing.T) {
	t.Parallel()
	info := Get()

	assert.True(t, strings.HasPrefix(info.GoVersion, "go"))
	assert.Contains(t, info.Platform, "/")
}

func TestInfo_JSONRoundtrip(t *testing.T) {
	t.Parallel()
	info := Get()

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var decoded Info
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, info.Version, decoded.Version)
	assert.Equal(t, info.VersionCode, decoded.VersionCode)
	assert.Equal(t, info.GitCommit, decoded.GitCommit)
	assert.Equal(t, info.GitBranch, decoded.GitBranch)
	assert.Equal(t, info.BuildDate, decoded.BuildDate)
	assert.Equal(t, info.SourceHash, decoded.SourceHash)
	assert.Equal(t, info.Builder, decoded.Builder)
	assert.Equal(t, info.GoVersion, decoded.GoVersion)
	assert.Equal(t, info.Platform, decoded.Platform)
}

func TestInfo_JSONFields(t *testing.T) {
	t.Parallel()
	info := Get()

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	expectedKeys := []string{
		"version", "version_code", "git_commit", "git_branch",
		"build_date", "source_hash", "builder", "go_version", "platform",
	}
	for _, key := range expectedKeys {
		assert.Contains(t, raw, key, "JSON should contain key: %s", key)
	}
}

func TestInfo_String(t *testing.T) {
	t.Parallel()
	info := Get()
	s := info.String()

	assert.Contains(t, s, "HelixAgent v")
	assert.Contains(t, s, "Version Code:")
	assert.Contains(t, s, "Git Commit:")
	assert.Contains(t, s, "Git Branch:")
	assert.Contains(t, s, "Build Date:")
	assert.Contains(t, s, "Source Hash:")
	assert.Contains(t, s, "Builder:")
	assert.Contains(t, s, "Go Version:")
	assert.Contains(t, s, "Platform:")
	assert.Contains(t, s, info.Version)
}

func TestShort(t *testing.T) {
	t.Parallel()
	s := Short()

	assert.True(t, strings.HasPrefix(s, "HelixAgent v"))
	assert.Contains(t, s, Version)
	assert.Contains(t, s, VersionCode)
	assert.Contains(t, s, GitCommit)
}

func TestInfo_JSON(t *testing.T) {
	t.Parallel()
	info := Get()
	j := info.JSON()

	assert.Contains(t, j, `"version"`)
	assert.Contains(t, j, `"version_code"`)

	var parsed map[string]interface{}
	err := json.Unmarshal([]byte(j), &parsed)
	require.NoError(t, err)
}

func TestInfo_String_MultiLine(t *testing.T) {
	t.Parallel()
	info := Get()
	s := info.String()

	lines := strings.Split(s, "\n")
	assert.Equal(t, 9, len(lines), "String() should have 9 lines")
}
