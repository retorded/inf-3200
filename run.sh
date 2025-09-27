#!/bin/bash

# Usage: ./run.sh <num_servers> or ./run.sh kill
COMMAND=$1

# Paths (defined once at the top)
OUTPUT_DIR="build"
SERVER_BIN="$OUTPUT_DIR/server"
LOG_DIR="$OUTPUT_DIR/logs"
JSON_FILE="$OUTPUT_DIR/servers.json"
BENCHMARK_FILE="$OUTPUT_DIR/benchmark.csv"
GO_SOURCE="cmd/server/main.go"
PROJECT_ROOT="$PWD"

# Init function - first-time setup for new environment
init() {
    echo "Initializing Go project..."
    
    # Initialize go module if it doesn't exist
    if [[ ! -f "$PROJECT_ROOT/src/go.mod" ]]; then
        echo "Creating go.mod file..."
        cd src && go mod init assignment1 && cd ..
    else
        echo "go.mod already exists"
    fi

    # Tidy up dependencies
    echo "Running go mod tidy..."
    cd src && go mod tidy && cd ..

    # Create necessary directories
    echo "Creating output directories..."
    mkdir -p "$OUTPUT_DIR" "$LOG_DIR"

    echo "Initialization complete!"
}

# Cleanup function - kills old servers and removes old builds/logs
cleanup() {
    echo "Cleaning up old servers..."
    if [[ -f "$JSON_FILE" ]]; then
        mapfile -t OLD_HOST_PORTS < <(jq -r '.[]' "$JSON_FILE")
        for HOSTPORT in "${OLD_HOST_PORTS[@]}"; do
            HOST="${HOSTPORT%%:*}"
            PORT="${HOSTPORT##*:}"
            echo "Killing server on $HOST:$PORT..."
            ssh "$HOST" "pkill -f '$SERVER_BIN.*-port $PORT'" || true
        done
        rm -f "$JSON_FILE"
    else
        echo "No servers.json found. Nothing to kill."
    fi

    echo "Cleaning up old builds and logs..."
    rm -f "$SERVER_BIN"
    rm -f "$LOG_DIR"/*.log
}

# Build function - builds the server binary
build() {
    echo "Building server..."
    # Ensure output directories exist
    mkdir -p "$OUTPUT_DIR" "$LOG_DIR"

    # Build the server binary
    cd src && go build -o "../$SERVER_BIN" "$GO_SOURCE" && cd ..
    if [[ $? -ne 0 ]]; then
        echo "Failed to build server binary"
        exit 1
    fi
    echo "Build complete: $SERVER_BIN"
}

# Start function - starts the servers
deploy() {
    echo "Starting $NUM_SERVERS servers..."
    
    # Get available nodes
    mapfile -t AVAILABLE_NODES < <(/share/ifi/available-nodes.sh)
    # For testing round-robin: use only first 3 nodes
    # NODES=("${NODES[@]:0:3}")
    NUM_NODES=${#AVAILABLE_NODES[@]}
    echo "Available nodes: $NUM_NODES"

    NETWORK=()

    # Find a free ephemeral port on each node
    NODES=()
    PORTS=()
    for ((i=0; i<NUM_SERVERS; i++)); do
        NODE=${AVAILABLE_NODES[$((i % NUM_NODES))]}

        # Find a free ephemeral port on node
        while true; do
            PORT=$(shuf -i 49152-65535 -n1)
            # Check if port is in use
            IN_USE=$(ssh "$NODE" "ss -tuln | grep -w :$PORT" || true)
            if [[ -z "$IN_USE" ]]; then
                break
            fi
        done

        NODES+=("$NODE")
        PORTS+=("$PORT")

        # Store host:port for JSON output
        NETWORK+=("${NODE}:${PORT}")
    done

    # Convert network array to comma-separated string for Go program
    NETWORK_STR=$(IFS=','; echo "${NETWORK[*]}")

    # Start the servers
    for ((i=0; i<NUM_SERVERS; i++)); do
        
        NODE=${NODES[$i]}
        PORT=${PORTS[$i]}

        # Log file path
        LOG_FILE="$LOG_DIR/server_${NODE}_${PORT}.log"

        # Start server using shared NFS path
        ssh -f "$NODE" "cd $PWD && $SERVER_BIN -port $PORT -network '$NETWORK_STR' > $LOG_FILE 2>&1 &"
    done

    echo "Started $NUM_SERVERS servers"

    # Using jq
    # Explanation:
    # - printf prints each element on a new line
    # - jq -R . wraps each line in quotes
    # - jq -s . collects all lines into a JSON array
    printf '%s\n' "${NETWORK[@]}" | jq -R . | jq -s -c . | tee "$JSON_FILE"

}

# Kill function - kills all running servers
kill() {
    if [[ ! -f "$JSON_FILE" ]]; then
        echo "No servers.json found. Nothing to kill."
        return 0
    fi

    # Read JSON array into bash array
    mapfile -t HOST_PORTS < <(jq -r '.[]' "$JSON_FILE")

    for HOSTPORT in "${HOST_PORTS[@]}"; do
        HOST="${HOSTPORT%%:*}"
        PORT="${HOSTPORT##*:}"

        echo "Killing server on $HOST:$PORT..."
        # Kill any server process matching the binary name and port
        ssh "$HOST" "pkill -f '$SERVER_BIN.*-port $PORT'" || true
    done

    # Remove JSON file after cleanup
    rm -f "$JSON_FILE"
    echo "All servers killed and servers.json removed."
}

benchmark() {
    local operations=${1:-1000}
    local trials=${2:-3}

    echo "Starting benchmark for $NUM_SERVERS nodes..."

    # Ensure output directory exists
    mkdir -p "$OUTPUT_DIR"
    
    # Initialize CSV if it doesn't exist
    if [[ ! -f "$BENCHMARK_FILE" ]]; then
        echo "timestamp,network_size,trial,operations,put_ops_per_sec,get_ops_per_sec" > "$BENCHMARK_FILE"
    fi
    
    for trial in $(seq 1 $trials); do
        echo "  Trial $trial/$trials"
        
        # Start network
        deploy
        # Sleep 2 seconds for server init
        sleep 2
        
        # Get all nodes from JSON file for distribution of test requests
        if [[ -f "$JSON_FILE" ]]; then
            # Get all server addresses from JSON file
            servers=$(jq -r '.[]' "$JSON_FILE" | tr '\n' ',' | sed 's/,$//')
            
            # Run benchmark with all servers
            if python3 chord-benchmark.py \
                --network-size $NUM_SERVERS \
                --trial $trial \
                --operations $operations \
                --csv-file "$BENCHMARK_FILE" \
                --servers "$servers"; then
                echo "Benchmark completed successfully"
            else 
                echo "Benchmark failed with errors"
                return 1
            fi
        else
            echo "ERROR: servers.json not found"
            continue
        fi  
        
        # Cleanup
        ./run.sh kill
        sleep 5
    done
}

# Main execution
if [[ "$COMMAND" == "kill" ]]; then
    kill
elif [[ "$COMMAND" == "init" ]]; then
    init
elif [[ "$COMMAND" == "cleanup" ]]; then
    cleanup
elif [[ "$COMMAND" == "build" ]]; then
    build
elif [[ "$COMMAND" == "benchmark" ]]; then
    NUM_SERVERS=$2
    trials=${3:-1000}
    operations=${4:-3}
    benchmark $trials $operations
elif [[ -n "$COMMAND" ]] && [[ "$COMMAND" =~ ^[0-9]+$ ]]; then
    NUM_SERVERS=$COMMAND
    init
    cleanup
    build
    deploy
else
    echo "Usage: $0 <num_servers> or $0 <command>"
    echo "  $0 5       - Run N servers (init + cleanup + build + run)"
    echo "  $0 init    - Initialize Go project"
    echo "  $0 cleanup - Kill old servers and clean build artifacts"
    echo "  $0 build   - Build the server binary"
    echo "  $0 kill    - Kill all running servers"
    exit 1
fi