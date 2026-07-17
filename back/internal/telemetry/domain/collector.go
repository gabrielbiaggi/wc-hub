package domain

import (
	"context"
	"time"
)

type Sample struct {
	ResourceType string
	ResourceID   string
	Metric       string
	Value        float64
	Unit         string
	CapturedAt   time.Time
}
type Collector interface {
	Name() string
	Collect(context.Context) ([]Sample, error)
}
