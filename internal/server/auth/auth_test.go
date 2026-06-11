package auth

import (
	"context"
	"testing"
	"time"

	authpb "github.com/eugegm01-dev/gophkeepston/api/proto/auth"
	"github.com/eugegm01-dev/gophkeepston/internal/pkg/jwt"
	"github.com/eugegm01-dev/gophkeepston/internal/server/middleware"
	pgxmock "github.com/pashagolub/pgxmock/v2"
)

func TestRegisterAndLogin(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	jwtMgr := jwt.NewManager("test-secret", 15*time.Minute, 72*time.Hour)
	rateLimiter := middleware.NewRateLimiter(5, 5*time.Minute)
	s := NewAuthService(mock, jwtMgr, rateLimiter)

	mock.ExpectQuery("SELECT id, encrypted_secret, salt FROM users WHERE login=\\$1").
		WithArgs("testuser").
		WillReturnRows(mock.NewRows([]string{"id", "encrypted_secret", "salt"}).
			AddRow("user123", []byte("enc"), []byte("salt123")))

	mock.ExpectExec("INSERT INTO refresh_tokens").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	resp, err := s.Login(context.Background(), &authpb.LoginRequest{Login: "testuser"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.EncryptedSecret == nil || resp.AccessToken == "" {
		t.Error("missing fields")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled: %v", err)
	}
}

func TestRefreshToken(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	jwtMgr := jwt.NewManager("test-secret", 15*time.Minute, 72*time.Hour)
	rateLimiter := middleware.NewRateLimiter(5, 5*time.Minute)
	s := NewAuthService(mock, jwtMgr, rateLimiter)

	mock.ExpectQuery(`SELECT user_id FROM refresh_tokens WHERE token_hash = \$1 AND expires_at > now\(\)`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(mock.NewRows([]string{"user_id"}).AddRow("user42"))

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM refresh_tokens WHERE token_hash = \$1`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectExec(`INSERT INTO refresh_tokens`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	resp, err := s.RefreshToken(context.Background(), &authpb.RefreshTokenRequest{RefreshToken: "valid_refresh"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.AccessToken == "" {
		t.Error("expected access token")
	}

	mock.ExpectQuery(`SELECT user_id FROM refresh_tokens`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(pgxmock.ErrCancelled)

	_, err = s.RefreshToken(context.Background(), &authpb.RefreshTokenRequest{RefreshToken: "bad"})
	if err == nil {
		t.Error("expected error for invalid token")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled: %v", err)
	}
}
