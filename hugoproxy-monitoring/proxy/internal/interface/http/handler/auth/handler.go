package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/go-chi/jwtauth/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/logger"
)

// User представляет модель пользователя
type User struct {
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
}

// RegisterRequest представляет данные для регистрации
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest представляет данные для входа
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse представляет ответ с JWT токеном
type LoginResponse struct {
	Token string `json:"token"`
}

// ErrorResponse представляет ответ об ошибке
type ErrorResponse struct {
	Error string `json:"error"`
}

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
	jwtSecret := "test-secret"
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		jwtSecret = secret
	}
	tokenAuth = jwtauth.New("HS256", []byte(jwtSecret), nil)
}

// RegisterHandler обрабатывает запрос на регистрацию пользователя
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("Register: invalid request format", zap.Error(err))
		http.Error(w, fmt.Sprintf("invalid request format: %v", err), http.StatusBadRequest)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Error("Register: failed to generate password hash", zap.String("email", req.Email), zap.Error(err))
		http.Error(w, fmt.Sprintf("failed to hash password: %v", err), http.StatusInternalServerError)
		return
	}

	userStore.Lock()
	defer userStore.Unlock()

	if _, exists := userStore.users[req.Email]; exists {
		log.Info("Register: user already exists", zap.String("email", req.Email))
		http.Error(w, ErrUserExists.Error(), http.StatusConflict)
		return
	}

	userStore.users[req.Email] = User{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
	}

	log.Info("Register: user created successfully", zap.String("email", req.Email))
	w.WriteHeader(http.StatusCreated)
}

// LoginHandler обрабатывает запрос на аутентификацию пользователя
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("Login: invalid request format", zap.Error(err))
		http.Error(w, fmt.Sprintf("invalid request format: %v", err), http.StatusBadRequest)
		return
	}

	userStore.RLock()
	user, exists := userStore.users[req.Email]
	userStore.RUnlock()

	if !exists {
		log.Info("Authentication failed: user not found", zap.String("email", req.Email), zap.String("ip", r.RemoteAddr))
		http.Error(w, ErrAuthFailed.Error(), http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		log.Info("Authentication failed: invalid password", zap.String("email", req.Email), zap.String("ip", r.RemoteAddr))
		http.Error(w, ErrAuthFailed.Error(), http.StatusUnauthorized)
		return
	}

	_, tokenString, err := tokenAuth.Encode(map[string]interface{}{"email": user.Email})
	if err != nil {
		log.Error("Login: failed to encode token", zap.String("email", req.Email), zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info("Login successful", zap.String("email", req.Email))
	json.NewEncoder(w).Encode(LoginResponse{Token: tokenString})
}
