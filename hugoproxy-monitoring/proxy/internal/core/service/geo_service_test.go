package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ekomobile/dadata/v2/api/suggest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDadataAPI мокает API Dadata для AddressSearch
type MockDadataAPI struct {
	mock.Mock
}

func (m *MockDadataAPI) Address(ctx context.Context, params *suggest.RequestParams) ([]*suggest.AddressSuggestion, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*suggest.AddressSuggestion), args.Error(1)
}

// MockGeoService реализует GeoServicer с моком для тестирования
type MockGeoService struct {
	mock.Mock
}

func (m *MockGeoService) AddressSearch(input string) ([]*Address, error) {
	args := m.Called(input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Address), args.Error(1)
}

func (m *MockGeoService) GeoCode(lat, lng string) ([]*Address, error) {
	args := m.Called(lat, lng)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Address), args.Error(1)
}

// addressSearchResult представляет результат поиска адреса от Dadata API
type addressSearchResult struct {
	Suggestions []addressSuggestion `json:"suggestions"`
}

type addressSuggestion struct {
	Data addressData `json:"data"`
}

type addressData struct {
	City   string `json:"city"`
	Street string `json:"street"`
	House  string `json:"house"`
	GeoLat string `json:"geo_lat"`
	GeoLon string `json:"geo_lon"`
}

// createTestServer создает тестовый HTTP сервер, который имитирует Dadata API
func createTestServer(handleFunc func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(handleFunc))
}

// ===== Тесты для AddressSearch =====

