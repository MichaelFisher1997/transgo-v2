# Build stage
FROM golang:1.24 AS builder
WORKDIR /build

# Copy go mod files from root for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Force cache bust for app/ directory copy
COPY ./.git/HEAD ./.git/HEAD.tmp

# Install dependencies
RUN apt-get update && \
    apt-get install -y curl && \
    echo "Installed curl version:" && curl --version

# Install templ with retries and fallback
RUN for i in 1 2 3; do \
        go install github.com/a-h/templ/cmd/templ@latest && break || \
        (echo "Attempt $i failed, retrying..." && sleep 5); \
    done || \
    (echo "Falling back to direct download" && \
     curl -sL https://github.com/tailwindlabs/tailwindcss/releases/latest/download/templ_linux_amd64.tar.gz | tar xz -C /usr/local/bin)

# Install tailwindcss
RUN curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64 && \
    chmod +x tailwindcss-linux-x64 && \
    mv tailwindcss-linux-x64 /usr/local/bin/tailwindcss

# Copy source files
COPY app/ ./app/

# Copy static directory to root for embedding
COPY app/static ./static

# Copy Tailwind config
COPY tailwind.config.js ./

# Generate static assets (now in ./static/css)
RUN mkdir -p static/css && \
    echo "@tailwind base; @tailwind components; @tailwind utilities;" > static/css/styles.css && \
    tailwindcss -i static/css/styles.css -o static/css/output.css

# Generate templ files
RUN cd app && templ generate -path ./views && cd ..

# Build a static binary (disable VCS stamping since we don't have full git repo in container)
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o transogo github.com/MichaelFisher1997/transgo-v2/app

# Runtime stage - use ubuntu for better compatibility
FROM ubuntu:22.04
WORKDIR /app

# Create necessary directories
RUN mkdir -p /app/media /app/views

# Copy the binary and embedded files from builder
COPY --from=builder /build/transogo .

# With go:embed, static files are part of the binary, no need to copy them explicitly
# COPY --from=builder /build/app/static/css/output.css /app/static/css/output.css
# COPY --from=builder /build/app/static/images/placeholder.png /app/static/images/placeholder.png
COPY --from=builder /build/app/views /app/views

# Set environment variables
ENV PORT=8080

# Expose the application port
EXPOSE $PORT

# Run the application
CMD ["./transogo"]
