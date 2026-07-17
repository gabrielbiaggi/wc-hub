package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment     string
	HTTPAddr        string
	PublicURL       string
	DatabaseURL     string
	LocalTargetName string
	SelfProtected   bool
	LocalAllowlist  []string
	LogLevelValue   string
	SessionTTL      time.Duration
	SecureCookies   bool
	EncryptionKey   string
	TOTPIssuer      string
}

func Load() Config {
	return Config{
		Environment:     env("WC_HUB_ENV", "development"),
		HTTPAddr:        env("WC_HUB_HTTP_ADDR", ":8080"),
		PublicURL:       env("WC_HUB_PUBLIC_URL", "http://localhost:5173"),
		DatabaseURL:     env("WC_HUB_DATABASE_URL", ""),
		LocalTargetName: env("WC_HUB_LOCAL_TARGET_NAME", "wc-hub-local"),
		SelfProtected:   envBool("WC_HUB_SELF_PROTECTED", true),
		LocalAllowlist:  split(env("WC_HUB_LOCAL_COMMAND_ALLOWLIST", "uptime,df,free,ip,ss,journalctl,docker,kubectl")),
		LogLevelValue:   env("WC_HUB_LOG_LEVEL", "info"),
		SessionTTL:      envDuration("WC_HUB_SESSION_TTL", 12*time.Hour),
		SecureCookies:   envBool("WC_HUB_SECURE_COOKIES", false),
		EncryptionKey:   env("WC_HUB_ENCRYPTION_KEY", ""),
		TOTPIssuer:      env("WC_HUB_TOTP_ISSUER", "WC Hub"),
	}
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func (c Config) LogLevel() slog.Level {
	switch strings.ToLower(c.LogLevelValue) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func envBool(key string, fallback bool) bool {
	value, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}
func split(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if v := strings.TrimSpace(item); v != "" {
			out = append(out, v)
		}
	}
	return out
}
