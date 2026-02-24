package contextkeys

import "context"

type contextKey string

const (
	// RequestIDKey ключ для хранения RequestID в контексте
	RequestIDKey contextKey = "request_id"
)

// GetRequestID возвращает RequestID из контекста
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// SetRequestID добавляет RequestID в контекст
func SetRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}
