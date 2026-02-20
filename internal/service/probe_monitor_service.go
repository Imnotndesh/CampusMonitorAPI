package service

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/mqtt"
	"CampusMonitorAPI/internal/repository"
)

type ProbeMonitor struct {
	mqttClient *mqtt.Client
	probeRepo  *repository.ProbeRepository
	log        *logger.Logger

	probeStatus map[string]*ProbeStatusCache
	probeConfig map[string]*ProbeConfigCache
	pingStatus  map[string]*PingStatus

	statusMux sync.RWMutex
	configMux sync.RWMutex
	pingMux   sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type ProbeStatusCache struct {
	ProbeID   string    `json:"probe_id"`
	Uptime    int64     `json:"uptime"`
	FreeHeap  int       `json:"free_heap"`
	RSSI      int       `json:"rssi"`
	IP        string    `json:"ip"`
	SSID      string    `json:"ssid"`
	TempC     float64   `json:"temp_c"`
	Timestamp string    `json:"timestamp"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ProbeConfigCache struct {
	ProbeID   string                 `json:"probe_id"`
	WiFi      map[string]interface{} `json:"wifi"`
	MQTT      map[string]interface{} `json:"mqtt"`
	HeapFree  int                    `json:"heap_free"`
	Uptime    int64                  `json:"uptime"`
	TempC     float64                `json:"temp_c"`
	Timestamp string                 `json:"timestamp"`
	UpdatedAt time.Time              `json:"updated_at"`
}

type PingStatus struct {
	Online    bool      `json:"online"`
	LastSeen  time.Time `json:"last_seen"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewProbeMonitor(mqttClient *mqtt.Client, probeRepo *repository.ProbeRepository, log *logger.Logger) *ProbeMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &ProbeMonitor{
		mqttClient:  mqttClient,
		probeRepo:   probeRepo,
		log:         log,
		probeStatus: make(map[string]*ProbeStatusCache),
		probeConfig: make(map[string]*ProbeConfigCache),
		pingStatus:  make(map[string]*PingStatus),
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (pm *ProbeMonitor) Start() {
	pm.log.Info("Starting Probe Monitor")

	// Subscribe to broadcast topics
	pm.wg.Add(1)
	go pm.subscribeToStatusBroadcasts()

	pm.wg.Add(1)
	go pm.subscribeToConfigBroadcasts()

	// Stale data cleanup worker
	pm.wg.Add(1)
	go pm.staleDataCleanup()

	pm.log.Info("Probe Monitor started successfully")
}

func (pm *ProbeMonitor) Shutdown() {
	pm.log.Info("Shutting down Probe Monitor...")
	pm.cancel()
	pm.wg.Wait()
	pm.log.Info("Probe Monitor stopped gracefully")
}

func (pm *ProbeMonitor) subscribeToStatusBroadcasts() {
	defer pm.wg.Done()

	topic := "campus/probes/+/status"
	pm.log.Info("Subscribing to status broadcasts: %s", topic)

	ch, err := pm.mqttClient.SubscribeChannel(topic)
	if err != nil {
		pm.log.Error("Failed to subscribe to status broadcasts: %v", err)
		return
	}

	for {
		select {
		case <-pm.ctx.Done():
			pm.log.Info("Status broadcast subscriber stopping")
			return
		case msg := <-ch:
			pm.handleStatusBroadcast(msg.Topic, msg.Payload)
		}
	}
}

func (pm *ProbeMonitor) subscribeToConfigBroadcasts() {
	defer pm.wg.Done()

	topic := "campus/probes/+/config"
	pm.log.Info("Subscribing to config broadcasts: %s", topic)

	ch, err := pm.mqttClient.SubscribeChannel(topic)
	if err != nil {
		pm.log.Error("Failed to subscribe to config broadcasts: %v", err)
		return
	}

	for {
		select {
		case <-pm.ctx.Done():
			pm.log.Info("Config broadcast subscriber stopping")
			return
		case msg := <-ch:
			pm.handleConfigBroadcast(msg.Topic, msg.Payload)
		}
	}
}

func (pm *ProbeMonitor) handleStatusBroadcast(topic string, payload []byte) {
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		pm.log.Error("Failed to unmarshal status broadcast: %v", err)
		return
	}

	probeID, ok := data["probe_id"].(string)
	if !ok {
		pm.log.Warn("Status broadcast missing probe_id")
		return
	}

	status := &ProbeStatusCache{
		ProbeID:   probeID,
		UpdatedAt: time.Now(),
	}

	if uptime, ok := data["uptime"].(float64); ok {
		status.Uptime = int64(uptime)
	}
	if heap, ok := data["free_heap"].(float64); ok {
		status.FreeHeap = int(heap)
	}
	if rssi, ok := data["rssi"].(float64); ok {
		status.RSSI = int(rssi)
	}
	if ip, ok := data["ip"].(string); ok {
		status.IP = ip
	}
	if ssid, ok := data["ssid"].(string); ok {
		status.SSID = ssid
	}
	if temp, ok := data["temp_c"].(float64); ok {
		status.TempC = temp
	}
	if ts, ok := data["timestamp"].(string); ok {
		status.Timestamp = ts
	}

	pm.statusMux.Lock()
	pm.probeStatus[probeID] = status
	pm.statusMux.Unlock()

	// Update last_seen in database
	go pm.probeRepo.UpdateLastSeen(context.Background(), probeID, time.Now())

	// Mark as online in ping status
	pm.setPingStatus(probeID, true)

	pm.log.Debug("Cached status broadcast from %s", probeID)
}

func (pm *ProbeMonitor) handleConfigBroadcast(topic string, payload []byte) {
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		pm.log.Error("Failed to unmarshal config broadcast: %v", err)
		return
	}

	probeID, ok := data["probe_id"].(string)
	if !ok {
		pm.log.Warn("Config broadcast missing probe_id")
		return
	}

	config := &ProbeConfigCache{
		ProbeID:   probeID,
		UpdatedAt: time.Now(),
	}

	if wifi, ok := data["wifi"].(map[string]interface{}); ok {
		config.WiFi = wifi
	}
	if mqtt, ok := data["mqtt"].(map[string]interface{}); ok {
		config.MQTT = mqtt
	}
	if heap, ok := data["heap_free"].(float64); ok {
		config.HeapFree = int(heap)
	}
	if uptime, ok := data["uptime"].(float64); ok {
		config.Uptime = int64(uptime)
	}
	if temp, ok := data["temp_c"].(float64); ok {
		config.TempC = temp
	}
	if ts, ok := data["timestamp"].(string); ok {
		config.Timestamp = ts
	}

	pm.configMux.Lock()
	pm.probeConfig[probeID] = config
	pm.configMux.Unlock()

	pm.log.Debug("Cached config broadcast from %s", probeID)
}

func (pm *ProbeMonitor) staleDataCleanup() {
	defer pm.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			pm.log.Info("Stale data cleanup worker stopping")
			return
		case <-ticker.C:
			pm.cleanupStaleData()
		}
	}
}

