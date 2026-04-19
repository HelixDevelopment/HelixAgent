package formatters

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
)

// LazyFormatterFunc is a function that creates a Formatter on demand.
// It is called at most once per formatter, on first access.
type LazyFormatterFunc func() (Formatter, error)

// lazyFormatter holds a factory for deferred formatter initialization.
type lazyFormatter struct {
	factory LazyFormatterFunc
	once    sync.Once
	result  Formatter
	initErr error
}

// get returns the initialized formatter, calling the factory on first access.
func (lf *lazyFormatter) get() (Formatter, error) {
	lf.once.Do(func() {
		lf.result, lf.initErr = lf.factory()
	})
	return lf.result, lf.initErr
}

// FormatterRegistry manages all available formatters.
//
// Concurrent-safe by construction (CONST-029):
//   - Every collection is a safe.Store; callers cannot forget a lock
//     because there is no exposed lock.
//   - regMu is a write-only serialiser for Register / RegisterLazy /
//     Unregister / Stop (Pattern Zeta). It protects the invariant that
//     a formatter added to `formatters` also lands in `metadata` and
//     the `byLanguage` index atomically, and that Unregister tears
//     those three rows down together. Readers never take regMu.
type FormatterRegistry struct {
	regMu          sync.Mutex
	formatters     *safe.Store[string, Formatter]          // name -> formatter (eager)
	lazyFormatters *safe.Store[string, *lazyFormatter]     // name -> lazy formatter
	byLanguage     *safe.Store[string, []Formatter]        // language -> formatters (eager only)
	lazyByLanguage *safe.Store[string, []string]           // language -> lazy formatter names
	metadata       *safe.Store[string, *FormatterMetadata] // name -> metadata
	config         *RegistryConfig
	logger         *logrus.Logger
}

// RegistryConfig configures the formatter registry
type RegistryConfig struct {
	// Paths
	SubmodulesPath string // Path to formatters/ directory
	BinariesPath   string // Path to compiled binaries
	ConfigsPath    string // Path to config files

	// Services
	ServicesComposeFile string // docker-compose.formatters.yml path
	ServicesEnabled     bool   // Enable service-based formatters

	// Behavior
	EnableCaching  bool
	CacheTTL       time.Duration
	DefaultTimeout time.Duration
	MaxConcurrent  int

	// Features
	EnableHotReload bool
	EnableMetrics   bool
	EnableTracing   bool
}

// NewFormatterRegistry creates a new formatter registry
func NewFormatterRegistry(config *RegistryConfig, logger *logrus.Logger) *FormatterRegistry {
	return &FormatterRegistry{
		formatters:     safe.NewStore[string, Formatter](),
		lazyFormatters: safe.NewStore[string, *lazyFormatter](),
		byLanguage:     safe.NewStore[string, []Formatter](),
		lazyByLanguage: safe.NewStore[string, []string](),
		metadata:       safe.NewStore[string, *FormatterMetadata](),
		config:         config,
		logger:         logger,
	}
}

// Register registers a formatter with metadata
func (r *FormatterRegistry) Register(formatter Formatter, metadata *FormatterMetadata) error {
	r.regMu.Lock()
	defer r.regMu.Unlock()

	name := formatter.Name()

	// Check for duplicate
	if _, exists := r.formatters.Get(name); exists {
		return fmt.Errorf("formatter %s already registered", name)
	}

	// Register formatter
	r.formatters.Put(name, formatter)
	r.metadata.Put(name, metadata)

	// Register by language
	for _, lang := range formatter.Languages() {
		langLower := strings.ToLower(lang)
		r.byLanguage.Update(langLower, func(cur []Formatter, _ bool) ([]Formatter, bool) {
			return append(cur, formatter), true
		})
	}

	r.logger.Infof("Registered formatter: %s (v%s) for languages: %v", name, formatter.Version(), formatter.Languages())

	return nil
}

