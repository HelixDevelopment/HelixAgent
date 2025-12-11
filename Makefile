.PHONY: all build test run fmt lint security-scan docker-build docker-run docker-stop docker-clean docker-logs docker-test docker-dev docker-prod coverage docker-clean-all install-deps help docs check-deps

# =============================================================================
# MAIN TARGETS
# =============================================================================

all: fmt vet lint test build

# =============================================================================
# BUILD TARGETS
# =============================================================================

build:
	@echo "🔨 Building SuperAgent..."
	go build -ldflags="-w -s" -o bin/superagent ./cmd/superagent

build-debug:
	@echo "🐛 Building SuperAgent (debug)..."
	go build -gcflags="all=-N -l" -o bin/superagent-debug ./cmd/superagent

build-all:
	@echo "🔨 Building all architectures..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o bin/superagent-linux-amd64 ./cmd/superagent
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-w -s" -o bin/superagent-linux-arm64 ./cmd/superagent
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-w -s" -o bin/superagent-darwin-amd64 ./cmd/superagent
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-w -s" -o bin/superagent-darwin-arm64 ./cmd/superagent
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-w -s" -o bin/superagent-windows-amd64.exe ./cmd/superagent

# =============================================================================
# RUN TARGETS
# =============================================================================

run:
	@echo "🚀 Running SuperAgent..."
	go run ./cmd/superagent/main.go

run-dev:
	@echo "🔧 Running SuperAgent in development mode..."
	GIN_MODE=debug go run ./cmd/superagent/main.go

# =============================================================================
# TEST TARGETS
# =============================================================================

test:
	@echo "🧪 Running tests..."
	go test -v ./...

test-coverage:
	@echo "📊 Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "📈 Coverage report generated: coverage.html"

test-coverage-100:
	@echo "📊 Running tests with 100% coverage requirement..."
	@go test -v -race -coverprofile=coverage.out ./...
	@coverage=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	if [ $$(echo "$$coverage < 100" | bc -l) -eq 1 ]; then \
		echo "❌ Coverage is $$coverage%, required 100%"; \
		exit 1; \
	else \
		echo "✅ Coverage is $$coverage%"; \
	fi
	go tool cover -html=coverage.out -o coverage.html

test-unit:
	@echo "🧪 Running unit tests..."
	go test -v ./internal/... -short

test-integration:
	@echo "🧪 Running integration tests..."
	go test -v ./tests/integration

test-e2e:
	@echo "🧪 Running end-to-end tests..."
	go test -v ./tests/e2e

test-security:
	@echo "🔒 Running security tests..."
	go test -v ./tests/security

test-stress:
	@echo "⚡ Running stress tests..."
	go test -v ./tests/stress

test-chaos:
	@echo "🌀 Running chaos tests..."
	go test -v ./tests/challenge

test-all-types:
	@echo "🧪 Running all 6 test types..."
	@echo "1. Unit tests..."
	go test -v ./internal/... -short
	@echo "2. Integration tests..."
	go test -v ./tests/integration
	@echo "3. E2E tests..."
	go test -v ./tests/e2e
	@echo "4. Security tests..."
	go test -v ./tests/security
	@echo "5. Stress tests..."
	go test -v ./tests/stress
	@echo "6. Chaos tests..."
	go test -v ./tests/challenge

test-bench:
	@echo "⚡ Running benchmark tests..."
	go test -bench=. -benchmem ./...

test-race:
	@echo "🏃 Running race condition tests..."
	go test -race ./...

# =============================================================================
# CODE QUALITY TARGETS
# =============================================================================

fmt:
	@echo "✨ Formatting code..."
	go fmt ./...

vet:
	@echo "🔍 Running go vet..."
	go vet ./...

lint:
	@echo "🔍 Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint not installed. Install with: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin"; \
	fi

security-scan:
	@echo "🔒 Running security scan..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "⚠️  gosec not installed. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi

# =============================================================================
# DOCKER TARGETS
# =============================================================================

