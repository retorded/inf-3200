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
├── run.sh                    # Main deployment script
├── go.mod                    # Go module definition
├── cmd/server/main.go        # Application entry point
├── internal/server/server.go # Server implementation
├── Makefile                  # Build automation
├── build/                    # Build artifacts (auto-generated)
└── doc/
    ├── group.txt            # Group members
    └── assignment.txt       # Assignment specification
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

## Requirements

- Go 1.25.0+
- Access to IFI cluster (`ificluster.ifi.uit.no`)
- SSH access to compute nodes
- `jq` command-line JSON processor

## Assignment Compliance

Fulfills all requirements:
- HTTP server responding to `/helloworld`
- Automatic deployment across cluster nodes  
- Round-robin distribution when servers > nodes
- JSON output format
- Zero-configuration first run
- Proper port management
- Integration with cluster infrastructure