package domain

import (
	"context"
	"time"
)

type Job struct {
	ID       string
	Kind     string
	Payload  []byte
	Attempts int
	RunAfter time.Time
}
type Queue interface {
	Enqueue(context.Context, Job) error
	Reserve(context.Context) (*Job, error)
	Complete(context.Context, string) error
	Fail(context.Context, string, error) error
}
