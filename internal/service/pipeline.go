package service

import (
	"context"
	"io/internal/storage"
	"log"
	"sync"
	"time"
)

type BatchProcessor struct {
	storage       storage.EngineBridge
	buffer        chan storage.LogEntry
	batchSize     int
	flushInterval time.Duration
	wg            sync.WaitGroup
	stopChan      chan struct{}
}

func NewBatchProcessor(engine storage.EngineBridge, batchSize int, flushInterval time.Duration) *BatchProcessor {
	return &BatchProcessor{
		storage:       engine,
		buffer:        make(chan storage.LogEntry, batchSize*10),
		batchSize:     batchSize,
		flushInterval: flushInterval,
		stopChan:      make(chan struct{}),
	}
}

func (p *BatchProcessor) Start() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		
		var currentBatch []storage.LogEntry
		ticker := time.NewTicker(p.flushInterval)
		defer ticker.Stop()

		for {
			select {
			case entry := <-p.buffer:
				currentBatch = append(currentBatch, entry)
				log.Printf("[Pipeline] Received metric from %s. Buffer size: %d/%d", entry.ClientID, len(currentBatch), p.batchSize)
				if len(currentBatch) >= p.batchSize {
					log.Println("[Pipeline] Batch size reached. Triggering FLUSH...")
					p.flush(currentBatch)
					currentBatch = make([]storage.LogEntry, 0, p.batchSize)
				}
			case <-ticker.C:
				if len(currentBatch) > 0 {
					log.Printf("[Pipeline] Timer expired. Flushing %d entries...", len(currentBatch))
					p.flush(currentBatch)
					currentBatch = make([]storage.LogEntry, 0, p.batchSize)
				}
			case <-p.stopChan:
				if len(currentBatch) > 0 {
					p.flush(currentBatch)
				}
				return
			}
		}
	}()
}

func (p *BatchProcessor) Submit(entry storage.LogEntry) {
	select {
	case p.buffer <- entry:
		// Logged in the loop above
	default:
		log.Println("❌ CRITICAL: Pipeline buffer full, dropping metric!")
	}
}

func (p *BatchProcessor) flush(batch []storage.LogEntry) {
	log.Printf("[Pipeline] 📤 Sending batch of %d to Rust Engine...", len(batch))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := p.storage.WriteBatch(ctx, batch); err != nil {
		log.Printf("❌ Storage Error: %v", err)
	} else {
		log.Println("[Pipeline] ✅ Rust Engine acknowledged the write.")
	}
}

func (p *BatchProcessor) Stop() {
	close(p.stopChan)
	p.wg.Wait()
}
