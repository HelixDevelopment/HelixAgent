# HelixAgent Toolkit Makefile

.PHONY: help build test clean lint fmt vet coverage docs docker-build docker-run

# Default target
help: ## Show this help message
	@echo "HelixAgent Toolkit - Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

# Build targets
build: ## Build all packages
	@echo "Building HelixAgent Toolkit..."
	@go build ./...

build-cli: ## Build the CLI tool
	@echo "Building CLI tool..."
	@go build -o bin/toolkit ./cmd/toolkit

build-all: ## Build for multiple architectures
	@echo "Building for multiple architectures..."
	@GOOS=linux GOARCH=amd64 go build -o bin/helixagent-linux-amd64 ./cmd/toolkit
	@GOOS=darwin GOARCH=amd64 go build -o bin/helixagent-darwin-amd64 ./cmd/toolkit
	@GOOS=darwin GOARCH=arm64 go build -o bin/helixagent-darwin-arm64 ./cmd/toolkit
	@GOOS=windows GOARCH=amd64 go build -o bin/helixagent-windows-amd64.exe ./cmd/toolkit

# Test targets
test: ## Run all tests
	@echo "Running all tests..."
	@go test -v ./...

test-unit: ## Run unit tests only (short)
	@echo "Running unit tests..."
	@go test -v -short ./...

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	@go test -v ./tests/integration/...

test-e2e: ## Run end-to-end tests
	@echo "Running end-to-end tests..."
	@go test -v ./tests/e2e/...

test-performance: ## Run performance tests
	@echo "Running performance tests..."
	@go test -v -bench=. ./tests/performance/...

test-security: ## Run security tests
	@echo "Running security tests..."
	@go test -v ./tests/security/...

test-chaos: ## Run chaos engineering tests
	@echo "Running chaos tests..."
	@go test -v ./tests/chaos/...

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-fuzz: ## Run fuzz tests
	@echo "Running fuzz tests..."
	@go test -fuzz=. -fuzztime=10s ./...

test-fuzz-verbose: ## Run fuzz tests with verbose output
	@echo "Running fuzz tests with verbose output..."
	@go test -fuzz=. -fuzztime=30s -v ./...

test-single: ## Run tests for a specific package (usage: make test-single PKG=./Providers/Chutes)
	@echo "Running tests for $(PKG)..."
	@go test -v $(PKG)

test-function: ## Run a specific test function (usage: make test-function FUNC=TestChutesProvider PKG=./Providers/Chutes)
	@echo "Running test function $(FUNC) in $(PKG)..."
	@go test -v -run $(FUNC) $(PKG)

# Code quality targets
lint: ## Run golangci-lint
	@echo "Running linter..."
	@golangci-lint run

fmt: ## Format Go code
	@echo "Formatting code..."
	@gofmt -w .

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

check: fmt vet lint ## Run all code quality checks

# Documentation targets
docs: ## Generate documentation
	@echo "Generating documentation..."
	@godoc -http=:6060 ./...

docs-serve: ## Serve documentation locally
	@echo "Serving documentation on http://localhost:6060"
	@godoc -http=:6060

# Docker targets
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t helixagent-toolkit .

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	@docker run -p 8080:8080 helixagent-toolkit

docker-test: ## Run tests in Docker
	@echo "Running tests in Docker..."
	@docker run --rm helixagent-toolkit make test

# Development targets
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@go clean ./...
	@rm -rf bin/
	@rm -f coverage.out coverage.html

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download

deps-tidy: ## Clean up dependencies
	@echo "Tidying dependencies..."
	@go mod tidy

deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	@go mod tidy && go get -u ./...

# CI/CD targets
ci: check test-coverage ## Run CI pipeline locally

# Utility targets
version: ## Show version information
	@echo "HelixAgent Toolkit"
	@go version
	@golangci-lint --version || echo "golangci-lint not installed"

install-tools: ## Install development tools
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/godoc@latest

# Quick development targets
dev-build: ## Quick build for development
	@echo "Quick development build..."
	@go build -o bin/dev ./...

dev-test: ## Quick test run for development
	@echo "Quick development test..."
	@go test ./...

dev: dev-build dev-test ## Full development cycle

# Provider-specific targets
test-chutes: ## Test Chutes provider
	@echo "Testing Chutes provider..."
	@go test -v ./Providers/Chutes/

test-siliconflow: ## Test SiliconFlow provider
	@echo "Testing SiliconFlow provider..."
	@go test -v ./Providers/SiliconFlow/

# Benchmark targets
bench: ## Run benchmarks
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

bench-profile: ## Run benchmarks with profiling
	@echo "Running benchmarks with profiling..."
	@go test -bench=. -benchmem -cpuprofile=cpu.prof -memprofile=mem.prof ./...

# Security targets
security-scan: ## Run security scan
	@echo "Running security scan..."
	@gosec ./... || echo "gosec not installed, run: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"

# Release targets
release-check: check test-coverage security-scan ## Run all checks for release
	@echo "All release checks passed!"

# Help for specific targets
help-build: ## Show build-related targets
	@echo "Build targets:"
	@echo "  build        - Build all packages"
	@echo "  build-all    - Build for multiple architectures"
	@echo "  docker-build - Build Docker image"

help-test: ## Show test-related targets
	@echo "Test targets:"
	@echo "  test              - Run all tests"
	@echo "  test-unit         - Run unit tests only"
	@echo "  test-integration  - Run integration tests"
	@echo "  test-e2e          - Run end-to-end tests"
	@echo "  test-performance  - Run performance tests"
	@echo "  test-security     - Run security tests"
	@echo "  test-chaos        - Run chaos tests"
	@echo "  test-coverage     - Run tests with coverage"
	@echo "  test-fuzz         - Run fuzz tests"
	@echo "  test-fuzz-verbose - Run fuzz tests with verbose output"
	@echo "  test-single       - Run tests for specific package"
	@echo "  test-function     - Run specific test function"