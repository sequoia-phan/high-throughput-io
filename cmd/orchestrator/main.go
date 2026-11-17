package main

import (
	"context"
	"flag"
	"io/internal/api"
	"io/internal/service"
	"io/internal/storage"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// ----- flags ---------------------------------------------------------
var (
	batchSizeFlag = flag.Int("batch-size", 20_000, "number of entries that trigger a flush")
	flushDurFlag  = flag.Duration("flush-interval", 500*time.Millisecond, "max time to wait before flushing a partial batch")

	batchSizeEnv = func() int {
		if v := os.Getenv("BATCH_SIZE"); v != "" {
			if i, err := strconv.Atoi(v); err == nil && i > 0 {
				return i
			}
		}
		return *batchSizeFlag
	}()
	flushDurEnv = func() time.Duration {
		if v := os.Getenv("FLUSH_INTERVAL"); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				return d
			}
		}
		return *flushDurFlag
	}()
)

// ---------------------------------------------------------------------

func main() {
	log.Println("🚀 Starting Production High-Throughput Push-Based Orchestrator...")

	// 1. Initialize Storage Engine - Using the socket path used by the Rust binary
	engine := storage.NewUDSEngine("/tmp/io_rust_engine.sock")

	// 2. Initialize Production Pipeline (The Shock Absorber)
	pipeline := service.NewBatchProcessor(engine, 20000, 50*time.Millisecond)
	pipeline.Start()

	// 3. Initialize Health Tracker (Tracks VM heartbeats)
	health := service.NewHealthTracker(15 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	health.Start(ctx)

	// 4. Initialize gRPC Server (The Sink)
	grpcServer := api.NewMetricsServer(pipeline, health)

	go grpcServer.Start("50099")

	log.Println("✅ System is live. Listening for gRPC pushes on port 50099.")
	log.Println("Connected to Rust Storage Engine via /tmp/io_rust_engine.sock")
	log.Println("Waiting for VM Agents to push metrics... Press Ctrl+C to stop.")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Stopping services...")
	pipeline.Stop()
	log.Println("Shutdown complete.")
}
