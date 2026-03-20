package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/service"

	"github.com/gorilla/mux"
)

type ReportHandler struct {
	reportService *service.ReportService
	log           *logger.Logger
}

func NewReportHandler(reportService *service.ReportService, log *logger.Logger) *ReportHandler {
	return &ReportHandler{
		reportService: reportService,
		log:           log,
	}
}

func (h *ReportHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/reports/generate", h.GenerateReport).Methods("GET", "POST")
}

func (h *ReportHandler) GenerateReport(w http.ResponseWriter, r *http.Request) {
	var req models.ReportRequest

	if r.Method == http.MethodPost {
		if err := decodeJSON(r, &req); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
	} else {
		req.Type = models.ReportType(r.URL.Query().Get("type"))
		if fromStr := r.URL.Query().Get("from"); fromStr != "" {
			if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
				req.From = t
			}
		}
		if toStr := r.URL.Query().Get("to"); toStr != "" {
			if t, err := time.Parse(time.RFC3339, toStr); err == nil {
				req.To = t
			}
		}
		if probeIDs := r.URL.Query()["probe_ids"]; len(probeIDs) > 0 {
			req.ProbeIDs = probeIDs
		}
		if groups := r.URL.Query()["groups"]; len(groups) > 0 {
			req.Groups = groups
		}
		req.Format = r.URL.Query().Get("format")
	}

	if req.Type == "" {
		respondError(w, http.StatusBadRequest, "report type is required")
		return
	}
	if req.Format == "" {
		req.Format = "json"
	}
	if req.Format != "json" && req.Format != "pdf" {
		respondError(w, http.StatusBadRequest, "format must be json or pdf")
		return
	}

	if req.Type == models.ReportTypeAlerts || req.Type == models.ReportTypeAnalytics || req.Type == models.ReportTypeOutage || req.Type == models.ReportTypeCommandSuccess {
		if req.From.IsZero() {
			req.From = time.Now().Add(-24 * time.Hour)
		}
		if req.To.IsZero() {
			req.To = time.Now()
		}
	}

	h.log.Info("Generating report: type=%s, format=%s, from=%v, to=%v", req.Type, req.Format, req.From, req.To)

	data, contentType, err := h.reportService.GenerateReport(r.Context(), &req)
	if err != nil {
		h.log.Error("Failed to generate report: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to generate report")
		return
	}

	w.Header().Set("Content-Type", contentType)
	if req.Format == "pdf" {
		w.Header().Set("Content-Disposition", "attachment; filename=report.pdf")
	}
	_, err = w.Write(data)
	if err != nil {
		return
	}
}
func decodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
