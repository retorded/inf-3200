# Simple Makefile for easier testing

# Number of servers to run
N=5

# Default target
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make init      - Initialize Go project (go mod tidy, create directories)"
	@echo "  make cleanup   - Kill old servers and clean build artifacts"
	@echo "  make build     - Build the server binary"
	@echo "  make run       - Run $(N) servers (init + cleanup + build + run)"
	@echo "  make kill      - Kill all running servers"

# Initialize Go project
.PHONY: init
init:
	@chmod +x run.sh
	@./run.sh init

# Cleanup old servers and build artifacts
.PHONY: cleanup
cleanup:
	@chmod +x run.sh
	@./run.sh cleanup

# Build the server binary
.PHONY: build
build:
	@chmod +x run.sh
	@./run.sh build

# Run servers (init + cleanup + build + run all in one script)
.PHONY: run
run:
	@chmod +x run.sh
	@./run.sh $(N)

# Kill all running servers
.PHONY: kill
kill:
	@chmod +x run.sh
	@./run.sh kill
