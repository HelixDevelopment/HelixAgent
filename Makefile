.PHONY: all build test run fmt lint security-scan docker-build docker-run docker-stop docker-clean docker-logs docker-test docker-dev docker-prod coverage docker-clean-all install-deps help docs check-deps test-all test-all-docker container-detect container-build container-start container-stop container-logs container-status container-test podman-build podman-run podman-stop podman-logs podman-clean podman-full

# =============================================================================
# MAIN TARGETS
# =============================================================================

all: fmt vet lint test build

# =============================================================================
# BUILD TARGETS
# =============================================================================

build:
	@echo "🔨 Building HelixAgent..."
	go build -ldflags="-w -s" -o bin/helixagent ./cmd/helixagent

build-debug:
	@echo "🐛 Building HelixAgent (debug)..."
	go build -gcflags="all=-N -l" -o bin/helixagent-debug ./cmd/helixagent

build-all:
	@echo "🔨 Building all architectures..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o bin/helixagent-linux-amd64 ./cmd/helixagent
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-w -s" -o bin/helixagent-linux-arm64 ./cmd/helixagent
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-w -s" -o bin/helixagent-darwin-amd64 ./cmd/helixagent
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-w -s" -o bin/helixagent-darwin-arm64 ./cmd/helixagent
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-w -s" -o bin/helixagent-windows-amd64.exe ./cmd/helixagent

# =============================================================================
# RUN TARGETS
# =============================================================================

run:
	@echo "🚀 Running HelixAgent..."
	go run ./cmd/helixagent/main.go

run-dev:
	@echo "🔧 Running HelixAgent in development mode..."
	GIN_MODE=debug go run ./cmd/helixagent/main.go

# =============================================================================
# TEST TARGETS
# =============================================================================

test:
	@echo "🧪 Running tests..."
	@# Check if infrastructure is available
	@if nc -z localhost $${POSTGRES_PORT:-15432} 2>/dev/null && nc -z localhost $${REDIS_PORT:-16379} 2>/dev/null; then \
		echo "✅ Infrastructure available - running full tests"; \
		DB_HOST=localhost DB_PORT=$${POSTGRES_PORT:-15432} DB_USER=helixagent DB_PASSWORD=helixagent123 DB_NAME=helixagent_db \
		DATABASE_URL="postgres://helixagent:helixagent123@localhost:$${POSTGRES_PORT:-15432}/helixagent_db?sslmode=disable" \
		REDIS_HOST=localhost REDIS_PORT=$${REDIS_PORT:-16379} REDIS_PASSWORD=helixagent123 \
		go test -v ./...; \
	else \
		echo "⚠️  Infrastructure not available - running unit tests only"; \
		echo "   Run 'make test-infra-start' first for full test suite"; \
		echo ""; \
		go test -v -short ./...; \
	fi

# Auto-start infrastructure if not running (supports Docker and Podman)
ensure-test-infra:
	@echo "🔍 Checking test infrastructure..."
	@INFRA_NEEDED=false; \
	if ! nc -z localhost $${POSTGRES_PORT:-15432} 2>/dev/null; then \
		INFRA_NEEDED=true; \
	fi; \
	if ! nc -z localhost $${REDIS_PORT:-16379} 2>/dev/null; then \
		INFRA_NEEDED=true; \
	fi; \
	if [ "$$INFRA_NEEDED" = "true" ]; then \
		echo "🐳 Starting test infrastructure..."; \
		$(MAKE) test-infra-auto-start || { \
			echo ""; \
			echo "⚠️  WARNING: Could not start test infrastructure."; \
			echo "   Tests requiring PostgreSQL/Redis will be skipped."; \
			echo ""; \
			echo "   To fix Podman rootless issue, run as root:"; \
			echo "     echo 'milosvasic:100000:65536' >> /etc/subuid"; \
			echo "     echo 'milosvasic:100000:65536' >> /etc/subgid"; \
			echo "   Then run: podman system migrate"; \
			echo ""; \
			exit 0; \
		}; \
	fi
	@echo "✅ Test infrastructure check complete"

