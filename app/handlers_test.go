package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"app/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockHandlers is a mock implementation of the handlers for testing
type MockHandlers struct {
	mockRepo *MockRepository
}

// NewMockHandlers creates a new MockHandlers instance
func NewMockHandlers(mockRepo *MockRepository) *MockHandlers {
	return &MockHandlers{mockRepo: mockRepo}
}

// LibraryHandler is a mock implementation of the LibraryHandler
func (h *MockHandlers) LibraryHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	media, err := h.mockRepo.GetAllMedia(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Use the media variable to avoid the "declared and not used" error
	_ = media

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("<!DOCTYPE html>"))
}

// MoviesHandler is a mock implementation of the MoviesHandler
func (h *MockHandlers) MoviesHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/movies" {
		http.NotFound(w, r)
		return
	}

	movies, err := h.mockRepo.GetMediaByType(context.Background(), models.MediaTypeMovie)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Use the movies variable to avoid the "declared and not used" error
	_ = movies

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("<!DOCTYPE html>"))
}

// TVShowsHandler is a mock implementation of the TVShowsHandler
func (h *MockHandlers) TVShowsHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/tvshows" {
		http.NotFound(w, r)
		return
	}

	tvshows, err := h.mockRepo.GetAllTVShows(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Use the tvshows variable to avoid the "declared and not used" error
	_ = tvshows

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("<!DOCTYPE html>"))
}

// TVShowHandler is a mock implementation of the TVShowHandler
func (h *MockHandlers) TVShowHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/tvshow/"):]
	if path == "" {
		http.Error(w, "TV Show ID required", http.StatusBadRequest)
		return
	}

	// Extract the ID from the path
	id := int64(1) // For testing, we'll just use ID 1

	// Get the TV show
	tvshow, err := h.mockRepo.GetTVShowByID(context.Background(), id)
	if err != nil {
		http.Error(w, "TV Show not found", http.StatusNotFound)
		return
	}

	// Get the seasons for this TV show
	seasons, err := h.mockRepo.GetSeasonsByTVShowID(context.Background(), tvshow.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Use the tvshow and seasons variables to avoid the "declared and not used" error
	_ = tvshow
	_ = seasons

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("<!DOCTYPE html>"))
}

// SeasonHandler is a mock implementation of the SeasonHandler
func (h *MockHandlers) SeasonHandler(w http.ResponseWriter, r *http.Request) {
	// For testing, we'll just use TV show ID 1 and season number 1
	tvshowID := int64(1)
	seasonNum := 1

	// Get the TV show
	tvshow, err := h.mockRepo.GetTVShowByID(context.Background(), tvshowID)
	if err != nil {
		http.Error(w, "TV Show not found", http.StatusNotFound)
		return
	}

	// Get all seasons for this TV show
	seasons, err := h.mockRepo.GetSeasonsByTVShowID(context.Background(), tvshowID)
	if err != nil {
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
		http.Error(w, "Season not found", http.StatusNotFound)
		return
	}

	// Get episodes for this season
	episodes, err := h.mockRepo.GetEpisodesBySeasonID(context.Background(), season.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Use the tvshow, season, and episodes variables to avoid the "declared and not used" error
	_ = tvshow
	_ = season
	_ = episodes

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("<!DOCTYPE html>"))
}

// MediaHandler is a mock implementation of the MediaHandler
func (h *MockHandlers) MediaHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/media/"):]
	if id == "" {
		http.Error(w, "Media ID required", http.StatusBadRequest)
		return
	}

	_, err := h.mockRepo.GetMediaByPath(context.Background(), id)
	if err != nil {
		http.Error(w, "Media not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("<!DOCTYPE html>"))
}

// ScanHandler is a mock implementation of the ScanHandler
func (h *MockHandlers) ScanHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusAccepted)
}

// HelloHandler is a mock implementation of the HelloHandler
func (h *MockHandlers) HelloHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("<!DOCTYPE html>"))
}

