# Stage 1: Build frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/web

# Copy frontend package files
COPY web/package*.json ./
RUN npm ci

# Copy frontend source and build
COPY web/ ./
RUN npm run build

# Stage 2: Build Go backend
FROM golang:1.25-alpine AS backend-builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend from previous stage
COPY --from=frontend-builder /app/web/dist ./internal/web/dist

# Build the binary with embedded frontend
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags "-extldflags '-static' -X bared/internal/version.Version=${VERSION} -X bared/internal/version.Commit=${COMMIT} -X bared/internal/version.BuildDate=${BUILD_DATE}" \
    -o brd ./cmd/brd

# Stage 3: Runtime
FROM alpine:latest

# Install runtime dependencies (database clients + wget for health checks)
RUN apk add --no-cache \
    mysql-client \
    postgresql-client \
    redis \
    ca-certificates \
    tzdata \
    wget

# Create non-root user
RUN addgroup -g 1000 backup && \
    adduser -D -u 1000 -G backup backup

# Create directories
RUN mkdir -p /backups /etc/bared /tmp && \
    chown -R backup:backup /backups /etc/bared /tmp

# Copy binary from builder
COPY --from=backend-builder /app/brd /usr/local/bin/brd
RUN chmod +x /usr/local/bin/brd

# Switch to non-root user
USER backup

# Set working directory
WORKDIR /etc/bared

# Expose HTTP port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

# Volumes
VOLUME ["/backups", "/etc/bared"]

# Default command
ENTRYPOINT ["/usr/local/bin/brd"]
CMD ["daemon", "--config", "/etc/bared/bared.yml"]
