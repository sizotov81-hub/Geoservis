package handler

import (
	"net/http"

	"go.uber.org/zap"

	"gitlab.com/s.izotov81/hugoproxy/internal/infrastructure/logger"
	"gitlab.com/s.izotov81/hugoproxy/internal/usecase/geo"
	"gitlab.com/s.izotov81/hugoproxy/pkg/responder"
)

// GeoController обрабатывает запросы, связанные с геоданными
type GeoController struct {
	geoService geo.GeoServicer
	responder  responder.Responder
}

// NewGeoController создает новый экземпляр GeoController
func NewGeoController(geoService geo.GeoServicer, responder responder.Responder) *GeoController {
	return &GeoController{
		geoService: geoService,
		responder:  responder,
	}
}

// Search обрабатывает запрос на поиск адреса
func (c *GeoController) Search(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())

	var req geo.SearchRequest
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
	c.responder.Respond(w, http.StatusOK, geo.SearchResponse{Addresses: addresses})
}

// Geocode обрабатывает запрос на геокодирование
func (c *GeoController) Geocode(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())

	var req geo.GeocodeRequest
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
	c.responder.Respond(w, http.StatusOK, geo.GeocodeResponse{Addresses: addresses})
}
