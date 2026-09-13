package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

func main() {

	targetURL := flag.String("url", "http://192.168.1.27:8080/api/v1/logs", "Target URL to benchmark")

	totalRequests := flag.Int("n", 5000, "Total number of requests")
	concurrency := flag.Int("c", 10, "Number of concurrent worker")
	flag.Parse()

	if *concurrency <= 0 || *totalRequests <= 0 {
		fmt.Println("Error: -n và -c phải lớn hơn 0")
		os.Exit(1)
	}

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
			MaxIdleConns:        1000,
			MaxIdleConnsPerHost: *concurrency * 2,
			IdleConnTimeout:     90 * time.Second,
			DisableKeepAlives:   false,
		},

		Timeout: 5 * time.Second,
	}

	reqPerWorker := *totalRequests / *concurrency

	var wg sync.WaitGroup

	fmt.Printf("=== START BENCHMARK: %d Requests | %d Workers ===\n", *totalRequests, *concurrency)
	startTime := time.Now()

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < reqPerWorker; j++ {

				req, err := http.NewRequest("POST", *targetURL, bytes.NewReader(payload))
				if err != nil {
					atomic.AddUint64(&errorCount, 1)
					continue
				}

				req.Header.Set("Content-Type", "application/json")

				resp, err := client.Do(req)
				if err != nil {
					atomic.AddUint64(&errorCount, 1)
					continue
				}

				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()

				if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK {
					atomic.AddUint64(&successCount, 1)
				} else {
					atomic.AddUint64(&errorCount, 1)
				}

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
