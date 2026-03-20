package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"
	_ "time"

	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/repository"

	"github.com/jung-kurt/gofpdf"
)

type ReportService struct {
	repo *repository.ReportRepository
}

func NewReportService(repo *repository.ReportRepository) *ReportService {
	return &ReportService{repo: repo}
}

// GenerateReport returns the report data as JSON (for API) and optionally as PDF bytes.
func (s *ReportService) GenerateReport(ctx context.Context, req *models.ReportRequest) ([]byte, string, error) {
	var data interface{}
	var err error
	switch req.Type {
	case models.ReportTypeAlerts:
		data, err = s.repo.AlertReportData(ctx, req.From, req.To, req.ProbeIDs)
	case models.ReportTypeAnalytics:
		data, err = s.repo.AnalyticsReportData(ctx, req.From, req.To, req.ProbeIDs)
	case models.ReportTypeFleet:
		data, err = s.repo.FleetReportData(ctx)
	case models.ReportTypeProbes:
		data, err = s.repo.ProbeStatusReportData(ctx)
	case models.ReportTypeCompliance:
		thresholds := models.ComplianceThresholds{MinRSSI: -70, MaxLatency: 100, MaxPacketLoss: 2.0}
		data, err = s.repo.ComplianceReportData(ctx, thresholds)
	case models.ReportTypeFirmwareVersion:
		data, err = s.repo.FirmwareVersionReportData(ctx)
	case models.ReportTypeOutage:
		data, err = s.repo.OutageReportData(ctx, req.From, req.To, req.ProbeIDs)
	case models.ReportTypeCommandSuccess:
		data, err = s.repo.CommandSuccessReportData(ctx, req.From, req.To, req.ProbeIDs)
	case models.ReportTypeNetworkBaseline:
		data, err = s.repo.GetNetworkBaselineReportData(ctx, req.From, req.To)
	case models.ReportTypeSiteSurvey:
		if req.From.IsZero() {
			req.From = time.Now().Add(-24 * time.Hour)
		}
		if req.To.IsZero() {
			req.To = time.Now()
		}
		data, err = s.repo.GetSiteSurveyReportData(ctx, req.Building, req.Floor, req.From, req.To)
	default:
		return nil, "", fmt.Errorf("unsupported report type: %s", req.Type)
	}
	if err != nil {
		return nil, "", err
	}

	if req.Format == "pdf" {
		pdfBytes, err := s.renderPDF(req.Type, data)
		return pdfBytes, "application/pdf", err
	}
	// Default JSON
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	return jsonBytes, "application/json", err
}

