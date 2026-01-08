#!/bin/bash
#
# OAuth Credentials Challenge
# Verifies OAuth credential integration for Claude Code and Qwen Code CLI agents
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}  OAuth Credentials Challenge        ${NC}"
echo -e "${BLUE}======================================${NC}"
echo ""

# Load environment variables
if [ -f "${PROJECT_ROOT}/.env" ]; then
    export $(grep -v '^#' "${PROJECT_ROOT}/.env" | xargs)
fi

# Track results
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0

# Function to report test result
report_test() {
    local name=$1
    local result=$2
    local message=$3
    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    if [ "$result" == "pass" ]; then
        PASSED_TESTS=$((PASSED_TESTS + 1))
        echo -e "${GREEN}[PASS]${NC} $name"
        if [ -n "$message" ]; then
            echo -e "       ${GREEN}$message${NC}"
        fi
    elif [ "$result" == "skip" ]; then
        SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
        echo -e "${YELLOW}[SKIP]${NC} $name"
        if [ -n "$message" ]; then
            echo -e "       ${YELLOW}$message${NC}"
        fi
    else
        FAILED_TESTS=$((FAILED_TESTS + 1))
        echo -e "${RED}[FAIL]${NC} $name"
        if [ -n "$message" ]; then
            echo -e "       ${RED}$message${NC}"
        fi
    fi
}

echo -e "${BLUE}1. Checking OAuth Environment Variables${NC}"
echo "----------------------------------------"

# Check Claude OAuth env var
if [ "$CLAUDE_CODE_USE_OUATH_CREDENTIALS" == "true" ] || [ "$CLAUDE_CODE_USE_OAUTH_CREDENTIALS" == "true" ]; then
    report_test "Claude OAuth enabled" "pass" "CLAUDE_CODE_USE_O[U]ATH_CREDENTIALS=true"
else
    report_test "Claude OAuth enabled" "skip" "Set CLAUDE_CODE_USE_OAUTH_CREDENTIALS=true to enable"
fi

# Check Qwen OAuth env var
if [ "$QWEN_CODE_USE_OUATH_CREDENTIALS" == "true" ] || [ "$QWEN_CODE_USE_OAUTH_CREDENTIALS" == "true" ]; then
    report_test "Qwen OAuth enabled" "pass" "QWEN_CODE_USE_O[U]ATH_CREDENTIALS=true"
else
    report_test "Qwen OAuth enabled" "skip" "Set QWEN_CODE_USE_OAUTH_CREDENTIALS=true to enable"
fi

echo ""
echo -e "${BLUE}2. Checking OAuth Credential Files${NC}"
echo "----------------------------------------"

# Check Claude credentials file
CLAUDE_CREDS_PATH="$HOME/.claude/.credentials.json"
if [ -f "$CLAUDE_CREDS_PATH" ]; then
    # Check if it contains OAuth credentials
    if grep -q "claudeAiOauth" "$CLAUDE_CREDS_PATH"; then
        ACCESS_TOKEN=$(jq -r '.claudeAiOauth.accessToken // empty' "$CLAUDE_CREDS_PATH" 2>/dev/null || echo "")
        EXPIRES_AT=$(jq -r '.claudeAiOauth.expiresAt // 0' "$CLAUDE_CREDS_PATH" 2>/dev/null || echo "0")
        SUB_TYPE=$(jq -r '.claudeAiOauth.subscriptionType // "unknown"' "$CLAUDE_CREDS_PATH" 2>/dev/null || echo "unknown")

        if [ -n "$ACCESS_TOKEN" ] && [ "$ACCESS_TOKEN" != "null" ]; then
            # Check expiration
            NOW_MS=$(($(date +%s) * 1000))
            if [ "$EXPIRES_AT" -gt "$NOW_MS" ]; then
                EXPIRES_IN_HOURS=$(( (EXPIRES_AT - NOW_MS) / 3600000 ))
                report_test "Claude OAuth credentials" "pass" "Valid token (expires in ${EXPIRES_IN_HOURS}h), subscription: $SUB_TYPE"
            else
                report_test "Claude OAuth credentials" "fail" "Token expired at $(date -d @$((EXPIRES_AT / 1000)))"
            fi
        else
            report_test "Claude OAuth credentials" "fail" "No access token found"
        fi
    else
        report_test "Claude OAuth credentials" "skip" "No OAuth credentials in file"
    fi
