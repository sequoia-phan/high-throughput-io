package main

import (
	"context"
	"fmt"
	"io/internal/storage"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

type Agent struct {
	clientID   string
	serverAddr string
	conn       *grpc.ClientConn
}

func NewAgent(id, addr string) *Agent {
	return &Agent{
		clientID:   id,
		serverAddr: addr,
	}
}

func (a *Agent) CollectMetrics() storage.LogEntry {
	cpu := a.readCPU()
	mem := a.readMem()
	return storage.LogEntry{
		ClientID:  a.clientID,
		TimeStamp: time.Now().Unix(),
		Level:     "INFO",
		Message:   fmt.Sprintf("CPU: %.2f%%, MEM: %.2f%%", cpu, mem),
	}
}

func (a *Agent) readCPU() float64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil { return 0.0 }
	lines := strings.Split(string(data), "\n")
	if len(lines) < 1 { return 0.0 }
	fields := strings.Fields(lines[0])
	if len(fields) < 5 { return 0.0 }
	user, _ := strconv.ParseFloat(fields[1], 64)
	nice, _ := strconv.ParseFloat(fields[2], 64)
	system, _ := strconv.ParseFloat(fields[3], 64)
	idle, _ := strconv.ParseFloat(fields[4], 64)
	total := user + nice + system + idle
	if total == 0 { return 0.0 }
	return ((total - idle) / total) * 100
}

func (a *Agent) readMem() float64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil { return 0.0 }
	var total, free float64
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 { total, _ = strconv.ParseFloat(fields[1], 64) }
		} else if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 { free, _ = strconv.ParseFloat(fields[1], 64) }
		}
	}
	if total == 0 { return 0.0 }
	return ((total - free) / total) * 100
}

func (a *Agent) Run() {
	for {
		log.Printf("--- Starting Connection Probe to %s ---", a.serverAddr)
		
		if err := a.testRawTCP(); err != nil {
			log.Printf("❌ RAW TCP FAILURE: %v", err)
			log.Println("The server is not accepting TCP connections. Check Firewall/Listening IP.")
			time.Sleep(5 * time.Second)
			continue
		}
		log.Println("✅ RAW TCP SUCCESS: The port is open and accepting data.")

		conn, err := grpc.NewClient(a.serverAddr, 
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Printf("❌ gRPC Client Error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		a.conn = conn

		if err := a.waitForReady(5 * time.Second); err != nil {
			log.Printf("❌ gRPC PROTOCOL FAILURE: %v", err)
			log.Println("TCP is open, but gRPC/HTTP2 handshake failed.")
			a.conn.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		log.Println("✅ gRPC CONNECTION ESTABLISHED.")
		
		for {
			metric := a.CollectMetrics()
			if err := a.realGrpcInvoke(metric); err != nil {
				log.Printf("❌ Push Error: %v", err)
				break 
			}
			log.Printf("[Push] ✅ %s", metric.Message)
			time.Sleep(2 * time.Second)
		}
		a.conn.Close()
	}
}

func (a *Agent) testRawTCP() error {
	conn, err := net.DialTimeout("tcp", a.serverAddr, 3*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write([]byte("PING"))
	return err
}

func (a *Agent) waitForReady(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		state := a.conn.GetState()
		if state == connectivity.Ready {
			return nil
		}
		if state == connectivity.TransientFailure {
			return fmt.Errorf("connection transient failure")
		}
		if !a.conn.WaitForStateChange(ctx, state) {
			return fmt.Errorf("timeout waiting for ready state")
		}
	}
}

func (a *Agent) realGrpcInvoke(entry storage.LogEntry) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var reply interface{}
	err := a.conn.Invoke(ctx, "/metrics.MetricsService/UploadMetric", &entry, &reply)
	if err != nil && !strings.Contains(err.Error(), "Unimplemented") {
		return err
	}
	return nil
}

func main() {
	clientID := "vm-pve-host" 
	serverAddr := "192.168.1.27:50099" 

	agent := NewAgent(clientID, serverAddr)
	agent.Run()
}
