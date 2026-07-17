package application

import (
	"context"
	"fmt"
	"github.com/webcreations/wc-hub/back/internal/jobs/domain"
	"log/slog"
	"time"
)

type Runner struct {
	queue    domain.Queue
	handlers map[string]domain.Handler
	logger   *slog.Logger
	workerID string
	count    int
	onResult func(context.Context, domain.Job, string, error)
}

func (r *Runner) SetResultHook(hook func(context.Context, domain.Job, string, error)) {
	r.onResult = hook
}

func NewRunner(queue domain.Queue, handlers []domain.Handler, logger *slog.Logger, workerID string, count int) *Runner {
	mapped := map[string]domain.Handler{}
	for _, handler := range handlers {
		mapped[handler.Kind()] = handler
	}
	return &Runner{queue: queue, handlers: mapped, logger: logger, workerID: workerID, count: count}
}
func (r *Runner) Start(ctx context.Context) {
	for index := 0; index < r.count; index++ {
		go r.loop(ctx, fmt.Sprintf("%s-%d", r.workerID, index+1))
	}
}
func (r *Runner) loop(ctx context.Context, worker string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job, err := r.queue.Reserve(ctx, worker)
			if err != nil {
				r.logger.Error("reserve job failed", "worker", worker, "error", err)
				continue
			}
			if job == nil {
				continue
			}
			handler := r.handlers[job.Kind]
			if handler == nil {
				err = fmt.Errorf("no handler registered for %s", job.Kind)
			} else {
				jobCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
				err = handler.Handle(jobCtx, *job)
				cancel()
			}
			if err != nil {
				r.logger.Error("job failed", "job_id", job.ID, "kind", job.Kind, "error", err)
				_ = r.queue.Fail(ctx, job.ID, err)
				if r.onResult != nil {
					r.onResult(ctx, *job, "failed", err)
				}
			} else {
				_ = r.queue.Complete(ctx, job.ID)
				if r.onResult != nil {
					r.onResult(ctx, *job, "succeeded", nil)
				}
			}
		}
	}
}
