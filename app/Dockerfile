# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o transogo .

# Runtime stage
FROM golang:1.24-alpine
WORKDIR /app
COPY --from=builder /app/transogo .
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=10s --retries=3 --start-period=5s \
  CMD curl -f http://localhost:8080/api/health || exit 1
CMD ["./transogo"]
