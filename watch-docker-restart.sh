#!/bin/bash

# Script to watch Docker logs and restart containers on "bind: address already in use" errors
# Usage: ./watch-docker-restart.sh
#   Monitors all containers via docker compose

set -euo pipefail

COOLDOWN_SECONDS=30  # Prevent rapid restarts
LAST_RESTART_FILE="/tmp/docker-watch-last-restart.txt"

# Function to restart a container
restart_container() {
    local container="$1"
    local now=$(date +%s)
    local last_restart=0
    
    # Check cooldown period
    if [ -f "$LAST_RESTART_FILE" ]; then
        last_restart=$(cat "$LAST_RESTART_FILE" 2>/dev/null || echo "0")
    fi
    
    local time_since_restart=$((now - last_restart))
    
    if [ $time_since_restart -lt $COOLDOWN_SECONDS ]; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] Skipping restart of $container (cooldown: $((COOLDOWN_SECONDS - time_since_restart))s remaining)"
        return
    fi
    
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Restarting container: $container"
    
    # Using docker compose
    local services=$(docker compose ps --format json | grep "$container" | jq -r '.Service')

    # Using docker compose
    docker compose restart "$services" 2>/dev/null || true
    
    # Record restart time
    echo "$now" > "$LAST_RESTART_FILE"
}

# Function to monitor all containers via docker compose
monitor_all_containers() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Monitoring all containers via docker compose"
    
    docker compose logs -f --tail=0 2>&1 | while IFS= read -r line; do
        # Docker compose logs format: container_name | log_line
        if echo "$line" | grep -qi "bind: address already in use"; then
            # Extract container name from log line (format: "container_name | message")
            local container=$(echo "$line" | sed -n 's/^\([^|]*\)|.*/\1/p' | xargs)
            
            if [ -n "$container" ]; then
                echo "[$(date '+%Y-%m-%d %H:%M:%S')] Detected 'bind: address already in use' in $container"
                restart_container "$container"
            else
                # Fallback: try to get container name from docker compose service name
                # This handles cases where the log format might be different
                echo "[$(date '+%Y-%m-%d %H:%M:%S')] Detected 'bind: address already in use' (attempting to identify container)"
                # Try to restart all services that might be affected
                for service in app; do
                    if docker compose ps "$service" --format json 2>/dev/null | grep -q "running"; then
                        restart_container "$service"
                        break
                    fi
                done
            fi
        fi
    done
}

# Main execution
# Check if docker compose is available
if ! command -v docker compose &> /dev/null; then
    echo "Error: docker compose not found. Please install docker compose."
    exit 1
fi

monitor_all_containers

