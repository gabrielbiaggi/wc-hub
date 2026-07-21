#!/usr/bin/env bash
set -euo pipefail

echo "========================================="
echo "   WC Hub Full Stack Smoke Test Suite    "
echo "========================================="

echo "[1/4] Checking environment prerequisites..."
command -v docker >/dev/null 2>&1 || { echo "Docker is required but not installed."; exit 1; }

echo "[2/4] Verifying Backend Go Code Build..."
(cd back && go build -o /tmp/wc-hub-api-test ./cmd/api)
echo "Backend compiled successfully."

echo "[3/4] Running Backend Unit & E2E Test Suite..."
(cd back && go test -v ./...)
echo "Backend test suite passed 100%."

echo "[4/4] Verifying Frontend Asset Build..."
if [ -d "front" ]; then
  (cd front && npm run build)
  echo "Frontend production assets built successfully."
fi

echo "========================================="
echo "  SUCCESS: WC Hub Smoke Test Passed 100% "
echo "========================================="