# Auto-start with Docker/Podman detection (supports compose and direct container run)
test-infra-auto-start:
	@echo "🔍 Detecting container runtime..."
	@if command -v docker &> /dev/null && docker info &> /dev/null 2>&1; then \
		echo "🐳 Using Docker..."; \
		docker compose -f docker-compose.test.yml up -d postgres redis 2>/dev/null || \
		docker-compose -f docker-compose.test.yml up -d postgres redis 2>/dev/null || \
		$(MAKE) test-infra-direct-start; \
	elif command -v podman &> /dev/null; then \
		echo "🦭 Using Podman..."; \
		if command -v podman-compose &> /dev/null; then \
			podman-compose -f docker-compose.test.yml up -d postgres redis; \
		else \
			echo "📦 Starting containers directly with Podman..."; \
			$(MAKE) test-infra-direct-start; \
		fi; \
	else \
		echo "❌ No container runtime found. Install Docker or Podman."; \
		exit 1; \
	fi
	@echo "⏳ Waiting for services to be ready..."
	@sleep 5
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		nc -z localhost $${POSTGRES_PORT:-15432} 2>/dev/null && break; \
		echo "  Waiting for PostgreSQL... ($$i/10)"; \
		sleep 2; \
	done
	@for i in 1 2 3 4 5; do \
		nc -z localhost $${REDIS_PORT:-16379} 2>/dev/null && break; \
		echo "  Waiting for Redis... ($$i/5)"; \
		sleep 1; \
	done
	@echo "✅ Test infrastructure started"

# Direct container start (fallback when compose is not available)
# Uses fully qualified image names for Podman compatibility
# Uses --userns=host for Podman to bypass subuid issues
test-infra-direct-start:
	@echo "📦 Starting containers directly..."
	@# Detect runtime and set options
	@RUNTIME="$$(command -v docker 2>/dev/null || command -v podman 2>/dev/null)"; \
	if [ -z "$$RUNTIME" ]; then echo "❌ No container runtime found"; exit 1; fi; \
	echo "Using: $$RUNTIME"; \
	# Set Podman-specific options \
	EXTRA_OPTS=""; \
	if echo "$$RUNTIME" | grep -q podman; then \
		EXTRA_OPTS="--userns=host"; \
		echo "Using Podman with --userns=host mode"; \
	fi; \
	# Create network if not exists \
	$$RUNTIME network create helixagent-test-net 2>/dev/null || true; \
	# Stop and remove existing containers \
	$$RUNTIME rm -f helixagent-test-postgres helixagent-test-redis 2>/dev/null || true; \
	# Start PostgreSQL (using fully qualified name for Podman) \
	echo "🐘 Starting PostgreSQL..."; \
	$$RUNTIME run -d --name helixagent-test-postgres $$EXTRA_OPTS \
		--network helixagent-test-net \
		-p $${POSTGRES_PORT:-15432}:5432 \
		-e POSTGRES_DB=helixagent_db \
		-e POSTGRES_USER=helixagent \
		-e POSTGRES_PASSWORD=helixagent123 \
		docker.io/library/postgres:15-alpine || exit 1; \
	# Start Redis (using fully qualified name for Podman) \
	echo "🔴 Starting Redis..."; \
	$$RUNTIME run -d --name helixagent-test-redis $$EXTRA_OPTS \
		--network helixagent-test-net \
		-p $${REDIS_PORT:-16379}:6379 \
		docker.io/library/redis:7-alpine redis-server --requirepass helixagent123 --appendonly yes || exit 1; \
	echo "✅ Containers started"

# Stop direct containers
test-infra-direct-stop:
	@RUNTIME="$$(command -v docker 2>/dev/null || command -v podman 2>/dev/null)"; \
	if [ -n "$$RUNTIME" ]; then \
		$$RUNTIME rm -f helixagent-test-postgres helixagent-test-redis 2>/dev/null || true; \
		$$RUNTIME network rm helixagent-test-net 2>/dev/null || true; \
		echo "✅ Test containers stopped"; \
	fi

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

test-complete:
	@echo "🧪 Running COMPLETE test suite (all 6 types) with full infrastructure..."
	@./scripts/run_complete_test_suite.sh --verbose --coverage

test-complete-keep:
	@echo "🧪 Running COMPLETE test suite (keeping containers for debugging)..."
	@./scripts/run_complete_test_suite.sh --verbose --coverage --keep

