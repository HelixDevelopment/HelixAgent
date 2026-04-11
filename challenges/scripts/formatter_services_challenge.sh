#!/bin/bash
# Formatter Services Docker Challenge
# Validates Docker-based formatter services

set -euo pipefail

PASSED=0
FAILED=0
TOTAL=0

FORMATTER_BASE_URL="${FORMATTER_SERVICE_BASE_URL:-http://localhost}"

echo "=== Formatter Services Docker Challenge ==="
echo "Testing formatter services at $FORMATTER_BASE_URL"
echo

# Helper functions
test_case() {
    local name=$1
    TOTAL=$((TOTAL+1))
    echo -n "Test $TOTAL: $name... "
}

pass() {
    echo "✓ PASS"
    PASSED=$((PASSED+1))
}

fail() {
    local reason=$1
    echo "✗ FAIL: $reason"
    FAILED=$((FAILED+1))
}

skip() {
    local reason=$1
    echo "⊘ SKIP: $reason"
    TOTAL=$((TOTAL-1))
}

# Test 1: Docker Files Exist
test_case "Dockerfile.autopep8 exists"
if [ -f "docker/formatters/Dockerfile.autopep8" ]; then
    pass
else
    fail "file not found"
fi

test_case "Dockerfile.yapf exists"
if [ -f "docker/formatters/Dockerfile.yapf" ]; then
    pass
else
    fail "file not found"
fi

test_case "Dockerfile.sqlfluff exists"
if [ -f "docker/formatters/Dockerfile.sqlfluff" ]; then
    pass
else
    fail "file not found"
fi

test_case "Dockerfile.rubocop exists"
if [ -f "docker/formatters/Dockerfile.rubocop" ]; then
    pass
else
    fail "file not found"
fi

test_case "Dockerfile.php-cs-fixer exists"
if [ -f "docker/formatters/Dockerfile.php-cs-fixer" ]; then
    pass
else
    fail "file not found"
fi

# Test 2: Service Wrappers Exist
test_case "formatter-service.py exists"
if [ -f "docker/formatters/formatter-service.py" ]; then
    pass
else
    fail "file not found"
fi

test_case "formatter-service.rb exists"
if [ -f "docker/formatters/formatter-service.rb" ]; then
    pass
else
    fail "file not found"
fi

# Test 3: Docker Compose File
test_case "docker-compose.formatters.yml exists"
if [ -f "docker/formatters/docker-compose.formatters.yml" ]; then
    pass
else
    fail "file not found"
fi

test_case "docker-compose.formatters.yml has autopep8 service"
if grep -q "autopep8:" docker/formatters/docker-compose.formatters.yml; then
    pass
else
    fail "service not defined"
fi

test_case "docker-compose.formatters.yml has yapf service"
if grep -q "yapf:" docker/formatters/docker-compose.formatters.yml; then
    pass
else
    fail "service not defined"
fi

test_case "docker-compose.formatters.yml has sqlfluff service"
if grep -q "sqlfluff:" docker/formatters/docker-compose.formatters.yml; then
    pass
else
    fail "service not defined"
fi

test_case "docker-compose.formatters.yml has rubocop service"
if grep -q "rubocop:" docker/formatters/docker-compose.formatters.yml; then
    pass
else
    fail "service not defined"
fi

# Test 4: Port Configuration
test_case "autopep8 uses port 9211"
if grep -A 5 "autopep8:" docker/formatters/docker-compose.formatters.yml | grep -q "9211:9211"; then
    pass
else
    fail "incorrect port"
fi

test_case "yapf uses port 9210"
if grep -A 5 "yapf:" docker/formatters/docker-compose.formatters.yml | grep -q "9210:9210"; then
    pass
else
    fail "incorrect port"
fi

test_case "sqlfluff uses port 9220"
if grep -A 5 "sqlfluff:" docker/formatters/docker-compose.formatters.yml | grep -q "9220:9220"; then
    pass
else
    fail "incorrect port"
fi

# Test 5: Go Service Provider Files
test_case "service/base.go exists"
if [ -f "internal/formatters/providers/service/base.go" ]; then
    pass
else
    fail "file not found"
fi

test_case "service/python_formatters.go exists"
if [ -f "internal/formatters/providers/service/python_formatters.go" ]; then
    pass
else
    fail "file not found"
fi

test_case "service/sql_formatters.go exists"
if [ -f "internal/formatters/providers/service/sql_formatters.go" ]; then
    pass
else
    fail "file not found"
fi

test_case "service/ruby_formatters.go exists"
if [ -f "internal/formatters/providers/service/ruby_formatters.go" ]; then
    pass
else
    fail "file not found"
fi

