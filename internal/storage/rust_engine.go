package storage

import "context"

type LogEntry struct {
	ClientID  string                 `json:"client_id"`
	TimeStamp int64                  `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
}

type EngineBridge interface {
	WriteBatch(ctx context.Context, logs []LogEntry) error
	Close() error
}

// Temporary mock
type MockRustEngine struct{}

func NewMockRustEngine() *MockRustEngine {
	return &MockRustEngine{}
}

func (m *MockRustEngine) WriteBatch(ctx context.Context, logs []LogEntry) error {
	return nil
}

func (m *MockRustEngine) Close() error {
	return nil
}
