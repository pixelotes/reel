package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"reel/internal/config"
	"reel/internal/core"
	"reel/internal/utils"
	"reel/web"

	"github.com/gorilla/mux"
)

type Server struct {
	config     *config.Config
	manager    *core.Manager
	logger     *utils.Logger
	httpServer *http.Server
	apiHandler *APIHandler
}

func NewServer(cfg *config.Config, manager *core.Manager, logger *utils.Logger) *Server {
	return &Server{
		config:     cfg,
		manager:    manager,
		logger:     logger,
		apiHandler: NewAPIHandler(manager, logger),
	}
}

func (s *Server) Start() error {
	router := mux.NewRouter()

	// API routes
	api := router.PathPrefix("/api/v1").Subrouter()

	// Auth
	api.HandleFunc("/login", s.apiHandler.Login).Methods("POST")

	// Protected routes (add auth middleware in production)
	protected := api.PathPrefix("").Subrouter()
	// protected.Use(s.authMiddleware) // Implement JWT middleware

	protected.HandleFunc("/media", s.apiHandler.GetMedia).Methods("GET")
	protected.HandleFunc("/media", s.apiHandler.AddMedia).Methods("POST")
	protected.HandleFunc("/media/{id}", s.apiHandler.DeleteMedia).Methods("DELETE")
	protected.HandleFunc("/media/{id}/retry", s.apiHandler.RetryMedia).Methods("POST")
	protected.HandleFunc("/media/{id}/search", s.apiHandler.ManualSearch).Methods("GET")
	protected.HandleFunc("/media/{id}/download", s.apiHandler.ManualDownload).Methods("POST")
	protected.HandleFunc("/media/{id}/tv-details", s.apiHandler.GetTVShowDetails).Methods("GET")
	protected.HandleFunc("/media/clear-failed", s.apiHandler.ClearFailed).Methods("POST")
	protected.HandleFunc("/search-metadata", s.apiHandler.SearchMetadata).Methods("GET")
	protected.HandleFunc("/status", s.apiHandler.GetSystemStatus).Methods("GET")
	protected.HandleFunc("/test/indexer", s.apiHandler.TestIndexer).Methods("GET")
	protected.HandleFunc("/test/torrent", s.apiHandler.TestTorrent).Methods("GET")

	// Web UI (if enabled)
	if s.config.App.UIEnabled {
		router.PathPrefix("/").Handler(http.FileServer(http.FS(web.Files)))
	}

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.App.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	s.logger.Info("Starting server on port", s.config.App.Port)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
