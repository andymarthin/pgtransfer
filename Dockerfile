# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments
ARG VERSION=dev

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o pgtransfer .

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    postgresql-client \
    openssh-client \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1001 -S pgtransfer && \
    adduser -u 1001 -S pgtransfer -G pgtransfer

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/pgtransfer /usr/local/bin/pgtransfer

# Make binary executable
RUN chmod +x /usr/local/bin/pgtransfer

# Switch to non-root user
USER pgtransfer

# Set entrypoint
ENTRYPOINT ["pgtransfer"]

# Default command
CMD ["--help"]