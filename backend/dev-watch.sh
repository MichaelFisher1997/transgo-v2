#!/bin/sh
set -eu

last=""
child=""
child_group=""

cleanup() {
  if [ -n "$child" ]; then
    if [ -n "$child_group" ]; then
      kill -TERM "-$child_group" 2>/dev/null || true
      for _ in 1 2 3 4 5; do
        kill -0 "-$child_group" 2>/dev/null || break
        sleep 1
      done
      kill -KILL "-$child_group" 2>/dev/null || true
    else
      kill -TERM "$child" 2>/dev/null || true
      for _ in 1 2 3 4 5; do
        kill -0 "$child" 2>/dev/null || break
        sleep 1
      done
      kill -KILL "$child" 2>/dev/null || true
    fi
    wait "$child" 2>/dev/null || true
  fi
  child=""
  child_group=""
}

trap cleanup EXIT INT TERM

while true; do
  next=$(find . -type f \( -name "*.go" -o -name "*.sql" \) -not -path "./tmp/*" -print0 | sort -z | xargs -0 sha256sum | sha256sum)
  if [ "$next" != "$last" ]; then
    last="$next"
    cleanup
    echo "[backend] source changed, restarting"
    if go build -o /tmp/transgo-dev .; then
      if command -v setsid >/dev/null 2>&1; then
        setsid /tmp/transgo-dev &
        child_group="$!"
      else
        /tmp/transgo-dev &
        child_group=""
      fi
      child="$!"
    else
      echo "[backend] build failed"
      child=""
    fi
  fi
  sleep 1
done
