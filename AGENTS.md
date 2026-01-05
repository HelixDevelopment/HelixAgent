# AGENTS.md

This document provides guidance for AI agents working on the SuperAgent project. It includes essential commands for building, testing, and formatting code, as well as code style guidelines.

## Project Overview

SuperAgent is an AI-powered ensemble LLM service written in Go (1.24+) that combines responses from multiple language models using intelligent aggregation strategies. It provides OpenAI-compatible APIs and supports 7 LLM providers (Claude, DeepSeek, Gemini, Qwen, ZAI, Ollama, OpenRouter).

## Quick Start

1. Install Go 1.24+ and Docker.
2. Clone the repository.
3. Run `make install-deps` to install development tools (golangci-lint, gosec).
4. Copy `.env.example` to `.env` and adjust settings if needed.
5. Run `make build` to build the binary.
6. Run `make test` to verify everything works.

## Build Commands

The project uses a Makefile with the following key targets:

### Core Build Commands
- `make build` – Build SuperAgent binary (output in `bin/`)
- `make build-debug` – Build with debug symbols
- `make build-all` – Build for all architectures (Linux, macOS, Windows)
- `make run` – Run SuperAgent locally
- `make run-dev` – Run in development mode (`GIN_MODE=debug`)

### Container Build Commands
- `make docker-build` – Build Docker image
- `make docker-run` – Start services with Docker Compose
- `make docker-stop` – Stop Docker services
- `make docker-clean` – Clean Docker containers and volumes
- `make docker-full` – Start full environment (all profiles)

## Linting & Formatting Commands

- `make fmt` – Format Go code with `go fmt`
- `make vet` – Run `go vet` for static analysis
- `make lint` – Run `golangci-lint` (install with `make install-deps`)
- `make security-scan` – Run `gosec` security scanner

Always run `make fmt vet lint` before committing.

## Testing Commands

### Basic Testing
- `make test` – Run all tests (verbose)
- `make test-unit` – Run unit tests only (`./internal/... -short`)
- `make test-coverage` – Run tests with coverage report (HTML output)
- `make test-bench` – Run benchmark tests
- `make test-race` – Run tests with race detection

### Specialized Test Suites
- `make test-integration` – Integration tests with Docker dependencies
- `make test-e2e` – End-to-end tests
- `make test-security` – Security tests
- `make test-stress` – Stress tests
- `make test-chaos` – Chaos/challenge tests

### Test Infrastructure Management
- `make test-infra-start` – Start PostgreSQL, Redis, Mock LLM containers
- `make test-infra-stop` – Stop test infrastructure
- `make test-infra-clean` – Stop and remove volumes
- `make test-with-infra` – Run all tests with Docker infrastructure

### Running a Single Test
To run a specific test or test suite, use the standard `go test` command:

```bash
# Run a single test function
go test -v -run TestFunctionName ./path/to/package

# Run all tests in a package
go test -v ./internal/llm

# Run tests matching a pattern
go test -v -run "Test.*Integration" ./...

# Run tests with coverage for a single package
go test -v -coverprofile=coverage.out ./internal/llm
```

## Code Style Guidelines

### General Principles
- Follow standard Go conventions as described in [Effective Go](https://go.dev/doc/effective_go).
- Write clear, readable, and maintainable code.
- Keep functions small and focused (single responsibility).
- Use comments to explain "why" not "what".
- Avoid premature optimization; profile first.

### Formatting
- Use `gofmt` (or `go fmt`) to format code. The project's Makefile provides `make fmt`.
- Use `goimports` to organize imports (standard library, third‑party, local).
- Imports should be grouped: standard library, external dependencies, internal packages (separated by a blank line).
- Line length: aim for ≤ 100 characters, but readability is more important.

### Naming Conventions
- Use `camelCase` for local variables and private functions.
- Use `PascalCase` for exported functions, types, constants, and fields.
- Use `UPPER_SNAKE_CASE` for exported constants.
- Acronyms should be all caps (e.g., `HTTP`, `URL`, `ID`).
- Use short, descriptive names; avoid abbreviations unless widely understood.
- Receiver names: use one or two letters (e.g., `s` for a service, `c` for a client).

### Error Handling
- Always check errors; do not ignore them.
- Use `if err != nil { return err }` pattern.
- Wrap errors with context using `fmt.Errorf("...: %w", err)`.
- Define custom error types when you need to expose specific error information.
- Use `defer` for cleanup (closing files, releasing resources).

### Types and Interfaces
- Use `interface` to define behavior, not data.
- Prefer small, focused interfaces (e.g., `io.Reader`, `io.Writer`).
- Use `struct` tags for JSON, YAML, database mapping, etc.
- Avoid `any` (`interface{}`); use generics or specific types when possible.
- Use type aliases and embedded structs judiciously.

### Concurrency
- Use goroutines and channels for concurrent tasks.
- Always provide a way to cancel or timeout operations (use `context.Context`).
- Protect shared data with `sync.Mutex` or `sync.RWMutex`.
- Consider using `sync.WaitGroup` to wait for goroutines to finish.

### Testing
- Write table‑driven tests when appropriate.
- Use the `testify` assertion library (already a project dependency).
- Mock external dependencies using interfaces.
- Place test files in the same package as the code being tested (suffix `_test.go`).
- Use `testdata/` directories for fixture files.

## Git & Commit Guidelines

- **Branch naming**: Use prefixes: `feat/`, `fix/`, `chore/`, `docs/`, `refactor/`, `test/`, followed by a short description (e.g., `feat/add-user-auth`).
- **Commit messages**: Follow [Conventional Commits](https://www.conventionalcommits.org/):
  - Format: `<type>(<scope>): <description>`
  - Common types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`.
  - Example: `feat(llm): add ensemble voting strategy`
- Always run `make fmt vet lint test` before committing.

## Cursor Rules

The repository includes a `.cursorrules` file with the following guidelines:

```
# Code style
- Use TypeScript with strict mode.
- Prefer functional programming patterns.
- Use async/await over callbacks.
- Write unit tests for all new features.
- Follow the existing project structure.
- Use JSDoc comments for public APIs.
- Avoid `any`; use proper types.
```

Note: Some of these rules are TypeScript‑specific; for Go code, follow the Go‑specific guidelines above.

## Additional Resources

- `CLAUDE.md` – Detailed project overview and architecture.
- `Makefile` – Complete list of available commands.
- `go.mod` – Go module dependencies.
- `.cursorrules` – Cursor‑specific guidelines.
- `docs/` – Project documentation.

---

*This document is intended for AI agents working in the SuperAgent repository. Keep it up to date as the project evolves.*