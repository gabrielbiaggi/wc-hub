package app

import (
	"testing"
	"time"
)

func TestLoginLimiterProgressiveBackoff(t *testing.T) {
	limiter := newLoginLimiter()
	now := time.Now()
	keys := []string{"ip:127.0.0.1", "account:user"}
	for range 4 {
		limiter.failure(keys, now)
	}
	if allowed, _ := limiter.allow(keys, now); !allowed {
		t.Fatal("blocked before threshold")
	}
	limiter.failure(keys, now)
	if allowed, retry := limiter.allow(keys, now); allowed || retry <= 0 {
		t.Fatal("expected backoff at threshold")
	}
	limiter.success(keys)
	if allowed, _ := limiter.allow(keys, now); !allowed {
		t.Fatal("successful login did not clear limiter")
	}
}
