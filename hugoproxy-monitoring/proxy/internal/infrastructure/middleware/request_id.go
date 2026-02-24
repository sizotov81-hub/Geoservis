package middleware

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"gitlab.com/s.izotov81/hugoproxy/internal/contextkeys"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/logger"
)

// RequestID middleware для генерации и добавления RequestID в контекст
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получаем или генерируем RequestID
		requestID := middleware.GetReqID(r.Context())
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Добавляем RequestID в контекст
		ctx := context.WithValue(r.Context(), contextkeys.RequestIDKey, requestID)

		// Добавляем логгер с RequestID в контекст
		log := logger.Get().With(zap.String("request_id", requestID))
		ctx = logger.WithContext(ctx, log)

		// Добавляем RequestID в заголовок ответа
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
