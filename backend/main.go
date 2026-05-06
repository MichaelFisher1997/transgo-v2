package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	// Load database configuration
	cfg := NewConfig()

	// Connect to the database
	database, err := NewDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Initialize repository
	repo := NewRepository(database)
	hub := NewEventHub()
	indexer := NewMediaIndexer(repo, hub)

	cleanupOrphanFFmpegProcesses()
	go cleanupStaleTempOutputs()

	// Initialize encoder worker pool
	encoder := NewEncoder(repo, hub, indexer)
	encoder.SetConcurrency(loadEncoderConcurrency(repo))

	// Initialize API handlers
	api := NewAPIHandlers(repo, encoder, indexer, hub)
	indexer.Start()

	// Setup HTTP routes
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/folders", api.GetFoldersHandler)
	mux.HandleFunc("/api/folders/", api.GetFolderContentsHandler)
	mux.HandleFunc("/api/files", api.DeleteMediaFileHandler)
	mux.HandleFunc("/api/probe", api.ProbeSourcesHandler)
	mux.HandleFunc("/api/jobs/batch", api.CreateJobsBatchHandler)
	mux.HandleFunc("/api/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			api.CreateJobHandler(w, r)
		} else if r.Method == http.MethodGet {
			api.GetJobsHandler(w, r)
		} else if r.Method == http.MethodDelete {
			api.ClearJobsHandler(w, r)
		} else {
			respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	})
	mux.HandleFunc("/api/jobs/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		parts := strings.Split(path, "/")
		if len(parts) < 4 {
			respondError(w, http.StatusBadRequest, "Invalid path")
			return
		}

		action := ""
		if len(parts) >= 5 {
			action = parts[4]
		}

		switch action {
		case "cancel":
			api.CancelJobHandler(w, r)
		case "retry":
			api.RetryJobHandler(w, r)
		case "cleanup":
			api.CleanupJobHandler(w, r)
		case "discard":
			api.DiscardJobHandler(w, r)
		case "events":
			api.GetJobEventsHandler(w, r)
		default:
			if r.Method == http.MethodGet {
				api.GetJobHandler(w, r)
			} else if r.Method == http.MethodDelete {
				api.DeleteJobHandler(w, r)
			} else {
				respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
			}
		}
	})
	mux.HandleFunc("/api/results", api.GetResultsHandler)
	mux.HandleFunc("/api/results/", api.GetResultHandler)
	mux.HandleFunc("/api/stats", api.GetStatsHandler)
	mux.HandleFunc("/api/settings", api.SettingsHandler)
	mux.HandleFunc("/api/health", api.HealthHandler)
	mux.HandleFunc("/api/encoder/preflight", api.PreflightHandler)
	mux.HandleFunc("/api/events", hub.ServeHTTP)

	// Serve frontend static files
	frontendDist := "./frontend/dist"
	_, hasFrontend := os.Stat(frontendDist)

	// Main handler: API routes first, then SPA fallback
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API routes go to mux
		if strings.HasPrefix(r.URL.Path, "/api/") {
			mux.ServeHTTP(w, r)
			return
		}

		// Static assets: serve if file exists
		if hasFrontend == nil {
			assetPath := frontendDist + r.URL.Path
			if info, err := os.Stat(assetPath); err == nil && !info.IsDir() {
				http.ServeFile(w, r, assetPath)
				return
			}
			// SPA fallback: serve index.html for all non-API routes
			http.ServeFile(w, r, frontendDist+"/index.html")
			return
		}

		// No frontend built
		serveFrontend(w, r)
	})

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{Addr: ":" + port, Handler: handler}
	serverErr := make(chan error, 1)
	log.Printf("Server listening on :%s", port)
	go func() {
		serverErr <- server.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("Received %s, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("HTTP shutdown failed: %v", err)
		}
		encoder.Stop()
		if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed during shutdown: %v", err)
		}
	case err := <-serverErr:
		encoder.Stop()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed to start: %v", err)
		}
	}
}
