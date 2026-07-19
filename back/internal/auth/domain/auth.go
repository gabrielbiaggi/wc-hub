package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrBootstrapClosed    = errors.New("bootstrap is already complete")
	ErrUnauthorized       = errors.New("unauthorized")
)

const (
	DevelopmentMasterUsername = "allmight"
	DevelopmentMasterIdentity = "allmight"
)

type User struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	TOTPEnabled bool     `json:"totp_enabled"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

func (u User) Can(permission string) bool {
	for _, candidate := range u.Permissions {
		if candidate == permission {
			return true
		}
	}
	return false
}

type Credentials struct {
	Email       string
	DisplayName string
	Password    string
}

type Session struct {
	ID       string
	User     User
	CSRFHash []byte
	Expires  time.Time
}

type Repository interface {
	Bootstrap(context.Context, Credentials, string) (User, error)
	PasswordIdentity(context.Context, string) (User, string, error)
	CreateSession(context.Context, string, []byte, []byte, string, string, time.Time) (string, error)
	SessionByToken(context.Context, []byte) (Session, error)
	DeleteSession(context.Context, []byte) error
	UpdateCSRF(context.Context, string, []byte) error
	StoreTOTPSecret(context.Context, string, []byte) error
	TOTPSecret(context.Context, string) ([]byte, error)
	EnableTOTP(context.Context, string) error
	BootstrapOpen(context.Context) (bool, error)
}
