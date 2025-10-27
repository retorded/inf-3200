# Assignment 2 - Distributed Hash Table (DHT) Implementation

A distributed hash table system based on the Chord protocol that automatically builds, deploys, and manages multiple DHT nodes across cluster nodes.

## What It Does

- **Chord DHT**: Implements a distributed hash table using the Chord protocol
- **Key-Value Storage**: Store and retrieve key-value pairs across the distributed ring
- **Automatic Deployment**: Builds from source and deploys across cluster nodes
- **Load Balancing**: Distributes keys evenly across nodes using consistent hashing
- **Performance Benchmarking**: Throughput testing and CSV reporting

## Project Structure

```
tonyt1573/
├── src/                      # Go source code
│   ├── go.mod               # Go module definition
│   ├── cmd/server/main.go   # Application entry point
│   └── internal/            # DHT implementation
│       ├── dht/             # Chord protocol implementation
│       │   ├── node.go      # Node and finger table logic
│       │   └── helper.go    # Hash functions and interval checks
│       └── server/          # HTTP server implementation
│           └── server.go    # REST API endpoints
├── doc/                     # Documentation
│   └── group.txt           # Group members
├── run.sh                   # Main deployment script
├── Makefile                 # Build automation
├── README.md               # This file
├── chord-benchmark.py      # Performance testing script
└── build/                  # Build artifacts (auto-generated)
    ├── server              # Compiled Go binary
    ├── servers.json        # JSON list of deployed servers
    ├── benchmark.csv       # Performance test results
    └── logs/               # Server log files for debugging
        ├── server_c0-1_50153.log
        ├── server_c1-0_49001.log
        └── ...
```

## Quick Start

**Deploy a Chord ring and test it:**

For simplicity we interact with the program components through our makefile targets

```bash
# Deploy 16 DHT nodes (automatically: init + cleanup + build + deploy)
make run

# Output: ["c0-1:50153", "c1-0:49001", "c1-1:55737", "c2-0:62341"]

# Kill all servers
make kill

# Clean up
make clean
```

To perform through-put testing of the chord ring at various node sizes, we use the benchmark target.

```bash
# Run performance benchmark
make benchmark
```

## Detailed usage

```bash
# Main commands
./run.sh <number>    # Deploy N DHT nodes: init + cleanup + build + deploy
./run.sh benchmark <nodes> <operations> <trials>  # Run performance test

# Examples:
./run.sh 4           # Deploy 4 DHT nodes
./run.sh benchmark 4 1000 3  # Test 4 nodes with 1000 operations, 3 trials

# Individual operations
./run.sh init        # Initialize Go project only
./run.sh cleanup     # Kill old servers only
./run.sh build       # Build binary only
./run.sh kill        # Kill all running servers

# Makefile commands
make help            # Show all available commands
make run             # Deploy 16 DHT nodes (init + cleanup + build + deploy)
make benchmark       # Run full benchmark suite (1,2,4,8,16 nodes)
make deliverable     # Create submission zip file
```

## API Specification

**DHT Endpoints:**

### **Storage Operations**
- **PUT**: `http://hostname:port/storage/<key>`
  - **Method**: PUT
  - **Body**: Value to store
  - **Response**: 200 OK (stored) or forwarded to correct node

- **GET**: `http://hostname:port/storage/<key>`
  - **Method**: GET
  - **Response**: 200 OK with value, 404 Not Found. Server internally forwards request to correct node.

### **Network Operations**
- **Network Info**: `http://hostname:port/network`
  - **Method**: GET
  - **Response**: JSON array of all node addresses

- **Health Check**: `http://hostname:port/helloworld`
  - **Method**: GET
  - **Response**: `hostname:port` (for health checking)

**Examples:**
```bash
# Get network topology
curl http://c0-1:50153/network
# Response: ["c0-1:50153", "c1-0:49001", "c1-1:55737", "c2-0:62341"]

# Store a key-value pair
curl -X PUT http://c0-1:50153/storage/mykey -d "myvalue"

# Retrieve a value
curl http://c0-1:50153/storage/mykey
# Response: myvalue
```

## Chord Protocol Implementation

### **Key Features**
- **Consistent Hashing**: Uses SHA-1 to map keys and nodes to a 16-bit identifier space
- **Finger Tables**: Each node maintains a finger table for O(log N) lookups
- **Successor Lists**: Each node knows its immediate successor and predecessor
- **Automatic Routing**: Requests are automatically routed to the correct node

### **Node Responsibilities**
- **Key Ownership**: Each node is responsible for keys in the interval (predecessor.id, node.id]
- **Forwarding**: Keys not owned by the node are forwarded using finger table
- **Load Balancing**: Keys are distributed evenly across the ring using consistent hashing

### **Finger Table Structure**
Each node maintains a finger table with M=16 entries:
- **Entry i**: Points to the first node with ID ≥ (node.id + 2^i) mod 2^M
- **Lookup**: Uses finger table to find the closest preceding node to any key
- **Routing**: Forwards requests to the finger table entry that gets closest to the target

