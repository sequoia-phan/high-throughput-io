# Rust Storage Engine

The high-performance backend responsible for persisting logs to disk with minimal overhead.

## Key Features
- **UDS Communication**: Uses Unix Domain Sockets for ultra-low latency IPC with the Go orchestrator.
- **Async Disk I/O**: Powered by `tokio` and `BufWriter` to batch writes and reduce syscalls.
- **Internal Metrics**: Exposes its own Prometheus endpoint to monitor disk flush durations and write failures.

## Development
- Build: `cargo build --release`
- Test: `cargo test`
- Run: `cargo run`
