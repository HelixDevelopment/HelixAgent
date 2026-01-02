.PHONY: all build test run fmt lint security-scan docker-build docker-run docker-stop docker-clean docker-logs docker-test docker-dev docker-prod coverage docker-clean-all install-deps help docs check-deps test-all test-all-docker container-detect container-build container-start container-stop container-logs container-status container-test podman-build podman-run podman-stop podman-logs podman-clean podman-full

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
	@echo "🧪 Running integration tests with Docker dependencies..."
	@./scripts/run-integration-tests.sh --package ./internal/services/...

test-integration-verbose:
	@echo "🧪 Running integration tests (verbose)..."
	@./scripts/run-integration-tests.sh --verbose --package ./internal/services/...

test-all:
	@echo "🧪 Running ALL tests with full infrastructure (no skipping)..."
	@./scripts/run_all_tests.sh

test-infra-start:
	@echo "🐳 Starting test infrastructure (PostgreSQL, Redis, Mock LLM)..."
	@docker compose -f docker-compose.test.yml up -d postgres redis mock-llm
	@echo "⏳ Waiting for services to be ready..."
	@sleep 5
	@echo "Checking PostgreSQL..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		docker compose -f docker-compose.test.yml exec -T postgres pg_isready -U superagent -d superagent_db > /dev/null 2>&1 && break; \
		echo "  Waiting for PostgreSQL... ($$i/10)"; \
		sleep 2; \
	done
	@echo "Checking Redis..."
	@for i in 1 2 3 4 5; do \
		docker compose -f docker-compose.test.yml exec -T redis redis-cli ping > /dev/null 2>&1 && break; \
		echo "  Waiting for Redis... ($$i/5)"; \
		sleep 1; \
	done
	@echo "Checking Mock LLM..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		curl -sf http://localhost:$${MOCK_LLM_PORT:-18081}/health > /dev/null 2>&1 && break; \
		echo "  Waiting for Mock LLM... ($$i/10)"; \
		sleep 2; \
	done
	@echo "✅ Test infrastructure is ready!"
	@echo ""
	@echo "Services available at:"
	@echo "  PostgreSQL: localhost:$${POSTGRES_PORT:-15432} (superagent/superagent123)"
	@echo "  Redis:      localhost:$${REDIS_PORT:-16379} (password: superagent123)"
	@echo "  Mock LLM:   http://localhost:$${MOCK_LLM_PORT:-18081}"

test-infra-stop:
	@echo "🐳 Stopping test infrastructure..."
	@docker compose -f docker-compose.test.yml down
	@echo "✅ Test infrastructure stopped"

test-infra-clean:
	@echo "🧹 Cleaning test infrastructure (including volumes)..."
	@docker compose -f docker-compose.test.yml down -v --remove-orphans
	@echo "✅ Test infrastructure cleaned"

test-infra-logs:
	@echo "📋 Showing test infrastructure logs..."
	@docker compose -f docker-compose.test.yml logs -f

test-infra-status:
	@echo "📊 Test infrastructure status:"
	@docker compose -f docker-compose.test.yml ps

test-with-infra:
	@echo "🧪 Running tests with infrastructure..."
	@$(MAKE) test-infra-start
	@echo ""
	@DB_HOST=localhost DB_PORT=$${POSTGRES_PORT:-15432} DB_USER=superagent DB_PASSWORD=superagent123 DB_NAME=superagent_db \
		DATABASE_URL="postgres://superagent:superagent123@localhost:$${POSTGRES_PORT:-15432}/superagent_db?sslmode=disable" \
		REDIS_HOST=localhost REDIS_PORT=$${REDIS_PORT:-16379} REDIS_PASSWORD=superagent123 \
		REDIS_URL="redis://:superagent123@localhost:$${REDIS_PORT:-16379}" \
		MOCK_LLM_URL=http://localhost:$${MOCK_LLM_PORT:-18081} MOCK_LLM_ENABLED=true \
		CLAUDE_API_KEY=mock-api-key CLAUDE_BASE_URL=http://localhost:$${MOCK_LLM_PORT:-18081}/v1 \
		DEEPSEEK_API_KEY=mock-api-key DEEPSEEK_BASE_URL=http://localhost:$${MOCK_LLM_PORT:-18081}/v1 \
		GEMINI_API_KEY=mock-api-key GEMINI_BASE_URL=http://localhost:$${MOCK_LLM_PORT:-18081}/v1 \
		QWEN_API_KEY=mock-api-key QWEN_BASE_URL=http://localhost:$${MOCK_LLM_PORT:-18081}/v1 \
		ZAI_API_KEY=mock-api-key ZAI_BASE_URL=http://localhost:$${MOCK_LLM_PORT:-18081}/v1 \
		OLLAMA_BASE_URL=http://localhost:$${MOCK_LLM_PORT:-18081} \
		JWT_SECRET=test-jwt-secret-key-for-testing \
		CI=true FULL_TEST_MODE=true \
		go test -v ./... -timeout 300s -cover
	@echo ""
	@echo "✅ Tests completed!"

