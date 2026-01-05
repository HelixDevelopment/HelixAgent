package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newProtocolPluginTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

func TestNewProtocolPluginSystem(t *testing.T) {
	log := newProtocolPluginTestLogger()
	ps := NewProtocolPluginSystem("/tmp/plugins", log)

	require.NotNil(t, ps)
	assert.NotNil(t, ps.plugins)
	assert.NotNil(t, ps.loadedPlugins)
	assert.Equal(t, "/tmp/plugins", ps.pluginDir)
}

func TestNewProtocolPluginRegistry(t *testing.T) {
	log := newProtocolPluginTestLogger()
	registry := NewProtocolPluginRegistry(log)

	require.NotNil(t, registry)
	assert.NotNil(t, registry.plugins)
	assert.NotNil(t, registry.logger)
}

func TestProtocolPluginRegistry_RegisterPlugin(t *testing.T) {
	log := newProtocolPluginTestLogger()
	registry := NewProtocolPluginRegistry(log)

	t.Run("register new plugin", func(t *testing.T) {
		plugin := &RegistryProtocolPlugin{
			ID:          "test-plugin-1",
			Name:        "Test Plugin",
			Version:     "1.0.0",
			Description: "A test plugin",
			Protocol:    "mcp",
			Tags:        []string{"test", "mcp"},
		}

		err := registry.RegisterPlugin(plugin)
		require.NoError(t, err)
		assert.False(t, plugin.CreatedAt.IsZero())
		assert.False(t, plugin.UpdatedAt.IsZero())
	})

	t.Run("register duplicate plugin", func(t *testing.T) {
		plugin := &RegistryProtocolPlugin{
			ID:   "duplicate-plugin",
			Name: "Duplicate",
		}

		err := registry.RegisterPlugin(plugin)
		require.NoError(t, err)

		// Try to register again
		err = registry.RegisterPlugin(plugin)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})
}

