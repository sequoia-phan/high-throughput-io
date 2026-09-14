.PHONY: run run-rust run-go benchmark test test-go test-rust

# Run both components locally. Ctrl-C stops the Go and Rust processes.
run:
	./scripts/run-local.sh

# These targets are useful when inspecting either component independently.
run-rust:
	cd "$(CURDIR)/rust_engine" && cargo run

run-go:
	go run ./cmd/orchestrator

# Example: make benchmark BENCHMARK_ARGS='-url http://localhost:8080/api/v1/logs -n 5000 -c 10'
benchmark:
	go run ./cmd/benchmark $(BENCHMARK_ARGS)

test: test-go test-rust

test-go:
	go test ./...

test-rust:
	cargo test --manifest-path rust_engine/Cargo.toml
