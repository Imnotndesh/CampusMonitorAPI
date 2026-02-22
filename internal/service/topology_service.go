package service

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"CampusMonitorAPI/internal/models"
	"CampusMonitorAPI/internal/repository"
)

type TopologyLayout struct {
	Center    BuildingNode   `json:"center"`
	Buildings []BuildingNode `json:"buildings"`
}

type BuildingNode struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Coordinates Coordinates `json:"coordinates"`
	Floors      []FloorNode `json:"floors"`
}

type Coordinates struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type FloorNode struct {
	Level      int      `json:"level"`
	FloorID    string   `json:"floor_id"` // e.g., "Ground", "1st Floor"
	ZIndex     int      `json:"z_index"`
	ProbeCount int      `json:"probe_count"`
	Probes     []string `json:"probes"`
}

type HeatmapResponse struct {
	Timestamp   time.Time     `json:"timestamp"`
	Metric      string        `json:"metric"`
	HeatmapData []FloorHealth `json:"heatmap_data"`
}

type FloorHealth struct {
	BuildingID   string  `json:"building_id"`
	FloorID      string  `json:"floor_id"`
	Status       string  `json:"status"`        // HEALTHY, WARNING, CRITICAL, OFFLINE
	ColorHex     string  `json:"color_hex"`     // Pre-computed hex color for UI rendering
	AverageValue float64 `json:"average_value"` // e.g., average RSSI or Latency
	ActiveAlerts int     `json:"active_alerts"`
}

type FloorDetails struct {
	Building      string        `json:"building"`
	Floor         string        `json:"floor"`
	OverallHealth string        `json:"overall_health"`
	Probes        []ProbeDetail `json:"probes"`
}

type ProbeDetail struct {
	ProbeID        string                 `json:"probe_id"`
	Status         string                 `json:"status"`
	LastSeen       time.Time              `json:"last_seen"`
	CurrentMetrics map[string]interface{} `json:"current_metrics"`
	ActiveAlerts   []models.Alert         `json:"active_alerts"`
}

// --- Service Interface & Implementation ---

type ITopologyService interface {
	GetLayout(ctx context.Context) (*TopologyLayout, error)
	GetHeatmap(ctx context.Context, metric string) (*HeatmapResponse, error)
	GetFloorDetails(ctx context.Context, building string, floor string) (*FloorDetails, error)
}

type TopologyService struct {
	probeRepo     *repository.ProbeRepository
	telemetryRepo *repository.TelemetryRepository
	alertRepo     *repository.AlertRepository
}

func NewTopologyService(
	probeRepo *repository.ProbeRepository,
	telemetryRepo *repository.TelemetryRepository,
	alertRepo *repository.AlertRepository,
) *TopologyService {
	return &TopologyService{
		probeRepo:     probeRepo,
		telemetryRepo: telemetryRepo,
		alertRepo:     alertRepo,
	}
}

