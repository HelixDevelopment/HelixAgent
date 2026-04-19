.PHONY: all build build-legacy-memory test run fmt lint security-scan security-scan-all deps-scan secrets-scan gosec-baseline security-scan-snyk security-scan-sonarqube security-scan-trivy security-scan-gosec security-scan-go security-scan-stop security-scan-semgrep security-scan-kics security-scan-grype security-scan-container security-scan-iac security-report docker-build docker-run docker-stop docker-clean docker-logs docker-test docker-dev docker-prod coverage docker-clean-all install-deps help docs check-deps test-all test-all-docker container-detect container-build container-start container-stop container-logs container-status container-test podman-build podman-run podman-stop podman-logs podman-clean podman-full test-no-skip test-all-must-pass test-performance test-performance-bench test-challenges test-coverage-100 benchmark-baseline benchmark-check audit audit-json audit-coverage audit-todo audit-skip audit-deadcode audit-concurrency test-submodule-sync

EXCLUDE_DIRS := cli_agents MCP MCP-Servers

TEST_PACKAGES := ./cmd/... ./internal/... ./pkg/... ./tests/... ./challenges/...

# Resource limits per Constitution Rule 15 (30-40% host resources)
RESOURCE_PREFIX := nice -n 19 ionice -c 3
GO_TEST_FLAGS := -p 1
export GOMAXPROCS := 2

all: fmt vet lint test build

# =============================================================================
# BUILD TARGETS
# =============================================================================

build:
	@echo "🔨 Building HelixAgent..."
	go build -mod=mod -ldflags="-w -s" -o bin/helixagent ./cmd/helixagent

build-debug:
	@echo "🐛 Building HelixAgent (debug)..."
	go build -mod=mod -gcflags="all=-N -l" -o bin/helixagent-debug ./cmd/helixagent

build-legacy-memory:
	@echo "Building HelixAgent (legacy memory, no HelixMemory)..."
	go build -mod=mod -tags nohelixmemory -ldflags="-w -s" -o bin/helixagent ./cmd/helixagent

build-all:
	@echo "🔨 Building all architectures..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -ldflags="-w -s" -o bin/helixagent-linux-amd64 ./cmd/helixagent
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -mod=mod -ldflags="-w -s" -o bin/helixagent-linux-arm64 ./cmd/helixagent
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -mod=mod -ldflags="-w -s" -o bin/helixagent-darwin-amd64 ./cmd/helixagent
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -mod=mod -ldflags="-w -s" -o bin/helixagent-darwin-arm64 ./cmd/helixagent
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -mod=mod -ldflags="-w -s" -o bin/helixagent-windows-amd64.exe ./cmd/helixagent

# =============================================================================
# RELEASE BUILD TARGETS
# =============================================================================

release:
	@echo "📦 Building release for helixagent (all platforms)..."
	@./scripts/build/build-release.sh --app helixagent --all-platforms

release-all:
	@echo "📦 Building releases for ALL apps (all platforms)..."
	@./scripts/build/build-all-releases.sh

release-helixagent:
	@echo "📦 Building release: helixagent..."
	@./scripts/build/build-release.sh --app helixagent --all-platforms

release-api:
	@echo "📦 Building release: api..."
	@./scripts/build/build-release.sh --app api --all-platforms

release-grpc-server:
	@echo "📦 Building release: grpc-server..."
	@./scripts/build/build-release.sh --app grpc-server --all-platforms

release-cognee-mock:
	@echo "📦 Building release: cognee-mock..."
	@./scripts/build/build-release.sh --app cognee-mock --all-platforms

release-sanity-check:
	@echo "📦 Building release: sanity-check..."
	@./scripts/build/build-release.sh --app sanity-check --all-platforms

release-mcp-bridge:
	@echo "📦 Building release: mcp-bridge..."
	@./scripts/build/build-release.sh --app mcp-bridge --all-platforms

release-generate-constitution:
	@echo "📦 Building release: generate-constitution..."
	@./scripts/build/build-release.sh --app generate-constitution --all-platforms

release-force:
	@echo "📦 Force rebuilding ALL apps (all platforms)..."
	@./scripts/build/build-all-releases.sh --force

release-clean:
	@echo "🧹 Cleaning release artifacts (keeping version data)..."
	@for app in helixagent api grpc-server cognee-mock sanity-check mcp-bridge generate-constitution; do \
		rm -rf "releases/$$app/"; \
	done
	@echo "✅ Release artifacts cleaned"

