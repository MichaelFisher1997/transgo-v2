package testutils

import (
	"database/sql"
	"fmt"
	"transogov2/app/models"
)

// MockTVShow creates a TVShow model with test data
func MockTVShow() models.TVShow {
	return models.TVShow{
		ID:          1,
		Title:       "Test Show",
		Path:        "/test/path",
		Description: sql.NullString{String: "Test description", Valid: true},
		PosterPath:  sql.NullString{String: "/test/poster.jpg", Valid: true},
		Rating:      sql.NullString{String: "8.5", Valid: true},
		Year:        sql.NullInt64{Int64: 2023, Valid: true},
	}
}

// MockTVShowEmpty creates a TVShow with empty/null fields
func MockTVShowEmpty() models.TVShow {
	return models.TVShow{
		ID:          2,
		Title:       "Empty Show",
		Path:        "/empty/path",
		Description: sql.NullString{Valid: false},
		PosterPath:  sql.NullString{Valid: false},
		Rating:      sql.NullString{Valid: false},
		Year:        sql.NullInt64{Valid: false},
	}
}

// MockSeasons creates a slice of Season models for a TVShow
func MockSeasons(tvshowID int64, count int) []models.Season {
	seasons := make([]models.Season, count)
	for i := 0; i < count; i++ {
		seasons[i] = models.Season{
			ID:       int64(i + 1),
			TVShowID: tvshowID,
			Number:   i + 1,
			Title:    fmt.Sprintf("Season %d", i+1),
		}
	}
	return seasons
}
