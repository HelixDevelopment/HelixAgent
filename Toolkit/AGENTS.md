# HelixAgent Development Guidelines

## Build/Lint/Test Commands
```bash
# Build
go build ./...

# Test single file
go test -v ./Providers/Chutes/chutes_test.go

# Test single function
go test -run TestChutesProvider ./Providers/Chutes/

# Run all tests
go test ./...
```

## Code Style Guidelines
- **Go 1.21+ required** - Use standard Go conventions (gofmt, go vet)
- **Imports** - Group imports (stdlib, third-party, internal), no unused imports
- **Naming** - Use CamelCase for exports, camelCase for private, constants UPPER_SNAKE_CASE
- **Error Handling** - Always handle errors, use structured error wrapping with context
- **Testing** - Use table-driven tests, include both positive and negative test cases
- **Interfaces** - Design for testability, keep interfaces small and focused
- **Documentation** - Document all exported functions with examples