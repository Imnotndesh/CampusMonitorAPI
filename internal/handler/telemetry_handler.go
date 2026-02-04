package handler

import (
	_ "encoding/json"
	"net/http"
	"strconv"
	"time"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/service"

	"github.com/gorilla/mux"
)

type TelemetryHandler struct {
	telemetryService *service.TelemetryService
	log              *logger.Logger
}

func NewTelemetryHandler(telemetryService *service.TelemetryService, log *logger.Logger) *TelemetryHandler {
	return &TelemetryHandler{
		telemetryService: telemetryService,
		log:              log,
	}
}

func (h *TelemetryHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/telemetry", h.QueryTelemetry).Methods("GET")
	r.HandleFunc("/telemetry/{probe_id}/latest", h.GetLatestTelemetry).Methods("GET")
	r.HandleFunc("/telemetry/{probe_id}/stats", h.GetProbeStats).Methods("GET")
}

func (h *TelemetryHandler) QueryTelemetry(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	req := &models.TelemetryQueryRequest{
		ProbeIDs: query["probe_id"],
		Type:     query.Get("type"),
		Limit:    100,
		Offset:   0,
	}

	if limit := query.Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			req.Limit = l
		}
	}

	if offset := query.Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			req.Offset = o
		}
	}

	if startTime := query.Get("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			req.StartTime = &t
		}
	}

	if endTime := query.Get("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			req.EndTime = &t
		}
	}

	response, err := h.telemetryService.GetTelemetry(r.Context(), req)
	if err != nil {
		h.log.Error("Failed to query telemetry: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *TelemetryHandler) GetLatestTelemetry(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["probe_id"]

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	telemetry, err := h.telemetryService.GetLatestTelemetry(r.Context(), probeID, limit)
	if err != nil {
		h.log.Error("Failed to get latest telemetry: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, telemetry)
}

func (h *TelemetryHandler) GetProbeStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["probe_id"]

	hours := 24
	if h := r.URL.Query().Get("hours"); h != "" {
		if parsed, err := strconv.Atoi(h); err == nil {
			hours = parsed
		}
	}

	stats, err := h.telemetryService.GetProbeStats(r.Context(), probeID, hours)
	if err != nil {
		h.log.Error("Failed to get probe stats: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, stats)
}
