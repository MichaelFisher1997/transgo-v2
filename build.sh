#!/usr/bin/env bash

set -e

echo "Generating templ files..."
cd app && templ generate -path ./views && cd ..

echo "Building Tailwind CSS..."
tailwindcss -i /home/micqdf/Documents/findser/transogov2/app/static/css/styles.css -o /home/micqdf/Documents/findser/transogov2/app/static/css/output.css --minify

echo "Building templ..."
templ generate /home/micqdf/Documents/findser/transogov2/app

echo "Build Go"
go build -o transogov2 /home/micqdf/Documents/findser/transogov2/app

echo "Local build complete!"

echo "Running Go tests"

cd app && go test -v