// RegisterLazy registers a formatter factory for deferred initialization.
// The factory is called at most once, when the formatter is first accessed
// via Get or GetByLanguage. The metadata must include Languages so that
// language-based lookups can discover the lazy formatter.
func (r *FormatterRegistry) RegisterLazy(factory LazyFormatterFunc, metadata *FormatterMetadata) error {
	r.regMu.Lock()
	defer r.regMu.Unlock()

	name := metadata.Name

	// Check for duplicate in both eager and lazy registrations
	if _, exists := r.formatters.Get(name); exists {
		return fmt.Errorf("formatter %s already registered", name)
	}
	if _, exists := r.lazyFormatters.Get(name); exists {
		return fmt.Errorf("formatter %s already registered (lazy)", name)
	}

	r.lazyFormatters.Put(name, &lazyFormatter{factory: factory})
	r.metadata.Put(name, metadata)

	// Register by language for lazy lookup
	for _, lang := range metadata.Languages {
		langLower := strings.ToLower(lang)
		r.lazyByLanguage.Update(langLower, func(cur []string, _ bool) ([]string, bool) {
			return append(cur, name), true
		})
	}

	r.logger.Infof("Registered lazy formatter: %s (v%s) for languages: %v",
		name, metadata.Version, metadata.Languages)

	return nil
}

// Unregister removes a formatter from the registry
func (r *FormatterRegistry) Unregister(name string) error {
	r.regMu.Lock()
	defer r.regMu.Unlock()

	formatter, eagerExists := r.formatters.Get(name)
	_, lazyExists := r.lazyFormatters.Get(name)

	if !eagerExists && !lazyExists {
		return fmt.Errorf("formatter %s not found", name)
	}

	// Remove eager language mappings
	if eagerExists {
		for _, lang := range formatter.Languages() {
			langLower := strings.ToLower(lang)
			r.byLanguage.Update(langLower, func(cur []Formatter, _ bool) ([]Formatter, bool) {
				for i, f := range cur {
					if f.Name() == name {
						return append(cur[:i], cur[i+1:]...), true
					}
				}
				return cur, true
			})
		}
	}

	// Remove lazy language mappings
	if lazyExists {
		if meta, ok := r.metadata.Get(name); ok && meta != nil {
			for _, lang := range meta.Languages {
				langLower := strings.ToLower(lang)
				r.lazyByLanguage.Update(langLower, func(cur []string, _ bool) ([]string, bool) {
					for i, n := range cur {
						if n == name {
							return append(cur[:i], cur[i+1:]...), true
						}
					}
					return cur, true
				})
			}
		}
	}

	// Remove from all registries
	r.formatters.Delete(name)
	r.lazyFormatters.Delete(name)
	r.metadata.Delete(name)

	r.logger.Infof("Unregistered formatter: %s", name)

	return nil
}

// Get retrieves a formatter by name. For lazily registered formatters,
// this triggers initialization on the first call.
func (r *FormatterRegistry) Get(name string) (Formatter, error) {
	// Check eagerly registered formatters first
	if formatter, exists := r.formatters.Get(name); exists {
		return formatter, nil
	}
	// Check lazily registered formatters
	lf, lazyExists := r.lazyFormatters.Get(name)
	if !lazyExists {
		return nil, fmt.Errorf("formatter %s not found", name)
	}

	formatter, err := lf.get()
	if err != nil {
		return nil, fmt.Errorf("lazy initialization of formatter %s failed: %w", name, err)
	}
	return formatter, nil
}

// GetByLanguage retrieves all formatters for a language.
// Lazy formatters are initialized on access.
func (r *FormatterRegistry) GetByLanguage(language string) []Formatter {
	langLower := strings.ToLower(language)
	eagerFormatters, _ := r.byLanguage.Get(langLower)
	lazyNames, _ := r.lazyByLanguage.Get(langLower)

	// Start with eager formatters
	result := make([]Formatter, 0, len(eagerFormatters)+len(lazyNames))
	result = append(result, eagerFormatters...)

	// Initialize and append lazy formatters
	for _, name := range lazyNames {
		lf, ok := r.lazyFormatters.Get(name)
		if !ok {
			continue
		}
		formatter, err := lf.get()
		if err != nil {
			r.logger.Warnf("Lazy initialization of formatter %s failed: %v", name, err)
			continue
		}
		result = append(result, formatter)
	}

	return result
}