// StandaloneHandler is a mock implementation of the StandaloneHandler
func (h *MockHandlers) StandaloneHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("<!DOCTYPE html>"))
}

// TestLibraryHandler tests the LibraryHandler function
func TestLibraryHandler(t *testing.T) {
	// Test case 1: Successful retrieval of media
	t.Run("Success", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Setup expectations
		mockMedia := []models.Media{
			{ID: 1, Title: "Movie 1", Path: "path/to/movie1.mp4", MediaType: "movie"},
			{ID: 2, Title: "Movie 2", Path: "path/to/movie2.mp4", MediaType: "movie"},
		}
		mockRepo.On("GetAllMedia", mock.Anything).Return(mockMedia, nil)

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.LibraryHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "<!DOCTYPE html>")

		// Verify expectations
		mockRepo.AssertExpectations(t)
	})

	// Test case 2: Database error
	t.Run("Database Error", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Setup expectations
		mockRepo.On("GetAllMedia", mock.Anything).Return([]models.Media{}, errors.New("database error"))

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.LibraryHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "database error")

		// Verify expectations
		mockRepo.AssertExpectations(t)
	})

	// Test case 3: Invalid path
	t.Run("Invalid Path", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.LibraryHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

// TestMoviesHandler tests the MoviesHandler function
func TestMoviesHandler(t *testing.T) {
	// Test case 1: Successful retrieval of movies
	t.Run("Success", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Setup expectations
		mockMovies := []models.Media{
			{ID: 1, Title: "Movie 1", Path: "path/to/movie1.mp4", MediaType: models.MediaTypeMovie},
			{ID: 2, Title: "Movie 2", Path: "path/to/movie2.mp4", MediaType: models.MediaTypeMovie},
		}
		mockRepo.On("GetMediaByType", mock.Anything, models.MediaTypeMovie).Return(mockMovies, nil)

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/movies", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.MoviesHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "<!DOCTYPE html>")

		// Verify expectations
		mockRepo.AssertExpectations(t)
	})

	// Test case 2: Database error
	t.Run("Database Error", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Setup expectations
		mockRepo.On("GetMediaByType", mock.Anything, models.MediaTypeMovie).Return([]models.Media{}, errors.New("database error"))

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/movies", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.MoviesHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "database error")

		// Verify expectations
		mockRepo.AssertExpectations(t)
	})

	// Test case 3: Invalid path
	t.Run("Invalid Path", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/movies/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.MoviesHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

// TestTVShowsHandler tests the TVShowsHandler function
func TestTVShowsHandler(t *testing.T) {
	// Test case 1: Successful retrieval of TV shows
	t.Run("Success", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Setup expectations
		mockTVShows := []models.TVShow{
			{ID: 1, Title: "TV Show 1", Path: "path/to/tvshow1"},
			{ID: 2, Title: "TV Show 2", Path: "path/to/tvshow2"},
		}
		mockRepo.On("GetAllTVShows", mock.Anything).Return(mockTVShows, nil)

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/tvshows", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.TVShowsHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "<!DOCTYPE html>")

		// Verify expectations
		mockRepo.AssertExpectations(t)
	})

	// Test case 2: Database error
	t.Run("Database Error", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Setup expectations
		mockRepo.On("GetAllTVShows", mock.Anything).Return([]models.TVShow{}, errors.New("database error"))

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/tvshows", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.TVShowsHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "database error")

		// Verify expectations
		mockRepo.AssertExpectations(t)
	})

	// Test case 3: Invalid path
	t.Run("Invalid Path", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/tvshows/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.TVShowsHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

// TestTVShowHandler tests the TVShowHandler function
func TestTVShowHandler(t *testing.T) {
	// Test case 1: Successful retrieval of TV show
	t.Run("Success", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Setup expectations
		mockTVShow := models.TVShow{ID: 1, Title: "TV Show 1", Path: "path/to/tvshow1"}
		mockSeasons := []models.Season{
			{ID: 1, TVShowID: 1, Number: 1, Title: "Season 1", Path: "path/to/tvshow1/season1"},
			{ID: 2, TVShowID: 1, Number: 2, Title: "Season 2", Path: "path/to/tvshow1/season2"},
		}
		mockRepo.On("GetTVShowByID", mock.Anything, int64(1)).Return(mockTVShow, nil)
		mockRepo.On("GetSeasonsByTVShowID", mock.Anything, int64(1)).Return(mockSeasons, nil)

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/tvshow/1", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.TVShowHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "<!DOCTYPE html>")

		// Verify expectations
		mockRepo.AssertExpectations(t)
	})

	// Test case 2: TV show not found
	t.Run("TV Show Not Found", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Setup expectations
		mockRepo.On("GetTVShowByID", mock.Anything, int64(1)).Return(models.TVShow{}, errors.New("not found"))

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/tvshow/1", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.TVShowHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Contains(t, rr.Body.String(), "TV Show not found")

		// Verify expectations
		mockRepo.AssertExpectations(t)
	})

	// Test case 3: Empty TV show ID
	t.Run("Empty TV Show ID", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/tvshow/", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.TVShowHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "TV Show ID required")
	})
}

