package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ekomobile/dadata/v2/api/suggest"
	"github.com/ekomobile/dadata/v2/client"
	"go.uber.org/zap"

	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/logger"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/metrics"
)

// GeoServicer определяет интерфейс для работы с геоданными
type GeoServicer interface {
	AddressSearch(ctx context.Context, input string) ([]*Address, error)
	GeoCode(ctx context.Context, lat, lng string) ([]*Address, error)
}

// GeoService реализует GeoServicer
type GeoService struct {
	api       *suggest.Api
	apiKey    string
	secretKey string
	httpClient *http.Client
}

// NewGeoService создает новый экземпляр GeoService
func NewGeoService(apiKey, secretKey string) *GeoService {
	endpointUrl, _ := url.Parse("https://suggestions.dadata.ru/suggestions/api/4_1/rs/")
	creds := client.Credentials{
		ApiKeyValue:    apiKey,
		SecretKeyValue: secretKey,
	}
	api := suggest.Api{
		Client: client.NewClient(endpointUrl, client.WithCredentialProvider(&creds)),
	}
	return &GeoService{
		api:       &api,
		apiKey:    apiKey,
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Address представляет информацию об адресе
// @Description Информация об адресе
type Address struct {
	City   string `json:"city" example:"Москва"`   // Название города
	Street string `json:"street" example:"Ленина"` // Название улицы
	House  string `json:"house" example:"11"`      // Номер дома
	Lat    string `json:"lat" example:"55.7558"`   // Широта
	Lon    string `json:"lon" example:"37.6173"`   // Долгота
}

// SearchRequest представляет запрос для поиска адреса
// @Description Запрос для поиска адреса
type SearchRequest struct {
	Query string `json:"query" example:"Москва Ленина 11"` // Поисковый запрос (город, улица, дом)
}

// SearchResponse представляет ответ с найденными адресами
// @Description Ответ с найденными адресами
type SearchResponse struct {
	Addresses []*Address `json:"addresses"` // Список найденных адресов
}

// GeocodeRequest представляет запрос для геокодирования
// @Description Запрос для геокодирования координат
type GeocodeRequest struct {
	Lat string `json:"lat" example:"55.7558"` // Широта
	Lng string `json:"lng" example:"37.6173"` // Долгота
}

// geocodeResponse представляет ответ от API геокодирования
type geocodeResponse struct {
	Suggestions []geocodeSuggestion `json:"suggestions"`
}

// geocodeSuggestion представляет один результат геокодирования
type geocodeSuggestion struct {
	Data geocodeData `json:"data"`
}

// geocodeData содержит данные адреса
type geocodeData struct {
	City   string `json:"city"`
	Street string `json:"street"`
	House  string `json:"house"`
	GeoLat string `json:"geo_lat"`
	GeoLon string `json:"geo_lon"`
}

// GeocodeResponse представляет ответ с геокодированными адресами
// @Description Ответ с найденными адресами
type GeocodeResponse struct {
	Addresses []*Address `json:"addresses"` // Список найденных адресов
}

// AddressSearch выполняет поиск адреса по строке запроса
func (g *GeoService) AddressSearch(ctx context.Context, input string) ([]*Address, error) {
	start := time.Now()
	log := logger.FromContext(ctx)

	rawRes, err := g.api.Address(ctx, &suggest.RequestParams{Query: input})
	duration := time.Since(start)

	metrics.ObserveExternalAPIRequest("AddressSearch", duration)

	if err != nil {
		log.Error("AddressSearch failed", zap.String("query", input), zap.Error(err), zap.Duration("duration", duration))
		return nil, fmt.Errorf("external API error: %w", err)
	}

	log.Debug("AddressSearch success", zap.String("query", input), zap.Int("results", len(rawRes)), zap.Duration("duration", duration))

	addresses := make([]*Address, 0, len(rawRes))
	for _, r := range rawRes {
		if r.Data.City == "" || r.Data.Street == "" {
			continue
		}
		addresses = append(addresses, &Address{
			City:   r.Data.City,
			Street: r.Data.Street,
			House:  r.Data.House,
			Lat:    r.Data.GeoLat,
			Lon:    r.Data.GeoLon,
		})
	}

	return addresses, nil
}

// GeoCode выполняет геокодирование координат
func (g *GeoService) GeoCode(ctx context.Context, lat, lng string) ([]*Address, error) {
	start := time.Now()
	log := logger.FromContext(ctx)

	reqBody := fmt.Sprintf(`{"lat": %s, "lon": %s}`, lat, lng)
	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://suggestions.dadata.ru/suggestions/api/4_1/rs/geolocate/address",
		bytes.NewReader([]byte(reqBody)))
	if err != nil {
		log.Error("GeoCode: failed to create request", zap.String("lat", lat), zap.String("lng", lng), zap.Error(err))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Token %s", g.apiKey))

	resp, err := g.httpClient.Do(req)
	if err != nil {
		log.Error("GeoCode: failed to execute request", zap.String("lat", lat), zap.String("lng", lng), zap.Error(err))
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Error("GeoCode: unexpected status code", zap.String("lat", lat), zap.String("lng", lng), zap.Int("status", resp.StatusCode), zap.String("body", string(body)))
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var geoResp geocodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&geoResp); err != nil {
		log.Error("GeoCode: failed to decode response", zap.String("lat", lat), zap.String("lng", lng), zap.Error(err))
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	addresses := make([]*Address, 0, len(geoResp.Suggestions))
	for _, s := range geoResp.Suggestions {
		addresses = append(addresses, &Address{
			City:   s.Data.City,
			Street: s.Data.Street,
			House:  s.Data.House,
			Lat:    s.Data.GeoLat,
			Lon:    s.Data.GeoLon,
		})
	}

	duration := time.Since(start)
	metrics.ObserveExternalAPIRequest("GeoCode", duration)
	log.Debug("GeoCode success", zap.String("lat", lat), zap.String("lng", lng), zap.Int("results", len(addresses)), zap.Duration("duration", duration))

	return addresses, nil
}
