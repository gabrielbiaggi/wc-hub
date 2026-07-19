package monitorapp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	adapter "github.com/webcreations/wc-hub/back/internal/adapters/monitoring"
	"net/http"
	"strings"
)

type Store struct{ db *pgxpool.Pool }

func NewStore(db *pgxpool.Pool) *Store { return &Store{db} }
func (s *Store) Targets(ctx context.Context) ([]adapter.Target, error) {
	rows, err := s.db.Query(ctx, `SELECT id,name,target,kind,interval_seconds,enabled,last_status,COALESCE(last_latency_ms,0),COALESCE(last_error,''),last_checked_at FROM monitor_targets ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []adapter.Target{}
	for rows.Next() {
		var item adapter.Target
		if err = rows.Scan(&item.ID, &item.Name, &item.Target, &item.Kind, &item.IntervalSeconds, &item.Enabled, &item.LastStatus, &item.LastLatencyMS, &item.LastError, &item.LastCheckedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) Result(ctx context.Context, id, status string, latency int, reason string) error {
	_, err := s.db.Exec(ctx, `UPDATE monitor_targets SET last_status=$2,last_latency_ms=$3,last_error=$4,last_checked_at=now(),updated_at=now() WHERE id=$1`, id, status, latency, truncate(reason, 500))
	return err
}
func (s *Store) Webhook(ctx context.Context) (string, error) {
	var value string
	err := s.db.QueryRow(ctx, `SELECT webhook_url FROM monitor_settings WHERE singleton=true`).Scan(&value)
	return value, err
}
func (s *Store) SetWebhook(ctx context.Context, value string) error {
	value = strings.TrimSpace(value)
	if value != "" && !strings.HasPrefix(value, "https://") {
		return fmt.Errorf("webhook must use HTTPS")
	}
	_, err := s.db.Exec(ctx, `UPDATE monitor_settings SET webhook_url=$1,updated_at=now() WHERE singleton=true`, value)
	return err
}
func (s *Store) Create(ctx context.Context, input adapter.Target) (adapter.Target, error) {
	normalize(&input)
	if err := valid(input); err != nil {
		return adapter.Target{}, err
	}
	input.ID = randomID()
	_, err := s.db.Exec(ctx, `INSERT INTO monitor_targets(id,name,target,kind,interval_seconds,enabled)VALUES($1,$2,$3,$4,$5,$6)`, input.ID, input.Name, input.Target, input.Kind, input.IntervalSeconds, input.Enabled)
	return input, err
}
func (s *Store) Update(ctx context.Context, id string, input adapter.Target) (adapter.Target, error) {
	normalize(&input)
	if err := valid(input); err != nil {
		return adapter.Target{}, err
	}
	_, err := s.db.Exec(ctx, `UPDATE monitor_targets SET name=$2,target=$3,kind=$4,interval_seconds=$5,enabled=$6,updated_at=now() WHERE id=$1`, id, input.Name, input.Target, input.Kind, input.IntervalSeconds, input.Enabled)
	input.ID = id
	return input, err
}
func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM monitor_targets WHERE id=$1`, id)
	return err
}
func (s *Store) Configured(ctx context.Context) (bool, error) {
	value, err := s.Webhook(ctx)
	return value != "", err
}
func valid(input adapter.Target) error {
	if input.Name == "" || len(input.Name) > 120 || len(input.Target) > 2048 || (input.Kind != "http" && input.Kind != "tcp") || input.IntervalSeconds < 15 || input.IntervalSeconds > 3600 {
		return fmt.Errorf("monitor target is invalid")
	}
	return nil
}
func normalize(input *adapter.Target) {
	input.Name = strings.TrimSpace(input.Name)
	input.Target = strings.TrimSpace(input.Target)
	input.Kind = strings.ToLower(strings.TrimSpace(input.Kind))
}
func randomID() string { b := make([]byte, 12); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func truncate(v string, n int) string {
	if len(v) > n {
		return v[:n]
	}
	return v
}

type AuthMiddleware func(string, http.HandlerFunc) http.HandlerFunc
type Handler struct{ store *Store }

func MountRoutes(mux *http.ServeMux, auth AuthMiddleware, store *Store) {
	h := &Handler{store}
	mux.HandleFunc("GET /api/v1/monitor/targets", auth("monitor.read", h.list))
	mux.HandleFunc("POST /api/v1/monitor/targets", auth("monitor.manage", h.create))
	mux.HandleFunc("PATCH /api/v1/monitor/targets/{id}", auth("monitor.manage", h.update))
	mux.HandleFunc("DELETE /api/v1/monitor/targets/{id}", auth("monitor.manage", h.remove))
	mux.HandleFunc("GET /api/v1/monitor/webhook", auth("monitor.read", h.webhook))
	mux.HandleFunc("PUT /api/v1/monitor/webhook", auth("monitor.manage", h.setWebhook))
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.Targets(r.Context())
	if err != nil {
		errorJSON(w, 500, "monitor_failed")
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"items": items})
}
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var input adapter.Target
	if !decode(w, r, &input) {
		return
	}
	item, err := h.store.Create(r.Context(), input)
	if err != nil {
		errorJSON(w, 400, "target_invalid")
		return
	}
	json.NewEncoder(w).Encode(item)
}
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var input adapter.Target
	if !decode(w, r, &input) {
		return
	}
	item, err := h.store.Update(r.Context(), r.PathValue("id"), input)
	if err != nil {
		errorJSON(w, 400, "target_invalid")
		return
	}
	json.NewEncoder(w).Encode(item)
}
func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Delete(r.Context(), r.PathValue("id")); err != nil {
		errorJSON(w, 500, "target_delete_failed")
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) webhook(w http.ResponseWriter, r *http.Request) {
	configured, err := h.store.Configured(r.Context())
	if err != nil {
		errorJSON(w, 500, "webhook_failed")
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"configured": configured})
}
func (h *Handler) setWebhook(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URL string `json:"url"`
	}
	if !decode(w, r, &input) {
		return
	}
	if err := h.store.SetWebhook(r.Context(), input.URL); err != nil {
		errorJSON(w, 400, "webhook_invalid")
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"configured": strings.TrimSpace(input.URL) != ""})
}
func decode(w http.ResponseWriter, r *http.Request, d any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(d) != nil {
		errorJSON(w, 400, "invalid_request")
		return false
	}
	return true
}
func errorJSON(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code}})
}
