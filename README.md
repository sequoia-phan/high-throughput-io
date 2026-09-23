# High-Throughput I/O Logging System

A hybrid Go/Rust system designed for extreme logging throughput.

## Architecture Overview
- **Go Orchestrator**: Handles API requests, orchestration, and worker management.
- **Rust Storage Engine**: A high-performance disk writer utilizing Unix Domain Sockets (UDS) and buffered async I/O for minimal latency.

## Project Structure
- `cmd/`: Entry points for various binaries (orchestrator, workers, benchmarks).
- `internal/`: Private application code (business logic, storage bridges, config).
- `pkg/`: Public library code (telemetry, metrics, logging).
- `rust_engine/`: The Rust-based high-performance storage backend.
- `deployments/`: Infrastructure configurations (Docker, K8s).
- `scripts/`: Automation and local setup scripts.
