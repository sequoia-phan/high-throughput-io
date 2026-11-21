# High-Throughput I/O Logging System — Detailed Documentation

This project experiments with Go-to-Rust ingestion, batching, Unix domain sockets and buffered file writes. The current runnable backend is the Go gRPC orchestrator and Rust engine; the Go agent runs separately. See [INGESTION_PIPELINE_REVIEW.md](INGESTION_PIPELINE_REVIEW.md) for the detailed source review and unverified performance hypotheses.

## Current data flow

1. `cmd/agent/main.go` reads CPU and memory information from `/proc` and invokes unary `UploadMetric` once per entry. It waits for the response, logs it and sleeps 200 µs. The RPC has a two-second deadline. On failure it reconnects; failed TCP probes or readiness checks wait five seconds. `GRPC_SERVER_ADDR` defaults to `127.0.0.1:50099`, and `AGENT_CLIENT_ID` defaults to `vm-pve-host`. The declared sleep flag/environment setting is not used by the loop.
2. `internal/api/grpc_server.go` converts each request to a `LogEntry`, marks the client seen, and submits it to the batch processor. It returns success after `Submit` even if the queue was full and the entry was dropped. The requests/sec log counts unary submissions, including drops, rather than disk writes. A streaming handler also exists, but the bundled agent does not use it; the handler currently returns the receive error at stream end without sending a success response.
3. `internal/service/pipeline.go` uses a 200,000-entry channel with the orchestrator's hard-coded batch size of 20,000. Submission is nonblocking and drops when full. One goroutine collects entries and synchronously flushes at 20,000 entries or a 50 ms ticker; it cannot drain the channel during a flush. The declared `BATCH_SIZE`, `FLUSH_INTERVAL` and corresponding flags are not applied by the current `main`. Shutdown flushes the current batch but does not drain the channel.
4. `internal/storage/uds_engine.go` connects to `/tmp/io_rust_engine.sock`, serializes individual entries as newline-delimited JSON, and writes them under a mutex. Each write has a two-second deadline. The five-second batch context is not used by the bridge. Failed batches are logged and not retried. Go does not read Rust's replies, so its acknowledgement log only means the socket writes completed; unread replies can cause socket backpressure.
5. `rust_engine/src/main.rs` accepts UDS connections, validates each JSON line, sends it to a bounded 50,000-entry channel, waits for the writer, and replies `OK` or `ERR` before reading the next line on that connection. The single writer appends to a file through a 64 KiB `BufWriter` and flushes each entry. The file is named `./logs/app_YYYY-MM-DD.log` using the date at startup; it is not automatically rotated. Flush is not `fsync`, so a reply does not guarantee durable storage.

There are two independent Unix socket buffers, a Go channel, a current Go batch, and a Rust channel. The normal single Go socket has at most one record awaiting Rust's writer at a time, because the Rust connection task waits for the writer and sends the reply before reading the next line. Multiple connections share the Rust channel.

## Other components

- `internal/service/health.go` tracks last-seen timestamps and logs clients that have timed out; it does not expose health metrics.
- `internal/api/log_handler.go` and `router.go` define an HTTP ingestion route, but the current orchestrator does not start that router. `cmd/benchmark` targets an HTTP endpoint, not the active gRPC path.
- `internal/config/config.go` defines `APP_ENV`, `APP_PORT` and `LOG_LEVEL` for its configuration loader; the current gRPC orchestrator does not use it.
- `pkg/telemetry` defines HTTP and UDS metrics, and `pkg/metrics` is a separate example HTTP server. They are not wired into the active orchestrator pipeline.
- Rust serves Prometheus metrics on `RUST_METRICS_ADDR` (default `0.0.0.0:9090`): `rust_write_bytes_total`, `rust_disk_flush_duration_seconds`, `rust_disk_write_failures_total`, `rust_disk_flush_failures_total`, and `rust_invalid_logs_total`. Both failure counters currently increment for either a write or flush failure; the duration histogram records successful flushes.

## Build and run

Requirements: Go 1.26.1 or newer, Rust/Cargo and Linux. The Makefile's Rust release target also needs the `x86_64-unknown-linux-musl` target and a suitable musl toolchain.

```bash
make build-agent build-server
make build-rust
```

From the repository root, start the components in separate terminals:

```bash
./rust_engine/target/x86_64-unknown-linux-musl/release/rust_engine
./bin/release/grpc-server
GRPC_SERVER_ADDR=127.0.0.1:50099 ./bin/release/grpc-agent
```

Alternatively, `scripts/run-local.sh` starts the Rust engine and orchestrator, but not an agent. It runs Rust from `rust_engine/`, so Rust's relative `./logs` directory is under that directory. When run from the repository root as above, logs are under the root's `logs/` directory. The Docker entrypoint starts Rust and the orchestrator with `/app` as working directory; the agent is not started automatically.

Rust's `RUST_SOCKET_PATH` can override its socket, but Go and the container entrypoint currently use the fixed default path. Changing only the Rust variable will prevent them from connecting. No authentication or encryption is configured for gRPC.

## Project structure

- `cmd/agent`, `cmd/orchestrator`: active Go agent and server entry points.
- `cmd/benchmark`: HTTP benchmark; `cmd/worker` is a placeholder.
- `internal/api`, `internal/service`, `internal/storage`, `internal/config`: handlers, pipeline, storage bridge and configuration helper.
- `rust_engine/`: UDS listener, file writer and Rust metrics.
- `k3s/`: Kubernetes manifests; `Dockerfile` and `entrypoint.sh` are at the root.
- `scripts/run-local.sh`: local backend startup helper.
- [grpc_impl/IMPLEMENTATION_PLAN.md](grpc_impl/IMPLEMENTATION_PLAN.md): historical plan, including work still to be implemented.

For future changes, add fields consistently to Go and Rust log models or implement `storage.EngineBridge` for another backend. TLS, streaming-client improvements, load balancing and broader observability remain possible future work; see the preserved implementation plan.
