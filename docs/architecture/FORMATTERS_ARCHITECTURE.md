# Code Formatters Integration Architecture

**Version**: 1.0
**Date**: 2026-01-29
**Status**: Design Phase

---

## Overview

This document defines the architecture for integrating 118+ open-source code formatters into HelixAgent, making them available to all 48+ CLI agents, all LLM providers, and the AI Debate system.

**Key Requirements**:
- Support ALL programming and scripting languages
- Integrate as Git submodules under `formatters/` directory
- Service-based formatters run in Docker/Podman containers
- 100% test coverage with comprehensive challenge scripts
- Integration with all 48 CLI agents and AI debate system
- No false positives in validation

---

## Architecture Principles

1. **Unified Interface**: All formatters expose a common `Formatter` interface
2. **Plugin Architecture**: Formatters are hot-reloadable plugins
3. **Performance First**: Native binaries preferred over interpreted runtimes
4. **Configuration Hierarchy**: System → User → Project → Request-level overrides
5. **Fail-Safe**: Formatting failures never block primary operations
6. **Observability**: All formatter invocations logged and traced
7. **Containerization**: Service-based formatters isolated in containers
8. **Version Management**: Each formatter pinned to specific versions via Git submodules

---

## Package Structure

```
helix_agent/
├── formatters/                           # Git submodules root
│   ├── clang-format/                     # Submodule: llvm/llvm-project
│   ├── rustfmt/                          # Submodule: rust-lang/rustfmt
│   ├── prettier/                         # Submodule: prettier/prettier
│   ├── black/                            # Submodule: psf/black
│   ├── ruff/                             # Submodule: astral-sh/ruff
│   ├── biome/                            # Submodule: biomejs/biome
│   ├── dprint/                           # Submodule: dprint/dprint
│   ├── google-java-format/               # Submodule: google/google-java-format
│   ├── ktlint/                           # Submodule: pinterest/ktlint
│   ├── scalafmt/                         # Submodule: scalameta/scalafmt
│   ├── swift-format/                     # Submodule: swiftlang/swift-format
│   ├── shfmt/                            # Submodule: mvdan/sh
│   ├── sqlfluff/                         # Submodule: sqlfluff/sqlfluff
│   ├── yamlfmt/                          # Submodule: google/yamlfmt
│   ├── taplo/                            # Submodule: tamasfe/taplo
│   ├── buf/                              # Submodule: bufbuild/buf
│   └── ... (118 total)
│
├── internal/
│   ├── formatters/                       # Formatter core
│   │   ├── registry.go                   # FormatterRegistry
│   │   ├── interface.go                  # Formatter interface
│   │   ├── executor.go                   # FormatterExecutor
│   │   ├── config.go                     # FormatterConfig
│   │   ├── cache.go                      # Result caching
│   │   ├── factory.go                    # Formatter factory
│   │   ├── health.go                     # Health checker
│   │   ├── versions.go                   # Version manager
│   │   ├── providers/                    # Formatter implementations
│   │   │   ├── native/                   # Native binary formatters
│   │   │   │   ├── clang_format.go
│   │   │   │   ├── rustfmt.go
│   │   │   │   ├── gofmt.go
│   │   │   │   ├── black.go
│   │   │   │   ├── ruff.go
│   │   │   │   ├── prettier.go
│   │   │   │   ├── biome.go
│   │   │   │   ├── dprint.go
│   │   │   │   ├── shfmt.go
│   │   │   │   ├── yamlfmt.go
│   │   │   │   ├── taplo.go
│   │   │   │   ├── buf.go
│   │   │   │   └── ... (60+ native)
│   │   │   ├── service/                  # Service-based formatters (RPC/HTTP)
│   │   │   │   ├── spotless.go           # Gradle/Maven service
│   │   │   │   ├── sqlfluff.go           # Python service
│   │   │   │   ├── rubocop.go            # Ruby service
│   │   │   │   ├── phpcs.go              # PHP service
│   │   │   │   ├── scalafmt.go           # Scala service
│   │   │   │   └── ... (20+ service)
│   │   │   ├── builtin/                  # Language built-in formatters
│   │   │   │   ├── gofmt.go              # Go built-in
│   │   │   │   ├── zig_fmt.go            # Zig built-in
│   │   │   │   ├── dart_format.go        # Dart built-in
│   │   │   │   ├── mix_format.go         # Elixir built-in
│   │   │   │   ├── terraform_fmt.go      # Terraform built-in
│   │   │   │   └── ... (15+ builtin)
│   │   │   └── unified/                  # Multi-language formatters
│   │   │       ├── prettier.go           # Prettier (10+ langs)
│   │   │       ├── dprint.go             # dprint (pluggable)
│   │   │       └── editorconfig.go       # EditorConfig
│   │   ├── middleware/                   # Formatter middleware
│   │   │   ├── validation.go             # Pre/post validation
│   │   │   ├── timeout.go                # Timeout handler
│   │   │   ├── retry.go                  # Retry logic
│   │   │   └── metrics.go                # Metrics collection
│   │   └── plugins/                      # Plugin system integration
│   │       ├── loader.go                 # Dynamic formatter loading
│   │       └── watcher.go                # Hot reload
│   │
│   ├── handlers/                         # HTTP handlers
│   │   └── formatters_handler.go         # Formatter API endpoints
│   │
│   └── services/                         # Services
│       ├── formatter_service.go          # Business logic
│       └── debate_formatter_integration.go  # AI Debate integration
│
├── docker/
│   └── formatters/                       # Formatter containers
│       ├── docker-compose.formatters.yml # All service-based formatters
│       ├── Dockerfile.spotless           # Java/Kotlin/Scala/Groovy
│       ├── Dockerfile.sqlfluff           # SQL multi-dialect
│       ├── Dockerfile.rubocop            # Ruby
│       ├── Dockerfile.phpcs              # PHP
│       ├── Dockerfile.scalafmt           # Scala (if standalone service)
│       ├── Dockerfile.npm-groovy-lint    # Groovy
│       ├── Dockerfile.styler             # R
│       ├── Dockerfile.psscriptanalyzer   # PowerShell
│       └── ... (20+ containers)
│
├── configs/
│   ├── formatters/                       # Formatter configurations
│   │   ├── default.yaml                  # System-wide defaults
│   │   ├── languages/                    # Per-language defaults
│   │   │   ├── c-cpp.yaml
│   │   │   ├── rust.yaml
│   │   │   ├── go.yaml
│   │   │   ├── python.yaml
│   │   │   ├── javascript.yaml
│   │   │   ├── java.yaml
│   │   │   └── ... (50+ language configs)
│   │   └── agents/                       # Per-agent overrides
│   │       ├── opencode.yaml
│   │       ├── crush.yaml
│   │       └── ... (48+ agent configs)
│   │
├── challenges/scripts/                   # Challenge scripts
│   ├── cli_agents_formatters_challenge.sh  # Final comprehensive challenge
│   ├── formatters_native_challenge.sh    # Native binary formatters (60 tests)
│   ├── formatters_service_challenge.sh   # Service-based formatters (20 tests)
│   ├── formatters_builtin_challenge.sh   # Built-in formatters (15 tests)
│   ├── formatters_unified_challenge.sh   # Unified formatters (10 tests)
│   ├── formatters_performance_challenge.sh  # Performance benchmarks (20 tests)
│   └── formatters_integration_challenge.sh  # Integration with AI Debate (30 tests)
│
└── tests/
    ├── formatters/                       # Unit tests
    │   ├── registry_test.go
    │   ├── executor_test.go
    │   ├── cache_test.go
    │   ├── providers/
    │   │   ├── native/
    │   │   │   ├── clang_format_test.go
    │   │   │   ├── rustfmt_test.go
    │   │   │   └── ... (60+ tests)
    │   │   ├── service/
    │   │   │   └── ... (20+ tests)
    │   │   └── builtin/
    │   │       └── ... (15+ tests)
    │   └── integration_test.go
    └── integration/
        └── cli_agents_formatters_test.go  # Go test for all CLI agents
```

