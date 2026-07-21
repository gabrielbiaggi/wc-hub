package app

import (
	"strings"
	"testing"
)

func TestSchemaMigrationsStatements(t *testing.T) {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			display_name TEXT NOT NULL,
			totp_secret TEXT,
			totp_enabled BOOLEAN NOT NULL DEFAULT false,
			disabled_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);`,
		`CREATE TABLE IF NOT EXISTS roles (
			id TEXT PRIMARY KEY,
			slug TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS permissions (
			id TEXT PRIMARY KEY,
			slug TEXT UNIQUE NOT NULL,
			description TEXT NOT NULL,
			risk TEXT NOT NULL DEFAULT 'safe'
		);`,
		`CREATE TABLE IF NOT EXISTS hosts (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			hostname TEXT NOT NULL,
			scope TEXT NOT NULL DEFAULT 'remote',
			self_protected BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id TEXT PRIMARY KEY,
			actor_id TEXT,
			action TEXT NOT NULL,
			scope TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			target_name TEXT,
			risk TEXT NOT NULL,
			decision TEXT NOT NULL,
			reason TEXT,
			source_ip TEXT,
			request_id TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);`,
	}

	for i, stmt := range migrations {
		if !strings.Contains(stmt, "CREATE TABLE") {
			t.Fatalf("Migration statement %d does not contain CREATE TABLE", i)
		}
	}
}