release-clean-all:
	@echo "🧹 Cleaning ALL release data (artifacts + version tracking)..."
	@for app in helixagent api grpc-server cognee-mock sanity-check mcp-bridge generate-constitution; do \
		rm -rf "releases/$$app/"; \
	done
	@rm -f releases/.version-data/*.version-code releases/.version-data/*.last-hash
	@echo "✅ All release data cleaned"

release-info:
	@echo "📊 Release Build Status"
	@echo ""
	@for app in helixagent api grpc-server cognee-mock sanity-check mcp-bridge generate-constitution; do \
		vc="0"; hash="none"; \
		if [ -f "releases/.version-data/$$app.version-code" ]; then \
			vc=$$(cat "releases/.version-data/$$app.version-code"); \
		fi; \
		if [ -f "releases/.version-data/$$app.last-hash" ]; then \
			hash=$$(cat "releases/.version-data/$$app.last-hash" | head -c 16); \
		fi; \
		printf "  %-24s version-code: %-4s hash: %s\n" "$$app" "$$vc" "$$hash"; \
	done

release-builder-image:
	@echo "🐳 Building release builder container image..."
	@RUNTIME=$$(command -v docker 2>/dev/null || command -v podman 2>/dev/null); \
	if [ -z "$$RUNTIME" ]; then echo "❌ No container runtime found"; exit 1; fi; \
	$$RUNTIME build -f docker/build/Dockerfile.builder -t helixagent-builder:latest .
	@echo "✅ Builder image ready"

# =============================================================================
# CI/CD CONTAINER BUILD SYSTEM
# =============================================================================

.PHONY: ci-all ci-go ci-mobile ci-web ci-desktop ci-integration ci-report ci-build-images ci-clean

CI_COMPOSE_CMD = $(shell command -v docker >/dev/null 2>&1 && echo "docker compose" || echo "podman-compose")
CI_IS_DOCKER = $(shell command -v docker >/dev/null 2>&1 && echo "yes" || echo "no")

# Helper: run CI phase with proper compose flags
# Docker supports --abort-on-container-exit --exit-code-from
# Podman-compose needs: up -d, wait, inspect exit code, down
define run_ci_phase
	@if [ "$(CI_IS_DOCKER)" = "yes" ]; then \
		$(CI_COMPOSE_CMD) -f docker-compose.ci.yml --profile $(1) up --build --abort-on-container-exit --exit-code-from $(2); \
	else \
		$(CI_COMPOSE_CMD) -f docker-compose.ci.yml --profile $(1) build; \
		$(CI_COMPOSE_CMD) -f docker-compose.ci.yml --profile $(1) up -d; \
		echo "Waiting for $(2) to complete..."; \
		podman wait $(2) >/dev/null 2>&1 || true; \
		echo "Streaming logs from $(2):"; \
		podman logs $(2) 2>&1; \
		EXIT_CODE=$$(podman inspect --format '{{.State.ExitCode}}' $(2) 2>/dev/null || echo 1); \
		echo "$(2) exited with code $$EXIT_CODE"; \
		$(CI_COMPOSE_CMD) -f docker-compose.ci.yml --profile $(1) down 2>/dev/null || true; \
		exit $$EXIT_CODE; \
	fi
	@$(CI_COMPOSE_CMD) -f docker-compose.ci.yml --profile $(1) down 2>/dev/null || true
endef

ci-build-images: ## Build all CI container images
	@echo "Building CI container images..."
	$(CI_COMPOSE_CMD) -f docker-compose.ci.yml --profile go-ci --profile mobile-ci --profile web-ci --profile desktop-ci --profile integration --profile report build
	@echo "CI images ready"

ci-go: ## Phase 1: Go builds + tests + integration services
	@echo "=== Phase 1: Go CI ==="
	$(call run_ci_phase,go-ci,ci-go-builder)

ci-mobile: ## Phase 2: Mobile builds + Robolectric + emulator E2E
	@echo "=== Phase 2: Mobile CI ==="
	$(call run_ci_phase,mobile-ci,ci-mobile-builder)

ci-web: ## Phase 3: Web builds + Playwright + Lighthouse
	@echo "=== Phase 3: Web CI ==="
	$(call run_ci_phase,web-ci,ci-web-builder)

ci-desktop: ## Phase 4: Desktop builds (Electron/Tauri)
	@echo "=== Phase 4: Desktop CI ==="
	$(call run_ci_phase,desktop-ci,ci-desktop-builder)

ci-integration: ## Phase 5: Full‑stack integration tests
	@echo "=== Phase 5: Integration CI ==="
	$(call run_ci_phase,integration,ci-integration-runner)

ci-report: ## Aggregate reports from all phases
	@echo "=== CI Report Aggregation ==="
	$(call run_ci_phase,report,ci-reporter)

ci-all: ci-go ci-mobile ci-web ci-desktop ci-integration ci-report ## Run all CI phases + report
	@echo "=== CI Complete ==="
	@echo "Reports: reports/summary.html, reports/results.json"
	@echo "Releases: releases/"

ci-clean: ## Remove CI containers, networks, volumes
	$(CI_COMPOSE_CMD) -f docker-compose.ci.yml --profile go-ci --profile mobile-ci --profile web-ci --profile desktop-ci --profile integration --profile report down -v --remove-orphans 2>/dev/null || true
	@echo "CI cleanup complete"

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
	@if nc -z localhost $${POSTGRES_PORT:-15432} 2>/dev/null && nc -z localhost $${REDIS_PORT:-16379} 2>/dev/null; then \
		echo "✅ Infrastructure available - running full tests"; \
		DB_HOST=localhost DB_PORT=$${POSTGRES_PORT:-15432} DB_USER=helixagent DB_PASSWORD=helixagent123 DB_NAME=helixagent_db \
		DATABASE_URL="postgres://helixagent:helixagent123@localhost:$${POSTGRES_PORT:-15432}/helixagent_db?sslmode=disable" \
		REDIS_HOST=localhost REDIS_PORT=$${REDIS_PORT:-16379} REDIS_PASSWORD=helixagent123 \
		$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) $(TEST_PACKAGES); \
	else \
		echo "⚠️  Infrastructure not available - running unit tests only"; \
		echo "   Run './bin/helixagent' first for proper container orchestration"; \
		echo ""; \
		$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) -short $(TEST_PACKAGES); \
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
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "📈 Coverage report generated: coverage.html"

test-coverage-100:
	@echo "📊 Running tests with 100% coverage requirement..."
	@$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) -race -coverprofile=coverage.out ./...
	@coverage=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	if [ $$(echo "$$coverage < 100" | bc -l) -eq 1 ]; then \
		echo "❌ Coverage is $$coverage%, required 100%"; \
		exit 1; \
	else \
		echo "✅ Coverage is $$coverage%"; \
	fi
	go tool cover -html=coverage.out -o coverage.html

test-no-skip:
	@echo "🔍 Checking for unconditionally disabled tests..."
	@# Only flag unconditional t.Skip() calls at function start without conditions
	@SKIPPED=$$(grep -rn "^\s*t\.Skip\(\"" ./tests/ --include="*.go" 2>/dev/null | grep -v "short\|integration\|infra\|server\|provider\|env\|condition" | wc -l); \
	if [ "$$SKIPPED" -gt 0 ]; then \
		echo "❌ Found $$SKIPPED unconditionally skipped tests"; \
		grep -rn "^\s*t\.Skip\(\"" ./tests/ --include="*.go" 2>/dev/null | grep -v "short\|integration\|infra\|server\|provider\|env\|condition"; \
		exit 1; \
	fi
	@echo "✅ No unconditionally disabled tests found"

test-all-must-pass:
	@echo "🧪 Running all tests (none can fail)..."
	@$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) -failfast $(TEST_PACKAGES) 2>&1 | tee test_output.log; \
	if grep -q "FAIL" test_output.log; then \
		echo "❌ Some tests failed"; \
		rm -f test_output.log; \
		exit 1; \
	fi
	@rm -f test_output.log
	@echo "✅ All tests passed"

test-performance:
	@echo "🏃 Running performance tests..."
	@$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) -timeout 180s ./tests/unit/concurrency/...
	@$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) -timeout 180s ./tests/unit/events/...
	@$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) -timeout 180s ./tests/unit/cache/...
	@$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) -timeout 180s ./tests/unit/http/...
	@echo "✅ Performance tests completed"

test-performance-bench:
	@echo "📊 Running performance benchmarks..."
	@$(RESOURCE_PREFIX) go test $(GO_TEST_FLAGS) -bench=. -benchmem ./internal/concurrency/...
	@$(RESOURCE_PREFIX) go test $(GO_TEST_FLAGS) -bench=. -benchmem ./internal/events/...
	@$(RESOURCE_PREFIX) go test $(GO_TEST_FLAGS) -bench=. -benchmem ./internal/cache/...
	@echo "✅ Performance benchmarks completed"

test-challenges:
	@echo "🏆 Running performance challenges..."
	@./challenges/scripts/performance_baseline_challenge.sh
	@./challenges/scripts/parallel_execution_challenge.sh
	@./challenges/scripts/lazy_loading_challenge.sh
	@./challenges/scripts/event_driven_challenge.sh
	@./challenges/scripts/mcp_connectivity_challenge.sh
	@./challenges/scripts/stress_resilience_challenge.sh
	@echo "✅ All performance challenges passed"

test-unit:
	@echo "🧪 Running unit tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./internal/... -short

test-submodule-sync:
	@echo "🧪 Running submodule sync tests..."
	$(RESOURCE_PREFIX) go test -v -mod=mod $(GO_TEST_FLAGS) ./tests/integration/... -run "TestSubmoduleSync" -short

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
	@echo "⚠️  WARNING: Manual container startup is deprecated. Use './bin/helixagent' instead."
	@echo "🐳 Starting test infrastructure..."
	@./scripts/deploy-containers.sh docker-compose.test.yml postgres redis mock-llm

test-infra-stop:
	@echo "⚠️  WARNING: Manual container stop is deprecated. Use './bin/helixagent' and Ctrl-C instead."
	@echo "🐳 Stopping test infrastructure..."
	@source ./scripts/container-runtime.sh 2>/dev/null || true; \
	if [ -z "$$COMPOSE_CMD" ]; then \
		if command -v docker &> /dev/null && docker info &> /dev/null 2>&1; then \
			COMPOSE_CMD="docker compose"; \
		elif command -v podman-compose &> /dev/null; then \
			COMPOSE_CMD="podman-compose"; \
		else \
			COMPOSE_CMD="docker compose"; \
		fi; \
	fi; \
	$$COMPOSE_CMD -f docker-compose.test.yml down
	@echo "✅ Test infrastructure stopped"

test-infra-clean:
	@echo "🧹 Cleaning test infrastructure (including volumes)..."
	@source ./scripts/container-runtime.sh 2>/dev/null || true; \
	if [ -z "$$COMPOSE_CMD" ]; then \
		if command -v docker &> /dev/null && docker info &> /dev/null 2>&1; then \
			COMPOSE_CMD="docker compose"; \
		elif command -v podman-compose &> /dev/null; then \
			COMPOSE_CMD="podman-compose"; \
		else \
			COMPOSE_CMD="docker compose"; \
		fi; \
	fi; \
	$$COMPOSE_CMD -f docker-compose.test.yml down -v --remove-orphans
	@echo "✅ Test infrastructure cleaned"

test-infra-logs:
	@echo "📋 Showing test infrastructure logs..."
	@source ./scripts/container-runtime.sh 2>/dev/null || true; \
	if [ -z "$$COMPOSE_CMD" ]; then \
		if command -v docker &> /dev/null && docker info &> /dev/null 2>&1; then \
			COMPOSE_CMD="docker compose"; \
		elif command -v podman-compose &> /dev/null; then \
			COMPOSE_CMD="podman-compose"; \
		else \
			COMPOSE_CMD="docker compose"; \
		fi; \
	fi; \
	$$COMPOSE_CMD -f docker-compose.test.yml logs -f

test-infra-status:
	@echo "📊 Test infrastructure status:"
	@source ./scripts/container-runtime.sh 2>/dev/null || true; \
	if [ -z "$$COMPOSE_CMD" ]; then \
		if command -v docker &> /dev/null && docker info &> /dev/null 2>&1; then \
			COMPOSE_CMD="docker compose"; \
		elif command -v podman-compose &> /dev/null; then \
			COMPOSE_CMD="podman-compose"; \
		else \
			COMPOSE_CMD="docker compose"; \
		fi; \
	fi; \
	$$COMPOSE_CMD -f docker-compose.test.yml ps

# =============================================================================
# FULL TEST INFRASTRUCTURE (includes Kafka, RabbitMQ, MinIO, Iceberg, Qdrant)
# =============================================================================

test-infra-full-start:
	@echo "🐳 Starting FULL test infrastructure (all services)..."
	@./scripts/start-full-test-infra.sh all

test-infra-full-start-basic:
	@echo "🐳 Starting basic test infrastructure (PostgreSQL, Redis, Mock LLM)..."
	@./scripts/start-full-test-infra.sh basic

test-infra-full-start-messaging:
	@echo "🐳 Starting messaging test infrastructure (+ Kafka, RabbitMQ)..."
	@./scripts/start-full-test-infra.sh messaging

test-infra-full-start-bigdata:
	@echo "🐳 Starting bigdata test infrastructure (+ MinIO, Iceberg, Qdrant)..."
	@./scripts/start-full-test-infra.sh bigdata

test-infra-full-stop:
	@echo "🐳 Stopping FULL test infrastructure..."
	@./scripts/stop-full-test-infra.sh

test-with-full-infra:
	@echo "🧪 Running tests with FULL infrastructure..."
	@$(MAKE) test-infra-full-start
	@echo ""
	@echo "⏳ Waiting for all services to stabilize..."
	@sleep 5
	@source .env.test 2>/dev/null || true && \
		DB_HOST=localhost DB_PORT=$${POSTGRES_PORT:-15432} DB_USER=helixagent DB_PASSWORD=helixagent123 DB_NAME=helixagent_db \
		DATABASE_URL="postgres://helixagent:helixagent123@localhost:$${POSTGRES_PORT:-15432}/helixagent_db?sslmode=disable" \
		REDIS_HOST=localhost REDIS_PORT=$${REDIS_PORT:-16379} REDIS_PASSWORD=helixagent123 \
		REDIS_URL="redis://:helixagent123@localhost:$${REDIS_PORT:-16379}" \
		KAFKA_BROKERS=localhost:9092 KAFKA_BROKER=localhost:9092 \
		RABBITMQ_HOST=localhost RABBITMQ_PORT=5672 RABBITMQ_USER=helixagent RABBITMQ_PASSWORD=helixagent123 \
		RABBITMQ_URL="amqp://helixagent:helixagent123@localhost:5672/" \
		MINIO_ENDPOINT=localhost:9000 MINIO_ACCESS_KEY=minioadmin MINIO_SECRET_KEY=minioadmin123 MINIO_USE_SSL=false \
		ICEBERG_CATALOG_URI=http://localhost:8181 \
		QDRANT_HOST=localhost QDRANT_PORT=6333 \
		MOCK_LLM_URL=http://localhost:$${MOCK_LLM_PORT:-18081} MOCK_LLM_ENABLED=true \
		CLAUDE_API_KEY=mock-api-key CLAUDE_BASE_URL=http://localhost:$${MOCK_LLM_PORT:-18081}/v1 \
		DEEPSEEK_API_KEY=mock-api-key DEEPSEEK_BASE_URL=http://localhost:$${MOCK_LLM_PORT:-18081}/v1 \
		GEMINI_API_KEY=mock-api-key GEMINI_BASE_URL=http://localhost:$${MOCK_LLM_PORT:-18081}/v1 \
		QWEN_API_KEY=mock-api-key QWEN_BASE_URL=http://localhost:$${MOCK_LLM_PORT:-18081}/v1 \
		ZAI_API_KEY=mock-api-key ZAI_BASE_URL=http://localhost:$${MOCK_LLM_PORT:-18081}/v1 \
		OLLAMA_BASE_URL=http://localhost:$${MOCK_LLM_PORT:-18081} \
		JWT_SECRET=test-jwt-secret-key-for-testing \
		CI=true FULL_TEST_MODE=true \
		$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./... -timeout 600s -cover
	@echo ""
	@echo "✅ Tests completed with FULL infrastructure!"

test-integration-full:
	@echo "🧪 Running integration tests with FULL infrastructure..."
	@$(MAKE) test-infra-full-start
	@echo ""
	@source .env.test 2>/dev/null || true && \
		$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/integration/... -timeout 600s
	@echo ""
	@echo "✅ Integration tests completed!"

# =============================================================================
# Container Harness Integration Tests (Real Containers via Container Adapter)
# =============================================================================

test-integration-containers:
	@echo "🐳 Running integration tests with Container Harness..."
	@echo "   This will boot real containers via the Containers module adapter"
	@echo ""
	@$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/integration/... \
		-run "Container" \
		-tags=integration \
		-timeout 600s
	@echo ""
	@echo "✅ Container integration tests completed!"

test-integration-containers-short:
	@echo "🐳 Running container integration tests (short mode)..."
	@$(RESOURCE_PREFIX) go test $(GO_TEST_FLAGS) ./tests/integration/... \
		-run "Container" \
		-tags=integration \
		-timeout 300s
	@echo "✅ Container tests completed!"

test-with-infra:
	@echo "⚠️  WARNING: This target uses manual container startup. Use './bin/helixagent' for proper container orchestration."
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
		$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./... -timeout 300s -cover
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
		CI=true $(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./... -timeout 300s
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
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/integration

test-e2e:
	@echo "🧪 Running end-to-end tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/e2e

test-security:
	@echo "🔒 Running security tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/security

test-pentest:
	@echo "🔓 Running penetration tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) -tags pentest ./tests/pentest/...

test-performance-full:
	@echo "📊 Running performance benchmarks and load tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) -tags performance -bench=. -benchmem ./tests/performance/...

test-stress:
	@echo "⚡ Running stress tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/stress

test-chaos:
	@echo "🌀 Running chaos tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/challenge

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
	$(RESOURCE_PREFIX) go test $(GO_TEST_FLAGS) -bench=. -benchmem ./...

test-race:
	@echo "🏃 Running race condition tests..."
	$(RESOURCE_PREFIX) go test $(GO_TEST_FLAGS) -race ./...

test-automation:
	@echo "🤖 Running full automation test suite..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) -timeout 600s ./tests/automation/...

test-automation-verbose:
	@echo "🤖 Running full automation test suite (verbose)..."
	./scripts/run_full_automation.sh --verbose

test-automation-coverage:
	@echo "🤖 Running full automation test suite with coverage..."
	./scripts/run_full_automation.sh --verbose --coverage

# =============================================================================
# CODE QUALITY TARGETS
# =============================================================================

fmt:
	@echo "✨ Formatting code..."
	@for pkg in $$(go list ./... 2>/dev/null); do \
		case "$$pkg" in \
			*cli_agents*|*MCP*|*MCP-Servers*) \
				echo "Skipping $$pkg" >&2; \
				continue ;; \
			*) go fmt $$pkg ;; \
		esac; \
	done

vet:
	@echo "🔍 Running go vet..."
	@for pkg in $$(go list ./... 2>/dev/null); do \
		case "$$pkg" in \
			*cli_agents*|*MCP*|*MCP-Servers*) \
				echo "Skipping $$pkg" >&2; \
				continue ;; \
			*) go vet $$pkg ;; \
		esac; \
	done

lint:
	@echo "🔍 Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --enable errcheck,govet --tests=false --max-issues-per-linter=100 ./internal/...; \
	else \
		echo "⚠️  golangci-lint not installed. Install with: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin"; \
	fi

security-scan:
	@echo "🔒 Running comprehensive security scan (all scanners)..."
	@echo "📋 Note: This includes Gosec, Trivy, Snyk, and Go static analysis"
	@echo "📋 For SonarQube, use 'make security-scan-sonarqube'"
	@./scripts/security-scan.sh all

# Phase 4 additions — dedicated deps and secrets scan targets.
# These run non-interactively and never start containers or require sudo.
# Reports land in reports/security/ as timestamped markdown + SARIF.
deps-scan:
	@echo "🔍 Running dependency vulnerability scan (govulncheck)..."
	@./scripts/security/deps-scan.sh

secrets-scan:
	@echo "🔍 Running secrets scan (gitleaks)..."
	@./scripts/security/secrets-scan.sh

gosec-baseline:
	@echo "🔍 Regenerating gosec baseline (.gosec-baseline.json)..."
	@GOMAXPROCS=2 nice -n 19 gosec -quiet -fmt=json -out=.gosec-baseline.json ./... || true
	@echo "✓ Baseline refreshed — commit .gosec-baseline.json to lock current state."

security-scan-all:
	@echo "🔒 Running ALL security scanners (including SonarQube)..."
	@./scripts/security-scan.sh all
	@echo ""
	@echo "📋 Starting SonarQube server (this may take 2-3 minutes)..."
	@./scripts/security-scan.sh start-sonar
	@echo "📋 Running SonarQube analysis..."
	@./scripts/security-scan.sh sonarqube || { \
		echo "⚠️  SonarQube scan failed or timed out. Continuing..."; \
		echo "📋 You can run SonarQube separately with: make security-scan-sonarqube"; \
	}

security-scan-snyk:
	@echo "🔒 Running Snyk vulnerability scanner..."
	@./scripts/security-scan.sh snyk

security-scan-sonarqube:
	@echo "🔒 Running SonarQube code analysis..."
	@echo "📋 Starting SonarQube server (this may take 2-3 minutes)..."
	@./scripts/security-scan.sh start-sonar
	@echo "📋 Running SonarQube analysis..."
	@./scripts/security-scan.sh sonarqube

security-scan-trivy:
	@echo "🔒 Running Trivy vulnerability scanner..."
	@./scripts/security-scan.sh trivy

security-scan-gosec:
	@echo "🔒 Running Gosec Go security checker..."
	@./scripts/security-scan.sh gosec

security-scan-go:
	@echo "🔒 Running Go static analysis (vet, staticcheck)..."
	@./scripts/security-scan.sh go

security-scan-stop:
	@echo "🔒 Stopping security scanning services..."
	@./scripts/security-scan.sh stop

security-scan-semgrep:
	@echo "🔒 Running Semgrep pattern-based security scanner..."
	@mkdir -p reports/security
	@if command -v docker >/dev/null 2>&1; then \
		docker run --rm -v "$(PWD):/app:ro" -v "$(PWD)/reports/security:/reports" returntocorp/semgrep:latest \
			--config auto --json --output /reports/semgrep-report.json --metrics off /app; \
	elif command -v podman >/dev/null 2>&1; then \
		podman run --rm -v "$(PWD):/app:ro" -v "$(PWD)/reports/security:/reports" returntocorp/semgrep:latest \
			--config auto --json --output /reports/semgrep-report.json --metrics off /app; \
	else \
		echo "⚠️  Docker/Podman not found. Install Docker or Podman to use Semgrep."; \
	fi

security-scan-kics:
	@echo "🔒 Running KICS Infrastructure-as-Code scanner..."
	@mkdir -p reports/security
	@if command -v docker >/dev/null 2>&1; then \
		docker run --rm -v "$(PWD):/app:ro" -v "$(PWD)/reports/security:/reports" checkmarx/kics:latest \
			scan -p /app -o /reports --report-formats json --ignore-on-exit all --silent; \
	elif command -v podman >/dev/null 2>&1; then \
		podman run --rm -v "$(PWD):/app:ro" -v "$(PWD)/reports/security:/reports" checkmarx/kics:latest \
			scan -p /app -o /reports --report-formats json --ignore-on-exit all --silent; \
	else \
		echo "⚠️  Docker/Podman not found. Install Docker or Podman to use KICS."; \
	fi

security-scan-grype:
	@echo "🔒 Running Grype vulnerability scanner..."
	@mkdir -p reports/security
	@if command -v docker >/dev/null 2>&1; then \
		docker run --rm -v "$(PWD):/app:ro" -v "$(PWD)/reports/security:/reports" anchore/grype:latest \
			dir:/app -o json --file /reports/grype-report.json; \
	elif command -v podman >/dev/null 2>&1; then \
		podman run --rm -v "$(PWD):/app:ro" -v "$(PWD)/reports/security:/reports" anchore/grype:latest \
			dir:/app -o json --file /reports/grype-report.json; \
	else \
		echo "⚠️  Docker/Podman not found. Install Docker or Podman to use Grype."; \
	fi

security-scan-container:
	@echo "🔒 Scanning container images with Trivy..."
	@if command -v trivy >/dev/null 2>&1; then \
		trivy image --severity HIGH,CRITICAL helixagent:latest; \
	else \
		echo "⚠️  Trivy not found. Install Trivy to scan container images."; \
	fi

security-scan-iac:
	@echo "🔒 Scanning Infrastructure-as-Code files..."
	@$(MAKE) security-scan-kics

security-report:
	@echo "📊 Generating consolidated security report..."
	@mkdir -p reports/security
	@echo "# Security Scan Report - $$(date +%Y-%m-%d)" > reports/security/consolidated-report.md
	@echo "" >> reports/security/consolidated-report.md
	@echo "## Scan Summary" >> reports/security/consolidated-report.md
	@echo "" >> reports/security/consolidated-report.md
	@if [ -f reports/security/gosec-report.json ]; then \
		echo "- Gosec: $$(cat reports/security/gosec-report.json | jq '.Stats.found | length // 0' 2>/dev/null || echo 'N/A') issues"; \
	fi
	@if [ -f reports/security/trivy-report.json ]; then \
		echo "- Trivy: $$(cat reports/security/trivy-report.json | jq '.Results | length // 0' 2>/dev/null || echo 'N/A') findings"; \
	fi
	@if [ -f reports/security/semgrep-report.json ]; then \
		echo "- Semgrep: $$(cat reports/security/semgrep-report.json | jq '.results | length // 0' 2>/dev/null || echo 'N/A') findings"; \
	fi
	@echo "" >> reports/security/consolidated-report.md
	@echo "## Full Reports" >> reports/security/consolidated-report.md
	@echo "- Gosec: reports/security/gosec-report.json" >> reports/security/consolidated-report.md
	@echo "- Trivy: reports/security/trivy-report.json" >> reports/security/consolidated-report.md
	@echo "- Semgrep: reports/security/semgrep-report.json" >> reports/security/consolidated-report.md
	@echo "- KICS: reports/security/results.json" >> reports/security/consolidated-report.md
	@echo "- Grype: reports/security/grype-report.json" >> reports/security/consolidated-report.md
	@echo "✅ Report generated: reports/security/consolidated-report.md"

sbom:
	@echo "📋 Generating SBOM (Software Bill of Materials)..."
	@./scripts/generate-sbom.sh

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
# COMPREHENSIVE INFRASTRUCTURE AUTO-START TARGETS
# =============================================================================
# These targets ensure ALL HelixAgent infrastructure boots automatically
# Works with both Docker and Podman, auto-detects runtime

infra-start:
	@echo "🚀 Starting ALL HelixAgent infrastructure (auto-boot)..."
	@./scripts/ensure-infrastructure.sh start
	@echo "✅ All infrastructure started!"

infra-stop:
	@echo "⏹️ Stopping ALL HelixAgent infrastructure..."
	@./scripts/ensure-infrastructure.sh stop
	@echo "✅ All infrastructure stopped!"

infra-restart:
	@echo "🔄 Restarting ALL HelixAgent infrastructure..."
	@./scripts/ensure-infrastructure.sh restart

infra-status:
	@echo "📊 Checking ALL infrastructure status..."
	@./scripts/ensure-infrastructure.sh status

infra-core:
	@echo "🔧 Starting core services (PostgreSQL, Redis, ChromaDB, Cognee)..."
	@./scripts/ensure-infrastructure.sh core

infra-mcp:
	@echo "🔌 Starting MCP servers..."
	@./scripts/ensure-infrastructure.sh mcp

infra-lsp:
	@echo "📝 Starting LSP servers..."
	@./scripts/ensure-infrastructure.sh lsp

infra-rag:
	@echo "🔍 Starting RAG services..."
	@./scripts/ensure-infrastructure.sh rag

# Run tests with full infrastructure auto-start
test-with-full-auto:
	@echo "🧪 Running tests with FULL auto-started infrastructure..."
	@$(MAKE) infra-start
	@echo ""
	@echo "⏳ Waiting for all services to stabilize..."
	@sleep 10
	@DB_HOST=localhost DB_PORT=$${DB_PORT:-5432} DB_USER=$${DB_USER:-helixagent} DB_PASSWORD=$${DB_PASSWORD:-helixagent123} DB_NAME=$${DB_NAME:-helixagent_db} \
		DATABASE_URL="postgres://$${DB_USER:-helixagent}:$${DB_PASSWORD:-helixagent123}@localhost:$${DB_PORT:-5432}/$${DB_NAME:-helixagent_db}?sslmode=disable" \
		REDIS_HOST=localhost REDIS_PORT=$${REDIS_PORT:-6379} REDIS_PASSWORD=$${REDIS_PASSWORD:-helixagent123} \
		COGNEE_URL=http://localhost:8000 CHROMADB_URL=http://localhost:8001 \
		HELIXAGENT_URL=http://localhost:7061 \
		CI=true FULL_TEST_MODE=true \
		$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./... -timeout 900s -cover
	@echo ""
	@echo "✅ Tests completed with full infrastructure!"

# Run challenges with full infrastructure auto-start
challenges-with-infra:
	@echo "🏆 Running ALL challenges with full auto-started infrastructure..."
	@$(MAKE) infra-start
	@echo ""
	@echo "⏳ Waiting for all services to stabilize..."
	@sleep 10
	@./challenges/scripts/run_all_challenges.sh
	@echo ""
	@echo "✅ All challenges completed!"

# Comprehensive infrastructure challenge
challenge-infra:
	@echo "🏆 Running comprehensive infrastructure challenge..."
	@./challenges/scripts/comprehensive_infrastructure_challenge.sh

# All CLI agents E2E challenge
challenge-cli-agents:
	@echo "🏆 Running all CLI agents E2E challenge..."
	@$(MAKE) infra-start
	@sleep 10
	@./challenges/scripts/all_agents_e2e_challenge.sh

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

install-hooks:
	@echo "🔧 Installing pre-commit hooks..."
	@if command -v pre-commit >/dev/null 2>&1; then \
		echo "✅ pre-commit already installed"; \
	else \
		echo "📦 Installing pre-commit..."; \
		pip install pre-commit || pip3 install pre-commit; \
	fi
	@pre-commit install
	@echo "✅ Pre-commit hooks installed"

install-security-tools:
	@echo "🔧 Installing security scanning tools..."
	@if command -v gosec >/dev/null 2>&1; then \
		echo "✅ gosec already installed"; \
	else \
		echo "📦 Installing gosec..."; \
		go install github.com/securego/gosec/v2/cmd/gosec@latest; \
	fi
	@if command -v trivy >/dev/null 2>&1; then \
		echo "✅ trivy already installed"; \
	else \
		echo "📦 Installing trivy..."; \
		curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b $$(go env GOPATH)/bin; \
	fi
	@if command -v hadolint >/dev/null 2>&1; then \
		echo "✅ hadolint already installed"; \
	else \
		echo "📦 Installing hadolint..."; \
		$(CONTAINER_RUNTIME) pull hadolint/hadolint:latest; \
	fi
	@echo "✅ Security tools installed"

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
	@echo "  test-infra-start   Start test infrastructure (PostgreSQL, Redis, Mock LLM) [DEPRECATED: use ./bin/helixagent]"
	@echo "  test-infra-stop    Stop test infrastructure [DEPRECATED: use ./bin/helixagent]"
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
	@echo "🚀 Full Infrastructure Auto-Start (Docker/Podman):"
	@echo "  infra-start         Start ALL infrastructure (auto-detects runtime)"
	@echo "  infra-stop          Stop ALL infrastructure"
	@echo "  infra-restart       Restart ALL infrastructure"
	@echo "  infra-status        Check ALL infrastructure status"
	@echo "  infra-core          Start core services (PostgreSQL, Redis, ChromaDB, Cognee)"
	@echo "  infra-mcp           Start MCP servers"
	@echo "  infra-lsp           Start LSP servers"
	@echo "  infra-rag           Start RAG services"
	@echo "  test-with-full-auto Run tests with full auto-started infrastructure"
	@echo "  challenges-with-infra Run ALL challenges with full infrastructure"
	@echo "  challenge-infra     Run comprehensive infrastructure challenge"
	@echo "  challenge-cli-agents Run all CLI agents E2E challenge"
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
	@echo "📦 Release Build:"
	@echo "  release             Build helixagent for all platforms"
	@echo "  release-all         Build ALL apps for all platforms"
	@echo "  release-<app>       Build a specific app (helixagent, api, grpc-server, ...)"
	@echo "  release-force       Force rebuild ALL apps (ignore change detection)"
	@echo "  release-clean       Clean release artifacts (keep version data)"
	@echo "  release-clean-all   Clean all release data"
	@echo "  release-info        Show version codes and hashes"
	@echo "  release-builder-image  Build the release builder container image"
	@echo ""
	@echo "⚙️  Setup:"
	@echo "  setup-dev          Setup development environment"
	@echo "  setup-prod         Setup production environment"
	@echo "  help               Show this help message"

# =============================================================================
# CI/CD VALIDATION TARGETS (Prevention Measures)
# =============================================================================

ci-validate-fallback:
	@echo "🔍 CI/CD: Validating reliable fallback mechanism..."
	@./challenges/scripts/reliable_fallback_challenge.sh || { echo "❌ Fallback validation failed!"; exit 1; }
	@echo "✅ Fallback mechanism validated"

ci-validate-monitoring:
	@echo "🔍 CI/CD: Validating monitoring systems..."
	@$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) -run "TestCircuitBreakerMonitor|TestOAuthTokenMonitor|TestProviderHealthMonitor|TestFallbackChainValidator" ./internal/services/... || { echo "❌ Monitoring validation failed!"; exit 1; }
	@echo "✅ Monitoring systems validated"

# Constitution Validation
validate-constitution:
	@echo "📜 Validating Constitution structure..."
	@if [ ! -f CONSTITUTION.json ]; then \
		echo "❌ CONSTITUTION.json not found"; \
		exit 1; \
	fi
	@jq empty CONSTITUTION.json || (echo "❌ Invalid JSON in CONSTITUTION.json"; exit 1)
	@jq -e '.version' CONSTITUTION.json > /dev/null || (echo "❌ Missing version field"; exit 1)
	@jq -e '.rules | type == "array"' CONSTITUTION.json > /dev/null || (echo "❌ Missing or invalid rules array"; exit 1)
	@echo "✅ Constitution structure valid"

check-compliance:
	@echo "🔍 Checking Constitution compliance..."
	@MANDATORY_COUNT=$$(jq '[.rules[] | select(.mandatory == true)] | length' CONSTITUTION.json 2>/dev/null || echo "0"); \
	if [ "$$MANDATORY_COUNT" -lt 15 ]; then \
		echo "❌ Expected at least 15 mandatory rules, found: $$MANDATORY_COUNT"; \
		exit 1; \
	else \
		echo "✅ Found $$MANDATORY_COUNT mandatory rules (≥15)"; \
	fi
	@jq -r '.rules[].description' CONSTITUTION.json 2>/dev/null | grep -qi "100.*test.*coverage" || (echo "❌ 100% test coverage rule missing"; exit 1)
	@jq -r '.rules[].description' CONSTITUTION.json 2>/dev/null | grep -qi "decoupl" || (echo "❌ Decoupling rule missing"; exit 1)
	@jq -r '.rules[].description' CONSTITUTION.json 2>/dev/null | grep -Eqi "(manual.*only|no.*github.*actions)" || (echo "❌ Manual CI/CD rule missing"; exit 1)
	@echo "✅ All mandatory Constitution rules present"

sync-constitution:
	@echo "🔄 Checking Constitution synchronization..."
	@if [ ! -f CONSTITUTION.json ] || [ ! -f CONSTITUTION.md ]; then \
		echo "❌ Constitution files missing"; \
		exit 1; \
	fi
	@grep -q "BEGIN_CONSTITUTION" AGENTS.md || (echo "❌ Constitution section missing in AGENTS.md"; exit 1)
	@grep -q "BEGIN_CONSTITUTION" CLAUDE.md || (echo "❌ Constitution section missing in CLAUDE.md"; exit 1)
	@echo "✅ Constitution synchronized across all documentation"

ci-validate-constitution: validate-constitution check-compliance sync-constitution
	@echo "✅ Constitution validation passed"

ci-validate-all:
	@echo "🔍 CI/CD: Running all validation checks..."
	@$(MAKE) repo-health
	@$(MAKE) ci-validate-fallback
	@$(MAKE) ci-validate-monitoring
	@$(MAKE) ci-validate-constitution
	@$(MAKE) ci-validate-concurrency
	@echo "🔐 Running new P0/P1 regression gates..."
	@./challenges/scripts/repo_hygiene_challenge.sh
	@./challenges/scripts/tls_posture_challenge.sh
	@./challenges/scripts/exec_hygiene_challenge.sh
	@echo "✅ All CI/CD validations passed"

ci-validate-concurrency:
	@echo "🔍 CONST-029: auditing Pattern-A sites against allowlist..."
	@./scripts/concurrency-audit.sh
	@echo "✅ Concurrency audit passed"

ci-pre-commit:
	@echo "🔍 Pre-commit validation..."
	@$(MAKE) fmt
	@$(MAKE) vet
	@$(MAKE) ci-validate-fallback
	@$(RESOURCE_PREFIX) go test $(GO_TEST_FLAGS) -run "TestReliableAPIProvidersCollection|TestFallbackChainIncludesWorkingProviders" ./internal/services/...
	@echo "✅ Pre-commit validation passed"

ci-pre-push:
	@echo "🔍 Pre-push validation..."
	@$(MAKE) ci-pre-commit
	@$(MAKE) test-unit
	@$(MAKE) ci-validate-monitoring
	@echo "✅ Pre-push validation passed"

# Monitoring endpoints
monitoring-status:
	@echo "📊 Checking monitoring status..."
	@curl -s http://localhost:7061/v1/monitoring/status | jq .

monitoring-circuit-breakers:
	@echo "📊 Checking circuit breakers..."
	@curl -s http://localhost:7061/v1/monitoring/circuit-breakers | jq .

monitoring-oauth-tokens:
	@echo "📊 Checking OAuth tokens..."
	@curl -s http://localhost:7061/v1/monitoring/oauth-tokens | jq .

monitoring-provider-health:
	@echo "📊 Checking provider health..."
	@curl -s http://localhost:7061/v1/monitoring/provider-health | jq .

monitoring-fallback-chain:
	@echo "📊 Checking fallback chain..."
	@curl -s http://localhost:7061/v1/monitoring/fallback-chain | jq .

monitoring-reset-circuits:
	@echo "🔄 Resetting all circuit breakers..."
	@curl -s -X POST http://localhost:7061/v1/monitoring/circuit-breakers/reset-all | jq .
	@echo "✅ Circuit breakers reset"

monitoring-validate-fallback:
	@echo "🔍 Validating fallback chain..."
	@curl -s -X POST http://localhost:7061/v1/monitoring/fallback-chain/validate | jq .

monitoring-force-health-check:
	@echo "🔍 Forcing provider health check..."
	@curl -s -X POST http://localhost:7061/v1/monitoring/provider-health/check | jq .

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
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./internal/verifier/... -cover
	@echo "✅ Verifier tests completed"

verifier-test-unit:
	@echo "🧪 Running verifier unit tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/unit/verifier/... -short
	@echo "✅ Verifier unit tests completed"

verifier-test-integration:
	@echo "🧪 Running verifier integration tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/integration/verifier/... -timeout 300s
	@echo "✅ Verifier integration tests completed"

verifier-test-e2e:
	@echo "🧪 Running verifier E2E tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/e2e/verifier/... -timeout 600s
	@echo "✅ Verifier E2E tests completed"

verifier-test-security:
	@echo "🔒 Running verifier security tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/security/verifier/...
	@echo "✅ Verifier security tests completed"

verifier-test-stress:
	@echo "⚡ Running verifier stress tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/stress/verifier/... -timeout 900s
	@echo "✅ Verifier stress tests completed"

verifier-test-chaos:
	@echo "🌀 Running verifier chaos tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/chaos/verifier/... -timeout 600s
	@echo "✅ Verifier chaos tests completed"

verifier-test-all:
	@echo "🧪 Running ALL verifier tests (6 types)..."
	@echo "1. Unit tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/unit/verifier/... -short
	@echo "2. Integration tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/integration/verifier/... -timeout 300s
	@echo "3. E2E tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/e2e/verifier/... -timeout 600s
	@echo "4. Security tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/security/verifier/...
	@echo "5. Stress tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/stress/verifier/... -timeout 900s
	@echo "6. Chaos tests..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) ./tests/chaos/verifier/... -timeout 600s
	@echo "✅ All verifier tests completed"

verifier-test-coverage:
	@echo "📊 Running verifier tests with coverage..."
	$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) -race -coverprofile=verifier-coverage.out ./internal/verifier/... ./tests/unit/verifier/...
	go tool cover -func=verifier-coverage.out
	go tool cover -html=verifier-coverage.out -o verifier-coverage.html
	@echo "📈 Verifier coverage report: verifier-coverage.html"

verifier-test-coverage-100:
	@echo "📊 Checking verifier 100% test coverage..."
	@$(RESOURCE_PREFIX) go test -v $(GO_TEST_FLAGS) -race -coverprofile=verifier-coverage.out ./internal/verifier/... ./tests/unit/verifier/...
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
	$(RESOURCE_PREFIX) go test $(GO_TEST_FLAGS) -bench=. -benchmem ./internal/verifier/...
	@echo "✅ Verifier benchmarks completed"

# =============================================================================
# PHONY TARGETS
# =============================================================================
.PHONY: all build build-debug build-all run run-dev test test-coverage test-unit test-integration test-bench test-race test-all test-with-infra test-infra-start test-infra-stop test-infra-clean test-infra-logs test-infra-status fmt vet lint security-scan security-scan-all security-scan-snyk security-scan-sonarqube security-scan-trivy security-scan-gosec security-scan-go security-scan-stop docker-build docker-build-prod docker-run docker-stop docker-logs docker-clean docker-clean-all docker-test docker-dev docker-prod docker-full docker-monitoring docker-ai install-deps install uninstall clean clean-all check-deps update-deps generate docs docs-api setup-dev setup-prod help test-pentest test-security test-stress test-chaos test-e2e verifier-init verifier-update verifier-build verifier-test verifier-test-unit verifier-test-integration verifier-test-e2e verifier-test-security verifier-test-stress verifier-test-chaos verifier-test-all verifier-test-coverage verifier-test-coverage-100 verifier-run verifier-health verifier-verify verifier-score verifier-providers verifier-metrics verifier-db-migrate verifier-db-sync verifier-clean verifier-docker-build verifier-docker-run verifier-docker-stop verifier-sdk-go verifier-sdk-python verifier-sdk-js verifier-sdk-all verifier-docs verifier-benchmark infra-start infra-stop infra-restart infra-status infra-core infra-mcp infra-lsp infra-rag test-with-full-auto challenges-with-infra challenge-infra challenge-cli-agents release release-all release-helixagent release-api release-grpc-server release-cognee-mock release-sanity-check release-mcp-bridge release-generate-constitution release-force release-clean release-clean-all release-info release-builder-image

# =============================================================================
# Provider Testing Targets
# =============================================================================

## Run all provider tests
.PHONY: test-providers
test-providers:
	@echo "Running comprehensive provider tests..."
	@GOMAXPROCS=2 go test -v ./tests/providers/... -timeout 2h

## Run tests for specific provider
.PHONY: test-provider
test-provider:
	@echo "Running tests for provider: $(PROVIDER)"
	@GOMAXPROCS=2 go test -v ./tests/providers/... -run "Test.*$(PROVIDER)" -timeout 30m

## Run tests for specific model
.PHONY: test-model
test-model:
	@echo "Running tests for model: $(MODEL)"
	@GOMAXPROCS=2 go test -v ./tests/providers/... -run "Test.*/$(MODEL)" -timeout 30m

