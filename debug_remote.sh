#!/bin/bash
# Debug Remote Distribution Script
# Shows exactly what's happening with service configuration

set -e

cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent

echo "========================================"
echo "DEBUG: Remote Distribution Configuration"
echo "========================================"
echo ""

# Export environment BEFORE starting HelixAgent
export CONTAINERS_REMOTE_ENABLED=true
export CONTAINERS_REMOTE_HOST_1_NAME=thinker
export CONTAINERS_REMOTE_HOST_1_ADDRESS=thinker.local
export CONTAINERS_REMOTE_HOST_1_PORT=22
export CONTAINERS_REMOTE_HOST_1_USER=milosvasic
export CONTAINERS_REMOTE_HOST_1_RUNTIME=podman

echo "Environment variables set:"
echo "  CONTAINERS_REMOTE_ENABLED=$CONTAINERS_REMOTE_ENABLED"
echo "  CONTAINERS_REMOTE_HOST_1_NAME=$CONTAINERS_REMOTE_HOST_1_NAME"
echo ""

# Verify containers/.env
echo "containers/.env contents:"
cat containers/.env
echo ""

# Start HelixAgent with maximum debug logging
echo "Starting HelixAgent with debug logging..."
./bin/helixagent 2>&1 | tee /tmp/helixagent-debug.log | grep -E "remote|deploy|compose|service|Remote" | head -100 &

PID=$!
echo "HelixAgent PID: $PID"
echo ""

# Wait for startup
sleep 10

# Check what's happening
echo "========================================"
echo "Checking distribution status..."
echo "========================================"
echo ""

echo "Local containers:"
podman ps --format "table {{.Names}}\t{{.Status}}" 2>/dev/null | grep helixagent || echo "  (none)"
echo ""

echo "Remote containers (thinker.local):"
ssh milosvasic@thinker.local 'podman ps --format "table {{.Names}}\t{{.Status}}"' 2>/dev/null || echo "  Cannot check remote"
echo ""

echo "========================================"
echo "Recent log entries:"
echo "========================================"
tail -30 /tmp/helixagent-debug.log | grep -E "remote|Remote|deploy|Deploy|compose|service|ERROR|error" || echo "(no relevant logs)"
