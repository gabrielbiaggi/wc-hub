package application

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"
)

func (s *Service) ConfigureTOTP(key, issuer string) error {
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(decoded) != 32 {
		return fmt.Errorf("WC_HUB_ENCRYPTION_KEY must be a base64-encoded 32-byte key")
	}
	block, err := aes.NewCipher(decoded)
	if err != nil {
		return err
	}
	s.aead, err = cipher.NewGCM(block)
	if err != nil {
		return err
	}
	s.issuer = issuer
	return nil
}

func (s *Service) EnrollTOTP(ctx context.Context, userID, email string) (string, string, error) {
	if s.aead == nil {
		return "", "", fmt.Errorf("TOTP encryption is not configured")
	}
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	encrypted, err := s.encrypt([]byte(secret))
	if err != nil {
		return "", "", err
	}
	if err = s.repo.StoreTOTPSecret(ctx, userID, encrypted); err != nil {
		return "", "", err
	}
	label := url.PathEscape(s.issuer + ":" + email)
	query := url.Values{"secret": {secret}, "issuer": {s.issuer}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	return secret, "otpauth://totp/" + label + "?" + query.Encode(), nil
}

func (s *Service) ConfirmTOTP(ctx context.Context, userID, code string) error {
	valid, err := s.VerifyTOTP(ctx, userID, code)
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("invalid TOTP code")
	}
	return s.repo.EnableTOTP(ctx, userID)
}

func (s *Service) VerifyTOTP(ctx context.Context, userID, code string) (bool, error) {
	if s.aead == nil || len(code) != 6 {
		return false, nil
	}
	encrypted, err := s.repo.TOTPSecret(ctx, userID)
	if err != nil {
		return false, err
	}
	plain, err := s.decrypt(encrypted)
	if err != nil {
		return false, err
	}
	counter := time.Now().Unix() / 30
	for offset := int64(-1); offset <= 1; offset++ {
		if hmac.Equal([]byte(totpCode(string(plain), counter+offset)), []byte(code)) {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) encrypt(value []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return s.aead.Seal(nonce, nonce, value, nil), nil
}
func (s *Service) decrypt(value []byte) ([]byte, error) {
	size := s.aead.NonceSize()
	if len(value) < size {
		return nil, fmt.Errorf("invalid encrypted TOTP secret")
	}
	return s.aead.Open(nil, value[:size], value[size:], nil)
}
func totpCode(secret string, counter int64) string {
	key, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	message := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		message[i] = byte(counter)
		counter >>= 8
	}
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(message)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	binary := int(sum[offset]&0x7f)<<24 | int(sum[offset+1])<<16 | int(sum[offset+2])<<8 | int(sum[offset+3])
	return fmt.Sprintf("%06d", binary%1000000)
}
