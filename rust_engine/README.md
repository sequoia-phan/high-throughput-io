# Rust Storage Engine

The high-performance backend responsible for persisting logs to disk with minimal overhead.

## Key Features
- **UDS Communication**: Uses Unix Domain Sockets for ultra-low latency IPC with the Go orchestrator.
- **Buffered Disk I/O**: Uses `tokio` and `BufWriter`; each accepted entry is flushed before acknowledgment.
- **Write Acknowledgments**: The UDS protocol replies `OK` after the entry is written and the userspace buffer is flushed. Go reports success only after receiving this reply.
- **Internal Metrics**: Exposes its own Prometheus endpoint to monitor disk flush durations and write failures.

## Development
- Build: `cargo build --release`
- Test: `cargo test`
- Run: `cargo run`

The acknowledgment confirms a successful write and userspace buffer flush. It does
not call `fsync`/`sync_data`, so it does not guarantee survival of an OS or power
failure. Flushes happen per entry to keep the acknowledgment tied to that entry,
which trades some throughput for clearer delivery semantics.

If a connection fails after Rust writes an entry but before Go receives its
acknowledgment, Go retries the entry. This preserves delivery across uncertain
failures but can produce duplicate log lines; the protocol does not provide
exactly-once delivery. Logs waiting in the Go queue are also held only in memory
and can be lost if the Go process exits unexpectedly.
