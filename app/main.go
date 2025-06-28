package main

import (
	"embed"
	"log"
	"net/http"
	"os"
)

//go:embed views
var viewsFS embed.FS

//go:embed static
var staticFS embed.FS

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

	// Initialize handlers
	handlers := NewHandlers(repo)

	// Setup HTTP routes
	mux := http.NewServeMux()

	// Serve static files first
	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))

	// Application routes
	mux.HandleFunc("GET /movies", handlers.MoviesHandler)
	mux.HandleFunc("GET /tvshows", handlers.TVShowsHandler)
	mux.HandleFunc("GET /tvshow/{id}/season/{seasonNum}", handlers.SeasonHandler)
	mux.HandleFunc("GET /tvshow/{id}", handlers.TVShowHandler)
	mux.HandleFunc("GET /media/{id}", handlers.MediaHandler)
	mux.HandleFunc("POST /scan", handlers.ScanHandler)
	mux.HandleFunc("GET /hello", handlers.HelloHandler)
	mux.HandleFunc("GET /standalone", handlers.StandaloneHandler)

	// Handle root route
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		handlers.LibraryHandler(w, r)
	}))

	// Automatically trigger a scan on startup
	moviesDir := os.Getenv("MOVIES_DIR")
	if moviesDir == "" {
		moviesDir = "./media/movies" // Default to a local path
	}
	tvDir := os.Getenv("TV_DIR")
	if tvDir == "" {
		tvDir = "./media/tv" // Default to a local path
	}
	log.Println("Initiating media scan on startup...")
	go ScanMedia(repo, moviesDir, tvDir)

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// Other existing code in main.go can remain unchanged