func (pm *ProbeMonitor) cleanupStaleData() {
	now := time.Now()
	staleThreshold := 15 * time.Minute

	// Cleanup stale status
	pm.statusMux.Lock()
	for probeID, status := range pm.probeStatus {
		if now.Sub(status.UpdatedAt) > staleThreshold {
			delete(pm.probeStatus, probeID)
			pm.log.Debug("Removed stale status for %s", probeID)
		}
	}
	pm.statusMux.Unlock()

	// Cleanup stale config
	pm.configMux.Lock()
	for probeID, config := range pm.probeConfig {
		if now.Sub(config.UpdatedAt) > staleThreshold {
			delete(pm.probeConfig, probeID)
			pm.log.Debug("Removed stale config for %s", probeID)
		}
	}
	pm.configMux.Unlock()

	// Mark offline probes
	pm.pingMux.Lock()
	for probeID, ping := range pm.pingStatus {
		if now.Sub(ping.LastSeen) > 3*time.Minute {
			pm.pingStatus[probeID] = &PingStatus{
				Online:    false,
				LastSeen:  ping.LastSeen,
				UpdatedAt: now,
			}
		}
	}
	pm.pingMux.Unlock()
}

func (pm *ProbeMonitor) setPingStatus(probeID string, online bool) {
	pm.pingMux.Lock()
	defer pm.pingMux.Unlock()

	pm.pingStatus[probeID] = &PingStatus{
		Online:    online,
		LastSeen:  time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Getters
func (pm *ProbeMonitor) GetProbeStatus(probeID string) *ProbeStatusCache {
	pm.statusMux.RLock()
	defer pm.statusMux.RUnlock()
	return pm.probeStatus[probeID]
}

func (pm *ProbeMonitor) GetProbeConfig(probeID string) *ProbeConfigCache {
	pm.configMux.RLock()
	defer pm.configMux.RUnlock()
	return pm.probeConfig[probeID]
}

func (pm *ProbeMonitor) GetPingStatus(probeID string) *PingStatus {
	pm.pingMux.RLock()
	defer pm.pingMux.RUnlock()

	status, exists := pm.pingStatus[probeID]
	if !exists {
		return &PingStatus{Online: false, UpdatedAt: time.Now()}
	}
	return status
}
