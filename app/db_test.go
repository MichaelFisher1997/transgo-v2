package main

import (
	"context"
	"testing"
	"app/models"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB creates a test database with the schema
func setupTestDB(t *testing.T) *sqlx.DB {
	db, err := sqlx.Connect("sqlite3", ":memory:")
	require.NoError(t, err)

	// Create schema
	_, err = db.Exec(`
		CREATE TABLE media (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			path TEXT NOT NULL UNIQUE,
			media_type TEXT NOT NULL,
			file_size INTEGER NOT NULL,
			file_extension TEXT NOT NULL,
			poster_path TEXT,
			rating TEXT,
			year INTEGER,
			description TEXT
		)
	`)
	require.NoError(t, err)

	return db
}

// insertTestMedia inserts test media into the database
func insertTestMedia(t *testing.T, db *sqlx.DB) []models.Media {
	testMedia := []models.Media{
		{
			Title:         "Test Movie 1",
			Path:          "/media/movies/test1.mp4",
			MediaType:     models.MediaTypeMovie,
			FileSize:      1024,
			FileExtension: ".mp4",
		},
		{
			Title:         "Test Movie 2",
			Path:          "/media/movies/test2.mp4",
			MediaType:     models.MediaTypeMovie,
			FileSize:      2048,
			FileExtension: ".mp4",
		},
	}

	for i, media := range testMedia {
		query := `INSERT INTO media (title, path, media_type, file_size, file_extension)
			VALUES (?, ?, ?, ?, ?)`
		result, err := db.Exec(query, media.Title, media.Path, media.MediaType, media.FileSize, media.FileExtension)
		require.NoError(t, err)
		
		id, err := result.LastInsertId()
		require.NoError(t, err)
		
		testMedia[i].ID = id
	}

	return testMedia
}

func TestNewRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestGetAllMedia(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	
	// Test with empty database
	media, err := repo.GetAllMedia(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, media)

	// Insert test data
	expectedMedia := insertTestMedia(t, db)

	// Test with data
	media, err = repo.GetAllMedia(context.Background())
	assert.NoError(t, err)
	assert.Len(t, media, len(expectedMedia))
	
	// Verify the data
	for i, m := range media {
		assert.Equal(t, expectedMedia[i].ID, m.ID)
		assert.Equal(t, expectedMedia[i].Title, m.Title)
		assert.Equal(t, expectedMedia[i].Path, m.Path)
		assert.Equal(t, expectedMedia[i].MediaType, m.MediaType)
		assert.Equal(t, expectedMedia[i].FileSize, m.FileSize)
		assert.Equal(t, expectedMedia[i].FileExtension, m.FileExtension)
	}
}

func TestGetMediaByPath(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	
	// Insert test data
	expectedMedia := insertTestMedia(t, db)

	// Test with existing path
	media, err := repo.GetMediaByPath(context.Background(), expectedMedia[0].Path)
	assert.NoError(t, err)
	assert.Equal(t, expectedMedia[0].ID, media.ID)
	assert.Equal(t, expectedMedia[0].Title, media.Title)
	assert.Equal(t, expectedMedia[0].Path, media.Path)
	
	// Test with non-existing path
	_, err = repo.GetMediaByPath(context.Background(), "/non/existing/path")
	assert.Error(t, err)
}

func TestSaveMedia(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	
	// Create test media
	testMedia := &models.Media{
		Title:         "New Test Movie",
		Path:          "/media/movies/new_test.mp4",
		MediaType:     models.MediaTypeMovie,
		FileSize:      4096,
		FileExtension: ".mp4",
	}
	
	// Save media
	id, err := repo.SaveMedia(context.Background(), testMedia)
	assert.NoError(t, err)
	assert.Greater(t, id, int64(0))
	
	// Verify it was saved correctly
	savedMedia, err := repo.GetMediaByPath(context.Background(), testMedia.Path)
	assert.NoError(t, err)
	assert.Equal(t, id, savedMedia.ID)
	assert.Equal(t, testMedia.Title, savedMedia.Title)
	assert.Equal(t, testMedia.Path, savedMedia.Path)
	assert.Equal(t, testMedia.MediaType, savedMedia.MediaType)
	assert.Equal(t, testMedia.FileSize, savedMedia.FileSize)
	assert.Equal(t, testMedia.FileExtension, savedMedia.FileExtension)
	
	// Test duplicate path
	duplicateMedia := &models.Media{
		Title:         "Duplicate Path",
		Path:          testMedia.Path, // Same path as before
		MediaType:     models.MediaTypeMovie,
		FileSize:      8192,
		FileExtension: ".mp4",
	}
	
	_, err = repo.SaveMedia(context.Background(), duplicateMedia)
	assert.Error(t, err) // Should fail due to unique constraint
}