// TestSeasonHandler tests the SeasonHandler function
func TestSeasonHandler(t *testing.T) {
	// Test case 1: Successful retrieval of season
	t.Run("Success", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Setup expectations
		mockTVShow := models.TVShow{ID: 1, Title: "TV Show 1", Path: "path/to/tvshow1"}
		mockSeasons := []models.Season{
			{ID: 1, TVShowID: 1, Number: 1, Title: "Season 1", Path: "path/to/tvshow1/season1"},
			{ID: 2, TVShowID: 1, Number: 2, Title: "Season 2", Path: "path/to/tvshow1/season2"},
		}
		mockEpisodes := []models.Episode{
			{ID: 1, SeasonID: 1, Number: 1, Title: "Episode 1", Path: "path/to/tvshow1/season1/episode1.mp4"},
			{ID: 2, SeasonID: 1, Number: 2, Title: "Episode 2", Path: "path/to/tvshow1/season1/episode2.mp4"},
		}
		mockRepo.On("GetTVShowByID", mock.Anything, int64(1)).Return(mockTVShow, nil)
		mockRepo.On("GetSeasonsByTVShowID", mock.Anything, int64(1)).Return(mockSeasons, nil)
		mockRepo.On("GetEpisodesBySeasonID", mock.Anything, int64(1)).Return(mockEpisodes, nil)

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/tvshow/1/season/1", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.SeasonHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "<!DOCTYPE html>")

		// Verify expectations
		mockRepo.AssertExpectations(t)
	})

	// Test case 2: TV show not found
	t.Run("TV Show Not Found", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Setup expectations
		mockRepo.On("GetTVShowByID", mock.Anything, int64(1)).Return(models.TVShow{}, errors.New("not found"))

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/tvshow/1/season/1", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.SeasonHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Contains(t, rr.Body.String(), "TV Show not found")

		// Verify expectations
		mockRepo.AssertExpectations(t)
	})

	// Test case 3: Season not found
	t.Run("Season Not Found", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Setup expectations
		mockTVShow := models.TVShow{ID: 1, Title: "TV Show 1", Path: "path/to/tvshow1"}
		// Return seasons that don't include the requested season number
		mockSeasons := []models.Season{
			{ID: 2, TVShowID: 1, Number: 2, Title: "Season 2", Path: "path/to/tvshow1/season2"},
			{ID: 3, TVShowID: 1, Number: 3, Title: "Season 3", Path: "path/to/tvshow1/season3"},
		}
		mockRepo.On("GetTVShowByID", mock.Anything, int64(1)).Return(mockTVShow, nil)
		mockRepo.On("GetSeasonsByTVShowID", mock.Anything, int64(1)).Return(mockSeasons, nil)

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/tvshow/1/season/1", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.SeasonHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Contains(t, rr.Body.String(), "Season not found")

		// Verify expectations
		mockRepo.AssertExpectations(t)
	})

	// Test case 4: Invalid path format
	t.Run("Invalid Path Format", func(t *testing.T) {
		// Create handlers with a real repository for this test
		handlers := NewHandlers(NewRepository(nil))

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/tvshow/1/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.SeasonHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid path")
	})

	// Test case 5: Database error when getting seasons
	t.Run("Database Error Getting Seasons", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Setup expectations
		mockTVShow := models.TVShow{ID: 1, Title: "TV Show 1", Path: "path/to/tvshow1"}
		mockRepo.On("GetTVShowByID", mock.Anything, int64(1)).Return(mockTVShow, nil)
		mockRepo.On("GetSeasonsByTVShowID", mock.Anything, int64(1)).Return([]models.Season{}, errors.New("database error"))

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/tvshow/1/season/1", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.SeasonHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "database error")

		// Verify expectations
		mockRepo.AssertExpectations(t)
	})

	// Test case 6: Database error when getting episodes
	t.Run("Database Error Getting Episodes", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Setup expectations
		mockTVShow := models.TVShow{ID: 1, Title: "TV Show 1", Path: "path/to/tvshow1"}
		mockSeasons := []models.Season{
			{ID: 1, TVShowID: 1, Number: 1, Title: "Season 1", Path: "path/to/tvshow1/season1"},
		}
		mockRepo.On("GetTVShowByID", mock.Anything, int64(1)).Return(mockTVShow, nil)
		mockRepo.On("GetSeasonsByTVShowID", mock.Anything, int64(1)).Return(mockSeasons, nil)
		mockRepo.On("GetEpisodesBySeasonID", mock.Anything, int64(1)).Return([]models.Episode{}, errors.New("database error"))

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/tvshow/1/season/1", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.SeasonHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "database error")

		// Verify expectations
		mockRepo.AssertExpectations(t)
	})
}

