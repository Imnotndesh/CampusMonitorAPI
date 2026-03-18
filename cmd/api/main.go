package main

import (
	"CampusMonitorAPI/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"CampusMonitorAPI/internal/config"
	"CampusMonitorAPI/internal/database"
	"CampusMonitorAPI/internal/handler"
	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/mqtt"
	"CampusMonitorAPI/internal/repository"
	"CampusMonitorAPI/internal/server"
	"CampusMonitorAPI/internal/service"

	"golang.org/x/oauth2"
)

func main() {
	// 1. Load Config
	cfg, err := config.Load()
	if err != nil {
		// Fallback logger since main logger isn't ready
		panic("Failed to load configuration: " + err.Error())
	}

	// 2. Initialize Logger
	log, err := logger.New(logger.Config{
		Level:       cfg.Logging.Level,
		Mode:        cfg.Logging.Mode,
		LogFilePath: cfg.Logging.FilePath,
		UseColors:   cfg.Logging.UseColors,
	})
	if err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer log.Close()

	if err := cfg.Validate(); err != nil {
		log.Fatal("Configuration validation failed: %v", err)
	}

	cfg.Print()
	log.Info("Starting Campus Monitor API Server")

	// 3. Database Connection
	db, err := database.New(&cfg.Database)
	if err != nil {
		log.Fatal("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Info("Database connected successfully")

	ctx := context.Background()
	if err := db.Health(ctx); err != nil {
		log.Fatal("Database health check failed: %v", err)
	}

	// 4. Initialize Repositories
	probeRepo := repository.NewProbeRepository(db.DB)
	telemetryRepo := repository.NewTelemetryRepository(db.DB)
	commandRepo := repository.NewCommandRepository(db.DB)
	alertRepo := repository.NewAlertRepository(db.DB)
	analyticsRepo := repository.NewAnalyticsRepository(db.DB)
	fleetRepo := repository.NewFleetRepository(db.DB)
	scheduleRepo := repository.NewScheduleRepository(db.DB)
	userRepo := repository.NewUserRepository(db.DB)
	oauthAccountRepo := repository.NewOAuthAccountRepository(db.DB)
	totpRepo := repository.NewTOTPRepository(db.DB)
	refreshTokenRepo := repository.NewRefreshTokenRepository(db.DB)
	oauthStateRepo := repository.NewOAuthStateRepository(db.DB)

	oauthConfigs := make(map[string]*oauth2.Config)
	for provider, pcfg := range cfg.Auth.OAuthProviders {
		oauthConfigs[provider] = &oauth2.Config{
			ClientID:     pcfg.ClientID,
			ClientSecret: pcfg.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  pcfg.AuthURL,
				TokenURL: pcfg.TokenURL,
			},
			RedirectURL: fmt.Sprintf("%s/api/v1/auth/oauth/%s/callback", cfg.Server.PublicURL, provider),
			Scopes:      pcfg.Scopes,
		}
	}
	// 5. Initialize MQTT Client
	mqttClient, err := mqtt.NewClient(mqtt.ClientConfig{
		MQTT:   &cfg.MQTT,
		Logger: log,
	})
	if err != nil {
		log.Fatal("Failed to create MQTT client: %v", err)
	}
	defer func(mqttClient *mqtt.Client) {
		err := mqttClient.Disconnect()
		if err != nil {
			log.Error("Failed to disconnect MQTT: %v", err)
		}
	}(mqttClient)
	srv := server.New(cfg, log)

	if err := mqttClient.Connect(); err != nil {
		log.Fatal("Failed to connect to MQTT broker: %v", err)
	}
	alertService := service.NewAlertService(alertRepo, srv.GetHub())
	alertEvaluator := service.NewAlertEvaluator(models.DEFAULT_ALERT_CONFIG, alertService)
	scheduleService := service.NewScheduleService(scheduleRepo, probeRepo, mqttClient, log)
	telemetryService := service.NewTelemetryService(telemetryRepo, probeRepo, alertEvaluator, log)
	probeService := service.NewProbeService(probeRepo, log)
	analyticsService := service.NewAnalyticsService(analyticsRepo, log)
	authService := service.NewAuthService(
		userRepo, oauthAccountRepo, totpRepo, refreshTokenRepo, oauthStateRepo,
		&cfg.Auth, log, oauthConfigs,
	)
	fleetService := service.NewFleetService(
		fleetRepo,
		probeRepo,
		commandRepo,
		telemetryRepo,
		alertRepo,
		mqttClient,
		log,
	)
	commandService := service.NewCommandService(commandRepo, mqttClient, probeRepo, telemetryService, fleetService, scheduleService, log)
	topologyService := service.NewTopologyService(probeRepo, telemetryRepo, alertRepo)

	// MQTT Subscriptions
	// Telemetry
	if err := mqttClient.Subscribe(cfg.MQTT.TelemetryTopic, handleTelemetry(telemetryService, log)); err != nil {
		log.Fatal("Failed to subscribe to telemetry topic: %v", err)
	}

	// Offline Telemetry
	if err := mqttClient.Subscribe("campus/probes/telemetry/offline", handleOfflineTelemetry(telemetryService, log)); err != nil {
		log.Fatal("Failed to subscribe to offline telemetry topic: %v", err)
	}
	// Command results
	if err := mqttClient.Subscribe("campus/probes/+/result", handleCommandResult(commandService, log)); err != nil {
		log.Fatal("Failed to subscribe to command results topic: %v", err)
	}
	if err := mqttClient.Subscribe("campus/fleet/status/+", handleFleetStatus(probeService, fleetService, log)); err != nil {
		log.Fatal("Failed to subscribe to fleet status topic: %v", err)
	}
	// Fleet Schedules
	if err := mqttClient.Subscribe("campus/fleet/schedules/status/+", handleScheduleStatus(fleetService, log)); err != nil {
		log.Fatal("Failed to subscribe to schedule status topic: %v", err)
	}

	log.Info("MQTT subscriptions active")

	log.Info("Started background monitors")
	probeMonitor := service.NewProbeMonitor(mqttClient, probeRepo, log)
	probeMonitor.Start()

	// 8. Initialize Handlers
	probeHandler := handler.NewProbeHandler(probeService, commandService, probeMonitor, log)
	telemetryHandler := handler.NewTelemetryHandler(telemetryService, log)
	commandHandler := handler.NewCommandHandler(commandService, log)
	analyticsHandler := handler.NewAnalyticsHandler(analyticsService, log)
	healthHandler := handler.NewHealthHandler(db, mqttClient, log)
	alertHandler := handler.NewAlertHandler(alertService, log)
	topologyHandler := handler.NewTopologyHandler(topologyService, log)
	authHandler := handler.NewAuthHandler(authService, log)
	fleetHandler := handler.NewFleetHandler(
		fleetService,
		probeService,
		commandService,
		log,
	)
	scheduleHandler := handler.NewScheduleHandler(scheduleService, log)

	// 9. Start HTTP Server
	srv.RegisterHandlers(
		probeHandler,
		telemetryHandler,
		commandHandler,
		analyticsHandler,
		healthHandler,
		topologyHandler,
		alertHandler,
		fleetHandler,
		scheduleHandler,
		authHandler,
	)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := srv.Start(ctx); err != nil {
		log.Fatal("Server failed: %v", err)
	}

	log.Info("API server ready")

	// 10. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Warn("Shutdown signal received")
	probeMonitor.Shutdown()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Server shutdown error: %v", err)
	}

	log.Info("Shutdown complete")
}

