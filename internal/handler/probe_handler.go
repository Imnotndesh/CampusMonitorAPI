package handler

import (
	"encoding/json"
	"net/http"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/service"

	"github.com/gorilla/mux"
)

type ProbeHandler struct {
	probeService   *service.ProbeService
	commandService *service.CommandService
	log            *logger.Logger
}

func NewProbeHandler(
	probeService *service.ProbeService,
	commandService *service.CommandService,
	log *logger.Logger,
) *ProbeHandler {
	return &ProbeHandler{
		probeService:   probeService,
		commandService: commandService,
		log:            log,
	}
}

func (h *ProbeHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/probes", h.CreateProbe).Methods("POST")
	r.HandleFunc("/probes", h.ListProbes).Methods("GET")
	r.HandleFunc("/probes/{id}", h.GetProbe).Methods("GET")
	r.HandleFunc("/probes/{id}", h.UpdateProbe).Methods("PUT", "PATCH")
	r.HandleFunc("/probes/{id}", h.DeleteProbe).Methods("DELETE")
	r.HandleFunc("/probes/{id}/command", h.SendCommand).Methods("POST")
	r.HandleFunc("/probes/{id}/adopt", h.AdoptProbe).Methods("POST")
	r.HandleFunc("/probes/active", h.GetActiveProbes).Methods("GET")
	r.HandleFunc("/probes/building/{building}", h.GetProbesByBuilding).Methods("GET")
}

func (h *ProbeHandler) CreateProbe(w http.ResponseWriter, r *http.Request) {
	var req models.CreateProbeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("Invalid request body: %v", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	probe, err := h.probeService.RegisterProbe(r.Context(), &req)
	if err != nil {
		h.log.Error("Failed to create probe: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, probe)
}

func (h *ProbeHandler) ListProbes(w http.ResponseWriter, r *http.Request) {
	probes, err := h.probeService.ListProbes(r.Context())
	if err != nil {
		h.log.Error("Failed to list probes: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, probes)
}

func (h *ProbeHandler) GetProbe(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["id"]

	probe, err := h.probeService.GetProbe(r.Context(), probeID)
	if err != nil {
		h.log.Error("Failed to get probe: %v", err)
		respondError(w, http.StatusNotFound, "Probe not found")
		return
	}

	respondJSON(w, http.StatusOK, probe)
}

func (h *ProbeHandler) UpdateProbe(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["id"]

	var req models.UpdateProbeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("Invalid request body: %v", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	probe, err := h.probeService.UpdateProbe(r.Context(), probeID, &req)
	if err != nil {
		h.log.Error("Failed to update probe: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, probe)
}

func (h *ProbeHandler) DeleteProbe(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["id"]

	if err := h.probeService.DeleteProbe(r.Context(), probeID); err != nil {
		h.log.Error("Failed to delete probe: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Probe deleted successfully"})
}

func (h *ProbeHandler) GetActiveProbes(w http.ResponseWriter, r *http.Request) {
	probes, err := h.probeService.GetActiveProbes(r.Context())
	if err != nil {
		h.log.Error("Failed to get active probes: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, probes)
}

func (h *ProbeHandler) GetProbesByBuilding(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	building := vars["building"]

	probes, err := h.probeService.GetProbesByBuilding(r.Context(), building)
	if err != nil {
		h.log.Error("Failed to get probes by building: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, probes)
}

func (h *ProbeHandler) SendCommand(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["id"]

	var req struct {
		Command string                 `json:"command"`
		Params  map[string]interface{} `json:"params,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("Invalid request body: %v", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	h.log.Info("Sending command %s to probe %s", req.Command, probeID)

	commandReq := &models.CommandRequest{
		ProbeID:     probeID,
		CommandType: req.Command,
		Payload:     req.Params,
	}

	command, err := h.commandService.IssueCommand(r.Context(), commandReq)
	if err != nil {
		h.log.Error("Failed to issue command: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Command sent successfully",
		"command": command,
	})
}

func (h *ProbeHandler) AdoptProbe(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["id"]

	var req models.UpdateProbeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("Invalid request body: %v", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Set status to active when adopting
	status := "active"
	req.Status = &status

	probe, err := h.probeService.UpdateProbe(r.Context(), probeID, &req)
	if err != nil {
		h.log.Error("Failed to adopt probe: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.log.Info("Probe %s adopted successfully", probeID)
	respondJSON(w, http.StatusOK, probe)
}
