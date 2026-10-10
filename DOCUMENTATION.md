# High-Throughput I/O Logging System - Detailed Documentation

## Overview
A hybrid Go/Rust system designed for extreme logging throughput. Go orchestrates API requests, worker management, and orchestration. Rust provides a high-performance disk writer utilizing Unix Domain Sockets (UDS) and buffered async I/O for minimal latency.

## Project Structure
- `cmd/`: Entry points for various binaries (orchestrator, worker, agent, benchmark).
- `internal/`: Private application code (business logic, storage bridges, config).
- `pkg/`: Public library code (telemetry, metrics, logging).
- `rust_engine/`: The Rust-based high-performance storage backend.
- `deployments/`: Infrastructure configurations (Docker, K8s).
- `scripts/`: Automation and local setup scripts.

## Core Components

### Go Orchestrator (`cmd/orchestrator/main.go`)
- Initializes storage engine (UDS client to Rust socket).
- Starts batch processor (shock absorber).
- Starts health tracker (VM heartbeats).
- Launches gRPC server on port 50099 (listening on 0.0.0.0).
- Handles graceful shutdown on SIGINT/SIGTERM.

### gRPC Server (`internal/api/grpc_server.go`)
- Exposes `/metrics.MetricsService/UploadMetric` (via generic handler).
- Unknown methods are captured and submitted as generic metrics.
- Binds to TCP address `0.0.0.0:<port>`.

### HTTP Log Handler (`internal/api/log_handler.go`)
- Accepts JSON batches via POST.
- Validates and forwards to Rust engine through `EngineBridge`.
- Returns appropriate HTTP status codes (202 Accepted, 400 Bad Request, 503 Service Unavailable, etc.).
- Handles context cancellation and storage errors.

### Batch Processor / Shock Absorber (`internal/service/pipeline.go` - assumed, not directly read but referenced)
- Buffers incoming logs and flushes based on batch size or timeout.
- Implemented via `service.NewBatchProcessor(engine, batchSize, flushInterval)`.
- Provides `Submit(entry)` and `Start()/Stop()` methods.

### Health Tracker (`internal/service/health.go` - assumed)
- Tracks VM agent heartbeats.
- Exposes health metrics.

### Config (`internal/config/config.go`)
- Loads environment variables: `APP_ENV`, `APP_PORT`, `LOG_LEVEL`.
- Provides defaults: development, 8080, info.

### VM Agent (`cmd/agent/main.go`)
- Collects system metrics (CPU, memory from `/proc/stat` and `/proc/meminfo`).
- Pushes metrics via gRPC to orchestrator (`/metrics.MetricsService/UploadMetric`).
- Implements connection logic:
  1. Test raw TCP connectivity.
  2. Establish gRPC connection with retry.
  3. Wait for gRPC ready state.
  4. Invoke UploadMetric every 2 seconds.
  5. On failure, close connection and retry after backoff.
- Uses client ID `vm-pve-host` and server address `192.168.1.27:50099` (hardcoded for example).

### Rust Storage Engine (`rust_engine/src/main.rs`)
- Binds to Unix Domain Socket at `/tmp/io_rust_engine.sock`.
- Spawns three async tasks:
  1. **Background Buffered Disk Writer**:
     - Receives `PendingLog` via mpsc channel (capacity 50,000).
     - Writes to daily log file (`./logs/app_YYYY-MM-DD.log`).
     - Uses `BufWriter` with 64KB buffer.
     - Flushes after each write; measures flush duration.
     - Tracks metrics: bytes written, write/flush failures.
  2. **Metrics HTTP Server**:
     - Serves Prometheus metrics on address from `RUST_METRICS_ADDR` (default `0.0.0.0:9090`).
     - Exposes counters: `RUST_WRITE_BYTES_TOTAL`, `RUST_DISK_FLUSH_DURATION`, `RUST_DISK_WRITE_FAILURES_TOTAL`, `RUST_DISK_FLUSH_FAILURES_TOTAL`, `RUST_INVALID_LOGS_TOTAL`.
  3. **UDS Socket Listener**:
     - Accepts connections, spawns task per connection.
     - Reads newline-delimited JSON lines.
     - Deserializes into `LogEntry` struct; validates required fields.
     - Sends line to disk writer channel, waits for ack via oneshot.
     - Responds `OK\n` on success, `ERR <message>\n` on failure.
     - Increments invalid logs metric on JSON parse failure.
