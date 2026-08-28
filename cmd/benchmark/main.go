package main

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func main() {

	targetURL := "http://192.168.1.27:8080/api/v1/logs"

	totalRequests := 10000
	concurrency := 10

	payload := []byte(`[
		{
			"client_id":"bench-vm-01",
			"timestamp":17000000,
			"level":"info",
			"message":"BenchMark High thoughput stream test",
			"payload":{"cpu_usage":42.5} 

		}
	]`)

	var successCount uint64
	var errorCount uint64

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
		},

		Timeout: 5 * time.Second,
	}

	reqPerWorker := totalRequests / concurrency

	var wg sync.WaitGroup

	fmt.Printf("=== START BENCHMARK: %d Requests | %d Workers ===\n", totalRequests, concurrency)
	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < reqPerWorker; j++ {
				req, _ := http.NewRequest("POST", targetURL, bytes.NewBuffer(payload))
				req.Header.Set("Content-Type", "application/json")

				resp, err := client.Do(req)
				if err != nil {
					atomic.AddUint64(&errorCount, 1)
					continue
				}

				if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK {
					atomic.AddUint64(&successCount, 1)
				} else {
					atomic.AddUint64(&errorCount, 1)
				}
				_ = resp.Body.Close()
			}
		}()
	}

	wg.Wait()
	duration := time.Since(startTime)

	rps := float64(successCount) / duration.Seconds()

	fmt.Println("=== BENCHMARK RESULT ===")
	fmt.Printf("Thời gian chạy: %v\n", duration)
	fmt.Printf("Thành công:     %d requests\n", successCount)
	fmt.Printf("Thất bại:       %d requests\n", errorCount)
	fmt.Printf("Throughput:     %.2f RPS (req/sec)\n", rps)
}
