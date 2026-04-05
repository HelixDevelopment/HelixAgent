package challenges

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadEnvFile_Basic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := `# comment
KEY1=value1
KEY2=value2
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	env, err := LoadEnvFile(path)
	require.NoError(t, err)
	assert.Equal(t, "value1", env["KEY1"])
	assert.Equal(t, "value2", env["KEY2"])
	assert.Len(t, env, 2)
}

func TestLoadEnvFile_QuotedValues(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := `DOUBLE="hello world"
SINGLE='single quoted'
NOQUOTE=plain
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	env, err := LoadEnvFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello world", env["DOUBLE"])
	assert.Equal(t, "single quoted", env["SINGLE"])
	assert.Equal(t, "plain", env["NOQUOTE"])
}

func TestLoadEnvFile_EmptyLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := `

KEY=val

# comment

`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	env, err := LoadEnvFile(path)
	require.NoError(t, err)
	assert.Equal(t, "val", env["KEY"])
	assert.Len(t, env, 1)
}

func TestLoadEnvFile_NoEquals(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := `VALID=yes
INVALID_LINE
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	env, err := LoadEnvFile(path)
	require.NoError(t, err)
	assert.Equal(t, "yes", env["VALID"])
	assert.Len(t, env, 1)
}

func TestLoadEnvFile_EmptyValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := `KEY=
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	env, err := LoadEnvFile(path)
	require.NoError(t, err)
	assert.Equal(t, "", env["KEY"])
}

func TestLoadEnvFile_NotFound(t *testing.T) {
	t.Parallel()
	_, err := LoadEnvFile("/nonexistent/.env")
	assert.Error(t, err)
}

func TestMergeEnvFiles_Override(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	path1 := filepath.Join(dir, "first.env")
	require.NoError(t, os.WriteFile(path1,
		[]byte("A=1\nB=2\n"), 0644))

	path2 := filepath.Join(dir, "second.env")
	require.NoError(t, os.WriteFile(path2,
		[]byte("B=overridden\nC=3\n"), 0644))

	env := MergeEnvFiles(path1, path2)
	assert.Equal(t, "1", env["A"])
	assert.Equal(t, "overridden", env["B"])
	assert.Equal(t, "3", env["C"])
}

func TestMergeEnvFiles_MissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path1 := filepath.Join(dir, "exists.env")
	require.NoError(t, os.WriteFile(path1,
		[]byte("KEY=val\n"), 0644))

	env := MergeEnvFiles(
		"/nonexistent/.env",
		path1,
	)
	assert.Equal(t, "val", env["KEY"])
}

func TestMergeEnvFiles_Empty(t *testing.T) {
	t.Parallel()
	env := MergeEnvFiles()
	assert.NotNil(t, env)
	assert.Empty(t, env)
}

func TestLoadEnvFile_SpacesAroundEquals(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := `KEY = value
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	env, err := LoadEnvFile(path)
	require.NoError(t, err)
	assert.Equal(t, "value", env["KEY"])
}