docker-build:
	@echo "🐳 Building Docker image..."
	docker build -t superagent:latest .

docker-build-prod:
	@echo "🐳 Building production Docker image..."
	docker build --target=production -t superagent:prod .

docker-run:
	@echo "🐳 Starting SuperAgent with Docker..."
	docker-compose up -d

docker-stop:
	@echo "🐳 Stopping SuperAgent..."
	docker-compose down

docker-logs:
	@echo "📋 Showing Docker logs..."
	docker-compose logs -f

docker-clean:
	@echo "🧹 Cleaning Docker containers..."
	docker-compose down -v --remove-orphans

docker-clean-all:
	@echo "🧹 Cleaning all Docker resources..."
	docker-compose down -v --remove-orphans
	docker system prune -f
	docker volume prune -f

docker-test:
	@echo "🧪 Running tests in Docker..."
	docker-compose -f docker-compose.test.yml up --build -d
	sleep 10
	docker-compose -f docker-compose.test.yml exec superagent go test ./...
	docker-compose -f docker-compose.test.yml down

docker-dev:
	@echo "🧪 Starting development environment..."
	docker-compose --profile dev up -d

docker-prod:
	@echo "🚀 Starting production environment..."
	docker-compose --profile prod up -d

docker-full:
	@echo "🚀 Starting full environment..."
	docker-compose --profile full up -d

docker-monitoring:
	@echo "📊 Starting monitoring stack..."
	docker-compose --profile monitoring up -d

docker-ai:
	@echo "🤖 Starting AI services..."
	docker-compose --profile ai up -d

# =============================================================================
# INSTALLATION TARGETS
# =============================================================================

install-deps:
	@echo "📦 Installing dependencies..."
	go mod download
	go mod tidy
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "✅ golangci-lint already installed"; \
	else \
		echo "📦 Installing golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin; \
	fi
	@if command -v gosec >/dev/null 2>&1; then \
		echo "✅ gosec already installed"; \
	else \
		echo "📦 Installing gosec..."; \
		go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest; \
	fi

install:
	@echo "📦 Installing SuperAgent..."
	mkdir -p /usr/local/bin
	cp bin/superagent /usr/local/bin/
	@echo "✅ SuperAgent installed to /usr/local/bin/superagent"

uninstall:
	@echo "🗑️ Uninstalling SuperAgent..."
	rm -f /usr/local/bin/superagent
	@echo "✅ SuperAgent uninstalled"

# =============================================================================
# UTILITIES TARGETS
# =============================================================================

clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html

clean-all:
	@echo "🧹 Cleaning all artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean -modcache
	go clean -testcache

check-deps:
	@echo "🔍 Checking dependencies..."
	go mod verify
	go list -u -m all

update-deps:
	@echo "📦 Updating dependencies..."
	go get -u ./...
	go mod tidy

generate:
	@echo "🔧 Generating code..."
	go generate ./...

# =============================================================================
# DOCUMENTATION TARGETS
# =============================================================================

docs:
	@echo "📚 Generating documentation..."
	@if command -v godoc >/dev/null 2>&1; then \
		echo "📖 Starting documentation server at http://localhost:6060"; \
		godoc -http=:6060; \
	else \
		echo "⚠️  godoc not installed. Install with: go install golang.org/x/tools/cmd/godoc@latest"; \
	fi

docs-api:
	@echo "📚 Generating API documentation..."
	@echo "API documentation available at: http://localhost:8080/docs"

