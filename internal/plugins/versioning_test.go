package plugins

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	t.Run("valid version", func(t *testing.T) {
		v, err := ParseVersion("1.2.3")
		require.NoError(t, err)
		assert.Equal(t, 1, v.Major)
		assert.Equal(t, 2, v.Minor)
		assert.Equal(t, 3, v.Patch)
	})

	t.Run("version with zeros", func(t *testing.T) {
		v, err := ParseVersion("0.0.0")
		require.NoError(t, err)
		assert.Equal(t, 0, v.Major)
		assert.Equal(t, 0, v.Minor)
		assert.Equal(t, 0, v.Patch)
	})

	t.Run("large version numbers", func(t *testing.T) {
		v, err := ParseVersion("100.200.300")
		require.NoError(t, err)
		assert.Equal(t, 100, v.Major)
		assert.Equal(t, 200, v.Minor)
		assert.Equal(t, 300, v.Patch)
	})

	t.Run("invalid format - too few parts", func(t *testing.T) {
		_, err := ParseVersion("1.2")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid version format")
	})

	t.Run("invalid format - too many parts", func(t *testing.T) {
		_, err := ParseVersion("1.2.3.4")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid version format")
	})

	t.Run("invalid major version", func(t *testing.T) {
		_, err := ParseVersion("abc.2.3")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid major version")
	})

	t.Run("invalid minor version", func(t *testing.T) {
		_, err := ParseVersion("1.xyz.3")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid minor version")
	})

	t.Run("invalid patch version", func(t *testing.T) {
		_, err := ParseVersion("1.2.foo")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid patch version")
	})

	t.Run("empty string", func(t *testing.T) {
		_, err := ParseVersion("")
		assert.Error(t, err)
	})
}

func TestVersion_String(t *testing.T) {
	v := &Version{Major: 1, Minor: 2, Patch: 3}
	assert.Equal(t, "1.2.3", v.String())

	v2 := &Version{Major: 0, Minor: 0, Patch: 1}
	assert.Equal(t, "0.0.1", v2.String())
}

func TestVersion_Compare(t *testing.T) {
	t.Run("equal versions", func(t *testing.T) {
		v1 := &Version{Major: 1, Minor: 2, Patch: 3}
		v2 := &Version{Major: 1, Minor: 2, Patch: 3}
		assert.Equal(t, 0, v1.Compare(v2))
	})

	t.Run("different major - v1 greater", func(t *testing.T) {
		v1 := &Version{Major: 2, Minor: 0, Patch: 0}
		v2 := &Version{Major: 1, Minor: 9, Patch: 9}
		assert.Equal(t, 1, v1.Compare(v2))
	})

	t.Run("different major - v2 greater", func(t *testing.T) {
		v1 := &Version{Major: 1, Minor: 9, Patch: 9}
		v2 := &Version{Major: 2, Minor: 0, Patch: 0}
		assert.Equal(t, -1, v1.Compare(v2))
	})

	t.Run("different minor - v1 greater", func(t *testing.T) {
		v1 := &Version{Major: 1, Minor: 3, Patch: 0}
		v2 := &Version{Major: 1, Minor: 2, Patch: 9}
		assert.Equal(t, 1, v1.Compare(v2))
	})

	t.Run("different minor - v2 greater", func(t *testing.T) {
		v1 := &Version{Major: 1, Minor: 2, Patch: 9}
		v2 := &Version{Major: 1, Minor: 3, Patch: 0}
		assert.Equal(t, -1, v1.Compare(v2))
	})

	t.Run("different patch - v1 greater", func(t *testing.T) {
		v1 := &Version{Major: 1, Minor: 2, Patch: 4}
		v2 := &Version{Major: 1, Minor: 2, Patch: 3}
		assert.Equal(t, 1, v1.Compare(v2))
	})

	t.Run("different patch - v2 greater", func(t *testing.T) {
		v1 := &Version{Major: 1, Minor: 2, Patch: 3}
		v2 := &Version{Major: 1, Minor: 2, Patch: 4}
		assert.Equal(t, -1, v1.Compare(v2))
	})
}

func TestVersion_Compatible(t *testing.T) {
	t.Run("same major - compatible", func(t *testing.T) {
		v1 := &Version{Major: 1, Minor: 2, Patch: 3}
		v2 := &Version{Major: 1, Minor: 5, Patch: 0}
		assert.True(t, v1.Compatible(v2))
	})

	t.Run("different major - incompatible", func(t *testing.T) {
		v1 := &Version{Major: 1, Minor: 2, Patch: 3}
		v2 := &Version{Major: 2, Minor: 0, Patch: 0}
		assert.False(t, v1.Compatible(v2))
	})

	t.Run("major zero compatible", func(t *testing.T) {
		v1 := &Version{Major: 0, Minor: 1, Patch: 0}
		v2 := &Version{Major: 0, Minor: 2, Patch: 0}
		assert.True(t, v1.Compatible(v2))
	})
}

func TestVersionManager(t *testing.T) {
	registry := NewRegistry()

	t.Run("new version manager", func(t *testing.T) {
		vm := NewVersionManager(registry)
		require.NotNil(t, vm)
		assert.Equal(t, registry, vm.registry)
	})
}

