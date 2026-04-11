package formatters

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionsManifest_GetVersion_Native(t *testing.T) {
	t.Parallel()
	m := &VersionsManifest{
		Native: map[string]VersionInfo{
			"black": {Version: "26.1a1", Commit: "abc123", GitRef: "v26.1a1"},
		},
	}

	info, ok := m.GetVersion("black", FormatterTypeNative)
	assert.True(t, ok)
	assert.Equal(t, "26.1a1", info.Version)
	assert.Equal(t, "abc123", info.Commit)
	assert.Equal(t, "v26.1a1", info.GitRef)
}

func TestVersionsManifest_GetVersion_Service(t *testing.T) {
	t.Parallel()
	m := &VersionsManifest{
		Service: map[string]VersionInfo{
			"sqlfluff": {Version: "3.4.1"},
		},
	}

	info, ok := m.GetVersion("sqlfluff", FormatterTypeService)
	assert.True(t, ok)
	assert.Equal(t, "3.4.1", info.Version)
}

func TestVersionsManifest_GetVersion_Builtin(t *testing.T) {
	t.Parallel()
	m := &VersionsManifest{
		Builtin: map[string]VersionInfo{
			"gofmt": {Version: "1.22.0"},
		},
	}

	info, ok := m.GetVersion("gofmt", FormatterTypeBuiltin)
	assert.True(t, ok)
	assert.Equal(t, "1.22.0", info.Version)
}

func TestVersionsManifest_GetVersion_NotFound(t *testing.T) {
	t.Parallel()
	m := &VersionsManifest{
		Native: map[string]VersionInfo{},
	}

	_, ok := m.GetVersion("nonexistent", FormatterTypeNative)
	assert.False(t, ok)
}

func TestVersionsManifest_GetVersion_UnknownType(t *testing.T) {
	t.Parallel()
	m := &VersionsManifest{}

	_, ok := m.GetVersion("test", FormatterType("custom"))
	assert.False(t, ok)
}

func TestVersionsManifest_SetVersion_Native(t *testing.T) {
	t.Parallel()
	m := &VersionsManifest{}
	m.SetVersion("black", FormatterTypeNative, VersionInfo{Version: "26.1a1"})

	info, ok := m.Native["black"]
	assert.True(t, ok)
	assert.Equal(t, "26.1a1", info.Version)
}

func TestVersionsManifest_SetVersion_Service(t *testing.T) {
	t.Parallel()
	m := &VersionsManifest{}
	m.SetVersion("sqlfluff", FormatterTypeService, VersionInfo{Version: "3.4.1"})

	info, ok := m.Service["sqlfluff"]
	assert.True(t, ok)
	assert.Equal(t, "3.4.1", info.Version)
}

func TestVersionsManifest_SetVersion_Builtin(t *testing.T) {
	t.Parallel()
	m := &VersionsManifest{}
	m.SetVersion("gofmt", FormatterTypeBuiltin, VersionInfo{Version: "1.22.0"})

	info, ok := m.Builtin["gofmt"]
	assert.True(t, ok)
	assert.Equal(t, "1.22.0", info.Version)
}

func TestVersionsManifest_SetVersion_UnknownType(t *testing.T) {
	t.Parallel()
	m := &VersionsManifest{}
	// Should not panic, just do nothing
	m.SetVersion("test", FormatterType("custom"), VersionInfo{Version: "1.0"})
	assert.Nil(t, m.Native)
	assert.Nil(t, m.Service)
	assert.Nil(t, m.Builtin)
}

func TestVersionsManifest_SetVersion_InitializesNilMap(t *testing.T) {
	t.Parallel()
	m := &VersionsManifest{}
	assert.Nil(t, m.Native)

	m.SetVersion("black", FormatterTypeNative, VersionInfo{Version: "1.0"})
	assert.NotNil(t, m.Native)
	assert.Equal(t, "1.0", m.Native["black"].Version)
}

func TestVersionsManifest_AllVersions(t *testing.T) {
	t.Parallel()
	m := &VersionsManifest{
		Native:  map[string]VersionInfo{"black": {Version: "26.1a1"}},
		Service: map[string]VersionInfo{"sqlfluff": {Version: "3.4.1"}},
		Builtin: map[string]VersionInfo{"gofmt": {Version: "1.22.0"}},
	}

	all := m.AllVersions()
	assert.Len(t, all, 3)
	assert.Equal(t, "26.1a1", all["black"].Version)
	assert.Equal(t, "3.4.1", all["sqlfluff"].Version)
	assert.Equal(t, "1.22.0", all["gofmt"].Version)
}

func TestVersionsManifest_AllVersions_Empty(t *testing.T) {
	t.Parallel()
	m := &VersionsManifest{}
	all := m.AllVersions()
	assert.Empty(t, all)
}

func TestVersionsManifest_Count(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		manifest *VersionsManifest
		expected int
	}{
		{
			"empty",
			&VersionsManifest{},
			0,
		},
		{
			"mixed",
			&VersionsManifest{
				Native:  map[string]VersionInfo{"a": {}, "b": {}},
				Service: map[string]VersionInfo{"c": {}},
				Builtin: map[string]VersionInfo{"d": {}},
			},
			4,
		},
		{
			"native only",
			&VersionsManifest{
				Native: map[string]VersionInfo{"a": {}, "b": {}, "c": {}},
			},
			3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, tc.manifest.Count())
		})
	}
}

func TestLoadVersionsManifest(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "versions.yaml")

	yamlContent := `native:
  black:
    version: "26.1a1"
    commit: "abc123"
    git_ref: "v26.1a1"
service:
  sqlfluff:
    version: "3.4.1"
builtin:
  gofmt:
    version: "1.22.0"
    note: "bundled with go"
`
	err := os.WriteFile(path, []byte(yamlContent), 0644)
	require.NoError(t, err)

	m, err := LoadVersionsManifest(path)
	require.NoError(t, err)

	assert.Equal(t, "26.1a1", m.Native["black"].Version)
	assert.Equal(t, "abc123", m.Native["black"].Commit)
	assert.Equal(t, "3.4.1", m.Service["sqlfluff"].Version)
	assert.Equal(t, "1.22.0", m.Builtin["gofmt"].Version)
	assert.Equal(t, "bundled with go", m.Builtin["gofmt"].Note)
}

func TestLoadVersionsManifest_FileNotFound(t *testing.T) {
	t.Parallel()
	_, err := LoadVersionsManifest("/nonexistent/versions.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read versions manifest")
}

func TestLoadVersionsManifest_InvalidYAML(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.yaml")

	err := os.WriteFile(path, []byte("{{invalid yaml"), 0644)
	require.NoError(t, err)

	_, err = LoadVersionsManifest(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse versions manifest")
}

func TestSaveVersionsManifest(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "versions.yaml")

	m := &VersionsManifest{
		Native: map[string]VersionInfo{
			"black": {Version: "26.1a1", Commit: "abc123"},
		},
		Service: map[string]VersionInfo{
			"sqlfluff": {Version: "3.4.1"},
		},
	}

	err := SaveVersionsManifest(m, path)
	require.NoError(t, err)

	// Read back
	loaded, err := LoadVersionsManifest(path)
	require.NoError(t, err)
	assert.Equal(t, "26.1a1", loaded.Native["black"].Version)
	assert.Equal(t, "3.4.1", loaded.Service["sqlfluff"].Version)
}

func TestSaveVersionsManifest_InvalidPath(t *testing.T) {
	t.Parallel()
	m := &VersionsManifest{}
	err := SaveVersionsManifest(m, "/nonexistent/dir/versions.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write versions manifest")
}