// renderPDF creates a PDF for the given report data.
func (s *ReportService) renderPDF(reportType models.ReportType, data interface{}) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, fmt.Sprintf("Report: %s", reportType))
	pdf.Ln(12)

	switch v := data.(type) {
	case *models.AlertReport:
		s.renderAlertReport(pdf, v)
	case *models.AnalyticsReport:
		s.renderAnalyticsReport(pdf, v)
	case *models.FleetReport:
		s.renderFleetReport(pdf, v)
	case *models.ProbeStatusReport:
		s.renderProbeStatusReport(pdf, v)
	case *models.ComplianceReport:
		s.renderComplianceReport(pdf, v)
	case *models.FirmwareVersionReport:
		s.renderFirmwareReport(pdf, v)
	case *models.OutageReport:
		s.renderOutageReport(pdf, v)
	case *models.CommandSuccessReport:
		s.renderCommandSuccessReport(pdf, v)
	case *models.NetworkBaselineReport:
		s.renderNetworkBaselineReport(pdf, v)
	case *models.SiteSurveyReport:
		s.renderSiteSurveyReport(pdf, v)
	default:
		pdf.Cell(40, 10, "Unsupported report data")
	}

	var buf bytes.Buffer
	err := pdf.Output(&buf)
	return buf.Bytes(), err
}
func (s *ReportService) renderNetworkBaselineReport(pdf *gofpdf.Fpdf, report *models.NetworkBaselineReport) {
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Network Baseline Report")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(40, 6, fmt.Sprintf("Period: %s - %s", report.Period.From.Format("2006-01-02"), report.Period.To.Format("2006-01-02")))
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(40, 6, "Metric")
	pdf.Cell(30, 6, "Min")
	pdf.Cell(30, 6, "Max")
	pdf.Cell(30, 6, "Avg")
	pdf.Cell(30, 6, "P50")
	pdf.Cell(30, 6, "P95")
	pdf.Cell(30, 6, "P99")
	pdf.Ln(6)

	pdf.SetFont("Arial", "", 8)
	// RSSI
	pdf.Cell(40, 5, "RSSI (dBm)")
	pdf.Cell(30, 5, fmt.Sprintf("%.1f", report.RSSI.Min))
	pdf.Cell(30, 5, fmt.Sprintf("%.1f", report.RSSI.Max))
	pdf.Cell(30, 5, fmt.Sprintf("%.1f", report.RSSI.Avg))
	pdf.Cell(30, 5, fmt.Sprintf("%.1f", report.RSSI.P50))
	pdf.Cell(30, 5, fmt.Sprintf("%.1f", report.RSSI.P95))
	pdf.Cell(30, 5, fmt.Sprintf("%.1f", report.RSSI.P99))
	pdf.Ln(5)
	// Latency
	pdf.Cell(40, 5, "Latency (ms)")
	pdf.Cell(30, 5, fmt.Sprintf("%.1f", report.Latency.Min))
	pdf.Cell(30, 5, fmt.Sprintf("%.1f", report.Latency.Max))
	pdf.Cell(30, 5, fmt.Sprintf("%.1f", report.Latency.Avg))
	pdf.Cell(30, 5, fmt.Sprintf("%.1f", report.Latency.P50))
	pdf.Cell(30, 5, fmt.Sprintf("%.1f", report.Latency.P95))
	pdf.Cell(30, 5, fmt.Sprintf("%.1f", report.Latency.P99))
	pdf.Ln(5)
	// Packet Loss
	pdf.Cell(40, 5, "Packet Loss (%)")
	pdf.Cell(30, 5, fmt.Sprintf("%.2f", report.PacketLoss.Min))
	pdf.Cell(30, 5, fmt.Sprintf("%.2f", report.PacketLoss.Max))
	pdf.Cell(30, 5, fmt.Sprintf("%.2f", report.PacketLoss.Avg))
	pdf.Cell(30, 5, fmt.Sprintf("%.2f", report.PacketLoss.P50))
	pdf.Cell(30, 5, fmt.Sprintf("%.2f", report.PacketLoss.P95))
	pdf.Cell(30, 5, fmt.Sprintf("%.2f", report.PacketLoss.P99))
	pdf.Ln(5)
	// Throughput (if available)
	if report.Throughput.SampleCount > 0 {
		pdf.Cell(40, 5, "Throughput (kbps)")
		pdf.Cell(30, 5, fmt.Sprintf("%.0f", report.Throughput.Min))
		pdf.Cell(30, 5, fmt.Sprintf("%.0f", report.Throughput.Max))
		pdf.Cell(30, 5, fmt.Sprintf("%.0f", report.Throughput.Avg))
		pdf.Cell(30, 5, fmt.Sprintf("%.0f", report.Throughput.P50))
		pdf.Cell(30, 5, fmt.Sprintf("%.0f", report.Throughput.P95))
		pdf.Cell(30, 5, fmt.Sprintf("%.0f", report.Throughput.P99))
		pdf.Ln(5)
	}
}

func (s *ReportService) renderAlertReport(pdf *gofpdf.Fpdf, report *models.AlertReport) {
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Summary")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(40, 6, fmt.Sprintf("Total alerts: %d", report.Summary.Total))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Active: %d, Resolved: %d", report.Summary.ActiveCount, report.Summary.ResolvedCount))
	pdf.Ln(6)
	pdf.Cell(40, 6, "By severity:")
	for sev, cnt := range report.Summary.BySeverity {
		pdf.Cell(40, 6, fmt.Sprintf("  %s: %d", sev, cnt))
	}
	pdf.Ln(6)
	pdf.Cell(40, 6, "By type:")
	for typ, cnt := range report.Summary.ByType {
		pdf.Cell(40, 6, fmt.Sprintf("  %s: %d", typ, cnt))
	}
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Alert List")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 8)
	// Table headers
	pdf.Cell(20, 6, "ID")
	pdf.Cell(30, 6, "Probe")
	pdf.Cell(30, 6, "Type")
	pdf.Cell(25, 6, "Severity")
	pdf.Cell(50, 6, "Message")
	pdf.Cell(40, 6, "Triggered")
	pdf.Ln(6)
	for _, a := range report.Alerts {
		pdf.Cell(20, 5, fmt.Sprintf("%d", a.ID))
		pdf.Cell(30, 5, a.ProbeID)
		pdf.Cell(30, 5, a.AlertType)
		pdf.Cell(25, 5, a.Severity)
		pdf.Cell(50, 5, a.Message)
		pdf.Cell(40, 5, a.TriggeredAt.Format("2006-01-02 15:04"))
		pdf.Ln(5)
	}
}

