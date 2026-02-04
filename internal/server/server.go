// internal/server/server.go

package server

import (
	"CampusMonitorAPI/internal/config"
	"CampusMonitorAPI/internal/handler"
	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/middleware"
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

type Server struct {
	httpServer *http.Server
	router     *mux.Router
	cfg        *config.Config
	log        *logger.Logger
}

func New(cfg *config.Config, log *logger.Logger) *Server {
	router := mux.NewRouter()

	server := &Server{
		router: router,
		cfg:    cfg,
		log:    log,
		httpServer: &http.Server{
			Addr:           fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
			Handler:        router,
			ReadTimeout:    cfg.Server.ReadTimeout,
			WriteTimeout:   cfg.Server.WriteTimeout,
			MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
		},
	}

	return server
}

func (s *Server) RegisterHandlers(
	probeHandler *handler.ProbeHandler,
	telemetryHandler *handler.TelemetryHandler,
	commandHandler *handler.CommandHandler,
	analyticsHandler *handler.AnalyticsHandler,
	healthHandler *handler.HealthHandler,
) {
	api := s.router.PathPrefix("/api/v1").Subrouter()

	api.Use(middleware.RequestLogger(s.log))
	api.Use(middleware.CORS(s.cfg.Security.CORSAllowedOrigins, s.cfg.Security.CORSAllowedMethods))
	api.Use(middleware.Recovery(s.log))

	if s.cfg.Security.EnableRateLimit {
		api.Use(middleware.RateLimit(s.cfg.Security.RateLimitPerMinute))
	}

	probeHandler.RegisterRoutes(api)
	telemetryHandler.RegisterRoutes(api)
	commandHandler.RegisterRoutes(api)
	analyticsHandler.RegisterRoutes(api)
	healthHandler.RegisterRoutes(s.router)

	s.log.Info("All handlers registered")
}

func (s *Server) Start() error {
	s.log.Info("Starting HTTP server on %s", s.httpServer.Addr)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed to start: %w", err)
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("Shutting down HTTP server...")

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	s.log.Info("HTTP server stopped")
	return nil
}
