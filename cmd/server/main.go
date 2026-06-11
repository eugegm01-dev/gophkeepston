package main

import (
	"log"
	"net"

	authpb "github.com/eugegm01-dev/gophkeepston/api/proto/auth"
	syncpb "github.com/eugegm01-dev/gophkeepston/api/proto/sync"
	"github.com/eugegm01-dev/gophkeepston/internal/server/auth"
	"github.com/eugegm01-dev/gophkeepston/internal/server/storage"
	"google.golang.org/grpc"
)

func main() {
	db, err := storage.NewPostgresDB("postgres://gophkeepston:secret@127.0.0.1:5432/gophkeepston?sslmode=disable")
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	authSvc := auth.NewAuthService(db, "my-secret-jwt-key")
	syncSvc := &syncpb.UnimplementedSyncServer{} // заглушка

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	authpb.RegisterAuthServer(grpcServer, authSvc)
	syncpb.RegisterSyncServer(grpcServer, syncSvc)

	log.Println("gRPC server listening on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
