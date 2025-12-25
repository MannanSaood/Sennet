# Sennet Control Plane - Dockerfile
# Multi-stage build for minimal production image

# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go.mod files first for better layer caching
COPY backend/go.mod backend/go.sum ./backend/
COPY gen/go/go.mod gen/go/go.sum ./gen/go/

# Copy generated protobuf code
COPY gen/go/ ./gen/go/

# Copy backend source
COPY backend/ ./backend/

WORKDIR /app/backend

# Download dependencies
RUN go mod download

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /sennet-server .

# Production stage
FROM alpine:3.19

# Install ca-certificates for HTTPS and tzdata for timezone
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' sennet

WORKDIR /app

# Copy binary from builder
COPY --from=builder /sennet-server .

# Create data directory for SQLite
RUN mkdir -p /data && chown sennet:sennet /data

# Switch to non-root user
USER sennet

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Environment variables
ENV PORT=8080
ENV DB_PATH=/data/sennet.db
ENV LATEST_VERSION=1.0.0

# Run the server
ENTRYPOINT ["./sennet-server"]
CMD ["-port", "8080", "-db", "/data/sennet.db"]
