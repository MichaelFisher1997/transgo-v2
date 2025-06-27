# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /build

# Copy go mod files from root for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Force cache bust for app/ directory copy
COPY ./.git/HEAD ./.git/HEAD.tmp

# Install templ
RUN go install github.com/a-h/templ/cmd/templ@latest

# Copy source files
COPY app/ ./app/

# Generate templ files
RUN cd app && templ generate -path ./views && cd ..

# Build a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o transogo transogov2/app

# Runtime stage - use alpine for minimal but complete environment
FROM alpine:3.22
WORKDIR /app

# Create necessary directories
RUN mkdir -p /app/media /app/static /app/views

# Copy the binary and embedded files from builder
COPY --from=builder /build/transogo .

# Create static directory if it doesn't exist in builder
RUN mkdir -p /build/app/static

# Copy files (will skip if source doesn't exist)
COPY --from=builder /build/app/static /app/static || true
COPY --from=builder /build/app/views /app/views

# Set environment variables
ENV PORT=8080

# Expose the application port
EXPOSE $PORT

# Run the application
CMD ["./transogo"]
