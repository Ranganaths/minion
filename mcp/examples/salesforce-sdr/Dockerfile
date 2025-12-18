# Multi-stage build for Virtual SDR Agent
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o virtual-sdr main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    nodejs \
    npm \
    && npm install -g npm@latest

# Create app user
RUN addgroup -g 1000 sdr && \
    adduser -D -u 1000 -G sdr sdr

WORKDIR /home/sdr

# Copy binary from builder
COPY --from=builder /app/virtual-sdr .

# Change ownership
RUN chown -R sdr:sdr /home/sdr

# Switch to non-root user
USER sdr

# Expose metrics port
EXPOSE 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9090/health || exit 1

# Run the application
CMD ["./virtual-sdr"]
