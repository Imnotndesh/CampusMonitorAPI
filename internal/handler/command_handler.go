// internal/handler/command_handler.go

package handler

import (
	"encoding/json"
	"net/http"

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
	r.HandleFunc("/commands/probe/{probe_id}", h.GetCommandHistory).Methods("GET")
	r.HandleFunc("/commands/pending", h.GetPendingCommands).Methods("GET")
}

func (h *CommandHandler) IssueCommand(w http.ResponseWriter, r *http.Request) {
	var req models.CommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("Invalid request body: %v", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
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
