package api

import (
	"io/internal/service"
	"io/internal/storage"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type MetricsServer struct {
	pipeline *service.BatchProcessor
	health   *service.HealthTracker
}

func NewMetricsServer(p *service.BatchProcessor, h *service.HealthTracker) *MetricsServer {
	return &MetricsServer{
		pipeline: p,
		health:   h,
	}
}

func (s *MetricsServer) handleUnknownMethod(srv interface{}, ss grpc.ServerStream) error {
	log.Println("[gRPC] Received request to unknown method. Capturing as generic metric...")
	s.pipeline.Submit(storage.LogEntry{
		ClientID: "vm-agent-generic",
		Message:   "Metric received via generic handler",
		Level:     "INFO",
	})
	return status.Error(codes.Unimplemented, "Method not implemented, but data captured")
}

func (s *MetricsServer) Start(port string) {
	lis, err := net.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		log.Fatalf("failed to listen on 0.0.0.0:%s: %v", port, err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnknownServiceHandler(s.handleUnknownMethod),
	)
	
	log.Printf("🚀 gRPC Production Server listening on ALL interfaces (0.0.0.0:%s)", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
