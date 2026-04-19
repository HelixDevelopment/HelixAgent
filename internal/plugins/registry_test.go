package plugins

import (
	"runtime"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	runtime.GOMAXPROCS(2)
}

// ---- NewRegistry ----

func TestNewRegistry_Initialization(t *testing.T) {
	registry := NewRegistry()
	require.NotNil(t, registry)
	assert.NotNil(t, registry.plugins)
	assert.Equal(t, 0, registry.plugins.Len())
}

func TestNewRegistry_EmptyList(t *testing.T) {
	registry := NewRegistry()
	list := registry.List()
	assert.NotNil(t, list)
	assert.Empty(t, list)
}

// ---- Register ----

func TestRegistry_Register_SinglePlugin(t *testing.T) {
	registry := NewRegistry()
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("single-plugin")

	err := registry.Register(plugin)
	assert.NoError(t, err)
	assert.Len(t, registry.List(), 1)
}

func TestRegistry_Register_MultiplePlugins(t *testing.T) {
	registry := NewRegistry()

	names := []string{"plugin-a", "plugin-b", "plugin-c"}
	for _, name := range names {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return(name)
		err := registry.Register(plugin)
		require.NoError(t, err)
	}

	assert.Len(t, registry.List(), 3)
}

func TestRegistry_Register_DuplicateName(t *testing.T) {
	registry := NewRegistry()

	plugin1 := new(MockLLMPlugin)
	plugin1.On("Name").Return("dup-plugin")
	plugin2 := new(MockLLMPlugin)
	plugin2.On("Name").Return("dup-plugin")

	err := registry.Register(plugin1)
	require.NoError(t, err)

	err = registry.Register(plugin2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")

	// Only one should be registered
	assert.Len(t, registry.List(), 1)
}

func TestRegistry_Register_DifferentNames(t *testing.T) {
	registry := NewRegistry()

	plugin1 := new(MockLLMPlugin)
	plugin1.On("Name").Return("unique-1")
	plugin2 := new(MockLLMPlugin)
	plugin2.On("Name").Return("unique-2")

	err := registry.Register(plugin1)
	require.NoError(t, err)
	err = registry.Register(plugin2)
	require.NoError(t, err)

	assert.Len(t, registry.List(), 2)
}

// ---- Unregister ----

func TestRegistry_Unregister_Existing(t *testing.T) {
	registry := NewRegistry()
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("unreg-me")

	err := registry.Register(plugin)
	require.NoError(t, err)
	assert.Len(t, registry.List(), 1)

	err = registry.Unregister("unreg-me")
	assert.NoError(t, err)
	assert.Empty(t, registry.List())
}

func TestRegistry_Unregister_NonExistent(t *testing.T) {
	registry := NewRegistry()
	err := registry.Unregister("ghost-plugin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRegistry_Unregister_ThenRegisterSameName(t *testing.T) {
	registry := NewRegistry()

	plugin1 := new(MockLLMPlugin)
	plugin1.On("Name").Return("recyclable")
	err := registry.Register(plugin1)
	require.NoError(t, err)

	err = registry.Unregister("recyclable")
	require.NoError(t, err)

	// Re-register with same name should work
	plugin2 := new(MockLLMPlugin)
	plugin2.On("Name").Return("recyclable")
	err = registry.Register(plugin2)
	assert.NoError(t, err)

	// Should get the new plugin
	got, exists := registry.Get("recyclable")
	assert.True(t, exists)
	assert.Equal(t, plugin2, got)
}

func TestRegistry_Unregister_OneOfMany(t *testing.T) {
	registry := NewRegistry()

	names := []string{"keep-a", "remove-b", "keep-c"}
	for _, name := range names {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return(name)
		_ = registry.Register(plugin)
	}

	err := registry.Unregister("remove-b")
	require.NoError(t, err)

	list := registry.List()
	assert.Len(t, list, 2)
	assert.Contains(t, list, "keep-a")
	assert.Contains(t, list, "keep-c")
	assert.NotContains(t, list, "remove-b")
}

func TestRegistry_Unregister_AlreadyUnregistered(t *testing.T) {
	registry := NewRegistry()
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("double-unreg")

	_ = registry.Register(plugin)
	err := registry.Unregister("double-unreg")
	require.NoError(t, err)

	err = registry.Unregister("double-unreg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ---- Get ----

func TestRegistry_Get_Existing(t *testing.T) {
	registry := NewRegistry()
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("get-me")

	_ = registry.Register(plugin)

	got, exists := registry.Get("get-me")
	assert.True(t, exists)
	assert.Equal(t, plugin, got)
}

func TestRegistry_Get_NonExistent(t *testing.T) {
	registry := NewRegistry()

	got, exists := registry.Get("invisible")
	assert.False(t, exists)
	assert.Nil(t, got)
}

func TestRegistry_Get_AfterUnregister(t *testing.T) {
	registry := NewRegistry()
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("gone")

	_ = registry.Register(plugin)
	_ = registry.Unregister("gone")

	got, exists := registry.Get("gone")
	assert.False(t, exists)
	assert.Nil(t, got)
}

func TestRegistry_Get_EmptyRegistry(t *testing.T) {
	registry := NewRegistry()

	got, exists := registry.Get("")
	assert.False(t, exists)
	assert.Nil(t, got)
}

// ---- List ----

func TestRegistry_List_Empty(t *testing.T) {
	registry := NewRegistry()
	list := registry.List()
	assert.NotNil(t, list)
	assert.Empty(t, list)
}

func TestRegistry_List_SinglePlugin(t *testing.T) {
	registry := NewRegistry()
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("lone-plugin")
	_ = registry.Register(plugin)

	list := registry.List()
	assert.Len(t, list, 1)
	assert.Equal(t, "lone-plugin", list[0])
}

func TestRegistry_List_MultiplePlugins_ContainsAll(t *testing.T) {
	registry := NewRegistry()

	expected := []string{"alpha", "beta", "gamma", "delta"}
	for _, name := range expected {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return(name)
		_ = registry.Register(plugin)
	}

	list := registry.List()
	assert.Len(t, list, 4)

	sort.Strings(list)
	sort.Strings(expected)
	assert.Equal(t, expected, list)
}

func TestRegistry_List_AfterUnregister(t *testing.T) {
	registry := NewRegistry()

	for _, name := range []string{"x", "y", "z"} {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return(name)
		_ = registry.Register(plugin)
	}

	_ = registry.Unregister("y")

	list := registry.List()
	assert.Len(t, list, 2)
	assert.Contains(t, list, "x")
	assert.Contains(t, list, "z")
	assert.NotContains(t, list, "y")
}

// ---- Concurrent access ----

func TestRegistry_ConcurrentRegister(t *testing.T) {
	registry := NewRegistry()
	var wg sync.WaitGroup

	// Register 50 plugins concurrently (each with unique name)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			plugin := new(MockLLMPlugin)
			name := "conc-reg-" + string(rune('A'+idx%26)) + string(rune('a'+idx/26))
			plugin.On("Name").Return(name)
			_ = registry.Register(plugin)
		}(i)
	}
	wg.Wait()

	list := registry.List()
	assert.Equal(t, 50, len(list))
}

func TestRegistry_ConcurrentGet(t *testing.T) {
	registry := NewRegistry()
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("conc-get-target")
	_ = registry.Register(plugin)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, exists := registry.Get("conc-get-target")
			assert.True(t, exists)
			assert.NotNil(t, got)
		}()
	}
	wg.Wait()
}