func (s *ReportService) renderAnalyticsReport(pdf *gofpdf.Fpdf, report *models.AnalyticsReport) {
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Overall Metrics")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(40, 6, fmt.Sprintf("Avg RSSI: %.2f dBm", report.Overall.AvgRSSI))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Min/Max RSSI: %d / %d dBm", report.Overall.MinRSSI, report.Overall.MaxRSSI))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Avg Latency: %.2f ms", report.Overall.AvgLatency))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Avg Packet Loss: %.2f %%", report.Overall.AvgPacketLoss))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Stability Score: %.2f", report.Overall.StabilityScore))
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Channel Distribution")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 8)
	pdf.Cell(30, 6, "Channel")
	pdf.Cell(30, 6, "Probe Count")
	pdf.Cell(30, 6, "Avg RSSI")
	pdf.Cell(30, 6, "Utilization")
	pdf.Ln(6)
	for _, c := range report.ChannelDist {
		pdf.Cell(30, 5, fmt.Sprintf("%d", c.Channel))
		pdf.Cell(30, 5, fmt.Sprintf("%d", c.ProbeCount))
		pdf.Cell(30, 5, fmt.Sprintf("%.2f", c.AvgRSSI))
		pdf.Cell(30, 5, fmt.Sprintf("%.2f%%", c.Utilization))
		pdf.Ln(5)
	}
	// ... add more sections (top APs, congestion, etc.)
}

func (s *ReportService) renderFleetReport(pdf *gofpdf.Fpdf, report *models.FleetReport) {
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Fleet Summary")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(40, 6, fmt.Sprintf("Total Probes: %d", report.Summary.TotalProbes))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Managed Probes: %d", report.Summary.ManagedProbes))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Active Probes: %d", report.Summary.ActiveProbes))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Stale Probes: %d", report.Summary.StaleProbes))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Health Score: %.2f", report.Summary.HealthScore))
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Groups")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 8)
	pdf.Cell(40, 6, "Group")
	pdf.Cell(30, 6, "Probes")
	pdf.Cell(30, 6, "Online")
	pdf.Cell(30, 6, "Alerts")
	pdf.Ln(6)
	for _, g := range report.Groups {
		pdf.Cell(40, 5, g.Name)
		pdf.Cell(30, 5, fmt.Sprintf("%d", g.ProbeCount))
		pdf.Cell(30, 5, fmt.Sprintf("%d", g.Online))
		pdf.Cell(30, 5, fmt.Sprintf("%d", g.AlertCount))
		pdf.Ln(5)
	}
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Command Summary")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(40, 6, fmt.Sprintf("Total Commands: %d", report.Commands.Total))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Pending: %d, Completed: %d, Failed: %d", report.Commands.Pending, report.Commands.Completed, report.Commands.Failed))
	pdf.Ln(6)
}

func (s *ReportService) renderProbeStatusReport(pdf *gofpdf.Fpdf, report *models.ProbeStatusReport) {
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Probe Status")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(40, 6, fmt.Sprintf("Total: %d, Active: %d, Offline: %d, Pending: %d",
		report.Summary.Total, report.Summary.Active, report.Summary.Offline, report.Summary.Pending))
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(35, 6, "Probe ID")
	pdf.Cell(35, 6, "Location")
	pdf.Cell(30, 6, "Building")
	pdf.Cell(20, 6, "Floor")
	pdf.Cell(20, 6, "Status")
	pdf.Cell(35, 6, "Last Seen")
	pdf.Cell(30, 6, "RSSI")
	pdf.Ln(6)
	pdf.SetFont("Arial", "", 8)
	for _, p := range report.Probes {
		pdf.Cell(35, 5, p.ProbeID)
		pdf.Cell(35, 5, p.Location)
		pdf.Cell(30, 5, p.Building)
		pdf.Cell(20, 5, p.Floor)
		pdf.Cell(20, 5, p.Status)
		pdf.Cell(35, 5, p.LastSeen.Format("2006-01-02 15:04"))
		rssiStr := "N/A"
		if p.RSSI != nil {
			rssiStr = fmt.Sprintf("%d", *p.RSSI)
		}
		pdf.Cell(30, 5, rssiStr)
		pdf.Ln(5)
	}
}

