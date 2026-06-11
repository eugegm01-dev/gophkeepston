package authclient

import (
	"context"
	"fmt"

	authpb "github.com/eugegm01-dev/gophkeepston/api/proto/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn *grpc.ClientConn
	auth authpb.AuthClient
}

func NewClient(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %w", err)
	}
	return &Client{
		conn: conn,
		auth: authpb.NewAuthClient(conn),
	}, nil
}

func (c *Client) Register(ctx context.Context, login string, encSecret []byte) (string, error) {
	resp, err := c.auth.Register(ctx, &authpb.RegisterRequest{
		Login:           login,
		EncryptedSecret: encSecret,
	})
	if err != nil {
		return "", err
	}
	return resp.UserId, nil
}

func (c *Client) Login(ctx context.Context, login string) (*authpb.LoginResponse, error) {
	return c.auth.Login(ctx, &authpb.LoginRequest{Login: login})
}

func (c *Client) Close() error {
	return c.conn.Close()
}
