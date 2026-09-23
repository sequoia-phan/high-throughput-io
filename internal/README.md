# Internal Logic

Contains private application code that is not intended for use by other projects.

- `api/`: HTTP handlers, routing, and middleware.
- `config/`: Configuration loading and environment variable management.
- `domain/`: Core business entities and domain models.
- `provider/`: External service providers and adapters.
- `service/`: Business logic implementation (Use Cases).
- `storage/`: Storage abstractions and the bridge to the Rust Engine.
- `store/`: Concrete data store implementations.