func TestRegistry_ConcurrentList(t *testing.T) {
	registry := NewRegistry()

	for i := 0; i < 10; i++ {
		plugin := new(MockLLMPlugin)
		name := "conc-list-" + string(rune('a'+i))
		plugin.On("Name").Return(name)
		_ = registry.Register(plugin)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			list := registry.List()
			assert.Len(t, list, 10)
		}()
	}
	wg.Wait()
}

func TestRegistry_ConcurrentRegisterAndGet(t *testing.T) {
	registry := NewRegistry()
	var wg sync.WaitGroup

	// Pre-register some plugins
	for i := 0; i < 5; i++ {
		plugin := new(MockLLMPlugin)
		name := "pre-" + string(rune('a'+i))
		plugin.On("Name").Return(name)
		_ = registry.Register(plugin)
	}

	// Concurrently register new and get existing
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			plugin := new(MockLLMPlugin)
			name := "new-" + string(rune('A'+idx))
			plugin.On("Name").Return(name)
			_ = registry.Register(plugin)
		}(i)
		go func() {
			defer wg.Done()
			_, _ = registry.Get("pre-a")
			_ = registry.List()
		}()
	}
	wg.Wait()
}

func TestRegistry_ConcurrentUnregisterAndList(t *testing.T) {
	registry := NewRegistry()

	for i := 0; i < 10; i++ {
		plugin := new(MockLLMPlugin)
		name := "unreg-conc-" + string(rune('a'+i))
		plugin.On("Name").Return(name)
		_ = registry.Register(plugin)
	}

	var wg sync.WaitGroup
	// Concurrently unregister some and list
	for i := 0; i < 5; i++ {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			name := "unreg-conc-" + string(rune('a'+idx))
			_ = registry.Unregister(name)
		}(i)
		go func() {
			defer wg.Done()
			_ = registry.List()
		}()
	}
	wg.Wait()

	// Remaining should be 5 (a-e removed, f-j remain)
	list := registry.List()
	assert.Len(t, list, 5)
}

// --- Benchmarks ---

func BenchmarkRegistry_Register(b *testing.B) {
	registry := NewRegistry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Each iteration needs a unique name
		plugin := new(MockLLMPlugin)
		name := "bench-" + string(rune('A'+i%26)) + string(rune('a'+i/26%26)) + string(rune('0'+i/676%10))
		plugin.On("Name").Return(name)
		b.StartTimer()

		_ = registry.Register(plugin)
	}
}

func BenchmarkRegistry_Get(b *testing.B) {
	registry := NewRegistry()

	// Pre-register plugins
	for i := 0; i < 100; i++ {
		plugin := new(MockLLMPlugin)
		name := "bench-get-" + string(rune('A'+i%26)) + string(rune('a'+i/26))
		plugin.On("Name").Return(name)
		_ = registry.Register(plugin)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		name := "bench-get-" + string(rune('A'+i%26)) + string(rune('a'+(i/26)%4))
		_, _ = registry.Get(name)
	}
}

func BenchmarkRegistry_List(b *testing.B) {
	registry := NewRegistry()

	// Pre-register plugins
	for i := 0; i < 50; i++ {
		plugin := new(MockLLMPlugin)
		name := "bench-list-" + string(rune('A'+i%26)) + string(rune('a'+i/26))
		plugin.On("Name").Return(name)
		_ = registry.Register(plugin)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.List()
	}
}
