# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk --no-cache add git gcc musl-dev

WORKDIR /app

# Copy only dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Install swag for Swagger documentation
RUN go install github.com/swaggo/swag/cmd/swag@latest

# Copy source code
COPY *.go ./
COPY internal/ ./internal/
COPY pkg/ ./pkg/
COPY static/ ./static/

# Generate Swagger documentation
RUN /go/bin/swag init -g main.go -o ./docs

# Build the binary with static linking
RUN CGO_ENABLED=0 GOOS=linux go build -o hugoproxy -ldflags="-s -w" .

# Runtime stage - minimal image
FROM scratch

# Copy CA certificates for HTTPS support
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary
COPY --from=builder /app/hugoproxy /hugoproxy

# Copy static files
COPY --from=builder /app/static /static

# Copy database migrations
COPY --from=builder /app/internal/infrastructure/db/migrations /migrations

# Run the application
CMD ["/hugoproxy"]
