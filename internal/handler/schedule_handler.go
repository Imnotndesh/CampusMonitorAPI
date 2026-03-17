package handler

import (
	"encoding/json"
	"net/http"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/service"

	"github.com/gorilla/mux"
)

type ScheduleHandler struct {
	scheduleService *service.ScheduleService
	log             *logger.Logger
}

func NewScheduleHandler(scheduleService *service.ScheduleService, log *logger.Logger) *ScheduleHandler {
	return &ScheduleHandler{
		scheduleService: scheduleService,
		log:             log,
	}
}

func (h *ScheduleHandler) RegisterRoutes(r *mux.Router) {
	// Probe-specific schedule routes
	r.HandleFunc("/probes/{probe_id}/tasks", h.ListTasks).Methods("GET")
	r.HandleFunc("/probes/{probe_id}/tasks", h.CreateTask).Methods("POST")
	r.HandleFunc("/probes/{probe_id}/tasks/{task_id}", h.GetTask).Methods("GET")
	r.HandleFunc("/probes/{probe_id}/tasks/{task_id}", h.UpdateTask).Methods("PUT")
	r.HandleFunc("/probes/{probe_id}/tasks/{task_id}", h.DeleteTask).Methods("DELETE")
}

func (h *ScheduleHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["probe_id"]

	tasks, err := h.scheduleService.List(r.Context(), probeID)
	if err != nil {
		h.log.Error("Failed to list tasks: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, tasks)
}

func (h *ScheduleHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["probe_id"]

	var task models.ScheduledTask
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		h.log.Warn("Invalid request body: %v", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	task.ProbeID = probeID

	if err := h.scheduleService.Create(r.Context(), &task); err != nil {
		h.log.Error("Failed to create task: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, task)
}

func (h *ScheduleHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["task_id"]

	task, err := h.scheduleService.Get(r.Context(), taskID)
	if err != nil {
		h.log.Error("Failed to get task: %v", err)
		respondError(w, http.StatusNotFound, "Task not found")
		return
	}
	respondJSON(w, http.StatusOK, task)
}

func (h *ScheduleHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["task_id"]

	var task models.ScheduledTask
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		h.log.Warn("Invalid request body: %v", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.scheduleService.Update(r.Context(), taskID, &task); err != nil {
		h.log.Error("Failed to update task: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, task)
}

func (h *ScheduleHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["task_id"]

	if err := h.scheduleService.Delete(r.Context(), taskID); err != nil {
		h.log.Error("Failed to delete task: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "Task deleted"})
}
