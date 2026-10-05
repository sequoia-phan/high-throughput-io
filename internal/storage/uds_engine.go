package storage

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/pkg/telemetry"
	"log/slog"
	"net"
	"sync"
	"time"
)

var ErrBufferFull = errors.New("storage queue is full")

type remoteStorageError struct{ message string }

func (e remoteStorageError) Error() string { return "Rust storage rejected log: " + e.message }

type queuedLog struct {
	entry LogEntry
	done  chan error
}

type UDSEngine struct {
	socketPath string
	logQueue   chan queuedLog
	connMu     sync.Mutex // Serialize each request/ack exchange on the stream socket.
	conn       net.Conn
	reader     *bufio.Reader
	queueMu    sync.Mutex // Protect enqueue against queue close and make batches atomic.
	closed     bool
	wg         sync.WaitGroup
}

func NewUDSEngine(socketPath string, bufferSize int) *UDSEngine {
	if bufferSize < 1 {
		bufferSize = 1
	}
	u := &UDSEngine{socketPath: socketPath, logQueue: make(chan queuedLog, bufferSize)}
	u.wg.Add(1)
	go u.workerLoop()
	return u
}

// WriteBatch enqueues the entire batch or none of it, then waits for the Rust
// writer to acknowledge every entry after writing and flushing it.
func (u *UDSEngine) WriteBatch(ctx context.Context, logs []LogEntry) error {
	if len(logs) == 0 {
		return nil
	}
	items := make([]queuedLog, len(logs))
	u.queueMu.Lock()
	if u.closed {
		u.queueMu.Unlock()
		return errors.New("storage engine is closed")
	}
	if len(logs) > cap(u.logQueue)-len(u.logQueue) {
		u.queueMu.Unlock()
		telemetry.DroppedLogsTotal.Add(float64(len(logs)))
		return ErrBufferFull
	}
	for i, entry := range logs {
		items[i] = queuedLog{entry: entry, done: make(chan error, 1)}
		u.logQueue <- items[i]
	}
	u.queueMu.Unlock()
	telemetry.IPCQueueDepth.Set(float64(len(u.logQueue)))

	for _, item := range items {
		select {
		case err := <-item.done:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (u *UDSEngine) workerLoop() {
	defer u.wg.Done()
	for item := range u.logQueue {
		telemetry.IPCQueueDepth.Set(float64(len(u.logQueue)))
		item.done <- u.writeToSocket(item.entry)
	}
}

func (u *UDSEngine) connectLocked() error {
	conn, err := net.DialTimeout("unix", u.socketPath, 2*time.Second)
	if err != nil {
		telemetry.UDSConnectionAttemptsTotal.WithLabelValues("failure").Inc()
		return err
	}
	u.conn = conn
	u.reader = bufio.NewReader(conn)
	telemetry.UDSConnectionAttemptsTotal.WithLabelValues("success").Inc()
	slog.Info("Connected to Rust UDS engine", "path", u.socketPath)
	return nil
}

func (u *UDSEngine) writeToSocket(entry LogEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode log entry: %w", err)
	}
	data = append(data, '\n')

	for {
		err = nil
		rejected := false
		u.connMu.Lock()
		if u.conn == nil {
			err = u.connectLocked()
		}
		if err == nil {
			_ = u.conn.SetDeadline(time.Now().Add(5 * time.Second))
			started := time.Now()
			for written := 0; written < len(data) && err == nil; {
				var n int
				n, err = u.conn.Write(data[written:])
				written += n
				if n == 0 && err == nil {
					err = io.ErrShortWrite
				}
			}
			if err == nil {
				var ack string
				ack, err = u.reader.ReadString('\n')
				if err == nil && ack != "OK\n" {
					rejected = true
					err = remoteStorageError{message: ack}
				}
			}
			duration := time.Since(started).Seconds()
			telemetry.UDSWriteDuration.Observe(duration)
			telemetry.UDSWriteLatencySeconds.Set(duration)
		}
		if err == nil {
			_ = u.conn.SetDeadline(time.Time{})
			u.connMu.Unlock()
			return nil
		}
		if rejected {
			// The Rust peer consumed this request and returned a definitive error.
			// Keep the stream aligned and report it instead of retrying the same log.
			u.connMu.Unlock()
			return err
		}
		telemetry.UDSWriteFailuresTotal.Inc()
		if u.conn != nil {
			_ = u.conn.Close()
			u.conn = nil
			u.reader = nil
		}
		u.connMu.Unlock()
		u.queueMu.Lock()
		closing := u.closed
		u.queueMu.Unlock()
		if closing {
			return fmt.Errorf("storage engine closed before acknowledgment: %w", err)
		}
		if errors.Is(err, io.EOF) {
			err = errors.New("Rust engine closed the connection before acknowledging the log")
		}
		slog.Warn("UDS write or acknowledgment failed; retrying", "error", err)
		time.Sleep(200 * time.Millisecond)
	}
}

// Close rejects new batches, drains queued entries, and waits for workers.
func (u *UDSEngine) Close() error {
	u.queueMu.Lock()
	if !u.closed {
		u.closed = true
		close(u.logQueue)
	}
	u.queueMu.Unlock()
	u.wg.Wait()
	u.connMu.Lock()
	defer u.connMu.Unlock()
	if u.conn != nil {
		err := u.conn.Close()
		u.conn = nil
		u.reader = nil
		return err
	}
	return nil
}