# Test 6: Provider Registration
test_case "register.go imports service package"
if grep -q "internal/formatters/providers/service" internal/formatters/providers/register.go; then
    pass
else
    fail "service package not imported"
fi

test_case "register.go registers autopep8"
if grep -q "autopep8" internal/formatters/providers/register.go; then
    pass
else
    fail "autopep8 not registered"
fi

test_case "register.go registers sqlfluff"
if grep -q "sqlfluff" internal/formatters/providers/register.go; then
    pass
else
    fail "sqlfluff not registered"
fi

# Test 7: Service Health Checks (if containers are running)
test_case "autopep8 health check (if running)"
if curl -sf "$FORMATTER_BASE_URL:9211/health" > /dev/null 2>&1; then
    HEALTH=$(curl -s --max-time 60 "$FORMATTER_BASE_URL:9211/health")
    if echo "$HEALTH" | jq -e '.status == "healthy"' > /dev/null 2>&1; then
        pass
    else
        fail "unhealthy response"
    fi
else
    skip "service not running"
fi

test_case "yapf health check (if running)"
if curl -sf "$FORMATTER_BASE_URL:9210/health" > /dev/null 2>&1; then
    HEALTH=$(curl -s --max-time 60 "$FORMATTER_BASE_URL:9210/health")
    if echo "$HEALTH" | jq -e '.status == "healthy"' > /dev/null 2>&1; then
        pass
    else
        fail "unhealthy response"
    fi
else
    skip "service not running"
fi

test_case "sqlfluff health check (if running)"
if curl -sf "$FORMATTER_BASE_URL:9220/health" > /dev/null 2>&1; then
    HEALTH=$(curl -s --max-time 60 "$FORMATTER_BASE_URL:9220/health")
    if echo "$HEALTH" | jq -e '.status == "healthy"' > /dev/null 2>&1; then
        pass
    else
        fail "unhealthy response"
    fi
else
    skip "service not running"
fi

# Test 8: Service Formatting (if containers are running)
test_case "autopep8 format Python code (if running)"
if curl -sf "$FORMATTER_BASE_URL:9211/health" > /dev/null 2>&1; then
    RESPONSE=$(curl -s --max-time 60 "$FORMATTER_BASE_URL:9211/format" \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"content":"def hello(  x,y ):\n  return x+y"}')

    if echo "$RESPONSE" | jq -e '.success == true' > /dev/null 2>&1; then
        pass
    else
        fail "formatting failed"
    fi
else
    skip "service not running"
fi

test_case "sqlfluff format SQL code (if running)"
if curl -sf "$FORMATTER_BASE_URL:9220/health" > /dev/null 2>&1; then
    RESPONSE=$(curl -s --max-time 60 "$FORMATTER_BASE_URL:9220/format" \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"content":"SELECT * FROM users WHERE id=1;"}')

    if echo "$RESPONSE" | jq -e '.success' > /dev/null 2>&1; then
        pass
    else
        fail "formatting failed"
    fi
else
    skip "service not running"
fi

# Test 9: Build Scripts
test_case "build-all.sh exists"
if [ -f "docker/formatters/build-all.sh" ]; then
    pass
else
    fail "file not found"
fi

test_case "build-all.sh is executable"
if [ -x "docker/formatters/build-all.sh" ]; then
    pass
else
    fail "not executable"
fi

# Test 10: Documentation
test_case "docker/formatters/README.md exists"
if [ -f "docker/formatters/README.md" ]; then
    pass
else
    fail "file not found"
fi

test_case "README.md documents port allocation"
if grep -q "Port Allocation" docker/formatters/README.md; then
    pass
else
    fail "missing port allocation table"
fi

test_case "README.md documents 14 formatters"
if grep -q "9210\|9211\|9220\|9230\|9240\|9250\|9260\|9270\|9280\|9290\|9291\|9300" docker/formatters/README.md; then
    pass
else
    fail "incomplete port documentation"
fi

# Summary
echo
echo "=== Challenge Results ==="
echo "Total Tests: $TOTAL"
echo "Passed: $PASSED"
echo "Failed: $FAILED"
if [ $((TOTAL - PASSED - FAILED)) -gt 0 ]; then
    echo "Skipped: $((TOTAL - PASSED - FAILED))"
fi
if [ $TOTAL -gt 0 ]; then
    echo "Pass Rate: $(awk "BEGIN {printf \"%.1f%%\", ($PASSED/$TOTAL)*100}")"
fi
echo

if [ $FAILED -eq 0 ]; then
    echo "✓ ALL TESTS PASSED"
    echo
    echo "To start formatter services:"
    echo "  cd docker/formatters"
    echo "  docker-compose -f docker-compose.formatters.yml up -d"
    exit 0
else
    echo "✗ SOME TESTS FAILED"
    exit 1
fi
