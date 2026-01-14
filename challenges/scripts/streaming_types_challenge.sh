#!/bin/bash

# ============================================================================
# Streaming Types Challenge Script
# ============================================================================
# Validates comprehensive support for ALL streaming types in HelixAgent:
# - SSE (Server-Sent Events)
# - WebSocket
# - AsyncGenerator
# - JSONL (JSON Lines)
# - MpscStream (Multi-Producer Single-Consumer)
# - EventStream (AWS format)
# - Stdout
#
# HelixAgent MUST support ALL streaming mechanisms to ensure compatibility
# with every CLI agent (OpenCode, ClaudeCode, KiloCode, etc.)
# ============================================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Project root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STREAMING_PKG="$PROJECT_ROOT/internal/streaming"
TESTS_DIR="$PROJECT_ROOT/tests/integration"

# Log functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Test function
run_test() {
    local test_name="$1"
    local test_cmd="$2"

    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo -n "  [$TOTAL_TESTS] $test_name... "

    if eval "$test_cmd" > /dev/null 2>&1; then
        echo -e "${GREEN}PASS${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
}

echo ""
echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║         STREAMING TYPES CHALLENGE - 50 TESTS                     ║"
echo "║   Validates ALL streaming mechanisms for CLI agent compatibility  ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo ""

# ============================================================================
# Section 1: Package Structure Validation (5 tests)
# ============================================================================
echo "═══════════════════════════════════════════════════════════════════"
echo "Section 1: Package Structure Validation"
echo "═══════════════════════════════════════════════════════════════════"

run_test "Streaming package directory exists" \
    "[ -d '$STREAMING_PKG' ]"

run_test "types.go exists" \
    "[ -f '$STREAMING_PKG/types.go' ]"

run_test "types_test.go exists" \
    "[ -f '$STREAMING_PKG/types_test.go' ]"

run_test "Package name is correct" \
    "grep -q 'package streaming' '$STREAMING_PKG/types.go'"

run_test "Integration tests exist" \
    "[ -f '$TESTS_DIR/streaming_types_integration_test.go' ]"

# ============================================================================
# Section 2: Streaming Type Constants (8 tests)
# ============================================================================
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "Section 2: Streaming Type Constants"
echo "═══════════════════════════════════════════════════════════════════"

run_test "StreamingType type defined" \
    "grep -q 'type StreamingType string' '$STREAMING_PKG/types.go'"

run_test "SSE constant defined" \
    "grep -q 'StreamingTypeSSE.*\"sse\"' '$STREAMING_PKG/types.go'"

run_test "WebSocket constant defined" \
    "grep -q 'StreamingTypeWebSocket.*\"websocket\"' '$STREAMING_PKG/types.go'"

run_test "AsyncGenerator constant defined" \
    "grep -q 'StreamingTypeAsyncGen.*\"async_generator\"' '$STREAMING_PKG/types.go'"

run_test "JSONL constant defined" \
    "grep -q 'StreamingTypeJSONL.*\"jsonl\"' '$STREAMING_PKG/types.go'"

run_test "MpscStream constant defined" \
    "grep -q 'StreamingTypeMpscStream.*\"mpsc_stream\"' '$STREAMING_PKG/types.go'"

run_test "EventStream constant defined" \
    "grep -q 'StreamingTypeEventStream.*\"event_stream\"' '$STREAMING_PKG/types.go'"

run_test "Stdout constant defined" \
    "grep -q 'StreamingTypeStdout.*\"stdout\"' '$STREAMING_PKG/types.go'"

# ============================================================================
# Section 3: SSE Implementation (6 tests)
# ============================================================================
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "Section 3: SSE (Server-Sent Events) Implementation"
echo "═══════════════════════════════════════════════════════════════════"

run_test "SSEWriter struct defined" \
    "grep -q 'type SSEWriter struct' '$STREAMING_PKG/types.go'"

run_test "NewSSEWriter function defined" \
    "grep -q 'func NewSSEWriter' '$STREAMING_PKG/types.go'"

run_test "WriteEvent method defined" \
    "grep -q 'func (s \*SSEWriter) WriteEvent' '$STREAMING_PKG/types.go'"

run_test "WriteData method defined" \
    "grep -q 'func (s \*SSEWriter) WriteData' '$STREAMING_PKG/types.go'"

