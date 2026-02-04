package service

import (
	"context"
	"time"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/repository"
)

type AnalyticsService struct {
	analyticsRepo *repository.AnalyticsRepository
	log           *logger.Logger
}

func NewAnalyticsService(
	analyticsRepo *repository.AnalyticsRepository,
	log *logger.Logger,
) *AnalyticsService {
	return &AnalyticsService{
		analyticsRepo: analyticsRepo,
		log:           log,
	}
}

func (s *AnalyticsService) GetRSSITimeSeries(ctx context.Context, probeID string, start, end time.Time, interval string) ([]repository.TimeSeriesPoint, error) {
	s.log.Debug("Getting RSSI time series: probe=%s, interval=%s", probeID, interval)
	return s.analyticsRepo.GetRSSITimeSeries(ctx, probeID, start, end, interval)
}

func (s *AnalyticsService) GetLatencyTimeSeries(ctx context.Context, probeID string, start, end time.Time, interval string) ([]repository.TimeSeriesPoint, error) {
	s.log.Debug("Getting latency time series: probe=%s, interval=%s", probeID, interval)
	return s.analyticsRepo.GetLatencyTimeSeries(ctx, probeID, start, end, interval)
}

func (s *AnalyticsService) GetHeatmapData(ctx context.Context, start, end time.Time) ([]repository.HeatmapData, error) {
	s.log.Debug("Getting heatmap data")
	return s.analyticsRepo.GetHeatmapData(ctx, start, end)
}

func (s *AnalyticsService) GetChannelDistribution(ctx context.Context, start, end time.Time) ([]repository.ChannelDistribution, error) {
	s.log.Debug("Getting channel distribution")
	return s.analyticsRepo.GetChannelDistribution(ctx, start, end)
}

func (s *AnalyticsService) GetAPAnalysis(ctx context.Context, start, end time.Time) ([]repository.APAnalysis, error) {
	s.log.Debug("Getting AP analysis")
	return s.analyticsRepo.GetAPAnalysis(ctx, start, end)
}

func (s *AnalyticsService) GetCongestionAnalysis(ctx context.Context, start, end time.Time) ([]repository.CongestionAnalysis, error) {
	s.log.Debug("Getting congestion analysis")
	return s.analyticsRepo.GetCongestionAnalysis(ctx, start, end)
}

func (s *AnalyticsService) GetPerformanceMetrics(ctx context.Context, probeID string, start, end time.Time) (*repository.PerformanceMetrics, error) {
	s.log.Debug("Getting performance metrics: probe=%s", probeID)
	return s.analyticsRepo.GetPerformanceMetrics(ctx, probeID, start, end)
}

func (s *AnalyticsService) GetProbeComparison(ctx context.Context, probeIDs []string, start, end time.Time) ([]repository.ProbeComparison, error) {
	s.log.Debug("Comparing probes: %v", probeIDs)
	return s.analyticsRepo.GetProbeComparison(ctx, probeIDs, start, end)
}

func (s *AnalyticsService) GetNetworkHealth(ctx context.Context) (*repository.NetworkHealth, error) {
	s.log.Debug("Getting network health")
	return s.analyticsRepo.GetNetworkHealth(ctx)
}

func (s *AnalyticsService) DetectAnomalies(ctx context.Context, probeID string, hours int) ([]repository.AnomalyDetection, error) {
	s.log.Info("Detecting anomalies: probe=%s, hours=%d", probeID, hours)
	return s.analyticsRepo.DetectAnomalies(ctx, probeID, hours)
}

func (s *AnalyticsService) GetRoamingAnalysis(ctx context.Context, probeID string, start, end time.Time) ([]repository.APAnalysis, error) {
	s.log.Debug("Getting roaming analysis: probe=%s", probeID)
	return s.analyticsRepo.GetRoamingAnalysis(ctx, probeID, start, end)
}