---

## Core Interfaces

### Formatter Interface

```go
// internal/formatters/interface.go

package formatters

import (
	"context"
	"time"
)

// Formatter is the universal interface for all code formatters
type Formatter interface {
	// Identity
	Name() string                   // e.g., "clang-format"
	Version() string                // e.g., "19.1.8"
	Languages() []string            // e.g., ["c", "cpp", "java", "javascript"]

	// Capabilities
	SupportsStdin() bool            // Can accept input via stdin
	SupportsInPlace() bool          // Can format files in-place
	SupportsCheck() bool            // Can check without formatting (dry-run)
	SupportsConfig() bool           // Accepts configuration files

	// Formatting
	Format(ctx context.Context, req *FormatRequest) (*FormatResult, error)
	FormatBatch(ctx context.Context, reqs []*FormatRequest) ([]*FormatResult, error)

	// Health
	HealthCheck(ctx context.Context) error

	// Configuration
	ValidateConfig(config map[string]interface{}) error
	DefaultConfig() map[string]interface{}
}

// FormatRequest represents a formatting request
type FormatRequest struct {
	// Input
	Content     string            // Code content
	FilePath    string            // Optional file path (for extension detection)
	Language    string            // Language override

	// Configuration
	Config      map[string]interface{}  // Formatter-specific config
	LineLength  int               // Max line length (if supported)
	IndentSize  int               // Indent size
	UseTabs     bool              // Use tabs vs spaces

	// Behavior
	CheckOnly   bool              // Dry-run (check if formatted)
	Timeout     time.Duration     // Max execution time

	// Context
	AgentName   string            // CLI agent requesting format
	SessionID   string            // Session context
	RequestID   string            // Request tracking
}

// FormatResult represents the formatting result
type FormatResult struct {
	// Output
	Content      string           // Formatted content
	Changed      bool             // Whether content was modified

	// Metadata
	FormatterName    string       // Formatter used
	FormatterVersion string       // Formatter version
	Duration         time.Duration  // Execution time

	// Diagnostics
	Success      bool             // Overall success
	Error        error            // Error if failed
	Warnings     []string         // Non-fatal warnings
	Stats        *FormatStats     // Formatting statistics
}

// FormatStats provides formatting statistics
type FormatStats struct {
	LinesTotal    int
	LinesChanged  int
	BytesTotal    int
	BytesChanged  int
	Violations    int  // Style violations fixed
}

// FormatterType defines the formatter architecture type
type FormatterType string

const (
	FormatterTypeNative  FormatterType = "native"   // Standalone binary
	FormatterTypeService FormatterType = "service"  // RPC/HTTP service
	FormatterTypeBuiltin FormatterType = "builtin"  // Language built-in
	FormatterTypeUnified FormatterType = "unified"  // Multi-language
)

// FormatterMetadata provides formatter metadata
type FormatterMetadata struct {
	Name            string
	Type            FormatterType
	Architecture    string          // "binary", "python", "node", "jvm", etc.
	GitHubURL       string
	Version         string
	Languages       []string
	License         string

	// Installation
	InstallMethod   string          // "binary", "apt", "brew", "npm", "pip", "gem", etc.
	BinaryPath      string          // Path to binary
	ServiceURL      string          // Service endpoint (if service-based)

	// Configuration
	ConfigFormat    string          // "yaml", "json", "toml", "ini", "none"
	DefaultConfig   string          // Path to default config file

	// Performance
	Performance     string          // "very_fast", "fast", "medium", "slow"
	Complexity      string          // "easy", "medium", "hard"

	// Integration
	SupportsStdin   bool
	SupportsInPlace bool
	SupportsCheck   bool
	SupportsConfig  bool
}
```

### FormatterRegistry

