package domain

import (
	"context"
	"time"
)

type Task struct {
	Name     string
	Schedule string
	Timeout  time.Duration
	Handler  func(context.Context) error
}
type Scheduler interface {
	Register(Task) error
	Start(context.Context) error
}
