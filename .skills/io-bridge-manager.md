# Skill: IO Bridge Manager

This document serves as a machine-readable skill for AI agents managing the communication bridge between the Go Orchestrator and the Rust Storage Engine.

## Contextual Knowledge
- **Communication Protocol**: The system uses Unix Domain Sockets (UDS) for IPC.
- **Socket Path**: `/tmp/io_rust_engine.sock`
- **Data Format**: Line-delimited JSON.
- **Schema**: `LogEntry` (contains client_id, timestamp, level, message, and optional payload).

## Key File Mapping
- **Go Side (The Client)**: `internal/storage/rust_engine.go` (Currently contains MockRustEngine; needs implementation of the UDS client).
- **Rust Side (The Server)**: `rust_engine/src/main.rs` (Implements the `UnixListener` and the background `BufWriter`).

## Operational Procedures

### 1. Debugging Connection Issues
When a "Connection Refused" or "Socket Not Found" error occurs:
1. Verify the Rust engine is running (`make run-rust`).
2. Check if the socket file exists at `/tmp/io_rust_engine.sock`.
3. Check Rust logs in `rust_engine/logs/` for permission errors during socket binding.

### 2. Modifying the Log Schema
If a field is added to the logs:
1. Update the `LogEntry` struct in `internal/storage/rust_engine.go`.
2. Update the `LogEntry` struct in `rust_engine/src/main.rs`.
3. Ensure the field is marked as `#[serde(default)]` in Rust if it is optional to prevent parsing failures.

### 3. Performance Tuning
To increase throughput:
1. **Buffer Size**: Modify `BufWriter::with_capacity` in `rust_engine/src/main.rs`.
2. **Channel Depth**: Adjust the `mpsc::channel` capacity in the Rust `main` function.
3. **Flush Interval**: Adjust `interval(Duration::from_secs(1))` to balance between durability and performance.

## Guardrails
- **Never** change the socket path in one language without updating the other.
- **Always** ensure JSON output from Go ends with a newline (`\n`), as the Rust engine uses `read_line`.
