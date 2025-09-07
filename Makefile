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
	@echo "  make run       - Run $(N) servers (depends on init + cleanup + build)"
	@echo "  make test      - Run servers and test with Python script"
	@echo "  make kill      - Kill all running servers"
	@echo "  make deliverable - Create zip file with proper folder structure (tonyt1573.zip/tonyt1573/)"
	@echo "  make deliverable D=-d - Create zip file and download to local Downloads folder"

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
run: init cleanup build
	@chmod +x run.sh
	@./run.sh $(N)

# Test: run servers, test with Python script, then kill servers
.PHONY: test
test: run
	@echo "Testing deployed servers..."
	@if [ -f build/servers.json ]; then \
		SERVERS=$$(cat build/servers.json); \
		echo "Testing with: $$SERVERS"; \
		echo "Waiting 1 second for all servers to be ready..."; \
		sleep 1; \
		python3 testscript.py "$$SERVERS"; \
		echo "Test completed. Cleaning up servers..."; \
		$(MAKE) kill; \
	else \
		echo "Error: servers.json not found."; \
		exit 1; \
	fi

# Kill all running servers
.PHONY: kill
kill:
	@chmod +x run.sh
	@./run.sh kill

# Create deliverable zip file with proper folder structure
.PHONY: deliverable
deliverable:
	@echo "Creating deliverable zip file with proper folder structure..."
	@USERNAME="tonyt1573"; \
	ZIP_NAME="$$USERNAME.zip"; \
	TEMP_DIR="$$USERNAME"; \
	rm -f "$$ZIP_NAME"; \
	rm -rf "$$TEMP_DIR"; \
	mkdir -p "$$TEMP_DIR"; \
	cp -r src doc run.sh Makefile README.md testscript.py "$$TEMP_DIR/" 2>/dev/null || true; \
	zip -r "$$ZIP_NAME" "$$TEMP_DIR"; \
	rm -rf "$$TEMP_DIR"; \
	echo "Deliverable created: $$ZIP_NAME"; 
