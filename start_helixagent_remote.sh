#!/bin/bash
# Properly start HelixAgent with remote distribution

set -e

cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent

# Ensure we're in the right directory
if [ ! -f "containers/.env" ]; then
    echo "ERROR: Not in HelixAgent root directory or containers/.env missing"
    exit 1
fi

# Export environment variables
export CONTAINERS_REMOTE_ENABLED=true
export CONTAINERS_REMOTE_HOST_1_NAME=thinker
export CONTAINERS_REMOTE_HOST_1_ADDRESS=thinker.local
export CONTAINERS_REMOTE_HOST_1_PORT=22
export CONTAINERS_REMOTE_HOST_1_USER=milosvasic
export CONTAINERS_REMOTE_HOST_1_RUNTIME=podman

echo "=========================================="
echo "Starting HelixAgent with Remote Distribution"
echo "=========================================="
echo "Working directory: $(pwd)"
echo "containers/.env exists: $(test -f containers/.env && echo 'YES' || echo 'NO')"
echo ""
echo "Environment:"
echo "  CONTAINERS_REMOTE_ENABLED=$CONTAINERS_REMOTE_ENABLED"
echo "  CONTAINERS_REMOTE_HOST_1_ADDRESS=$CONTAINERS_REMOTE_HOST_1_ADDRESS"
echo ""

# Verify config
if go run -exec "echo test" cmd/helixagent/main.go 2>/dev/null | grep -q "test"; then
    echo "Go environment OK"
fi

# Start HelixAgent
exec ./bin/helixagent
