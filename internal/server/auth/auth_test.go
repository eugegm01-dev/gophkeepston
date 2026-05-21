package auth

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	authpb "github.com/eugegm01-dev/gophkeepston/api/proto/auth"
)

func TestRegisterAndLogin(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	s := NewAuthService(db, "test-secret")

	// Register: ожидаем вставку
	mock.ExpectExec("INSERT INTO users").WithArgs(sqlmock.AnyArg(), "testuser", []byte("encrypted")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	resp, err := s.Register(context.Background(), &authpb.RegisterRequest{Login: "testuser", EncryptedSecret: []byte("encrypted")})
	if err != nil {
		t.Fatal(err)
	}
	if resp.UserId == "" {
		t.Error("expected user_id")
	}

	// Login: запрос вернёт id и encrypted_secret
	rows := sqlmock.NewRows([]string{"id", "encrypted_secret"}).AddRow("user123", []byte("encrypted"))
	mock.ExpectQuery("SELECT id, encrypted_secret FROM users WHERE login=\\$1").WithArgs("testuser").WillReturnRows(rows)

	// После успешного SELECT будет INSERT в refresh_tokens
	mock.ExpectExec(`INSERT INTO refresh_tokens`).WithArgs(
		sqlmock.AnyArg(),
		"user123",
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
	).WillReturnResult(sqlmock.NewResult(1, 1))

	loginResp, err := s.Login(context.Background(), &authpb.LoginRequest{Login: "testuser"})
	if err != nil {
		t.Fatal(err)
	}
	if loginResp.EncryptedSecret == nil || loginResp.AccessToken == "" || loginResp.RefreshToken == "" {
		t.Error("missing login fields")
	}

	// Проверяем, что все ожидания выполнены
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