func (s *ReportService) renderComplianceReport(pdf *gofpdf.Fpdf, report *models.ComplianceReport) {
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Compliance Report")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(40, 6, fmt.Sprintf("Thresholds: RSSI >= %d, Latency <= %d ms, Packet Loss <= %.2f%%",
		report.Thresholds.MinRSSI, report.Thresholds.MaxLatency, report.Thresholds.MaxPacketLoss))
	pdf.Ln(8)
	pdf.Cell(40, 6, fmt.Sprintf("Compliant Probes: %d / %d (%.1f%%)",
		report.Compliant, report.TotalProbes, float64(report.Compliant)/float64(report.TotalProbes)*100))
	pdf.Ln(10)
	if len(report.NonCompliant) > 0 {
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(40, 6, "Probe ID")
		pdf.Cell(40, 6, "Location")
		pdf.Cell(30, 6, "RSSI")
		pdf.Cell(30, 6, "Latency")
		pdf.Cell(30, 6, "Packet Loss")
		pdf.Cell(50, 6, "Reason")
		pdf.Ln(6)
		pdf.SetFont("Arial", "", 8)
		for _, nc := range report.NonCompliant {
			pdf.Cell(40, 5, nc.ProbeID)
			pdf.Cell(40, 5, nc.Location)
			rssi := "N/A"
			if nc.RSSI != nil {
				rssi = fmt.Sprintf("%d", *nc.RSSI)
			}
			pdf.Cell(30, 5, rssi)
			lat := "N/A"
			if nc.Latency != nil {
				lat = fmt.Sprintf("%d", *nc.Latency)
			}
			pdf.Cell(30, 5, lat)
			loss := "N/A"
			if nc.PacketLoss != nil {
				loss = fmt.Sprintf("%.2f", *nc.PacketLoss)
			}
			pdf.Cell(30, 5, loss)
			pdf.Cell(50, 5, nc.Reason)
			pdf.Ln(5)
		}
	}
}

func (s *ReportService) renderFirmwareReport(pdf *gofpdf.Fpdf, report *models.FirmwareVersionReport) {
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Firmware Version Report")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(40, 6, fmt.Sprintf("Total Probes: %d", report.Summary.TotalProbes))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Unique Versions: %d", report.Summary.UniqueVersions))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Most Common: %s", report.Summary.MostCommon))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Up to Date: %d", report.Summary.UpToDateCount))
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(40, 6, "Version")
	pdf.Cell(30, 6, "Count")
	pdf.Ln(6)
	pdf.SetFont("Arial", "", 8)
	for _, v := range report.ByVersion {
		pdf.Cell(40, 5, v.Version)
		pdf.Cell(30, 5, fmt.Sprintf("%d", v.Count))
		pdf.Ln(5)
	}
	if len(report.OutdatedProbes) > 0 {
		pdf.Ln(10)
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(40, 6, "Outdated Probes")
		pdf.Cell(40, 6, "Current Version")
		pdf.Cell(30, 6, "Target Version")
		pdf.Ln(6)
		pdf.SetFont("Arial", "", 8)
		for _, o := range report.OutdatedProbes {
			pdf.Cell(40, 5, o.ProbeID)
			pdf.Cell(40, 5, o.CurrentVersion)
			pdf.Cell(30, 5, o.TargetVersion)
			pdf.Ln(5)
		}
	}
}

