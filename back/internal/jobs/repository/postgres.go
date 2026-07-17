package repository

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webcreations/wc-hub/back/internal/jobs/domain"
	"time"
)

type Postgres struct{ db *pgxpool.Pool }

func NewPostgres(db *pgxpool.Pool) *Postgres { return &Postgres{db: db} }
func (r *Postgres) Enqueue(ctx context.Context, input domain.Enqueue) (domain.Job, error) {
	if input.Priority == 0 {
		input.Priority = 100
	}
	if input.MaxAttempts == 0 {
		input.MaxAttempts = 5
	}
	if input.RunAfter.IsZero() {
		input.RunAfter = time.Now().UTC()
	}
	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return domain.Job{}, err
	}
	var job domain.Job
	err = r.db.QueryRow(ctx, `INSERT INTO jobs(kind,payload,priority,max_attempts,run_after,created_by) VALUES($1,$2,$3,$4,$5,NULLIF($6,'')::uuid) RETURNING id::text,kind,payload,status,priority,attempts,max_attempts,run_after,created_at`, input.Kind, payload, input.Priority, input.MaxAttempts, input.RunAfter, input.CreatedBy).Scan(&job.ID, &job.Kind, &job.Payload, &job.Status, &job.Priority, &job.Attempts, &job.MaxAttempts, &job.RunAfter, &job.CreatedAt)
	return job, err
}
func (r *Postgres) Reserve(ctx context.Context, worker string) (*domain.Job, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `UPDATE jobs SET status=CASE WHEN attempts>=max_attempts THEN 'failed' ELSE 'queued' END,locked_at=NULL,locked_by=NULL,last_error='worker lease expired',run_after=now(),updated_at=now() WHERE status='running' AND locked_at<now()-interval '10 minutes'`)
	if err != nil {
		return nil, err
	}
	var job domain.Job
	err = tx.QueryRow(ctx, `SELECT id::text,kind,payload,status,priority,attempts,max_attempts,run_after,created_by::text,created_at FROM jobs WHERE status='queued' AND run_after<=now() ORDER BY priority,run_after,created_at FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&job.ID, &job.Kind, &job.Payload, &job.Status, &job.Priority, &job.Attempts, &job.MaxAttempts, &job.RunAfter, &job.CreatedBy, &job.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE jobs SET status='running',locked_at=now(),locked_by=$2,started_at=COALESCE(started_at,now()),attempts=attempts+1,updated_at=now() WHERE id=$1`, job.ID, worker)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	job.Status = "running"
	job.LockedBy = worker
	job.Attempts++
	return &job, nil
}
func (r *Postgres) Complete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE jobs SET status='succeeded',finished_at=now(),locked_at=NULL,locked_by=NULL,updated_at=now() WHERE id=$1 AND status='running'`, id)
	return err
}
func (r *Postgres) Fail(ctx context.Context, id string, cause error) error {
	_, err := r.db.Exec(ctx, `UPDATE jobs SET status=CASE WHEN attempts>=max_attempts THEN 'failed' ELSE 'queued' END,run_after=CASE WHEN attempts>=max_attempts THEN run_after ELSE now()+make_interval(secs=>LEAST(300,power(2,attempts)::int*5)) END,finished_at=CASE WHEN attempts>=max_attempts THEN now() ELSE NULL END,locked_at=NULL,locked_by=NULL,last_error=$2,updated_at=now() WHERE id=$1`, id, cause.Error())
	return err
}
func (r *Postgres) List(ctx context.Context, limit int) ([]domain.Job, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `SELECT id::text,kind,payload,status,priority,attempts,max_attempts,run_after,COALESCE(locked_by,''),COALESCE(last_error,''),created_by::text,created_at,started_at,finished_at FROM jobs ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Job{}
	for rows.Next() {
		var item domain.Job
		if err = rows.Scan(&item.ID, &item.Kind, &item.Payload, &item.Status, &item.Priority, &item.Attempts, &item.MaxAttempts, &item.RunAfter, &item.LockedBy, &item.LastError, &item.CreatedBy, &item.CreatedAt, &item.StartedAt, &item.FinishedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
