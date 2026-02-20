package handler

import (
	"net/http"
	"strconv"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/service"

	"github.com/gorilla/mux"
)

type AlertHandler struct {
	alertService service.IAlertService
	log          *logger.Logger
}

func NewAlertHandler(alertService service.IAlertService, log *logger.Logger) *AlertHandler {
	return &AlertHandler{
		alertService: alertService,
		log:          log,
	}
}

func (h *AlertHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/alerts/active", h.GetActiveAlerts).Methods("GET")
	r.HandleFunc("/alerts/history", h.GetAlertHistory).Methods("GET")
	r.HandleFunc("/alerts/probe/{probe_id}", h.GetProbeAlerts).Methods("GET")
	r.HandleFunc("/alerts/acknowledge/{id}", h.Acknowledge).Methods("PUT")
	r.HandleFunc("/alerts/resolve/{id}", h.Resolve).Methods("PUT")
	r.HandleFunc("/alerts/{id}", h.Delete).Methods("DELETE")
	r.HandleFunc("/alerts/test", h.SendTest).Methods("POST")

}

func (h *AlertHandler) GetActiveAlerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := h.alertService.GetActiveAlerts(r.Context())
	if err != nil {
		h.log.Error("Failed to get active alerts: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, alerts)
}
func (h *AlertHandler) SendTest(w http.ResponseWriter, r *http.Request) {
	if err := h.alertService.SendTestAlert(r.Context()); err != nil {
		h.log.Error("Failed to send test alert: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.log.Info("Simulation: Test alert triggered successfully")
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Test alert dispatched to all connected clients",
		"type":    "SIMULATION",
	})
}

func (h *AlertHandler) GetAlertHistory(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil {
			offset = parsed
		}
	}

	alerts, err := h.alertService.GetAlertHistory(r.Context(), limit, offset)
	if err != nil {
		h.log.Error("Failed to get alert history: %v", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, alerts)
}

func (h *AlertHandler) GetProbeAlerts(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	probeID := vars["probe_id"]

	alerts, err := h.alertService.GetProbeAlerts(r.Context(), probeID)
	if err != nil {
		h.log.Error("Failed to get alerts for probe %s: %v", probeID, err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, alerts)
}

func (h *AlertHandler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid alert ID")
		return
	}

	if err := h.alertService.Acknowledge(r.Context(), uint(id)); err != nil {
		h.log.Error("Failed to acknowledge alert %d: %v", id, err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "alert acknowledged"})
}

func (h *AlertHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid alert ID")
		return
	}

	if err := h.alertService.Resolve(r.Context(), uint(id)); err != nil {
		h.log.Error("Failed to resolve alert %d: %v", id, err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "alert resolved"})
}

func (h *AlertHandler) Delete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid alert ID")
		return
	}

	if err := h.alertService.DeleteAlert(r.Context(), uint(id)); err != nil {
		h.log.Error("Failed to delete alert %d: %v", id, err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
