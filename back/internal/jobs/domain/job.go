package domain

import (
	"context"
	"encoding/json"
	"time"
)

type Job struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	Payload     json.RawMessage `json:"payload"`
	Status      string          `json:"status"`
	Priority    int16           `json:"priority"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
	RunAfter    time.Time       `json:"run_after"`
	LockedBy    string          `json:"locked_by,omitempty"`
	LastError   string          `json:"last_error,omitempty"`
	CreatedBy   *string         `json:"created_by,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	FinishedAt  *time.Time      `json:"finished_at,omitempty"`
}
type Enqueue struct {
	Kind        string    `json:"kind"`
	Payload     any       `json:"payload"`
	Priority    int16     `json:"priority"`
	MaxAttempts int       `json:"max_attempts"`
	RunAfter    time.Time `json:"run_after"`
	CreatedBy   string    `json:"-"`
}
type Queue interface {
	Enqueue(context.Context, Enqueue) (Job, error)
	Reserve(context.Context, string) (*Job, error)
	Complete(context.Context, string) error
	Fail(context.Context, string, error) error
	List(context.Context, int) ([]Job, error)
}
type Handler interface {
	Kind() string
	Handle(context.Context, Job) error
}