## How It Works

**Automatic Process:** `./run.sh 4` does:
1. **Init**: Set up Go module and dependencies
2. **Cleanup**: Kill existing servers and clean artifacts  
3. **Build**: Compile Go source to binary
4. **Deploy**: Start DHT nodes on available nodes
5. **Network Setup**: Each node receives the full network topology
6. **Ring Initialization**: Nodes calculate their position in the ring
7. **Finger Table**: Each node builds its finger table for routing
8. **Output**: Return JSON list of deployed nodes

**Chord Ring Initialization:**
1. All nodes hash their addresses to get IDs in the identifier space
2. Nodes are sorted by ID to form the ring
3. Each node determines its successor and predecessor
4. Finger tables are built using the sorted node list
5. Nodes can now route requests using the Chord protocol

## Performance Benchmarking

The system includes performance testing:

### **Benchmark Features**
- **Throughput Measurement**: Measures PUT and GET operations per second, 1000 operations per method.
- **Load Distribution**: Randomly distributes requests across all nodes
- **Scalability Testing**: Tests with 1, 2, 4, 8, and 16 nodes
- **Error Detection**: Stops immediately on errors to preserve logs for debugging
- **CSV Reporting**: Outputs results to `build/benchmark.csv`

### **Benchmark Results**
Results are saved to `build/benchmark.csv` with columns:
- `timestamp`: When the test was run
- `network_size`: Number of nodes in the ring
- `trial`: Trial number (1, 2, 3, etc.)
- `operations`: Number of operations performed
- `put_ops_per_sec`: PUT operations per second
- `get_ops_per_sec`: GET operations per second

## Build Artifacts

The `build/` directory contains generated files for debugging and monitoring:

### **`build/server`**
- Compiled Go binary ready for deployment
- Built from `src/cmd/server/main.go` and `src/internal/`

### **`build/servers.json`**
- JSON array of deployed node endpoints
- Example: `["c0-1:50153", "c1-0:49001", "c1-1:55737", "c2-0:62341"]`
- Used by benchmark scripts and for cleanup

### **`build/benchmark.csv`**
- Performance test results in CSV format
- Contains throughput measurements for different network sizes
- Used for analysis and plotting

### **`build/logs/`**
- Individual log files for each deployed node
- Format: `server_<hostname>_<port>.log`
- Contains DHT operations, routing decisions, and error messages
- Useful for debugging routing issues or node failures

**Example log file:**
```
2025/09/26 17:31:44 Node 12345 stored key 'abc123' (id: 45678) and value length 8
2025/09/26 17:31:45 Put(): Key 'xyz789' (id: 23456) not found, check address 'c1-0:49001' (id: 23456)
2025/09/26 17:31:45 Node 12345 retrieved key 'abc123' (id: 45678) and value length 8
```

**Debugging tips:**
- Check logs if nodes fail to start or respond
- Look for routing loops or incorrect finger table entries
- Verify key storage and retrieval operations
- Monitor forwarding patterns for load balancing

## Requirements

- Go 1.25.0+
- Access to IFI cluster (`ificluster.ifi.uit.no`)
- SSH access to compute nodes
- `jq` command-line JSON processor
- Python 3.6+ (for benchmarking)

## Creating Deliverable

Create a submission-ready zip file:

```bash
# Create zip file with proper structure
make deliverable
```

**Deliverable Structure:**
```
tonyt1573.zip/
└── tonyt1573/                    # Top-level folder (UiT username)
    ├── src/                      # Implementation folder
    │   ├── go.mod
    │   ├── cmd/server/main.go
    │   └── internal/
    │       ├── dht/
    │       │   ├── node.go
    │       │   └── helper.go
    │       └── server/
    │           └── server.go
    ├── doc/
    │   └── group.txt            # Group members
    ├── run.sh                   # Main deployment script
    ├── Makefile                 # Build automation
    ├── README.md               # Documentation
    ├── chord-benchmark.py      # Performance testing script
    └── testscript.py           # Basic test script
```

## Technical Details

### **Hash Function**
- Uses SHA-1 to hash node addresses and keys
- Maps to a 16-bit identifier space (0 to 65535)
- Provides good distribution for load balancing

### **Interval Logic**
- **Key Ownership**: `[predecessor.id, node.id]` (right-inclusive)
- **Finger Table**: `(node.id, key.id)` (open interval for closest preceding)
- **Wrap-around**: Handles circular nature of the identifier space

### **Thread Safety**
- Uses `sync.Map` for thread-safe key-value storage
- Handles concurrent PUT/GET operations safely
- No race conditions in finger table access

### **Error Handling**
- Automatic request forwarding when keys don't belong to current node
- Graceful handling of network timeouts
- Comprehensive logging to separate log files for debugging