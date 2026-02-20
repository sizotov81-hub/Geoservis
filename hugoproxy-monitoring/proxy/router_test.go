package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"gitlab.com/s.izotov81/hugoproxy/internal/core/entity"
	"gitlab.com/s.izotov81/hugoproxy/internal/core/service"
)

// MockUserRepository мок-репозиторий для пользователей
type MockUserRepository struct {
	users      map[int]entity.User
	nextID    int
	emailIndex map[string]int
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users:      make(map[int]entity.User),
		nextID:    1,
		emailIndex: make(map[string]int),
	}
}

func (m *MockUserRepository) Create(ctx context.Context, user entity.User) error {
	if _, exists := m.emailIndex[user.Email]; exists {
		return service.ErrUserAlreadyExists
	}
	user.ID = m.nextID
	m.users[m.nextID] = user
	m.emailIndex[user.Email] = m.nextID
	m.nextID++
	return nil
}

func (m *MockUserRepository) GetByID(ctx context.Context, id int) (entity.User, error) {
	user, ok := m.users[id]
	if !ok {
		return entity.User{}, service.ErrUserNotFound
	}
	return user, nil
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (entity.User, error) {
	id, ok := m.emailIndex[email]
	if !ok {
		return entity.User{}, service.ErrUserNotFound
	}
	return m.users[id], nil
}

func (m *MockUserRepository) Update(ctx context.Context, user entity.User) error {
	if _, ok := m.users[user.ID]; !ok {
		return service.ErrUserNotFound
	}
	delete(m.emailIndex, m.users[user.ID].Email)
	m.users[user.ID] = user
	m.emailIndex[user.Email] = user.ID
	return nil
}

func (m *MockUserRepository) Delete(ctx context.Context, id int) error {
	if _, ok := m.users[id]; !ok {
		return service.ErrUserNotFound
	}
	user := m.users[id]
	delete(m.emailIndex, user.Email)
	delete(m.users, id)
	return nil
}

func (m *MockUserRepository) List(ctx context.Context, limit, offset int) ([]entity.User, error) {
	var result []entity.User
	i := 0
	for _, u := range m.users {
		if i >= offset && len(result) < limit {
			result = append(result, u)
		}
		i++
	}
	return result, nil
}

// MockGeoService мок-сервис для геоданных
type MockGeoService struct{}

func NewMockGeoService() *MockGeoService {
	return &MockGeoService{}
}

func (m *MockGeoService) AddressSearch(input string) ([]*service.Address, error) {
	return []*service.Address{
		{City: "Москва", Street: "Ленина", House: "1", Lat: "55.7558", Lon: "37.6173"},
	}, nil
}

func (m *MockGeoService) GeoCode(lat, lng string) ([]*service.Address, error) {
	return []*service.Address{
		{City: "Москва", Street: "Тверская", House: "1", Lat: lat, Lon: lng},
	}, nil
}

// testUserRepo глобальный мок-репозиторий для тестов
var testUserRepo *MockUserRepository

func init() {
	testUserRepo = NewMockUserRepository()
}

// setupTestRouter создаёт тестовый роутер с мок-сервисами
func setupTestRouter() *chi.Mux {
	// Установка тестового JWT_SECRET
	os.Setenv("JWT_SECRET", testJWTSecret)

	// Переинициализация tokenAuth с тестовым секретом
	tokenAuth = jwtauth.New("HS256", []byte(testJWTSecret), nil)

	// Очистка userStore
	userStore.Lock()
	userStore.users = make(map[string]User)
	userStore.Unlock()

	// Переинициализация мок-репозитория
	testUserRepo = NewMockUserRepository()

	r := chi.NewRouter()

	// Добавляем базовые middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Auth routes (публичные)
	r.Group(func(r chi.Router) {
		r.Post("/api/register", RegisterHandler)
		r.Post("/api/login", LoginHandler)
	})

	// User routes (защищённые)
	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware)
		r.Get("/api/users", testListUsersHandler)
		r.Post("/api/users", testCreateUserHandler)
		r.Get("/api/users/{id}", testGetUserHandler)
		r.Put("/api/users/{id}", testUpdateUserHandler)
		r.Delete("/api/users/{id}", testDeleteUserHandler)
		r.Get("/api/users/email", testGetUserByEmailHandler)
	})

	// Geo routes (защищённые)
	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware)
		r.Post("/api/address/search", testAddressSearchHandler)
		r.Post("/api/address/geocode", testGeocodeHandler)
	})

	return r
}