// GetLayout builds the 2D/3D map grid based on your registered probes.
// It assigns X/Y coordinates to buildings and Z-indexes to floors.
func (s *TopologyService) GetLayout(ctx context.Context) (*TopologyLayout, error) {
	probes, err := s.probeRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch probes for layout: %w", err)
	}

	// CHANGED: Map now stores an array of probe IDs instead of just an integer count
	buildingMap := make(map[string]map[string][]string) // Building -> Floor -> []ProbeIDs

	for _, p := range probes {
		bName := p.Building
		fName := p.Floor

		if bName == "" {
			bName = "Unknown Building"
		}
		if fName == "" {
			fName = "Ground"
		}

		if _, exists := buildingMap[bName]; !exists {
			buildingMap[bName] = make(map[string][]string)
		}

		// Append the actual ProbeID to the floor's array
		buildingMap[bName][fName] = append(buildingMap[bName][fName], p.ProbeID)
	}

	layout := &TopologyLayout{
		Center: BuildingNode{
			ID:          "SERVER_ROOM",
			Name:        "Core Network Operations",
			Coordinates: Coordinates{X: 0, Y: 0},
			Floors:      []FloorNode{{Level: 1, FloorID: "Ground", ZIndex: 0, ProbeCount: 0, Probes: []string{}}},
		},
		Buildings: []BuildingNode{},
	}

	// Calculate points in a circle around the center (0,0)
	radius := 250.0 // Distance from center
	angleStep := (2 * math.Pi) / float64(len(buildingMap))
	if len(buildingMap) == 0 {
		angleStep = 0
	}
	currentAngle := 0.0

	for bName, floors := range buildingMap {
		bNode := BuildingNode{
			ID:   strings.ReplaceAll(strings.ToUpper(bName), " ", "_"),
			Name: bName,
			Coordinates: Coordinates{
				X: math.Round(math.Cos(currentAngle) * radius),
				Y: math.Round(math.Sin(currentAngle) * radius),
			},
			Floors: []FloorNode{},
		}

		zIdx := 0
		// 'pIDs' is now the array of Probe IDs for this specific floor
		for fName, pIDs := range floors {
			bNode.Floors = append(bNode.Floors, FloorNode{
				Level:      parseFloorLevel(fName),
				FloorID:    fName,
				ZIndex:     zIdx,
				ProbeCount: len(pIDs), // Count is now just the length of the array
				Probes:     pIDs,      // Assign the array of IDs here!
			})
			zIdx++
		}

		layout.Buildings = append(layout.Buildings, bNode)
		currentAngle += angleStep
	}

	return layout, nil
}

// GetHeatmap aggregates telemetry (e.g., RSSI, latency) to calculate color codes for the UI squares.
func (s *TopologyService) GetHeatmap(ctx context.Context, metric string) (*HeatmapResponse, error) {
	probes, err := s.probeRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch probes for heatmap: %w", err)
	}

	// Group probes by Building -> Floor
	floorProbes := make(map[string]map[string][]string)
	for _, p := range probes {
		bName := p.Building
		fName := p.Floor
		if bName == "" {
			bName = "Unknown Building"
		}
		if fName == "" {
			fName = "Ground"
		}

		if _, exists := floorProbes[bName]; !exists {
			floorProbes[bName] = make(map[string][]string)
		}
		floorProbes[bName][fName] = append(floorProbes[bName][fName], p.ProbeID)
	}

	heatmap := &HeatmapResponse{
		Timestamp:   time.Now(),
		Metric:      metric,
		HeatmapData: []FloorHealth{},
	}

	for bName, floors := range floorProbes {
		for fName, pIDs := range floors {
			health := s.calculateFloorHealth(ctx, pIDs, metric)
			health.BuildingID = strings.ReplaceAll(strings.ToUpper(bName), " ", "_")
			health.FloorID = fName
			heatmap.HeatmapData = append(heatmap.HeatmapData, health)
		}
	}

	return heatmap, nil
}

// GetFloorDetails provides the drill-down view for the side panel when a floor is clicked.
func (s *TopologyService) GetFloorDetails(ctx context.Context, building string, floor string) (*FloorDetails, error) {
	probes, err := s.probeRepo.GetByBuildingAndFloor(ctx, building, floor)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch probes for floor details: %w", err)
	}

	details := &FloorDetails{
		Building: building,
		Floor:    floor,
		Probes:   []ProbeDetail{},
	}

	worstStatus := "HEALTHY"

	for _, p := range probes {
		pd := ProbeDetail{
			ProbeID:  p.ProbeID,
			Status:   p.Status,
			LastSeen: p.LastSeen,
		}

		// Fetch latest telemetry mapped exactly to your hypertable schema
		if tel, err := s.telemetryRepo.GetLatestByProbe(ctx, p.ProbeID); err == nil && tel != nil {
			pd.CurrentMetrics = map[string]interface{}{
				"rssi":         tel.RSSI,
				"latency":      tel.Latency,
				"packet_loss":  tel.PacketLoss,
				"throughput":   tel.Throughput,
				"link_quality": tel.LinkQuality,
			}
		}

		// Attach active alerts
		if alerts, err := s.alertRepo.GetActiveByProbe(ctx, p.ProbeID); err == nil {
			pd.ActiveAlerts = alerts
			if len(alerts) > 0 {
				worstStatus = "WARNING"
				for _, a := range alerts {
					if a.Severity == "CRITICAL" {
						worstStatus = "CRITICAL"
					}
				}
			}
		} else {
			pd.ActiveAlerts = []models.Alert{}
		}

		details.Probes = append(details.Probes, pd)
	}

	details.OverallHealth = worstStatus
	return details, nil
}

