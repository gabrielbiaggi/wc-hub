package application

import (
	"context"
	"testing"
	"time"

	"github.com/webcreations/wc-hub/back/internal/auth/domain"
)

func TestDevelopmentMasterPasswordAndExpiryUseConfiguredTimezone(t *testing.T) {
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.July, 19, 16, 42, 10, 0, location)
	if got := DevelopmentMasterPassword(now, location); got != "Hub1907202616" {
		t.Fatalf("unexpected hourly password: %s", got)
	}
	expires := developmentMasterExpiry(now, location)
	want := time.Date(2026, time.July, 19, 17, 0, 0, 0, location).UTC()
	if !expires.Equal(want) {
		t.Fatalf("unexpected expiry: got %s want %s", expires, want)
	}
}

type developmentMasterRepository struct {
	identity       string
	sessionExpires time.Time
}

func (r *developmentMasterRepository) Bootstrap(context.Context, domain.Credentials, string) (domain.User, error) {
	return domain.User{}, nil
}
func (r *developmentMasterRepository) PasswordIdentity(_ context.Context, identity string) (domain.User, string, error) {
	r.identity = identity
	return domain.User{ID: "master-id", Email: identity, DisplayName: "All Might", Roles: []string{"god-admin"}, Permissions: []string{"overview.read"}}, "", nil
}
func (r *developmentMasterRepository) CreateSession(_ context.Context, _ string, _, _ []byte, _, _ string, expires time.Time) (string, error) {
	r.sessionExpires = expires
	return "session-id", nil
}
func (*developmentMasterRepository) SessionByToken(context.Context, []byte) (domain.Session, error) {
	return domain.Session{}, nil
}
func (*developmentMasterRepository) DeleteSession(context.Context, []byte) error      { return nil }
func (*developmentMasterRepository) UpdateCSRF(context.Context, string, []byte) error { return nil }
func (*developmentMasterRepository) StoreTOTPSecret(context.Context, string, []byte) error {
	return nil
}
func (*developmentMasterRepository) TOTPSecret(context.Context, string) ([]byte, error) {
	return nil, nil
}
func (*developmentMasterRepository) EnableTOTP(context.Context, string) error    { return nil }
func (*developmentMasterRepository) BootstrapOpen(context.Context) (bool, error) { return false, nil }

func TestDevelopmentMasterLoginIssuesOnlyAnHourlySession(t *testing.T) {
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.July, 19, 16, 42, 10, 0, location)
	repository := &developmentMasterRepository{}
	service := New(repository, 12*time.Hour)
	if err = service.ConfigureDevelopmentMaster("gabrielbiaggi3@gmail.com"); err != nil {
		t.Fatal(err)
	}
	password := DevelopmentMasterPassword(now, location)
	tokens, err := service.LoginDevelopmentMaster(context.Background(), "allmight", password, "", "test", "127.0.0.1", now, location)
	if err != nil {
		t.Fatal(err)
	}
	if repository.identity != "gabrielbiaggi3@gmail.com" || tokens.User.Email != "gabrielbiaggi3@gmail.com" {
		t.Fatalf("unexpected master identity: repo=%q user=%q", repository.identity, tokens.User.Email)
	}
	if !tokens.Expires.Equal(repository.sessionExpires) || !tokens.Expires.Equal(developmentMasterExpiry(now, location)) {
		t.Fatalf("master session did not expire at the hour boundary: %s", tokens.Expires)
	}
	if _, err = service.LoginDevelopmentMaster(context.Background(), "allmight", password+"x", "", "test", "127.0.0.1", now, location); err == nil {
		t.Fatal("expected an invalid hourly password to be rejected")
	}
}
