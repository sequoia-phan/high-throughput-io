# Internal Logic

- `api/`: gRPC service, HTTP handlers, routing and middleware. The current orchestrator starts the gRPC service only.
- `config/`: configuration loader, not used by the current gRPC orchestrator.
- `service/`: batch processor and health tracker.
- `storage/`: log model, storage interface, mock and Unix socket bridge.

The previously listed `domain/`, `provider/` and `store/` directories are not present; they remain possible future organization plans.
