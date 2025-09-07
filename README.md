# Assignment 1 Part A - Distributed HTTP Server System

A distributed HTTP server system implemented in Go that automatically deploys multiple servers across cluster nodes with round-robin distribution.

## Overview

This project implements a simple HTTP server that responds to `/helloworld` requests with the server's hostname and port. The system includes automated deployment across multiple cluster nodes with intelligent load distribution.

## Features

- **HTTP Server**: Responds to GET requests at `/helloworld` with `hostname:port`
- **Distributed Deployment**: Automatically deploys servers across available cluster nodes
- **Round-Robin Distribution**: Distributes servers evenly across nodes
- **Automatic Port Management**: Finds free ports in the ephemeral range (49152-65535)
- **Graceful Shutdown**: Handles SIGTERM and SIGINT signals properly
- **Zero-Configuration**: Runs without manual intervention on first execution

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

## Prerequisites

- Go 1.25.0 or later
- Access to the IFI cluster (`ificluster.ifi.uit.no`)
- SSH access to compute nodes
- `jq` command-line JSON processor

## Quick Start

### 1. Deploy Servers

```bash
# Deploy 5 servers across available nodes
./run.sh 5

# Output example:
# ["c0-1:50153", "c1-0:49001", "c1-1:55737", "c2-0:61234", "c2-1:54321"]
```

### 2. Test the Deployment

```bash
# Test with the provided test script
python3 testscript.py '["c0-1:50153", "c1-0:49001", "c1-1:55737"]'

# Expected output:
# received "c0-1:50153"
# received "c1-0:49001"
# received "c1-1:55737"
# Success!
```

### 3. Clean Up

```bash
# Kill all running servers
./run.sh kill
```

## Usage

### Command Line Interface

```bash
# Deploy N servers
./run.sh <number>

# Examples:
./run.sh 3     # Deploy 3 servers
./run.sh 10    # Deploy 10 servers

# Individual operations
./run.sh init     # Initialize Go project
./run.sh cleanup  # Kill old servers and clean artifacts
./run.sh build    # Build server binary
./run.sh kill     # Kill all running servers
```

### Makefile Commands

```bash
make help      # Show available commands
make run       # Deploy 5 servers (default)
make kill      # Kill all servers
make init      # Initialize project
make cleanup   # Clean up old servers
make build     # Build binary
```

## API Specification

### HTTP Endpoint

- **URL**: `http://hostname:port/helloworld`
- **Method**: GET
- **Response**: `hostname:port` (e.g., `c0-1:50153`)

### Example Request/Response

```bash
curl http://c0-1:50153/helloworld
# Response: c0-1:50153
```

## Architecture

### Server Distribution

The system uses round-robin distribution to ensure even load across available nodes:

- **Available nodes**: Retrieved from `/share/ifi/available-nodes.sh`
- **Distribution**: `NODE = NODES[i % NUM_NODES]`
- **Port allocation**: Random ports in range 49152-65535
- **Conflict resolution**: Checks port availability before deployment

### Deployment Process

1. **Initialize**: Set up Go module and dependencies
2. **Cleanup**: Kill existing servers and clean artifacts
3. **Build**: Compile Go source to binary
4. **Deploy**: Start servers on available nodes
5. **Output**: Return JSON list of deployed servers

## Development

### Building from Source

```bash
# Initialize Go module
go mod init assignment1

# Install dependencies
go mod tidy

# Build binary
go build -o build/server cmd/server/main.go
```

### Testing Round-Robin Distribution

To test with limited nodes, uncomment line 80 in `run.sh`:

```bash
# For testing round-robin: use only first 3 nodes
NODES=("${NODES[@]:0:3}")
```

Then run with more servers than nodes:
```bash
./run.sh 10  # Will distribute 10 servers across 3 nodes
```

## Cluster Integration

### Required Cluster Scripts

- `/share/ifi/available-nodes.sh` - Lists available compute nodes
- SSH access to compute nodes (passwordless)

### Node Requirements

- Compute nodes (cX-Y format)
- Ephemeral port range available (49152-65535)
- Go runtime environment

## Troubleshooting

### Common Issues

1. **Port conflicts**: The system automatically finds free ports
2. **Node unavailability**: Script handles node failures gracefully
3. **Build failures**: Check Go installation and dependencies

### Debug Mode

Enable verbose logging by modifying the script or checking log files in `build/logs/`.

## Assignment Compliance

This implementation fulfills all assignment requirements:

- HTTP server responding to `/helloworld`
- Automatic deployment across cluster nodes
- Round-robin distribution when servers > nodes
- JSON output format
- Zero-configuration first run
- Proper port management
- Integration with cluster infrastructure

## Group Members

See `doc/group.txt` for group member information.

## License

This project is part of INF-3200 Distributed Systems Fundamentals assignment.