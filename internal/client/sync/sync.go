// Package sync implements client-side data synchronization with the server.
// It provides Push/Pull operations and a FullSync function for initial sync.
package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	syncpb "github.com/eugegm01-dev/gophkeepston/api/proto/sync"
	"github.com/eugegm01-dev/gophkeepston/internal/client/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

var NewClientFunc = NewClient

// Client is a gRPC-based sync client.
type Client struct {
	conn  *grpc.ClientConn
	sync  syncpb.SyncClient
	token string
}

// NewClient creates a new sync client connected to the given address.
func NewClient(addr, token string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	return &Client{
		conn:  conn,
		sync:  syncpb.NewSyncClient(conn),
		token: token,
	}, nil
}

// Push sends encrypted entries to the server.
func (c *Client) Push(ctx context.Context, entries []*syncpb.Entry) error {
	md := metadata.Pairs("authorization", "Bearer "+c.token)
	ctx = metadata.NewOutgoingContext(ctx, md)

	_, err := c.sync.Push(ctx, &syncpb.PushRequest{Entries: entries})
	return err
}

// Pull retrieves new entries from the server since a given version.
func (c *Client) Pull(ctx context.Context, sinceVersion int64) ([]*syncpb.Entry, error) {
	md := metadata.Pairs("authorization", "Bearer "+c.token)
	ctx = metadata.NewOutgoingContext(ctx, md)

	resp, err := c.sync.Pull(ctx, &syncpb.PullRequest{SinceVersion: sinceVersion})
	if err != nil {
		return nil, err
	}
	return resp.Entries, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// FullSync performs a full two-way sync: uploads local entries and downloads new server entries.
func FullSync(st *store.Store, userID string, token string, serverAddr string) error {
	cli, err := NewClientFunc(serverAddr, token)
	if err != nil {
		return err
	}
	defer cli.Close()

	// Получаем максимальную локальную версию (можно хранить в отдельной записи или вычислять)
	maxLocalVersion := getMaxLocalVersion(st, userID)

	// Pull
	serverEntries, err := cli.Pull(context.Background(), maxLocalVersion)
	if err != nil {
		return err
	}
	for _, se := range serverEntries {
		// Если локально нет или версия сервера новее – сохраняем
		localData, err := st.Get(userID, se.Id)
		if err != nil || getVersion(localData) < se.Version {
			st.Put(userID, se.Id, se.EncryptedData)
		}
	}

	// Push – отправляем только изменённые локальные записи
	localIDs, err := st.List(userID)
	if err != nil {
		return err
	}
	for _, id := range localIDs {
		data, err := st.Get(userID, id)
		if err != nil {
			continue
		}
		ver := getVersion(data)
		if ver > maxLocalVersion { // изменилась после последней синхронизации
			entry := &syncpb.Entry{
				Id:            id,
				Type:          "password", // можно извлечь из метаданных
				EncryptedData: data,
				Version:       ver,
				UpdatedAt:     time.Now().Unix(),
			}
			cli.Push(context.Background(), []*syncpb.Entry{entry})
		}
	}
	return nil
}

func getVersion(data []byte) int64 {
	var meta struct{ Version int64 }
	json.Unmarshal(data, &meta) // предполагаем, что в данных есть поле version
	return meta.Version
}

func getMaxLocalVersion(st *store.Store, userID string) int64 {
	ids, _ := st.List(userID)
	var max int64
	for _, id := range ids {
		data, _ := st.Get(userID, id)
		v := getVersion(data)
		if v > max {
			max = v
		}
	}
	return max
}

// NewClientWithConn creates a sync client using an existing gRPC connection.
func NewClientWithConn(conn *grpc.ClientConn, token string) *Client {
	return &Client{
		conn:  conn,
		sync:  syncpb.NewSyncClient(conn),
		token: token,
	}
}
