package main

import (
	"embed"
	"log"
	"net/http"
	"os"

	"github.com/jmoiron/sqlx"
)

//go:embed static
var staticFiles embed.FS

var db *sqlx.DB

func main() {
	// Initialize database
	cfg := NewConfig()
	var err error
	db, err = NewDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create repository
	repo := NewRepository(db)

	// Create handlers
	handlers := NewHandlers(repo)

	// Set up routes
	http.HandleFunc("/", handlers.LibraryHandler)
	http.HandleFunc("/movies", handlers.MoviesHandler)
	http.HandleFunc("/tvshows", handlers.TVShowsHandler)
	http.HandleFunc("/tvshow/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tvshow/" {
			http.Redirect(w, r, "/tvshows", http.StatusFound)
			return
		}

		// Check if it's a season request
		if len(r.URL.Path) >= 15 && r.URL.Path[8:15] == "/season" {
			handlers.SeasonHandler(w, r)
			return
		}

		handlers.TVShowHandler(w, r)
	})
	http.HandleFunc("/media/", handlers.MediaHandler)
	http.HandleFunc("/scan", handlers.ScanHandler)
	http.HandleFunc("/hello", handlers.HelloHandler)
	http.HandleFunc("/standalone", handlers.StandaloneHandler)

	// Serve static files
	http.Handle("/static/", http.FileServer(http.FS(staticFiles)))

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