## Run tests for specific category
.PHONY: test-category
test-category:
	@echo "Running tests for category: $(CATEGORY)"
	@GOMAXPROCS=2 go test -v ./tests/providers/... -run "Test$(CATEGORY)" -timeout 30m

# =============================================================================
# Challenge Targets
# =============================================================================

## Run all challenges
## Run the Go-based challenge bank under ./tests/challenges/
## (distinct from `make test-challenges`, which runs the shell-script
## performance challenges under ./challenges/scripts/).
.PHONY: test-challenges-go
test-challenges-go:
	@echo "Running Go challenge bank (./tests/challenges/)..."
	@GOMAXPROCS=2 go test -v ./tests/challenges/... -timeout 4h

## Run challenges for specific provider
.PHONY: test-challenges-provider
test-challenges-provider:
	@echo "Running challenges for provider: $(PROVIDER)"
	@GOMAXPROCS=2 go test -v ./tests/challenges/... -run "TestChallenges/$(PROVIDER)" -timeout 2h

## Run challenges for specific category
.PHONY: test-challenges-category
test-challenges-category:
	@echo "Running challenges for category: $(CATEGORY)"
	@GOMAXPROCS=2 go test -v ./tests/challenges/... -run "TestChallenges/.*/$(CATEGORY)" -timeout 2h

