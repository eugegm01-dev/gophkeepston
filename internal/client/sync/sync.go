// Package sync implements client-side data synchronization with the server.
// It provides Push/Pull operations and a FullSync function for initial sync.
package sync

import (
	"context"
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

	// 1. Pull: получаем записи с версией больше локальной максимальной
	maxLocal, err := st.GetMaxVersion(userID)
	if err != nil {
		return err
	}
	serverEntries, err := cli.Pull(context.Background(), maxLocal)
	if err != nil {
		return err
	}
	for _, se := range serverEntries {
		localVer, _ := st.GetVersion(userID, se.Id)
		if se.Version > localVer {
			if err := st.Put(userID, se.Id, se.EncryptedData); err != nil {
				return err
			}
			_ = st.PutVersion(userID, se.Id, se.Version)
		}
	}

	// 2. Push: отправляем все локальные записи с версией > 0 (т.е. все новые/изменённые)
	ids, err := st.List(userID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		localVer, err := st.GetVersion(userID, id)
		if err != nil || localVer == 0 {
			continue
		}
		data, err := st.Get(userID, id)
		if err != nil {
			continue
		}
		entry := &syncpb.Entry{
			Id:            id,
			Type:          "password",
			EncryptedData: data,
			Version:       localVer,
			UpdatedAt:     time.Now().Unix(),
		}
		if err := cli.Push(context.Background(), []*syncpb.Entry{entry}); err != nil {
			return err
		}
	}
	return nil
}

// NewClientWithConn creates a sync client using an existing gRPC connection.
func NewClientWithConn(conn *grpc.ClientConn, token string) *Client {
	return &Client{
		conn:  conn,
		sync:  syncpb.NewSyncClient(conn),
		token: token,
	}
}