- Uses Tokio async runtime, dependencies: tokio (full), serde/serde_json, chrono, prometheus, lazy_static.

### Log Entry Structure (Shared)
```go
type LogEntry struct {
    ClientID  string                 `json:"client_id"`
    TimeStamp int64                  `json:"timestamp"`
    Level     string                 `json:"level"`
    Message   string                 `json:"message"`
    Payload   map[string]interface{} `json:"payload,omitempty"`
}
```
In Rust:
```rust
#[derive(Debug, Deserialize)]
struct LogEntry {
    client_id: String,
    timestamp: i64,
    level: String,
    message: String,
    #[serde(default)]
    payload: Option<Value>,
}
```

## Data Flow
1. **VM Agent** collects CPU/Memory metrics, packages as `LogEntry`.
2. Agent connects via gRPC to orchestrator (`192.168.1.27:50099`).
3. Orchestrator's gRPC server receives the entry, forwards to batch processor.
4. Batch processor buffers; when batch size (100) or timeout (1s) reached, sends slice to Rust engine via `UDSEngine.WriteBatch`.
5. `UDSEngine` (Go) connects to UDS (`/tmp/io_rust_engine.sock`), marshals slice to JSON (newline-delimited), writes to socket.
6. **Rust Engine** receives bytes, splits lines, validates JSON, forwards each line to disk writer channel.
7. Disk writer appends line to buffered writer, flushes, updates metrics.
8. Rust engine replies `OK\n` or `ERR\n` over the socket; Go side logs success/failure.

## Reliability Features
- **Connection Retries**: Agent retries TCP/gRPC on failure with 5s backoff.
- **Circuit Breaker**: If gRPC handshake fails, connection closed and retried.
- **Backpressure**: Batch processor absorbs spikes; if Rust engine slow, channel may block (unbounded in Go? Actually channel in Rust is buffered 50k; Go side writes synchronously; if Rust slow, write may block or timeout after 2s).
- **Error Handling**: Rust engine reports errors via socket response; Go logs and closes connection on write error.
- **Metrics**: Both Go (via Prometheus client in orchestrator? Not seen) and Rust expose Prometheus metrics.
- **Log Rotation**: Daily log files based on date; manual rotation needed (or external logrotate).
- **Resource Cleanup**: Sockets removed on startup; connections closed on error/shutdown.

## Build & Run
### Go Components
```bash
make build-agent   # builds vm-agent binary (static Linux)
# Orchestrator, worker, etc. built via `go build ./cmd/...`
```

### Rust Component
```bash
cd rust_engine
cargo build --release   # produces target/release/rust_engine
```

### Execution
1. Start Rust engine: `./target/release/rust_engine` (ensures `/tmp/io_rust_engine.sock` created).
2. Start orchestrator: `./cmd/orchestrator/orchestrator` (or `go run ./cmd/orchestrator/main.go`).
3. Start VM agent: `./vm-agent` (or `go run ./cmd/agent/main.go`).

### Environment Variables
- Go: `APP_ENV`, `APP_PORT`, `LOG_LEVEL`.
- Rust: `RUST_METRICS_ADDR` (default `:9090`).

## Dependencies
### Go
- `github.com/prometheus/client_golang v1.24.1`
- `google.golang.org/grpc v1.84.0`

### Rust
- `tokio = { version = "1.38", features = ["full"] }`
- `serde = { version = "1.0", features = ["derive"] }`
- `serde_json = "1.0"`
- `chrono = "0.4"`
- `prometheus = "0.13.4"`
- `lazy_static = "1.4"`

## Configuration Files
- `.env.example`: Example environment file.
- `Makefile`: Contains `build-agent` and `clean` targets.

## Extensibility
- Add new metric types by extending `LogEntry` struct.
- Replace storage engine by implementing `storage.EngineBridge` interface.
- Adjust batch size/flush interval in orchestrator.
- Modify Rust engine to write to different destinations (e.g., database, remote storage).

## Notes
- The system assumes a trusted network; no authentication/encryption on gRPC or UDS.
- For production, consider adding TLS to gRPC and using SASL or similar for UDS.
- The VM agent hardcodes client ID and server address; in practice, these should be configurable.
- Log files are stored in `./logs/` relative to the Rust engine working directory.
- The orchestrator listens on all interfaces (`0.0.0.0:50099`); adjust firewall accordingly.

---
*Documentation generated from source code review.*