## Run challenges for specific difficulty
.PHONY: test-challenges-difficulty
test-challenges-difficulty:
	@echo "Running challenges for difficulty: $(DIFFICULTY)"
	@GOMAXPROCS=2 go test -v ./tests/challenges/... -difficulty=$(DIFFICULTY) -timeout 2h

# =============================================================================
# Benchmark Targets
# =============================================================================

## Run all benchmarks
.PHONY: test-benchmarks
test-benchmarks:
	@echo "Running all benchmarks..."
	@GOMAXPROCS=2 go test -bench=. -benchmem ./tests/benchmarks/... -timeout 4h

## Run benchmarks for specific provider
.PHONY: test-benchmarks-provider
test-benchmarks-provider:
	@echo "Running benchmarks for provider: $(PROVIDER)"
	@GOMAXPROCS=2 go test -bench=. -benchmem ./tests/benchmarks/... -run "$(PROVIDER)" -timeout 2h

## Run specific benchmark test
.PHONY: test-benchmark
test-benchmark:
	@echo "Running benchmark: $(BENCHMARK)"
	@GOMAXPROCS=2 go test -bench=$(BENCHMARK) -benchmem ./tests/benchmarks/... -timeout 1h

## Capture performance baselines to golden files
.PHONY: benchmark-baseline
benchmark-baseline:
	@bash scripts/benchmark-baseline.sh

