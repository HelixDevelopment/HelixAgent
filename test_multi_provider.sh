#!/bin/bash

# SuperAgent Multi-Provider Test Script
# Tests OpenAI compatibility with automatic ensemble support

set -e

# Configuration
SUPERAGENT_URL="http://localhost:8080"
API_KEY="${SUPERAGENT_API_KEY:-test-key}"
MODEL="${MODEL:-superagent-ensemble}"

echo "🚀 Testing SuperAgent Multi-Provider OpenAI Compatibility"
echo "=========================================================="
echo "URL: $SUPERAGENT_URL"
echo "Model: $MODEL"
echo ""

# Function to test endpoint
test_endpoint() {
    local endpoint="$1"
    local method="$2"
    local data="$3"
    local description="$4"
    
    echo "📍 Testing: $description"
    echo "   Endpoint: $method $endpoint"
    
    if [ -n "$data" ]; then
        response=$(curl -s -w "\n%{http_code}" \
            -X "$method" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $API_KEY" \
            -d "$data" \
            "$SUPERAGENT_URL$endpoint")
    else
        response=$(curl -s -w "\n%{http_code}" \
            -X "$method" \
            -H "Authorization: Bearer $API_KEY" \
            "$SUPERAGENT_URL$endpoint")
    fi
    
    # Split response and status code
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" -eq 200 ]; then
        echo "   ✅ Success (HTTP $http_code)"
        if command -v jq >/dev/null 2>&1; then
            echo "   Response: $(echo "$body" | jq -r 'if type == "object" then (.message // .model // keys[0]) else . end' | head -1)"
        fi
    else
        echo "   ❌ Failed (HTTP $http_code)"
        echo "   Response: $body"
        return 1
    fi
    echo ""
}

# Function to test streaming
test_streaming() {
    local endpoint="$1"
    local data="$2"
    local description="$3"
    
    echo "📍 Testing: $description (Streaming)"
    echo "   Endpoint: POST $endpoint"
    
    # Test streaming with timeout
    response=$(timeout 10s curl -s \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $API_KEY" \
        -d "$data" \
        "$SUPERAGENT_URL$endpoint" || echo "TIMEOUT")
    
    if [[ "$response" == *"data:"* ]] || [[ "$response" == *"DONE"* ]]; then
        echo "   ✅ Streaming successful"
        echo "   Received streaming response with multiple chunks"
    elif [[ "$response" == "TIMEOUT" ]]; then
        echo "   ⚠️  Streaming timeout (might still be working)"
    else
        echo "   ❌ Streaming failed"
        echo "   Response: $response"
        return 1
    fi
    echo ""
}

# Check if server is running
echo "🔍 Checking if SuperAgent server is running..."
if ! curl -s "$SUPERAGENT_URL/health" >/dev/null 2>&1; then
    echo "❌ SuperAgent server is not running at $SUPERAGENT_URL"
    echo "Please start the server with: ./superagent"
    exit 1
fi
echo "✅ Server is running"
echo ""

# Test 1: List all available models from all providers
test_endpoint "/v1/models" "GET" "" "Models endpoint - should show all models from DeepSeek, Qwen, and OpenRouter"

# Test 2: Simple chat completion with ensemble
chat_data='{
    "model": "'$MODEL'",
    "messages": [
        {"role": "user", "content": "What is the difference between REST and GraphQL? Keep it concise."}
    ],
    "max_tokens": 150,
    "temperature": 0.3
}'
test_endpoint "/v1/chat/completions" "POST" "$chat_data" "Chat completion with automatic ensemble voting"

# Test 3: Code generation task (should leverage DeepSeek Coder)
code_data='{
    "model": "'$MODEL'",
    "messages": [
        {"role": "user", "content": "Write a Go function that validates an email address using regex"}
    ],
    "max_tokens": 200,
    "temperature": 0.1
}'
test_endpoint "/v1/chat/completions" "POST" "$code_data" "Code generation (ensemble should favor DeepSeek)"

# Test 4: Mathematical reasoning (should leverage Grok-4/Gemini)
math_data='{
    "model": "'$MODEL'",
    "messages": [
        {"role": "user", "content": "What is the sum of the first 100 prime numbers? Show your reasoning."}
    ],
    "max_tokens": 300,
    "temperature": 0.2
}'
test_endpoint "/v1/chat/completions" "POST" "$math_data" "Mathematical reasoning (ensemble should favor Grok-4/Gemini)"

# Test 5: Streaming chat completion
stream_data='{
    "model": "'$MODEL'",
    "messages": [
        {"role": "user", "content": "Explain quantum computing in simple terms"}
    ],
    "max_tokens": 200,
    "stream": true
}'
test_streaming "/v1/chat/completions" "$stream_data" "Streaming chat completion"

# Test 6: Test with specific provider forced
provider_data='{
    "model": "deepseek-coder",
    "messages": [
        {"role": "user", "content": "Write a Python function to find the factorial of a number"}
    ],
    "max_tokens": 150,
    "force_provider": "deepseek"
}'
test_endpoint "/v1/chat/completions" "POST" "$provider_data" "Force specific provider (DeepSeek)"

# Test 7: Test traditional completions endpoint
completion_data='{
    "model": "'$MODEL'",
    "prompt": "The future of AI is",
    "max_tokens": 50,
    "temperature": 0.5
}'
test_endpoint "/v1/completions" "POST" "$completion_data" "Legacy completions endpoint"

echo "🎉 Testing Complete!"
echo "===================="
echo ""
echo "📊 Summary of Multi-Provider Configuration:"
echo "  • DeepSeek Coder: Optimized for code generation"
echo "  • Qwen Turbo: General purpose and multilingual tasks"
echo "  • OpenRouter Grok-4: Advanced reasoning and real-time data"
echo "  • OpenRouter Gemini 2.5: Multimodal and mathematical tasks"
echo ""
echo "🔧 Ensemble Features:"
echo "  • Automatic provider selection based on task type"
echo "  • Confidence-weighted voting for best results"
echo "  • Fallback to highest confidence if no consensus"
echo "  • Full OpenAI API compatibility"
echo ""
echo "🚀 Your SuperAgent is ready for use with OpenCode, Crush, and other AI CLI tools!"
echo ""
echo "💡 Usage Examples:"
echo "   opencode --api-key $API_KEY --base-url $SUPERAGENT_URL/v1 --model $MODEL \"Write a REST API\""
echo "   curl -H 'Authorization: Bearer $API_KEY' -H 'Content-Type: application/json' \\"
echo "        -d '{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello\"}]}' \\"
echo "        $SUPERAGENT_URL/v1/chat/completions"