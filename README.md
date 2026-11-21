# High-Throughput I/O Logging System

A Go/Rust experiment for studying burst ingestion, batching, Unix sockets, and buffered disk I/O.

## Overview

This system provides a high-throughput logging pipeline where:
- **Go** runs an agent and a gRPC orchestrator with a batch processor.
- **Rust** provides a high-performance storage backend using Unix Domain Sockets (UDS) and buffered async I/O.

## Architecture

```text
VM agent --gRPC--> Go orchestrator --Unix socket--> Rust engine
                         |                              |
                   bounded queue                 buffered file writer
                                                        |
                                                   Linux filesystem
```

## Components

### Go Orchestrator (`cmd/orchestrator/main.go`)
- Starts a gRPC server on port 50099 (listening on 0.0.0.0).
- Initializes a batch processor (shock absorber) to buffer incoming logs.
- Starts a health tracker to monitor VM agent heartbeats.
- Connects to the Rust engine via UDS at `/tmp/io_rust_engine.sock`.

### gRPC Server (`internal/api/grpc_server.go`)
- Implements unary `UploadMetric` and a client-streaming handler; the bundled agent uses unary calls.
- Logs unary requests per second after submission, including records dropped by a full queue.

### HTTP Log Handler (`internal/api/log_handler.go`)
- Exists as an HTTP handler for JSON log batches via POST, but is not started by the current orchestrator.
- Validates and forwards to the Rust engine.
- Returns appropriate HTTP status codes.

### Batch Processor (`internal/service/pipeline.go`)
- Buffers logs and flushes based on batch size (20,000) or ticker (50 ms). The 200,000-entry channel drops new entries when full.
- The single worker cannot drain its queue while synchronously writing a batch.

### VM Agent (`cmd/agent/main.go`)
- Collects system metrics (CPU, memory) from `/proc/stat` and `/proc/meminfo`.
- Pushes one metric per synchronous gRPC call, then sleeps 200 µs. The declared sleep flag/environment variable is not used by the loop.
- Retries connection probes and gRPC readiness; failed probes/readiness checks wait five seconds.

### Rust Storage Engine (`rust_engine/src/main.rs`)
- **UDS Listener**: Binds to `/tmp/io_rust_engine.sock`, accepts connections, reads newline-delimited JSON.
- **Buffered Disk Writer**: Async task that writes logs to date-named files (`./logs/app_YYYY-MM-DD.log`) using a 64 KiB buffer that is flushed per entry; no fsync is performed.
- **Metrics Server**: Serves Prometheus metrics on `:9090` (configurable via `RUST_METRICS_ADDR`).
- **Features**: Connection per client, JSON validation, backpressure via channel (50k capacity), Prometheus metrics for bytes, flushes, failures and invalid input. The file is named at startup, not rotated at midnight.

## Data Flow

1. VM Agent collects CPU/memory metrics → packages as `LogEntry`.
2. Agent connects to `GRPC_SERVER_ADDR` (default `127.0.0.1:50099`).
3. Orchestrator's gRPC server receives entry → forwards to batch processor.
4. Batch processor buffers; when batch size (20,000) or ticker (50 ms) reached → sends slice to Rust engine via UDS.
5. Rust engine receives bytes, splits lines, validates JSON → forwards each line to disk writer channel.
6. Disk writer appends line to buffered writer, flushes, updates metrics.
7. Rust replies per entry, but Go does not read the replies. Go logs socket-write success, not Rust acknowledgement or durable persistence. The gRPC handler also returns success when the Go queue drops an entry.

## Build & Run

### Prerequisites
- Go 1.26.1+
- Rust + Cargo
- Linux (for `/proc` stat access and UDS)
- Docker (optional, for containerized deployment)

### Build (Local)
```bash
# Build Go agent and server (static Linux binaries) -> bin/release/
make build-agent build-server

# Build Rust engine (requires the musl target and toolchain) -> rust_engine/target/x86_64-unknown-linux-musl/release/rust_engine
make build-rust

# Or build all at once
make build
```

### Protobuf Generation
The Go gRPC code depends on compiled `.proto` definitions. To (re)generate the Go sources:

```bash
# Install protoc and plugins if needed (once)
# go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
# go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate from the proto file
protoc --go_out=. --go_opt=module=io \
  --go-grpc_out=. --go-grpc_opt=module=io grpc_impl/metrics.proto
```

Regenerate whenever `grpc_impl/metrics.proto` changes.

### Run (Local)
1. Start Rust engine:
   ```bash
   ./rust_engine/target/x86_64-unknown-linux-musl/release/rust_engine
   ```
2. Start orchestrator:
   ```bash
   ./bin/release/grpc-server   # or: go run ./cmd/orchestrator/main.go
   ```
3. Start VM agent:
   ```bash
   ./bin/release/grpc-agent    # or: go run ./cmd/agent/main.go
   ```

### Run (Containerized)
Build and run the Docker container:

```bash
# Build the Docker image
docker build -t high-throughput-io .

# Run the container (exposes gRPC on 50099, Rust metrics on 9090)
docker run -d --name htiol \
  -p 50099:50099 \
  -p 9090:9090 \
  high-throughput-io
```

The container starts both the Rust engine and the Go orchestrator automatically via the entrypoint script.
Logs can be viewed with `docker logs -f htiol`.

### Configuration
- **Agent**: `GRPC_SERVER_ADDR` (default `127.0.0.1:50099`) and `AGENT_CLIENT_ID` (default `vm-pve-host`).
- **Orchestrator**: the running pipeline uses hard-coded batch size 20,000, interval 50 ms, and socket path `/tmp/io_rust_engine.sock`; declared batch flags/environment variables are not applied.
- **Rust**: `RUST_METRICS_ADDR` (default `0.0.0.0:9090`) and `RUST_SOCKET_PATH` (default `/tmp/io_rust_engine.sock`). Changing the latter alone does not change the Go or entrypoint path.

## Current limitations

- Go's bounded queue drops records when full, but unary gRPC still reports success.
- Go writes UDS records without reading Rust's replies; socket backpressure can stall writes. Failed batches are logged and not retried.
- Rust flushes every entry to the file interface without `fsync`; this is not a durability guarantee.
- The active orchestrator does not expose Go Prometheus metrics. Rust exposes metrics on port 9090.
- The Rust filename uses the date at startup; automatic daily rotation is not implemented.
- See [ingestion review](INGESTION_PIPELINE_REVIEW.md) for source references and hypotheses.

## Extensibility
- Add new log fields by extending `LogEntry` struct in both Go and Rust.
- Replace storage engine by implementing `storage.EngineBridge` interface.
- Adjust batch size/flush interval in `cmd/orchestrator/main.go`.
- Modify Rust engine to write to different destinations (e.g., remote storage, database).

---
*See [DOCUMENTATION.md](DOCUMENTATION.md) for in-depth details.*

## QEMU Guest Agent Snippet (historical plan)

The following was documented for a `deploy-vm.sh` provisioning script. That script is not present in this checkout; retain this snippet as a deployment reference. The intended snippet location is:

```
/var/lib/vz/snippets/qemu-guest-agent.yaml
```

Contents:

```
#cloud-config
package_update: true
packages:
  - qemu-guest-agent
runcmd:
  - systemctl enable --now qemu-guest-agent
```

This file is referenced in the script via the variables `CI_SNIPPET_STORAGE` and `CI_VENDOR_FILE`, which default to `local` and `qemu-guest-agent.yaml` respectively, resulting in the path `local:snippets/qemu-guest-agent.yaml` in Proxmox VE's storage.