## Check benchmarks against baselines
.PHONY: benchmark-check
benchmark-check:
	@echo "Running benchmarks and comparing against baselines..."
	@GOMAXPROCS=2 nice -n 19 go test -bench=. -benchmem -count=3 \
		-run=^$$ ./internal/handlers/ -p 1 -timeout=300s > /tmp/bench-current-handlers.txt 2>/dev/null
	@echo "Handler benchmarks captured. Compare with: benchstat benchmarks/baselines/handlers.txt /tmp/bench-current-handlers.txt"

# =============================================================================
# Comprehensive Test Targets
# =============================================================================

## Run ALL tests (providers, challenges, benchmarks)
.PHONY: test-all-providers
test-all-providers: test-providers test-challenges test-benchmarks
	@echo "All provider tests completed!"

## Generate comprehensive test report
.PHONY: test-report
test-report:
	@echo "Generating comprehensive test report..."
	@mkdir -p test-reports
	@go test -v ./tests/... -json > test-reports/results.jsonl 2>&1 || true
	@echo "Report saved to test-reports/results.jsonl"

## Run tests with coverage
.PHONY: test-coverage-providers
test-coverage-providers:
	@echo "Running provider tests with coverage..."
	@go test -coverprofile=test-reports/providers.coverage.out ./tests/providers/... -timeout 2h
	@go tool cover -html=test-reports/providers.coverage.out -o test-reports/providers-coverage.html

