#!/bin/bash
# System Monitor for HelixAgent with HelixLLM

LOGFILE="system_monitor_$(date +%Y%m%d-%H%M%S).log"

echo "===================================" | tee -a "$LOGFILE"
echo "HelixAgent + HelixLLM System Monitor" | tee -a "$LOGFILE"
echo "Started: $(date)" | tee -a "$LOGFILE"
echo "Log: $LOGFILE" | tee -a "$LOGFILE"
echo "===================================" | tee -a "$LOGFILE"
echo "" | tee -a "$LOGFILE"

check_health() {
    echo "--- $(date) ---" | tee -a "$LOGFILE"
    
    # Check HelixAgent process
    if pgrep -x "helixagent" > /dev/null; then
        echo "✓ HelixAgent process: RUNNING (PID: $(pgrep -x helixagent))" | tee -a "$LOGFILE"
    else
        echo "✗ HelixAgent process: NOT RUNNING" | tee -a "$LOGFILE"
    fi
    
    # Check containers
    echo "" | tee -a "$LOGFILE"
    echo "Container Status:" | tee -a "$LOGFILE"
    podman ps --format "table {{.Names}}\t{{.Status}}" 2>/dev/null | tee -a "$LOGFILE"
    
    # Check HelixAgent health endpoint
    echo "" | tee -a "$LOGFILE"
    echo "HelixAgent Health:" | tee -a "$LOGFILE"
    HEALTH=$(curl -sf http://localhost:7061/health 2>/dev/null)
    if [ $? -eq 0 ]; then
        echo "✓ HelixAgent API: HEALTHY" | tee -a "$LOGFILE"
        echo "$HEALTH" | tee -a "$LOGFILE"
    else
        echo "✗ HelixAgent API: Not ready yet" | tee -a "$LOGFILE"
    fi
    
    # Check HelixLLM infrastructure
    echo "" | tee -a "$LOGFILE"
    echo "HelixLLM Infrastructure:" | tee -a "$LOGFILE"
    
    # Qdrant
    if curl -sf http://localhost:6333/healthz > /dev/null 2>&1; then
        echo "✓ Qdrant: HEALTHY" | tee -a "$LOGFILE"
    else
        echo "✗ Qdrant: Not ready" | tee -a "$LOGFILE"
    fi
    
    # Redis
    if podman exec helixagent-helixllm-redis redis-cli -a helixllm123 --no-auth-warning ping > /dev/null 2>&1; then
        echo "✓ HelixLLM Redis: HEALTHY" | tee -a "$LOGFILE"
    else
        echo "✗ HelixLLM Redis: Not ready" | tee -a "$LOGFILE"
    fi
    
    # PostgreSQL
    if podman exec helixagent-helixllm-postgres pg_isready -U helix -d helixllm > /dev/null 2>&1; then
        echo "✓ HelixLLM PostgreSQL: HEALTHY" | tee -a "$LOGFILE"
    else
        echo "✗ HelixLLM PostgreSQL: Not ready" | tee -a "$LOGFILE"
    fi
    
    echo "" | tee -a "$LOGFILE"
    echo "===================================" | tee -a "$LOGFILE"
    echo "" | tee -a "$LOGFILE"
}

# Run continuous monitoring
echo "Starting continuous monitoring..." | tee -a "$LOGFILE"
echo "Press Ctrl+C to stop" | tee -a "$LOGFILE"
echo "" | tee -a "$LOGFILE"

# Initial check
check_health

# Monitor loop
while true; do
    sleep 30
    check_health
done
