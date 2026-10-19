# High-Throughput I/O Logging System

A hybrid Go/Rust system designed for extreme logging throughput.

## Overview

This system provides a high-throughput logging pipeline where:
- **Go** handles orchestration, API services (gRPC/HTTP), and worker management.
- **Rust** provides a high-performance storage backend using Unix Domain Sockets (UDS) and buffered async I/O for minimal latency and maximum throughput.

## Architecture

```
+----------------+     gRPC/HTTP     +---------------------+     UDS     +-------------------+
||   VM Agents    | ----------------> |   Orchestrator      | ---------> |  Rust Engine      ||
|| (cpu/mem metrics) |   (port 50099)  | (batch processor,  |   socket    | (UDS listener +   |
|+----------------+                   |  health tracker)    |            |  buffered writer)  |
|                                     +---------------------+            +-------------------+
|                                                                     |
|                                                                     v
|                                                       +-------------------+
|                                                       |   Log Files       |
|                                                       | (daily rotated)   |
|                                                       +-------------------+
```

## Components

### Go Orchestrator (`cmd/orchestrator/main.go`)
- Starts a gRPC server on port 50099 (listening on 0.0.0.0).
- Initializes a batch processor (shock absorber) to buffer incoming logs.
- Starts a health tracker to monitor VM agent heartbeats.
- Connects to the Rust engine via UDS at `/tmp/io_rust_engine.sock`.

### gRPC Server (`internal/api/grpc_server.go`)
- Handles `UploadMetric` RPC (via generic handler for flexibility).
- Captures unknown methods and logs them as generic metrics.

### HTTP Log Handler (`internal/api/log_handler.go`)
- Accepts JSON log batches via POST.
- Validates and forwards to the Rust engine.
- Returns appropriate HTTP status codes.

### Batch Processor (`internal/service/pipeline.go` - referenced)
- Buffers logs and flushes based on batch size (5000) or timeout (200ms).
- Implements the shock absorber pattern to protect the Rust engine from traffic spikes.

### VM Agent (`cmd/agent/main.go`)
- Collects system metrics (CPU, memory) from `/proc/stat` and `/proc/meminfo`.
- Pushes metrics to the orchestrator via gRPC every 1ms (configurable in code).
- Implements connection retry logic with exponential backoff.

### Rust Storage Engine (`rust_engine/src/main.rs`)
- **UDS Listener**: Binds to `/tmp/io_rust_engine.sock`, accepts connections, reads newline-delimited JSON.
- **Buffered Disk Writer**: Async task that writes logs to daily files (`./logs/app_YYYY-MM-DD.log`) using a 64KB buffer.
- **Metrics Server**: Serves Prometheus metrics on `:9090` (configurable via `RUST_METRICS_ADDR`).
- **Features**: Connection per client, JSON validation, backpressure via channel (50k capacity), detailed Prometheus metrics.

## Data Flow

1. VM Agent collects CPU/memory metrics → packages as `LogEntry`.
2. Agent connects via gRPC to orchestrator (`192.168.1.27:50099`).
3. Orchestrator's gRPC server receives entry → forwards to batch processor.
4. Batch processor buffers; when batch size (5000) or timeout (200ms) reached → sends slice to Rust engine via UDS.
5. Rust engine receives bytes, splits lines, validates JSON → forwards each line to disk writer channel.
6. Disk writer appends line to buffered writer, flushes, updates metrics.
7. Rust engine replies `OK\n` or `ERR\n` over the socket; Go side logs success/failure.

## Build & Run

### Prerequisites
- Go 1.26.1+
- Rust + Cargo
- Linux (for `/proc` stat access and UDS)

### Build
```bash
# Build Go agent (static binary)
make build-agent

# Build all Go components
go build ./cmd/...

# Build Rust engine (release)
cd rust_engine
cargo build --release
```

### Protobuf Generation
The Go gRPC code depends on compiled `.proto` definitions. To (re)generate the Go sources:

```bash
# Install protoc and plugins if needed (once)
# go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
# go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate from the proto file
protoc --go_out=. --go-grpc_out=. grpc_impl/metrics.proto
# Move generated files to the expected package location
mv grpc_impl/*.pb.go internal/api/
```

Regenerate whenever `grpc_impl/metrics.proto` changes.

### Run
1. Start Rust engine:
   ```bash
   ./rust_engine/target/release/rust_engine
   ```
2. Start orchestrator:
   ```bash
   ./cmd/orchestrator/orchestrator   # or: go run ./cmd/orchestrator/main.go
   ```
3. Start VM agent:
   ```bash
   ./vm-agent   # or: go run ./cmd/agent/main.go
   ```

### Configuration
- **Go**: Set environment variables `APP_ENV`, `APP_PORT`, `LOG_LEVEL`.
- **Rust**: Set `RUST_METRICS_ADDR` for Prometheus metrics endpoint (default `:9090`).

## Key Features
- **High Throughput**: Buffered async I/O in Rust, batch processing in Go.
- **Resilience**: Connection retries, circuit breakers, backpressure handling.
- **Observability**: Prometheus metrics from both Go (via orchestrator) and Rust components.
- **Scalability**: Horizontal scaling of VM agents; orchestrator handles many connections.
- **Daily Log Rotation**: Log files named by date (manual logrotate recommended).
- **Minimal Dependencies**: Clean, focused dependency lists.

## Extensibility
- Add new log fields by extending `LogEntry` struct in both Go and Rust.
- Replace storage engine by implementing `storage.EngineBridge` interface.
- Adjust batch size/flush interval in `cmd/orchestrator/main.go`.
- Modify Rust engine to write to different destinations (e.g., remote storage, database).

---
*See [DOCUMENTATION.md](DOCUMENTATION.md) for in-depth details.*