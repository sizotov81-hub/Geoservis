package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/go-chi/jwtauth/v5"
	"golang.org/x/crypto/bcrypt"
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
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatalf("JWT_SECRET environment variable is required")
	}
	tokenAuth = jwtauth.New("HS256", []byte(jwtSecret), nil)
}

// User представляет модель пользователя системы
// @Description Информация о пользователе системы
type User struct {
	Email        string `json:"email" example:"user@example.com"` // Email пользователя
	PasswordHash string `json:"-"`                                // Хэш пароля (не возвращается в ответах)
}

// RegisterRequest представляет запрос на регистрацию
// @Description Данные для регистрации нового пользователя
type RegisterRequest struct {
	Email    string `json:"email" example:"user@example.com"`     // Email пользователя
	Password string `json:"password" example:"securepassword123"` // Пароль пользователя
}

// LoginRequest представляет запрос на аутентификацию
// @Description Данные для входа пользователя
type LoginRequest struct {
	Email    string `json:"email" example:"user@example.com"`     // Email пользователя
	Password string `json:"password" example:"securepassword123"` // Пароль пользователя
}

// LoginResponse представляет ответ с JWT токеном
// @Description Ответ сервера с JWT токеном после успешной аутентификации
type LoginResponse struct {
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."` // JWT токен
}

// ErrorResponse представляет стандартный ответ об ошибке
// @Description Стандартный формат ответа при возникновении ошибки
type ErrorResponse struct {
	Error string `json:"error" example:"error message"` // Описание ошибки
}

// AuthMiddleware middleware для проверки JWT токена
// @Security BearerAuth
// @Description Middleware проверяет валидность JWT токена в заголовке Authorization.
// Добавляется к защищенным маршрутам для проверки аутентификации.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		// Добавляем префикс "Bearer ", если его нет
		if authHeader != "" && !strings.HasPrefix(authHeader, "Bearer ") {
			authHeader = "Bearer " + authHeader
			r.Header.Set("Authorization", authHeader)
		}

		// Проверка валидности токена
		token, err := jwtauth.VerifyRequest(tokenAuth, r, jwtauth.TokenFromHeader)
		if err != nil || token == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "Forbidden"})
			return
		}

		next.ServeHTTP(w, r)
	})
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
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Генерация хэша пароля
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	userStore.Lock()
	defer userStore.Unlock()

	// Проверка существования пользователя
	if _, exists := userStore.users[req.Email]; exists {
		http.Error(w, ErrUserExists.Error(), http.StatusConflict)
		return
	}

	// Сохранение пользователя
	userStore.users[req.Email] = User{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
	}

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
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userStore.RLock()
	user, exists := userStore.users[req.Email]
	userStore.RUnlock()

	// Проверка существования пользователя
	if !exists {
		log.Printf("Authentication failed: user not found, email=%s, ip=%s", req.Email, r.RemoteAddr)
		http.Error(w, ErrAuthFailed.Error(), http.StatusUnauthorized)
		return
	}

	// Проверка пароля
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		log.Printf("Authentication failed: invalid password, email=%s, ip=%s", req.Email, r.RemoteAddr)
		http.Error(w, ErrAuthFailed.Error(), http.StatusUnauthorized)
		return
	}

	// Генерация JWT токена
	_, tokenString, err := tokenAuth.Encode(map[string]interface{}{"email": user.Email})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Возврат токена
	json.NewEncoder(w).Encode(LoginResponse{Token: tokenString})
}