func TestProtocolPluginRegistry_GetPlugin(t *testing.T) {
	log := newProtocolPluginTestLogger()
	registry := NewProtocolPluginRegistry(log)

	// Register a plugin first
	plugin := &RegistryProtocolPlugin{
		ID:          "get-plugin",
		Name:        "Get Test Plugin",
		Version:     "1.0.0",
		Description: "A test plugin for get",
	}
	registry.RegisterPlugin(plugin)

	t.Run("get existing plugin", func(t *testing.T) {
		result, err := registry.GetPlugin("get-plugin")
		require.NoError(t, err)
		assert.Equal(t, "get-plugin", result.ID)
		assert.Equal(t, "Get Test Plugin", result.Name)
	})

	t.Run("get non-existent plugin", func(t *testing.T) {
		result, err := registry.GetPlugin("non-existent")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProtocolPluginRegistry_SearchPlugins(t *testing.T) {
	log := newProtocolPluginTestLogger()
	registry := NewProtocolPluginRegistry(log)

	// Register several plugins
	registry.RegisterPlugin(&RegistryProtocolPlugin{
		ID:          "mcp-plugin-1",
		Name:        "MCP Tool Manager",
		Description: "Manages MCP tools",
		Protocol:    "mcp",
		Tags:        []string{"tools", "mcp"},
	})
	registry.RegisterPlugin(&RegistryProtocolPlugin{
		ID:          "lsp-plugin-1",
		Name:        "LSP Formatter",
		Description: "Formats code using LSP",
		Protocol:    "lsp",
		Tags:        []string{"formatting", "lsp"},
	})
	registry.RegisterPlugin(&RegistryProtocolPlugin{
		ID:          "mcp-plugin-2",
		Name:        "MCP Resource Handler",
		Description: "Handles MCP resources",
		Protocol:    "mcp",
		Tags:        []string{"resources", "mcp"},
	})

	t.Run("search by protocol", func(t *testing.T) {
		results := registry.SearchPlugins("", "mcp", nil)
		assert.Len(t, results, 2)
	})

	t.Run("search by query", func(t *testing.T) {
		results := registry.SearchPlugins("Tool", "", nil)
		assert.Len(t, results, 1)
		assert.Equal(t, "mcp-plugin-1", results[0].ID)
	})

	t.Run("search by tags", func(t *testing.T) {
		results := registry.SearchPlugins("", "", []string{"tools"})
		assert.Len(t, results, 1)
		assert.Equal(t, "mcp-plugin-1", results[0].ID)
	})

	t.Run("search with no filters", func(t *testing.T) {
		results := registry.SearchPlugins("", "", nil)
		assert.Len(t, results, 3)
	})

	t.Run("search with no results", func(t *testing.T) {
		results := registry.SearchPlugins("nonexistent", "acp", nil)
		assert.Len(t, results, 0)
	})
}

func TestProtocolPluginRegistry_UpdatePluginStats(t *testing.T) {
	log := newProtocolPluginTestLogger()
	registry := NewProtocolPluginRegistry(log)

	plugin := &RegistryProtocolPlugin{
		ID:        "stats-plugin",
		Name:      "Stats Test Plugin",
		Downloads: 100,
		Rating:    3.5,
	}
	registry.RegisterPlugin(plugin)

	t.Run("update downloads and rating", func(t *testing.T) {
		err := registry.UpdatePluginStats("stats-plugin", 500, 4.5)
		require.NoError(t, err)

		updated, _ := registry.GetPlugin("stats-plugin")
		assert.Equal(t, 500, updated.Downloads)
		assert.Equal(t, 4.5, updated.Rating)
	})

	t.Run("update with negative downloads keeps old value", func(t *testing.T) {
		// Get current downloads
		current, _ := registry.GetPlugin("stats-plugin")
		oldDownloads := current.Downloads

		err := registry.UpdatePluginStats("stats-plugin", -1, 4.0)
		require.NoError(t, err)

		updated, _ := registry.GetPlugin("stats-plugin")
		assert.Equal(t, oldDownloads, updated.Downloads)
		assert.Equal(t, 4.0, updated.Rating)
	})

	t.Run("update non-existent plugin", func(t *testing.T) {
		err := registry.UpdatePluginStats("non-existent", 100, 4.0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProtocolPluginRegistry_ListPopularPlugins(t *testing.T) {
	log := newProtocolPluginTestLogger()
	registry := NewProtocolPluginRegistry(log)

	// Register plugins with different download counts
	for i := 1; i <= 5; i++ {
		registry.RegisterPlugin(&RegistryProtocolPlugin{
			ID:        "popular-" + string(rune('0'+i)),
			Name:      "Popular Plugin",
			Downloads: i * 100,
		})
	}

	t.Run("list with limit less than total", func(t *testing.T) {
		results := registry.ListPopularPlugins(3)
		assert.Len(t, results, 3)
	})

	t.Run("list with limit greater than total", func(t *testing.T) {
		results := registry.ListPopularPlugins(10)
		assert.Len(t, results, 5)
	})
}

func TestNewProtocolTemplateManager(t *testing.T) {
	log := newProtocolPluginTestLogger()
	tm := NewProtocolTemplateManager(log)

	require.NotNil(t, tm)
	assert.NotNil(t, tm.templates)
	assert.NotNil(t, tm.logger)
}

func TestProtocolTemplateManager_AddTemplate(t *testing.T) {
	log := newProtocolPluginTestLogger()
	tm := NewProtocolTemplateManager(log)

	t.Run("add new template", func(t *testing.T) {
		template := &ProtocolTemplate{
			ID:          "mcp-basic",
			Name:        "Basic MCP Plugin",
			Description: "A basic MCP plugin template",
			Protocol:    "mcp",
			Version:     "1.0.0",
			Files: map[string]string{
				"main.go": "package main",
			},
		}

		err := tm.AddTemplate(template)
		require.NoError(t, err)
		assert.False(t, template.CreatedAt.IsZero())
	})

	t.Run("add duplicate template", func(t *testing.T) {
		template := &ProtocolTemplate{
			ID:   "duplicate-template",
			Name: "Duplicate Template",
		}

		err := tm.AddTemplate(template)
		require.NoError(t, err)

		err = tm.AddTemplate(template)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestProtocolTemplateManager_GetTemplate(t *testing.T) {
	log := newProtocolPluginTestLogger()
	tm := NewProtocolTemplateManager(log)

	// Add a template first
	tm.AddTemplate(&ProtocolTemplate{
		ID:          "get-template",
		Name:        "Get Template Test",
		Description: "A template for testing get",
		Protocol:    "lsp",
	})

	t.Run("get existing template", func(t *testing.T) {
		result, err := tm.GetTemplate("get-template")
		require.NoError(t, err)
		assert.Equal(t, "get-template", result.ID)
		assert.Equal(t, "Get Template Test", result.Name)
	})

	t.Run("get non-existent template", func(t *testing.T) {
		result, err := tm.GetTemplate("non-existent")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProtocolPluginSystem_GetPlugin(t *testing.T) {
	log := newProtocolPluginTestLogger()
	ps := NewProtocolPluginSystem("/tmp/plugins", log)

	t.Run("get non-existent plugin", func(t *testing.T) {
		result, err := ps.GetPlugin("non-existent")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProtocolPluginSystem_ListPlugins(t *testing.T) {
	log := newProtocolPluginTestLogger()
	ps := NewProtocolPluginSystem("/tmp/plugins", log)

	t.Run("empty list", func(t *testing.T) {
		plugins := ps.ListPlugins()
		assert.Len(t, plugins, 0)
	})
}

func TestProtocolTemplateManager_ListTemplates(t *testing.T) {
	log := newProtocolPluginTestLogger()
	tm := NewProtocolTemplateManager(log)

	t.Run("empty list", func(t *testing.T) {
		templates := tm.ListTemplates()
		assert.Len(t, templates, 0)
	})

	t.Run("list with templates", func(t *testing.T) {
		tm.AddTemplate(&ProtocolTemplate{
			ID:       "list-template-1",
			Name:     "Template 1",
			Protocol: "mcp",
		})
		tm.AddTemplate(&ProtocolTemplate{
			ID:       "list-template-2",
			Name:     "Template 2",
			Protocol: "lsp",
		})

		templates := tm.ListTemplates()
		assert.Len(t, templates, 2)
	})
}

func TestProtocolTemplateManager_ListTemplatesByProtocol(t *testing.T) {
	log := newProtocolPluginTestLogger()
	tm := NewProtocolTemplateManager(log)

	// Add templates with different protocols
	tm.AddTemplate(&ProtocolTemplate{
		ID:       "proto-template-mcp-1",
		Name:     "MCP Template 1",
		Protocol: "mcp",
	})
	tm.AddTemplate(&ProtocolTemplate{
		ID:       "proto-template-mcp-2",
		Name:     "MCP Template 2",
		Protocol: "mcp",
	})
	tm.AddTemplate(&ProtocolTemplate{
		ID:       "proto-template-lsp",
		Name:     "LSP Template",
		Protocol: "lsp",
	})

	t.Run("list mcp templates", func(t *testing.T) {
		templates := tm.ListTemplatesByProtocol("mcp")
		assert.Len(t, templates, 2)
	})

	t.Run("list lsp templates", func(t *testing.T) {
		templates := tm.ListTemplatesByProtocol("lsp")
		assert.Len(t, templates, 1)
	})

	t.Run("list non-existent protocol", func(t *testing.T) {
		templates := tm.ListTemplatesByProtocol("acp")
		assert.Len(t, templates, 0)
	})
}

func TestProtocolTemplateManager_GeneratePluginFromTemplate(t *testing.T) {
	log := newProtocolPluginTestLogger()
	tm := NewProtocolTemplateManager(log)

	// Add a template
	tm.AddTemplate(&ProtocolTemplate{
		ID:          "generate-template",
		Name:        "Generate Template",
		Description: "A template for generation",
		Protocol:    "mcp",
		Version:     "1.0.0",
		Files: map[string]string{
			"main.go":   "package main",
			"plugin.go": "package main\n\nfunc init() {}",
		},
		Tags:         []string{"test"},
		Author:       "Test Author",
		Category:     "tools",
		Requirements: []string{"go >= 1.20"},
	})

	t.Run("generate from existing template", func(t *testing.T) {
		config := map[string]interface{}{
			"setting1": "value1",
		}
		result, err := tm.GeneratePluginFromTemplate("generate-template", config)
		require.NoError(t, err)
		assert.Contains(t, result.ID, "generate-template-generated-")
		assert.Equal(t, "Generate Template", result.Name)
		assert.Equal(t, config, result.Config)
		assert.Len(t, result.Files, 2)
		assert.Equal(t, []string{"test"}, result.Tags)
	})

	t.Run("generate from non-existent template", func(t *testing.T) {
		result, err := tm.GeneratePluginFromTemplate("non-existent", nil)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestProtocolTemplateManager_InitializeDefaultTemplates(t *testing.T) {
	log := newProtocolPluginTestLogger()
	tm := NewProtocolTemplateManager(log)

	err := tm.InitializeDefaultTemplates()
	require.NoError(t, err)

	// Should have default templates
	templates := tm.ListTemplates()
	assert.NotEmpty(t, templates)

	// Check that we have at least one mcp and one lsp template
	mcpTemplates := tm.ListTemplatesByProtocol("mcp")
	lspTemplates := tm.ListTemplatesByProtocol("lsp")
	assert.NotEmpty(t, mcpTemplates)
	assert.NotEmpty(t, lspTemplates)
}

// Type structure tests

func TestProtocolPlugin_Structure(t *testing.T) {
	plugin := &ProtocolPlugin{
		ID:          "test-plugin",
		Name:        "Test Plugin",
		Version:     "1.0.0",
		Description: "A test plugin",
		Protocol:    "mcp",
		Author:      "Test Author",
		License:     "MIT",
		Homepage:    "https://example.com",
		Metadata:    map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, "test-plugin", plugin.ID)
	assert.Equal(t, "Test Plugin", plugin.Name)
	assert.Equal(t, "1.0.0", plugin.Version)
	assert.Equal(t, "mcp", plugin.Protocol)
	assert.Equal(t, "MIT", plugin.License)
}

func TestRegistryProtocolPlugin_Structure(t *testing.T) {
	plugin := &RegistryProtocolPlugin{
		ID:          "registry-plugin",
		Name:        "Registry Plugin",
		Version:     "2.0.0",
		Description: "A registry test plugin",
		Protocol:    "lsp",
		Author:      "Registry Author",
		License:     "Apache-2.0",
		Tags:        []string{"test", "lsp"},
		Downloads:   1000,
		Rating:      4.5,
	}

	assert.Equal(t, "registry-plugin", plugin.ID)
	assert.Equal(t, "Registry Plugin", plugin.Name)
	assert.Equal(t, 1000, plugin.Downloads)
	assert.Equal(t, 4.5, plugin.Rating)
	assert.Len(t, plugin.Tags, 2)
}

func TestProtocolTemplate_Structure(t *testing.T) {
	template := &ProtocolTemplate{
		ID:           "template-1",
		Name:         "Test Template",
		Description:  "A test template",
		Protocol:     "mcp",
		Version:      "1.0.0",
		Files:        map[string]string{"main.go": "content"},
		Config:       map[string]interface{}{"setting": "value"},
		Tags:         []string{"template", "test"},
		Author:       "Template Author",
		Category:     "tools",
		Requirements: []string{"go >= 1.20"},
	}

	assert.Equal(t, "template-1", template.ID)
	assert.Equal(t, "Test Template", template.Name)
	assert.Len(t, template.Files, 1)
	assert.Len(t, template.Requirements, 1)
}

// Benchmarks

func BenchmarkProtocolPluginRegistry_RegisterPlugin(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	registry := NewProtocolPluginRegistry(log)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.RegisterPlugin(&RegistryProtocolPlugin{
			ID:   "bench-" + string(rune(i)),
			Name: "Benchmark Plugin",
		})
	}
}

func BenchmarkProtocolPluginRegistry_SearchPlugins(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	registry := NewProtocolPluginRegistry(log)

	// Pre-populate with plugins
	for i := 0; i < 100; i++ {
		registry.RegisterPlugin(&RegistryProtocolPlugin{
			ID:       "search-" + string(rune(i)),
			Name:     "Search Plugin",
			Protocol: "mcp",
			Tags:     []string{"test"},
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.SearchPlugins("Search", "mcp", []string{"test"})
	}
}

// Additional tests for error paths

func TestProtocolPluginSystem_UnloadPlugin_NonExistent(t *testing.T) {
	log := newProtocolPluginTestLogger()
	ps := NewProtocolPluginSystem("/tmp/plugins", log)

	err := ps.UnloadPlugin("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not loaded")
}

func TestProtocolPluginSystem_EnablePlugin_NonExistent(t *testing.T) {
	log := newProtocolPluginTestLogger()
	ps := NewProtocolPluginSystem("/tmp/plugins", log)

	err := ps.EnablePlugin("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not loaded")
}

func TestProtocolPluginSystem_DisablePlugin_NonExistent(t *testing.T) {
	log := newProtocolPluginTestLogger()
	ps := NewProtocolPluginSystem("/tmp/plugins", log)

	err := ps.DisablePlugin("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not loaded")
}

func TestProtocolPluginSystem_ExecutePluginOperation_NonExistent(t *testing.T) {
	log := newProtocolPluginTestLogger()
	ps := NewProtocolPluginSystem("/tmp/plugins", log)
	ctx := context.Background()

	result, err := ps.ExecutePluginOperation(ctx, "non-existent", "test-op", nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not loaded")
}

func TestProtocolPluginSystem_GetPluginCapabilities_NonExistent(t *testing.T) {
	log := newProtocolPluginTestLogger()
	ps := NewProtocolPluginSystem("/tmp/plugins", log)

	result, err := ps.GetPluginCapabilities("non-existent")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not loaded")
}

func TestProtocolPluginSystem_ConfigurePlugin_NonExistent(t *testing.T) {
	log := newProtocolPluginTestLogger()
	ps := NewProtocolPluginSystem("/tmp/plugins", log)

	err := ps.ConfigurePlugin("non-existent", map[string]interface{}{"key": "value"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not loaded")
}

func TestProtocolPluginSystem_DiscoverPlugins(t *testing.T) {
	log := newProtocolPluginTestLogger()

	t.Run("empty plugin directory path", func(t *testing.T) {
		ps := NewProtocolPluginSystem("", log)
		plugins, err := ps.DiscoverPlugins()
		require.NoError(t, err)
		assert.Len(t, plugins, 0)
	})

	t.Run("non-existent directory returns empty list", func(t *testing.T) {
		ps := NewProtocolPluginSystem("/non-existent-directory-for-testing-xyz123", log)
		plugins, err := ps.DiscoverPlugins()
		require.NoError(t, err)
		assert.Len(t, plugins, 0)
	})

	t.Run("discovers .so files in directory", func(t *testing.T) {
		// Create temporary directory with test plugin files
		tmpDir := t.TempDir()

		// Create some .so files
		soFile1 := filepath.Join(tmpDir, "plugin1.so")
		soFile2 := filepath.Join(tmpDir, "plugin2.so")
		err := os.WriteFile(soFile1, []byte("fake plugin"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(soFile2, []byte("fake plugin"), 0644)
		require.NoError(t, err)

		ps := NewProtocolPluginSystem(tmpDir, log)
		plugins, err := ps.DiscoverPlugins()
		require.NoError(t, err)
		assert.Len(t, plugins, 2)
		assert.Contains(t, plugins, soFile1)
		assert.Contains(t, plugins, soFile2)
	})

	t.Run("ignores non-.so files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create various files
		soFile := filepath.Join(tmpDir, "valid.so")
		txtFile := filepath.Join(tmpDir, "readme.txt")
		goFile := filepath.Join(tmpDir, "main.go")
		err := os.WriteFile(soFile, []byte("fake plugin"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(txtFile, []byte("readme content"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(goFile, []byte("package main"), 0644)
		require.NoError(t, err)

		ps := NewProtocolPluginSystem(tmpDir, log)
		plugins, err := ps.DiscoverPlugins()
		require.NoError(t, err)
		assert.Len(t, plugins, 1)
		assert.Contains(t, plugins, soFile)
	})

	t.Run("ignores directories", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a .so file
		soFile := filepath.Join(tmpDir, "plugin.so")
		err := os.WriteFile(soFile, []byte("fake plugin"), 0644)
		require.NoError(t, err)

		// Create a directory named like a .so file
		dirWithSoName := filepath.Join(tmpDir, "subdir.so")
		err = os.Mkdir(dirWithSoName, 0755)
		require.NoError(t, err)

		ps := NewProtocolPluginSystem(tmpDir, log)
		plugins, err := ps.DiscoverPlugins()
		require.NoError(t, err)
		assert.Len(t, plugins, 1)
		assert.Contains(t, plugins, soFile)
	})

	t.Run("empty directory returns empty list", func(t *testing.T) {
		tmpDir := t.TempDir()

		ps := NewProtocolPluginSystem(tmpDir, log)
		plugins, err := ps.DiscoverPlugins()
		require.NoError(t, err)
		assert.Len(t, plugins, 0)
	})

	t.Run("returns error when path is a file not a directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "notadir")
		err := os.WriteFile(filePath, []byte("content"), 0644)
		require.NoError(t, err)

		ps := NewProtocolPluginSystem(filePath, log)
		plugins, err := ps.DiscoverPlugins()
		assert.Error(t, err)
		assert.Nil(t, plugins)
		assert.Contains(t, err.Error(), "not a directory")
	})
}

func TestProtocolPluginSystem_AutoLoadPlugins(t *testing.T) {
	log := newProtocolPluginTestLogger()

	t.Run("non-existent directory does not error", func(t *testing.T) {
		ps := NewProtocolPluginSystem("/non-existent-path-for-testing", log)
		err := ps.AutoLoadPlugins()
		// Should not error since DiscoverPlugins handles non-existent directory gracefully
		require.NoError(t, err)
		// No plugins should be loaded
		assert.Len(t, ps.ListPlugins(), 0)
	})

	t.Run("empty directory loads no plugins", func(t *testing.T) {
		tmpDir := t.TempDir()
		ps := NewProtocolPluginSystem(tmpDir, log)
		err := ps.AutoLoadPlugins()
		require.NoError(t, err)
		assert.Len(t, ps.ListPlugins(), 0)
	})

	t.Run("fails gracefully for invalid plugin files", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Create a fake .so file that isn't a real plugin
		soFile := filepath.Join(tmpDir, "fake.so")
		err := os.WriteFile(soFile, []byte("not a real plugin"), 0644)
		require.NoError(t, err)

		ps := NewProtocolPluginSystem(tmpDir, log)
		// Should not error overall, but individual plugin load will fail
		err = ps.AutoLoadPlugins()
		require.NoError(t, err)
		// No valid plugins loaded
		assert.Len(t, ps.ListPlugins(), 0)
	})
}

func TestProtocolPluginSystem_LoadPlugin_InvalidPath(t *testing.T) {
	log := newProtocolPluginTestLogger()
	ps := NewProtocolPluginSystem("/tmp/plugins", log)

	err := ps.LoadPlugin("/non-existent-plugin-path.so")
	assert.Error(t, err)
}

// Additional DiscoverPlugins edge case tests

func TestProtocolPluginSystem_DiscoverPlugins_SymlinksAndSpecialFiles(t *testing.T) {
	log := newProtocolPluginTestLogger()

	t.Run("handles multiple .so files with special names", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create .so files with various naming patterns
		files := []string{
			"my-plugin.so",
			"plugin_v1.2.3.so",
			"libplugin.so",
			"plugin.so.backup", // Should NOT be discovered (doesn't end with .so)
		}

		for _, f := range files {
			path := filepath.Join(tmpDir, f)
			err := os.WriteFile(path, []byte("content"), 0644)
			require.NoError(t, err)
		}

		ps := NewProtocolPluginSystem(tmpDir, log)
		plugins, err := ps.DiscoverPlugins()
		require.NoError(t, err)
		// Only files ending with .so should be discovered
		assert.Len(t, plugins, 3)
	})

	t.Run("handles .so files in directory root only - does not recurse", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create .so in root
		rootSo := filepath.Join(tmpDir, "root.so")
		err := os.WriteFile(rootSo, []byte("content"), 0644)
		require.NoError(t, err)

		// Create subdirectory with .so file
		subDir := filepath.Join(tmpDir, "subdir")
		err = os.Mkdir(subDir, 0755)
		require.NoError(t, err)

		subSo := filepath.Join(subDir, "nested.so")
		err = os.WriteFile(subSo, []byte("content"), 0644)
		require.NoError(t, err)

		ps := NewProtocolPluginSystem(tmpDir, log)
		plugins, err := ps.DiscoverPlugins()
		require.NoError(t, err)
		// Only root level .so file should be found
		assert.Len(t, plugins, 1)
		assert.Contains(t, plugins, rootSo)
	})
}

func TestProtocolPluginSystem_DiscoverPlugins_ReturnsFullPaths(t *testing.T) {
	log := newProtocolPluginTestLogger()
	tmpDir := t.TempDir()

	soFile := filepath.Join(tmpDir, "test-plugin.so")
	err := os.WriteFile(soFile, []byte("content"), 0644)
	require.NoError(t, err)

	ps := NewProtocolPluginSystem(tmpDir, log)
	plugins, err := ps.DiscoverPlugins()
	require.NoError(t, err)
	require.Len(t, plugins, 1)

	// Ensure full path is returned, not just filename
	assert.Equal(t, soFile, plugins[0])
	assert.True(t, filepath.IsAbs(plugins[0]))
}
