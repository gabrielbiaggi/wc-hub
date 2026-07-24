package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webcreations/wc-hub/back/internal/inventory/domain"
)

type Postgres struct{ db *pgxpool.Pool }

func NewPostgres(db *pgxpool.Pool) *Postgres { return &Postgres{db: db} }

func (r *Postgres) ListIntegrations(ctx context.Context) ([]domain.Integration, error) {
	rows, err := r.db.Query(ctx, `SELECT id::text,name,provider,status::text,config,last_checked_at,created_at FROM integrations ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.Integration{}
	for rows.Next() {
		var item domain.Integration
		var raw []byte
		if err = rows.Scan(&item.ID, &item.Name, &item.Provider, &item.Status, &raw, &item.LastCheckedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &item.Config)
		result = append(result, item)
	}
	return result, rows.Err()
}
func (r *Postgres) CreateIntegration(ctx context.Context, item domain.Integration, actorID string) (domain.Integration, error) {
	if item.Name == "" || item.Provider == "" {
		return item, fmt.Errorf("name and provider are required")
	}
	raw, _ := json.Marshal(item.Config)
	err := r.db.QueryRow(ctx, `INSERT INTO integrations(name,provider,status,config,created_by) VALUES($1,$2,'pending',$3,$4) RETURNING id::text,status::text,created_at`, item.Name, item.Provider, raw, actorID).Scan(&item.ID, &item.Status, &item.CreatedAt)
	return item, err
}
func (r *Postgres) UpsertIntegration(ctx context.Context, name, provider, status string, config map[string]any) error {
	if name == "" || provider == "" {
		return nil
	}
	if status == "" {
		status = "connected"
	}
	raw, _ := json.Marshal(config)
	_, err := r.db.Exec(ctx, `
		INSERT INTO integrations(name, provider, status, config, last_checked_at)
		VALUES($1, $2, $3::integration_status, $4, now())
		ON CONFLICT(provider, name) DO UPDATE SET status=EXCLUDED.status, config=EXCLUDED.config, last_checked_at=now(), updated_at=now()
	`, name, provider, status, raw)
	return err
}
func (r *Postgres) ListHosts(ctx context.Context) ([]domain.Host, error) {
	rows, err := r.db.Query(ctx, `SELECT id::text,integration_id::text,name,hostname,scope::text,status::text,self_protected,labels,facts,last_seen_at,created_at FROM hosts ORDER BY self_protected DESC,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.Host{}
	for rows.Next() {
		var item domain.Host
		var labels, facts []byte
		if err = rows.Scan(&item.ID, &item.IntegrationID, &item.Name, &item.Hostname, &item.Scope, &item.Status, &item.SelfProtected, &labels, &facts, &item.LastSeenAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(labels, &item.Labels)
		_ = json.Unmarshal(facts, &item.Facts)
		result = append(result, item)
	}
	return result, rows.Err()
}
func (r *Postgres) CreateHost(ctx context.Context, item domain.Host, actorID string) (domain.Host, error) {
	if item.Name == "" || item.Hostname == "" {
		return item, fmt.Errorf("name and hostname are required")
	}
	if item.Scope != "local" && item.Scope != "remote" && item.Scope != "cloud" {
		return item, fmt.Errorf("invalid scope")
	}
	if item.SelfProtected && item.Scope != "local" {
		return item, fmt.Errorf("self-protected host must be local")
	}
	labels, _ := json.Marshal(item.Labels)
	facts, _ := json.Marshal(item.Facts)
	err := r.db.QueryRow(ctx, `INSERT INTO hosts(integration_id,name,hostname,scope,status,self_protected,labels,facts,created_by) VALUES($1,$2,$3,$4,'unknown',$5,$6,$7,$8) RETURNING id::text,status::text,created_at`, item.IntegrationID, item.Name, item.Hostname, item.Scope, item.SelfProtected, labels, facts, actorID).Scan(&item.ID, &item.Status, &item.CreatedAt)
	return item, err
}
