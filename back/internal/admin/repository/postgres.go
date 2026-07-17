package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	admindomain "github.com/webcreations/wc-hub/back/internal/admin/domain"
)

var (
	ErrNotFound      = errors.New("admin resource not found")
	ErrProtectedRole = errors.New("built-in administrator role cannot be removed")
	ErrRoleInUse     = errors.New("role is assigned to users")
)

type Postgres struct{ db *pgxpool.Pool }

func NewPostgres(db *pgxpool.Pool) *Postgres { return &Postgres{db: db} }

func (r *Postgres) ListUsers(ctx context.Context) ([]admindomain.User, error) {
	rows, err := r.db.Query(ctx, `SELECT u.id::text,u.email,u.display_name,u.totp_enabled,u.disabled_at,u.last_login_at,u.created_at,COALESCE(array_agg(ro.slug ORDER BY ro.slug) FILTER (WHERE ro.id IS NOT NULL),'{}') FROM users u LEFT JOIN user_roles ur ON ur.user_id=u.id LEFT JOIN roles ro ON ro.id=ur.role_id GROUP BY u.id ORDER BY u.display_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []admindomain.User{}
	for rows.Next() {
		var item admindomain.User
		if err = rows.Scan(&item.ID, &item.Email, &item.DisplayName, &item.TOTPEnabled, &item.DisabledAt, &item.LastLoginAt, &item.CreatedAt, &item.Roles); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Postgres) CreateUser(ctx context.Context, email, displayName, password string, roleIDs []string) (admindomain.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return admindomain.User{}, err
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return admindomain.User{}, err
	}
	defer tx.Rollback(ctx)
	var item admindomain.User
	err = tx.QueryRow(ctx, `INSERT INTO users(email,display_name,password_hash) VALUES(lower($1),$2,$3) RETURNING id::text,email,display_name,totp_enabled,disabled_at,last_login_at,created_at`, strings.TrimSpace(email), strings.TrimSpace(displayName), string(hash)).Scan(&item.ID, &item.Email, &item.DisplayName, &item.TOTPEnabled, &item.DisabledAt, &item.LastLoginAt, &item.CreatedAt)
	if err != nil {
		return item, err
	}
	if err = replaceUserRoles(ctx, tx, item.ID, roleIDs); err != nil {
		return item, err
	}
	item.Roles, err = roleSlugs(ctx, tx, item.ID)
	if err != nil {
		return item, err
	}
	return item, tx.Commit(ctx)
}

func (r *Postgres) UpdateUser(ctx context.Context, id, email, displayName string, disabled bool, roleIDs []string) (admindomain.User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return admindomain.User{}, err
	}
	defer tx.Rollback(ctx)
	var item admindomain.User
	err = tx.QueryRow(ctx, `UPDATE users SET email=lower($2),display_name=$3,disabled_at=CASE WHEN $4 THEN COALESCE(disabled_at,now()) ELSE NULL END,updated_at=now() WHERE id=$1 RETURNING id::text,email,display_name,totp_enabled,disabled_at,last_login_at,created_at`, id, strings.TrimSpace(email), strings.TrimSpace(displayName), disabled).Scan(&item.ID, &item.Email, &item.DisplayName, &item.TOTPEnabled, &item.DisabledAt, &item.LastLoginAt, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	if err != nil {
		return item, err
	}
	if err = replaceUserRoles(ctx, tx, id, roleIDs); err != nil {
		return item, err
	}
	if disabled {
		if _, err = tx.Exec(ctx, `DELETE FROM auth_sessions WHERE user_id=$1`, id); err != nil {
			return item, err
		}
	}
	item.Roles, err = roleSlugs(ctx, tx, id)
	if err != nil {
		return item, err
	}
	return item, tx.Commit(ctx)
}

func (r *Postgres) DisableUser(ctx context.Context, id string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE users SET disabled_at=COALESCE(disabled_at,now()),updated_at=now() WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err = tx.Exec(ctx, `DELETE FROM auth_sessions WHERE user_id=$1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func replaceUserRoles(ctx context.Context, tx pgx.Tx, userID string, roleIDs []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id=$1`, userID); err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO user_roles(user_id,role_id) VALUES($1,$2)`, userID, roleID); err != nil {
			return err
		}
	}
	return nil
}
func roleSlugs(ctx context.Context, tx pgx.Tx, userID string) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT r.slug FROM user_roles ur JOIN roles r ON r.id=ur.role_id WHERE ur.user_id=$1 ORDER BY r.slug`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []string{}
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r *Postgres) ListRoles(ctx context.Context) ([]admindomain.Role, error) {
	rows, err := r.db.Query(ctx, `SELECT r.id::text,r.slug,r.name,COALESCE(r.description,''),COALESCE(array_agg(p.slug ORDER BY p.slug) FILTER(WHERE p.id IS NOT NULL),'{}'),COUNT(DISTINCT ur.user_id)::int FROM roles r LEFT JOIN role_permissions rp ON rp.role_id=r.id LEFT JOIN permissions p ON p.id=rp.permission_id LEFT JOIN user_roles ur ON ur.role_id=r.id GROUP BY r.id ORDER BY r.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []admindomain.Role{}
	for rows.Next() {
		var item admindomain.Role
		if err = rows.Scan(&item.ID, &item.Slug, &item.Name, &item.Description, &item.Permissions, &item.UserCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (r *Postgres) CreateRole(ctx context.Context, slug, name, description string) (admindomain.Role, error) {
	var item admindomain.Role
	err := r.db.QueryRow(ctx, `INSERT INTO roles(slug,name,description) VALUES(lower($1),$2,$3) RETURNING id::text,slug,name,COALESCE(description,'')`, strings.TrimSpace(slug), strings.TrimSpace(name), strings.TrimSpace(description)).Scan(&item.ID, &item.Slug, &item.Name, &item.Description)
	item.Permissions = []string{}
	return item, err
}
func (r *Postgres) UpdateRole(ctx context.Context, id, name, description string, permissionIDs []string) (admindomain.Role, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return admindomain.Role{}, err
	}
	defer tx.Rollback(ctx)
	var item admindomain.Role
	err = tx.QueryRow(ctx, `UPDATE roles SET name=$2,description=$3 WHERE id=$1 RETURNING id::text,slug,name,COALESCE(description,'')`, id, strings.TrimSpace(name), strings.TrimSpace(description)).Scan(&item.ID, &item.Slug, &item.Name, &item.Description)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	if err != nil {
		return item, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, id); err != nil {
		return item, err
	}
	for _, permissionID := range permissionIDs {
		if _, err = tx.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2)`, id, permissionID); err != nil {
			return item, err
		}
	}
	rows, err := tx.Query(ctx, `SELECT p.slug FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE rp.role_id=$1 ORDER BY p.slug`, id)
	if err != nil {
		return item, err
	}
	for rows.Next() {
		var slug string
		if err = rows.Scan(&slug); err != nil {
			rows.Close()
			return item, err
		}
		item.Permissions = append(item.Permissions, slug)
	}
	rows.Close()
	return item, tx.Commit(ctx)
}
func (r *Postgres) DeleteRole(ctx context.Context, id string) error {
	var slug string
	var users int
	err := r.db.QueryRow(ctx, `SELECT r.slug,COUNT(ur.user_id)::int FROM roles r LEFT JOIN user_roles ur ON ur.role_id=r.id WHERE r.id=$1 GROUP BY r.id`, id).Scan(&slug, &users)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if slug == "god-admin" {
		return ErrProtectedRole
	}
	if users > 0 {
		return ErrRoleInUse
	}
	_, err = r.db.Exec(ctx, `DELETE FROM roles WHERE id=$1`, id)
	return err
}
func (r *Postgres) ListPermissions(ctx context.Context) ([]admindomain.Permission, error) {
	rows, err := r.db.Query(ctx, `SELECT id::text,slug,description,risk::text FROM permissions ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []admindomain.Permission{}
	for rows.Next() {
		var item admindomain.Permission
		if err = rows.Scan(&item.ID, &item.Slug, &item.Description, &item.Risk); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (r *Postgres) ListAlerts(ctx context.Context) ([]admindomain.Alert, error) {
	rows, err := r.db.Query(ctx, `SELECT id::text,resource_type,resource_id::text,severity,title,COALESCE(description,''),status,acknowledged_by::text,acknowledged_at,resolved_at,created_at FROM alerts ORDER BY CASE status WHEN 'open' THEN 0 WHEN 'acknowledged' THEN 1 ELSE 2 END,created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []admindomain.Alert{}
	for rows.Next() {
		var item admindomain.Alert
		if err = rows.Scan(&item.ID, &item.ResourceType, &item.ResourceID, &item.Severity, &item.Title, &item.Description, &item.Status, &item.AcknowledgedBy, &item.AcknowledgedAt, &item.ResolvedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (r *Postgres) UpdateAlert(ctx context.Context, id, status, actorID string) (admindomain.Alert, error) {
	var item admindomain.Alert
	err := r.db.QueryRow(ctx, `UPDATE alerts SET status=$2,acknowledged_by=CASE WHEN $2='acknowledged' THEN $3::uuid ELSE acknowledged_by END,acknowledged_at=CASE WHEN $2='acknowledged' THEN now() ELSE acknowledged_at END,resolved_at=CASE WHEN $2='resolved' THEN now() ELSE NULL END WHERE id=$1 RETURNING id::text,resource_type,resource_id::text,severity,title,COALESCE(description,''),status,acknowledged_by::text,acknowledged_at,resolved_at,created_at`, id, status, actorID).Scan(&item.ID, &item.ResourceType, &item.ResourceID, &item.Severity, &item.Title, &item.Description, &item.Status, &item.AcknowledgedBy, &item.AcknowledgedAt, &item.ResolvedAt, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}
