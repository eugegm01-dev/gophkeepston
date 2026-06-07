package sync

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	syncpb "github.com/eugegm01-dev/gophkeepston/api/proto/sync"
	"github.com/eugegm01-dev/gophkeepston/internal/client/store"
	syncpolicy "github.com/eugegm01-dev/gophkeepston/internal/sync"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

var NewClientFunc = NewClient

type Client struct {
	conn  *grpc.ClientConn
	sync  syncpb.SyncClient
	token string
}

func tlsCredentials() (credentials.TransportCredentials, error) {
	caCertPath := os.Getenv("TLS_CA_CERT")
	if caCertPath != "" {
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("read CA cert: %w", err)
		}
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("parse CA cert: invalid PEM")
		}
		return credentials.NewTLS(&tls.Config{RootCAs: certPool}), nil
	}
	return credentials.NewTLS(&tls.Config{}), nil
}

func NewClient(addr, token string) (*Client, error) {
	creds, err := tlsCredentials()
	if err != nil {
		return nil, fmt.Errorf("tls credentials: %w", err)
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	return &Client{
		conn:  conn,
		sync:  syncpb.NewSyncClient(conn),
		token: token,
	}, nil
}

func (c *Client) Push(ctx context.Context, entries []*syncpb.Entry) error {
	md := metadata.Pairs("authorization", "Bearer "+c.token)
	ctx = metadata.NewOutgoingContext(ctx, md)
	_, err := c.sync.Push(ctx, &syncpb.PushRequest{Entries: entries})
	return err
}

func (c *Client) Pull(ctx context.Context, sinceVersion int64) ([]*syncpb.Entry, error) {
	md := metadata.Pairs("authorization", "Bearer "+c.token)
	ctx = metadata.NewOutgoingContext(ctx, md)
	resp, err := c.sync.Pull(ctx, &syncpb.PullRequest{SinceVersion: sinceVersion})
	if err != nil {
		return nil, err
	}
	return resp.Entries, nil
}

func (c *Client) Delete(ctx context.Context, entryID string, version int64) error {
	md := metadata.Pairs("authorization", "Bearer "+c.token)
	ctx = metadata.NewOutgoingContext(ctx, md)
	_, err := c.sync.Delete(ctx, &syncpb.DeleteRequest{EntryId: entryID, Version: version})
	return err
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func FullSync(ctx context.Context, st *store.Store, userID, token, serverAddr string) error {
	cli, err := NewClientFunc(serverAddr, token)
	if err != nil {
		return err
	}
	defer cli.Close()

	resolver, err := syncpolicy.NewResolver("last-write-wins")
	if err != nil {
		return err
	}

	// 1. PULL
	maxLocal, err := st.GetMaxVersion(userID)
	if err != nil {
		return err
	}
	serverEntries, err := cli.Pull(ctx, maxLocal)
	if err != nil {
		return err
	}
	for _, se := range serverEntries {
		localVer, _ := st.GetVersion(userID, se.Id)
		localEntry := &syncpolicy.Entry{ID: se.Id, Version: localVer, UpdatedAt: 0}
		remoteEntry := &syncpolicy.Entry{ID: se.Id, Version: se.Version, UpdatedAt: se.UpdatedAt}
		winner, err := resolver.Resolve(localEntry, remoteEntry)
		if err != nil {
			continue
		}
		if winner == remoteEntry {
			if err := st.Put(userID, se.Id, se.EncryptedData); err != nil {
				return err
			}
			_ = st.PutVersion(userID, se.Id, se.Version)
		}
	}

	// 2. PUSH
	ids, err := st.List(userID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		localVer, err := st.GetVersion(userID, id)
		if err != nil || localVer == 0 || localVer == -1 {
			continue
		}
		data, err := st.Get(userID, id)
		if err != nil {
			slog.Warn("failed to get entry for push", "id", id, "error", err)
			continue
		}

		entryType := "text"
		if len(data) > 0 && data[0] == '{' {
			var probe struct{ Type string } // ← probe declared HERE, inside loop
			probeLen := min(len(data), 64)
			if err := json.Unmarshal(data[:probeLen], &probe); err == nil && probe.Type != "" {
				entryType = probe.Type
			}
		}

		entry := &syncpb.Entry{
			Id:            id,
			Type:          entryType,
			EncryptedData: data,
			Version:       localVer,
			UpdatedAt:     time.Now().Unix(),
		}
		if err := cli.Push(ctx, []*syncpb.Entry{entry}); err != nil {
			return fmt.Errorf("push %s: %w", id, err)
		}
	}

	// 3. DELETE
	for _, id := range ids {
		localVer, err := st.GetVersion(userID, id)
		if err != nil || localVer != -1 {
			continue
		}
		if err := cli.Delete(ctx, id, localVer); err != nil {
			continue
		}
		_ = st.Delete(userID, id)
		_ = st.DeleteVersion(userID, id)
	}
	return nil
}

// NewClientWithConn creates a sync client using an existing gRPC connection.
// Добавлено для совместимости с тестами
func NewClientWithConn(conn *grpc.ClientConn, token string) *Client {
	return &Client{
		conn:  conn,
		sync:  syncpb.NewSyncClient(conn),
		token: token,
	}
}
