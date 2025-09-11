package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"reel/internal/config"
	"reel/internal/core"
	"reel/internal/database"
	"reel/internal/handlers"
	"reel/internal/utils"
)

func main() {
	configPath := flag.String("config", "config.yml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Initialize logger to write to both file and console
	logFile, err := os.OpenFile(filepath.Join(cfg.App.DataPath, "app.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	logger := utils.NewLogger(cfg.App.Debug, multiWriter)

	// Initialize database
	db, err := database.NewSQLite(cfg.Database.Path)
	if err != nil {
		logger.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// Run migrations
	if err := database.RunMigrations(db, logger); err != nil {
		logger.Fatal("Failed to run migrations:", err)
	}

	// Create manager
	manager := core.NewManager(cfg, db, logger)

	// Start web server
	server := handlers.NewServer(cfg, manager, logger)

	// Handle shutdown gracefully
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := server.Start(); err != nil {
			logger.Fatal("Server failed to start:", err)
		}
	}()

	manager.StartScheduler()

	logger.Info("Reel started successfully on port", cfg.App.Port)

	// Wait for interrupt
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	logger.Info("Shutting down...")
	manager.Stop()
	server.Stop(ctx)
}
