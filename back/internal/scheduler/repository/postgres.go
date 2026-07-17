package repository

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	jobs "github.com/webcreations/wc-hub/back/internal/jobs/domain"
	"time"
)

type Postgres struct {
	db    *pgxpool.Pool
	queue jobs.Queue
}

func NewPostgres(db *pgxpool.Pool, queue jobs.Queue) *Postgres {
	return &Postgres{db: db, queue: queue}
}
func (r *Postgres) RunDue(ctx context.Context) error {
	rows, err := r.db.Query(ctx, `UPDATE schedules SET last_run_at=now(),next_run_at=now()+make_interval(secs=>interval_seconds) WHERE id IN(SELECT id FROM schedules WHERE enabled AND next_run_at<=now() FOR UPDATE SKIP LOCKED) RETURNING job_kind,payload`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var payload json.RawMessage
		if err = rows.Scan(&kind, &payload); err != nil {
			return err
		}
		var value any
		if err = json.Unmarshal(payload, &value); err != nil {
			return err
		}
		if _, err = r.queue.Enqueue(ctx, jobs.Enqueue{Kind: kind, Payload: value, Priority: 100, MaxAttempts: 5, RunAfter: time.Now().UTC()}); err != nil {
			return err
		}
	}
	return rows.Err()
}
func (r *Postgres) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = r.RunDue(ctx)
			}
		}
	}()
}
