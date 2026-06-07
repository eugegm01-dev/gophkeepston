package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	authpb "github.com/eugegm01-dev/gophkeepston/api/proto/auth"
	syncpb "github.com/eugegm01-dev/gophkeepston/api/proto/sync"
	"github.com/eugegm01-dev/gophkeepston/internal/config"
	"github.com/eugegm01-dev/gophkeepston/internal/pkg/jwt"
	"github.com/eugegm01-dev/gophkeepston/internal/server/auth"
	"github.com/eugegm01-dev/gophkeepston/internal/server/middleware"
	"github.com/eugegm01-dev/gophkeepston/internal/server/storage"
	"github.com/eugegm01-dev/gophkeepston/internal/server/sync"
)

func main() {
	// 1. Инициализация логгера
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// 2. Парсинг конфигурации (один раз!)
	cfg, err := config.Load() // ← строка ~25, заменить весь блок парсинга
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}
	// 3. Инициализация БД
	ctx := context.Background()
	db, err := storage.NewPostgresDB(ctx, cfg.DatabaseDSN)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// 4. Инициализация JWT менеджера
	jwtManager := jwt.NewManager(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	rateLimiter := middleware.NewRateLimiter(5, 5*time.Minute)
	if cfg.DatabaseDSN == "" {
		slog.Error("DATABASE_DSN is required")
		os.Exit(1)
	}
	if cfg.JWTSecret == "" {
		slog.Error("JWT_SECRET is required")
		os.Exit(1)
	}
	if cfg.AccessTokenTTL <= 0 || cfg.RefreshTokenTTL <= 0 {
		slog.Error("JWT TTLs must be positive")
		os.Exit(1)
	}
	if _, err := os.Stat(cfg.TLSCertPath); err != nil {
		slog.Error("TLS certificate not found", "path", cfg.TLSCertPath)
		os.Exit(1)
	}
	if _, err := os.Stat(cfg.TLSKeyPath); err != nil {
		slog.Error("TLS key not found", "path", cfg.TLSKeyPath)
		os.Exit(1)
	}

	// 5. Инициализация TLS
	creds, err := credentials.NewServerTLSFromFile(cfg.TLSCertPath, cfg.TLSKeyPath)
	if err != nil {
		slog.Error("Failed to load TLS credentials", "error", err)
		os.Exit(1)
	}

	// 6. Запуск gRPC сервера
	grpcServer := grpc.NewServer(
		grpc.Creds(creds),
		grpc.UnaryInterceptor(middleware.UnaryAuthInterceptor(jwtManager, rateLimiter)),
	)
	authSvc := auth.NewAuthService(db, jwtManager, rateLimiter)
	authpb.RegisterAuthServer(grpcServer, authSvc)
	syncSvc := sync.NewSyncService(db)
	syncpb.RegisterSyncServer(grpcServer, syncSvc)
	// 7. Слушатель
	lis, err := net.Listen("tcp", cfg.ServerAddr)
	if err != nil {
		slog.Error("Failed to listen", "error", err)
		os.Exit(1)
	}

	go func() {
		slog.Info("gRPC server is running", "addr", cfg.ServerAddr)
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("Failed to serve", "error", err)
		}
	}()

	// 8. Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("Shutting down gracefully...")
	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		slog.Info("Server stopped")
	case <-time.After(5 * time.Second):
		slog.Warn("Force stopping server")
		grpcServer.Stop()
	}
}
