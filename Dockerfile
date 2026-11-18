# Multi-stage Dockerfile for High-Throughput I/O Logging System
# Builds Go binaries (agent and server) and Rust engine, then runs server + rust engine in a minimal container.

# ========== GO BUILD STAGE ==========
FROM golang:1.26-alpine AS go-builder

WORKDIR /app

# Copy go mod and sum
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build Go agent and server (statically linked)
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/grpc-agent ./cmd/agent/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/grpc-server ./cmd/orchestrator/main.go

# ========== RUST BUILD STAGE ==========
FROM rust:1-slim AS rust-builder

# Musl tool
RUN apt-get update && \
    apt-get install -y --no-install-recommends musl-tools && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy Rust source
COPY rust_engine/ ./rust_engine/

# Build Rust engine with musl target for static linking
RUN rustup target add x86_64-unknown-linux-musl && \
    cd rust_engine && \
    cargo build --release --target x86_64-unknown-linux-musl

# ========== FINAL STAGE ==========
FROM alpine:3.20

# Install ca-certificates for TLS if needed (not used internally but good practice)
RUN apk add --no-cache ca-certificates

# Create non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Create logs directory and set permissions
RUN mkdir -p logs && chown -R appuser:appgroup logs

# Copy built binaries from builder stages
COPY --from=go-builder /bin/grpc-agent /bin/grpc-agent
COPY --from=go-builder /bin/grpc-server /bin/grpc-server
COPY --from=rust-builder /app/rust_engine/target/x86_64-unknown-linux-musl/release/rust_engine /bin/rust_engine

# Ensure binaries are executable
RUN chmod +x /bin/grpc-agent /bin/grpc-server /bin/rust_engine

# Set ownership to non-root user
RUN chown -R appuser:appgroup /bin/grpc-agent /bin/grpc-server /bin/rust_engine

# Expose ports
EXPOSE 50099 9090

# Switch to non-root user
USER appuser

# Entry point script to start both services (preserve executable bit)
COPY --chmod=+x entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]