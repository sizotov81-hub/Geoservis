package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/go-chi/jwtauth/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"gitlab.com/s.izotov81/hugoproxy/internal/domain/entity"
	"gitlab.com/s.izotov81/hugoproxy/internal/domain/repository"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/logger"
)

var (
	ErrUserExists         = errors.New("user already exists")
	ErrAuthFailed         = errors.New("authentication failed")
	ErrInvalidEmail       = errors.New("invalid email format")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type UserResponse struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type Handler struct {
	userRepo  repository.UserRepository
	tokenAuth *jwtauth.JWTAuth
}

func NewHandler(userRepo repository.UserRepository, jwtSecret string) *Handler {
	return &Handler{
		userRepo:  userRepo,
		tokenAuth: jwtauth.New("HS256", []byte(jwtSecret), nil),
	}
}

func (h *Handler) GetTokenAuth() *jwtauth.JWTAuth {
	return h.tokenAuth
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	h, ok := r.Context().Value(authHandlerKey{}).(*Handler)
	if !ok {
		http.Error(w, "handler not configured", http.StatusInternalServerError)
		return
	}
	h.register(w, r)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("Register: invalid request format", zap.Error(err))
		http.Error(w, fmt.Sprintf("invalid request format: %v", err), http.StatusBadRequest)
		return
	}

	if err := validateEmail(req.Email); err != nil {
		log.Warn("Register: invalid email", zap.String("email", req.Email), zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validatePassword(req.Password); err != nil {
		log.Warn("Register: weak password", zap.String("email", req.Email), zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Error("Register: failed to hash password", zap.String("email", req.Email), zap.Error(err))
		http.Error(w, fmt.Sprintf("failed to hash password: %v", err), http.StatusInternalServerError)
		return
	}

	user := entity.User{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
	}

	if err := h.userRepo.Create(r.Context(), user); err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			log.Info("Register: user already exists", zap.String("email", req.Email))
			http.Error(w, ErrUserExists.Error(), http.StatusConflict)
			return
		}
		log.Error("Register: failed to create user", zap.String("email", req.Email), zap.Error(err))
		http.Error(w, fmt.Sprintf("failed to create user: %v", err), http.StatusInternalServerError)
		return
	}

	log.Info("Register: user created successfully", zap.String("email", req.Email))
	w.WriteHeader(http.StatusCreated)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	h, ok := r.Context().Value(authHandlerKey{}).(*Handler)
	if !ok {
		http.Error(w, "handler not configured", http.StatusInternalServerError)
		return
	}
	h.login(w, r)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("Login: invalid request format", zap.Error(err))
		http.Error(w, fmt.Sprintf("invalid request format: %v", err), http.StatusBadRequest)
		return
	}

	if err := validateEmail(req.Email); err != nil {
		log.Warn("Login: invalid email", zap.String("email", req.Email), zap.Error(err))
		http.Error(w, ErrInvalidCredentials.Error(), http.StatusUnauthorized)
		return
	}

	user, err := h.userRepo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			log.Info("Authentication failed: user not found", zap.String("email", req.Email), zap.String("ip", r.RemoteAddr))
		} else {
			log.Error("Login: failed to get user", zap.String("email", req.Email), zap.Error(err))
		}
		http.Error(w, ErrInvalidCredentials.Error(), http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		log.Info("Authentication failed: invalid password", zap.String("email", req.Email), zap.String("ip", r.RemoteAddr))
		http.Error(w, ErrInvalidCredentials.Error(), http.StatusUnauthorized)
		return
	}

	_, tokenString, err := h.tokenAuth.Encode(map[string]interface{}{"email": user.Email, "user_id": user.ID})
	if err != nil {
		log.Error("Login: failed to encode token", zap.String("email", req.Email), zap.Error(err))
		http.Error(w, fmt.Sprintf("failed to encode token: %v", err), http.StatusInternalServerError)
		return
	}

	log.Info("Login successful", zap.String("email", req.Email))
	json.NewEncoder(w).Encode(LoginResponse{Token: tokenString})
}

func validateEmail(email string) error {
	if email == "" {
		return ErrInvalidEmail
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return ErrInvalidEmail
	}

	if len(email) > 254 {
		return ErrInvalidEmail
	}

	return nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return ErrWeakPassword
	}
	return nil
}

type authHandlerKey struct{}