```go
// internal/formatters/registry.go

package formatters

import (
	"context"
	"sync"
)

// FormatterRegistry manages all available formatters
type FormatterRegistry struct {
	mu          sync.RWMutex
	formatters  map[string]Formatter          // name -> formatter
	byLanguage  map[string][]Formatter        // language -> formatters
	metadata    map[string]*FormatterMetadata // name -> metadata
	config      *RegistryConfig
	logger      *logrus.Logger
}

// RegistryConfig configures the formatter registry
type RegistryConfig struct {
	// Paths
	SubmodulesPath    string  // Path to formatters/ directory
	BinariesPath      string  // Path to compiled binaries
	ConfigsPath       string  // Path to config files

	// Services
	ServicesComposeFile string  // docker-compose.formatters.yml path
	ServicesEnabled     bool    // Enable service-based formatters

	// Behavior
	EnableCaching       bool
	CacheTTL            time.Duration
	DefaultTimeout      time.Duration
	MaxConcurrent       int

	// Features
	EnableHotReload     bool
	EnableMetrics       bool
	EnableTracing       bool
}

func NewFormatterRegistry(config *RegistryConfig, logger *logrus.Logger) *FormatterRegistry

// Registration
func (r *FormatterRegistry) Register(formatter Formatter, metadata *FormatterMetadata) error
func (r *FormatterRegistry) Unregister(name string) error

// Discovery
func (r *FormatterRegistry) Get(name string) (Formatter, error)
func (r *FormatterRegistry) GetByLanguage(language string) []Formatter
func (r *FormatterRegistry) GetMetadata(name string) (*FormatterMetadata, error)
func (r *FormatterRegistry) List() []string
func (r *FormatterRegistry) ListByType(ftype FormatterType) []string

// Detection
func (r *FormatterRegistry) DetectFormatter(filePath string, content string) (Formatter, error)
func (r *FormatterRegistry) DetectLanguage(filePath string, content string) (string, error)

// Health
func (r *FormatterRegistry) HealthCheckAll(ctx context.Context) map[string]error

// Lifecycle
func (r *FormatterRegistry) Start(ctx context.Context) error
func (r *FormatterRegistry) Stop(ctx context.Context) error
```

### FormatterExecutor

```go
// internal/formatters/executor.go

package formatters

import (
	"context"
)

// FormatterExecutor executes formatting requests with middleware
type FormatterExecutor struct {
	registry   *FormatterRegistry
	cache      *FormatterCache
	middleware []Middleware
	logger     *logrus.Logger
}

// Middleware wraps formatter execution
type Middleware func(next ExecuteFunc) ExecuteFunc

type ExecuteFunc func(ctx context.Context, formatter Formatter, req *FormatRequest) (*FormatResult, error)

func NewFormatterExecutor(registry *FormatterRegistry, logger *logrus.Logger) *FormatterExecutor

// Execution
func (e *FormatterExecutor) Execute(ctx context.Context, req *FormatRequest) (*FormatResult, error)
func (e *FormatterExecutor) ExecuteBatch(ctx context.Context, reqs []*FormatRequest) ([]*FormatResult, error)

// Middleware
func (e *FormatterExecutor) Use(middleware ...Middleware)

// Built-in middleware
func TimeoutMiddleware(defaultTimeout time.Duration) Middleware
func RetryMiddleware(maxRetries int) Middleware
func CacheMiddleware(cache *FormatterCache) Middleware
func ValidationMiddleware() Middleware
func MetricsMiddleware() Middleware
func TracingMiddleware() Middleware
```

---

## Git Submodules Organization

### Submodule Structure

```bash
# Root formatters/ directory with 118 submodules
formatters/
├── .gitmodules                    # Git submodules configuration
├── README.md                      # Formatters directory overview
├── VERSIONS.md                    # Pinned versions for all formatters
├── scripts/
│   ├── init-all.sh                # Initialize all submodules
│   ├── update-all.sh              # Update all submodules
│   ├── build-all.sh               # Build all native binaries
│   └── health-check-all.sh        # Health check all formatters
│
└── <118 formatter submodules>
```

### Submodule Initialization Commands

```bash
# Initialize formatters directory
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent
mkdir -p formatters

# Add submodules (example commands)
git submodule add https://github.com/llvm/llvm-project formatters/clang-format
git submodule add https://github.com/rust-lang/rustfmt formatters/rustfmt
git submodule add https://github.com/prettier/prettier formatters/prettier
git submodule add https://github.com/psf/black formatters/black
git submodule add https://github.com/astral-sh/ruff formatters/ruff
git submodule add https://github.com/biomejs/biome formatters/biome
git submodule add https://github.com/dprint/dprint formatters/dprint
git submodule add https://github.com/google/google-java-format formatters/google-java-format
git submodule add https://github.com/pinterest/ktlint formatters/ktlint
git submodule add https://github.com/scalameta/scalafmt formatters/scalafmt
git submodule add https://github.com/mvdan/sh formatters/shfmt
git submodule add https://github.com/sqlfluff/sqlfluff formatters/sqlfluff
git submodule add https://github.com/google/yamlfmt formatters/yamlfmt
git submodule add https://github.com/tamasfe/taplo formatters/taplo
git submodule add https://github.com/bufbuild/buf formatters/buf
# ... (118 total submodules)

# Initialize and update all submodules
git submodule update --init --recursive

# Pin to specific versions
cd formatters/clang-format && git checkout release/19.x && cd ../..
cd formatters/rustfmt && git checkout v1.8.1 && cd ../..
# ... (pin all submodules to stable versions)

# Commit submodule configuration
git add .gitmodules formatters/
git commit -m "feat: Add 118 formatter Git submodules"
```

### Version Management

```yaml
# formatters/VERSIONS.md

# Formatter Versions
# This file documents the pinned versions for all formatter submodules
# Last Updated: 2026-01-29

native:
  clang-format:
    version: "19.1.8"
    commit: "abc123..."
    git_ref: "release/19.x"

  rustfmt:
    version: "1.8.1"
    commit: "def456..."
    git_ref: "v1.8.1"

  black:
    version: "26.1a1"
    commit: "ghi789..."
    git_ref: "26.1a1"

  ruff:
    version: "0.9.6"
    commit: "jkl012..."
    git_ref: "v0.9.6"

  prettier:
    version: "3.4.2"
    commit: "mno345..."
    git_ref: "3.4.2"

  # ... (118 total entries)

service:
  sqlfluff:
    version: "3.4.1"
    commit: "pqr678..."
    git_ref: "3.4.1"

  rubocop:
    version: "1.72.0"
    commit: "stu901..."
    git_ref: "v1.72.0"

  # ... (20+ service formatters)

builtin:
  gofmt:
    version: "go1.24.11"
    note: "Built-in to Go toolchain"

  zig_fmt:
    version: "0.14.0"
    note: "Built-in to Zig compiler"

  # ... (15 built-in formatters)
```

