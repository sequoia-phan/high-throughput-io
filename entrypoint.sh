#!/bin/sh
# Entrypoint for the high-throughput-io container
# Starts the Rust engine and the Go gRPC orchestrator

set -e

# Define paths
RUST_BIN="/bin/rust_engine"
GO_SERVER_BIN="/bin/grpc-server"
SOCKET_PATH="/tmp/io_rust_engine.sock"

# Start Rust engine in background
echo "Starting Rust engine..."
$RUST_BIN &
RUST_PID=$!

# Wait for the Unix domain socket to be created
echo "Waiting for Rust socket at $SOCKET_PATH..."
for i in {1..50}; do
  if [ -S "$SOCKET_PATH" ]; then
    echo "Rust socket created."
    break
  fi
  # Check if Rust process is still alive
  if ! kill -0 $RUST_PID 2>/dev/null; then
    echo "Rust engine exited unexpectedly!" >&2
    exit 1
  fi
  sleep 0.2
done

if [ ! -S "$SOCKET_PATH" ]; then
  echo "Timed out waiting for Rust socket." >&2
  kill $RUST_PID 2>/dev/null || true
  exit 1
fi

# Start Go orchestrator in background
echo "Starting Go orchestrator..."
$GO_SERVER_BIN &
GO_PID=$!

# Wait for either process to exit
wait -n
EXIT_CODE=$?

# Propagate exit code
echo "One of the processes exited with code $EXIT_CODE. Shutting down."
kill $RUST_PID $GO_PID 2>/dev/null || true
wait $RUST_PID 2>/dev/null || true
wait $GO_PID 2>/dev/null || true
exit $EXIT_CODE