# =============================================================================
# Sub-Agent Testing
# =============================================================================

## Test sub-agent system
.PHONY: test-subagents
test-subagents:
	@echo "Testing sub-agent system..."
	@go test -v ./internal/agents/subagent/... -timeout 30m

## Test specific sub-agent
.PHONY: test-subagent
test-subagent:
	@echo "Testing sub-agent: $(AGENT)"
	@go test -v ./internal/agents/subagent/... -run "Test$(AGENT)" -timeout 15m

# =============================================================================
# MCP Testing
# =============================================================================

## Test MCP client
.PHONY: test-mcp
test-mcp:
	@echo "Testing MCP client..."
	@go test -v ./internal/mcp/... -timeout 30m

## Test MCP with specific server
.PHONY: test-mcp-server
test-mcp-server:
	@echo "Testing MCP server: $(SERVER)"
	@go test -v ./internal/mcp/... -run "Test$(SERVER)" -timeout 15m

# =============================================================================
# Search Testing
# =============================================================================

## Test search providers
.PHONY: test-search
test-search:
	@echo "Testing search providers..."
	@go test -v ./internal/search/... -timeout 30m

## Test specific search provider
.PHONY: test-search-provider
test-search-provider:
	@echo "Testing search provider: $(PROVIDER)"
	@go test -v ./internal/search/... -run "Test$(PROVIDER)" -timeout 15m