---

## Docker/Podman Containerization

### Service-Based Formatters

Service-based formatters (Python, Ruby, JVM, Node.js) run in isolated containers for consistent environments.

```yaml
# docker/formatters/docker-compose.formatters.yml

services:
  # SQLFluff (Python) - SQL multi-dialect formatter
  sqlfluff:
    build:
      context: ../../formatters/sqlfluff
      dockerfile: ../../docker/formatters/Dockerfile.sqlfluff
    container_name: helixagent-formatter-sqlfluff
    ports:
      - "9201:8080"
    environment:
      - SQLFLUFF_DIALECT=postgres
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped
    networks:
      - formatters-net

  # RuboCop (Ruby) - Ruby formatter + linter
  rubocop:
    build:
      context: ../../formatters/rubocop
      dockerfile: ../../docker/formatters/Dockerfile.rubocop
    container_name: helixagent-formatter-rubocop
    ports:
      - "9202:8080"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped
    networks:
      - formatters-net

  # Spotless (JVM) - Multi-language formatter for Java/Kotlin/Scala/Groovy
  spotless:
    build:
      context: ../../formatters/spotless
      dockerfile: ../../docker/formatters/Dockerfile.spotless
    container_name: helixagent-formatter-spotless
    ports:
      - "9203:8080"
    environment:
      - JAVA_OPTS=-Xmx2g
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped
    networks:
      - formatters-net

  # PHP-CS-Fixer (PHP) - PHP formatter
  php-cs-fixer:
    build:
      context: ../../formatters/php-cs-fixer
      dockerfile: ../../docker/formatters/Dockerfile.php-cs-fixer
    container_name: helixagent-formatter-php-cs-fixer
    ports:
      - "9204:8080"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped
    networks:
      - formatters-net

  # npm-groovy-lint (Groovy) - Groovy/Jenkinsfile formatter
  npm-groovy-lint:
    build:
      context: ../../formatters/npm-groovy-lint
      dockerfile: ../../docker/formatters/Dockerfile.npm-groovy-lint
    container_name: helixagent-formatter-npm-groovy-lint
    ports:
      - "9205:8080"
    environment:
      - NODE_ENV=production
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped
    networks:
      - formatters-net

  # Scalafmt (Scala) - Scala formatter (if standalone service mode)
  scalafmt:
    build:
      context: ../../formatters/scalafmt
      dockerfile: ../../docker/formatters/Dockerfile.scalafmt
    container_name: helixagent-formatter-scalafmt
    ports:
      - "9206:8080"
    environment:
      - JAVA_OPTS=-Xmx1g
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped
    networks:
      - formatters-net

  # Styler (R) - R formatter
  styler:
    build:
      context: ../../formatters/styler
      dockerfile: ../../docker/formatters/Dockerfile.styler
    container_name: helixagent-formatter-styler
    ports:
      - "9207:8080"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped
    networks:
      - formatters-net

  # PSScriptAnalyzer (PowerShell) - PowerShell formatter
  psscriptanalyzer:
    build:
      context: ../../formatters/psscriptanalyzer
      dockerfile: ../../docker/formatters/Dockerfile.psscriptanalyzer
    container_name: helixagent-formatter-psscriptanalyzer
    ports:
      - "9208:8080"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped
    networks:
      - formatters-net

  # StandardRB (Ruby) - Opinionated Ruby formatter
  standardrb:
    build:
      context: ../../formatters/standardrb
      dockerfile: ../../docker/formatters/Dockerfile.standardrb
    container_name: helixagent-formatter-standardrb
    ports:
      - "9209:8080"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped
    networks:
      - formatters-net

  # YAPF (Python) - Google's Python formatter
  yapf:
    build:
      context: ../../formatters/yapf
      dockerfile: ../../docker/formatters/Dockerfile.yapf
    container_name: helixagent-formatter-yapf
    ports:
      - "9210:8080"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped
    networks:
      - formatters-net

  # autopep8 (Python) - PEP 8 formatter
  autopep8:
    build:
      context: ../../formatters/autopep8
      dockerfile: ../../docker/formatters/Dockerfile.autopep8
    container_name: helixagent-formatter-autopep8
    ports:
      - "9211:8080"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped
    networks:
      - formatters-net

  # Perltidy (Perl) - Perl formatter
  perltidy:
    build:
      context: ../../formatters/perltidy
      dockerfile: ../../docker/formatters/Dockerfile.perltidy
    container_name: helixagent-formatter-perltidy
    ports:
      - "9212:8080"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped
    networks:
      - formatters-net

  # Cljfmt (Clojure) - Clojure formatter
  cljfmt:
    build:
      context: ../../formatters/cljfmt
      dockerfile: ../../docker/formatters/Dockerfile.cljfmt
    container_name: helixagent-formatter-cljfmt
    ports:
      - "9213:8080"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped
    networks:
      - formatters-net

  # Fantomas (F#) - F# formatter
  fantomas:
    build:
      context: ../../formatters/fantomas
      dockerfile: ../../docker/formatters/Dockerfile.fantomas
    container_name: helixagent-formatter-fantomas
    ports:
      - "9214:8080"
    environment:
      - DOTNET_RUNNING_IN_CONTAINER=true
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped
    networks:
      - formatters-net

  # OCamlformat (OCaml) - OCaml formatter (RPC server mode)
  ocamlformat:
    build:
      context: ../../formatters/ocamlformat
      dockerfile: ../../docker/formatters/Dockerfile.ocamlformat
    container_name: helixagent-formatter-ocamlformat
    ports:
      - "9215:8080"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped
    networks:
      - formatters-net

networks:
  formatters-net:
    driver: bridge
    name: helixagent-formatters
```

