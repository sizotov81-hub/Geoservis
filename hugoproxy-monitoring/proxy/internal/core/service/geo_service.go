package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ekomobile/dadata/v2/api/suggest"
	"github.com/ekomobile/dadata/v2/client"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/metrics"
)

// GeoServicer определяет интерфейс для работы с геоданными
type GeoServicer interface {
	AddressSearch(input string) ([]*Address, error)
	GeoCode(lat, lng string) ([]*Address, error)
}

// GeoService реализует GeoServicer
type GeoService struct {
	api       *suggest.Api
	apiKey    string
	secretKey string
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

// SearchRequest represents search request
// @Description Запрос для поиска адреса
type SearchRequest struct {
	Query string `json:"query" example:"Москва Ленина 11"` // Поисковый запрос (город, улица, дом)
}

// SearchResponse represents search response
// @Description Ответ с найденными адресами
type SearchResponse struct {
	Addresses []*Address `json:"addresses"` // Список найденных адресов
}

// GeocodeRequest represents geocode request
// @Description Запрос для геокодирования координат
type GeocodeRequest struct {
	Lat string `json:"lat" example:"55.7558"` // Широта
	Lng string `json:"lng" example:"37.6173"` // Долгота
}

// GeocodeResponse represents geocode response
// @Description Ответ с геокодированными адресами
type GeocodeResponse struct {
	Addresses []*Address `json:"addresses"` // Список найденных адресов
}

func (g *GeoService) AddressSearch(input string) ([]*Address, error) {
	var res []*Address
	start := time.Now()
	rawRes, err := g.api.Address(context.Background(), &suggest.RequestParams{Query: input})
	duration := time.Since(start)

	metrics.ObserveExternalAPIRequest("AddressSearch", duration)

	if err != nil {
		return nil, err
	}

	for _, r := range rawRes {
		if r.Data.City == "" || r.Data.Street == "" {
			continue
		}
		res = append(res, &Address{
			City:   r.Data.City,
			Street: r.Data.Street,
			House:  r.Data.House,
			Lat:    r.Data.GeoLat,
			Lon:    r.Data.GeoLon,
		})
	}
	return res, nil
}

func (g *GeoService) GeoCode(lat, lng string) ([]*Address, error) {
	start := time.Now()
	httpClient := &http.Client{}
	data := strings.NewReader(fmt.Sprintf(`{"lat": %s, "lon": %s}`, lat, lng))
	req, err := http.NewRequest("POST", "https://suggestions.dadata.ru/suggestions/api/4_1/rs/geolocate/address", data)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Token %s", g.apiKey))
	duration := time.Since(start)

	metrics.ObserveExternalAPIRequest("GeoCode", duration)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var geoCode GeoCode
	if err := json.NewDecoder(resp.Body).Decode(&geoCode); err != nil {
		return nil, err
	}

	var res []*Address
	for _, r := range geoCode.Suggestions {
		res = append(res, &Address{
			City:   string(r.Data.City),
			Street: string(r.Data.Street),
			House:  r.Data.House,
			Lat:    r.Data.GeoLat,
			Lon:    r.Data.GeoLon,
		})
	}
	return res, nil
}
