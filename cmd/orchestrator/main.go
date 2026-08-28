package main

import (
	"context"
	"errors"
	"fmt"
	"io/internal/api"
	"io/internal/config"
	"io/internal/storage"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {

	// debug mode
	go func() {
		http.ListenAndServe("192.168.1.27:6060", nil)
	}()

	// Structured JSON Logger for K8s
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// loading config
	cfg := config.Load()

	socketPath := "/tmp/io_rust_engine.sock"

	// Initial async UDS rust engine with in-mem ring buffer = 10,000 slots
	rustEngine := storage.NewUDSEngine(socketPath, 10000)

	defer rustEngine.Close()

	router := api.NewRouter(cfg, rustEngine)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// run Server in background with goroutine
	go func() {
		slog.Info("Server is listening", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}