// GetMetadata retrieves formatter metadata
func (r *FormatterRegistry) GetMetadata(name string) (*FormatterMetadata, error) {
	metadata, exists := r.metadata.Get(name)
	if !exists {
		return nil, fmt.Errorf("formatter %s not found", name)
	}
	return metadata, nil
}

// List returns all registered formatter names (both eager and lazy)
func (r *FormatterRegistry) List() []string {
	eagerKeys := r.formatters.Keys()
	lazyKeys := r.lazyFormatters.Keys()

	seen := make(map[string]struct{}, len(eagerKeys)+len(lazyKeys))
	names := make([]string, 0, len(eagerKeys)+len(lazyKeys))
	for _, name := range eagerKeys {
		names = append(names, name)
		seen[name] = struct{}{}
	}
	for _, name := range lazyKeys {
		if _, ok := seen[name]; !ok {
			names = append(names, name)
		}
	}
	return names
}

// ListByType returns all formatters of a specific type
func (r *FormatterRegistry) ListByType(ftype FormatterType) []string {
	names := make([]string, 0)
	r.metadata.Range(func(name string, metadata *FormatterMetadata) bool {
		if metadata.Type == ftype {
			names = append(names, name)
		}
		return true
	})
	return names
}

// DetectFormatter detects the appropriate formatter for a file
func (r *FormatterRegistry) DetectFormatter(filePath string, content string) (Formatter, error) {
	// Detect language from file extension
	language := r.DetectLanguageFromPath(filePath)
	if language == "" {
		return nil, fmt.Errorf("unable to detect language from file path: %s", filePath)
	}

	// Get formatters for language
	formatters := r.GetByLanguage(language)
	if len(formatters) == 0 {
		return nil, fmt.Errorf("no formatters available for language: %s", language)
	}

	// Return the first (highest priority) formatter
	return formatters[0], nil
}

// DetectLanguage detects the language from file path and content
func (r *FormatterRegistry) DetectLanguage(filePath string, content string) (string, error) {
	language := r.DetectLanguageFromPath(filePath)
	if language == "" {
		return "", fmt.Errorf("unable to detect language from file path: %s", filePath)
	}

	return language, nil
}

// DetectLanguageFromPath detects language from file extension
func (r *FormatterRegistry) DetectLanguageFromPath(filePath string) string {
	ext := filepath.Ext(filePath)
	if ext == "" {
		return ""
	}

	// Remove leading dot
	ext = strings.TrimPrefix(ext, ".")
	ext = strings.ToLower(ext)

	// Map extensions to languages
	extensionMap := map[string]string{
		"c":          "c",
		"h":          "c",
		"cc":         "cpp",
		"cpp":        "cpp",
		"cxx":        "cpp",
		"hpp":        "cpp",
		"hxx":        "cpp",
		"rs":         "rust",
		"go":         "go",
		"py":         "python",
		"pyw":        "python",
		"js":         "javascript",
		"jsx":        "javascript",
		"ts":         "typescript",
		"tsx":        "typescript",
		"java":       "java",
		"kt":         "kotlin",
		"kts":        "kotlin",
		"scala":      "scala",
		"sc":         "scala",
		"groovy":     "groovy",
		"gvy":        "groovy",
		"gy":         "groovy",
		"gsh":        "groovy",
		"clj":        "clojure",
		"cljs":       "clojure",
		"cljc":       "clojure",
		"rb":         "ruby",
		"php":        "php",
		"swift":      "swift",
		"dart":       "dart",
		"m":          "objectivec",
		"mm":         "objectivec",
		"sh":         "bash",
		"bash":       "bash",
		"ps1":        "powershell",
		"psm1":       "powershell",
		"lua":        "lua",
		"pl":         "perl",
		"pm":         "perl",
		"r":          "r",
		"sql":        "sql",
		"yaml":       "yaml",
		"yml":        "yaml",
		"json":       "json",
		"toml":       "toml",
		"xml":        "xml",
		"html":       "html",
		"htm":        "html",
		"css":        "css",
		"scss":       "scss",
		"sass":       "sass",
		"less":       "less",
		"md":         "markdown",
		"markdown":   "markdown",
		"graphql":    "graphql",
		"gql":        "graphql",
		"proto":      "protobuf",
		"tf":         "terraform",
		"tfvars":     "terraform",
		"dockerfile": "dockerfile",
		"hs":         "haskell",
		"ml":         "ocaml",
		"mli":        "ocaml",
		"fs":         "fsharp",
		"fsx":        "fsharp",
		"ex":         "elixir",
		"exs":        "elixir",
		"erl":        "erlang",
		"hrl":        "erlang",
		"zig":        "zig",
		"nim":        "nim",
	}

	return extensionMap[ext]
}

