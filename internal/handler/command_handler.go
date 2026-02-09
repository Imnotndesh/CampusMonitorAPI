// internal/handler/command_handler.go (Updated)

package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/service"

	"github.com/gorilla/mux"
)

type CommandHandler struct {
	commandService *service.CommandService
	log            *logger.Logger
}

func NewCommandHandler(commandService *service.CommandService, log *logger.Logger) *CommandHandler {
	return &CommandHandler{
		commandService: commandService,
		log:            log,
	}
}

func (h *CommandHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/commands", h.IssueCommand).Methods("POST")
	r.HandleFunc("/commands/{id}", h.GetCommand).Methods("GET")
	r.HandleFunc("/commands/probe/{probe_id}", h.GetCommandHistory).Methods("GET")
	r.HandleFunc("/commands/pending", h.GetPendingCommands).Methods("GET")
	r.HandleFunc("/commands/broadcast", h.BroadcastCommand).Methods("POST")
	r.HandleFunc("/commands/statistics", h.GetStatistics).Methods("GET")
	r.HandleFunc("/commands/{id}/result", h.UpdateCommandResult).Methods("PUT")
	r.HandleFunc("/commands/{id}", h.DeleteCommand).Methods("DELETE")
}

func (h *CommandHandler) IssueCommand(w http.ResponseWriter, r *http.Request) {
	var req models.CommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("Invalid request body: %v", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ProbeID == "" || req.CommandType == "" {
		respondError(w, http.StatusBadRequest, "probe_id and command_type are required")
		return
	}

	command, err := h.commandService.IssueCommand(r.Context(), &req)
	if err != nil {
		h.log.Error("Failed to issue command: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, command)
}

func (h *CommandHandler) GetCommand(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid command ID")
		return
	}

	command, err := h.commandService.GetCommandHistory(r.Context(), strconv.Itoa(id))
	if err != nil {
		h.log.Error("Failed to get command: %v", err)
		respondError(w, http.StatusNotFound, "Command not found")
		return
	}

	respondJSON(w, http.StatusOK, command)
}

func (h *CommandHandler) GetCommandHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["probe_id"]

	commands, err := h.commandService.GetCommandHistory(r.Context(), probeID)
	if err != nil {
		h.log.Error("Failed to get command history: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, commands)
}

func (h *CommandHandler) GetPendingCommands(w http.ResponseWriter, r *http.Request) {
	commands, err := h.commandService.GetPendingCommands(r.Context())
	if err != nil {
		h.log.Error("Failed to get pending commands: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, commands)
}

func (h *CommandHandler) BroadcastCommand(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CommandType string                 `json:"command_type"`
		Params      map[string]interface{} `json:"params,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("Invalid request body: %v", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.CommandType == "" {
		respondError(w, http.StatusBadRequest, "command_type is required")
		return
	}

	err := h.commandService.BroadcastCommand(r.Context(), req.CommandType, req.Params)
	if err != nil {
		h.log.Error("Failed to broadcast command: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Broadcast command sent successfully",
	})
}

func (h *CommandHandler) GetStatistics(w http.ResponseWriter, r *http.Request) {
	stats, err := h.commandService.GetCommandStatistics(r.Context())
	if err != nil {
		h.log.Error("Failed to get command statistics: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

func (h *CommandHandler) UpdateCommandResult(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid command ID")
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		h.log.Warn("Invalid request body: %v", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.commandService.UpdateResultByID(r.Context(), id, result); err != nil {
		h.log.Error("Failed to update command result: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Command result updated",
	})
}
func (h *CommandHandler) DeleteCommand(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid command ID")
		return
	}

	if err := h.commandService.DeleteCommand(r.Context(), id); err != nil {
		h.log.Error("Failed to delete command: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Command deleted",
	})
}