### Example Dockerfile (SQLFluff)

```dockerfile
# docker/formatters/Dockerfile.sqlfluff

FROM python:3.12-slim

LABEL maintainer="HelixAgent Team"
LABEL formatter="sqlfluff"
LABEL version="3.4.1"

WORKDIR /app

# Install SQLFluff
RUN pip install --no-cache-dir sqlfluff==3.4.1

# Create non-root user
RUN useradd -m -u 1000 formatter && \
    chown -R formatter:formatter /app
USER formatter

# Copy service wrapper
COPY docker/formatters/services/sqlfluff_service.py /app/service.py

# Health check endpoint
HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
  CMD python -c "import requests; requests.get('http://localhost:8080/health')"

# Expose HTTP service
EXPOSE 8080

# Start service
CMD ["python", "service.py"]
```

### Service Wrapper (SQLFluff)

```python
# docker/formatters/services/sqlfluff_service.py

from flask import Flask, request, jsonify
import sqlfluff
import time

app = Flask(__name__)

@app.route('/health', methods=['GET'])
def health():
    return jsonify({"status": "healthy", "formatter": "sqlfluff", "version": sqlfluff.__version__})

@app.route('/format', methods=['POST'])
def format_code():
    start = time.time()
    data = request.json

    content = data.get('content', '')
    dialect = data.get('dialect', 'postgres')
    config = data.get('config', {})

    try:
        # Format using sqlfluff API
        result = sqlfluff.fix(
            content,
            dialect=dialect,
            **config
        )

        duration = time.time() - start

        return jsonify({
            "success": True,
            "content": result.tree.raw,
            "changed": result.tree.raw != content,
            "duration": duration,
            "formatter_name": "sqlfluff",
            "formatter_version": sqlfluff.__version__
        })

    except Exception as e:
        return jsonify({
            "success": False,
            "error": str(e)
        }), 400

@app.route('/check', methods=['POST'])
def check_code():
    data = request.json

    content = data.get('content', '')
    dialect = data.get('dialect', 'postgres')

    try:
        result = sqlfluff.lint(content, dialect=dialect)

        return jsonify({
            "success": True,
            "violations": len(result.violations),
            "formatted": result.num_violations == 0
        })

    except Exception as e:
        return jsonify({
            "success": False,
            "error": str(e)
        }), 400

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8080)
```

---

## API Endpoints

### REST API

```
POST   /v1/format              # Format code
POST   /v1/format/batch        # Format multiple files
POST   /v1/format/check        # Check if code is formatted (dry-run)
GET    /v1/formatters          # List all formatters
GET    /v1/formatters/:name    # Get formatter metadata
GET    /v1/formatters/:name/health  # Health check formatter
GET    /v1/formatters/detect   # Detect formatter for file
POST   /v1/formatters/:name/validate-config  # Validate config
```

### Request/Response Examples

#### Format Code

```bash
POST /v1/format
Content-Type: application/json

{
  "content": "def foo( x,y ):\n return x+y",
  "language": "python",
  "formatter": "black",  // Optional: auto-detect if omitted
  "config": {
    "line_length": 88,
    "skip_string_normalization": false
  },
  "agent_name": "opencode",
  "session_id": "abc123"
}
```

```json
HTTP/1.1 200 OK
Content-Type: application/json

{
  "success": true,
  "content": "def foo(x, y):\n    return x + y\n",
  "changed": true,
  "formatter_name": "black",
  "formatter_version": "26.1a1",
  "duration_ms": 45,
  "stats": {
    "lines_total": 2,
    "lines_changed": 2,
    "bytes_total": 31,
    "bytes_changed": 31,
    "violations": 5
  }
}
```

#### List Formatters

```bash
GET /v1/formatters?language=python
```

```json
HTTP/1.1 200 OK
Content-Type: application/json

{
  "count": 4,
  "formatters": [
    {
      "name": "black",
      "type": "native",
      "version": "26.1a1",
      "languages": ["python"],
      "performance": "medium",
      "supported": true,
      "installed": true
    },
    {
      "name": "ruff",
      "type": "native",
      "version": "0.9.6",
      "languages": ["python"],
      "performance": "very_fast",
      "supported": true,
      "installed": true
    },
    {
      "name": "autopep8",
      "type": "service",
      "version": "2.0.4",
      "languages": ["python"],
      "performance": "medium",
      "supported": true,
      "installed": true,
      "service_url": "http://localhost:9211"
    },
    {
      "name": "yapf",
      "type": "service",
      "version": "0.40.2",
      "languages": ["python"],
      "performance": "slow",
      "supported": true,
      "installed": true,
      "service_url": "http://localhost:9210"
    }
  ]
}
```

#### Detect Formatter

```bash
GET /v1/formatters/detect?file_path=src/main.rs
```

```json
HTTP/1.1 200 OK
Content-Type: application/json

{
  "language": "rust",
  "formatters": [
    {
      "name": "rustfmt",
      "type": "native",
      "priority": 1,
      "reason": "official Rust formatter"
    }
  ]
}
```

---

## CLI Agent Integration

### Configuration

Each of the 48 CLI agents receives formatter configuration via their respective config files (OpenCode TOML, Crush JSON, etc.).

```toml
# /tmp/opencode-config.toml (extended with formatters)

[formatters]
enabled = true
auto_format = true                     # Auto-format on save
format_on_debate = true                # Auto-format AI debate outputs
default_line_length = 88
default_indent_size = 4
use_tabs = false

[formatters.preferences]
python = "ruff"                        # Prefer Ruff for Python (fastest)
javascript = "biome"                   # Prefer Biome for JS/TS (faster than Prettier)
typescript = "biome"
rust = "rustfmt"
go = "gofmt"
c = "clang-format"
cpp = "clang-format"
java = "google-java-format"
kotlin = "ktlint"
scala = "scalafmt"
ruby = "rubocop"
php = "php-cs-fixer"
swift = "swift-format"
shell = "shfmt"
sql = "sqlfluff"
yaml = "yamlfmt"
json = "jq"
toml = "taplo"
markdown = "prettier"
html = "prettier"
css = "prettier"

[formatters.fallback]
python = ["black", "autopep8"]         # Fallback chain
javascript = ["prettier", "dprint"]
typescript = ["prettier", "dprint"]

[formatters.overrides]
# Project-specific overrides
"src/**/*.py" = { formatter = "black", line_length = 120 }
"tests/**/*.py" = { formatter = "black", line_length = 100 }
"*.js" = { formatter = "prettier", semi = false, single_quote = true }
```