test-infra-start:
	@echo "🐳 Starting test infrastructure (PostgreSQL, Redis, Mock LLM)..."
	@docker compose -f docker-compose.test.yml up -d postgres redis mock-llm
	@echo "⏳ Waiting for services to be ready..."
	@sleep 5
	@echo "Checking PostgreSQL..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		docker compose -f docker-compose.test.yml exec -T postgres pg_isready -U helixagent -d helixagent_db > /dev/null 2>&1 && break; \
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
	@echo "  PostgreSQL: localhost:$${POSTGRES_PORT:-15432} (helixagent/helixagent123)"
	@echo "  Redis:      localhost:$${REDIS_PORT:-16379} (password: helixagent123)"
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
	@DB_HOST=localhost DB_PORT=$${POSTGRES_PORT:-15432} DB_USER=helixagent DB_PASSWORD=helixagent123 DB_NAME=helixagent_db \
		DATABASE_URL="postgres://helixagent:helixagent123@localhost:$${POSTGRES_PORT:-15432}/helixagent_db?sslmode=disable" \
		REDIS_HOST=localhost REDIS_PORT=$${REDIS_PORT:-16379} REDIS_PASSWORD=helixagent123 \
		REDIS_URL="redis://:helixagent123@localhost:$${REDIS_PORT:-16379}" \
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
	@DB_HOST=localhost DB_PORT=5432 DB_USER=helixagent DB_PASSWORD=helixagent123 DB_NAME=helixagent_db \
		REDIS_HOST=localhost REDIS_PORT=6379 REDIS_PASSWORD=helixagent123 \
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
	@echo "🧪 Running all 6 test types with full infrastructure..."
	@./scripts/run_complete_test_suite.sh --verbose

test-all-types-coverage:
	@echo "🧪 Running all 6 test types with full infrastructure and coverage..."
	@./scripts/run_complete_test_suite.sh --verbose --coverage

# Individual test types with infrastructure
test-type-unit:
	@echo "🧪 Running unit tests with infrastructure..."
	@./scripts/run_complete_test_suite.sh --type unit --verbose

test-type-integration:
	@echo "🧪 Running integration tests with infrastructure..."
	@./scripts/run_complete_test_suite.sh --type integration --verbose

test-type-e2e:
	@echo "🧪 Running E2E tests with infrastructure..."
	@./scripts/run_complete_test_suite.sh --type e2e --verbose

test-type-security:
	@echo "🔒 Running security tests with infrastructure..."
	@./scripts/run_complete_test_suite.sh --type security --verbose

test-type-stress:
	@echo "⚡ Running stress tests with infrastructure..."
	@./scripts/run_complete_test_suite.sh --type stress --verbose

test-type-chaos:
	@echo "🌀 Running chaos tests with infrastructure..."
	@./scripts/run_complete_test_suite.sh --type chaos --verbose

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
	docker build -t helixagent:latest .

docker-build-prod:
	@echo "🐳 Building production Docker image..."
	docker build --target=production -t helixagent:prod .

docker-run:
	@echo "🐳 Starting HelixAgent with Docker..."
	docker compose up -d

docker-stop:
	@echo "🐳 Stopping HelixAgent..."
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
	docker compose -f docker-compose.test.yml exec helixagent go test ./...
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
	podman build -t helixagent:latest .

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
	@echo "📦 Installing HelixAgent..."
	mkdir -p /usr/local/bin
	cp bin/helixagent /usr/local/bin/
	@echo "✅ HelixAgent installed to /usr/local/bin/helixagent"

uninstall:
	@echo "🗑️ Uninstalling HelixAgent..."
	rm -f /usr/local/bin/helixagent
	@echo "✅ HelixAgent uninstalled"

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
	@echo "API documentation available at: http://localhost:7061/docs"

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
	@echo "🚀 HelixAgent Makefile Commands"
	@echo ""
	@echo "🔨 Build Commands:"
	@echo "  build              Build HelixAgent binary"
	@echo "  build-debug        Build HelixAgent binary (debug mode)"
	@echo "  build-all          Build for all architectures"
	@echo ""
	@echo "🏃 Run Commands:"
	@echo "  run                Run HelixAgent locally"
	@echo "  run-dev            Run HelixAgent in development mode"
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
	@echo "  install            Install HelixAgent to system"
	@echo "  uninstall          Remove HelixAgent from system"
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
# LLMSVERIFIER INTEGRATION TARGETS
# =============================================================================

