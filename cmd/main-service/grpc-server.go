package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	"github.com/FischukSergey/otus-ms/internal/handlers/sources"
	"github.com/FischukSergey/otus-ms/internal/jwks"
	custommiddleware "github.com/FischukSergey/otus-ms/internal/middleware"
	sourcerepo "github.com/FischukSergey/otus-ms/internal/store/sources"
	pb "github.com/FischukSergey/otus-ms/pkg/news_sources/v1"
)

// GRPCServer оборачивает gRPC сервер и управляет его жизненным циклом.
type GRPCServer struct {
	server *grpc.Server
	addr   string
	logger *slog.Logger
}

// GRPCServerDeps содержит зависимости для инициализации gRPC сервера.
type GRPCServerDeps struct {
	Addr        string
	DB          *pgxpool.Pool
	JWKSManager *jwks.Manager
	JWTIssuer   string
	SkipVerify  bool
	Logger      *slog.Logger
}

// NewGRPCServer создаёт gRPC сервер с JWT-аутентификацией.
func NewGRPCServer(deps *GRPCServerDeps) *GRPCServer {
	authInterceptor := custommiddleware.UnaryAuthInterceptor(
		custommiddleware.GRPCAuthConfig{
			Issuer:     deps.JWTIssuer,
			SkipVerify: deps.SkipVerify,
		},
		deps.JWKSManager,
		deps.Logger,
	)

	srv := grpc.NewServer(
		grpc.UnaryInterceptor(authInterceptor),
	)

	repo := sourcerepo.NewRepository(deps.DB)
	handler := sources.NewGRPCHandler(repo, deps.Logger)
	pb.RegisterNewsSourcesServiceServer(srv, handler)

	return &GRPCServer{
		server: srv,
		addr:   deps.Addr,
		logger: deps.Logger,
	}
}

// Start запускает gRPC сервер (блокирующий вызов).
func (s *GRPCServer) Start() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("grpc listen %s: %w", s.addr, err)
	}
	s.logger.Info("gRPC server starting", "addr", s.addr)
	if err := s.server.Serve(lis); err != nil {
		return fmt.Errorf("grpc serve: %w", err)
	}
	return nil
}

// Stop выполняет graceful shutdown gRPC сервера.
func (s *GRPCServer) Stop(_ context.Context) {
	s.logger.Info("gRPC server stopping...")
	s.server.GracefulStop()
	s.logger.Info("gRPC server stopped")
}
