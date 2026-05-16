#!/bin/bash
# Setup script for rootless Podman - enables container support for helix_agent/Cognee
# Run with: sudo ./scripts/setup-podman-rootless.sh

set -e

USER_NAME="${1:-milosvasic}"
SUBUID_START=100000
SUBUID_COUNT=65536

echo "=========================================="
echo "Rootless Podman Setup for HelixAgent"
echo "=========================================="
echo ""

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo "ERROR: This script must be run as root (use sudo)"
    exit 1
fi

echo "[1/5] Configuring /etc/subuid..."
if ! grep -q "^${USER_NAME}:" /etc/subuid 2>/dev/null; then
    echo "${USER_NAME}:${SUBUID_START}:${SUBUID_COUNT}" >> /etc/subuid
    echo "  Added: ${USER_NAME}:${SUBUID_START}:${SUBUID_COUNT}"
else
    echo "  Already configured"
fi

echo "[2/5] Configuring /etc/subgid..."
if ! grep -q "^${USER_NAME}:" /etc/subgid 2>/dev/null; then
    echo "${USER_NAME}:${SUBUID_START}:${SUBUID_COUNT}" >> /etc/subgid
    echo "  Added: ${USER_NAME}:${SUBUID_START}:${SUBUID_COUNT}"
else
    echo "  Already configured"
fi

echo "[3/5] Setting up newuidmap/newgidmap permissions..."
if [ -f /usr/bin/newuidmap ]; then
    chmod u+s /usr/bin/newuidmap
    echo "  Set setuid on /usr/bin/newuidmap"
fi
if [ -f /usr/bin/newgidmap ]; then
    chmod u+s /usr/bin/newgidmap
    echo "  Set setuid on /usr/bin/newgidmap"
fi

echo "[4/5] Running podman system migrate..."
su - "${USER_NAME}" -c "podman system migrate" 2>/dev/null || true

echo "[5/5] Testing container functionality..."
if su - "${USER_NAME}" -c "podman run --rm alpine:latest echo 'Container test successful!'" 2>/dev/null; then
    echo ""
    echo "=========================================="
    echo "SUCCESS: Rootless Podman is now configured!"
    echo "=========================================="
    echo ""
    echo "You can now run:"
    echo "  podman-compose up -d"
    echo ""
else
    echo ""
    echo "WARNING: Container test failed. Manual intervention may be needed."
    echo "Try running: podman system migrate"
fi