func TestVersionManager_RegisterVersion(t *testing.T) {
	registry := NewRegistry()
	vm := NewVersionManager(registry)

	t.Run("register valid version", func(t *testing.T) {
		err := vm.RegisterVersion("test-plugin", "1.2.3")
		require.NoError(t, err)

		v, exists := vm.GetVersion("test-plugin")
		assert.True(t, exists)
		assert.Equal(t, "1.2.3", v.String())
	})

	t.Run("register invalid version", func(t *testing.T) {
		err := vm.RegisterVersion("bad-plugin", "invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid version")
	})

	t.Run("overwrite existing version", func(t *testing.T) {
		err := vm.RegisterVersion("overwrite-plugin", "1.0.0")
		require.NoError(t, err)

		err = vm.RegisterVersion("overwrite-plugin", "2.0.0")
		require.NoError(t, err)

		v, exists := vm.GetVersion("overwrite-plugin")
		assert.True(t, exists)
		assert.Equal(t, "2.0.0", v.String())
	})
}

func TestVersionManager_GetVersion(t *testing.T) {
	registry := NewRegistry()
	vm := NewVersionManager(registry)

	t.Run("get existing version", func(t *testing.T) {
		err := vm.RegisterVersion("existing", "1.0.0")
		require.NoError(t, err)

		v, exists := vm.GetVersion("existing")
		assert.True(t, exists)
		assert.NotNil(t, v)
	})

	t.Run("get non-existent version", func(t *testing.T) {
		v, exists := vm.GetVersion("non-existent")
		assert.False(t, exists)
		assert.Nil(t, v)
	})
}

func TestVersionManager_CheckCompatibility(t *testing.T) {
	registry := NewRegistry()
	vm := NewVersionManager(registry)
	err := vm.RegisterVersion("test-plugin", "2.1.0")
	require.NoError(t, err)

	t.Run("compatible version", func(t *testing.T) {
		err := vm.CheckCompatibility("test-plugin", "2.0.0")
		assert.NoError(t, err)
	})

	t.Run("incompatible version", func(t *testing.T) {
		err := vm.CheckCompatibility("test-plugin", "3.0.0")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not compatible")
	})

	t.Run("unregistered plugin", func(t *testing.T) {
		err := vm.CheckCompatibility("unknown", "1.0.0")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no version registered")
	})

	t.Run("invalid required version", func(t *testing.T) {
		err := vm.CheckCompatibility("test-plugin", "invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid required version")
	})
}

func TestVersionManager_IsUpdateAvailable(t *testing.T) {
	registry := NewRegistry()
	vm := NewVersionManager(registry)
	err := vm.RegisterVersion("test-plugin", "1.5.0")
	require.NoError(t, err)

	t.Run("newer version available", func(t *testing.T) {
		available, err := vm.IsUpdateAvailable("test-plugin", "2.0.0")
		require.NoError(t, err)
		assert.True(t, available)
	})

	t.Run("same version", func(t *testing.T) {
		available, err := vm.IsUpdateAvailable("test-plugin", "1.5.0")
		require.NoError(t, err)
		assert.False(t, available)
	})

	t.Run("older version", func(t *testing.T) {
		available, err := vm.IsUpdateAvailable("test-plugin", "1.0.0")
		require.NoError(t, err)
		assert.False(t, available)
	})

	t.Run("unregistered plugin", func(t *testing.T) {
		available, err := vm.IsUpdateAvailable("unknown", "1.0.0")
		require.NoError(t, err)
		assert.True(t, available) // No current version means update available
	})

	t.Run("invalid new version", func(t *testing.T) {
		_, err := vm.IsUpdateAvailable("test-plugin", "invalid")
		assert.Error(t, err)
	})
}

func TestVersionManager_UpdateVersion(t *testing.T) {
	registry := NewRegistry()
	vm := NewVersionManager(registry)

	t.Run("update version", func(t *testing.T) {
		err := vm.RegisterVersion("update-test", "1.0.0")
		require.NoError(t, err)

		err = vm.UpdateVersion("update-test", "2.0.0")
		require.NoError(t, err)

		v, exists := vm.GetVersion("update-test")
		assert.True(t, exists)
		assert.Equal(t, "2.0.0", v.String())
	})

	t.Run("update with invalid version", func(t *testing.T) {
		err := vm.UpdateVersion("update-test", "bad-version")
		assert.Error(t, err)
	})
}

func TestVersionManager_GetAllVersions(t *testing.T) {
	registry := NewRegistry()
	vm := NewVersionManager(registry)

	err := vm.RegisterVersion("plugin-a", "1.0.0")
	require.NoError(t, err)
	err = vm.RegisterVersion("plugin-b", "2.0.0")
	require.NoError(t, err)

	versions := vm.GetAllVersions()
	assert.Len(t, versions, 2)
	assert.Equal(t, "1.0.0", versions["plugin-a"])
	assert.Equal(t, "2.0.0", versions["plugin-b"])
}

func TestVersionManager_ValidateVersionConstraints(t *testing.T) {
	registry := NewRegistry()
	vm := NewVersionManager(registry)

	err := vm.RegisterVersion("plugin-a", "2.0.0")
	require.NoError(t, err)
	err = vm.RegisterVersion("plugin-b", "3.0.0")
	require.NoError(t, err)

	t.Run("all constraints satisfied", func(t *testing.T) {
		constraints := map[string]string{
			"plugin-a": "2.0.0",
			"plugin-b": "3.5.0",
		}
		err := vm.ValidateVersionConstraints(constraints)
		assert.NoError(t, err)
	})

	t.Run("constraint not satisfied", func(t *testing.T) {
		constraints := map[string]string{
			"plugin-a": "3.0.0", // Incompatible - current is 2.x
		}
		err := vm.ValidateVersionConstraints(constraints)
		assert.Error(t, err)
	})
}
