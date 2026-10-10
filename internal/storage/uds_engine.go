package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
	"log"
)

type UDSEngine struct {
	socketPath string
	conn       net.Conn
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
	return nil
}

func (u *UDSEngine) WriteBatch(ctx context.Context, logs []LogEntry) error {
	if err := u.connect(); err != nil {
		return err
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	data, err := json.Marshal(logs)
	if err != nil {
		return fmt.Errorf("failed to marshal batch: %w", err)
	}
	data = append(data, '\n')

	u.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))

	n, err := u.conn.Write(data)
	if err != nil {
		log.Printf("[UDS] ❌ Write error: %v", err)
		u.conn.Close()
		u.conn = nil
		return fmt.Errorf("socket write error: %w", err)
	}

	log.Printf("[UDS] 📤 Successfully wrote %d bytes to Rust Engine", n)
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