### Integration Points

1. **On File Save**: CLI agents call `POST /v1/format` when user saves a file
2. **On Debate Output**: AI Debate system auto-formats generated code before returning to user
3. **Pre-Commit**: Git pre-commit hooks format staged files
4. **Batch Format**: Format entire project via `opencode format .` or `crush format .`

### CLI Agent Commands

```bash
# OpenCode
opencode format file.py                      # Format single file
opencode format src/                         # Format directory
opencode format --check .                    # Check formatting (dry-run)
opencode format --formatter black file.py    # Use specific formatter

# Crush
crush format file.rs
crush format --all                           # Format entire project
crush format --check                         # CI/CD check

# HelixCode, Aider, etc. (same pattern)
helixcode format ...
aider format ...
```

---

## AI Debate System Integration

### Automatic Formatting

The AI Debate system automatically formats generated code before returning responses to users.

```go
// internal/services/debate_formatter_integration.go

package services

import (
	"context"
	"strings"

	"dev.helix.agent/internal/formatters"
)

// DebateFormatterIntegration integrates formatters into AI Debate
type DebateFormatterIntegration struct {
	debateService      *DebateService
	formatterExecutor  *formatters.FormatterExecutor
	config             *DebateFormatterConfig
	logger             *logrus.Logger
}

// DebateFormatterConfig configures formatter integration
type DebateFormatterConfig struct {
	Enabled              bool
	AutoFormat           bool   // Auto-format all code blocks
	FormatLanguages      []string  // Languages to format (empty = all)
	IgnoreLanguages      []string  // Languages to skip
	MaxCodeBlockSize     int       // Max bytes to format
	Timeout              time.Duration
	ContinueOnError      bool   // Continue if formatting fails
}

// FormatDebateResponse formats code blocks in a debate response
func (d *DebateFormatterIntegration) FormatDebateResponse(
	ctx context.Context,
	response string,
	agentName string,
	sessionID string,
) (string, error) {
	if !d.config.Enabled || !d.config.AutoFormat {
		return response, nil
	}

	// Extract code blocks
	codeBlocks := d.extractCodeBlocks(response)
	if len(codeBlocks) == 0 {
		return response, nil
	}

	// Format each code block
	formatted := response
	for _, block := range codeBlocks {
		if d.shouldFormat(block) {
			formattedCode, err := d.formatCodeBlock(ctx, block, agentName, sessionID)
			if err != nil {
				if d.config.ContinueOnError {
					d.logger.Warnf("Failed to format code block: %v", err)
					continue
				}
				return "", err
			}

			// Replace original with formatted
			formatted = strings.Replace(formatted, block.Original, formattedCode, 1)
		}
	}

	return formatted, nil
}

// extractCodeBlocks extracts code blocks from markdown
func (d *DebateFormatterIntegration) extractCodeBlocks(content string) []CodeBlock {
	// Parse markdown code blocks (```language\ncode\n```)
	// ...
}

// formatCodeBlock formats a single code block
func (d *DebateFormatterIntegration) formatCodeBlock(
	ctx context.Context,
	block CodeBlock,
	agentName string,
	sessionID string,
) (string, error) {
	req := &formatters.FormatRequest{
		Content:   block.Code,
		Language:  block.Language,
		Timeout:   d.config.Timeout,
		AgentName: agentName,
		SessionID: sessionID,
	}

	result, err := d.formatterExecutor.Execute(ctx, req)
	if err != nil {
		return "", err
	}

	// Reconstruct code block with formatted code
	return fmt.Sprintf("```%s\n%s\n```", block.Language, result.Content), nil
}

// shouldFormat determines if a code block should be formatted
func (d *DebateFormatterIntegration) shouldFormat(block CodeBlock) bool {
	// Check size limits
	if len(block.Code) > d.config.MaxCodeBlockSize {
		return false
	}

	// Check language filters
	if len(d.config.FormatLanguages) > 0 {
		return contains(d.config.FormatLanguages, block.Language)
	}

	if len(d.config.IgnoreLanguages) > 0 {
		return !contains(d.config.IgnoreLanguages, block.Language)
	}

	return true
}

type CodeBlock struct {
	Original string  // Original markdown block
	Language string  // Language identifier
	Code     string  // Code content
}
```

### Debate Event Stream

When the AI Debate system formats code, events are streamed to the user:

```json
{
  "event": "debate.format.started",
  "data": {
    "code_blocks": 3,
    "languages": ["python", "javascript", "rust"]
  }
}

{
  "event": "debate.format.progress",
  "data": {
    "block": 1,
    "language": "python",
    "formatter": "ruff",
    "status": "formatting"
  }
}

{
  "event": "debate.format.completed",
  "data": {
    "block": 1,
    "language": "python",
    "formatter": "ruff",
    "duration_ms": 45,
    "changed": true
  }
}

// ... (for each code block)

{
  "event": "debate.format.finished",
  "data": {
    "total_blocks": 3,
    "formatted": 3,
    "failed": 0,
    "total_duration_ms": 132
  }
}
```

---

## Configuration Schema

### System-Wide Configuration