run_test "WriteDone method defined" \
    "grep -q 'func (s \*SSEWriter) WriteDone' '$STREAMING_PKG/types.go'"

run_test "WriteHeartbeat method defined" \
    "grep -q 'func (s \*SSEWriter) WriteHeartbeat' '$STREAMING_PKG/types.go'"

# ============================================================================
# Section 4: WebSocket Implementation (5 tests)
# ============================================================================
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "Section 4: WebSocket Implementation"
echo "═══════════════════════════════════════════════════════════════════"

run_test "WebSocketWriter struct defined" \
    "grep -q 'type WebSocketWriter struct' '$STREAMING_PKG/types.go'"

run_test "NewWebSocketWriter function defined" \
    "grep -q 'func NewWebSocketWriter' '$STREAMING_PKG/types.go'"

run_test "WriteMessage method defined" \
    "grep -q 'func (w \*WebSocketWriter) WriteMessage' '$STREAMING_PKG/types.go'"

run_test "WriteJSON method defined" \
    "grep -q 'func (w \*WebSocketWriter) WriteJSON' '$STREAMING_PKG/types.go'"

run_test "WriteBinary method defined" \
    "grep -q 'func (w \*WebSocketWriter) WriteBinary' '$STREAMING_PKG/types.go'"

# ============================================================================
# Section 5: JSONL Implementation (5 tests)
# ============================================================================
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "Section 5: JSONL (JSON Lines) Implementation"
echo "═══════════════════════════════════════════════════════════════════"

run_test "JSONLWriter struct defined" \
    "grep -q 'type JSONLWriter struct' '$STREAMING_PKG/types.go'"

run_test "NewJSONLWriter function defined" \
    "grep -q 'func NewJSONLWriter' '$STREAMING_PKG/types.go'"

run_test "NewJSONLWriterHTTP function defined" \
    "grep -q 'func NewJSONLWriterHTTP' '$STREAMING_PKG/types.go'"

run_test "WriteLine method defined" \
    "grep -q 'func (j \*JSONLWriter) WriteLine' '$STREAMING_PKG/types.go'"

run_test "WriteChunk method defined" \
    "grep -q 'func (j \*JSONLWriter) WriteChunk' '$STREAMING_PKG/types.go'"

# ============================================================================
# Section 6: AsyncGenerator Implementation (5 tests)
# ============================================================================
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "Section 6: AsyncGenerator Implementation"
echo "═══════════════════════════════════════════════════════════════════"

run_test "AsyncGenerator struct defined" \
    "grep -q 'type AsyncGenerator struct' '$STREAMING_PKG/types.go'"

run_test "NewAsyncGenerator function defined" \
    "grep -q 'func NewAsyncGenerator' '$STREAMING_PKG/types.go'"

run_test "Yield method defined" \
    "grep -q 'func (g \*AsyncGenerator) Yield' '$STREAMING_PKG/types.go'"

run_test "Next method defined" \
    "grep -q 'func (g \*AsyncGenerator) Next' '$STREAMING_PKG/types.go'"

run_test "Channel method defined" \
    "grep -q 'func (g \*AsyncGenerator) Channel' '$STREAMING_PKG/types.go'"

# ============================================================================
# Section 7: EventStream Implementation (4 tests)
# ============================================================================
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "Section 7: EventStream (AWS) Implementation"
echo "═══════════════════════════════════════════════════════════════════"

run_test "EventStreamWriter struct defined" \
    "grep -q 'type EventStreamWriter struct' '$STREAMING_PKG/types.go'"

run_test "NewEventStreamWriter function defined" \
    "grep -q 'func NewEventStreamWriter' '$STREAMING_PKG/types.go'"

run_test "NewEventStreamWriterHTTP function defined" \
    "grep -q 'func NewEventStreamWriterHTTP' '$STREAMING_PKG/types.go'"

run_test "EventStreamMessage struct defined" \
    "grep -q 'type EventStreamMessage struct' '$STREAMING_PKG/types.go'"

# ============================================================================
# Section 8: MpscStream Implementation (4 tests)
# ============================================================================
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "Section 8: MpscStream (Multi-Producer Single-Consumer) Implementation"
echo "═══════════════════════════════════════════════════════════════════"

run_test "MpscStream struct defined" \
    "grep -q 'type MpscStream struct' '$STREAMING_PKG/types.go'"

