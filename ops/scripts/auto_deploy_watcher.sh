#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="${REPO_DIR:-/home/bill/dev/wc-hub}"
BRANCH="${BRANCH:-main}"
POLL_INTERVAL="${POLL_INTERVAL:-30}"

echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] Auto-Deploy Watcher iniciado para $REPO_DIR (branch: $BRANCH, polling: ${POLL_INTERVAL}s)"

cd "$REPO_DIR"

while true; do
  git fetch origin "$BRANCH" >/dev/null 2>&1 || true
  LOCAL_COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "")
  REMOTE_COMMIT=$(git rev-parse "origin/$BRANCH" 2>/dev/null || echo "")

  if [ -n "$LOCAL_COMMIT" ] && [ -n "$REMOTE_COMMIT" ] && [ "$LOCAL_COMMIT" != "$REMOTE_COMMIT" ]; then
    echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] Novo commit detectado em origin/$BRANCH ($LOCAL_COMMIT -> $REMOTE_COMMIT)"
    echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] Executando git pull..."
    git pull origin "$BRANCH"

    echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] Atualizando deploy do WC-Hub..."
    if command -v kubectl >/dev/null 2>&1 && [ -n "${KUBECONFIG:-}" ]; then
      echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] Executando kubectl rollout restart no K3s..."
      kubectl rollout restart deployment/wc-hub-back deployment/wc-hub-front -n wc-hub || true
    fi

    if command -v docker >/dev/null 2>&1; then
      echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] Executando docker compose restart..."
      docker compose -f docker-compose.yml -f docker-compose.dev.yml restart back front || docker compose restart back front || true
    fi

    echo "[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] Deploy atualizado com sucesso para o commit $REMOTE_COMMIT!"
  fi

  sleep "$POLL_INTERVAL"
done
