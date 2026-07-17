package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Target struct {
	TicketID, SessionID, UserID, HostID, HostName, Address, SSHUser string
	Port                                                            int
}
type Postgres struct{ db *pgxpool.Pool }

func NewPostgres(db *pgxpool.Pool) *Postgres { return &Postgres{db: db} }
func (r *Postgres) CreateTicket(ctx context.Context, userID, hostID, remoteAddress string) (string, string, error) {
	var name, scope string
	var self bool
	var facts []byte
	err := r.db.QueryRow(ctx, `SELECT name,scope::text,self_protected,facts FROM hosts WHERE id=$1`, hostID).Scan(&name, &scope, &self, &facts)
	if err != nil {
		return "", "", err
	}
	if self || scope == "local" {
		return "", "", fmt.Errorf("terminal access to the local self target is blocked")
	}
	settings := map[string]any{}
	_ = json.Unmarshal(facts, &settings)
	address, _ := settings["ssh_address"].(string)
	user, _ := settings["ssh_user"].(string)
	if address == "" {
		address, _ = settings["address"].(string)
	}
	if user == "" {
		user = "root"
	}
	if address == "" {
		return "", "", fmt.Errorf("host facts must include ssh_address")
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)
	var sessionID string
	err = tx.QueryRow(ctx, `INSERT INTO terminal_sessions(user_id,host_id,status,remote_address) VALUES($1,$2,'ticketed',NULLIF($3,'')::inet) RETURNING id::text`, userID, hostID, remoteAddress).Scan(&sessionID)
	if err != nil {
		return "", "", err
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256([]byte(token))
	_, err = tx.Exec(ctx, `INSERT INTO terminal_tickets(token_hash,session_id,expires_at) VALUES($1,$2,now()+interval '45 seconds')`, digest[:], sessionID)
	if err != nil {
		return "", "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", err
	}
	return token, sessionID, nil
}
func (r *Postgres) Consume(ctx context.Context, token string) (Target, error) {
	digest := sha256.Sum256([]byte(token))
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Target{}, err
	}
	defer tx.Rollback(ctx)
	var target Target
	var facts []byte
	err = tx.QueryRow(ctx, `UPDATE terminal_tickets t SET used_at=now() FROM terminal_sessions s JOIN hosts h ON h.id=s.host_id WHERE t.session_id=s.id AND t.token_hash=$1 AND t.used_at IS NULL AND t.expires_at>now() AND h.scope<>'local' AND NOT h.self_protected RETURNING t.id::text,s.id::text,s.user_id::text,h.id::text,h.name,h.facts`, digest[:]).Scan(&target.TicketID, &target.SessionID, &target.UserID, &target.HostID, &target.HostName, &facts)
	if err != nil {
		return Target{}, err
	}
	settings := map[string]any{}
	_ = json.Unmarshal(facts, &settings)
	target.Address, _ = settings["ssh_address"].(string)
	target.SSHUser, _ = settings["ssh_user"].(string)
	if target.SSHUser == "" {
		target.SSHUser = "root"
	}
	target.Port = 22
	if value, ok := settings["ssh_port"].(float64); ok {
		target.Port = int(value)
	}
	if target.Address == "" {
		return Target{}, fmt.Errorf("SSH address missing")
	}
	_, err = tx.Exec(ctx, `UPDATE terminal_sessions SET status='connecting',started_at=now() WHERE id=$1`, target.SessionID)
	if err != nil {
		return Target{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Target{}, err
	}
	return target, nil
}
func (r *Postgres) Close(ctx context.Context, sessionID, status string) error {
	_, err := r.db.Exec(ctx, `UPDATE terminal_sessions SET status=$2,ended_at=now() WHERE id=$1`, sessionID, status)
	return err
}
func (r *Postgres) Recent(ctx context.Context) ([]map[string]any, error) {
	rows, err := r.db.Query(ctx, `SELECT s.id::text,u.email,h.name,s.status,s.started_at,s.ended_at,s.created_at FROM terminal_sessions s JOIN users u ON u.id=s.user_id JOIN hosts h ON h.id=s.host_id ORDER BY s.created_at DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var id, email, host, status string
		var started, ended *time.Time
		var created time.Time
		if err = rows.Scan(&id, &email, &host, &status, &started, &ended, &created); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"id": id, "user_email": email, "host_name": host, "status": status, "started_at": started, "ended_at": ended, "created_at": created})
	}
	return result, rows.Err()
}