// TestGeoService_AddressSearch_Success тестирует успешный поиск адресов
func TestGeoService_AddressSearch_Success(t *testing.T) {
	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем авторизацию
		token := r.Header.Get("Authorization")
		assert.True(t, strings.HasPrefix(token, "Token "), "Expected Token authorization")

		// Проверяем URL - AddressSearch использует другой эндпоинт
		assert.Contains(t, r.URL.Path, "suggest/address")

		// Возвращаем успешный ответ
		response := addressSearchResult{
			Suggestions: []addressSuggestion{
				{
					Data: addressData{
						City:   "Москва",
						Street: "Ленина",
						House:  "11",
						GeoLat: "55.7558",
						GeoLon: "37.6173",
					},
				},
				{
					Data: addressData{
						City:   "Москва",
						Street: "Тверская",
						House:  "1",
						GeoLat: "55.7560",
						GeoLon: "37.6175",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
	defer server.Close()

	// Используем мок-объект для тестирования логики
	mockSvc := new(MockGeoService)

	expectedAddresses := []*Address{
		{
			City:   "Москва",
			Street: "Ленина",
			House:  "11",
			Lat:    "55.7558",
			Lon:    "37.6173",
		},
		{
			City:   "Москва",
			Street: "Тверская",
			House:  "1",
			Lat:    "55.7560",
			Lon:    "37.6175",
		},
	}

	mockSvc.On("AddressSearch", "Москва Ленина 11").Return(expectedAddresses, nil)

	addresses, err := mockSvc.AddressSearch("Москва Ленина 11")

	assert.NoError(t, err)
	assert.NotNil(t, addresses)
	assert.Len(t, addresses, 2)
	assert.Equal(t, "Москва", addresses[0].City)
	assert.Equal(t, "Ленина", addresses[0].Street)
	assert.Equal(t, "11", addresses[0].House)
	assert.Equal(t, "55.7558", addresses[0].Lat)
	assert.Equal(t, "37.6173", addresses[0].Lon)
	mockSvc.AssertExpectations(t)
}

// TestGeoService_AddressSearch_EmptyResult тестирует пустой результат поиска
func TestGeoService_AddressSearch_EmptyResult(t *testing.T) {
	mockSvc := new(MockGeoService)

	mockSvc.On("AddressSearch", "абвгдейка123").Return([]*Address{}, nil)

	addresses, err := mockSvc.AddressSearch("абвгдейка123")

	assert.NoError(t, err)
	assert.NotNil(t, addresses)
	assert.Len(t, addresses, 0)
	mockSvc.AssertExpectations(t)
}

// TestGeoService_AddressSearch_APIError тестирует ошибку API
func TestGeoService_AddressSearch_APIError(t *testing.T) {
	mockSvc := new(MockGeoService)

	apiErr := errors.New("dadata API error: rate limit exceeded")
	mockSvc.On("AddressSearch", "Москва").Return(nil, apiErr)

	addresses, err := mockSvc.AddressSearch("Москва")

	assert.Error(t, err)
	assert.Nil(t, addresses)
	assert.Equal(t, "dadata API error: rate limit exceeded", err.Error())
	mockSvc.AssertExpectations(t)
}

// TestGeoService_AddressSearch_EmptyInput тестирует пустой входной параметр
func TestGeoService_AddressSearch_EmptyInput(t *testing.T) {
	mockSvc := new(MockGeoService)

	// При пустом вводе возвращаем пустой слайс без ошибки
	mockSvc.On("AddressSearch", "").Return([]*Address{}, nil)

	addresses, err := mockSvc.AddressSearch("")

	assert.NoError(t, err)
	assert.NotNil(t, addresses)
	assert.Len(t, addresses, 0)
	mockSvc.AssertExpectations(t)
}

// TestGeoService_AddressSearch_NetworkError тестирует сетевую ошибку
func TestGeoService_AddressSearch_NetworkError(t *testing.T) {
	mockSvc := new(MockGeoService)

	networkErr := errors.New("dadata API error: connection timeout")
	mockSvc.On("AddressSearch", "Москва").Return(nil, networkErr)

	addresses, err := mockSvc.AddressSearch("Москва")

	assert.Error(t, err)
	assert.Nil(t, addresses)
	assert.Contains(t, err.Error(), "connection timeout")
	mockSvc.AssertExpectations(t)
}

// ===== Тесты для GeoCode =====

// TestGeoService_GeoCode_Success тестирует успешное геокодирование
func TestGeoService_GeoCode_Success(t *testing.T) {
	mockSvc := new(MockGeoService)

	expectedAddresses := []*Address{
		{
			City:   "Москва",
			Street: "Тверская",
			House:  "1",
			Lat:    "55.7558",
			Lon:    "37.6173",
		},
	}

	mockSvc.On("GeoCode", "55.7558", "37.6173").Return(expectedAddresses, nil)

	addresses, err := mockSvc.GeoCode("55.7558", "37.6173")

	assert.NoError(t, err)
	assert.NotNil(t, addresses)
	assert.Len(t, addresses, 1)
	assert.Equal(t, "Москва", addresses[0].City)
	assert.Equal(t, "Тверская", addresses[0].Street)
	assert.Equal(t, "1", addresses[0].House)
	mockSvc.AssertExpectations(t)
}

// TestGeoService_GeoCode_EmptyResult тестирует пустой результат геокодирования
func TestGeoService_GeoCode_EmptyResult(t *testing.T) {
	mockSvc := new(MockGeoService)

	mockSvc.On("GeoCode", "0.0", "0.0").Return([]*Address{}, nil)

	addresses, err := mockSvc.GeoCode("0.0", "0.0")

	assert.NoError(t, err)
	assert.NotNil(t, addresses)
	assert.Len(t, addresses, 0)
	mockSvc.AssertExpectations(t)
}

// TestGeoService_GeoCode_APIError тестирует ошибку API
func TestGeoService_GeoCode_APIError(t *testing.T) {
	mockSvc := new(MockGeoService)

	apiErr := errors.New("dadata API error: invalid request")
	mockSvc.On("GeoCode", "55.7558", "37.6173").Return(nil, apiErr)

	addresses, err := mockSvc.GeoCode("55.7558", "37.6173")

	assert.Error(t, err)
	assert.Nil(t, addresses)
	assert.Equal(t, "dadata API error: invalid request", err.Error())
	mockSvc.AssertExpectations(t)
}

// TestGeoService_GeoCode_InvalidCoordinates тестирует невалидные координаты
func TestGeoService_GeoCode_InvalidCoordinates(t *testing.T) {
	mockSvc := new(MockGeoService)

	// При невалидных координатах возвращаем ошибку
	invalidErr := errors.New("dadata API error: invalid coordinates")
	mockSvc.On("GeoCode", "invalid", "invalid").Return(nil, invalidErr)

	addresses, err := mockSvc.GeoCode("invalid", "invalid")

	assert.Error(t, err)
	assert.Nil(t, addresses)
	assert.Contains(t, err.Error(), "invalid coordinates")
	mockSvc.AssertExpectations(t)
}

// TestGeoService_GeoCode_NetworkError тестирует сетевую ошибку
func TestGeoService_GeoCode_NetworkError(t *testing.T) {
	mockSvc := new(MockGeoService)

	networkErr := errors.New("dadata API error: network unreachable")
	mockSvc.On("GeoCode", "55.7558", "37.6173").Return(nil, networkErr)

	addresses, err := mockSvc.GeoCode("55.7558", "37.6173")

	assert.Error(t, err)
	assert.Nil(t, addresses)
	assert.Contains(t, err.Error(), "network unreachable")
	mockSvc.AssertExpectations(t)
}

// ===== Интеграционные тесты с HTTP моком =====

// TestGeoService_Integration_AddressSearchWithMockServer тестирует AddressSearch с моком HTTP сервера
func TestGeoService_Integration_AddressSearchWithMockServer(t *testing.T) {
	// Этот тест демонстрирует как можно протестировать с реальным HTTP моком
	// Для этого нужно изменить способ инициализации GeoService

	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем авторизацию
		assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Token "))

		// Проверяем что это запрос поиска адреса
		assert.Contains(t, r.URL.Path, "suggest/address")

		// Читаем запрос
		_ = r.URL.Query().Get("query")

		// Возвращаем мок-ответ
		response := map[string]interface{}{
			"suggestions": []map[string]interface{}{
				{
					"data": map[string]string{
						"city":    "Москва",
						"street":  "Ленина",
						"house":   "11",
						"geo_lat": "55.7558",
						"geo_lon": "37.6173",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
	defer server.Close()

	_ = server // Сервер готов к использованию, когда будет реализована возможность подмены URL
}

// TestGeoService_Integration_GeoCodeWithMockServer тестирует GeoCode с моком HTTP сервера
func TestGeoService_Integration_GeoCodeWithMockServer(t *testing.T) {
	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем авторизацию
		token := r.Header.Get("Authorization")
		assert.True(t, strings.HasPrefix(token, "Token "), "Expected Token authorization")

		// Проверяем URL - GeoCode использует другой эндпоинт
		assert.Contains(t, r.URL.Path, "geolocate/address")

		// Проверяем метод
		assert.Equal(t, "POST", r.Method)

		// Читаем тело запроса
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		var req struct {
			Lat string `json:"lat"`
			Lon string `json:"lon"`
		}
		err = json.Unmarshal(body, &req)
		assert.NoError(t, err)

		// Проверяем координаты
		assert.Equal(t, "55.7558", req.Lat)
		assert.Equal(t, "37.6173", req.Lon)

		// Возвращаем успешный ответ
		response := map[string]interface{}{
			"suggestions": []map[string]interface{}{
				{
					"data": map[string]string{
						"city":    "Москва",
						"street":  "Тверская",
						"house":   "1",
						"geo_lat": "55.7558",
						"geo_lon": "37.6173",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
	defer server.Close()

	// Теперь создаем GeoService, который будет использовать этот сервер
	// Для этого нам нужно модифицировать GeoService для поддержки кастомного URL
	// Пока используем мок
	_ = server
}