func (s *TopologyService) calculateFloorHealth(ctx context.Context, probeIDs []string, metric string) FloorHealth {
	health := FloorHealth{
		Status:       "OFFLINE",
		ColorHex:     "#52525b", // Zinc-600 (Offline/Unknown)
		AverageValue: 0,
		ActiveAlerts: 0,
	}

	if len(probeIDs) == 0 {
		return health
	}

	var totalValue float64
	var validReadings int

	for _, pid := range probeIDs {
		// 1. Check for alerts
		if alerts, err := s.alertRepo.GetActiveByProbe(ctx, pid); err == nil {
			health.ActiveAlerts += len(alerts)
		}

		// 2. Fetch recent telemetry from hypertable
		if tel, err := s.telemetryRepo.GetLatestByProbe(ctx, pid); err == nil && tel != nil {
			// Skip stale data (older than 15 mins)
			if time.Since(tel.Timestamp) > 15*time.Minute {
				continue
			}

			switch metric {
			case "signal", "rssi":
				// RSSI is usually stored as a negative integer (e.g., -60)
				if tel.RSSI != nil {
					totalValue += float64(*tel.RSSI)
					validReadings++
				}
			case "latency":
				if tel.Latency != nil {
					totalValue += float64(*tel.Latency)
					validReadings++
				}
			case "packet_loss":
				totalValue += float64(*tel.PacketLoss)
				validReadings++
			}
		}
	}

	// 3. Determine Color and Status based on aggregated metric
	if validReadings > 0 {
		health.AverageValue = totalValue / float64(validReadings)

		switch metric {
		case "signal", "rssi":
			// RSSI logic: closer to 0 is better. -50 is excellent, -90 is terrible.
			if health.AverageValue >= -65 {
				health.Status, health.ColorHex = "HEALTHY", "#10b981" // Emerald
			} else if health.AverageValue >= -80 {
				health.Status, health.ColorHex = "WARNING", "#f59e0b" // Amber
			} else {
				health.Status, health.ColorHex = "CRITICAL", "#ef4444" // Red
			}
		case "latency":
			if health.AverageValue <= 50 {
				health.Status, health.ColorHex = "HEALTHY", "#10b981"
			} else if health.AverageValue <= 150 {
				health.Status, health.ColorHex = "WARNING", "#f59e0b"
			} else {
				health.Status, health.ColorHex = "CRITICAL", "#ef4444"
			}
		case "packet_loss":
			if health.AverageValue <= 1.0 {
				health.Status, health.ColorHex = "HEALTHY", "#10b981"
			} else if health.AverageValue <= 5.0 {
				health.Status, health.ColorHex = "WARNING", "#f59e0b"
			} else {
				health.Status, health.ColorHex = "CRITICAL", "#ef4444"
			}
		default:
			// Fallback generic color
			health.Status, health.ColorHex = "UNKNOWN", "#3b82f6" // Blue
		}
	} else if health.ActiveAlerts > 0 {
		// Fallback: No recent telemetry, but active alerts exist
		health.Status, health.ColorHex = "WARNING", "#f59e0b"
	}

	// Active critical alerts override normal health colors
	if health.ActiveAlerts > 0 && health.Status == "HEALTHY" {
		health.Status, health.ColorHex = "WARNING", "#f59e0b"
	}

	return health
}

func parseFloorLevel(floorStr string) int {
	lower := strings.ToLower(strings.TrimSpace(floorStr))
	if strings.Contains(lower, "ground") || lower == "g" {
		return 0
	}
	if strings.Contains(lower, "basement") || strings.Contains(lower, "b") {
		return -1
	}

	// Extract the first consecutive block of numbers (e.g., "Floor 2" -> 2)
	for _, word := range strings.Fields(floorStr) {
		numericOnly := strings.TrimFunc(word, func(r rune) bool {
			return r < '0' || r > '9'
		})
		if num, err := strconv.Atoi(numericOnly); err == nil {
			return num
		}
	}
	return 1
}
