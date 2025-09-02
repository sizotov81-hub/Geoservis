package geo_proxy

import (
	"log"
	"time"

	"gitlab.com/s.izotov81/hugoproxy/internal/core/service"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/cache"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/metrics"
)

// GeoServiceProxy прокси для кэширования запросов к геосервису
type GeoServiceProxy struct {
	geoService service.GeoServicer
	cache      cache.Cache
	ttl        time.Duration
}

// NewGeoServiceProxy создает новый экземпляр прокси
func NewGeoServiceProxy(geoService service.GeoServicer, cache cache.Cache, ttl time.Duration) *GeoServiceProxy {
	return &GeoServiceProxy{
		geoService: geoService,
		cache:      cache,
		ttl:        ttl,
	}
}

// AddressSearch ищет адреса с использованием кэширования
func (p *GeoServiceProxy) AddressSearch(input string) ([]*service.Address, error) {
	cacheKey := "search:" + input

	// Попытка получить данные из кэша
	start := time.Now()
	cached, found := p.cache.Get(cacheKey)
	cacheDuration := time.Since(start)

	metrics.ObserveCacheRequest("AddressSearch", found, cacheDuration)

	if found {
		return cached.([]*service.Address), nil
	}

	// Если данных нет в кэше, запрашиваем у оригинального сервиса
	data, err := p.geoService.AddressSearch(input)
	if err != nil {
		return nil, err
	}

	// Сохраняем результат в кэш
	start = time.Now()
	p.cache.Set(cacheKey, data, p.ttl)
	cacheDuration = time.Since(start)
	metrics.ObserveCacheRequest("AddressSearch_Set", true, cacheDuration)

	return data, nil
}

// GeoCode выполняет геокодирование с использованием кэширования
func (p *GeoServiceProxy) GeoCode(lat, lng string) ([]*service.Address, error) {
	cacheKey := "geocode:" + lat + ":" + lng

	// Попытка получить данные из кэша
	start := time.Now()
	cached, found := p.cache.Get(cacheKey)
	cacheDuration := time.Since(start)

	metrics.ObserveCacheRequest("GeoCode", found, cacheDuration)

	if found {
		return cached.([]*service.Address), nil
	}

	log.Printf("Cache MISS for key: %s", cacheKey)

	// Если данных нет в кэше, запрашиваем у оригинального сервиса
	data, err := p.geoService.GeoCode(lat, lng)
	if err != nil {
		return nil, err
	}

	// Сохраняем результат в кэш
	start = time.Now()
	p.cache.Set(cacheKey, data, p.ttl)
	cacheDuration = time.Since(start)
	metrics.ObserveCacheRequest("GeoCode_Set", true, cacheDuration)
	return data, nil
}
