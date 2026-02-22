package main

import (
	"CampusMonitorAPI/internal/models"
	"context"
	"os"
	"os/signal"
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
	telemetryService := service.NewTelemetryService(telemetryRepo, probeRepo, alertEvaluator, log)
	probeService := service.NewProbeService(probeRepo, log)
	analyticsService := service.NewAnalyticsService(analyticsRepo, log)
	commandService := service.NewCommandService(commandRepo, mqttClient, probeRepo, telemetryService, log)
	topologyService := service.NewTopologyService(probeRepo, telemetryRepo, alertRepo)

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
	// Background pinging service

	// 9. Start HTTP Server
	srv.RegisterHandlers(
		probeHandler,
		telemetryHandler,
		commandHandler,
		analyticsHandler,
		healthHandler,
		topologyHandler,
		alertHandler,
	)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := srv.Start(ctx); err != nil {
		log.Fatal("Server failed: %v", err)
	}

	log.Info("API server ready on http://%s:%d", cfg.Server.Host, cfg.Server.Port)

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