run_test "NewMpscStream function defined" \
    "grep -q 'func NewMpscStream' '$STREAMING_PKG/types.go'"

run_test "GetProducer method defined" \
    "grep -q 'func (m \*MpscStream) GetProducer' '$STREAMING_PKG/types.go'"

run_test "Consumer method defined" \
    "grep -q 'func (m \*MpscStream) Consumer' '$STREAMING_PKG/types.go'"

# ============================================================================
# Section 9: Stdout Implementation (4 tests)
# ============================================================================
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "Section 9: Stdout Streaming Implementation"
echo "═══════════════════════════════════════════════════════════════════"

run_test "StdoutWriter struct defined" \
    "grep -q 'type StdoutWriter struct' '$STREAMING_PKG/types.go'"

run_test "NewStdoutWriter function defined" \
    "grep -q 'func NewStdoutWriter' '$STREAMING_PKG/types.go'"

run_test "WriteLine method defined" \
    "grep -q 'func (s \*StdoutWriter) WriteLine' '$STREAMING_PKG/types.go'"

run_test "Flush method defined" \
    "grep -q 'func (s \*StdoutWriter) Flush' '$STREAMING_PKG/types.go'"

# ============================================================================
# Section 10: Universal Streamer (4 tests)
# ============================================================================
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "Section 10: Universal Streamer"
echo "═══════════════════════════════════════════════════════════════════"

run_test "UniversalStreamer struct defined" \
    "grep -q 'type UniversalStreamer struct' '$STREAMING_PKG/types.go'"

run_test "NewUniversalStreamer function defined" \
    "grep -q 'func NewUniversalStreamer' '$STREAMING_PKG/types.go'"

run_test "ContentTypeForStreamingType function defined" \
    "grep -q 'func ContentTypeForStreamingType' '$STREAMING_PKG/types.go'"

run_test "IsStreamingSupported function defined" \
    "grep -q 'func IsStreamingSupported' '$STREAMING_PKG/types.go'"

# ============================================================================
# Section 11: Unit Tests Compilation and Execution (4 tests)
# ============================================================================
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "Section 11: Unit Tests Compilation and Execution"
echo "═══════════════════════════════════════════════════════════════════"

cd "$PROJECT_ROOT"

run_test "Streaming package compiles" \
    "go build ./internal/streaming/..."

run_test "Unit tests compile" \
    "go test -c ./internal/streaming/... -o /dev/null"

run_test "Unit tests pass" \
    "go test -v ./internal/streaming/... -count=1 -timeout=60s"

run_test "Race condition test passes" \
    "go test -v ./internal/streaming/... -race -count=1 -timeout=120s"

# ============================================================================
# Summary
# ============================================================================
echo ""
echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║                    CHALLENGE RESULTS                              ║"
echo "╠══════════════════════════════════════════════════════════════════╣"
echo "║  Total Tests:  $TOTAL_TESTS"
echo "║  Passed:       $PASSED_TESTS"
echo "║  Failed:       $FAILED_TESTS"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo ""

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}[SUCCESS] ALL TESTS PASSED!${NC}"
    echo ""
    echo -e "${GREEN}[SUCCESS] Streaming types system verified:${NC}"
    echo -e "${GREEN}[SUCCESS]   - 7 streaming types fully implemented${NC}"
    echo -e "${GREEN}[SUCCESS]   - SSE (OpenCode, ClaudeCode, Plandex, Crush)${NC}"
    echo -e "${GREEN}[SUCCESS]   - WebSocket (ClaudeCode)${NC}"
    echo -e "${GREEN}[SUCCESS]   - AsyncGenerator (KiloCode, Cline, OllamaCode)${NC}"
    echo -e "${GREEN}[SUCCESS]   - JSONL (GeminiCLI)${NC}"
    echo -e "${GREEN}[SUCCESS]   - MpscStream (Forge)${NC}"
    echo -e "${GREEN}[SUCCESS]   - EventStream (Amazon Q)${NC}"
    echo -e "${GREEN}[SUCCESS]   - Stdout (Aider, GPT Engineer)${NC}"
    echo -e "${GREEN}[SUCCESS]   - All unit tests pass${NC}"
    echo -e "${GREEN}[SUCCESS]   - Race condition tests pass${NC}"
    echo ""
    exit 0
else
    echo -e "${RED}[FAILED] $FAILED_TESTS tests failed${NC}"
    echo ""
    exit 1
fi
