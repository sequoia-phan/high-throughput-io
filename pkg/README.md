# Public Packages

- `telemetry/`: Prometheus HTTP and UDS metric definitions and HTTP middleware. The active gRPC orchestrator does not currently wire these metrics into its pipeline.
- `metrics/`: a standalone example HTTP metrics server with placeholder handlers, not the active orchestrator.

Client libraries, logging wrappers and distributed tracing were described previously as possible future packages; they are not present here.
