#!/usr/bin/env sh
set -eu
curl --fail --silent --show-error "${WC_HUB_HEALTH_URL:-http://127.0.0.1:8088/healthz}"

