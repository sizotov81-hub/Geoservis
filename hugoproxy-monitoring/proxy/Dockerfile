# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk --no-cache add git

WORKDIR /app

# Copy only dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY *.go ./
COPY internal/ ./internal/
COPY static/ ./static/

# Build the binary with static linking
RUN CGO_ENABLED=0 GOOS=linux go build -o hugoproxy -ldflags="-s -w" .

# Runtime stage - minimal image
FROM scratch

# Copy CA certificates for HTTPS support
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary
COPY --from=builder /app/hugoproxy .

# Copy static files
COPY --from=builder /app/static ./static

# Copy database migrations
COPY --from=builder /app/internal/infrastructure/db/migrations ./migrations

# Create non-root user for security
USER 1000:1000

# Set working directory
WORKDIR /root/

# Run the application
CMD ["./hugoproxy"]
