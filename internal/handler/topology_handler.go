package handler

import (
	"net/http"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/service"

	"github.com/gorilla/mux"
)

type TopologyHandler struct {
	topologyService service.ITopologyService
	log             *logger.Logger
}

func NewTopologyHandler(topologyService service.ITopologyService, log *logger.Logger) *TopologyHandler {
	return &TopologyHandler{
		topologyService: topologyService,
		log:             log,
	}
}

func (h *TopologyHandler) RegisterRoutes(r *mux.Router) {
	// e.g. GET /api/v1/topology/layout
	r.HandleFunc("/topology/layout", h.GetLayout).Methods("GET")

	// e.g. GET /api/v1/topology/heatmap?metric=signal
	r.HandleFunc("/topology/heatmap", h.GetHeatmap).Methods("GET")

	// e.g. GET /api/v1/topology/building/LIB-01/floor/2
	r.HandleFunc("/topology/building/{building}/floor/{floor}", h.GetFloorDetails).Methods("GET")
}

func (h *TopologyHandler) GetLayout(w http.ResponseWriter, r *http.Request) {
	layout, err := h.topologyService.GetLayout(r.Context())
	if err != nil {
		h.log.Error("Failed to get topology layout: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to calculate topology layout")
		return
	}

	respondJSON(w, http.StatusOK, layout)
}

func (h *TopologyHandler) GetHeatmap(w http.ResponseWriter, r *http.Request) {
	// Determine which metric to colorize (default to signal/rssi)
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "rssi"
	}

	heatmap, err := h.topologyService.GetHeatmap(r.Context(), metric)
	if err != nil {
		h.log.Error("Failed to get topology heatmap: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to calculate heatmap")
		return
	}

	respondJSON(w, http.StatusOK, heatmap)
}

func (h *TopologyHandler) GetFloorDetails(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	building := vars["building"]
	floor := vars["floor"]

	details, err := h.topologyService.GetFloorDetails(r.Context(), building, floor)
	if err != nil {
		h.log.Error("Failed to get floor details for building %s, floor %s: %v", building, floor, err)
		respondError(w, http.StatusInternalServerError, "Failed to fetch floor details")
		return
	}

	respondJSON(w, http.StatusOK, details)
}
