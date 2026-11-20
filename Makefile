.PHONY: build-agent build-server build-rust build clean clean-go clean-rust scp-agent scp-server scp-rust scp-all

# Directories
BIN_DIR := bin/release

# Go agent
AGENT_BIN := $(BIN_DIR)/grpc-agent
AGENT_SRC := cmd/agent/main.go

# Go server (orchestrator)
SERVER_BIN := $(BIN_DIR)/grpc-server
SERVER_SRC := cmd/orchestrator/main.go

# Rust engine (built in place)
RUST_SRC := rust_engine
RUST_TARGET := x86_64-unknown-linux-musl

# Go binary path (adjust if necessary)
GO_BIN ?= go

# Ensure bin/release directory exists
$(BIN_DIR):
	mkdir -p $@

# Build Go agent (static Linux binary)
build-agent: $(BIN_DIR)
	@echo "🔨 Building VM Agent (Static Linux Binary)..."
	CGO_ENABLED=0 GOOS=linux $(GO_BIN) build -o $(AGENT_BIN) $(AGENT_SRC)
	@echo "✅ Agent built successfully: $(AGENT_BIN)"

# Build Go server (orchestrator) (static Linux binary)
build-server: $(BIN_DIR)
	@echo "🔨 Building gRPC Server (Static Linux Binary)..."
	CGO_ENABLED=0 GOOS=linux $(GO_BIN) build -o $(SERVER_BIN) $(SERVER_SRC)
	@echo "✅ Server built successfully: $(SERVER_BIN)"

# Build Rust engine (statically linked via musl)
build-rust:
	@echo "🔨 Building Rust Engine (statically linked via musl)..."
	cd $(RUST_SRC) && cargo build --release --target $(RUST_TARGET)
	@echo "✅ Rust engine built successfully: $(RUST_SRC)/target/$(RUST_TARGET)/release/rust_engine"

# Build Go server and agent
build-go: build-agent build-server
	@echo "🎉 Build complete:"
	@echo "   Go agent:   $(AGENT_BIN)"
	@echo "   Go server:  $(SERVER_BIN)"

# Build all components
build: build-agent build-server build-rust
	@echo "🎉 Build complete:"
	@echo "   Go agent:   $(AGENT_BIN)"
	@echo "   Go server:  $(SERVER_BIN)"
	@echo "   Rust engine: $(RUST_SRC)/target/$(RUST_TARGET)/release/rust_engine"

# Clean Go binaries (only Go binaries in bin/)
clean-go:
	@echo "🧹 Cleaning Go binaries..."
	rm -rf bin
	@echo "✅ Cleaned bin directory"

# Clean Rust build artifacts
clean-rust:
	@echo "🧹 Cleaning Rust build artifacts..."
	cd $(RUST_SRC) && cargo clean
	@echo "✅ Cleaned Rust target directory"

# Clean all generated binaries
clean: clean-go clean-rust
	@echo "✅ All cleaned"


# SCP configuration (override via environment or command line)
SCP_KEY  ?=  # e.g., ~/.ssh/id_rsa
SCP_USER ?=  # e.g., ubuntu
SCP_HOST ?=  # e.g., 192.168.1.100
SCP_DEST ?=  # e.g., /home/ubuntu/deploy

# Helper to build scp command if variables are set
define scp-cmd
	if [ -n "$(SCP_KEY)" -a -n "$(SCP_USER)" -a -n "$(SCP_HOST)" -a -n "$(SCP_DEST)" ]; then \
		scp -i "$(SCP_KEY)" "$(1)" "$(SCP_USER)@$(SCP_HOST):$(SCP_DEST)/"; \
	else \
		@echo "⚠️  SCP variables not fully set. Set SCP_KEY, SCP_USER, SCP_HOST, SCP_DEST to use scp targets."; \
		exit 1; \
	fi
endef

# SCP agent binary
scp-agent: build-agent
	@echo "📤 Copying Go agent to remote host..."
	$(call scp-cmd,$(AGENT_BIN))

# SCP server binary
scp-server: build-server
	@echo "📤 Copying Go server to remote host..."
	$(call scp-cmd,$(SERVER_BIN))

# SCP rust binary
scp-rust: build-rust
	@echo "📤 Copying Rust engine to remote host..."
	$(call scp-cmd,$(RUST_BIN))

# SCP all binaries
scp-all: build
	@echo "📤 Copying all binaries to remote host..."
	$(call scp-cmd,$(AGENT_BIN) $(SERVER_BIN) $(RUST_BIN))