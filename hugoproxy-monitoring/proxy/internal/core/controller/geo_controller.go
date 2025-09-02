package controller

import (
	"net/http"

	"gitlab.com/s.izotov81/hugoproxy/internal/core/service"
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
	var req service.SearchRequest
	if err := c.responder.Decode(r, &req); err != nil {
		c.responder.Error(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	addresses, err := c.geoService.AddressSearch(req.Query)
	if err != nil {
		c.responder.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}

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
	var req service.GeocodeRequest
	if err := c.responder.Decode(r, &req); err != nil {
		c.responder.Error(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	addresses, err := c.geoService.GeoCode(req.Lat, req.Lng)
	if err != nil {
		c.responder.Error(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	c.responder.Respond(w, http.StatusOK, service.GeocodeResponse{Addresses: addresses})
}
