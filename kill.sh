#!/bin/bash
set -e

# Paths
OUTPUT_DIR="$PWD/src/output"
JSON_FILE="$OUTPUT_DIR/servers.json"

if [[ ! -f "$JSON_FILE" ]]; then
    echo "No servers.json found. Nothing to kill."
    exit 0
fi

# Read JSON array into bash array
mapfile -t HOST_PORTS < <(jq -r '.[]' "$JSON_FILE")

for HOSTPORT in "${HOST_PORTS[@]}"; do
    HOST="${HOSTPORT%%:*}"
    PORT="${HOSTPORT##*:}"

    echo "Killing server on $HOST:$PORT..."
    # Kill any server process matching the binary name and port
    ssh "$HOST" "pkill -f 'server.*-port $PORT'" || true
done

# Remove JSON file after cleanup
rm -f "$JSON_FILE"
echo "All servers killed and servers.json removed."
