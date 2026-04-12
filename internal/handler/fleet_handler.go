package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/service"

	"github.com/gorilla/mux"
)

type FleetHandler struct {
	fleetService   *service.FleetService
	probeService   *service.ProbeService
	commandService *service.CommandService
	log            *logger.Logger
}

func NewFleetHandler(
	fleetService *service.FleetService,
	probeService *service.ProbeService,
	commandService *service.CommandService,
	log *logger.Logger,
) *FleetHandler {
	return &FleetHandler{
		fleetService:   fleetService,
		probeService:   probeService,
		commandService: commandService,
		log:            log,
	}
}

func (h *FleetHandler) RegisterRoutes(r *mux.Router) {
	// Fleet probe management
	r.HandleFunc("/fleet/probes", h.ListFleetProbes).Methods("GET")
	r.HandleFunc("/fleet/probes/{id}/enroll", h.EnrollProbe).Methods("POST")
	r.HandleFunc("/fleet/probes/{id}/unenroll", h.UnenrollProbe).Methods("POST")
	r.HandleFunc("/fleet/probes/{id}", h.GetFleetProbe).Methods("GET")
	r.HandleFunc("/fleet/probes/{id}", h.UpdateFleetProbe).Methods("PUT", "PATCH")

	// Fleet commands
	r.HandleFunc("/fleet/commands", h.SendFleetCommand).Methods("POST")
	r.HandleFunc("/fleet/commands", h.ListFleetCommands).Methods("GET")
	r.HandleFunc("/fleet/commands/{id}", h.GetFleetCommandStatus).Methods("GET")
	r.HandleFunc("/fleet/commands/{id}/cancel", h.CancelFleetCommand).Methods("POST")

	// Configuration templates
	r.HandleFunc("/fleet/templates", h.CreateTemplate).Methods("POST")
	r.HandleFunc("/fleet/templates", h.ListTemplates).Methods("GET")
	r.HandleFunc("/fleet/templates/{id}", h.GetTemplate).Methods("GET")
	r.HandleFunc("/fleet/templates/{id}/apply", h.ApplyTemplate).Methods("POST")
	r.HandleFunc("/fleet/templates/{id}", h.DeleteTemplate).Methods("DELETE")
	r.HandleFunc("/fleet/unenrolled-probes", h.ListUnenrolledProbes).Methods("GET")

	// Probe schedules
	r.HandleFunc("/fleet/probes/{id}/schedules", h.GetProbeSchedules).Methods("GET")
	r.HandleFunc("/fleet/probes/{id}/schedules/{schedule_id}", h.DeleteProbeSchedule).Methods("DELETE")
	r.HandleFunc("/fleet/groups/{id}/schedules", h.GetGroupSchedules).Methods("GET")

	// Group management
	r.HandleFunc("/fleet/groups", h.CreateGroup).Methods("POST")
	r.HandleFunc("/fleet/groups", h.ListGroups).Methods("GET")
	r.HandleFunc("/fleet/groups/{id}", h.DeleteGroup).Methods("DELETE")

	// Fleet status
	r.HandleFunc("/fleet/status", h.GetFleetStatus).Methods("GET")
	r.HandleFunc("/fleet/health", h.GetFleetHealth).Methods("GET")
}

