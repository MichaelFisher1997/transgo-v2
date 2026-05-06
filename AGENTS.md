# AGENTS.md — TransGo v2

## Current Shape
- This repo is now a two-part app: Go API in `backend/` and React/Vite UI in `frontend/`. The old root `app/` templ guidance is stale; do not follow it for current work.
- Backend module is `transgo-v2/backend`; run Go commands from `backend/`, not the repo root.
- Frontend uses Bun, Vite, React, Tailwind, and TypeScript. Use Bun instead of npm/yarn/pnpm.
- Backend uses standard-library HTTP, PostgreSQL, `sqlx`, and FFmpeg/FFprobe for media encoding.

## High-Value Commands
- Backend compile check: `cd backend && go test ./... -run '^$'`
- Backend tests: `cd backend && go test ./... -v`
- Frontend build/typecheck: `cd frontend && bun run build`
- Full production build: `./build.sh` builds `frontend/dist` and writes root `./transgo` from `backend/`.
- Production container: `docker compose up --build` serves the built SPA from the Go binary on `:8080`.
- Dev workflow currently uses host tools plus Docker Postgres: `./dev.sh`; frontend HMR is `:5173`, backend API is `:8080`.

## Runtime Gotchas
- Production `Dockerfile` copies root `./transgo` and `frontend/dist`; if backend or frontend changed, run `./build.sh` before `docker compose up --build`.
- `docker-compose.yml` mounts `/mnt/BigNAS/Media/Movies:/app/media:rw` and `/dev/dri:/dev/dri`; AV1 Quick Sync depends on `/dev/dri` and FFmpeg support.
- Dev mode sets `MEDIA_DIRS=/mnt/BigNAS/Media/Movies`; production Compose sets `MEDIA_DIRS=/app/media`. Keep path validation changes compatible with both.
- Frontend dev proxies `/api` to `API_PROXY_TARGET` or `http://localhost:8080` in `frontend/vite.config.ts`.
- If Docker image pulls or package downloads fail with TLS/registry/network errors, stop and report the network issue before changing architecture or workflow.

## Backend Notes
- Entry point and routes: `backend/main.go`.
- DB setup and startup migrations: `backend/db.go`; schema source: `backend/schema.sql`.
- Encoding worker and FFmpeg progress parsing: `backend/encoder.go`.
- API handlers and preflight: `backend/handlers.go`.
- Job state is DB-backed, but active cancellation functions are in-memory; startup migration marks stale `running`/`cancelling` jobs failed.
- Encode outputs are written beside sources using `ENCODER_OUTPUT_SUFFIX` (default `.av1`), via temp files that preserve the real container extension.
- Cleanup is intended to stay manual-only; do not add automatic original deletion without explicit user approval.

## Frontend Notes
- API client and shared types are `frontend/src/api.ts` and `frontend/src/types.ts`; update both when backend JSON changes.
- Browse UI is filename-metadata driven for codec/resolution/source filters; it is not yet backed by persistent ffprobe metadata.
- Jobs/Results pages expect progress fields like `fps`, `speed`, `eta_seconds`, `processed_frames`, `total_frames`, and `current_file_percent`.

## Verification Notes
- `go test ./...` from repo root is misleading because the real backend module is under `backend/`.
- The CI workflow `.github/workflows/build-and-test.yml` still contains old `app/` templ/Tailwind steps and root `go test`; treat it as stale until updated.
- PostgreSQL is required for normal backend startup; use `docker compose up -d postgres` or `docker compose -f docker-compose.dev.yml up -d postgres`.
