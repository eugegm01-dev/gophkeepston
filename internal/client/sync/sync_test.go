package sync

import (
	"context"
	"net"
	"testing"
	"time"

	syncpb "github.com/eugegm01-dev/gophkeepston/api/proto/sync"
	"github.com/eugegm01-dev/gophkeepston/internal/client/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type fakeSyncServer struct {
	syncpb.UnimplementedSyncServer
	pushed []*syncpb.Entry
}

func (s *fakeSyncServer) Push(ctx context.Context, req *syncpb.PushRequest) (*syncpb.PushResponse, error) {
	s.pushed = append(s.pushed, req.Entries...)
	return &syncpb.PushResponse{}, nil
}

func (s *fakeSyncServer) Pull(ctx context.Context, req *syncpb.PullRequest) (*syncpb.PullResponse, error) {
	return &syncpb.PullResponse{
		Entries: []*syncpb.Entry{
			{Id: "entry1", EncryptedData: []byte("data1"), Version: 1, UpdatedAt: time.Now().Unix()},
		},
	}, nil
}

func TestPushAndPull(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	fake := &fakeSyncServer{}
	syncpb.RegisterSyncServer(srv, fake)
	go srv.Serve(lis)
	defer srv.Stop()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := NewClientWithConn(conn, "test-token")

	// Push
	entries := []*syncpb.Entry{
		{Id: "entry1", EncryptedData: []byte("secret1"), Version: 1},
	}
	if err := client.Push(context.Background(), entries); err != nil {
		t.Fatal(err)
	}
	if len(fake.pushed) != 1 || fake.pushed[0].Id != "entry1" {
		t.Errorf("push failed: %+v", fake.pushed)
	}

	// Pull
	pulled, err := client.Pull(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pulled) != 1 || pulled[0].Id != "entry1" {
		t.Errorf("pull failed: %+v", pulled)
	}
}
func TestFullSync(t *testing.T) {
	// Создаём временное хранилище
	st, err := store.NewStore("test_fullsync.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	userID := "alice"
	// Кладём локальную запись
	st.Put(userID, "entry1", []byte("data1"))

	// Поднимаем фейковый сервер
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	fake := &fakeSyncServer{}
	syncpb.RegisterSyncServer(srv, fake)
	go srv.Serve(lis)
	defer srv.Stop()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Подменяем NewClientFunc на создание клиента с нашим соединением
	oldFunc := NewClientFunc
	NewClientFunc = func(addr, token string) (*Client, error) {
		return NewClientWithConn(conn, token), nil
	}
	defer func() { NewClientFunc = oldFunc }()

	err = FullSync(st, userID, "test-token", "bufnet")
	if err != nil {
		t.Fatal(err)
	}

	if len(fake.pushed) != 1 || fake.pushed[0].Id != "entry1" {
		t.Errorf("push failed: %+v", fake.pushed)
	}
}
