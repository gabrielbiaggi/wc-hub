package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/webcreations/wc-hub/back/internal/auth/domain"
)

type Postgres struct{ db *pgxpool.Pool }

func NewPostgres(db *pgxpool.Pool) *Postgres { return &Postgres{db: db} }

func (r *Postgres) BootstrapOpen(ctx context.Context) (bool, error) {
	var open bool
	err := r.db.QueryRow(ctx, `SELECT NOT EXISTS (SELECT 1 FROM users WHERE email<>$1)`, domain.DevelopmentMasterIdentity).Scan(&open)
	return open, err
}

func (r *Postgres) EnsureDevelopmentMaster(ctx context.Context) (domain.User, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(918273646)`); err != nil {
		return domain.User{}, err
	}
	var user domain.User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email,display_name,password_hash)
		VALUES ($1,'All Might','!hourly-development-master-has-no-password-hash!')
		ON CONFLICT (email) DO UPDATE SET display_name=EXCLUDED.display_name,disabled_at=NULL,updated_at=now()
		RETURNING id::text,email,display_name,totp_enabled`, domain.DevelopmentMasterIdentity).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.TOTPEnabled)
	if err != nil {
		return domain.User{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO user_roles (user_id,role_id) SELECT $1,id FROM roles WHERE slug='god-admin' ON CONFLICT DO NOTHING`, user.ID); err != nil {
		return domain.User{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.User{}, err
	}
	return r.hydratePermissions(ctx, user)
}

func (r *Postgres) Bootstrap(ctx context.Context, credentials domain.Credentials, passwordHash string) (domain.User, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(918273645)`); err != nil {
		return domain.User{}, err
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE email<>$1`, domain.DevelopmentMasterIdentity).Scan(&count); err != nil {
		return domain.User{}, err
	}
	if count != 0 {
		return domain.User{}, domain.ErrBootstrapClosed
	}
	var user domain.User
	err = tx.QueryRow(ctx, `INSERT INTO users (email, display_name, password_hash) VALUES ($1,$2,$3) RETURNING id::text,email,display_name,totp_enabled`, credentials.Email, credentials.DisplayName, passwordHash).Scan(&user.ID, &user.Email, &user.DisplayName, &user.TOTPEnabled)
	if err != nil {
		return domain.User{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) SELECT $1,id FROM roles WHERE slug='god-admin'`, user.ID)
	if err != nil {
		return domain.User{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.User{}, err
	}
	return r.hydratePermissions(ctx, user)
}

func (r *Postgres) PasswordIdentity(ctx context.Context, email string) (domain.User, string, error) {
	var user domain.User
	var hash string
	err := r.db.QueryRow(ctx, `SELECT id::text,email,display_name,password_hash,totp_enabled FROM users WHERE email=$1 AND disabled_at IS NULL`, email).Scan(&user.ID, &user.Email, &user.DisplayName, &hash, &user.TOTPEnabled)
	if err != nil {
		return domain.User{}, "", err
	}
	user, err = r.hydratePermissions(ctx, user)
	return user, hash, err
}

func (r *Postgres) CreateSession(ctx context.Context, userID string, tokenHash, csrfHash []byte, userAgent, remoteIP string, expires time.Time) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `INSERT INTO auth_sessions (user_id,token_hash,csrf_hash,user_agent,remote_address,expires_at) VALUES ($1,$2,$3,$4,NULLIF($5,'')::inet,$6) RETURNING id::text`, userID, tokenHash, csrfHash, userAgent, remoteIP, expires).Scan(&id)
	return id, err
}

func (r *Postgres) SessionByToken(ctx context.Context, tokenHash []byte) (domain.Session, error) {
	var session domain.Session
	err := r.db.QueryRow(ctx, `SELECT s.id::text,s.csrf_hash,s.expires_at,u.id::text,u.email,u.display_name,u.totp_enabled FROM auth_sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now() AND u.disabled_at IS NULL`, tokenHash).Scan(&session.ID, &session.CSRFHash, &session.Expires, &session.User.ID, &session.User.Email, &session.User.DisplayName, &session.User.TOTPEnabled)
	if err != nil {
		return domain.Session{}, err
	}
	session.User, err = r.hydratePermissions(ctx, session.User)
	return session, err
}

func (r *Postgres) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := r.db.Exec(ctx, `DELETE FROM auth_sessions WHERE token_hash=$1`, tokenHash)
	return err
}
func (r *Postgres) UpdateCSRF(ctx context.Context, sessionID string, csrfHash []byte) error {
	_, err := r.db.Exec(ctx, `UPDATE auth_sessions SET csrf_hash=$2,last_seen_at=now() WHERE id=$1`, sessionID, csrfHash)
	return err
}
func (r *Postgres) StoreTOTPSecret(ctx context.Context, userID string, secret []byte) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET totp_secret_ciphertext=$2,totp_enabled=false,updated_at=now() WHERE id=$1`, userID, secret)
	return err
}
func (r *Postgres) TOTPSecret(ctx context.Context, userID string) ([]byte, error) {
	var secret []byte
	err := r.db.QueryRow(ctx, `SELECT totp_secret_ciphertext FROM users WHERE id=$1 AND totp_secret_ciphertext IS NOT NULL`, userID).Scan(&secret)
	return secret, err
}
func (r *Postgres) EnableTOTP(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET totp_enabled=true,updated_at=now() WHERE id=$1 AND totp_secret_ciphertext IS NOT NULL`, userID)
	return err
}

func (r *Postgres) hydratePermissions(ctx context.Context, user domain.User) (domain.User, error) {
	rows, err := r.db.Query(ctx, `SELECT DISTINCT r.slug,p.slug FROM user_roles ur JOIN roles r ON r.id=ur.role_id JOIN role_permissions rp ON rp.role_id=r.id JOIN permissions p ON p.id=rp.permission_id WHERE ur.user_id=$1 ORDER BY r.slug,p.slug`, user.ID)
	if err != nil {
		return user, err
	}
	defer rows.Close()
	roleSeen := map[string]bool{}
	permissionSeen := map[string]bool{}
	for rows.Next() {
		var role, permission string
		if err = rows.Scan(&role, &permission); err != nil {
			return user, err
		}
		if !roleSeen[role] {
			user.Roles = append(user.Roles, role)
			roleSeen[role] = true
		}
		if !permissionSeen[permission] {
			user.Permissions = append(user.Permissions, permission)
			permissionSeen[permission] = true
		}
	}
	return user, rows.Err()
}

func normalizeNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrUnauthorized
	}
	return err
}
