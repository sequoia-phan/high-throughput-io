#!/usr/bin/env bash
# Start the Rust disk engine and the Go gRPC Orchestrator for local development.
# Improved to use built binaries if available, otherwise fall back to `go run` and `cargo run`.
set -euo pipefail

# Get the absolute path of the project root
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUST_DIR="$PROJECT_DIR/rust_engine"
SOCKET_PATH="/tmp/io_rust_engine.sock"
RUST_PID=""
GO_PID=""

cd "$PROJECT_DIR"

# Dependency check
for command in cargo go; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "Error: $command is required but was not found in PATH." >&2
    exit 1
  fi
done

cleanup() {
  trap - EXIT INT TERM
  [[ -n "$GO_PID" ]] && kill "$GO_PID" 2>/dev/null || true
  [[ -n "$RUST_PID" ]] && kill "$RUST_PID" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo "--- [1/2] Starting Rust Engine ---"
echo "Expect socket at: $SOCKET_PATH"

# Check if we have a pre-built Rust binary (release build)
if [[ -f "$RUST_DIR/target/release/rust_engine" ]]; then
  echo "Using pre-built Rust binary: $RUST_DIR/target/release/rust_engine"
  (cd "$RUST_DIR" && ./target/release/rust_engine) &
else
  echo "Building and running Rust engine with cargo run (dev mode)..."
  (cd "$RUST_DIR" && cargo run) &
fi
RUST_PID=$!

# Wait for Rust socket to be created before starting Go
echo "Waiting for Rust socket..."
for i in {1..50}; do
  if [[ -S "$SOCKET_PATH" ]]; then
    echo "✅ Rust engine socket created."
    break
  fi
  if ! kill -0 "$RUST_PID" 2>/dev/null; then
    echo "Error: Rust engine exited prematurely." >&2
    exit 1
  fi
  sleep 0.2
done

if [[ ! -S "$SOCKET_PATH" ]]; then
  echo "Error: Timed out waiting for Rust engine socket at $SOCKET_PATH." >&2
  exit 1
fi

echo "--- [2/2] Starting Go Orchestrator ---"
# Check if we have a pre-built Go server binary
if [[ -f "$PROJECT_DIR/bin/release/grpc-server" ]]; then
  echo "Using pre-built Go server binary: $PROJECT_DIR/bin/release/grpc-server"
  "$PROJECT_DIR/bin/release/grpc-server" &
else
  echo "Building and running Go orchestrator with go run (dev mode)..."
  go run ./cmd/orchestrator/main.go &
fi
GO_PID=$!

# Keep script alive until Go process exits
wait "$GO_PID"