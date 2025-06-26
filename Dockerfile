# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY app/go.mod app/go.sum ./
RUN go mod download
COPY app/ .
# Install templ and generate templ files
RUN go install github.com/a-h/templ/cmd/templ@latest
RUN cd /app && templ generate
RUN go clean -modcache && CGO_ENABLED=0 GOOS=linux go build -o transogo .

# Runtime stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/transogo .
COPY --from=builder /app/static ./static
COPY --from=builder /app/views ./views
EXPOSE 8080
CMD ["./transogo"]
