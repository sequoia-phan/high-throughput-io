# Rust Storage Engine

The Rust engine accepts newline-delimited JSON over a Unix domain socket and appends valid records to a file.

- The listener starts a task per connection. Each task sends a record to a shared bounded 50,000-entry writer channel, waits for the writer and sends `OK` or `ERR` before reading the next record.
- A single task writes through a 64 KiB Tokio `BufWriter` and flushes each record. The filename `./logs/app_YYYY-MM-DD.log` uses the date when the writer starts; automatic midnight rotation is not implemented.
- The reply confirms the file write and userspace buffer flush, not `fsync` or durable storage. The current Go client does not read replies or retry failed batches. Unread replies can eventually block the connection.
- Prometheus metrics are served on `RUST_METRICS_ADDR` (default `0.0.0.0:9090`). `RUST_SOCKET_PATH` defaults to `/tmp/io_rust_engine.sock`; the Go orchestrator currently uses that fixed path.

From this directory: `cargo build --release`, `cargo test`, or `cargo run`. Log paths are relative to the process working directory. See the root [documentation](../DOCUMENTATION.md) for the complete flow and limitations.
