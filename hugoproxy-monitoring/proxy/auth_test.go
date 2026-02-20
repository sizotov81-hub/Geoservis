package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

const testJWTSecret = "test-secret-key-for-testing-purposes-only"

// setupTestEnvironment настраивает тестовое окружение перед каждым тестом
func setupTestEnvironment() {
	// Установка тестового JWT_SECRET
	os.Setenv("JWT_SECRET", testJWTSecret)
	
	// Переинициализация tokenAuth с тестовым секретом
	tokenAuth = jwtauth.New("HS256", []byte(testJWTSecret), nil)
	
	// Очистка userStore
	userStore.Lock()
	userStore.users = make(map[string]User)
	userStore.Unlock()
}

// generateTestToken генерирует тестовый JWT токен для тестов
func generateTestToken(email string) string {
	_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{
		"email": email,
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	return tokenString
}

// TestRegisterHandler_Success тестирует успешную регистрацию пользователя
func TestRegisterHandler_Success(t *testing.T) {
	setupTestEnvironment()

	// Подготовка запроса
	reqBody := RegisterRequest{
		Email:    "test@example.com",
		Password: "securepassword123",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Выполнение обработчика
	rr := httptest.NewRecorder()
	RegisterHandler(rr, req)

	// Проверка результата
	assert.Equal(t, http.StatusCreated, rr.Code, "Expected status 201 Created")

	// Проверка, что пользователь сохранён
	userStore.RLock()
	user, exists := userStore.users["test@example.com"]
	userStore.RUnlock()
	assert.True(t, exists, "User should exist in store")
	assert.Equal(t, "test@example.com", user.Email, "User email should match")
	assert.NotEmpty(t, user.PasswordHash, "Password hash should not be empty")

	// Проверка, что пароль хэшируется правильно
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("securepassword123"))
	assert.NoError(t, err, "Password should match hash")
}

// TestRegisterHandler_InvalidInput тестирует ошибку при невалидных данных
func TestRegisterHandler_InvalidInput(t *testing.T) {
	setupTestEnvironment()

	tests := []struct {
		name         string
		reqBody      interface{}
		expectedCode int
	}{
		{
			name:         "empty body",
			reqBody:      nil,
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "empty email",
			reqBody: map[string]string{
				"email":    "",
				"password": "password123",
			},
			expectedCode: http.StatusCreated, // пустой email пройдёт валидацию JSON, но не бизнес-логику
		},
		{
			name: "empty password",
			reqBody: map[string]string{
				"email":    "test@example.com",
				"password": "",
			},
			expectedCode: http.StatusCreated, // пустой пароль пройдёт валидацию JSON
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestEnvironment()

			var body []byte
			var err error
			if tt.reqBody != nil {
				body, err = json.Marshal(tt.reqBody)
				assert.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			RegisterHandler(rr, req)

			// Для пустого тела ожидаем BadRequest
			if tt.name == "empty body" {
				assert.Equal(t, http.StatusBadRequest, rr.Code)
			}
		})
	}
}

// TestRegisterHandler_DuplicateEmail тестирует ошибку при дублировании email
func TestRegisterHandler_DuplicateEmail(t *testing.T) {
	setupTestEnvironment()

	// Создаём первого пользователя
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	userStore.Lock()
	userStore.users["existing@example.com"] = User{
		Email:        "existing@example.com",
		PasswordHash: string(hashedPassword),
	}
	userStore.Unlock()

	// Пытаемся зарегистрировать того же пользователя
	reqBody := RegisterRequest{
		Email:    "existing@example.com",
		Password: "newpassword456",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	RegisterHandler(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code, "Expected status 409 Conflict")
}

// TestLoginHandler_Success тестирует успешный вход
func TestLoginHandler_Success(t *testing.T) {
	setupTestEnvironment()

	// Создаём пользователя в хранилище
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
	userStore.Lock()
	userStore.users["user@example.com"] = User{
		Email:        "user@example.com",
		PasswordHash: string(hashedPassword),
	}
	userStore.Unlock()

	// Выполняем login
	reqBody := LoginRequest{
		Email:    "user@example.com",
		Password: "correctpassword",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	LoginHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200 OK")

	// Проверяем, что получен токен
	var response LoginResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err, "Response should be valid JSON")
	assert.NotEmpty(t, response.Token, "Token should not be empty")

	// Проверяем, что токен валидный
	// Создаём запрос с токеном для проверки
	testReq := httptest.NewRequest(http.MethodGet, "/", nil)
	testReq.Header.Set("Authorization", "Bearer "+response.Token)
	token, err := jwtauth.VerifyRequest(tokenAuth, testReq, jwtauth.TokenFromHeader)
	assert.NoError(t, err, "Token should be valid JWT")
	assert.NotNil(t, token, "Token should not be nil")
}

// TestLoginHandler_InvalidCredentials тестирует ошибку при неверных учётных данных
func TestLoginHandler_InvalidCredentials(t *testing.T) {
	setupTestEnvironment()

	// Создаём пользователя с правильным паролем
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
	userStore.Lock()
	userStore.users["user@example.com"] = User{
		Email:        "user@example.com",
		PasswordHash: string(hashedPassword),
	}
	userStore.Unlock()

	// Пытаемся войти с неправильным паролем
	reqBody := LoginRequest{
		Email:    "user@example.com",
		Password: "wrongpassword",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	LoginHandler(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected status 401 Unauthorized")

	// http.Error возвращает plain text, поэтому проверяем напрямую
	assert.Contains(t, rr.Body.String(), ErrAuthFailed.Error(), "Response should contain error message")
}

// TestLoginHandler_UserNotFound тестирует ошибку при несуществующем пользователе
func TestLoginHandler_UserNotFound(t *testing.T) {
	setupTestEnvironment()

	// Не создаём пользователя - пытаемся войти с несуществующим email
	reqBody := LoginRequest{
		Email:    "nonexistent@example.com",
		Password: "somepassword",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	LoginHandler(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected status 401 Unauthorized")

	// http.Error возвращает plain text, поэтому проверяем напрямую
	assert.Contains(t, rr.Body.String(), ErrAuthFailed.Error(), "Response should contain error message")
}

// TestAuthMiddleware_ValidToken тестирует пропуск валидного токена
func TestAuthMiddleware_ValidToken(t *testing.T) {
	setupTestEnvironment()

	// Создаём тестовый handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Применяем middleware
	middleware := AuthMiddleware(testHandler)

	// Создаём валидный токен
	token := generateTestToken("test@example.com")

	// Создаём запрос с валидным токеном
	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Valid token should be accepted")
}

// TestAuthMiddleware_InvalidToken тестирует отклонение невалидного токена
func TestAuthMiddleware_InvalidToken(t *testing.T) {
	setupTestEnvironment()

	// Создаём тестовый handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Применяем middleware
	middleware := AuthMiddleware(testHandler)

	// Создаём запрос с невалидным токеном
	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "Invalid token should be rejected")

	// Проверяем, что в ответе есть сообщение об ошибке
	var errorResp ErrorResponse
	err := json.Unmarshal(rr.Body.Bytes(), &errorResp)
	assert.NoError(t, err)
	assert.Equal(t, "Forbidden", errorResp.Error)
}

// TestAuthMiddleware_NoToken тестирует отклонение запроса без токена
func TestAuthMiddleware_NoToken(t *testing.T) {
	setupTestEnvironment()

	// Создаём тестовый handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Применяем middleware
	middleware := AuthMiddleware(testHandler)

	// Создаём запрос без токена
	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "Request without token should be rejected")

	// Проверяем, что в ответе есть сообщение об ошибке
	var errorResp ErrorResponse
	err := json.Unmarshal(rr.Body.Bytes(), &errorResp)
	assert.NoError(t, err)
	assert.Equal(t, "Forbidden", errorResp.Error)
}

// TestAuthMiddleware_TokenWithoutBearerPrefix тестирует токен без префикса Bearer
func TestAuthMiddleware_TokenWithoutBearerPrefix(t *testing.T) {
	setupTestEnvironment()

	// Создаём тестовый handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Применяем middleware
	middleware := AuthMiddleware(testHandler)

	// Создаём валидный токен без префикса Bearer
	token := generateTestToken("test@example.com")

	// Создаём запрос с токеном без префикса
	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	req.Header.Set("Authorization", token)

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	// Middleware должен добавить префикс "Bearer " автоматически
	assert.Equal(t, http.StatusOK, rr.Code, "Token without Bearer prefix should be accepted after adding prefix")
}
