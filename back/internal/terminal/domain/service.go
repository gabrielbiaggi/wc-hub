package domain

import "context"

// Service is intentionally transport-agnostic. Implementations must evaluate
// policy before opening a PTY and must never expose a raw local shell.
type Service interface {
	Open(ctx context.Context, actorID, targetID string, cols, rows uint16) (Session, error)
	Resize(ctx context.Context, sessionID string, cols, rows uint16) error
	Close(ctx context.Context, sessionID string) error
}
type Session struct {
	ID       string `json:"id"`
	TargetID string `json:"target_id"`
	Protocol string `json:"protocol"`
}
