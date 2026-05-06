#!/usr/bin/env bash

set -e

echo "Building frontend with Bun..."
cd frontend
bun install
bun run build
cd ..

echo "Building backend..."
cd backend
go mod tidy
go build -o ../transgo .
cd ..

echo "Build complete! Run ./transgo to start the server."