func handleTelemetry(service *service.TelemetryService, log *logger.Logger) mqtt.MessageHandler {
	return func(topic string, payload []byte) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := service.ProcessMessage(ctx, payload); err != nil {
			log.Error("Failed to process telemetry: %v", err)
			return err
		}
		return nil
	}
}

func handleOfflineTelemetry(service *service.TelemetryService, log *logger.Logger) mqtt.MessageHandler {
	return func(topic string, payload []byte) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		log.Info("Processing offline telemetry")
		if err := service.ProcessMessage(ctx, payload); err != nil {
			log.Error("Failed to process offline telemetry: %v", err)
			return err
		}
		return nil
	}
}
func handleScheduleStatus(service *service.FleetService, log *logger.Logger) mqtt.MessageHandler {
	return func(topic string, payload []byte) error {
		parts := strings.Split(topic, "/")
		if len(parts) < 5 {
			return fmt.Errorf("invalid topic format")
		}
		probeID := parts[len(parts)-1]

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := service.UpdateProbeSchedules(ctx, probeID, payload); err != nil {
			log.Error("Failed to update probe schedules: %v", err)
			return err
		}
		return nil
	}
}

func handleCommandResult(service *service.CommandService, log *logger.Logger) mqtt.MessageHandler {
	return func(topic string, payload []byte) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := service.ProcessCommandResult(ctx, payload); err != nil {
			log.Error("Failed to process command result: %v", err)
			return err
		}
		return nil
	}
}
func handleFleetStatus(probeService *service.ProbeService, fleetService *service.FleetService, log *logger.Logger) mqtt.MessageHandler {
	return func(topic string, payload []byte) error {
		parts := strings.Split(topic, "/")
		if len(parts) < 4 {
			return fmt.Errorf("invalid topic format: %s", topic)
		}
		probeID := parts[len(parts)-1]

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var status map[string]interface{}
		if err := json.Unmarshal(payload, &status); err != nil {
			log.Error("Failed to parse fleet status: %v", err)
			return err
		}

		if err := probeService.UpdateLastSeen(ctx, probeID, time.Now()); err != nil {
			log.Warn("Failed to update last_seen for %s: %v", probeID, err)
		}

		fleetUpdate := &models.FleetUpdateRequest{}

		if groups, ok := status["groups"].(string); ok && groups != "" {
			groupList := strings.Split(groups, ",")
			fleetUpdate.Groups = &groupList
		}

		if location, ok := status["location"].(string); ok && location != "" {
			fleetUpdate.Location = &location
		}

		if tags, ok := status["tags"].(map[string]interface{}); ok && len(tags) > 0 {
			fleetUpdate.Tags = tags
		}

		if maintWindow, ok := status["maintenance_window"].(string); ok && maintWindow != "" {
			// Parse "02:00-04:00" format
			parts := strings.Split(maintWindow, "-")
			if len(parts) == 2 {
				fleetUpdate.MaintenanceWindow = &models.MaintenanceWindow{
					Start: parts[0],
					End:   parts[1],
				}
			}
		}

		if fleetUpdate.Groups != nil || fleetUpdate.Location != nil ||
			fleetUpdate.Tags != nil || fleetUpdate.MaintenanceWindow != nil {
			if err := fleetService.UpdateFleetProbe(ctx, probeID, fleetUpdate); err != nil {
				log.Warn("Failed to update fleet probe %s: %v", probeID, err)
			}
		}

		if fwVersion, ok := status["fw_version"].(string); ok && fwVersion != "" {
			if err := probeService.UpdateFirmwareVersion(ctx, probeID, fwVersion); err != nil {
				log.Warn("Failed to update firmware version for %s: %v", probeID, err)
			}

			if err := fleetService.UpdateFirmwareVersion(ctx, probeID, fwVersion); err != nil {
				log.Warn("Failed to update fleet firmware version for %s: %v", probeID, err)
			}
		}

		log.Debug("Fleet status processed for %s: fw=%s",
			probeID, status)

		return nil
	}
}
