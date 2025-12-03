# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o brd ./cmd/brd

# Runtime stage
FROM alpine:latest

# Install runtime dependencies (database clients)
RUN apk add --no-cache \
    mysql-client \
    postgresql-client \
    redis \
    ca-certificates \
    tzdata

# Create non-root user
RUN addgroup -g 1000 backup && \
    adduser -D -u 1000 -G backup backup

# Create directories
RUN mkdir -p /backups /etc/bared && \
    chown -R backup:backup /backups /etc/bared

# Copy binary from builder
COPY --from=builder /app/brd /usr/local/bin/brd
RUN chmod +x /usr/local/bin/brd

# Switch to non-root user
USER backup

# Set working directory
WORKDIR /etc/bared

# Volumes
VOLUME ["/backups", "/etc/bared"]

# Default command
ENTRYPOINT ["/usr/local/bin/brd"]
CMD ["--help"]
