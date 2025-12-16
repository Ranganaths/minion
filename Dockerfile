# Multi-stage build for optimized production image

# Stage 1: Build
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o minion \
    ./cmd/minion

# Stage 2: Runtime
FROM alpine:latest

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 minion && \
    adduser -D -u 1000 -G minion minion

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/minion /app/minion

# Copy configuration files (optional)
COPY --from=builder /build/config /app/config

# Set ownership
RUN chown -R minion:minion /app

# Switch to non-root user
USER minion

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/minion", "health"]

# Run the application
ENTRYPOINT ["/app/minion"]
CMD ["serve"]