verifier-init:
	@echo "🔍 Initializing LLMsVerifier submodule..."
	git submodule update --init --recursive LLMsVerifier
	@echo "✅ LLMsVerifier submodule initialized"

verifier-update:
	@echo "🔄 Updating LLMsVerifier submodule..."
	git submodule update --remote LLMsVerifier
	@echo "✅ LLMsVerifier submodule updated"

verifier-build:
	@echo "🔨 Building verifier components..."
	go build -o bin/verifier-cli ./LLMsVerifier/llm-verifier/cmd/...
	@echo "✅ Verifier CLI built to bin/verifier-cli"

verifier-test:
	@echo "🧪 Running verifier tests..."
	go test -v ./internal/verifier/... -cover
	@echo "✅ Verifier tests completed"

verifier-test-unit:
	@echo "🧪 Running verifier unit tests..."
	go test -v ./tests/unit/verifier/... -short
	@echo "✅ Verifier unit tests completed"

verifier-test-integration:
	@echo "🧪 Running verifier integration tests..."
	go test -v ./tests/integration/verifier/... -timeout 300s
	@echo "✅ Verifier integration tests completed"

verifier-test-e2e:
	@echo "🧪 Running verifier E2E tests..."
	go test -v ./tests/e2e/verifier/... -timeout 600s
	@echo "✅ Verifier E2E tests completed"

verifier-test-security:
	@echo "🔒 Running verifier security tests..."
	go test -v ./tests/security/verifier/...
	@echo "✅ Verifier security tests completed"

verifier-test-stress:
	@echo "⚡ Running verifier stress tests..."
	go test -v ./tests/stress/verifier/... -timeout 900s
	@echo "✅ Verifier stress tests completed"

verifier-test-chaos:
	@echo "🌀 Running verifier chaos tests..."
	go test -v ./tests/chaos/verifier/... -timeout 600s
	@echo "✅ Verifier chaos tests completed"

verifier-test-all:
	@echo "🧪 Running ALL verifier tests (6 types)..."
	@echo "1. Unit tests..."
	go test -v ./tests/unit/verifier/... -short
	@echo "2. Integration tests..."
	go test -v ./tests/integration/verifier/... -timeout 300s
	@echo "3. E2E tests..."
	go test -v ./tests/e2e/verifier/... -timeout 600s
	@echo "4. Security tests..."
	go test -v ./tests/security/verifier/...
	@echo "5. Stress tests..."
	go test -v ./tests/stress/verifier/... -timeout 900s
	@echo "6. Chaos tests..."
	go test -v ./tests/chaos/verifier/... -timeout 600s
	@echo "✅ All verifier tests completed"

verifier-test-coverage:
	@echo "📊 Running verifier tests with coverage..."
	go test -v -race -coverprofile=verifier-coverage.out ./internal/verifier/... ./tests/unit/verifier/...
	go tool cover -func=verifier-coverage.out
	go tool cover -html=verifier-coverage.out -o verifier-coverage.html
	@echo "📈 Verifier coverage report: verifier-coverage.html"

