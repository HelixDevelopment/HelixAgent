#!/bin/sh
# Start all MCP servers for HelixAgent
# Each server runs on a dedicated port in SSE mode
# Handles both Node.js and Python-based MCP servers

set -e

LOG_DIR="/var/log/mcp-servers"
mkdir -p "$LOG_DIR"

echo "Starting MCP servers..."

# Function to start a Node.js server
start_node_server() {
    name=$1
    port=$2
    dir=$3

    # The active-server sources are optional in this image (Dockerfile
    # ARG ACTIVE_SERVERS, default `absent`). Without this guard the `cd`
    # below fails, and `set -e` at the top of this script turns one absent
    # directory into "no MCP server starts at all" — the container dies on
    # boot instead of serving the servers it DOES have.
    if [ ! -d "$dir" ]; then
        echo "Skipping $name: $dir is not present in this image"
        echo "  (built without the active MCP server sources — rebuild with"
        echo "   --build-arg ACTIVE_SERVERS=present to include them)"
        return 0
    fi

    echo "Starting $name (Node.js) on port $port..."
    cd "$dir"

    if [ -f "dist/index.js" ]; then
        MCP_PORT=$port node "dist/index.js" >> "$LOG_DIR/$name.log" 2>&1 &
    elif [ -f "build/index.js" ]; then
        MCP_PORT=$port node "build/index.js" >> "$LOG_DIR/$name.log" 2>&1 &
    elif [ -f "index.js" ]; then
        MCP_PORT=$port node "index.js" >> "$LOG_DIR/$name.log" 2>&1 &
    else
        echo "Warning: Could not find Node.js entry point for $name"
        return 1
    fi

    echo "$!" > "/var/run/mcp-$name.pid"
    echo "$name started with PID $(cat /var/run/mcp-$name.pid)"
}

# Function to start a Python server
start_python_server() {
    name=$1
    port=$2
    dir=$3
    module=$4

    # See start_node_server(): active-server sources are optional in this image.
    if [ ! -d "$dir" ]; then
        echo "Skipping $name: $dir is not present in this image"
        echo "  (built without the active MCP server sources — rebuild with"
        echo "   --build-arg ACTIVE_SERVERS=present to include them)"
        return 0
    fi

    echo "Starting $name (Python) on port $port..."
    cd "$dir"

    if [ -n "$module" ]; then
        MCP_PORT=$port python3 -m "$module" >> "$LOG_DIR/$name.log" 2>&1 &
    elif [ -f "src/__main__.py" ]; then
        MCP_PORT=$port python3 -m src >> "$LOG_DIR/$name.log" 2>&1 &
    elif [ -f "__main__.py" ]; then
        MCP_PORT=$port python3 . >> "$LOG_DIR/$name.log" 2>&1 &
    else
        echo "Warning: Could not find Python entry point for $name"
        return 1
    fi

    echo "$!" > "/var/run/mcp-$name.pid"
    echo "$name started with PID $(cat /var/run/mcp-$name.pid)"
}

# Start active servers (from servers repo)
# Node.js servers
start_node_server "everything" 3007 "/app/servers/src/everything"
start_node_server "filesystem" 3002 "/app/servers/src/filesystem"
start_node_server "memory" 3004 "/app/servers/src/memory"
start_node_server "sequential-thinking" 3006 "/app/servers/src/sequentialthinking"

# Python servers
start_python_server "fetch" 3001 "/app/servers/src/fetch" "mcp_server_fetch"
start_python_server "git" 3003 "/app/servers/src/git" "mcp_server_git"
start_python_server "time" 3005 "/app/servers/src/time" "mcp_server_time"

# Start archived servers (from servers-archived repo - they're in src/ subdirectory)
# Node.js servers
start_node_server "postgres" 3008 "/app/servers-archived/src/postgres"
start_node_server "slack" 3010 "/app/servers-archived/src/slack"
start_node_server "github" 3011 "/app/servers-archived/src/github"
start_node_server "gitlab" 3012 "/app/servers-archived/src/gitlab"
start_node_server "google-maps" 3013 "/app/servers-archived/src/google-maps"
start_node_server "brave-search" 3014 "/app/servers-archived/src/brave-search"
start_node_server "puppeteer" 3015 "/app/servers-archived/src/puppeteer"
start_node_server "redis" 3016 "/app/servers-archived/src/redis"
start_node_server "gdrive" 3018 "/app/servers-archived/src/gdrive"
start_node_server "everart" 3019 "/app/servers-archived/src/everart"
start_node_server "aws-kb-retrieval" 3020 "/app/servers-archived/src/aws-kb-retrieval-server"

# Python servers
start_python_server "sqlite" 3009 "/app/servers-archived/src/sqlite" "mcp_server_sqlite"
start_python_server "sentry" 3017 "/app/servers-archived/src/sentry" "mcp_server_sentry"

echo ""
echo "All MCP servers started. Monitoring..."

# Keep container running and monitor servers
while true; do
    sleep 30

    # Check if servers are still running
    for pid_file in /var/run/mcp-*.pid; do
        if [ -f "$pid_file" ]; then
            name=$(basename "$pid_file" .pid | sed 's/mcp-//')
            pid=$(cat "$pid_file")
            if ! kill -0 "$pid" 2>/dev/null; then
                echo "Warning: $name (PID $pid) has stopped"
            fi
        fi
    done
done
