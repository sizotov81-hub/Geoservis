package geo_proxy

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.com/s.izotov81/hugoproxy/internal/core/service"
)

// MockGeoService мок геосервиса для тестирования
type MockGeoService struct {
	mock.Mock
}

func (m *MockGeoService) AddressSearch(ctx context.Context, input string) ([]*service.Address, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*service.Address), args.Error(1)
}

func (m *MockGeoService) GeoCode(ctx context.Context, lat, lng string) ([]*service.Address, error) {
	args := m.Called(ctx, lat, lng)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*service.Address), args.Error(1)
}

// MockCache мок кэша для тестирования
type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(key string) (interface{}, bool) {
	args := m.Called(key)
	return args.Get(0), args.Bool(1)
}

func (m *MockCache) Set(key string, value interface{}, ttl time.Duration) {
	m.Called(key, value, ttl)
}

func (m *MockCache) Delete(key string) {
	m.Called(key)
}

func TestGeoServiceProxy_AddressSearch_CacheHit(t *testing.T) {
	mockService := new(MockGeoService)
	mockCache := new(MockCache)
	proxy := NewGeoServiceProxy(mockService, mockCache, 5*time.Minute)

	// Ожидаемый результат
	expected := []*service.Address{{City: "Moscow"}}

	// Настройка моков
	mockCache.On("Get", "search:query").Return(expected, true).Once()
	// Сервис не должен вызываться, так как данные в кэше

	result, err := proxy.AddressSearch(context.Background(), "query")
	assert.NoError(t, err)
	assert.Equal(t, expected, result)

	mockCache.AssertExpectations(t)
	mockService.AssertExpectations(t)
}

func TestGeoServiceProxy_AddressSearch_CacheMiss(t *testing.T) {
	mockService := new(MockGeoService)
	mockCache := new(MockCache)
	proxy := NewGeoServiceProxy(mockService, mockCache, 5*time.Minute)

	// Ожидаемый результат
	expected := []*service.Address{{City: "Moscow"}}

	// Настройка моков
	mockCache.On("Get", "search:query").Return(nil, false).Once()
	mockService.On("AddressSearch", mock.Anything, "query").Return(expected, nil).Once()
	mockCache.On("Set", "search:query", expected, 5*time.Minute).Once()

	result, err := proxy.AddressSearch(context.Background(), "query")
	assert.NoError(t, err)
	assert.Equal(t, expected, result)

	mockCache.AssertExpectations(t)
	mockService.AssertExpectations(t)
}

func TestGeoServiceProxy_AddressSearch_ServiceError(t *testing.T) {
	mockService := new(MockGeoService)
	mockCache := new(MockCache)
	proxy := NewGeoServiceProxy(mockService, mockCache, 5*time.Minute)

	// Ожидаемая ошибка
	expectedError := errors.New("service error")

	// Настройка моков
	mockCache.On("Get", "search:query").Return(nil, false).Once()
	mockService.On("AddressSearch", mock.Anything, "query").Return([]*service.Address(nil), expectedError).Once()
	// Set не должен вызываться при ошибке

	result, err := proxy.AddressSearch(context.Background(), "query")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to search addresses from geo service")
	assert.Nil(t, result)

	mockCache.AssertExpectations(t)
	mockService.AssertExpectations(t)
}

func TestGeoServiceProxy_GeoCode_CacheHit(t *testing.T) {
	mockService := new(MockGeoService)
	mockCache := new(MockCache)
	proxy := NewGeoServiceProxy(mockService, mockCache, 5*time.Minute)

	// Ожидаемый результат
	expected := []*service.Address{{City: "Moscow"}}

	// Настройка моков
	mockCache.On("Get", "geocode:55.7558:37.6173").Return(expected, true).Once()
	// Сервис не должен вызываться, так как данные в кэше

	result, err := proxy.GeoCode(context.Background(), "55.7558", "37.6173")
	assert.NoError(t, err)
	assert.Equal(t, expected, result)

	mockCache.AssertExpectations(t)
	mockService.AssertExpectations(t)
}

func TestGeoServiceProxy_GeoCode_CacheMiss(t *testing.T) {
	mockService := new(MockGeoService)
	mockCache := new(MockCache)
	proxy := NewGeoServiceProxy(mockService, mockCache, 5*time.Minute)

	// Ожидаемый результат
	expected := []*service.Address{{City: "Moscow"}}

	// Настройка моков
	mockCache.On("Get", "geocode:55.7558:37.6173").Return(nil, false).Once()
	mockService.On("GeoCode", mock.Anything, "55.7558", "37.6173").Return(expected, nil).Once()
	mockCache.On("Set", "geocode:55.7558:37.6173", expected, 5*time.Minute).Once()

	result, err := proxy.GeoCode(context.Background(), "55.7558", "37.6173")
	assert.NoError(t, err)
	assert.Equal(t, expected, result)

	mockCache.AssertExpectations(t)
	mockService.AssertExpectations(t)
}

func TestGeoServiceProxy_GeoCode_ServiceError(t *testing.T) {
	mockService := new(MockGeoService)
	mockCache := new(MockCache)
	proxy := NewGeoServiceProxy(mockService, mockCache, 5*time.Minute)

	// Ожидаемая ошибка
	expectedError := errors.New("service error")

	// Настройка моков
	mockCache.On("Get", "geocode:55.7558:37.6173").Return(nil, false).Once()
	mockService.On("GeoCode", mock.Anything, "55.7558", "37.6173").Return([]*service.Address(nil), expectedError).Once()

	result, err := proxy.GeoCode(context.Background(), "55.7558", "37.6173")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to geocode from geo service")
	assert.Nil(t, result)

	mockCache.AssertExpectations(t)
	mockService.AssertExpectations(t)
}