verifier-test-coverage-100:
	@echo "📊 Checking verifier 100% test coverage..."
	@go test -v -race -coverprofile=verifier-coverage.out ./internal/verifier/... ./tests/unit/verifier/...
	@coverage=$$(go tool cover -func=verifier-coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	if [ $$(echo "$$coverage < 100" | bc -l) -eq 1 ]; then \
		echo "❌ Verifier coverage is $$coverage%, required 100%"; \
		exit 1; \
	else \
		echo "✅ Verifier coverage is $$coverage%"; \
	fi

verifier-run:
	@echo "🚀 Running verifier service..."
	VERIFIER_ENABLED=true go run ./cmd/helixagent/main.go

verifier-health:
	@echo "💚 Checking verifier health..."
	curl -s http://localhost:8081/api/v1/verifier/health | jq .

verifier-verify:
	@echo "🔍 Running model verification..."
	@if [ -z "$(MODEL)" ]; then \
		echo "Usage: make verifier-verify MODEL=gpt-4 PROVIDER=openai"; \
		exit 1; \
	fi
	curl -s -X POST http://localhost:8081/api/v1/verifier/verify \
		-H "Content-Type: application/json" \
		-d '{"model_id":"$(MODEL)","provider":"$(PROVIDER)"}' | jq .

verifier-score:
	@echo "📊 Getting model score..."
	@if [ -z "$(MODEL)" ]; then \
		echo "Usage: make verifier-score MODEL=gpt-4"; \
		exit 1; \
	fi
	curl -s http://localhost:8081/api/v1/verifier/scores/$(MODEL) | jq .

verifier-providers:
	@echo "📋 Listing verified providers..."
	curl -s http://localhost:8081/api/v1/verifier/providers | jq .

verifier-metrics:
	@echo "📈 Getting verifier metrics..."
	curl -s http://localhost:8081/metrics/verifier

verifier-db-migrate:
	@echo "🗄️ Running verifier database migrations..."
	go run ./cmd/verifier-migrate/main.go
	@echo "✅ Verifier migrations completed"

verifier-db-sync:
	@echo "🔄 Syncing verifier database..."
	go run ./cmd/verifier-sync/main.go
	@echo "✅ Verifier database synced"

verifier-clean:
	@echo "🧹 Cleaning verifier artifacts..."
	rm -f bin/verifier-cli
	rm -f verifier-coverage.out verifier-coverage.html
	rm -f ./data/llm-verifier.db
	@echo "✅ Verifier artifacts cleaned"

verifier-docker-build:
	@echo "🐳 Building verifier Docker image..."
	docker build -t helixagent-verifier:latest -f Dockerfile.verifier .
	@echo "✅ Verifier Docker image built"

verifier-docker-run:
	@echo "🐳 Running verifier in Docker..."
	docker compose --profile verifier up -d
	@echo "✅ Verifier services started"

verifier-docker-stop:
	@echo "🐳 Stopping verifier Docker services..."
	docker compose --profile verifier down
	@echo "✅ Verifier services stopped"

verifier-sdk-go:
	@echo "📦 Building Go SDK for verifier..."
	cd pkg/sdk/go/verifier && go build ./...
	@echo "✅ Go SDK built"

verifier-sdk-python:
	@echo "🐍 Building Python SDK for verifier..."
	cd pkg/sdk/python/helixagent_verifier && pip install -e .
	@echo "✅ Python SDK installed"

verifier-sdk-js:
	@echo "📦 Building JavaScript SDK for verifier..."
	cd pkg/sdk/javascript/helixagent-verifier && npm install && npm run build
	@echo "✅ JavaScript SDK built"

verifier-sdk-all:
	@echo "📦 Building all verifier SDKs..."
	$(MAKE) verifier-sdk-go
	$(MAKE) verifier-sdk-python
	$(MAKE) verifier-sdk-js
	@echo "✅ All verifier SDKs built"

verifier-docs:
	@echo "📚 Generating verifier documentation..."
	@mkdir -p docs/verifier
	@echo "Generating API documentation..."
	@if command -v swag >/dev/null 2>&1; then \
		swag init -g internal/handlers/verification_handler.go -o docs/verifier/api; \
	else \
		echo "⚠️  swag not installed. Install with: go install github.com/swaggo/swag/cmd/swag@latest"; \
	fi
	@echo "✅ Verifier documentation generated"

verifier-benchmark:
	@echo "⚡ Running verifier benchmarks..."
	go test -bench=. -benchmem ./internal/verifier/...
	@echo "✅ Verifier benchmarks completed"

# =============================================================================
# PHONY TARGETS
# =============================================================================
.PHONY: all build build-debug build-all run run-dev test test-coverage test-unit test-integration test-bench test-race test-all test-with-infra test-infra-start test-infra-stop test-infra-clean test-infra-logs test-infra-status fmt vet lint security-scan docker-build docker-build-prod docker-run docker-stop docker-logs docker-clean docker-clean-all docker-test docker-dev docker-prod docker-full docker-monitoring docker-ai install-deps install uninstall clean clean-all check-deps update-deps generate docs docs-api setup-dev setup-prod help verifier-init verifier-update verifier-build verifier-test verifier-test-unit verifier-test-integration verifier-test-e2e verifier-test-security verifier-test-stress verifier-test-chaos verifier-test-all verifier-test-coverage verifier-test-coverage-100 verifier-run verifier-health verifier-verify verifier-score verifier-providers verifier-metrics verifier-db-migrate verifier-db-sync verifier-clean verifier-docker-build verifier-docker-run verifier-docker-stop verifier-sdk-go verifier-sdk-python verifier-sdk-js verifier-sdk-all verifier-docs verifier-benchmark
