// Package authclient provides a gRPC client for the Auth service.
package authclient

import (
	"context"
	"fmt"

	authpb "github.com/eugegm01-dev/gophkeepston/api/proto/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is a gRPC client for authentication operations.
type Client struct {
	conn *grpc.ClientConn
	auth authpb.AuthClient
}

// NewClient creates a new auth client connected to the given address.
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

// Register sends a registration request and returns the new user ID.
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

// Login authenticates a user and returns tokens.
func (c *Client) Login(ctx context.Context, login string) (*authpb.LoginResponse, error) {
	return c.auth.Login(ctx, &authpb.LoginRequest{Login: login})
}

// Refresh refreshes an access token using a refresh token.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (*authpb.RefreshTokenResponse, error) {
	return c.auth.RefreshToken(ctx, &authpb.RefreshTokenRequest{RefreshToken: refreshToken})
}

func (c *Client) Close() error {
	return c.conn.Close()
}
