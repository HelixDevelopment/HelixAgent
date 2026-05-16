# Contributing to HelixAgent

Thank you for your interest in contributing to HelixAgent! This document provides guidelines and instructions for contributing.

## Code of Conduct

We are committed to providing a welcoming and inclusive environment for all contributors. Please be respectful and constructive in all interactions.

## Getting Started

### Prerequisites

- **Go 1.24+** (with toolchain go1.24.11)
- **Docker** and **Docker Compose** (or Podman)
- **Make** (build automation)
- **Git** (version control)

### Development Setup

```bash
# Clone the repository
git clone https://github.com/your-org/HelixAgent.git
cd HelixAgent

# Install development dependencies
make install-deps

# Download Go dependencies
make deps

# Run tests to verify setup
make test

# Build the application
make build
```

### Project Structure

```
helix_agent/
├── cmd/              # Application entry points
├── internal/         # Private application code
│   ├── llm/          # LLM provider implementations
│   ├── services/     # Business logic
│   ├── handlers/     # HTTP handlers
│   └── ...
├── tests/            # Test suites
├── docs/             # Documentation
└── Makefile          # Build automation
```

## Development Workflow

### 1. Create a Branch

```bash
# Create a feature branch
git checkout -b feature/your-feature-name

# Or a bugfix branch
git checkout -b fix/issue-description
```

### 2. Make Changes

Follow these guidelines when making changes:

- **Code Style**: Follow Go conventions and run `make fmt`
- **Testing**: Add tests for new functionality
- **Documentation**: Update relevant documentation
- **Commits**: Write clear, descriptive commit messages

### 3. Run Quality Checks

```bash
# Format code
make fmt

# Run linter
make lint

# Run static analysis
make vet

# Run security scan
make security-scan

# Run all tests
make test
```

### 4. Submit a Pull Request

1. Push your branch to the remote repository
2. Create a Pull Request against the `main` branch
3. Fill in the PR template with relevant information
4. Request review from maintainers

## Code Style Guidelines

### Go Code

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting (run `make fmt`)
- Use meaningful variable and function names
- Add comments for exported functions and types
- Keep functions focused and concise

### Example

```go
// ProcessRequest handles incoming API requests and returns a response.
// It validates the input, processes it through the appropriate handler,
// and returns the result or an error.
func ProcessRequest(ctx context.Context, req *Request) (*Response, error) {
    if err := req.Validate(); err != nil {
        return nil, fmt.Errorf("invalid request: %w", err)
    }
    
    // Process the request
    result, err := processInternal(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("processing failed: %w", err)
    }
    
    return &Response{Data: result}, nil
}
```

### Testing

- Write table-driven tests where appropriate
- Use `testify/assert` and `testify/require` for assertions
- Mock external dependencies
- Aim for 80%+ code coverage

```go
func TestProcessRequest(t *testing.T) {
    tests := []struct {
        name    string
        input   *Request
        want    *Response
        wantErr bool
    }{
        {
            name:  "valid request",
            input: &Request{Data: "test"},
            want:  &Response{Data: "processed"},
        },
        {
            name:    "invalid request",
            input:   &Request{},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ProcessRequest(context.Background(), tt.input)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

## Testing Requirements

### Test Types

| Type | Command | Description |
|------|---------|-------------|
| Unit | `make test-unit` | Individual function tests |
| Integration | `make test-integration` | Component interaction tests |
| E2E | `make test-e2e` | End-to-end flow tests |
| Security | `make test-security` | Security vulnerability tests |
| Performance | `make test-bench` | Benchmark tests |

### Running Tests

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run specific test
go test -v -run TestFunctionName ./path/to/package

# Run with race detection
make test-race
```

### Test Infrastructure

For integration and E2E tests:

```bash
# Start test infrastructure (PostgreSQL, Redis)
make test-infra-start

# Run tests with infrastructure
make test-with-infra

# Stop infrastructure
make test-infra-stop
```

## Documentation

### When to Update Documentation

- Adding new features or APIs
- Changing existing behavior
- Adding new configuration options
- Fixing bugs that affect documented behavior

### Documentation Locations

| Type | Location |
|------|----------|
| API Reference | `docs/api/` |
| Guides | `docs/guides/` |
| Architecture | `docs/architecture/` |
| Package Docs | `internal/*/README.md` |

## Adding New Features

### Adding a New LLM Provider

1. Create provider package: `internal/llm/providers/<name>/`
2. Implement `LLMProvider` interface
3. Add tests: `internal/llm/providers/<name>/*_test.go`
4. Register in `internal/services/provider_registry.go`
5. Add environment variables to `.env.example`
6. Update documentation

### Adding a New API Endpoint

1. Add handler in `internal/handlers/`
2. Add route in `internal/router/`
3. Add tests for the handler
4. Update API documentation in `docs/api/`
5. Update OpenAPI spec if applicable

## Commit Messages

Follow conventional commit format:

```
type(scope): description

[optional body]

[optional footer]
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

### Examples

```
feat(llm): add support for new provider XYZ

fix(handlers): resolve race condition in debate handler

docs(readme): update installation instructions

test(services): add unit tests for intent classifier
```

## Pull Request Process

### Before Submitting

- [ ] Code follows style guidelines
- [ ] All tests pass (`make test`)
- [ ] Linter passes (`make lint`)
- [ ] Documentation updated (if applicable)
- [ ] Commit messages follow conventions

### PR Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
How were changes tested?

## Checklist
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] Changelog updated (if applicable)
```

### Review Process

1. Automated checks run (CI pipeline)
2. Code review by maintainers
3. Address feedback and update PR
4. Approval and merge

## Getting Help

- **Issues**: Open a GitHub issue for bugs or feature requests
- **Discussions**: Use GitHub Discussions for questions
- **Documentation**: Check `docs/` for guides and references

## License

By contributing to HelixAgent, you agree that your contributions will be licensed under the same license as the project.

---

Thank you for contributing to HelixAgent!
