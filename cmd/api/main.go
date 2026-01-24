package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"CampusMonitorAPI/internal/config"
	"CampusMonitorAPI/internal/database"
	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/mqtt"
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

	log.Info("Database health check passed")

	mqttClient, err := mqtt.NewClient(mqtt.ClientConfig{
		MQTT:   &cfg.MQTT,
		Logger: log,
	})
	if err != nil {
		log.Fatal("Failed to create MQTT client: %v", err)
	}
	defer mqttClient.Disconnect()

	if err := mqttClient.Connect(); err != nil {
		log.Fatal("Failed to connect to MQTT broker: %v", err)
	}

	log.Info("Subscribing to: %s", cfg.MQTT.TelemetryTopic)
	if err := mqttClient.Subscribe(cfg.MQTT.TelemetryTopic, handleTelemetry(log)); err != nil {
		log.Fatal("Failed to subscribe to telemetry topic: %v", err)
	}

	log.Info("Subscribing to: campus/probes/telemetry/offline")
	if err := mqttClient.Subscribe("campus/probes/telemetry/offline", handleOfflineTelemetry(log)); err != nil {
		log.Fatal("Failed to subscribe to offline telemetry topic: %v", err)
	}

	log.Info("MQTT subscriptions active - waiting for messages...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	log.Info("API server ready (press Ctrl+C to shutdown)")
	<-quit

	log.Warn("Shutdown signal received")
	time.Sleep(cfg.Server.ShutdownTimeout)
	log.Info("Shutdown complete")
}

func handleTelemetry(log *logger.Logger) mqtt.MessageHandler {
	return func(topic string, payload []byte) error {
		log.Info("TELEMETRY RECEIVED on %s", topic)
		log.Info("Raw payload: %s", string(payload))

		var data map[string]interface{}
		if err := json.Unmarshal(payload, &data); err != nil {
			log.Error("Failed to parse telemetry JSON: %v", err)
			return err
		}

		log.Info("Parsed telemetry: ProbeID=%s, Type=%s, RSSI=%v",
			data["pid"], data["type"], data["rssi"])

		return nil
	}
}

func handleOfflineTelemetry(log *logger.Logger) mqtt.MessageHandler {
	return func(topic string, payload []byte) error {
		log.Warn("OFFLINE TELEMETRY RECEIVED on %s", topic)
		log.Info("Offline payload: %s", string(payload))
		return nil
	}
}
