package cloudflare

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestEncryptedCredentialAndAllowlists(t *testing.T) {
	kek := []byte("0123456789abcdef0123456789abcdef")
	token := []byte("cloudflare-scoped-token-value")
	envelope, err := SealToken(kek, token)
	if err != nil {
		t.Fatal(err)
	}
	decryptor, err := NewAESGCMEnvelopeDecryptor(kek)
	if err != nil {
		t.Fatal(err)
	}
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.cloudflare.com" || r.Header.Get("Authorization") != "Bearer "+string(token) {
			t.Fatalf("unexpected Cloudflare request: %s auth=%q", r.URL, r.Header.Get("Authorization"))
		}
		body := `{"success":true,"errors":[],"result":[],"result_info":{"page":1,"total_pages":1}}`
		if strings.Contains(r.URL.Path, "/tunnels") {
			body = `{"success":true,"errors":[],"result":[{"id":"t1","name":"home","status":"healthy","connections":[]}],"result_info":{"page":1,"total_pages":1}}`
		} else if strings.Contains(r.URL.Path, "/dns_records") {
			body = `{"success":true,"errors":[],"result":[{"id":"r1","type":"A","name":"home.example.com","content":"192.0.2.1","proxied":true}],"result_info":{"page":1,"total_pages":1}}`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	client, err := New(Config{
		CredentialSource: CredentialSourceFunc(func(context.Context) (TokenEnvelope, error) { return envelope, nil }),
		Decryptor:        decryptor,
		AllowedAccounts:  []string{"account-1"},
		AllowedZones:     []string{"zone-1"},
		HTTPClient:       &http.Client{Transport: transport},
	})
	if err != nil {
		t.Fatal(err)
	}
	tunnels, err := client.ListTunnels(context.Background(), "account-1")
	if err != nil || len(tunnels) != 1 || tunnels[0].Status != "healthy" {
		t.Fatalf("unexpected tunnels: %#v err=%v", tunnels, err)
	}
	records, err := client.ListDNSRecords(context.Background(), "zone-1")
	if err != nil || len(records) != 1 || records[0].ZoneID != "zone-1" {
		t.Fatalf("unexpected records: %#v err=%v", records, err)
	}
	if _, err = client.ListTunnels(context.Background(), "outside"); !errors.Is(err, ErrAccountNotAllowed) {
		t.Fatalf("expected account allowlist error, got %v", err)
	}
	if _, err = client.ListDNSRecords(context.Background(), "outside"); !errors.Is(err, ErrZoneNotAllowed) {
		t.Fatalf("expected zone allowlist error, got %v", err)
	}
}

func TestTamperedEnvelopeIsRejected(t *testing.T) {
	kek := []byte("0123456789abcdef0123456789abcdef")
	envelope, err := SealToken(kek, []byte("cloudflare-scoped-token-value"))
	if err != nil {
		t.Fatal(err)
	}
	envelope.Ciphertext[0] ^= 0xff
	decryptor, _ := NewAESGCMEnvelopeDecryptor(kek)
	if _, err = decryptor.Decrypt(context.Background(), envelope); err == nil {
		t.Fatal("expected tampered credential to be rejected")
	}
}
