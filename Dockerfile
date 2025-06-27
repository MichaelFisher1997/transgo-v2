# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app

# Copy go mod files first for better caching
COPY app/go.mod app/go.sum ./


# Download dependencies
RUN go mod download

# Copy source files
COPY app/ ./


# Install templ and generate templ files
RUN go install github.com/a-h/templ/cmd/templ@latest
RUN templ generate

# Build a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o transogo .

# Runtime stage - use alpine for minimal but complete environment
FROM alpine:3.22
WORKDIR /app

# Create necessary directories
RUN mkdir -p /app/media /app/static /app/views

# Copy the binary and embedded files from builder
COPY --from=builder /app/transogo .
COPY --from=builder /app/static /app/static
COPY --from=builder /app/views /app/views

# Set environment variables
ENV PORT=8080

# Expose the application port
EXPOSE $PORT

# Run the application
CMD ["./transogo"]
