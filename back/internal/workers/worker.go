package workers

import (
	"context"

	jobs "github.com/webcreations/wc-hub/back/internal/jobs/domain"
)

// Handler implementations run outside request lifecycles and receive only the
// scoped credentials required for their job kind.
type Handler interface {
	Kind() string
	Handle(context.Context, jobs.Job) error
}
