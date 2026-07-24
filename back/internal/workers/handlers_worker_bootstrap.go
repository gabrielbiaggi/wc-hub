package workers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkerNode struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	HardwareFingerprint string     `json:"hardware_fingerprint"`
	PublicKey           string     `json:"public_key"`
	IPAddress           string     `json:"ip_address"`
	Status              string     `json:"status"`
	ApprovedAt          *time.Time `json:"approved_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type BootstrapRepository interface {
	RegisterBootstrap(ctx context.Context, name, fingerprint, pubKey, ip string) (*WorkerNode, error)
	ListPending(ctx context.Context) ([]WorkerNode, error)
	ListAll(ctx context.Context) ([]WorkerNode, error)
	Approve(ctx context.Context, id string) (*WorkerNode, error)
	Reject(ctx context.Context, id string) (*WorkerNode, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "wrk-" + hex.EncodeToString(b)
}

func (r *PostgresRepository) RegisterBootstrap(ctx context.Context, name, fingerprint, pubKey, ip string) (*WorkerNode, error) {
	id := generateID()
	var node WorkerNode
	err := r.pool.QueryRow(ctx, `
		INSERT INTO worker_nodes(id, name, hardware_fingerprint, public_key, ip_address, status)
		VALUES($1, $2, $3, $4, $5, 'pending_approval')
		ON CONFLICT(hardware_fingerprint) DO UPDATE SET
			name = EXCLUDED.name,
			public_key = EXCLUDED.public_key,
			ip_address = EXCLUDED.ip_address,
			updated_at = now()
		RETURNING id, name, hardware_fingerprint, public_key, ip_address, status, approved_at, created_at, updated_at
	`, id, name, fingerprint, pubKey, ip).Scan(
		&node.ID, &node.Name, &node.HardwareFingerprint, &node.PublicKey,
		&node.IPAddress, &node.Status, &node.ApprovedAt, &node.CreatedAt, &node.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *PostgresRepository) ListPending(ctx context.Context) ([]WorkerNode, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, hardware_fingerprint, public_key, ip_address, status, approved_at, created_at, updated_at
		FROM worker_nodes
		WHERE status = 'pending_approval'
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]WorkerNode, 0)
	for rows.Next() {
		var item WorkerNode
		if err := rows.Scan(&item.ID, &item.Name, &item.HardwareFingerprint, &item.PublicKey, &item.IPAddress, &item.Status, &item.ApprovedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *PostgresRepository) ListAll(ctx context.Context) ([]WorkerNode, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, hardware_fingerprint, public_key, ip_address, status, approved_at, created_at, updated_at
		FROM worker_nodes
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]WorkerNode, 0)
	for rows.Next() {
		var item WorkerNode
		if err := rows.Scan(&item.ID, &item.Name, &item.HardwareFingerprint, &item.PublicKey, &item.IPAddress, &item.Status, &item.ApprovedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *PostgresRepository) Approve(ctx context.Context, id string) (*WorkerNode, error) {
	now := time.Now()
	var node WorkerNode
	err := r.pool.QueryRow(ctx, `
		UPDATE worker_nodes
		SET status = 'approved', approved_at = $1, updated_at = $1
		WHERE id = $2
		RETURNING id, name, hardware_fingerprint, public_key, ip_address, status, approved_at, created_at, updated_at
	`, now, id).Scan(
		&node.ID, &node.Name, &node.HardwareFingerprint, &node.PublicKey,
		&node.IPAddress, &node.Status, &node.ApprovedAt, &node.CreatedAt, &node.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("worker not found")
	}
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *PostgresRepository) Reject(ctx context.Context, id string) (*WorkerNode, error) {
	now := time.Now()
	var node WorkerNode
	err := r.pool.QueryRow(ctx, `
		UPDATE worker_nodes
		SET status = 'rejected', updated_at = $1
		WHERE id = $2
		RETURNING id, name, hardware_fingerprint, public_key, ip_address, status, approved_at, created_at, updated_at
	`, now, id).Scan(
		&node.ID, &node.Name, &node.HardwareFingerprint, &node.PublicKey,
		&node.IPAddress, &node.Status, &node.ApprovedAt, &node.CreatedAt, &node.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("worker not found")
	}
	if err != nil {
		return nil, err
	}
	return &node, nil
}

type AuthMiddleware func(permission string, next http.HandlerFunc) http.HandlerFunc

type BootstrapHandler struct {
	repo BootstrapRepository
}

func NewBootstrapHandler(repo BootstrapRepository) *BootstrapHandler {
	return &BootstrapHandler{repo: repo}
}

func (h *BootstrapHandler) MountRoutes(mux *http.ServeMux, authMiddleware AuthMiddleware) {
	// Bootstrap is unauthenticated for initial node registration request
	mux.HandleFunc("POST /api/v1/workers/bootstrap", h.Bootstrap)

	// Admin operations require permissions
	mux.HandleFunc("GET /api/v1/workers/pending", authMiddleware("worker.read", h.ListPending))
	mux.HandleFunc("GET /api/v1/workers", authMiddleware("worker.read", h.ListAll))
	mux.HandleFunc("POST /api/v1/workers/{id}/approve", authMiddleware("worker.manage", h.Approve))
	mux.HandleFunc("POST /api/v1/workers/{id}/reject", authMiddleware("worker.manage", h.Reject))
}

func (h *BootstrapHandler) Bootstrap(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name                string `json:"name"`
		HardwareFingerprint string `json:"hardware_fingerprint"`
		PublicKey           string `json:"public_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeWorkerJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_payload", "message": "Payload de bootstrap inválido."}})
		return
	}
	if strings.TrimSpace(req.HardwareFingerprint) == "" || strings.TrimSpace(req.PublicKey) == "" {
		writeWorkerJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "missing_fields", "message": "Hardware fingerprint e public_key são obrigatórios."}})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "Worker Node " + req.HardwareFingerprint[:min(8, len(req.HardwareFingerprint))]
	}

	ip := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip = strings.Split(forwarded, ",")[0]
	}

	node, err := h.repo.RegisterBootstrap(r.Context(), req.Name, req.HardwareFingerprint, req.PublicKey, ip)
	if err != nil {
		writeWorkerJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]string{"code": "bootstrap_failed", "message": "Falha ao registrar bootstrap: " + err.Error()}})
		return
	}

	writeWorkerJSON(w, http.StatusAccepted, node)
}

func (h *BootstrapHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.ListPending(r.Context())
	if err != nil {
		writeWorkerJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]string{"code": "list_failed", "message": err.Error()}})
		return
	}
	writeWorkerJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *BootstrapHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.ListAll(r.Context())
	if err != nil {
		writeWorkerJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]string{"code": "list_failed", "message": err.Error()}})
		return
	}
	writeWorkerJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *BootstrapHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	node, err := h.repo.Approve(r.Context(), id)
	if err != nil {
		writeWorkerJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "not_found", "message": err.Error()}})
		return
	}
	writeWorkerJSON(w, http.StatusOK, node)
}

func (h *BootstrapHandler) Reject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	node, err := h.repo.Reject(r.Context(), id)
	if err != nil {
		writeWorkerJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "not_found", "message": err.Error()}})
		return
	}
	writeWorkerJSON(w, http.StatusOK, node)
}

func writeWorkerJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
