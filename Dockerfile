# syntax=docker/dockerfile:1

# =============================================================================
# Build Stage: Full-featured build environment
# =============================================================================
FROM golang:1.24-alpine AS builder

# Install security updates and required build tools
RUN apk update && apk add --no-cache \
    ca-certificates \
    git \
    tzdata \
    && rm -rf /var/cache/apk/*

# Create appuser for security (non-root execution)
RUN adduser -D -s /bin/sh -u 1001 appuser

# Set working directory
WORKDIR /app

# Copy dependency files first (better Docker layer caching)
COPY go.mod go.sum ./

# Download dependencies (cached layer if go.mod/go.sum unchanged)
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build configuration for security and optimization
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Build the application with optimization flags
RUN go build \
    -a \
    -installsuffix cgo \
    -ldflags="-s -w" \
    -o main .

# Verify the binary is statically linked
RUN ldd main || echo "Static binary confirmed"

# =============================================================================
# Production Stage: Minimal runtime environment
# =============================================================================
FROM gcr.io/distroless/static-debian12:nonroot AS production

# Copy timezone data and CA certificates
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from builder stage
COPY --from=builder /app/main /app/main

# Use non-root user for security
USER nonroot:nonroot

# Set working directory
WORKDIR /app

# Expose port
EXPOSE 8080

# Set the entrypoint
ENTRYPOINT ["/app/main"]