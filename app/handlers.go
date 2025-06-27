package main

import (
	"context"
	"log"
	"net/http"
	"os" // Added for environment variables
	"strconv"
	"strings"

	"app/models"
	"app/views/pages"
)

// Handlers holds the repository dependencies
type Handlers struct {
	repo *Repository
}

// NewHandlers creates a new Handlers instance
func NewHandlers(repo *Repository) *Handlers {
	return &Handlers{repo: repo}
}

// LibraryHandler handles the main library page
func (h *Handlers) LibraryHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	pages.Home().Render(context.Background(), w)
}

// MoviesHandler handles the movies page
func (h *Handlers) MoviesHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/movies" {
		http.NotFound(w, r)
		return
	}

	movies, err := h.repo.GetMediaByType(context.Background(), "movie")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pages.Movies(movies).Render(context.Background(), w)
}

// TVShowsHandler handles the TV shows page
func (h *Handlers) TVShowsHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/tvshows" {
		http.NotFound(w, r)
		return
	}

	tvshows, err := h.repo.GetAllTVShows(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pages.TVShows(tvshows).Render(context.Background(), w)
}

// TVShowHandler handles the TV show detail page
func (h *Handlers) TVShowHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/tvshow/")
	if path == "" {
		http.Error(w, "TV Show ID required", http.StatusBadRequest)
		return
	}

	// Extract the ID from the path
	idStr := strings.Split(path, "/")[0]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Printf("Error parsing TV Show ID '%s': %v", idStr, err)
		http.Error(w, "Invalid TV Show ID", http.StatusBadRequest)
		return
	}

	// Get the TV show
	tvshow, err := h.repo.GetTVShowByID(context.Background(), id)
	if err != nil {
		log.Printf("Error retrieving TV Show with ID %d: %v", id, err)
		http.Error(w, "TV Show not found", http.StatusNotFound)
		return
	}

	// Get the seasons for this TV show
	seasons, err := h.repo.GetSeasonsByTVShowID(context.Background(), tvshow.ID)
	if err != nil {
		log.Printf("Error retrieving seasons for TV Show ID %d: %v", tvshow.ID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Render the page
	err = pages.TVShow(tvshow, seasons).Render(context.Background(), w)
	if err != nil {
		log.Printf("Error rendering TV Show page for ID %d: %v", tvshow.ID, err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		return
	}
}

// SeasonHandler handles the season detail page
func (h *Handlers) SeasonHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/tvshow/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[1] != "season" {
		log.Printf("Invalid path format: %s", r.URL.Path)
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Extract the TV show ID and season number
	tvshowIDStr := parts[0]
	seasonNumStr := parts[2]

	tvshowID, err := strconv.ParseInt(tvshowIDStr, 10, 64)
	if err != nil {
		log.Printf("Error parsing TV Show ID '%s': %v", tvshowIDStr, err)
		http.Error(w, "Invalid TV Show ID", http.StatusBadRequest)
		return
	}

	seasonNum, err := strconv.Atoi(seasonNumStr)
	if err != nil {
		log.Printf("Error parsing Season Number '%s': %v", seasonNumStr, err)
		http.Error(w, "Invalid Season Number", http.StatusBadRequest)
		return
	}

	// Get the TV show
	tvshow, err := h.repo.GetTVShowByID(context.Background(), tvshowID)
	if err != nil {
		log.Printf("Error retrieving TV Show with ID %d: %v", tvshowID, err)
		http.Error(w, "TV Show not found", http.StatusNotFound)
		return
	}

	// Get all seasons for this TV show
	seasons, err := h.repo.GetSeasonsByTVShowID(context.Background(), tvshowID)
	if err != nil {
		log.Printf("Error retrieving seasons for TV Show ID %d: %v", tvshowID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Find the requested season
	var season *models.Season
	for i := range seasons {
		if seasons[i].Number == seasonNum {
			season = &seasons[i]
			break
		}
	}

	if season == nil {
		log.Printf("Season %d not found for TV Show ID %d", seasonNum, tvshowID)
		http.Error(w, "Season not found", http.StatusNotFound)
		return
	}

	// Get episodes for this season
	episodes, err := h.repo.GetEpisodesBySeasonID(context.Background(), season.ID)
	if err != nil {
		log.Printf("Error retrieving episodes for Season ID %d: %v", season.ID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Render the page
	err = pages.Season(tvshow, *season, episodes).Render(context.Background(), w)
	if err != nil {
		log.Printf("Error rendering Season page for TV Show ID %d, Season %d: %v", tvshowID, seasonNum, err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		return
	}
}

// MediaHandler handles the media detail page
func (h *Handlers) MediaHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/media/")
	if id == "" {
		http.Error(w, "Media ID required", http.StatusBadRequest)
		return
	}

	media, err := h.repo.GetMediaByPath(context.Background(), id)
	if err != nil {
		http.Error(w, "Media not found", http.StatusNotFound)
		return
	}
	pages.Media(media).Render(context.Background(), w)
}

// ScanHandler handles the media scan request
func (h *Handlers) ScanHandler(w http.ResponseWriter, r *http.Request) {
	moviesDir := os.Getenv("MOVIES_DIR")
	if moviesDir == "" {
		moviesDir = "./media/movies" // Default to a local path
	}
	tvDir := os.Getenv("TV_DIR")
	if tvDir == "" {
		tvDir = "./media/tv" // Default to a local path
	}

	go ScanMedia(h.repo, moviesDir, tvDir)
	w.WriteHeader(http.StatusAccepted)
}

// HelloHandler handles the hello world page
func (h *Handlers) HelloHandler(w http.ResponseWriter, r *http.Request) {
	pages.Hello().Render(context.Background(), w)
}

// StandaloneHandler handles the standalone hello world page
func (h *Handlers) StandaloneHandler(w http.ResponseWriter, r *http.Request) {
	pages.Standalone().Render(context.Background(), w)
}
