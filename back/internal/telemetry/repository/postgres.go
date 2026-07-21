package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webcreations/wc-hub/back/internal/telemetry/domain"
	"time"
)

type Postgres struct{ db *pgxpool.Pool }

func NewPostgres(db *pgxpool.Pool) *Postgres { return &Postgres{db: db} }
func (r *Postgres) ProvisionToken(ctx context.Context, hostID, actorID string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256([]byte(token))
	_, err := r.db.Exec(ctx, `INSERT INTO agent_tokens(host_id,token_hash,created_by) VALUES($1,$2,NULLIF($3,'')::uuid)`, hostID, digest[:], actorID)
	return token, err
}
func (r *Postgres) Authenticate(ctx context.Context, token string) (string, error) {
	digest := sha256.Sum256([]byte(token))
	var hostID string
	err := r.db.QueryRow(ctx, `UPDATE agent_tokens SET last_used_at=now() WHERE token_hash=$1 AND revoked_at IS NULL AND(expires_at IS NULL OR expires_at>now()) RETURNING host_id::text`, digest[:]).Scan(&hostID)
	return hostID, err
}
func (r *Postgres) Ingest(ctx context.Context, hostID string, batch domain.Batch) error {
	if batch.CapturedAt.IsZero() {
		batch.CapturedAt = time.Now().UTC()
	}
	if len(batch.Samples) > 5000 {
		return fmt.Errorf("too many samples")
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, sample := range batch.Samples {
		if !allowed(sample.Name) {
			continue
		}
		_, err = tx.Exec(ctx, `INSERT INTO metrics_snapshots(captured_at,resource_type,resource_id,metric,value,unit,labels) VALUES($1,'host',$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, batch.CapturedAt, hostID, sample.Name, sample.Value, sample.Unit, sample.Labels)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE hosts SET last_seen_at=$2,status='online',agent_status='online' WHERE id=$1`, hostID, batch.CapturedAt)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *Postgres) Latest(ctx context.Context) ([]domain.HostMetric, error) {
	rows, err := r.db.Query(ctx, `SELECT DISTINCT ON(m.resource_id,m.metric) m.resource_id::text,h.name,m.metric,m.value,COALESCE(m.unit,''),m.captured_at FROM metrics_snapshots m JOIN hosts h ON h.id=m.resource_id WHERE m.resource_type='host' ORDER BY m.resource_id,m.metric,m.captured_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.HostMetric{}
	for rows.Next() {
		var item domain.HostMetric
		if err = rows.Scan(&item.HostID, &item.HostName, &item.Metric, &item.Value, &item.Unit, &item.CapturedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (r *Postgres) Maintenance(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `UPDATE hosts SET agent_status='offline' WHERE agent_status='online' AND last_seen_at<now()-interval '2 minutes'`)
	return err
}
func allowed(name string) bool {
	switch name {
	case "node_load1", "node_load5", "node_load15", "node_memory_MemTotal_bytes", "node_memory_MemAvailable_bytes", "node_filesystem_size_bytes", "node_filesystem_avail_bytes", "node_network_receive_bytes_total", "node_network_transmit_bytes_total", "node_cpu_seconds_total", "node_rapl_package_joules_total", "node_hwmon_power_average_watt", "node_hwmon_power_input_watt", "DCGM_FI_DEV_GPU_UTIL", "DCGM_FI_DEV_FB_USED", "DCGM_FI_DEV_FB_FREE", "DCGM_FI_DEV_GPU_TEMP":
		return true
	}
	return false
}