test-all-docker:
	@echo "🐳 Starting test infrastructure and running all tests..."
	@docker compose -f docker-compose.test.yml up -d postgres redis mock-llm
	@echo "⏳ Waiting for services..."
	@sleep 10
	@DB_HOST=localhost DB_PORT=5432 DB_USER=superagent DB_PASSWORD=superagent123 DB_NAME=superagent_db \
		REDIS_HOST=localhost REDIS_PORT=6379 REDIS_PASSWORD=superagent123 \
		MOCK_LLM_URL=http://localhost:8081 MOCK_LLM_ENABLED=true \
		CLAUDE_API_KEY=mock CLAUDE_BASE_URL=http://localhost:8081/v1 \
		DEEPSEEK_API_KEY=mock DEEPSEEK_BASE_URL=http://localhost:8081/v1 \
		GEMINI_API_KEY=mock GEMINI_BASE_URL=http://localhost:8081/v1 \
		QWEN_API_KEY=mock QWEN_BASE_URL=http://localhost:8081/v1 \
		ZAI_API_KEY=mock ZAI_BASE_URL=http://localhost:8081/v1 \
		OLLAMA_BASE_URL=http://localhost:8081 \
		CI=true go test -v ./... -timeout 300s
	@docker compose -f docker-compose.test.yml down

test-integration-coverage:
	@echo "🧪 Running integration tests with coverage..."
	@./scripts/run-integration-tests.sh --coverage --package ./internal/services/...

test-integration-keep:
	@echo "🧪 Running integration tests (keep containers)..."
	@./scripts/run-integration-tests.sh --keep --package ./internal/services/...

test-integration-all:
	@echo "🧪 Running all tests with Docker dependencies..."
	@./scripts/run-integration-tests.sh --coverage

test-integration-old:
	@echo "🧪 Running legacy integration tests..."
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
	docker compose up -d

docker-stop:
	@echo "🐳 Stopping SuperAgent..."
	docker compose down

docker-logs:
	@echo "📋 Showing Docker logs..."
	docker compose logs -f

docker-clean:
	@echo "🧹 Cleaning Docker containers..."
	docker compose down -v --remove-orphans

docker-clean-all:
	@echo "🧹 Cleaning all Docker resources..."
	docker compose down -v --remove-orphans
	docker system prune -f
	docker volume prune -f

docker-test:
	@echo "🧪 Running tests in Docker..."
	docker compose -f docker-compose.test.yml up --build -d
	sleep 10
	docker compose -f docker-compose.test.yml exec superagent go test ./...
	docker compose -f docker-compose.test.yml down

docker-dev:
	@echo "🧪 Starting development environment..."
	docker compose --profile dev up -d

docker-prod:
	@echo "🚀 Starting production environment..."
	docker compose --profile prod up -d

docker-full:
	@echo "🚀 Starting full environment..."
	docker compose --profile full up -d

docker-monitoring:
	@echo "📊 Starting monitoring stack..."
	docker compose --profile monitoring up -d

docker-ai:
	@echo "🤖 Starting AI services..."
	docker compose --profile ai up -d

# =============================================================================
# CONTAINER RUNTIME TARGETS (Docker/Podman)
# =============================================================================

container-detect:
	@echo "🔍 Detecting container runtime..."
	@./scripts/container-runtime.sh

container-build:
	@echo "🔨 Building container image..."
	@./scripts/container-runtime.sh build

container-start:
	@echo "🚀 Starting services..."
	@./scripts/container-runtime.sh start

container-stop:
	@echo "⏹️ Stopping services..."
	@./scripts/container-runtime.sh stop

container-logs:
	@echo "📋 Showing logs..."
	@./scripts/container-runtime.sh logs

container-status:
	@echo "📊 Checking status..."
	@./scripts/container-runtime.sh status

container-test:
	@echo "🧪 Running container compatibility tests..."
	@./tests/container/container_runtime_test.sh

# Podman-specific targets
podman-build:
	@echo "🦭 Building with Podman..."
	podman build -t superagent:latest .

podman-run:
	@echo "🦭 Running with Podman Compose..."
	podman-compose up -d

podman-stop:
	@echo "🦭 Stopping Podman services..."
	podman-compose down

podman-logs:
	@echo "📋 Showing Podman logs..."
	podman-compose logs -f

podman-clean:
	@echo "🧹 Cleaning Podman containers..."
	podman-compose down -v --remove-orphans

podman-full:
	@echo "🦭 Starting full Podman environment..."
	podman-compose --profile full up -d

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
	@echo "🧪 Test Commands:"
	@echo "  test               Run all tests (quick, may skip some)"
	@echo "  test-with-infra    Run ALL tests with Docker infrastructure"
	@echo "  test-all           Run ALL tests with full infrastructure script"
	@echo "  test-coverage      Run tests with coverage report"
	@echo "  test-unit          Run unit tests only"
	@echo ""
	@echo "🐳 Test Infrastructure:"
	@echo "  test-infra-start   Start test infrastructure (PostgreSQL, Redis, Mock LLM)"
	@echo "  test-infra-stop    Stop test infrastructure"
	@echo "  test-infra-clean   Stop and clean test infrastructure (remove volumes)"
	@echo "  test-infra-logs    Show test infrastructure logs"
	@echo "  test-infra-status  Show test infrastructure status"
	@echo ""
	@echo "🧪 More Test Commands:"
	@echo "  test-integration          Run integration tests with Docker deps"
	@echo "  test-integration-verbose  Run integration tests (verbose)"
	@echo "  test-integration-coverage Run integration tests with coverage"
	@echo "  test-integration-keep     Run tests, keep containers running"
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
.PHONY: all build build-debug build-all run run-dev test test-coverage test-unit test-integration test-bench test-race test-all test-with-infra test-infra-start test-infra-stop test-infra-clean test-infra-logs test-infra-status fmt vet lint security-scan docker-build docker-build-prod docker-run docker-stop docker-logs docker-clean docker-clean-all docker-test docker-dev docker-prod docker-full docker-monitoring docker-ai install-deps install uninstall clean clean-all check-deps update-deps generate docs docs-api setup-dev setup-prod help
