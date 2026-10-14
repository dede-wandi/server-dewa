# ==========================================
# Multi-stage Dockerfile for DewaCell API
# Ultra-lightweight Single-Container Microservices
# ==========================================

# Step 1: Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates tzdata

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy application source code
COPY . .

# Build statically linked binary with optimizations (-ldflags="-w -s")
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/bin/server ./cmd/api

# Step 2: Minimal runtime stage
FROM alpine:3.19

# Security: Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /home/appuser

# Copy timezone data & certificates
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/bin/server /home/appuser/server

# Set permissions
RUN chown -R appuser:appgroup /home/appuser

USER appuser

# Environment variables
ENV PORT=8080 \
    APP_ENV=production \
    TZ=Asia/Jakarta

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:${PORT}/health || exit 1

ENTRYPOINT ["/home/appuser/server"]
