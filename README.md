# Assignment 1 Part A - Distributed HTTP Server System

A distributed HTTP server system that automatically builds, deploys, and manages multiple servers across cluster nodes.

## What It Does

- **HTTP Server**: Responds to GET `/helloworld` with `hostname:port`
- **Automatic Deployment**: Builds from source and deploys across cluster nodes
- **Smart Distribution**: Uses round-robin when servers > available nodes
- **Zero Configuration**: First run handles everything automatically

## Project Structure

```
tonyt1573/
├── src/                      # Go source code
│   ├── go.mod               # Go module definition
│   ├── cmd/server/main.go   # Application entry point
│   └── internal/server/     # Server implementation
│       └── server.go
├── doc/                     # Documentation
│   └── group.txt           # Group members
├── run.sh                   # Main deployment script
├── Makefile                 # Build automation
├── README.md               # This file
├── testscript.py           # Test script
└── build/                  # Build artifacts (auto-generated)
    ├── server              # Compiled Go binary
    ├── servers.json        # JSON list of deployed servers
    └── logs/               # Server log files for debugging
        ├── server_c0-1_50153.log
        ├── server_c1-0_49001.log
        └── ...
```

## Quick Start

**One command does everything:** builds from source, deploys servers, returns JSON list.

```bash
# Deploy 3 servers (automatically: init + cleanup + build + deploy)
./run.sh 3

# Output: ["c0-1:50153", "c1-0:49001", "c1-1:55737"]

# Test the deployment
python3 testscript.py '["c0-1:50153", "c1-0:49001", "c1-1:55737"]'

# Clean up
./run.sh kill
```

## Usage

```bash
# Main command (does everything automatically)
./run.sh <number>    # Deploy N servers: init + cleanup + build + deploy

# Examples:
./run.sh 3           # Deploy 3 servers
./run.sh 10          # Deploy 10 servers

# Individual operations (if needed)
./run.sh init        # Initialize Go project only
./run.sh cleanup     # Kill old servers only
./run.sh build       # Build binary only
./run.sh kill        # Kill all running servers

# Makefile commands
make help            # Show all available commands
make run             # Deploy 5 servers (init + cleanup + build + deploy)
make test            # Deploy servers, test, and cleanup
make deliverable     # Create submission zip file
```

## API Specification

**HTTP Endpoint:**
- **URL**: `http://hostname:port/helloworld`
- **Method**: GET
- **Response**: `hostname:port` (e.g., `c0-1:50153`)

**Example:**
```bash
curl http://c0-1:50153/helloworld
# Response: c0-1:50153
```

## How It Works

**Automatic Process:** `./run.sh 3` does:
1. **Init**: Set up Go module and dependencies
2. **Cleanup**: Kill existing servers and clean artifacts  
3. **Build**: Compile Go source to binary
4. **Deploy**: Start servers on available nodes
5. **Output**: Return JSON list of deployed servers

**Distribution Logic:**
- Gets available nodes from `/share/ifi/available-nodes.sh`
- Uses round-robin: `NODE = NODES[i % NUM_NODES]`
- Finds free ports in range 49152-65535
- Only multiple servers per node when servers > available nodes

## Testing Round-Robin Distribution

To test with limited nodes, uncomment line 80 in `run.sh`:
```bash
# For testing round-robin: use only first 3 nodes
NODES=("${NODES[@]:0:3}")
```

Then run: `./run.sh 10` (will distribute 10 servers across 3 nodes)

## Build Artifacts

The `build/` directory contains generated files for debugging and monitoring:

### **`build/server`**
- Compiled Go binary ready for deployment
- Built from `src/cmd/server/main.go` and `src/internal/server/`

### **`build/servers.json`**
- JSON array of deployed server endpoints
- Example: `["c0-1:50153", "c1-0:49001", "c1-1:55737"]`
- Used by test scripts and for cleanup

### **`build/logs/`**
- Individual log files for each deployed server
- Format: `server_<hostname>_<port>.log`
- Contains server startup, request logs, and error messages
- Useful for debugging connection issues or server failures

**Example log file:**
```
2025/09/07 14:32:28 Server running on hostname: c0-1
2025/09/07 14:32:28 Server starting on port 50153
2025/09/07 14:32:30 Request received from 172.21.21.1:45638
2025/09/07 14:32:30 Shutting down server...
2025/09/07 14:32:30 Server exited cleanly
```

**Debugging tips:**
- Check logs if servers fail to start or respond
- Look for port conflicts or connection errors
- Verify server startup and shutdown messages

## Requirements

- Go 1.25.0+
- Access to IFI cluster (`ificluster.ifi.uit.no`)
- SSH access to compute nodes
- `jq` command-line JSON processor

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
    │   └── internal/server/server.go
    ├── doc/
    │   └── group.txt            # Group members
    ├── run.sh                   # Main deployment script
    ├── Makefile                 # Build automation
    ├── README.md               # Documentation
    └── testscript.py           # Test script
```


PLAN

1. Pass all nodes as input to all nodes "[c8-11:75748, c11-9:4646 etc]"
2. Convert to golang slice []string
3. Convert all to ring id and sort to ascending order
4. Create finger tables for the node with length M.