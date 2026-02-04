package handler

import (
	"context"
	"net/http"
	"time"

	"CampusMonitorAPI/internal/database"
	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/mqtt"

	"github.com/gorilla/mux"
)

type HealthHandler struct {
	db         *database.Database
	mqttClient *mqtt.Client
	log        *logger.Logger
}

func NewHealthHandler(db *database.Database, mqttClient *mqtt.Client, log *logger.Logger) *HealthHandler {
	return &HealthHandler{
		db:         db,
		mqttClient: mqttClient,
		log:        log,
	}
}

func (h *HealthHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/health", h.Health).Methods("GET")
	r.HandleFunc("/health/live", h.Liveness).Methods("GET")
	r.HandleFunc("/health/ready", h.Readiness).Methods("GET")
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	response := models.HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
	}

	dbErr := h.db.Health(ctx)
	response.Services.Database = (dbErr == nil)

	mqttHealth, mqttErr := h.mqttClient.Health(ctx)
	response.Services.MQTT = (mqttErr == nil && mqttHealth.Connected)

	if !response.Services.Database || !response.Services.MQTT {
		response.Status = "degraded"
		h.log.Warn("Health check degraded - DB: %v, MQTT: %v", response.Services.Database, response.Services.MQTT)
	}

	statusCode := http.StatusOK
	if response.Status == "degraded" {
		statusCode = http.StatusServiceUnavailable
	}

	respondJSON(w, statusCode, response)
}

func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status": "alive",
	})
}

func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	dbErr := h.db.Health(ctx)
	mqttConnected := h.mqttClient.IsConnected()

	if dbErr != nil || !mqttConnected {
		h.log.Warn("Readiness check failed - DB error: %v, MQTT connected: %v", dbErr, mqttConnected)
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not ready",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status": "ready",
	})
}
