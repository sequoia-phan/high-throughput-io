package service

import (
	"context"
	"log"
	"sync"
	"time"
)

type HealthStatus string

const (
	StatusOnline      HealthStatus = "ONLINE"
	StatusOffline     HealthStatus = "OFFLINE"
)

type HealthTracker struct {
	mu       sync.RWMutex
	lastSeen map[string]time.Time
	timeout  time.Duration
}

func NewHealthTracker(timeout time.Duration) *HealthTracker {
	return &HealthTracker{
		lastSeen: make(map[string]time.Time),
		timeout:  timeout,
	}
}

func (h *HealthTracker) MarkSeen(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastSeen[id] = time.Now()
}

func (h *HealthTracker) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-ticker.C:
				h.checkHealth()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (h *HealthTracker) checkHealth() {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	for id, lastSeen := range h.lastSeen {
		if now.Sub(lastSeen) > h.timeout {
			log.Printf("[Health] Target %s has gone OFFLINE (No push received)", id)
		}
	}
}

func (h *HealthTracker) GetStatus(id string) HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if lastSeen, ok := h.lastSeen[id]; ok && time.Since(lastSeen) < h.timeout {
		return StatusOnline
	}
	return StatusOffline
}