# =============================================================================
# Codebase Testing
# =============================================================================

## Test codebase indexing
.PHONY: test-codebase
test-codebase:
	@echo "Testing codebase indexing..."
	@go test -v ./internal/codebase/... -timeout 30m

# =============================================================================
# Integration Testing
# =============================================================================

## Run full integration tests
## Run the repo-wide long-form integration sweep (6h budget). Distinct
## from `make test-integration-full`, which runs the targeted
## tests/integration/ suite against the FULL infrastructure (600s).
.PHONY: test-integration-long
test-integration-long:
	@echo "Running long-form integration tests (6h budget)..."
	@GOMAXPROCS=2 nice -n 19 go test -v ./... -run "Integration" -timeout 6h

## NOTE: `test-smoke` was removed in the 2026-04-11 dead-code sweep —
## it referenced tests/providers/ which was a stranded leftover from
## an earlier API refactor (see commit 2b90d313 "refactor: remove
## dead code and tests that reference removed APIs"). A replacement
## smoke-test target against the current provider registry can be
## added here if needed.

# =============================================================================
# AUDIT TARGETS
# =============================================================================

## Run full codebase audit (markdown report)
audit:
	@echo "Running full codebase audit..."
	@mkdir -p reports/audit
	go run ./cmd/audit --root . --output reports/audit/full-report.md

