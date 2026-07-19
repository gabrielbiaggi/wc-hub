package app

import (
	"sync"
	"time"
)

type loginAttempt struct {
	failures    int
	windowStart time.Time
	blockedTill time.Time
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
	window   time.Duration
	limit    int
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{attempts: map[string]loginAttempt{}, window: 15 * time.Minute, limit: 5}
}

func (l *loginLimiter) allow(keys []string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var retry time.Duration
	for _, key := range keys {
		attempt := l.attempts[key]
		if now.Sub(attempt.windowStart) > l.window {
			delete(l.attempts, key)
			continue
		}
		if remaining := attempt.blockedTill.Sub(now); remaining > retry {
			retry = remaining
		}
	}
	return retry <= 0, retry
}

func (l *loginLimiter) failure(keys []string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, key := range keys {
		attempt := l.attempts[key]
		if attempt.windowStart.IsZero() || now.Sub(attempt.windowStart) > l.window {
			attempt = loginAttempt{windowStart: now}
		}
		attempt.failures++
		if attempt.failures >= l.limit {
			exponent := attempt.failures - l.limit
			if exponent > 9 {
				exponent = 9
			}
			backoff := time.Second * time.Duration(1<<exponent)
			if backoff > 15*time.Minute {
				backoff = 15 * time.Minute
			}
			attempt.blockedTill = now.Add(backoff)
		}
		l.attempts[key] = attempt
	}
	if len(l.attempts) > 10_000 {
		for key, attempt := range l.attempts {
			if now.Sub(attempt.windowStart) > l.window {
				delete(l.attempts, key)
			}
		}
	}
}

func (l *loginLimiter) success(keys []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, key := range keys {
		delete(l.attempts, key)
	}
}
