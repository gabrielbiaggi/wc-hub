package app

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/webcreations/wc-hub/back/internal/platform/config"
)

func TestDevelopmentMasterEnvironmentGuard(t *testing.T) {
	for _, environment := range []string{"development", "local", "test", " DEVELOPMENT "} {
		if !developmentMasterAllowed(environment) {
			t.Fatalf("expected %q to allow development master", environment)
		}
	}
	for _, environment := range []string{"production", "staging", "", "prod"} {
		if developmentMasterAllowed(environment) {
			t.Fatalf("expected %q to reject development master", environment)
		}
	}
}

func TestApplicationRejectsDevelopmentMasterWithoutStrongSecret(t *testing.T) {
	_, _, err := New(context.Background(), config.Config{
		DatabaseURL:               "postgres://not-used",
		Environment:               "production",
		DevelopmentMasterLogin:    true,
		DevelopmentMasterTimezone: "America/Sao_Paulo",
		DevelopmentMasterEmail:    "gabrielbiaggi3@gmail.com",
	}, slog.Default())
	if err == nil || !strings.Contains(err.Error(), "MASTER_SECRET") {
		t.Fatalf("expected secret guard, got %v", err)
	}
}