func (s *ReportService) renderOutageReport(pdf *gofpdf.Fpdf, report *models.OutageReport) {
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Outage Report")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(40, 6, fmt.Sprintf("Period: %s – %s", report.Period.From.Format("2006-01-02"), report.Period.To.Format("2006-01-02")))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Total Outages: %d", report.Summary.TotalOutages))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Affected Probes: %d", report.Summary.AffectedProbes))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Total Downtime: %s", report.Summary.TotalDowntime))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Avg Outage Duration: %s", report.Summary.AvgOutageDuration))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Longest Outage: %s", report.Summary.LongestOutage))
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(35, 6, "Probe ID")
	pdf.Cell(40, 6, "Start")
	pdf.Cell(40, 6, "End")
	pdf.Cell(35, 6, "Duration")
	pdf.Cell(40, 6, "Reason")
	pdf.Ln(6)
	pdf.SetFont("Arial", "", 8)
	for _, o := range report.Outages {
		pdf.Cell(35, 5, o.ProbeID)
		pdf.Cell(40, 5, o.Start.Format("2006-01-02 15:04"))
		endStr := "ongoing"
		if o.End != nil {
			endStr = o.End.Format("2006-01-02 15:04")
		}
		pdf.Cell(40, 5, endStr)
		pdf.Cell(35, 5, o.Duration.String())
		pdf.Cell(40, 5, o.Reason)
		pdf.Ln(5)
	}
}

func (s *ReportService) renderCommandSuccessReport(pdf *gofpdf.Fpdf, report *models.CommandSuccessReport) {
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, "Command Success Report")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(40, 6, fmt.Sprintf("Period: %s – %s", report.Period.From.Format("2006-01-02"), report.Period.To.Format("2006-01-02")))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Total Commands: %d", report.Overall.Total))
	pdf.Ln(6)
	pdf.Cell(40, 6, fmt.Sprintf("Success Rate: %.2f%%", report.Overall.SuccessRate))
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(40, 6, "Command Type")
	pdf.Cell(30, 6, "Total")
	pdf.Cell(30, 6, "Success")
	pdf.Cell(30, 6, "Failed")
	pdf.Cell(30, 6, "Success Rate")
	pdf.Ln(6)
	pdf.SetFont("Arial", "", 8)
	for _, t := range report.ByType {
		pdf.Cell(40, 5, t.CommandType)
		pdf.Cell(30, 5, fmt.Sprintf("%d", t.Total))
		pdf.Cell(30, 5, fmt.Sprintf("%d", t.Succeeded))
		pdf.Cell(30, 5, fmt.Sprintf("%d", t.Failed))
		pdf.Cell(30, 5, fmt.Sprintf("%.2f%%", t.SuccessRate))
		pdf.Ln(5)
	}
}
func (s *ReportService) renderSiteSurveyReport(pdf *gofpdf.Fpdf, report *models.SiteSurveyReport) {
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "Site Survey Report")
	pdf.Ln(12)
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 6, fmt.Sprintf("Building: %s | Floor: %s", report.Building, report.Floor))
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(40, 6, fmt.Sprintf("Generated: %s", report.GeneratedAt.Format("2006-01-02 15:04:05")))
	pdf.Ln(12)

	// Heatmap section (list of locations with RSSI)
	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(40, 6, "Signal Strength by Location")
	pdf.Ln(6)
	pdf.SetFont("Arial", "", 8)
	for _, hp := range report.Heatmap {
		pdf.Cell(40, 5, hp.Location)
		pdf.Cell(30, 5, fmt.Sprintf("%.1f dBm", hp.RSSI))
		pdf.Ln(5)
	}
	pdf.Ln(6)

	// Channel usage
	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(40, 6, "Channel Utilization")
	pdf.Ln(6)
	pdf.SetFont("Arial", "", 8)
	for _, cu := range report.ChannelUsage {
		pdf.Cell(30, 5, fmt.Sprintf("CH %d", cu.Channel))
		pdf.Cell(40, 5, fmt.Sprintf("%.1f%%", cu.Utilization))
		pdf.Ln(5)
	}
	pdf.Ln(6)

	// AP list
	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(40, 6, "Access Points Detected")
	pdf.Ln(6)
	pdf.SetFont("Arial", "", 8)
	for _, ap := range report.APList {
		pdf.Cell(40, 5, ap.BSSID)
		pdf.Cell(20, 5, fmt.Sprintf("CH %d", ap.Channel))
		pdf.Cell(30, 5, fmt.Sprintf("%.1f dBm", ap.AvgRSSI))
		pdf.Cell(30, 5, fmt.Sprintf("%d probes", ap.ProbesSeen))
		pdf.Ln(5)
	}
	pdf.Ln(6)

	// Recommendations
	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(40, 6, "Recommendations")
	pdf.Ln(6)
	pdf.SetFont("Arial", "", 8)
	for _, rec := range report.Recommendations {
		pdf.MultiCell(0, 5, rec, "", "", false)
		pdf.Ln(2)
	}
}
