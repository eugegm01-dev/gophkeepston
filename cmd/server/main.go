package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	authpb "github.com/eugegm01-dev/gophkeepston/api/proto/auth"
	syncpb "github.com/eugegm01-dev/gophkeepston/api/proto/sync"
	"github.com/eugegm01-dev/gophkeepston/internal/pkg/jwt"
	"github.com/eugegm01-dev/gophkeepston/internal/server/auth"
	"github.com/eugegm01-dev/gophkeepston/internal/server/middleware"
	"github.com/eugegm01-dev/gophkeepston/internal/server/storage"
	serversync "github.com/eugegm01-dev/gophkeepston/internal/server/sync"
	"google.golang.org/grpc"
)

func main() {
	db, err := storage.NewPostgresDB("postgres://gophkeepston:secret@127.0.0.1:5432/gophkeepston?sslmode=disable")
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	jwtManager := jwt.NewManager("my-secret-jwt-key")

	authSvc := auth.NewAuthService(db, "my-secret-jwt-key")
	syncSvc := serversync.NewSyncService(db)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.UnaryAuthInterceptor(jwtManager)),
	)

	authpb.RegisterAuthServer(grpcServer, authSvc)
	syncpb.RegisterSyncServer(grpcServer, syncSvc)

	log.Println("gRPC server listening on :50051")
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down gracefully...")
		grpcServer.GracefulStop()
	}()
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
