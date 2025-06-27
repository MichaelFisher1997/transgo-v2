#!/usr/bin/env bash

set -e

echo "Generating templ files..."
cd app && templ generate -path ./views && cd ..

echo "Building Tailwind CSS..."
tailwindcss -i app/static/css/styles.css -o app/static/css/output.css --minify

echo "Local build complete!"
