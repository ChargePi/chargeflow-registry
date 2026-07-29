// Package adminserver hosts the standalone gRPC server that exposes the AdminAPI,
// kept separate from the general registry gRPC server so it can be run, scaled,
// and network-isolated independently.
package adminserver

import (
	"context"
	"net"
	"runtime/debug"

	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	adminv1 "github.com/ChargePi/chargeflow-registry/gen/proto/admin/v1"
	grpcHandler "github.com/ChargePi/chargeflow-registry/internal/grpc"
)

// Server wraps the gRPC server exposing the AdminAPI.
type Server struct {
	grpcServer *grpc.Server
	logger     *zap.Logger
}

// NewServer builds a gRPC server with tracing, logging, and recovery
// interceptors, with the health and admin services registered.
func NewServer(logger *zap.Logger, adminSvc grpcHandler.AdminService) *Server {
	logger = logger.Named("admin-grpc-server")

	recoveryHandler := func(p any) error {
		logger.Error("recovered from panic", zap.Any("panic", p), zap.String("stack", string(debug.Stack())))
		return status.Errorf(codes.Internal, "%s", p)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			grpc_zap.UnaryServerInterceptor(logger),
			grpc_recovery.UnaryServerInterceptor(grpc_recovery.WithRecoveryHandler(recoveryHandler)),
		),
		grpc.ChainStreamInterceptor(
			grpc_zap.StreamServerInterceptor(logger),
			grpc_recovery.StreamServerInterceptor(grpc_recovery.WithRecoveryHandler(recoveryHandler)),
		),
	)

	grpc_health_v1.RegisterHealthServer(grpcServer, health.NewServer())
	adminv1.RegisterAdminServiceServer(grpcServer, grpcHandler.NewAdminHandler(adminSvc))

	return &Server{grpcServer: grpcServer, logger: logger}
}

// Start listens on addr and serves in a goroutine, logging fatal on failure.
func (s *Server) Start(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go func() {
		s.logger.Info("Starting admin gRPC server", zap.String("address", addr))
		if err := s.grpcServer.Serve(lis); err != nil {
			s.logger.Fatal("failed to serve admin gRPC", zap.Error(err))
		}
	}()

	return nil
}

// Shutdown gracefully stops the gRPC server.
func (s *Server) Shutdown(_ context.Context) {
	s.grpcServer.GracefulStop()
}
