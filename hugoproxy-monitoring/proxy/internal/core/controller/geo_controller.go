package controller

import (
	"net/http"

	"go.uber.org/zap"

	"gitlab.com/s.izotov81/hugoproxy/internal/core/service"
	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/logger"
	"gitlab.com/s.izotov81/hugoproxy/pkg/responder"
)

// GeoController обрабатывает запросы, связанные с геоданными
type GeoController struct {
	geoService service.GeoServicer
	responder  responder.Responder
}

// NewGeoController создает новый экземпляр GeoController
func NewGeoController(geoService service.GeoServicer, responder responder.Responder) *GeoController {
	return &GeoController{
		geoService: geoService,
		responder:  responder,
	}
}

// Search обрабатывает запрос на поиск адреса
// @Summary Поиск адреса
// @Description Поиск адреса по строке запроса
// @Tags address
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body service.SearchRequest true "Поисковый запрос"
// @Success 200 {object} service.SearchResponse "Успешный ответ с найденными адресами"
// @Failure 400 {object} responder.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} responder.ErrorResponse "Не авторизован"
// @Failure 500 {object} responder.ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/address/search [post]
func (c *GeoController) Search(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())

	var req service.SearchRequest
	if err := c.responder.Decode(r, &req); err != nil {
		log.Warn("Search: invalid request format", zap.Error(err))
		c.responder.Error(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	if req.Query == "" {
		log.Warn("Search: query parameter is required")
		c.responder.Error(w, http.StatusBadRequest, "Query parameter is required")
		return
	}

	addresses, err := c.geoService.AddressSearch(r.Context(), req.Query)
	if err != nil {
		log.Error("Search: failed to search addresses", zap.String("query", req.Query), zap.Error(err))
		c.responder.Error(w, http.StatusInternalServerError, "Failed to search addresses")
		return
	}

	log.Info("Search: success", zap.String("query", req.Query), zap.Int("results", len(addresses)))
	c.responder.Respond(w, http.StatusOK, service.SearchResponse{Addresses: addresses})
}

// Geocode обрабатывает запрос на геокодирование
// @Summary Геокодирование адреса
// @Description Получение адреса по координатам
// @Tags address
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body service.GeocodeRequest true "Координаты для геокодирования"
// @Success 200 {object} service.GeocodeResponse "Успешный ответ с найденными адресами"
// @Failure 400 {object} responder.ErrorResponse "Некорректный запрос"
// @Failure 401 {object} responder.ErrorResponse "Не авторизован"
// @Failure 500 {object} responder.ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/address/geocode [post]
func (c *GeoController) Geocode(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())

	var req service.GeocodeRequest
	if err := c.responder.Decode(r, &req); err != nil {
		log.Warn("Geocode: invalid request format", zap.Error(err))
		c.responder.Error(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	if req.Lat == "" || req.Lng == "" {
		log.Warn("Geocode: lat and lng parameters are required")
		c.responder.Error(w, http.StatusBadRequest, "Lat and Lng parameters are required")
		return
	}

	addresses, err := c.geoService.GeoCode(r.Context(), req.Lat, req.Lng)
	if err != nil {
		log.Error("Geocode: failed to geocode coordinates", zap.String("lat", req.Lat), zap.String("lng", req.Lng), zap.Error(err))
		c.responder.Error(w, http.StatusInternalServerError, "Failed to geocode coordinates")
		return
	}

	log.Info("Geocode: success", zap.String("lat", req.Lat), zap.String("lng", req.Lng), zap.Int("results", len(addresses)))
	c.responder.Respond(w, http.StatusOK, service.GeocodeResponse{Addresses: addresses})
}
