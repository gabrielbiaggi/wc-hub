package application

import (
	"context"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/webcreations/wc-hub/back/internal/auth/domain"
)

type Tokens struct {
	Session string      `json:"-"`
	CSRF    string      `json:"csrf_token"`
	Expires time.Time   `json:"expires_at"`
	User    domain.User `json:"user"`
}

type Service struct {
	repo         domain.Repository
	ttl          time.Duration
	aead         cipher.AEAD
	issuer       string
	masterEmail  string
	masterSecret []byte
}

func New(repo domain.Repository, ttl time.Duration) *Service { return &Service{repo: repo, ttl: ttl} }

func (s *Service) BootstrapOpen(ctx context.Context) (bool, error) { return s.repo.BootstrapOpen(ctx) }

func (s *Service) Bootstrap(ctx context.Context, credentials domain.Credentials, userAgent, remoteIP string) (Tokens, error) {
	credentials.Email = strings.ToLower(strings.TrimSpace(credentials.Email))
	credentials.DisplayName = strings.TrimSpace(credentials.DisplayName)
	if err := validate(credentials); err != nil {
		return Tokens{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(credentials.Password), 12)
	if err != nil {
		return Tokens{}, err
	}
	user, err := s.repo.Bootstrap(ctx, credentials, string(hash))
	if err != nil {
		return Tokens{}, err
	}
	return s.issue(ctx, user, userAgent, remoteIP, time.Now().UTC().Add(s.ttl))
}

func (s *Service) Login(ctx context.Context, email, password, userAgent, remoteIP string) (Tokens, error) {
	user, passwordHash, err := s.repo.PasswordIdentity(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		// Keep a comparable password-hash workload when the identity is absent.
		if passwordHash == "" {
			_ = bcrypt.CompareHashAndPassword([]byte("$2a$12$QJAVmY6X9SCSud5Vv0wRveW6KuX0zYBjvlD9wh7yZK1dXjPjLhXQe"), []byte(password))
		}
		return Tokens{}, domain.ErrInvalidCredentials
	}
	return s.issue(ctx, user, userAgent, remoteIP, time.Now().UTC().Add(s.ttl))
}

func DevelopmentMasterPassword(secret []byte, now time.Time, location *time.Location) string {
	if location == nil {
		location = time.UTC
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte("wc-hub/master-password/v1\x00" + now.In(location).Format("2006010215")))
	code := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return "Hub-" + code[:16]
}

func (s *Service) ConfigureDevelopmentMaster(email, encodedSecret string) error {
	parsedEmail, secret, err := ValidateDevelopmentMasterConfig(email, encodedSecret)
	if err != nil {
		return err
	}
	s.masterEmail, s.masterSecret = parsedEmail, secret
	return nil
}

func ValidateDevelopmentMasterConfig(email, encodedSecret string) (string, []byte, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if _, err := mail.ParseAddress(email); err != nil {
		return "", nil, fmt.Errorf("WC_HUB_MASTER_EMAIL must be a valid email")
	}
	secret, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedSecret))
	if err != nil || len(secret) < 32 {
		return "", nil, fmt.Errorf("WC_HUB_MASTER_SECRET must be base64-encoded and contain at least 32 bytes")
	}
	return email, secret, nil
}

func developmentMasterExpiry(now time.Time, location *time.Location) time.Time {
	if location == nil {
		location = time.UTC
	}
	return now.In(location).Truncate(time.Hour).Add(time.Hour).UTC()
}

func (s *Service) LoginDevelopmentMaster(ctx context.Context, username, password, totpCode, userAgent, remoteIP string, now time.Time, location *time.Location) (Tokens, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	expected := DevelopmentMasterPassword(s.masterSecret, now, location)
	validPassword := len(password) == len(expected) && subtle.ConstantTimeCompare([]byte(password), []byte(expected)) == 1
	validIdentity := username == domain.DevelopmentMasterUsername || username == s.masterEmail
	if len(s.masterSecret) < 32 || !validIdentity || !validPassword {
		return Tokens{}, domain.ErrInvalidCredentials
	}
	user, _, err := s.repo.PasswordIdentity(ctx, s.masterEmail)
	if err != nil {
		return Tokens{}, domain.ErrInvalidCredentials
	}
	if user.TOTPEnabled {
		valid, verifyErr := s.VerifyTOTP(ctx, user.ID, strings.TrimSpace(totpCode))
		if verifyErr != nil || !valid {
			return Tokens{}, domain.ErrInvalidCredentials
		}
	}
	return s.issue(ctx, user, userAgent, remoteIP, developmentMasterExpiry(now, location))
}

func (s *Service) Authenticate(ctx context.Context, token string) (domain.Session, error) {
	if token == "" {
		return domain.Session{}, domain.ErrUnauthorized
	}
	session, err := s.repo.SessionByToken(ctx, digest(token))
	if err != nil || time.Now().After(session.Expires) {
		return domain.Session{}, domain.ErrUnauthorized
	}
	return session, nil
}

func (s *Service) VerifyCSRF(session domain.Session, token string) bool {
	want := session.CSRFHash
	got := digest(token)
	return len(want) == len(got) && subtle.ConstantTimeCompare(want, got) == 1
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.repo.DeleteSession(ctx, digest(token))
}

func (s *Service) RefreshCSRF(ctx context.Context, sessionID string) (string, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", err
	}
	if err = s.repo.UpdateCSRF(ctx, sessionID, digest(token)); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) issue(ctx context.Context, user domain.User, userAgent, remoteIP string, expires time.Time) (Tokens, error) {
	sessionToken, err := randomToken(32)
	if err != nil {
		return Tokens{}, err
	}
	csrfToken, err := randomToken(32)
	if err != nil {
		return Tokens{}, err
	}
	_, err = s.repo.CreateSession(ctx, user.ID, digest(sessionToken), digest(csrfToken), userAgent, remoteIP, expires)
	if err != nil {
		return Tokens{}, err
	}
	return Tokens{Session: sessionToken, CSRF: csrfToken, Expires: expires, User: user}, nil
}

func validate(credentials domain.Credentials) error {
	if _, err := mail.ParseAddress(credentials.Email); err != nil {
		return fmt.Errorf("valid email is required")
	}
	if len(credentials.DisplayName) < 2 {
		return fmt.Errorf("display name is required")
	}
	if len(credentials.Password) < 14 {
		return fmt.Errorf("password must contain at least 14 characters")
	}
	return nil
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
func digest(value string) []byte { result := sha256.Sum256([]byte(value)); return result[:] }
