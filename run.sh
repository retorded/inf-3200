#!/bin/bash

# Usage: ./run.sh <num_servers>
NUM_SERVERS=$1
if [[ -z "$NUM_SERVERS" ]]; then
    echo "Usage: $0 <num_servers>"
    exit 1
fi

# Paths
OUTPUT_DIR="$PWD/src/output"
SERVER_BIN="$OUTPUT_DIR/server"
LOG_DIR="$OUTPUT_DIR/logs"
JSON_FILE="$OUTPUT_DIR/servers.json"


# Ensure output directories exist
mkdir -p "$OUTPUT_DIR" "$LOG_DIR"

# Clean up old servers
if [[ -f "$JSON_FILE" ]]; then
    echo "Cleaning up old servers..."
    mapfile -t OLD_HOST_PORTS < <(jq -r '.[]' "$JSON_FILE")
    for HOSTPORT in "${OLD_HOST_PORTS[@]}"; do
        HOST="${HOSTPORT%%:*}"
        PORT="${HOSTPORT##*:}"
        echo "Killing server on $HOST:$PORT..."
        ssh "$HOST" "pkill -f '$SERVER_BIN.*-port $PORT'" || true
    done
    rm -f "$JSON_FILE"
fi

# Clean old artifacts
echo "Cleaning up old builds and logs..."
rm -f "$SERVER_BIN"
rm -f "$LOG_DIR"/*.log

# Build the server binary
echo "Building server..."
go build -o "$SERVER_BIN" "$PWD/src/server/main.go"
if [[ $? -ne 0 ]]; then
    echo "Failed to build server binary"
    exit 1
fi

# Get available nodes
mapfile -t NODES < <(/share/ifi/available-nodes.sh)
NUM_NODES=${#NODES[@]}
HOST_PORTS=()

for ((i=0; i<NUM_SERVERS; i++)); do
    NODE=${NODES[$((i % NUM_NODES))]}

    # Find a free ephemeral port on node
    while true; do
        PORT=$(shuf -i 49152-65535 -n1)
        # Check if port is in use
        IN_USE=$(ssh "$NODE" "ss -tuln | grep -w :$PORT" || true)
        if [[ -z "$IN_USE" ]]; then
            break
        fi
    done

    # Log file path
    LOG_FILE="$LOG_DIR/server_${NODE}_${PORT}.log"

    # Start server in background
    ssh -f "$NODE" "$SERVER_BIN -port $PORT > $LOG_FILE 2>&1 &"

    # Store host:port for JSON output
    HOST_PORTS+=("${NODE}:${PORT}")
done

# Using jq
# Explanation:
# - printf prints each element on a new line
# - jq -R . wraps each line in quotes
# - jq -s . collects all lines into a JSON array
printf '%s\n' "${HOST_PORTS[@]}" | jq -R . | jq -s -c . | tee "$JSON_FILE"

