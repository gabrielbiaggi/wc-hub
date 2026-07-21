#!/usr/bin/env bash
set -euo pipefail

echo "=================================================="
echo "    WC Hub Database Backup & Restore Validator    "
echo "=================================================="

BACKUP_DIR="${TMPDIR:-/tmp}/wc-hub-backups"
mkdir -p "${BACKUP_DIR}"

BACKUP_FILE="${BACKUP_DIR}/backup_$(date +%Y%m%d_%H%M%S).sql"

echo "[1/3] Generating PostgreSQL database dump..."
: "${PGDATABASE:?PGDATABASE is required; this validator never simulates backup success}"
: "${PGUSER:?PGUSER is required}"
: "${PGHOST:?PGHOST is required}"
command -v pg_dump >/dev/null 2>&1 || { echo "pg_dump is required"; exit 1; }
command -v pg_restore >/dev/null 2>&1 || { echo "pg_restore is required"; exit 1; }

BACKUP_FILE="${BACKUP_FILE%.sql}.dump"
pg_dump --format=custom --no-owner --no-privileges -U "${PGUSER}" -h "${PGHOST}" "${PGDATABASE}" > "${BACKUP_FILE}"
echo "Backup generated: ${BACKUP_FILE}"

echo "[2/3] Validating backup integrity..."
if [ ! -s "${BACKUP_FILE}" ]; then
  echo "ERROR: Backup file is empty!"
  exit 1
fi

echo "[3/3] Verifying that PostgreSQL can read the archive..."
pg_restore --list "${BACKUP_FILE}" >/dev/null
echo "Archive is readable. A destructive restore is intentionally not run against the source database."
echo "Backup & Restore verification PASSED."