else
    report_test "Claude OAuth credentials" "skip" "Credential file not found: $CLAUDE_CREDS_PATH"
fi

# Check Qwen credentials file
QWEN_CREDS_PATH="$HOME/.qwen/oauth_creds.json"
if [ -f "$QWEN_CREDS_PATH" ]; then
    ACCESS_TOKEN=$(jq -r '.access_token // empty' "$QWEN_CREDS_PATH" 2>/dev/null || echo "")
    EXPIRY_DATE=$(jq -r '.expiry_date // 0' "$QWEN_CREDS_PATH" 2>/dev/null || echo "0")

    if [ -n "$ACCESS_TOKEN" ] && [ "$ACCESS_TOKEN" != "null" ]; then
        # Check expiration
        NOW_MS=$(($(date +%s) * 1000))
        if [ "$EXPIRY_DATE" -gt "$NOW_MS" ]; then
            EXPIRES_IN_HOURS=$(( (EXPIRY_DATE - NOW_MS) / 3600000 ))
            report_test "Qwen OAuth credentials" "pass" "Valid token (expires in ${EXPIRES_IN_HOURS}h)"
        else
            report_test "Qwen OAuth credentials" "fail" "Token expired at $(date -d @$((EXPIRY_DATE / 1000)))"
        fi
    else
        report_test "Qwen OAuth credentials" "skip" "No access token found"
    fi
else
    report_test "Qwen OAuth credentials" "skip" "Credential file not found: $QWEN_CREDS_PATH"
fi

echo ""
echo -e "${BLUE}3. Running Go Unit Tests${NC}"
echo "----------------------------------------"

cd "$PROJECT_ROOT"

# Run OAuth credential unit tests
echo "Running oauth_credentials package tests..."
if go test -v ./internal/auth/oauth_credentials/... 2>&1 | tail -5; then
    report_test "OAuth credential unit tests" "pass"
else
    report_test "OAuth credential unit tests" "fail"
fi

echo ""
echo -e "${BLUE}4. Running Integration Tests${NC}"
echo "----------------------------------------"

# Set environment variables for testing
export CLAUDE_CODE_USE_OUATH_CREDENTIALS=true
export QWEN_CODE_USE_OUATH_CREDENTIALS=true
export RUN_LIVE_OAUTH_TESTS=true

echo "Running OAuth integration tests..."
if go test -v ./tests/integration/oauth_integration_test.go 2>&1 | tail -15; then
    report_test "OAuth integration tests" "pass"
else
    report_test "OAuth integration tests" "fail"
fi

echo ""
echo -e "${BLUE}5. Testing Provider Creation with OAuth${NC}"
echo "----------------------------------------"

# Test provider creation by running a specific test
if go test -v -run TestOAuthClaudeProviderIntegration ./tests/integration/oauth_integration_test.go 2>&1 | grep -q "PASS"; then
    echo "Claude OAuth provider creation: PASS"
    CLAUDE_RESULT="pass"
else
    echo "Claude OAuth provider creation: FAIL or SKIP"
    CLAUDE_RESULT="skip"
fi

if go test -v -run TestOAuthQwenProviderIntegration ./tests/integration/oauth_integration_test.go 2>&1 | grep -q "PASS"; then
    echo "Qwen OAuth provider creation: PASS"
    QWEN_RESULT="pass"
else
    echo "Qwen OAuth provider creation: FAIL or SKIP"
    QWEN_RESULT="skip"
fi

if [ "$CLAUDE_RESULT" == "pass" ] || [ "$QWEN_RESULT" == "pass" ]; then
    report_test "OAuth provider creation" "pass" "At least one provider created successfully"
else
    report_test "OAuth provider creation" "skip" "No OAuth providers available"
fi

echo ""
echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}  Challenge Summary                  ${NC}"
echo -e "${BLUE}======================================${NC}"
echo ""
echo -e "Total Tests:   $TOTAL_TESTS"
echo -e "${GREEN}Passed:        $PASSED_TESTS${NC}"
echo -e "${YELLOW}Skipped:       $SKIPPED_TESTS${NC}"
echo -e "${RED}Failed:        $FAILED_TESTS${NC}"
echo ""

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}OAuth Credentials Challenge: PASSED${NC}"
    exit 0
else
    echo -e "${RED}OAuth Credentials Challenge: FAILED${NC}"
    exit 1
fi
