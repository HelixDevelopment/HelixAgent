# Contributing to HelixAgent

Thank you for your interest in contributing to HelixAgent!

## Development Setup

### Prerequisites

- Go 1.25.3+
- Docker or Podman (for container orchestration)
- golangci-lint
- gosec

### Getting Started

```bash
# Clone the repository (SSH only - HTTPS is forbidden)
git clone --recurse-submodules git@github.com:vasic-digital/HelixAgent.git
cd HelixAgent

# Install development tools
make install-deps

# Build the project
make build

# Run unit tests (no infrastructure needed)
make test-unit

# Run with auto container orchestration
./bin/helixagent
```

**Note:** HelixAgent automatically manages all infrastructure containers on startup. Do not start containers manually. For isolated test runs requiring infrastructure, use `make test-infra-start`.

## Code Style

### Go Conventions

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` for formatting
- Use `goimports` for import organization
- Line length ≤ 100 characters

### Naming

- `camelCase` for private functions/variables
- `PascalCase` for exported functions/types
- `UPPER_SNAKE_CASE` for constants
- Acronyms in all caps (`HTTP`, `URL`, `ID`)

### Error Handling

```go
if err != nil {
    return fmt.Errorf("context: %w", err)
}
```

### Testing

- Write table-driven tests
- Use `testify` for assertions
- Place tests in same package with `_test.go` suffix
- Use `testdata/` for fixtures

## Commit Guidelines

### Branch Naming

- `feat/` - New features
- `fix/` - Bug fixes
- `chore/` - Maintenance
- `docs/` - Documentation
- `refactor/` - Code refactoring
- `test/` - Test changes

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

Example:
```
feat(llm): add ensemble voting strategy

Implements confidence-weighted voting for multi-provider responses.
```

## Pull Request Process

1. Create a feature branch
2. Make your changes
3. Run quality checks:
   ```bash
   make fmt vet lint test
   ```
4. Update documentation if needed
5. Submit PR with description of changes

## Code Review Checklist

- [ ] Code compiles without errors
- [ ] All tests pass
- [ ] No linting errors
- [ ] Documentation updated
- [ ] Commit messages follow conventions

## Reporting Issues

- Use GitHub Issues
- Include reproduction steps
- Include relevant logs
- Specify environment (OS, Go version)

## License

By contributing, you agree that your contributions will be licensed under the project's license.

## Contact

- GitHub Issues: https://github.com/vasic-digital/HelixAgent/issues
