package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/go-chi/jwtauth/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/logger"
)

// Глобальные переменные для аутентификации
var (
	// ErrUserExists ошибка при попытке регистрации существующего пользователя
	ErrUserExists = errors.New("user already exists")
	// ErrAuthFailed ошибка при неудачной аутентификации
	ErrAuthFailed = errors.New("authentication failed")
	// tokenAuth экземпляр JWTAuth для работы с JWT токенами
	tokenAuth *jwtauth.JWTAuth
	// userStore хранилище пользователей в памяти
	userStore = struct {
		sync.RWMutex
		users map[string]User
	}{users: make(map[string]User)}
)

func init() {
	// Инициализация JWT аутентификации с алгоритмом HS256
	jwtSecret := "test-secret" // Значение по умолчанию для тестов
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		jwtSecret = secret
	}
	tokenAuth = jwtauth.New("HS256", []byte(jwtSecret), nil)
}

// newAuthMiddleware создает middleware для проверки JWT токена
func newAuthMiddleware(jwtSecret string) func(next http.Handler) http.Handler {
	tAuth := jwtauth.New("HS256", []byte(jwtSecret), nil)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")

			if authHeader != "" && !strings.HasPrefix(authHeader, "Bearer ") {
				authHeader = "Bearer " + authHeader
				r.Header.Set("Authorization", authHeader)
			}

			token, err := jwtauth.VerifyRequest(tAuth, r, jwtauth.TokenFromHeader)
			if err != nil || token == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(ErrorResponse{Error: "Forbidden"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RegisterHandler обрабатывает запрос на регистрацию пользователя
// @Summary Регистрация нового пользователя
// @Description Создает нового пользователя в системе
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Данные для регистрации"
// @Success 201 "Пользователь успешно зарегистрирован"
// @Failure 400 {object} ErrorResponse "Некорректные данные запроса"
// @Failure 409 {object} ErrorResponse "Пользователь уже существует"
// @Failure 500 {object} ErrorResponse "Ошибка сервера"
// @Router /api/register [post]
// @security BearerAuth
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("Register: invalid request format", zap.Error(err))
		http.Error(w, fmt.Sprintf("invalid request format: %v", err), http.StatusBadRequest)
		return
	}

	// Генерация хэша пароля
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Error("Register: failed to generate password hash", zap.String("email", req.Email), zap.Error(err))
		http.Error(w, fmt.Sprintf("failed to hash password: %v", err), http.StatusInternalServerError)
		return
	}

	userStore.Lock()
	defer userStore.Unlock()

	// Проверка существования пользователя
	if _, exists := userStore.users[req.Email]; exists {
		log.Info("Register: user already exists", zap.String("email", req.Email))
		http.Error(w, ErrUserExists.Error(), http.StatusConflict)
		return
	}

	// Сохранение пользователя
	userStore.users[req.Email] = User{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
	}

	log.Info("Register: user created successfully", zap.String("email", req.Email))
	w.WriteHeader(http.StatusCreated)
}

// LoginHandler обрабатывает запрос на аутентификацию пользователя
// @Summary Аутентификация пользователя
// @Description Проверяет учетные данные и возвращает JWT токен
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Данные для входа"
// @Success 200 {object} LoginResponse "Успешная аутентификация"
// @Failure 400 {object} ErrorResponse "Некорректные данные запроса"
// @Failure 401 {object} ErrorResponse "Ошибка аутентификации"
// @Failure 500 {object} ErrorResponse "Ошибка сервера"
// @Router /api/login [post]
// @security BearerAuth
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("Login: invalid request format", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userStore.RLock()
	user, exists := userStore.users[req.Email]
	userStore.RUnlock()

	// Проверка существования пользователя
	if !exists {
		log.Info("Authentication failed: user not found", zap.String("email", req.Email), zap.String("ip", r.RemoteAddr))
		http.Error(w, ErrAuthFailed.Error(), http.StatusUnauthorized)
		return
	}

	// Проверка пароля
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		log.Info("Authentication failed: invalid password", zap.String("email", req.Email), zap.String("ip", r.RemoteAddr))
		http.Error(w, ErrAuthFailed.Error(), http.StatusUnauthorized)
		return
	}

	// Генерация JWT токена
	_, tokenString, err := tokenAuth.Encode(map[string]interface{}{"email": user.Email})
	if err != nil {
		log.Error("Login: failed to encode token", zap.String("email", req.Email), zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info("Login successful", zap.String("email", req.Email))
	// Возврат токена
	json.NewEncoder(w).Encode(LoginResponse{Token: tokenString})
}
