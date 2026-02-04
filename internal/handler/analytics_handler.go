// internal/handler/analytics_handler.go

package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/service"

	"github.com/gorilla/mux"
)

type AnalyticsHandler struct {
	analyticsService *service.AnalyticsService
	log              *logger.Logger
}

func NewAnalyticsHandler(analyticsService *service.AnalyticsService, log *logger.Logger) *AnalyticsHandler {
	return &AnalyticsHandler{
		analyticsService: analyticsService,
		log:              log,
	}
}

func (h *AnalyticsHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/analytics/timeseries/rssi", h.GetRSSITimeSeries).Methods("GET")
	r.HandleFunc("/analytics/timeseries/latency", h.GetLatencyTimeSeries).Methods("GET")
	r.HandleFunc("/analytics/heatmap", h.GetHeatmap).Methods("GET")
	r.HandleFunc("/analytics/channels", h.GetChannelDistribution).Methods("GET")
	r.HandleFunc("/analytics/aps", h.GetAPAnalysis).Methods("GET")
	r.HandleFunc("/analytics/congestion", h.GetCongestionAnalysis).Methods("GET")
	r.HandleFunc("/analytics/performance/{probe_id}", h.GetPerformanceMetrics).Methods("GET")
	r.HandleFunc("/analytics/comparison", h.GetProbeComparison).Methods("GET")
	r.HandleFunc("/analytics/health", h.GetNetworkHealth).Methods("GET")
	r.HandleFunc("/analytics/anomalies/{probe_id}", h.DetectAnomalies).Methods("GET")
	r.HandleFunc("/analytics/roaming/{probe_id}", h.GetRoamingAnalysis).Methods("GET")
}

func (h *AnalyticsHandler) GetRSSITimeSeries(w http.ResponseWriter, r *http.Request) {
	probeID := r.URL.Query().Get("probe_id")
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "5 minutes"
	}

	start, end := parseTimeRange(r)

	data, err := h.analyticsService.GetRSSITimeSeries(r.Context(), probeID, start, end, interval)
	if err != nil {
		h.log.Error("Failed to get RSSI time series: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, data)
}

func (h *AnalyticsHandler) GetLatencyTimeSeries(w http.ResponseWriter, r *http.Request) {
	probeID := r.URL.Query().Get("probe_id")
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "5 minutes"
	}

	start, end := parseTimeRange(r)

	data, err := h.analyticsService.GetLatencyTimeSeries(r.Context(), probeID, start, end, interval)
	if err != nil {
		h.log.Error("Failed to get latency time series: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, data)
}

func (h *AnalyticsHandler) GetHeatmap(w http.ResponseWriter, r *http.Request) {
	start, end := parseTimeRange(r)

	data, err := h.analyticsService.GetHeatmapData(r.Context(), start, end)
	if err != nil {
		h.log.Error("Failed to get heatmap data: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, data)
}

func (h *AnalyticsHandler) GetChannelDistribution(w http.ResponseWriter, r *http.Request) {
	start, end := parseTimeRange(r)

	data, err := h.analyticsService.GetChannelDistribution(r.Context(), start, end)
	if err != nil {
		h.log.Error("Failed to get channel distribution: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, data)
}

func (h *AnalyticsHandler) GetAPAnalysis(w http.ResponseWriter, r *http.Request) {
	start, end := parseTimeRange(r)

	data, err := h.analyticsService.GetAPAnalysis(r.Context(), start, end)
	if err != nil {
		h.log.Error("Failed to get AP analysis: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, data)
}

func (h *AnalyticsHandler) GetCongestionAnalysis(w http.ResponseWriter, r *http.Request) {
	start, end := parseTimeRange(r)

	data, err := h.analyticsService.GetCongestionAnalysis(r.Context(), start, end)
	if err != nil {
		h.log.Error("Failed to get congestion analysis: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, data)
}

func (h *AnalyticsHandler) GetPerformanceMetrics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["probe_id"]

	start, end := parseTimeRange(r)

	data, err := h.analyticsService.GetPerformanceMetrics(r.Context(), probeID, start, end)
	if err != nil {
		h.log.Error("Failed to get performance metrics: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, data)
}

func (h *AnalyticsHandler) GetProbeComparison(w http.ResponseWriter, r *http.Request) {
	probeIDsStr := r.URL.Query().Get("probe_ids")
	probeIDs := strings.Split(probeIDsStr, ",")

	start, end := parseTimeRange(r)

	data, err := h.analyticsService.GetProbeComparison(r.Context(), probeIDs, start, end)
	if err != nil {
		h.log.Error("Failed to get probe comparison: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, data)
}

func (h *AnalyticsHandler) GetNetworkHealth(w http.ResponseWriter, r *http.Request) {
	data, err := h.analyticsService.GetNetworkHealth(r.Context())
	if err != nil {
		h.log.Error("Failed to get network health: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, data)
}

func (h *AnalyticsHandler) DetectAnomalies(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["probe_id"]

	hours := 24
	if h := r.URL.Query().Get("hours"); h != "" {
		if parsed, err := strconv.Atoi(h); err == nil {
			hours = parsed
		}
	}

	data, err := h.analyticsService.DetectAnomalies(r.Context(), probeID, hours)
	if err != nil {
		h.log.Error("Failed to detect anomalies: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, data)
}

func (h *AnalyticsHandler) GetRoamingAnalysis(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["probe_id"]

	start, end := parseTimeRange(r)

	data, err := h.analyticsService.GetRoamingAnalysis(r.Context(), probeID, start, end)
	if err != nil {
		h.log.Error("Failed to get roaming analysis: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, data)
}

func parseTimeRange(r *http.Request) (time.Time, time.Time) {
	end := time.Now()
	start := end.Add(-24 * time.Hour)

	if startStr := r.URL.Query().Get("start_time"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			start = t
		}
	}

	if endStr := r.URL.Query().Get("end_time"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			end = t
		}
	}

	return start, end
}
