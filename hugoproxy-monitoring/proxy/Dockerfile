# Build stage
FROM golang:1.23 as builder

WORKDIR /app
COPY . .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o hugoproxy .

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/hugoproxy .
COPY --from=builder /app/static ./static
COPY .env .
# Копируем директорию с миграциями
COPY --from=builder /app/internal/infrastructure/db/migrations ./migrations
CMD ["./hugoproxy"]

