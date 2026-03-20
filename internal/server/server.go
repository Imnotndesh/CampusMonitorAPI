package server

import (
	"CampusMonitorAPI/internal/config"
	"CampusMonitorAPI/internal/handler"
	"CampusMonitorAPI/internal/logger"
	"CampusMonitorAPI/internal/middleware"
	"CampusMonitorAPI/internal/websocket"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

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
	fleetHandler *handler.FleetHandler,
	scheduleHandler *handler.ScheduleHandler,
	authHandler *handler.AuthHandler,
	reportHandler *handler.ReportHandler,
) {
	// Public auth routes (no auth required)
	s.router.Use(middleware.CORS(s.cfg.Security.CORSAllowedOrigins, s.cfg.Security.CORSAllowedMethods))
	s.router.Use(middleware.Recovery(s.log))

	healthHandler.RegisterRoutes(s.router)

	authRouter := s.router.PathPrefix("/api/v1/auth").Subrouter()
	authRouter.Use(middleware.RequestLogger(s.log))
	authHandler.RegisterRoutes(authRouter)

	api := s.router.PathPrefix("/api/v1").Subrouter()
	api.Use(middleware.Auth(s.cfg.Auth.JWTSecret))
	api.Use(middleware.RequestLogger(s.log))
	if s.cfg.Security.EnableRateLimit {
		api.Use(middleware.RateLimit(s.cfg.Security.RateLimitPerMinute))
	}

	if s.cfg.Security.EnableRateLimit {
		api.Use(middleware.RateLimit(s.cfg.Security.RateLimitPerMinute))
	}
	probeHandler.RegisterRoutes(api)
	telemetryHandler.RegisterRoutes(api)
	commandHandler.RegisterRoutes(api)
	analyticsHandler.RegisterRoutes(api)
	topologyHandler.RegisterRoutes(api)
	alertHandler.RegisterRoutes(api)
	fleetHandler.RegisterRoutes(api)
	reportHandler.RegisterRoutes(api)
	scheduleHandler.RegisterRoutes(api)
	s.router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS,PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	s.router.HandleFunc("/api/v1/ws", func(w http.ResponseWriter, r *http.Request) {
		websocket.ServeWs(s.wsHub, w, r, s.log)
	}).Methods("GET")

	s.log.Info("All handlers and WebSocket endpoint registered")
}

func (s *Server) Start(ctx context.Context) error {
	go s.wsHub.Run(ctx)

	addr := s.httpServer.Addr
	if s.hasTLSCertificates() {
		s.log.Info("Starting HTTPS server on %s (TLS enabled)", addr)
	} else {
		s.log.Info("Starting HTTP server on %s (TLS disabled)", addr)
	}

	errChan := make(chan error, 1)
	go func() {
		if s.hasTLSCertificates() {
			certFile := filepath.Join(s.cfg.Server.CertDir, "cert.pem")
			keyFile := filepath.Join(s.cfg.Server.CertDir, "key.pem")
			if err := s.httpServer.ListenAndServeTLS(certFile, keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errChan <- fmt.Errorf("HTTPS server failed: %w", err)
			}
		} else {
			if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errChan <- fmt.Errorf("HTTP server failed: %w", err)
			}
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
func (s *Server) hasTLSCertificates() bool {
	if s.cfg.Server.CertDir == "" {
		return false
	}
	certFile := filepath.Join(s.cfg.Server.CertDir, "cert.pem")
	keyFile := filepath.Join(s.cfg.Server.CertDir, "key.pem")
	_, errCert := os.Stat(certFile)
	_, errKey := os.Stat(keyFile)
	return errCert == nil && errKey == nil
}
func (s *Server) GetHub() *websocket.Hub {
	return s.wsHub
}
