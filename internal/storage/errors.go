package storage

import "errors"

// ErrBufferFull is returned when the storage engine cannot accept more logs
// because the internal ring buffer is completely full.
var ErrBufferFull = errors.New("storage engine buffer full")

// ErrEngineUnavailable is returned when the connection to the Rust engine is lost.
var ErrEngineUnavailable = errors.New("storage engine unavailable")
