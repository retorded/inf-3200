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

# Cleanup function - kills old servers and removes old logs
cleanup() {

    # Use kill function to handle server cleanup
    echo "Cleaning up old servers..."
    kill
    
    echo "Cleaning up logs..."
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

deploy() {
    
    # Get available nodes
    mapfile -t AVAILABLE_NODES < <(/share/ifi/available-nodes.sh)
    NUM_NODES=${#AVAILABLE_NODES[@]}
    echo "Available nodes: $NUM_NODES"
    echo "Starting $NUM_SERVERS servers..."

    # Find a free ephemeral port on each node
    NODES=()
    PORTS=()
    NETWORK=()
    for ((i=0; i<NUM_SERVERS; i++)); do
        NODE=${AVAILABLE_NODES[$((i % NUM_NODES))]}

        # Start node and perform wellness check. Continue if successful.
        while true; do

            # Create a random port between 49152 and 65535
            PORT=$(shuf -i 49152-65535 -n1)

            # Log file path
            LOG_FILE="$LOG_DIR/server_${NODE}_${PORT}.log"

            # Start server using shared NFS path
            ssh -f "$NODE" "cd $PWD && $SERVER_BIN -hostname $NODE -port $PORT -logfile $LOG_FILE &"

            # Perform wellness check with manual retry
            HOST_PORT=""
            for ((attempt=1; attempt<=10; attempt++)); do
                HOST_PORT=$(curl -s "http://$NODE:$PORT/ping" --connect-timeout 1 --max-time 1 2>/dev/null || echo "")
                
                if [[ -n "$HOST_PORT" ]]; then
                    break
                fi
                sleep 0.1
            done

            # If wellness check succeeded, break out of the retry loop
            if [[ -n "$HOST_PORT" ]]; then
                break
            fi

            # Wellness check failed - kill process and try again (new port will be found in next iteration)
            echo "Wellness check failed for $NODE:$PORT, trying different port..."
            ssh "$NODE" "pkill -f '$SERVER_BIN.*-port $PORT'" > /dev/null 2>&1
            rm -f "$LOG_FILE"
        done

        NODES+=("$NODE")
        PORTS+=("$PORT")

        # Store host:port for JSON output
        NETWORK+=("${NODE}:${PORT}")

        # Save network to JSON file for other functions
        printf '%s\n' "${NETWORK[@]}" | jq -R . | jq -s -c . > "$JSON_FILE"
    done

    echo "All servers started successfully!"
    

    # Convert network array to comma-separated string for PUT requests
    #NETWORK_STR=$(IFS=','; echo "${NETWORK[*]}")

    # # Initialize the ring network by sending PUT requests to all nodes
    # echo "Initializing ring network..."
    # for HOST_PORT in "${NETWORK[@]}"; do
    #     if ! curl -X PUT "http://$HOST_PORT/network?network=$NETWORK_STR" \
    #         -H "Content-Type: application/json" \
    #         --connect-timeout 5 \
    #         --max-time 10 \
    #         --show-error; then
    #         echo "ERROR: Failed to initialize node $HOST_PORT"
    #         exit 1
    #     fi
    # done

    # Ask the first node to join the second node
    # curl -X POST "http://${NETWORK[1]}/join?nprime=${NETWORK[0]}"


    echo "Deployment complete! Started $NUM_SERVERS servers."
    #printf '%s\n' "${NETWORK[@]}" | jq -R . | jq -s -c .
    echo "Network: ${NETWORK[*]}"
}

reset() {
    # Sending leave request to all nodes in json file
    if [[ ! -f "$JSON_FILE" ]]; then
        echo "No servers.json found. Nothing to kill."
        return 0
    fi

    # Read JSON array into bash array
    mapfile -t HOST_PORTS < <(jq -r '.[]' "$JSON_FILE")

    for HOSTPORT in "${HOST_PORTS[@]}"; do
        HOST="${HOSTPORT%%:*}"
        PORT="${HOSTPORT##*:}"

        # Leave the ring
        curl -X POST "http://$HOST:$PORT/leave"
    done

    for HOSTPORT in "${HOST_PORTS[@]}"; do
        HOST="${HOSTPORT%%:*}"
        PORT="${HOSTPORT##*:}"

        # Recover node so it can start processing requests again
        curl -X POST "http://$HOST:$PORT/sim-recover"
    done

    echo "All nodes left the ring and recovered."
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

        # Kill any server process matching the binary name and port
        ssh "$HOST" "pkill -f '$SERVER_BIN.*-port $PORT'" || true
    done

    # Remove JSON file after cleanup
    rm -f "$JSON_FILE"
    echo "All servers killed and servers.json removed."
}

benchmark-throughput() {
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

        sleep 2
        
        # Get all nodes from JSON file for distribution of test requests
        if [[ -f "$JSON_FILE" ]]; then
            # Get all server addresses from JSON file
            servers=$(jq -r '.[]' "$JSON_FILE" | tr '\n' ',' | sed 's/,$//')
            
            # Run benchmark with all servers
            if python3 chord-throughput-benchmark.py \
                --network-size $NUM_SERVERS \
                --trial $trial \
                --operations $operations \
                --csv-file "$BENCHMARK_FILE" \
                --servers "$servers"; then
                echo "Benchmark completed successfully"
            else 
                echo "Benchmark failed with errors - stopping entire benchmark"
                echo "Logs preserved in: $LOG_DIR"
                echo " Check server logs for details:"
                exit 1
            fi
        else
            echo "ERROR: servers.json not found"
            continue
        fi  

        # Cleanup servers and logs between trials
        cleanup
    done
}


benchmark-dynamic() {

    echo "Starting dynamic benchmark for $NUM_SERVERS nodes..."

    # Ensure output directory exists
    mkdir -p "$OUTPUT_DIR"

    sleep 2
    
    # Get all nodes from JSON file for distribution of test requests
    if [[ -f "$JSON_FILE" ]]; then
        # Get all server addresses from JSON file
        mapfile -t SERVERS < <(jq -r '.[]' "$JSON_FILE")

        # Run benchmark with all servers
        if python3 chord-dynamic-benchmark.py "${SERVERS[@]}"; then
            echo "Dynamic benchmark completed successfully"
        else 
            echo "Dynamic benchmark failed with errors - stopping entire benchmark"
            echo "Logs preserved in: $LOG_DIR"
            echo " Check server logs for details:"
            exit 1  
        fi
    else
        echo "ERROR: servers.json not found"
        continue
    fi  
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
elif [[ "$COMMAND" == "reset" ]]; then
    reset
elif [[ "$COMMAND" == "benchmark-throughput" ]]; then
    NUM_SERVERS=$2
    operations=${3:-1000}
    trials=${4:-3}
    benchmark-throughput $operations $trials
elif [[ "$COMMAND" == "benchmark-dynamic" ]]; then
    NUM_SERVERS=$2
    benchmark-dynamic 
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