// maxConcurrentHealthChecks limits the number of parallel health checks
const maxConcurrentHealthChecks = 10

// HealthCheckAll performs health checks on all formatters with bounded concurrency.
// Iterates over a Snapshot so writers (Register/Unregister) are not blocked by
// potentially-slow HealthCheck calls.
func (r *FormatterRegistry) HealthCheckAll(ctx context.Context) map[string]error {
	snapshot := r.formatters.Snapshot()

	results := make(map[string]error)
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, maxConcurrentHealthChecks)

	for name, formatter := range snapshot {
		wg.Add(1)
		go func(name string, formatter Formatter) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			err := formatter.HealthCheck(ctx)

			mu.Lock()
			results[name] = err
			mu.Unlock()

			if err != nil {
				r.logger.Warnf("Health check failed for formatter %s: %v", name, err)
			} else {
				r.logger.Debugf("Health check passed for formatter %s", name)
			}
		}(name, formatter)
	}

	wg.Wait()

	return results
}

// Start initializes the registry
func (r *FormatterRegistry) Start(ctx context.Context) error {
	r.logger.Info("Starting formatter registry")

	// Perform health checks
	results := r.HealthCheckAll(ctx)

	// Log summary
	healthy := 0
	unhealthy := 0
	for _, err := range results {
		if err == nil {
			healthy++
		} else {
			unhealthy++
		}
	}

	r.logger.Infof("Formatter registry started: %d healthy, %d unhealthy", healthy, unhealthy)

	return nil
}

// Stop shuts down the registry.
//
// Preserves the original (admittedly asymmetric) behaviour of clearing
// only the eager collections and metadata; lazy registrations survive a
// Stop. Changing that semantic is out of scope for this migration.
func (r *FormatterRegistry) Stop(ctx context.Context) error {
	r.logger.Info("Stopping formatter registry")

	r.regMu.Lock()
	defer r.regMu.Unlock()

	r.formatters.Clear()
	r.byLanguage.Clear()
	r.metadata.Clear()

	r.logger.Info("Formatter registry stopped")

	return nil
}

// Count returns the number of registered formatters (both eager and lazy)
func (r *FormatterRegistry) Count() int {
	eagerKeys := r.formatters.Keys()
	lazyKeys := r.lazyFormatters.Keys()

	seen := make(map[string]struct{}, len(eagerKeys)+len(lazyKeys))
	for _, name := range eagerKeys {
		seen[name] = struct{}{}
	}
	for _, name := range lazyKeys {
		seen[name] = struct{}{}
	}
	return len(seen)
}

// CountByLanguage returns the number of formatters for a language
func (r *FormatterRegistry) CountByLanguage(language string) int {
	langLower := strings.ToLower(language)
	formatters, _ := r.byLanguage.Get(langLower)
	return len(formatters)
}

// GetPreferredFormatter returns the preferred formatter for a language
func (r *FormatterRegistry) GetPreferredFormatter(language string, preferences map[string]string) (Formatter, error) {
	// Check if there's a preference
	if preferences != nil {
		if preferred, ok := preferences[strings.ToLower(language)]; ok {
			return r.Get(preferred)
		}
	}

	// Fall back to first formatter for language
	formatters := r.GetByLanguage(language)
	if len(formatters) == 0 {
		return nil, fmt.Errorf("no formatters available for language: %s", language)
	}

	return formatters[0], nil
}
