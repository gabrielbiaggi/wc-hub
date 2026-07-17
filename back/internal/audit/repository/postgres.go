package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	security "github.com/webcreations/wc-hub/back/internal/security/domain"
)

type Entry struct {
	ID           string    `json:"id"`
	ActorEmail   string    `json:"actor_email,omitempty"`
	Action       string    `json:"action"`
	Scope        string    `json:"scope"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id,omitempty"`
	TargetName   string    `json:"target_name,omitempty"`
	Risk         string    `json:"risk"`
	Decision     string    `json:"decision"`
	Reason       string    `json:"reason,omitempty"`
	RequestID    string    `json:"request_id,omitempty"`
	OccurredAt   time.Time `json:"occurred_at"`
	EventHash    string    `json:"event_hash"`
}

type Record struct {
	ActorID, Action, ResourceType, ResourceID, TargetName, Decision, Reason, RequestID, SourceIP string
	Scope                                                                                        security.Scope
	Risk                                                                                         security.Risk
	Payload                                                                                      any
}
type Postgres struct{ db *pgxpool.Pool }

func NewPostgres(db *pgxpool.Pool) *Postgres { return &Postgres{db: db} }

func (r *Postgres) Append(ctx context.Context, record Record) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(782341998)`); err != nil {
		return err
	}
	previous := ""
	_ = tx.QueryRow(ctx, `SELECT event_hash FROM audit_logs ORDER BY occurred_at DESC,id DESC LIMIT 1`).Scan(&previous)
	at := time.Now().UTC()
	payload, err := json.Marshal(record.Payload)
	if err != nil {
		return err
	}
	material, _ := json.Marshal([]any{previous, record.ActorID, record.Action, record.Scope, record.ResourceType, record.ResourceID, record.TargetName, record.Risk, record.Decision, record.Reason, record.RequestID, at, payload})
	digest := sha256.Sum256(material)
	eventHash := hex.EncodeToString(digest[:])
	_, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,scope,resource_type,resource_id,target_name,risk,decision,reason,request_id,source_ip,payload_redacted,previous_hash,event_hash,occurred_at) VALUES(NULLIF($1,'')::uuid,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,$8,NULLIF($9,''),NULLIF($10,''),NULLIF($11,'')::inet,$12,$13,$14,$15)`, record.ActorID, record.Action, record.Scope, record.ResourceType, record.ResourceID, record.TargetName, record.Risk, record.Decision, record.Reason, record.RequestID, record.SourceIP, payload, previous, eventHash, at)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Postgres) List(ctx context.Context, limit int) ([]Entry, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `SELECT a.id::text,COALESCE(u.email,''),a.action,a.scope::text,a.resource_type,COALESCE(a.resource_id,''),COALESCE(a.target_name,''),a.risk::text,a.decision,COALESCE(a.reason,''),COALESCE(a.request_id,''),a.occurred_at,a.event_hash FROM audit_logs a LEFT JOIN users u ON u.id=a.actor_id ORDER BY a.occurred_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]Entry, 0, limit)
	for rows.Next() {
		var e Entry
		if err = rows.Scan(&e.ID, &e.ActorEmail, &e.Action, &e.Scope, &e.ResourceType, &e.ResourceID, &e.TargetName, &e.Risk, &e.Decision, &e.Reason, &e.RequestID, &e.OccurredAt, &e.EventHash); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
