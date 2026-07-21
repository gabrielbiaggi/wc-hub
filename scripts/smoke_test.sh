#!/usr/bin/env bash
set -euo pipefail

echo "========================================="
echo "   WC Hub Full Stack Smoke Test Suite    "
echo "========================================="

echo "[1/4] Checking environment prerequisites..."
command -v docker >/dev/null 2>&1 || { echo "Docker is required but not installed."; exit 1; }
docker compose config --quiet

echo "[2/4] Verifying Backend Go Code Build..."
docker run --rm -v "$PWD/back:/app" -w /app golang:1.25 /usr/local/go/bin/go build -o /tmp/wc-hub-api-test ./cmd/api
echo "Backend compiled successfully."

echo "[3/4] Running Backend Unit & E2E Test Suite..."
docker run --rm -v "$PWD/back:/app" -w /app golang:1.25 /usr/local/go/bin/go test ./...
echo "Backend test suite passed 100%."

echo "[4/4] Verifying Frontend Asset Build..."
if [ -d "front" ]; then
  docker build -f front/Dockerfile -t wc-hub-front:smoke front
  echo "Frontend production assets built successfully."
fi

echo "========================================="
echo "  SUCCESS: WC Hub Smoke Test Passed 100% "
echo "========================================="