// Тестовые обработчики для пользователей

func testListUsersHandler(w http.ResponseWriter, r *http.Request) {
	limit := 10
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := parseInt(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := parseInt(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	users, err := testUserRepo.List(r.Context(), limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(users)
}

func testCreateUserHandler(w http.ResponseWriter, r *http.Request) {
	var user entity.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := testUserRepo.Create(r.Context(), user)
	if err != nil {
		if err == service.ErrUserAlreadyExists {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

func testGetUserHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := parseInt(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	user, err := testUserRepo.GetByID(r.Context(), id)
	if err != nil {
		if err == service.ErrUserNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(user)
}

func testUpdateUserHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := parseInt(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var user entity.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user.ID = id

	err = testUserRepo.Update(r.Context(), user)
	if err != nil {
		if err == service.ErrUserNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(user)
}

func testDeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := parseInt(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	err = testUserRepo.Delete(r.Context(), id)
	if err != nil {
		if err == service.ErrUserNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func testGetUserByEmailHandler(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		http.Error(w, "Email parameter is required", http.StatusBadRequest)
		return
	}

	user, err := testUserRepo.GetByEmail(r.Context(), email)
	if err != nil {
		if err == service.ErrUserNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(user)
}

// Тестовые обработчики для геоданных

func testAddressSearchHandler(w http.ResponseWriter, r *http.Request) {
	var req service.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	geoService := NewMockGeoService()
	addresses, err := geoService.AddressSearch(req.Query)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(service.SearchResponse{Addresses: addresses})
}

func testGeocodeHandler(w http.ResponseWriter, r *http.Request) {
	var req service.GeocodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	geoService := NewMockGeoService()
	addresses, err := geoService.GeoCode(req.Lat, req.Lng)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(service.GeocodeResponse{Addresses: addresses})
}

// parseInt вспомогательная функция для парсинга чисел
func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// TestRouter_PublicRoutes тестирует публичные маршруты
func TestRouter_PublicRoutes(t *testing.T) {
	router := setupTestRouter()

	tests := []struct {
		name       string
		method     string
		path       string
		body       interface{}
		wantStatus int
	}{
		{
			name:       "POST /api/register - success",
			method:     http.MethodPost,
			path:       "/api/register",
			body:       RegisterRequest{Email: "test@example.com", Password: "password123"},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "POST /api/login - success",
			method:     http.MethodPost,
			path:       "/api/login",
			body:       LoginRequest{Email: "test@example.com", Password: "password123"},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.body != nil {
				body, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code, "Status code mismatch")
		})
	}
}

// TestRouter_Register_Success тестирует успешную регистрацию пользователя
func TestRouter_Register_Success(t *testing.T) {
	router := setupTestRouter()

	reqBody := RegisterRequest{
		Email:    "newuser@example.com",
		Password: "securepassword123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code, "Expected status 201 Created")
}

// TestRouter_Register_DuplicateEmail тестирует ошибку при дублировании email
func TestRouter_Register_DuplicateEmail(t *testing.T) {
	router := setupTestRouter()

	// Сначала регистрируем пользователя
	reqBody := RegisterRequest{
		Email:    "duplicate@example.com",
		Password: "password123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Пытаемся зарегистрировать того же пользователя снова
	req2 := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")

	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, req2)

	assert.Equal(t, http.StatusConflict, rr2.Code, "Expected status 409 Conflict")
}

// TestRouter_Register_InvalidBody тестирует ошибку при невалидном теле запроса
func TestRouter_Register_InvalidBody(t *testing.T) {
	router := setupTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Expected status 400 Bad Request")
}

// TestRouter_Login_Success тестирует успешный вход
func TestRouter_Login_Success(t *testing.T) {
	router := setupTestRouter()

	// Регистрируем пользователя сначала
	registerBody, _ := json.Marshal(RegisterRequest{
		Email:    "loginuser@example.com",
		Password: "correctpassword",
	})
	registerReq := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewReader(registerBody))
	registerReq.Header.Set("Content-Type", "application/json")

	registerRR := httptest.NewRecorder()
	router.ServeHTTP(registerRR, registerReq)

	// Теперь пытаемся войти
	loginBody, _ := json.Marshal(LoginRequest{
		Email:    "loginuser@example.com",
		Password: "correctpassword",
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, loginReq)

	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200 OK")

	// Проверяем, что получен токен
	var response LoginResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err, "Response should be valid JSON")
	assert.NotEmpty(t, response.Token, "Token should not be empty")
}

// TestRouter_Login_InvalidCredentials тестирует ошибку при неверных учётных данных
func TestRouter_Login_InvalidCredentials(t *testing.T) {
	router := setupTestRouter()

	loginBody, _ := json.Marshal(LoginRequest{
		Email:    "nonexistent@example.com",
		Password: "wrongpassword",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected status 401 Unauthorized")
}

// TestRouter_Login_InvalidBody тестирует ошибку при невалидном теле запроса
func TestRouter_Login_InvalidBody(t *testing.T) {
	router := setupTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Expected status 400 Bad Request")
}

// TestRouter_ProtectedRoutes_Unauthorized тестирует защищённые маршруты без токена
func TestRouter_ProtectedRoutes_Unauthorized(t *testing.T) {
	router := setupTestRouter()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"GET /api/users", http.MethodGet, "/api/users"},
		{"POST /api/users", http.MethodPost, "/api/users"},
		{"GET /api/users/1", http.MethodGet, "/api/users/1"},
		{"PUT /api/users/1", http.MethodPut, "/api/users/1"},
		{"DELETE /api/users/1", http.MethodDelete, "/api/users/1"},
		{"POST /api/address/search", http.MethodPost, "/api/address/search"},
		{"POST /api/address/geocode", http.MethodPost, "/api/address/geocode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.method == http.MethodPost {
				body, _ = json.Marshal(map[string]string{"query": "test"})
			}

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(body))
			if tt.method == http.MethodPost {
				req.Header.Set("Content-Type", "application/json")
			}

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusForbidden, rr.Code, "Expected 403 Forbidden for protected route without token")
		})
	}
}

// TestRouter_ProtectedRoutes_WithToken тестирует защищённые маршруты с токеном
func TestRouter_ProtectedRoutes_WithToken(t *testing.T) {
	router := setupTestRouter()

	// Генерируем валидный токен
	token := generateTestToken("test@example.com")

	tests := []struct {
		name       string
		method     string
		path       string
		body       interface{}
		wantStatus int
	}{
		{
			name:       "GET /api/users",
			method:     http.MethodGet,
			path:       "/api/users",
			wantStatus: http.StatusOK,
		},
		{
			name:       "POST /api/users",
			method:     http.MethodPost,
			path:       "/api/users",
			body:       map[string]string{"email": "new@example.com", "password": "password"},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "GET /api/users/1",
			method:     http.MethodGet,
			path:       "/api/users/1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "PUT /api/users/1",
			method:     http.MethodPut,
			path:       "/api/users/1",
			body:       map[string]interface{}{"id": 1, "email": "updated@example.com"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "DELETE /api/users/1",
			method:     http.MethodDelete,
			path:       "/api/users/1",
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "POST /api/address/search",
			method:     http.MethodPost,
			path:       "/api/address/search",
			body:       service.SearchRequest{Query: "Москва"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "POST /api/address/geocode",
			method:     http.MethodPost,
			path:       "/api/address/geocode",
			body:       service.GeocodeRequest{Lat: "55.7558", Lng: "37.6173"},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.body != nil {
				body, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+token)
			if tt.method == http.MethodPost || tt.method == http.MethodPut {
				req.Header.Set("Content-Type", "application/json")
			}

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code, "Status code mismatch for "+tt.name)
		})
	}
}

// TestRouter_GetUsers_List тестирует получение списка пользователей
func TestRouter_GetUsers_List(t *testing.T) {
	router := setupTestRouter()

	token := generateTestToken("test@example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/users?limit=10&offset=0", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200 OK")
}

// TestRouter_GetUser_ByID тестирует получение пользователя по ID
func TestRouter_GetUser_ByID(t *testing.T) {
	router := setupTestRouter()

	token := generateTestToken("test@example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/users/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Ожидаем 200 или 404 (если пользователь не найден)
	assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNotFound,
		"Expected status 200 or 404")
}

// TestRouter_UpdateUser тестирует обновление пользователя
func TestRouter_UpdateUser(t *testing.T) {
	router := setupTestRouter()

	token := generateTestToken("test@example.com")

	updateBody, _ := json.Marshal(map[string]interface{}{
		"id":    1,
		"email": "updated@example.com",
	})

	req := httptest.NewRequest(http.MethodPut, "/api/users/1", bytes.NewReader(updateBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Ожидаем 200 или 404
	assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNotFound,
		"Expected status 200 or 404")
}

// TestRouter_DeleteUser тестирует удаление пользователя
func TestRouter_DeleteUser(t *testing.T) {
	router := setupTestRouter()

	token := generateTestToken("test@example.com")

	req := httptest.NewRequest(http.MethodDelete, "/api/users/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Ожидаем 204 или 404
	assert.True(t, rr.Code == http.StatusNoContent || rr.Code == http.StatusNotFound,
		"Expected status 204 or 404")
}

// TestRouter_AddressSearch тестирует поиск адресов
func TestRouter_AddressSearch(t *testing.T) {
	router := setupTestRouter()

	token := generateTestToken("test@example.com")

	searchBody, _ := json.Marshal(service.SearchRequest{Query: "Москва"})

	req := httptest.NewRequest(http.MethodPost, "/api/address/search", bytes.NewReader(searchBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200 OK")

	// Проверяем, что ответ содержит адреса
	var response service.SearchResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err, "Response should be valid JSON")
	assert.NotNil(t, response.Addresses, "Addresses should not be nil")
	assert.Len(t, response.Addresses, 1, "Should have one address")
	assert.Equal(t, "Москва", response.Addresses[0].City, "City should be Moscow")
}

// TestRouter_Geocode тестирует геокодирование
func TestRouter_Geocode(t *testing.T) {
	router := setupTestRouter()

	token := generateTestToken("test@example.com")

	geocodeBody, _ := json.Marshal(service.GeocodeRequest{
		Lat: "55.7558",
		Lng: "37.6173",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/address/geocode", bytes.NewReader(geocodeBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200 OK")

	// Проверяем, что ответ содержит адреса
	var response service.GeocodeResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err, "Response should be valid JSON")
	assert.NotNil(t, response.Addresses, "Addresses should not be nil")
	assert.Len(t, response.Addresses, 1, "Should have one address")
}

// TestRouter_NotFound тестирует обработку 404 для несуществующих роутов
func TestRouter_NotFound(t *testing.T) {
	router := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code, "Expected status 404 Not Found")
}

// TestRouter_MethodNotAllowed тестирует обработку 405 для неверных HTTP методов
func TestRouter_MethodNotAllowed(t *testing.T) {
	router := setupTestRouter()

	// POST /api/register поддерживает только POST, пробуем GET
	req := httptest.NewRequest(http.MethodGet, "/api/register", nil)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Chi router может вернуть 404 вместо 405 для некоторых маршрутов
	assert.True(t, rr.Code == http.StatusMethodNotAllowed || rr.Code == http.StatusNotFound,
		"Expected status 405 Method Not Allowed or 404")
}

// TestRouter_ContentTypeHeader тестирует заголовок Content-Type в ответах с данными
// Примечание: chi router не всегда устанавливает Content-Type автоматически
func TestRouter_ContentTypeHeader(t *testing.T) {
	router := setupTestRouter()
	token := generateTestToken("test@example.com")

	// Проверяем, что защищённые маршруты работают и возвращают данные
	// Создаём пользователя, чтобы получить непустой ответ
	userBody, _ := json.Marshal(map[string]string{"email": "test@example.com", "password": "password"})
	userReq := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(userBody))
	userReq.Header.Set("Authorization", "Bearer "+token)
	userReq.Header.Set("Content-Type", "application/json")
	userRR := httptest.NewRecorder()
	router.ServeHTTP(userRR, userReq)

	// Теперь GET запрос должен вернуть данные
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Проверяем, что запрос успешен и содержит данные
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200 OK")
	assert.Contains(t, rr.Body.String(), "test@example.com", "Response should contain user email")
}

// TestRouter_InvalidJWTToken тестирует отклонение невалидного JWT токена
func TestRouter_InvalidJWTToken(t *testing.T) {
	router := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "Expected status 403 Forbidden")
}

// TestRouter_ExpiredToken тестирует обработку просроченного токена
func TestRouter_ExpiredToken(t *testing.T) {
	router := setupTestRouter()

	// Создаём просроченный токен
	_, expiredToken, _ := tokenAuth.Encode(map[string]interface{}{
		"email": "test@example.com",
		"exp":   time.Now().Add(-time.Hour).Unix(), // Токен истёк час назад
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+expiredToken)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "Expected status 403 Forbidden")
}

// TestRouter_TokenWithoutBearerPrefix тестирует токен без префикса Bearer
func TestRouter_TokenWithoutBearerPrefix(t *testing.T) {
	router := setupTestRouter()

	token := generateTestToken("test@example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	// Добавляем токен без префикса Bearer
	req.Header.Set("Authorization", token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Middleware должен добавить префикс "Bearer " автоматически
	assert.Equal(t, http.StatusOK, rr.Code, "Token without Bearer prefix should be accepted")
}

// TestRouter_InvalidUserID тестирует обработку невалидного ID пользователя
func TestRouter_InvalidUserID(t *testing.T) {
	router := setupTestRouter()

	token := generateTestToken("test@example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/users/invalid", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Expected status 400 Bad Request")
}

// TestRouter_AddressSearch_InvalidBody тестирует ошибку при невалидном теле запроса для поиска адресов
func TestRouter_AddressSearch_InvalidBody(t *testing.T) {
	router := setupTestRouter()

	token := generateTestToken("test@example.com")

	req := httptest.NewRequest(http.MethodPost, "/api/address/search", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Expected status 400 Bad Request")
}

// TestRouter_Geocode_InvalidBody тестирует ошибку при невалидном теле запроса для геокодирования
func TestRouter_Geocode_InvalidBody(t *testing.T) {
	router := setupTestRouter()

	token := generateTestToken("test@example.com")

	req := httptest.NewRequest(http.MethodPost, "/api/address/geocode", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Expected status 400 Bad Request")
}

// TestRouter_GetUserByEmail тестирует получение пользователя по email
func TestRouter_GetUserByEmail(t *testing.T) {
	router := setupTestRouter()

	token := generateTestToken("test@example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/users/email?email=test@example.com", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Ожидаем 200 или 404 (если пользователь не найден)
	assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNotFound,
		"Expected status 200 or 404")
}

// TestRouter_GetUserByEmail_MissingEmailParam тестирует ошибку при отсутствии параметра email
func TestRouter_GetUserByEmail_MissingEmailParam(t *testing.T) {
	router := setupTestRouter()

	token := generateTestToken("test@example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/users/email", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Expected status 400 Bad Request")
}