docs-build:
	@echo "📚 Building comprehensive documentation..."
	@mkdir -p Website/docs
	@cp -r docs/* Website/docs/
	@echo "✅ Documentation built in Website/docs/"

docs-user-manuals:
	@echo "📚 Building user manuals..."
	@mkdir -p Website/user-manuals
	@echo "User manuals will be generated here" > Website/user-manuals/README.md
	@echo "✅ User manuals directory created"

docs-video-courses:
	@echo "🎥 Building video course materials..."
	@mkdir -p Website/video-courses
	@echo "Video course materials will be generated here" > Website/video-courses/README.md
	@echo "✅ Video courses directory created"

# =============================================================================
# PROVISIONING TARGETS
# =============================================================================

setup-dev:
	@echo "🔧 Setting up development environment..."
	cp .env.example .env
	@echo "✅ .env file created from template"
	@echo "🔧 Please edit .env file with your configuration"

setup-prod:
	@echo "🚀 Setting up production environment..."
	cp .env.example .env.prod
	@echo "✅ .env.prod file created from template"
	@echo "🔧 Please edit .env.prod file with production configuration"

# =============================================================================
# HELP TARGET
# =============================================================================

help:
	@echo "🚀 SuperAgent Makefile Commands"
	@echo ""
	@echo "🔨 Build Commands:"
	@echo "  build              Build SuperAgent binary"
	@echo "  build-debug        Build SuperAgent binary (debug mode)"
	@echo "  build-all          Build for all architectures"
	@echo ""
	@echo "🏃 Run Commands:"
	@echo "  run                Run SuperAgent locally"
	@echo "  run-dev            Run SuperAgent in development mode"
	@echo ""
	@echo "🧪 Test Commands (6 Types):"
	@echo "  test               Run all tests"
	@echo "  test-coverage      Run tests with coverage report"
	@echo "  test-coverage-100  Run tests with 100% coverage requirement"
	@echo "  test-unit          Run unit tests only"
	@echo "  test-integration   Run integration tests only"
	@echo "  test-e2e           Run end-to-end tests"
	@echo "  test-security      Run security tests"
	@echo "  test-stress        Run stress tests"
	@echo "  test-chaos         Run chaos tests"
	@echo "  test-all-types     Run all 6 test types"
	@echo "  test-bench         Run benchmark tests"
	@echo "  test-race          Run tests with race detection"
	@echo ""
	@echo "✨ Code Quality:"
	@echo "  fmt                Format Go code"
	@echo "  vet                Run go vet"
	@echo "  lint               Run linter"
	@echo "  security-scan      Run security scan"
	@echo ""
	@echo "🐳 Docker Commands:"
	@echo "  docker-build        Build Docker image"
	@echo "  docker-run         Start services with Docker Compose"
	@echo "  docker-stop         Stop Docker services"
	@echo "  docker-logs         Show Docker logs"
	@echo "  docker-clean        Clean Docker containers"
	@echo "  docker-full         Start full environment"
	@echo "  docker-monitoring   Start monitoring stack"
	@echo "  docker-ai           Start AI services"
	@echo ""
	@echo "📦 Installation:"
	@echo "  install-deps       Install development dependencies"
	@echo "  install            Install SuperAgent to system"
	@echo "  uninstall          Remove SuperAgent from system"
	@echo ""
	@echo "🧰 Utilities:"
	@echo "  clean              Clean build artifacts"
	@echo "  clean-all          Clean all artifacts and caches"
	@echo "  check-deps         Check dependencies"
	@echo "  update-deps        Update dependencies"
	@echo "  generate           Generate code"
	@echo ""
	@echo "📚 Documentation:"
	@echo "  docs               Serve documentation"
	@echo "  docs-api           Show API documentation endpoint"
	@echo "  docs-build         Build comprehensive documentation"
	@echo "  docs-user-manuals  Build user manuals"
	@echo "  docs-video-courses Build video course materials"
	@echo ""
	@echo "⚙️  Setup:"
	@echo "  setup-dev          Setup development environment"
	@echo "  setup-prod         Setup production environment"
	@echo "  help               Show this help message"

# =============================================================================
# PHONY TARGETS
# =============================================================================
.PHONY: all build build-debug build-all run run-dev test test-coverage test-unit test-integration test-bench test-race fmt vet lint security-scan docker-build docker-build-prod docker-run docker-stop docker-logs docker-clean docker-clean-all docker-test docker-dev docker-prod docker-full docker-monitoring docker-ai install-deps install uninstall clean clean-all check-deps update-deps generate docs docs-api setup-dev setup-prod help
