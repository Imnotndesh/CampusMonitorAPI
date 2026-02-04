package main

import (
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
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration: %v", err)
	}

	log, err := logger.New(logger.Config{
		Level:       cfg.Logging.Level,
		Mode:        cfg.Logging.Mode,
		LogFilePath: cfg.Logging.FilePath,
		UseColors:   cfg.Logging.UseColors,
	})
	if err != nil {
		logger.Fatal("Failed to initialize logger: %v", err)
	}
	defer log.Close()

	if err := cfg.Validate(); err != nil {
		log.Fatal("Configuration validation failed: %v", err)
	}

	cfg.Print()
	log.Info("Starting Campus Monitor API Server")

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

	probeRepo := repository.NewProbeRepository(db.DB)
	telemetryRepo := repository.NewTelemetryRepository(db.DB)
	commandRepo := repository.NewCommandRepository(db.DB)
	repository.NewAlertRepository(db.DB)
	analyticsRepo := repository.NewAnalyticsRepository(db.DB)

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
			return
		}
	}(mqttClient)

	if err := mqttClient.Connect(); err != nil {
		log.Fatal("Failed to connect to MQTT broker: %v", err)
	}

	telemetryService := service.NewTelemetryService(telemetryRepo, probeRepo, log)
	probeService := service.NewProbeService(probeRepo, log)
	commandService := service.NewCommandService(commandRepo, mqttClient, log)
	analyticsService := service.NewAnalyticsService(analyticsRepo, log)

	if err := mqttClient.Subscribe(cfg.MQTT.TelemetryTopic, handleTelemetry(telemetryService, log)); err != nil {
		log.Fatal("Failed to subscribe to telemetry topic: %v", err)
	}

	if err := mqttClient.Subscribe("campus/probes/telemetry/offline", handleOfflineTelemetry(telemetryService, log)); err != nil {
		log.Fatal("Failed to subscribe to offline telemetry topic: %v", err)
	}

	log.Info("MQTT subscriptions active")

	probeHandler := handler.NewProbeHandler(probeService, log)
	telemetryHandler := handler.NewTelemetryHandler(telemetryService, log)
	commandHandler := handler.NewCommandHandler(commandService, log)
	analyticsHandler := handler.NewAnalyticsHandler(analyticsService, log)
	healthHandler := handler.NewHealthHandler(db, mqttClient, log)

	srv := server.New(cfg, log)
	srv.RegisterHandlers(probeHandler, telemetryHandler, commandHandler, analyticsHandler, healthHandler)

	go func() {
		if err := srv.Start(); err != nil {
			log.Fatal("Server failed: %v", err)
		}
	}()

	log.Info("API server ready on http://%s:%d", cfg.Server.Host, cfg.Server.Port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Warn("Shutdown signal received")

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
