package api

import (
	"context"
	"io/internal/service"
	"io/internal/storage"
	"log"
	"net"

	"google.golang.org/grpc"
)

type MetricsServer struct {
	pipeline *service.BatchProcessor
	health   *service.HealthTracker
	UnimplementedMetricsServiceServer
}

func NewMetricsServer(p *service.BatchProcessor, h *service.HealthTracker) *MetricsServer {
	return &MetricsServer{
		pipeline: p,
		health:   h,
	}
}

// UploadMetric implements the MetricsServiceServer interface
func (s *MetricsServer) UploadMetric(ctx context.Context, req *MetricRequest) (*MetricResponse, error) {
	if req == nil {
		return &MetricResponse{
			Success:      false,
			ErrorMessage: "empty request",
		}, nil
	}

	// Track health
	if req.ClientId != "" {
		s.health.MarkSeen(req.ClientId)
	}

	// Convert protobuf request to internal LogEntry
	payload := make(map[string]interface{}, len(req.Payload))
	for k, v := range req.Payload {
		payload[k] = v
	}

	entry := storage.LogEntry{
		ClientID:  req.ClientId,
		TimeStamp: req.Timestamp,
		Level:     req.Level,
		Message:   req.Message,
		Payload:   payload,
	}

	// Submit to pipeline (shock absorber)
	s.pipeline.Submit(entry)

	return &MetricResponse{
		Success:      true,
		ErrorMessage: "",
	}, nil
}

// StreamMetrics implements the MetricsServiceServer interface
func (s *MetricsServer) StreamMetrics(stream grpc.ClientStreamingServer[MetricRequest, MetricResponse]) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		if req.ClientId != "" {
			s.health.MarkSeen(req.ClientId)
		}

		payload := make(map[string]interface{}, len(req.Payload))
		for k, v := range req.Payload {
			payload[k] = v
		}

		entry := storage.LogEntry{
			ClientID:  req.ClientId,
			TimeStamp: req.Timestamp,
			Level:     req.Level,
			Message:   req.Message,
			Payload:   payload,
		}

		s.pipeline.Submit(entry)
	}
}

func (s *MetricsServer) Start(port string) {
	lis, err := net.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		log.Fatalf("failed to listen on 0.0.0.0:%s: %v", port, err)
	}

	grpcServer := grpc.NewServer()
	// Register the actual MetricsService implementation
	RegisterMetricsServiceServer(grpcServer, s)
	
	log.Printf("🚀 gRPC Production Server listening on ALL interfaces (0.0.0.0:%s)", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