func (h *FleetHandler) EnrollProbe(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["id"]

	// Verify probe exists
	_, err := h.probeService.GetProbe(r.Context(), probeID)
	if err != nil {
		h.log.Warn("Attempted to enroll non-existent probe: %s", probeID)
		respondError(w, http.StatusNotFound, "Probe not found")
		return
	}

	var req models.FleetEnrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("Invalid enroll request: %v", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user := getUserFromContext(r)

	if err := h.fleetService.EnrollProbe(r.Context(), probeID, &req, user); err != nil {
		h.log.Error("Failed to enroll probe %s: %v", probeID, err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message":  "Probe enrolled in fleet management",
		"probe_id": probeID,
	})
}
func (h *FleetHandler) ListUnenrolledProbes(w http.ResponseWriter, r *http.Request) {
	probes, err := h.fleetService.GetUnenrolledProbes(r.Context())
	if err != nil {
		h.log.Error("Failed to list unenrolled probes: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to fetch unenrolled probes")
		return
	}
	respondJSON(w, http.StatusOK, probes)
}
func (h *FleetHandler) UnenrollProbe(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["id"]

	if err := h.fleetService.UnenrollProbe(r.Context(), probeID); err != nil {
		h.log.Error("Failed to unenroll probe %s: %v", probeID, err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message":  "Probe removed from fleet management",
		"probe_id": probeID,
	})
}

func (h *FleetHandler) GetFleetProbe(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["id"]

	probe, err := h.fleetService.GetFleetProbe(r.Context(), probeID)
	if err != nil {
		h.log.Error("Failed to get fleet probe %s: %v", probeID, err)
		respondError(w, http.StatusNotFound, "Fleet probe not found")
		return
	}

	respondJSON(w, http.StatusOK, probe)
}

func (h *FleetHandler) ListFleetProbes(w http.ResponseWriter, r *http.Request) {
	group := r.URL.Query().Get("group")

	probes, err := h.fleetService.ListFleetProbes(r.Context(), group)
	if err != nil {
		h.log.Error("Failed to list fleet probes: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, probes)
}

func (h *FleetHandler) UpdateFleetProbe(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["id"]

	var req models.FleetUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("Invalid update request: %v", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.fleetService.UpdateFleetProbe(r.Context(), probeID, &req); err != nil {
		h.log.Error("Failed to update fleet probe %s: %v", probeID, err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Fleet probe updated",
	})
}

func (h *FleetHandler) SendFleetCommand(w http.ResponseWriter, r *http.Request) {
	var req models.FleetCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("Invalid fleet command request: %v", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.CommandType == "" {
		respondError(w, http.StatusBadRequest, "command_type is required")
		return
	}

	if !req.TargetAll && len(req.Groups) == 0 && len(req.ProbeIDs) == 0 {
		respondError(w, http.StatusBadRequest, "Must specify targets (target_all, groups, or probe_ids)")
		return
	}

	user := getUserFromContext(r)

	cmd, err := h.fleetService.SendFleetCommand(r.Context(), &req, user)
	if err != nil {
		h.log.Error("Failed to send fleet command: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusAccepted, cmd)
}

func (h *FleetHandler) GetFleetCommandStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	commandID := vars["id"]

	status, err := h.fleetService.GetFleetCommandStatus(r.Context(), commandID)
	if err != nil {
		h.log.Error("Failed to get command status: %v", err)
		respondError(w, http.StatusNotFound, "Command not found")
		return
	}

	respondJSON(w, http.StatusOK, status)
}

func (h *FleetHandler) ListFleetCommands(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	commands, err := h.fleetService.ListFleetCommands(r.Context(), status, limit)
	if err != nil {
		h.log.Error("Failed to list fleet commands: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, commands)
}

func (h *FleetHandler) CancelFleetCommand(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	commandID := vars["id"]

	if err := h.fleetService.CancelFleetCommand(r.Context(), commandID); err != nil {
		h.log.Error("Failed to cancel command %s: %v", commandID, err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Command cancelled",
	})
}

func (h *FleetHandler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var template models.FleetConfigTemplate
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		h.log.Warn("Invalid template request: %v", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if template.Name == "" {
		respondError(w, http.StatusBadRequest, "template name is required")
		return
	}

	user := getUserFromContext(r)

	if err := h.fleetService.CreateTemplate(r.Context(), &template, user); err != nil {
		h.log.Error("Failed to create template: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, template)
}

func (h *FleetHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid template ID")
		return
	}

	template, err := h.fleetService.GetTemplate(r.Context(), id)
	if err != nil {
		h.log.Error("Failed to get template %d: %v", id, err)
		respondError(w, http.StatusNotFound, "Template not found")
		return
	}

	respondJSON(w, http.StatusOK, template)
}

func (h *FleetHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.fleetService.ListTemplates(r.Context())
	if err != nil {
		h.log.Error("Failed to list templates: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, templates)
}

func (h *FleetHandler) ApplyTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid template ID")
		return
	}

	var req struct {
		ProbeIDs []string `json:"probe_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("Invalid apply template request: %v", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.ProbeIDs) == 0 {
		respondError(w, http.StatusBadRequest, "probe_ids required")
		return
	}

	user := getUserFromContext(r)

	if err := h.fleetService.ApplyTemplate(r.Context(), id, req.ProbeIDs, user); err != nil {
		h.log.Error("Failed to apply template: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Template applied successfully",
	})
}

func (h *FleetHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid template ID")
		return
	}

	if err := h.fleetService.DeleteTemplate(r.Context(), id); err != nil {
		h.log.Error("Failed to delete template %d: %v", id, err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Group Handlers

func (h *FleetHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("Invalid create group request: %v", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "group name is required")
		return
	}

	group, err := h.fleetService.CreateGroup(r.Context(), req.Name, req.Description)
	if err != nil {
		h.log.Error("Failed to create group: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, group)
}

func (h *FleetHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.fleetService.ListGroups(r.Context())
	if err != nil {
		h.log.Error("Failed to list groups: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, groups)
}

func (h *FleetHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID := vars["id"]

	if err := h.fleetService.DeleteGroup(r.Context(), groupID); err != nil {
		h.log.Error("Failed to delete group %s: %v", groupID, err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Fleet Status Handlers

func (h *FleetHandler) GetFleetStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.fleetService.GetFleetStatus(r.Context())
	if err != nil {
		h.log.Error("Failed to get fleet status: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, status)
}

func (h *FleetHandler) GetFleetHealth(w http.ResponseWriter, r *http.Request) {
	// Quick health check for fleet operations
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"services": map[string]bool{
			"fleet_db": true,
			"mqtt":     true,
		},
	}

	respondJSON(w, http.StatusOK, health)
}

// GetProbeSchedules returns the last known schedules for a probe
func (h *FleetHandler) GetProbeSchedules(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["id"]

	var schedulesJSON []byte
	err := h.fleetService.GetProbeSchedules(r.Context(), probeID, &schedulesJSON)
	if err != nil {
		h.log.Error("Failed to get probe schedules: %v", err)
		respondError(w, http.StatusNotFound, "No schedules found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(schedulesJSON)
}

// DeleteProbeSchedule sends a command to delete a specific schedule
func (h *FleetHandler) DeleteProbeSchedule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["id"]
	scheduleID := vars["schedule_id"]

	cmdReq := &models.FleetCommandRequest{
		CommandType: "delete_schedule",
		Payload: map[string]interface{}{
			"id": scheduleID,
		},
		ProbeIDs: []string{probeID},
		Strategy: "immediate",
	}

	user := getUserFromContext(r)
	_, err := h.fleetService.SendFleetCommand(r.Context(), cmdReq, user)
	if err != nil {
		h.log.Error("Failed to send delete schedule command: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Delete command sent"})
}

// GetGroupSchedules aggregates schedules for all probes in a group
func (h *FleetHandler) GetGroupSchedules(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID := vars["id"]

	// Get all probes in this group from fleet_probes
	probes, err := h.fleetService.ListFleetProbes(r.Context(), groupID)
	if err != nil {
		h.log.Error("Failed to list probes in group: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := make(map[string]interface{})
	for _, p := range probes {
		var schedulesJSON []byte
		err := h.fleetService.GetProbeSchedules(r.Context(), p.ProbeID, &schedulesJSON)
		if err == nil {
			result[p.ProbeID] = json.RawMessage(schedulesJSON)
		}
	}

	respondJSON(w, http.StatusOK, result)
}

// Helper function
func getUserFromContext(r *http.Request) string {
	// This would come from your auth middleware
	// For now, return a default
	return "system"
}
