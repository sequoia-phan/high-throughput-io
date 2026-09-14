#!/usr/bin/env bash
# Start the Rust disk engine and the Go HTTP gateway for local development.
set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
rust_dir="$project_dir/rust_engine"
socket_path="/tmp/io_rust_engine.sock"
rust_pid=""
go_pid=""

cd "$project_dir"

# Local configuration is optional. Values already exported in the shell win.
if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

for command in cargo go; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "Error: '$command' is required but was not found in PATH." >&2
    exit 1
  fi
done

cleanup() {
  trap - EXIT INT TERM
  [[ -n "$go_pid" ]] && kill "$go_pid" 2>/dev/null || true
  [[ -n "$rust_pid" ]] && kill "$rust_pid" 2>/dev/null || true
  [[ -n "$go_pid" ]] && wait "$go_pid" 2>/dev/null || true
  [[ -n "$rust_pid" ]] && wait "$rust_pid" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo "Starting Rust engine (metrics: http://localhost:9090/metrics)..."
(cd "$rust_dir" && cargo run) &
rust_pid=$!

for _ in {1..100}; do
  if [[ -S "$socket_path" ]]; then
    break
  fi
  if ! kill -0 "$rust_pid" 2>/dev/null; then
    echo "Error: Rust engine exited before creating $socket_path." >&2
    exit 1
  fi
  sleep 0.1
done

if [[ ! -S "$socket_path" ]]; then
  echo "Error: timed out waiting for Rust engine socket: $socket_path" >&2
  exit 1
fi

echo "Starting Go orchestrator (API: http://localhost:${APP_PORT:-8080})..."
go run ./cmd/orchestrator &
go_pid=$!

wait "$go_pid"
