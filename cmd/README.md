# Command Entry Points

- `agent/`: collects Linux CPU/memory information and sends unary gRPC requests. Set `GRPC_SERVER_ADDR` and optionally `AGENT_CLIENT_ID`. The loop currently sleeps 200 µs after each successful request; the declared sleep flag/environment variable is not used.
- `orchestrator/`: starts the batch processor, health tracker and gRPC server. Its active settings are hard-coded to a 20,000-entry batch and 50 ms ticker; declared batch flags/environment variables are not applied.
- `benchmark/`: HTTP load generator, targeting an HTTP ingestion endpoint rather than the active gRPC server. Run load tests only after establishing suitable limits.
- `worker/`: placeholder, not an implemented worker.

See the root [README](../README.md) for build and run instructions. A full Go queue drops incoming records; increasing batch size or changing the ticker alone does not establish or fix the cause of a bottleneck.

## Planned configuration wiring

The earlier plan described `-sleep`/`AGENT_SLEEP` for the agent and `-batch-size`/`BATCH_SIZE` and `-flush-interval`/`FLUSH_INTERVAL` for the orchestrator. These remain intended configuration knobs, but are not currently wired into the running loops. Once implemented, their behavior and defaults should be verified before using them in load tests.