## Run full codebase audit (JSON report)
audit-json:
	@echo "Running full codebase audit (JSON)..."
	@mkdir -p reports/audit
	go run ./cmd/audit --root . --output reports/audit/full-report.json --format json

## Scan for coverage gaps only
audit-coverage:
	@echo "Scanning for coverage gaps..."
	go run ./cmd/audit --root . --scanners coverage

## Scan for TODO/FIXME markers only
audit-todo:
	@echo "Scanning for TODO/FIXME markers..."
	go run ./cmd/audit --root . --scanners todo

## Scan for skipped tests only
audit-skip:
	@echo "Scanning for skipped tests..."
	go run ./cmd/audit --root . --scanners skip

## Scan for dead code only
audit-deadcode:
	@echo "Scanning for dead code..."
	go run ./cmd/audit --root . --scanners deadcode

## Scan for concurrency hazards only
audit-concurrency:
	@echo "Scanning for concurrency hazards..."
	go run ./cmd/audit --root . --scanners concurrency

# =============================================================================
# P0 FOUNDATION TARGETS (2026-04-18)
# =============================================================================
# Scaffolded by the phased plan in
# docs/reports/2026-04-18-unfinished-work-report-and-phased-plan.md.
# All targets are non-interactive (CONST-019) and resource-capped (CONST-022).

## Run read-only repo health sanity checks
repo-health:
	@./scripts/repo-health.sh

## Aggregate per-module coverage floor gate (runs make coverage-local in each)
coverage-floor:
	@echo "🎯 Running per-module coverage floor (90% lines, 85% branches)..."
	@./scripts/coverage-floor.sh

## Capture baseline metrics snapshot for a phase-closure report
metrics-snapshot:
	@echo "📊 Capturing metrics snapshot..."
	@mkdir -p reports/metrics-snapshots
	@./scripts/metrics-snapshot.sh "reports/metrics-snapshots/$$(date -u +%Y-%m-%dT%H%M%SZ)"

## Run every security gate (gosec + govulncheck + gitleaks + compose scanners)
security-gates-all: security-scan-gosec deps-scan secrets-scan security-scan-trivy
	@echo "✓ security gates complete — reports under reports/security/"

.PHONY: repo-health coverage-floor metrics-snapshot security-gates-all

# =============================================================================
# FULL TEST MATRIX (2026-04-19)
# =============================================================================
# Chains every self-contained test + gate that does NOT require operator
# action (no API keys, no container daemon, no running HelixAgent binary).
#
# Operator-gated items documented inline — they MUST be run separately
# by the operator. Goal: `make full-test-matrix` is a SAFE CI smoke
# target that can run unattended.
#
# Exit codes:
#   0   — every self-contained step passed
#   !=0 — at least one step failed; summary table printed below

## Run every self-contained test + gate — fast to complete, safe to run
## unattended. Operator-gated items (integration tests, release builds,
## HelixQA autonomous sessions) are explicitly NOT in this chain.
full-test-matrix:
	@echo "╔══════════════════════════════════════════════════════════════════╗"
	@echo "║                    FULL TEST MATRIX                              ║"
	@echo "║  Self-contained tests + gates only. Operator-gated steps are     ║"
	@echo "║  documented inline; they are NOT run here.                        ║"
	@echo "╚══════════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "▶ Step 1/8 — fmt + vet"
	@$(MAKE) fmt vet
	@echo ""
	@echo "▶ Step 2/8 — repo-health (7 sanity checks)"
	@$(MAKE) repo-health
	@echo ""
	@echo "▶ Step 3/8 — P0 hygiene challenge (9 assertions)"
	@./challenges/scripts/repo_hygiene_challenge.sh
	@echo ""
	@echo "▶ Step 4/8 — P1 TLS posture challenge (3 assertions)"
	@./challenges/scripts/tls_posture_challenge.sh
	@echo ""
	@echo "▶ Step 5/8 — P1 exec-site hygiene challenge (2 assertions + warnings)"
	@./challenges/scripts/exec_hygiene_challenge.sh
	@echo ""
	@echo "▶ Step 6/8 — unit tests (-short, 265 packages)"
	@$(MAKE) test-unit
	@echo ""
	@echo "▶ Step 7/8 — dependency CVE scan (govulncheck)"
	@$(MAKE) deps-scan
	@echo ""
	@echo "▶ Step 8/8 — metrics snapshot (baseline capture for phase reports)"
	@$(MAKE) metrics-snapshot
	@echo ""
	@echo "╔══════════════════════════════════════════════════════════════════╗"
	@echo "║                ✓ FULL TEST MATRIX COMPLETE                      ║"
	@echo "╚══════════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "Operator-gated next steps (NOT run by this target):"
	@echo "  • Integration tests:        make test-with-infra      (requires running HelixAgent binary)"
	@echo "  • Race detection (8 pkg):   go test -race ./internal/... -short -p 1  (known debt — see docs/reports/2026-04-19-adbcd-execution-log.md § A4)"
	@echo "  • Lint (163 warnings):      make lint                 (dedicated lint-hygiene track)"
	@echo "  • Release build (all apps): make release-all          (container-based, ~hours)"
	@echo "  • Full challenge suite:     ./challenges/scripts/run_all_challenges.sh  (requires running HelixAgent binary)"
	@echo "  • Boot HelixAgent:          ./bin/helixagent          (auto-orchestrates ALL containers per Constitution)"
	@echo "  • HelixQA autonomous:       helixqa run --autonomous  (requires vision-model backend)"

.PHONY: full-test-matrix
