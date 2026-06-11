// Package authclient provides a gRPC client for the Auth service.
package authclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	authpb "github.com/eugegm01-dev/gophkeepston/api/proto/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Client is a gRPC client for authentication operations.
type Client struct {
	conn *grpc.ClientConn
	auth authpb.AuthClient
}

// #08: строим TLS-credentials.
// Если задана переменная окружения TLS_CA_CERT — загружаем свой CA (для самоподписанных сертификатов).
// Иначе используем системный пул доверенных сертификатов.
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
	// Системный пул — для продакшна с нормальными сертификатами
	return credentials.NewTLS(&tls.Config{}), nil
}

// NewClient creates a new auth client connected to the given address.
func NewClient(addr string) (*Client, error) {
	// #08: заменяем insecure на TLS.
	// Все токены и зашифрованные данные теперь защищены транспортным шифрованием.
	creds, err := tlsCredentials()
	if err != nil {
		return nil, fmt.Errorf("tls credentials: %w", err)
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
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
