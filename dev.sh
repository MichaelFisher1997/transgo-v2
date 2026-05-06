#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"

export PORT="${PORT:-8080}"
export DB_HOST="${DB_HOST:-localhost}"
export DB_PORT="${DB_PORT:-5432}"
export DB_USER="${DB_USER:-postgres}"
export DB_PASSWORD="${DB_PASSWORD:-postgres}"
export DB_NAME="${DB_NAME:-media_library}"
export MEDIA_DIRS="${MEDIA_DIRS:-/mnt/BigNAS/Media/Movies}"
export ENCODER_CONCURRENCY="${ENCODER_CONCURRENCY:-2}"
export ENCODER_OUTPUT_SUFFIX="${ENCODER_OUTPUT_SUFFIX:-.av1}"
export API_PROXY_TARGET="${API_PROXY_TARGET:-http://localhost:8080}"

backend_pid=""
frontend_pid=""

cleanup() {
  if [[ -n "$backend_pid" ]] && kill -0 "$backend_pid" 2>/dev/null; then
    kill "$backend_pid" 2>/dev/null || true
  fi
  if [[ -n "$frontend_pid" ]] && kill -0 "$frontend_pid" 2>/dev/null; then
    kill "$frontend_pid" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

hash_backend() {
  (cd "$BACKEND_DIR" && find . -type f \( -name '*.go' -o -name '*.sql' \) -not -path './tmp/*' -print0 | sort -z | xargs -0 sha256sum) | sha256sum
}

start_backend() {
  if [[ -n "$backend_pid" ]] && kill -0 "$backend_pid" 2>/dev/null; then
    kill "$backend_pid" 2>/dev/null || true
    wait "$backend_pid" 2>/dev/null || true
  fi
  echo "[dev] starting backend on :$PORT"
  (cd "$BACKEND_DIR" && go run .) &
  backend_pid="$!"
}

watch_backend() {
  local last_hash
  last_hash="$(hash_backend)"
  start_backend
  while true; do
    sleep 1
    local next_hash
    next_hash="$(hash_backend)"
    if [[ "$next_hash" != "$last_hash" ]]; then
      last_hash="$next_hash"
      echo "[dev] backend source changed, restarting"
      start_backend
    fi
  done
}

echo "[dev] starting postgres"
docker compose up -d postgres

if [[ ! -d "$FRONTEND_DIR/node_modules" ]]; then
  echo "[dev] installing frontend dependencies"
  (cd "$FRONTEND_DIR" && bun install)
fi

watch_backend &

echo "[dev] starting frontend on :5173"
(cd "$FRONTEND_DIR" && bun run dev --host 0.0.0.0) &
frontend_pid="$!"

wait "$frontend_pid"