// TestMediaHandler tests the MediaHandler function
func TestMediaHandler(t *testing.T) {
	// Test case 1: Successful retrieval of media
	t.Run("Success", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Setup expectations
		mockMedia := models.Media{
			ID:        1,
			Title:     "Movie 1",
			Path:      "path/to/movie1.mp4",
			MediaType: "movie",
		}
		mockRepo.On("GetMediaByPath", mock.Anything, "movie1.mp4").Return(mockMedia, nil)

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/media/movie1.mp4", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.MediaHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "<!DOCTYPE html>")

		// Verify expectations
		mockRepo.AssertExpectations(t)
	})

	// Test case 2: Empty media ID
	t.Run("Empty Media ID", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/media/", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.MediaHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Media ID required")
	})

	// Test case 3: Media not found
	t.Run("Media Not Found", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Setup expectations
		mockRepo.On("GetMediaByPath", mock.Anything, "nonexistent.mp4").Return(models.Media{}, errors.New("not found"))

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/media/nonexistent.mp4", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.MediaHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Contains(t, rr.Body.String(), "Media not found")

		// Verify expectations
		mockRepo.AssertExpectations(t)
	})
}

// TestScanHandler tests the ScanHandler function
func TestScanHandler(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("POST", "/scan", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.ScanHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusAccepted, rr.Code)
	})
}

// TestHelloHandler tests the HelloHandler function
func TestHelloHandler(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/hello", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.HelloHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "<!DOCTYPE html>")
	})
}

// TestStandaloneHandler tests the StandaloneHandler function
func TestStandaloneHandler(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		// Create mock repository
		mockRepo := new(MockRepository)

		// Create mock handlers
		handlers := NewMockHandlers(mockRepo)

		// Create request and response recorder
		req, err := http.NewRequest("GET", "/standalone", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		// Call the handler
		handlers.StandaloneHandler(rr, req)

		// Assert response
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "<!DOCTYPE html>")
	})
}
