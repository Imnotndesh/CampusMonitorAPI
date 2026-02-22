package server

import (
	"CampusMonitorAPI/internal/config"
	"CampusMonitorAPI/internal/handler"
	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/middleware"
	"CampusMonitorAPI/internal/websocket"
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
	wsHub      *websocket.Hub
}

func New(cfg *config.Config, log *logger.Logger) *Server {
	router := mux.NewRouter()
	wsHub := websocket.NewHub(log)

	server := &Server{
		router: router,
		cfg:    cfg,
		log:    log,
		wsHub:  wsHub,
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
	topologyHandler *handler.TopologyHandler,
	alertHandler *handler.AlertHandler,
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
	topologyHandler.RegisterRoutes(api)
	alertHandler.RegisterRoutes(api)

	s.router.HandleFunc("/api/v1/ws", func(w http.ResponseWriter, r *http.Request) {
		websocket.ServeWs(s.wsHub, w, r, s.log)
	}).Methods("GET")

	s.log.Info("All handlers and WebSocket endpoint registered")
	s.log.Info("All handlers registered")
}

func (s *Server) Start(ctx context.Context) error {
	go s.wsHub.Run(ctx)

	s.log.Info("Starting HTTP server on %s", s.httpServer.Addr)
	errChan := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("server failed to start: %w", err)
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		s.log.Info("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.Server.ShutdownTimeout)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("Shutting down HTTP server...")

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	s.log.Info("HTTP server stopped")
	return nil
}
func (s *Server) GetHub() *websocket.Hub {
	return s.wsHub
}