```yaml
# configs/formatters/default.yaml

formatters:
  # Global settings
  enabled: true
  auto_format: true
  format_on_save: true
  format_on_debate: true

  # Paths
  submodules_path: "./formatters"
  binaries_path: "./bin/formatters"
  configs_path: "./configs/formatters"

  # Services
  services_compose_file: "./docker/formatters/docker-compose.formatters.yml"
  services_enabled: true

  # Performance
  cache_enabled: true
  cache_ttl: 3600  # seconds
  default_timeout: 30  # seconds
  max_concurrent: 10

  # Features
  hot_reload: true
  metrics: true
  tracing: true

  # Defaults
  default_line_length: 88
  default_indent_size: 4
  use_tabs: false

  # Preferences (language -> formatter)
  preferences:
    python: "ruff"
    javascript: "biome"
    typescript: "biome"
    rust: "rustfmt"
    go: "gofmt"
    c: "clang-format"
    cpp: "clang-format"
    java: "google-java-format"
    kotlin: "ktlint"
    scala: "scalafmt"
    swift: "swift-format"
    dart: "dart_format"
    ruby: "rubocop"
    php: "php-cs-fixer"
    elixir: "mix_format"
    haskell: "ormolu"
    ocaml: "ocamlformat"
    fsharp: "fantomas"
    clojure: "cljfmt"
    erlang: "erlfmt"
    bash: "shfmt"
    powershell: "psscriptanalyzer"
    lua: "stylua"
    perl: "perltidy"
    r: "air"
    sql: "sqlfluff"
    yaml: "yamlfmt"
    json: "jq"
    toml: "taplo"
    xml: "xmllint"
    html: "prettier"
    css: "prettier"
    scss: "prettier"
    markdown: "prettier"
    graphql: "prettier"
    protobuf: "buf"
    terraform: "terraform_fmt"
    dockerfile: "hadolint"

  # Fallback chains
  fallback:
    python: ["black", "autopep8"]
    javascript: ["prettier", "dprint"]
    typescript: ["prettier", "dprint"]
    ruby: ["standardrb", "rufo"]
    java: ["spotless"]
    kotlin: ["ktfmt"]
    css: ["stylelint"]

  # Language-specific configs
  language_configs:
    python:
      line_length: 88
      skip_string_normalization: false
      skip_magic_trailing_comma: false

    javascript:
      semi: false
      single_quote: true
      trailing_comma: "es5"
      print_width: 100

    typescript:
      semi: false
      single_quote: true
      trailing_comma: "all"
      print_width: 100

    go:
      # gofmt is opinionated, no config

    rust:
      edition: "2024"
      max_width: 100

    c:
      style: "Google"
      indent_width: 2

    cpp:
      style: "Google"
      indent_width: 2
```

### Per-Language Configuration

```yaml
# configs/formatters/languages/python.yaml

language: python

formatters:
  - name: "ruff"
    priority: 1
    enabled: true
    config:
      line_length: 88
      target_version: "py312"
      select: ["ALL"]
      ignore: ["E501"]

  - name: "black"
    priority: 2
    enabled: true
    config:
      line_length: 88
      target_version: ["py312"]
      skip_string_normalization: false

  - name: "autopep8"
    priority: 3
    enabled: true
    config:
      max_line_length: 88
      aggressive: 2

  - name: "yapf"
    priority: 4
    enabled: false  # Slow, disabled by default
    config:
      style: "pep8"
      column_limit: 88

overrides:
  # Project-specific overrides
  - pattern: "tests/**/*.py"
    config:
      line_length: 100
      formatter: "black"

  - pattern: "scripts/**/*.py"
    config:
      line_length: 120
      formatter: "ruff"
```

### Per-Agent Configuration

```yaml
# configs/formatters/agents/opencode.yaml

agent: opencode

formatters:
  enabled: true
  auto_format: true
  format_on_save: true
  format_on_debate: true

  # Override preferences for this agent
  preferences:
    python: "black"  # OpenCode prefers Black over Ruff
    javascript: "prettier"  # OpenCode prefers Prettier over Biome
    typescript: "prettier"

  # Agent-specific overrides
  overrides:
    - pattern: "*.test.js"
      config:
        formatter: "prettier"
        semi: true

    - pattern: "*.spec.ts"
      config:
        formatter: "prettier"
        semi: true

  # Disable specific formatters for this agent
  disabled:
    - "yapf"
    - "autopep8"
```

---

## Testing Strategy

### Test Categories

1. **Unit Tests** (`tests/formatters/`) - 118+ tests
   - One test per formatter
   - Interface compliance
   - Configuration validation
   - Error handling

2. **Integration Tests** (`tests/integration/`) - 50+ tests
   - Registry operations
   - Executor middleware
   - Cache behavior
   - Service communication

3. **CLI Agent Tests** (`tests/integration/cli_agents_formatters_test.go`) - 48+ tests
   - One test per CLI agent
   - Verify formatter availability
   - Verify configuration

4. **AI Debate Tests** (`tests/integration/debate_formatter_test.go`) - 30+ tests
   - Auto-formatting of debate outputs
   - Code block detection
   - Multi-language formatting

5. **Challenge Scripts** (`challenges/scripts/`) - 200+ tests
   - Native formatter challenges (60 tests)
   - Service formatter challenges (20 tests)
   - Built-in formatter challenges (15 tests)
   - Unified formatter challenges (10 tests)
   - Performance benchmarks (20 tests)
   - Integration challenges (30 tests)
   - **Final challenge**: `cli_agents_formatters_challenge.sh` (100+ tests)

### Final Challenge: CliAgentsFormatters

