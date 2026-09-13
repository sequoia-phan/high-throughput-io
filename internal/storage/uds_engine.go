package storage

import (
	"context"
	"encoding/json"
	"errors"
	"io/pkg/telemetry"
	"log/slog"
	"net"
	"runtime"
	"sync"
	"time"
)

type UDSEngine struct {
	socketPath string
	conn       net.Conn
	mu         sync.Mutex
	logQueue   chan LogEntry
	quit       chan struct{}
	wg         sync.WaitGroup
}

// NewUDSEngine, maximize logs in RAM
func NewUDSEngine(socketPath string, bufferSize int) *UDSEngine {
	engine := &UDSEngine{
		socketPath: socketPath,
		logQueue:   make(chan LogEntry, bufferSize),
		quit:       make(chan struct{}),
	}

	workerCount := runtime.NumCPU()
	engine.wg.Add(workerCount)

	//adding more CPU for parallel proccessing
	for i := 0; i < runtime.NumCPU(); i++ {
		go engine.workerLoop()
	}

	return engine
}

func (u *UDSEngine) connect() error {
	conn, err := net.DialTimeout("unix", u.socketPath, 2*time.Second)
	if err != nil {
		telemetry.UDSConnectionAttemptsTotal.WithLabelValues("failure").Inc()
		return err
	}

	u.conn = conn
	telemetry.UDSConnectionAttemptsTotal.WithLabelValues("success").Inc()
	slog.Info("Connected successfully to Rust UDS Engine", "path", u.socketPath)
	return nil
}

func (u *UDSEngine) WriteBatch(ctx context.Context, logs []LogEntry) error {
	for _, entry := range logs {
		select {
		case u.logQueue <- entry:
			telemetry.IPCQueueDepth.Set(float64(len(u.logQueue)))
		default:
			slog.Warn("Log buffer queue full! Dropping entry to protect HTTP thread", "client_id", entry.ClientID)
			telemetry.DroppedLogsTotal.Inc()
			return errors.New("buffer_full")
		}
	}
	return nil
}

func (u *UDSEngine) workerLoop() {
	defer u.wg.Done()

	for {
		select {
		case entry := <-u.logQueue:
			telemetry.IPCQueueDepth.Set(float64(len(u.logQueue)))
			u.writeToSocket(entry)
		case <-u.quit:
			for len(u.logQueue) > 0 {
				entry := <-u.logQueue
				telemetry.IPCQueueDepth.Set(float64(len(u.logQueue)))
				u.writeToSocket(entry)
			}
			return
		}
	}
}

func (u *UDSEngine) writeToSocket(entry LogEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	data = append(data, '\n')

	for {
		if u.conn == nil {
			if err := u.connect(); err != nil {
				time.Sleep(500 * time.Millisecond)
				continue
			}
		}

		_ = u.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		writeStarted := time.Now()
		_, err = u.conn.Write(data)
		writeDuration := time.Since(writeStarted).Seconds()
		telemetry.UDSWriteDuration.Observe(writeDuration)
		telemetry.UDSWriteLatencySeconds.Set(writeDuration)

		if err != nil {
			slog.Error("Failed to write to UDS, socket resetting...", "error", err)
			telemetry.UDSWriteFailuresTotal.Inc()
			u.mu.Lock()
			if u.conn != nil {
				_ = u.conn.Close()
				u.conn = nil
			}
			u.mu.Unlock()
			continue
		}
		break
	}
}

func (u *UDSEngine) Close() error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.conn != nil {
		return u.conn.Close()
	}
	return nil
}
