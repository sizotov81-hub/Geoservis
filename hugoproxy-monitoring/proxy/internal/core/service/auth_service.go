package service

import (
	"context"
	"errors"
	"fmt"

	"gitlab.com/s.izotov81/hugoproxy/internal/core/entity"
	"gitlab.com/s.izotov81/hugoproxy/internal/core/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidCredentials ошибка при неверных учётных данных
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrUserNotFound ошибка при отсутствии пользователя
	ErrUserNotFound = repository.ErrUserNotFound
	// ErrUserAlreadyExists ошибка при попытке регистрации существующего пользователя
	ErrUserAlreadyExists = repository.ErrUserAlreadyExists
)

// AuthService предоставляет сервис аутентификации
type AuthService struct {
	userRepo  repository.UserRepository
	jwtSecret string
}

// NewAuthService создает новый экземпляр AuthService
func NewAuthService(userRepo repository.UserRepository, jwtSecret string) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
	}
}

// RegisterRequest представляет запрос на регистрацию
type RegisterRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"securepassword123"`
}

// LoginRequest представляет запрос на аутентификацию
type LoginRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"securepassword123"`
}

// LoginResponse представляет ответ с JWT токеном
type LoginResponse struct {
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

// Register регистрирует нового пользователя
func (s *AuthService) Register(ctx context.Context, req RegisterRequest) error {
	if err := validateEmail(req.Email); err != nil {
		return fmt.Errorf("invalid email: %w", err)
	}

	if err := validatePassword(req.Password); err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user := entity.User{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
	}

	return s.userRepo.Create(ctx, user)
}

// Login аутентифицирует пользователя и возвращает JWT токен
func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	if err := validateEmail(req.Email); err != nil {
		return nil, fmt.Errorf("invalid email: %w", err)
	}

	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.generateJWTToken(user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &LoginResponse{Token: token}, nil
}

// generateJWTToken генерирует JWT токен
func (s *AuthService) generateJWTToken(email string) (string, error) {
	// Используем jwtauth для генерации токена
	// Реализация через замыкание в main.go
	return "", errors.New("token generation not implemented in service layer")
}

// validateEmail проверяет корректность email
func validateEmail(email string) error {
	if email == "" {
		return errors.New("email is required")
	}
	if len(email) < 3 || len(email) > 254 {
		return errors.New("email length is invalid")
	}
	return nil
}

// validatePassword проверяет сложность пароля
func validatePassword(password string) error {
	if password == "" {
		return errors.New("password is required")
	}
	if len(password) < 6 {
		return errors.New("password must be at least 6 characters")
	}
	return nil
}