```bash
# challenges/scripts/cli_agents_formatters_challenge.sh

#!/bin/bash
# CliAgentsFormatters Challenge
# Validates all formatters work in all CLI agents and AI Debate system

set -euo pipefail

PASSED=0
FAILED=0
TOTAL=0

# Test matrix: 118 formatters × 48 agents = 5,664 tests
# Plus: 118 formatters × AI Debate = 118 tests
# Total: 5,782 tests

echo "=== CliAgentsFormatters Challenge ==="
echo "Testing 118 formatters across 48 CLI agents + AI Debate system"
echo

# For each formatter
for FORMATTER in $(curl -s http://localhost:7061/v1/formatters | jq -r '.formatters[].name'); do
  FORMATTER_LANGS=$(curl -s "http://localhost:7061/v1/formatters/$FORMATTER" | jq -r '.languages[]')

  # Test in each CLI agent
  for AGENT in opencode crush helixcode aider claudecode cline goose ...; do
    for LANG in $FORMATTER_LANGS; do
      TOTAL=$((TOTAL+1))

      # Generate test code for language
      TEST_CODE=$(generate_test_code "$LANG")

      # Test 1: Formatter available in agent config
      if ! agent_has_formatter "$AGENT" "$FORMATTER"; then
        echo "❌ FAIL: $AGENT missing formatter $FORMATTER"
        FAILED=$((FAILED+1))
        continue
      fi

      # Test 2: Agent can invoke formatter
      RESULT=$(agent_format "$AGENT" "$FORMATTER" "$TEST_CODE" "$LANG")
      if [ $? -ne 0 ]; then
        echo "❌ FAIL: $AGENT cannot invoke $FORMATTER for $LANG"
        FAILED=$((FAILED+1))
        continue
      fi

      # Test 3: Formatting produces valid output
      if ! is_valid_code "$RESULT" "$LANG"; then
        echo "❌ FAIL: $FORMATTER produced invalid $LANG code in $AGENT"
        FAILED=$((FAILED+1))
        continue
      fi

      # Test 4: Formatting is idempotent
      RESULT2=$(agent_format "$AGENT" "$FORMATTER" "$RESULT" "$LANG")
      if [ "$RESULT" != "$RESULT2" ]; then
        echo "❌ FAIL: $FORMATTER not idempotent for $LANG in $AGENT"
        FAILED=$((FAILED+1))
        continue
      fi

      PASSED=$((PASSED+1))
      echo "✓ $AGENT: $FORMATTER ($LANG) OK"
    done
  done

  # Test in AI Debate system
  for LANG in $FORMATTER_LANGS; do
    TOTAL=$((TOTAL+1))

    TEST_CODE=$(generate_test_code "$LANG")

    # Send debate request with code
    DEBATE_RESPONSE=$(send_debate_request "$TEST_CODE" "$LANG")

    # Verify formatter was used
    if ! debate_used_formatter "$DEBATE_RESPONSE" "$FORMATTER"; then
      echo "❌ FAIL: AI Debate did not use $FORMATTER for $LANG"
      FAILED=$((FAILED+1))
      continue
    fi

    # Verify code is formatted
    DEBATE_CODE=$(extract_code_from_debate "$DEBATE_RESPONSE" "$LANG")
    if ! is_formatted "$DEBATE_CODE" "$FORMATTER" "$LANG"); then
      echo "❌ FAIL: AI Debate code not formatted by $FORMATTER ($LANG)"
      FAILED=$((FAILED+1))
      continue
    fi

    PASSED=$((PASSED+1))
    echo "✓ AI Debate: $FORMATTER ($LANG) OK"
  done
done

echo
echo "=== Results ==="
echo "Total: $TOTAL"
echo "Passed: $PASSED"
echo "Failed: $FAILED"
echo "Pass Rate: $(awk "BEGIN {printf \"%.2f%%\", ($PASSED/$TOTAL)*100}")"

if [ $FAILED -gt 0 ]; then
  echo "❌ CHALLENGE FAILED"
  exit 1
fi

echo "✓ ALL TESTS PASSED"
exit 0
```

---

## Implementation Phases

### Phase 1: Core Infrastructure (2-3 days)
- Create `internal/formatters/` package structure
- Implement `Formatter` interface
- Implement `FormatterRegistry`
- Implement `FormatterExecutor` with middleware
- Write unit tests

### Phase 2: Git Submodules (1 day)
- Add all 118 formatters as Git submodules
- Create `formatters/scripts/` management scripts
- Create `formatters/VERSIONS.md` version tracking
- Build native binaries for all native formatters

### Phase 3: Native Formatters (3-4 days)
- Implement 60+ native binary formatters
- Wire into registry
- Write unit tests for each
- Create challenge script

### Phase 4: Service Formatters (3-4 days)
- Create Dockerfiles for 20+ service formatters
- Create `docker-compose.formatters.yml`
- Implement service client providers
- Create service wrappers (Python, Ruby, etc.)
- Wire into registry
- Write unit tests
- Create challenge script

### Phase 5: Built-in Formatters (1-2 days)
- Implement 15+ built-in formatter wrappers
- Wire into registry
- Write unit tests
- Create challenge script

### Phase 6: API Endpoints (1 day)
- Implement `formatters_handler.go`
- Add routes to router
- Write integration tests

### Phase 7: CLI Agent Integration (2-3 days)
- Update all 48 CLI agent config generators
- Add formatter preferences to configs
- Implement CLI commands (`format`, `format --check`, etc.)
- Write integration tests for each agent
- Create challenge script

### Phase 8: AI Debate Integration (2 days)
- Implement `debate_formatter_integration.go`
- Add auto-formatting to debate flow
- Add event streaming for format progress
- Write integration tests
- Create challenge script

### Phase 9: Testing & Validation (3-4 days)
- Write all challenge scripts
- Run all challenges
- Fix failures
- Achieve 100% pass rate

### Phase 10: Documentation (2 days)
- Update `CLAUDE.md`
- Update `docs/architecture/`
- Update `docs/user/`
- Update API reference
- Update video courses
- Update website
- Update SQL schemas
- Update diagrams

**Total Estimated Time**: 20-28 days

---

## Success Criteria

1. ✓ All 118 formatters integrated as Git submodules
2. ✓ All formatters available via unified API
3. ✓ All 48 CLI agents have formatter configuration
4. ✓ AI Debate system auto-formats all code outputs
5. ✓ 100% test coverage (unit + integration + challenges)
6. ✓ Zero false positives in validation
7. ✓ All challenges pass at 100%
8. ✓ Documentation complete and up-to-date
9. ✓ Performance benchmarks documented
10. ✓ Service containers running and healthy

---

## Dependencies

- Docker/Podman for service containers
- Git for submodules
- Language runtimes for testing (Python, Ruby, Node.js, Java, Go, Rust, etc.)
- Build tools (cargo, npm, maven, gradle, etc.)

---

## Next Steps

1. Review and approve this architecture
2. Begin Phase 1: Core Infrastructure
3. Proceed sequentially through phases
4. Continuous testing and validation

---

**Document Version**: 1.0
**Last Updated**: 2026-01-29
**Status**: Awaiting Approval
