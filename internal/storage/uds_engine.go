package storage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type UDSEngine struct {
	socketPath string
	conn       net.Conn
	reader     *bufio.Reader
	mu         sync.Mutex
}

func NewUDSEngine(path string) *UDSEngine {
	return &UDSEngine{
		socketPath: path,
	}
}

func (u *UDSEngine) connect() error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.conn != nil {
		return nil
	}

	log.Printf("[UDS] Attempting to connect to Rust socket at %s...", u.socketPath)
	conn, err := net.Dial("unix", u.socketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to Rust engine socket at %s: %w", u.socketPath, err)
	}
	log.Println("[UDS] ✅ Successfully connected to Rust Engine socket")
	u.conn = conn
	u.reader = bufio.NewReader(conn)

	return nil
}

func (u *UDSEngine) WriteBatch(ctx context.Context, logs []LogEntry) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := u.connect(); err != nil {
		return err
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	// Close the connection on an I/O or protocol error.
	// Never reuse a connection with uncertain ACK state.
	resetConnection := func() {
		if u.conn != nil {
			_ = u.conn.Close()
		}
		u.conn = nil
		u.reader = nil
	}

	for i, entry := range logs {
		if err := ctx.Err(); err != nil {
			resetConnection()
			return err
		}

		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf(
				"marshal record %d: %w", i, err,
			)
		}

		data = append(data, '\n')

		// Apply a deadline to both writing and reading.
		deadline := time.Now().Add(2 * time.Second)

		if ctxDeadline, ok := ctx.Deadline(); ok &&
			ctxDeadline.Before(deadline) {
			deadline = ctxDeadline
		}

		if err := u.conn.SetDeadline(deadline); err != nil {
			resetConnection()
			return fmt.Errorf(
				"set deadline for record %d: %w", i, err,
			)
		}

		// Write the entire record, handling short writes.
		for written := 0; written < len(data); {
			n, err := u.conn.Write(data[written:])
			written += n

			if err != nil {
				resetConnection()
				return fmt.Errorf(
					"UDS write record %d: %w", i, err,
				)
			}

			if n == 0 {
				resetConnection()
				return fmt.Errorf(
					"UDS write record %d: %w",
					i, io.ErrShortWrite,
				)
			}
		}

		// Rust sends one acknowledgement for each record.
		response, err := u.reader.ReadString('\n')
		if err != nil {
			resetConnection()
			return fmt.Errorf(
				"UDS read ACK for record %d: %w", i, err,
			)
		}

		response = strings.TrimSpace(response)

		if response != "OK" {
			resetConnection()
			return fmt.Errorf(
				"Rust rejected record %d: %s",
				i, response,
			)
		}
	}

	// Clear deadlines before reusing the connection.
	if err := u.conn.SetDeadline(time.Time{}); err != nil {
		resetConnection()
		return fmt.Errorf("clear UDS deadline: %w", err)
	}

	log.Printf(
		"[UDS] Batch acknowledged: %d records",
		len(logs),
	)

	return nil
}

func (u *UDSEngine) Close() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.conn != nil {
		err := u.conn.Close()
		u.conn = nil
		return err
	}
	return nil
}
