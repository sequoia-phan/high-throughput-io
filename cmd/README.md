# Command Entry Points

This directory contains the main entry points for the different binaries of the system.

- `benchmark/`: Tools to measure the throughput and latency of the logging pipeline.
- `orchestrator/`: The main control plane that manages log flow and coordinates workers.
- `worker/`: Processing units that handle log ingestion and forward them to the storage engine.
- `ioctl/`: System-level control tools for managing the engine state.
